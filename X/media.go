// media：列表縮圖管線（v1.1 設計 §8.2；Task 12）。三段：
//
//  1. fetchImage 接縫——媒體 CDN（pbs.twimg.com 等）直載的獨立輕量
//     http.Client，不經引擎 transport（D15/D22：媒體 CDN ≠ GraphQL API
//     單一出口，REQ-TRN-010 偏離已登錄）；timeout 10s、單檔 ≤8 MiB、
//     Content-Type 必須 image/*。錯誤文案不涉憑證（REQ-INV-01）。
//  2. thumbStore——解碼位元組 LRU（≤24 MiB，D14：量測後可調的常數），
//     這是 <80 MiB 行程預算的防線；所有讀寫都在主執行緒段（apply 回呼
//     內／view 建構）。Bitmap.ReleasePixels() 本任務不呼叫（D14：量測後
//     補強開關，T15 決定）。
//  3. app 側派發——in-flight 去重（URL 鍵、mutex 守護）＋跨 feed 全域
//     信號量（≤4）於 goroutine 側；完成經 publish 回主執行緒：世代＋槽
//     雙檢查（T6 形）→ ui.DecodeImage（主執行緒——保守：Bitmap 建構的
//     執行緒約束未經查證，主執行緒是安全面）→ put → 重繪。失敗 → 佔位
//     改「可重試」按鈕，點擊重試（不自動重試）。
//
// 管線狀態（thumbLRU／thumbInflight／thumbFailed／thumbSem）為套件級：
// 語義本就跨 feed 全域（信號量橫跨三槽），且本任務檔案範圍不含 app 結
// 構所在的 main.go；各測試以 thumbPipelineCleanup 隔離復位。
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// thumbConcurrency 是縮圖下載的跨 feed 全域併發上限（信號量，goroutine 側）。
const thumbConcurrency = 4

// thumbLRUBytes 是 thumbStore 的解碼位元組上限：24 MiB（D14：量測後可調
// 的常數——<80 MiB 行程預算的防線）。
const thumbLRUBytes = 24 << 20

// thumbMaxBytes 是單一媒體檔的下載上限：8 MiB（超限即錯誤，不讀入）。
const thumbMaxBytes = 8 << 20

// thumbFetchTimeout 是單一縮圖下載的逾時（brief 規定 10 秒）。
const thumbFetchTimeout = 10 * time.Second

// thumbLoadingText／thumbRetryText 是縮圖佔位文案（契約：未命中 → 載入
// 中；失敗 → 可重試，點擊才再派發，不自動重試）。
const (
	thumbLoadingText = "載入中…"
	thumbRetryText   = "縮圖載入失敗，點擊重試"
)

// thumbClient 是縮圖下載的獨立輕量客戶端（D15：不經引擎 transport；
// Client.Timeout 涵蓋建連到讀畢本體的全程）。
var thumbClient = &http.Client{Timeout: thumbFetchTimeout}

// fetchImage 是媒體下載接縫：測試注入替身；正式路徑＝獨立輕量
// http.Client 直載公開 CDN（D15/D22——REQ-TRN-010 偏離已登錄：媒體 CDN
// ≠ GraphQL API 單一出口）。守門：HTTP 200、Content-Type 必須 image/*
// （否則錯誤）、單檔 ≤8 MiB（超限即錯誤，不整讀入）。錯誤文案僅含下載
// 事實，不含任何憑證材料（REQ-INV-01）。
var fetchImage = func(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("縮圖下載失敗：請求建構無效: %w", err)
	}
	resp, err := thumbClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("縮圖下載失敗: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("縮圖下載失敗：HTTP %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("縮圖下載失敗：非圖片內容類型 %q", ct)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, thumbMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("縮圖下載失敗: %w", err)
	}
	if len(data) > thumbMaxBytes {
		return nil, fmt.Errorf("縮圖下載失敗：檔案超過 %d MiB 上限", thumbMaxBytes>>20)
	}
	return data, nil
}

