package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	xapi "github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"
)

// testService 回唯一的測試 service 名：以奈秒時戳命名，令每次測試在系統
// 憑證庫（Windows Credential Manager）中互不碰撞、也不污染正式 service；
// 測試結束經 store.Delete 清理（Delete 對不存在的鍵不視為錯誤）。
func testService(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("RIGG.X.test.%d", time.Now().UnixNano())
}

// TestCookieStore_RoundTrip 驗 Save→Load 回同一字串，且重開 store（模擬
// 重啟）後仍讀回同一字串。
func TestCookieStore_RoundTrip(t *testing.T) {
	service := testService(t)
	s1, err := NewCookieStore(service, cookieLineKey)
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	t.Cleanup(func() { _ = s1.Delete() })

	ctx := context.Background()
	line := "auth_token=abc; ct0=def; twid=u%3D1; lang=zh-CN"
	if err := s1.Save(ctx, line); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s1.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != line {
		t.Fatalf("同實例 Load 回 %q，欲得 %q", got, line)
	}

	// 重開 store（模擬重啟）：新實例自系統憑證庫讀回同一行。
	s2, err := NewCookieStore(service, cookieLineKey)
	if err != nil {
		t.Fatalf("重開 NewCookieStore: %v", err)
	}
	got2, err := s2.Load(ctx)
	if err != nil {
		t.Fatalf("重開後 Load: %v", err)
	}
	if got2 != line {
		t.Fatalf("重開後 Load 回 %q，欲得 %q", got2, line)
	}

	// 空行 Save 視為清除：其後 Load 回空字串且無錯誤（首次啟動的正常狀態）。
	if err := s2.Save(ctx, ""); err != nil {
		t.Fatalf("空行 Save: %v", err)
	}
	got3, err := s2.Load(ctx)
	if err != nil {
		t.Fatalf("清除後 Load: %v", err)
	}
	if got3 != "" {
		t.Fatalf("清除後 Load 回 %q，欲得空字串", got3)
	}
}

// TestCookieStore_ImplementsInterface 驟 NewCookieStore 產物滿足
// xapi.CookieStore（編譯期斷言另由 cookies_store.go 的
// var _ xapi.CookieStore = (*SecretsCookieStore)(nil) 機械強制；此處執行期
// 再斷言非 nil 並重複介面指派）。
func TestCookieStore_ImplementsInterface(t *testing.T) {
	s, err := NewCookieStore(testService(t), "iface-check")
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	if s == nil {
		t.Fatal("NewCookieStore 回 nil store")
	}
	var _ xapi.CookieStore = s
}
