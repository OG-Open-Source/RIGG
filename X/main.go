// X 桌面 app 入口（MyGO 原生 UI）：視窗內容在 login 與 home 兩個 view 間
// 分流，啟動鏈預熱（startToken：代理解析→註冊表預載）與 quit flush。引擎
// 接縫（xSession／newSessionFor）見 session.go；錯誤文案映射見 messages.go；
// 畫面見 view_login.go／view_home.go。
package main

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"slices"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/graphql"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// 響應式斷點（DIP；Task 14／D20 佈局裁決）——layoutBand 的帶界。
const (
	bpTablet  = 768  // <768 單欄（Select 下拉）
	bpLaptop  = 1024 // 768–1023 側欄兩欄；≥1024 完整版型＋detail 並列
	bpDesktop = 1200 // ≥1200 寬內容區（卡片 max-width 640 置中）
)

// login view 變體（Task 14）：帳號選單的新增帳號／失效重登入口共用
// login view，模式旗標分流（view 路由：sess 非 nil 且 mode 為 none 呈
// home，其餘呈 login view 變體）。
const (
	loginModeNone    = 0 // 首次登入（原形）
	loginModeAdd     = 1 // 新增帳號（home 的帳號選單進入）
	loginModeRelogin = 2 // 失效帳號重登（重新貼上 Cookie）
)

// layoutBand 回窗寬對應的佈局帶 0..3（測試斷言用）：<768 單欄（0）、
// 768–1023 側欄兩欄（1）、1024–1199 完整版型＋detail 並列（2）、≥1200
// 寬內容區置中（3）。
func (a *app) layoutBand(width float32) int {
	switch {
	case width < bpTablet:
		return 0
	case width < bpLaptop:
		return 1
	case width < bpDesktop:
		return 2
	default:
		return 3
	}
}

