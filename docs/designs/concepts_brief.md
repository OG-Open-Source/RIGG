# MyGO X 引擎 v1（XApis-golang）

* **Author:** EGM Synthesizer；審查視角：design（§1、§3、§5）／codebase-details（§1、§6）／zoom-out（§4、§6）／comprehension（§0、§1、§2）／implementation-readiness（§2、§7）／structural（§5、§7）
* **Date:** 2026-11-11

> **審查狀態（元資料）：** 六視角全新上下文審查已完成（父層平行派發，報告全文收錄於本輪）。六位審查者狀態皆為 **completed**。本簡報為去重與衝突調解後的合成版；衝突裁決集中在 §4 與 §7。

## Overview

目標使用者是在桌面環境以自有 Cookie 會話讀取 X 時間線的單機使用者。核心價值：以 Golang 1.27.3 依 `XAPIS_BEHAVIOR_SPEC.md`（下稱「規格」，1922 行，唯一行為參考）的 `REQ-*` 行為契約重建 X Web 客戶端，作為 MyGO 桌面應用（https://github.com/egoist/mygo）的原生引擎，住在 `internal/xapi` 套件樹，且該套件樹不得 import MyGO 任何套件。v1 邊界：Cookie 匯入登入（Viewer 驗證）、HomeTimeline 讀取與游標翻頁、XCTID（v1 是否實作純算與素材鏈，依已採納的 P0 探測實驗結果裁決，見 §4.1）、傳輸與指紋。寫入、媒體、文章、私信、CLI、Web 控制台（第 8 章）、前端工作台（第 9 章）、閘道程序（第 10 章）不在 v1。工作目錄現況：規格一份、OG-Open-Source README 標準一份（內容是標準文件本身，非專案 README）、`CODE_OF_CONDUCT.md`（Contributor Covenant 3.0）一份；無程式碼、無 git repo、無測試 fixture。

## Analysis

### 0. 迷你詞彙表（comprehension）

新手先讀本節再讀 §1 之後。未列出的術語以規格 §5（L244–277）為準。

* **XCTID**：`x-client-transaction-id` 標頭的值，每請求現算的一次性簽章（規格 §5）。缺失是否被 X 服務端拒絕屬**待查證**——需求方已採納探測先行路線，由 P0 探測實驗以實證裁決（§4.1）。
* **XAuth**：鑑權狀態容器，持有 bearer、代理、cookie 映射、XCTID 產生器；規格 REQ-AUTH-001（L401）規定它是唯一持有者。
* **auth_token / ct0 / twid**：三種 X Cookie。`auth_token` 存在即視為已登入；`ct0` 是 CSRF cookie，必須與 `x-csrf-token` 標頭逐字相同；`twid` 是使用者識別 cookie。
* **shell**：`https://x.com/i/flow/login` 回傳的 app shell HTML，XCTID 素材與 GraphQL 操作註冊表皆從其 JS 分片重建。shell 有 A/B 路由（§16-4，L1813）：僅舊 client-web shell 含素材。
* **素材**：自 X 前端線上擷取的執行期資料（`static/graphql.json`、`static/transaction_l1.json`），可替換、會隨 X 發版漂移，不是規格（§3.2-3，L170）。
* **六族**：六種標頭族（REQ-HDR-001，L420）——GraphQL GET、GraphQL POST、REST GET、REST FORM、UPLOAD、DOC；v1 只需要 GraphQL 兩族。
* **緊湊 JSON**：分隔符 `,`/`:`、不轉義非 ASCII 的序列化（REQ-GQL-002、REQ-INV-07）。
* **Cursor**：X 回傳的不透明翻頁字串；游標萃取須對回應樹做深度遍歷（REQ-RD-012）。
* **chrome_150 vs Chrome/153**：並存不是筆誤。chrome_150 是傳輸層 TLS 模板版本（tls-client profile），Chrome/153 是應用層 UA／Client Hints 指紋版本；REQ-INV-09（L1564）允許兩層版本各自自洽。
* **MyGO**：宿主桌面框架（egoist 開發，MIT，https://github.com/egoist/mygo），純 Go、零 cgo，提供視窗（WebView 或原生 UI）、typed IPC（Bind Go 服務產生 TS client）、SQLite／Fetch／netproxy 插件；引擎不得 import MyGO。
* **clean-room**：本專案只依規格的行為描述重寫，不得複製原專案程式碼、註解或組織（§3.2）；`static/` 素材是執行期資料非規格。
* **X 風控動機句**：X 以傳輸層 TLS 指紋、標頭順序與內容特徵識別自動化流量，命中時回 GraphQL code `226`（疑似自動化）或限流碼；這是整個指紋、標頭順序、XCTID 工作的動機。Castle／jetfuel 是登入流程的反自動化令牌與端點，均不在 v1。

### 1. 範圍切分與依賴閉包

