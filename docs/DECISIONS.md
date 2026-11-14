# 決策記錄（DECISIONS）

> 本檔登錄「MyGO X 引擎各輪」（v1、v1.1 及後續輪次）對規格
> （`XAPIS_BEHAVIOR_SPEC.md`，唯一行為參考）的解讀與偏離，作為
> `docs/designs/x-engine-v1.md` §3.8 所述「開工時回寫」的落點。
> 本檔不改動凍結的規格本身；每筆裁決一節，含「依據」與「偏離登錄」欄，
> 並標「適用版本」（v1 裁決由 v1.1 沿用者如實標註；規格 REQ 條目的輪次
> 適用性另見〈規格 REQ×版本適用性對照表〉節）。
> 基準日：2026-11-11（Task 0，P0 前置裁決）；v1.1 輪續編 D11–D22
> （2026-11-14，Task 15 定稿入編）。

## 已定案裁決

### D1 OQ3：HomeTimeline 以 7.8.2 方法表為準

*   **適用版本**：v1 定案；v1.1 沿用（D16 的 HomeLatestTimeline 同構套用）。
*   **裁決**：HomeTimeline 以 7.8.2 方法表為準——無已讀 → `GET` ＋
    `requestContext=launch`；有已讀 → `POST` ＋ `seenTweetIds` 置 variables 末。
*   **REQ-GQL-005 解讀**：「帶 seenTweetIds 的 query 型必須 POST」。
*   **三個落地點**：線路形態、參數順序、XCTID 的 METHOD 輸入（REQ-XCT-007）。
*   **依據**：設計文件 §3.4（資料流第 2 步：GET/POST 雙形態的有序組裝細節）與
    Open Questions OQ3；Alternative I（POST 單形態）被否決的理由。
*   **偏離登錄**：無（對規格為解讀，非偏離）。雙形態的正確性不做先驗假設：
    由 P0 探測實驗矩陣（HomeTimeline GET／POST 兩形態）與 L2 煙霧一併驗證，
    任一形態被服務端拒絕即回報並改判（見〈探測實驗結論〉節）。

### D2 OQ5：七層套件樹

*   **適用版本**：v1 定案；v1.1 沿用。
*   **裁決**：七層套件樹
    `X/internal/xapi/{fingerprint,transport,auth,headers,graphql,xctid,api}` ＋
    `xerrors`。
*   **依據**：設計文件 §3.2（套件組成與職責表，OQ5＝七層）；Alternative J
    （收斂單套件）被否決的理由——規格域（TRN/AUTH/HDR/GQL/XCT）與程式碼
    一一對應。xctid 套件在探測通過路徑下整個不建。
*   **偏離登錄**：無（目錄樹屬本專案自由度，非規格條目）。

### D3 TRN-011 偏離：傳輸層以 bogdanfinn/tls-client 承接

*   **適用版本**：v1 定案；v1.1 沿用。
*   **裁決**：curl_cffi 指名改以 `bogdanfinn/tls-client`（chrome_150 profile）
    承接，uTLS 單一 TLS 棧。
*   **依據**：設計文件「提出的解法」（純 Go、零 cgo、符合 MyGO 單一執行檔
    屬性）與 G4（TRN-011 偏離登錄即本條）；能力前提由本檔〈spike 結論〉節
    實證（chrome_150 存在、標頭序、重定向控制、per-client proxy 四點全過）。
*   **偏離登錄**：規格 TRN-011 指名 curl_cffi（Python 生態）——Go 實作改以
    bogdanfinn/tls-client v1.18.0 承接同等職責（Chrome TLS 指紋模擬）。
    釘選版本與證據見〈spike 結論〉節。

### D4 §2-6 錯誤字串偏離

*   **適用版本**：v1 定案；v1.1 沿用。
*   **裁決**：規格「錯誤字串逐字保留」、REQ-AUTH-004 精確文案、REQ-GQL-007
    訊息格式**不適用**；行為層僅保留錯誤分類（kind/code 機械可分，
    REQ-LOG-042/043 不變）。
*   **依據**：設計文件 Alternative G 被否決的理由——本專案為 clean-room
    重寫，不繼承原專案的使用者可見契約；文案由 app 層決定（OQ10）。
*   **偏離登錄**：對規格 §2-6 的「逐字保留」約束、REQ-AUTH-004 的精確文案、
    REQ-GQL-007 的訊息格式均登錄為偏離。缺 `ct0` 仍拋錯（REQ-AUTH-004 的
    行為面），僅文案不受約束。

### D5 12.2 錯誤文案存活清單

*   **適用版本**：v1 定案；v1.1 沿用。
*   **裁決**：保留 `proxy_timeout` / `login_timeout` / `proxy_configuration`；
    閘道專屬四條（`gateway_auth` / `gateway_busy` / `gateway_unavailable` /
    `gateway_error`）映射為直連語義新文案。
*   **依據**：設計文件 §3.2 `xerrors` 列（閘道錯誤文案存活清單）與 §3.8
    （錯誤文案存活清單屬回寫本檔的裁決之一）。
*   **偏離登錄**：閘道專屬四條原文案隨 D8（閘道移除）一併失準，映射為
    直連語義新文案後，code 值維持機械可分（REQ-LOG-042/043 不變）。

### D6 代理解析裁減

*   **適用版本**：v1 定案；直連語義其後經 D10 修訂，v1.1 沿用修訂結果。
*   **裁決**：`X_PROXY` → `HTTPS_PROXY`/`ALL_PROXY` → MyGO netproxy → 失敗
    關閉；移除 Windows 註冊表層與本機埠探測層（後者以安全理由）。
*   **依據**：設計文件 §3.7（代理解析鏈）與 Open Questions OQ6（建議預設值
    即此鏈；前置「驗證 netproxy 已涵蓋系統代理職能」屬 app 層整合工作）。
*   **偏離登錄**：規格中的 Windows 註冊表代理層與本機埠探測層不予實作
    （本機埠探測層以安全理由移除）。失敗關閉維持 REQ-INV-02；直連回退仍為
    Alternative F 否決狀態，最終立場待 OQ12（見〈探測實驗結論〉直連對照）。

### D7 v1 不跟隨重定向

