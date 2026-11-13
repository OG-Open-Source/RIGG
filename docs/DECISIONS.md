# 決策記錄（DECISIONS）

> 本檔登錄「MyGO X 引擎 v1」對規格（`XAPIS_BEHAVIOR_SPEC.md`，唯一行為參考）的
> 解讀與偏離，作為 `docs/designs/x-engine-v1.md` §3.8 所述「開工時回寫」的落點。
> 本檔不改動凍結的規格本身；每筆裁決一節，含「依據」與「偏離登錄」欄。
> 基準日：2026-11-11（Task 0，P0 前置裁決）。

## 已定案裁決

### D1 OQ3：HomeTimeline 以 7.8.2 方法表為準

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

*   **裁決**：七層套件樹
    `X/internal/xapi/{fingerprint,transport,auth,headers,graphql,xctid,api}` ＋
    `xerrors`。
*   **依據**：設計文件 §3.2（套件組成與職責表，OQ5＝七層）；Alternative J
    （收斂單套件）被否決的理由——規格域（TRN/AUTH/HDR/GQL/XCT）與程式碼
    一一對應。xctid 套件在探測通過路徑下整個不建。
*   **偏離登錄**：無（目錄樹屬本專案自由度，非規格條目）。

### D3 TRN-011 偏離：傳輸層以 bogdanfinn/tls-client 承接

*   **裁決**：curl_cffi 指名改以 `bogdanfinn/tls-client`（chrome_150 profile）
    承接，uTLS 單一 TLS 棧。
*   **依據**：設計文件「提出的解法」（純 Go、零 cgo、符合 MyGO 單一執行檔
    屬性）與 G4（TRN-011 偏離登錄即本條）；能力前提由本檔〈spike 結論〉節
    實證（chrome_150 存在、標頭序、重定向控制、per-client proxy 四點全過）。
*   **偏離登錄**：規格 TRN-011 指名 curl_cffi（Python 生態）——Go 實作改以
    bogdanfinn/tls-client v1.18.0 承接同等職責（Chrome TLS 指紋模擬）。
    釘選版本與證據見〈spike 結論〉節。

### D4 §2-6 錯誤字串偏離

*   **裁決**：規格「錯誤字串逐字保留」、REQ-AUTH-004 精確文案、REQ-GQL-007
    訊息格式**不適用**；行為層僅保留錯誤分類（kind/code 機械可分，
    REQ-LOG-042/043 不變）。
*   **依據**：設計文件 Alternative G 被否決的理由——本專案為 clean-room
    重寫，不繼承原專案的使用者可見契約；文案由 app 層決定（OQ10）。
*   **偏離登錄**：對規格 §2-6 的「逐字保留」約束、REQ-AUTH-004 的精確文案、
    REQ-GQL-007 的訊息格式均登錄為偏離。缺 `ct0` 仍拋錯（REQ-AUTH-004 的
    行為面），僅文案不受約束。

### D5 12.2 錯誤文案存活清單

*   **裁決**：保留 `proxy_timeout` / `login_timeout` / `proxy_configuration`；
    閘道專屬四條（`gateway_auth` / `gateway_busy` / `gateway_unavailable` /
    `gateway_error`）映射為直連語義新文案。
*   **依據**：設計文件 §3.2 `xerrors` 列（閘道錯誤文案存活清單）與 §3.8
    （錯誤文案存活清單屬回寫本檔的裁決之一）。
*   **偏離登錄**：閘道專屬四條原文案隨 D8（閘道移除）一併失準，映射為
    直連語義新文案後，code 值維持機械可分（REQ-LOG-042/043 不變）。

### D6 代理解析裁減

*   **裁決**：`X_PROXY` → `HTTPS_PROXY`/`ALL_PROXY` → MyGO netproxy → 失敗
    關閉；移除 Windows 註冊表層與本機埠探測層（後者以安全理由）。
*   **依據**：設計文件 §3.7（代理解析鏈）與 Open Questions OQ6（建議預設值
    即此鏈；前置「驗證 netproxy 已涵蓋系統代理職能」屬 app 層整合工作）。
*   **偏離登錄**：規格中的 Windows 註冊表代理層與本機埠探測層不予實作
    （本機埠探測層以安全理由移除）。失敗關閉維持 REQ-INV-02；直連回退仍為
    Alternative F 否決狀態，最終立場待 OQ12（見〈探測實驗結論〉直連對照）。

### D7 v1 不跟隨重定向

*   **裁決**：v1 不跟隨重定向（REQ-TRN-040 以嚴格超集滿足，完整語義隨 v2）。
*   **依據**：設計文件 §3.2 transport 列（「重定向一律不跟隨」）與 §3.4 第 5
    步；G4（TRN-040 以「v1 一律不跟隨重定向」嚴格超集滿足）。實作前提
    `WithNotFollowRedirects` 的存在與行為由〈spike 結論〉第 3 點實證。
*   **偏離登錄**：無（嚴格超集：不跟隨是規格重定向語義的子集行為，v2 再補
    完整語義）。

### D8 閘道移除

*   **裁決**：第 10 章與 REQ-TRN-030..041 不適用。
*   **依據**：設計文件 Alternative B 被否決的理由——閘道存在理由是隔離
    Python 的 TLS 限制；全 Go 後只剩程序間 RPC 的部署與除錯負擔。
*   **偏離登錄**：規格第 10 章（Go TLS 閘道子程序）與 REQ-TRN-030..041
    （閘道客戶端契約）整域登錄為不適用；相關錯誤文案的處置見 D5。

### D9 transport 無 cookie jar；Cookie 標頭由呼叫端注入

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

### D10 代理改為可選：正常用途直連（需求方 2026-11-13 裁決）

* **裁決**：桌面 app（正常用途、自有帳號）不再要求代理——`ResolveProxy`
  三層皆無時回 `("", nil)` 直連；transport 以 `Options.AllowDirect: true`
  於 app 層顯式選入（引擎預設仍 fail-closed）。系統偵測命中但預檢失敗 →
  視同未偵測到（落入直連）；使用者顯式配置（`X_PROXY` 等）仍原樣採用。
* **依據**：OQ12 探測實證（直連可達且被正常接受、0×226/344，見探測節）＋
  需求方裁決「設計給正常用途，代理應該用不到」。REQ-INV-02 的
  fail-closed 原契約針對多帳號控制台的強制每帳號代理部署；本桌面 app
  單一自有帳號情境予以修訂。
* **偏離登錄**：REQ-INV-02 於本 app 之適用修訂為「代理可選、直連為基線、
  顯式配置的代理仍被採用」。成本：直連暴露真實 IP 且無指紋代理保護——
  若 X 日後對該出口 IP 形態收緊風控（226/344 上升），重新啟用代理即可
  （transport 支援不變）。
* **配套**：註冊表快照經 mygo.json `resources` 出貨至執行檔旁（MyGO 會
  攤平路徑：出貨為 exe 旁的 `graphql.json`；`registryPath()` 優先 exe
  目錄、退回開發期的 `static/graphql.json`）——修正 L2 後首次實機
  登入的兩個失敗根因（快照未出貨＋代理 fail-closed）。
