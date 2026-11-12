package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubSystemProxy 以測試替身替換 detectSystemProxy 注入縫，測試結束還原。
func stubSystemProxy(t *testing.T, proxy string, ok bool) {
	t.Helper()
	orig := detectSystemProxy
	detectSystemProxy = func() (string, bool) { return proxy, ok }
	t.Cleanup(func() { detectSystemProxy = orig })
}

// clearProxyEnv 清空代理鏈的環境變數：空值視同未設定（ResolveProxy 以
// 非空判定），t.Setenv 同時令測試互斥，防並行互踩。
func clearProxyEnv(t *testing.T) {
	t.Helper()
	t.Setenv("X_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("ALL_PROXY", "")
}

// TestProxyOrder 驟解析順序 X_PROXY → HTTPS_PROXY → ALL_PROXY → 系統偵測
// （系統層以注入替身模擬；env 命中即回、不經預檢，替身指向可達位址，
// 若 env 層誤漏，系統值會現形使斷言失敗）。
func TestProxyOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	stubSystemProxy(t, srv.URL, true)

	cases := []struct {
		name       string
		xProxy     string
		httpsProxy string
		allProxy   string
		want       string
	}{
		{"X_PROXY 優先", "http://x-proxy.test:18080", "http://https-proxy.test:18081", "http://all-proxy.test:18082", "http://x-proxy.test:18080"},
		{"HTTPS_PROXY 次之", "", "http://https-proxy.test:18081", "http://all-proxy.test:18082", "http://https-proxy.test:18081"},
		{"ALL_PROXY 再次之", "", "", "http://all-proxy.test:18082", "http://all-proxy.test:18082"},
		{"系統偵測收尾", "", "", "", srv.URL},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearProxyEnv(t)
			if tc.xProxy != "" {
				t.Setenv("X_PROXY", tc.xProxy)
			}
			if tc.httpsProxy != "" {
				t.Setenv("HTTPS_PROXY", tc.httpsProxy)
			}
			if tc.allProxy != "" {
				t.Setenv("ALL_PROXY", tc.allProxy)
			}
			got, err := ResolveProxy()
			if err != nil {
				t.Fatalf("ResolveProxy: %v", err)
			}
			if got != tc.want {
				t.Fatalf("ResolveProxy 回 %q，欲得 %q", got, tc.want)
			}
		})
	}
}

// TestProxyFailClosed 驟三層皆空 → ("", error)（fail-closed，REQ-INV-02）。
func TestProxyFailClosed(t *testing.T) {
	clearProxyEnv(t)
	stubSystemProxy(t, "", false)
	got, err := ResolveProxy()
	if err == nil {
		t.Fatal("三層皆空應回錯誤（fail-closed）")
	}
	if got != "" {
		t.Fatalf("fail-closed 應回空字串，得 %q", got)
	}
}

// TestProxyDetectPreflight 驟系統偵測命中後的 5 秒短逾時預檢：可達位址
// （httptest listener，直接命中）→ 回偵測位址；不可達位址 → 回 error。
func TestProxyDetectPreflight(t *testing.T) {
	clearProxyEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	stubSystemProxy(t, srv.URL, true)
	got, err := ResolveProxy()
	if err != nil {
		t.Fatalf("可達偵測位址應通過預檢: %v", err)
	}
	if got != srv.URL {
		t.Fatalf("ResolveProxy 回 %q，欲得 %q", got, srv.URL)
	}

	stubSystemProxy(t, "http://127.0.0.1:1", true)
	got2, err := ResolveProxy()
	if err == nil {
		t.Fatal("不可達偵測位址應回錯誤")
	}
	if got2 != "" {
		t.Fatalf("預檢失敗應回空字串，得 %q", got2)
	}
}
