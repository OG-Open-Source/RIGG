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

// timelineState 是單一 feed 的隔離狀態（Task 8）：posts／cursor／seen／
// end／busy／err／list 皆屬該 feed——切換 tab 不互相污染。
type timelineState struct {
	posts  []post
	cursor string
	seen   []string // 上限 seenCap=500：append 後 len>500 → 截尾保留最近（丟最舊）
	end    bool
	busy   bool
	err    error
	list   ui.ListState
}

// homeEndText 是到底提示文案（契約 4）。
const homeEndText = "已載入全部貼文"

// sidebarWidth 是左側欄寬（DIP；MyGO Sidebar 的建議寬度）。
const sidebarWidth = float32(220)

// accountCapText 是「新增帳號」達帳號上限時的提示文案（D19：上限呈提示
// 態，不解鎖路徑）。
const accountCapText = "最多 5 個帳號——請先移除一個"

// 側欄時間線 tabs 的項目 id（Sidebar 以 id 記選擇；帳號項以 user_id 為
// id——前綴區隔，兩者不碰撞）。
const (
	sideIDForYou    = "feed-foryou"
	sideIDFollowing = "feed-following"
)

// homeView 繪製 home 畫面（Task 14 四帶佈局，D20：頂列恆顯——左上帳號
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
func (a *app) homeView(c *ui.Context) {
	w, _ := c.Size()
	band := a.layoutBand(w)
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
			// 既有內容原樣（REQ-LOG-042/043 同形：僅顯示）。
			t := c.Theme()
			ui.Row(c).Padding(8, 16).AlignItems(ui.Center).Background(t.Surface).Children(func() {
				ui.Text(c, Sanitize(errorText(a.switchErr))).TextColor(t.Danger).Grow(1)
			})
		}
		if band < 2 && a.detail != nil {
			// 窄帶全寬展開檢視（F2）：feed（與 band 1 側欄）於展開期
			// 間不可達——收合按鈕（detailView 內）closeDetail 返回
			// feed，切換帳號／登出經墓碑 closeDetail 同效（§8.6 語義
			// 不變：播放器生命週期隨 detail）。幀內快照：收合按鈕於本
			// 幀歸零 detail 不影響本幀渲染。
			d := *a.detail
			ui.Column(c).Fill().Background(c.Theme().Surface).Children(func() {
				a.detailView(c, d)
			})
			return
		}
		if band == 0 {
			a.homeFeedView(c, band)
		} else {
			ui.Row(c).Fill().Children(func() {
				a.homeSidebar(c)
				// feed 欄：≥1200 內容 max-width 640 置中（AlignItems
				// Center 置中其 max-width 卡片欄，D20）。
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
		}
	})
	a.removeModal(c)
}

// homeToolbar 繪製頂列（D20：恆顯）：左上帳號選單（band 0 收合為
// ui.Select 下拉；其餘帶 ui.MenuButton）、右上搜尋框（Task 10 入口整
// 合——Enter 提交 submitSearch，空 query 由其忽略）。無下方 bar（需求
// 方裁決：Desktop only）。
func (a *app) homeToolbar(c *ui.Context, band int) {
	ui.Toolbar(c, func() {
		if band == 0 {
			a.accountSelect(c)
		} else {
			a.accountMenu(c)
		}
		ui.Spacer(c)
		in := ui.TextInput(c, &a.searchInput).Placeholder("搜尋貼文").Label("搜尋")
		if band == 0 {
			in = in.Width(160) // 窄帶收合：固定窄寬（其餘帶固定寬、Spacer 推右）
		} else {
			in = in.Width(320)
		}
		if in.Submitted() {
			a.submitSearch(a.searchInput)
		}
	})
}

// accountMenu 繪製左上帳號選單（ui.MenuButton，Task 14／D19）：各帳號
// （accountLabel：失效者加「（失效）」，點擊失效者走「重新貼上 Cookie」
// 入口、活躍者勾選無操作、其餘 switchTo）＋「新增帳號」（達 maxAccounts
// 呈提示態——禁用＋上限文案 Subtitle）＋「移除 @xxx」（開 Modal 二次確
// 認）。切換進行中（switchBusy）動作全數禁用（帳號選單 Task 8 接線：
// switchBusy 期間禁再切換）。按鈕文字為活躍帳號（無則「帳號」），元件
// Label「帳號選單」供輔助技術與測試定位。
func (a *app) accountMenu(c *ui.Context) {
	label := "帳號"
	if acc, ok := a.accountByID(a.activeID); ok {
		label = a.accountLabel(acc)
	}
	ui.MenuButton(c, label, func(m *ui.Menu) {
		for _, acc := range a.accounts {
			if m.Item(a.accountLabel(acc)).Checked(acc.UserID == a.activeID).Disabled(a.switchBusy).Chosen() {
				if a.accInvalid[acc.UserID] {
					a.openRelogin(acc.UserID) // 失效者：重新貼上 Cookie 入口
				} else if acc.UserID != a.activeID {
					a.switchTo(acc.UserID) // 背景切換（switchBusy 防再入）
				}
			}
		}
		m.Separator()
		// 新增帳號：上限（maxAccounts=5，D19）呈提示態——禁用＋上限文
		// 案（Subtitle；彈出式選單於標籤下第二行呈現）。
		atCap := len(a.accounts) >= maxAccounts
		add := m.Item("新增帳號").Disabled(a.switchBusy || atCap)
		if atCap {
			add = add.Subtitle(accountCapText)
		}
		if add.Chosen() {
			a.beginLoginVariant(loginModeAdd, "")
		}
		m.Separator()
		for _, acc := range a.accounts {
			if m.Item("移除 @" + acc.ScreenName).Disabled(a.switchBusy).Chosen() {
				a.removeUserID = acc.UserID
				a.removeOpen = true // Modal 二次確認（removeModal）
			}
		}
	}).Label("帳號選單")
}

