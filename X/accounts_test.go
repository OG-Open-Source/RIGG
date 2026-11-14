package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// accountTestService 是帳號測試唯一的 service 名：以奈秒時戳命名，令每次
// 測試執行在系統憑證庫（Windows Credential Manager）中互不碰撞、也不污染
// 正式 service。cookies_store_test.go 的 testService helper 已占用該名，
// 故取 accountTestService；取值模式同任務簡報（x-test-<unixnano>）。
var accountTestService = fmt.Sprintf("x-test-%d", time.Now().UnixNano())

// withAccountService 以唯一測試 service 覆寫 accountService seam（測試不得
// 打真實預設 service），測試結束還原。
func withAccountService(t *testing.T) {
	t.Helper()
	orig := accountService
	accountService = accountTestService
	t.Cleanup(func() { accountService = orig })
}

// cleanupSecrets 註冊測試結束後直接以測試 service 刪除指定鍵（不經 seam，
// 不依賴 cleanup 順序；Delete 對不存在的鍵不視為錯誤）。
func cleanupSecrets(t *testing.T, keys ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, key := range keys {
			st, err := NewCookieStore(accountTestService, key)
			if err != nil {
				t.Logf("清理 %q: %v", key, err)
				continue
			}
			_ = st.Delete()
		}
	})
}

// TestAccountRoundTrip 驗 saveAccount→loadAccounts 回同值：索引指向的
// 帳號逐鍵讀回，欄位逐一相等。另驗索引不存在時 loadAccounts 回空清單與
// 零值索引、無錯誤（首次啟動的正常狀態）。
func TestAccountRoundTrip(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("1001"))
	ctx := context.Background()

	// 索引不存在：空清單、零值索引、無錯誤。
	accounts, idx, err := loadAccounts(ctx)
	if err != nil {
		t.Fatalf("無索引時 loadAccounts: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("無索引時 loadAccounts 回 %d 帳號，欲得 0", len(accounts))
	}
	if idx.Version != 0 || idx.Current != "" || len(idx.Order) != 0 {
		t.Fatalf("無索引時索引回 %+v，欲得零值", idx)
	}

	want := Account{
		UserID:     "1001",
		ScreenName: "goated",
		Name:       "合成帳號",
		CookieLine: "auth_token=synthetic-a; ct0=synthetic-b", // fixture 一律合成
	}
	if err := saveAccount(ctx, want); err != nil {
		t.Fatalf("saveAccount: %v", err)
	}
	if err := saveIndex(ctx, accountIndex{Version: accountIndexVersion, Order: []string{want.UserID}, Current: want.UserID}); err != nil {
		t.Fatalf("saveIndex: %v", err)
	}
	accounts, idx, err = loadAccounts(ctx)
	if err != nil {
		t.Fatalf("loadAccounts: %v", err)
	}
	if len(accounts) != 1 || accounts[0] != want {
		t.Fatalf("loadAccounts 回 %+v，欲得 [%+v]", accounts, want)
	}
	if !slices.Equal(idx.Order, []string{want.UserID}) {
		t.Fatalf("索引 Order 回 %v，欲得 [%s]", idx.Order, want.UserID)
	}
	if idx.Current != want.UserID {
		t.Fatalf("索引 Current 回 %q，欲得 %q", idx.Current, want.UserID)
	}
	if idx.Version != accountIndexVersion {
		t.Fatalf("索引 Version 回 %d，欲得 %d", idx.Version, accountIndexVersion)
	}
}

