package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/graphql"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// maxWalkDepth 是深度遍歷的遞迴深度上限，取 encoding/json 的最大嵌套深度
// （10000）：Send 的 data 已過該解析器，深度必然受限；設上限使 walk 對任何
// 輸入（含理論上成環的非 JSON Go 值——JSON 樹經 unmarshal 後不可成環）皆
// 保證終止，不受敵意輸入觸發堆疊溢位。
const maxWalkDepth = 10000

// Post 是時間線輸出的單一貼文：WorkID 取自 result.rest_id；WorkURL 為
// "https://x.com/" + ScreenName + "/status/" + WorkID（REQ-DAT-014 語義的
// 識別三元組）；Title 依 REQ-DAT-014（postTitle）；Media 為
// extended_entities.media 萃取（REQ-DAT-015/016，見 media.go——無媒體時
// 為 nil）。
type Post struct {
	WorkID     string
	WorkURL    string
	ScreenName string
	Title      string
	Media      []MediaItem
}

// User 是貼文作者的公開身分（自 core.user_results.result 萃取）。
type User struct {
	UserID     string
	ScreenName string
	Name       string
}

// TimelineResult 是一次時間線讀取的解析結果：Posts／Users 皆為首見順序
// （去促銷與 work_id 去重後，REQ-WEB-104 語義）；Cursor 為 Bottom 游標，
// 空字串表示無游標（呼叫端停止翻頁）。
type TimelineResult struct {
	Posts  []Post
	Users  []User
	Cursor string
}

// ViewerInfo 是當前會話身分（Viewer 回應深度遍歷萃取）。
type ViewerInfo struct {
	UserID     string
	ScreenName string
	Name       string
}

// walk 以先序深度遍歷 root 的整棵樹（map/slice 任意嵌套），對每個物件節點
// 呼叫 visit。遍歷序為決定性：slice 依索引、map 依鍵名排序——「首個命中」
// 因此可重現。不假設任何 timeline 路徑（REQ-RD-012/013）：X 會在回應形態
// 間搬移條目，萃取一律走整棵 data 樹，以節點自帶的識別欄位
// （entryId／cursorType／value）定位。
func walk(root any, visit func(map[string]any)) {
	var rec func(node any, depth int)
	rec = func(node any, depth int) {
		if depth > maxWalkDepth {
			return
		}
		switch v := node.(type) {
		case map[string]any:
			visit(v)
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				rec(v[k], depth+1)
			}
		case []any:
			for _, e := range v {
				rec(e, depth+1)
			}
		}
	}
	rec(root, 0)
}

// ExtractCursor 深度遍歷 root，於任何節點的 content 或 content.itemContent
// 內尋找 cursorType == direction 且 value 非空的游標（REQ-RD-012），回傳
// 遍歷序首個命中者的 value；找不到（含 value 為空字串者）回空字串——呼叫
// 端據此停止翻頁。
func ExtractCursor(root any, direction string) string {
	found := ""
	walk(root, func(m map[string]any) {
		if found != "" {
			return
		}
		content, ok := m["content"].(map[string]any)
		if !ok {
			return
		}
		if ct, _ := content["cursorType"].(string); ct == direction {
			if v, _ := content["value"].(string); v != "" {
				found = v
				return
			}
		}
		item, ok := content["itemContent"].(map[string]any)
		if !ok {
			return
		}
		if ct, _ := item["cursorType"].(string); ct == direction {
			if v, _ := item["value"].(string); v != "" {
				found = v
			}
		}
	})
	return found
}

// ExtractTweets 深度遍歷 root，收集 entryId 以 "tweet-" 開頭的節點之
// content.itemContent.tweet_results.result（REQ-RD-013）；result 為
// TweetWithVisibilityResults 形態（內含 tweet 物件）時解包為內層貼文。
// 順序＝遍歷序（slice 索引序為主）；promoted- 前綴條目因前綴不符自然排除，
// promotedMetadata 的排除屬 HomeTimeline 的輸出語義（REQ-WEB-104）。
func ExtractTweets(root any) []any {
	var out []any
	walk(root, func(m map[string]any) {
		entryID, ok := m["entryId"].(string)
		if !ok || !strings.HasPrefix(entryID, "tweet-") {
			return
		}
		content, ok := m["content"].(map[string]any)
		if !ok {
			return
		}
		item, ok := content["itemContent"].(map[string]any)
		if !ok {
			return
		}
		results, ok := item["tweet_results"].(map[string]any)
		if !ok {
			return
		}
		result, ok := results["result"].(map[string]any)
		if !ok {
			return
		}
		if inner, ok := result["tweet"].(map[string]any); ok {
			result = inner // TweetWithVisibilityResults.tweet 解包
		}
		out = append(out, result)
	})
	return out
}

