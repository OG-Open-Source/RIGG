package api

// api 測試：以本地 CONNECT 中繼代理＋raw 捕捉伺服器（基建模式複製自
// Task 5 transport/client_test.go——測試 helper 不得跨套件 import）驗證
// Sender 的設計 §3.4 七步流程：wire 鍵序（REQ-GQL-001/003/024）、讀取
// 語義（REQ-GQL-006/007）、錯誤分類（REQ-RD-001..003；清除決策在 app 層
// REQ-LOG-042/043）、16 MiB 上限（REQ-INV-14）與 Set-Cookie write-through
//（REQ-AUTH-014）。transport 代理必配（fail-closed），本地目標一律經
// 中繼到達；Sender 的目標 host 經同套件白盒欄位 baseOverride 指向捕捉
// 伺服器（op.URL() 恆為 https://x.com，本地無法滿足 TLS 驗證）。

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/fingerprint"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/graphql"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/transport"
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

// capturedRequest 是捕捉伺服器記錄的單一請求：raw head（wire 原樣序與
// 大小寫）＋raw body（POST 鍵序驗證用）。
type capturedRequest struct {
	head []byte
	body []byte
}

// requestLine 回請求首行（如 "GET /i/api/graphql/QID/Test?... HTTP/1.1"）。
func (c capturedRequest) requestLine() string {
	if i := bytes.IndexByte(c.head, '\n'); i >= 0 {
		return strings.TrimRight(string(c.head[:i]), "\r\n")
	}
	return string(c.head)
}

// headerValue 自 raw head 取單一標頭值（不分大小寫比對名稱、跳過請求行；
// 多值取首個）。
func (c capturedRequest) headerValue(name string) string {
	prefix := strings.ToLower(name) + ":"
	for i, line := range strings.Split(string(c.head), "\r\n") {
		if i == 0 {
			continue // 請求行
		}
		if strings.HasPrefix(strings.ToLower(line), prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	return ""
}

// query 回請求目標的 query string（不含 "?"）。
func (c capturedRequest) query() string {
	parts := strings.Fields(c.requestLine())
	if len(parts) < 2 {
		return ""
	}
	if i := strings.IndexByte(parts[1], '?'); i >= 0 {
		return parts[1][i+1:]
	}
	return ""
}

// bodyKeys 以 JSON decoder token 流取 body 物件的鍵序（wire 原樣順序，
// 不經任何重新序列化）。
func (c capturedRequest) bodyKeys(t *testing.T) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(c.body))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("body 非 JSON：%v（body=%q）", err, c.body)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		t.Fatalf("body 頂層應為 JSON 物件，得到 %v", tok)
	}
	var keys []string
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			t.Fatalf("讀取 body 鍵失敗：%v", err)
		}
		keys = append(keys, kt.(string))
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			t.Fatalf("跳過 body 值失敗：%v", err)
		}
	}
	return keys
}

// cookieParam 自單行 cookie 字串取指定參數值（k=v 以 ; 分段）。
func cookieParam(t *testing.T, line, name string) string {
	t.Helper()
	for _, seg := range strings.Split(line, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(seg), "=")
		if ok && k == name {
			return v
		}
	}
	return ""
}

// readBodyByContentLength 依 head 內的 Content-Length 讀取請求 body。
func readBodyByContentLength(br *bufio.Reader, head []byte) []byte {
	n := int64(0)
	for _, line := range strings.Split(string(head), "\r\n") {
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			if v, err := strconv.ParseInt(strings.TrimSpace(line[len("content-length:"):]), 10, 64); err == nil {
				n = v
			}
		}
	}
	if n <= 0 {
		return nil
	}
	buf := make([]byte, n)
	_, _ = io.ReadFull(br, buf)
	return buf
}

// rawResponse 組 raw HTTP/1.1 回應位元組（Content-Length 恆計算正確、
// Connection: close——捕捉伺服器一連線一請求）。
func rawResponse(status int, headers []string, body []byte) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "HTTP/1.1 %d %s\r\n", status, http.StatusText(status))
	for _, h := range headers {
		b.WriteString(h)
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "Content-Length: %d\r\nConnection: close\r\n\r\n", len(body))
	b.Write(body)
	return b.Bytes()
}

// jsonResponse 組 JSON 回應；setCookies 逐筆輸出 Set-Cookie 標頭。
func jsonResponse(status int, body string, setCookies ...string) []byte {
	headers := make([]string, 0, len(setCookies)+1)
	headers = append(headers, "Content-Type: application/json")
	for _, sc := range setCookies {
		headers = append(headers, "Set-Cookie: "+sc)
	}
	return rawResponse(status, headers, []byte(body))
}

