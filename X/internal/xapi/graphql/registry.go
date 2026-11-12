package graphql

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// Registry 是已載入的操作註冊表（REQ-GQL-010：單一 JSON 快照、執行期唯讀）。
// 欄位於載入後即固定，供同套件內查詢。
type Registry struct {
	generatedAt     string
	frontendSha     *string // nil 對應 JSON null（REQ-GQL-011 容忍）
	operations      []*Operation
	byName          map[string]*Operation
	featureDefaults map[string]bool
}

// Operation 是單一 GraphQL 操作的 metadata。
type Operation struct {
	// Name 是操作名（如 "HomeTimeline"）。
	Name string
	// QueryID 是註冊表記錄的 queryId。
	QueryID string
	// Method 由 operationType 推導：mutation→POST、query→GET
	//（REQ-GQL-001/003）。呼叫端可顯式覆寫（REQ-GQL-005：HomeTimeline 與
	// SearchTimeline 屬 query 型但必須以 POST 送出）。
	Method string
	// FeatureSwitches 是操作宣告的 feature switch 清單（序即宣告序）。
	FeatureSwitches []string
	// DeclaredFieldToggles 是操作宣告的 fieldToggles 全集（保守回退用，
	// REQ-GQL-016）。欄位名不與同名解析方法 FieldToggles 衝突（Go 不允許
	// 欄位與方法同名），JSON 鍵仍為 fieldToggles。
	DeclaredFieldToggles []string
}

// registryFile／registryOperation 對應快照 JSON 的頂層與操作條目
// （REQ-GQL-011/012）；frontendSha 用指標以區分 null。operations 以 slice
// 載入保序。
type registryFile struct {
	GeneratedAt     string              `json:"generatedAt"`
	FrontendSha     *string             `json:"frontendSha"`
	Operations      []registryOperation `json:"operations"`
	FeatureDefaults map[string]bool     `json:"featureDefaults"`
}

type registryOperation struct {
	OperationName   string   `json:"operationName"`
	OperationType   string   `json:"operationType"`
	QueryID         string   `json:"queryId"`
	FeatureSwitches []string `json:"featureSwitches"`
	FieldToggles    []string `json:"fieldToggles"`
}

// LoadRegistry 讀取並解析註冊表快照（REQ-GQL-010/011/012），回傳唯讀註冊表。
func LoadRegistry(path string) (*Registry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("graphql: load registry %s: %w", path, err)
	}
	var file registryFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("graphql: load registry %s: %w", path, err)
	}
	reg := &Registry{
		generatedAt:     file.GeneratedAt,
		frontendSha:     file.FrontendSha,
		byName:          make(map[string]*Operation, len(file.Operations)),
		featureDefaults: file.FeatureDefaults,
	}
	for i := range file.Operations {
		e := &file.Operations[i]
		op := &Operation{
			Name:                 e.OperationName,
			QueryID:              e.QueryID,
			Method:               deriveMethod(e.OperationType),
			FeatureSwitches:      e.FeatureSwitches,
			DeclaredFieldToggles: e.FieldToggles,
		}
		reg.operations = append(reg.operations, op)
		reg.byName[op.Name] = op
	}
	return reg, nil
}

// deriveMethod 由 operationType 推導送出方法：mutation→POST，其餘（query）
// →GET（REQ-GQL-001/003）。
func deriveMethod(operationType string) string {
	if operationType == "mutation" {
		return "POST"
	}
	return "GET"
}

// Operation 依名稱查詢操作 metadata；查無時回傳 *xerrors.MissingFieldError
// （REQ-GQL-014），訊息含操作名並提示重新產生註冊表。回傳值與註冊表共用，
// Method 欄位可由呼叫端顯式覆寫（REQ-GQL-005）。
func (r *Registry) Operation(name string) (*Operation, error) {
	op, ok := r.byName[name]
	if !ok {
		return nil, &xerrors.MissingFieldError{
			Msg: fmt.Sprintf("operation %q: not found in registry; regenerate the registry snapshot", name),
		}
	}
	return op, nil
}

// Path 回傳 /i/api/graphql/<queryId>/<name>（REQ-GQL-020）。
func (o *Operation) Path() string {
	return "/i/api/graphql/" + o.QueryID + "/" + o.Name
}

// URL 回傳 https://x.com/i/api/graphql/<queryId>/<name>（REQ-GQL-021）。
func (o *Operation) URL() string {
	return "https://x.com" + o.Path()
}

// Features 依操作宣告的 featureSwitches 清單逐一取值：default 取
// featureDefaults（缺鍵 false），overrides 命中者覆寫（REQ-GQL-015）；對未
// 宣告的 switch 不生效。鍵序＝宣告序。解析後為空 → 回 nil，呼叫端據此整鍵
// 省略 features（REQ-GQL-004/024）。
func (o *Operation) Features(reg *Registry, overrides map[string]bool) OrderedParams {
	if len(o.FeatureSwitches) == 0 {
		return nil
	}
	params := make(OrderedParams, 0, len(o.FeatureSwitches))
	for _, name := range o.FeatureSwitches {
		value := false
		if reg != nil {
			value = reg.featureDefaults[name] // map 讀取缺鍵得零值 false
		}
		if v, ok := overrides[name]; ok {
			value = v
		}
		params = append(params, Param{K: name, V: strconv.FormatBool(value)})
	}
	return params
}

// FieldToggles 解析 fieldToggles 的 v1 退化序（REQ-GQL-016/023）：explicit
// 非空 → 全用 explicit（V 為固定值字串；map 無插入序，按鍵名排序得決定性
// 輸出）；否則用操作宣告的 fieldToggles 全集，值取 featureDefaults（缺鍵
// false）。解析後為空 → 回 nil。
func (o *Operation) FieldToggles(reg *Registry, explicit map[string]string) OrderedParams {
	if len(explicit) > 0 {
		keys := make([]string, 0, len(explicit))
		for k := range explicit {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		params := make(OrderedParams, 0, len(keys))
		for _, k := range keys {
			params = append(params, Param{K: k, V: explicit[k]})
		}
		return params
	}
	if len(o.DeclaredFieldToggles) == 0 {
		return nil
	}
	params := make(OrderedParams, 0, len(o.DeclaredFieldToggles))
	for _, name := range o.DeclaredFieldToggles {
		value := false
		if reg != nil {
			value = reg.featureDefaults[name]
		}
		params = append(params, Param{K: name, V: strconv.FormatBool(value)})
	}
	return params
}
