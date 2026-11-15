// view_search 測試：以 ui.Tester 無窗驅動頂列搜尋入口（Task 10）——
// 斷言 brief 契約：提交後結果落 feeds[feedSearch]（游標與 end 正確）、
// 搜尋不觸及 For You 的 seen（逐位元組不變）、點回 tab 關閉（槽歸零、
// 不影響他 feed）、product 恆 "Top"（Task 9 裁決：必填變數）；另釘空
// query 忽略與 busy 防抖。fakeSession 注入（view_login_test.go，search
// 腳本同 home/homeLatest 形）；夾具全合成、不打真實網路。
package main

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// typeSearch 點選搜尋框、鍵入 query 並以 Enter 提交（頂列入口的 UI 徑）。
func typeSearch(t *testing.T, tt *ui.Tester, query string) {
	t.Helper()
	if err := tt.Click("搜尋"); err != nil {
		t.Fatalf("點選搜尋框: %v", err)
	}
	tt.Type(query)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
}

// TestSubmitSearchPopulatesSearchFeed：fakeSession.Search 回 fixture →
// posts 落 feeds[feedSearch]、游標與 end 正確、busy 復位，且畫面以文字
// 標示當前為搜尋結果（tabs 不新增第三 tab）。apply 以佇列模擬主執行緒
// 時序（TestAtEndTriggersPerFeed 模板——佇列須於 NewTester 前接線）：套
// 用延後至幀間，使「首頁結果已套用、AtEnd 尚未評估」的凍結態可斷言
// （AtEnd 翻頁契約見 TestSearchAtEndPagesWithStoredCursor）。
func TestSubmitSearchPopulatesSearchFeed(t *testing.T) {
	fake := &fakeSession{
		home: pageQueue(pageOf(mkPosts("a1", "a2"), "c2")),
		search: func(string, string, string) (*timelineResult, error) {
			return pageOf(mkPosts("s1", "s2"), "sp2"), nil
		},
	}
	a := newTestApp()
	a.sess = fake
	var pending []func()
	a.apply = func(fn func()) { pending = append(pending, fn) }
	tt := ui.NewTester(a.view, 480, 720)
	flush := func() {
		for len(pending) > 0 {
			fns := pending
			pending = nil
			for _, fn := range fns {
				fn()
			}
		}
	}
	tt.Frame() // For You 首頁工作派出（結果入佇列）
	flush()
	typeSearch(t, tt, "MyGO") // Frame 內提交：槽起跑（busy 佔住，AtEnd 不派發）
	flush()                   // 套用首頁結果
	f := a.feeds[feedSearch]
	if !a.searchOpen {
		t.Fatal("提交後應檢視搜尋結果（searchOpen）")
	}
	if !sameStrings(workIDs(f.posts), []string{"s1", "s2"}) {
		t.Errorf("搜尋結果應落 feedSearch 槽，得 %q", workIDs(f.posts))
	}
	if f.cursor != "sp2" {
		t.Errorf("游標應留存 %q，得 %q", "sp2", f.cursor)
	}
	if f.end {
		t.Error("游標非空不得標記到底")
	}
	if f.busy {
		t.Error("完成後 busy 應復位")
	}
	if got := countCalls(fake.callLog(), "Search"); got != 1 {
		t.Errorf("Search 應呼叫 1 次，得 %d（日誌 %q）", got, fake.callLog())
	}
	// 重繪：列表以首頁結果呈現（AtEnd 評估會派發翻頁——本 fixture 第二
	// 頁與首頁同形，去重後游標同前頁 → 到底；翻頁契約另見釘測）。
	tt.Frame()
	flush()
	if !tt.HasText("搜尋結果") || !tt.HasText("貼文s1") {
		t.Errorf("應以文字標示搜尋結果並顯示貼文，texts %q", tt.Texts())
	}
}

// TestSearchDoesNotTouchForYouSeen：搜尋前後 feeds[feedForYou].seen 逐位
// 元組不變——搜尋結果（含其槽內 seen 累積）絕不混入 For You 的 seen。
func TestSearchDoesNotTouchForYouSeen(t *testing.T) {
	fake := &fakeSession{
		home: pageQueue(pageOf(mkPosts("a1", "a2"), "")),
		search: func(string, string, string) (*timelineResult, error) {
			return pageOf(mkPosts("s1", "s2"), ""), nil
		},
	}
	a, tt := homeFixture(t, fake)
	for i := 0; i < 3; i++ {
		tt.Frame() // For You 載一頁到底（seen 累積）
	}
	fy := a.feeds[feedForYou]
	if !sameStrings(fy.seen, []string{"a1", "a2"}) {
		t.Fatalf("前置：For You seen 應累積，得 %q", fy.seen)
	}
	before := slices.Clone(fy.seen)
	a.submitSearch("MyGO") // 直接呼叫：行為面與 UI 提交同徑（UI 徑見上一測試）
	tt.Frame()
	if !slices.Equal(before, a.feeds[feedForYou].seen) {
		t.Errorf("搜尋不得觸及 For You seen：前 %q，後 %q", before, a.feeds[feedForYou].seen)
	}
	if !sameStrings(workIDs(a.feeds[feedSearch].posts), []string{"s1", "s2"}) {
		t.Errorf("搜尋結果應落自身槽，得 %q", workIDs(a.feeds[feedSearch].posts))
	}
}