*   **適用版本**：v1 定案；v1.1 沿用（完整語義仍隨 v2）。
*   **裁決**：v1 不跟隨重定向（REQ-TRN-040 以嚴格超集滿足，完整語義隨 v2）。
*   **依據**：設計文件 §3.2 transport 列（「重定向一律不跟隨」）與 §3.4 第 5
    步；G4（TRN-040 以「v1 一律不跟隨重定向」嚴格超集滿足）。實作前提
    `WithNotFollowRedirects` 的存在與行為由〈spike 結論〉第 3 點實證。
*   **偏離登錄**：無（嚴格超集：不跟隨是規格重定向語義的子集行為，v2 再補
    完整語義）。

### D8 閘道移除

*   **適用版本**：v1 定案；v1.1 沿用。
*   **裁決**：第 10 章與 REQ-TRN-030..041 不適用。
*   **依據**：設計文件 Alternative B 被否決的理由——閘道存在理由是隔離
    Python 的 TLS 限制；全 Go 後只剩程序間 RPC 的部署與除錯負擔。
*   **偏離登錄**：規格第 10 章（Go TLS 閘道子程序）與 REQ-TRN-030..041
    （閘道客戶端契約）整域登錄為不適用；相關錯誤文案的處置見 D5。

### D9 transport 無 cookie jar；Cookie 標頭由呼叫端注入

*   **適用版本**：v1 定案；v1.1 沿用（write-through 帳號化不改變本不變量）。
*   **裁決**：transport 不持有任何 cookie 狀態——tls-client 的 jar 掛點以
    no-op cookie jar 中和；`Cookie` 標頭由呼叫端（Sender，自 XAuth 取值）
    於請求上注入。Header-Order 於 cookie 存在時恆涵蓋之，並錨定於
    `priority` 之前（Chrome 觀察序）。
*   **依據**：REQ-AUTH-001（XAuth 為唯一憑證持有者）與 Task 9 設計
    （回應 `Set-Cookie` 的差異經 `UpdateCookies` 記帳並寫回 XAuth，非由
    傳輸層自建第二持有者）。任務審查發現：jar 若存在，fhttp 的 send 會在
    Header-Order 附加之後以 `AddCookie` 追加 `Cookie`，使 cookie 逃出順序
    控制落於 `priority` 之後（偏離 Chrome wire 序），且構成 REQ-AUTH-001
    的第二憑證持有者——故必須中和。實測附帶事實：plain
    `NewHttpClient`（v1.18.0）jar 為 nil、尚無注入，但以明確 no-op 釘死
    不變式，免受上游預設 jar 變更影響。
*   **偏離登錄**：REQ-TRN-012 的標頭補齊語義登錄為「cookie 不補、不改值，
    僅排序」；transport 的 cookie 職責限於 Header-Order 涵蓋與錨定。
    代理格式非法（無 scheme/host 或不支援的 scheme）同於無代理，於
    `New` 即 fail-closed 拒絕（REQ-INV-02），不待送出、不得直連。

### D10 代理改為可選：正常用途直連（需求方 2026-11-13 裁決）

*   **適用版本**：v1 輪內修訂（2026-11-13）；v1.1 沿用。
*   **裁決**：桌面 app（正常用途、自有帳號）不再要求代理——`ResolveProxy`
    三層皆無時回 `("", nil)` 直連；transport 以 `Options.AllowDirect: true`
    於 app 層顯式選入（引擎預設仍 fail-closed）。系統偵測命中但預檢失敗 →
    視同未偵測到（落入直連）；使用者顯式配置（`X_PROXY` 等）仍原樣採用。
*   **依據**：OQ12 探測實證（直連可達且被正常接受、0×226/344，見探測節）＋
    需求方裁決「設計給正常用途，代理應該用不到」。REQ-INV-02 的
    fail-closed 原契約針對多帳號控制台的強制每帳號代理部署；本桌面 app
    單一自有帳號情境予以修訂。
*   **偏離登錄**：REQ-INV-02 於本 app 之適用修訂為「代理可選、直連為基線、
    顯式配置的代理仍被採用」。成本：直連暴露真實 IP 且無指紋代理保護——
    若 X 日後對該出口 IP 形態收緊風控（226/344 上升），重新啟用代理即可
    （transport 支援不變）。
*   **配套**：註冊表快照經 mygo.json `resources` 出貨至執行檔旁（MyGO 會
    攤平路徑：出貨為 exe 旁的 `graphql.json`；`registryPath()` 優先 exe
    目錄、退回開發期的 `static/graphql.json`）——修正 L2 後首次實機
    登入的兩個失敗根因（快照未出貨＋代理 fail-closed）。

### D11 多帳號模型 A：索引 secret＋每帳號 secret（v1.1）

*   **適用版本**：v1.1。
*   **裁決**：多帳號採模型 A——單一活躍會話；索引 secret `x-accounts`
    （`{version, order[], current}`）＋每帳號 secret `x-account-<user_id>`
    （Account JSON：中繼資料與 cookie 行同鍵原子寫）；`newSessionFor(acc)`
    依帳號建構會話；`xSession` 補 `Close()`；`Validate` 改為
    `Verify(cookieLine) (ViewerInfo, error)`（切換流需取回身分，login 流
    同法複用）。Secrets 無 List API（實測）——索引 secret 是列舉帳號的
    唯一路徑，「自建帳號清單遍歷」否決。v1 單鍵 `x-cookie-line` 遷移：
    「先寫後刪」（crash-safe）、Viewer 失敗保留舊鍵、降級期雙寫舊鍵
    （v1.1 期間回退不丟會話）。
*   **依據**：需求方 2026-11-14 裁決＋Secrets 實測約束；v1.1 設計 §8.3
    （儲存佈局與遷移序列）。實作落點：`X/accounts.go`、`X/migrate.go`
    （遷移三態測試：新裝／升級／回退）。
*   **偏離登錄**：REQ-WEB-069／070 為 web 控制台域條目（409 API 行為），
    於本桌面 app 採用其**身分語義**：以身分綁定一致（UserID 定鍵，登入
    身分與憑證一致）與重複綁定＝同鍵覆蓋（同 UserID 再匯入覆蓋並提示
    「已更新」）承接入帳號模型；409 回應體本身不適用（無 web API 面）。

