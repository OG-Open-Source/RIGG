// view_home：home 畫面（Task 14 四帶佈局，D20）——頂列（Toolbar）恆顯：
// 左上帳號選單（MenuButton／band 0 收合為 Select；清單／新增／移除＋
// Modal 確認）、右上搜尋框（Enter 提交；點回任一 tab 或「返回時間線」
// 關閉搜尋）；無下方 bar（需求方裁決）。左側欄（band ≥1）承載帳號清單
// ＋tabs；For You／Following 兩個時間線（feeds 三槽隔離：切換只換渲染
// 的槽，狀態互不污染；feedSearch 槽由 Task 10 的搜尋結果使用——
// ephemeral，行為面見 view_search.go）、捲動近底觸發翻頁（逐 feed）、
// 到底提示、錯誤橫幅（不清列表，REQ-LOG-042/043：任何錯誤絕不清憑證，
// 也不清已載內容）。翻頁觸發（契約 3/4）：feed 未載入，或其列表於上一
// 框已顯示到底（ui.List 的 AtEnd 語義涵蓋「捲動近底」與「內容不足一
// 屏」）；Cursor 空字串（或與前一頁相同）→ 到底提示、不再觸發。去重與
// seen 累積見 mergePosts。
package main

import (
	"context"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/theme"
)

// feed 槽索引（Task 8）：activeFeed 用裸 int＋常數（ui.Tabs 需要 *int）。
const (
	feedForYou    = 0
	feedFollowing = 1
	feedSearch    = 2
)

// seenCap 是 seen（seenTweetIds）的累積上限（Review Focus 4）：逾限截尾
// 保留最近（丟最舊），避免無界成長。
const seenCap = 500

// feedAutoFillMax 是 D25 的每槽自動補載上限：AtEnd 且內容未滿一屏
// （Visible().first==0，全螢幕 tall window 情境）時最多自動連載的頁數
// ——使列表可捲即止，之後需使用者真捲動（first>0）才續翻。
const feedAutoFillMax = 2

// postsCap 是單槽 posts 的 D26 上限：長捲動的無界成長截尾保留最近
// （ListState.Key 以 work_id 為識別，列位移不破壞捲動語義）。
const postsCap = 600

// timelineState 是單一 feed 的隔離狀態（Task 8）：posts／cursor／seen／
// end／busy／err／list 皆屬該 feed——切換 tab 不互相污染。
type timelineState struct {
	posts    []post
	cursor   string
	seen     []string // 上限 seenCap=500：append 後 len>500 → 截尾保留最近（丟最舊）
	end      bool
	busy     bool
	err      error
	list     ui.ListState
	autoFill int // D25：未滿屏 AtEnd 的自動補載計數（feedAutoFillMax 封頂；!AtEnd 歸零）
}

// homeEndText 是到底提示文案（契約 4）。
const homeEndText = "已載入全部貼文"

