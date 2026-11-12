package headers

import (
	"reflect"
	"strings"
	"testing"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/fingerprint"
)

// 測試夾具 cookie 行（task-7-brief Step 1）：登入態、未登入態（僅 ct0，
// REQ-AUTH-005 情境）、空行。
const (
	bearerTest = "bearer-test"
	lineLogin  = "auth_token=AT; ct0=CT; lang=zh-TW; att=ATT"
	lineGuest  = "ct0=CT"
	lineEmpty  = ""
)

// testFP 為決定性指紋：v1 的 GraphQL 族值集不取用指紋欄位，僅固定參數
// 位置（參數保留給 v2 REST/UPLOAD 族的 accept-language 預設）。
var testFP = fingerprint.NewForTest("1920")

// findSlot 依名稱找槽位，回傳值與在場與否——僅供測試斷言使用。
func findSlot(slots []Slot, name string) (string, bool) {
	for _, s := range slots {
		if s.Name == name {
			return s.Value, true
		}
	}
	return "", false
}

// TestGraphQLPostFamily_Order_REQ_HDR_003 登入態＋referer 槽 → 輸出恰為
// REQ-HDR-003 逐字 11 槽表中「在場者」，順序與表逐字一致，值正確；缺席
// 的可選槽位（x-client-transaction-id、x-guest-token——slots 未給）整列
// 跳過。（REQ-HDR-003、REQ-RD-001）
func TestGraphQLPostFamily_Order_REQ_HDR_003(t *testing.T) {
	got, err := Build(FamGraphQLPost, bearerTest, lineLogin, testFP,
		map[string]string{"referer": "https://x.com/home"})
	if err != nil {
		t.Fatalf("Build(FamGraphQLPost) error = %v, 期望 nil", err)
	}

	// REQ-HDR-003 逐字順序表；value 為空字串標記「該槽缺席 → 整列跳過」。
	wantTable := []struct {
		name  string
		value string
	}{
		{"content-type", "application/json"},
		{"authorization", "Bearer " + bearerTest},
		{"x-twitter-auth-type", "OAuth2Session"},
		{"x-csrf-token", "CT"},
		{"x-twitter-client-language", "zh-tw"}, // REQ-HDR-010：cookie lang 小寫
		{"x-twitter-active-user", "yes"},
		{"x-client-transaction-id", ""}, // slots 未給 → 缺席（探測節：v1 恒不填）
		{"x-guest-token", ""},           // slots 未給 → 缺席
		{"x-att", "ATT"},                // REQ-HDR-011：cookie att 在場
		{"origin", "https://x.com"},
		{"referer", "https://x.com/home"},
	}

	// 缺席槽位確實整列跳過。
	for _, w := range wantTable {
		if w.value != "" {
			continue
		}
		if v, ok := findSlot(got, w.name); ok {
			t.Errorf("槽位 %q 不應輸出（slots 未給），got %q", w.name, v)
		}
	}

	// 在場槽位依表逐字排序、值逐字正確、數量恰為表上非空列數。
	var want []Slot
	for _, w := range wantTable {
		if w.value == "" {
			continue
		}
		want = append(want, Slot{Name: w.name, Value: w.value})
	}
	if len(got) != len(want) {
		t.Fatalf("輸出槽數 = %d, 期望 %d；got=%v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("輸出[%d] = %v, 期望 %v（順序與 REQ-HDR-003 表逐字一致）", i, got[i], w)
		}
	}
}

// TestGraphQLGetFamily_SameSequence_REQ_RD_001 FamGraphQLGet 與
// FamGraphQLPost 同序同槽，content-type: application/json 皆在
// （v1 中 query 型 GET 也用 POST 族標頭）。（REQ-RD-001）
func TestGraphQLGetFamily_SameSequence_REQ_RD_001(t *testing.T) {
	slots := map[string]string{"referer": "https://x.com/home"}
	post, err := Build(FamGraphQLPost, bearerTest, lineLogin, testFP, slots)
	if err != nil {
		t.Fatalf("Build(FamGraphQLPost) error = %v, 期望 nil", err)
	}
	get, err := Build(FamGraphQLGet, bearerTest, lineLogin, testFP, slots)
	if err != nil {
		t.Fatalf("Build(FamGraphQLGet) error = %v, 期望 nil", err)
	}
	if len(get) == 0 {
		t.Fatal("FamGraphQLGet 輸出為空, 期望與 FamGraphQLPost 同序同槽")
	}
	if !reflect.DeepEqual(get, post) {
		t.Fatalf("FamGraphQLGet 與 FamGraphQLPost 輸出不同：\nget=%v\npost=%v", get, post)
	}
	if v, ok := findSlot(get, "content-type"); !ok || v != "application/json" {
		t.Fatalf("content-type = (%q, %v), 期望 (\"application/json\", true)", v, ok)
	}
}

