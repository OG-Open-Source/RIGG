package transport

// transport 測試：以本地 CONNECT 中繼代理＋httptest／raw 捕捉伺服器驗證
// REQ-INV-02、REQ-TRN-012/013/014/015/035/040 與 REQ-HDR-002/013。
// 註：本機無 C 工具鏈，以純 go test 執行（-race 延後）。

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/fingerprint"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// startProxyRelay 啟動測試用 CONNECT 中繼代理：對每個 CONNECT target
// 回 200 Connection established 後，於客戶端與目標之間雙向拷貝位元組。
// transport 的 fail-closed 約束要求每個 Client 必須走代理，本地測試目標
// 亦需經此中繼到達。
func startProxyRelay(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("代理監聽失敗：%v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go relayTunnel(conn)
		}
	}()
	return "http://" + ln.Addr().String()
}

// relayTunnel 處理單一 CONNECT：讀完請求頭、撥接目標、雙向中繼。
func relayTunnel(client net.Conn) {
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(30 * time.Second))
	br := bufio.NewReader(client)
	reqLine, err := br.ReadString('\n')
	if err != nil {
		return
	}
	if !strings.HasPrefix(reqLine, "CONNECT ") {
		return
	}
	target := strings.Fields(reqLine)[1]
	for {
		l, err := br.ReadString('\n')
		if err != nil || l == "\r\n" || l == "\n" {
			break
		}
	}
	upstream, err := net.Dial("tcp", target)
	if err != nil {
		_, _ = client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer upstream.Close()
	_ = upstream.SetDeadline(time.Now().Add(30 * time.Second))
	_, _ = client.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, br); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, upstream); done <- struct{}{} }()
	<-done
}

// startCaptureServer 啟動 raw TCP 捕捉伺服器：讀到 CRLFCRLF 即回 200，
// 並記錄完整請求頭原始位元組（保留 wire 順序與原樣大小寫——httptest 的
// http.Server 會 canonicalize，無法驗證接收序與大小寫）。
func startCaptureServer(t *testing.T) (baseURL string, captured func() [][]byte) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("捕捉伺服器監聽失敗：%v", err)
	}
	t.Cleanup(func() { ln.Close() })
	var mu sync.Mutex
	var heads [][]byte
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(15 * time.Second))
				br := bufio.NewReader(c)
				var head bytes.Buffer
				for {
					line, err := br.ReadString('\n')
					if err != nil {
						return
					}
					head.WriteString(line)
					if line == "\r\n" {
						break
					}
				}
				mu.Lock()
				heads = append(heads, append([]byte(nil), head.Bytes()...))
				mu.Unlock()
				// 固定附帶 Set-Cookie：jar 若未被中和，下一請求即被注入
				// jarinject——供 TestCookieOrderWithJarNeutralized 驗證。
				_, _ = c.Write([]byte("HTTP/1.1 200 OK\r\nSet-Cookie: jarinject=9; Path=/\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok"))
			}(conn)
		}
	}()
	return "http://" + ln.Addr().String(), func() [][]byte {
		mu.Lock()
		defer mu.Unlock()
		return append([][]byte(nil), heads...)
	}
}