**1.1 HomeTimeline 送出方法矛盾（Blocker）。** REQ-GQL-005（L298）規定 HomeTimeline「即使屬 query 型，必須以 POST 送出」；7.8.2 方法表（L641）規定 `get_home_timeline`「無已讀記錄 → **GET** 且不含 seenTweetIds；有已讀 → **POST**」。裁決輸入必須涵蓋完整條款鏈：REQ-GQL-001（L294）、REQ-RD-001（L623）、REQ-RD-004（L626）、REQ-RD-014（L651，SearchTimeline 恆 POST），並連動 REQ-XCT-007（L496）的 `<METHOD>` 進 sha256 輸入——方法不定，對拍向量無法定稿。implementation-readiness 建議預設：以方法表為準（其對應上游測試證據），REQ-GQL-005 解讀為「帶 seenTweetIds 的 query 型必須 POST」，寫入決策記錄並標記為解讀非修改；裁決結果必須同時寫死三個落地點——線路形態、參數順序、XCTID 的 METHOD 值。zoom-out 提出更激進的替代（v1 只實作 POST 單形態），歸入 §4.4 權衡。衝突調解：三視角（design、codebase-details、implementation-readiness）要求以方法表為準；zoom-out 的單形態簡化列為權衡選項，由 P0 一併裁決。

**1.2 冷啟動鏈與狀態機（Blocker，design F9 深化）。** v1 依賴兩份執行期素材：XCTID 素材（REQ-XCT-013，L502）與註冊表（REQ-GQL-030..038）。失效模式：shell A/B 路由耗盡（REQ-GQL-031，L477，`X_SHELL_ATTEMPTS` 預設 12，最壞近六分鐘）、代理未配置、快取缺漏或結構不符。更關鍵的是 app 層錯誤呈現：缺素材時 Viewer 驗證失敗，UI 會把「素材不可用」誤呈為「憑證無效」。建議：啟動狀態機（代理就緒 → 素材就緒 → 憑證就緒），每段有獨立錯誤態與重試入口；bootstrap 為可取消背景任務並回報進度；`MaterialUnavailable` 與 `CredentialInvalid` 必須是不同錯誤類別（REQ-LOG-042，L606）。素材鏈與註冊表鏈各自需要一次 shell 抓取（design F12）：合併為單一 bootstrap 管線，取一次 shell 同時產出兩者。

**1.3 guest token、本地 ct0、twid 可裁剪（共識）。** §5（L252）明載 `guest_token` 是匿名請求憑證、已登入會話不需要。REQ-AUTH-009/010、REQ-AUTH-005（L405）、REQ-AUTH-008（twid）全部延後。但 design F4 補充：XAuth 應保留 guest token **方法位**（回 unimplemented），否則 v2 登入流將被迫改公開介面。

**1.4 cookie 合併與 XAuth 併發（Blocker，design F2 深化）。** REQ-AUTH-014（L414）必須存在：REQ-HDR-008（L427）的「XCTID 計算後以最終 Cookie 狀態重取 csrf」是跨兩步 read-after-write，XAuth 併發必然競爭。建議：XAuth 以不可變 snapshot＋原子替換（版本化）實作，lazy 初始化走 singleflight；`-race` 進 CI。`Set-Cookie` 落盤須 write-through（design F13）：回應處理完成前同步寫入儲存交易，否則行程被殺會丟 ct0 輪替、353 自癒失效；`Close()` 順序 cancel → 收斂 → flush → 關 SQLite；IPC 取消映射為 context cancel。

**1.5 匿名與會話的 jar 隔離（Blocker，design F1 深化）。** REQ-AUTH-012（L412）、REQ-INV-16（L1571）、§14.4（L1751）共同要求匿名素材抓取不得污染帳號 Cookie；若兩者共用同一 tls-client 實例，`/i/flow/login` 的 Set-Cookie（含匿名清除 ct0 情境）會污染帳號 jar。建議：transport 定義 `session` 與 `anonymous` 兩種建構形態（共享指紋與代理、jar 完全隔離）；加測試「匿名素材抓取後 XAuth snapshot 逐鍵不變」。codebase-details F4 補充：兩種 shell 抓取策略相反——XCTID 素材必須匿名，註冊表 shell 應攜登入態（REQ-GQL-035 的 user.config 覆寫）；兩套件各自持有抓取客戶端。zoom-out 對此鏈的存�另有裁決提案（§4.1）；若鏈被砍，INV-16 的操作化條文 REQ-AUTH-012 亦隨之出域。

**1.6 閉包完整清單（codebase-details F2/F8/F9/F15 深化）。** FeatureConcept 未列出但閉包內會被拉入：REQ-AUTH-002（L402，預設公開 web bearer）——整份前版簡報缺席，而它是每個 GraphQL 請求的必送標頭（REQ-HDR-003 第二位 authorization），缺了第一個請求就被拒；REQ-AUTH-012（INV-16 的操作化條文）與 REQ-AUTH-013（L413，強制重建素材入口）。REQ-INV-13（L1568，可觀察性）未納入——REQ-TRN-036/038 的診斷掛鉤語義需 transport 定義 OnRequest/OnResponse 觀測鉤（脫敏 method/path/status/耗時/csrf_matches_cookie/Set-Cookie 名稱），MyGO 錯誤 UI 的輸入即此。Go 落地陷阱：`url.Values.Encode()` 會排序鍵，違反 REQ-GQL-001 順序與 REQ-GQL-025 插入序，必須自建有序參數結構並定義「緊湊 JSON 先序列化再 percent-encode」。§2-6（L152）的錯誤字串逐字保留對 REQ-AUTH-004（L404）精確中文文案與 REQ-GQL-007 錯誤訊息格式成立；跨 app 邊界的處置見 §3.2 與 §4.5 的裁決。