var defaultJSONResponse = jsonResponse(http.StatusOK, `{"data":{}}`)

// newCaptureServer 啟動 raw TCP 捕捉伺服器：逐連線讀 head（至 CRLFCRLF）
// 與 body（依 Content-Length），記錄後以 respond(seq, head, body) 產生的
// raw 回應位元組答覆。respond 為 nil 時回 200 + {"data":{}}。
func newCaptureServer(t *testing.T, respond func(seq int, head, body []byte) []byte) (baseURL string, captured func() []capturedRequest) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("捕捉伺服器監聽失敗：%v", err)
	}
	t.Cleanup(func() { ln.Close() })
	var mu sync.Mutex
	var reqs []capturedRequest
	go func() {
		seq := 0
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			seq++
			n := seq
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(30 * time.Second))
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
				body := readBodyByContentLength(br, head.Bytes())
				mu.Lock()
				reqs = append(reqs, capturedRequest{
					head: append([]byte(nil), head.Bytes()...),
					body: append([]byte(nil), body...),
				})
				mu.Unlock()
				resp := defaultJSONResponse
				if respond != nil {
					resp = respond(n, head.Bytes(), body)
				}
				_, _ = c.Write(resp)
			}(conn)
		}
	}()
	return "http://" + ln.Addr().String(), func() []capturedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]capturedRequest(nil), reqs...)
	}
}

// sendStore 是 CookieStore 的測試替身：記錄每次 Save 的行（write-through
// 觀測點；另用以斷言引擎不清除 cookie——非 2xx／逾時路徑 Save 不得被呼
// 叫，REQ-LOG-042/043）。
type sendStore struct {
	mu    sync.Mutex
	saves []string
}

var _ auth.CookieStore = (*sendStore)(nil)

func (s *sendStore) Load(ctx context.Context) (string, error) { return "", nil }

func (s *sendStore) Save(ctx context.Context, cookieLine string) error {
	s.mu.Lock()
	s.saves = append(s.saves, cookieLine)
	s.mu.Unlock()
	return nil
}

func (s *sendStore) saveCalls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.saves...)
}

// testRegistryJSON 是本套件的合成註冊表 fixture（測試專用，非 X 原文）：
// TestQuery 宣告 featureSwitches＋fieldToggles（驗證 GET 鍵序與在場）、
// TestPlain／TestMutation 無宣告（驗證 features 整鍵省略；mutation 推導
// POST 方法，REQ-GQL-001/003）。
const testRegistryJSON = `{"generatedAt":"2026-11-12T00:00:00+08:00","frontendSha":null,
 "operations":[
  {"operationName":"TestQuery","operationType":"query","queryId":"QIDGET","featureSwitches":["withTestFeature"],"fieldToggles":["withTestToggle"]},
  {"operationName":"TestPlain","operationType":"query","queryId":"QIDPLAIN","featureSwitches":[],"fieldToggles":[]},
  {"operationName":"TestMutation","operationType":"mutation","queryId":"QIDPOST","featureSwitches":[],"fieldToggles":[]}],
 "featureDefaults":{"withTestFeature":true,"withTestToggle":false}}`

// malformedRegistryJSON 與 testRegistryJSON 同構，僅 TestMutation 的
// queryId 內嵌控制字元（JSON \n 轉義→解析後為換行）——模擬損毀的
// static/graphql.json（REQ-GQL-014 的資料異常情境）。控制字元使
// fhttp.NewRequest 的 URL 解析失敗：註定 req=nil 的路徑，修復前 POST
// 分支的 err 蔭蔽會讓 nil req 進入標頭迴圈而 panic。
// （註：路徑含空白時 NewRequest 不失敗——fhttp 逕行 %20 轉義——故本
// 情境以控制字元觸發 URL 組裝失敗。）
const malformedRegistryJSON = `{"generatedAt":"2026-11-12T00:00:00+08:00","frontendSha":null,
 "operations":[
  {"operationName":"TestMutation","operationType":"mutation","queryId":"QID\nPOST","featureSwitches":[],"fieldToggles":[]}],
 "featureDefaults":{}}`

// loadTestRegistry 將合成 fixture 寫入暫存檔並載入（走真檔案路徑）。
func loadTestRegistry(t *testing.T) *graphql.Registry {
	t.Helper()
	return loadRegistryFromJSON(t, testRegistryJSON)
}

// loadRegistryFromJSON 將提供的註冊表 JSON 寫入暫存檔並載入（走真檔案
// 路徑，與生產 LoadRegistry 同一入口）。
func loadRegistryFromJSON(t *testing.T, raw string) *graphql.Registry {
	t.Helper()
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("寫入測試註冊表：%v", err)
	}
	reg, err := graphql.LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry error = %v, 期望 nil", err)
	}
	return reg
}