### D12 切換即重驗 Viewer＋有效性 TTL 快取＋墓碑（v1.1）

*   **適用版本**：v1.1。
*   **裁決**：切換時重驗 Viewer（方案甲）；重驗結果帶 TTL 快取
    （10 分鐘，`verifyTTL`）——TTL 內切換免重驗、直接以快取身分重建
    session；驗證失敗不切換、標記失效、原會話不動；舊 transport 於新
    會話驗證成功後關閉。啟動時 lazy 預熱全部帳號憑證（僅 cookie 行與
    中繼資料，不建 transport）；**切走即墓碑**——舊會話的時間線狀態
    （posts/cursor/seen/list）一律丟棄，僅保留帳號中繼資料＋憑證＋有效性
    紀錄，切回時重新載入時間線（重新 Home 首頁）。
*   **依據**：需求方 2026-11-14 兩次裁決（方案甲＋墓碑機制）；v1.1 設計
    §8.5。TTL 值 10 分鐘隨實作定案（`X/accounts.go` `verifyTTL`）；兼顧
    切換速度與低常駐 RAM（D14 防線的帳號側配套）。
*   **偏離登錄**：無（切換流屬 app 層自由度；REQ-WEB-069 身分語義經重驗
    維持，失效呈現走 credential 錯誤文案＋「重新貼上 Cookie」入口，
    不移除 key）。

### D13 GIF 走 mp4 variant（v1.1）

*   **適用版本**：v1.1。
*   **裁決**：GIF（`animated_gif`）與 video 走同一解析路徑——取
    `content_type == "video/mp4"` 中位元率最高者為播放來源；HLS（m3u8）
    不採用。
*   **依據**：需求方 2026-11-14 裁決；P0' 探測實證（timeline 回應內
    mp4 variant 全 200，見〈v1.1 P0' 探測結論〉媒體節）。實作落點：
    `X/internal/xapi/api/media.go`（`bestMP4Variant`）。
*   **偏離登錄**：REQ-DAT-016 的 mp4 位元率語義擴及 `animated_gif`
    （規格未對 GIF 另立規則，屬同語義延伸）；m3u8 variant 不入選為本檔
    裁決（規格未禁止，P0' 實證 HLS 非必要）。

### D14 媒體 RAM 預算（v1.1）

*   **適用版本**：v1.1。
*   **裁決**：解碼位元組 LRU 上限 24 MiB（`thumbLRUBytes`——量測後可調
    的常數）＋行程峰值 <80 MiB 驗收線；`ReleasePixels` 為釋放槓桿；縮圖
    取 `name=small`（medium 備用）；單一媒體檔下載上限 8 MiB
    （`thumbMaxBytes`，超限即錯誤、不整讀入）。「快取上限＝80 MiB」的
    字面執行否決（與行程峰值驗收線自相矛盾）。
*   **依據**：需求方 2026-11-14 裁決（LRU 先量測再定案——接受）＋v1 L2
    RAM 基線（閒置 11.3 MiB → 載入 26.0 MiB → 翻頁 10 頁峰值 30.0 MiB，
    見〈L2 驗收記錄〉）＋zoom-out 紀律。實作落點：`X/media.go`
    （`thumbStore` LRU、`fetchImage` 守門）。
*   **偏離登錄**：無（RAM 預算屬本專案自設 NFR；80 MiB 驗收線為 v1.1
    設計自設值，非規格條目）。
*   **驗收狀態**：三場景 L2 RAM 量測（閒置／100 帖含媒體捲動／播放中，
    行程峰值 <80 MiB）**待驗**——需真實憑證，由控制器另行執行後回寫
    數值（見〈v1.1 收尾登錄〉節）。實作者端已驗事實：v1 L2 基線峰值
    30 MiB 量級；v1.1 單元防線釘測全綠（LRU 逐出、單檔超限拒收）。

### D15 媒體下載獨立出口（v1.1）

*   **適用版本**：v1.1。
*   **裁決**：媒體（縮圖／影片）以獨立輕量 `net/http` 客戶端直載公開
    CDN，不走引擎 transport；沿用代理解析結果（D10 直連語義）；單檔與
    總量上限寫死；媒體 403/404 不落六型別錯誤語義，app 層以可重試佔位
    呈現。
*   **依據**：v1.1 設計 §1.7 權衡（design F7 主張的 transport 路徑否決
    ——需為媒體開二進位下載面且媒體 403/404 不落六型別語義，成本高於
    收益）；P0' 實抓收尾（見〈v1.1 P0' 探測結論〉媒體節）。實作落點：
    `X/media.go`（`fetchImage` seam）。
*   **偏離登錄**：REQ-TRN-010（所有 X 請求必須經單一客戶端出口）登錄為
    偏離——單一出口指 X API（GraphQL）流量；媒體 CDN
    （`pbs.twimg.com`／`video.twimg.com`）≠ GraphQL API，走獨立出口
    （免認證實證見 D22）。

### D16 HomeLatestTimeline 規格外登錄（v1.1）

*   **適用版本**：v1.1。
*   **裁決**：`HomeLatestTimeline` 在凍結規格外，以 P0' 探測＋快照為
    契約、隨 X 前端發版漂移：與 HomeTimeline 同構的雙形態（首頁 GET 無
    seen；cursor/seen 非空 → POST 帶 seenTweetIds）＋空 featureSwitches。
    P0' 實證：GET 20/20、POST（cursor+seen）20/20、零失敗，queryId
    `IVA5BUbeYPTrpYekQH7jiA`（v1 探測節記錄值至今有效）。
*   **依據**：需求方 2026-11-14 裁決（原開放問題 #5：執行 P0' 探測鏈，
    空 features → features 回填 → 仍失敗才降級登錄——首臂即全過）；
    探測結論回寫本檔〈v1.1 P0' 探測結論〉節。實作落點：
    `X/internal/xapi/api/timeline.go`（與 HomeTimeline 共用
    `homeTimelineOp`）。
*   **偏離登錄**：本操作不在 `XAPIS_BEHAVIOR_SPEC.md` 凍結範圍——登錄為
    「規格外操作，以探測＋快照為契約」；queryId 不得硬編碼為行為契約，
    須按 Task 8 快照機制重取。

