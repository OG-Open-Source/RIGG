// probe11 是 v1.1 P0' 探測工具（計畫 Task 1）：以 v1 既有引擎套件組裝
// sender，提供三個子命令供 Task 2 執行探測、Task 7/9 引用結論——
//
//   - latest：HomeTimeline 雙形態各送一次（GET 啟動形態＋POST seen 形態），
//     輸出 JSON 摘要（請求數／ok／data 佈林，headers 全數脫敏）。
//   - searchid：自檔案或 stdin 讀 chunk JS，輸出 queryId 對照 JSON
//     （純函式、無網路、不需 cookie；真實抓取由 Task 2 以 shell 配合）。
//   - media：以 Sender.Send 取得 HomeTimeline 的原始 data JSON，輸出其中
//     的媒體 CDN URL 逐條清單（v1 的 TimelineResult 尚無媒體欄位，Task 11
//     才加——此處直接掃原始 JSON，不擴引擎介面）。
//   - search（Task 2 儀器）：以記憶體 Operation 驗證 SearchTimeline——
//     queryId 由 searchid 萃取值以參數傳入（不寫死於工具）；REQ-GQL-005
//     恆 POST；空 FeatureSwitches → features 整鍵省略（空 features 臂）。
//
// stdout 一律為 JSON 摘要；stderr 為錯誤訊息。任何輸出（含錯誤）絕不含
// cookie 值（REQ-INV-01）：headers 經 summarize 全數遮罩，錯誤訊息輸出前
// 經 scrub 遮除 cookie 行與其鍵值。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/api"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/fingerprint"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/graphql"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/transport"
)

// maskValue 是所有脫敏輸出的固定遮罩（headers 值與錯誤訊息中的憑證值
// 一律整值取代，不保留長度與片段）。
const maskValue = "***"

// probePageSize 是每次 HomeTimeline 的 count（brief：呼叫
// HomeTimeline("", 20, …)）。
const probePageSize = 20

// probeReq 是單次請求的摘要條目：Sender 不暴露 HTTP status（brief：在
// 摘要記 ok/err 即可，不擴引擎介面）；data 為「回應 data 非空」佈林
// （latest 臂以解析結果含貼文計）。
type probeReq struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
	Data bool   `json:"data"`
	Err  string `json:"err,omitempty"`
}

// extractQueryIDs 以 regex 掃描 chunk JS 中操作名與 queryId 的鄰接對
// （沿 v1 探測節方法，DECISIONS〈queryId 附註〉）：鍵可帶引號或不帶
// （webpack 產物兩形態皆見），兩鍵相鄰成對、鍵序兩種皆收錄；同操作名
// 後見者覆蓋。queryId 為公開介面事實（DECISIONS 同節），非機密。
var (
	reOpQid = regexp.MustCompile(`"?operationName"?\s*:\s*"([A-Za-z0-9_]+)"\s*,\s*"?queryId"?\s*:\s*"([A-Za-z0-9_/-]+)"`)
	reQidOp = regexp.MustCompile(`"?queryId"?\s*:\s*"([A-Za-z0-9_/-]+)"\s*,\s*"?operationName"?\s*:\s*"([A-Za-z0-9_]+)"`)
)

func extractQueryIDs(js string) map[string]string {
	ids := make(map[string]string)
	for _, m := range reOpQid.FindAllStringSubmatch(js, -1) {
		ids[m[1]] = m[2]
	}
	for _, m := range reQidOp.FindAllStringSubmatch(js, -1) {
		ids[m[2]] = m[1]
	}
	return ids
}

// summarize 將請求數、各請求的 ok/data 佈林與請求標頭組成 JSON 摘要字
// 串：headers 值全數以固定遮罩取代（REQ-INV-01：輸出絕不含 cookie 值；
// 遮罩不保留長度）；Err 欄位輸出前先經 scrub 遮除 cookie 行與其各值
// ——與 stderr 同一套輸出端結構性保險，不依賴引擎錯誤字串的內容。輸出
// 以兩格縮排、不轉義 HTML 字元。
func summarize(headers map[string]string, cookieLine string, reqs []probeReq) string {
	masked := make(map[string]string, len(headers))
	for k := range headers {
		masked[k] = maskValue
	}
	clean := make([]probeReq, len(reqs))
	for i, r := range reqs {
		if r.Err != "" {
			r.Err = scrub(r.Err, cookieLine)
		}
		clean[i] = r
	}
	out := struct {
		Count    int               `json:"count"`
		Requests []probeReq        `json:"requests"`
		Headers  map[string]string `json:"headers,omitempty"`
	}{Count: len(reqs), Requests: clean, Headers: masked}
	s, err := marshalJSON(out)
	if err != nil {
		// 欄位僅含 int／bool／string／map[string]string，序列化不會失敗；
		// 保險起見回退為僅含請求數的固定單行摘要（不回印動態值）。
		return fmt.Sprintf(`{"count":%d,"error":"summarize 序列化失敗"}`, len(reqs))
	}
	return s
}

