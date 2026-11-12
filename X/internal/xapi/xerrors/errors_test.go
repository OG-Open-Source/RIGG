package xerrors

import (
	"errors"
	"strings"
	"testing"
)

// TestClassify_Credential_REQ_12_3 逐一斷言憑證失效碼（設計 §3.5）：
// 32/89/215/326 → (credential, false, true)。
func TestClassify_Credential_REQ_12_3(t *testing.T) {
	for _, code := range []int{32, 89, 215, 326} {
		kind, retryable, credImpacting := ClassifyGraphQLCode(code)
		if kind != KindCredential {
			t.Errorf("ClassifyGraphQLCode(%d) kind = %q, 期望 %q", code, kind, KindCredential)
		}
		if retryable {
			t.Errorf("ClassifyGraphQLCode(%d) retryable = true, 期望 false", code)
		}
		if !credImpacting {
			t.Errorf("ClassifyGraphQLCode(%d) credentialImpacting = false, 期望 true", code)
		}
	}
}

// TestClassify_Backoff 斷言退避碼（88/344/226）與其餘既有錯誤碼
// （187/385/179/239）皆為 (graphql, false, false)，即退避但不重試
// （設計 §3.5）。
func TestClassify_Backoff(t *testing.T) {
	for _, code := range []int{88, 344, 226, 187, 385, 179, 239} {
		kind, retryable, credImpacting := ClassifyGraphQLCode(code)
		if kind != KindGraphql {
			t.Errorf("ClassifyGraphQLCode(%d) kind = %q, 期望 %q", code, kind, KindGraphql)
		}
		if retryable {
			t.Errorf("ClassifyGraphQLCode(%d) retryable = true, 期望 false", code)
		}
		if credImpacting {
			t.Errorf("ClassifyGraphQLCode(%d) credentialImpacting = true, 期望 false", code)
		}
	}
}

// TestClassify_Csrf353 斷言 CSRF 自癒碼 353 → (graphql, true, false)：
// 重建標頭後恰重試一次，重試與否由呼叫端管理（設計 §3.5）。
func TestClassify_Csrf353(t *testing.T) {
	kind, retryable, credImpacting := ClassifyGraphQLCode(353)
	if kind != KindGraphql {
		t.Errorf("ClassifyGraphQLCode(353) kind = %q, 期望 %q", kind, KindGraphql)
	}
	if !retryable {
		t.Errorf("ClassifyGraphQLCode(353) retryable = false, 期望 true")
	}
	if credImpacting {
		t.Errorf("ClassifyGraphQLCode(353) credentialImpacting = true, 期望 false")
	}
}

// TestClassify_Default 斷言未知碼落在安全預設 (graphql, false, false)，
// 絕不誤標為憑證失效（不誤清憑證，REQ-LOG-043）。
func TestClassify_Default(t *testing.T) {
	for _, code := range []int{0, -1, 999} {
		kind, retryable, credImpacting := ClassifyGraphQLCode(code)
		if kind != KindGraphql {
			t.Errorf("ClassifyGraphQLCode(%d) kind = %q, 期望 %q", code, kind, KindGraphql)
		}
		if retryable {
			t.Errorf("ClassifyGraphQLCode(%d) retryable = true, 期望 false", code)
		}
		if credImpacting {
			t.Errorf("ClassifyGraphQLCode(%d) credentialImpacting = true, 期望 false", code)
		}
	}
}

// TestErrorsAs_Kinds 對六型別各建一值，經 errors.As 取回具體型別後，
// 以型別斷言驗 kind/retryable/credentialImpacting 三元組（設計 §3.5）。
func TestErrorsAs_Kinds(t *testing.T) {
	t.Run("GraphQLError", func(t *testing.T) {
		var err error = &GraphQLError{
			OperationName: "HomeTimeline",
			Errors:        []GraphErr{{Message: "x", Code: 88}},
			Code:          88,
		}
		var target *GraphQLError
		if !errors.As(err, &target) {
			t.Fatalf("errors.As 無法取得 *GraphQLError")
		}
		assertTriple(t, target.Kind(), target.Retryable(), target.CredentialImpacting(), KindGraphql, false, false)
	})

	t.Run("InvalidInputError", func(t *testing.T) {
		var err error = &InvalidInputError{Msg: "bad input"}
		var target *InvalidInputError
		if !errors.As(err, &target) {
			t.Fatalf("errors.As 無法取得 *InvalidInputError")
		}
		assertTriple(t, target.Kind(), target.Retryable(), target.CredentialImpacting(), KindInput, false, false)
	})

	t.Run("MissingFieldError", func(t *testing.T) {
		var err error = &MissingFieldError{Msg: "missing field"}
		var target *MissingFieldError
		if !errors.As(err, &target) {
			t.Fatalf("errors.As 無法取得 *MissingFieldError")
		}
		assertTriple(t, target.Kind(), target.Retryable(), target.CredentialImpacting(), KindInput, false, false)
	})

	t.Run("MaterialError", func(t *testing.T) {
		var err error = &MaterialError{Msg: "material unavailable"}
		var target *MaterialError
		if !errors.As(err, &target) {
			t.Fatalf("errors.As 無法取得 *MaterialError")
		}
		assertTriple(t, target.Kind(), target.Retryable(), target.CredentialImpacting(), KindMaterial, true, false)
	})

	t.Run("CredentialInvalidError", func(t *testing.T) {
		var err error = &CredentialInvalidError{Msg: "session expired"}
		var target *CredentialInvalidError
		if !errors.As(err, &target) {
			t.Fatalf("errors.As 無法取得 *CredentialInvalidError")
		}
		assertTriple(t, target.Kind(), target.Retryable(), target.CredentialImpacting(), KindCredential, false, true)
	})

	t.Run("TransportError", func(t *testing.T) {
		var err error = &TransportError{Msg: "timeout"}
		var target *TransportError
		if !errors.As(err, &target) {
			t.Fatalf("errors.As 無法取得 *TransportError")
		}
		assertTriple(t, target.Kind(), target.Retryable(), target.CredentialImpacting(), KindTransport, true, false)
	})
}

func assertTriple(t *testing.T, kind Kind, retryable, credImpacting bool, wantKind Kind, wantRetryable, wantCredImpacting bool) {
	t.Helper()
	if kind != wantKind {
		t.Errorf("Kind() = %q, 期望 %q", kind, wantKind)
	}
	if retryable != wantRetryable {
		t.Errorf("Retryable() = %v, 期望 %v", retryable, wantRetryable)
	}
	if credImpacting != wantCredImpacting {
		t.Errorf("CredentialImpacting() = %v, 期望 %v", credImpacting, wantCredImpacting)
	}
}

// TestGraphQLError_Message 斷言 Error() 為 Go 慣例組合訊息，含操作名
// 與所有內層錯誤訊息（順序不拘，但三段都在）。
func TestGraphQLError_Message(t *testing.T) {
	err := &GraphQLError{
		OperationName: "HomeTimeline",
		Errors: []GraphErr{
			{Message: "Timeout", Code: 344},
			{Message: "Limited", Code: 88},
		},
	}
	msg := err.Error()
	for _, want := range []string{"HomeTimeline", "Timeout", "Limited"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, 缺少片段 %q", msg, want)
		}
	}
}
