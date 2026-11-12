package main

import "testing"

// TestSanitize_Masks_REQ_INV_01 驟 Sanitize 將 auth_token＝/ct0= 後值遮為
// ***（保留鍵名）、無憑證輸入直通、重複出現全遮（REQ-INV-01）。
func TestSanitize_Masks_REQ_INV_01(t *testing.T) {
	in := "auth_token=abc; ct0=def; twid=u%3D1; lang=zh-CN"
	want := "auth_token=***; ct0=***; twid=u%3D1; lang=zh-CN"
	if got := Sanitize(in); got != want {
		t.Fatalf("Sanitize 回 %q，欲得 %q", got, want)
	}

	// 無憑證輸入直通。
	clean := "twid=u%3D1; lang=zh-CN"
	if got := Sanitize(clean); got != clean {
		t.Fatalf("無憑證輸入應直通，回 %q", got)
	}

	// 重複出現也遮。
	rep := "auth_token=a1; x=1; auth_token=a2"
	wantRep := "auth_token=***; x=1; auth_token=***"
	if got := Sanitize(rep); got != wantRep {
		t.Fatalf("重複憑證應全遮，回 %q，欲得 %q", got, wantRep)
	}

	// 空字串直通。
	if got := Sanitize(""); got != "" {
		t.Fatalf("空字串應直通，回 %q", got)
	}
}
