// media 測試：thumbStore 的 LRU 行為（逐出最舊、get 觸碰移尾）、fetchImage
// 接縫正式路徑的守門（Content-Type 必須 image/*、單檔 ≤8 MiB、成功回傳
// 位元組——httptest 本地伺服器，不打真 CDN），以及 feed 列縮圖格的行為面
// （佔位「載入中」→ 載入完成呈圖、失敗 → 可重試按鈕且不自動重試、點擊
// 才再派發）。LRU 測試不真解碼大圖：thumbEntry.bytes 由 putSized 的顯式
// 位元組參數注入（put 的位元組注入縫），src 以真 DecodeImage 的 1×1 PNG
// 合成小圖擔任（測試 fixture 一律合成）。
package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/api"
)

// testPNG 以 png.Encode 合成一張 1×1 PNG（測試夾具值，非真 CDN 內容）。
func testPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatalf("合成 PNG 夾具失敗: %v", err)
	}
	return buf.Bytes()
}

// testImageSource 以 testPNG 經真 DecodeImage 得到合法 ImageSource（LRU
// 測試的 src 擔任者；大小由 putSized 注入，與此小圖的實際尺寸無關）。
func testImageSource(t *testing.T) ui.ImageSource {
	t.Helper()
	src, err := ui.DecodeImage(testPNG(t))
	if err != nil {
		t.Fatalf("DecodeImage 合成 PNG 失敗: %v", err)
	}
	return src
}

// thumbPipelineCleanup 註冊套件級縮圖管線狀態的還原（測試間隔離）：
// thumbLRU 換新、in-flight 與失敗態清空。
func thumbPipelineCleanup(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		thumbLRU = newThumbStore()
		thumbMu.Lock()
		thumbInflight = map[string]bool{}
		thumbMu.Unlock()
		thumbFailed = map[string]bool{}
	})
}

// stubFetchImage 以替身替換 fetchImage 接縫，測試後還原（接縫注入慣例：
// call-time 解析＋t.Cleanup 還原）。
func stubFetchImage(t *testing.T, fn func(ctx context.Context, url string) ([]byte, error)) {
	t.Helper()
	orig := fetchImage
	fetchImage = fn
	t.Cleanup(func() { fetchImage = orig })
}

// mediaPost 造一則含單一 photo 媒體的貼文（ThumbURL 為合成測試 URL）。
func mediaPost(id, thumbURL string) post {
	return post{
		WorkID:     id,
		WorkURL:    "https://x.com/u/status/" + id,
		ScreenName: "u",
		Title:      "貼文" + id,
		Media:      []api.MediaItem{{Kind: "photo", ThumbURL: thumbURL, URL: thumbURL, Width: 100, Height: 100}},
	}
}

// TestThumbLRUEvictsOldest：put 超限（每筆注入 10 MiB 假位元組，三筆
// 30 MiB > 24 MiB 上限）→ 最舊 URL 消失、其餘保留、bytes 回落 ≤ 上限。
func TestThumbLRUEvictsOldest(t *testing.T) {
	ts := newThumbStore()
	src := testImageSource(t)
	ts.putSized("u1", src, 10<<20)
	ts.putSized("u2", src, 10<<20)
	ts.putSized("u3", src, 10<<20) // 30 MiB > 24 MiB → 自最舊逐出 u1
	if _, ok := ts.get("u1"); ok {
		t.Errorf("最舊的 u1 應已逐出")
	}
	if _, ok := ts.get("u2"); !ok {
		t.Errorf("u2 應保留（逐出後 20 MiB ≤ 上限）")
	}
	if _, ok := ts.get("u3"); !ok {
		t.Errorf("最新的 u3 應保留")
	}
	if ts.bytes > thumbLRUBytes {
		t.Errorf("逐出後 bytes = %d，應回落 ≤ 上限 %d", ts.bytes, thumbLRUBytes)
	}
	if ts.bytes != 20<<20 {
		t.Errorf("逐出後 bytes = %d, want %d（u2+u3）", ts.bytes, 20<<20)
	}
}

// TestThumbGetTouchRefreshesOrder：get 命中即移尾 → 逐出順序改變——u1
// 觸碰後，後續超限 put 逐出的是 u2（未觸碰的最舊者）而非 u1。
func TestThumbGetTouchRefreshesOrder(t *testing.T) {
	ts := newThumbStore()
	src := testImageSource(t)
	ts.putSized("u1", src, 10<<20)
	ts.putSized("u2", src, 10<<20)
	if _, ok := ts.get("u1"); !ok { // 觸碰：u1 移尾
		t.Fatalf("u1 應命中")
	}
	ts.putSized("u3", src, 10<<20) // 30 MiB > 24 MiB → 逐出未觸碰的最舊者 u2
	if _, ok := ts.get("u2"); ok {
		t.Errorf("未觸碰的 u2 應被逐出（get 觸碰應刷新 u1 的逐出順序）")
	}
	if _, ok := ts.get("u1"); !ok {
		t.Errorf("觸碰過的 u1 應保留")
	}
	if _, ok := ts.get("u3"); !ok {
		t.Errorf("最新的 u3 應保留")
	}
}

