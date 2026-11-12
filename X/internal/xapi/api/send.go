package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/fingerprint"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/graphql"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/headers"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/transport"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// maxResponseBytes 是回應讀取上限 16 MiB（REQ-INV-14：資源上限必須存在
// 且不可繞過）。
const maxResponseBytes = 16 << 20

// defaultReferer 是 referer 傳空字串時的預設值（REQ-RD-002；可由呼叫端
// 傳入非空 referer 逐請求覆寫）。
const defaultReferer = "https://x.com/home"

// Sender 是引擎的統一發送層：組裝 transport＋auth＋headers＋graphql，
// 單一 Send 入口（設計 §3.4 七步）。依賴皆於建構期注入、建構後不變。
type Sender struct {
	client *transport.Client
	auth   *auth.Auth
	reg    *graphql.Registry
	fp     *fingerprint.Fingerprint

	// baseOverride 僅供同套件測試注入（白盒）：非空時以之取代 op.URL()
	// 的 "https://x.com" 前綴，使請求指向本地捕捉伺服器（測試無法對本地
	// 伺服器滿足 https://x.com 的 TLS 驗證）。生產路徑恆為空字串，Send
	// 逐字使用 op.URL()（REQ-GQL-021）。
	baseOverride string
}

// NewSender 組裝統一發送器：c 為 transport 客戶端（代理必配，fail-closed）、
// a 為 XAuth（唯一憑證持有者）、reg 為操作註冊表、fp 為指紋檔（headers
// 之用）。
func NewSender(c *transport.Client, a *auth.Auth, reg *graphql.Registry, fp *fingerprint.Fingerprint) *Sender {
	return &Sender{client: c, auth: a, reg: reg, fp: fp}
}

