// view_login：login 畫面——Cookie 貼上輸入、代理狀態列、提交動作、錯誤
// 提示區；成功後切換 home（CUJ 1）。錯誤一律只顯示（errorText 依 kind 取
// messages.go 文案），絕不清除任何憑證或狀態（REQ-LOG-042/043：素材／
// 傳輸類錯誤的重試入口即重新提交「登入」，credential 類提示重新匯入）。
// 防抖（契約 6）：提交進行中再點一律忽略——按鈕保持可見可點，忽略由
// submitLogin 的 busy 閘門執行。
package main

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// loginView 繪製 login 畫面並處理提交（按鈕或輸入欄 Enter）。
func (a *app) loginView(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(16).Padding(24).Children(func() {
		ui.Text(c, "X").FontSize(26).Bold()
		// 連線狀態列（startToken 的解析結果）：D10 下直連是基線，偵測或
		// 配置到代理時升級顯示。
		proxyLine := "連線：直連模式（未配置代理）"
		switch {
		case a.proxyErr != nil:
			proxyLine = "代理：" + errorText(a.proxyErr)
		case a.proxy != "":
			proxyLine = "代理：已就緒"
		}
		ui.Text(c, proxyLine).TextColor(t.TextMuted)
		// 啟動（指紋／註冊表／儲存層）錯誤區：fail-closed 呈現，提交時
		// newSession 對同一來源再驗，錯誤不會被吞。
		if a.startErr != nil {
			ui.Text(c, Sanitize(errorText(a.startErr))).TextColor(t.Danger)
		}
		// Cookie 貼上輸入。輸入緩衝保留使用者貼上的原值以供提交（登入
		// 必需）；顯示以 Password 遮罩（MyGO 標準密碼輸入模式：只遮顯示
		// 不動緩衝）——憑證材料的顯示面全部收斂於契約 7 的 Sanitize 閘
		// 之後，屏上無明文（REQ-INV-01）。
		input := ui.TextInput(c, &a.cookieInput).
			Placeholder("貼上整行 Cookie（k=v; k=v; …）").Label("Cookie").Width(360).Password()
		// 錯誤提示區：kind → 文案；credential → 重新匯入提示、
		// material/transport → 重試入口（下方登入按鈕）。絕不清除。
		if a.loginErr != nil {
			ui.Text(c, Sanitize(errorText(a.loginErr))).TextColor(t.Danger)
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			if a.loginBusy {
				ui.Spinner(c)
				ui.Text(c, "正在驗證…").TextColor(t.TextMuted)
			}
			if ui.PrimaryButton(c, "登入").Clicked() || input.Submitted() {
				a.submitLogin()
			}
		})
	})
}

// submitLogin 處理提交：防抖閘門（進行中或已登入一律忽略，契約 6）→
// 背景組裝會話並驗證（newSession → Validate＝PrepareAuth＋Viewer）→
// 成功時首次落盤（契約 2）→ 經 publish 回 UI 執行緒套用結果。背景工作
// 絕不直接改動 app 狀態（併發面見 app 註解）。
func (a *app) submitLogin() {
	if a.loginBusy || a.sess != nil {
		return // 防抖：提交進行中（或已登入）再點一律忽略
	}
	cookie := strings.TrimSpace(a.cookieInput)
	if cookie == "" {
		a.loginErr = &xerrors.InvalidInputError{Msg: "請先貼上 Cookie 行後再登入"}
		return
	}
	a.loginBusy = true
	a.loginErr = nil
	a.goAsync(func(ctx context.Context) {
		sess, err := a.startSession()
		if err == nil {
			err = sess.Validate(cookie)
		}
		if err == nil {
			// 契約 2：首次 Save 由 login 流完成（正式路徑經 AppState.Flush
			// 冪等寫入；其後 Sender 的 write-through 吸收輪替）。落盤失敗
			// 不阻擋登入——生命週期錯誤由 Flush 記於 AppState.Err。
			_ = a.persistLogin(sess)
		}
		a.publish(func() {
			a.loginBusy = false
			a.loginErr = err
			if err == nil {
				a.sess = sess
				if es, ok := sess.(*engineSession); ok {
					a.auth = es.auth // quit flush 用
				}
			}
		})
	})
}