// TestIndexOrderAndCurrent 驗 saveIndex→loadAccounts 依索引 Order 回帳號：
// 儲存序與索引序不同時，讀回以索引序為準，Current 原樣回傳。
func TestIndexOrderAndCurrent(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("2001"), accountKey("2002"), accountKey("2003"))
	ctx := context.Background()

	for _, uid := range []string{"2001", "2002", "2003"} {
		acc := Account{
			UserID:     uid,
			ScreenName: "user" + uid,
			Name:       "合成帳號" + uid,
			CookieLine: "auth_token=synthetic-" + uid,
		}
		if err := saveAccount(ctx, acc); err != nil {
			t.Fatalf("saveAccount(%s): %v", uid, err)
		}
	}
	wantOrder := []string{"2003", "2001", "2002"}
	if err := saveIndex(ctx, accountIndex{Version: accountIndexVersion, Order: wantOrder, Current: "2001"}); err != nil {
		t.Fatalf("saveIndex: %v", err)
	}

	accounts, idx, err := loadAccounts(ctx)
	if err != nil {
		t.Fatalf("loadAccounts: %v", err)
	}
	if len(accounts) != len(wantOrder) {
		t.Fatalf("loadAccounts 回 %d 帳號，欲得 %d", len(accounts), len(wantOrder))
	}
	for i, want := range wantOrder {
		if accounts[i].UserID != want {
			t.Fatalf("loadAccounts 第 %d 個回 user %s，欲得 %s（順序應依索引）", i, accounts[i].UserID, want)
		}
	}
	if !slices.Equal(idx.Order, wantOrder) {
		t.Fatalf("索引 Order 回 %v，欲得 %v", idx.Order, wantOrder)
	}
	if idx.Current != "2001" {
		t.Fatalf("索引 Current 回 %q，欲得 %q", idx.Current, "2001")
	}
}

// TestDeleteAccountRemovesFromIndex 驗 deleteAccount 刪帳號 secret、索引
// 除名，且被刪者為 Current 時 Current 歸空；其餘帳號不受影響。
func TestDeleteAccountRemovesFromIndex(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("3001"), accountKey("3002"))
	ctx := context.Background()

	for _, uid := range []string{"3001", "3002"} {
		acc := Account{
			UserID:     uid,
			ScreenName: "user" + uid,
			Name:       "合成帳號" + uid,
			CookieLine: "auth_token=synthetic-" + uid,
		}
		if err := saveAccount(ctx, acc); err != nil {
			t.Fatalf("saveAccount(%s): %v", uid, err)
		}
	}
	if err := saveIndex(ctx, accountIndex{Version: accountIndexVersion, Order: []string{"3001", "3002"}, Current: "3001"}); err != nil {
		t.Fatalf("saveIndex: %v", err)
	}

	if err := deleteAccount(ctx, "3001"); err != nil {
		t.Fatalf("deleteAccount: %v", err)
	}

	accounts, idx, err := loadAccounts(ctx)
	if err != nil {
		t.Fatalf("loadAccounts: %v", err)
	}
	if len(accounts) != 1 || accounts[0].UserID != "3002" {
		t.Fatalf("loadAccounts 回 %+v，欲得僅剩 user 3002", accounts)
	}
	if !slices.Equal(idx.Order, []string{"3002"}) {
		t.Fatalf("索引 Order 回 %v，欲得 [3002]", idx.Order)
	}
	if idx.Current != "" {
		t.Fatalf("索引 Current 回 %q，被刪者為活躍帳號時應歸空", idx.Current)
	}

	// 帳號 secret 本身須已刪除：直接讀鍵回空（Load 對缺失鍵回 ("", nil)）。
	st, err := NewCookieStore(accountTestService, accountKey("3001"))
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	if line, err := st.Load(ctx); err != nil || line != "" {
		t.Fatalf("被刪帳號鍵 Load 回 (%q, %v)，欲得 (\"\", nil)", line, err)
	}
}

// TestAccountKeyFormat 驗 accountKey 的鍵格式："x-account-" + userID。
func TestAccountKeyFormat(t *testing.T) {
	if got := accountKey("123"); got != "x-account-123" {
		t.Fatalf("accountKey(%q) 回 %q，欲得 %q", "123", got, "x-account-123")
	}
}

// TestLoadAccountsMissingAccountKey 驗索引存在但帳號鍵缺失時 loadAccounts
// 回 material 錯誤（不靜默跳過；帳號資料絕不靜默清空）。
func TestLoadAccountsMissingAccountKey(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("4001"))
	ctx := context.Background()

	// 只寫索引、不寫帳號鍵：模擬索引引用的 secret 缺失。
	if err := saveIndex(ctx, accountIndex{Version: accountIndexVersion, Order: []string{"4001"}, Current: "4001"}); err != nil {
		t.Fatalf("saveIndex: %v", err)
	}

	accounts, idx, err := loadAccounts(ctx)
	if err == nil {
		t.Fatalf("帳號鍵缺失時 loadAccounts 應回錯誤，卻得帳號 %+v 索引 %+v", accounts, idx)
	}
	var material *xerrors.MaterialError
	if !errors.As(err, &material) {
		t.Fatalf("錯誤 %v 應為 material 語義（*xerrors.MaterialError）", err)
	}
}
