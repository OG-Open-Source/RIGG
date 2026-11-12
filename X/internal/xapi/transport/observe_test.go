package transport

// observe 測試：REQ-INV-13 觀測鉤——OnRequest／OnResponse 各觸發一次、
// 參數正確；鉤子 panic 以 recover 吸收，不得中斷主流程。

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
)

// TestObserveHooks_REQ_INV_13：OnRequest／OnResponse 各觸發一次、參數
// 正確（method/url/status/耗時>0/Set-Cookie 名稱清單）；OnRequest 與
// OnResponse 的 panic 均被吸收，請求仍成功回傳。
func TestObserveHooks_REQ_INV_13(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", "a=1; Path=/")
		w.Header().Add("Set-Cookie", "b=2; Path=/")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	var mu sync.Mutex
	var reqCalls int
	var reqMethod, reqURL string
	var respCalls int
	var respMethod, respURL string
	var respStatus int
	var respElapsed time.Duration
	var respCookies []string

	c := newTestClient(t, func(o *Options) {
		o.OnRequest = func(method, url string) {
			mu.Lock()
			defer mu.Unlock()
			reqCalls++
			reqMethod, reqURL = method, url
		}
		o.OnResponse = func(method, url string, status int, elapsed time.Duration, setCookieNames []string) {
			mu.Lock()
			defer mu.Unlock()
			respCalls++
			respMethod, respURL, respStatus, respElapsed = method, url, status, elapsed
			respCookies = append([]string(nil), setCookieNames...)
		}
	})

	target := srv.URL + "/observe"
	resp, err := c.Do(newRequest(t, fhttp.MethodGet, target))
	if err != nil {
		t.Fatalf("Do 失敗：%v", err)
	}
	defer resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()
	if reqCalls != 1 {
		t.Errorf("OnRequest 應觸發 1 次，得到 %d", reqCalls)
	}
	if reqMethod != fhttp.MethodGet || reqURL != target {
		t.Errorf("OnRequest 參數錯誤：method=%q url=%q，期望 %q %q", reqMethod, reqURL, fhttp.MethodGet, target)
	}
	if respCalls != 1 {
		t.Errorf("OnResponse 應觸發 1 次，得到 %d", respCalls)
	}
	if respMethod != fhttp.MethodGet || respURL != target {
		t.Errorf("OnResponse method/url 錯誤：method=%q url=%q", respMethod, respURL)
	}
	if respStatus != http.StatusOK {
		t.Errorf("OnResponse status = %d，期望 %d", respStatus, http.StatusOK)
	}
	if respElapsed <= 0 {
		t.Errorf("OnResponse 耗時應 > 0，得到 %v", respElapsed)
	}
	if len(respCookies) != 2 || respCookies[0] != "a" || respCookies[1] != "b" {
		t.Errorf("OnResponse Set-Cookie 名稱清單 = %v，期望 [a b]", respCookies)
	}
}

// TestObserveHooks_PanicAbsorbed_REQ_INV_13：兩鉤子的函式 body panic
// 皆以 recover 吸收，Do 仍成功回傳回應（鉤子不得中斷主流程）。
func TestObserveHooks_PanicAbsorbed_REQ_INV_13(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, func(o *Options) {
		o.OnRequest = func(method, url string) { panic("onrequest 爆炸") }
		o.OnResponse = func(method, url string, status int, elapsed time.Duration, setCookieNames []string) {
			panic("onresponse 爆炸")
		}
	})

	resp, err := c.Do(newRequest(t, fhttp.MethodGet, srv.URL+"/panic"))
	if err != nil {
		t.Fatalf("鉤子 panic 應被吸收，Do 不應失敗：%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("鉤子 panic 後請求仍應成功，得到 status %d", resp.StatusCode)
	}
}