// sendFixture 聚集一個 Send 測試的全部接線：本地代理、捕捉伺服器、
// transport、XAuth（接線 fake store 並 PrepareAuth 播種 cookie）、註冊表
// 與受測 Sender（baseOverride 白盒指向捕捉伺服器）。
type sendFixture struct {
	base     string
	auth     *auth.Auth
	store    *sendStore
	reg      *graphql.Registry
	sender   *Sender
	captured func() []capturedRequest
}

// newSendFixture 建構上列接線；mutate 允許測試調整 transport Options
// （如 Timeout）。auth 固定以 auth_token＋ct0 播種（多數測試需要非空
// cookie 行以驗證 ct0 → x-csrf-token 同代）。
func newSendFixture(t *testing.T, respond func(seq int, head, body []byte) []byte, mutate func(*transport.Options)) *sendFixture {
	t.Helper()
	base, captured := newCaptureServer(t, respond)
	opts := transport.Options{ProxyURL: startProxyRelay(t)}
	if mutate != nil {
		mutate(&opts)
	}
	c, err := transport.New(fingerprint.Get(), opts)
	if err != nil {
		t.Fatalf("transport.New 失敗：%v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	store := &sendStore{}
	a := auth.NewAuth("bearer-send-test", auth.WithCookieStore(store))
	if err := a.PrepareAuth("auth_token=TOKEN123; ct0=CT0VALUE456"); err != nil {
		t.Fatalf("PrepareAuth 失敗：%v", err)
	}
	reg := loadTestRegistry(t)
	s := NewSender(c, a, reg, fingerprint.Get())
	s.baseOverride = base // 白盒測試注入：目標指向本地捕捉伺服器
	return &sendFixture{base: base, auth: a, store: store, reg: reg, sender: s, captured: captured}
}

// TestSend_QueryGET_REQ_RD_001：op.Method=GET → 伺服器收到 GET、query
// string 以 variables= 起始且鍵序 variables→features→fieldToggles
// （REQ-GQL-001）、POST 族標頭（content-type: application/json、
// authorization Bearer）在、cookie 標頭存在、x-csrf-token 與 cookie 行內
// ct0 同代逐字一致（Task 6 caller rule 的線上驗證，REQ-HDR-008）。
func TestSend_QueryGET_REQ_RD_001(t *testing.T) {
	fx := newSendFixture(t, nil, nil)
	op, err := fx.reg.Operation("TestQuery")
	if err != nil {
		t.Fatalf("Operation(TestQuery) error = %v, 期望 nil", err)
	}
	data, err := fx.sender.Send(op, graphql.OrderedParams{{K: "count", V: "20"}}, "https://x.com/home")
	if err != nil {
		t.Fatalf("Send error = %v, 期望 nil", err)
	}
	if string(data) != "{}" {
		t.Errorf("data = %s, 期望 {}", data)
	}

	reqs := fx.captured()
	if len(reqs) != 1 {
		t.Fatalf("捕捉伺服器應收到 1 個請求，得到 %d", len(reqs))
	}
	req := reqs[0]
	line := req.requestLine()
	if !strings.HasPrefix(line, "GET /i/api/graphql/QIDGET/TestQuery?") {
		t.Fatalf("應以 GET 送出至 op.Path()，請求行：%s", line)
	}
	q := req.query()
	if !strings.HasPrefix(q, "variables=") {
		t.Errorf("query string 應以 variables= 起始（REQ-GQL-001），得到：%s", q)
	}
	iVar, iFeat, iTog := strings.Index(q, "variables="), strings.Index(q, "features="), strings.Index(q, "fieldToggles=")
	if iVar < 0 || iFeat < 0 || iTog < 0 || !(iVar < iFeat && iFeat < iTog) {
		t.Errorf("query 鍵序應為 variables→features→fieldToggles（REQ-GQL-001），得到：%s", q)
	}
	if got := req.headerValue("content-type"); got != "application/json" {
		t.Errorf("content-type 應為 application/json（POST 族標頭＋GET 方法，REQ-RD-001），得到 %q", got)
	}
	if got := req.headerValue("authorization"); got != "Bearer bearer-send-test" {
		t.Errorf("authorization 應為 Bearer 值（POST 族標頭），得到 %q", got)
	}
	cookieLine := req.headerValue("cookie")
	if cookieLine == "" {
		t.Fatalf("wire 應含 cookie 標頭；head：%q", req.head)
	}
	ct0 := cookieParam(t, cookieLine, "ct0")
	if ct0 != "CT0VALUE456" {
		t.Fatalf("cookie 行內 ct0 = %q, 期望 CT0VALUE456（行：%q）", ct0, cookieLine)
	}
	if got := req.headerValue("x-csrf-token"); got != ct0 {
		t.Errorf("x-csrf-token = %q，應與同一 cookie 行內 ct0 同代逐字一致（REQ-HDR-008 caller rule）", got)
	}
}

// TestSend_POSTBodyKeyOrder_REQ_GQL_003：op.Method=POST（mutation 推導）→
// body 鍵序恰 variables→queryId（features/fieldToggles 空時整鍵省略）；
// variables 為緊湊 JSON、queryId 為 op.QueryID 的 JSON 字串。
func TestSend_POSTBodyKeyOrder_REQ_GQL_003(t *testing.T) {
	fx := newSendFixture(t, nil, nil)
	op, err := fx.reg.Operation("TestMutation")
	if err != nil {
		t.Fatalf("Operation(TestMutation) error = %v, 期望 nil", err)
	}
	if _, err := fx.sender.Send(op, graphql.OrderedParams{{K: "tweet_id", V: `"123"`}}, ""); err != nil {
		t.Fatalf("Send error = %v, 期望 nil", err)
	}

	reqs := fx.captured()
	if len(reqs) != 1 {
		t.Fatalf("捕捉伺服器應收到 1 個請求，得到 %d", len(reqs))
	}
	req := reqs[0]
	if !strings.HasPrefix(req.requestLine(), "POST /i/api/graphql/QIDPOST/TestMutation") {
		t.Fatalf("應以 POST 送出至 op.URL()，請求行：%s", req.requestLine())
	}
	keys := req.bodyKeys(t)
	want := []string{"variables", "queryId"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("body 鍵序 = %v, 期望 %v（REQ-GQL-003：features/fieldToggles 空時省略）", keys, want)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(req.body, &parsed); err != nil {
		t.Fatalf("body 解析失敗：%v（body=%q）", err, req.body)
	}
	if string(parsed["variables"]) != `{"tweet_id":"123"}` {
		t.Errorf("variables = %s, 期望緊湊 JSON {\"tweet_id\":\"123\"}", parsed["variables"])
	}
	if string(parsed["queryId"]) != `"QIDPOST"` {
		t.Errorf("queryId = %s, 期望 \"QIDPOST\"（op.QueryID 的 JSON 字串）", parsed["queryId"])
	}
}

// TestSend_FeaturesOmittedWhenEmpty_REQ_GQL_024：無 feature switch 宣告的
// 操作 → GET 的 query string 與 POST 的 body 皆無 features／fieldToggles 鍵
// （REQ-GQL-004/024：解析後為空 → 整鍵省略）。
func TestSend_FeaturesOmittedWhenEmpty_REQ_GQL_024(t *testing.T) {
	fx := newSendFixture(t, nil, nil)
	opGet, err := fx.reg.Operation("TestPlain")
	if err != nil {
		t.Fatalf("Operation(TestPlain) error = %v, 期望 nil", err)
	}
	if _, err := fx.sender.Send(opGet, graphql.OrderedParams{{K: "count", V: "20"}}, ""); err != nil {
		t.Fatalf("Send(GET) error = %v, 期望 nil", err)
	}
	opPost, err := fx.reg.Operation("TestMutation")
	if err != nil {
		t.Fatalf("Operation(TestMutation) error = %v, 期望 nil", err)
	}
	if _, err := fx.sender.Send(opPost, graphql.OrderedParams{{K: "tweet_id", V: `"123"`}}, ""); err != nil {
		t.Fatalf("Send(POST) error = %v, 期望 nil", err)
	}

	reqs := fx.captured()
	if len(reqs) != 2 {
		t.Fatalf("捕捉伺服器應收到 2 個請求，得到 %d", len(reqs))
	}
	if q := reqs[0].query(); !strings.HasPrefix(q, "variables=") || strings.Contains(q, "features=") || strings.Contains(q, "fieldToggles=") {
		t.Errorf("GET query 應僅含 variables=（無 features/fieldToggles），得到：%s", q)
	}
	keys := reqs[1].bodyKeys(t)
	want := []string{"variables", "queryId"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("POST body 鍵序 = %v, 期望 %v（無 features/fieldToggles）", keys, want)
	}
}

// TestSend_ErrorsNoData_REQ_GQL_006：200＋errors 且無 data → 讀取語義拋
// *xerrors.GraphQLError：errors.As 可得、Code 為首錯 code（344）、攜帶
// 操作名；分類導出 graphql／不重試／不牽連憑證（設計 §3.5）。
func TestSend_ErrorsNoData_REQ_GQL_006(t *testing.T) {
	fx := newSendFixture(t, func(_ int, _, _ []byte) []byte {
		return jsonResponse(http.StatusOK, `{"errors":[{"code":344,"message":"Rate limited"}]}`)
	}, nil)
	op, err := fx.reg.Operation("TestPlain")
	if err != nil {
		t.Fatalf("Operation(TestPlain) error = %v, 期望 nil", err)
	}
	data, err := fx.sender.Send(op, nil, "")
	var gerr *xerrors.GraphQLError
	if !errors.As(err, &gerr) {
		t.Fatalf("錯誤應可 errors.As 為 *xerrors.GraphQLError，得到 %T：%v", err, err)
	}
	if data != nil {
		t.Errorf("無 data 時不應回傳 data，得到 %s", data)
	}
	if gerr.Code != 344 {
		t.Errorf("Code = %d, 期望 344（首錯 code，REQ-GQL-007）", gerr.Code)
	}
	if gerr.OperationName != "TestPlain" {
		t.Errorf("OperationName = %q, 期望 TestPlain（REQ-GQL-007）", gerr.OperationName)
	}
	if len(gerr.Errors) != 1 || gerr.Errors[0].Message != "Rate limited" {
		t.Errorf("Errors 應保留內層錯誤，得到 %+v", gerr.Errors)
	}
	if gerr.Kind() != xerrors.KindGraphql || gerr.Retryable() || gerr.CredentialImpacting() {
		t.Errorf("code 344 分類應為 graphql/不重試/不牽連憑證，得到 kind=%s retryable=%v cred=%v",
			gerr.Kind(), gerr.Retryable(), gerr.CredentialImpacting())
	}
}

// TestSend_ErrorsWithData_OK：200＋errors 與 data 並存 → 讀取語義不拋
// （REQ-GQL-006：僅完全沒有 data 才拋），回 data 欄位的 raw JSON。
func TestSend_ErrorsWithData_OK(t *testing.T) {
	fx := newSendFixture(t, func(_ int, _, _ []byte) []byte {
		return jsonResponse(http.StatusOK, `{"errors":[{"code":999,"message":"whatever"}],"data":{"viewer":{"id":"v1"}}}`)
	}, nil)
	op, err := fx.reg.Operation("TestPlain")
	if err != nil {
		t.Fatalf("Operation(TestPlain) error = %v, 期望 nil", err)
	}
	data, err := fx.sender.Send(op, nil, "")
	if err != nil {
		t.Fatalf("Send error = %v, 期望 nil（有 data 即不拋）", err)
	}
	if string(data) != `{"viewer":{"id":"v1"}}` {
		t.Errorf("data = %s, 期望 {\"viewer\":{\"id\":\"v1\"}}", data)
	}
}

// TestSend_HTTP401_REQ_LOG_043：401 → errors.As *xerrors.CredentialInvalidError
// （kind=credential、不可重試、牽連憑證、訊息含狀態碼）；且 CookieStore
// 未被清空、cookie 行不變——本任務不清除，清除決策在 app 層
// （REQ-LOG-042/043）。
func TestSend_HTTP401_REQ_LOG_043(t *testing.T) {
	fx := newSendFixture(t, func(_ int, _, _ []byte) []byte {
		return jsonResponse(http.StatusUnauthorized, `{"errors":[{"code":32,"message":"Unauthorized"}]}`)
	}, nil)
	op, err := fx.reg.Operation("TestPlain")
	if err != nil {
		t.Fatalf("Operation(TestPlain) error = %v, 期望 nil", err)
	}
	lineBefore := fx.auth.CookieHeader()
	data, err := fx.sender.Send(op, nil, "")
	var cerr *xerrors.CredentialInvalidError
	if !errors.As(err, &cerr) {
		t.Fatalf("錯誤應可 errors.As 為 *xerrors.CredentialInvalidError，得到 %T：%v", err, err)
	}
	if data != nil {
		t.Errorf("401 時不應回傳 data，得到 %s", data)
	}
	if cerr.Kind() != xerrors.KindCredential || cerr.Retryable() || !cerr.CredentialImpacting() {
		t.Errorf("401 分類應為 credential/不可重試/牽連憑證，得到 kind=%s retryable=%v cred=%v",
			cerr.Kind(), cerr.Retryable(), cerr.CredentialImpacting())
	}
	if !strings.Contains(cerr.Error(), "401") {
		t.Errorf("錯誤訊息應含狀態碼語義，得到：%s", cerr.Error())
	}
	if saves := fx.store.saveCalls(); len(saves) != 0 {
		t.Errorf("引擎不得清除 cookie（清除決策在 app 層）：CookieStore 不應有任何寫入，得到 %v", saves)
	}
	if after := fx.auth.CookieHeader(); after != lineBefore {
		t.Errorf("401 後 cookie 行不得變動，before=%q after=%q", lineBefore, after)
	}
}

// TestSend_Timeout_REQ_LOG_043：伺服器慢（2s）＋客戶端逾時 300ms →
// *xerrors.TransportError（kind=transport、可重試）；逾時路徑 CookieStore
// 不變（絕不清憑證）。
func TestSend_Timeout_REQ_LOG_043(t *testing.T) {
	fx := newSendFixture(t, func(_ int, _, _ []byte) []byte {
		time.Sleep(2 * time.Second)
		return jsonResponse(http.StatusOK, `{"data":{}}`)
	}, func(o *transport.Options) { o.Timeout = 300 * time.Millisecond })
	op, err := fx.reg.Operation("TestPlain")
	if err != nil {
		t.Fatalf("Operation(TestPlain) error = %v, 期望 nil", err)
	}
	start := time.Now()
	_, err = fx.sender.Send(op, nil, "")
	elapsed := time.Since(start)
	var terr *xerrors.TransportError
	if !errors.As(err, &terr) {
		t.Fatalf("逾時錯誤應可 errors.As 為 *xerrors.TransportError，得到 %T：%v", err, err)
	}
	if elapsed >= 2*time.Second {
		t.Errorf("逾時應由客戶端於 300ms 附近觸發，實際耗時 %v", elapsed)
	}
	if terr.Kind() != xerrors.KindTransport || !terr.Retryable() || terr.CredentialImpacting() {
		t.Errorf("逾時分類應為 transport/可重試/不牽連憑證，得到 kind=%s retryable=%v cred=%v",
			terr.Kind(), terr.Retryable(), terr.CredentialImpacting())
	}
	if saves := fx.store.saveCalls(); len(saves) != 0 {
		t.Errorf("逾時路徑 CookieStore 不得變動（不清憑證），得到 %v", saves)
	}
}

// TestSend_SetCookieWriteThrough_REQ_AUTH_014：第一請求回
// Set-Cookie: ct0=NEW → fake CookieStore 的 Save 恰被呼叫一次（同步
// write-through、於 Send 回傳前完成），行為合併後的完整 cookie 行；第二
// 請求的 cookie 標頭含輪替後 ct0（XAuth 已更新，REQ-AUTH-014）。
func TestSend_SetCookieWriteThrough_REQ_AUTH_014(t *testing.T) {
	fx := newSendFixture(t, func(seq int, _, _ []byte) []byte {
		if seq == 1 {
			return jsonResponse(http.StatusOK, `{"data":{"first":true}}`, "ct0=NEWCT0; Path=/; HttpOnly")
		}
		return jsonResponse(http.StatusOK, `{"data":{"second":true}}`)
	}, nil)
	op, err := fx.reg.Operation("TestPlain")
	if err != nil {
		t.Fatalf("Operation(TestPlain) error = %v, 期望 nil", err)
	}

	if _, err := fx.sender.Send(op, nil, ""); err != nil {
		t.Fatalf("Send(1) error = %v, 期望 nil", err)
	}
	saves := fx.store.saveCalls()
	if len(saves) != 1 {
		t.Fatalf("第一請求後 write-through 應同步 Save 恰一次，得到 %d：%v", len(saves), saves)
	}
	if !strings.Contains(saves[0], "ct0=NEWCT0") {
		t.Errorf("Save 行應含輪替後 ct0，得到 %q", saves[0])
	}
	if !strings.Contains(saves[0], "auth_token=TOKEN123") {
		t.Errorf("Save 行應為合併後的完整 cookie 行，得到 %q", saves[0])
	}
	if !strings.Contains(fx.auth.CookieHeader(), "ct0=NEWCT0") {
		t.Errorf("XAuth 應已更新，CookieHeader = %q", fx.auth.CookieHeader())
	}

	if _, err := fx.sender.Send(op, nil, ""); err != nil {
		t.Fatalf("Send(2) error = %v, 期望 nil", err)
	}
	reqs := fx.captured()
	if len(reqs) != 2 {
		t.Fatalf("捕捉伺服器應收到 2 個請求，得到 %d", len(reqs))
	}
	cookieLine := reqs[1].headerValue("cookie")
	if !strings.Contains(cookieLine, "ct0=NEWCT0") {
		t.Errorf("第二請求的 cookie 標頭應含輪替後 ct0（XAuth 已更新），得到 %q", cookieLine)
	}
	if saves := fx.store.saveCalls(); len(saves) != 1 {
		t.Errorf("第二回應無 Set-Cookie，不得再 Save，得到 %v", saves)
	}
}

// TestSend_BodySizeCap_REQ_INV_14：回應 >16 MiB → *xerrors.TransportError
// （讀取上限不可繞過，REQ-INV-14）。
func TestSend_BodySizeCap_REQ_INV_14(t *testing.T) {
	fx := newSendFixture(t, func(_ int, _, _ []byte) []byte {
		return jsonResponse(http.StatusOK, `{"data":"`+strings.Repeat("a", 17<<20)+`"}`)
	}, nil)
	op, err := fx.reg.Operation("TestPlain")
	if err != nil {
		t.Fatalf("Operation(TestPlain) error = %v, 期望 nil", err)
	}
	data, err := fx.sender.Send(op, nil, "")
	var terr *xerrors.TransportError
	if !errors.As(err, &terr) {
		t.Fatalf("超限錯誤應可 errors.As 為 *xerrors.TransportError，得到 %T：%v", err, err)
	}
	if data != nil {
		t.Errorf("超限時不應回傳 data，得到 %d 位元組", len(data))
	}
	if !strings.Contains(terr.Error(), "16 MiB") {
		t.Errorf("錯誤訊息應含 16 MiB 上限語義，得到：%s", terr.Error())
	}
}

// TestSend_RefererDefault_REQ_RD_002：referer 傳空字串 → 預設
// https://x.com/home（REQ-RD-002）。
func TestSend_RefererDefault_REQ_RD_002(t *testing.T) {
	fx := newSendFixture(t, nil, nil)
	op, err := fx.reg.Operation("TestPlain")
	if err != nil {
		t.Fatalf("Operation(TestPlain) error = %v, 期望 nil", err)
	}
	if _, err := fx.sender.Send(op, nil, ""); err != nil {
		t.Fatalf("Send error = %v, 期望 nil", err)
	}
	reqs := fx.captured()
	if len(reqs) != 1 {
		t.Fatalf("捕捉伺服器應收到 1 個請求，得到 %d", len(reqs))
	}
	if got := reqs[0].headerValue("referer"); got != "https://x.com/home" {
		t.Errorf("referer 空值應預設 https://x.com/home（REQ-RD-002），得到 %q", got)
	}
}

// TestSend_MalformedQueryID_REQ_GQL_014：registry 資料異常（queryId 內嵌
// 控制字元，模擬損毀的 static/graphql.json）→ POST 分支 NewRequest 的 URL
// 解析失敗，Send 須回 *xerrors.InvalidInputError（errors.As 斷言；kind=input、
// 不重試、不牽連憑證）且**無 panic**——修復前 POST 分支的 case 級 err 蔭蔽
// 使外層檢查不觸發，nil req 進入標頭迴圈即 nil-pointer panic。測試本身跑完
// 未崩潰即無 panic 的證明。URL 組裝失敗須於送出前攔下：捕捉伺服器不得收
// 到任何請求。
func TestSend_MalformedQueryID_REQ_GQL_014(t *testing.T) {
	fx := newSendFixture(t, nil, nil)
	badReg := loadRegistryFromJSON(t, malformedRegistryJSON)
	fx.reg = badReg            // 白盒注入：與 Sender 共用同一份損毀註冊表
	fx.sender.reg = badReg
	op, err := badReg.Operation("TestMutation")
	if err != nil {
		t.Fatalf("Operation(TestMutation) error = %v, 期望 nil", err)
	}
	data, err := fx.sender.Send(op, graphql.OrderedParams{{K: "tweet_id", V: `"123"`}}, "")
	var ierr *xerrors.InvalidInputError
	if !errors.As(err, &ierr) {
		t.Fatalf("錯誤應可 errors.As 為 *xerrors.InvalidInputError，得到 %T：%v", err, err)
	}
	if data != nil {
		t.Errorf("組裝失敗時不應回傳 data，得到 %s", data)
	}
	if ierr.Kind() != xerrors.KindInput || ierr.Retryable() || ierr.CredentialImpacting() {
		t.Errorf("registry 資料異常分類應為 input/不重試/不牽連憑證，得到 kind=%s retryable=%v cred=%v",
			ierr.Kind(), ierr.Retryable(), ierr.CredentialImpacting())
	}
	if !strings.Contains(ierr.Error(), "TestMutation") {
		t.Errorf("錯誤訊息應含操作名，得到：%s", ierr.Error())
	}
	if reqs := fx.captured(); len(reqs) != 0 {
		t.Errorf("請求組裝失敗須於送出前攔下，捕捉伺服器收到 %d 個請求", len(reqs))
	}
}

// wireHeaderNames 自 raw head 依序抽取標頭名（小寫、跳過請求行，遇空行止）
// ——wire 原樣順序的觀測點（fhttp 的 Header 是無序 map，順序只能於線上驗）。
func wireHeaderNames(t *testing.T, head []byte) []string {
	t.Helper()
	var names []string
	for i, line := range strings.Split(string(head), "\r\n") {
		if i == 0 {
			continue // 請求行
		}
		if line == "" {
			break // head 結尾
		}
		name, _, ok := strings.Cut(line, ":")
		if !ok {
			t.Fatalf("head 行非標頭列：%q（head=%q）", line, head)
		}
		names = append(names, strings.ToLower(strings.TrimSpace(name)))
	}
	return names
}

// subsequenceNames 過濾 names 僅保留 want 中出現者（保序）——隔離 transport
// 於寫出時自加、不經 Header map 的列外標頭（host／content-length 等）。
func subsequenceNames(names []string, want map[string]bool) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if want[n] {
			out = append(out, n)
		}
	}
	return out
}

// TestSend_WireHeaderOrder_REQ_HDR_003：以 raw 捕捉伺服器驗證**線上**標頭
// 順序（REQ-HDR-002/003；fhttp Header 是無序 map，順序僅能於線上觀測）：
// transport 補齊段依 transportHeaderSequence（cookie 錨定於 priority 前），
// GraphQL 族槽依族表序（content-type → authorization → x-twitter-auth-type
// → x-csrf-token → x-twitter-active-user → origin → referer；缺席槽位整列
// 跳過、其餘維持相對序）。GET 與 POST 同族同序（REQ-RD-001）。修復前
// Sender 不宣告 Header-Order，族段被 transport 收尾成字母序——本測試即其
// 迴歸防線。
func TestSend_WireHeaderOrder_REQ_HDR_003(t *testing.T) {
	fx := newSendFixture(t, nil, nil)
	opGet, err := fx.reg.Operation("TestPlain")
	if err != nil {
		t.Fatalf("Operation(TestPlain) error = %v, 期望 nil", err)
	}
	if _, err := fx.sender.Send(opGet, nil, "https://x.com/home"); err != nil {
		t.Fatalf("Send(GET) error = %v, 期望 nil", err)
	}
	opPost, err := fx.reg.Operation("TestMutation")
	if err != nil {
		t.Fatalf("Operation(TestMutation) error = %v, 期望 nil", err)
	}
	if _, err := fx.sender.Send(opPost, nil, ""); err != nil {
		t.Fatalf("Send(POST) error = %v, 期望 nil", err)
	}

	reqs := fx.captured()
	if len(reqs) != 2 {
		t.Fatalf("捕捉伺服器應收到 2 個請求，得到 %d", len(reqs))
	}

	// 受控標頭全集＝transport 序列＋cookie＋本 fixture 在場的族槽
	//（lang／att 缺席 → x-twitter-client-language／x-att 整列跳過）。
	controlled := map[string]bool{
		"user-agent": true, "sec-ch-ua": true, "sec-ch-ua-mobile": true,
		"sec-ch-ua-platform": true, "accept": true, "accept-language": true,
		"cookie": true, "priority": true, "sec-fetch-site": true,
		"sec-fetch-mode": true, "sec-fetch-dest": true,
		"content-type": true, "authorization": true, "x-twitter-auth-type": true,
		"x-csrf-token": true, "x-twitter-active-user": true, "origin": true,
		"referer": true,
	}
	wantOrder := []string{
		"user-agent", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform",
		"accept", "accept-language", "cookie", "priority",
		"sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest",
		"content-type", "authorization", "x-twitter-auth-type", "x-csrf-token",
		"x-twitter-active-user", "origin", "referer",
	}

	for i, req := range reqs {
		method := "GET"
		if i == 1 {
			method = "POST"
		}
		got := subsequenceNames(wireHeaderNames(t, req.head), controlled)
		if !reflect.DeepEqual(got, wantOrder) {
			t.Errorf("%s 線上標頭序（受控集）不符（REQ-HDR-002/003）\n got=%v\nwant=%v", method, got, wantOrder)
		}
		// 族序相對斷言（與全序斷言獨立，防全序整段錯位掩蓋族段）：族段
		// 內容類型恆在 authorization 之前、x-csrf-token 在 x-twitter-auth-type
		// 之後（REQ-HDR-003 逐字序列）。
		names := wireHeaderNames(t, req.head)
		idx := func(name string) int {
			for j, n := range names {
				if n == name {
					return j
				}
			}
			t.Fatalf("%s 線上缺標頭 %s", method, name)
			return -1
		}
		if !(idx("content-type") < idx("authorization") &&
			idx("authorization") < idx("x-twitter-auth-type") &&
			idx("x-twitter-auth-type") < idx("x-csrf-token") &&
			idx("x-csrf-token") < idx("x-twitter-active-user") &&
			idx("x-twitter-active-user") < idx("origin") &&
			idx("origin") < idx("referer")) {
			t.Errorf("%s GraphQL 族序不符 REQ-HDR-003：%v", method, names)
		}
	}
}
