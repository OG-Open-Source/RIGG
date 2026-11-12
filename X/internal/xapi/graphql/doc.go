// Package graphql 提供緊湊 JSON 序列化器（分隔符「,」「:」、
// SetEscapeHTML(false)、有序結構），同時支撐 GET query string 與 POST
// body 的鍵序；並負責操作註冊表的載入與快取（static/graphql.json 快照或
// 重建產物）與每操作 metadata：queryId、operationType、features／
// fieldToggles 解析、送出方法（REQ-GQL-001..025）。
package graphql
