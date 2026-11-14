// switch 測試：Task 6 的帳號切換序列（§8.4 序列 1–8）、世代邊界
// （D18，Review Focus 1）、切換失敗收斂（Review Focus 3）、憑證預熱
// （§8.5）與 quit flush 正規化。接縫以注入替身隔離
// （switchSessionFactory／spawn／apply，沿 view_login_test.go 的
// fakeSession 模式）；帳號儲存沿 accountTestService seam 隔離，絕不打
// 真實預設 service；fixture 一律合成。
package main

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// TestQuitFlushTargetsActiveStore：指派 activeStore 後 quit flush 落帳號鍵
// （Step 0 正規化——Flush 以參數收目標，不再暫替 AppState.store）：
// 帳號鍵先有中繼資料，flush 以輪替後的行落盤——adapter 的
// read-modify-write 須保留 ScreenName、僅更新 CookieLine（裁決：鍵內容
// 碰撞——raw 行不得覆蓋 Account JSON）。
func TestQuitFlushTargetsActiveStore(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("7400"))
	ctx := context.Background()

	const stored = "auth_token=synthetic-7400; ct0=synthetic-7400b" // fixture 一律合成
	if err := saveAccount(ctx, Account{UserID: "7400", ScreenName: "goated", Name: "合成帳號", CookieLine: stored}); err != nil {
		t.Fatalf("saveAccount: %v", err)
	}
	if err := saveIndex(ctx, accountIndex{Version: accountIndexVersion, Order: []string{"7400"}, Current: "7400"}); err != nil {
		t.Fatalf("saveIndex: %v", err)
	}

	a := newTestApp()
	a.activeStore = newAccountCookieStore("7400")
	authx := auth.NewAuth("")
	// 引擎行與已存值輪替（ct0 變更）：flush 應把新行經 adapter 落帳號鍵。
	if err := authx.PrepareAuth("auth_token=synthetic-7400; ct0=synthetic-7400c"); err != nil {
		t.Fatalf("PrepareAuth: %v", err)
	}
	a.auth = authx

	a.flushActive() // quit handler 本體

	if a.state.Err() != nil {
		t.Fatalf("quit flush 不應記錯誤，得 %v", a.state.Err())
	}
	accs, idx, err := loadAccounts(ctx)
	if err != nil {
		t.Fatalf("flush 後 loadAccounts（鍵應仍為 Account JSON）: %v", err)
	}
	if len(accs) != 1 {
		t.Fatalf("loadAccounts 回 %d 帳號，欲得 1", len(accs))
	}
	if accs[0].ScreenName != "goated" || accs[0].Name != "合成帳號" {
		t.Errorf("quit flush 不得摧毀中繼資料：得 %+v", accs[0])
	}
	if accs[0].CookieLine != authx.CookieHeader() {
		t.Errorf("quit flush 應落輪替後的行：得 %q，欲得引擎行", accs[0].CookieLine)
	}
	if idx.Current != "7400" {
		t.Errorf("quit flush 不得動索引：Current = %q", idx.Current)
	}
}

// withSwitchFactory 以替身覆寫 switchSessionFactory seam（§8.4 步驟 4）：
// 任何帳號的組裝都回傳注入的 fakeSession（絕不組真實引擎鏈），測試結束
// 還原（同 withAccountService 的 seam 覆寫模式）。
func withSwitchFactory(t *testing.T, fake *fakeSession) {
	t.Helper()
	orig := switchSessionFactory
	switchSessionFactory = func(Account, string) (xSession, error) { return fake, nil }
	t.Cleanup(func() { switchSessionFactory = orig })
}

// countCalls 數呼叫日誌中某接縫的次數。
func countCalls(calls []string, name string) int {
	n := 0
	for _, c := range calls {
		if c == name {
			n++
		}
	}
	return n
}