### 2. 套件邊界、接縫與開工就緒

**2.1 邊界介面建議預設（implementation-readiness）。** `xapi.NewAuth(opts...) *Auth`；`PrepareAuth(cookiesStr) error`（含 REQ-AUTH-003/004 逐字文案）；`UpdateCookies(map)`；`IsLoggedIn()`；`CookieHeader()`；`CSRFToken()`；窄介面 `CookieStore{Load,Save}`。目錄：`go.mod/go.sum`、`cmd/mygo/main.go`、`internal/xapi/{fingerprint,transport,auth,headers,graphql,xctid,api}`、`internal/app/...`、`internal/xapi/xerrors`、`docs/DECISIONS.md`。zoom-out 反對七層與 CookieStore 介面，裁決見 §4.3。

**2.2 錯誤模型（design F3＋implementation-readiness）。** `xerrors` 型別集：`*GraphQLError{OperationName, Errors, ResJSON, Code}`（`Error()` 依 REQ-GQL-007 逐字格式；讀取路徑僅在完全沒有 `data` 時拋）、`*InvalidInputError`、`*MissingFieldError`、`*MaterialError`、`*CredentialInvalidError`、`*TransportError`。跨 typed IPC 後只剩字串，機械分類不可能——錯誤 wire schema 必須為 `kind`＋`code`＋訊息＋`retryable`＋`credential_impacting`；app 只 switch on `kind`；`res_json` 留引擎診斷層，IPC 只帶脫敏摘要。app 以 `errors.As` 判別四類，機械落實 REQ-LOG-042/043。

**2.3 tls-client 能力風險（Blocker，design F5＋codebase-details F1 深化）。** REQ-TRN-011（L369）指名 curl_cffi，採 tls-client 是 MUST 級偏離，前版未登錄。能力未驗證四點：(a) `chrome_150` profile 存在性隨版本，內建 profile 歷來落後 Chrome，且「隨機 TLS 擴充順序」option 未交代；(b) 標頭順序與小寫輸出（REQ-HDR-002/013）依賴 bogdanfinn/fhttp 保有插入序與原樣大小寫，標準 `net/http` 的 textproto 會破壞 HDR-013；(c) REQ-TRN-040 逐跳重定向控制（見 §2.5）；(d) per-client proxy 語義。建議：P0 加半天 spike 出「tls-client 能力對照表」；補偏離登錄；修正 P1 出口措辭（REQ-TRN-001..020 中 TRN-011/016 已排除，不得寫「全綠」）；uTLS 之後不得再引入第二套 TLS 棧。

**2.4 MyGO 插件重疊。** REQ-TRN-010（L368）禁止 X 流量走 MyGO Fetch 插件。netproxy 用於系統代理偵測屬 §13.4 鏈一環。SQLite 插件承載 app 側持久化。env 組態面的歸屬裁決見 §4.6。

**2.5 重定向（Blocker，design F6＋zoom-out）。** REQ-TRN-040（L394）與 §14.4（L1749–1750）要求「POST 收 302 不重放」、跨主機剝除憑證標頭；tls-client 預設自動跟隨且 301/302/303 會把 POST 轉 GET 重放，直接衝突。兩案並存：design 主張停用 auto-follow 自寫逐跳迴路（每跳依 REQ-TRN-015 重推導 sec-fetch-site）；zoom-out 主張 v1 一律不跟隨（嚴格超集）。裁決傾向 zoom-out 案（見 §4.4），完整逐跳語義隨 v2 登入流。

**2.6 Cookie 儲存與憑證保護（implementation-readiness＋design F14）。** 建議預設：MyGO secrets API 若有則用；否則 SQLite 明文＋OS 使用者隔離，不做自製加密（假安全感）；取捨登錄 README。REQ-INV-01（L1556）的落點在輸出脫敏（單一脫敏函式遮蔽 `auth_token`/`ct0` 值），非靜態加密。design F14 主張升級為 OS 憑證庫（DPAPI）——歸入開放問題。前置：確認 MyGO SQLite 插件與 secrets API 實際能力。

**2.7 指紋維運（design F15/F16）。** REQ-TRN-003/004 的指紋常數是逐字契約，本 repo 無權單方更新——「指紋凍結＝上游節奏依賴」。常數集中單檔並標註「僅能跟隨上游規格版本」，CHANGELOG 記錄規格版本對應。素材漂移（§16-1）無人監測：app 一鍵重建＋維護者 CI 定時匿名 bootstrap diff，漂移開 issue；`generatedAt`／`frontendSha` 在關於頁可見。此為長期維護人力承諾，需指派負責人。

### 3. XCTID 浮點語義、錯誤落地與法務