// thumbEntry 是快取的一枚縮圖：src 為 DecodeImage 產物，bytes 為解碼後
// 估算（w*h*4，put 依 ImageSize 計算；測試經 putSized 顯式注入）。
type thumbEntry struct {
	src   ui.ImageSource // ui.DecodeImage 產物
	bytes int            // 解碼後估算（w*h*4）
}

// thumbStore 是解碼位元組 LRU：key = ThumbURL；order 為 LRU 序（get 觸
// 碰即移尾）；bytes 為現存總解碼位元組。所有讀寫都在主執行緒段（apply
// 回呼內／view 建構）；mu 為契約欄位（防未來跨執行緒誤用的守護）。
type thumbStore struct {
	mu      sync.Mutex
	entries map[string]*thumbEntry // key = ThumbURL
	order   []string               // LRU 序（get 觸碰即移尾）
	bytes   int
}

// newThumbStore 建構空儲存。
func newThumbStore() *thumbStore {
	return &thumbStore{entries: make(map[string]*thumbEntry)}
}

// get 回快取的縮圖來源：命中即觸碰移尾（LRU 序刷新）。主執行緒呼叫。
func (t *thumbStore) get(url string) (ui.ImageSource, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.entries[url]
	if !ok {
		return nil, false
	}
	t.touchLocked(url)
	return e.src, true
}

// put 快取一枚解碼結果：位元組依 ImageSize 估算（w*h*4），超限自最舊
// 逐出。主執行緒呼叫。
func (t *thumbStore) put(url string, src ui.ImageSource) {
	w, h := ui.ImageSize(src)
	t.putSized(url, src, int(w)*int(h)*4)
}

// putSized 以顯式位元組數插入——put 的位元組注入縫（正式路徑由 put 依
// ImageSize 估算；測試注入假大小驅動逐出，不真解碼大圖）。同 key 重插
// 即換新（舊位元組扣除、序移尾）。超限自最舊逐出；僅餘一筆（即新插者）
// 時不逐出——單筆超限亦保留，避免逐出後同 URL 立即重取的迴圈（單筆上
// 界由 8 MiB 下載上限兜底；D14 量測後可調）。
func (t *thumbStore) putSized(url string, src ui.ImageSource, nbytes int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if e, ok := t.entries[url]; ok {
		t.bytes -= e.bytes
		t.removeLocked(url)
	}
	t.entries[url] = &thumbEntry{src: src, bytes: nbytes}
	t.order = append(t.order, url)
	t.bytes += nbytes
	for t.bytes > thumbLRUBytes && len(t.order) > 1 {
		oldest := t.order[0]
		t.bytes -= t.entries[oldest].bytes
		delete(t.entries, oldest)
		t.removeLocked(oldest)
	}
}

// touchLocked 將 url 移至 LRU 序尾（get 命中／重插即觸碰）。呼叫端須持鎖。
func (t *thumbStore) touchLocked(url string) {
	t.removeLocked(url)
	t.order = append(t.order, url)
}

// removeLocked 自 LRU 序移除 url（存在時）。呼叫端須持鎖。
func (t *thumbStore) removeLocked(url string) {
	for i, u := range t.order {
		if u == url {
			t.order = append(t.order[:i], t.order[i+1:]...)
			return
		}
	}
}

// —— app 側派發（套件級管線狀態；理由見檔頭）——

// thumbLRU 是縮圖快取（主執行緒專屬：get/put 只在 apply 回呼內或 view
// 建構段呼叫）。
var thumbLRU = newThumbStore()

// thumbInflight 是在途下載去重表（key = ThumbURL）：thumbMu 守護——派發
// 標記與完成清除分處 view 建構段與 publish 回呼（同為主執行緒，鎖為契約
// 面的守護，防未來兩側分離後的誤用）。
var (
	thumbMu       sync.Mutex
	thumbInflight = map[string]bool{}
)