// seedSwitchAccounts 落兩個帳號與索引（Current=first）——切換測試的儲存
// 側夾具（accountTestService 隔離；fixture 一律合成）。
func seedSwitchAccounts(t *testing.T, first, second string) {
	t.Helper()
	ctx := context.Background()
	for _, uid := range []string{first, second} {
		acc := Account{UserID: uid, ScreenName: "u" + uid, Name: "合成帳號" + uid, CookieLine: "auth_token=synthetic-" + uid + "; ct0=synthetic-" + uid + "b"}
		if err := saveAccount(ctx, acc); err != nil {
			t.Fatalf("saveAccount(%s): %v", uid, err)
		}
	}
	if err := saveIndex(ctx, accountIndex{Version: accountIndexVersion, Order: []string{first, second}, Current: first}); err != nil {
		t.Fatalf("saveIndex: %v", err)
	}
}

// TestSwitchGenerationDiscardsStalePublish（Review Focus 1 釘測）：舊 feed
// 背景工作於切換後 publish → 世代比對丟棄——a.gen 已變、posts 不含舊帳號
// 帖（含切換前已顯示者，墓碑）、seen 零混入。非同步 spawn＋佇列 apply
// （模擬主執行緒序列化），切換本體以同步 spawn 使序列可控。
func TestSwitchGenerationDiscardsStalePublish(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("7001"), accountKey("7002"))
	seedSwitchAccounts(t, "7001", "7002")

	release := make(chan struct{})
	old := &fakeSession{userID: "7001", home: func(string, []string) (*timelineResult, error) {
		<-release // 模擬在途：切換完成後才回程
		return pageOf(mkPosts("old1", "old2"), "c2"), nil
	}}
	fresh := &fakeSession{userID: "7002", home: pageQueue(pageOf(mkPosts("new1"), ""))}

	a := newTestApp()
	// 非同步 spawn（真 goroutine）＋佇列 apply（模擬主執行緒套用）——
	// 同 TestLoginView_Debounce 的併發模板。
	var mu sync.Mutex
	var pending []func()
	a.apply = func(fn func()) {
		mu.Lock()
		pending = append(pending, fn)
		mu.Unlock()
	}
	flushAll := func() {
		for {
			mu.Lock()
			if len(pending) == 0 {
				mu.Unlock()
				return
			}
			fn := pending[0]
			pending = pending[1:]
			mu.Unlock()
			fn()
		}
	}
	flushOne := func() {
		t.Helper()
		mu.Lock()
		if len(pending) == 0 {
			mu.Unlock()
			t.Fatal("佇列空：預期的切換 publish 不在佇列")
		}
		fn := pending[0]
		pending = pending[1:]
		mu.Unlock()
		fn()
	}
	var wg sync.WaitGroup
	a.spawn = func(fn func(ctx context.Context)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn(context.Background())
		}()
	}

	a.sess = old
	a.feeds[feedForYou].posts = mkPosts("seed") // 切換前已顯示的舊帳號帖
	a.feeds[feedForYou].seen = []string{"seed"}
	a.gen = 0

	a.loadFeed(feedForYou) // 舊 feed 工作派出（阻塞於 release）

	// 切換以同步 spawn 執行（序列可控）；其 publish 入佇列、尚未套用。
	a.spawn = func(fn func(ctx context.Context)) { fn(context.Background()) }
	withSwitchFactory(t, fresh)
	a.switchTo("7002")
	flushOne() // 套用替換＋gen++＋墓碑（其內自動載入的 publish 隨後入佇列）

	close(release) // 舊工作回程 → 其 publish 入佇列（此刻 gen 已變）
	wg.Wait()
	flushAll() // 新帳號首頁套用；舊帳號 publish 應被世代檢查丟棄

	if a.gen != 1 {
		t.Fatalf("切換後 a.gen 應已變（+1），得 %d", a.gen)
	}
	if got := workIDs(a.feeds[feedForYou].posts); !sameStrings(got, []string{"new1"}) {
		t.Errorf("世代過期的舊 publish 不得污染新帳號畫面：posts = %q，欲得 [new1]", got)
	}
	for _, stale := range []string{"seed", "old1", "old2"} {
		if slices.Contains(workIDs(a.feeds[feedForYou].posts), stale) {
			t.Errorf("舊帳號帖 %s 不得出現在新帳號畫面：posts = %q", stale, workIDs(a.feeds[feedForYou].posts))
		}
	}
	if !sameStrings(a.feeds[feedForYou].seen, []string{"new1"}) {
		t.Errorf("seen 零混入：得 %q，欲得 [new1]", a.feeds[feedForYou].seen)
	}
	// 墓碑涵蓋全部三槽：Following／search 亦歸零，且落回 For You。
	if len(a.feeds[feedFollowing].posts) != 0 || len(a.feeds[feedFollowing].seen) != 0 {
		t.Errorf("墓碑應歸零 Following 槽：posts=%q seen=%q", workIDs(a.feeds[feedFollowing].posts), a.feeds[feedFollowing].seen)
	}
	if len(a.feeds[feedSearch].posts) != 0 {
		t.Errorf("墓碑應歸零 search 預留槽：posts = %q", workIDs(a.feeds[feedSearch].posts))
	}
	if a.activeFeed != feedForYou {
		t.Errorf("切換後應落回 For You，得 %d", a.activeFeed)
	}
	if a.switchBusy {
		t.Error("切換完成後 switchBusy 應復位")
	}
}

