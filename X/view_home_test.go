// view_home 測試：以 ui.Tester 無窗驅動 home view，斷言 brief 行為契約
// 3（去重合併、首見順序、seenTweetIds 累積）、4（游標空 → 到底、不再
// 觸發）、5（錯誤不清列表）。翻頁觸發以列表「內容不足一屏即 AtEnd」的
// 語義呈現（近底捲動與此同值）；測試以 tt.Frame() 推進各框。
package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// mkPosts 造一組貼文（WorkID 為唯一識別；Title 供 HasText 斷言）。
func mkPosts(ids ...string) []post {
	out := make([]post, 0, len(ids))
	for _, id := range ids {
		out = append(out, post{
			WorkID:     id,
			WorkURL:    "https://x.com/u/status/" + id,
			ScreenName: "u",
			Title:      "貼文" + id,
		})
	}
	return out
}

// pageOf 以貼文與游標組一頁結果。
func pageOf(posts []post, cursor string) *timelineResult {
	return &timelineResult{Posts: posts, Cursor: cursor}
}

// pageQueue 回依序彈出頁面的 home 腳本；佇列耗盡後回空頁（無游標）。
func pageQueue(pages ...*timelineResult) func(string, []string) (*timelineResult, error) {
	return func(string, []string) (*timelineResult, error) {
		if len(pages) == 0 {
			return &timelineResult{}, nil
		}
		p := pages[0]
		pages = pages[1:]
		return p, nil
	}
}

// homeFixture 組裝已登入（fake session）的 home 測試環境。
func homeFixture(t *testing.T, fake *fakeSession) (*app, *ui.Tester) {
	t.Helper()
	a := newTestApp()
	a.sess = fake
	tt := ui.NewTester(a.view, 480, 720)
	return a, tt
}

// workIDs 回當前列表的 WorkID 序（斷言去重與首見順序）。
func workIDs(posts []post) []string {
	out := make([]string, 0, len(posts))
	for _, p := range posts {
		out = append(out, p.WorkID)
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestHomeView_Dedup（契約 3）：兩頁含重複 work_id → 列表不重複、首見
// 順序保留；seenTweetIds 由已顯示貼文累積（第二次呼叫帶首頁全部 work_id）。
func TestHomeView_Dedup(t *testing.T) {
	fake := &fakeSession{home: pageQueue(
		pageOf(mkPosts("a", "b", "c"), "p2"),
		pageOf(mkPosts("b", "d"), ""), // 第二頁含重複 work_id；游標空 → 到底
	)}
	a, tt := homeFixture(t, fake)
	for i := 0; i < 6; i++ {
		tt.Frame() // 推進：首頁載入 → 近底觸發第二頁 → 到底
	}
	want := []string{"a", "b", "c", "d"}
	if got := workIDs(a.posts); !sameStrings(got, want) {
		t.Errorf("去重合併後 work_id 序 = %q, want %q（首見順序、無重複）", got, want)
	}
	if !tt.HasText("貼文d") {
		t.Errorf("列表應顯示合併後的貼文，texts %q", tt.Texts())
	}
	if !tt.HasText(homeEndText) {
		t.Errorf("游標空應顯示到底提示 %q，texts %q", homeEndText, tt.Texts())
	}
	// seenTweetIds 由已顯示貼文累積（跨頁去重，Task 10 介面契約）：
	// 第二次呼叫應帶首頁全部 work_id。
	if seen := fake.seenOf(); !sameStrings(seen, []string{"a", "b", "c"}) {
		t.Errorf("第二頁呼叫的 seen = %q, want [a b c]", seen)
	}
	if got := len(fake.callLog()); got != 2 {
		t.Errorf("兩頁後應停止翻頁，Home 呼叫數 = %d（日誌 %q）", got, fake.callLog())
	}
}

// TestHomeView_ErrorKeepsList（契約 5，CUJ 3）：翻頁失敗 → 錯誤橫幅顯示
// 文案與重試入口、既有列表原樣；錯誤後不得自動重試（避免每框重試風暴）。
func TestHomeView_ErrorKeepsList(t *testing.T) {
	calls := 0
	fake := &fakeSession{home: func(string, []string) (*timelineResult, error) {
		calls++
		if calls == 1 {
			return pageOf(mkPosts("a", "b"), "p2"), nil
		}
		return nil, &xerrors.TransportError{Msg: "連線逾時"}
	}}
	a, tt := homeFixture(t, fake)
	for i := 0; i < 6; i++ {
		tt.Frame()
	}
	if !tt.HasText(msgTransport) {
		t.Errorf("錯誤橫幅應顯示 transport 文案，texts %q", tt.Texts())
	}
	if !tt.HasText("重試") {
		t.Error("錯誤橫幅應提供重試入口")
	}
	if got := workIDs(a.posts); !sameStrings(got, []string{"a", "b"}) {
		t.Errorf("翻頁失敗後既有列表應原樣，得 %q", got)
	}
	if a.end || tt.HasText(homeEndText) {
		t.Error("錯誤路徑不得顯示到底提示")
	}
	if calls != 2 {
		t.Errorf("錯誤後不得自動重試，Home 呼叫數 = %d", calls)
	}
}

// TestHomeView_EndOfFeed（契約 4）：Cursor 空字串 → 顯示到底提示、不再
// 觸發（後續 frame 不再呼叫 fake）。
func TestHomeView_EndOfFeed(t *testing.T) {
	fake := &fakeSession{home: pageQueue(pageOf(mkPosts("x"), ""))}
	a, tt := homeFixture(t, fake)
	for i := 0; i < 4; i++ {
		tt.Frame()
	}
	if !tt.HasText(homeEndText) {
		t.Errorf("游標空應顯示到底提示 %q，texts %q", homeEndText, tt.Texts())
	}
	if !tt.HasText("貼文x") {
		t.Errorf("貼文應已顯示，texts %q", tt.Texts())
	}
	if !a.end {
		t.Error("游標空後 app 應標記到底（不再觸發）")
	}
	if got := len(fake.callLog()); got != 1 {
		t.Errorf("到底後不得再呼叫 fake，Home 呼叫數 = %d（日誌 %q）", got, fake.callLog())
	}
}
