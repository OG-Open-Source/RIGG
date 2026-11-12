// transport 客戶端核心：tls-client（chrome_150 profile）session 形態建構
// 與 Do 管線（預設標頭補齊 → 導覽標頭移除 → sec-fetch 推導 → Header-Order
// 附加 → 送出）。全部 X 流量的單一出口（REQ-TRN-010..020）。
package transport

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/fingerprint"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// defaultTimeout 是 Options.Timeout 零值時的請求逾時（REQ-TRN-035）。
const defaultTimeout = 30 * time.Second

// navigationHeaders 是導覽請求殘留標頭，Do 一律移除（REQ-TRN-013）。
var navigationHeaders = []string{"upgrade-insecure-requests", "sec-fetch-user"}

// transportHeaderSequence 是 transport 補齊標頭與 cookie 的固定輸出序
//（Chrome 慣例相對序）：transport 實際補上的標頭依此序置於 Header-Order
// 前段；cookie 為呼叫端注入（REQ-AUTH-001，XAuth 單一持有者），存在時恆
// 錨定於 priority 之前（Chrome 觀察：cookie 在 priority 前）。
var transportHeaderSequence = []string{
	"user-agent", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform",
	"accept", "accept-language",
	"cookie", "priority",
	"sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest",
}

// supportedProxySchemes 是 tls-client v1.18.0 支援的代理 scheme
//（http／https／socks5／socks5h／socks4）；其餘於 New 即拒絕（fail-closed，
// 不等到送出才失敗，更不得直連）。
var supportedProxySchemes = map[string]bool{
	"http": true, "https": true, "socks5": true, "socks5h": true, "socks4": true,
}

// noopCookieJar 是 fhttp.CookieJar 的無操作實作：transport 不持有任何
// cookie 狀態（REQ-AUTH-001——XAuth 為唯一憑證持有者；Set-Cookie 的記帳
// 由上層經回應處理，傳輸層不得成為第二持有者）。以此明確中和 tls-client
// 的 jar 掛點，杜絕 jar 於 Header-Order 附加後以 AddCookie 追加 Cookie
// 造成失序與憑證分流。
type noopCookieJar struct{}

var _ fhttp.CookieJar = noopCookieJar{}

func (noopCookieJar) SetCookies(u *url.URL, cookies []*fhttp.Cookie) {}

func (noopCookieJar) Cookies(u *url.URL) []*fhttp.Cookie { return nil }

// Options 是 transport 建構選項。
type Options struct {
	// ProxyURL 為必填代理位址；空字串使 New 直接回 *xerrors.TransportError
	//（fail-closed，REQ-INV-02/REQ-TRN-018），本套件不存在直連路徑——
	// 除非 AllowDirect 為 true（見下）。
	ProxyURL string

	// AllowDirect 允許無代理直連，預設 false（fail-closed 不變，REQ-INV-02）。
	// 例外掛點僅供「使用者明確指示直連」的情境（DECISIONS OQ12：本環境無
	// 系統代理、直連實證可達且被接受；L2 煙霧程式以此構造並記錄）。正式
	// app 路徑不得設置本欄位——設置即偏離 D6 的失敗關閉裁決。
	AllowDirect bool

	// Timeout 是單一請求逾時；零值取 30 秒（REQ-TRN-035）。
	Timeout time.Duration

	// OnRequest 於每次送出前呼叫（method、url）；panic 被吸收（REQ-INV-13）。
	OnRequest func(method, url string)

	// OnResponse 於回應後呼叫（method、url、status、耗時、Set-Cookie 名稱
	// 清單）；panic 被吸收（REQ-INV-13）。
	OnResponse func(method, url string, status int, elapsed time.Duration, setCookieNames []string)
}

// Client 是 session 形態的 X 傳輸客戶端：chrome_150 TLS 指紋、必配代理、
// 一律不跟隨重定向、不持有任何 cookie 狀態（jar 以 no-op 中和；Cookie 標頭
// 由呼叫端自 XAuth 注入，REQ-AUTH-001；探測裁定 v1 無 anonymous 形態，見
// DECISIONS.md）。
type Client struct {
	client tls_client.HttpClient
	fp     *fingerprint.Fingerprint
	opts   Options
}