// TestSwitchFailureRollback（Review Focus 3 釘測）：目標帳號 Validate 失敗
// → 索引 Current 回原帳號、a.sess 為原會話（未被 Close、未替換）、
// accInvalid[目標] 為真、secret 保留、錯誤區呈現（switchErr）、世代不變。
func TestSwitchFailureRollback(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("7301"), accountKey("7302"))
	seedSwitchAccounts(t, "7301", "7302")

	old := &fakeSession{userID: "7301"}
	bad := &fakeSession{userID: "7302", validate: func(string) (viewerInfo, error) {
		return viewerInfo{}, &xerrors.CredentialInvalidError{Msg: "HTTP 401"}
	}}
	a := newTestApp()
	a.sess = old
	withSwitchFactory(t, bad)

	a.switchTo("7302")

	idx, err := loadIndex(context.Background())
	if err != nil {
		t.Fatalf("loadIndex: %v", err)
	}
	if idx == nil || idx.Current != "7301" {
		t.Fatalf("索引 Current 應回寫原帳號，得 %+v", idx)
	}
	if a.sess != old {
		t.Error("失敗收斂：a.sess 應為原會話（未替換）")
	}
	if countCalls(old.callLog(), "Close") != 0 {
		t.Error("失敗收斂：原會話不得被 Close")
	}
	if !a.accInvalid["7302"] {
		t.Error("Validate 失敗應標記目標帳號失效")
	}
	var cie *xerrors.CredentialInvalidError
	if !errors.As(a.switchErr, &cie) {
		t.Errorf("切換失敗應呈現於錯誤區（switchErr），得 %v", a.switchErr)
	}
	if a.switchBusy {
		t.Error("失敗後 switchBusy 應復位（可再切換）")
	}
	if a.gen != 0 {
		t.Errorf("失敗不得推進世代，得 %d", a.gen)
	}
	// secret 保留：目標帳號鍵原值可讀（絕不清除）。
	accs, _, err := loadAccounts(context.Background())
	if err != nil {
		t.Fatalf("loadAccounts: %v", err)
	}
	for _, acc := range accs {
		if acc.UserID == "7302" && acc.CookieLine != "auth_token=synthetic-7302; ct0=synthetic-7302b" {
			t.Errorf("失敗收斂不得動 secret：CookieLine = %q", acc.CookieLine)
		}
	}
}

// TestSwitchTTLCacheSkipsValidation：fresh（TTL 內）時 newSess 的 Validate
// 計數為 0（§8.4 步驟 5 的跳過路徑），切換照常完成。
func TestSwitchTTLCacheSkipsValidation(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("7201"), accountKey("7202"))
	seedSwitchAccounts(t, "7201", "7202")

	old := &fakeSession{userID: "7201"}
	fresh := &fakeSession{userID: "7202", home: pageQueue(pageOf(mkPosts("n"), ""))}
	a := newTestApp()
	a.sess = old
	withSwitchFactory(t, fresh)
	a.creds.mark("7202") // TTL 內（§8.5）

	a.switchTo("7202")

	if got := countCalls(fresh.callLog(), "Validate"); got != 0 {
		t.Errorf("TTL 內切換應跳過 Validate，得 %d 次", got)
	}
	if a.sess != fresh {
		t.Error("跳過驗證的切換仍應完成替換")
	}
	if a.gen != 1 {
		t.Errorf("成功切換應 gen++，得 %d", a.gen)
	}
	if a.accInvalid["7202"] {
		t.Error("TTL 內成功切換不得標記失效")
	}
}

