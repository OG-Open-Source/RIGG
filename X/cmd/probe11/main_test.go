package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestExtractQueryIDs 驗證 extractQueryIDs 對 webpack chunk 形態 JS 的
// queryId 鄰接對萃取（Task 1 Step 1）：fixture 為合成（clean-room，不含
// X 真實回應原文），含兩種鄰接形態——brief 例示的引號鍵（operationName
// 在前）與 v1 探測節記錄的 main chunk 形態（無引號鍵、queryId 在前，
// DECISIONS〈queryId 附註〉）——皆應收進同一對照 map。
func TestExtractQueryIDs(t *testing.T) {
	js := `self.webpackChunk_x_com_pwa.push([[572],{` +
		// brief 例示的鄰接形態：引號鍵、operationName 在前。
		`82615:(e,t,n)=>{const o={"operationName":"HomeLatestTimeline","queryId":"abc123-_def","operationType":"query"};},` +
		// v1 探測節記錄的形態：無引號鍵、queryId 在前。
		`55501:(e,t)=>{e.exports={queryId:"ghi456-_jkl",operationName:"SearchTimeline",operationType:"query"}}` +
		`}]);`
	got := extractQueryIDs(js)
	if len(got) != 2 {
		t.Fatalf("extractQueryIDs 回 %d 鍵（%v），期望恰含兩操作名", len(got), got)
	}
	if got["HomeLatestTimeline"] != "abc123-_def" {
		t.Fatalf("HomeLatestTimeline queryId = %q, 期望 abc123-_def", got["HomeLatestTimeline"])
	}
	if got["SearchTimeline"] != "ghi456-_jkl" {
		t.Fatalf("SearchTimeline queryId = %q, 期望 ghi456-_jkl", got["SearchTimeline"])
	}
}

// TestSummarizeNoCookieLeak 驗證 summarize 的脫敏契約（REQ-INV-01）：含
// cookie 值的 headers map 經 summarize 後，輸出絕不含值本身，且值以固定
// 遮罩呈現、輸出仍為合法 JSON；err 路徑同契約——錯誤字串內嵌 cookie 行
// （或其值）時，輸出前必被遮除，不依賴引擎錯誤字串的內容。
func TestSummarizeNoCookieLeak(t *testing.T) {
	cookieLine := "auth_token=SECRET; ct0=ct0value"
	headers := map[string]string{
		"cookie": cookieLine,
	}
	reqs := []probeReq{
		{Name: "latest.get", OK: true, Data: true},
		// err 路徑：人造錯誤字串內嵌完整 cookie 行，輸出不得含其任一值。
		{Name: "latest.post-seen", Err: "Post \"https://x.com/i/api/graphql/HomeLatestTimeline\": 憑證被拒（" + cookieLine + "）"},
	}
	out := summarize(headers, cookieLine, reqs)
	if strings.Contains(out, "SECRET") {
		t.Fatalf("summarize 輸出含 cookie 值片段 SECRET：\n%s", out)
	}
	if strings.Contains(out, "ct0value") {
		t.Fatalf("summarize 輸出含 cookie 值片段 ct0value：\n%s", out)
	}
	if !strings.Contains(out, maskValue) {
		t.Fatalf("summarize 輸出未含固定遮罩 %q（headers 未被脫敏）：\n%s", maskValue, out)
	}
	if !json.Valid([]byte(out)) {
		t.Fatalf("summarize 輸出非合法 JSON：\n%s", out)
	}
}

// TestMediaURLs 驗證 mediaURLs 對原始回應 JSON 的媒體 URL 萃取：JSON 內
// 斜線以 \/ 轉義時正規化、重複 URL 去重（保留首見序）、非媒體 CDN 的
// URL 不收錄。fixture 為合成。
func TestMediaURLs(t *testing.T) {
	raw := []byte(`{"a":"https:\/\/pbs.twimg.com\/media\/Fabc1?format=jpg&name=small",` +
		`"b":"https:\/\/video.twimg.com\/x.mp4",` +
		`"c":"https:\/\/pbs.twimg.com\/media\/Fabc1?format=jpg&name=small",` +
		`"d":"https://example.com/not-media"}`)
	got := mediaURLs(raw)
	want := []string{
		"https://pbs.twimg.com/media/Fabc1?format=jpg&name=small",
		"https://video.twimg.com/x.mp4",
	}
	if len(got) != len(want) {
		t.Fatalf("mediaURLs 回 %d 筆（%v），期望 %d 筆", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mediaURLs[%d] = %q, 期望 %q", i, got[i], want[i])
		}
	}
}
