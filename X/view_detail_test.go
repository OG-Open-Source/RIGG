// view_detail 測試：展開檢視的播放器生命週期與 Kind 分派（Task 13；設計
// §8.6）——播放器以 openPlayer seam 注入記錄替身（僅計數 Close）、fetchImage
// seam 注入合成 PNG（測試夾具一律合成，不打真 CDN／真影片）；檢視以
// ui.Tester 無窗驅動，視窗 1280 寬（≥1024 右欄並列形態，本任務宿主）。
package main

import (
	"context"
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/api"
)

// fakePlayer 是 player seam 的記錄替身：僅計數 Close 呼叫（單一活躍規則
// 的斷言面；渲染處的型別斷言落空即走 Ready 前縮圖分支）。
type fakePlayer struct{ closes int }

func (f *fakePlayer) Close() { f.closes++ }

// stubOpenPlayer 以替身替換 openPlayer 接縫，測試後還原（接縫注入慣例：
// call-time 解析＋t.Cleanup 還原，同 stubFetchImage）。
func stubOpenPlayer(t *testing.T, fn func(url string) (player, error)) {
	t.Helper()
	orig := openPlayer
	openPlayer = fn
	t.Cleanup(func() { openPlayer = orig })
}

// detailPhotoPost 造一則 photo 媒體貼文：URL 為 name=large 形（展開檢視
// 大圖直取；ThumbURL 為 name=small，合成測試值非真 CDN）。
func detailPhotoPost() post {
	return post{
		WorkID:     "p1",
		WorkURL:    "https://x.com/u/status/p1",
		ScreenName: "u",
		Title:      "圖帖p1",
		Media: []api.MediaItem{{
			Kind:     "photo",
			ThumbURL: "https://pbs.twimg.test/media/Fp1?format=jpg&name=small",
			URL:      "https://pbs.twimg.test/media/Fp1?format=jpg&name=large",
			Width:    800,
			Height:   600,
		}},
	}
}

// detailVideoPost 造一則 video 媒體貼文：URL 為最高位元率 mp4（D13；合成
// 測試值非真影片）。
func detailVideoPost() post {
	return post{
		WorkID:     "v1",
		WorkURL:    "https://x.com/u/status/v1",
		ScreenName: "u",
		Title:      "影片帖v1",
		Media: []api.MediaItem{{
			Kind:     "video",
			ThumbURL: "https://pbs.twimg.test/media/Fv1?format=jpg&name=small",
			URL:      "https://video.twimg.test/Fv1.mp4",
		}},
	}
}

// detailGifPost 造一則 gif 媒體貼文：gif 與 video 同一解析路徑、亦以
// mp4 為 URL（D13；合成測試值非真影片）。
func detailGifPost() post {
	return post{
		WorkID:     "g1",
		WorkURL:    "https://x.com/u/status/g1",
		ScreenName: "u",
		Title:      "動圖帖g1",
		Media: []api.MediaItem{{
			Kind:     "gif",
			ThumbURL: "https://pbs.twimg.test/media/Fg1?format=jpg&name=small",
			URL:      "https://video.twimg.test/Fg1.mp4",
		}},
	}
}

// detailFixture 組裝已登入（fake session）的 home＋展開檢視測試環境：
// 視窗 1280 寬（≥1024 右欄並列形態）。
func detailFixture(t *testing.T, fake *fakeSession) (*app, *ui.Tester) {
	t.Helper()
	a := newTestApp()
	a.sess = fake
	tt := ui.NewTester(a.view, 1280, 720)
	return a, tt
}

