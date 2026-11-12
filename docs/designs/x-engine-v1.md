# MyGO X 引擎 v1（RIGG/X）

*   **Author:** 需求方（RIGG / X）
*   **Status:** Proposed
*   **Date:** 2026-11-11

--------------------------------------------------------------------------------

## Overview

XApis 是一套以 Python 實作的 X（Twitter）Web 客戶端，其完整可觀察行為已由
`XAPIS_BEHAVIOR_SPEC.md`（下稱「規格」，1922 行，唯一行為參考）凍結為 clean-room
行為契約。本設計把這份契約以 **Go 1.27.3** 重寫為一個原生引擎，嵌在名為
**MyGO**（https://github.com/egoist/mygo）的桌面應用之中。開發此客戶端的首要
目的是**拯救 RAM 消耗**：以純 Go（零 cgo）單一執行檔取代高消耗的 SPA／WebView
或 Python 常駐棧，因此 app 採 MyGO **原生 UI 模板**（`package ui`，無 WebView、
無 HTML/JS），UI 與引擎同行程直呼，沒有序列化邊界。本專案是 **RIGG monorepo**
（repo 根：`C:\Users\User\Documents\GitHub\RIGG`）中的 **X 重寫**：X app 位於
monorepo 的 `X/` 子目錄，module 為 `github.com/OG-Open-Source/RIGG/X`（版本
標籤格式 `X/v0.1.0`，即 Go 子目錄 module 的標籤慣例），以
`mygo init -template native -module github.com/OG-Open-Source/RIGG/X X`
初始化（目前 `X/` 尚未建立，init 為實作第一步）；宿主框架釘選 MyGO v0.9.0。
引擎住在 `X/internal/xapi` 套件樹，該套件樹
**不得** import MyGO 的任何套件，以維持逐位元組對拍測試能力與日後機械式抽離的
可能。

v1 的價值主張只有一條線：**使用者在桌面環境貼上自己的 X Cookie，立刻看到自己的
首頁時間線，並能持續翻頁**。寫入、媒體、文章、私信、帳密登入、Web 控制台全部
不在 v1。工程上的關鍵約束有三條：X 以 TLS 指紋與標頭特徵風控非瀏覽器流量，故
傳輸層必須模擬 Chrome；規格以 `REQ-*` 編號逐條凍結行為，實作以需求編號為工作
單元；上游 XApis 授權狀態未確認，確認前倉庫維持私有、不散布。

## The Problem

現有途徑有三個不可接受的代價：

1.  **RAM 消耗**。Python SDK ＋ FastAPI 控制台 ＋ Node 回放 ＋ MySQL 的組合是為
    多使用者 Web 工作台設計的；單機桌面使用者要的是常駐記憶體以 MB 計、數 MB
    大小的單一執行檔，不是一個拖著直譯器、瀏覽器核心與資料庫常駐數百 MB 的棧。
2.  **風控牆**。X 服務端以傳輸層 TLS 指紋、標頭順序與內容特徵辨識自動化流量
    （命中回 GraphQL code 226「疑似自動化」或限流碼）。普通 HTTP 客戶端連第一個
    請求都過不了，必須模擬 Chrome 的 TLS ClientHello 與應用層指紋。
3.  **行為漂移**。X 前端發版會使 XCTID 素材與 GraphQL 操作註冊表漂移；沒有
    內建重建路徑與明確錯誤分類的客戶端，會把「素材過期」誤判成「憑證失效」，
    誘導使用者做錯誤的修復動作。

提出的解法：以 Go 1.27.3 依規格重建客戶端行為，傳輸層採行程內
`bogdanfinn/tls-client`（基於 uTLS 的 Chrome TLS 指紋模擬，純 Go、零 cgo，
符合 MyGO 單一執行檔屬性）；登入只做 Cookie 匯入，以 Viewer 請求驗證會話；
錯誤以型別化分類跨邊界傳遞，讓 app 層能機械區分「素材不可用／憑證失效／限流
退避／傳輸故障」。XCTID 是否為 v1 必要，由 P0 一日探測實驗以實證裁決，不預設。

## Goals

*   **G1 可建置**：單一 Go module（`github.com/OG-Open-Source/RIGG/X`，位於
    repo `X/` 子目錄，版本標籤 `X/v0.1.0` 格式）；`go build ./...` 與
    `go test -race ./...` 全綠；CI 靜態檢查「`X/internal/xapi` 不得 import
    MyGO」。
*   **G2 Cookie 登入**：貼上 `auth_token`+`ct0` 完整 cookie 字串後，引擎解析
    （REQ-AUTH-003/004）、以 Viewer 驗證身分（REQ-LOG-041/043 語義）、成功後
    會話可用；失敗時錯誤分類機械可分，素材失敗與傳輸逾時**不得**清除憑證。
*   **G3 首頁時間線**：HomeTimeline 讀取與游標翻頁（REQ-RD-001..004、
    REQ-RD-012/013），輸出去促銷條目並以 `work_id` 去重（REQ-WEB-104 語義，
    app 層）。
*   **G4 傳輸與指紋合規**：REQ-TRN-001..005、010..015、018..020 全數有對應
    實作或已登錄偏離（TRN-011 的 curl_cffi 指名改以 tls-client chrome_150
    profile 承接；TRN-040 以「v1 一律不跟隨重定向」嚴格超集滿足）。
*   **G5 XCTID 依證據裁決**：P0 探測實驗通過 → 僅保留標頭可選槽位、整條延後
    v2；不通過 → 實作純算 REQ-XCT-001..011＋隨倉庫素材，素材網路抓取
    （REQ-XCT-013）仍延後。
*   **G6 驗收以實證為準**：L2（§14.1：真實 Cookie＋代理下 Viewer 回傳身分、
    HomeTimeline 回傳貼文）＋結構性質測試；L1 逐位元組向量為後補的阻塞交付物。
*   **G7 最小開源交付**：MIT `/LICENSE`、專案 README（含 CoC 連結與
    Attribution）、`.gitignore`、`go.sum`、`docs/DECISIONS.md`；`.github`
    submodule 與 CHANGELOG 隨發布閘處理。

## Non-Goals

*   帳密登入（jetfuel 流程 7.7.1、行動端路徑 7.7.2）、Castle token（7.6）、
    Node 回放——依賴瀏覽器擷取的 Castle profile，屬 v2。
*   寫入（7.9）、媒體上傳（7.10）、文章（7.11）、私信（7.12）、資料處理匯出
    （7.13）、CLI 產品化（7.14/7.15）。
*   Web 控制台（規格第 8 章）、規格的前端工作台（第 9 章）、Go 閘道程序與部署
    拓撲（第 10 章；其存在理由是隔離 Python 的 TLS 限制，全 Go 後消失）。
*   AI 與資訊來源契約（6.3/6.4）、MySQL（6.5）、養護與配額狀態機（8.6/8.11）。
*   guest token（REQ-AUTH-009/010，XAuth 保留方法位）、本地 ct0 產生
    （REQ-AUTH-005）、twid 解析（REQ-AUTH-008）、連結與使用者名稱解析器
    （REQ-RD-010/011）、推文計重（REQ-DAT-001..005）。