// homeView 繪製 home 畫面（D28 RWD 重構：頂列恆顯——帳號新增/移除與搜
// 尋；Lobe DraggableSideNav 帳號側欄；無下方 bar）：
//
//   - 側欄（DraggableSideNav）：拖曳縮放（64–360 DIP）、收合成 rail；
//     跨越 bpTablet 時自動收合/展開（D28）。
//   - 內容區：tabs（Segmented→ui.Tabs）＋feed；展開檢視 band<2 全寬
//     頁、band≥2 右欄並列；≥1200 內容 max-width 640 置中。
//
// 選單、右上搜尋框；無下方 bar）：
//
//   - band 0（<768）：單欄＋頂列收合（帳號 ui.Select 下拉），tabs 仍於
//     欄頂（無側欄）。
//   - band 1（768–1023）：兩欄——左側欄（帳號清單＋tabs）＋時間線。
//   - band 2（1024–1199）：完整版型——側欄＋時間線＋展開檢視並列
//     （a.detail 非 nil 時右欄 detailView）。
//   - band 3（≥1200）：同 band 2＋內容欄 max-width 640 置中、兩側留白。
//
// 窄帶（band 0/1）的展開檢視以全寬 detail 頁承接（final review F2，控制
// 器裁決 2026-11-15）：a.detail 非 nil 時整個內容區（含 band 1 側欄）改
// 渲染 detailView——feed 於展開期間不可達，收合按鈕（detailView 內）即
// 「離開展開檢視」的出口（closeDetail）；§8.8 的 ui.Router 頁形態不予實
// 作，Router URL/歷史語義延後 v1.2（DECISIONS〈v1.1 收尾登錄〉）。移除
// 帳號的確認 Modal 於各帶頂層呈現；切換失敗的錯誤區（switchErr）呈於頂
// 列之下。
// homeView 是 home 畫面本體（D28 RWD 重構）：頂列（帳號新增/移除＋搜
// 尋）＋ Lobe DraggableSideNav（帳號側欄——拖曳縮放、收合成 64 DIP
// rail、狀態傳播 body，取代 D20 的四帶手寫側欄）＋內容區（feed；展開
// 檢視 band<2 全寬頁、band≥2 右欄並列——F2 裁決不變）。tabs（Segmented）
// 統一呈於 feed 標題列（各帶一致）。帶轉換跨越 768 時自動收合/展開側
// 欄（僅邊界介入，不與使用者的拖曳對抗）。
func (a *app) homeView(c *ui.Context) {
	theme.Apply(c) // Lobe MyGo 設計語言（淺色/深色隨系統；TokensOf 取對應 token）
	w, _ := c.Size()
	band := a.layoutBand(w)
	// D28：跨越窄帶邊界時自動收合/展開側欄（初始 prevWidth 為 0——首幀
	// 依寬度設定；之後僅邊界跨越時介入）。
	if a.prevWidth >= bpTablet && w < bpTablet {
		a.navExpanded = false
	} else if a.prevWidth < bpTablet && w >= bpTablet && a.prevWidth > 0 {
		a.navExpanded = true
	} else if a.prevWidth == 0 {
		a.navExpanded = w >= bpTablet
	}
	a.prevWidth = w
	// 選單一律以主題彈出呈現（PopoverMenus）：與自繪 UI 同貌，內窗可檢
	// 查（Menu／ChooseMenuItem／Texts）。
	if c.MenuPresentation() != ui.PopoverMenus {
		c.SetMenuPresentation(ui.PopoverMenus)
	}
	ui.Column(c).Fill().Children(func() {
		a.homeToolbar(c, band)
		if a.switchErr != nil {
			// 帳號選單錯誤區（switchErr，Task 8 接線由本任務的選單落
			// 地）：最近一次切換失敗的文案；成功切換即清除（runSwitch）。
			// 既有內容原樣（REQ-LOG-042/043 同形：僅顯示）。Lobe Alert
			//（Error 染色；Selectable 供複製細節）。
			feedback.Alert(c, Sanitize(errorText(a.switchErr)), feedback.AlertProps{
				Type:       feedback.Error,
				Selectable: true,
			})
		}
		ui.Row(c).Fill().Children(func() {
			// D28：帳號側欄（DraggableSideNav）——展開為帳號清單（頭像＋
			// 標籤＋失效標記，點擊切換/重登），收合成頭像 rail。
			layout.DraggableSideNav(c, &a.navWidth, &a.navExpanded,
				layout.DraggableSideNavProps{MinWidth: 64, MaxWidth: 360},
				func(nav layout.SideNavState) {
					if nav.Expanded {
						a.accountNavList(c)
					} else {
						a.accountNavRail(c)
					}
				})
			// 內容區：展開檢視（窄帶全寬頁／寬帶右欄並列）＋feed。
			if band < 2 && a.detail != nil {
				// 窄帶全寬展開檢視（F2）：feed 於展開期間不可達——收合
				// 按鈕（detailView 內）closeDetail 返回 feed，切換帳號／
				// 登出經墓碑 closeDetail 同效（§8.6 語義不變）。幀內快
				// 照：收合按鈕於本幀歸零 detail 不影響本幀渲染。
				d := *a.detail
				ui.Column(c).Fill().Background(c.Theme().Surface).Children(func() {
					a.detailView(c, d)
				})
				return
			}
			fc := ui.Column(c).Grow(1)
			if band >= 3 {
				fc = fc.AlignItems(ui.Center)
			}
			fc.Children(func() {
				a.homeFeedView(c, band)
			})
			if band >= 2 && a.detail != nil {
				d := *a.detail // 幀內快照：收合按鈕於本幀歸零 detail 不影響本幀渲染
				ui.Column(c).Width(detailPanelWidth).Background(c.Theme().Surface).Children(func() {
					a.detailView(c, d)
				})
			}
		})
	})
	a.removeModal(c)
}