// postTitle 依 REQ-DAT-014 取標題：note_tweet.note_tweet_results.result.text
// 優先（長推的 legacy.full_text 僅為截斷預覽），否則 legacy.full_text；
// 兩者皆缺回空字串。
func postTitle(m map[string]any) string {
	if note, ok := m["note_tweet"].(map[string]any); ok {
		if results, ok := note["note_tweet_results"].(map[string]any); ok {
			if result, ok := results["result"].(map[string]any); ok {
				if text, _ := result["text"].(string); text != "" {
					return text
				}
			}
		}
	}
	legacy, _ := m["legacy"].(map[string]any)
	text, _ := legacy["full_text"].(string)
	return text
}

// userDisplayField 自作者物件（core.user_results.result）取顯示欄位 key
// （screen_name／name）：legacy 優先（legacy.<key>）；L2 實證形狀下作者
// 物件已無 legacy、core 直接攜顯示欄位，故 legacy 缺失或對應欄位空時回退
// 讀作者物件自身的 core.<key>（與 Viewer 結果同形）；兩者皆缺回空字串。
// 純回退：legacy 優先序不變。
func userDisplayField(result map[string]any, key string) string {
	if legacy, ok := result["legacy"].(map[string]any); ok {
		if v, _ := legacy[key].(string); v != "" {
			return v
		}
	}
	if core, ok := result["core"].(map[string]any); ok {
		if v, _ := core[key].(string); v != "" {
			return v
		}
	}
	return ""
}

// postUser 自貼文 result 的 core.user_results.result 萃取作者（rest_id；
// screen_name／name 依 userDisplayField：legacy 優先、作者物件自身 core
// 回退）；結構不符時回零值 User（呼叫端依 UserID 空值跳過）。
func postUser(m map[string]any) User {
	core, _ := m["core"].(map[string]any)
	userResults, _ := core["user_results"].(map[string]any)
	result, _ := userResults["result"].(map[string]any)
	if result == nil {
		return User{}
	}
	userID, _ := result["rest_id"].(string)
	return User{
		UserID:     userID,
		ScreenName: userDisplayField(result, "screen_name"),
		Name:       userDisplayField(result, "name"),
	}
}

// parseTimelineResult 是時間線操作的共用解析尾段：Send 回傳的 data 解析為
// TimelineResult——游標取 ExtractCursor(data, "Bottom")（無 → 空字串，呼叫
// 端停止）；條目經 ExtractTweets 收集（promoted- 前綴已排除）後，result 含
// promotedMetadata 者再排除，並以 work_id 去重、保留首見順序（REQ-WEB-104
// 語義；跨頁去重由呼叫端以 seenTweetIDs 累積，設計 §25）；User 亦以 UserID
// 去重（首見順序）。缺 rest_id 的條目視為結構不符，靜默跳過——時間線容忍
// 個別壞條目。HomeTimeline／HomeLatestTimeline／SearchTimeline 共用：組裝
// 形態各異（變數雙形態 vs 搜尋形），解析語義同一。
func parseTimelineResult(opName string, data []byte) (*TimelineResult, error) {
	var root any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return nil, &xerrors.TransportError{Msg: fmt.Sprintf("%s: 回應 data 解析失敗: %v", opName, err)}
		}
	}
	res := &TimelineResult{Cursor: ExtractCursor(root, "Bottom")}
	seenWork := make(map[string]bool)
	seenUser := make(map[string]bool)
	for _, raw := range ExtractTweets(root) {
		result, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, promoted := result["promotedMetadata"]; promoted {
			continue // REQ-WEB-104：result 含 promotedMetadata → 排除
		}
		workID, _ := result["rest_id"].(string)
		if workID == "" {
			continue // 缺 rest_id：結構不符，靜默跳過（不拋錯）
		}
		if seenWork[workID] {
			continue // work_id 去重：保留首見順序（REQ-WEB-104）
		}
		seenWork[workID] = true
		user := postUser(result)
		res.Posts = append(res.Posts, Post{
			WorkID:     workID,
			WorkURL:    "https://x.com/" + user.ScreenName + "/status/" + workID,
			ScreenName: user.ScreenName,
			Title:      postTitle(result),
			Media:      extractMedia(result), // extended_entities.media（REQ-DAT-015/016，見 media.go）
		})
		if user.UserID != "" && !seenUser[user.UserID] {
			seenUser[user.UserID] = true
			res.Users = append(res.Users, user)
		}
	}
	return res, nil
}

