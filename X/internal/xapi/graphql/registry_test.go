package graphql

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// testRegistryJSON 是 brief 指定的合成 fixture（測試專用，非 X 原文）：
// frontendSha 為 null（REQ-GQL-011 容忍）、HomeTimeline 未宣告任何 feature
// switch（REQ-GQL-004 情境）。
const testRegistryJSON = `{"generatedAt":"2026-11-12T00:00:00+08:00","frontendSha":null,
 "operations":[
  {"operationName":"Viewer","operationType":"query","queryId":"VIEWERID","featureSwitches":["withCommunitiesMemberships"],"fieldToggles":[]},
  {"operationName":"HomeTimeline","operationType":"query","queryId":"HOMEID","featureSwitches":[],"fieldToggles":[]}],
 "featureDefaults":{"withCommunitiesMemberships":true}}`

// writeRegistry 將合成 fixture 寫入暫存檔，回傳路徑供 LoadRegistry 使用。
func writeRegistry(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, []byte(testRegistryJSON), 0o644); err != nil {
		t.Fatalf("寫入測試註冊表：%v", err)
	}
	return path
}

// loadTestRegistry 載入合成 fixture，失敗即終止測試。
func loadTestRegistry(t *testing.T) *Registry {
	t.Helper()
	reg, err := LoadRegistry(writeRegistry(t))
	if err != nil {
		t.Fatalf("LoadRegistry error = %v, 期望 nil", err)
	}
	return reg
}

// TestFeaturesEmptyOmitted_REQ_GQL_004_024 操作未宣告任何 feature switch →
// Features 回 nil，呼叫端據此整鍵省略 features（REQ-GQL-004/024）。
func TestFeaturesEmptyOmitted_REQ_GQL_004_024(t *testing.T) {
	reg := loadTestRegistry(t)
	op, err := reg.Operation("HomeTimeline")
	if err != nil {
		t.Fatalf("Operation(HomeTimeline) error = %v, 期望 nil", err)
	}
	if got := op.Features(reg, nil); got != nil {
		t.Fatalf("HomeTimeline.Features(nil) = %v, 期望 nil", got)
	}
	if got := op.Features(reg, map[string]bool{}); got != nil {
		t.Fatalf("HomeTimeline.Features(空 overrides) = %v, 期望 nil", got)
	}
}

// TestFeaturesDefaultsAndOverrides_REQ_GQL_015 features 依宣告的 switch
// 清單逐一取值：default 取 featureDefaults（缺省 false），overrides 命中者
// 覆寫（REQ-GQL-015）。
func TestFeaturesDefaultsAndOverrides_REQ_GQL_015(t *testing.T) {
	reg := loadTestRegistry(t)
	op, err := reg.Operation("Viewer")
	if err != nil {
		t.Fatalf("Operation(Viewer) error = %v, 期望 nil", err)
	}

	got := op.Features(reg, nil)
	want := OrderedParams{{K: "withCommunitiesMemberships", V: "true"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Viewer.Features(nil) = %v, 期望 %v", got, want)
	}

	got = op.Features(reg, map[string]bool{"withCommunitiesMemberships": false})
	want = OrderedParams{{K: "withCommunitiesMemberships", V: "false"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Viewer.Features(override false) = %v, 期望 %v", got, want)
	}
}

// TestFieldTogglesFallback_REQ_GQL_016 fieldToggles 的 v1 退化序：explicit
// 非空 → 以 explicit 為準；否則用操作宣告的 fieldToggles 全集，值取
// featureDefaults（缺省 false）（REQ-GQL-016）。
func TestFieldTogglesFallback_REQ_GQL_016(t *testing.T) {
	reg := loadTestRegistry(t) // featureDefaults 無 withArticlePlainText
	op := &Operation{
		Name:                 "PublishArticle",
		QueryID:              "ARTICLEID",
		Method:               "POST",
		DeclaredFieldToggles: []string{"withArticlePlainText"},
	}

	got := op.FieldToggles(reg, nil)
	want := OrderedParams{{K: "withArticlePlainText", V: "false"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FieldToggles(註冊表回退) = %v, 期望 %v", got, want)
	}

	got = op.FieldToggles(reg, map[string]string{"withArticlePlainText": "true"})
	want = OrderedParams{{K: "withArticlePlainText", V: "true"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FieldToggles(explicit) = %v, 期望 %v", got, want)
	}
}

// TestRegistryLookup_REQ_GQL_014 查無操作 → *xerrors.MissingFieldError，
// 訊息含操作名（REQ-GQL-014）。
func TestRegistryLookup_REQ_GQL_014(t *testing.T) {
	reg := loadTestRegistry(t)
	_, err := reg.Operation("Nope")
	var missing *xerrors.MissingFieldError
	if !errors.As(err, &missing) {
		t.Fatalf("Operation(Nope) error = %T(%v), 期望 *xerrors.MissingFieldError", err, err)
	}
	if !strings.Contains(err.Error(), "Nope") {
		t.Fatalf("錯誤訊息未含操作名：%q", err.Error())
	}
}

// TestOperationPathURL_REQ_GQL_020_021 path 與 url 形式
// （REQ-GQL-020/021）。
func TestOperationPathURL_REQ_GQL_020_021(t *testing.T) {
	reg := loadTestRegistry(t)
	op, err := reg.Operation("HomeTimeline")
	if err != nil {
		t.Fatalf("Operation(HomeTimeline) error = %v, 期望 nil", err)
	}
	if got, want := op.Path(), "/i/api/graphql/HOMEID/HomeTimeline"; got != want {
		t.Fatalf("Path() = %q, 期望 %q", got, want)
	}
	if got, want := op.URL(), "https://x.com/i/api/graphql/HOMEID/HomeTimeline"; got != want {
		t.Fatalf("URL() = %q, 期望 %q", got, want)
	}
}

// TestRegistryShape_REQ_GQL_010_011 頂層欄位讀回正確：generatedAt、
// frontendSha（JSON null 容忍）（REQ-GQL-010/011）；operationType 推導
// 送出方法（mutation→POST、query→GET）。
func TestRegistryShape_REQ_GQL_010_011(t *testing.T) {
	reg := loadTestRegistry(t)
	if got, want := reg.generatedAt, "2026-11-12T00:00:00+08:00"; got != want {
		t.Fatalf("generatedAt = %q, 期望 %q", got, want)
	}
	if reg.frontendSha != nil {
		t.Fatalf("frontendSha = %q, 期望 nil（JSON null 容忍）", *reg.frontendSha)
	}
	if len(reg.operations) != 2 {
		t.Fatalf("operations 數 = %d, 期望 2", len(reg.operations))
	}
	if !reg.featureDefaults["withCommunitiesMemberships"] {
		t.Fatal("featureDefaults[withCommunitiesMemberships] = false, 期望 true")
	}
	viewer, err := reg.Operation("Viewer")
	if err != nil {
		t.Fatalf("Operation(Viewer) error = %v, 期望 nil", err)
	}
	if got := viewer.Method; got != "GET" {
		t.Fatalf("Viewer.Method = %q, 期望 GET（operationType=query 推導）", got)
	}
}