*   註冊表重建鏈（REQ-GQL-030..038）與 XCTID 素材網路抓取（REQ-XCT-013）——
    前者依探測結果決定「隨倉庫快照」或「v2 手動工具」，後者在兩條路徑下皆延後。
*   多帳號管理、任何伺服器端部署形態。
*   時間線內影片播放——MyGO 0.9.0 已提供 `ui/video` 原生播放（各系統引擎：
    AVFoundation／Media Foundation／GStreamer），屬 v2 展示層議題，v1 時間線
    不承載播放。

## Open Questions

> 以下未決事項跨起草階段保留；裁決後移除。來源：`concepts_brief.md` §7
> （開放問題 2–10）與兩個待執行 spike。每項附建議預設值。

*   **OQ1（spike）tls-client 能力**：`chrome_150` profile 在將釘選版本的
    存在性、bogdanfinn/fhttp 保留標頭插入序與原樣小寫（REQ-HDR-002/013）、
    per-client proxy 語義。半天 spike 出能力對照表。未驗證前，P1 不可開工。
*   **OQ2（spike，已裁決執行）XCTID 探測實驗**：完整標頭、刻意不帶 XCTID，
    Viewer 與 HomeTimeline 各形態各 ≥20 次、跨 2 會話；通過 → G5 走延後路徑。
    需求方提供真實 Cookie 與代理。
*   **OQ3 HomeTimeline 形態**：方法表為準（GET/POST 雙形態，REQ-GQL-005 解讀
    為「帶 seenTweetIds 的 query 型必須 POST」）vs POST 單形態。影響線路形態、
    參數順序與 XCTID 的 METHOD 輸入（REQ-XCT-007）。影響 Technical Plan，
    起草時裁決。
*   **OQ4 素材策略**（探測不通過時才生效）：隨倉庫附初始
    `static/transaction_l1.json`（§13.3 L1698「隨倉庫提供」）＋解析順序
    （呼叫端 option → 使用者資料目錄快取 → 隨倉庫初始檔）＋失敗降級。
*   **OQ5 目錄樹與套件粒度**：七層（fingerprint/transport/auth/headers/graphql/
    xctid/api）vs 收斂（單套件＋至多兩個內部子套件）。影響 Technical Plan 與
    Implementation Plan，起草時裁決。
*   **OQ6 代理解析層**：建議 `X_PROXY` → 標準 env → MyGO netproxy → 失敗關閉；
    移除 Windows 註冊表層與本機埠探測層（後者以安全理由登錄）。前置：驗證
    netproxy 已涵蓋系統代理職能。
*   **OQ7 Cookie 儲存選型**：MyGO secrets API（若有）> SQLite 明文＋OS 使用者
    隔離 > OS 憑證庫（DPAPI）；REQ-INV-01 的落點在輸出脫敏而非靜態加密。
    已查證（MyGO 0.9.0，2026-11-11）：SQLite 插件 API 不變、改以 native
    module 載入（`MYGO_NATIVE_SQLITECORE`），查詢與寫入更快；native module
    機制上游仍標**實驗性**，列為依賴風險。secrets API 能力仍待
    `go doc -all github.com/egoist/mygo` 查證。
*   **OQ8 L1 基線向量**：需求方提供瀏覽器擷取向量（欄位含 METHOD/PATH、兩形態）
    作為 XCTID 簽收阻塞交付物；v1 先以 L2＋結構性質簽收。
*   **OQ9 上游授權確認**：§19.1-3 散布閘；確認前 repo 私有；只擋散布不擋本地
    提交。
*   **OQ10 error→UI 文案層**：app 端各錯誤 `kind` 的中文文案定稿歸屬（引擎只
    保證 `kind`/`code` 穩定）。
*   **OQ11 素材漂移維運指派**：維護者 CI 匿名 bootstrap diff＋app 一鍵重建＋
    關於頁 `generatedAt` 展示的負責人。
*   **OQ12 X 對非官方客戶端直連的限制與懲罰**（需求方 2026-11-11 提出）：
    未知。驗證方法：併入 P0 探測實驗，加直連對照組（同指紋標頭、無代理 vs
    有代理，觀察 403/226/344/連線行為差異）；結論決定 Alternative F 的最終
    立場與 OQ6 代理解析鏈是否保留「必須代理」。驗證前維持失敗關閉。

## Critical User Journeys (CUJs)

### CUJ 1: Cookie 匯入登入

*   **Given**: 使用者已在瀏覽器登入 X 並複製完整 cookie 字串；app 已配置可用
    代理（或系統代理偵測成功）。
*   **When**: 在 login page 貼上 cookie 字串並提交。
*   **Then**: 引擎解析 cookie（REQ-AUTH-003/004：缺 `ct0` 且有 `auth_token`
    時回精確中文錯誤）；Viewer 請求回傳身分；畫面顯示 `@handle`；
    Set-Cookie 以 write-through 落盤（MyGO SQLite 插件）；導向 home page。
    失敗時顯示對應錯誤類別的訊息與重試入口，且不誤清憑證。

### CUJ 2: 首頁時間線瀏覽與翻頁

*   **Given**: 已有有效會話（CUJ 1 完成）。
*   **When**: 開啟 home page，向下捲動觸發載入下一頁。
*   **Then**: 首批與後續貼文皆經同一發送路徑（REQ-RD-001）；輸出排除促銷條目
    並以 `work_id` 去重；游標以深度遍歷萃取（REQ-RD-012），無游標或游標與前
    一頁相同時停止載入並提示已到底。

### CUJ 3: 錯誤與復原

*   **Given**: 代理未配置、素材不可用（探測不通過路徑下）、或會話已失效——
    任一成立。
*   **When**: app 發起任一 X 請求。
*   **Then**: 錯誤以 `kind`+`code` 跨 IPC 傳遞；UI 呈現與類別對應的狀態與
    重試入口（代理設定／重新抓取素材／重新匯入 Cookie）；素材失敗與逾時
    **不得**清除已存 Cookie（REQ-LOG-042/043）；僅 GraphQL code 32/89/215/326
    或 HTTP 401/403 將會話標記為失效。

## Technical Plan (Architecture)

### 3.1 總體拓撲

