// transport 觀測鉤（REQ-INV-13）：OnRequest／OnResponse 脫敏回呼。
// 鉤子屬觀測面：panic 以 recover 吸收，絕不中斷主流程；參數僅含
// method/url/status/耗時/Set-Cookie 名稱，不含標頭值或回應內容。
package transport

import "time"

// fireOnRequest 於送出前呼叫 OnRequest；nil 鉤子零成本跳過，panic 吸收。
func (c *Client) fireOnRequest(method, url string) {
	if c.opts.OnRequest == nil {
		return
	}
	func() {
		defer func() { _ = recover() }()
		c.opts.OnRequest(method, url)
	}()
}

// fireOnResponse 於回應後呼叫 OnResponse；panic 吸收，耗時與 Set-Cookie
// 名稱清單由 Do 組好傳入。
func (c *Client) fireOnResponse(method, url string, status int, elapsed time.Duration, setCookieNames []string) {
	if c.opts.OnResponse == nil {
		return
	}
	func() {
		defer func() { _ = recover() }()
		c.opts.OnResponse(method, url, status, elapsed, setCookieNames)
	}()
}