**3.1 cubic-bezier 求值未固定求解器參數（Blocker）。** REQ-XCT-005（L494）未固定求解器容差、迭代次數與半值捨入規則；Python `round()` 為 round-half-even，Go `math.Round` 為 half-away-from-zero，十進位半值點輸出不同即逐位元組不同。建議：自行實作 round-half-even、固定 epsilon 與迭代次數。REQ-XCT-003（L492）整數下標運算無浮點風險。

**3.2 ValueError 語義的規格缺口（codebase-details F11 修正）。** REQ-XCT-012（L501）僅規定「結構不符」拋 `ValueError`；「快取不存在」的行為規格未定——前版把兩者綁同一條 REQ 是過度引用。改寫為：規格缺口＋引擎自訂 `MaterialError`，與 `CredentialInvalidError` 機械可分。

**3.3 其他 XCTID 落地（design F10/F17/F18）。** 網路素材成功後是否回寫快取，規格沉默（REQ-AUTH-011，L411）——建議持久化（原子寫入＋`generatedAt`/來源）；註冊表寫檔同補原子性。測試 fixture 著作權：X 的 webpack 分片原文複製進 MIT repo 是著作權風險（§3.1-2、§3.2-1、REQ-GQL-033）——fixture 一律合成，L1 基線以「輸入＋期望輸出」記錄。時鐘防護：REQ-XCT-002/007 在系統時鐘早於 2023-05-01 時 `t` 為負、編碼語義未定——`t<0` 拋輸入錯誤提示檢查系統時間，不夾值。

**3.4 上游授權確認是散布前置條件（Blocker）。** §19.1-3（L1881）與 §16-14（L1823）：散布前必須先確認上游授權狀態；上游未觀察到 `/LICENSE`。確認只擋散布不擋本地提交。README 重寫時 Attribution 必須保留 OG-Open-Source 與 XApis 署名；公開前檢查清單含「Attribution 未被覆蓋」。

**3.5 交付清單補項（codebase-details F6/F14）。** 缺 `go.sum`（OG 標準 L66 lockfile MUST）與 `.gitignore`（§16-13，L1822——本地會產生素材快取與憑證檔，無 `.gitignore` 第一個 commit 就可能把敏感檔入歷史）；P5 出口補 issue 範本（§17，L1846，SHOULD）。CoC：`CODE_OF_CONDUCT.md` 檔案已存在，但 §17（L1837）要求的是 **README 內連結**——README 尚待重寫前不得收帳。`.github` 依 §17（L1832）必須以 submodule 掛載於 `/.github/`。

### 4. 衝突裁決與簡化權衡（zoom-out 主軸）

**4.1【已裁決：採用探測先行（需求方 2026-11-11 確認）】XCTID 先探測後實作。** zoom-out 主張：以 P0 一日 L2 探測實驗取代整條素材網路重建鏈（REQ-XCT-013、REQ-AUTH-011 網路路徑、REQ-GQL-030/031、REQ-AUTH-013）——帶完整標頭、刻意不帶 XCTID，對 Viewer 與 HomeTimeline 各形態各 ≥20 次、跨 2 會話，觀察 403/errors/226/344。依據：規格全文無「XCTID 不可缺席」的 MUST（REQ-WR-001 只約束 mutation，不在 v1）；§16-5（L1814）明言服務端校驗屬外部依賴。通過 → XCTID 整條延後 v2（保留標頭可選槽位）；不通過 → 進入降級形態（純算 001..011＋隨倉庫 `transaction_l1.json`，013 與網路路徑仍砍）。探測蓋不住的唯一風險是軟降級（226 上升），後果落在單一自有帳號。此提案同時消滅 §1.2 的冷啟動 Blocker 與 §1.5 的 jar 隔離複雜度，故列為最高優先裁決項。

**4.2【裁決提案】註冊表靜態快照 vs 重建鏈。** zoom-out：隨倉庫附僅含 HomeTimeline＋Viewer 的 `static/graphql.json`（§3.2-3/§13.3 合法；由登入態瀏覽器擷取，user.config 覆寫天然內含；登錄 `generatedAt`/`frontendSha`），重建降為 v2 手動工具，REQ-GQL-031 整條消失。與 design F12/F16（雙鏈合併、漂移監測）互斥：若探測實驗砍掉 XCTID，本提案成立且大幅簡化；若 XCTID 存活，雙鏈合併＋監測仍需要。

**4.3【否決 zoom-out 部分】七層收斂與 CookieStore。** zoom-out 主張單一公開套件 `internal/xapi`＋至多兩個內部子套件、砍掉 CookieStore 介面（單實作者 YAGNI；字串進出即可）。調解：接縫最小面照單吸收——`Options{Cookie, ProxyURL}`、`New`、`Session.Viewer()`、`Session.HomeTimeline(cursor,count,seen)`、`Session.Cookies()`、`Session.UpdateCookies(raw)`、`ErrAuthExpired/ErrMaterialUnavailable/ErrRateLimited/*GraphQLError`；但 CookieStore 窄介面保留（implementation-readiness 與 design 的 v2 會話失效語義需要它，且成本僅兩方法）；目錄樹裁決歸開放問題。

