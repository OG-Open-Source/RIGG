// view_home：home 畫面——For You／Following 兩個時間線（ui.Tabs 切換，
// feeds 三槽隔離：切換只換渲染的槽，狀態互不污染；feedSearch 槽由 Task
// 10 的搜尋結果使用——ephemeral，行為面見 view_search.go）、頂列搜尋入
// 口（Enter 提交；點回任一 tab 關閉搜尋）、捲動近底觸發翻頁（逐 feed）、
// 到底提示、錯誤橫幅（不清列表，REQ-LOG-042/043：任何錯誤絕不清憑證，也
// 不清已載內容）。翻頁觸發（契約 3/4）：feed 未載入，或其列表於上一框
// 已顯示到底（ui.List 的 AtEnd 語義涵蓋「捲動近底」與「內容不足一屏」）；
// Cursor 空字串（或與前一頁相同）→ 到底提示、不再觸發。去重與 seen 累
// 積見 mergePosts。
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

// homeView 繪製 home 畫面：無展開貼文時為 feed（homeFeedView）；展開檢
// 視開啟且窗寬 ≥1024 時右欄並列（feed｜detailView，Task 13 宿主形態）
//——<1024 的 Router 頁形態屬 Task 14 佈局接線，本任務窄窗沿 feed 單欄
//（展開狀態保留，Task 14 接線後呈現）。
func (a *app) homeView(c *ui.Context) {
	if a.detail == nil {
		a.homeFeedView(c)
		return
	}
	if w, _ := c.Size(); w < 1024 {
		a.homeFeedView(c)
		return
	}
	d := *a.detail // 幀內快照：收合按鈕於本幀歸零 detail 不影響本幀渲染
	ui.Row(c).Fill().Children(func() {
		ui.Column(c).Grow(1).Children(func() {
			a.homeFeedView(c)
		})
		ui.Column(c).Width(detailPanelWidth).Background(c.Theme().Surface).Children(func() {
			a.detailView(c, d)
		})
	})
}

// homeFeedView 繪製 feed 單欄：tabs、標題列（載入中含 spinner）、錯誤橫
// 幅、活躍 feed 的時間線列表與到底提示。
func (a *app) homeFeedView(c *ui.Context) {
	t := c.Theme()
	// wasSearch 是進框前的檢視狀態：本框 Enter 剛開啟的搜尋不被同框的
	// 殘餘 tab 輸入關閉（關閉只對「已在檢視」的搜尋生效）。
	wasSearch := a.searchOpen
	ui.Column(c).Fill().Children(func() {
		// 頂列搜尋入口（Task 10）：MyGO 內建單行輸入（*string 綁定），
		// Enter（Submitted）提交 submitSearch（空 query 由其忽略）。版
		// 面暫置於 tabs 之上，Toolbar 收斂屬 Task 14。
		in := ui.TextInput(c, &a.searchInput).Placeholder("搜尋貼文").Label("搜尋")
		if in.Submitted() {
			a.submitSearch(a.searchInput)
		}
		// tabs（MyGO 內建）：點選即改寫 a.activeFeed——建構返回前該框
		// 輸入已套用，其後整框即以切換後的槽渲染（下方讀取 activeFeed
		// 取得新選擇）。搜尋結果（feedSearch）不在 tabs：MyGO Tabs 會
		// 把越界選擇鉗回範圍，故 activeFeed 恆為 tab 索引、搜尋以
		// searchOpen 另行標示——其間點回任一 tab（Changed）即關閉。
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
		// err）；feedSearch 非其管轄（大聲預設不派發，載入走 submitSearch）。
		if len(f.posts) == 0 || f.list.AtEnd() {
			a.loadFeed(kind)
		}
		ui.Row(c).Padding(12, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			title := "時間線"
			if a.searchOpen {
				title = "搜尋結果" // 搜尋指示：文字標示當前為搜尋結果（不新增第三 tab）
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
			if body.RowClicked() {
				a.openDetail(p) // 展開檢視入口（單一活躍播放器規則見 openDetail）
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
// HomeLatestTimeline，雙形態同構；cursor／seen 各自累積；feedSearch 非
// 本派發管轄——大聲預設不派發，Task 10 搜尋走 submitSearch）→ 經 publish
// 回 UI 執行緒套用。背景工作以會話子樹派發（goSession，§8.4），起跑前捕捉
// 世代（gen）與槽，回寫前比對——世代過期（帳號切換墓碑已歸零）或槽已被
// 換新的舊結果整批丟棄，不得污染新畫面（D18 世代邊界雙保險：cancel 斬
// 在途、gen 防已回程；Review Focus 1）。逐 feed 化（Task 8）：寫回一律指
// 向派發時捕捉的槽 kind，與其後的 tab 切換無關——結果屬於該 feed。錯誤
// 只設該槽 err（橫幅呈現），既有列表原樣（契約 5）。
func (a *app) loadFeed(kind int) {
	f := a.feeds[kind]
	if f.busy || f.end || f.err != nil || a.sess == nil {
		return
	}
	// 大聲預設（Task 8 review 釘注）：本派發僅管轄 For You／Following
	// 兩槽——未知 kind（含 feedSearch：其載入走 submitSearch，Task 10）
	// 一律不派發、快速失敗。須在佔用 busy 之前攔下：誤入的呼叫不得把
	// 槽卡在 busy（AtEnd 逐框重入即永久佔住）；亦不寫 err（橫幅將成
	// 逐框風暴）。
	switch kind {
	case feedForYou, feedFollowing:
	default:
		return
	}
	f.busy = true
	f.err = nil
	sess, cursor, seen, gen := a.sess, f.cursor, f.seen, a.gen
	a.goSession(func(ctx context.Context) {
		var res *timelineResult
		var err error
		if kind == feedFollowing {
			res, err = sess.HomeLatest(cursor, seen) // Following＝最新時間線
		} else {
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
