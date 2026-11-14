// session：view↔engine 接縫（設計 §3.3 的 app 層 Session 構造）。view 僅
// 依賴 xSession 介面（測試以 fakeSession 注入，見 view_login_test.go）；
// engineSession 為唯一正式實作，由 newSession 依 brief 鏈組裝：
// fingerprint.Get() → auth.NewAuth(bearer, WithCookieStore) →（非空行時）
// PrepareAuth(cookieLine) → graphql.LoadRegistry(註冊表快照) →
// transport.New(fp, Options{ProxyURL}) → api.NewSender。
package main

import (
	"os"
	"path/filepath"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/api"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/fingerprint"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/graphql"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/transport"
)

// timelineResult／post／viewerInfo 是引擎資料型別的 app 層別名：view 與
// 測試替身得以字面值構造（引擎型別本身為純資料結構；view 對引擎的唯一
// 依賴面是 xSession 介面，別名不擴大依賴）。viewerInfo 供 login 流以
// Validate 回傳值建 Account（REQ-WEB-069：身分與憑證綁定）。
type (
	timelineResult = api.TimelineResult
	post           = api.Post
	viewerInfo     = api.ViewerInfo
)

// homePageSize 是每次翻頁的 count（契約 3：HomeTimeline(cursor, 20, seen)）。
const homePageSize = 20

// registryPath 解析註冊表快照位置：優先可執行檔同目錄的扁平檔名
// （mygo build 的 resources 會攤平路徑——static/graphql.json 出貨為
// exe 旁的 graphql.json），退回工作目錄相對路徑（go run／開發情境，
// cwd 為模組根，static/ 在原位）。兩處皆無時由 LoadRegistry 回錯
// （login 頁呈現，fail-closed）。
func registryPath() string {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "graphql.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join("static", "graphql.json")
}

// engineStore 是引擎接線的系統憑證庫儲存層：main 於啟動時指派
// （NewCookieStore），newSession 以 WithCookieStore 接入——Sender 回應
// Set-Cookie 的同步 write-through（Task 9）與 login 流的首次 Save（契約 2）
// 共用同一儲存層。引擎自身不主動 Save（REQ-AUTH-002）。
var engineStore auth.CookieStore

// xSession 是 view↔engine 的窄接縫：login/home view 僅依賴本介面，測試以
// 假實作注入（view 不直接依賴具體 engine 型別）。
type xSession interface {
	// Validate 解析單行 cookie（PrepareAuth）並以 Viewer 驗證身分
	// （REQ-RD 表 get_viewer）：成功回傳 Viewer 身分（login 流以此建
	// Account，REQ-WEB-069），失敗原樣向上（kind 分類由 xerrors 型別
	// 承載，絕不清除憑證）。
	Validate(cookieLine string) (viewerInfo, error)
	// Home 讀取一頁時間線：count 固定 homePageSize；seen 為已顯示
	// work_id 累積（seenTweetIds，跨頁去重，Task 10 介面契約）。
	Home(cursor string, seen []string) (*timelineResult, error)
}

// engineSession 是 xSession 的正式實作：小結構體包 auth＋sender。
type engineSession struct {
	auth   *auth.Auth
	sender *api.Sender
}

// 編譯期斷言：engineSession 滿足接縫。
var _ xSession = (*engineSession)(nil)

// newSession 以引擎接線的系統憑證庫儲存層（engineStore）組裝會話——
// assembleSession 的薄包裝（login 流以空行起手，備料與驗證由
// Validate(cookieLine) 完成）。
func newSession(cookieLine, proxyURL string) (*engineSession, error) {
	return assembleSession(cookieLine, proxyURL, engineStore)
}

// assembleSession 組裝引擎會話（preflight 裁決：newSession 本體重構為此形，
// 供遷移等需指定 store 的呼叫端復用組裝鏈）：
//
//   - store 非 nil → 以 WithCookieStore 接入（Sender 回應 Set-Cookie 的
//     write-through 落該儲存層）；nil → 不帶（遷移的臨時會話：只驗身分，
//     不接 write-through 儲存層）。
//   - cookieLine 非空 → 於組裝內 PrepareAuth（自動登入路徑，供未來以已存
//     cookie 靜默續登；v1 未接線）。
//   - cookieLine 為空 → 略過（login 流由 Validate(cookieLine) 完成備料與
//     驗證；空行不觸發 ct0 本地生成，REQ-AUTH-005）。
//
// 代理語義（D10：正常用途直連，需求方 2026-11-13 裁決）：proxyURL 非空
// 時 transport 經代理；空時以 AllowDirect 直連（OQ12 探測實證）。註冊表
// 載入失敗回原錯——由 login 頁錯誤區呈現。
func assembleSession(cookieLine, proxyURL string, store auth.CookieStore) (*engineSession, error) {
	fp := fingerprint.Get()
	opts := []auth.AuthOption(nil)
	if store != nil {
		opts = []auth.AuthOption{auth.WithCookieStore(store)}
	}
	a := auth.NewAuth("", opts...) // bearer 空 → 預設公開 web bearer（REQ-AUTH-002）
	if cookieLine != "" {
		if err := a.PrepareAuth(cookieLine); err != nil {
			return nil, err
		}
	}
	reg, err := graphql.LoadRegistry(registryPath())
	if err != nil {
		return nil, err
	}
	tr, err := transport.New(fp, transport.Options{ProxyURL: proxyURL, AllowDirect: true})
	if err != nil {
		return nil, err
	}
	return &engineSession{auth: a, sender: api.NewSender(tr, a, reg, fp)}, nil
}

// Validate 實作 xSession：PrepareAuth → Viewer（回傳身分，login 流以之建
// Account，REQ-WEB-069）。失敗原樣向上（kind 分類由 xerrors 型別承載，
// login 頁經 errorText 映射顯示，絕不清除憑證）。
func (s *engineSession) Validate(cookieLine string) (viewerInfo, error) {
	if err := s.auth.PrepareAuth(cookieLine); err != nil {
		return viewerInfo{}, err
	}
	return s.sender.Viewer()
}

// Home 實作 xSession。
func (s *engineSession) Home(cursor string, seen []string) (*timelineResult, error) {
	return s.sender.HomeTimeline(cursor, homePageSize, seen)
}