// Send 以單一入口送出一個 GraphQL 操作並回傳回應的 data 欄位（JSON raw）。
// 流程（設計 §3.4 七步）：
//
//  1. features／fieldToggles 解析（v1 無 explicit overrides，REQ-GQL-015/016）。
//  2. cookie 行單次載入（Task 6 caller rule）：該行同時供 Cookie 標頭與
//     headers.Build 的 x-csrf-token 解析——同一行即 REQ-HDR-008 的請求級
//     同代保證，絕不另呼 CSRFToken() 配對。
//  3. GraphQL POST 族標頭（GET 亦用此族，REQ-RD-001）。
//  4. 組請求：GET → query string 鍵序 variables→features→fieldToggles
//     （REQ-GQL-001；解析後為空者整鍵省略，REQ-GQL-024）；POST → body
//     鍵序 variables→features→fieldToggles→queryId（REQ-GQL-003）。
//  5. c.Do 送出（transport 補預設標頭、Header-Order 含 cookie 錨定、
//     sec-fetch 推導、不跟隨重定向）。
//  6. 回應：讀 body（16 MiB 上限，REQ-INV-14）→ Set-Cookie 合併回 XAuth
//     並同步 write-through（任何狀態碼、於回傳前完成）→ 非 2xx 分類 →
//     2xx 讀取語義「有 errors 且無 data 才拋」（REQ-GQL-006/007）。
//  7. Do 的錯誤（含逾時）已為 *xerrors.TransportError，原樣向上。
func (s *Sender) Send(op *graphql.Operation, variables graphql.OrderedParams, referer string) (json.RawMessage, error) {
	// 1) features／fieldToggles（v1 無 explicit overrides）。
	features := op.Features(s.reg, nil)
	toggles := op.FieldToggles(s.reg, nil)

	// 2) cookie 行單次載入（Task 6 caller rule，見函式註解）。
	cookieLine := s.auth.CookieHeader()

	// 3) GraphQL POST 族標頭（REQ-RD-001）；referer 空值預設（REQ-RD-002）。
	if referer == "" {
		referer = defaultReferer
	}
	hs, err := headers.Build(headers.FamGraphQLPost, s.auth.Bearer(), cookieLine, s.fp,
		map[string]string{"referer": referer})
	if err != nil {
		return nil, err
	}

	// 4) 組請求。variables 先以緊湊 JSON 序列化（REQ-GQL-002），鍵序由
	//    OrderedParams 承載，一切輸出絕不排序。
	varsJSON, err := graphql.MarshalOrdered(variables)
	if err != nil {
		return nil, &xerrors.InvalidInputError{Msg: fmt.Sprintf("operation %s: variables 序列化失敗: %v", op.Name, err)}
	}
	params := graphql.OrderedParams{{K: "variables", V: string(varsJSON)}}
	if len(features) > 0 {
		fj, err := graphql.MarshalOrdered(features)
		if err != nil {
			return nil, &xerrors.InvalidInputError{Msg: fmt.Sprintf("operation %s: features 序列化失敗: %v", op.Name, err)}
		}
		params = append(params, graphql.Param{K: "features", V: string(fj)})
	}
	if len(toggles) > 0 {
		tj, err := graphql.MarshalOrdered(toggles)
		if err != nil {
			return nil, &xerrors.InvalidInputError{Msg: fmt.Sprintf("operation %s: fieldToggles 序列化失敗: %v", op.Name, err)}
		}
		params = append(params, graphql.Param{K: "fieldToggles", V: string(tj)})
	}

	target := op.URL()
	if s.baseOverride != "" {
		target = s.baseOverride + op.Path()
	}

	var req *fhttp.Request
	switch op.Method {
	case "GET":
		// query string＝有序 k=<percent-encoded V>（REQ-GQL-001），body nil。
		req, err = fhttp.NewRequest("GET", target+"?"+params.EncodeQuery(), nil)
	case "POST":
		// body 鍵序 variables→features→fieldToggles→queryId（REQ-GQL-003）；
		// queryId 為 op.QueryID 的 JSON 字串。case 內錯誤一律用獨立名稱
		//（qerr/berr）：若以 `err :=` 蔭蔽外層 err，NewRequest 失敗時外層
		// 檢查不觸發、nil req 進入標頭迴圈即 panic（REQ-GQL-014——registry
		// 資料異常須以型別化錯誤回報，絕不成崩潰）。
		qid, qerr := graphql.CompactJSON(op.QueryID)
		if qerr != nil {
			return nil, &xerrors.InvalidInputError{Msg: fmt.Sprintf("operation %s: queryId 序列化失敗: %v", op.Name, qerr)}
		}
		body, berr := graphql.MarshalOrdered(append(params, graphql.Param{K: "queryId", V: string(qid)}))
		if berr != nil {
			return nil, &xerrors.InvalidInputError{Msg: fmt.Sprintf("operation %s: body 序列化失敗: %v", op.Name, berr)}
		}
		req, err = fhttp.NewRequest("POST", target, bytes.NewReader(body))
	default:
		return nil, &xerrors.InvalidInputError{Msg: fmt.Sprintf("operation %s: 不支援的送出方法 %q（僅 GET／POST）", op.Name, op.Method)}
	}
	if err != nil {
		return nil, &xerrors.InvalidInputError{Msg: fmt.Sprintf("operation %s: 建構請求失敗: %v", op.Name, err)}
	}
	// 依序加入族表輸出的每個 Slot（Name 全小寫，REQ-HDR-013）；直接以
	// map 賦值保留原樣小寫（fhttp 寫出時以存入鍵呈現）。cookie 為呼叫端
	// 自有（DECISIONS D9：transport 無 jar），自同一行注入；未 Prepare 前
	// 為空行，空值標頭不上線路。
	//
	// 同步以 Header-Order 依槽位序宣告族標頭（REQ-HDR-002/003）：fhttp 的
	// Header 是無序 map，不宣告則 transport 收尾時族標頭整段落入字母序，
	// 族序不上線路。宣告值為逐一名稱的切片（fhttp v0.7.3 與 transport 的
	// attachHeaderOrder 皆按元素迭代）。cookie 不入清單——transport 既有
	// 的 sequence-anchoring 負責將其錨定於 priority 前；其餘 caller 自設
	// 標頭本層無（僅族槽＋cookie）。
	order := make([]string, 0, len(hs)+1)
	for _, slot := range hs {
		req.Header[slot.Name] = []string{slot.Value}
		order = append(order, slot.Name)
	}
	if cookieLine != "" {
		req.Header["cookie"] = []string{cookieLine}
	}
	req.Header[fhttp.HeaderOrderKey] = order

	// 5) 送出；Do 的錯誤（含逾時）已為 *xerrors.TransportError，原樣向上。
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 6a) 讀 body（上限 16 MiB，REQ-INV-14）：多讀一位元組偵測超限。
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, &xerrors.TransportError{Msg: fmt.Sprintf("%s %s: 讀取回應失敗: %v", op.Method, op.Name, err)}
	}
	if len(body) > maxResponseBytes {
		return nil, &xerrors.TransportError{Msg: fmt.Sprintf("%s %s: 回應超過讀取上限 16 MiB（REQ-INV-14）", op.Method, op.Name)}
	}

	// 6b) Set-Cookie 合併回 XAuth 並同步 write-through（任何狀態碼、於
	//     回傳前完成，設計 §3.6）。
	s.absorbSetCookies(resp)

	// 6c) 非 2xx 分類（REQ-RD-003 raise_for_status 語義）；絕不清除
	//     cookie——清除決策在 app 層（REQ-LOG-042/043）。
	if status := resp.StatusCode; status < 200 || status >= 300 {
		return nil, classifyStatusError(op.Method, op.Name, status)
	}

	// 6d) 讀取語義（REQ-GQL-006/007）：有 data 即回（errors 並存不拋）；
	//     無 data 而有 errors → GraphQLError（Code 為首錯 code）；兩者皆
	//     無 → 不拋，回空 data。
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, &xerrors.TransportError{Msg: fmt.Sprintf("%s %s: 回應 JSON 解析失敗: %v", op.Method, op.Name, err)}
	}
	if data, ok := parsed["data"]; ok {
		return data, nil
	}
	if errsRaw, ok := parsed["errors"]; ok {
		var errs []xerrors.GraphErr
		if err := json.Unmarshal(errsRaw, &errs); err != nil {
			return nil, &xerrors.TransportError{Msg: fmt.Sprintf("%s %s: errors 欄位解析失敗: %v", op.Method, op.Name, err)}
		}
		gerr := &xerrors.GraphQLError{OperationName: op.Name, Errors: errs, ResJSON: body}
		if len(errs) > 0 {
			gerr.Code = errs[0].Code
		}
		return nil, gerr
	}
	return nil, nil
}

