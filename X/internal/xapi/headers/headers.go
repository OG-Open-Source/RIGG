package headers

import (
	"fmt"
	"sort"
	"strings"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/fingerprint"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// Family 是六種標頭族（REQ-HDR-001）：GraphQL GET／GraphQL POST 於 v1
// 實作輸出，其餘四族為 v2 填表點（Build 回未支援錯誤）。
type Family int

const (
	// FamGraphQLGet 是 GraphQL GET 族（v1 中 query 型 GET 亦用 POST 族
	// 標頭——REQ-RD-001「POST 族標頭＋GET 方法」）。
	FamGraphQLGet Family = iota
	// FamGraphQLPost 是 GraphQL POST 族。
	FamGraphQLPost
	// FamRESTGet 是 REST GET 族（REQ-HDR-004，v2 填表）。
	FamRESTGet
	// FamRESTForm 是 REST FORM 族（REQ-HDR-005，v2 填表）。
	FamRESTForm
	// FamUpload 是 UPLOAD 族（REQ-HDR-006，v2 填表）。
	FamUpload
	// FamDoc 是 DOC 族（REQ-HDR-007，v2 填表）。
	FamDoc
)

// Slot 是單一輸出標頭：Name 全小寫（REQ-HDR-013），Value 逐字。
type Slot struct {
	Name  string
	Value string
}

// slotSpec 是族表的一列：小寫槽位名與取值器。取值器回 (value, present)，
// present=false 表示該槽缺席、整列跳過（可選槽位缺席不重塑序列，
// REQ-HDR-003「缺席者整列跳過，其餘維持相對順序」）。
type slotSpec struct {
	name    string
	resolve func(c *buildCtx) (string, bool)
}

// buildCtx 聚集取值器可見的三個輸入：bearer 參數、單次 cookieLine 載入
// 的解析結果、呼叫端給的 slots。fp 不入 ctx：v1 的 GraphQL 族值集不取用
// 指紋欄位（Build 的 fp 參數保留給 v2 REST/UPLOAD 族的 accept-language
// 預設），故無需傳遞。
type buildCtx struct {
	bearer  string
	cookies map[string]string
	slots   map[string]string
}

// fromSlots 取可選槽位的呼叫端值：鍵以小寫給定；「出現」以非空值為準
// （空值視同缺席，同 auth.UpdateCookies 的空值鍵語義）。僅在 map 中出現
// 時輸出（task-7-brief：可選槽位）。
func (c *buildCtx) fromSlots(key string) (string, bool) {
	v, ok := c.slots[key]
	return v, ok && v != ""
}

// graphqlTable 是 REQ-HDR-003 的 GraphQL 族逐字序列，GET 與 POST 共用同
// 一張表（兩者差異僅 HTTP 方法，無標頭差異——content-type 恆為
// application/json，POST 族標頭的字面語義）。
//
// 條件在場規則（REQ-HDR-008..011）：csrf 逐字取自本次 cookieLine 載入的
// ct0（與 Cookie 標頭同一行，Task 6 caller rule）；auth-type 僅登入；
// client-language 取 lang 小寫；x-att 僅 cookie 有 att 才輸出（slots 鍵
// 僅作值覆寫）。XCTID 與 x-guest-token 僅由 slots 給出（XCTID 探測通過，
// v1 恒不填，槽位保留）。
var graphqlTable = []slotSpec{
	{name: "content-type", resolve: func(c *buildCtx) (string, bool) {
		return "application/json", true // 恒在（REQ-HDR-003）
	}},
	{name: "authorization", resolve: func(c *buildCtx) (string, bool) {
		return "Bearer " + c.bearer, true // 恒在（值來自參數）
	}},
	{name: "x-twitter-auth-type", resolve: func(c *buildCtx) (string, bool) {
		if c.cookies["auth_token"] == "" {
			return "", false // REQ-HDR-009：僅已登入，未登入送出會被拒
		}
		return "OAuth2Session", true
	}},
	{name: "x-csrf-token", resolve: func(c *buildCtx) (string, bool) {
		v := c.cookies["ct0"]
		return v, v != "" // REQ-HDR-008：同源於同一行 cookieLine，逐字一致
	}},
	{name: "x-twitter-client-language", resolve: func(c *buildCtx) (string, bool) {
		v := c.cookies["lang"]
		if v == "" {
			return "", false // REQ-HDR-010：缺失或空則不送
		}
		return strings.ToLower(v), true
	}},
	{name: "x-twitter-active-user", resolve: func(c *buildCtx) (string, bool) {
		return "yes", true // 恒在
	}},
	{name: "x-client-transaction-id", resolve: func(c *buildCtx) (string, bool) {
		return c.fromSlots("x-client-transaction-id") // 可選槽位；v1 恒缺席
	}},
	{name: "x-guest-token", resolve: func(c *buildCtx) (string, bool) {
		return c.fromSlots("x-guest-token") // 可選槽位
	}},
	{name: "x-att", resolve: func(c *buildCtx) (string, bool) {
		att := c.cookies["att"]
		if att == "" {
			return "", false // REQ-HDR-011：僅 cookie 存在 att 時加入
		}
		if v, ok := c.fromSlots("x-att"); ok {
			return v, true // slots 鍵作值覆寫（在場仍由 cookie 決定）
		}
		return att, true
	}},
	{name: "origin", resolve: func(c *buildCtx) (string, bool) {
		return "https://x.com", true // 恒在
	}},
	{name: "referer", resolve: func(c *buildCtx) (string, bool) {
		return c.fromSlots("referer") // 可選槽位（值由呼叫端給）
	}},
}

// familyTables 是六族資料表（REQ-HDR-001，以 Family 常數索引）：v2 補族
// ＝填表。nil 表即「未在 v1 實作」，Build 回明確未支援錯誤。
var familyTables = [][]slotSpec{
	FamGraphQLGet:  graphqlTable,
	FamGraphQLPost: graphqlTable,
	FamRESTGet:     nil, // REQ-HDR-004：v2 填表
	FamRESTForm:    nil, // REQ-HDR-005：v2 填表
	FamUpload:      nil, // REQ-HDR-006：v2 填表
	FamDoc:         nil, // REQ-HDR-007：v2 填表
}

// Build 以族表組裝有序槽位序列：遍歷該族 slotSpec，取值器決定在場與否；
// slots 內未登記的額外鍵依字母序追加於尾（REQ-HDR-002），名稱一律小寫
// （REQ-HDR-013）。cookieLine 以單次載入解析（與呼叫端注入 Cookie 標頭
// 的同一行），csrf/att 同源於此（REQ-HDR-008 的請求層級保證——參見
// auth.CookieHeader 的 caller rule）。未實作族回 *xerrors.InvalidInputError
// （kind=input：呼叫端選了 v1 不存在的族，不重試、不清憑證）。
func Build(f Family, bearer, cookieLine string, fp *fingerprint.Fingerprint, slots map[string]string) ([]Slot, error) {
	if f < 0 || int(f) >= len(familyTables) || familyTables[f] == nil {
		return nil, &xerrors.InvalidInputError{
			Msg: fmt.Sprintf("headers: 此標頭族未在 v1 實作（family=%d）", int(f)),
		}
	}
	_ = fp // v1 GraphQL 族值集不取用指紋；參數保留給 v2 REST/UPLOAD 族
	ctx := &buildCtx{bearer: bearer, cookies: parseCookieLine(cookieLine), slots: slots}
	table := familyTables[f]

	out := make([]Slot, 0, len(table))
	registered := make(map[string]struct{}, len(table))
	for _, spec := range table {
		registered[spec.name] = struct{}{}
		if v, ok := spec.resolve(ctx); ok {
			out = append(out, Slot{Name: spec.name, Value: v})
		}
	}

	// 未登記的額外鍵：依字母序追加於後（REQ-HDR-002）。名稱小寫化
	// （REQ-HDR-013 對所有輸出標頭名生效）；空鍵、空值與（小寫後）已
	// 登記的鍵跳過——已登記槽位的值一律由族表取值器決定，slots 不可
	// 覆寫固定槽位（content-type／authorization／x-twitter-active-user／
	// origin）與 cookie 衍生槽位（auth-type／csrf／client-language）。
	type extra struct{ raw, name, value string }
	var extras []extra
	for k, v := range slots {
		name := strings.ToLower(k)
		if k == "" || v == "" {
			continue
		}
		if _, dup := registered[name]; dup {
			continue
		}
		extras = append(extras, extra{raw: k, name: name, value: v})
	}
	sort.Slice(extras, func(i, j int) bool {
		if extras[i].name != extras[j].name {
			return extras[i].name < extras[j].name
		}
		return extras[i].raw < extras[j].raw // 小寫後同名的決定性次序
	})
	for _, e := range extras {
		out = append(out, Slot{Name: e.name, Value: e.value})
	}
	return out, nil
}

// parseCookieLine 解析單行 cookie 字串為映射（與 auth 套件 Task 6 的解析
// 語義一致，REQ-AUTH-003）：以 ; 分段、trim 空白、空段／無 = 段／空鍵段
// 忽略、k=v 以第一個 = 切分（值可含 =，如 ct0=a=b）。空輸入回空映射。
// 私有複製而非共用：headers 只需要解析，不得反向依賴 auth 的狀態容器
// （Build 收 cookieLine 字串正是為了單次載入同源，見 Build 註解）。
func parseCookieLine(s string) map[string]string {
	cookies := make(map[string]string)
	for _, seg := range strings.Split(s, ";") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		k, v, ok := strings.Cut(seg, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		cookies[k] = strings.TrimSpace(v)
	}
	return cookies
}