// New 建構 session 形態傳輸客戶端。
//
//   - ProxyURL 空字串或格式／scheme 無效 → 回 *xerrors.TransportError
//     （fail-closed，REQ-INV-02/REQ-TRN-018）：無有效代理即拒絕，絕不退回
//     直連。唯一例外：AllowDirect=true 時空 ProxyURL 以直連建構（僅供
//     「使用者明確指示直連」的情境，DECISIONS OQ12；正式 app 不得設置）。
//     ProxyURL 非空時一律照常驗證並走代理，AllowDirect 不生效。
//   - Timeout 零值 → 30 秒（REQ-TRN-035）。
//   - 重定向一律不跟隨（REQ-TRN-040）：POST 收 3xx 亦原樣回傳回應本身。
//   - cookie jar 以 no-op 中和：本層無 cookie 狀態，Cookie 標頭由呼叫端
//     注入（REQ-AUTH-001）。
func New(fp *fingerprint.Fingerprint, opts Options) (*Client, error) {
	if fp == nil {
		return nil, &xerrors.InvalidInputError{Msg: "transport: 指紋不得為 nil"}
	}
	if strings.TrimSpace(opts.ProxyURL) == "" && !opts.AllowDirect {
		return nil, &xerrors.TransportError{Msg: "代理未設定：transport 拒絕直連（fail-closed，REQ-INV-02）"}
	}
	var proxyOpts []tls_client.HttpClientOption
	if strings.TrimSpace(opts.ProxyURL) != "" {
		pu, perr := url.Parse(opts.ProxyURL)
		if perr != nil || pu.Host == "" || !supportedProxySchemes[strings.ToLower(pu.Scheme)] {
			return nil, &xerrors.TransportError{Msg: fmt.Sprintf("代理設定無效（fail-closed，REQ-INV-02）：%q", opts.ProxyURL)}
		}
		proxyOpts = append(proxyOpts, tls_client.WithProxyUrl(opts.ProxyURL))
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ms := int(timeout / time.Millisecond)
	if ms < 1 {
		ms = 1
	}
	hc, err := tls_client.NewHttpClient(
		tls_client.NewNoopLogger(),
		append(proxyOpts,
			tls_client.WithClientProfile(profiles.Chrome_150),
			tls_client.WithCookieJar(noopCookieJar{}),
			tls_client.WithNotFollowRedirects(),
			tls_client.WithTimeoutMilliseconds(ms),
		)...)
	if err != nil {
		return nil, &xerrors.TransportError{Msg: fmt.Sprintf("tls-client 建構失敗: %v", err)}
	}
	return &Client{client: hc, fp: fp, opts: opts}, nil
}

// Do 送出單一請求：補預設標頭 → 推導 sec-fetch → 移除導覽標頭 → 附加
// Header-Order → 送出（不跟隨重定向，3xx 原樣回傳）。送出失敗一律包成
// *xerrors.TransportError（kind=transport、可重試、不牽連憑證）；回應狀態
// 碼不在本層判讀（屬 graphql/api 層職責）。
func (c *Client) Do(req *fhttp.Request) (*fhttp.Response, error) {
	// 送出前取定觀測參數：後續標頭管線不更動 method/url。
	method, target := req.Method, req.URL.String()
	start := time.Now()

	c.prepareHeaders(req)
	c.fireOnRequest(method, target)

	resp, err := c.client.Do(req)
	if err != nil {
		// 訊息承載底層錯誤供日誌；分類語義固定，app 只 switch on kind
		//（REQ-LOG-042/043）。
		return nil, &xerrors.TransportError{Msg: fmt.Sprintf("%s %s 失敗: %v", method, target, err)}
	}

	setCookieNames := make([]string, 0, len(resp.Cookies()))
	for _, ck := range resp.Cookies() {
		setCookieNames = append(setCookieNames, ck.Name)
	}
	c.fireOnResponse(method, target, resp.StatusCode, time.Since(start), setCookieNames)
	return resp, nil
}

// Close 釋放底層 tls-client 連線資源。
func (c *Client) Close() error {
	c.client.Close()
	return nil
}

// prepareHeaders 執行送出前的標頭管線（就地修改 req.Header）：
//
//  1. 移除導覽殘留標頭（REQ-TRN-013）。
//  2. 預設標頭補齊（REQ-TRN-012）：呼叫端未給值才補，絕不覆蓋呼叫端值。
//  3. sec-fetch 推導（REQ-TRN-014/015）：sec-fetch-site 依目標 host 逐請求
//     推導並恆以推導值為準（殘值即指紋破口）；mode/dest 缺值才補。
//  4. 附加 Header-Order（REQ-HDR-002/013）：fhttp 僅認此法保留插入序與
//     原樣大小寫；值為全小寫名稱有序切片，不含 Header-Order 自身。Cookie
//     標頭為呼叫端自有（REQ-AUTH-001），本函式不補、不改值，僅在其存在
//     時納入順序並錨定於 priority 前。
func (c *Client) prepareHeaders(req *fhttp.Request) {
	h := req.Header
	if h == nil {
		h = make(fhttp.Header)
		req.Header = h
	}

	// 1) 導覽殘留標頭移除——不分存入大小寫全數刪除。
	for _, name := range navigationHeaders {
		for k := range h {
			if strings.EqualFold(k, name) {
				delete(h, k)
			}
		}
	}

	// 2) 預設標頭補齊（REQ-TRN-012，值取自指紋檔）。added 記錄實際由
	// transport 補上的名稱（供 Header-Order 前段排序）。
	var added []string
	addDefault := func(name, value string) {
		if hasHeader(h, name) {
			return
		}
		h[name] = []string{value}
		added = append(added, name)
	}
	addDefault("user-agent", c.fp.UA)
	addDefault("sec-ch-ua", c.fp.SecChUA)
	addDefault("sec-ch-ua-mobile", c.fp.SecChUAMobile)
	addDefault("sec-ch-ua-platform", c.fp.SecChUAPlatform)
	addDefault("accept", "*/*")
	addDefault("accept-language", c.fp.AcceptLanguage)
	addDefault("priority", "u=1, i")

	// 3) sec-fetch 推導（REQ-TRN-014/015）。
	addDefault("sec-fetch-mode", "cors")
	addDefault("sec-fetch-dest", "empty")
	// site 為逐請求推導值，恆覆寫；transport 對其擁有唯一控制權。
	site := deriveSecFetchSite(effectiveHost(req))
	h["sec-fetch-site"] = []string{site}
	added = append(added, "sec-fetch-site")

	// 4) Header-Order 附加（REQ-HDR-002/013）。
	attachHeaderOrder(h, added)
}

// hasHeader 以不分大小寫方式判定標頭是否已有值（呼叫端已設即不補）。
func hasHeader(h fhttp.Header, name string) bool {
	for k := range h {
		if strings.EqualFold(k, name) {
			return true
		}
	}
	return false
}

// effectiveHost 回傳請求的生效 host：req.Host（呼叫端覆寫的 Host 標頭）
// 優先，否則 URL host。
func effectiveHost(req *fhttp.Request) string {
	if req.Host != "" {
		return req.Host
	}
	return req.URL.Host
}

// deriveSecFetchSite 依目標 host 推導 sec-fetch-site（REQ-TRN-015）：
// host 恰 x.com 或 www.x.com → same-origin；*.x.com（不含裸 x.com）、
// twitter.com、*.twitter.com → same-site；其他 → cross-site。
func deriveSecFetchSite(host string) string {
	h := strings.ToLower(host)
	if i := strings.IndexByte(h, ':'); i >= 0 {
		h = h[:i] // 去埠號
	}
	switch {
	case h == "x.com" || h == "www.x.com":
		return "same-origin"
	case strings.HasSuffix(h, ".x.com") || h == "twitter.com" || strings.HasSuffix(h, ".twitter.com"):
		return "same-site"
	default:
		return "cross-site"
	}
}

// attachHeaderOrder 依最終標頭集組出 Header-Order 值並附加（REQ-HDR-002/
// 013）。fhttp 的 Header 是無序 map、不追蹤插入序，故順序由本函式顯式
// 組成，規則：
//
//  1. 固定序列 transportHeaderSequence（前段）：transport 實際補上的標頭
//     依此序；cookie 雖為呼叫端注入（REQ-AUTH-001），存在時同樣錨定於
//     序列位置——priority 之前（Chrome 觀察序）。
//  2. 呼叫端經 Header-Order 宣告且仍存在的其餘標頭（宣告序保留，不分原
//     宣告大小寫比對）。
//  3. 其餘現存標頭（呼叫端已設但未宣告者），字典序收尾以保確定性。
//
// 值全小寫、涵蓋全部現存標頭（magic key 除外）、不含 Header-Order 自身
//（避免自引用）；fhttp 寫出時以存入原樣大小寫呈現、magic key 不上 wire。
func attachHeaderOrder(h fhttp.Header, added []string) {
	present := make(map[string]bool, len(h))
	for k := range h {
		if k == fhttp.HeaderOrderKey || k == fhttp.PHeaderOrderKey {
			continue
		}
		present[strings.ToLower(k)] = true
	}
	addedSet := make(map[string]bool, len(added))
	for _, name := range added {
		addedSet[name] = true
	}

	order := make([]string, 0, len(present))
	seen := make(map[string]bool, len(present))
	push := func(lower string) {
		if lower == "" || seen[lower] || !present[lower] {
			return
		}
		seen[lower] = true
		order = append(order, lower)
	}
	for _, name := range transportHeaderSequence {
		if name == "cookie" || addedSet[name] {
			push(name)
		}
	}
	if declared, ok := h[fhttp.HeaderOrderKey]; ok {
		for _, name := range declared {
			push(strings.ToLower(name))
		}
	}
	var rest []string
	for lower := range present {
		if !seen[lower] {
			seen[lower] = true
			rest = append(rest, lower)
		}
	}
	sort.Strings(rest)
	order = append(order, rest...)
	if len(order) > 0 {
		h[fhttp.HeaderOrderKey] = order
	}
}
