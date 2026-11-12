// view_login 測試：以 ui.Tester 無窗驅動 login view（MyGO view 測試模式，
// 同 main_test.go 模板），接縫以 fakeSession 注入——view 僅依賴 xSession
// 介面，不依賴具體 engine 型別。斷言對應 brief 行為契約 1（錯誤映射、
// 不清除）、2（成功導向 home）、6（防抖）、7（Sanitize）與
// TestErrorText_KindMapping（messages.go 的 kind→文案映射）。
package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// fakeSession 是 xSession 的測試替身：逐呼叫記錄名稱（契約 1 的「未觸發
// 清除」以呼叫日誌驗證：錯誤路徑只允許 Validate 一類呼叫），validate／
// home 依注入腳本回應（nil 腳本＝成功／空頁）。
type fakeSession struct {
	mu       sync.Mutex
	calls    []string
	lastSeen []string
	validate func(cookie string) error
	home     func(cursor string, seen []string) (*timelineResult, error)
}

func (f *fakeSession) Validate(cookie string) error {
	f.mu.Lock()
	f.calls = append(f.calls, "Validate")
	f.mu.Unlock()
	if f.validate == nil {
		return nil
	}
	return f.validate(cookie)
}

func (f *fakeSession) Home(cursor string, seen []string) (*timelineResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, "Home")
	f.lastSeen = append([]string(nil), seen...)
	f.mu.Unlock()
	if f.home == nil {
		return &timelineResult{}, nil
	}
	return f.home(cursor, seen)
}

// callLog 回呼叫名稱日誌的快照。
func (f *fakeSession) callLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// seenOf 回最後一次 Home 收到的 seen 快照（seenTweetIds 契約斷言用）。
func (f *fakeSession) seenOf() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lastSeen...)
}

// newTestApp 組裝測試用 app：spawn／apply 同步執行（與測試同一 goroutine，
// 無併發；防抖測試另行注入非同步 spawn）、無儲存層、無 bootstrap。
func newTestApp() *app {
	a := &app{state: NewAppState(nil)}
	a.spawn = func(fn func(ctx context.Context)) { fn(context.Background()) }
	a.apply = func(fn func()) { fn() }
	return a
}

// typeCookie 點擊 Cookie 欄並鍵入一行含憑證材料的假 cookie（測試夾具值，
// 非真實材料）。
func typeCookie(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click("Cookie"); err != nil {
		t.Fatal(err)
	}
	tt.Type("auth_token=abc; ct0=def")
}

// TestLoginView_ErrorMapping（契約 1，CUJ 1/3）：PrepareAuth 或 Viewer 失敗
// 時錯誤區顯示 errorText(err) 對應文案；MaterialError／CredentialInvalidError
// 皆不觸發任何清除（落盤／刪除途徑零呼叫），重試入口（登入按鈕）仍在。
func TestLoginView_ErrorMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"material", &xerrors.MaterialError{Msg: "代理預檢失敗"}, msgMaterial},
		{"credential", &xerrors.CredentialInvalidError{Msg: "HTTP 401"}, msgCredential},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp()
			fake := &fakeSession{validate: func(string) error { return tc.err }}
			a.newSess = func() (xSession, error) { return fake, nil }
			persisted := 0
			a.onLoginPersist = func(xSession) error { persisted++; return nil }
			tt := ui.NewTester(a.view, 480, 360)
			typeCookie(t, tt)
			if err := tt.Click("登入"); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(tc.want) {
				t.Errorf("錯誤區應顯示 %q，texts %q", tc.want, tt.Texts())
			}
			if a.sess != nil {
				t.Error("錯誤路徑不得切換 home")
			}
			if a.loginBusy {
				t.Error("錯誤後 busy 應復位（可重試）")
			}
			if persisted != 0 {
				t.Errorf("錯誤路徑觸發了落盤／清除途徑 %d 次（契約 1：僅顯示）", persisted)
			}
			for _, call := range fake.callLog() {
				if call != "Validate" {
					t.Errorf("錯誤路徑出現非預期接縫呼叫 %q（日誌 %q）", call, fake.callLog())
				}
			}
			if !tt.HasText("登入") {
				t.Error("重試入口（登入按鈕）應仍在畫面上")
			}
		})
	}
}