// classifyStatusError 將非 2xx 狀態碼分類（REQ-RD-003＋設計 §3.5）：
// 401/403 → 憑證失效（訊息含狀態碼語義）；429 屬限流而非 GraphQL 錯誤，
// 與其餘非 2xx 一律 → transport（訊息含狀態碼；重試策略屬呼叫端）。
func classifyStatusError(method, opName string, status int) error {
	if status == 401 || status == 403 {
		return &xerrors.CredentialInvalidError{Msg: fmt.Sprintf("%s %s: HTTP %d", method, opName, status)}
	}
	return &xerrors.TransportError{Msg: fmt.Sprintf("%s %s: HTTP %d", method, opName, status)}
}

// absorbSetCookies 解析回應的 Set-Cookie（k=v; attrs 取分號前段 k=v、
// trim），合併進 XAuth（REQ-AUTH-014：不刪既有鍵、空值鍵跳過——引擎絕
// 不清除 cookie，REQ-LOG-042/043）並同步 write-through：auth 接線了
// CookieStore 時，於回傳前以合併後的最新單行 Save（設計 §3.6）。Save
// 失敗不影響回應語義——cookie 已於 XAuth 記帳（v1 無日誌掛點）。此處的
// CookieHeader() 為落盤行讀取（非請求路徑的 csrf 配對，不違 Task 6
// caller rule；brief 對 write-through 逐字指定 store.Save(ctx, a.CookieHeader())）。
func (s *Sender) absorbSetCookies(resp *fhttp.Response) {
	values := resp.Header.Values("Set-Cookie")
	if len(values) == 0 {
		return
	}
	merged := make(map[string]string, len(values))
	for _, v := range values {
		kv, _, _ := strings.Cut(v, ";") // 屬性段丟棄，僅取 k=v
		k, val, ok := strings.Cut(strings.TrimSpace(kv), "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		merged[k] = strings.TrimSpace(val)
	}
	if len(merged) == 0 {
		return
	}
	s.auth.UpdateCookies(merged)
	if store := s.auth.Store(); store != nil {
		_ = store.Save(context.Background(), s.auth.CookieHeader())
	}
}