// accountNavList 是側欄展開態的帳號清單（D28）：Avatar＋標籤（失效者
// 加「（失效）」、活躍者勾選），點擊切換/重登；新增/移除在頂列（＋/−，
// 全帶）。Lobe Avatar（marbled 色播種 ScreenName——D27 wave 2 項目提前
// 兌現：側欄的頭像語義 MyGO ui.Avatar 無 marbled 面）。
func (a *app) accountNavList(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Padding(8, 12).Gap(4).Children(func() {
		ui.Text(c, "帳號").TextColor(t.TextMuted).FontSize(12)
		for _, acc := range a.accounts {
			acc := acc
			invalid := a.accInvalid[acc.UserID]
			active := acc.UserID == a.activeID
			row := ui.Row(c).Padding(6, 8).Gap(8).AlignItems(ui.Center)
			row.Children(func() {
				display.Avatar(c, acc.ScreenName, display.AvatarProps{Size: 28})
				ui.Column(c).Gap(0).Children(func() {
					label := acc.ScreenName
					if invalid {
						label += "（失效）"
					}
					ui.Text(c, Sanitize("@"+label)).FontSize(13)
				})
			})
			row = row.Background(t.Surface)
			if row.Clicked() {
				switch {
				case invalid:
					a.openRelogin(acc.UserID)
				case !active:
					a.switchTo(acc.UserID)
				}
			}
		}
	})
}

// accountNavRail 是側欄收合態（64 DIP rail）：各帳號 Avatar 直欄，點擊
// 切換/重登（標籤省略——rail 語義）。
func (a *app) accountNavRail(c *ui.Context) {
	ui.Column(c).Padding(8, 8).Gap(8).AlignItems(ui.Center).Children(func() {
		for _, acc := range a.accounts {
			acc := acc
			display.Avatar(c, acc.ScreenName, display.AvatarProps{Size: 32})
		}
	})
}

// homeToolbar 繪製頂列（D20：恆顯）：左上帳號選單（band 0 收合為
// ui.Select 下拉；其餘帶 ui.MenuButton）、右上搜尋框（Task 10 入口整
// 合——Enter 提交 submitSearch，空 query 由其忽略）。無下方 bar（需求
// 方裁決：Desktop only）。
func (a *app) homeToolbar(c *ui.Context, band int) {
	ui.Toolbar(c, func() {
		// 帳號新增/移除（全帶一致——D28：側欄只管切換/重登，管理動作
		// 在頂列；移除針對活躍帳號，Modal 二次確認沿 accountMenu 同路
		// 徑）。
		if ui.Button(c, "＋").Disabled(a.switchBusy || len(a.accounts) >= maxAccounts).Clicked() {
			a.beginLoginVariant(loginModeAdd, "")
		}
		if ui.Button(c, "−").Disabled(a.switchBusy || a.activeID == "").Clicked() {
			a.removeUserID = a.activeID
			a.removeOpen = true
		}
		ui.Spacer(c)
		// 搜尋（MyGO 內建 ui.SearchField——mygo-components 對照：含清除
		// 鈕與 Escape 的原生搜尋欄，Lobe SearchBar 不採）：Enter 提交
		//（Submitted）；Label「搜尋」供輔助技術與測試定位、Placeholder
		//「搜尋貼文」空態顯示。窄帶收合固定窄寬（其餘帶固定寬、Spacer
		// 推右）。
		sf := ui.SearchField(c, &a.searchInput).Label("搜尋").Placeholder("搜尋貼文")
		if band == 0 {
			sf = sf.Width(160)
		} else {
			sf = sf.Width(320)
		}
		if sf.Submitted() {
			a.submitSearch(a.searchInput)
		}
	})
}

