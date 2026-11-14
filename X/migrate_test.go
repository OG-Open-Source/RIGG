// migrate 測試：v1 舊鍵（x-cookie-line）→ 帳號模型的遷移序列（設計 §8.3，
// 「先寫後刪」——任何失敗點絕不清除舊鍵；成功後舊鍵保留作降級雙寫目標）。
// 以 accountService seam 指向唯一測試 service（withAccountService，絕不打
// 真實預設 service）、migrateSessionFactory 注入 fakeSession（不打真實
// X）；fixture 一律合成。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

// seedLegacyLine 於測試 service 預置 v1 舊鍵值（遷移的前置狀態）。
func seedLegacyLine(t *testing.T, line string) {
	t.Helper()
	st, err := newAccountStore(cookieLineKey)
	if err != nil {
		t.Fatalf("newAccountStore(%s): %v", cookieLineKey, err)
	}
	if err := st.Save(context.Background(), line); err != nil {
		t.Fatalf("預置舊鍵 %s: %v", cookieLineKey, err)
	}
}

// loadLegacyLine 讀回舊鍵現值（斷言「舊鍵仍在／未動」用）。
func loadLegacyLine(t *testing.T) string {
	t.Helper()
	st, err := newAccountStore(cookieLineKey)
	if err != nil {
		t.Fatalf("newAccountStore(%s): %v", cookieLineKey, err)
	}
	line, err := st.Load(context.Background())
	if err != nil {
		t.Fatalf("讀取舊鍵 %s: %v", cookieLineKey, err)
	}
	return line
}

// injectMigrateFactory 以替身工廠覆寫 migrateSessionFactory seam，測試結束
// 還原；回傳呼叫記錄旗標位址（no-op 斷言用）。
func injectMigrateFactory(t *testing.T, validate func(cookie string) (viewerInfo, error)) *bool {
	t.Helper()
	called := false
	orig := migrateSessionFactory
	migrateSessionFactory = func(cookieLine, proxyURL string) (xSession, error) {
		called = true
		return &fakeSession{validate: validate}, nil
	}
	t.Cleanup(func() { migrateSessionFactory = orig })
	return &called
}

// TestMigrateCreatesAccountAndIndex：舊鍵有值且索引不存在 → 以臨時會話
// Validate 取得身分 → saveAccount（身分與憑證綁定）＋ saveIndex
// (Order=[uid], Current=uid)；舊鍵保留（降級雙寫目標，v1.2 清理）。
func TestMigrateCreatesAccountAndIndex(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("9001"), cookieLineKey)
	ctx := context.Background()

	const legacy = "auth_token=synthetic-9001; ct0=synthetic-9001b"
	seedLegacyLine(t, legacy)
	injectMigrateFactory(t, func(string) (viewerInfo, error) {
		return viewerInfo{UserID: "9001", ScreenName: "goated", Name: "合成帳號"}, nil
	})

	if err := migrateLegacyStore(ctx, ""); err != nil {
		t.Fatalf("migrateLegacyStore: %v", err)
	}

	idx, err := loadIndex(ctx)
	if err != nil {
		t.Fatalf("loadIndex: %v", err)
	}
	if idx == nil {
		t.Fatal("遷移成功後索引應存在")
	}
	if !slices.Equal(idx.Order, []string{"9001"}) {
		t.Fatalf("索引 Order 回 %v，欲得 [9001]", idx.Order)
	}
	if idx.Current != "9001" {
		t.Fatalf("索引 Current 回 %q，欲得 %q", idx.Current, "9001")
	}
	if idx.Version != accountIndexVersion {
		t.Fatalf("索引 Version 回 %d，欲得 %d", idx.Version, accountIndexVersion)
	}

	// 帳號鍵 x-account-9001 有值且 CookieLine 綁定舊鍵值（REQ-WEB-069）。
	st, err := newAccountStore(accountKey("9001"))
	if err != nil {
		t.Fatalf("newAccountStore(%s): %v", accountKey("9001"), err)
	}
	raw, err := st.Load(ctx)
	if err != nil {
		t.Fatalf("讀取帳號鍵: %v", err)
	}
	if raw == "" {
		t.Fatal("遷移成功後帳號鍵 x-account-9001 應有值")
	}
	var acc Account
	if err := json.Unmarshal([]byte(raw), &acc); err != nil {
		t.Fatalf("帳號鍵 JSON 解析: %v", err)
	}
	want := Account{UserID: "9001", ScreenName: "goated", Name: "合成帳號", CookieLine: legacy}
	if acc != want {
		t.Fatalf("帳號回 %+v，欲得 %+v", acc, want)
	}

	// 舊鍵仍在（不刪：降級雙寫需要它，v1.2 清理）。
	if line := loadLegacyLine(t); line != legacy {
		t.Fatalf("遷移成功後舊鍵應保留原值，得 %q 欲得 %q", line, legacy)
	}
}