// accountSelect 是 band 0 的帳號收合（ui.Select 下拉，D20）：選項為帳號
// 標籤（失效者含「（失效）」）；選擇即分派——失效者走重登入口、其餘切換
// （活躍者無操作）。切換進行中禁用（防再入造成呈現錯位——同步由
// syncAccountSelection 於序列收斂時回寫）。
func (a *app) accountSelect(c *ui.Context) {
	if len(a.accounts) == 0 {
		return
	}
	opts := make([]string, 0, len(a.accounts))
	byLabel := make(map[string]string, len(a.accounts))
	for _, acc := range a.accounts {
		label := a.accountLabel(acc)
		opts = append(opts, label)
		byLabel[label] = acc.UserID
	}
	sel := ui.Select(c, &a.selectAcc, opts).Disabled(a.switchBusy)
	if sel.Changed() {
		if uid, ok := byLabel[a.selectAcc]; ok && uid != a.activeID {
			if a.accInvalid[uid] {
				a.openRelogin(uid)
			} else {
				a.switchTo(uid)
			}
		}
	}
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

// homeSidebar 繪製左側欄（band ≥1，D20）：時間線 tabs（For You／
// Following）＋帳號清單（失效者標記）。選擇記於 sideSel（ui.Sidebar 的
// id 語義：帳號項 id=user_id、tabs 項 id=sideID*）；Changed 分派見
// sidebarChosen。切換進行中帳號項禁用（switchBusy 禁再切換）。
func (a *app) homeSidebar(c *ui.Context) {
	sb := ui.Sidebar(c, &a.sideSel, func() {
		ui.SidebarSection(c, "時間線", nil, func() {
			ui.SidebarItem(c, sideIDForYou, nil, "For You")
			ui.SidebarItem(c, sideIDFollowing, nil, "Following")
		})
		ui.SidebarSection(c, "帳號", nil, func() {
			for _, acc := range a.accounts {
				ui.SidebarItem(c, acc.UserID, nil, a.accountLabel(acc)).Disabled(a.switchBusy)
			}
		})
	}).Width(sidebarWidth)
	if sb.Changed() {
		a.sidebarChosen(a.sideSel)
	}
}

// sidebarChosen 分派側欄選擇：時間線項 → activeFeed（搜尋開啟時一併關
// 閉——ephemeral 槽歸零，同 tabs 點回語義）；帳號項 → 失效者走重登入口、
// 活躍者無操作、其餘 switchTo（背景切換，switchBusy 防再入）。失效者不
// 可為選擇（其項點擊是動作而非切換）——呈現回歸活躍帳號。
func (a *app) sidebarChosen(id string) {
	switch id {
	case sideIDForYou:
		if a.searchOpen {
			a.closeSearch()
		}
		a.activeFeed = feedForYou
	case sideIDFollowing:
		if a.searchOpen {
			a.closeSearch()
		}
		a.activeFeed = feedFollowing
	default:
		if a.accInvalid[id] {
			a.openRelogin(id)
			a.syncAccountSelection()
			return
		}
		if id != a.activeID {
			a.switchTo(id)
		}
	}
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
		if band == 0 {
			// tabs（MyGO 內建；僅 band 0——band ≥1 由側欄承載）：點選
			// 即改寫 a.activeFeed——建構返回前該框輸入已套用，其後整框
			// 即以切換後的槽渲染（下方讀取 activeFeed 取得新選擇）。搜
			// 尋結果（feedSearch）不在 tabs：MyGO Tabs 會把越界選擇鉗回
			// 範圍，故 activeFeed 恆為 tab 索引、搜尋以 searchOpen 另行
			// 標示——其間點回任一 tab（Changed）即關閉（已選 tab 的點
			// 擊不觸發 Changed——顯式返回入口見標題列）。
			tabs := ui.Tabs(c, &a.activeFeed, "For You", "Following")
			if wasSearch && tabs.Changed() {
				a.closeSearch() // 點回 For You/Following：ephemeral 槽歸零
			}
		}
		kind := a.activeFeed
		f := a.feeds[kind]
		if a.searchOpen {
			kind, f = feedSearch, a.feeds[feedSearch] // 搜尋結果走 feed 渲染路徑
		}
		// 翻頁觸發：先於列表建構檢查（AtEnd 反映上一框的版面；首載以
		// posts 為空表達）。loadFeed 自帶防抖與終止條件（busy／end／
		// err），三槽同徑（含 feedSearch 的游標接續頁，F3；搜尋首頁
		// 起跑走 submitSearch）。
		if len(f.posts) == 0 || f.list.AtEnd() {
			a.loadFeed(kind)
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
				ui.Spinner(c)
			}
		})
		// 錯誤橫幅（契約 5）：顯示 errorText 文案與重試入口；既有列表
		// 原樣。錯誤後不自動重試（避免每框風暴），重試走顯式入口。
		if f.err != nil {
			ui.Row(c).Padding(8, 16).Gap(10).AlignItems(ui.Center).Background(t.Surface).Children(func() {
				ui.Text(c, Sanitize(errorText(f.err))).TextColor(t.Danger).Grow(1)
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
}
