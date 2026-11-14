// session_switch 測試：Task 5 的帳號化會話工廠（newSessionFor）——store
// 以 x-account-<uid> 鍵建構、write-through 落活躍帳號鍵（Review Focus 2）、
// transport Close 冪等。全程經 newStore seam 注入記錄器替身，絕不觸及真實
// 系統憑證庫（含預設 service 的建構）；fixture 一律合成。
package main

import (
	"context"
	"sync"
	"testing"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"
)

// keyedStore 是 newStore seam 的記錄器替身：以建構時的鍵標記自身，記錄
// 每次 Save 的行（write-through 斷言面），不打系統憑證庫。
type keyedStore struct {
	key string

	mu    sync.Mutex
	saves []string
}

func (s *keyedStore) Load(ctx context.Context) (string, error) { return "", nil }

func (s *keyedStore) Save(ctx context.Context, line string) error {
	s.mu.Lock()
	s.saves = append(s.saves, line)
	s.mu.Unlock()
	return nil
}

func (s *keyedStore) Delete() error { return nil }

// savedLines 回 Save 行的快照。
func (s *keyedStore) savedLines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.saves...)
}

// withKeyedStores 以記錄器替身覆寫 newStore seam：每次建構依鍵登記於回傳
// 的映射，測試結束還原（同 withAccountService 的 seam 覆寫模式）。
func withKeyedStores(t *testing.T) map[string]*keyedStore {
	t.Helper()
	orig := newStore
	stores := make(map[string]*keyedStore)
	newStore = func(service, key string) (auth.CookieStore, error) {
		st := &keyedStore{key: key}
		stores[key] = st
		return st, nil
	}
	t.Cleanup(func() { newStore = orig })
	return stores
}

// writeThrough 以會話的 write-through 語義觸發落盤：absorbSetCookies 為
// 引擎內部（api 套件未導出），故依 brief 認可的觀測路徑，以 auth 層公開
// 面（Store）重放其 Save 步驟——auth 接線 store 時以合併後最新單行 Save。
func writeThrough(s *engineSession) {
	if store := s.auth.Store(); store != nil {
		_ = store.Save(context.Background(), s.auth.CookieHeader())
	}
}

// TestNewSessionForUsesAccountKey：newSessionFor 以 x-account-<uid> 鍵建構
// 儲存層（恰一次、非舊鍵），並將其接線進會話 Auth（write-through 落鍵的
// 前提）。Task 6 起 store 為 newAccountCookieStore 的 adapter（鍵內容碰撞
// 裁決）包於帳號鍵 store 之下——斷言面同步調整：以 write-through 經
// adapter 落帳號鍵驗證接線（raw 行不得直寫帳號鍵）。
func TestNewSessionForUsesAccountKey(t *testing.T) {
	stores := withKeyedStores(t)

	sess, err := newSessionFor(Account{UserID: "9001", CookieLine: "c"}, "")
	if err != nil {
		t.Fatalf("newSessionFor(9001): %v", err)
	}
	defer func() { _ = sess.Close() }()

	st, ok := stores[accountKey("9001")]
	if !ok {
		t.Fatalf("應以 %q 鍵建構 store，實際建構 %d 個", accountKey("9001"), len(stores))
	}
	if len(stores) != 1 {
		t.Errorf("應恰建構 1 個 store，得到 %d", len(stores))
	}
	if sess.auth == nil || sess.auth.Store() == nil {
		t.Fatal("會話 Auth 應接線帳號鍵 adapter（write-through 落鍵前提）")
	}
	writeThrough(sess)
	if got := st.savedLines(); len(got) != 1 {
		t.Errorf("write-through 應經 adapter 落帳號鍵恰一次，得到 %d", len(got))
	}
}

// TestWriteThroughTargetsAccountKey：write-through 落活躍帳號的鍵——帳號
// 9001 的 Save 落 x-account-9001（非舊鍵 x-cookie-line）；帳號 9002 的
// write-through 落自己的鍵，不覆蓋第一個帳號（Review Focus 2）。
func TestWriteThroughTargetsAccountKey(t *testing.T) {
	stores := withKeyedStores(t)

	s1, err := newSessionFor(Account{UserID: "9001", CookieLine: "c"}, "")
	if err != nil {
		t.Fatalf("newSessionFor(9001): %v", err)
	}
	defer func() { _ = s1.Close() }()
	s2, err := newSessionFor(Account{UserID: "9002", CookieLine: "c"}, "")
	if err != nil {
		t.Fatalf("newSessionFor(9002): %v", err)
	}
	defer func() { _ = s2.Close() }()

	if s1.auth.Store() == nil || s2.auth.Store() == nil {
		t.Fatal("兩會話皆應接線 write-through 儲存層")
	}

	writeThrough(s1)
	writeThrough(s2)
	writeThrough(s1) // ct0 輪替情境：同帳號再次落盤

	r1, ok := stores[accountKey("9001")]
	if !ok {
		t.Fatal("帳號 9001 的 store 應經 seam 建構")
	}
	r2, ok := stores[accountKey("9002")]
	if !ok {
		t.Fatal("帳號 9002 的 store 應經 seam 建構")
	}
	if got := len(r1.savedLines()); got != 2 {
		t.Errorf("帳號 9001 的 store 應恰收到 2 次 Save，得到 %d", got)
	}
	if got := len(r2.savedLines()); got != 1 {
		t.Errorf("帳號 9002 的 store 應恰收到 1 次 Save（不覆蓋他帳），得到 %d", got)
	}
	if _, ok := stores[cookieLineKey]; ok {
		t.Errorf("write-through 不得以舊鍵 %q 建構／落盤", cookieLineKey)
	}
}

// TestEngineSessionCloseIdempotent：Close 兩次不 panic、第二次回 nil；
// tr 未接線（零值會話）同樣安全。
func TestEngineSessionCloseIdempotent(t *testing.T) {
	_ = withKeyedStores(t) // newSessionFor 走 seam，不觸及真實憑證庫

	sess, err := newSessionFor(Account{UserID: "9003", CookieLine: "c"}, "")
	if err != nil {
		t.Fatalf("newSessionFor(9003): %v", err)
	}
	if sess.tr == nil {
		t.Fatal("engineSession 應持有 transport（Close 接線前提）")
	}
	if err := sess.Close(); err != nil {
		t.Errorf("首次 Close: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Errorf("第二次 Close 應冪等回 nil，得 %v", err)
	}

	var bare engineSession // tr 未接線：nil 檢查路徑
	if err := bare.Close(); err != nil {
		t.Errorf("tr 未接線的 Close 應回 nil，得 %v", err)
	}
}
