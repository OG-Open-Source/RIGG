// view_login：login 畫面——Cookie 貼上輸入、代理狀態列、提交動作、錯誤
// 提示區；成功後切換 home（CUJ 1）。錯誤一律只顯示（errorText 依 kind 取
// messages.go 文案），絕不清除任何憑證或狀態（REQ-LOG-042/043：素材／
// 傳輸類錯誤的重試入口即重新提交「登入」，credential 類提示重新匯入）。
// 防抖（契約 6）：提交進行中再點一律忽略——按鈕保持可見可點，忽略由
// submitLogin 的 busy 閘門執行。
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// loginView 繪製 login 畫面並處理提交（按鈕或輸入欄 Enter）。Task 14：
// 帳號選單的新增帳號／失效重登入口共用本畫面（loginMode 分流）——變體
// 標題帶入口語義（重登帶目標帳號標籤），並提供「返回」回 home（首次登
// 入形無返回：無 home 可回）。
func (a *app) loginView(c *ui.Context) {
	theme.Apply(c) // Lobe 設計語言（與 home 同一 token 面）
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(16).Padding(24).Children(func() {
		title := "X"
		switch a.loginMode {
		case loginModeAdd:
			title = "新增帳號"
		case loginModeRelogin:
			if acc, ok := a.accountByID(a.loginUserID); ok {
				title = "重新貼上 Cookie——" + a.accountLabel(acc)
			} else {
				title = "重新貼上 Cookie"
			}
		}
		ui.Text(c, title).FontSize(26).Bold()
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
		// newSession 對同一來源再驗，錯誤不會被吞。Lobe Alert
		//（mygo-components 對照：MyGO 無橫幅 widget——真缺口，Error 染
		// 色＋Selectable）。
		if a.startErr != nil {
			feedback.Alert(c, Sanitize(errorText(a.startErr)), feedback.AlertProps{
				Type:       feedback.Error,
				Selectable: true,
			})
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
		// 變體的返回入口（回 home；輸入與錯誤一併復位——resetLoginForm）。
		if a.loginMode != loginModeNone {
			if ui.Button(c, "返回").Clicked() {
				a.resetLoginForm()
			}
		}
	})
}

// submitLogin 處理提交（契約 6 防抖；Task 14 擴充 login view 變體）：
// 首次登入形沿原徑——背景組裝會話並驗證（newSession → Validate＝
// PrepareAuth＋Viewer）→ 成功時首次落盤（契約 2）→ publish 直接替換
// a.sess 導向 home。新增帳號（loginModeAdd，home 的帳號選單進入）：驗證
// ＋persistLogin 落盤（REQ-WEB-070 同鍵覆蓋＋索引追加）後沿 switchTo 路
// 徑原地切換——切換序列的世代邊界（gen++）與三槽墓碑不可省，不走首次
// 登入的直接替換。失效重登（loginModeRelogin）：驗證身分須為目標帳號
// （REQ-WEB-069 身分與憑證綁定），通過即以新 cookie 行覆蓋同鍵、清除失效
// 標記，再沿 switchTo 原地重試切換（序列的 Validate 對新值重驗——TTL 外
// 必然重驗；再失敗即再標失效，收斂同 switchTo）。模式與重登目標於派發時
// 捕捉（UI 執行緒）：背景只用捕捉值，不讀共享欄位（app 併發模型）。
func (a *app) submitLogin() {
	if a.loginBusy {
		return
	}
	if a.loginMode == loginModeNone && a.sess != nil {
		return // 防抖：首次登入形已登入再點一律忽略
	}
	cookie := strings.TrimSpace(a.cookieInput)
	if cookie == "" {
		a.loginErr = &xerrors.InvalidInputError{Msg: "請先貼上 Cookie 行後再登入"}
		return
	}
	a.loginBusy = true
	a.loginErr = nil
	mode := a.loginMode
	relogTarget, _ := a.accountByID(a.loginUserID) // 重登目標快照（身分欄位保留）
	a.goAsync(func(ctx context.Context) {
		sess, err := a.startSession()
		var info viewerInfo
		if err == nil {
			info, err = sess.Validate(cookie)
		}
		relogUpdated := Account{}
		if err == nil {
			switch mode {
			case loginModeAdd:
				// 契約 2 同形：首次落盤由 login 流完成（同 UserID 覆蓋
				// 同鍵 REQ-WEB-070；錯誤記 AppState.Err、不阻擋後續切
				// 換——切換序列的索引寫入為真源）。臨時會話棄用：切換
				// 序列另建目標會話（transport 收斂 best-effort）。
				_ = a.persistLogin(sess, info, cookie)
				_ = sess.Close()
			case loginModeRelogin:
				switch {
				case relogTarget.UserID == "":
					err = &xerrors.InvalidInputError{Msg: "找不到要重新登入的帳號"}
					_ = sess.Close()
				case info.UserID != relogTarget.UserID:
					err = &xerrors.InvalidInputError{Msg: "貼上的 Cookie 不屬於帳號 @" + relogTarget.ScreenName}
					_ = sess.Close()
				default:
					relogUpdated = relogTarget
					relogUpdated.CookieLine = cookie
					if serr := saveAccount(ctx, relogUpdated); serr != nil {
						// 落盤失敗不阻擋重試切換：切換序列以儲存層真源
						// 驗證，舊值失敗即再標失效（錯誤先行記錄，同
						// persistLogin 的 setErr 語義）。
						a.state.setErr(fmt.Errorf("relogin: %w", serr))
					}
					_ = sess.Close()
				}
			default:
				// 契約 2：首次落盤由 login 流完成（以身分建帳號＋dualWrite；
				// 正式路徑經 AppState.Flush 冪等寫入；其後 Sender 的
				// write-through 吸收輪替）。落盤失敗不阻擋登入——帳號落盤／
				// 雙寫／Flush 錯誤皆記於 AppState.Err（persistLogin 內，
				// 同 Flush 自記語義）；正式路徑回 nil，`_ =` 只容測試替身。
				_ = a.persistLogin(sess, info, cookie)
			}
		}
		a.publish(func() {
			a.loginBusy = false
			a.loginErr = err
			if err != nil {
				return
			}
			switch mode {
			case loginModeAdd:
				// 快照即刻可見（選單免重跑 warmup）＋原地切換至新帳號。
				a.upsertAccount(Account{UserID: info.UserID, ScreenName: info.ScreenName, Name: info.Name, CookieLine: cookie})
				a.resetLoginForm()
				a.switchTo(info.UserID)
			case loginModeRelogin:
				a.updateAccount(relogUpdated)
				a.resetLoginForm()
				a.switchTo(relogUpdated.UserID)
			default:
				a.sess = sess
				if es, ok := sess.(*engineSession); ok {
					a.auth = es.auth // quit flush 用
				}
				// 活躍儲存層自登入成功即接線（final review F1）：quit
				// flush（flushActive）與切換序列步驟 1 的兜底 flush 自首
				// 登即落活躍帳號鍵，毋須等待首次切換。最小接線（控制器裁
				// 決 2026-11-15）：會話本身不重組——assembleSession 未帶
				// 儲存層，ct0 實務不輪替（探測節 60 回應 0 次輪替），
				// Sender 的即時 write-through 屬冗餘，flush 兜底足夠。
				a.activeStore = newAccountCookieStore(info.UserID)
				a.activeID = info.UserID
				a.syncAccountSelection()
				if info.UserID != "" {
					a.upsertAccount(Account{UserID: info.UserID, ScreenName: info.ScreenName, Name: info.Name, CookieLine: cookie})
				}
			}
		})
	})
}