### D17 格式化鏈（v1.1）

*   **適用版本**：v1.1。
*   **裁決**：Go 格式化鏈定案：`cd X` → `gofumpt -w -extra .` →
    `golangci-lint run`（v2 預設 linter：errcheck/govet/ineffassign/
    staticcheck/unused）→ `go vet ./...`＋`go test -race -count=1 ./...`
    全綠＋web 編譯冒煙，為每 commit 退出條件。oxfmt 自鏈中移除（不支援
    Go——需求方 2026-11-14 裁決；若日後有 JS/TS 資產再單獨採用）。
    執行位置約束（工具鏈實測）：gofumpt 限定 `X/` 執行；golangci-lint
    v2.15.0 必須在 `X/` 內執行（repo 根無 go.mod → exit 1）；
    `vendor/XApis/gateway/` 為獨立 Go module，不在範圍。
*   **依據**：需求方 2026-11-14 裁決＋格式化工具鏈實測結論；Task 15
    全鏈執行實錄（見〈v1.1 收尾登錄〉節）。
*   **偏離登錄**：無（工程鏈屬本專案自由度；v1 僅 `go vet`＋
    `go test -race`，v1.1 起升級為本鏈）。

### D18 世代邊界（v1.1）

*   **適用版本**：v1.1。
*   **裁決**：帳號切換／登出即 cancel 舊 context 子樹（在途 feed 工作
    收斂）並遞增世代號；非同步回寫（publish）攜世代號——世代不符的
    過期結果整批丟棄；時間線狀態 `timelineState` 按槽位×2（For You／
    Following）隔離，搜尋結果為查詢時存在、關閉即棄的 ephemeral 狀態。
*   **依據**：v1.1 設計 §1.1／§8.4（世代邊界）；實作落點 `X/session.go`
    ＋`X/view_home.go`（世代＋槽指標雙檢查；in-flight 過期即棄釘測）。
*   **偏離登錄**：無（app 層併發結構屬本專案自由度）。

### D19 帳號數上限 5（v1.1）

*   **適用版本**：v1.1。
*   **裁決**：帳號數上限 5（`maxAccounts = 5`）；新增第 6 個時呈提示態
    （禁用＋上限文案），提示清理；目標用戶為非專業用戶（3–5 帳號即
    足夠）。移除帳號＝刪 secret＋索引除名＋二次確認（Modal）。
*   **依據**：需求方 2026-11-14 裁決（原開放問題 #2）。實作落點：
    `X/accounts.go`（`maxAccounts`）＋頂列帳號選單提示態。
*   **偏離登錄**：無（產品約束屬需求方裁量）。

### D20 頂列佈局與斷點（v1.1）

*   **適用版本**：v1.1。
*   **裁決**：主內容承載於頂列：左上帳號切換（選單）、右上搜尋；不做
    下方 bar（Desktop only，行動端不考量）。水平響應式四帶：
    band 0（<768，僅 Select 收合）、band 1（768–1023，兩欄）、
    band 2（1024–1199，完整版型——側欄＋時間線＋展開檢視並列）、
    band 3（≥1200，內容欄 640 置中）；斷點 768/1024/1200（需求方提供
    階梯），`Tester.SetSize` 三寬度釘測。
*   **依據**：需求方 2026-11-14 裁決（原開放問題 #1）；v1.1 設計 §9。
    實作落點：`X/view_home.go`（`layoutBand`）＋`X/view_layout_test.go`
    （斷點逐字釘測）。
*   **偏離登錄**：無（UI 佈局屬 app 層自由度；MyGO 內建元件 Tabs/
    Sidebar/Select/Modal/Route/MenuButton 於 v1.1 兌現）。

### D21 搜尋入 v1.1（v1.1）

*   **適用版本**：v1.1。
*   **裁決**：`SearchTimeline` 入 v1.1——頂列搜尋入口＋結果頁；queryId
    併 P0' 探測萃取得 `vLHjqbDv7TPFMBkIIO1nOA`（登入態 shell chunk 萃取，
    105 操作之一）。REQ-RD-014 的「不得改用 GET」恆 POST 適用；P0' 進一步
    實證：variables **必須含 `product`**（缺 → 422
    `GRAPHQL_VALIDATION_FAILED` "must be defined: variable product"），
    形狀 `{product, rawQuery, count}`；product=Top/Latest 各 10/10 零失敗
    （空 featureSwitches 可用）。結果 `timelineState` 為 ephemeral
    （查詢時存在、關閉即棄）。
*   **依據**：需求方 2026-11-14 裁決（#1 附帶範圍擴張）；〈v1.1 P0' 探測
    結論〉SearchTimeline 節。實作落點：`X/internal/xapi/api/timeline.go`
    （`SearchTimeline` Sender 方法）＋頂列搜尋接線。
*   **偏離登錄**：REQ-RD-014 的 product 必填性為規格未記載的補充登錄
    （422 實證）——契約以探測＋快照為準、隨前端輪動（同 D16 立場）。

### D22 媒體 CDN 免認證定案（v1.1）

*   **適用版本**：v1.1。
*   **裁決**：媒體直載免認證定案——`pbs.twimg.com` 縮圖
    （`?format=<ext>&name=small/medium/large/orig`；`format` 必填且沿用
    `media_url_https` 副檔名、`name` 選填）與 `video.twimg.com` mp4
    variant（最高位元率，展開播放來源——D13）均無簽章、無過期參數、
    不需憑證；官方 API 限流體系（`x-rate-limit-*`、OAuth 權杖、
    docs.x.com 體系）不適用於媒體 CDN；403 僅限保護/已刪內容。P0' 實抓
    收尾：縮圖 10/10、影片 5/5 全 200。降級退路（解析＋計數標示＋外開）
    保留為應急。
*   **依據**：需求方 2026-11-14 裁決「搜尋並驗證」＋本輪查證（v1.1 設計
    §1.7）＋P0' 實抓（〈v1.1 P0' 探測結論〉媒體節）。
*   **偏離登錄**：無新增（媒體出口偏離已登錄於 D15；本節為免認證事實的
    定案與實證收尾）。

## 規格 REQ×版本適用性對照表（Task 15，2026-11-14）

