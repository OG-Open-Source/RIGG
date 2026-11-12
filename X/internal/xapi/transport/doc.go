// Package transport 承接 tls-client 建構（chrome_150 profile）：預設標頭
// 補齊與 sec-fetch-site 推導、session 形態（探測裁定 v1 無匿名素材抓取，
// 不實作 anonymous 形態與 jar 隔離，見 DECISIONS.md）、cookie 狀態為零
//（jar 以 no-op 中和，Cookie 標頭由呼叫端自 XAuth 注入，REQ-AUTH-001）、
// 重定向一律不跟隨、OnRequest／OnResponse 脫敏觀測掛鉤、逾時預設 30 秒、
// fail-closed 代理（無代理即拒絕，不存在直連路徑）。所有 X 流量的單一
// 出口在此（REQ-TRN-010..015、018..020）。
package transport