**4.4【吸收】v1 一律不跟隨重定向（zoom-out）。** 不跟隨是跨主機不洩漏的嚴格超集（REQ-TRN-040 目的），實作成本低於自寫逐跳迴路；完整語義隨 v2。HomeTimeline 單一 POST 形態：zoom-out 主張 v1 只實作 POST（REQ-GQL-005 理由欄「現行前端行為」；方法表 GET 列是上游實作反射），煙霧測試發現需要 GET 時補分支成本極低——與 implementation-readiness 的「以方法表為準」並列為 P0 二選一。

**4.5【吸收，附登錄】錯誤字串逐字保留降級為分類保留（zoom-out）。** MyGO 有自己的 UI 文案層，跨邊界契約是分類與 code；逐字斷言是儀式成本。調解：引擎內部（`Error()` 字串、REQ-AUTH-004 文案、REQ-GQL-007 格式）仍逐字，測試在引擎層斷言；跨 IPC 只帶 `kind`+`code`+訊息；一次偏離登錄。design F7 相關：12.2（L1590–1602）的 `gateway_busy` 等文案描述已移除的閘道——裁決存活清單（保留 `proxy_timeout`/`login_timeout`/`proxy_configuration` 與 REQ-TRN-018 文案），閘道專屬四條映射為直連語義並登錄。

**4.6【吸收】env 組態面出規（zoom-out）。** 引擎只認 Options 欄位（`WithShellAttempts`/`WithProfilePath` 等）；env 讀取屬 app 層，消滅「internal/xapi 讀環境變數」接縫污染。app 可保留讀 `X_PROXY` 等同名 env 的薄層。

**4.7【吸收】其餘砍留。** 砍（Python 限制反射清單，全 Go 後目標域不存在，連登錄都不需要）：REQ-TRN-017、REQ-AUTH-012（若素材鏈砍）、REQ-TRN-032、REQ-TRN-034、REQ-TRN-038/039、REQ-TRN-016、Result3。砍：REQ-TRN-006（SHOULD，測試直接建構）、fieldToggles 子集表 v1 只剩一列註解（優先序退化為「呼叫端顯式 → 註冊表」）。留：REQ-TRN-005（四行隨機數）、最小不變量集合——REQ-AUTH-001/014、REQ-GQL-002/003＋REQ-INV-07、REQ-HDR-008、REQ-GQL-006/007＋12.3 v1 子集（含 code 88 限流，12.3 L1611；239 隨 guest token 延後不列）、REQ-LOG-043、REQ-RD-012/013、REQ-GQL-013、REQ-INV-16（若素材鏈留）。v2 逃生通道只須定六件事：有序可選槽位標頭模型、六族名稱表為資料、緊湊序列化器同支撐 GET query string 與 mutation body 鍵序、XAuth 單一持有者＋UpdateCookies 合併、錯誤三分（素材/憑證/限流）、匿名與會話客戶端隔離。

**4.8【吸收】v1 錯誤碼子集（codebase-details F12 修正）。** 32/89/215/326 憑證失效、88/344/226 退避不重試、353 重組後重試一次。

### 5. 法務與開源交付

上游授權確認閘（§3.4）。`README.md` 重寫（含 Attribution 保留、Prerequisites 逐項版本、Usage 範例、CoC 連結）。MIT `/LICENSE`、`.github` submodule、`go.sum`、`.gitignore`、CHANGELOG、linter/CI、issue 範本。版本化選 SemVer。§19.2 第三方登錄補 Go 依賴（`bogdanfinn/tls-client`、MyGO 插件）授權欄位。若採 §4.1 探測通過路徑，`static/transaction_l1.json` 與 `static/graphql.json` 隨倉庫附檔（§13.3，L1698 明載「隨倉庫提供」——附檔不是偏離），登錄擷取來源與日期，不得當成不變式。

### 6. 可測性與驗收調整

**6.1 驗收策略（implementation-readiness＋design OQ3）。** v1 簽收以 L2（§14.1 L2，L1718）＋結構性質測試為準；L1 向量為需求方提供後補的**阻塞交付物**（XCTID 簽收條件）。向量欄位必須含 METHOD/PATH（REQ-XCT-007、REQ-HDR-012），並覆蓋 HomeTimeline GET/POST 兩形態。注入 API：`xctid.NewGenerator(material, WithClock, WithRand)`；正式路徑 `time.Now`＋`crypto/rand`。§14.2（L1722–1728）映射為 `go test -race ./...`；§14.4 的 ct0 一致性抽檢（L1751）納入 P1/P2 測試。

**6.2 規格即 fixture 的部分。** Cookie 解析（REQ-AUTH-003/004/006/007/008/014）、標頭族順序與小寫（REQ-HDR-001..013）、緊湊序列化與鍵序（REQ-GQL-002/003、REQ-INV-07）可從規格直接造測試；fixture 一律合成（§3.3）。Go 陷阱：`encoding/json` 須 `SetEscapeHTML(false)`＋有序結構；`url.Values.Encode()` 禁用（§1.6）；游標深度遍歷須有序解析或串流遍歷（REQ-RD-012）。

