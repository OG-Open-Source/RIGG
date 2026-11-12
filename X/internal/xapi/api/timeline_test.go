package api

// timeline 測試：以合成註冊表（Viewer／HomeTimeline 兩操作，query 型、
// 無 featureSwitches／fieldToggles——DECISIONS 探測節：X 接受無 features
// 的此類請求）與捕捉伺服器（基建複製自 send_test.go，同套件共用 helper）
// 驗證：雙形態變數組裝（REQ-RD-001、REQ-GQL-005）、游標與條目的深度遍歷
// 萃取（REQ-RD-012/013，不硬編 timeline 路徑）、去促銷與 work_id 去重
//（REQ-WEB-104 語義）、note_tweet 標題優先（REQ-DAT-014）、缺 rest_id 的
// 壞條目靜默跳過。fixture 一律合成（DECISIONS：不得複製 X 原文）；欄位名
//（entryId、cursorType、value、tweet_results、core.user_results.result、
// legacy.full_text、note_tweet.note_tweet_results.result.text、
// TweetWithVisibilityResults.tweet）為公開介面事實。

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/fingerprint"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/graphql"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/transport"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// timelineRegistryJSON 是本檔的合成註冊表 fixture（測試專用，非 X 原文）：
// Viewer 與 HomeTimeline 皆為 query 型、無 features／fieldToggles 宣告。
const timelineRegistryJSON = `{"generatedAt":"2026-11-12T00:00:00+08:00","frontendSha":null,
 "operations":[
  {"operationName":"Viewer","operationType":"query","queryId":"VIEWERID","featureSwitches":[],"fieldToggles":[]},
  {"operationName":"HomeTimeline","operationType":"query","queryId":"HOMEID","featureSwitches":[],"fieldToggles":[]}],
 "featureDefaults":{}}`

// loadTimelineRegistry 將合成註冊表寫入暫存檔並載入（走真檔案路徑）。
func loadTimelineRegistry(t *testing.T) *graphql.Registry {
	t.Helper()
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, []byte(timelineRegistryJSON), 0o644); err != nil {
		t.Fatalf("寫入時間線測試註冊表：%v", err)
	}
	reg, err := graphql.LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry error = %v, 期望 nil", err)
	}
	return reg
}

// newTimelineFixture 接線捕捉伺服器＋transport＋XAuth＋合成註冊表與受測
// Sender（baseOverride 白盒指向捕捉伺服器）；結構沿用 sendFixture。
func newTimelineFixture(t *testing.T, respond func(seq int, head, body []byte) []byte) *sendFixture {
	t.Helper()
	base, captured := newCaptureServer(t, respond)
	c, err := transport.New(fingerprint.Get(), transport.Options{ProxyURL: startProxyRelay(t)})
	if err != nil {
		t.Fatalf("transport.New 失敗：%v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	a := auth.NewAuth("bearer-timeline-test")
	if err := a.PrepareAuth("auth_token=TOKEN123; ct0=CT0VALUE456"); err != nil {
		t.Fatalf("PrepareAuth 失敗：%v", err)
	}
	reg := loadTimelineRegistry(t)
	s := NewSender(c, a, reg, fingerprint.Get())
	s.baseOverride = base // 白盒測試注入：目標指向本地捕捉伺服器
	return &sendFixture{base: base, auth: a, reg: reg, sender: s, captured: captured}
}

// timelineQueryVar 自 GET query string 取指定參數的 percent-decoded 值
// （EncodeQuery 僅編碼值、鍵原樣；同鍵多值取首個；查無回空字串）。
func timelineQueryVar(t *testing.T, q, key string) string {
	t.Helper()
	for _, kv := range strings.Split(q, "&") {
		k, v, ok := strings.Cut(kv, "=")
		if ok && k == key {
			s, err := url.QueryUnescape(v)
			if err != nil {
				t.Fatalf("query 參數 %s 解碼失敗：%v", key, err)
			}
			return s
		}
	}
	return ""
}

// jsonKeyOrder 以 token 流取 JSON 物件的鍵序（wire 原樣順序，不經重新
// 序列化）。
func jsonKeyOrder(t *testing.T, raw []byte) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("非 JSON：%v（raw=%q）", err, raw)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		t.Fatalf("應為 JSON 物件，得到 %v", tok)
	}
	var keys []string
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			t.Fatalf("讀取鍵失敗：%v", err)
		}
		keys = append(keys, kt.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatalf("跳過值失敗：%v", err)
		}
	}
	return keys
}

