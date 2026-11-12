// app_state：app 層生命週期——階段推進（proxy → credential）與收斂式
// 關閉（cancel → 等收斂 → flush → 關儲存層）。探測通過 ⇒ 無 material
// 階段（D6：代理於啟動解析、fail-closed，不存在執行期素材切換）。
// MyGO quit handler（0.9.0：OS 結束會話時 OnBeforeQuit 傾聽者須同步完成）
// 註冊同一 flush：Flush 即得以回呼參數注入的單元，main 側日後以
// mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { state.Flush(auth) }) 註冊，
// 本任務不觸碰 mygo 套件接線。
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	xapi "github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"
)

// Stage 是 app 生命週期階段。
type Stage string

const (
	// StageProxy 是初始階段：代理解析／預檢完成前。
	StageProxy Stage = "proxy"
	// StageCredential 是代理就緒後的階段：憑證（cookie）就緒前。
	StageCredential Stage = "credential"
)

// flushTimeout 是 Close／quit flush 的落盤逾時：系統憑證庫呼叫不受
// ctx 取消控制，以短逾時兜底，避免收斂關閉被單一儲存層卡死。
const flushTimeout = 5 * time.Second

// AppState 是 app 生命週期狀態：階段、當前錯誤與就緒旗標；並持有引擎
// 背景工作的 context 與收斂用 WaitGroup，供 Close 依序收斂。每段錯誤態
// 獨立：Err 非零時 Advance 不前進，呼叫端處置後自行清除再推進。
type AppState struct {
	// Stage 是當前階段（StageProxy → StageCredential）。
	Stage Stage
	// Err 是當前階段的錯誤態；非 nil 時階段不前進。
	Err error
	// Ready 表示全階段完成（credential 就緒）。
	Ready bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	store  xapi.CookieStore
}

// NewAppState 建構生命週期狀態：初始 StageProxy、未 Ready，並建立引擎
// 背景工作共用的 context。store 可為 nil（無儲存層時 Close 略過 flush）。
func NewAppState(store xapi.CookieStore) *AppState {
	ctx, cancel := context.WithCancel(context.Background())
	return &AppState{
		Stage:  StageProxy,
		ctx:    ctx,
		cancel: cancel,
		store:  store,
	}
}

// Context 回引擎背景工作共用的 context：Close 時被取消，工作應監聽
// Done 並儘快收斂。
func (s *AppState) Context() context.Context { return s.ctx }

// Go 以生命週期 context 跑一段背景工作並計入收斂：Close 等其結束後才
// 進行 flush，未收斂的寫入不會與落盤競爭。
func (s *AppState) Go(fn func(ctx context.Context)) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		fn(s.ctx)
	}()
}

// Advance 推進階段：StageProxy → StageCredential → Ready。Err 非零時
// 不前進（帶錯誤的階段由呼叫端處置；每段錯誤態獨立，不自動跳過）。
// Ready 後再 Advance 為無操作。
func (s *AppState) Advance() {
	if s.Err != nil {
		return
	}
	switch s.Stage {
	case StageProxy:
		s.Stage = StageCredential
	case StageCredential:
		s.Ready = true
	}
}

// setErr 記錄首個生命週期錯誤（不覆蓋既有錯誤態）。
func (s *AppState) setErr(err error) {
	if s.Err == nil {
		s.Err = err
	}
}

// Flush 將 Auth 當前 cookie 行與儲存層比對，僅於有未落盤變更時 Save
// （Sender 的 write-through 已落盤者不重寫）。此即 MyGO quit handler
// 註冊的同一 flush：以回呼參數注入（見檔頭）。行為：
//
//   - 無儲存層、無 Auth 或當前行為空（未 Prepare）→ 不做事；
//   - 儲存層讀回失敗視同「有未落盤變更」，仍嘗試 Save（冪等寫）；
//   - Save 失敗記入 Err（訊息僅含底層儲存層錯誤，不含 cookie 內容，
//     REQ-INV-01）。
//
// 於 Close 內呼叫時 ctx 已取消，flush 另以 WithoutCancel 短逾時 context
// 進行。
func (s *AppState) Flush(a *xapi.Auth) {
	if s.store == nil || a == nil {
		return
	}
	line := a.CookieHeader()
	if line == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), flushTimeout)
	defer cancel()
	stored, err := s.store.Load(ctx)
	if err == nil && stored == line {
		return // 已落盤
	}
	if err := s.store.Save(ctx, line); err != nil {
		s.setErr(fmt.Errorf("生命週期 flush 失敗: %w", err))
	}
}

// Close 依序收斂：cancel → 等背景工作收斂 → flush（CookieStore.Save 若有
// 未落盤變更）→ 關儲存層（store 實作 Close 時；SQLite 連線關閉即此步，
// 系統憑證庫無可關資源則自動略過）。close 錯誤同樣記入 Err（不覆蓋）。
func (s *AppState) Close(a *xapi.Auth) {
	s.cancel()
	s.wg.Wait()
	s.Flush(a)
	if c, ok := s.store.(interface{ Close() error }); ok {
		if err := c.Close(); err != nil {
			s.setErr(fmt.Errorf("關閉儲存層失敗: %w", err))
		}
	}
}