// thumbFailed 是下載／解碼失敗的 URL（主執行緒專屬）：佔位改「可重試」
// 按鈕的依據——失敗不自動重試（避免逐框風暴），點擊重試才清除再派發。
var thumbFailed = map[string]bool{}

// thumbSem 是跨 feed 全域下載信號量（容量 thumbConcurrency=4）：goroutine
// 側於下載前取得、fetchImage 回返即釋放（排隊者依派發序取得名額）。
var thumbSem = make(chan struct{}, thumbConcurrency)

// ensureThumb 確保該 URL 的縮圖在載入中（view 建構段呼叫，逐框重入冪
// 等）：已快取 → 不做事；失敗態 → 不做事（佔位呈重試按鈕，不自動重試）；
// 在途 → 不做事；否則同步標記在途並以會話子樹派發（goSession）——起跑
// 前捕捉世代與槽，完成回寫前比對（T6 形），過期結果整批丟棄。同步標記
// 在途（而非 goroutine 內）防逐框重複派發：標記到清除之間的框只會看到
// 「載入中」佔位。
func (a *app) ensureThumb(kind int, f *timelineState, url string) {
	if url == "" {
		return
	}
	if _, ok := thumbLRU.get(url); ok {
		return // 已快取
	}
	if thumbFailed[url] {
		return // 失敗態：不自動重試（重試入口 retryThumb）
	}
	thumbMu.Lock()
	busy := thumbInflight[url]
	if !busy {
		thumbInflight[url] = true
	}
	thumbMu.Unlock()
	if busy {
		return // 在途：等完成回寫
	}
	gen := a.gen
	a.goSession(func(ctx context.Context) {
		// 信號量：跨 feed 全域併發上限（≤4）；會話斬斷（切換／Close）
		// 即放棄排隊，以取消錯誤走失敗回寫（清除在途標記後丟棄）。
		select {
		case thumbSem <- struct{}{}:
		case <-ctx.Done():
			a.finishThumb(url, nil, ctx.Err(), kind, f, gen)
			return
		}
		data, err := fetchImage(ctx, url)
		<-thumbSem // 釋放名額（下載既畢；發佈不佔下載名額）
		a.finishThumb(url, data, err, kind, f, gen)
	})
}

// finishThumb 完成回寫（經 publish 於主執行緒套用）：先清除在途標記
// （即使世代／槽檢查不過也必清——過期結果丟棄，該 URL 可再派發）；
// ui.DecodeImage 於主執行緒執行（保守：Bitmap 建構的執行緒約束未經查
// 證，主執行緒是安全面）；下載或解碼失敗 → 記失敗態（佔位改可重試，
// 不自動重試）；成功 → put 入 LRU，次框渲染即命中。
func (a *app) finishThumb(url string, data []byte, err error, kind int, f *timelineState, gen int) {
	a.publish(func() {
		thumbMu.Lock()
		delete(thumbInflight, url)
		thumbMu.Unlock()
		// 世代與槽一致（T6 形）：gen 比對擋切換後的舊帳號結果；槽指標
		// 比對擋該槽被歸零換新後的殘餘回寫——兩者皆丟棄（不寫快取與
		// 失敗態；該 URL 由新畫面的逐框重入自然再派發）。
		if gen != a.gen || kind < 0 || kind >= len(a.feeds) || a.feeds[kind] != f {
			return
		}
		if err != nil {
			thumbFailed[url] = true
			return
		}
		src, derr := ui.DecodeImage(data)
		if derr != nil {
			thumbFailed[url] = true
			return
		}
		thumbLRU.put(url, src)
	})
}

// retryThumb 是點擊重試入口（不自動重試的對應側）：清除失敗態後即時再
// 派發（ensureThumb 的其餘守門照常）。主執行緒呼叫（點擊處理段）。
func (a *app) retryThumb(kind int, f *timelineState, url string) {
	delete(thumbFailed, url)
	a.ensureThumb(kind, f, url)
}