// tlItem 以 tweet_results.result 包裝單一貼文 result 物件。
func tlItem(result map[string]any) map[string]any {
	return map[string]any{"tweet_results": map[string]any{"result": result}}
}

// tlEntry 合成單一時間線條目：entryId ＋ content.itemContent。
func tlEntry(entryID string, item map[string]any) map[string]any {
	return map[string]any{"entryId": entryID, "content": map[string]any{"itemContent": item}}
}

// tlResult 合成一般貼文 result：rest_id＋legacy.full_text＋core 作者節點
// （識別值由 restID 派生，非 X 原文）。
func tlResult(restID, fullText string) map[string]any {
	return map[string]any{
		"rest_id": restID,
		"legacy":  map[string]any{"full_text": fullText},
		"core": map[string]any{
			"user_results": map[string]any{"result": map[string]any{
				"rest_id": "u-" + restID,
				"legacy":  map[string]any{"screen_name": "user_" + restID, "name": "User " + restID},
			}},
		},
	}
}

// tlTweetEntry 合成一般 tweet- 條目（含作者）。
func tlTweetEntry(entryID, restID, fullText string) map[string]any {
	return tlEntry(entryID, tlItem(tlResult(restID, fullText)))
}

// tlData 合成 data 信封內容：home.home_timeline_urt.instructions，每個
// section 為一段 TimelineAddEntries（形態僅供深度遍歷練習——引擎不硬編
// 此路徑，REQ-RD-012/013）。
func tlData(sections ...[]map[string]any) map[string]any {
	instructions := make([]any, 0, len(sections))
	for _, sec := range sections {
		entries := make([]any, 0, len(sec))
		for _, e := range sec {
			entries = append(entries, e)
		}
		instructions = append(instructions, map[string]any{"type": "TimelineAddEntries", "entries": entries})
	}
	return map[string]any{
		"home": map[string]any{
			"home_timeline_urt": map[string]any{"instructions": instructions},
		},
	}
}

// tlRespond 以合成 data 組捕捉伺服器回應（{"data":…} 信封，HTTP 200）。
func tlRespond(t *testing.T, data map[string]any) func(int, []byte, []byte) []byte {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("fixture 序列化失敗：%v", err)
	}
	body := `{"data":` + string(raw) + `}`
	return func(int, []byte, []byte) []byte {
		return jsonResponse(http.StatusOK, body)
	}
}