// openRelogin 進入失效帳號的「重新貼上 Cookie」入口（login view 變體，
// D20/§8.5）：目標帳號預標（loginUserID），提交成功 → 更新該帳號 cookie
// （覆蓋同鍵）→ 沿 switchTo 原地重試切換（submitLogin 的 relogin 分支）。
func (a *app) openRelogin(userID string) {
	a.beginLoginVariant(loginModeRelogin, userID)
}

// accountLabel 是帳號的顯示標籤（選單／側欄／窄帶下拉）：@screen_name；
// 失效者加「（失效）」（accInvalid，Task 6 呈現語義）。
func (a *app) accountLabel(acc Account) string {
	label := "@" + acc.ScreenName
	if a.accInvalid[acc.UserID] {
		label += "（失效）"
	}
	return label
}

// removeModal 繪製移除帳號的二次確認（ui.Modal）：確認走 removeConfirm
// （背景 deleteAccount）；取消／Escape／點外由 Modal 的 *open 語義收起。
func (a *app) removeModal(c *ui.Context) {
	if !a.removeOpen {
		return
	}
	ui.Modal(c, &a.removeOpen, func() {
		title := "移除帳號"
		if acc, ok := a.accountByID(a.removeUserID); ok {
			title = "移除 @" + acc.ScreenName
		}
		ui.Column(c).Gap(12).Children(func() {
			ui.Text(c, title).FontSize(18).Bold()
			ui.Text(c, "將自系統憑證庫刪除此帳號的憑證並自清單除名，無法復原。")
			ui.Row(c).Gap(10).Justify(ui.End).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					a.removeOpen = false
				}
				if ui.PrimaryButton(c, "確認移除").Clicked() {
					a.removeConfirm()
				}
			})
		})
	})
}