// TestOpenDetailClosesPreviousPlayer（§8.6 單一活躍播放器）：連開兩帖 →
// 第一個 player 的 Close 被呼叫恰一次、第二個成為活躍（a.player）。
func TestOpenDetailClosesPreviousPlayer(t *testing.T) {
	var players []*fakePlayer
	stubOpenPlayer(t, func(string) (player, error) {
		p := &fakePlayer{}
		players = append(players, p)
		return p, nil
	})
	a, _ := detailFixture(t, &fakeSession{})
	a.openDetail(detailVideoPost())
	a.openDetail(detailVideoPost())
	if len(players) != 2 {
		t.Fatalf("連開兩帖應建兩個播放器，得 %d", len(players))
	}
	if players[0].closes != 1 {
		t.Errorf("第一個 player 的 Close 應被呼叫恰一次，得 %d", players[0].closes)
	}
	if players[1].closes != 0 {
		t.Errorf("活躍中的第二個 player 不得被關閉，得 %d", players[1].closes)
	}
	if a.player == nil || a.player != players[1] {
		t.Errorf("a.player 應為第二個播放器（單一活躍），得 %v", a.player)
	}
}

// TestCloseDetailClosesPlayer（§8.6 離開展開檢視）：closeDetail → 播放器
// Close 恰一次、a.player 與 a.detail 歸零；收合按鈕（view 路徑）同理；
// 其後再收合（冪等）不重複 Close。
func TestCloseDetailClosesPlayer(t *testing.T) {
	var players []*fakePlayer
	stubOpenPlayer(t, func(string) (player, error) {
		p := &fakePlayer{}
		players = append(players, p)
		return p, nil
	})
	a, tt := detailFixture(t, &fakeSession{})
	a.openDetail(detailVideoPost())
	a.closeDetail()
	if len(players) != 1 {
		t.Fatalf("開一帖應建一個播放器，得 %d", len(players))
	}
	if players[0].closes != 1 {
		t.Errorf("closeDetail 應 Close 播放器恰一次，得 %d", players[0].closes)
	}
	if a.player != nil {
		t.Errorf("closeDetail 後 a.player 應為 nil，得 %v", a.player)
	}
	if a.detail != nil {
		t.Errorf("closeDetail 後 a.detail 應為 nil，得 %v", a.detail)
	}
	a.closeDetail() // 冪等：無播放器／detail 時再收合不得重複 Close
	if players[0].closes != 1 {
		t.Errorf("重複收合不得再 Close，得 %d", players[0].closes)
	}
	// view 路徑：點收合按鈕 → 同樣 Close 恰一次、detail 歸零；媒體區不
	// 以關閉後狀態渲染（無失敗殘影）。
	a.openDetail(detailVideoPost())
	tt.Frame()
	if !tt.HasText(detailCloseText) {
		t.Fatalf("展開檢視應呈收合按鈕，texts %q", tt.Texts())
	}
	if err := tt.Click(detailCloseText); err != nil {
		t.Fatalf("點收合按鈕: %v", err)
	}
	// 收合幀：closeDetail 於建構中套用，MyGO 同幀重建（consumed → 下一
	// pass）——呈現幀即為 feed 單欄，不殘留展開檢視或媒體失敗文案。
	if tt.HasText(msgMedia) {
		t.Errorf("收合後不得殘留媒體失敗文案，texts %q", tt.Texts())
	}
	tt.Frame()
	if len(players) != 2 || players[1].closes != 1 {
		t.Errorf("收合按鈕應 Close 播放器恰一次，players=%d closes=%d", len(players), players[len(players)-1].closes)
	}
	if a.detail != nil || a.player != nil {
		t.Errorf("收合按鈕路徑應歸零 detail／player，detail=%v player=%v", a.detail, a.player)
	}
}