// homeTimelineOp 是 HomeTimeline 與 HomeLatestTimeline 的共用實作：兩操作
// 對 X 端雙形態同構（D16；HomeLatestTimeline 依 P0' 實證——DECISIONS
// 〈v1.1 P0' 探測〉節：GET 20/20、POST(cursor+seen) 20/20、零失敗、空
// features），組裝與解析路徑完全一致，僅註冊表操作名不同。變數雙形態
//（OQ3 裁決、DECISIONS D1、REQ-GQL-005）：
//
//   - 共同：count（數字）、includePromotedContent=true、withCommunity=true。
//   - cursor 非空 → 加 cursor 變數。
//   - seenTweetIDs 空 → 依註冊表方法（query 型 → GET）且無 seenTweetIds；
//     此時 cursor 亦為空 → 加 requestContext="launch"（首頁啟動）。
//   - seenTweetIDs 非空 → 覆寫為 POST，seenTweetIds（字串陣列，以
//     CompactJSON 序列化）置於變數末。
//
// 解析共用 parseTimelineResult（游標、去促銷、去重語義見該函式註解）。
// Operation 以複製覆寫 Method，不變動註冊表共用值（並行安全）。
func (s *Sender) homeTimelineOp(opName string, cursor string, count int, seenTweetIDs []string) (*TimelineResult, error) {
	op, err := s.reg.Operation(opName)
	if err != nil {
		return nil, err
	}
	vars := graphql.OrderedParams{
		{K: "count", V: strconv.Itoa(count)},
		{K: "includePromotedContent", V: "true"},
		{K: "withCommunity", V: "true"},
	}
	method := op.Method
	if cursor != "" {
		cv, err := graphql.CompactJSON(cursor)
		if err != nil {
			return nil, &xerrors.InvalidInputError{Msg: fmt.Sprintf("%s: cursor 序列化失敗: %v", opName, err)}
		}
		vars = append(vars, graphql.Param{K: "cursor", V: string(cv)})
	}
	if len(seenTweetIDs) == 0 {
		if cursor == "" {
			vars = append(vars, graphql.Param{K: "requestContext", V: `"launch"`})
		}
	} else {
		sv, err := graphql.CompactJSON(seenTweetIDs)
		if err != nil {
			return nil, &xerrors.InvalidInputError{Msg: fmt.Sprintf("%s: seenTweetIds 序列化失敗: %v", opName, err)}
		}
		vars = append(vars, graphql.Param{K: "seenTweetIds", V: string(sv)})
		method = "POST" // REQ-GQL-005：query 型操作顯式覆寫為 POST
	}
	opCall := *op
	opCall.Method = method
	data, err := s.Send(&opCall, vars, "")
	if err != nil {
		return nil, err
	}
	return parseTimelineResult(opName, data)
}

// HomeTimeline 讀取首頁時間線（REQ-RD-001..004）並解析為 TimelineResult
//（變數雙形態與解析語義見 homeTimelineOp——共用實作）。
func (s *Sender) HomeTimeline(cursor string, count int, seenTweetIDs []string) (*TimelineResult, error) {
	return s.homeTimelineOp("HomeTimeline", cursor, count, seenTweetIDs)
}

// HomeLatestTimeline 與 HomeTimeline 同構（P0' 實證：GET 20/20、POST(seen)
// 20/20、零失敗、空 features——DECISIONS〈v1.1 P0' 探測〉節；D16：雙形態
// 同構 HomeTimeline）。形態分支與 HomeTimeline 完全一致：cursor=="" 且
// seen 空 → GET 啟動形態；cursor 非空或 seen 非空 → POST 帶 seenTweetIds
//（seen 置變數末，沿 HomeTimeline 的既有排序）。
func (s *Sender) HomeLatestTimeline(cursor string, count int, seenTweetIDs []string) (*TimelineResult, error) {
	return s.homeTimelineOp("HomeLatestTimeline", cursor, count, seenTweetIDs)
}

