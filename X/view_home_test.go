// view_home 測試：以 ui.Tester 無窗驅動 home view，斷言 brief 行為契約
// 3（去重合併、首見順序、seenTweetIds 累積）、4（游標空 → 到底、不再
// 觸發）、5（錯誤不清列表），以及 Task 8 的 feeds 三槽隔離（tabs 切換
// 不互相污染）、逐 feed 翻頁觸發與 seen 上限截尾（Review Focus 4）。
// 翻頁觸發以列表「內容不足一屏即 AtEnd」的語義呈現（近底捲動與此同
// 值）；測試以 tt.Frame() 推進各框。
package main

import (
	"slices"
	"strconv"
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
	if got := workIDs(a.feeds[feedForYou].posts); !sameStrings(got, want) {
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
	if got := workIDs(a.feeds[feedForYou].posts); !sameStrings(got, []string{"a", "b"}) {
		t.Errorf("翻頁失敗後既有列表應原樣，得 %q", got)
	}
	if a.feeds[feedForYou].end || tt.HasText(homeEndText) {
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
	if !a.feeds[feedForYou].end {
		t.Error("游標空後 app 應標記到底（不再觸發）")
	}
	if got := len(fake.callLog()); got != 1 {
		t.Errorf("到底後不得再呼叫 fake，Home 呼叫數 = %d（日誌 %q）", got, fake.callLog())
	}
}

// TestSeenCapTrimsTail（Review Focus 4 釘測）：merge 501 帖 → seen 截尾
// 至上限 500、保留最近（丟最舊）——最早被丟的 work_id 不在 seen、最新
// 的在；上限僅約束 seen（顯示列表 posts 不截尾）。
func TestSeenCapTrimsTail(t *testing.T) {
	f := &timelineState{}
	ids := make([]string, 0, seenCap+1)
	for i := 1; i <= seenCap+1; i++ {
		ids = append(ids, "w"+strconv.Itoa(i))
	}
	f.mergePosts(pageOf(mkPosts(ids...), ""))
	if len(f.seen) != seenCap {
		t.Fatalf("seen 應截尾至上限 %d，得 %d", seenCap, len(f.seen))
	}
	if slices.Contains(f.seen, ids[0]) {
		t.Errorf("最早的 seen %s 應被截尾丟棄（保留最近 %d 筆）", ids[0], seenCap)
	}
	if got, want := f.seen[0], ids[1]; got != want {
		t.Errorf("截尾後最舊的保留者應為 %s，得 %s", want, got)
	}
	if got, want := f.seen[len(f.seen)-1], ids[seenCap]; got != want {
		t.Errorf("最新的 seen 應保留：尾筆 = %s，欲得 %s", got, want)
	}
	if len(f.posts) != seenCap+1 {
		t.Errorf("上限僅約束 seen（posts 不截尾）：posts = %d，欲得 %d", len(f.posts), seenCap+1)
	}
}

// TestTabsKeepIndependentState：For You 載兩頁 → 點選 Following 載一頁
// → 切回 For You——posts/cursor/list 原樣、seen 互不混入；兩 feed 的
// Home／HomeLatest 呼叫次數正確（三槽隔離，切換不互相污染）。
func TestTabsKeepIndependentState(t *testing.T) {
	fake := &fakeSession{
		home: pageQueue(
			pageOf(mkPosts("a1", "a2"), "c2"),
			pageOf(mkPosts("a3"), ""),
		),
		homeLatest: pageQueue(pageOf(mkPosts("b1", "b2"), "")),
	}
	a, tt := homeFixture(t, fake)
	for i := 0; i < 6; i++ {
		tt.Frame() // For You：首頁＋近底翻頁 → 到底
	}
	fy := a.feeds[feedForYou]
	wantPosts, wantCursor := workIDs(fy.posts), fy.cursor
	if !sameStrings(wantPosts, []string{"a1", "a2", "a3"}) || wantCursor != "" || !fy.end {
		t.Fatalf("前置：For You 應載兩頁到底，posts=%q cursor=%q end=%v", wantPosts, fy.cursor, fy.end)
	}
	f0, l0 := fy.list.Visible()
	// 切 Following（tab 點選）：載一頁即到底（HomeLatest）。
	if err := tt.Click("Following"); err != nil {
		t.Fatalf("點選 Following tab: %v", err)
	}
	fw := a.feeds[feedFollowing]
	if a.activeFeed != feedFollowing || !sameStrings(workIDs(fw.posts), []string{"b1", "b2"}) || !fw.end {
		t.Fatalf("Following 應載一頁到底：activeFeed=%d posts=%q end=%v", a.activeFeed, workIDs(fw.posts), fw.end)
	}
	// 切回 For You：狀態原樣。
	if err := tt.Click("For You"); err != nil {
		t.Fatalf("點選 For You tab: %v", err)
	}
	tt.Frame()
	if got := workIDs(fy.posts); !sameStrings(got, wantPosts) {
		t.Errorf("切回後 For You posts 應原樣，得 %q，欲得 %q", got, wantPosts)
	}
	if got := fy.cursor; got != wantCursor {
		t.Errorf("切回後 For You cursor 應原樣，得 %q，欲得 %q", got, wantCursor)
	}
	if f1, l1 := fy.list.Visible(); f1 != f0 || l1 != l0 {
		t.Errorf("切回後 For You 列表版位應原樣：Visible = (%d,%d)，欲得 (%d,%d)", f1, l1, f0, l0)
	}
	// seen 各自累積、互不混入。
	if !sameStrings(fy.seen, []string{"a1", "a2", "a3"}) {
		t.Errorf("For You seen 應原樣（不被 Following 污染），得 %q", fy.seen)
	}
	if !sameStrings(fw.seen, []string{"b1", "b2"}) {
		t.Errorf("Following seen 應獨立累積，得 %q", fw.seen)
	}
	// 呼叫次數：For You 兩頁（Home）、Following 一頁（HomeLatest）。
	if got := countCalls(fake.callLog(), "Home"); got != 2 {
		t.Errorf("Home 應呼叫 2 次（For You 兩頁），得 %d（日誌 %q）", got, fake.callLog())
	}
	if got := countCalls(fake.callLog(), "HomeLatest"); got != 1 {
		t.Errorf("HomeLatest 應呼叫 1 次（Following 一頁），得 %d（日誌 %q）", got, fake.callLog())
	}
}

// TestAtEndTriggersPerFeed：僅活躍 feed 觸發翻頁——Following 活躍期間的
// AtEnd 不得觸發 For You 的 loadFeed（For You 游標 c2 仍待翻頁、非 busy
// 亦然）；切回後才續翻，且收到的 seen 是 For You 自己的累積。apply 以佇
// 列模擬主執行緒（spawn 同步、publish 於框外由測試套用——TestLoginView_
// Debounce 的模板去 goroutine 形，序列全可控）。
func TestAtEndTriggersPerFeed(t *testing.T) {
	var a *app
	var pending []func()
	calls := 0
	fake := &fakeSession{
		home: func(string, []string) (*timelineResult, error) {
			calls++
			if calls == 1 {
				return pageOf(mkPosts("a1", "a2"), "c2"), nil
			}
			return &timelineResult{}, nil // 第二頁起：空頁（無游標 → 到底）
		},
		homeLatest: pageQueue(
			pageOf(mkPosts("b1", "b2"), "d2"),
			pageOf(mkPosts("b3"), ""),
		),
	}
	// 夾具手動組裝（不用 homeFixture）：apply 佇列須於 NewTester 之前接
	// 線——NewTester 會先建一個初始框，其內的首載結果須入佇列（而非同
	// 步套用），For You 的「已載首頁、待翻頁」凍結態才可重現。
	a = newTestApp()
	a.sess = fake
	a.apply = func(fn func()) { pending = append(pending, fn) }
	tt := ui.NewTester(a.view, 480, 720)
	flushAll := func() {
		for len(pending) > 0 {
			fns := pending
			pending = nil
			for _, fn := range fns {
				fn()
			}
		}
	}

	tt.Frame() // For You 首頁工作派出（結果入佇列）
	flushAll() // 套用：posts=[a1 a2]、cursor=c2（未到底、非 busy）
	fy := a.feeds[feedForYou]
	if !sameStrings(workIDs(fy.posts), []string{"a1", "a2"}) || fy.cursor != "c2" || fy.end || fy.busy {
		t.Fatalf("前置：For You 應載首頁待翻頁，posts=%q cursor=%q end=%v busy=%v", workIDs(fy.posts), fy.cursor, fy.end, fy.busy)
	}
	// 切 Following：載入、AtEnd 翻頁、到底（homeLatest 兩頁後停止）。
	a.activeFeed = feedFollowing
	for i := 0; i < 3; i++ {
		tt.Frame()
		flushAll()
	}
	if got := countCalls(fake.callLog(), "Home"); got != 1 {
		t.Errorf("Following 活躍期間的 AtEnd 不得觸發 For You 翻頁：Home 呼叫 %d 次（日誌 %q）", got, fake.callLog())
	}
	if got := countCalls(fake.callLog(), "HomeLatest"); got != 2 {
		t.Errorf("Following 應自走兩頁（AtEnd 觸發），HomeLatest 呼叫 %d 次（日誌 %q）", got, fake.callLog())
	}
	if !sameStrings(workIDs(fy.posts), []string{"a1", "a2"}) || fy.cursor != "c2" || fy.end || fy.busy {
		t.Errorf("For You 槽應原樣（未翻頁）：posts=%q cursor=%q end=%v busy=%v", workIDs(fy.posts), fy.cursor, fy.end, fy.busy)
	}
	// 切回 For You：近底（AtEnd 語義）→ 續翻其第二頁。
	a.activeFeed = feedForYou
	tt.Frame()
	flushAll()
	if got := countCalls(fake.callLog(), "Home"); got != 2 {
		t.Errorf("切回 For You 後應續翻（僅活躍 feed），Home 呼叫 %d 次（日誌 %q）", got, fake.callLog())
	}
	if !fy.end {
		t.Error("第二頁無游標後 For You 應標記到底")
	}
	// 翻頁收到的 seen 是 For You 自己的累積（不含 Following 的 b*）。
	if seen := fake.seenOf(); !sameStrings(seen, []string{"a1", "a2"}) {
		t.Errorf("For You 翻頁應帶自身 seen，得 %q，欲得 [a1 a2]", seen)
	}
}