// homeFeedView 繪製 feed 單欄：band 0 的 tabs（band ≥1 由側欄承載）、
// 標題列（載入中含 spinner；搜尋結果帶「返回時間線」顯式返回入口）、
// 錯誤橫幅、活躍 feed 的時間線列表與到底提示。
func (a *app) homeFeedView(c *ui.Context, band int) {
	t := c.Theme()
	// wasSearch 是進框前的檢視狀態：本框 Enter 剛開啟的搜尋不被同框的
	// 殘餘 tab 輸入關閉（關閉只對「已在檢視」的搜尋生效）。
	wasSearch := a.searchOpen
	root := ui.Column(c).Fill()
	if band >= 3 {
		// 寬內容區（D20）：卡片欄 max-width 640——上層容器（AlignItems
		// Center）置中、兩側留白。
		root = root.MaxWidth(640)
	}
	root.Children(func() {
		// tabs（MyGO 內建 ui.Tabs——mygo-components 對照：少數頁面成列
		// 的原生 widget；D28 起各帶一致，取代側欄承載。Segmented 的
		// Tester 點擊鏈未過驗證，故取 ui.Tabs——T8 釘測已證）：點選即
		// 改寫 a.activeFeed——建構返回前該框輸入已套用，其後整框即以
		// 切換後的槽渲染。搜尋結果（feedSearch）不在 tabs：越界選擇會
		// 被鉗回範圍，故 activeFeed 恆為 tab 索引、搜尋以 searchOpen 另
		// 行標示——其間點回任一 tab（Changed）即關閉（已選 tab 的點擊
		// 不觸發 Changed——顯式返回入口見標題列）。
		tabs := ui.Tabs(c, &a.activeFeed, "For You", "Following")
		if wasSearch && tabs.Changed() {
			a.closeSearch() // 點回 For You/Following：ephemeral 槽歸零
		}
		kind := a.activeFeed
		f := a.feeds[kind]
		if a.searchOpen {
			kind, f = feedSearch, a.feeds[feedSearch] // 搜尋結果走 feed 渲染路徑
		}
		// 翻頁觸發：先於列表建構檢查（AtEnd 反映上一框的版面；首載以
		// posts 為空表達）。loadFeed 自帶防抖與終止條件（busy／end／
		// err），三槽同徑（含 feedSearch 的游標接續頁，F3；搜尋首頁
		// 起跑走 submitSearch）。D25：AtEnd 含「內容未滿一屏」——全螢
		// 幕 tall window 逐框連環翻頁的根因。分派：first>0（真捲到底）
		// 正常翻；first==0（未滿屏）僅自動補載 feedAutoFillMax 頁使列
		// 表可捲即止；!AtEnd 歸零計數（使用者捲動／內容超窗後重置）。
		if len(f.posts) == 0 {
			a.loadFeed(kind)
		} else if f.list.AtEnd() {
			if first, _ := f.list.Visible(); first > 0 {
				f.autoFill = 0
				a.loadFeed(kind)
			} else if f.autoFill < feedAutoFillMax {
				f.autoFill++
				a.loadFeed(kind)
			}
		} else {
			f.autoFill = 0
		}
		ui.Row(c).Padding(12, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			title := "時間線"
			if a.searchOpen {
				title = "搜尋結果" // 搜尋指示：文字標示當前為搜尋結果（不新增第三 tab）
				// 顯式返回入口（Task 10 UX 註記釘注）：已選 tab 的點擊
				// 不觸發 Changed（mygo 只標記真切換），自預設 tab 開啟
				// 的搜尋無 tab 返回路徑——「返回時間線」是顯式出口。
				if ui.Button(c, searchBackText).Clicked() {
					a.closeSearch() // ephemeral 槽歸零；同幀重建自行收斂
				}
			}
			ui.Text(c, title).FontSize(20).Bold().Grow(1)
			if f.busy {
				ui.Spinner(c) // mygo-components 對照：進行中的原生 widget
			}
		})
		// 錯誤橫幅（契約 5）：Lobe Alert（mygo-components 對照：MyGO 無
		// 橫幅 widget——真缺口，Error 染色＋Selectable 供複製）；重試為
		// 顯式入口（原生 ui.Button）——錯誤後不自動重試（避免每框風
		// 暴）；既有列表原樣。
		if f.err != nil {
			feedback.Alert(c, Sanitize(errorText(f.err)), feedback.AlertProps{
				Type:       feedback.Error,
				Selectable: true,
			})
			ui.Row(c).Padding(0, 16, 8).Children(func() {
				if ui.Button(c, "重試").Clicked() {
					f.err = nil
					if a.searchOpen {
						a.submitSearch(a.searchQuery) // 搜尋重試＝最後 query 再提交
					} else {
						a.loadFeed(a.activeFeed)
					}
				}
			})
		}
		// 載入骨架（Lobe Skeleton，sweep）：首載（posts 空＋busy）的佔位；
		// 空狀態（Lobe Empty）：載入完畢且無帖（含到底）——取代空白列表。
		if len(f.posts) == 0 && f.busy && f.err == nil {
			ui.Column(c).Padding(8, 16).Gap(12).Children(func() {
				for i := 0; i < 3; i++ {
					display.Skeleton(c, display.SkeletonProps{Height: 64, Animation: display.SkeletonSweep})
				}
			})
		}
		if len(f.posts) == 0 && f.end && f.err == nil && !f.busy {
			display.Empty(c, display.EmptyProps{
				Title:       "沒有貼文",
				Description: homeEndText,
			})
		}
		// 時間線列表：work_id 去重合併（首見順序）後的已顯示貼文；顯示
		// 欄位（衍生顯示字串）經 Sanitize 後上屏（契約 7）。列表狀態屬
		// 該 feed（槽隔離——含捲動版位與 AtEnd 語義）。列主體可點（
		// RowClicked）開啟展開檢視（Task 13）；列內的 Link／重試按鈕自
		// 行收 clicks，不觸發開帖。
		ui.List(c, &f.list, len(f.posts), func(i int) {
			p := f.posts[i]
			body := ui.Column(c).Padding(10, 16).Gap(4).Children(func() {
				ui.Text(c, Sanitize("@"+p.ScreenName)).TextColor(t.TextMuted)
				if p.Title != "" {
					ui.Text(c, Sanitize(p.Title))
				}
				if p.WorkURL != "" {
					ui.Link(c, "開啟貼文", Sanitize(p.WorkURL))
				}
				a.thumbRow(c, kind, f, p) // 縮圖格（僅含媒體貼文渲染；Task 12）
			})
			// 展開檢視入口（單一活躍播放器規則見 openDetail）：各帶皆
			// 可開帖——窄帶（band <2）以全寬 detail 頁承接（F2 裁決，
			// homeView 分流；收合路徑恆在 detailView 內，無殘留播放器
			// 風險），寬帶為右欄並列。
			if body.RowClicked() {
				a.openDetail(p)
			}
		}).Grow(1)
		// 到底提示：游標空（或與前頁相同）後顯示，且不再觸發翻頁。
		if f.end {
			ui.Text(c, homeEndText).TextColor(t.TextMuted).Padding(12, 16)
		}
	})
}