// SearchTimeline 恆 POST（REQ-RD-005/REQ-GQL-005 語義——P0' 實證 20/20：
// product=Top 10/10、product=Latest 10/10、零失敗、空 features）。
// variables 鍵序（P0' 實證形）：product→rawQuery→count；cursor 非空時附於
// 其後（同 HomeTimeline 的 cursor 追加位置）。無 seenTweetIds。
// product 值："Top"（預設）｜"Latest"——呼叫端（Task 10）決定。
func (s *Sender) SearchTimeline(query, product, cursor string, count int) (*TimelineResult, error) {
	if product == "" {
		// P0' 實證：variables 缺 product → 422 GRAPHQL_VALIDATION_FAILED
		// "must be defined: variable product"（DECISIONS〈v1.1 P0' 探測〉節）
		// ——必填變數於送出前拒絕，不發請求。
		return nil, &xerrors.InvalidInputError{Msg: "SearchTimeline: product 為必填變數，不可為空字串（\"Top\"／\"Latest\"）"}
	}
	op, err := s.reg.Operation("SearchTimeline")
	if err != nil {
		return nil, err
	}
	pv, err := graphql.CompactJSON(product)
	if err != nil {
		return nil, &xerrors.InvalidInputError{Msg: fmt.Sprintf("SearchTimeline: product 序列化失敗: %v", err)}
	}
	rv, err := graphql.CompactJSON(query)
	if err != nil {
		return nil, &xerrors.InvalidInputError{Msg: fmt.Sprintf("SearchTimeline: rawQuery 序列化失敗: %v", err)}
	}
	vars := graphql.OrderedParams{
		{K: "product", V: string(pv)},
		{K: "rawQuery", V: string(rv)},
		{K: "count", V: strconv.Itoa(count)},
	}
	if cursor != "" {
		cv, err := graphql.CompactJSON(cursor)
		if err != nil {
			return nil, &xerrors.InvalidInputError{Msg: fmt.Sprintf("SearchTimeline: cursor 序列化失敗: %v", err)}
		}
		vars = append(vars, graphql.Param{K: "cursor", V: string(cv)})
	}
	opCall := *op
	opCall.Method = "POST" // REQ-GQL-005／REQ-RD-014：query 型恆 POST，複製覆寫不改註冊表
	data, err := s.Send(&opCall, vars, "")
	if err != nil {
		return nil, err
	}
	return parseTimelineResult("SearchTimeline", data)
}

// Viewer 讀取當前會話身分（REQ-RD 表 get_viewer）：op Viewer、variables
// {"withCommunitiesMemberships": true}。回應不硬編路徑：深度遍歷找首個含
// 非空 rest_id 的物件作為 viewer 使用者結果，再取其 core.screen_name／
// core.name；找不到 rest_id 或缺 screen_name／name → *xerrors.MissingFieldError。
func (s *Sender) Viewer() (ViewerInfo, error) {
	op, err := s.reg.Operation("Viewer")
	if err != nil {
		return ViewerInfo{}, err
	}
	vars := graphql.OrderedParams{{K: "withCommunitiesMemberships", V: "true"}}
	data, err := s.Send(op, vars, "")
	if err != nil {
		return ViewerInfo{}, err
	}
	var root any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return ViewerInfo{}, &xerrors.TransportError{Msg: fmt.Sprintf("Viewer: 回應 data 解析失敗: %v", err)}
		}
	}
	var info ViewerInfo
	found := false
	walk(root, func(m map[string]any) {
		if found {
			return
		}
		id, ok := m["rest_id"].(string)
		if !ok || id == "" {
			return
		}
		core, _ := m["core"].(map[string]any)
		screenName, _ := core["screen_name"].(string)
		name, _ := core["name"].(string)
		info = ViewerInfo{UserID: id, ScreenName: screenName, Name: name}
		found = true
	})
	if !found {
		return ViewerInfo{}, &xerrors.MissingFieldError{Msg: "Viewer: 回應中找不到 rest_id（深度遍歷未命中）"}
	}
	if info.ScreenName == "" {
		return ViewerInfo{}, &xerrors.MissingFieldError{Msg: "Viewer: viewer 結果缺少 core.screen_name"}
	}
	if info.Name == "" {
		return ViewerInfo{}, &xerrors.MissingFieldError{Msg: "Viewer: viewer 結果缺少 core.name"}
	}
	return info, nil
}