> 機械查閱點：登錄 v1.1 新適用／翻轉／偏離的規格 REQ 條目在兩輪的適用
> 狀態與對應裁決編號（D10↔REQ-INV-02 的翻轉先前無對照點，本表補齊）。
> 「不適用」＝該輪範圍外；「偏離」＝行為與條文字面不同（細節見對應
> D 節）；其餘 REQ 的輪次歸屬沿用各輪設計文件的範圍清單，不在此重列。

| REQ 條目 | 章節域 | v1 狀態 | v1.1 狀態 | 裁決編號 |
| --- | --- | --- | --- | --- |
| `REQ-DAT-011` | 輸出欄位（媒體欄） | 不適用（媒體域不在 v1） | 適用（子集：`Media` 欄承載媒體 URL 語義） | D15 |
| `REQ-DAT-013` | `work_type` 三值 | 不適用 | 適用 | D13 |
| `REQ-DAT-015` | `extended_entities` 優先 | 不適用 | 適用（`legacy.extended_entities.media` 單錨，無 fallback） | D13／D15 |
| `REQ-DAT-016` | mp4 最高位元率 | 不適用 | 適用（擴及 GIF——D13） | D13 |
| `REQ-DAT-018` | 下載參數與檔名 | 不適用 | 部分（`?format=…&name=…` 參數語義採用；`<序號>.jpg/.mp4` 落盤檔名不適用——app 直載渲染、不落序號檔） | D22 |
| `REQ-WEB-069` | 身分與憑證綁定 | 不適用（web 控制台域） | 適用（身分語義採用；409 API 行為不適用） | D11 |
| `REQ-WEB-070` | 同帳號重複綁定 | 不適用（web 控制台域） | 適用（同 UserID 覆蓋同鍵；409 API 行為不適用） | D11 |
| `REQ-TRN-010` | X 流量單一出口 | 適用 | 適用＋偏離（媒體 CDN 獨立出口） | D15（D22 實證） |
| `REQ-INV-02` | 代理 fail-closed | 適用 | 修訂（代理可選、直連為基線、顯式配置仍採用） | D10 |

## spike 結論：tls-client 能力對照（Task 0 Step 2，2026-11-11）

> Spike 程式位於 `%TEMP%\opencode\spike_tlsclient\`（repo 外暫存，不入
> git）。環境：Go 1.27.3 windows/amd64；`go vet`、`go build` 乾淨；
> 全部檢查 PASS。方法：源碼檢查（module cache）＋單元級 wire 斷言
> （`fhttp.Header.Write` 位元組輸出）＋端到端斷言（本地 raw socket /
> 假代理 / 本地 302 伺服器）。

**評估版本（建議釘選）**：`github.com/bogdanfinn/tls-client v1.18.0`
（`go get @latest` 於 2026-11-11 解析之最新版；傳遞依賴：bogdanfinn/fhttp
v0.7.3、bogdanfinn/utls v1.8.2-barnius、bogdanfinn/quic-go-utls
v1.0.13-utls）。

### 1. chrome_150 profile 存在性——存在，無需降級

*   `profiles.Chrome_150` 存在（`ClientHelloID{Client="Chrome",
    Version="150"}`），`GetClientHelloSpec()` 可解析（16 cipher suites、
    18 extensions）；字串鍵 `"chrome_150"` 已註冊於 `profiles.
    MappedTLSClients`；`profiles.DefaultClientProfile == Chrome_150`（上游
    預設即 chrome_150）。
*   相鄰版本清單：chrome_131 / chrome_133 / chrome_144 / chrome_146 /
    chrome_150（＋PSK 變體）/ chrome_152（＋PSK）。最近的更新 profile 為
    chrome_152、最近的較舊為 chrome_146。**結論：chrome_150 精確存在，
    不需自訂 ClientHelloSpec，D3 不受影響。**

### 2. 標頭順序與原樣小寫——成立，但附帶實作約束

*   fhttp 的順序控制是**顯式的**：`Header-Order:`（`fhttp.HeaderOrderKey`）
    magic key，值為小寫標頭名有序切片。設置後，wire 輸出嚴格按該序，
    且標頭名**按存入時的原樣寫出**（`user-agent` 維持小寫、混合大小寫
    `X-Custom-Mixed` 原樣保留——寫出時無 canonicalization）。
*   **約束（登錄給 REQ-HDR-002/013 實作）**：**不設** `Header-Order:` 時，
    輸出順序是**逐位元組字典序**，不是插入序——fhttp 不追蹤插入序。
    引擎的 headers 套件必須**每次請求都附上** `Header-Order:`。
*   端到端（tls-client + chrome_150，HTTP/1.1 對本地 raw socket）同樣
    驗證通過：自訂順序與原樣大小寫在真實傳輸路徑的 wire 上成立。
*   HTTP/2 備註：h2 協議本身即強制標頭名小寫；偽標頭順序由
    `PHeader-Order:`（`PHeaderOrderKey`）控制，chrome_150 profile 自帶
    `GetPseudoHeaderOrder()`。

### 3. 重定向控制——存在且行為符合

*   `WithNotFollowRedirects()` 存在（client_options.go）。原始碼：設置後
    redirectFunc 回傳 `http.ErrUseLastResponse`，即 http.Client **回傳重定向
    回應本身**。
*   實測：預設客戶端對 302 跟隨至 200（target 被擊中一次）；帶
    `WithNotFollowRedirects()` 的客戶端 `Do()` 直接回傳 302 回應本身
    （status=302、Location 標頭在、target 零擊中）。**結論：D7 的 v1
    語義（一律不跟隨）可直接實作。**

### 4. per-client proxy——成立（實例級綁定）

*   `WithProxyUrl(string)` 為 per-client 建構選項，內部為每個實例建立獨立
    connectDialer（以 CONNECT 隧道承接）。
*   實測：帶代理客戶端經其代理隧道送出（假代理觀察到
    `CONNECT 127.0.0.1:<port> HTTP/1.1` ＋隧道內 GET，隧道內應答送達客戶端）；
    無代理客戶端直達 origin，代理零流量；`GetProxy()` 回傳值證實配置為
    實例級。**結論：session／anonymous 雙客戶端（共享指紋與代理、cookie jar
    完全隔離）的 per-instance 代理語義成立。**

### 附帶證據（best-effort，非阻塞）

*   以 chrome_150 profile 對 `https://tls.peet.ws/api/all` 實際握手成功
    （HTTP/2）：`ja4=t13d1516h2_8daaf6152771_806a8c22fdea`、
    `ja3_hash=f984bd5bc7358922cde86ed4471a2e89`、User-Agent 回顯正確——
    chrome_150 ClientHello 確實驅動真實 wire。