```text
┌────────────── MyGO 桌面 app（module github.com/OG-Open-Source/RIGG/X，X/）────┐
│  login page          home page          啟動狀態機        代理解析             │
│  （貼 Cookie）      （時間線/翻頁）  （代理→素材→憑證） （X_PROXY→env→     │
│        │                  │                  │            netproxy→關閉）   │
│        └── 原生 UI view 直接呼叫（同行程；錯誤以 kind/code 呈現，不含憑證）──┘ │
│                               │                                                │
│  CookieStore（MyGO SQLite 插件實作，v0.9.0 起走 native module）── write-through ─┐│
└────────────────────────────────────────────────────────┼──────────────────────┘
                                                         │ 窄介面（Load/Save）
┌──────────────────────────── X 引擎（internal/xapi，不得 import MyGO）──────────┐
│  api ──► graphql ──► headers ──► transport ──► bogdanfinn/tls-client          │
│   │        │          │             │                  （chrome_150、uTLS）    │
│  auth ────┴──────────┴─────────────┘   fingerprint（九欄位常數）               │
│   │                                        │                                  │
│  xerrors（kind/code wire schema）      xctid（依探測結果存在或略去）           │
└──────────────────────┬─────────────────────────────────────────────────────────┘
                       │ HTTPS（固定標頭順序、代理必須、不跟隨重定向）
                       ▼
          系統代理／使用者代理 ──► x.com / api.x.com（GraphQL、Viewer）
```

單一出口（REQ-TRN-010）：所有 X 流量經 `transport` 的同一客戶端建構路徑；
MyGO Fetch 插件不得承載 X 請求。應用層（`X/` 模組根）對引擎只有兩個接觸面：
引擎公開 API（見 3.3）與 `CookieStore` 窄介面。引擎與 UI 同行程，無 IPC；
錯誤以 Go 值直傳，`kind`/`code` 欄位供 view 層映射。

### 3.2 套件組成與職責（OQ5＝七層）

| 套件 | 職責 | 對應規格 |
| --- | --- | --- |
| `internal/xapi/fingerprint` | 九欄位指紋檔：UA、Client Hints、語言、時區、四選一螢幕抽樣；程序內單一真源、啟動後不變、不持久化；常數集中單檔並標註「僅能跟隨上游規格版本」 | REQ-TRN-001..006 |
| `internal/xapi/transport` | tls-client 建構（chrome_150 profile）；預設標頭補齊與 sec-fetch-site 推導；`session`／`anonymous` 兩種建構形態（共享指紋與代理、cookie jar 完全隔離；若素材鏈被探測砍除，僅保留 `session` 形態）；重定向一律不跟隨；`OnRequest`/`OnResponse` 觀測掛鉤（脫敏 method/path/status/耗時/`csrf_matches_cookie`/Set-Cookie 名稱）；逾時預設 30 秒；fail-closed 代理 | REQ-TRN-010..015、018..020、REQ-TRN-040（超集滿足）、REQ-INV-02/13 |
| `internal/xapi/auth` | XAuth：不可變 snapshot＋原子替換（版本化計數）、`PrepareAuth`/`UpdateCookies`/`IsLoggedIn`/`CookieHeader`/`CSRFToken`；預設公開 web bearer 常數；guest token 方法位（回 unimplemented）；`CookieStore` 窄介面宣告 | REQ-AUTH-001..004、006、007、014、REQ-AUTH-002 |
| `internal/xapi/headers` | 六族名稱表以資料存在（v2 補族＝填表）；v1 實作 GraphQL GET／POST 兩族輸出；有序可選槽位模型（XCTID、`x-att`、`x-guest-token` 缺席不重塑 API）；標頭名小寫、未登記者依字母序追加 | REQ-HDR-001..013 |
| `internal/xapi/graphql` | 緊湊 JSON 序列化器（分隔符 `,`/`:`、`SetEscapeHTML(false)`、有序結構）同支撐 GET query string 與 POST body 鍵序；註冊表載入與快取（`static/graphql.json` 快照或重建產物）；每操作 metadata（queryId、operationType、features/fieldToggles 解析、送出方法） | REQ-GQL-001..007、010..016、020..025、REQ-INV-07 |
| `internal/xapi/xctid` | 純算產生器（SHA-256、Base64、cubic-bezier、round-half-even 自實作）；`WithClock`/`WithRand` 注入；素材載入與結構檢查；`t<0` 拋輸入錯誤。**探測通過則本套件整個不建立**，headers 的 XCTID 槽位保留為可選空位 | REQ-XCT-001..012 |
| `internal/xapi/api` | 統一發送函式；`Viewer`；`HomeTimeline`（雙形態，見 3.4）；游標深度遍歷（有序解析）；條目萃取；「有 errors 且無 data 才拋」的讀取語義 | REQ-RD-001..004、010 範圍外、012..014、REQ-GQL-006/007 |
| `internal/xapi/xerrors` | `*GraphQLError`、`*InvalidInputError`、`*MissingFieldError`、`*MaterialError`、`*CredentialInvalidError`、`*TransportError`；wire schema（`kind`/`code`/訊息/`retryable`/`credential_impacting`）；閘道錯誤文案存活清單（保留 `proxy_timeout`/`login_timeout`/`proxy_configuration`，閘道專屬四條映射為直連語義） | 規格 12.1/12.2、REQ-LOG-042/043 |
| `X/` 應用層（模組根） | MyGO 原生 UI 接線：`main.go` 與 login/home 兩個 view、`CookieStore` 的 SQLite 插件實作（v0.9.0 起以 native module 載入、API 不變）、代理解析與預檢、啟動狀態機、去促銷與 `work_id` 去重、單一脫敏函式（遮蔽 `auth_token`/`ct0` 值） | REQ-WEB-104 語義、REQ-INV-01、§13.4 子集 |

### 3.3 引擎公開 API（接縫最小面）

`internal/xapi` 內可見的引擎面（應用層 import `github.com/OG-Open-Source/RIGG/X/internal/xapi`）：建構 `Options`（Cookie 原始字串、代理 URL、
`CookieStore`、可選 Clock/Rand 注入）→ `New` → `Session`；會話操作
`Viewer()`、`HomeTimeline(cursor, count, seenTweetIds)`、`Cookies()`、
`UpdateCookies(raw)`、`Close()`；錯誤以 `xerrors` 型別呈現，app 以 `errors.As`
判別四類（`*CredentialInvalidError`、`*MaterialError`、`*TransportError`、
`*GraphQLError`）。此介面形狀為 v2 逃生通道的一部分：v1 之後新增能力以新方法
擴充，不改動既有簽名。

### 3.4 資料流：一次 HomeTimeline 請求

1. app 呼叫 `Session.HomeTimeline(cursor, count, seen)`。
2. `graphql` 自註冊表取 `HomeTimeline` metadata（queryId、送出方法）；依
   OQ3 裁決：無已讀記錄 → `GET`（query string 有序組裝：`variables`→
   `features`（該操作宣告時）→`fieldToggles`（非空時），無 cursor 時帶
   `requestContext=launch`、不含 `seenTweetIds`）；有已讀 → `POST`（body 鍵序
   `variables`→`features`→`fieldToggles`→`queryId`，`seenTweetIds` 置於
   variables 末）。`variables` 以緊湊 JSON 序列化。
3. `headers` 建對應標頭族（GET 用 GraphQL GET 族、POST 用 GraphQL POST 族）；
   `x-csrf-token` 逐字取自當前 snapshot 的 `ct0`（REQ-HDR-008）。