// app 是視窗呈現的應用狀態。所有欄位一律於 UI 執行緒讀寫：輸入處理器
// 同步改動；背景結果經 publish（正式路徑 win.Update＝主執行緒套用＋重繪）
// 回流——背景 goroutine 絕不直接改動本結構（MyGO 併發模型：goroutine 經
// Window.Update 改狀態）。唯一例外是生命週期錯誤記錄：背景工作逕呼叫
// a.state.setErr（AppState.Err 鎖內同步，見 app_state.go），不觸及本結構
// 其餘欄位。
//
// spawn／apply／newSess／onLoginPersist 是 view↔engine 接縫的測試注入點
// （nil → 各 goAsync／publish／startSession／persistLogin 的正式路徑）；
// view 本身僅依賴 xSession 介面（session.go）。
type app struct {
	// state 是生命週期狀態（階段推進 proxy→credential 與收斂；Task 12）。
	state *AppState

	// —— 接縫注入點（測試替身；nil → 正式路徑）——
	spawn          func(fn func(ctx context.Context))                            // 背景工作派發
	apply          func(fn func())                                               // 主執行緒套用背景結果
	newSess        func() (xSession, error)                                      // 會話建構
	onLoginPersist func(sess xSession, info viewerInfo, cookieLine string) error // 登入成功首次落盤

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

	// activeStore 是當前活躍帳號的憑證庫儲存層（quit flush 目標；§8.3 寫回
	// 帳號化）：首次登入成功即接線（submitLogin default 分支，final review
	// F1）、切換成功時替換（runSwitch）。nil ⇒ 登入前／登出後——quit
	// flush 沿用 AppState 既有儲存層（雙寫期的 v1 行為）。
	activeStore auth.CookieStore

	// —— 帳號切換（Task 6；設計 §8.4／§8.5）——
	// gen 是活躍會話世代：每次成功切換 +1；feed 的 publish 回寫前比對
	// 捕捉值，不一致即整批丟棄（D18 世代邊界，Review Focus 1）。
	gen int
	// sessCtx／sessCancel 是活躍會話的 context 子樹（state.Context() 的子
	// 樹）：feed 背景工作經其派發（goSession，會話確立或舊子樹被斬斷後
	// 的首次派發重建）；switchTo 步驟 3 以 sessCancel 斬斷在途（冪等）；
	// Close 經父 context 收斂並等待。
	sessCtx    context.Context
	sessCancel context.CancelFunc
	// accounts 是 warmup 載入的帳號清單快照（帳號選單 Task 8 消費；
	// warmup 冪等可重跑）。accInvalid 標記切換驗證失敗的帳號（secret
	// 保留、僅呈現失效；Task 8 消費）。switchErr 是最近一次切換失敗的
	// 錯誤（帳號選單錯誤區呈現，Task 8 接線）。switchBusy 期間禁再入。
	accounts   []Account
	accInvalid map[string]bool
	switchErr  error
	switchBusy bool
	// creds 是憑證有效性 TTL 快取（verifyTTL 內切換免重驗，§8.5）。
	creds *credCache

	// —— home 畫面狀態（Task 8：feeds 三槽；Task 10：搜尋）——
	// feeds 是固定三槽的時間線狀態（非 map）：For You／Following／
	// search（feedSearch 由 Task 10 的搜尋結果使用——ephemeral，見
	// view_search.go）。各槽完全隔離——posts／cursor／seen／end／busy／
	// err／list 各自累積，切換 tab 不互相污染；seen 上限 seenCap
	// （Review Focus 4）。activeFeed 是活躍 tab 索引（ui.Tabs 的 *int
	// 裸 int；MyGO Tabs 會把越界選擇鉗回範圍，feedSearch 不寫入此欄）。
	feeds      [3]*timelineState
	activeFeed int
	// searchInput 是頂列搜尋框的輸入緩衝（ui.TextInput *string 綁定；
	// 提交值以 submitSearch 的 TrimSpace 為準，空 query 忽略）。
	// searchOpen 標示當前檢視搜尋結果槽（feedSearch）——點回任一 tab
	// 即關閉。searchQuery 是最後成功提交的 query（錯誤重試＝原 query
	// 再提交；不做搜尋歷史，Non-Goals）。
	searchInput string
	searchOpen  bool
	searchQuery string

	// —— 展開檢視（Task 13；設計 §8.6 播放器生命週期）——
	// detail 是當前展開的貼文（nil＝無）；player 是單一活躍播放器
	//（nil＝無——開新帖先 Close 舊的，收合／切換帳號經 closeDetail 釋
	// 放）；playerPath 是 D24 video 暫存檔路徑（收合時隨 player 刪除）；
	// playerLoading 是 D24 的播放媒體下載中旗標（video 懶建立／gif 自動
	// 播放的過渡態）；dmedia 是大圖／播放器錯誤的載入狀態（detailView
	// 消費，openDetail 起手歸零）。行為面見 view_detail.go。
	detail        *post
	player        player
	playerPath    string
	playerLoading bool
	dmedia        detailMedia

	// —— 帳號選單（Task 14；D20 頂列佈局）——
	// activeID 是活躍帳號 user_id 的記憶體快照（選單勾選、側欄選擇與
	// 「移除活躍帳號 → 登出」的判定）：login 成功／switch 成功時設置、
	// logout 清除、warmup 自索引 Current 同步。sideSel／selectAcc 是側
	// 欄與窄帶下拉的選擇呈現（syncAccountSelection 同步；Sidebar／
	// Select 的 id 語義見 view_home.go）。
	activeID  string
	sideSel   string
	selectAcc string
	// loginMode 是 login view 的變體（loginModeNone／Add／Relogin）；
	// loginUserID 是重登目標（beginLoginVariant 設置、resetLoginForm
	// 復位）。
	loginMode   int
	loginUserID string
	// removeOpen／removeUserID 是「移除 @xxx」的確認 Modal（二次確認後
	// removeConfirm 執行；Modal 開關即 *bool）。
	removeOpen   bool
	removeUserID string
	// offResize 是視窗 OnResize 的解除函式（main 註冊處留存；視窗與
	// app 同生命週期，無需解除）。
	offResize func()
}