**6.3 煙霧與端到端前置。** 真實 Cookie 與代理由需求方提供，憑證不得入 repo（REQ-INV-01、§16-13）。「Viewer 回傳身分」「HomeTimeline 回傳貼文」的完成判準與環境條件在 P0 寫死。

**6.4 代理解析。** §13.4（L1704）含五層（`X_PROXY` → 標準環境變數 → Windows 註冊表 → 本機埠探測 → 直連）；v1 採子集。implementation-readiness 建議最終順序：`X_PROXY` → `HTTPS_PROXY`/`ALL_PROXY` → MyGO netproxy → 失敗關閉；移除註冊表層與埠探測層；前置驗證 netproxy 已涵蓋註冊表職能。design F8：代理解析時點未定義，VPN 切換後停在死代理——啟動解析＋顯式重新偵測，偵測結果先短逾時預檢；移除埠探測層以「安全理由」登錄。此偏離（REQ-INV-02 常開＋層數裁減）須在決策記錄與 README 登錄。

### 7. 開工順序與階段（implementation-readiness 主軸）

commit 粒度：一個需求域一個 commit、Conventional Commits、退出條件 `go test -race` 綠。開工順序 5 commit：`docs:`（規格、簡報、重寫 README、LICENSE、CoC、`.github` submodule、`.gitignore`、`docs/DECISIONS.md` 含裁決回寫）→ `chore:`（骨架＋CI＋import 禁令）→ `feat:`（transport）→ `feat:`（headers, auth）→ `feat:`（xctid，若存活）。版本釘選：`go.mod` 寫 `go 1.27.3`；tls-client 釘最新穩定（先驗證含 chrome_150）；MyGO 釘 tag；版本補入 §19.2 登錄。接縫文件與 `docs/DECISIONS.md` 為具名交付物（comprehension B）。

## Goals vs. Non-Goals

* **In-Scope Goals:**
  * 可建置的單一 Go module：`internal/xapi` 套件樹（分層結構依 §4.3 裁決）＋ MyGO app 骨架（login／home 兩頁，typed IPC）。
  * Cookie 匯入登入：REQ-AUTH-001/002/003/004/006/007/014、REQ-WEB-066 語義、REQ-LOG-041..043 的驗證與失效分類（含精確錯誤文案於引擎層）。
  * HomeTimeline 讀取與翻頁：REQ-RD-001..004、REQ-RD-012/013、REQ-GQL-001..007、REQ-GQL-005（形態依 §4.4 裁決）、REQ-WEB-104 去促銷與 work_id 去重（app 層）。
  * XCTID：兩條路徑二選一——探測通過則僅留標頭可選槽位；探測不通過則 REQ-XCT-001..012（含 011 注入）＋隨倉庫素材，REQ-XCT-013 延後。
  * 註冊表：隨倉庫 `static/graphql.json` 快照（§4.2 裁決通過時）；REQ-GQL-010..016、REQ-GQL-020..025 的執行期語義。
  * 傳輸與指紋：REQ-TRN-001..005、REQ-TRN-010..015、REQ-TRN-018..020（TRN-011 偏離登錄；TRN-040 以「不跟隨」超集滿足；TRN-006 以測試輔助存在）；REQ-INV-01/02/07/09/13；OnRequest/OnResponse 觀測掛鉤。
  * 開源與工程交付：MIT `/LICENSE`、專案 README（重寫）、CoC 連結、`.github` submodule、`go.sum`、`.gitignore`、CHANGELOG、git init 與 5-commit 序列、`docs/DECISIONS.md`、上游授權確認閘。

* **Out-of-Scope Non-Goals:**
  * 帳密登入 jetfuel 流程（7.7.1）、行動端路徑（7.7.2）、Castle token（7.6）、Node 回放。
  * 寫入（7.9）、媒體（7.10）、文章（7.11）、私信（7.12）、資料處理匯出（7.13）、CLI（7.14/7.15）。
  * Web 控制台（第 8 章）、前端工作台（第 9 章）、Go 閘道與部署（第 10 章、REQ-TRN-030..041、GW 域、DEP 域）。
  * AI 與資訊來源契約（6.3/6.4）、MySQL 結構（6.5）、養護與配額（8.6/8.11）。
  * guest token（REQ-AUTH-009/010，留方法位）、本地 ct0 產生（REQ-AUTH-005）、twid 解析（REQ-AUTH-008）、連結/使用者名稱解析器（REQ-RD-010/011）、推文計重（REQ-DAT-001..005）。
  * 註冊表重建鏈（REQ-GQL-030..038，若 §4.2 裁決通過則降為 v2 手動工具）、REQ-XCT-013 素材網路抓取（兩條路徑下皆延後）。
  * Python 限制反射清單（§4.7 砍除項，連登錄都不需要）。

## Alternatives Weighed