// TestHomeTimeline_GET_REQ_RD_001：cursor=""、seen=nil → wire 收到 GET、
// query 以 variables= 起始、變數含 "requestContext":"launch"（首頁啟動）、
// 無 seenTweetIds；共同變數 count／includePromotedContent／withCommunity
// 依序在場（契約 1，DECISIONS D1）。
func TestHomeTimeline_GET_REQ_RD_001(t *testing.T) {
	fx := newTimelineFixture(t, tlRespond(t, tlData()))
	res, err := fx.sender.HomeTimeline("", 20, nil)
	if err != nil {
		t.Fatalf("HomeTimeline error = %v, 期望 nil", err)
	}
	if res == nil {
		t.Fatal("HomeTimeline 不應回 nil result")
	}
	if len(res.Posts) != 0 || len(res.Users) != 0 || res.Cursor != "" {
		t.Errorf("空回應應得空結果，得到 %+v", res)
	}

	reqs := fx.captured()
	if len(reqs) != 1 {
		t.Fatalf("捕捉伺服器應收到 1 個請求，得到 %d", len(reqs))
	}
	req := reqs[0]
	if !strings.HasPrefix(req.requestLine(), "GET /i/api/graphql/HOMEID/HomeTimeline?") {
		t.Fatalf("應以 GET 送出至 op.Path()，請求行：%s", req.requestLine())
	}
	q := req.query()
	if !strings.HasPrefix(q, "variables=") {
		t.Errorf("query string 應以 variables= 起始，得到：%s", q)
	}
	vars := timelineQueryVar(t, q, "variables")
	keys := jsonKeyOrder(t, []byte(vars))
	want := []string{"count", "includePromotedContent", "withCommunity", "requestContext"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("variables 鍵序 = %v, 期望 %v（契約 1：共同變數＋requestContext）", keys, want)
	}
	if !strings.Contains(vars, `"requestContext":"launch"`) {
		t.Errorf("無已讀且無游標時應含 \"requestContext\":\"launch\"，得到：%s", vars)
	}
	if strings.Contains(vars, "seenTweetIds") {
		t.Errorf("GET 形態不得含 seenTweetIds，得到：%s", vars)
	}
	if !strings.Contains(vars, `"count":20`) {
		t.Errorf("count 應以數字上線，得到：%s", vars)
	}
}