// TestSwitchSuccessClosesOldSession：成功切換後舊會話的 Close 呼叫次數
// ≥1（§8.4 步驟 6，transport 收斂——簡報 §1.3 實測根因）。
func TestSwitchSuccessClosesOldSession(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("7101"), accountKey("7102"))
	seedSwitchAccounts(t, "7101", "7102")

	old := &fakeSession{userID: "7101"}
	fresh := &fakeSession{userID: "7102", home: pageQueue(pageOf(mkPosts("n"), ""))}
	a := newTestApp()
	a.sess = old
	withSwitchFactory(t, fresh)

	a.switchTo("7102")

	if closes := countCalls(old.callLog(), "Close"); closes < 1 {
		t.Fatalf("成功切換應 Close 舊會話 ≥1 次，得 %d（日誌 %q）", closes, old.callLog())
	}
	if a.sess != fresh || fresh.userID != "7102" {
		t.Errorf("成功切換應替換為目標帳號的會話，得 %v", a.sess)
	}
	idx, err := loadIndex(context.Background())
	if err != nil {
		t.Fatalf("loadIndex: %v", err)
	}
	if idx == nil || idx.Current != "7102" {
		t.Fatalf("成功切換後索引 Current 應為目標，得 %+v", idx)
	}
}

// TestWarmupLoadsAllAccounts：warmup 讀索引＋全部帳號 secret 進 a.accounts
// （記憶體快照）；冪等——重跑同值；無舊鍵時遷移 no-op、無錯誤。
func TestWarmupLoadsAllAccounts(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("6100"), accountKey("6101"), cookieLineKey)
	seedSwitchAccounts(t, "6100", "6101")

	a := newTestApp()
	a.warmup()
	if len(a.accounts) != 2 {
		t.Fatalf("warmup 應載入全部帳號，得 %d", len(a.accounts))
	}
	if a.accounts[0].UserID != "6100" || a.accounts[1].UserID != "6101" {
		t.Errorf("帳號快照應依索引序：得 %v", []string{a.accounts[0].UserID, a.accounts[1].UserID})
	}
	if a.state.Err() != nil {
		t.Errorf("warmup 不應記錯誤，得 %v", a.state.Err())
	}
	a.warmup() // 冪等，可重跑
	if len(a.accounts) != 2 || a.accounts[0].UserID != "6100" || a.accounts[1].UserID != "6101" {
		t.Errorf("重跑 warmup 應同值：得 %+v", a.accounts)
	}
}

// TestGoSessionSubtreeCancel：feed 背景工作以會話子樹派發（§8.4）——
// sessCancel 斬斷在途（switchTo 步驟 3 語義）；取消後的下一次派發取得新
// 子樹（步驟 8 自動載入語義）；Close 經父 context 收斂並等待。
func TestGoSessionSubtreeCancel(t *testing.T) {
	a := newApp() // spawn／apply nil：生產路徑（state.GoSession 派發）

	started := make(chan struct{})
	done := make(chan struct{})
	a.goSession(func(ctx context.Context) {
		close(started)
		<-ctx.Done() // 僅於子樹取消後收斂
		close(done)
	})
	<-started
	if a.sessCtx == nil || a.sessCtx.Err() != nil {
		t.Fatal("派發後應有活躍且未取消的會話子樹")
	}

	a.sessCancel() // switchTo 步驟 3：斬斷舊在途
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sessCancel 後子樹工作未被斬斷")
	}

	// 重建：取消後的下一次派發取得新子樹。
	rebuilt := make(chan struct{})
	a.goSession(func(ctx context.Context) {
		if ctx.Err() != nil {
			t.Error("重建的子樹不應已取消")
		}
		close(rebuilt)
	})
	<-rebuilt

	a.state.Close(nil) // 收斂：Close 經父 context 取代子樹並等待工作結束
}