4. `transport` 補預設標頭（UA、Client Hints、`accept`、`accept-language`、
   `priority`）、移除導覽標頭、依目標主機推導 `sec-fetch-site`；不帶 XCTID
   （探測通過路徑）或先算 XCTID 再以最終 Cookie 狀態重取 csrf（探測不通過
   路徑，`auth` 的 snapshot 代數保證 read-after-write 一致）。
5. tls-client 經帳號代理送出；重定向一律不跟隨（v1 語義）。
6. 回應處理：非 2xx 依 12.4 語義分類；JSON 解析後「有 `errors` 且無 `data`」
   → `*GraphQLError`（讀取語義，REQ-GQL-006）；回應 `Set-Cookie` 差異合併回
   XAuth（REQ-AUTH-014）並 write-through 至 `CookieStore`（同步、在回應處理
   完成前）。
7. `api` 做游標深度遍歷（`cursorType == "Bottom"` 且 `value` 非空）與條目
   萃取（`entryId` 以 `tweet-` 開頭）；回傳 `posts`/`users`/`cursor`。

### 3.5 錯誤分類（同行程直呼；形狀預留 IPC wire schema）

| 引擎型別 | wire `kind` | `retryable` | `credential_impacting` | app 處置 |
| --- | --- | --- | --- | --- |
| `*CredentialInvalidError` | `credential` | false | true | 標記會話失效，導回 login page |
| `*MaterialError` | `material` | true（重試 bootstrap） | false | 顯示素材狀態與重試入口，**不清憑證** |
| `*TransportError`／逾時 | `transport` | true | false | 顯示連線狀態，**不清憑證** |
| `*GraphQLError`（code 32/89/215/326） | `graphql` | false | true | 同 `credential` |
| `*GraphQLError`（code 88/344/226） | `graphql` | 退避不重試 | false | 提示稍後再試 |
| `*GraphQLError`（code 353） | `graphql` | 一次（重組後） | false | 引擎內部重試一次，不外洩 |
| `*InvalidInputError` | `input` | false | false | 表單層提示 |

`res_json` 僅存在於引擎診斷層（`OnResponse` 掛鉤、脫敏後）。app 以 Native
為主，**不採 WebView 作為 UI 形態**；唯一預期的例外是「軟體內瀏覽網站」——
在 native UI 內嵌 web 內容（MyGO 支援 native UI 中內嵌 web 頁面），該內容
僅承載瀏覽，不承載 X 引擎介面。view 層以 `errors.As` 取得錯誤值後讀
`kind`/`code` 欄位映射畫面；引擎介面永不跨越序列化邊界。

### 3.6 併發與生命週期

XAuth 為不可變 snapshot，`UpdateCookies` 以原子替換產生新版本；標頭建造與
「XCTID 後重取 csrf」綁定同一 snapshot 代數，消除 read-after-write 競爭
（`go test -race` 為退出條件）。lazy 初始化（素材、註冊表）以 singleflight
收斂。所有 goroutine 掛在同一 context 樹；UI 取消映射為 context cancel，
已取消請求的回應不再寫入 XAuth。`Close()` 順序：cancel 根 context → 等
in-flight 收斂 → flush 暫存 → 關閉 SQLite。MyGO 0.9.0 起，OS 結束會話
（登出／關機）時 quit handler 會執行——註冊 quit handler 走同一 flush
順序，避免關機時丟失未落盤的 ct0 輪替。

### 3.7 啟動狀態機與設定面

啟動順序：**代理就緒 →（素材就緒，僅探測不通過路徑）→ 憑證就緒**，每段有
獨立錯誤態與重試入口；素材/註冊表 bootstrap 為可取消背景任務並回報進度
（shell 重試預算 `X_SHELL_ATTEMPTS` 語義，預設 12、夾 1–12）。代理解析：
`X_PROXY` → `HTTPS_PROXY`/`ALL_PROXY` → MyGO netproxy → 失敗關閉；偵測結果
先短逾時預檢再綁定，提供顯式「重新偵測」；代理變更重建客戶端並回收連線。
引擎只認 Options 欄位，env 讀取屬 `internal/app` 薄層。

### 3.8 決策記錄落點

本節承載的裁決（OQ3＝方法表為準、OQ5＝七層、TRN-011 偏離登錄、錯誤文案存活
清單、代理解析層裁減、v1 不跟隨重定向）於開工時回寫 `docs/DECISIONS.md`，
作為對規格的解讀與偏離登錄，不改動凍結的規格本身。工程事實登錄：本專案為
RIGG monorepo（repo 根 `C:\Users\User\Documents\GitHub\RIGG`）中的 X 重寫；
app module `github.com/OG-Open-Source/RIGG/X` 以
`mygo init -template native -module github.com/OG-Open-Source/RIGG/X X`
初始化於 `X/`（尚未建立，init 為實作第一步）；版本標籤 `X/v0.1.0` 格式；
宿主框架釘選 MyGO v0.9.0
（SQLite 插件走 native module、`ui/video` 原生播放、quit handler 於 OS
會話結束時執行；native module 機制上游標實驗性，列為依賴風險）；app 以
Native 為主，WebView 僅作為「軟體內瀏覽網站」的內嵌內容，不作為 UI 形態。

## Alternatives Considered

### Alternative A: 引擎與 app 拆成兩個獨立 Go module

*   **Pros**: SDK 邊界最乾淨；日後可單獨發布版本。
*   **Cons**: 跨 module 的本機開發要走 `go.work` 或 `replace`，版本同步與
    CI 矩陣成本高於收益；本專案只有一個消費者。
*   **Reason for Rejection**: 採 monorepo 內單一 module
    `github.com/OG-Open-Source/RIGG/X`，以「`X/internal/xapi` 不得 import
    MyGO」的 CI 靜態檢查達成同等邊界；日後抽離只是搬目錄加 `go mod init`。

### Alternative B: 保留規格第 10 章的 Go TLS 閘道子程序

*   **Pros**: 完整對齊規格的部署拓撲與 `REQ-GW-*`/`REQ-DEP-*` 契約。
*   **Cons**: 閘道存在理由是隔離 Python 的 TLS 限制；全 Go 後它只剩程序間
    RPC 的部署與除錯負擔，桌面單機場景更是純粹開銷。
*   **Reason for Rejection**: 傳輸改為行程內 `bogdanfinn/tls-client`
    （chrome_150 profile）；`REQ-TRN-030..041` 閘道客戶端與第 10 章不適用，
    偏離登錄於 `docs/DECISIONS.md`。

### Alternative C: 帳密登入（jetfuel＋Castle）先行

*   **Pros**: 使用者不需要先在瀏覽器登入一次。
*   **Cons**: jetfuel 全流程（REQ-LOG-001..022）依賴瀏覽器擷取的 Castle
    profile 與挑戰處理，是 v1 面積的數倍；Castle 純算另有 zlib 逐位元組
    對拍風險。
*   **Reason for Rejection**: v1 以 Cookie 匯入＋Viewer 驗證覆蓋價值主張；
    帳密登入整條延後 v2。

### Alternative D: 以 cgo 綁 zlib 提前支援 Castle 純算