// thumbRow 渲染貼文的縮圖格（Task 12）：僅貼文含 Media 時渲染（其餘貼
// 文與今日完全同形），列表縮圖取首個媒體項（設計 §8.2）。thumbStore 命
// 中即 ui.Image；未命中 → 佔位（主題色塊＋「載入中」）並經 ensureThumb
// 派發（in-flight 去重＋信號量，見 media.go）；失敗 → 「可重試」按鈕，
// 點擊才再派發（不自動重試）。
func (a *app) thumbRow(c *ui.Context, kind int, f *timelineState, p post) {
	if len(p.Media) == 0 {
		return
	}
	m := p.Media[0]
	if m.ThumbURL == "" {
		return
	}
	if src, ok := thumbLRU.get(m.ThumbURL); ok {
		ui.Image(c, src).Size(160, 90) // 給定尺寸即縮放至貼合（MyGO Image 語義）
		return
	}
	if thumbFailed[m.ThumbURL] {
		if ui.Button(c, thumbRetryText).Clicked() {
			a.retryThumb(kind, f, m.ThumbURL) // 顯式重試：清除失敗態再派發
		}
		return
	}
	a.ensureThumb(kind, f, m.ThumbURL) // 未命中：派發（冪等；逐框重入去重）
	t := c.Theme()
	ui.Column(c).Background(t.Surface).Size(160, 90).AlignItems(ui.Center).Children(func() {
		ui.Text(c, thumbLoadingText).TextColor(t.TextMuted)
	})
}