// reMediaURL 比對 JSON 內的媒體 CDN URL（pbs.twimg.com／video.twimg.com，
// 供 Task 2 實抓縮圖與 mp4 variant）；URL 邊界以引號、反斜線與空白為界。
var reMediaURL = regexp.MustCompile(`https://(?:pbs|video)\.twimg\.com[^"\\\s]*`)

// mediaURLs 自原始回應 JSON 抽取媒體 CDN URL，首見順序去重：JSON 內斜線
// 常以 \/ 轉義，先正規化再掃描。
func mediaURLs(raw []byte) []string {
	normalized := bytes.ReplaceAll(raw, []byte(`\/`), []byte(`/`))
	seen := make(map[string]bool)
	out := []string{}
	for _, u := range reMediaURL.FindAllString(string(normalized), -1) {
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// marshalJSON 以兩格縮排、不轉義 HTML 字元序列化（媒體 URL 內的 & 保持
// 原樣，便於 Task 2 直接取用）。
func marshalJSON(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// writeJSON 序列化並寫 stdout（marshalJSON 失敗即退出——輸出絕不回退為
// 列印未序列化的原值）。
func writeJSON(v any) {
	s, err := marshalJSON(v)
	if err != nil {
		failf("輸出序列化失敗: %v", err)
	}
	fmt.Println(s)
}

// scrub 將訊息中出現的 cookie 行與其各鍵值替換為固定遮罩（REQ-INV-01 的
// 輸出端結構性保險：引擎錯誤字串依規格本不含值，此處保證探測端持有的
// 原始行無論如何不滲入輸出）。
func scrub(msg, cookieLine string) string {
	if cookieLine == "" {
		return msg
	}
	msg = strings.ReplaceAll(msg, cookieLine, maskValue)
	for _, seg := range strings.Split(cookieLine, ";") {
		if _, v, ok := strings.Cut(strings.TrimSpace(seg), "="); ok && v != "" {
			msg = strings.ReplaceAll(msg, v, maskValue)
		}
	}
	return msg
}

// failf 列印固定格式錯誤訊息（不自帶動態憑證值）並 exit 1。
func failf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "probe11: "+format+"\n", args...)
	os.Exit(1)
}

// failErr 列印引擎錯誤（先經 scrub 遮除憑證值）並 exit 1。
func failErr(cookieLine string, err error) {
	fmt.Fprintf(os.Stderr, "probe11: %s\n", scrub(err.Error(), cookieLine))
	os.Exit(1)
}

// registryPath 解析註冊表快照位置（沿用 X/session.go 的兩級查找）：優先
// 可執行檔同目錄的扁平檔名（mygo build 的 resources 會攤平路徑），退回
// 工作目錄相對路徑（go run／開發情境，cwd 為模組根，static/ 在原位）。
func registryPath() string {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "graphql.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join("static", "graphql.json")
}

// probeSession 聚集一次組裝的探測產物：sender（發送）、auth（摘要用
// bearer）與 reg（media 子命令直取操作 metadata）。
type probeSession struct {
	sender *api.Sender
	auth   *auth.Auth
	reg    *graphql.Registry
}

// probeCookie 讀取環境變數 PROBE_COOKIE；空（含全空白）→ 列印不含值的
// 錯誤並回 false（呼叫端 exit 1，brief：錯誤訊息不得含值）。
func probeCookie() (string, bool) {
	line := os.Getenv("PROBE_COOKIE")
	if strings.TrimSpace(line) == "" {
		fmt.Fprintln(os.Stderr, "probe11: PROBE_COOKIE 未設定或為空白——請於 shell 設定單行 cookie 後重試（值不落檔案與輸出）")
		return "", false
	}
	return line, true
}

// newProbeSession 組裝探測會話（brief 鏈，同 §8.4；AllowDirect 直連同 v1
// 探測情境 DECISIONS OQ12）：fingerprint.Get → auth.NewAuth（bearer 空 →
// 預設公開 web bearer）→ PrepareAuth(cookieLine) → LoadRegistry →
// transport.New → api.NewSender。
func newProbeSession(cookieLine string) (*probeSession, error) {
	fp := fingerprint.Get()
	a := auth.NewAuth("")
	if err := a.PrepareAuth(cookieLine); err != nil {
		return nil, err
	}
	reg, err := graphql.LoadRegistry(registryPath())
	if err != nil {
		return nil, err
	}
	tr, err := transport.New(fp, transport.Options{ProxyURL: "", AllowDirect: true})
	if err != nil {
		return nil, err
	}
	return &probeSession{sender: api.NewSender(tr, a, reg, fp), auth: a, reg: reg}, nil
}

// runLatest 以雙形態各送一次 HomeTimeline（brief：兩呼叫各 1 次，Task 2
// 會以迴圈擴增）：臂一 GET 啟動形態（cursor 空、seen 空 →
// requestContext=launch）；臂二 POST 形態（seen 非空 → 引擎顯式覆寫
// POST、seenTweetIds 置變數末）。Sender 不暴露 HTTP status，摘要記
// ok/err 即可（不擴引擎介面）。
func runLatest(s *probeSession) []probeReq {
	reqs := make([]probeReq, 0, 2)
	res, err := s.sender.HomeTimeline("", probePageSize, nil)
	reqs = append(reqs, newProbeReq("latest.get", res, err))
	res, err = s.sender.HomeTimeline("", probePageSize, []string{"1"})
	reqs = append(reqs, newProbeReq("latest.post-seen", res, err))
	return reqs
}

// newProbeReq 由一次 HomeTimeline 結果組摘要條目：ok＝無錯；data＝回應
// 含貼文（「data 非空」的可觀測代理，對應 Task 2 判準「各回 200 且
// data 非空」）；err 為引擎錯誤字串（stdout 輸出前經 summarize scrub
// 遮除憑證值，REQ-INV-01）。
func newProbeReq(name string, res *api.TimelineResult, err error) probeReq {
	req := probeReq{Name: name}
	if err != nil {
		req.Err = err.Error()
		return req
	}
	req.OK = true
	req.Data = res != nil && len(res.Posts) > 0
	return req
}

// probeHeaders 組摘要用的請求標頭映射（探測端已知的注入者）：cookie 為
// 呼叫端注入（DECISIONS D9：transport 無 jar），authorization 為 GraphQL
// 族固定槽位。兩者值經 summarize 全數遮罩後才會出現在輸出。
func probeHeaders(a *auth.Auth, cookieLine string) map[string]string {
	return map[string]string{
		"authorization": "Bearer " + a.Bearer(),
		"cookie":        cookieLine,
	}
}

// runLatestCmd：latest 子命令——雙形態探測並輸出脫敏 JSON 摘要。
func runLatestCmd() {
	cookieLine, ok := probeCookie()
	if !ok {
		os.Exit(1)
	}
	sess, err := newProbeSession(cookieLine)
	if err != nil {
		failErr(cookieLine, err)
	}
	fmt.Println(summarize(probeHeaders(sess.auth, cookieLine), cookieLine, runLatest(sess)))
}

// newProbeReqRaw 由一次原始 JSON 回應組摘要條目（Send 直呼路徑——回應
// 未經解析，data 以「含 tweet_results」計）：ok＝無錯且 raw 非空。
func newProbeReqRaw(name string, raw json.RawMessage, err error) probeReq {
	req := probeReq{Name: name}
	if err != nil {
		req.Err = err.Error()
		return req
	}
	req.OK = true
	req.Data = bytes.Contains(raw, []byte(`"tweet_results"`))
	return req
}

// runSearchCmd：search 子命令（Task 2 儀器）——args: <queryId> <query>
// [product] [count]。以記憶體 Operation 送 SearchTimeline：REQ-GQL-005
// 恆 POST；FeatureSwitches 空 → Features() 回 nil → features 整鍵省略
// （空 features 臂——P0' 實證可行）；variables 需含 product（P0' 422
// 實證：GRAPHQL_VALIDATION_FAILED "must be defined: variable product"）。
// queryId 為公開介面事實，由參數傳入、不寫死於工具。
func runSearchCmd(args []string) {
	if len(args) < 2 {
		failf("用法: probe11 search <queryId> <query> [product] [count]")
	}
	qid, query := args[0], args[1]
	product := "Top"
	if len(args) > 2 {
		product = args[2]
	}
	count := 20
	if len(args) > 3 {
		n, err := strconv.Atoi(args[3])
		if err != nil || n <= 0 || n > 100 {
			failf("probe11: count 必須為 1..100 的整數")
		}
		count = n
	}
	cookieLine, ok := probeCookie()
	if !ok {
		os.Exit(1)
	}
	sess, err := newProbeSession(cookieLine)
	if err != nil {
		failErr(cookieLine, err)
	}
	op := &graphql.Operation{Name: "SearchTimeline", QueryID: qid, Method: "POST"}
	qv, qerr := graphql.CompactJSON(query)
	if qerr != nil {
		failErr(cookieLine, qerr)
	}
	pv, perr := graphql.CompactJSON(product)
	if perr != nil {
		failErr(cookieLine, perr)
	}
	vars := graphql.OrderedParams{
		{K: "product", V: string(pv)},
		{K: "rawQuery", V: string(qv)},
		{K: "count", V: strconv.Itoa(count)},
	}
	raw, err := sess.sender.Send(op, vars, "")
	fmt.Println(summarize(probeHeaders(sess.auth, cookieLine), cookieLine,
		[]probeReq{newProbeReqRaw("search.post", raw, err)}))
}

// runSearchIDCmd：searchid 子命令——自檔案路徑參數（"-" 為 stdin）或
// stdin 讀 chunk JS，輸出 queryId 對照 JSON。純函式路徑：無網路、不讀
// PROBE_COOKIE（Task 2 於 latest 失敗時以此重建 chunk map 核對 queryId，
// 不應被 cookie 有無綁住）。
func runSearchIDCmd(args []string) {
	var js []byte
	var err error
	if len(args) > 0 && args[0] != "-" {
		js, err = os.ReadFile(args[0])
		if err != nil {
			failf("讀取 JS 檔失敗: %v", err)
		}
	} else {
		js, err = io.ReadAll(os.Stdin)
		if err != nil {
			failf("讀取 stdin 失敗: %v", err)
		}
	}
	writeJSON(extractQueryIDs(string(js)))
}

// runMediaCmd：media 子命令——以 Sender.Send 直取 HomeTimeline 的原始
// data JSON（不走解析後的 TimelineResult：v1 尚無媒體欄位，本任務不加，
// Task 11 才加），輸出媒體 CDN URL 逐條清單供 Task 2 實抓。變數組裝同
// HomeTimeline 的啟動形態（cursor 空、seen 空 → requestContext=launch），
// 方法用註冊表原值（GET），不覆寫。
func runMediaCmd() {
	cookieLine, ok := probeCookie()
	if !ok {
		os.Exit(1)
	}
	sess, err := newProbeSession(cookieLine)
	if err != nil {
		failErr(cookieLine, err)
	}
	op, err := sess.reg.Operation("HomeTimeline")
	if err != nil {
		failErr(cookieLine, err)
	}
	vars := graphql.OrderedParams{
		{K: "count", V: strconv.Itoa(probePageSize)},
		{K: "includePromotedContent", V: "true"},
		{K: "withCommunity", V: "true"},
		{K: "requestContext", V: `"launch"`},
	}
	data, err := sess.sender.Send(op, vars, "")
	if err != nil {
		failErr(cookieLine, err)
	}
	urls := mediaURLs(data)
	writeJSON(struct {
		Count int      `json:"count"`
		URLs  []string `json:"urls"`
	}{Count: len(urls), URLs: urls})
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "latest":
		runLatestCmd()
	case "searchid":
		runSearchIDCmd(os.Args[2:])
	case "media":
		runMediaCmd()
	case "search":
		runSearchCmd(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "probe11: 未知子命令 %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

// usage 列印子命令用法（stderr）。
func usage() {
	fmt.Fprint(os.Stderr, `用法: probe11 <子命令> [參數]

子命令:
  latest             HomeTimeline 雙形態各送一次（GET 啟動形態＋POST seen 形態），輸出脫敏 JSON 摘要
  searchid [檔案|-]  自檔案或 stdin 讀 chunk JS，輸出 queryId 對照 JSON（無網路、不需 cookie）
  media              送一次 HomeTimeline，輸出回應中的媒體 CDN URL 清單 JSON
  search <qid> <query> [product] [count]
                     以記憶體 Operation 送 SearchTimeline（恆 POST、空
                     features），輸出脫敏 JSON 摘要（Task 2 儀器）

環境:
  PROBE_COOKIE       單行 cookie（latest／media 必需；值不落檔案與輸出）
`)
}