1. **【否決】一開始就拆兩個 Go module（引擎 SDK＋app）**——跨 module 本機開發與版本同步成本高於 `internal/` 邊界；「機械式抽離」可用「`internal/xapi` 不得 import MyGO」的靜態約束加 CI 檢查達成。
2. **【否決】保留閘道子程序（第 10 章、REQ-TRN-030..041）**——閘道存在理由是隔離 Python TLS 限制；全 Go 後成為部署與除錯負擔。
3. **【否決】帳密登入先行**——jetfuel 流程依賴 Castle profile 與挑戰處理（REQ-LOG-001..022），是 v1 面積的數倍。
4. **【否決】cgo zlib 後端以提前支援 Castle**——違反零 cgo 約束，且 Castle 不在 v1。
5. **【否決】X 流量走 MyGO Fetch 插件**——違反 REQ-TRN-010 單一出口，繞過標頭補齊、sec-fetch 推導與指紋。
6. **【否決】代理缺口時直連回退**——需求方明確要求失敗關閉（REQ-INV-02 語義常開）；代價是登錄此偏離（§6.4）。
7. **【否決】CookieStore 窄介面（zoom-out：字串進出即可）**——v2 會話失效語義（12.3 code 32/89）需要持久化掛點，且成本僅兩方法；接縫最小面其餘照單吸收。
8. **【否決】錯誤字串全面逐字保留（前版）**——引擎層保留逐字（REQ-AUTH-004、REQ-GQL-007），跨 app 邊界降為分類＋code；一次偏離登錄（§4.5）。
9. **【已定案 2026-11-11：採用】XCTID 先探測後實作（zoom-out）**——P0 執行一日 L2 探測實驗裁決 XCTID 是否 v1 必要（§4.1）；通過則整條延後 v2，消滅三個 Blocker。
10. **【懸置】註冊表靜態快照 vs 重建鏈（zoom-out vs design）**——依 §4.1 探測結果二選一（§4.2）。
11. **【懸置】七層套件 vs 收斂套件**——接縫最小面照吸收，CookieStore 留；目錄樹歸開放問題（§4.3）。
12. **【懸置】重定向自寫逐跳迴路 vs v1 不跟隨**——傾向不跟隨（嚴格超集），完整語義隨 v2（§2.5）。
13. **【懸置】HomeTimeline 雙形態 vs POST 單形態**——implementation-readiness（方法表為準）與 zoom-out（POST 單形態）並列，P0 二選一（§1.1、§4.4）。
14. **【懸置】env 組態面去留**——引擎只認 Options、env 讀取歸 app 層；app 是否保留同名 env 薄層由需求方定（§4.6）。
15. **【懸置】WebView 優先或原生 UI 優先**——屬 app 側決策，不影響引擎邊界；typed IPC 的 Bind 面向兩者相容。

## Key Decisions & Staging for Next Steps

**已定案：**
* 倉庫：單一 repo／單一 module；`internal/xapi` 不得 import MyGO；CI 檢查 import 禁令。
* 傳輸：行程內 `bogdanfinn/tls-client`（chrome_150）；TRN-011 偏離登錄；uTLS 單一 TLS 棧。
* 生命週期：XAuth 不可變 snapshot＋原子替換、singleflight、`-race` 進 CI；Cookie write-through；`Close()` 順序 cancel → 收斂 → flush → 關 SQLite。
* jar 隔離：匿名與會話客戶端分離（若素材鏈存活）；匿名抓取後 snapshot 不變測試。
* 錯誤：`xerrors` 型別集＋wire schema（`kind`/`code`/`retryable`/`credential_impacting`）；`MaterialError` 與 `CredentialInvalidError` 機械可分；v1 錯誤碼子集含 88（§4.8）。
* 素材：網路成功後回寫快取（原子寫入＋`generatedAt`/來源）；shell 雙鏈合併為單一 bootstrap 管線（若素材鏈存活）。
* 範圍策略（2026-11-11 裁決）：XCTID 探測先行——P0 執行一日 L2 探測實驗定存廢；通過 → 整條延後 v2（保留標頭可選槽位）；不通過 → 降級形態（純算 REQ-XCT-001..011＋隨倉庫素材，REQ-XCT-013 與網路鏈仍砍）。
* REQ-AUTH-013（強制重建素材入口）入 v1 引擎 API 與 app UI；連續 N 次無 data 失敗提示重建，不自動清憑證。
* 指紋：不持久化，每次啟動重抽（REQ-TRN-001 原樣滿足，就地關閉原開放問題 5）；常數集中單檔標註上游節奏依賴。
* 重定向：v1 一律不跟隨（§4.4 傾向）。
* 測試：L2＋結構性質先行；`WithClock`/`WithRand` 注入；fixture 一律合成；`t<0` 拋輸入錯誤。
* 工程：`docs/DECISIONS.md` 具名交付物；OnRequest/OnResponse 觀測掛鉤；`url.Values.Encode()` 禁用；`SetEscapeHTML(false)`。