// TestFetchImageRejectsNonImage：Content-Type 非 image/*（text/html）→
// 錯誤（httptest 本地，不打真 CDN）。
func TestFetchImageRejectsNonImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html>not an image</html>"))
	}))
	t.Cleanup(srv.Close)
	if data, err := fetchImage(context.Background(), srv.URL); err == nil {
		t.Errorf("非圖片內容類型應回錯誤，得 data 長度 %d", len(data))
	}
}

// TestFetchImageRejectsOversize：回應超過單檔上限（8 MiB）→ 錯誤。
func TestFetchImageRejectsOversize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, thumbMaxBytes+1))
	}))
	t.Cleanup(srv.Close)
	if _, err := fetchImage(context.Background(), srv.URL); err == nil {
		t.Errorf("超過單檔上限（%d MiB）應回錯誤", thumbMaxBytes>>20)
	}
}

// TestFetchImageOK：image/png 回應 → 位元組原樣回傳、無錯誤。
func TestFetchImageOK(t *testing.T) {
	want := testPNG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(want)
	}))
	t.Cleanup(srv.Close)
	data, err := fetchImage(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("image/png 應成功下載，得錯誤: %v", err)
	}
	if !bytes.Equal(data, want) {
		t.Errorf("下載位元組應與夾具一致，得長度 %d, want %d", len(data), len(want))
	}
}

// TestHomeView_ThumbPlaceholderThenLoaded：含媒體貼文首框呈「載入中」
// 佔位並派發恰一次下載；完成回寫（主執行緒 DecodeImage → put）後次框
// 呈圖、佔位文字消失、結果入 thumbStore。不含媒體的貼文不渲染縮圖格。
func TestHomeView_ThumbPlaceholderThenLoaded(t *testing.T) {
	thumbPipelineCleanup(t)
	const thumbURL = "https://pbs.twimg.test/media/Fabc1?format=jpg&name=small"
	calls := 0
	stubFetchImage(t, func(context.Context, string) ([]byte, error) {
		calls++
		return testPNG(t), nil
	})
	plain := mkPosts("a")[0]
	fake := &fakeSession{home: pageQueue(pageOf([]post{plain, mediaPost("m1", thumbURL)}, ""))}
	_, tt := homeFixture(t, fake)
	if !tt.HasText(thumbLoadingText) {
		t.Fatalf("首框應呈載入中佔位，texts %q", tt.Texts())
	}
	if calls != 1 {
		t.Fatalf("首框應派發恰一次下載，calls = %d", calls)
	}
	tt.Frame() // 完成回寫（DecodeImage → put）後重繪
	if tt.HasText(thumbLoadingText) {
		t.Errorf("載入完成後不應再顯示佔位文字，texts %q", tt.Texts())
	}
	if _, ok := thumbLRU.get(thumbURL); !ok {
		t.Errorf("解碼結果應入 thumbStore（key = ThumbURL）")
	}
}

// TestHomeView_ThumbFailRetryOnlyOnClick：下載失敗 → 佔位改「可重試」
// 按鈕；其後多框不得自動重試（calls 不增）；點擊重試 → 再派發恰一次，
// 成功後呈圖、重試按鈕消失。
func TestHomeView_ThumbFailRetryOnlyOnClick(t *testing.T) {
	thumbPipelineCleanup(t)
	const thumbURL = "https://pbs.twimg.test/media/Fabc1?format=jpg&name=small"
	calls := 0
	stubFetchImage(t, func(context.Context, string) ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("模擬下載失敗")
		}
		return testPNG(t), nil
	})
	fake := &fakeSession{home: pageQueue(pageOf([]post{mediaPost("m1", thumbURL)}, ""))}
	a, tt := homeFixture(t, fake)
	a.gen = 1  // 非零世代：其後的點擊重試派發捕捉 gen=1，回寫比對一致（T6 形）
	tt.Frame() // 失敗回寫套用
	if !tt.HasText(thumbRetryText) {
		t.Fatalf("失敗後應呈可重試按鈕（%q），texts %q", thumbRetryText, tt.Texts())
	}
	tt.Frame()
	tt.Frame() // 推進多框：不得自動重試
	if calls != 1 {
		t.Errorf("失敗後不得自動重試，calls = %d, want 1", calls)
	}
	if err := tt.Click(thumbRetryText); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("點擊重試應再派發恰一次，calls = %d, want 2", calls)
	}
	tt.Frame() // 重試成功回寫後重繪
	if tt.HasText(thumbRetryText) {
		t.Errorf("重試成功後不應再顯示重試按鈕，texts %q", tt.Texts())
	}
	if _, ok := thumbLRU.get(thumbURL); !ok {
		t.Errorf("重試成功的解碼結果應入 thumbStore")
	}
}