// TestMigrateKeepsLegacyOnViewerFailure：Validate 失敗 → 舊鍵保留、無索引、
// 回 error，且錯誤訊息不含 cookie 值（REQ-INV-01）。
func TestMigrateKeepsLegacyOnViewerFailure(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("9002"), cookieLineKey)
	ctx := context.Background()

	const legacy = "auth_token=synthetic-9002; ct0=synthetic-9002b"
	seedLegacyLine(t, legacy)
	injectMigrateFactory(t, func(string) (viewerInfo, error) {
		return viewerInfo{}, errors.New("合成驗證失敗")
	})

	err := migrateLegacyStore(ctx, "")
	if err == nil {
		t.Fatal("Validate 失敗時 migrateLegacyStore 應回錯誤")
	}
	if strings.Contains(err.Error(), legacy) {
		t.Errorf("錯誤訊息不得含 cookie 值（REQ-INV-01），得 %q", err.Error())
	}

	// 舊鍵保留（絕不清除——遷移序列任何失敗點）。
	if line := loadLegacyLine(t); line != legacy {
		t.Fatalf("失敗後舊鍵應保留原值，得 %q 欲得 %q", line, legacy)
	}
	// 無索引（未寫入任何帳號模型資料）。
	idx, err := loadIndex(ctx)
	if err != nil {
		t.Fatalf("loadIndex: %v", err)
	}
	if idx != nil {
		t.Fatalf("失敗後不應寫入索引，得 %+v", *idx)
	}
}

// TestMigrateNoopWhenIndexExists：索引已存在 → no-op（不組臨時會話、不動
// 舊鍵）、nil error。
func TestMigrateNoopWhenIndexExists(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, cookieLineKey)
	ctx := context.Background()

	const legacy = "auth_token=synthetic-7001; ct0=synthetic-7001b"
	seedLegacyLine(t, legacy)
	wantIdx := accountIndex{Version: accountIndexVersion, Order: []string{"7001"}, Current: "7001"}
	if err := saveIndex(ctx, wantIdx); err != nil {
		t.Fatalf("saveIndex: %v", err)
	}
	called := injectMigrateFactory(t, func(string) (viewerInfo, error) {
		return viewerInfo{UserID: "9003"}, nil
	})

	if err := migrateLegacyStore(ctx, ""); err != nil {
		t.Fatalf("索引已存在時 migrateLegacyStore 應為 no-op，得錯誤 %v", err)
	}
	if *called {
		t.Error("索引已存在時不應組裝臨時會話（工廠被呼叫）")
	}
	// 舊鍵不動。
	if line := loadLegacyLine(t); line != legacy {
		t.Fatalf("no-op 時舊鍵不應被動，得 %q 欲得 %q", line, legacy)
	}
	// 索引不變。
	idx, err := loadIndex(ctx)
	if err != nil {
		t.Fatalf("loadIndex: %v", err)
	}
	if idx == nil || !slices.Equal(idx.Order, wantIdx.Order) || idx.Current != wantIdx.Current || idx.Version != wantIdx.Version {
		t.Fatalf("no-op 時索引不應被動，得 %+v 欲得 %+v", idx, wantIdx)
	}
}

// TestDualWriteWritesLegacyKey：dualWrite 把活躍帳號 cookie 行寫入 v1 舊鍵
// （降級雙寫期語義）。
func TestDualWriteWritesLegacyKey(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, cookieLineKey)

	if err := dualWrite("abc"); err != nil {
		t.Fatalf("dualWrite: %v", err)
	}
	if line := loadLegacyLine(t); line != "abc" {
		t.Fatalf("dualWrite 後舊鍵回 %q，欲得 %q", line, "abc")
	}
}
