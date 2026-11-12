// messages：錯誤 kind → 中文文案的單一來源（OQ10：需求方定稿前用本檔的
// 暫定文案；view 一律經 errorText 取得文案，不內嵌字面值）。app 只依
// errors.As 判別 kind 決定文案，不解析錯誤訊息文字（REQ-LOG-042/043）；
// 文案為固定表格，絕不拼接錯誤原始訊息——憑證材料因此不可能進顯示面
// （REQ-INV-01），其餘含憑證可能性的顯示路徑另經 Sanitize（view 層）。
package main

import (
	"errors"
	"fmt"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// 各 kind 的暫定文案。credential／material／transport 為 brief 逐字暫定；
// input（表單層提示）與 unknown 為覆蓋全部 kind 的延伸（自我審查要求
// errorText 對每個 Kind 有歸屬）。
const (
	msgCredential = "會話已失效，請重新匯入 Cookie"
	msgMaterial   = "素材不可用，請重試"
	msgTransport  = "連線失敗，請檢查代理"
	msgInput      = "Cookie 格式不正確，請確認後重試"
	msgUnknown    = "發生錯誤，請重試"
)

// graphqlErrPrefix 是 credential 以外 GraphQL code 的文案前綴
// （brief：「X 回應錯誤：<code>」）。
const graphqlErrPrefix = "X 回應錯誤："

// errorText 依錯誤類別回傳畫面文案（kind → copy 映射）：
//
//   - *CredentialInvalidError → 重新匯入提示；
//   - *MaterialError → 素材重試提示；
//   - *TransportError → 連線檢查提示；
//   - *GraphQLError → 依 code 分支：32/89/215/326（credential-impacting，
//     ClassifyGraphQLCode 同表）→ credential 文案；其他 →
//     「X 回應錯誤：<code>」；
//   - 其餘依 Kind() 標籤兜底（input → 表單提示；engine 新增 kind 時不致
//     顯示 Go 錯誤字串）；未知錯誤 → 通用提示。
//
// 所有分支皆為固定文案或純數字碼——呼叫端重試入口由畫面承載
// （material/transport → 重試；credential → 重新匯入），且任何路徑都
// 不清除憑證（清除決策不在顯示層，REQ-LOG-042/043）。
func errorText(err error) string {
	if err == nil {
		return ""
	}
	var cred *xerrors.CredentialInvalidError
	if errors.As(err, &cred) {
		return msgCredential
	}
	var mat *xerrors.MaterialError
	if errors.As(err, &mat) {
		return msgMaterial
	}
	var tr *xerrors.TransportError
	if errors.As(err, &tr) {
		return msgTransport
	}
	var gql *xerrors.GraphQLError
	if errors.As(err, &gql) {
		switch gql.Code {
		case 32, 89, 215, 326:
			return msgCredential
		default:
			return fmt.Sprintf("%s%d", graphqlErrPrefix, gql.Code)
		}
	}
	var kinder interface{ Kind() xerrors.Kind }
	if errors.As(err, &kinder) {
		switch kinder.Kind() {
		case xerrors.KindCredential:
			return msgCredential
		case xerrors.KindMaterial:
			return msgMaterial
		case xerrors.KindTransport:
			return msgTransport
		case xerrors.KindInput:
			return msgInput
		}
	}
	return msgUnknown
}