// newTestClient 以獨立代理建構受測 Client，並註冊關閉清理。
func newTestClient(t *testing.T, mutate func(*Options)) *Client {
	t.Helper()
	opts := Options{ProxyURL: startProxyRelay(t)}
	if mutate != nil {
		mutate(&opts)
	}
	c, err := New(fingerprint.Get(), opts)
	if err != nil {
		t.Fatalf("New 失敗：%v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// newRequest 建構 fhttp 請求。
func newRequest(t *testing.T, method, url string) *fhttp.Request {
	t.Helper()
	req, err := fhttp.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("建構請求失敗：%v", err)
	}
	return req
}

// TestNew_ProxyMissingFailClosed_REQ_INV_02：ProxyURL 為空 → New 回
// *xerrors.TransportError（errors.As 可驗證）且訊息含「代理」語義；
// 不得存在直連路徑（fail-closed，REQ-INV-02/REQ-TRN-018）。
func TestNew_ProxyMissingFailClosed_REQ_INV_02(t *testing.T) {
	c, err := New(fingerprint.Get(), Options{ProxyURL: ""})
	if err == nil {
		t.Fatal("New 於無代理時應回錯（fail-closed），卻成功")
	}
	if c != nil {
		t.Error("New 失敗時 Client 應為 nil")
	}
	var te *xerrors.TransportError
	if !errors.As(err, &te) {
		t.Fatalf("錯誤應可 errors.As 為 *xerrors.TransportError，得到 %T：%v", err, err)
	}
	if !strings.Contains(te.Error(), "代理") {
		t.Errorf("錯誤訊息應含「代理」語義，得到：%s", te.Error())
	}
	if te.Kind() != xerrors.KindTransport || !te.Retryable() || te.CredentialImpacting() {
		t.Errorf("分類應為 transport/可重試/不牽連憑證，得到 kind=%s retryable=%v cred=%v",
			te.Kind(), te.Retryable(), te.CredentialImpacting())
	}
}

// TestNew_AllowDirectEscapeHatch_OQ12：AllowDirect=true 時空 ProxyURL 可
// 建構，且請求直接達目標（不打任何代理即可連線）；ProxyURL 非空時本欄位
// 不生效（仍走代理）。預設值恆為 false——空代理仍 fail-closed，由
// TestNew_ProxyMissingFailClosed_REQ_INV_02 涵蓋。此例外僅供「使用者明確
// 指示直連」的情境（DECISIONS OQ12；L2 煙霧），正式 app 路徑不得設置。
func TestNew_AllowDirectEscapeHatch_OQ12(t *testing.T) {
	c, err := New(fingerprint.Get(), Options{AllowDirect: true})
	if err != nil {
		t.Fatalf("AllowDirect=true 時空代理應可建構（OQ12 例外），得到錯誤：%v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	resp, err := c.Do(newRequest(t, "GET", srv.URL))
	if err != nil {
		t.Fatalf("直連請求應成功：%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("直連應回 200，得到 %d", resp.StatusCode)
	}
}

// TestDefaultHeadersCompletion_REQ_TRN_012：呼叫端僅帶 x-csrf-token →
// 伺服器端收到全部七個預設標頭（逐字）；呼叫端已設 user-agent 時
// 不被覆蓋。
func TestDefaultHeadersCompletion_REQ_TRN_012(t *testing.T) {
	var mu sync.Mutex
	var got []http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.Header.Clone())
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, nil)
	fp := fingerprint.Get()

	// 請求一：僅 x-csrf-token。
	req := newRequest(t, fhttp.MethodGet, srv.URL+"/probe")
	req.Header["x-csrf-token"] = []string{"csrf-value"}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do 失敗：%v", err)
	}
	_ = resp.Body.Close()

	// 請求二：呼叫端已設 user-agent。
	req2 := newRequest(t, fhttp.MethodGet, srv.URL+"/probe2")
	req2.Header["user-agent"] = []string{"test-ua"}
	resp2, err := c.Do(req2)
	if err != nil {
		t.Fatalf("Do(2) 失敗：%v", err)
	}
	_ = resp2.Body.Close()

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("伺服器應收到 2 個請求，得到 %d", len(got))
	}
	h := got[0]
	for _, tc := range []struct{ name, want string }{
		{"User-Agent", fp.UA},
		{"Sec-Ch-Ua", fp.SecChUA},
		{"Sec-Ch-Ua-Mobile", fp.SecChUAMobile},
		{"Sec-Ch-Ua-Platform", fp.SecChUAPlatform},
		{"Accept", "*/*"},
		{"Accept-Language", fp.AcceptLanguage},
		{"Priority", "u=1, i"},
	} {
		if gotVal := h.Get(tc.name); gotVal != tc.want {
			t.Errorf("預設標頭 %s = %q，期望逐字 %q", tc.name, gotVal, tc.want)
		}
	}
	if gotVal := h.Get("X-Csrf-Token"); gotVal != "csrf-value" {
		t.Errorf("呼叫端標頭 x-csrf-token 應保留，得到 %q", gotVal)
	}

	if gotVal := got[1].Get("User-Agent"); gotVal != "test-ua" {
		t.Errorf("呼叫端已設 user-agent 時不得覆蓋，得到 %q，期望 %q", gotVal, "test-ua")
	}
}

// TestStripNavigationHeaders_REQ_TRN_013：導覽殘留標頭
// upgrade-insecure-requests、sec-fetch-user 一律移除（兩種存入大小寫皆測）。
func TestStripNavigationHeaders_REQ_TRN_013(t *testing.T) {
	var mu sync.Mutex
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = r.Header.Clone()
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, nil)
	req := newRequest(t, fhttp.MethodGet, srv.URL+"/nav")
	req.Header.Set("Upgrade-Insecure-Requests", "1") // canonical 大小寫
	req.Header["sec-fetch-user"] = []string{"?1"}    // 原樣小寫
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do 失敗：%v", err)
	}
	_ = resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()
	if gotVal := got.Get("Upgrade-Insecure-Requests"); gotVal != "" {
		t.Errorf("upgrade-insecure-requests 應被移除，伺服器收到 %q", gotVal)
	}
	if gotVal := got.Get("Sec-Fetch-User"); gotVal != "" {
		t.Errorf("sec-fetch-user 應被移除，伺服器收到 %q", gotVal)
	}
}