*   該回應目前形態缺 `akamai_fingerprint_hash` 欄位（對本專案非阻塞；
    Akamai 指紋對照留待 Step 3 探測實驗一併觀察）。

## 探測實驗結論（Task 0 Step 3–4，2026-11-12）

> 暫存程式與原始結果檔在 `%TEMP%\opencode\probe_xctid\`（repo 外，不入 git）。
> 方法：`github.com/bogdanfinn/tls-client v1.18.0`＋`chrome_150` profile（即 D3 釘選
> 版本），唯讀 GraphQL 操作（Viewer／HomeTimeline），全程**無任何寫入操作**；
> 每請求間隔 300–600ms 隨機。標頭為 REQ-TRN-012..015＋REQ-HDR-003 的 v1 子集
> （Chrome/153 逐字指紋，REQ-TRN-003/004），**刻意不帶 `x-client-transaction-id`**
> （探測變因）。憑證於執行期讀取，所有輸出已脫敏（cookie 值不落任何檔案或日誌；
> ct0 僅以長度摘要呈現）。

### 環境與代理偵測

*   代理偵測：註冊表 `ProxyEnable=0`（`ProxyServer` 有殘值 `http://127.0.0.1:9099`
    但該埠未監聽）、`HTTPS_PROXY`/`ALL_PROXY` 未設、無 PAC（`AutoConfigURL` 空）
    → **系統代理不存在**。依探測規則改跑全直連組（見〈直連對照（OQ12）〉）。
*   直連出口：Cloudflare `cdn-cgi/trace`（同一傳輸路徑）→ `colo=KHH, loc=TW,
    warp=off`——真直連（非透明隧道），且 cookie 有效（`x.com/home` 200，未導向
    登入流）。

### 矩陣執行結果

| 組 | 操作（形態） | n | 狀態碼分佈 | 判準成功率* | 226 | 344 | latency p50/p95 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| A | `Viewer`（GET） | 20 | 200×20 | 100%（20/20） | 0 | 0 | 206/288ms |
| B | `HomeTimeline`（GET，`requestContext=launch`） | 20 | 200×20 | 100%（20/20） | 0 | 0 | 576/1083ms |
| C | `HomeTimeline`（POST，`seenTweetIds` 置 variables 末） | 20 | 200×20 | 100%（20/20） | 0 | 0 | 567/679ms |

\* 判準＝HTTP 非 403 且非「有 errors 無 data」且非傳輸層失敗（簡報 OQ-XCTID
判準；傳輸錯誤視為「未接受」——本次 0 件）。

*   全程 HTTP/2；`data` 全數存在、`errors` 零出現、傳輸層錯誤 0 件。
*   限流桶：`x-rate-limit-remaining` 顯示 `Viewer` 自成 ~90/15min 桶（88→69）；
    `HomeTimeline` 的 GET 與 POST **各有** 500/15min 桶（B、C 計數器各自
    499→480 獨立遞減，未互相擠佔）。
*   ct0 輪替：60 回應中 0 次 `Set-Cookie` 輪替 `ct0`（單會話內穩定，未觸發
    同步路徑）。

### XCTID 存廢

*   **判決：OQ-XCTID →「XCTID 可延後」（探測通過）。**
*   依據：組 A/B/C 合計 60/60 在**無 `x-client-transaction-id`** 下被正常接受
    （非 403、無 errors、有 data），佔比 100% ≥ 判準 90%；226/344 出現 0 次。
*   範圍限制：實證僅涵蓋本次兩個唯讀查詢操作與單一會話/單一出口 IP；
    寫入操作（mutation）與長時高頻行為的風險評分不在本次探測範圍。v1 引擎
    只做唯讀查詢（規格 §1 範圍），故按 D2 路徑 **xctid 套件整個不建**，
    v1 不實作 XCTID（REQ-XCT-* 對 v1 登記為不適用，完整簽章能力留 v2）。
    執行期若出現 226/344（軟降級指標），依規格退避路徑處理，不改變本判決。

### HomeTimeline 兩形態可用性

*   **判決：兩形態皆可用，D1 成立。** GET（`requestContext=launch`）與 POST
    （`seenTweetIds` 置 variables 末、body 鍵序 variables→queryId）在完全相同的
    v1 子集標頭下均 200＋data（各 20/20），無形態被服務端拒絕。
*   兩形態各有 500/15min 限流桶（見上）——限流預算按「操作×形態」計，
    D1 的雙形態不產生額外配額成本。

### 直連對照（OQ12）

*   **判決：直連「可達且被正常接受」——非「不可達」亦非「被拒」；代理對照組
    因環境偵測不到代理而不成立（偵測結果見〈環境與代理偵測〉）。**
*   依據：60/60 請求直連 x.com 全數 200＋data（出口 `loc=TW`，warp=off）。
    OQ12 原題「直連 vs 代理行為差異」在本環境無法對照；依探測規則（偵測不到
    代理→只跑直連組並註明），直連組由計畫的 30 提升至 60（每臂 ≥20）以維持
    OQ-XCTID 判準統計力。
*   對 D6 的影響：失敗關閉與代理解析鏈裁決不變；本次實證僅說明「直連回退在
    可直連的網路環境下可被 X 正常接受」，不構成把直連升為預設路徑的依據
    （帳號風險面仍屬 app 層決策）。

### queryId 附註（供 Task 8 快照與 Task 14；公開介面事實）