*   **Pros**: 一次到位支援未來的帳密登入。
*   **Cons**: 違反零 cgo 約束（破壞「任何機器可交叉編譯」與單一執行檔屬性），
    且 Castle 不在 v1。
*   **Reason for Rejection**: Castle 整域延後；屆時再評估純 Go DEFLATE 的
    位元組對拍可行性。

### Alternative E: X 流量走 MyGO Fetch 插件

*   **Pros**: 直接復用宿主能力，少寫傳輸層。
*   **Cons**: 已查證（MyGO `docs/plugins/fetch.md`）：Fetch 插件的消費者是
    WebView 頁面（提供 TS client、過濾 `Set-Cookie`、走系統代理）；本 app
    為 native UI、無頁面，消費者不存在。其 `fetch.New` 雖可注入自訂
    `http.Client`，但那只是把引擎客戶端再包一層頁面導向的插件邊界；TLS
    指紋（uTLS ClientHello）插件本身不提供，仍需自持。另違反 REQ-TRN-010
    單一出口，繞過標頭補齊與 sec-fetch-site 推導（REQ-TRN-012..015）。
*   **Reason for Rejection**: 傳輸層是本專案的承重牆，必須自持；宿主插件
    不減少任何必做工作，只增加一層間接。

### Alternative F: 代理缺失時直連回退

*   **Pros**: 提升無代理環境的可用性。
*   **Cons**: 直連暴露真實 IP；X 對非官方軟體直連的限制與懲罰**未經驗證**
    （風控形態、限流策略、是否直接 403）——見 OQ12。
*   **Reason for Rejection**: 現依規格 REQ-INV-02 維持失敗關閉（代理缺失一律
    明確報錯並給設定入口）。此立場為暫定：OQ12 的驗證結果（直連 vs 代理
    對照組）若顯示直連可行且無懲罰，由需求方修訂此約束並登錄偏離；在此之前
    不得存在直連路徑。

### Alternative G: 繼承原專案的錯誤字串契約（逐字保留）

*   **Pros**: 與上游使用者所見文案完全對齊。
*   **Cons**: 本專案為 clean-room 重寫，**不繼承原專案的使用者可見契約**；
    本 repo 的慣例是純 Go 與 Go 社群規範（`fmt.Errorf`＋`%w` 包裝、型別化
    錯誤）；原專案文案對 MyGO 的 UI 文案層沒有約束力，逐字斷言是儀式成本。
*   **Reason for Rejection**: 需求方 2026-11-11 裁決——不管理原專案的錯誤
    字串限制。引擎採 Go 社群慣例；規格 §2-6 對「錯誤字串逐字保留」、
    REQ-AUTH-004 精確文案、REQ-GQL-007 訊息格式在本專案**不適用**，登錄為
    規格偏離。行為層面保留的只有錯誤**分類**：kind/code 機械可分
    （REQ-LOG-042/043 不變）；缺 `ct0` 仍拋錯（REQ-AUTH-004 的行為），文案
    由 app 層決定（OQ10）。

### Alternative H: XCTID 直接實作（不做探測）

*   **Pros**: 完整對齊規格標頭族；不賭外部行為。
*   **Cons**: 規格全文無「XCTID 不可缺席」的 MUST（REQ-WR-001 只約束
    mutation，不在 v1）；素材網路鏈依賴 shell A/B 路由（REQ-GQL-031）與
    代理，是冷啟動 Blocker 與 jar 隔離複雜度的來源；§16-5 明言服務端校驗
    屬外部依賴。
*   **Reason for Rejection**: 採用探測先行（需求方 2026-11-11 裁決）——一日
    L2 探測實驗定存廢：通過則整條延後 v2（消滅冷啟動、jar 隔離、浮點語義
    三個 Blocker）；不通過則降級為「純算＋隨倉庫素材」形態。探測蓋不住的
    唯一風險是軟降級（226 上升），後果落在單一自有帳號。存廡由實驗結果
    實證（OQ2），不做先驗假設。

### Alternative I: HomeTimeline 只實作 POST 單形態

*   **Pros**: 一套線路形態與測試向量；REQ-GQL-005 原文完全滿足。
*   **Cons**: 7.8.2 方法表的 GET 列（無已讀 → GET，對應上游測試證據）成為
    永久偏離；若服務端要求首頁首載必須 GET，仍要補分支。
*   **Reason for Rejection**: OQ3 裁決採方法表為準（GET/POST 雙形態）；
    REQ-GQL-005 解讀為「帶 seenTweetIds 的 query 型必須 POST」，解讀與三個
    落地點（線路形態、參數順序、XCTID METHOD 輸入）寫入
    `docs/DECISIONS.md`。雙形態的正確性不做先驗假設：由 P0 探測實驗（矩陣
    含 HomeTimeline GET／POST 兩形態）與 L2 煙霧一併驗證，任一形態被服務端
    拒絕即回報並改判。

### Alternative J: 引擎收斂為單一套件（至多兩個內部子套件）

*   **Pros**: 面積最小；無過早抽象；單一消費者下公開面最小。
*   **Cons**: v2 加入寫入、媒體、私信時需拆子套件——機械式但觸及面廣；
    規格域（TRN/AUTH/HDR/GQL/XCT）與程式碼的對應關係不明顯。
*   **Reason for Rejection**: OQ5 裁決採七層套件樹
    （fingerprint/transport/auth/headers/graphql/xctid/api＋xerrors），規格
    域一一對應，v2 各歸其位；xctid 套件在探測通過路徑下整個不建。

### Alternative K: 砍除 CookieStore 窄介面（Cookie 以字串進出）

*   **Pros**: 少一個介面，接縫更小。
*   **Cons**: v2 會話失效語義（12.3 code 32/89：清除憑證、作廢會話）需要
    持久化掛點；屆時補介面是公開 API 變更。
*   **Reason for Rejection**: 介面成本僅 `Load`/`Save` 兩方法，保留。

### Alternative L: 引擎自行讀取環境變數（`X_PROXY` 等）

*   **Pros**: 對齊規格 §13 的設定矩陣，CLI 情境零接線。
*   **Cons**: `internal/xapi` 依賴程序環境即產生隱式接縫；桌面 app 的設定
    來源是 UI 與作業系統，不是 shell 環境。
*   **Reason for Rejection**: 引擎只認 `Options` 欄位；env 讀取屬 `X/` 應用
    層薄層（可保留同名 env 的讀取）。

### 未決的權衡（依探測實驗結果）

*   **註冊表靜態快照 vs 重建鏈**：探測通過（XCTID 延後）→ 隨倉庫附僅含
    HomeTimeline＋Viewer 的 `static/graphql.json` 快照（§13.3 L1698 明載
    「隨倉庫提供」，登錄 `generatedAt`/`frontendSha`），重建鏈降為 v2 手動
    工具；探測不通過 → 素材與註冊表共享單一 bootstrap 管線（一次 shell
    同時產出兩者）＋漂移監測。
*   **重定向逐跳迴路**：v1 一律不跟隨（跨主機不洩漏憑證的嚴格超集）；
    REQ-TRN-040 完整語義隨 v2 登入流。