**階段劃分：**
* **P0 裁決與 spike**——入口：本簡報經需求方審閱（已開始：2026-11-11 裁決採用 XCTID 探測先行）。出口：開放問題 2–10 全數裁決並回寫 `docs/DECISIONS.md`；tls-client 半天 spike 出能力對照表（chrome_150 存在性、fhttp 插入序與原樣大小寫、per-client proxy）；XCTID 一日 L2 探測實驗完成，依結果定 §4.1 存廢與 §4.2 註冊表路線。負責：需求方（裁決、提供探測用憑證與代理）＋實作者（spike 與探測執行）。
* **P1 引擎核心**——入口：P0 完成。出口：REQ-TRN-001..005/010..015/018..020（排除 011/016，偏離已登錄）與 REQ-HDR-001..013（GraphQL 兩族）單元測試全綠；匿名 jar 隔離測試；ct0 一致性抽檢。負責：實作者。
* **P2 XCTID（依裁決）**——入口：P1 完成＋開放問題 3 裁決。出口：探測通過則槽位測試綠；不通過則純算結構性質測試全綠＋（若取得向量）L1 對拍全綠。負責：實作者（向量擷取安排歸需求方）。
* **P3 GraphQL 讀取鏈**——入口：P2 完成。出口：緊湊序列化、Viewer、HomeTimeline（依裁決形態）、游標深度遍歷、REQ-LOG-043 失效分類的離線單元測試全綠。負責：實作者。
* **P4 app 骨架與整合**——入口：P3 完成。出口：MyGO 兩頁、typed IPC、SQLite 儲存、啟動狀態機、代理解析；真實 Cookie＋代理煙霧測試（L2 驗收）與端到端演示。負責：實作者＋需求方提供憑證與代理。
* **P5 開源交付（收縮版）**——入口：P4 完成。出口：上游授權確認閘通過、README 重寫（含 CoC 連結與 Attribution）、`.github` submodule、`go.sum`/`.gitignore`/CHANGELOG；issue 範本（SHOULD）。負責：需求方（授權確認）＋實作者（文件）。

**待需求方裁決（合併去重後單一清單，每題附建議預設值＋依據＋裁決責任人）：**
1. **【已定案 2026-11-11】XCTID 探測實驗**：需求方已裁決採用探測先行路線——P0 執行一日 L2 探測實驗（完整標頭、刻意不帶 XCTID；Viewer 與 HomeTimeline 各形態各 ≥20 次、跨 2 會話），以結果定存廢（通過則整條延後 v2，消滅三個 Blocker）。執行：實作者；憑證與代理：需求方提供；裁決記錄回寫 `docs/DECISIONS.md`。
2. **HomeTimeline 形態**：方法表為準（GET/POST 雙形態）vs POST 單形態。建議預設：方法表為準，REQ-GQL-005 解讀為「帶 seenTweetIds 的 query 型必須 POST」，三個落地點寫死（線路、參數順序、XCTID METHOD）。依據：REQ-RD-001/004/014、REQ-XCT-007。責任人：需求方。
3. **素材策略**：隨倉庫附初始檔＋解析順序（呼叫端 option → `XCTID_PROFILE_FILE` → 使用者資料目錄快取 → 隨倉庫初始檔）＋失敗降級。建議預設：如左；§13.3 L1698「隨倉庫提供」已給傾向答案。依據：REQ-XCT-012、REQ-AUTH-011。責任人：需求方。
4. **L1 基線向量**：需求方提供瀏覽器擷取向量（欄位含 METHOD/PATH、兩形態）作為 XCTID 簽收阻塞交付物，或僅 L2 簽收。建議預設：L2 先行，向量後補。依據：§14.1、REQ-XCT-011。責任人：需求方（擷取安排）。
5. **上游授權確認**：結論與時程；確認前 repo 私有。建議預設：私有至確認完成。依據：§19.1-3、§16-14。責任人：需求方。
6. **代理解析層**：移除 Windows 註冊表層與埠探測層的取捨（前版漏計註冊表層，已補）。建議預設：`X_PROXY` → 標準 env → netproxy → 失敗關閉；netproxy 職能驗證先行。依據：§13.4、REQ-INV-02、REQ-TRN-018。責任人：需求方＋實作者（netproxy 驗證）。
7. **Cookie 儲存選型**：MyGO secrets API vs SQLite 明文＋OS 隔離 vs OS 憑證庫（DPAPI）。建議預設：MyGO secrets API 若有則用，否則 SQLite 明文＋OS 隔離＋README 登錄威脅模型。依據：REQ-INV-01 落點在輸出脫敏。責任人：實作者（能力確認）＋需求方（接受度）。
8. **目錄樹與套件粒度**：七層 vs 收斂（§4.3）。建議預設：implementation-readiness 目錄樹（七層）起手，接縫面照 zoom-out 最小面；P0 依 spike 結果收斂。依據：REQ-AUTH-001、§3.2-2。責任人：實作者。
9. **error→UI 文案層**：app 端各 `kind` 的中文文案由誰定稿。建議預設：需求方定稿，引擎只保證 `kind`/`code` 穩定。依據：wire schema（§2.2）。責任人：需求方。
10. **素材漂移維運指派**：CI 匿名 bootstrap diff、app 一鍵重建、關於頁 `generatedAt` 展示的負責人。建議預設：維護者 CI＋app 手動入口。依據：§16-1、design F16。責任人：需求方（指派）。