*   萃取方法：`x.com/home` HTML → `main.<hash>.js`（本次
    `main.d5b28133f54b0c74a.js`）以模式
    `queryId:"…",operationName:"…",operationType:"query"` grep（105 組）得
    `Viewer`；`HomeTimeline` 不在 main，改以 HTML 內 webpack runtime 的
    chunk map（`t.u=e=>` 之 id→name 與 id→hash 表；chunk 檔名形如
    `<name>.<hash>a.js`——副檔名 `.js` 前多一個 `a`）重建 lazy chunk URL，
    抓 HomeTimeline 系 chunk（
    `shared~bundle.Compose~bundle.HomeTimeline~bundle.LoggedInMain`）補得。
    公開 web bearer（規格 §3.1-2 介面事實）同自 main 裸常數取得，值不落本檔
    （執行期同法可重取）。
*   所得值（2026-11-12，`main.d5b28133f54b0c74a.js`）：
    *   `Viewer` → `TpQxQtWMTMnIAMPlMGMqNw`
    *   `HomeTimeline` → `KmJCeZ__ef1fNos7tKN_fg`
    *   （附帶）`HomeLatestTimeline` → `IVA5BUbeYPTrpYekQH7jiA`
*   queryId 隨前端發版輪動：引擎不得將其硬編碼為行為契約，須按 Task 8 快照
    機制重取（本節值僅為當次事實記錄，可入快照檔）。

### 偏離註記

*   **單一會話 vs 計畫的 2 會話**：本次僅有需求方提供的一組 cookie（單一登入
    會話），未做第二會話交叉驗證；判決因而帶「單會話」但書，建議 L2 煙霧
    （Task 14）以第二會話複驗 226/344 指標。
*   **代理組缺席**：計畫矩陣為 A/B/C 走代理＋D 直連對照；因環境偵測不到代理，
    全部 60 請求改走直連（每臂 20）。OQ-XCTID 判準的套用對象由「代理下
    A/B/C」如實改為「直連 A/B/C」。
*   探測程式與原始結果（`results-20261112-001048.jsonl`）留存於
    `%TEMP%\opencode\probe_xctid\`，不入 git。

## L2 驗收記錄（Task 14，2026-11-12）

> 煙霧程式與脫敏結果檔（`results-20261112-222449.json`）在
> `%TEMP%\opencode\l2_smoke\`（repo 外，不入 git）。方法：真實引擎套件
> （auth／transport／graphql/api）＋由探測節 queryIds 構造的
> `X/static/graphql.json` 快照（`featureSwitches` 空——60/60 無 features
> 被接受的探測實證）；唯讀操作（Viewer／HomeTimeline），全程無任何寫入
> 操作；請求間隔 300–600ms 隨機；輸出全程脫敏（cookie 值不落任何檔案
> 或日誌；bearer／ct0／cursor 僅以長度摘要呈現；帖文僅記長度摘要）。

### 結果

*   **快照載入驗證 ✓**：`LoadRegistry` 載入 `X/static/graphql.json` 成功，
    `Viewer`／`HomeTimeline` 皆查得（queryId 與〈探測實驗結論〉一致，
    method 由 query 型推導為 GET）。
*   **Viewer 身分驗證 ✓**：3/3 成功（200＋data），回身分 screen_name
    非空、UserID 非空（長 19）。本組 cookie 行僅 `auth_token`＋`ct0`
    兩鍵、無 `twid`，「與 twid 一致」項無從對比——依 brief 判準以
    「至少非空」通過。
*   **HomeTimeline 首頁**：GET（`requestContext=launch`）200＋data，
    去重後 28 帖（X 實際回傳多於請求 `count=20`，屬會話串展開差異）、
    28 位作者、Bottom 游標在（長 46）。
*   **翻頁**：第 2 頁起帶累積 `seenTweetIds`（→ POST，D1 雙形態），
    翻滿 10 頁上限停止（游標全程未耗盡）；每頁 28–32 帖、全數新帖
    （跨頁零重複），累計 298 帖。GET 與 POST 兩形態於真實引擎路徑下
    均被接受——D1 的 L2 複證。
*   **延遲概況**：Viewer p50/p95 = 203/220ms（n=3）；HomeTimeline
    p50/p95 = 586/1011ms（n=10）——與探測節量級一致。
*   **226/344 計數（軟降級早期預警）**：全程 15 請求（含 bearer 派生的
    HTML/JS 取得）皆 200：226＝0、344＝0；`Set-Cookie` 輪替 `ct0` 0 次。
*   **RAM 基線（NFR，GetProcessMemoryInfo）**：閒置 11.3 MiB（peak
    11.3）；載入後 26.0 MiB（peak 26.0）；翻頁 10 頁後 27.0 MiB
    （peak 30.0）——全程峰值 30 MiB 量級。

### 偏離記錄

*   **直連例外（OQ12）**：本環境無系統代理（見〈環境與代理偵測〉），且
    使用者明確指示本實驗走直連。transport 新增 `Options.AllowDirect`
    （預設 false、fail-closed 語義不變；commit cd367ff）——**僅煙霧程式
    設置 true**；正式 app 路徑仍 fail-closed（REQ-INV-02），D6 裁決不變。
*   **第二會話複驗缺第二會話**：僅一組 cookie（單一登入會話；探測節同
    源），以煙霧全程 226/344 統計作同會話第二時段的複驗（0/0，無早期
    預警訊號）。真第二會話交叉驗證仍待需求方提供第二組憑證。
*   **Task 11 取消與 Task 5 session-only 的最終確認**：L2 全程無
    `x-client-transaction-id`、無匿名形態，引擎唯讀查詢路徑完整可用——
    〈XCTID 存廢〉「可延後」判決與 session-only 裁決於真實引擎路徑複證
    成立；xctid 套件維持不建（D2），若日後出現 226/344 再依退避路徑
    處理並回補 Task 11。

### L2 發現（登記待辦，本任務不修復）

*   **時間線作者欄位形態輪替（前端回應形態變更）**：L2 實測中，貼文
    作者物件已無 `result.legacy`（Task 12 fixtures 所據形態），
    `screen_name`／`name` 移至 `result.core`（與 Viewer 回應的使用者
    結果同形態）。現行 `postUser` 萃取：`rest_id`（WorkID）與帖文
    Title 正常，但 `Post.ScreenName`／`User.ScreenName`／`User.Name`
    為空，`WorkURL` 退化為 `https://x.com//status/<id>`（REQ-DAT-014
    語義受損）。建議修法（待控制器裁決派發）：`postUser` 於
    `legacy` 缺失時回退 `result.core.screen_name`／`result.core.name`
    （Viewer 萃取路徑已實證該位置）。本任務依紀律不動 Task 12 程式碼，
    僅如實登記。
