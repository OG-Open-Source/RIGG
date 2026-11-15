// view_detail 測試：展開檢視的播放器生命週期與 Kind 分派（Task 13；設計
// §8.6＋D24 播放修訂）——播放器以 openPlayer（video 暫存檔路徑）與
// newGifPlayer（GIF 純 Go 解碼）兩接縫注入記錄替身（僅計數 Close）；下載
// 以 fetchMediaBytes／fetchMediaToFile 接縫注入合成資料（測試夾具一律合
// 成，不打真 CDN／真影片）；goClose 注入同步執行（Close 計數斷言確定
// 性）。檢視以 ui.Tester 無窗驅動，視窗 1280 寬（≥1024 右欄並列形態）。
package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"os"
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
func stubOpenPlayer(t *testing.T, fn func(path string) (player, error)) {
	t.Helper()
	orig := openPlayer
	openPlayer = fn
	t.Cleanup(func() { openPlayer = orig })
}

// stubNewGifPlayer 以替身替換 newGifPlayer 接縫（D24 GIF 純 Go 播放器建
// 構），測試後還原。
func stubNewGifPlayer(t *testing.T, fn func(data []byte) (player, error)) {
	t.Helper()
	orig := newGifPlayer
	newGifPlayer = fn
	t.Cleanup(func() { newGifPlayer = orig })
}

// stubFetchMediaBytes／stubFetchMediaToFile 以替身替換 D24 播放下載接縫
// （正式路徑打真 CDN——測試一律合成、不打網路），測試後還原。
func stubFetchMediaBytes(t *testing.T, fn func(ctx context.Context, url string, maxBytes int64) ([]byte, error)) {
	t.Helper()
	orig := fetchMediaBytes
	fetchMediaBytes = fn
	t.Cleanup(func() { fetchMediaBytes = orig })
}

func stubFetchMediaToFile(t *testing.T, fn func(ctx context.Context, url string, maxBytes int64) (string, error)) {
	t.Helper()
	orig := fetchMediaToFile
	fetchMediaToFile = fn
	t.Cleanup(func() { fetchMediaToFile = orig })
}

// stubGoClose 把播放器非同步關閉改為同步執行（Close 計數斷言的確定性；
// 正式路徑＝go f()——Media Foundation 收斂移出 UI 執行緒，P4）。
func stubGoClose(t *testing.T) {
	t.Helper()
	orig := goClose
	goClose = func(f func()) { f() }
	t.Cleanup(func() { goClose = orig })
}

// tinyGIF 造一枚 2×2 單格合成 GIF（NewGIFDecoder 的合法輸入；夾具一律
// 合成，非真 CDN 內容）。
func tinyGIF(t *testing.T) []byte {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.RGBA{R: 255, A: 255}})
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("合成 GIF: %v", err)
	}
	return buf.Bytes()
}

// tempMediaFile 造一枚真實暫存檔（fetchMediaToFile 替身的回傳值——
// openPlayer seam 收到路徑；測試後清理）。
func tempMediaFile(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "x-media-test-*.mp4")
	if err != nil {
		t.Fatalf("暫存檔: %v", err)
	}
	path := f.Name()
	_ = f.Close()
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
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

// detailVideoPost 造一則 video 媒體貼文：URL 為最高位元率 mp4（D24：懶
// 建立——開帖不建播放器，點播放才下載暫存檔建構；合成測試值非真影片）。
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