// TestSecFetchSite_REQ_TRN_015：sec-fetch-site 依目標 host 推導；
// sec-fetch-mode: cors 與 sec-fetch-dest: empty 恆存在。host 以 req.Host
// 覆寫技巧注入（httptest 伺服器仍為本地位址）。
func TestSecFetchSite_REQ_TRN_015(t *testing.T) {
	cases := []struct{ host, wantSite string }{
		{"x.com", "same-origin"},
		{"www.x.com", "same-origin"},
		{"api.x.com", "same-site"},
		{"twitter.com", "same-site"},
		{"sub.twitter.com", "same-site"},
		{"example.com", "cross-site"},
	}
	var mu sync.Mutex
	gotSites := make(map[string]string, len(cases))
	gotHosts := make(map[string]string, len(cases))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotSites[r.Host] = r.Header.Get("Sec-Fetch-Site")
		gotHosts[r.Host] = r.Header.Get("Sec-Fetch-Mode") + "|" + r.Header.Get("Sec-Fetch-Dest")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, nil)
	for i, tc := range cases {
		req := newRequest(t, fhttp.MethodGet, srv.URL+"/site")
		req.Host = tc.host
		resp, err := c.Do(req)
		if err != nil {
			t.Fatalf("第 %d 個請求（%s）Do 失敗：%v", i, tc.host, err)
		}
		_ = resp.Body.Close()
	}
	mu.Lock()
	defer mu.Unlock()
	for _, tc := range cases {
		if gotHosts[tc.host] == "" {
			t.Errorf("host %s 未被伺服器收到（Host 覆寫未生效？）", tc.host)
			continue
		}
		if site := gotSites[tc.host]; site != tc.wantSite {
			t.Errorf("host %s 的 sec-fetch-site = %q，期望 %q", tc.host, site, tc.wantSite)
		}
		if pair := gotHosts[tc.host]; pair != "cors|empty" {
			t.Errorf("host %s 的 sec-fetch-mode|dest = %q，期望 cors|empty", tc.host, pair)
		}
	}
}