// TestDetailShowsLargeImageAndVideo（Kind 分派，§8.6）：photo 帖 →
// fetchImage 以 Media[0].URL（name=large）呼叫、解碼結果入展開狀態（不
// 走縮圖 LRU）；video 帖 → openPlayer 以 Media[0].URL（mp4）呼叫；gif 帖
// → 同 video 臂（D13：gif 走 mp4 variant，同一解析路徑）。
func TestDetailShowsLargeImageAndVideo(t *testing.T) {
	photo, videoPost := detailPhotoPost(), detailVideoPost()
	gotImage, gotPlayer := "", ""
	stubFetchImage(t, func(_ context.Context, url string) ([]byte, error) {
		gotImage = url
		return testPNG(t), nil
	})
	stubOpenPlayer(t, func(url string) (player, error) {
		gotPlayer = url
		return &fakePlayer{}, nil
	})
	a, tt := detailFixture(t, &fakeSession{})
	tt.Frame()
	// photo 帖：大圖走 fetchImage＋DecodeImage 直取 URL（name=large）。
	a.openDetail(photo)
	if gotImage != photo.Media[0].URL {
		t.Errorf("photo 帖應以 Media[0].URL（name=large）呼叫 fetchImage，得 %q, want %q", gotImage, photo.Media[0].URL)
	}
	if a.dmedia.img == nil {
		t.Error("photo 大圖應完成 DecodeImage 入展開狀態（dmedia.img 非 nil）")
	}
	if _, ok := thumbLRU.get(photo.Media[0].URL); ok {
		t.Error("大圖不得寫入縮圖 LRU（生命週期與展開頁同生共死）")
	}
	tt.Frame() // 右欄渲染展開檢視
	if !tt.HasText(photo.Title) {
		t.Errorf("展開檢視應顯示貼文（右欄），texts %q", tt.Texts())
	}
	// video 帖：播放器以 Media[0].URL（mp4）建構。
	a.openDetail(videoPost)
	if gotPlayer != videoPost.Media[0].URL {
		t.Errorf("video 帖應以 Media[0].URL（mp4）呼叫 openPlayer，得 %q, want %q", gotPlayer, videoPost.Media[0].URL)
	}
	if a.player == nil {
		t.Error("video 帖應建立單一活躍播放器")
	}
	// gif 帖（D13）：gif 與 video 同臂——openPlayer 以 Media[0].URL（mp4）
	// 建構單一活躍播放器。
	gif := detailGifPost()
	gotPlayer = ""
	a.openDetail(gif)
	if gotPlayer != gif.Media[0].URL {
		t.Errorf("gif 帖應以 Media[0].URL（mp4）呼叫 openPlayer（D13），得 %q, want %q", gotPlayer, gif.Media[0].URL)
	}
	if a.player == nil {
		t.Error("gif 帖應建立單一活躍播放器")
	}
}

// TestMediaFailureShowsExternalLink（媒體失敗 UX，§8.6／Alternative 11）：
// openPlayer 回錯（ErrUnsupported 同形）→ 錯誤文案＋WorkURL 外開連結呈
// 於展開檢視；點擊連結以外開 WorkURL。媒體失敗不落引擎 kind 文案。
func TestMediaFailureShowsExternalLink(t *testing.T) {
	stubOpenPlayer(t, func(string) (player, error) {
		return nil, errors.New("video: the system has no media framework that plays video")
	})
	p := detailVideoPost()
	a, tt := detailFixture(t, &fakeSession{})
	tt.Frame()
	a.openDetail(p)
	tt.Frame()
	if !tt.HasText(msgMedia) {
		t.Errorf("媒體失敗應顯示文案 %q，texts %q", msgMedia, tt.Texts())
	}
	if !tt.HasText(msgOpenExternal) {
		t.Errorf("媒體失敗應有外開連結 %q，texts %q", msgOpenExternal, tt.Texts())
	}
	if tt.HasText(msgTransport) || tt.HasText(msgUnknown) {
		t.Errorf("媒體失敗不得映射引擎 kind 文案，texts %q", tt.Texts())
	}
	if err := tt.Click(msgOpenExternal); err != nil {
		t.Fatalf("點擊外開連結: %v", err)
	}
	urls := tt.OpenedURLs()
	if len(urls) == 0 || urls[len(urls)-1] != p.WorkURL {
		t.Errorf("外開連結應開啟 WorkURL %q，得 %q", p.WorkURL, urls)
	}
}

