package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"

	xapi "github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"
)

// TestAppState_Advance 驟階段推進：初始 StageProxy → Advance →
// StageCredential → Advance → Ready；帶 Err 的階段不前進。
func TestAppState_Advance(t *testing.T) {
	s := NewAppState(nil)
	if s.Stage != StageProxy {
		t.Fatalf("初始 Stage = %q，欲得 %q", s.Stage, StageProxy)
	}
	s.Advance()
	if s.Stage != StageCredential {
		t.Fatalf("一次 Advance 後 Stage = %q，欲得 %q", s.Stage, StageCredential)
	}
	if s.Ready {
		t.Fatal("credential 階段未再 Advance 前不應 Ready")
	}
	s.Advance()
	if !s.Ready {
		t.Fatal("第二次 Advance 後應 Ready")
	}
	if s.Stage != StageCredential {
		t.Fatalf("Ready 後 Stage 應維持 %q，得 %q", StageCredential, s.Stage)
	}

	// 帶 Err 的階段不前進。（Err 欄位已鎖內封裝：以 setErr 寫入——新狀態
	// 首錄語義與舊直接欄位寫入等價。）
	s2 := NewAppState(nil)
	s2.setErr(errors.New("boom"))
	s2.Advance()
	if s2.Stage != StageProxy {
		t.Fatalf("帶 Err 時 Advance 不應推進，Stage = %q", s2.Stage)
	}
	if s2.Ready {
		t.Fatal("帶 Err 時不應 Ready")
	}
}

// recStore 是記錄呼叫序的 CookieStore 替身，附 Close 以驗收尾步驟。
type recStore struct {
	rec   func(string)
	saved string
}

func (r *recStore) Load(ctx context.Context) (string, error) {
	r.rec("load")
	return r.saved, nil
}

func (r *recStore) Save(ctx context.Context, cookieLine string) error {
	r.rec("save")
	r.saved = cookieLine
	return nil
}

func (r *recStore) Close() error {
	r.rec("close")
	return nil
}

// TestAppState_Close_Order 驟 Close 順序：cancel → 等收斂（背景工作於
// ctx 取消後才結束）→ flush（未落盤變更 Save）→ 關儲存層。
func TestAppState_Close_Order(t *testing.T) {
	var mu sync.Mutex
	var events []string
	rec := func(e string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, e)
	}
	st := &recStore{rec: rec}
	s := NewAppState(st)
	a := xapi.NewAuth("")
	if err := a.PrepareAuth("auth_token=abc; ct0=def"); err != nil {
		t.Fatalf("PrepareAuth: %v", err)
	}
	done := make(chan struct{})
	s.Go(func(ctx context.Context) {
		defer close(done)
		<-ctx.Done() // 背景工作僅於 cancel 後收斂
		rec("worker-done")
	})

	s.Close(a)
	<-done

	mu.Lock()
	defer mu.Unlock()
	want := []string{"worker-done", "load", "save", "close"}
	if len(events) != len(want) {
		t.Fatalf("事件序 = %v，欲得 %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("事件序 = %v，欲得 %v", events, want)
		}
	}
	if st.saved == "" {
		t.Fatal("flush 應已 Save 未落盤變更")
	}
}

// TestAppState_ErrConcurrent 釘測錯誤態的跨 goroutine 同步（Review
// Important 1 修復）：背景 goroutine 併發 setErr（模擬 runSwitch／warmup
// ／persistLogin 的 goAsync 寫入），與讀取側（Err()／Advance，模擬 UI 執
// 行緒）交錯，-race 下無競爭；收斂後首錄不覆蓋——Err 恰為注入錯誤之一，
// 且帶 Err 的 Advance 不推進。競爭偵測屬機率性（寫讀交錯由排程決定、
// 不用 t.Parallel），以高頻交錯＋全套件 -race 閘常駐釘住。
func TestAppState_ErrConcurrent(t *testing.T) {
	s := NewAppState(nil)
	const writers = 8
	const writes = 500
	var wg sync.WaitGroup
	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go func(i int) {
			defer wg.Done()
			for j := 0; j < writes; j++ {
				s.setErr(fmt.Errorf("err-%d", i))
			}
		}(i)
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	reads := 0
	for {
		select {
		case <-done:
			err := s.Err()
			if err == nil {
				t.Fatal("併發 setErr 收斂後 Err 應非 nil")
			}
			if !strings.HasPrefix(err.Error(), "err-") {
				t.Fatalf("Err 應為注入錯誤之一（首錄不覆蓋），得 %v", err)
			}
			// 帶 Err 的 Advance 不推進（讀取路徑同鎖）：比對 Advance 前
			// 後的階段與就緒旗標——不預設 Stage 仍為 proxy（讀取迴圈的
			// 早期 Advance 可能先於任何 writer 落地（排程決定），此時
			// Stage 已合法推進至 credential；固定斷言 proxy 對此機率性
			// 交錯即為 flake，-count 高頻複跑實測 ~6%）。收斂後 Err 恆
			// 非零（首錄不清除），Advance 必然原地不動。
			stage, ready := s.Stage, s.Ready
			s.Advance()
			if s.Stage != stage || s.Ready != ready {
				t.Fatalf("帶 Err 時 Advance 不應推進：stage %q→%q ready=%v→%v", stage, s.Stage, ready, s.Ready)
			}
			if reads == 0 {
				t.Fatal("讀取迴圈未執行（測試失效）")
			}
			return
		default:
			_ = s.Err()
			s.Advance()
			reads++
			runtime.Gosched()
		}
	}
}