// TestHomeTimeline_POST_REQ_GQL_005：seen 非空 → 覆寫為 POST（REQ-GQL-005：
// query 型操作顯式覆寫）、body 頂層鍵序 variables→queryId（無 features 宣告
// 時省略）、variables 內 seenTweetIds 為末鍵、無 requestContext；且註冊表
// 共用的 Operation.Method 不得被原地改寫。
func TestHomeTimeline_POST_REQ_GQL_005(t *testing.T) {
	fx := newTimelineFixture(t, tlRespond(t, tlData()))
	res, err := fx.sender.HomeTimeline("", 20, []string{"1770", "1771"})
	if err != nil {
		t.Fatalf("HomeTimeline error = %v, 期望 nil", err)
	}
	if res == nil {
		t.Fatal("HomeTimeline 不應回 nil result")
	}

	reqs := fx.captured()
	if len(reqs) != 1 {
		t.Fatalf("捕捉伺服器應收到 1 個請求，得到 %d", len(reqs))
	}
	req := reqs[0]
	if !strings.HasPrefix(req.requestLine(), "POST /i/api/graphql/HOMEID/HomeTimeline") {
		t.Fatalf("有已讀時應以 POST 送出，請求行：%s", req.requestLine())
	}
	topKeys := jsonKeyOrder(t, req.body)
	if want := []string{"variables", "queryId"}; !reflect.DeepEqual(topKeys, want) {
		t.Errorf("body 頂層鍵序 = %v, 期望 %v（無 features/fieldToggles 時省略）", topKeys, want)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(req.body, &parsed); err != nil {
		t.Fatalf("body 解析失敗：%v（body=%q）", err, req.body)
	}
	varKeys := jsonKeyOrder(t, parsed["variables"])
	want := []string{"count", "includePromotedContent", "withCommunity", "seenTweetIds"}
	if !reflect.DeepEqual(varKeys, want) {
		t.Errorf("variables 鍵序 = %v, 期望 %v（seenTweetIds 置於變數末，契約 1）", varKeys, want)
	}
	if strings.Contains(string(parsed["variables"]), "requestContext") {
		t.Errorf("POST 形態不得含 requestContext，得到：%s", parsed["variables"])
	}
	var vars map[string]json.RawMessage
	if err := json.Unmarshal(parsed["variables"], &vars); err != nil {
		t.Fatalf("variables 解析失敗：%v", err)
	}
	if string(vars["seenTweetIds"]) != `["1770","1771"]` {
		t.Errorf("seenTweetIds = %s, 期望 [\"1770\",\"1771\"]（字串陣列）", vars["seenTweetIds"])
	}

	op, err := fx.reg.Operation("HomeTimeline")
	if err != nil {
		t.Fatalf("Operation(HomeTimeline) error = %v, 期望 nil", err)
	}
	if op.Method != "GET" {
		t.Errorf("註冊表共用的 Operation.Method 不得被覆寫，得到 %q", op.Method)
	}
}

// TestHomeTimeline_Cursor：cursor 非空、seen=nil → 仍為 GET，variables 含
// cursor 值且無 requestContext（cursor 與 requestContext 互斥，契約 1）。
func TestHomeTimeline_Cursor(t *testing.T) {
	fx := newTimelineFixture(t, tlRespond(t, tlData()))
	if _, err := fx.sender.HomeTimeline("DAABCgAB", 20, nil); err != nil {
		t.Fatalf("HomeTimeline error = %v, 期望 nil", err)
	}

	reqs := fx.captured()
	if len(reqs) != 1 {
		t.Fatalf("捕捉伺服器應收到 1 個請求，得到 %d", len(reqs))
	}
	req := reqs[0]
	if !strings.HasPrefix(req.requestLine(), "GET /i/api/graphql/HOMEID/HomeTimeline?") {
		t.Fatalf("有游標無已讀時應仍以 GET 送出，請求行：%s", req.requestLine())
	}
	vars := timelineQueryVar(t, req.query(), "variables")
	keys := jsonKeyOrder(t, []byte(vars))
	want := []string{"count", "includePromotedContent", "withCommunity", "cursor"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("variables 鍵序 = %v, 期望 %v（cursor 變數在場）", keys, want)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(vars), &parsed); err != nil {
		t.Fatalf("variables 解析失敗：%v（vars=%q）", err, vars)
	}
	if string(parsed["cursor"]) != `"DAABCgAB"` {
		t.Errorf("cursor = %s, 期望 \"DAABCgAB\"", parsed["cursor"])
	}
	if _, ok := parsed["requestContext"]; ok {
		t.Errorf("cursor 非空時不得含 requestContext，得到：%s", vars)
	}
	if _, ok := parsed["seenTweetIds"]; ok {
		t.Errorf("seen=nil 時不得含 seenTweetIds，得到：%s", vars)
	}
}

// TestExtractCursorDeep_REQ_RD_012：深層嵌套含 cursorType:"Bottom"、
// value:"CUR" → "CUR"；cursorType:"Top" 不誤取；value:"" 視同無；多個
// Bottom 取遍歷序首個；方向參數生效；無游標回空字串。
func TestExtractCursorDeep_REQ_RD_012(t *testing.T) {
	root := map[string]any{
		"data": map[string]any{
			"a": map[string]any{
				"b": map[string]any{
					"c": map[string]any{
						"entries": []any{
							map[string]any{"entryId": "cursor-top-0", "content": map[string]any{"cursorType": "Top", "value": "TOPVAL"}},
							map[string]any{"entryId": "tweet-0", "content": map[string]any{"itemContent": tlItem(map[string]any{"rest_id": "1"})}},
							map[string]any{"entryId": "cursor-bottom-0", "content": map[string]any{"cursorType": "Bottom", "value": ""}},
							map[string]any{"entryId": "cursor-bottom-1", "content": map[string]any{"itemContent": map[string]any{"cursorType": "Bottom", "value": "CUR"}}},
							map[string]any{"entryId": "cursor-bottom-2", "content": map[string]any{"cursorType": "Bottom", "value": "SECOND"}},
						},
					},
				},
			},
		},
	}
	if got := ExtractCursor(root, "Bottom"); got != "CUR" {
		t.Errorf("ExtractCursor(Bottom) = %q, 期望 \"CUR\"（Top 不誤取、空 value 視同無、首個命中）", got)
	}
	if got := ExtractCursor(root, "Top"); got != "TOPVAL" {
		t.Errorf("ExtractCursor(Top) = %q, 期望 \"TOPVAL\"（方向參數生效）", got)
	}
	if got := ExtractCursor(map[string]any{"x": []any{map[string]any{"y": 1}}}, "Bottom"); got != "" {
		t.Errorf("無游標時應回空字串，得到 %q", got)
	}
	if got := ExtractCursor(nil, "Bottom"); got != "" {
		t.Errorf("nil 樹應回空字串，得到 %q", got)
	}
}

// TestExtractTweets_REQ_RD_013：tweet- 前綴三條（其一深嵌於 conversation
// 模組、其一經 TweetWithVisibilityResults 包裹）＋cursor- 一條＋promoted-
// 一條 → 恰三條 result；包裹者解包為內層 tweet；順序＝遍歷序。
func TestExtractTweets_REQ_RD_013(t *testing.T) {
	inner := map[string]any{"rest_id": "2002", "legacy": map[string]any{"full_text": "inner-b"}}
	root := map[string]any{"data": map[string]any{"instructions": []any{
		map[string]any{"entries": []any{
			tlTweetEntry("tweet-1", "2001", "first"),
			tlEntry("promoted-tweet-9", tlItem(map[string]any{
				"rest_id":          "9009",
				"promotedMetadata": map[string]any{"advertiser": "x"},
			})),
			tlEntry("tweet-2", tlItem(map[string]any{
				"__typename": "TweetWithVisibilityResults",
				"tweet":      inner,
			})),
			map[string]any{"entryId": "conversationthread-0", "content": map[string]any{
				"items": []any{
					tlTweetEntry("tweet-3", "2003", "third"),
				},
			}},
			tlEntry("cursor-bottom-0", map[string]any{"cursorType": "Bottom", "value": "CUR"}),
		}},
	}}}
	got := ExtractTweets(root)
	if len(got) != 3 {
		t.Fatalf("應恰收集 3 條 result（tweet- 前綴；cursor-/promoted- 排除），得到 %d：%+v", len(got), got)
	}
	m0, ok := got[0].(map[string]any)
	if !ok || m0["rest_id"] != "2001" {
		t.Errorf("第 1 條應為 rest_id 2001 的 result，得到 %+v", got[0])
	}
	m1, ok := got[1].(map[string]any)
	if !ok || m1["rest_id"] != "2002" {
		t.Errorf("第 2 條應為解包後的內層 tweet（rest_id 2002），得到 %+v", got[1])
	} else if _, wrapped := m1["tweet"]; wrapped {
		t.Errorf("TweetWithVisibilityResults 應解包為內層物件，仍含 tweet 鍵：%+v", m1)
	}
	m2, ok := got[2].(map[string]any)
	if !ok || m2["rest_id"] != "2003" {
		t.Errorf("第 3 條應為深嵌於 conversation 模組的 tweet-3（rest_id 2003），得到 %+v", got[2])
	}
	for i, r := range got {
		if m, ok := r.(map[string]any); ok {
			if _, bad := m["promotedMetadata"]; bad {
				t.Errorf("第 %d 條不應為促銷 result：%+v", i, m)
			}
		}
	}
	if got := ExtractTweets(nil); len(got) != 0 {
		t.Errorf("nil 樹應得空結果，得到 %d", len(got))
	}
}

// TestTimelinePromotedAndDedup_REQ_WEB_104：單一回應含兩段（模擬兩頁內容
// 同樹）且共用同一 work_id、一條 promoted- 前綴條目、一條 result 帶
// promotedMetadata 的 tweet- 條目 → 輸出去重去促銷、順序保留首見
// （REQ-WEB-104 語義；跨頁去重由呼叫端以 seenTweetIDs 累積，設計 §25）。
func TestTimelinePromotedAndDedup_REQ_WEB_104(t *testing.T) {
	data := tlData(
		[]map[string]any{
			tlTweetEntry("tweet-1", "3001", "alpha"),
			tlEntry("promoted-tweet-8", tlItem(map[string]any{
				"rest_id":          "3008",
				"legacy":           map[string]any{"full_text": "ad"},
				"promotedMetadata": map[string]any{"advertiser": "x"},
			})),
		},
		[]map[string]any{
			tlTweetEntry("tweet-1-again", "3001", "alpha"), // 同 work_id 重現 → 去重
			tlEntry("tweet-2", tlItem(func() map[string]any {
				r := tlResult("3002", "beta")
				r["promotedMetadata"] = map[string]any{"advertiser": "y"} // result 含 promotedMetadata → 排除
				return r
			}())),
			tlTweetEntry("tweet-3", "3003", "gamma"),
			tlEntry("cursor-bottom-0", map[string]any{"cursorType": "Bottom", "value": "BOTTOMCUR"}),
		},
	)
	fx := newTimelineFixture(t, tlRespond(t, data))
	res, err := fx.sender.HomeTimeline("", 20, nil)
	if err != nil {
		t.Fatalf("HomeTimeline error = %v, 期望 nil", err)
	}
	wantPosts := []Post{
		{WorkID: "3001", WorkURL: "https://x.com/user_3001/status/3001", ScreenName: "user_3001", Title: "alpha"},
		{WorkID: "3003", WorkURL: "https://x.com/user_3003/status/3003", ScreenName: "user_3003", Title: "gamma"},
	}
	if !reflect.DeepEqual(res.Posts, wantPosts) {
		t.Errorf("Posts = %+v, 期望 %+v（去促銷、work_id 去重、首見順序）", res.Posts, wantPosts)
	}
	wantUsers := []User{
		{UserID: "u-3001", ScreenName: "user_3001", Name: "User 3001"},
		{UserID: "u-3003", ScreenName: "user_3003", Name: "User 3003"},
	}
	if !reflect.DeepEqual(res.Users, wantUsers) {
		t.Errorf("Users = %+v, 期望 %+v（作者隨去重去促銷，首見順序）", res.Users, wantUsers)
	}
	if res.Cursor != "BOTTOMCUR" {
		t.Errorf("Cursor = %q, 期望 \"BOTTOMCUR\"（ExtractCursor(data, \"Bottom\")）", res.Cursor)
	}
}

// TestPostTitleNoteTweet_REQ_DAT_014：有 note_tweet → 標題取
// note_tweet.note_tweet_results.result.text（長推的 legacy.full_text 僅為
// 截斷預覽）；僅 legacy.full_text → 取之。
func TestPostTitleNoteTweet_REQ_DAT_014(t *testing.T) {
	long := tlResult("4001", "TRUNC PREVIEW…")
	long["note_tweet"] = map[string]any{
		"note_tweet_results": map[string]any{"result": map[string]any{"text": "FULL NOTE TEXT"}},
	}
	data := tlData([]map[string]any{
		tlEntry("tweet-1", tlItem(long)),
		tlTweetEntry("tweet-2", "4002", "PLAIN FULL TEXT"),
	})
	fx := newTimelineFixture(t, tlRespond(t, data))
	res, err := fx.sender.HomeTimeline("", 20, nil)
	if err != nil {
		t.Fatalf("HomeTimeline error = %v, 期望 nil", err)
	}
	if len(res.Posts) != 2 {
		t.Fatalf("應得 2 條貼文，得到 %d：%+v", len(res.Posts), res.Posts)
	}
	if res.Posts[0].Title != "FULL NOTE TEXT" {
		t.Errorf("有 note_tweet 時 Title = %q, 期望 \"FULL NOTE TEXT\"（note 優先）", res.Posts[0].Title)
	}
	if res.Posts[1].Title != "PLAIN FULL TEXT" {
		t.Errorf("僅 full_text 時 Title = %q, 期望 \"PLAIN FULL TEXT\"", res.Posts[1].Title)
	}
	if res.Posts[0].WorkID != "4001" || res.Posts[1].WorkID != "4002" {
		t.Errorf("WorkID 應取自 rest_id，得到 %q／%q", res.Posts[0].WorkID, res.Posts[1].WorkID)
	}
}

// TestViewer_REQ_RD：合成 Viewer 回應 → ViewerInfo{UserID, ScreenName,
// Name}；wire 為 GET /VIEWERID/Viewer、variables 僅含
// withCommunitiesMemberships:true（無 features——DECISIONS 探測節）；回應
// 缺 rest_id → *xerrors.MissingFieldError。
func TestViewer_REQ_RD(t *testing.T) {
	fx := newTimelineFixture(t, func(seq int, _, _ []byte) []byte {
		if seq == 1 {
			return jsonResponse(http.StatusOK,
				`{"data":{"viewer":{"results":{"rest_id":"9001","core":{"screen_name":"go_engine","name":"Go Engine"},"__typename":"User"}}}}`)
		}
		return jsonResponse(http.StatusOK,
			`{"data":{"viewer":{"results":{"core":{"screen_name":"x","name":"y"}}}}}`)
	})

	info, err := fx.sender.Viewer()
	if err != nil {
		t.Fatalf("Viewer error = %v, 期望 nil", err)
	}
	want := ViewerInfo{UserID: "9001", ScreenName: "go_engine", Name: "Go Engine"}
	if info != want {
		t.Errorf("ViewerInfo = %+v, 期望 %+v", info, want)
	}

	reqs := fx.captured()
	if len(reqs) != 1 {
		t.Fatalf("捕捉伺服器應收到 1 個請求，得到 %d", len(reqs))
	}
	if !strings.HasPrefix(reqs[0].requestLine(), "GET /i/api/graphql/VIEWERID/Viewer?") {
		t.Fatalf("Viewer 應以 GET 送出至 op.Path()，請求行：%s", reqs[0].requestLine())
	}
	q := reqs[0].query()
	if !strings.HasPrefix(q, "variables=") || strings.Contains(q, "features=") || strings.Contains(q, "fieldToggles=") {
		t.Errorf("query 應僅含 variables=（無 features/fieldToggles），得到：%s", q)
	}
	if vars := timelineQueryVar(t, q, "variables"); vars != `{"withCommunitiesMemberships":true}` {
		t.Errorf("variables = %s, 期望 {\"withCommunitiesMemberships\":true}", vars)
	}

	if _, err := fx.sender.Viewer(); err == nil {
		t.Fatal("缺 rest_id 時應回錯誤，得到 nil")
	} else {
		var mfe *xerrors.MissingFieldError
		if !errors.As(err, &mfe) {
			t.Fatalf("錯誤應可 errors.As 為 *xerrors.MissingFieldError，得到 %T：%v", err, err)
		}
		if mfe.Kind() != xerrors.KindInput {
			t.Errorf("MissingFieldError.Kind() = %s, 期望 %s", mfe.Kind(), xerrors.KindInput)
		}
	}
}

// TestPostUser_CoreFallback_L2：L2 實證形狀——作者物件
// （core.user_results.result）已無 legacy，screen_name/name 攜於作者物件
// 自身的 core（與 Viewer 結果同形）→ postUser 回退讀 core：Post.ScreenName
// 非空、WorkURL 正確組出；legacy 與 core 並存 → legacy 優先（純回退，優先
// 序不變）；legacy 對應欄位空 → 僅該欄位回退 core；缺 rest_id 跳過語義
// 不變。
func TestPostUser_CoreFallback_L2(t *testing.T) {
	// L2 實證形狀：tweet result 的 core.user_results.result 容器不變
	//（rest_id 照常萃取），作者物件無 legacy 鍵、screen_name/name 攜於
	// 作者物件自身的 core（與 Viewer 結果同形）。
	coreOnly := map[string]any{
		"rest_id": "6001",
		"legacy":  map[string]any{"full_text": "alpha"},
		"core": map[string]any{
			"user_results": map[string]any{"result": map[string]any{
				"rest_id": "7001",
				"core":    map[string]any{"screen_name": "someone", "name": "Someone"}, // 無 legacy 鍵
			}},
		},
	}
	both := map[string]any{
		"rest_id": "6002",
		"legacy":  map[string]any{"full_text": "beta"},
		"core": map[string]any{
			"user_results": map[string]any{"result": map[string]any{
				"rest_id": "7002",
				"legacy":  map[string]any{"screen_name": "legacy_sn", "name": "Legacy Name"},
				"core":    map[string]any{"screen_name": "core_sn", "name": "Core Name"},
			}},
		},
	}
	partial := map[string]any{
		"rest_id": "6003",
		"legacy":  map[string]any{"full_text": "gamma"},
		"core": map[string]any{
			"user_results": map[string]any{"result": map[string]any{
				"rest_id": "7003",
				"legacy":  map[string]any{"screen_name": "", "name": "Partial Name"}, // 對應欄位空
				"core":    map[string]any{"screen_name": "core_sn2", "name": "Core Name2"},
			}},
		},
	}
	data := tlData([]map[string]any{
		tlEntry("tweet-1", tlItem(coreOnly)),
		tlEntry("tweet-2", tlItem(both)),
		tlEntry("tweet-3", tlItem(partial)),
	})
	fx := newTimelineFixture(t, tlRespond(t, data))
	res, err := fx.sender.HomeTimeline("", 20, nil)
	if err != nil {
		t.Fatalf("HomeTimeline error = %v, 期望 nil", err)
	}
	wantPosts := []Post{
		{WorkID: "6001", WorkURL: "https://x.com/someone/status/6001", ScreenName: "someone", Title: "alpha"},
		{WorkID: "6002", WorkURL: "https://x.com/legacy_sn/status/6002", ScreenName: "legacy_sn", Title: "beta"},
		{WorkID: "6003", WorkURL: "https://x.com/core_sn2/status/6003", ScreenName: "core_sn2", Title: "gamma"},
	}
	if !reflect.DeepEqual(res.Posts, wantPosts) {
		t.Errorf("Posts = %+v, 期望 %+v（無 legacy → core 回退；並存 → legacy 優先；欄位空 → 單欄回退）", res.Posts, wantPosts)
	}
	wantUsers := []User{
		{UserID: "7001", ScreenName: "someone", Name: "Someone"},
		{UserID: "7002", ScreenName: "legacy_sn", Name: "Legacy Name"},
		{UserID: "7003", ScreenName: "core_sn2", Name: "Partial Name"},
	}
	if !reflect.DeepEqual(res.Users, wantUsers) {
		t.Errorf("Users = %+v, 期望 %+v", res.Users, wantUsers)
	}
}

// TestTimelineBadEntrySkipped：某 tweet 條目缺 rest_id → 視為結構不符，
// 靜默跳過不拋錯，其餘條目照收（時間線容忍個別壞條目）。
func TestTimelineBadEntrySkipped(t *testing.T) {
	data := tlData([]map[string]any{
		tlTweetEntry("tweet-1", "5001", "ok"),
		tlEntry("tweet-bad", tlItem(map[string]any{
			"legacy": map[string]any{"full_text": "no rest id"},
		})),
		tlTweetEntry("tweet-2", "5002", "fine"),
	})
	fx := newTimelineFixture(t, tlRespond(t, data))
	res, err := fx.sender.HomeTimeline("", 20, nil)
	if err != nil {
		t.Fatalf("HomeTimeline error = %v, 期望 nil（壞條目不拋錯）", err)
	}
	wantPosts := []Post{
		{WorkID: "5001", WorkURL: "https://x.com/user_5001/status/5001", ScreenName: "user_5001", Title: "ok"},
		{WorkID: "5002", WorkURL: "https://x.com/user_5002/status/5002", ScreenName: "user_5002", Title: "fine"},
	}
	if !reflect.DeepEqual(res.Posts, wantPosts) {
		t.Errorf("Posts = %+v, 期望 %+v（缺 rest_id 條目跳過、其餘照收）", res.Posts, wantPosts)
	}
	wantUsers := []User{
		{UserID: "u-5001", ScreenName: "user_5001", Name: "User 5001"},
		{UserID: "u-5002", ScreenName: "user_5002", Name: "User 5002"},
	}
	if !reflect.DeepEqual(res.Users, wantUsers) {
		t.Errorf("Users = %+v, 期望 %+v", res.Users, wantUsers)
	}
	if res.Cursor != "" {
		t.Errorf("無游標時 Cursor 應為空字串，得到 %q", res.Cursor)
	}
}
