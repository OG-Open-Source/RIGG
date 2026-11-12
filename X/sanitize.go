// sanitize：憑證脫敏的單一閘門（REQ-INV-01）——任何可能含 cookie 材料
// 的字串進日誌／錯誤訊息前必經 Sanitize：auth_token＝/ct0= 後的值遮為
// ***（保留鍵名），其餘原樣。
package main

import "regexp"

// credValueRe 匹配 auth_token／ct0 的鍵值對：鍵名前須有詞邊界（不誤傷
// xct0 之類的長鍵），值止於 ';' 或字串尾（cookie 值不含未編碼 ';'）；
// 鍵名入捕獲群供保留。(?i) 使大小寫變體一律遮蔽——過遮是安全方向。
var credValueRe = regexp.MustCompile(`(?i)\b(auth_token|ct0)=[^;]*`)

// Sanitize 回脫敏後的字串：遮蔽全部 auth_token／ct0 值（重複出現全遮），
// 其餘內容逐字保留；無憑證輸入直通。鍵名保留供問題定位，值以 *** 取代。
func Sanitize(s string) string {
	return credValueRe.ReplaceAllString(s, "${1}=***")
}