*   **指紋跨重啟持久化**：不持久化，每次啟動重抽（REQ-TRN-001 原樣滿足）。

--------------------------------------------------------------------------------

## Non-Functional Requirements

*   **Security & Privacy**：憑證（cookie 字串）僅存於 `CookieStore`（SQLite）
    與記憶體 XAuth；任何日誌、診斷、view 顯示經 `sanitize.go` 單一脫敏函式
    （遮蔽 `auth_token`/`ct0` 值，REQ-INV-01）；憑證不入 git（根
    `.gitignore`）；錯誤分類不攜帶原始回應（`res_json` 留引擎診斷層）。
    威脅模型取捨（SQLite 明文＋OS 使用者隔離 vs OS 憑證庫）依 OQ7 裁決後
    登錄 README。
*   **Performance（核心動機）**：本專案存在理由是 RAM 節約——純 Go、零 cgo、
    單一執行檔、native UI 無 WebView 引擎。P4 建立記憶體基線（閒置、載入
    100 帖、連續翻頁 10 頁三個場景，量測常駐 RSS），基線數值與任何回歸
    記錄於 `docs/DECISIONS.md`；不在設計階段發明未經量測的數字。
*   **Rollout & Rollback**：版本以 `X/v0.1.0` 標籤遞增（SemVer）；MyGO
    updater 插件（0.9.0 已改善）為發布期選項，v1 以手動安裝為準；引擎
    無狀態遷移問題（CookieStore schema 變更時附遷移步驟）。
*   **平台**：Windows 為首要目標（需求方環境），macOS/Linux 由 MyGO 純 Go
    屬性免費獲得，v1 不做平台專項驗證。

## Additional Resources

*   `XAPIS_BEHAVIOR_SPEC.md`（規格，唯一行為參考；遷入 RIGG repo 根）
*   `docs/designs/concepts_brief.md`（構想簡報，本設計的上游輸入）
*   MyGO：https://github.com/egoist/mygo 、`go doc -all github.com/egoist/mygo`、
    v0.9.0 release notes（2026-11-11）、`docs/plugins/fetch.md`（E 項查證依據）
*   bogdanfinn/tls-client（含 profile 清單與 `fhttp` 標頭序行為，spike 對象）
*   規格 §17（OG-Open-Source 登錄）、§19（授權）——開源交付的約束來源

## Future Improvements

*   **帳密登入**：jetfuel 流程＋Castle 純算（含 zlib 對拍可行性評估）。
*   **寫入與媒體**：發帖、串文、互動、媒體上傳（REQ-WR/MED 域）。
*   **時間線影片**：`ui/video` 原生播放（0.9.0 已就緒）。
*   **軟體內瀏覽**：native UI 內嵌 web 頁面瀏覽站外連結。
*   **註冊表重建工具**：`refresh-graphql` 等價物的桌面化（漂移自癒）。
*   **多帳號**：Session 以 (帳號, 代理) 為實例鍵的擴充。

--------------------------------------------------------------------------------

## Detailed Implementation Plan

> 設計階段僅產出文件；所有程式碼檔案在實作階段建立。P0 的兩個 spike 以
> 暫存程式（repo 外，如 `%TEMP%\opencode`）執行，結論回寫
> `docs/DECISIONS.md`，不在 repo 內留下程式檔。

### 實作順序（對應 5 個 commit）

| 順序 | Commit | 內容 | 退出條件 |
| --- | --- | --- | --- |
| C1 | `docs:` | repo 文件層（下表 #1–#7） | 文件齊備；`.github` submodule 掛載；OQ 裁決回寫 |
| C2 | `chore:` | `mygo init` 建立 `X/`、go.mod 依賴釘選、七層套件骨架（空實作＋doc.go）、CI | `go build ./...` 綠；CI 綠含 import 禁令檢查 |
| C3 | `feat(transport):` | fingerprint＋transport（#10–#14） | 指紋、標頭補齊、sec-fetch 推導、不跟隨、觀測鉤單元測試全綠 |
| C4 | `feat(auth):` | auth＋headers＋xerrors（#15–#18） | Cookie 解析、snapshot 併發（`-race`）、標頭族順序測試全綠 |
| C5 | `feat(api):` | graphql＋api＋app 層（#19–#26，xctid 依探測結果 #27） | L2 煙霧＋app 端到端；`go test -race ./...` 全綠 |

前置（P0，先於 C2）：tls-client spike 能力對照表、XCTID 探測實驗（含
HomeTimeline 兩形態與直連對照組）、OQ3–OQ12 裁決回寫。

### Target Files to Create/Modify

#### 1. `README.md`（Create，RIGG 根）

*   **Type / Scope**: 專案 README（OG-Open-Source 標準：Introduction/
    Features/Attribution/Installation/Usage/Contributing/Versioning）。
*   **Proposed Changes**: 現 repo 根無 README。內容：RIGG monorepo 介紹與
    X 重寫定位、RAM 動機、Prerequisites 逐項版本（Go 1.27.3、MyGO v0.9.0、
    平台）、Attribution 保留 OG-Open-Source 與 XApis 署名、CoC 連結
    （§17 MUST）、上游授權確認前的私有狀態聲明。

#### 2. `XAPIS_BEHAVIOR_SPEC.md`（Create，自 `XApis-golang` 暫存區遷入 RIGG 根）

*   **Type / Scope**: 行為規格（唯一行為參考；檔名符合 §17
    `^[A-Z_]+\.md$`）。
*   **Proposed Changes**: 原樣遷入，不改內容；UTF-8 無 BOM、LF。

#### 3. `docs/designs/concepts_brief.md`（Create，自暫存區遷入）

*   **Type / Scope**: 構想簡報存檔（本設計的上游輸入，供追溯）。
*   **Proposed Changes**: 原樣遷入。

#### 4. `docs/DECISIONS.md`（Create，RIGG 根）

*   **Type / Scope**: 決策記錄（ADR 形式，一決策一節）。
*   **Proposed Changes**: 逐條登錄：OQ3（HomeTimeline 方法表解讀＋三落地
    點）、OQ5（七層）、TRN-011 curl_cffi→tls-client 偏離、§2-6 錯誤字串
    偏離（Alternative G 裁決）、12.2 錯誤文案存活清單、代理解析層裁減
    （OQ6）、v1 不跟隨重定向、閘道移除（Alternative B）、OQ12 直連暫定
    立場、spike 與探測實驗結論（P0 產出）。

#### 5. `.gitignore`（Modify，RIGG 根；暫存區版本遷入後擴充）

*   **Type / Scope**: 版本控制排除規則（REQ-INV-01、§16-13）。
*   **Proposed Changes**: 增補 `.env*`、`data/`、`*.local`、憑證與素材
    快取產物、`X/` 內 MyGO 建置產物。

#### 6. `.gitattributes`（Create，RIGG 根）

*   **Type / Scope**: 換行與編碼基準。
*   **Proposed Changes**: `* text=auto eol=lf`＋二進位標記，符合規格 LF
    基準，消除 CRLF 轉換警告。