// detailGifPost 造一則 gif 媒體貼文：D24——gif 走純 Go GIF 解碼（URL 為
// GIF 本體，非 mp4；合成測試值非真內容）。
func detailGifPost() post {
	return post{
		WorkID:     "g1",
		WorkURL:    "https://x.com/u/status/g1",
		ScreenName: "u",
		Title:      "動圖帖g1",
		Media: []api.MediaItem{{
			Kind:     "gif",
			ThumbURL: "https://pbs.twimg.test/media/Fg1?format=jpg&name=small",
			URL:      "https://video.twimg.test/Fg1.gif",
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

// TestOpenDetailClosesPreviousPlayer（§8.6 單一活躍播放器；D24 gif 路徑）：
// 連開兩則 gif 帖（自動下載＋解碼播放）→ 第一個 player 的 Close 被呼叫
// 恰一次、第二個成為活躍（a.player）。
func TestOpenDetailClosesPreviousPlayer(t *testing.T) {
	stubGoClose(t)
	stubFetchMediaBytes(t, func(context.Context, string, int64) ([]byte, error) {
		return tinyGIF(t), nil
	})
	var players []*fakePlayer
	stubNewGifPlayer(t, func([]byte) (player, error) {
		p := &fakePlayer{}
		players = append(players, p)
		return p, nil
	})
	a, _ := detailFixture(t, &fakeSession{})
	a.openDetail(detailGifPost())
	a.openDetail(detailGifPost())
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
	stubGoClose(t)
	stubFetchMediaBytes(t, func(context.Context, string, int64) ([]byte, error) {
		return tinyGIF(t), nil
	})
	var players []*fakePlayer
	stubNewGifPlayer(t, func([]byte) (player, error) {
		p := &fakePlayer{}
		players = append(players, p)
		return p, nil
	})
	a, tt := detailFixture(t, &fakeSession{})
	a.openDetail(detailGifPost())
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
	a.openDetail(detailGifPost())
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

// TestDetailShowsLargeImageAndVideo（Kind 分派，§8.6＋D24）：photo 帖 →
// fetchImage 以 Media[0].URL（name=large）呼叫、解碼結果入展開狀態（不
// 走縮圖 LRU）；video 帖 → 懶建立（開帖不建播放器；點「播放」→
// fetchMediaToFile 下載暫存檔 → openPlayer 以暫存檔路徑建構）；gif 帖 →
// fetchMediaBytes＋newGifPlayer（D24 純 Go 解碼、自動播放）。
func TestDetailShowsLargeImageAndVideo(t *testing.T) {
	photo, videoPost, gifPost := detailPhotoPost(), detailVideoPost(), detailGifPost()
	gotImage, gotPlayerPath := "", ""
	stubFetchImage(t, func(_ context.Context, url string) ([]byte, error) {
		gotImage = url
		return testPNG(t), nil
	})
	stubFetchMediaToFile(t, func(_ context.Context, url string, _ int64) (string, error) {
		gotPlayerPath = tempMediaFile(t)
		return gotPlayerPath, nil
	})
	var players []*fakePlayer
	stubOpenPlayer(t, func(path string) (player, error) {
		p := &fakePlayer{}
		players = append(players, p)
		return p, nil
	})
	stubFetchMediaBytes(t, func(context.Context, string, int64) ([]byte, error) {
		return tinyGIF(t), nil
	})
	stubNewGifPlayer(t, func([]byte) (player, error) {
		p := &fakePlayer{}
		players = append(players, p)
		return p, nil
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
	// video 帖：懶建立（D24）——開帖不建播放器、呈播放鈕；點擊後以下載
	// 的暫存檔路徑建構。
	a.openDetail(videoPost)
	if a.player != nil || len(players) != 0 {
		t.Fatalf("video 帖開帖不得建播放器（懶建立，D24），player=%v players=%d", a.player, len(players))
	}
	tt.Frame()
	if !tt.HasText(videoPlayText) {
		t.Fatalf("video 帖應呈播放鈕（懶建立），texts %q", tt.Texts())
	}
	if err := tt.Click(videoPlayText); err != nil {
		t.Fatalf("點播放鈕: %v", err)
	}
	if a.player == nil {
		t.Fatal("點播放後應建立單一活躍播放器")
	}
	if gotPlayerPath == "" || a.playerPath != gotPlayerPath {
		t.Errorf("openPlayer 應以暫存檔路徑建構且 playerPath 留存，got %q playerPath %q", gotPlayerPath, a.playerPath)
	}
	// gif 帖（D24）：fetchMediaBytes＋newGifPlayer——自動播放（開帖即建）。
	before := len(players)
	a.openDetail(gifPost)
	if a.player == nil {
		t.Fatal("gif 帖應自動建立單一活躍播放器（D24）")
	}
	if len(players) != before+1 {
		t.Errorf("gif 帖應經 newGifPlayer 建構播放器，players=%d", len(players))
	}
}

// TestMediaFailureShowsExternalLink（媒體失敗 UX，§8.6／Alternative 11＋
// D24）：video 懶建立——點「播放」後下載失敗（fetchMediaToFile 回錯）→
// 錯誤文案＋WorkURL 外開連結呈於展開檢視；點擊連結以外開 WorkURL。媒體
// 失敗不落引擎 kind 文案。
func TestMediaFailureShowsExternalLink(t *testing.T) {
	stubFetchMediaToFile(t, func(context.Context, string, int64) (string, error) {
		return "", errors.New("video: the system has no media framework that plays video")
	})
	p := detailVideoPost()
	a, tt := detailFixture(t, &fakeSession{})
	tt.Frame()
	a.openDetail(p)
	tt.Frame()
	if err := tt.Click(videoPlayText); err != nil {
		t.Fatalf("點播放鈕: %v", err)
	}
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

// TestSwitchTombstoneClosesDetail（§8.6 切換帳號）：展開檢視開啟（gif 播
// 放器活躍，D24）時切換帳號 → 步驟 8 墓碑 closeDetail：播放器 Close 恰
// 一次、detail／player 歸零——殘留播放器／detail 跨帳號即洩漏。
func TestSwitchTombstoneClosesDetail(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("7601"), accountKey("7602"))
	seedSwitchAccounts(t, "7601", "7602")
	stubGoClose(t)
	stubFetchMediaBytes(t, func(context.Context, string, int64) ([]byte, error) {
		return tinyGIF(t), nil
	})
	var players []*fakePlayer
	stubNewGifPlayer(t, func([]byte) (player, error) {
		p := &fakePlayer{}
		players = append(players, p)
		return p, nil
	})
	a := newTestApp()
	a.sess = &fakeSession{userID: "7601"}
	a.openDetail(detailGifPost())
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
// 點列主體開啟展開檢視（photo 帖派發大圖下載）；點列內的 WorkURL Link 不
// 開帖——Link 自行收 clicks，外開 URL。
func TestRowClickOpensDetailNotLink(t *testing.T) {
	gotImage := ""
	stubFetchImage(t, func(_ context.Context, url string) ([]byte, error) {
		gotImage = url
		return testPNG(t), nil
	})
	p := detailPhotoPost()
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
	if gotImage != p.Media[0].URL {
		t.Errorf("photo 帖應派發大圖下載，got %q", gotImage)
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

// TestNarrowBandRowClickOpensDetail（final review F2 釘測，D24 契約）：
// 窄帶（900px，band 1）點媒體列同樣開啟展開檢視——以全寬 detail 頁承接
// （§8.8 的 Router 頁形態不實作，控制器裁決 2026-11-15）：gif 帖自動建
// 播放器（newGifPlayer）、feed 標示（含側欄）不可達、帖文渲染；收合按鈕
// （detailView 內）closeDetail 返回 feed。
func TestNarrowBandRowClickOpensDetail(t *testing.T) {
	stubGoClose(t)
	stubFetchMediaBytes(t, func(context.Context, string, int64) ([]byte, error) {
		return tinyGIF(t), nil
	})
	var players []*fakePlayer
	stubNewGifPlayer(t, func([]byte) (player, error) {
		p := &fakePlayer{}
		players = append(players, p)
		return p, nil
	})
	p := detailGifPost()
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
	if len(players) != 1 || a.player == nil {
		t.Fatalf("gif 帖應自動建立活躍播放器（D24），players=%d player=%v", len(players), a.player)
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

// TestNarrowResizeRendersDetailFullWidth（final review F2 釘測，D24 契
// 約）：≥1024 開帖（gif 播放器活躍）後縮窗至 <1024 → 展開檢視以全寬
// detail 頁延續（非收合）：播放器不被關閉（生命週期隨 detail，§8.6）、
// 收合按鈕仍在；收合按鈕路徑 closeDetail 歸零 detail／player 並返回 feed。
func TestNarrowResizeRendersDetailFullWidth(t *testing.T) {
	stubGoClose(t)
	stubFetchMediaBytes(t, func(context.Context, string, int64) ([]byte, error) {
		return tinyGIF(t), nil
	})
	var players []*fakePlayer
	stubNewGifPlayer(t, func([]byte) (player, error) {
		p := &fakePlayer{}
		players = append(players, p)
		return p, nil
	})
	a := newTestApp()
	a.sess = &fakeSession{}
	tt := ui.NewTester(a.view, 1280, 720)
	a.openDetail(detailGifPost())
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