*   **回註（2026-11-14，Task 15）**：上項發現已於 v1 輪修復——`postUser`
    於 `legacy` 缺失時回退 `result.core.screen_name`／`result.core.name`
    （與 Viewer 萃取路徑同形），修復後複驗 30/30 通過；本登記待辦結案
    （已由 v1.1 輪受理並核銷）。

### D10 代理改為可選：正常用途直連（需求方 2026-11-13 裁決）

> **歸屬正規化（2026-11-14，Task 15）**：本節原嵌於〈L2 驗收記錄〉之下
> （D1–D9 掛〈已定案裁決〉、D10 嵌 L2 節的序列斷裂）；現已移至
> 〈已定案裁決〉節 D9 之後，內容逐字保留、僅增「適用版本」標註。

## v1.1 P0' 探測結論（Task 2，2026-11-14）

執行方式：`X/cmd/probe11`（Task 1 工具＋Task 2 增 `search` 子命令）以
需求方提供的單一 cookie 會話實測；判準沿用 D1（每臂 ≥20、HTTP 200 且回
應 data 含貼文）。程式內錨點見 `X/internal/xapi/api/probe_findings.go`。

### HomeLatestTimeline（規格外操作，D16 依據）

* **結果**：GET 20/20、POST（cursor+seen）20/20、**零失敗**——空
  featureSwitches 可用，queryId `IVA5BUbeYPTrpYekQH7jiA`（v1 探測節記錄
  值）至今有效。
* **裁決（D16 定稿）**：與 HomeTimeline 同構的雙形態（首頁 GET 無 seen；
  cursor/seen 非空 → POST 帶 seenTweetIds）＋空 features。登錄「本操作
  在凍結規格外，以探測＋快照為契約、隨 X 前端發版漂移」。

### SearchTimeline（REQ-RD-014：恆 POST）

* **queryId**：`vLHjqbDv7TPFMBkIIO1nOA`（登入態 shell
  `main.73ebd7d6b601a4f8a.js` chunk map 萃取，105 操作之一）。
* **結果**：product=Top 10/10、product=Latest 10/10、**零失敗**——空
  featureSwitches 可用。
* **關鍵發現**：variables **必須含 `product`**（缺 → 422
  `GRAPHQL_VALIDATION_FAILED` "must be defined: variable product"）；
  形狀 `{product, rawQuery, count}`（恆 POST、body 鍵序沿 REQ-GQL-003）。
* **前端輪動註記**：登出態 shell 已換 Vite 架構（`x-web/x-web/entry-*`），
  登入態 shell 仍為 `responsive-web/client-web/main.*.js`——chunk 萃取
  應以登入態 shell 為入口。

### 媒體 CDN 直載（D22 依據）

* **縮圖**：`?format=<ext>&name=small` 10/10（HTTP 200、Content-Type
  `image/*`）——`format` 必須沿用 `media_url_https` 副檔名。
* **影片**：timeline 回應內 video.twimg.com URL 5/5 全 200（mp4
  `video/mp4` 3、HLS `application/x-mpegURL` 2）——免認證、免簽章。
* **裁決（D22 定稿）**：媒體直載以獨立輕量 http 客戶端可行；mp4 variant
  （最高位元率）為展開播放來源（D13）；m3u8 不採用。降級退路（解析＋
  計數＋外開）保留為應急。

### 修訂登錄（本節同時處置）

* **「web 已實證」虛假引用撤回**：v1 對 web 建置從未留驗收記錄；實況為
  「`go tool mygo build -platform web` 編譯冒煙通過（2026-11-13 起，
  每 commit 退出條件）＋**資料面未驗證**（瀏覽器 CORS＋raw TLS 限制，
  資料面 relay 屬 v2）」。
* probe11 的 `search` 子命令與變數修正（CompactJSON 前序列化、product
  變數）隨本節 commit——工具屬 T2 執行儀器。

## v1.1 收尾登錄（Task 15，2026-11-14）

*   **格式化鏈執行實錄（D17）**：`cd X` → `gofumpt -w -extra .`
    （gofumpt v0.10.0；基線 18 檔全數格式化，diff 逐行複核為純格式變更
    ——CJK 全形括號註解縮排、同型參數合併（`opName string, cursor
    string` → `opName, cursor string`，編譯等價）、複合值與引數串換行、
    註解對齊，無語義變更）→ golangci-lint v2.15.0（v2 預設 linter；首輪
    18 指摘＋複驗 6 指摘全數修復——errcheck 之未檢查 `Close` 回傳值以
    `_ =` 顯式捨棄（沿用既有 idiom）、staticcheck QF1001 以 De Morgan
    等價改寫（`!(a<b && b<c)` → `a>=b || b>=c`，邏輯逐一核對等價，斷言
    行為不變）→ `golangci-lint run` 0 issues、`go vet ./...` 乾淨、
    `go test -race -count=1 ./...` 10 套件全綠。oxfmt 不在鏈中（需求方
    2026-11-14 裁決移除）；`vendor/XApis/gateway/` 為獨立 Go module，
    未觸碰。
*   **雙平台建置（D10 配套複證）**：`CGO_ENABLED=0 go tool mygo build
    -platform windows/amd64 -skip-installer` 通過，`graphql.json` 確認
    出貨於 exe 旁（mygo.json `resources` 攤平，D10「配套」項的機制於
    v1.1 末端複證）；`go tool mygo build -platform web` 冒煙通過（web
    為測試載體，資料面 relay 屬 v2——沿 Task 2 修訂登錄）。
*   **L2 RAM 驗收（D14）——待驗**：三場景量測（閒置／100 帖含媒體捲動／
    播放中，行程峰值 <80 MiB）需真實憑證，由控制器另行執行；數值取得後
    回寫本節與 D14「驗收狀態」。實作者端已驗事實：v1 L2 基線峰值 30 MiB
    量級（見〈L2 驗收記錄〉RAM 基線）；v1.1 單元防線釘測全綠（解碼位元
    組 LRU 逐出、單檔 8 MiB 超限拒收、世代邊界過期丟棄）。