#### 7. `.github`（Create，git submodule 掛載於 `/.github/`）

*   **Type / Scope**: OG-Open-Source 組織標準檔（§17 MUST）。
*   **Proposed Changes**: 以 submodule 掛載組織 `.github`；若 GitHub Actions
    不執行 submodule 內 workflow，則補 repo 層最小 workflow（見 #8）並在
    DECISIONS 登錄差異。

#### 8. `.github` 之上的 CI 來源（視 #7 驗證結果，Create）

*   **Type / Scope**: CI workflow（build＋vet＋test＋import 禁令）。
*   **Proposed Changes**: 步驟：checkout（含 submodule）、setup-go 1.27.3、
    `go build ./...`、`go vet ./...`、`go test -race ./...`（工作目錄
    `X/`）、import 禁令檢查（`X/internal/xapi` 依賴圖不含
    `github.com/egoist/mygo`）。

#### 9. `X/`（Create，`mygo init -template native -module github.com/OG-Open-Source/RIGG/X X`）

*   **Type / Scope**: MyGO 原生 UI app 模組根。
*   **Proposed Changes**: init 產生的 `go.mod`（module 路徑、`go 1.27.3`）、
    `mygo.json`／`mygo.config.ts`（app 名稱、視窗預設）、`main.go` 骨架；
    `go.sum` 隨依賴釘選生成（MyGO v0.9.0、bogdanfinn/tls-client 最新穩定
    版——以 spike 確認含 chrome_150 後釘選）。

#### 10. `X/internal/xapi/doc.go`（Create）

*   **Type / Scope**: 引擎套件文件。
*   **Proposed Changes**: clean-room 來源聲明（僅依規格）、不得 import
    MyGO 的邊界說明、六族「v1 裁剪非最終形態」註記、探測實驗對 xctid
    存廢的影響說明。

#### 11. `X/internal/xapi/xerrors/errors.go`（Create）

*   **Type / Scope**: 錯誤型別集與分類。
*   **Proposed Changes**: 六型別（GraphQLError{OperationName, Errors,
    ResJSON, Code}、InvalidInputError、MissingFieldError、MaterialError、
    CredentialInvalidError、TransportError），各攜 `kind`/`code`/
    `retryable`/`credential_impacting` 欄位；`errors.As` 判別輔助；12.2
    錯誤碼→kind 映射與閘道文案存活清單（`proxy_timeout`/`login_timeout`/
    `proxy_configuration` 保留，閘道專屬四條映射直連語義）；Go 慣例
    `Error()`（無逐字契約，Alternative G）。

#### 12. `X/internal/xapi/xerrors/errors_test.go`（Create）

*   **Type / Scope**: 單元測試。
*   **Proposed Changes**: code 32/89/215/326→credential、88/344/226→退避、
    353→重試一次、讀取路徑「有 errors 且無 data 才拋」分類表逐一斷言
    （依 3.5 表）。

#### 13. `X/internal/xapi/fingerprint/fingerprint.go`（Create）

*   **Type / Scope**: 應用層指紋真源。
*   **Proposed Changes**: 九欄位結構（REQ-TRN-002）與 Chrome/153 常數
    （REQ-TRN-003/004，集中單檔、標註「僅能跟隨上游規格版本」）、四選一
    螢幕抽樣（REQ-TRN-005）、程序內單例、`Reset` 僅供測試建構路徑使用。

#### 14. `X/internal/xapi/fingerprint/fingerprint_test.go`＋`X/internal/xapi/transport/`（Create）

*   **Type / Scope**: transport 套件與測試。
*   **Proposed Changes**: `client.go`：tls-client 建構（chrome_150）、預設
    標頭補齊與移除導覽標頭（REQ-TRN-012..014）、sec-fetch-site 推導
    （REQ-TRN-015）、`session`/`anonymous` 兩形態（jar 隔離；探測通過則僅
    `session`）、不跟隨重定向、30 秒預設逾時、fail-closed 代理
    （REQ-TRN-018）、`OnRequest`/`OnResponse` 觀測鉤（脫敏）。測試：
    `client_test.go`（補齊/移除/sec-fetch 矩陣）、`observe_test.go`
    （掛鉤與脫敏）、jar 隔離測試（若 anonymous 形態存在）。

#### 15. `X/internal/xapi/auth/auth.go`＋`auth_test.go`（Create）

*   **Type / Scope**: 鑑權狀態容器。
*   **Proposed Changes**: XAuth 不可變 snapshot＋原子替換（版本計數）、
    `PrepareAuth`（REQ-AUTH-003 解析；缺 `ct0` 有 `auth_token` 拾
    InvalidInputError——行為保留、文案 Go 慣例）、`UpdateCookies` 合併語義
    （REQ-AUTH-014）、`IsLoggedIn`、`CookieHeader`、`CSRFToken`、預設公開
    web bearer（REQ-AUTH-002）、guest token 方法位（unimplemented）、
    `CookieStore{Load,Save}` 介面宣告。測試：解析矩陣、合併語義、`-race`
    併發更新。

#### 16. `X/internal/xapi/headers/headers.go`＋`headers_test.go`（Create）

*   **Type / Scope**: 標頭族建造。
*   **Proposed Changes**: 六族有序資料表（v2 填表即用）；GraphQL GET/POST
    兩族輸出；有序可選槽位（XCTID、`x-att`、`x-guest-token`）；小寫輸出
    （REQ-HDR-013）、未登記者字母序追加（REQ-HDR-002）、`x-csrf-token`
    逐字取自 snapshot（REQ-HDR-008）。測試：族順序、小寫、槽位缺席。

#### 17. `X/internal/xapi/graphql/compact.go`＋`compact_test.go`（Create）

*   **Type / Scope**: 緊湊序列化器。
*   **Proposed Changes**: 有序緊湊 JSON（分隔符 `,`/`:`、
    `SetEscapeHTML(false)`、非 ASCII 不轉義，REQ-GQL-002/INV-07）；有序
    query string 組裝（禁 `url.Values.Encode()`；緊湊 JSON 先序列化再
    percent-encode）；POST body 鍵序 `variables`→`features`→
    `fieldToggles`→`queryId`（REQ-GQL-003）。測試：鍵序、轉義、空
    features 不出鍵（REQ-GQL-004/024）。

#### 18. `X/internal/xapi/graphql/registry.go`＋`registry_test.go`（Create）

*   **Type / Scope**: 註冊表載入與操作 metadata。
*   **Proposed Changes**: `loadRegistry`（REQ-GQL-010..016：頂層欄位、快取、
    強制重載、查無操作拋 MissingFieldError）、`path()`/`url()`（
    REQ-GQL-020/021）、features/fieldToggles 解析（REQ-GQL-015、023 的
    v1 退化序：呼叫端顯式 → 註冊表）。fixture 為合成註冊表（僅
    HomeTimeline＋Viewer，含 `generatedAt`）。探測不通過路徑另加
    `bootstrap.go`（shell 抓取＋分片解析，匿名客戶端）。

