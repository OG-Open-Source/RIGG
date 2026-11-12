// X 桌面 app 入口（MyGO 原生 UI）：視窗內容在 login 與 home 兩個 view 間
// 分流，啟動鏈預熱（startToken：代理解析→註冊表預載）與 quit flush。引擎
// 接縫（xSession／newSession）見 session.go；錯誤文案映射見 messages.go；
// 畫面見 view_login.go／view_home.go。
package main

import (
	"context"
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/graphql"
)

// app 是視窗呈現的應用狀態。所有欄位一律於 UI 執行緒讀寫：輸入處理器
// 同步改動；背景結果經 publish（正式路徑 win.Update＝主執行緒套用＋重繪）
// 回流——背景 goroutine 絕不直接改動本結構（MyGO 併發模型：goroutine 經
// Window.Update 改狀態）。
//
// spawn／apply／newSess／onLoginPersist 是 view↔engine 接縫的測試注入點
// （nil → 各 goAsync／publish／startSession／persistLogin 的正式路徑）；
// view 本身僅依賴 xSession 介面（session.go）。
type app struct {
	// state 是生命週期狀態（階段推進 proxy→credential 與收斂；Task 12）。
	state *AppState

	// —— 接縫注入點（測試替身；nil → 正式路徑）——
	spawn          func(fn func(ctx context.Context)) // 背景工作派發
	apply          func(fn func())                    // 主執行緒套用背景結果
	newSess        func() (xSession, error)           // 會話建構
	onLoginPersist func(sess xSession) error          // 登入成功首次落盤

	// —— bootstrap（startToken 結果；login 代理狀態列與錯誤區消費）——
	proxy    string // 解析出的代理 URL（空＝尚未就緒）
	proxyErr error  // 代理解析錯誤（fail-closed 呈現，D6）
	startErr error  // 指紋／註冊表／儲存層初始化錯誤（login 頁錯誤區）

	// —— login 畫面狀態 ——
	cookieInput string // 使用者貼上的整行 cookie（提交原值，不回顯衍生字串）
	loginBusy   bool   // 提交進行中（防抖閘門）
	loginErr    error  // 提交錯誤（errorText 映射後顯示；僅顯示不清除）

	// —— 會話（非 nil ⇒ home 畫面）——
	sess xSession
	auth *auth.Auth // 當前引擎會話的 XAuth（quit flush 用；engineSession 時設置）

	// —— home 畫面狀態 ——
	posts    []post   // 已顯示貼文（work_id 去重、首見順序）
	cursor   string   // 下一頁游標（空＝已到底）
	seen     []string // seenTweetIds：已顯示 work_id 累積（跨頁去重）
	homeBusy bool
	homeErr  error
	end      bool
	list     ui.ListState
}

// newApp 建構 app：生命週期狀態先以無儲存層起手（main 於 store 建構成功
// 後替換；測試逕用此形）。
func newApp() *app {
	return &app{state: NewAppState(nil)}
}

// view 是視窗內容：已登入呈 home，否則 login（成功後自動切換，CUJ 1）。
func (a *app) view(c *ui.Context) {
	if a.sess != nil {
		a.homeView(c)
		return
	}
	a.loginView(c)
}

// goAsync 以背景工作執行 fn：正式路徑經 AppState.Go（生命週期 context，
// Close 時收斂後才 flush）；測試注入同步或非同步替身。
func (a *app) goAsync(fn func(ctx context.Context)) {
	if a.spawn != nil {
		a.spawn(fn)
		return
	}
	a.state.Go(fn)
}

// publish 於主執行緒套用背景結果：正式路徑為 win.Update（main 於視窗建立
// 後綁定）；測試注入同步替身。綁定前不會有背景工作（startToken 於綁定後
// 才派發）。
func (a *app) publish(fn func()) {
	if a.apply != nil {
		a.apply(fn)
		return
	}
	fn()
}

// startSession 經接縫建構新會話：正式路徑 newSession（login 流以空行組裝，
// 備料與驗證由 Validate 完成）；測試注入替身。組裝錯誤（代理 fail-closed、
// 註冊表載入）由 login 頁錯誤區呈現。
func (a *app) startSession() (xSession, error) {
	if a.newSess != nil {
		return a.newSess()
	}
	return newSession("", a.proxy)
}

// persistLogin 於登入成功後完成首次落盤（契約 2）：正式路徑經
// AppState.Flush 冪等寫入（Save 失敗記於 AppState.Err，不阻擋登入）；
// 測試注入記錄替身（錯誤路徑「未觸發任何落盤／清除」的斷言點）。
func (a *app) persistLogin(sess xSession) error {
	if a.onLoginPersist != nil {
		return a.onLoginPersist(sess)
	}
	es, ok := sess.(*engineSession)
	if !ok || es.auth == nil {
		return nil // 替身會話：無引擎憑證可落盤
	}
	a.state.Flush(es.auth)
	return nil
}

// startToken 於視窗就緒後預熱啟動鏈（AppState bootstrap：ResolveProxy →
// Advance → 憑證就緒交 login 流；fingerprint.Get() 程序內單一真源、無錯誤
// 路徑）：代理解析與註冊表預載在背景進行，結果經 publish 回 UI 執行緒。
// 任何失敗呈現於 login 頁錯誤區（fail-closed，D6／REQ-INV-02）。
func (a *app) startToken() {
	a.goAsync(func(ctx context.Context) {
		proxy, perr := ResolveProxy()
		var rerr error
		if perr == nil {
			if _, err := graphql.LoadRegistry(registryPath); err != nil {
				rerr = err
			}
		}
		a.publish(func() {
			if perr != nil {
				a.proxyErr = perr
			} else {
				a.proxy = proxy
				a.state.Advance() // proxy 段就緒 → credential 段（憑證就緒於 login 流完成）
			}
			if rerr != nil {
				a.startErr = rerr
			}
		})
	})
}

func main() {
	a := newApp()
	store, err := NewCookieStore("", cookieLineKey)
	if err != nil {
		a.startErr = err // 儲存層不可用：login 頁錯誤區呈現（fail-closed）
	} else {
		engineStore = store
		a.state = NewAppState(store)
	}
	// quit handler：同步 flush（OnBeforeQuit 須於回呼內完成，含 OS 結束
	// 會話的 SessionEnding）；僅落盤、絕不清除（REQ-LOG-042/043）。
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		a.state.Flush(a.auth)
	})
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:     "X",
			Width:     480,
			Height:    720,
			MinWidth:  360,
			MinHeight: 480,
			// Opens where the user left it last time.
			StateKey: "main",
			// The window shows the interface MyGo draws, not a web page.
			Content: ui.View(a.view),
		})
		a.apply = win.Update // 背景結果自此經主執行緒套用＋重繪
		a.spawn = a.state.Go // 背景工作自此掛生命週期 context（收斂後 flush）
		a.startToken()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