// TestClosingSearchClearsState：點回 For You/Following tab → feeds[feedSearch]
// 歸零槽（posts/seen/cursor/busy/end 全清、指標替換），For You／Following
// 兩槽原樣不受影響。
func TestClosingSearchClearsState(t *testing.T) {
	fake := &fakeSession{
		home:       pageQueue(pageOf(mkPosts("a1"), "")),
		homeLatest: pageQueue(pageOf(mkPosts("b1"), "")),
		search: func(string, string, string) (*timelineResult, error) {
			return pageOf(mkPosts("s1", "s2"), ""), nil
		},
	}
	a, tt := homeFixture(t, fake)
	for i := 0; i < 3; i++ {
		tt.Frame() // For You 載入
	}
	// 前置：切至 Following（使「點回 For You」為真切換）→ 提交搜尋。
	if err := tt.Click("Following"); err != nil {
		t.Fatalf("點選 Following tab: %v", err)
	}
	tt.Frame()
	a.submitSearch("MyGO")
	tt.Frame()
	tt.Frame() // submitSearch 為狀態直呼：其後兩幀使搜尋檢視穩定可互動（測試慣例）
	if !a.searchOpen || len(a.feeds[feedSearch].posts) != 2 {
		t.Fatalf("前置：搜尋應已呈現，open=%v posts=%q", a.searchOpen, workIDs(a.feeds[feedSearch].posts))
	}
	if err := tt.Click("For You"); err != nil {
		t.Fatalf("點回 For You tab: %v", err)
	}
	tt.Frame()
	if a.searchOpen {
		t.Error("點回 tab 應關閉搜尋")
	}
	fs := a.feeds[feedSearch]
	if len(fs.posts) != 0 || len(fs.seen) != 0 || fs.cursor != "" || fs.busy || fs.end || fs.err != nil {
		t.Errorf("feedSearch 槽應歸零：posts=%d seen=%d cursor=%q busy=%v end=%v err=%v",
			len(fs.posts), len(fs.seen), fs.cursor, fs.busy, fs.end, fs.err)
	}
	if !sameStrings(workIDs(a.feeds[feedForYou].posts), []string{"a1"}) {
		t.Errorf("For You 槽應不受影響，得 %q", workIDs(a.feeds[feedForYou].posts))
	}
	if !sameStrings(workIDs(a.feeds[feedFollowing].posts), []string{"b1"}) {
		t.Errorf("Following 槽應不受影響，得 %q", workIDs(a.feeds[feedFollowing].posts))
	}
	if !tt.HasText("時間線") {
		t.Errorf("關閉後應回到時間線標示，texts %q", tt.Texts())
	}
}

// TestSearchPassesProductTop：product 參數恆 "Top"（Task 9 裁決：
// SearchTimeline 的 product 為必填變數，搜尋結果頁用 "Top"）；query／
// cursor 如提交傳遞。
func TestSearchPassesProductTop(t *testing.T) {
	fake := &fakeSession{}
	a := newTestApp()
	a.sess = fake
	a.submitSearch("MyGO")
	if got := countCalls(fake.callLog(), "Search"); got != 1 {
		t.Fatalf("Search 應呼叫 1 次，得 %d（日誌 %q）", got, fake.callLog())
	}
	if fake.lastProduct != "Top" {
		t.Errorf("product 應為 %q（Task 9 裁決），得 %q", "Top", fake.lastProduct)
	}
	if fake.lastQuery != "MyGO" || fake.lastCursor != "" {
		t.Errorf("query/cursor 應如提交傳遞，得 (%q, %q)", fake.lastQuery, fake.lastCursor)
	}
}