// TestAuthTypeOnlyWhenLoggedIn_REQ_HDR_009 x-twitter-auth-type:
// OAuth2Session 僅於 auth_token 存在且非空時輸出；未登入（含空行）整列
// 跳過。（REQ-HDR-009）
func TestAuthTypeOnlyWhenLoggedIn_REQ_HDR_009(t *testing.T) {
	cases := []struct {
		name    string
		line    string
		present bool
	}{
		{"logged_in", lineLogin, true},
		{"guest_no_auth_token", lineGuest, false},
		{"empty_line", lineEmpty, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Build(FamGraphQLPost, bearerTest, tc.line, testFP, nil)
			if err != nil {
				t.Fatalf("Build error = %v, 期望 nil", err)
			}
			v, ok := findSlot(got, "x-twitter-auth-type")
			if tc.present {
				if !ok {
					t.Fatalf("登入態應有 x-twitter-auth-type；got=%v", got)
				}
				if v != "OAuth2Session" {
					t.Fatalf("x-twitter-auth-type = %q, 期望 %q", v, "OAuth2Session")
				}
				return
			}
			if ok {
				t.Fatalf("未登入態不應有 x-twitter-auth-type；got %q", v)
			}
		})
	}
}

// TestClientLanguageFromCookie_REQ_HDR_010 x-twitter-client-language 取
// cookie lang 值小寫；lang 缺失或空 → 該槽整列跳過。（REQ-HDR-010）
func TestClientLanguageFromCookie_REQ_HDR_010(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		want  string
		exist bool
	}{
		{"lang_lowercased", lineLogin, "zh-tw", true},
		{"no_lang", lineGuest, "", false},
		{"empty_lang", "ct0=CT; lang=", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Build(FamGraphQLPost, bearerTest, tc.line, testFP, nil)
			if err != nil {
				t.Fatalf("Build error = %v, 期望 nil", err)
			}
			v, ok := findSlot(got, "x-twitter-client-language")
			if ok != tc.exist || (ok && v != tc.want) {
				t.Fatalf("x-twitter-client-language = (%q, %v), 期望 (%q, %v)", v, ok, tc.want, tc.exist)
			}
		})
	}
}

// TestCsrfFromCookieLine_REQ_HDR_008 x-csrf-token 逐字等於同一行 cookieLine
// 內的 ct0（第一個 = 切分、值可含 =）；ct0 缺失或空 → 該槽整列跳過。
// （REQ-HDR-008——同源於單次 cookieLine 載入）
func TestCsrfFromCookieLine_REQ_HDR_008(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		want  string
		exist bool
	}{
		{"same_line_ct0", lineGuest, "CT", true},
		{"value_contains_equals", "auth_token=AT; ct0=a=b", "a=b", true},
		{"no_ct0", "auth_token=AT", "", false},
		{"empty_line", lineEmpty, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Build(FamGraphQLPost, bearerTest, tc.line, testFP, nil)
			if err != nil {
				t.Fatalf("Build error = %v, 期望 nil", err)
			}
			v, ok := findSlot(got, "x-csrf-token")
			if ok != tc.exist || (ok && v != tc.want) {
				t.Fatalf("x-csrf-token = (%q, %v), 期望 (%q, %v)", v, ok, tc.want, tc.exist)
			}
		})
	}
}

// TestOptionalSlots_REQ_HDR_011 未登入態（cookie 無 att）：slots 未給
// x-att／x-guest-token／x-client-transaction-id → 三槽皆缺席；給
// x-client-transaction-id: TX → 出現於其表中位置（x-twitter-active-user
// 之後、x-guest-token 之前）；給 x-guest-token: GT → 同理出現於其位。
// （REQ-HDR-011 可選槽位模型）
func TestOptionalSlots_REQ_HDR_011(t *testing.T) {
	t.Run("absent_when_not_in_slots", func(t *testing.T) {
		got, err := Build(FamGraphQLPost, bearerTest, lineGuest, testFP, nil)
		if err != nil {
			t.Fatalf("Build error = %v, 期望 nil", err)
		}
		for _, name := range []string{"x-att", "x-guest-token", "x-client-transaction-id"} {
			if v, ok := findSlot(got, name); ok {
				t.Errorf("slots 未給 %q，不應輸出；got %q（全槽=%v）", name, v, got)
			}
		}
	})
	t.Run("xctid_at_table_position", func(t *testing.T) {
		got, err := Build(FamGraphQLPost, bearerTest, lineGuest, testFP,
			map[string]string{"x-client-transaction-id": "TX"})
		if err != nil {
			t.Fatalf("Build error = %v, 期望 nil", err)
		}
		wantNames := []string{
			"content-type", "authorization", "x-csrf-token", "x-twitter-active-user",
			"x-client-transaction-id", "origin",
		}
		if len(got) != len(wantNames) {
			t.Fatalf("輸出槽數 = %d, 期望 %d；got=%v", len(got), len(wantNames), got)
		}
		for i, name := range wantNames {
			if got[i].Name != name {
				t.Fatalf("輸出[%d].Name = %q, 期望 %q（XCTID 應在其表中位置）", i, got[i].Name, name)
			}
		}
		if v, _ := findSlot(got, "x-client-transaction-id"); v != "TX" {
			t.Fatalf("x-client-transaction-id = %q, 期望 %q", v, "TX")
		}
	})
	t.Run("guest_token_at_table_position", func(t *testing.T) {
		got, err := Build(FamGraphQLPost, bearerTest, lineGuest, testFP,
			map[string]string{"x-guest-token": "GT"})
		if err != nil {
			t.Fatalf("Build error = %v, 期望 nil", err)
		}
		wantNames := []string{
			"content-type", "authorization", "x-csrf-token", "x-twitter-active-user",
			"x-guest-token", "origin",
		}
		if len(got) != len(wantNames) {
			t.Fatalf("輸出槽數 = %d, 期望 %d；got=%v", len(got), len(wantNames), got)
		}
		for i, name := range wantNames {
			if got[i].Name != name {
				t.Fatalf("輸出[%d].Name = %q, 期望 %q（x-guest-token 應在其表中位置）", i, got[i].Name, name)
			}
		}
		if v, _ := findSlot(got, "x-guest-token"); v != "GT" {
			t.Fatalf("x-guest-token = %q, 期望 %q", v, "GT")
		}
	})
}