// newApp 建構 app：生命週期狀態先以無儲存層起手（main 於 store 建構成功
// 後替換；測試逕用此形）。
func newApp() *app {
	a := &app{
		state:      NewAppState(nil),
		accInvalid: map[string]bool{},
		creds:      &credCache{},
		feeds:      [3]*timelineState{{}, {}, {}},
	}
	// 列識別（ListState.Key）：以 work_id 為 row identity——posts 截尾
	//（D26）與回填時列位移不破壞捲動語義（ui.txt:5770-5775）。
	for _, s := range a.feeds {
		s.list.Key = func(i int) any { return s.posts[i].WorkID }
	}
	return a
}

// accountByID 回帳號快照中指定 user_id 的帳號（帳號選單／側欄／重登入口
// 的標籤與目標解析）。
func (a *app) accountByID(userID string) (Account, bool) {
	for _, acc := range a.accounts {
		if acc.UserID == userID {
			return acc, true
		}
	}
	return Account{}, false
}

// syncAccountSelection 使側欄選擇（sideSel）與窄帶下拉（selectAcc）呈現
// 活躍帳號（切換成功／失敗回滾／登出／warmup 後的呈現同步）；無活躍帳
// 號（或不在快照）時復位。呈現欄位僅此同（UI 執行緒）寫入。
func (a *app) syncAccountSelection() {
	if acc, ok := a.accountByID(a.activeID); ok {
		a.sideSel = acc.UserID
		a.selectAcc = "@" + acc.ScreenName
		return
	}
	a.sideSel = ""
	a.selectAcc = ""
}

// upsertAccount 以記憶體快照同步單一帳號（落盤由呼叫端完成）：同 UserID
// 覆蓋，否則依新增序追加（與索引 Order 同形）。login／新增帳號成功後的
// 快照維護（選單即刻可見，免重跑 warmup）。
func (a *app) upsertAccount(acc Account) {
	for i := range a.accounts {
		if a.accounts[i].UserID == acc.UserID {
			a.accounts[i] = acc
			return
		}
	}
	a.accounts = append(a.accounts, acc)
}

// beginLoginVariant 進入 login view 變體（新增帳號／失效重登）：預設模
// 式與目標、清輸入與錯誤；回 home 走 loginView 的「返回」入口。
func (a *app) beginLoginVariant(mode int, userID string) {
	a.loginMode = mode
	a.loginUserID = userID
	a.cookieInput = ""
	a.loginErr = nil
}

// resetLoginForm 復位 login 表單（變體返回或完成後）：回到首次登入形、
// 清輸入與錯誤。
func (a *app) resetLoginForm() {
	a.beginLoginVariant(loginModeNone, "")
}