// TestSubmitSearchIgnoresEmptyQuery：空（含全空白）query 提交一律忽略——
// 不派發、不切換檢視、不改動槽。
func TestSubmitSearchIgnoresEmptyQuery(t *testing.T) {
	calls := 0
	fake := &fakeSession{search: func(string, string, string) (*timelineResult, error) {
		calls++
		return &timelineResult{}, nil
	}}
	a := newTestApp()
	a.sess = fake
	a.submitSearch("")
	a.submitSearch("   ")
	if calls != 0 {
		t.Errorf("空 query 不得派發，Search 呼叫 %d 次", calls)
	}
	if a.searchOpen || len(a.feeds[feedSearch].posts) != 0 {
		t.Error("空 query 不得切換至搜尋結果或改動槽")
	}
}

// TestSearchBusyGuardsResubmit：搜尋進行中（busy 未復位）再提交一律忽略
// （沿 homeBusy 防抖模式）；結果套用後 busy 復位、槽內容正確。
func TestSearchBusyGuardsResubmit(t *testing.T) {
	calls := 0
	fake := &fakeSession{search: func(string, string, string) (*timelineResult, error) {
		calls++
		return pageOf(mkPosts("s1"), ""), nil
	}}
	a := newTestApp()
	a.sess = fake
	var pending []func()
	a.apply = func(fn func()) { pending = append(pending, fn) } // 結果延後套用
	a.submitSearch("MyGO")                                      // 派發（結果入佇列，busy 佔住）
	a.submitSearch("MyGO")                                      // busy 中再提交：防抖忽略
	if calls != 1 {
		t.Fatalf("busy 期間再提交應忽略，Search 呼叫 %d 次", calls)
	}
	for len(pending) > 0 {
		fns := pending
		pending = nil
		for _, fn := range fns {
			fn()
		}
	}
	f := a.feeds[feedSearch]
	if f.busy || !sameStrings(workIDs(f.posts), []string{"s1"}) || !f.end {
		t.Errorf("套用後應完成：busy=%v posts=%q end=%v", f.busy, workIDs(f.posts), f.end)
	}
}

// TestSearchAtEndPagesWithStoredCursor（§8.7 翻頁契約，final review F3 釘
// 測）：首頁結果帶非空游標 → 列表 AtEnd 觸發第二頁——Search 以槽內留存
// 游標呼叫（無 seen 參數）、結果去重合併入同一槽；第二頁游標空 → 到底、
// 不再觸發。apply 以佇列模擬主執行緒時序（TestAtEndTriggersPerFeed 模
// 板）：套用延後至幀間，AtEnd 於其後的框反映上一框版面。
func TestSearchAtEndPagesWithStoredCursor(t *testing.T) {
	var cursors []string
	fake := &fakeSession{search: func(q, product, cursor string) (*timelineResult, error) {
		cursors = append(cursors, cursor)
		if cursor == "" {
			return pageOf(mkPosts("s1", "s2"), "sp2"), nil // 首頁：游標 sp2
		}
		return pageOf(mkPosts("s2", "s3"), ""), nil // 第二頁：含重複、游標空 → 到底
	}}
	a := newTestApp()
	a.sess = fake
	var pending []func()
	a.apply = func(fn func()) { pending = append(pending, fn) }
	tt := ui.NewTester(a.view, 480, 720)
	flush := func() {
		for len(pending) > 0 {
			fns := pending
			pending = nil
			for _, fn := range fns {
				fn()
			}
		}
	}
	typeSearch(t, tt, "MyGO") // Frame 內提交：feedSearch 槽起跑（busy 佔住）
	flush()                   // 套用首頁結果
	f := a.feeds[feedSearch]
	if !sameStrings(workIDs(f.posts), []string{"s1", "s2"}) || f.cursor != "sp2" || f.end {
		t.Fatalf("前置：首頁結果應落槽（游標留存），posts=%q cursor=%q end=%v", workIDs(f.posts), f.cursor, f.end)
	}
	if len(cursors) != 1 || cursors[0] != "" {
		t.Fatalf("前置：首頁應以空游標起跑，cursors=%q", cursors)
	}
	tt.Frame() // AtEnd → 以留存游標派發第二頁
	flush()
	if len(cursors) != 2 || cursors[1] != "sp2" {
		t.Errorf("AtEnd 應以槽內留存游標翻頁，cursors=%q", cursors)
	}
	if !sameStrings(workIDs(f.posts), []string{"s1", "s2", "s3"}) {
		t.Errorf("第二頁結果應去重合併入槽，得 %q", workIDs(f.posts))
	}
	if !f.end || f.cursor != "" {
		t.Errorf("第二頁游標空應到底：end=%v cursor=%q", f.end, f.cursor)
	}
	tt.Frame() // 到底後不得再觸發
	flush()
	if got := countCalls(fake.callLog(), "Search"); got != 2 {
		t.Errorf("到底後不得再觸發，Search 呼叫 %d 次（日誌 %q）", got, fake.callLog())
	}
}
