// session：view↔engine 接縫（設計 §3.3 的 app 層 Session 構造）。view 僅
// 依賴 xSession 介面（測試以 fakeSession 注入，見 view_login_test.go）；
// engineSession 為唯一正式實作，由 assembleSession 依 brief 鏈組裝：
// fingerprint.Get() → auth.NewAuth(bearer, WithCookieStore) →（非空行時）
// PrepareAuth(cookieLine) → graphql.LoadRegistry(註冊表快照) →
// transport.New(fp, Options{ProxyURL}) → api.NewSender。帳號路徑以
// newSessionFor 組裝（per-account store 接入 write-through，§8.3 寫回
// 帳號化）；login 流此時尚無 Account，逕以 assembleSession 組裝。
package main

import (
	"os"
	"path/filepath"
	"sync"

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
	mediaItem      = api.MediaItem
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

// newStore 是帳號儲存層的建構 seam（§8.3 寫回帳號化）：正式路徑走
// NewCookieStore（service 空字串＝MyGO 預設 app identifier service）；測試
// 以記錄器覆寫隔離（session_switch_test.go），絕不打真實預設 service。
// Task 6 起 newSessionFor 經 newAccountCookieStore（鍵內容碰撞裁決的
// adapter）使用本 seam——adapter 包於 seam 之下，raw 行讀寫收斂為
// Account JSON 的 CookieLine 讀寫。
var newStore = func(service, key string) (auth.CookieStore, error) { return NewCookieStore(service, key) }

// switchSessionFactory 是切換序列的會話工廠 seam（§8.4 步驟 4）：正式路徑
// newSessionFor（帳號鍵 adapter 儲存層）；測試注入替身（switch_test.go），
// 絕不組真實引擎鏈／打真實網路。包級函式值形（同 migrateSessionFactory、
// dualWrite；t.Cleanup 還原）。
var switchSessionFactory = func(acc Account, proxyURL string) (xSession, error) {
	return newSessionFor(acc, proxyURL)
}

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
	// HomeLatest 讀取 Following（最新）時間線一頁（Task 7 的引擎操作
	// HomeLatestTimeline；與 HomeTimeline 同構，D16——P0' 實證）：count
	// 固定 homePageSize；seen 同 Home。
	HomeLatest(cursor string, seen []string) (*timelineResult, error)
	// Search 讀取一頁搜尋結果（Task 9 的引擎操作 SearchTimeline；Task 9
	// 裁決：恆 POST、product 為必填變數——由呼叫端傳入，搜尋結果頁用
	// "Top"；無 seenTweetIds）。count 固定 homePageSize（同 Home）。
	Search(query, product, cursor string) (*timelineResult, error)
	// Close 釋放會話資源（transport 收斂；冪等）：switchTo 步驟 6 於舊
	// 會話替換前呼叫（設計 §8.4——簡報 §1.3 實測根因）。
	Close() error
}

// engineSession 是 xSession 的正式實作：小結構體包 auth＋sender，並持有
// transport 以供 Close 收斂（client.go:168 既有 API 的 app 層接線）。
type engineSession struct {
	auth      *auth.Auth
	sender    *api.Sender
	tr        *transport.Client
	closeOnce sync.Once // Close 冪等：transport 釋放僅執行一次
}

// 編譯期斷言：engineSession 滿足接縫。
var _ xSession = (*engineSession)(nil)

// Close 釋放會話持有的 transport 資源：冪等——以 sync.Once 保證僅關閉
// 一次，其後呼叫（含 tr 未接線的零值會話）回 nil。Task 6 切換序列於舊
// 會話替換前呼叫（設計 §8.4 步驟 6）。
func (s *engineSession) Close() error {
	if s == nil {
		return nil
	}
	var err error
	s.closeOnce.Do(func() {
		if s.tr != nil {
			err = s.tr.Close()
		}
	})
	return err
}

// newSessionFor 以帳號組裝會話（§8.3 寫回帳號化；設計 §8.4 會話工廠）：
// 以該帳號專屬的 adapter 儲存層（x-account-<user_id> 鍵經
// newAccountCookieStore——Task 6 鍵內容碰撞裁決：write-through 的 raw 行
// 收斂為 Account JSON 的 CookieLine 讀寫，不得覆蓋中繼資料）接入 Auth——
// Sender 回應 Set-Cookie 的同步 write-through 落活躍帳號鍵、絕不覆蓋
// 他帳（Review Focus 2）；PrepareAuth(acc.CookieLine) 完成備料（cookie 行
// 已知）。其餘鏈路同 assembleSession。儲存層建構失敗原樣向上。
func newSessionFor(acc Account, proxyURL string) (*engineSession, error) {
	return assembleSession(acc.CookieLine, proxyURL, newAccountCookieStore(acc.UserID))
}

// assembleSession 組裝引擎會話（組裝鏈唯一本體：login 流與 newSessionFor
// 皆以此收斂，遷移等需指定 store 的呼叫端亦復用）：
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
	return &engineSession{auth: a, sender: api.NewSender(tr, a, reg, fp), tr: tr}, nil
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

// HomeLatest 實作 xSession（Task 7 的 HomeLatestTimeline 引擎操作）。
func (s *engineSession) HomeLatest(cursor string, seen []string) (*timelineResult, error) {
	return s.sender.HomeLatestTimeline(cursor, homePageSize, seen)
}

// Search 實作 xSession（Task 9 的 SearchTimeline 引擎操作；product 必
// 填——Task 9 裁決，由呼叫端傳入，搜尋結果頁用 "Top"）。
func (s *engineSession) Search(query, product, cursor string) (*timelineResult, error) {
	return s.sender.SearchTimeline(query, product, cursor, homePageSize)
}