#### 19. `X/internal/xapi/api/send.go`（Create）

*   **Type / Scope**: 統一發送函式。
*   **Proposed Changes**: REQ-RD-001..004：query 型以 POST 族標頭＋宣告
    方法送出、timeout 30 秒、`raise_for_status` 語義先行的 Go 對應、
    「有 errors 且無 data」→ GraphQLError、Set-Cookie 差異回寫 XAuth
    （REQ-AUTH-014）＋write-through 呼叫 CookieStore。

#### 20. `X/internal/xapi/api/timeline.go`＋`timeline_test.go`（Create）

*   **Type / Scope**: 讀取方法與解析。
*   **Proposed Changes**: `Viewer`（`withCommunitiesMemberships=true`）、
    `HomeTimeline`（雙形態：無已讀 GET＋`requestContext=launch`；有已讀
    POST＋`seenTweetIds` 置 variables 末——OQ3 裁決）、游標深度遍歷
    （REQ-RD-012：`cursorType=="Bottom"` 且 value 非空）、條目萃取
    （REQ-RD-013：`entryId` 以 `tweet-` 開頭）。測試以合成回應 fixture：
    翻頁、游標重複停止、無游標停止、深層結構遍歷。

#### 21. `X/cookies_store.go`＋`cookies_store_test.go`（Create）

*   **Type / Scope**: CookieStore 的 SQLite 實作（應用層）。
*   **Proposed Changes**: MyGO SQLite 插件（v0.9.0 native module 載入）；
    單表 `accounts(cookie_line, updated_at)`；write-through 語義（引擎
    回應處理中同步呼叫 `Save`）；`Load` 於啟動時注入 Options。

#### 22. `X/proxy.go`＋`proxy_test.go`（Create）

*   **Type / Scope**: 代理解析鏈（應用層）。
*   **Proposed Changes**: `X_PROXY` → `HTTPS_PROXY`/`ALL_PROXY` → MyGO
    netproxy → 失敗關閉；偵測結果短逾時預檢；顯式重新偵測；代理變更重建
    引擎客戶端。測試：順序、預檢失敗路徑。

#### 23. `X/app_state.go`（Create）

*   **Type / Scope**: 應用狀態與啟動狀態機。
*   **Proposed Changes**: 代理就緒→（素材就緒，探測不通過路徑）→憑證就緒
    三段狀態，各段獨立錯誤態與重試入口；bootstrap 為可取消背景任務並回報
    進度；`Close()` 順序（cancel→收斂→flush→關 SQLite）＋quit handler
    （0.9.0）註冊同一 flush。

#### 24. `X/view_login.go`＋`view_login_test.go`（Create）

*   **Type / Scope**: login page（原生 UI view）與測試。
*   **Proposed Changes**: cookie 貼上輸入、代理狀態提示、提交動作呼叫
    `PrepareAuth`→`Viewer` 驗證、成功導向 home、失敗依 `kind` 映射訊息與
    重試入口（不清憑證類錯誤的提示區別）。測試以 MyGO view Tester
    （無窗測試）：錯誤映射、成功路徑、重複提交防抖。

#### 25. `X/view_home.go`＋`view_home_test.go`（Create）

*   **Type / Scope**: home page（原生 UI view）與測試。
*   **Proposed Changes**: 時間線列表（虛擬化列表）、捲動觸發翻頁、去促銷
    （`promotedMetadata`／`entryId` 以 `promoted` 開頭）與 `work_id` 去重、
    到底狀態提示、錯誤橫幅（kind 映射）。測試：去重合併、翻頁觸發、
    錯誤不清洗既有列表。

#### 26. `X/sanitize.go`＋`sanitize_test.go`（Create）

*   **Type / Scope**: 單一脫敏函式（應用層）。
*   **Proposed Changes**: 遮蔽 `auth_token`/`ct0` 值（保留鍵名與長度輪廓）；
    view 顯示、日誌、診斷輸出唯一入口。測試：鍵值遮蔽、無憑證輸入直通。

#### 27. `X/internal/xapi/xctid/`（Create——僅探測不通過路徑）

*   **Type / Scope**: XCTID 純算套件（條件性）。
*   **Proposed Changes**: `generator.go`（REQ-XCT-001..010：幀選擇、下標
    運算、摘要、payload、XOR 編碼；`WithClock`/`WithRand` 注入）、
    `bezier.go`＋`rounding.go`（cubic-bezier 求值、round-half-even 自實
    作、固定 epsilon）、`material.go`（REQ-XCT-012 結構檢查→
    MaterialError；`t<0` 拋輸入錯誤）。測試：結構性質（Base64 可解、
    payload 段數、XOR 往返、rnd 獨立性）＋L1 向量對拍（向量為需求方後補
    的阻塞交付物）。探測通過則本目錄**不建立**，headers 槽位留空。

#### 28. `X/static/graphql.json`／`X/static/transaction_l1.json`（Create——條件性）

*   **Type / Scope**: 隨倉庫執行期資料（§13.3 L1698「隨倉庫提供」）。
*   **Proposed Changes**: 快照由需求方以登入態瀏覽器擷取；`graphql.json`
    僅含 HomeTimeline＋Viewer（探測通過路徑）；`transaction_l1.json` 為
    XCTID 素材基線（探測不通過路徑）。兩者登錄 `generatedAt`／來源／擷取
    日期於 DECISIONS，聲明為可替換資料、非不變式。

### Verification Plan

*   **Compilation/Build**: `go build ./...`（於 `X/`）；`mygo build` 產出
    單一執行檔煙霧。
*   **Unit Tests**: `go test -race ./...` 全綠為每個 commit 的退出條件；
    測試名引用 REQ 編號（如 `TestPrepareAuth_MissingCt0_REQ-AUTH-004`）
    建立追溯。
*   **Static Analysis & Linting**: `go vet ./...`；CI import 禁令檢查
    （`X/internal/xapi` 依賴圖不含 `github.com/egoist/mygo`）。
*   **Test Coverage Requirement**: 不設百分比門檻；門檻為「每個已實作
    REQ 至少一條測試」的追溯表完整。
*   **E2E/Integration**：
    1. P0 spike／探測（repo 外暫存程式）：tls-client 能力對照表；XCTID
       探測矩陣（Viewer/HomeTimeline × GET/POST × 帶/不帶 XCTID × 代理/
       直連，≥20 次跨 2 會話）。
    2. L2 煙霧（真實 Cookie＋代理，憑證不入 repo）：Viewer 回傳身分、
       HomeTimeline 回傳貼文、翻頁至無游標。
    3. App 端到端：`go tool mygo dev` 手動走 CUJ 1–3；選配
       `mygo-automation` 做無窗驅動驗證。
*   **Traceability**: `docs/DECISIONS.md` 維護「已實作 REQ ↔ 測試 ↔ 偏離
    登錄」三欄表，驗收以第 14 章裁剪範圍（L2＋結構性質）為準。

--------------------------------------------------------------------------------

*Drafted with EGM*
