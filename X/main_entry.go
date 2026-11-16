//go:build !snapshot

// main_entry：正式入口（snapshot 建置時由 snapshot_main.go 的 main 取代
// ——main.go 其餘型別與方法兩態共用）。
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

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
