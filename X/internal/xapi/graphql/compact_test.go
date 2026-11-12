package graphql

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMarshalOrdered_Compact_REQ_GQL_002 緊湊序列化：鍵序即參數序、分隔符
// 「,」「:」、非 ASCII（中文）原樣 UTF-8、<>& 不轉義，值為預序列化 JSON 值
// 字串直接內嵌（REQ-GQL-002）。
func TestMarshalOrdered_Compact_REQ_GQL_002(t *testing.T) {
	got, err := MarshalOrdered(OrderedParams{
		{K: "count", V: "20"},
		{K: "name", V: "\"美國\""},
		{K: "h", V: "\"<a>&\""},
	})
	if err != nil {
		t.Fatalf("MarshalOrdered error = %v, 期望 nil", err)
	}
	want := `{"count":20,"name":"美國","h":"<a>&"}`
	if string(got) != want {
		t.Fatalf("MarshalOrdered = %s, 期望 %s", got, want)
	}

	// 內層引號正確跳脫：預序列化值（"a\"b" 對應 Go 字串 a"b）直接內嵌後
	// 仍是合法 JSON，可原樣讀回——不得二次跳脫。
	round, err := MarshalOrdered(OrderedParams{{K: "k", V: "\"a\\\"b\""}})
	if err != nil {
		t.Fatalf("MarshalOrdered(內層引號) error = %v, 期望 nil", err)
	}
	var back map[string]any
	if err := json.Unmarshal(round, &back); err != nil {
		t.Fatalf("內層引號輸出非合法 JSON：%v（%s）", err, round)
	}
	if back["k"] != `a"b` {
		t.Fatalf("內層引號 round-trip = %v, 期望 a\"b", back["k"])
	}
}

// TestEncodeQuery_Order_REQ_GQL_001 query string 裝配：順序即參數序
// （variables 在前、features 次之、fieldToggles 末，REQ-GQL-001），值以
// url.QueryEscape 語義 percent-encode；不經 url.Values（其 Encode 會排序鍵，
// 為 REQ-GQL-001/025 所禁）。
func TestEncodeQuery_Order_REQ_GQL_001(t *testing.T) {
	p := OrderedParams{
		{K: "variables", V: `{"count":20}`},
		{K: "features", V: `{"a":true}`},
		{K: "fieldToggles", V: `{"s":"a & b"}`},
	}
	got := p.EncodeQuery()
	want := "variables=%7B%22count%22%3A20%7D&features=%7B%22a%22%3Atrue%7D&fieldToggles=%7B%22s%22%3A%22a+%26+b%22%7D"
	if got != want {
		t.Fatalf("EncodeQuery = %s, 期望 %s", got, want)
	}
	// 索引斷言：variables 在前、features 次之、fieldToggles 末。
	iVar := strings.Index(got, "variables=")
	iFeat := strings.Index(got, "features=")
	iTog := strings.Index(got, "fieldToggles=")
	if iVar != 0 || iVar > iFeat || iFeat > iTog {
		t.Fatalf("EncodeQuery 順序錯誤：variables@%d、features@%d、fieldToggles@%d", iVar, iFeat, iTog)
	}
}
