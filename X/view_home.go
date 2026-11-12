// view_home：home 畫面——時間線列表、捲動近底觸發翻頁、到底提示、錯誤
// 橫幅（不清列表，REQ-LOG-042/043：任何錯誤絕不清憑證，也不清已載內容）。
// 翻頁觸發（契約 3/4）：首頁未載入，或列表於上一框已顯示到底（ui.List 的
// AtEnd 語義涵蓋「捲動近底」與「內容不足一屏」）；Cursor 空字串（或與前
// 一頁相同）→ 到底提示、不再觸發。去重與 seen 累積見 mergePosts。
package main

import (
	"context"

	"github.com/egoist/mygo/ui"
)

// homeEndText 是到底提示文案（契約 4）。
const homeEndText = "已載入全部貼文"

// homeView 繪製 home 畫面：標題列（載入中含 spinner）、錯誤橫幅、時間線
// 列表與到底提示。
func (a *app) homeView(c *ui.Context) {
	t := c.Theme()
	// 翻頁觸發：先於列表建構檢查（AtEnd 反映上一框的版面；首頁以
	// posts 為空表達）。loadMore 自帶防抖與終止條件（busy／end／err）。
	if len(a.posts) == 0 || a.list.AtEnd() {
		a.loadMore()
	}
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(12, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "時間線").FontSize(20).Bold().Grow(1)
			if a.homeBusy {
				ui.Spinner(c)
			}
		})
		// 錯誤橫幅（契約 5）：顯示 errorText 文案與重試入口；既有列表
		// 原樣。錯誤後不自動重試（避免每框風暴），重試走顯式入口。
		if a.homeErr != nil {
			ui.Row(c).Padding(8, 16).Gap(10).AlignItems(ui.Center).Background(t.Surface).Children(func() {
				ui.Text(c, Sanitize(errorText(a.homeErr))).TextColor(t.Danger).Grow(1)
				if ui.Button(c, "重試").Clicked() {
					a.homeErr = nil
					a.loadMore()
				}
			})
		}
		// 時間線列表：work_id 去重合併（首見順序）後的已顯示貼文；
		// 顯示欄位（衍生顯示字串）經 Sanitize 後上屏（契約 7）。
		ui.List(c, &a.list, len(a.posts), func(i int) {
			p := a.posts[i]
			ui.Column(c).Padding(10, 16).Gap(4).Children(func() {
				ui.Text(c, Sanitize("@"+p.ScreenName)).TextColor(t.TextMuted)
				if p.Title != "" {
					ui.Text(c, Sanitize(p.Title))
				}
				if p.WorkURL != "" {
					ui.Link(c, "開啟貼文", Sanitize(p.WorkURL))
				}
			})
		}).Grow(1)
		// 到底提示：游標空（或與前頁相同）後顯示，且不再觸發翻頁。
		if a.end {
			ui.Text(c, homeEndText).TextColor(t.TextMuted).Padding(12, 16)
		}
	})
}

// loadMore 載入下一頁（或首頁）：防抖與終止閘門（busy／已到底／有未處置
// 錯誤／無會話）→ 背景呼叫接縫 Home（契約 3：HomeTimeline(cursor, 20,
// seen)）→ 經 publish 回 UI 執行緒套用。錯誤只設 homeErr（橫幅呈現），
// 既有列表原樣（契約 5）。
func (a *app) loadMore() {
	if a.homeBusy || a.end || a.homeErr != nil || a.sess == nil {
		return
	}
	a.homeBusy = true
	a.homeErr = nil
	sess, cursor, seen := a.sess, a.cursor, a.seen
	a.goAsync(func(ctx context.Context) {
		res, err := sess.Home(cursor, seen)
		a.publish(func() {
			a.homeBusy = false
			if err != nil {
				a.homeErr = err // 既有列表原樣；由重試入口或近底捲動再觸發
				return
			}
			if res == nil {
				res = &timelineResult{}
			}
			prev := a.cursor
			a.mergePosts(res)
			if res.Cursor == "" || res.Cursor == prev {
				// 無游標或游標與前一頁相同 → 到底（CUJ 2），不再觸發。
				a.cursor, a.end = "", true
			} else {
				a.cursor = res.Cursor
			}
		})
	})
}

// mergePosts 合併一頁結果：work_id 去重、保留首見順序（契約 3）；seen
// （seenTweetIds）由已顯示貼文累積（跨頁去重，Task 10 介面契約）。
func (a *app) mergePosts(res *timelineResult) {
	known := make(map[string]bool, len(a.posts)+len(res.Posts))
	for _, p := range a.posts {
		known[p.WorkID] = true
	}
	for _, p := range res.Posts {
		if p.WorkID == "" || known[p.WorkID] {
			continue
		}
		known[p.WorkID] = true
		a.posts = append(a.posts, p)
		a.seen = append(a.seen, p.WorkID)
	}
}