// TestNoRedirect_REQ_TRN_040：/a 回 302 → /b；Do 回傳 302 回應本身，
// /b 零命中（一律不跟隨重定向）。
func TestNoRedirect_REQ_TRN_040(t *testing.T) {
	var hitsB int32
	var mu sync.Mutex
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/b", http.StatusFound)
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hitsB++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := newTestClient(t, nil)
	resp, err := c.Do(newRequest(t, fhttp.MethodGet, srv.URL+"/a"))
	if err != nil {
		t.Fatalf("Do 失敗：%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("應回傳 302 回應本身，得到 %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/b" {
		t.Errorf("Location 應為 /b，得到 %q", loc)
	}
	mu.Lock()
	defer mu.Unlock()
	if hitsB != 0 {
		t.Errorf("/b 不應被跟隨命中，得到 %d 次", hitsB)
	}
}

// TestHeaderOrderPreserved_REQ_HDR_002_013：呼叫端以 Header-Order 宣告
// 特定順序的三個自訂大小寫標頭 → wire 順序與原樣大小寫一致（raw 捕捉
// 斷言接收序與大小寫），且 Header-Order 值雙重印證相對順序、不含自引用。
func TestHeaderOrderPreserved_REQ_HDR_002_013(t *testing.T) {
	base, captured := startCaptureServer(t)
	c := newTestClient(t, nil)

	req := newRequest(t, fhttp.MethodGet, base+"/order")
	req.Header["X-Custom-B"] = []string{"b-value"}
	req.Header["accept"] = []string{"application/json"} // 呼叫端已設的預設名
	req.Header["X-Custom-a"] = []string{"a-value"}
	req.Header[fhttp.HeaderOrderKey] = []string{"X-Custom-B", "accept", "X-Custom-a"}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do 失敗：%v", err)
	}
	_ = resp.Body.Close()

	// 斷言一：Header-Order 值（附加於請求上）相對順序正確且不含自引用。
	declared := req.Header[fhttp.HeaderOrderKey]
	idx := func(name string) int {
		for i, n := range declared {
			if n == name {
				return i
			}
		}
		return -1
	}
	if idx("x-custom-b") < 0 || idx("accept") < 0 || idx("x-custom-a") < 0 {
		t.Fatalf("Header-Order 值應含三個自訂標頭，得到 %v", declared)
	}
	if !(idx("x-custom-b") < idx("accept") && idx("accept") < idx("x-custom-a")) {
		t.Errorf("Header-Order 相對順序錯誤，得到 %v", declared)
	}
	for _, n := range declared {
		if n == "header-order:" || n == "pheader-order:" {
			t.Errorf("Header-Order 值不得自引用，得到 %v", declared)
		}
		if n != strings.ToLower(n) {
			t.Errorf("Header-Order 值應全小寫，發現 %q", n)
		}
	}

	// 斷言二：wire 接收序與原樣大小寫（raw 位元組）。
	heads := captured()
	if len(heads) == 0 {
		t.Fatal("捕捉伺服器未收到請求")
	}
	lines := strings.Split(string(heads[0]), "\r\n")
	pos := map[string]int{}
	for i, l := range lines {
		for _, name := range []string{"X-Custom-B:", "accept:", "X-Custom-a:"} {
			if strings.HasPrefix(l, name) {
				pos[name] = i
			}
		}
	}
	for _, name := range []string{"X-Custom-B:", "accept:", "X-Custom-a:"} {
		if _, ok := pos[name]; !ok {
			t.Fatalf("wire 未以原樣大小寫寫出 %s；捕捉：%q", name, heads[0])
		}
	}
	if !(pos["X-Custom-B:"] < pos["accept:"] && pos["accept:"] < pos["X-Custom-a:"]) {
		t.Errorf("wire 順序應為 X-Custom-B → accept → X-Custom-a，得到位置 %v；捕捉：%q", pos, heads[0])
	}
	if !strings.Contains(string(heads[0]), "accept: application/json") {
		t.Errorf("呼叫端已設的 accept 值不得被覆蓋；捕捉：%q", heads[0])
	}
}

// TestMalformedProxy_REQ_INV_02：代理位址格式非法 → New 回
// *xerrors.TransportError（錯誤路徑不得 panic，更不得靜默直連）。
func TestMalformedProxy_REQ_INV_02(t *testing.T) {
	for _, bad := range []string{"not a url", "http://", "127.0.0.1:9099", "ftp://nope.example"} {
		c, err := New(fingerprint.Get(), Options{ProxyURL: bad})
		if err == nil {
			t.Fatalf("代理 %q 應於 New 被拒（fail-closed），卻成功", bad)
		}
		if c != nil {
			t.Errorf("代理 %q 拒絕時 Client 應為 nil", bad)
		}
		var te *xerrors.TransportError
		if !errors.As(err, &te) {
			t.Fatalf("代理 %q 的錯誤應可 errors.As 為 *xerrors.TransportError，得到 %T：%v", bad, err, err)
		}
		if !strings.Contains(te.Error(), "代理") {
			t.Errorf("代理 %q 的錯誤訊息應含「代理」語義，得到：%s", bad, te.Error())
		}
	}
}