// TestSwitchTombstoneClosesDetail（§8.6 切換帳號）：展開檢視開啟（播放
// 器活躍）時切換帳號 → 步驟 8 墓碑 closeDetail：播放器 Close 恰一次、
// detail／player 歸零——殘留播放器／detail 跨帳號即洩漏。
func TestSwitchTombstoneClosesDetail(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("7601"), accountKey("7602"))
	seedSwitchAccounts(t, "7601", "7602")
	var players []*fakePlayer
	stubOpenPlayer(t, func(string) (player, error) {
		p := &fakePlayer{}
		players = append(players, p)
		return p, nil
	})
	a := newTestApp()
	a.sess = &fakeSession{userID: "7601"}
	a.openDetail(detailVideoPost())
	if a.player == nil || a.detail == nil {
		t.Fatalf("前置：展開檢視應開啟且播放器活躍，detail=%v player=%v", a.detail, a.player)
	}
	withSwitchFactory(t, &fakeSession{userID: "7602"})
	a.switchTo("7602") // 同步 spawn／apply：切換序列於本行完成
	if len(players) != 1 || players[0].closes != 1 {
		t.Errorf("切換墓碑應 Close 播放器恰一次，closes=%d", players[0].closes)
	}
	if a.detail != nil || a.player != nil {
		t.Errorf("切換墓碑應歸零 detail／player，detail=%v player=%v", a.detail, a.player)
	}
	if a.gen != 1 {
		t.Errorf("切換應推進世代，得 %d", a.gen)
	}
}

// TestRowClickOpensDetailNotLink（展開檢視入口，Task 13 view_home 接線）：
// 點列主體開啟展開檢視（video 帖建播放器恰一次）；點列內的 WorkURL
// Link 不開帖——Link 自行收 clicks，外開 URL。
func TestRowClickOpensDetailNotLink(t *testing.T) {
	calls := 0
	stubOpenPlayer(t, func(string) (player, error) {
		calls++
		return &fakePlayer{}, nil
	})
	p := detailVideoPost()
	fake := &fakeSession{home: pageQueue(pageOf([]post{p}, ""))}
	a, tt := detailFixture(t, fake)
	for i := 0; i < 3; i++ {
		tt.Frame() // feed 載入（右欄形態，1280 寬）
	}
	if err := tt.Click(p.Title); err != nil {
		t.Fatalf("點列主體: %v", err)
	}
	tt.Frame()
	if a.detail == nil || a.detail.WorkID != p.WorkID {
		t.Fatalf("點列主體應開啟展開檢視，detail=%v", a.detail)
	}
	if calls != 1 {
		t.Errorf("video 帖應建播放器恰一次，calls=%d", calls)
	}
	// 收合後點列內 Link：不得開帖（Link 自行收 clicks）。
	a.closeDetail()
	tt.Frame()
	if err := tt.Click("開啟貼文"); err != nil {
		t.Fatalf("點列內 Link: %v", err)
	}
	tt.Frame()
	if a.detail != nil {
		t.Errorf("點列內 Link 不得開啟展開檢視，detail=%v", a.detail)
	}
	if len(tt.OpenedURLs()) == 0 {
		t.Errorf("點 Link 應外開 URL，得 %q", tt.OpenedURLs())
	}
}