// view 是視窗內容：已登入且非 login 變體（新增帳號／失效重登，Task 14）
// 呈 home，否則 login（變體由 loginView 分流；成功後自動切換，CUJ 1）。
func (a *app) view(c *ui.Context) {
	if a.sess != nil && a.loginMode == loginModeNone {
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

// goSession 以活躍會話子樹派發 feed 背景工作（§8.4 世代邊界）：正式路徑
// 經 AppState.GoSession 掛 sessCtx（state.Context() 的子樹）——switchTo
// 步驟 3 以 sessCancel 斬斷在途、Close 經父 context 收斂並等待。子樹隨
// 會話確立而重建：舊子樹已取消（切換後）或尚未建立（login 後首載）時建
// 新子樹——取消的子樹不得派發新工作（否則即刻 done）。測試注入 spawn 時
// 沿用替身（同步／佇列控制；世代檢查是 app 層的雙保險，不依賴 ctx）。
func (a *app) goSession(fn func(ctx context.Context)) {
	if a.spawn != nil {
		a.spawn(fn)
		return
	}
	if a.sessCancel == nil || a.sessCtx.Err() != nil {
		a.sessCtx, a.sessCancel = context.WithCancel(a.state.Context())
	}
	a.state.GoSession(a.sessCtx, fn)
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

// startSession 經接縫建構新會話：正式路徑 assembleSession（login 流此時
// 尚無 Account——空行、不接儲存層組裝，備料與驗證由 Validate 完成；會話
// 本身不帶儲存層，活躍儲存層由 submitLogin default 分支於成功後接線
// activeStore——final review F1 最小接線裁決）；測試注入替身。組裝錯誤
// （代理 fail-closed、註冊表載入）由 login 頁錯誤區呈現。
func (a *app) startSession() (xSession, error) {
	if a.newSess != nil {
		return a.newSess()
	}
	return assembleSession("", a.proxy, nil)
}

// persistLogin 於登入成功後完成首次落盤（契約 2）：以 Validate 回傳的身分
// 建 Account 落盤（REQ-WEB-069：身分與憑證綁定；REQ-WEB-070：同 UserID 再
// 匯入＝saveAccount 覆蓋同鍵）、索引 Order 追加且 Current 指向該帳號，並
// dualWrite 同步寫 v1 舊鍵（降級雙寫期）；其後正式路徑經 AppState.Flush 冪
// 等寫入引擎憑證。錯誤一律記於 AppState.Err（setErr，同 Flush 自記語義：
// 首錄不覆蓋）、不回傳不阻擋登入；Flush 無條件執行——契約 2 的引擎憑證
// 首存（主項）不遭帳號寫／雙寫（降級次要項）錯誤閘門。帳號寫之間仍依序
// 把關：帳號鍵未落盤或索引讀失敗即不寫索引（索引不得指向缺失帳號、不得
// 以殘缺 Order 覆寫真源）。測試注入記錄替身（錯誤路徑「未觸發任何落盤／
// 清除」的斷言點）。
func (a *app) persistLogin(sess xSession, info viewerInfo, cookieLine string) error {
	if a.onLoginPersist != nil {
		return a.onLoginPersist(sess, info, cookieLine)
	}
	es, ok := sess.(*engineSession)
	if !ok || es.auth == nil {
		return nil // 替身會話：無引擎憑證可落盤
	}
	ctx := context.Background()
	acc := Account{UserID: info.UserID, ScreenName: info.ScreenName, Name: info.Name, CookieLine: cookieLine}
	if err := saveAccount(ctx, acc); err != nil {
		a.state.setErr(err)
	} else if idx, err := loadIndex(ctx); err != nil {
		a.state.setErr(err)
	} else {
		order := []string(nil)
		if idx != nil {
			order = idx.Order
		}
		if !slices.Contains(order, acc.UserID) {
			order = append(order, acc.UserID)
		}
		if err := saveIndex(ctx, accountIndex{Version: accountIndexVersion, Order: order, Current: acc.UserID}); err != nil {
			a.state.setErr(err)
		}
	}
	if err := dualWrite(cookieLine); err != nil {
		a.state.setErr(err) // 訊息已含「migrate: 雙寫舊鍵失敗」前綴，不含 cookie 值（REQ-INV-01）
	}
	// Flush 無條件（契約 2 主項）：Save 失敗由 Flush 自記 AppState.Err。
	// 目標沿會話接線的儲存層（login 流未接線 → nil → AppState 既有目標，
	// v1 舊鍵，降級雙寫期）。
	a.state.Flush(es.auth, es.auth.Store())
	return nil
}

// —— 帳號切換（Task 6；設計 §8.4 序列 1–8、§8.5）——

// switchSnapshot 是 switchTo 於 UI 執行緒捕捉的現役會話快照：切換背景全
// 程僅用快照，不在背景 goroutine 讀共享欄位（app 併發模型：背景結果一律
// 經 publish 回流）。
type switchSnapshot struct {
	sess   xSession           // 現役會話（成功時步驟 6 Close；失敗不動）
	auth   *auth.Auth         // 現役引擎憑證（步驟 1 flush 的行來源）
	store  auth.CookieStore   // 現役活躍儲存層（步驟 1 flush 目標）
	proxy  string             // 會話組裝的代理（步驟 4）
	cancel context.CancelFunc // 現役會話子樹（步驟 3 斬斷；冪等）
}

// switchTo 切換活躍帳號（D12）：UI 執行緒入口——防再入（switchBusy）後
// 捕捉現役快照，序列 1–8 經 goAsync 背景執行；狀態替換一律經 publish 回
// UI 執行緒。失敗收斂（Review Focus 3）：原會話不動、索引回寫原帳號、
// 標記失效、secret 保留、錯誤區呈現（switchErr，帳號選單 Task 8 接線）。
// UI 於 switchBusy 期間禁翻頁與再切換（帳號選單 Task 8 接線）。
func (a *app) switchTo(userID string) {
	if a.switchBusy {
		return
	}
	a.switchBusy = true
	snap := switchSnapshot{
		sess:   a.sess,
		auth:   a.auth,
		store:  a.activeStore,
		proxy:  a.proxy,
		cancel: a.sessCancel,
	}
	a.goAsync(func(ctx context.Context) {
		a.runSwitch(ctx, userID, snap)
	})
}

// runSwitch 執行切換序列（§8.4 逐字；崩潰安全：索引先寫（2）、會話後換
// （7）——中途崩潰下次啟動以索引 Current 為準重載）：
//
//  1. flush 當前：activeStore.Save(當前 cookieLine)（write-through 已同
//     步，此為兜底——以 Flush 的比對語義冪等寫入；首次登入後即有活躍儲
//     存層（F1 接線），nil 僅餘登入前／替身情境——其憑證由 persistLogin
//     落盤）。
//  2. 索引 Current = 目標 → saveIndex。
//  3. sessCancel()（斬斷舊在途）。
//  4. newSessionFor(目標)（無網路；失敗 → 回滾索引、原會話不動、錯誤區）。
//  5. 有效性檢查：TTL 內（credCache）→ 跳過；TTL 外 → Validate；
//     失敗 → 標記帳號失效（accInvalid）、回滾索引至原帳號、原會話不動、
//     錯誤文案（switchErr）。
//  6. 成功：舊 sess.Close()（transport 收斂——簡報 §1.3 實測根因）。
//  7. 替換 a.sess／a.auth／a.activeStore；a.gen++。
//  8. feeds 歸零（墓碑；三槽全數重置）＋落回 For You；自動載入 For You
//     首頁。
func (a *app) runSwitch(ctx context.Context, userID string, snap switchSnapshot) {
	// 步驟 1：flush 當前（僅於活躍儲存層已確立時）。
	if snap.store != nil {
		a.state.Flush(snap.auth, snap.store)
	}
	// 步驟 2：索引 Current = 目標（帳號清單自儲存層權威讀取——目標的
	// CookieLine 供步驟 5 驗證）。
	accs, idx, err := loadAccounts(ctx)
	if err != nil {
		a.failSwitch(err)
		return
	}
	if len(idx.Order) == 0 {
		a.failSwitch(&xerrors.MaterialError{Msg: "帳號索引不存在或為空（系統憑證庫）"})
		return
	}
	var target Account
	found := false
	for _, acc := range accs {
		if acc.UserID == userID {
			target, found = acc, true
			break
		}
	}
	if !found {
		a.failSwitch(&xerrors.MaterialError{Msg: fmt.Sprintf("帳號 %s 不在索引中（系統憑證庫）", userID)})
		return
	}
	prev := idx.Current
	idx.Current = userID
	if err := saveIndex(ctx, idx); err != nil {
		a.failSwitch(err) // 索引未動，無回滾可寫
		return
	}
	// 步驟 3：斬斷舊在途（sessCtx 子樹；冪等）。
	if snap.cancel != nil {
		snap.cancel()
	}
	// 步驟 4：組目標會話（無網路）。
	newSess, err := switchSessionFactory(target, snap.proxy)
	if err != nil {
		a.rollbackIndex(ctx, prev)
		a.failSwitch(err)
		return
	}
	// 步驟 5：有效性檢查（TTL 內跳過重驗，§8.5）。
	if !a.creds.fresh(userID) {
		if _, verr := newSess.Validate(target.CookieLine); verr != nil {
			a.creds.invalidate(userID)
			_ = newSess.Close() // 棄用未接線的會話：transport 收斂（best-effort）
			a.rollbackIndex(ctx, prev)
			a.publish(func() {
				a.accInvalid[userID] = true
				a.switchErr = verr
				a.switchBusy = false
				a.syncAccountSelection() // 選擇呈現回歸活躍帳號（原會話不動）
			})
			return
		}
		a.creds.mark(userID)
	}
	// 步驟 6：成功——關舊會話（transport 收斂）。
	if snap.sess != nil {
		if err := snap.sess.Close(); err != nil {
			a.state.setErr(fmt.Errorf("switch: 舊會話收斂失敗: %w", err))
		}
	}
	// 步驟 7＋8：替換＋世代推進＋墓碑＋自動載入（單一 UI 轉移）。
	a.publish(func() {
		a.sess = newSess
		if es, ok := newSess.(*engineSession); ok && es.auth != nil {
			a.auth = es.auth
		} else {
			a.auth = nil // 替身會話：無引擎憑證可 flush（同 login 流語義）
		}
		a.activeStore = newAccountCookieStore(userID) // 與會話同鍵的 adapter（quit flush 目標）
		a.gen++
		a.switchErr = nil
		a.switchBusy = false
		a.activeID = userID // 活躍快照（帳號選單勾選／側欄選擇／移除判定）
		if _, ok := a.accountByID(userID); !ok {
			a.upsertAccount(target) // 快照缺目標（未經 warmup 等）即補——選單即刻可見
		}
		a.syncAccountSelection()
		// 步驟 8：feeds 歸零（墓碑；三槽全數——posts/cursor/seen/list/
		// busy/err/end 皆隨新槽歸零）並落回 For You，自動載入其首頁
		// （CUJ：切換後呈現新帳號的 For You）。搜尋為 ephemeral：槽已
		// 換新，檢視一併離開（searchOpen 復位）。展開檢視同理：舊帳號
		// 的展開頁與其播放器一併釋放（§8.6 切換帳號 Close——殘留播放器
		// ／detail 跨帳號即洩漏），detail 狀態歸零使在途大圖回寫落空。
		a.closeDetail()
		a.feeds = [3]*timelineState{{}, {}, {}}
		a.activeFeed = feedForYou
		a.searchOpen = false
		debug.FreeOSMemory()   // D26：切換回收 RSS 高水位（展開期大圖／播放器緩衝）
		a.loadFeed(feedForYou) // 自動載入 For You 首頁
	})
}

// rollbackIndex 回寫索引 Current 至原帳號（步驟 4／5 失敗收斂，Review
// Focus 3）。重新載入索引以保留併寫入的 Order 變更；錯誤記 AppState.Err
// （不覆蓋 switchErr 的首錄位）。
func (a *app) rollbackIndex(ctx context.Context, prev string) {
	idx, err := loadIndex(ctx)
	if err == nil && idx == nil {
		err = &xerrors.MaterialError{Msg: "帳號索引不存在，無法回寫（系統憑證庫）"}
	}
	if err == nil {
		idx.Current = prev
		err = saveIndex(ctx, *idx)
	}
	if err != nil {
		a.state.setErr(fmt.Errorf("switch: 索引回寫失敗: %w", err))
	}
}

// failSwitch 收斂切換失敗：錯誤呈現（switchErr，帳號選單錯誤區 Task 14
// 接線）、防再入閘門復位、選擇呈現回歸活躍帳號。原會話不動（快照未被替
// 換）、secret 保留（序列全程無刪除途徑）。
func (a *app) failSwitch(err error) {
	a.publish(func() {
		a.switchErr = err
		a.switchBusy = false
		a.syncAccountSelection()
	})
}

// updateAccount 是重登後的快照同步（UI 執行緒）：覆蓋同 UserID 項並清除
// 失效標記——新 cookie 待 switchTo 的 Validate 重驗，再失敗即再標記（收
// 斂不變式：標記只反映最近一次驗證結果）。
func (a *app) updateAccount(acc Account) {
	a.upsertAccount(acc)
	delete(a.accInvalid, acc.UserID)
}

// logout 登出至 login view（D20 帳號選單：移除活躍帳號）：斬斷會話子樹
// （在途 feed 工作收斂）、關閉會話（transport）、歸零 home 狀態——detail
// （含播放器，§8.6）、搜尋（ephemeral）、三槽（墓碑替換）；a.sess=nil 使
// view 路由回 login（首次登入形）。活躍帳號快照與選擇呈現一併復位。
func (a *app) logout() {
	if a.sessCancel != nil {
		a.sessCancel()
	}
	a.closeDetail()
	a.closeSearch()
	a.feeds = [3]*timelineState{{}, {}, {}}
	a.activeFeed = feedForYou
	a.switchErr = nil
	a.gen++ // 在途 feed 回寫以世代檢查丟棄（D18 同形）
	a.activeID = ""
	a.syncAccountSelection()
	if sess := a.sess; sess != nil {
		a.sess = nil
		a.auth = nil
		a.activeStore = nil
		a.goAsync(func(ctx context.Context) { _ = sess.Close() }) // transport 收斂（best-effort）
	}
}

// removeConfirm 執行帳號移除（Modal 二次確認後）：deleteAccount（背景；
// 帳號鍵刪除＋索引除名由其完成，冪等）→ publish 記憶體快照除名；被移除
// 者為活躍帳號 → logout（登出至 login view）。錯誤記 AppState.Err（同
// persistLogin 語義：首錄不覆蓋、絕不清除其他帳號憑證）。
func (a *app) removeConfirm() {
	uid := a.removeUserID
	a.removeOpen = false
	a.removeUserID = ""
	a.goAsync(func(ctx context.Context) {
		err := deleteAccount(ctx, uid)
		a.publish(func() {
			if err != nil {
				a.state.setErr(err)
				return
			}
			a.accounts = slices.DeleteFunc(a.accounts, func(acc Account) bool { return acc.UserID == uid })
			if a.activeID == uid {
				a.logout() // 活躍帳號被移除：登出至 login view
			}
		})
	})
}

// warmup 憑證預熱（§8.5 lazy 預熱＋D23 啟動自動還原）：啟動後背景執行
// （冪等，可重跑）——先遷移 v1 舊鍵（§8.3：索引不存在且舊鍵有值；
// migrate.go 載明的啟動接線點），再 loadAccounts 讀索引＋全部帳號 secret
// 進 a.accounts（記憶體快照；帳號選單 Task 8 消費）。遷移／讀取失敗記
// AppState.Err（setErr，同 persistLogin 語義；呈現面 Task 8 接線），
// a.accounts 維持既有值。D23：loadAccounts 成功且索引 Current 指向存在
// 帳號時，以 newSessionFor（無網路）還原會話並直接進 home——cookie 失效
// 由後續自然使用暴露（既有 credential 錯誤 UX），不在啟動時阻塞重驗。
func (a *app) warmup() {
	ctx := context.Background()
	if err := migrateLegacyStore(ctx, a.proxy); err != nil {
		a.state.setErr(err)
	}
	accs, idx, err := loadAccounts(ctx)
	var restore *Account
	var restoredSess *engineSession
	if err == nil && idx.Current != "" {
		for i := range accs {
			if accs[i].UserID == idx.Current {
				restore = &accs[i]
				break
			}
		}
	}
	if restore != nil {
		// D23 還原會話（背景執行緒安全：newSessionFor 無網路 I/O，
		// transport 建構即註冊；proxy 已於派發前寫入）。
		sess, serr := newSessionFor(*restore, a.proxy)
		if serr != nil {
			a.state.setErr(serr)
			restore = nil
		} else {
			restoredSess = sess
		}
	}
	a.publish(func() {
		if err == nil {
			a.accounts = accs
			// 帳號選單（Task 14）：活躍快照自索引 Current 同步（切換進
			// 行中不覆寫——以序列結果為準）；側欄／窄帶下拉呈現隨之同步。
			if !a.switchBusy {
				a.activeID = idx.Current
				a.syncAccountSelection()
			}
		}
		// D23 啟動自動還原：會話就緒且使用者尚在 login（未進行任何
		// login 變體流程）→ 直接進 home。切換進行中或已在 home 不介入。
		if restore != nil && restoredSess != nil && a.sess == nil &&
			a.loginMode == loginModeNone && !a.switchBusy {
			a.sess = restoredSess
			a.auth = restoredSess.auth
			a.activeStore = newAccountCookieStore(restore.UserID)
			a.activeID = restore.UserID
			a.gen++
			a.syncAccountSelection()
		}
	})
	if err != nil {
		a.state.setErr(err)
	}
}

// startToken 於視窗就緒後預熱啟動鏈（AppState bootstrap：ResolveProxy →
// Advance → 憑證就緒交 login 流；fingerprint.Get() 程序內單一真源、無錯誤
// 路徑）：代理解析與註冊表預載在背景進行，結果經 publish 回 UI 執行緒。
// 任何失敗呈現於 login 頁錯誤區（fail-closed，D6／REQ-INV-02）。proxy 段
// 就緒後派發憑證預熱（§8.5：遷移 → loadAccounts → a.accounts；於本回呼
// ——UI 執行緒——派發，a.proxy 寫入先於 goroutine 起始，warmup 讀取無
// 競爭；proxy fail-closed 時不派發——D6 登入已 dead-end）。
func (a *app) startToken() {
	a.goAsync(func(ctx context.Context) {
		proxy, perr := ResolveProxy()
		var rerr error
		if perr == nil {
			if _, err := graphql.LoadRegistry(registryPath()); err != nil {
				rerr = err
			}
		}
		a.publish(func() {
			if perr != nil {
				a.proxyErr = perr
			} else {
				a.proxy = proxy
				a.state.Advance() // proxy 段就緒 → credential 段（憑證就緒於 login 流完成）
				a.goAsync(func(ctx context.Context) { a.warmup() })
			}
			if rerr != nil {
				a.startErr = rerr
			}
		})
	})
}

// flushActive 是 quit flush 本體（main 的 OnBeforeQuit 註冊同一閉包）：
// 目標為活躍帳號的儲存層（§8.3 寫回帳號化；Task 6 切換時替換
// activeStore）；未確立（nil）時沿 AppState 既有目標（v1 舊鍵，降級雙寫
// 期）。目標以參數傳入、不暫替 AppState.store（T5 minor 正規化：暫替不
// 復原債消除；TestQuitFlushTargetsActiveStore 釘測）。
func (a *app) flushActive() {
	a.state.Flush(a.auth, a.activeStore)
}

func main() {
	a := newApp()
	// v1 舊鍵儲存層（降級雙寫期）仍為 AppState 的 flush 目標（persistLogin
	// 契約 2 主項冪等寫與 Close 收斂）。engineStore 全域已廢（§8.3 寫回
	// 帳號化：per-account store 於 newSessionFor 內建構，write-through 直落
	// 活躍帳號鍵）。
	store, err := NewCookieStore("", cookieLineKey)
	if err != nil {
		a.startErr = err // 儲存層不可用：login 頁錯誤區呈現（fail-closed）
	} else {
		a.state = NewAppState(store)
	}
	// quit handler：同步 flush（OnBeforeQuit 須於回呼內完成，含 OS 結束
	// 會話的 SessionEnding）；僅落盤、絕不清除（REQ-LOG-042/043）。目標以
	// 參數指入（Task 6 正規化——T5 minor 的「暫替 AppState.store 不復原」
	// 債消除；本體見 flushActive，TestQuitFlushTargetsActiveStore 釘測）。
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		a.flushActive()
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
		// OnResize → Invalidate：跨帶縮放即重繪（Task 14/D20 佈局；main
		// 建構處註冊一次；off 函式留存——視窗與 app 同生命週期，無需解除）。
		a.offResize = win.OnResize(func() { win.Invalidate() })
		// spawn 不綁定（nil → goAsync 的正式路徑 state.Go；goSession 的
		// 正式路徑為會話子樹派發）——綁定權留給測試注入。
		a.startToken()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
