// Package api 是引擎的統一發送層：單一發送函式、Viewer、HomeTimeline
// （GET／POST 雙形態）、游標深度遍歷（有序解析）與條目萃取；讀取語義為
// 「有 errors 且無 data 才拋」（REQ-RD-001..014、REQ-GQL-006/007）。
package api
