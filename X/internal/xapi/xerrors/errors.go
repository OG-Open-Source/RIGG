package xerrors

import (
	"fmt"
	"strings"
)

// Kind 是錯誤的機械分類 wire 標籤（設計 §3.5）。app 只 switch on kind
// 決定處置，不解析訊息文字（REQ-LOG-042/043）。
type Kind string

const (
	// KindCredential 表示憑證失效：標記會話失效並導回 login page。
	KindCredential Kind = "credential"
	// KindMaterial 表示素材不可用：可重試 bootstrap，但絕不清憑證。
	KindMaterial Kind = "material"
	// KindTransport 表示傳輸層失敗或逾時：可重試，但絕不清憑證。
	KindTransport Kind = "transport"
	// KindGraphql 表示 GraphQL 回應錯誤（含退避與 CSRF 自癒子集）。
	KindGraphql Kind = "graphql"
	// KindInput 表示輸入層錯誤：僅供表單層提示，不重試、不清憑證。
	KindInput Kind = "input"
)

// GraphErr 是 GraphQL 回應中的單一錯誤條目。
type GraphErr struct {
	// Message 是上游錯誤訊息文字。
	Message string
	// Code 是上游錯誤碼（供 ClassifyGraphQLCode 分類）。
	Code int
}

// GraphQLError 表示 GraphQL 回應「有 errors 且無 data」時的讀取語義錯誤
// （REQ-GQL-006）。其分類三元組由 Code 經 ClassifyGraphQLCode 導出，故
// 呼叫端只需填 OperationName/Errors/ResJSON/Code；憑證失效碼（32/89/215/
// 326）自然呈現 KindCredential 與 credentialImpacting=true。未經分類時
// 為安全的 graphql/不重試/不清憑證（設計 §3.5）。ResJSON 僅供引擎診斷層
// 承載，不在 Error() 中展開，避免訊息爆炸與洩漏面。
type GraphQLError struct {
	OperationName string
	Errors        []GraphErr
	ResJSON       []byte
	Code          int
}

// Error 回傳 Go 慣例組合訊息：操作名、頂層碼與所有內層錯誤訊息。
func (e *GraphQLError) Error() string {
	msgs := make([]string, 0, len(e.Errors))
	for _, ge := range e.Errors {
		msgs = append(msgs, fmt.Sprintf("%s (code %d)", ge.Message, ge.Code))
	}
	if len(msgs) == 0 {
		return fmt.Sprintf("graphql %s: code %d", e.OperationName, e.Code)
	}
	return fmt.Sprintf("graphql %s: code %d: %s", e.OperationName, e.Code, strings.Join(msgs, "; "))
}

// Kind 由 Code 經 ClassifyGraphQLCode 導出錯誤分類。
func (e *GraphQLError) Kind() Kind {
	kind, _, _ := ClassifyGraphQLCode(e.Code)
	return kind
}

// Retryable 由 Code 經 ClassifyGraphQLCode 導出。code 353 重建標頭後恰可
// 重試一次（retryable=true 僅表示呼叫端可重試一次，重試與否由呼叫端管理）。
func (e *GraphQLError) Retryable() bool {
	_, retryable, _ := ClassifyGraphQLCode(e.Code)
	return retryable
}

// CredentialImpacting 由 Code 經 ClassifyGraphQLCode 導出。
func (e *GraphQLError) CredentialImpacting() bool {
	_, _, credImpacting := ClassifyGraphQLCode(e.Code)
	return credImpacting
}

// Unwrap 目前無底層錯誤可展開，回傳 nil。
func (e *GraphQLError) Unwrap() error { return nil }

// InvalidInputError 表示輸入層錯誤（kind=input，不重試、不清憑證）。
type InvalidInputError struct {
	Msg string
}

// Error 回傳簡潔的輸入錯誤訊息。
func (e *InvalidInputError) Error() string { return fmt.Sprintf("invalid input: %s", e.Msg) }

// Kind 回傳 KindInput。
func (e *InvalidInputError) Kind() Kind { return KindInput }

// Retryable 為 false：輸入錯誤重試無益。
func (e *InvalidInputError) Retryable() bool { return false }

// CredentialImpacting 為 false：輸入問題絕不牽連憑證。
func (e *InvalidInputError) CredentialImpacting() bool { return false }