// TestLowercaseNames_REQ_HDR_013 所有輸出 Slot.Name 全小寫且非空——涵蓋
// 登記表槽位與未登記鍵（含混合大小寫輸入）。（REQ-HDR-013）
func TestLowercaseNames_REQ_HDR_013(t *testing.T) {
	scenarios := []struct {
		name  string
		line  string
		slots map[string]string
	}{
		{"logged_in_full", lineLogin, map[string]string{
			"referer": "https://x.com/home", "x-guest-token": "GT",
			"x-client-transaction-id": "TX", "Mixed-Case-Unknown": "v"}},
		{"guest_bare", lineGuest, nil},
		{"empty_line", lineEmpty, nil},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			got, err := Build(FamGraphQLPost, bearerTest, sc.line, testFP, sc.slots)
			if err != nil {
				t.Fatalf("Build error = %v, 期望 nil", err)
			}
			if len(got) == 0 {
				t.Fatal("輸出為空，無法斷言小寫")
			}
			for _, s := range got {
				if s.Name == "" {
					t.Errorf("Slot.Name 為空字串；got=%v", got)
				}
				if s.Name != strings.ToLower(s.Name) {
					t.Errorf("Slot.Name = %q, 期望全小寫（REQ-HDR-013）", s.Name)
				}
			}
			if _, given := sc.slots["Mixed-Case-Unknown"]; !given {
				return
			}
			if v, ok := findSlot(got, "mixed-case-unknown"); !ok || v != "v" {
				t.Fatalf("未登記鍵應小寫輸出：mixed-case-unknown = (%q, %v), 期望 (%q, true)", v, ok, "v")
			}
		})
	}
}

// TestUnknownKeysAlphabetical_REQ_HDR_002 slots 內未登記鍵依字母序追加於
// 全部登記槽位之後（z-custom 在 a-custom 之後），值逐字正確。
// （REQ-HDR-002）
func TestUnknownKeysAlphabetical_REQ_HDR_002(t *testing.T) {
	got, err := Build(FamGraphQLPost, bearerTest, lineLogin, testFP, map[string]string{
		"referer":  "https://x.com/home",
		"z-custom": "1",
		"a-custom": "2",
	})
	if err != nil {
		t.Fatalf("Build error = %v, 期望 nil", err)
	}
	wantNames := []string{
		"content-type", "authorization", "x-twitter-auth-type", "x-csrf-token",
		"x-twitter-client-language", "x-twitter-active-user", "x-att", "origin",
		"referer", "a-custom", "z-custom",
	}
	if len(got) != len(wantNames) {
		t.Fatalf("輸出槽數 = %d, 期望 %d；got=%v", len(got), len(wantNames), got)
	}
	for i, name := range wantNames {
		if got[i].Name != name {
			t.Fatalf("輸出[%d].Name = %q, 期望 %q（未登記鍵依字母序追加於尾）", i, got[i].Name, name)
		}
	}
	if v := got[len(got)-2]; v.Name != "a-custom" || v.Value != "2" {
		t.Fatalf("倒數第二槽 = %v, 期望 {a-custom 2}", v)
	}
	if v := got[len(got)-1]; v.Name != "z-custom" || v.Value != "1" {
		t.Fatalf("最後一槽 = %v, 期望 {z-custom 1}", v)
	}
}

// TestUnsupportedFamily 其餘四族（REST GET／REST FORM／UPLOAD／DOC）未在
// v1 實作 → Build 回 error 非 nil 且無輸出。
func TestUnsupportedFamily(t *testing.T) {
	for _, f := range []Family{FamRESTGet, FamRESTForm, FamUpload, FamDoc} {
		got, err := Build(f, bearerTest, lineLogin, testFP, nil)
		if err == nil {
			t.Errorf("Build(Family=%d) error = nil, 期望「此標頭族未在 v1 實作」錯誤", int(f))
		}
		if got != nil {
			t.Errorf("Build(Family=%d) 輸出 = %v, 期望 nil", int(f), got)
		}
	}
}