// TestNarrowBandRowClickOpensDetail（final review F2 釘測，取代 T13 的
// 過渡窄帶閘釘注）：窄帶（900px，band 1）點媒體列同樣開啟展開檢視——以
// 全寬 detail 頁承接（§8.8 的 Router 頁形態不實作，控制器裁決
// 2026-11-15）：openPlayer 以媒體 URL 呼叫、feed 標示（含側欄）不可達、
// 帖文渲染；收合按鈕（detailView 內）closeDetail 返回 feed。
func TestNarrowBandRowClickOpensDetail(t *testing.T) {
	calls := 0
	stubOpenPlayer(t, func(string) (player, error) {
		calls++
		return &fakePlayer{}, nil
	})
	p := detailVideoPost()
	fake := &fakeSession{home: pageQueue(pageOf([]post{p}, ""))}
	a := newTestApp()
	a.sess = fake
	tt := ui.NewTester(a.view, 900, 720) // 窄帶：900px（band 1）
	for i := 0; i < 3; i++ {
		tt.Frame() // feed 載入（窄帶沿 feed 單欄＋側欄）
	}
	if err := tt.Click(p.Title); err != nil {
		t.Fatalf("點列主體: %v", err)
	}
	tt.Frame()
	if a.detail == nil || a.detail.WorkID != p.WorkID {
		t.Fatalf("窄帶點媒體列應開啟展開檢視，detail=%v", a.detail)
	}
	if calls != 1 {
		t.Errorf("video 帖應建播放器恰一次（openPlayer），calls=%d", calls)
	}
	if a.player == nil {
		t.Error("窄帶開帖應建立活躍播放器")
	}
	if tt.HasText("時間線") {
		t.Errorf("展開期間 feed 不可達（全寬 detail 頁，含側欄），texts %q", tt.Texts())
	}
	if !tt.HasText(p.Title) {
		t.Errorf("全寬展開檢視應渲染帖文，texts %q", tt.Texts())
	}
	// 收合返回 feed。
	if err := tt.Click(detailCloseText); err != nil {
		t.Fatalf("點收合按鈕: %v", err)
	}
	tt.Frame()
	if a.detail != nil || a.player != nil {
		t.Errorf("收合應歸零 detail／player，detail=%v player=%v", a.detail, a.player)
	}
	if !tt.HasText("時間線") {
		t.Errorf("收合後應返回 feed，texts %q", tt.Texts())
	}
}

// TestNarrowResizeRendersDetailFullWidth（final review F2 釘測）：≥1024
// 開帖（播放器活躍）後縮窗至 <1024 → 展開檢視以全寬 detail 頁延續（非
// 收合）：播放器不被關閉（生命週期隨 detail，§8.6）、收合按鈕仍在；收
// 合按鈕路徑 closeDetail 歸零 detail／player 並返回 feed。
func TestNarrowResizeRendersDetailFullWidth(t *testing.T) {
	var players []*fakePlayer
	stubOpenPlayer(t, func(string) (player, error) {
		p := &fakePlayer{}
		players = append(players, p)
		return p, nil
	})
	a := newTestApp()
	a.sess = &fakeSession{}
	tt := ui.NewTester(a.view, 1280, 720)
	a.openDetail(detailVideoPost())
	tt.Frame() // 右欄並列：展開檢視渲染
	if a.detail == nil || a.player == nil {
		t.Fatalf("前置：展開檢視應開啟且播放器活躍，detail=%v player=%v", a.detail, a.player)
	}
	tt.SetSize(900, 720)
	tt.Frame() // 窄幀：全寬 detail 頁延續
	if a.detail == nil || a.player == nil {
		t.Errorf("縮窗後展開檢視應以全寬頁延續，detail=%v player=%v", a.detail, a.player)
	}
	if len(players) != 1 || players[0].closes != 0 {
		t.Errorf("縮窗不得關閉播放器（檢視延續），players=%d closes=%d", len(players), players[len(players)-1].closes)
	}
	if !tt.HasText(detailCloseText) {
		t.Fatalf("窄帶應渲染收合按鈕，texts %q", tt.Texts())
	}
	if err := tt.Click(detailCloseText); err != nil {
		t.Fatalf("點收合按鈕: %v", err)
	}
	tt.Frame()
	if a.detail != nil || a.player != nil {
		t.Errorf("收合應歸零 detail／player，detail=%v player=%v", a.detail, a.player)
	}
	if len(players) != 1 || players[0].closes != 1 {
		t.Errorf("收合應 Close 播放器恰一次，players=%d closes=%d", len(players), players[len(players)-1].closes)
	}
}