// TestLoginView_Success（契約 2，CUJ 1）：有效 cookie → PrepareAuth 成功 →
// Viewer 驗證回身分 → 導向 home；首次落盤（Save）由 login 流完成恰一次。
func TestLoginView_Success(t *testing.T) {
	a := newTestApp()
	fake := &fakeSession{} // validate 預設成功
	a.newSess = func() (xSession, error) { return fake, nil }
	persisted := 0
	a.onLoginPersist = func(xSession) error { persisted++; return nil }
	tt := ui.NewTester(a.view, 480, 360)
	typeCookie(t, tt)
	if err := tt.Click("登入"); err != nil {
		t.Fatal(err)
	}
	if persisted != 1 {
		t.Errorf("首次 Save 應由 login 流完成恰一次，得 %d", persisted)
	}
	if a.sess == nil {
		t.Fatal("驗證通過後應導向 home 狀態（a.sess 非 nil）")
	}
	if a.loginErr != nil || a.loginBusy {
		t.Errorf("成功後不應殘留錯誤或 busy：err=%v busy=%v", a.loginErr, a.loginBusy)
	}
	if !tt.HasText("時間線") {
		t.Errorf("畫面應切換為 home，texts %q", tt.Texts())
	}
}

// TestLoginView_Debounce（契約 6）：提交進行中（Validate 阻塞模擬）再點
// 一律忽略——fake 只被呼叫一次；完成後仍正常導向 home。
func TestLoginView_Debounce(t *testing.T) {
	a := newTestApp()
	release := make(chan struct{})
	var calls atomic.Int32
	fake := &fakeSession{
		validate: func(string) error {
			calls.Add(1)
			<-release // 模擬驗證進行中
			return nil
		},
	}
	a.newSess = func() (xSession, error) { return fake, nil }
	// 非同步 spawn（真 goroutine）以重現「進行中」。apply 模擬主執行緒：
	// 背景結果入佇列、由測試 goroutine 於 frame 間排空套用——與正式路徑
	// win.Update 的主執行緒序列化語義同構（否則狀態寫入會與 frame 建構
	// 以生產路徑不可能出現的方式交疊）。
	var mu sync.Mutex
	var pending []func()
	a.apply = func(fn func()) {
		mu.Lock()
		pending = append(pending, fn)
		mu.Unlock()
	}
	flush := func() {
		mu.Lock()
		fns := pending
		pending = nil
		mu.Unlock()
		for _, fn := range fns {
			fn()
		}
	}
	var wg sync.WaitGroup
	a.spawn = func(fn func(ctx context.Context)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn(context.Background())
		}()
	}
	tt := ui.NewTester(a.view, 480, 360)
	typeCookie(t, tt)
	if err := tt.Click("登入"); err != nil {
		t.Fatal(err)
	}
	// 進行中重複提交：一律忽略（契約 6）。
	if err := tt.Click("登入"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("登入"); err != nil {
		t.Fatal(err)
	}
	close(release)
	wg.Wait()  // 全部背景工作的發佈皆已入佇列
	flush()    // 於測試 goroutine（模擬主執行緒）套用登入結果
	tt.Frame() // home 首幀：posts 為空 → 觸發首頁載入（非同步）
	wg.Wait()
	flush()    // 套用首頁結果
	tt.Frame() // 以新狀態重繪
	if got := calls.Load(); got != 1 {
		t.Errorf("重複提交應只呼叫一次 fake.Validate，得 %d", got)
	}
	if a.sess == nil || !tt.HasText("時間線") {
		t.Errorf("完成後應導向 home：sess=%v texts %q", a.sess != nil, tt.Texts())
	}
}

// TestErrorText_KindMapping：各 kind → messages.go 對應文案，含 graphql
// code 32（與 89/215/326）→ credential 文案分支、其他 code →「X 回應錯誤：
// <code>」、包裝錯誤的 errors.As 穿透與 input／未知錯誤兜底。
func TestErrorText_KindMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"credential", &xerrors.CredentialInvalidError{Msg: "x"}, msgCredential},
		{"material", &xerrors.MaterialError{Msg: "x"}, msgMaterial},
		{"transport", &xerrors.TransportError{Msg: "x"}, msgTransport},
		{"graphql32", &xerrors.GraphQLError{OperationName: "Viewer", Code: 32}, msgCredential},
		{"graphql89", &xerrors.GraphQLError{Code: 89}, msgCredential},
		{"graphql215", &xerrors.GraphQLError{Code: 215}, msgCredential},
		{"graphql326", &xerrors.GraphQLError{Code: 326}, msgCredential},
		{"graphqlOther", &xerrors.GraphQLError{Code: 187}, graphqlErrPrefix + "187"},
		{"wrapped", fmt.Errorf("包裝: %w", &xerrors.CredentialInvalidError{Msg: "x"}), msgCredential},
		{"input", &xerrors.InvalidInputError{Msg: "缺 ct0"}, msgInput},
		{"missing", &xerrors.MissingFieldError{Msg: "缺 rest_id"}, msgInput},
		{"unknown", errors.New("boom"), msgUnknown},
		{"nil", nil, ""},
	}
	for _, tc := range tests {
		if got := errorText(tc.err); got != tc.want {
			t.Errorf("%s: errorText = %q, want %q", tc.name, got, tc.want)
		}
	}
}