// loadFeed 載入 feed kind 的下一頁（或首頁）：防抖與終止閘門（busy／已
// 到底／有未處置錯誤／無會話）→ 背景呼叫接縫——For You 走 Home（契約
// 3：HomeTimeline(cursor, 20, seen)）、Following 走 HomeLatest（Task 7：
// HomeLatestTimeline，雙形態同構；cursor／seen 各自累積）、feedSearch 走
// Search（Task 10；§8.7 翻頁契約「各 feed 的 List AtEnd 各自觸發」——
// final review F3 兌現：以槽內留存游標接續翻頁，Search 無 seen 參數；
// 首頁起跑走 submitSearch）→ 經 publish 回 UI 執行緒套用。背景工作以會
// 話子樹派發（goSession，§8.4），起跑前捕捉世代（gen）與槽，回寫前比對
// ——世代過期（帳號切換墓碑已歸零）或槽已被換新的舊結果整批丟棄，不得
// 污染新畫面（D18 世代邊界雙保險：cancel 斬在途、gen 防已回程；Review
// Focus 1）。逐 feed 化（Task 8）：寫回一律指向派發時捕捉的槽 kind，與
// 其後的 tab 切換無關——結果屬於該 feed。錯誤只設該槽 err（橫幅呈現），
// 既有列表原樣（契約 5）。
func (a *app) loadFeed(kind int) {
	f := a.feeds[kind]
	if f.busy || f.end || f.err != nil || a.sess == nil {
		return
	}
	// 派發管轄（Task 8 大聲預設的收斂，final review F3）：三槽皆由本徑
	// 翻頁——feedSearch 須已有提交過的 query（submitSearch 起跑時留
	// 存），缺查詢詞不派發；未知 kind 一律不派發、快速失敗。須在佔用
	// busy 之前攔下：誤入的呼叫不得把槽卡在 busy（AtEnd 逐框重入即永
	// 久佔住）；亦不寫 err（橫幅將成逐框風暴）。
	switch kind {
	case feedForYou, feedFollowing:
	case feedSearch:
		if a.searchQuery == "" {
			return
		}
	default:
		return
	}
	f.busy = true
	f.err = nil
	sess, cursor, seen, gen, query := a.sess, f.cursor, f.seen, a.gen, a.searchQuery
	a.goSession(func(ctx context.Context) {
		var res *timelineResult
		var err error
		switch kind {
		case feedFollowing:
			res, err = sess.HomeLatest(cursor, seen) // Following＝最新時間線
		case feedSearch:
			// 搜尋接續頁：以派發時捕捉的 query 與槽內留存游標（§8.7：
			// Search 無 seen——去重由 mergePosts 的 work_id 比對承擔）。
			res, err = sess.Search(query, searchProduct, cursor)
		default:
			res, err = sess.Home(cursor, seen) // For You＝首頁推薦時間線
		}
		a.publish(func() {
			// 世代與槽一致（T6 形的逐 feed 化）：gen 比對擋切換後的舊
			// 帳號結果；槽指標比對擋該 feed 被墓碑歸零後的殘餘回寫。
			if gen != a.gen || a.feeds[kind] != f {
				return
			}
			f.busy = false
			if err != nil {
				f.err = err // 既有列表原樣；由重試入口或近底捲動再觸發
				return
			}
			if res == nil {
				res = &timelineResult{}
			}
			prev := f.cursor
			f.mergePosts(res)
			if res.Cursor == "" || res.Cursor == prev {
				// 無游標或游標與前一頁相同 → 到底（CUJ 2），不再觸發。
				f.cursor, f.end = "", true
			} else {
				f.cursor = res.Cursor
			}
		})
	})
}

// mergePosts 合併一頁結果到該 feed：work_id 去重、保留首見順序（契約
// 3）；seen（seenTweetIds）由已顯示貼文累積（跨頁去重，Task 10 介面契
// 約）——上限 seenCap（Review Focus 4）：逾限截尾保留最近（丟最舊）。
func (f *timelineState) mergePosts(res *timelineResult) {
	known := make(map[string]bool, len(f.posts)+len(res.Posts))
	for _, p := range f.posts {
		known[p.WorkID] = true
	}
	for _, p := range res.Posts {
		if p.WorkID == "" || known[p.WorkID] {
			continue
		}
		known[p.WorkID] = true
		f.posts = append(f.posts, p)
		f.seen = append(f.seen, p.WorkID)
	}
	if len(f.seen) > seenCap {
		// 截尾：保留最近的 seenCap 筆（丟最舊）；複製出新底層陣列，與
		// 派發時捕捉的舊 seen 切片脫鉤（背景 goroutine 仍持舊切片）。
		f.seen = append([]string(nil), f.seen[len(f.seen)-seenCap:]...)
	}
	// D26 RAM 槓桿：posts 上限 postsCap——長捲動的無界成長截尾保留最近
	// （ListState.Key 以 work_id 為識別，列位移不破壞捲動語義；Media
	// 切片隨帖釋放）。
	if len(f.posts) > postsCap {
		f.posts = append([]post(nil), f.posts[len(f.posts)-postsCap:]...)
	}
}
