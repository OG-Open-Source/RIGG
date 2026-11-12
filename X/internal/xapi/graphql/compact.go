package graphql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// Param 是一筆有序參數：K 為參數名；V 為「已序列化」的 JSON 值字串
// （如 "20"、`"Top"`），由呼叫端以 CompactJSON／標準編碼器預先產生——
// 本包不再對 V 做任何轉換，僅直接內嵌。
type Param struct {
	K string
	V string
}

// OrderedParams 是保持插入順序的參數序列：GET query string 與 POST body
// 的鍵序皆由此承載（REQ-GQL-001/003），一般 REST 參數的插入序裝配同規
// （REQ-GQL-025）。一切輸出絕不排序。
type OrderedParams []Param

// appendJSONString 以 SetEscapeHTML(false) 的標準編碼器將單一 Go 字串編為
// JSON 字串字面值（含前後引號、無換行）：非 ASCII 原樣 UTF-8、<>& 原樣
// （REQ-GQL-002）。Encode 附加的結尾換行在此去除。
func appendJSONString(dst []byte, s string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return append(dst, bytes.TrimSuffix(buf.Bytes(), []byte{'\n'})...), nil
}

// MarshalOrdered 將有序參數序列編為緊湊 JSON 物件：鍵序即 p 的順序、
// 分隔符「,」「:」、SetEscapeHTML(false)、非 ASCII 原樣（REQ-GQL-002）。
// V 為預序列化的 JSON 值字串，直接內嵌；非合法 JSON 值（含空字串）回傳
// 錯誤。任何鍵皆不排序。
func MarshalOrdered(p OrderedParams) ([]byte, error) {
	buf := make([]byte, 0, 64)
	buf = append(buf, '{')
	for i, kv := range p {
		if i > 0 {
			buf = append(buf, ',')
		}
		key, err := appendJSONString(buf, kv.K)
		if err != nil {
			return nil, fmt.Errorf("graphql: marshal param %d key: %w", i, err)
		}
		buf = key
		if !json.Valid([]byte(kv.V)) {
			return nil, fmt.Errorf(
				"graphql: param %d (%s): V must be pre-serialized JSON, got %q", i, kv.K, kv.V)
		}
		buf = append(buf, ':')
		buf = append(buf, kv.V...)
	}
	buf = append(buf, '}')
	return buf, nil
}

// CompactJSON 以標準編碼器輸出緊湊 JSON（無縮排、SetEscapeHTML(false)、
// 非 ASCII 原樣，REQ-GQL-002），供陣列／標量等任意值使用；物件請走
// MarshalOrdered 以保鍵序。
func CompactJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte{'\n'}), nil
}

// EncodeQuery 以 p 的順序輸出 k=<percent-encoded V> 以「&」串接的 query
// string（REQ-GQL-001/025）：順序即插入序，絕不排序——刻意不經
// url.Values（其 Encode 會依鍵排序，破壞 REQ-GQL-001 的鍵序）。鍵為引擎
// 內部固定名（variables／features／fieldToggles 等）故原樣寫出，值以
// url.QueryEscape 編碼。
func (p OrderedParams) EncodeQuery() string {
	var sb strings.Builder
	for i, kv := range p {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(kv.K)
		sb.WriteByte('=')
		sb.WriteString(url.QueryEscape(kv.V))
	}
	return sb.String()
}
