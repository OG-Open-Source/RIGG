package main

import (
	"context"
	"errors"
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

	// 帶 Err 的階段不前進。
	s2 := NewAppState(nil)
	s2.Err = errors.New("boom")
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