// Unwrap 回傳 nil。
func (e *InvalidInputError) Unwrap() error { return nil }

// MissingFieldError 表示回應缺少必要欄位，屬輸入層錯誤（kind=input）。
type MissingFieldError struct {
	Msg string
}

// Error 回傳簡潔的缺欄訊息。
func (e *MissingFieldError) Error() string { return fmt.Sprintf("missing field: %s", e.Msg) }

// Kind 回傳 KindInput。
func (e *MissingFieldError) Kind() Kind { return KindInput }

// Retryable 為 false。
func (e *MissingFieldError) Retryable() bool { return false }

// CredentialImpacting 為 false。
func (e *MissingFieldError) CredentialImpacting() bool { return false }

// Unwrap 回傳 nil。
func (e *MissingFieldError) Unwrap() error { return nil }

// MaterialError 表示素材不可用（kind=material）：可重試 bootstrap，
// 但絕不清憑證（設計 §3.5）。
type MaterialError struct {
	Msg string
}

// Error 回傳簡潔的素材錯誤訊息。
func (e *MaterialError) Error() string { return fmt.Sprintf("material unavailable: %s", e.Msg) }

// Kind 回傳 KindMaterial。
func (e *MaterialError) Kind() Kind { return KindMaterial }

// Retryable 為 true：素材狀態可重試 bootstrap。
func (e *MaterialError) Retryable() bool { return true }

// CredentialImpacting 為 false：素材問題絕不牽連憑證。
func (e *MaterialError) CredentialImpacting() bool { return false }

// Unwrap 回傳 nil。
func (e *MaterialError) Unwrap() error { return nil }

// CredentialInvalidError 表示憑證失效（kind=credential）：標記會話失效
// 並導回 login page（設計 §3.5）。
type CredentialInvalidError struct {
	Msg string
}

// Error 回傳簡潔的憑證失效訊息。
func (e *CredentialInvalidError) Error() string {
	return fmt.Sprintf("credential invalid: %s", e.Msg)
}

// Kind 回傳 KindCredential。
func (e *CredentialInvalidError) Kind() Kind { return KindCredential }

// Retryable 為 false：憑證失效不可原樣重試。
func (e *CredentialInvalidError) Retryable() bool { return false }

// CredentialImpacting 為 true。
func (e *CredentialInvalidError) CredentialImpacting() bool { return true }

// Unwrap 回傳 nil。
func (e *CredentialInvalidError) Unwrap() error { return nil }

// TransportError 表示傳輸層失敗或逾時（kind=transport）：可重試，
// 但絕不清憑證（設計 §3.5）。
type TransportError struct {
	Msg string
}

// Error 回傳簡潔的傳輸錯誤訊息。
func (e *TransportError) Error() string { return fmt.Sprintf("transport: %s", e.Msg) }

// Kind 回傳 KindTransport。
func (e *TransportError) Kind() Kind { return KindTransport }

// Retryable 為 true。
func (e *TransportError) Retryable() bool { return true }

// CredentialImpacting 為 false：連線問題絕不清憑證。
func (e *TransportError) CredentialImpacting() bool { return false }

// Unwrap 回傳 nil。
func (e *TransportError) Unwrap() error { return nil }

// ClassifyGraphQLCode 依設計 §3.5 表將 GraphQL 錯誤碼映射為
// (kind, retryable, credentialImpacting)：
//
//   - 32/89/215/326：憑證失效 → (KindCredential, false, true)。
//   - 88/344/226：退避不重試 → (KindGraphql, false, false)。
//   - 353：重建標頭後恰重試一次 → (KindGraphql, true, false)；
//     retryable=true 僅表示呼叫端可重試一次，重試與否由呼叫端管理。
//   - 187/385/179/239：既有錯誤 → (KindGraphql, false, false)。
//   - 其他未知碼：安全預設 (KindGraphql, false, false)，絕不誤清憑證。
func ClassifyGraphQLCode(code int) (Kind, bool, bool) {
	switch code {
	case 32, 89, 215, 326:
		return KindCredential, false, true
	case 353:
		return KindGraphql, true, false
	case 88, 344, 226:
		return KindGraphql, false, false
	case 187, 385, 179, 239:
		return KindGraphql, false, false
	default:
		return KindGraphql, false, false
	}
}