// TestCookieOrderWithJarNeutralized：jar 以 no-op 中和，Cookie 標頭為呼叫
// 端自有（REQ-AUTH-001：XAuth 為唯一憑證持有者；transport 無 cookie 狀態）。
// 前一回應的 Set-Cookie 不得於後續請求被 jar 注入；呼叫端自設的 cookie 在
// wire 上位於 priority 之前（Chrome 觀察序），且 Header-Order 清單含
// cookie 於 priority 前。
func TestCookieOrderWithJarNeutralized(t *testing.T) {
	base, captured := startCaptureServer(t)
	c := newTestClient(t, nil)

	// 請求一（播種）：捕捉伺服器固定回 Set-Cookie: jarinject=9——jar 若
	// 未中和，此 cookie 會被存下並注入後續請求。
	resp, err := c.Do(newRequest(t, fhttp.MethodGet, base+"/seed"))
	if err != nil {
		t.Fatalf("Do(seed) 失敗：%v", err)
	}
	_ = resp.Body.Close()

	// 請求二：呼叫端自設 Cookie（未來 Sender 自 XAuth 注入）。
	req := newRequest(t, fhttp.MethodGet, base+"/call")
	req.Header["Cookie"] = []string{"a=1"}
	resp2, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do(call) 失敗：%v", err)
	}
	_ = resp2.Body.Close()

	heads := captured()
	if len(heads) < 2 {
		t.Fatalf("捕捉伺服器應收到 2 個請求，得到 %d", len(heads))
	}

	// 斷言一：jar 未注入——收到的 cookie 恰為呼叫端設的值。
	wire2 := string(heads[1])
	if strings.Contains(strings.ToLower(wire2), "jarinject") {
		t.Errorf("jar 應被中和（不得注入第二份 cookie）；wire：%q", wire2)
	}
	lines2 := strings.Split(wire2, "\r\n")
	cookieLine := ""
	for _, l := range lines2 {
		if strings.HasPrefix(strings.ToLower(l), "cookie:") {
			cookieLine = l
			break
		}
	}
	if cookieLine == "" {
		t.Fatalf("wire 應含 cookie 標頭；捕捉：%q", wire2)
	}
	if !strings.EqualFold(cookieLine, "Cookie: a=1") {
		t.Errorf("收到的 cookie 應恰為呼叫端值，得到 %q", cookieLine)
	}

	// 斷言二：wire 上 cookie 在 priority 之前。
	prioLine := -1
	for i, l := range lines2 {
		if strings.HasPrefix(strings.ToLower(l), "priority:") {
			prioLine = i
			break
		}
	}
	cookieIdx := -1
	for i, l := range lines2 {
		if strings.HasPrefix(strings.ToLower(l), "cookie:") {
			cookieIdx = i
			break
		}
	}
	if prioLine < 0 {
		t.Fatalf("wire 應含 priority 標頭（transport 預設補齊）；捕捉：%q", wire2)
	}
	if !(cookieIdx >= 0 && cookieIdx < prioLine) {
		t.Errorf("cookie 應在 priority 之前（Chrome 觀察序）；捕捉：%q", wire2)
	}

	// 斷言三：Header-Order 清單含 cookie 且在 priority 前。
	declared := req.Header[fhttp.HeaderOrderKey]
	idx := func(name string) int {
		for i, n := range declared {
			if n == name {
				return i
			}
		}
		return -1
	}
	if idx("cookie") < 0 || idx("priority") < 0 {
		t.Fatalf("Header-Order 應含 cookie 與 priority，得到 %v", declared)
	}
	if !(idx("cookie") < idx("priority")) {
		t.Errorf("Header-Order 中 cookie 應在 priority 之前，得到 %v", declared)
	}
}

// TestTimeout_REQ_TRN_035：Timeout 300ms、伺服器延遲 2s → Do 回逾時
// 錯誤，且 errors.As 為 *xerrors.TransportError（分類不因包裝而失真）。
func TestTimeout_REQ_TRN_035(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 客戶端逾時斷線時，context 取消使 handler 提前返回，測試不留尾。
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, func(o *Options) { o.Timeout = 300 * time.Millisecond })
	start := time.Now()
	_, err := c.Do(newRequest(t, fhttp.MethodGet, srv.URL+"/slow"))
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("逾時後 Do 應回錯")
	}
	if elapsed >= 2*time.Second {
		t.Errorf("逾時應由客戶端於 300ms 附近觸發，實際耗時 %v", elapsed)
	}
	var te *xerrors.TransportError
	if !errors.As(err, &te) {
		t.Fatalf("逾時錯誤應可 errors.As 為 *xerrors.TransportError，得到 %T：%v", err, err)
	}
	if te.Kind() != xerrors.KindTransport || !te.Retryable() {
		t.Errorf("逾時分類應為 transport/可重試，得到 kind=%s retryable=%v", te.Kind(), te.Retryable())
	}
}
