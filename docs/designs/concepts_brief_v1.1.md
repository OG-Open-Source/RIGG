# MyGO X 引擎 v1.1 構想簡報

* **Author:** 控制器合成（合成者 subagent 兩度因平台基礎設施故障失敗，改由控制器直接合成）；審查視角輸入：design／codebase-details／zoom-out／comprehension／implementation-readiness／structural 六份全新上下文審查（全文彙編於本輪工作檔 `reviews.md`）
* **Date:** 2026-11-14
* **Status:** Proposed——開放問題已全數裁決（2026-11-14，見 Key Decisions「已裁決記錄」）；待需求方審閱全文後升 Design，再入計畫

> **審查狀態（元資料）：** 六視角審查全部 completed；需求方已裁決：切換即重驗 Viewer（後經墓碑裁決修訂為重驗＋TTL 快取）、GIF 走 mp4 variant、oxfmt 移除、帳號上限 5、墓碑機制＋lazy 憑證預熱、頂列佈局＋搜尋入範圍、LRU 先量測再定案、媒體 CDN 查證（§1.7）。凡與審查者推測衝突之處，以兩份實測附錄為準：MyGO API 一手查證（go doc，`mygo-api-verification.md`）與工具鏈實測（`formatting-toolchain-findings.md`、`verification-addendum.md`）。本簡報標註證據出處（檔案:行 或 go doc 輸出行號）；未經查證的項目明確標記【未查證】。

## Overview

v1 已通過 L2 驗收：Viewer 3/3、首頁 28 帖、10 頁分頁累計 298 帖零跨頁重複、226/344 出現 0 次、RAM 閒置 11.3 MiB／峰值 30 MiB（`docs/DECISIONS.md` 驗收節）。v1.1 在此基礎上擴充五域：

1. **多帳號（模型 A＋逃生通道）**——單活躍會話，切換＝寫回當前 cookie 行 → 載入目標帳號 → 重建 session 並重驗 Viewer（TTL 有效性快取節流）；帳號清單存 MyGO Secrets（每帳號一 key，UserID 識別）＋當前帳號指標（存索引 secret）；**上限 5 帳號**；啟動 lazy 憑證預熱＋切走即墓碑。
2. **媒體**——時間線解析 `extended_entities`（REQ-DAT-015/016）→ 列表非同步縮圖 → 貼文展開檢視大圖＋`ui/video` 內嵌播放（GIF 走 mp4 variant，需求方 2026-11-14 裁決）。
3. **雙時間線**——For You（`HomeTimeline`）與 Following（`HomeLatestTimeline`）雙 tab；後者為**規格外操作且未經探測**，開工前置 P0' 探測輪。
4. **搜尋（需求方 2026-11-14 新增）**——頂列右上角搜尋入口 → `SearchTimeline` 結果頁（規格 REQ-RD-014 已覆蓋行為，queryId 待探測）。
5. **UI/UX**——頂列骨架（左上帳號切換、右上搜尋；無下方 bar，Desktop only）；MyGO 內建元件優先（已盤點，見 §2）；水平響應式（768/1024/1200 斷點）。

**既定方針**（需求方 2026-11-13/14）：web 暫緩發布但保持可編譯作為測試載體；格式化鏈＝`cd X` → `gofumpt -w -extra .` → `golangci-lint run`（oxfmt 已裁決移除）；RAM 行程峰值 <80 MiB 為驗收線。

**輸入清單**：`XAPIS_BEHAVIOR_SPEC.md`（凍結規格，唯一行為權威）；`docs/designs/x-engine-v1.md`（v1 設計）；`docs/DECISIONS.md`（D1–D10＋探測＋L2＋驗收）；v1 全部 app 層與引擎程式碼；六份 v1.1 審查報告；MyGO v0.9.0 API 查證附錄；格式化工具鏈實測。

## Analysis

### 0. 迷你詞彙表（v1.1 新增）

* **帳號（Account）**：一份持久化的憑證與身分組合——`{user_id, screen_name, name, cookie_line}`，存於一個 secret；`user_id` 由 Viewer 回應取得，作為唯一識別與 key 來源。
* **索引 secret**：因 MyGO Secrets **無 List/Enumerate API**（core.txt:3736-3790），帳號清單與當前帳號指標必須存於一個固定 key 的 secret（JSON），是帳號清單的「真源」。
* **世代（generation）**：一次會話切換的邊界。切換前的 in-flight 請求與其 publish 屬於舊世代，切換後一律作廢——防止跨帳號資料污染。
* **HomeLatestTimeline**：X 的「Following」時間線 GraphQL 操作。queryId `IVA5BUbeYPTrpYekQH7jiA` 已於 P0 探測「附帶」萃取（DECISIONS 探測節），但**從未發過請求**、不在凍結規格 §7.8 內。
* **縮圖 LRU**：以「解碼後位元圖位元組」計量的記憶體快取上限（建議 ≤24 MiB，先量測再定案），滾出即逐出；配 `Bitmap.ReleasePixels()`（ui.txt:784-795）釋放 CPU 像素。
* **展開檢視**：列表中點開一則貼文後的完整檢視頁（`ui.Route`），大圖＋`ui/video` 播放器。
* **cd X**：格式化與 lint 的必要工作目錄（module 根）；repo 根無 go.mod。

### 1. 風險優先（先風險後優點）

**1.1 app 層無帳號/世代邊界（Blocker，design F1）。** `main.go:50-57` 的 posts/cursor/seen/homeBusy/homeErr/list 是**單份**狀態；`view_home.go:75` 在背景起跑前捕捉 `sess, cursor, seen`、`:88` 直接 `mergePosts` 進同一份狀態。多帳號下切換會讓舊會話的 in-flight Home 回應經 publish 落入新畫面：跨帳號貼文污染、cursor 覆寫，最嚴重的是 `seen`（seenTweetIds）混入他帳 work_id 後**以新帳號身分隨 POST 送出**——資料污染＋異常行為輪廓（風控面）。Task 6 的 XAuth snapshot 模型只隔離單一 Auth 實例內的 read-after-write，不解決兩個 Auth 實例之間的 app 狀態歸屬。**必要設計**：切換即 cancel 舊會話的 context 子樹；publish 攜世代號，過期即棄；或每帳號獨立的 feed 容器。此項決定 `app` struct 形狀，須在設計文件定形，不能留給實作期。

**1.2 write-through 落點：單 key 互覆（Blocker，design F2＋zoom-out #2）。** `cookies_store.go:21` 用單一 key `x-cookie-line`；`session.go:50` 的 `engineStore` 是全域單例；`api/send.go` 的 Set-Cookie write-through 與 `main.go:159-161` 的 quit flush 都寫同一鍵。多帳號下，活躍帳號的 ct0 輪替會把**別的帳號**的 cookie 蓋掉。`NewCookieStore(service, key)` 本身已參數化（cookies_store.go:48），改造是機械式的：**廢除 `engineStore` 全域**，`newSession` 內以帳號派生 key 建每帳號 store 並接線到該會話的 Auth；quit flush 目標跟隨活躍帳號。此項與 schema 同一 commit 落地，晚於切換上線即有污染窗口。

**1.3 切換失敗中間態與 transport 洩漏（Blocker，design F3＋實測）。** 已實測確認：`transport.Client.Close()` **存在**（`transport/client.go:168-169`），但 app 層**從未呼叫**——全 repo 非測試的 `.Close()` 呼叫點僅 `proxy.go:88`（探測連線）與 `app_state.go:135-136`（關閉的是 store 非 transport）。v1 單會話是一次性洩漏（量測未顯）；**模型 A 每次切換洩漏一個 tls-client**，累積侵蝕 80 MiB。切換序列必須定義收斂態：

```
Flush 當前帳號 cookie → 索引 current 寫目標帳號 → newSessionFor(目標)
→ Viewer 重驗（需求方案甲裁決）→ 成功：替換 sess/auth/store，關閉舊 transport
→ 失敗：標記該帳號失效（不清除其 secret）、索引 current 回寫原帳號、原會話不動
```

順序敏感點：舊 transport 只能在**新會話驗證成功後**關閉（失敗時舊會話仍須可用）；「索引先寫、會話後換」提供崩潰安全。`xSession` 介面須補 `Close() error`。

**1.4 HomeLatestTimeline：規格外、未探測（Blocker，design F5＋codebase #2＋comprehension 3.1 共識）。** 規格 grep 零命中——整條操作在凍結規格外；queryId 是探測「附帶」萃取（20XX 年前端 chunk map 重建），可用性實證（60/60）只涵蓋 Viewer/HomeTimeline；`timeline.go` 的 variables 為 HomeTimeline 硬編碼。**開工前置 P0' 探測輪**：L2 式腳本，空 features 與宣告 features 兩臂、GET 與 POST 兩形態、每臂 ≥20 次（D1 判準），必要時從前端 chunk 抽 features 回填快照；結論回寫 DECISIONS 並登錄「本操作在規格外、以探測＋快照為契約、隨 X 前端發版漂移」。

**1.5 MyGO Secrets 硬約束（三項，實測）。** (a) **無 List/Enumerate**——索引 secret 是唯一可行方案，不是偏好；(b) **無 app 層 preferences API**（僅 `Theme.Preferences()` 的 accent/無障礙）——當前帳號指標必須存 secret；(c) **`MaxSecretSize = 2560`**（core.txt:150-154）——實測單一 cookie 行 216 bytes、含中繼資料 JSON 303 bytes，餘裕充足；但使用者貼整份 Cookie header 可能逼近上限，設計需含「大值降級：加密落檔（`PathUserData`）＋Secrets 存金鑰」（MyGO 官方建議路徑）。

**1.6 媒體 RAM 預算矛盾（Blocker，design F6＋implementation-readiness）。** 「快取上限守 80 MiB」與「行程峰值 <80 MiB」自相矛盾（L2 實測峰值 30 MiB）。合成案：**解碼位元圖 LRU 上限 ≤24 MiB（先量測再定案）＋行程峰值 <80 MiB 為驗收線**；`Bitmap.ReleasePixels()` 為釋放槓桿（GPU 上傳後丟 CPU 像素，主執行緒呼叫）；縮圖一律用 `?name=small/medium`（`name=orig` 單張 RGBA 解碼約 8 MB）；量測場景新增「含媒體 100 帖」與「播放影片中」。不做磁碟快取。

**1.7 媒體下載出口：REQ-TRN-010 單一出口偏離（design F7）＋已驗證的低風險。** 縮圖/影片在 `pbs.twimg.com`/`video.twimg.com`。合成裁決：**獨立輕量 `net/http` 客戶端載公開 CDN**，沿用代理解析結果（D10 直連語義），單檔與總量上限寫死；DECISIONS 登錄偏離——媒體 CDN ≠ GraphQL API，REQ-TRN-010 的單一出口指 X API 流量。錯誤映射：媒體 403/404 不落六型別語義，app 層以可重試佔位呈現。**需求方「搜尋並驗證」裁決（2026-11-14）已執行**：官方 API 限流（`x-rate-limit-*`、OAuth 權杖、docs.x.com 體系）管不到媒體 CDN——`pbs.twimg.com` 實證免認證、無簽章、無過期參數（`?format=jpg&name=small/medium/large/orig` 全 200；`format=` 必填、`name=` 選填；錯誤組合回 404 非 403）；avatar 屬無過期參數的永久 URL；僅保護/已刪/受限內容回 403（本應如此）。mp4 variant（video.twimg.com）同為直接 URL。風險結論：媒體直載不需簽章或憑證，攔截情境限縮於保護性內容；降級退路（解析＋計數＋外開）保留為應急但風險下修。

**1.8 既有事實基礎的裂縫（codebase-details 稽核）。** (a) `auth.WithProxies` 存在但**無任何消費端**（auth.go:44-51 存入、零讀取點）——「帳號級代理逃生通道」目前是死選項，介面設計不得假設接上即生效；(b) DECISIONS「web 建置已實證」是**虛假引用**（web 驗收記錄不存在，命令只出現在 README Usage）——需撤回或補實證；(c) `Post` 結構無媒體欄位（timeline.go:23-28）——媒體萃取是新增欄位而非改造；(d) registry 快照僅 Viewer＋HomeTimeline 兩操作（static/graphql.json），新增 HomeLatestTimeline 需改快照與 `registryPath()` 出貨機制（後者已就緒：mygo.json `resources` 攤平出貨至 exe 旁，session.go 已兩級查找）。

**1.9 格式化工具鏈（需求方 2026-11-14 裁決）。** `oxfmt` v0.68.0 是 Oxc 的 JS/TS 格式化器，**不支援 Go**；repo 根執行會改寫凍結規格書、`X/static/graphql.json` 與整棵 `vendor/XApis/`。裁決：移除。校正後鏈：`cd X` → `gofumpt -w -extra .`（現況 6 檔待格式化）→ `golangci-lint run`（v2 預設：errcheck/govet/ineffassign/staticcheck/unused；**必須在 module 內執行**，repo 根 exit 1）。`vendor/XApis/gateway/` 是獨立 Go module（2 個 .go 檔），格式化與 lint 皆須排除。

### 2. MyGO v0.9.0 API 一手查證（go doc 實測）

六位審查者的共同疑慮已由實測解除；以下為設計的事實基礎（行號＝go doc 輸出檔行號，存 `mygo-api-verification.md`）。

**2.1 圖片管線。** `ui` **沒有**內建網路圖片載入：`ui.Image(c, src ImageSource)` 只吃已解碼的 `*Bitmap`/`*SVG`/`*native.Texture`（ui.txt:1933, 5504-5508）。但 `DecodeImage(data []byte)` 的文件明寫是為「downloaded」而設（ui.txt:5510-5513）；`DecodeBitmap` 支援 PNG/JPEG/GIF(首格)/WebP/BMP（ui.txt:766-769）。**自寫下載＋解碼是唯一路徑，但不需自寫快取演算法**——「可見列才載＋LRU 逐出＋ReleasePixels」即足。`Bitmap.ReleasePixels()` 是官方 RAM 槓桿：丟 CPU 像素、留 GPU 副本，需要時自保留的 data 重新解碼（ui.txt:784-795）。

**2.2 影片與 GIF。** `package ui/video` 存在：AVFoundation(macOS)/Media Foundation(Windows)/GStreamer(Linux)/瀏覽器 video element（video.txt:1-8）。`video.OpenURL(url)` 支援 `http:`/`https:`/HLS（video.txt:46-49）；**`VideoPlayer.Close()` 存在**（ui.txt:9439）；播放控制齊備（Play/Pause/Seek/SetVolume/SetMuted/SetLoop，ui.txt:9435-9511）。關鍵語義：「The video moves on only while the view builds it and some of the window shows」（ui.txt:2571-2577）——**列表虛擬化天然停播**，滾出畫面不再前進。`ui.NewGIFDecoder` 純 Go 解 GIF（ui.txt:9377-9384）存在但**不採用**（需求方裁決 GIF 走 mp4 variant）。web 相容已實測：js/wasm 編譯 exit 0，wasm 增量僅 45.2 KB（`verification-addendum.md` §1）。

**2.3 佈局與響應式。** `Context.Size()` 回視窗內容尺寸 DIP（ui.txt:1369-1370）；**`Window.OnResize(fn)` 存在**（core.txt:4584-4585）；`Tester.SetSize(width, height)` 使響應式佈局可用單元測試驗證（ui.txt:8453-8454）。`Services.Invalidate()` 可在任何 goroutine 呼叫（ui.txt:7907-7920）——背景載圖完成的喚醒路徑。**結論：水平響應式不需降級 v1.2**。

**2.4 內建元件盤點（需求方「MyGO 內建優先」完全可兌現）。**

| 需求 | 元件 | 出處 |
|---|---|---|
| For You／Following 雙 tab | `ui.Tabs(c, *int, labels...)`（`Changed`） | ui.txt:2343-2355 |
| 寬窗帳號側欄 | `ui.Sidebar` + `SidebarSection` + `SidebarItem`（含鍵盤導覽） | ui.txt:2202-2225 |
| 窄窗帳號切換 | `ui.Select(c, *string, options)` | ui.txt:2199-2200 |
| 移除帳號二次確認 | `ui.Modal(c, *bool, fn)` | ui.txt:1973 |
| 展開檢視頁 | `ui.Route`/`Router`（`Match`/`Param`/`Title`，含 BackButton 歷史） | ui.txt:7381-7420, 7502 |
| 時間線虛擬化 | `ui.List` + `ListState.Key`（回填舊帖不跳位）＋ `ItemKey` | ui.txt:1945-1947, 5770-5775, 3767 |
| 頭像 | `ui.Avatar(c, name, image *Bitmap)` | ui.txt:1650 |
| 開原帖（fallback） | `ui.Link(c, label, url)`（點擊開瀏覽器） | ui.txt:1938-1940 |
| 頂列 | `ui.Toolbar`；角標 `ui.Badge`；分段控制 `ui.Segmented` | ui.txt:2489, 1660, 2188 |

**2.5 儲存。** Secrets：`Set`/`Get`/`Delete`/`Service` 四方法（core.txt:3773-3789），無 List；`MaxSecretSize = 2560`。無 app 層 preferences。詳見 §1.5。

### 3. 現有程式碼事實基礎（v1 可復用面）

* **可復用測試基建**：transport 的 `startProxyRelay`/`startCaptureServer`；api 套件的合成註冊表 fixture 與 `tlItem/tlEntry/...` 合成器家族（新增 HomeLatestTimeline 測試幾乎只需擴充註冊表與萃取路徑）；app 層 `fakeSession` 注入模式（view 測試不碰真引擎）。
* **既有耦合陷阱**：`engineStore` 全域單例（session.go:50）、單 key write-through（send.go）、quit flush 固定目標（main.go:159-161）——三者構成 §1.2 的同一問題，須同一 commit 修。
* **接縫現況**：`newSession(cookieLine, proxyURL)` 已是事實上的工廠（zoom-out：無需再包抽象層）；`app.newSess` 注入點存在（main.go:33）；`AllowDirect` 直連選入已在（D10）。
* **規格條目現況**：REQ-DAT-010/012/014 已實作；REQ-DAT-011/013/015/016/018（媒體域）與 REQ-WEB-069/070（身分綁定）為 v1.1 新適用；REQ-WEB-066/068（讀取路徑）已免費獲得。

### 4. 規格新適用 REQ 對照（v1.1 生效面）

| REQ 域 | 條目 | v1.1 語義 | 落點 |
|---|---|---|---|
| DAT-013 | work_type 三分 | 帖/轉貼/引用的 `WorkType` | 引擎 `Post` 欄位 |
| DAT-015 | extended_entities 優先序 | 媒體萃取錨點 `extended_entities.media` | 引擎 timeline 萃取 |
| DAT-016 | mp4 最高位元率 | video/gif 的 variant 選擇 | 引擎（GIF 亦走 mp4，需求方裁決） |
| DAT-011 | 完整輸出欄位 | Post 補媒體欄位（Kind/ThumbURL/URL/Width/Height） | 引擎 `Post.Media` |
| DAT-018 | `?format=jpg&name=orig` | 縮圖用 small/medium；orig 留下載語義 | app 縮圖 URL |
| WEB-069/070 | 身分綁定一致／重複綁定拒絕 | 同 UserID 重複匯入＝覆蓋同 key，不產生重複帳號 | app accountsStore |
| TRN-010 | 單一出口 | **偏離登錄**：媒體 CDN 獨立 http 客戶端（§1.7） | DECISIONS |

規格未枚舉 `video_info.variants` 等 JSON 結構本身——結構細節以探測實證（fixture 紀律：欄位名可寫，不得複製 X 原文）。

### 5. 各域設計骨架（建議預設，待設計文件定形）

**5.1 多帳號。** Account 型別 `{UserID, ScreenName, Name, CookieLine string}`；索引 secret `x-accounts`（JSON `{version, order[], current}`）；每帳號 `x-account-<user_id>`（JSON 打包，中繼資料與 cookie 同鍵原子寫）；`newSessionFor(acc)`；`xSession` 補 `Close()`；`Validate` 改 `Verify(cookieLine) (ViewerInfo, error)`（切換流需取回身分，login 流同法複用）。**遷移**（Blocker）：v1 單鍵 `x-cookie-line` → 索引＋每帳號；序列「先寫後刪」（crash-safe）、Viewer 失敗保留舊鍵、降級期雙寫舊鍵（v1.1 期間回退不丟會話）。**帳號數上限 5（需求方 2026-11-14：3–5，目標非專業用戶）**，新增第 6 個時提示清理。移除帳號＝刪 secret＋索引除名＋二次確認（Modal）。**墓碑機制與憑證預熱（需求方 2026-11-14）**：兼顧切換速度與低常駐 RAM——(a) 啟動時 **lazy 預熱**全部帳號憑證（從 Secrets 逐帳號載入記憶體，僅 cookie 行與中繼資料，不建 transport）；(b) **有效性快取**：Viewer 重驗結果帶 TTL（建議 10 分鐘），TTL 內切換免重驗、直接以快取身分重建 session——降低取得次數並加快切換；(c) **切走即墓碑**：舊會話的時間線狀態（posts/cursor/seen/list）一律丟棄，僅保留帳號中繼資料＋憑證＋有效性紀錄；切回時重新載入時間線（重新 Home 首頁）。此為 D12（切換即重驗）的修訂：重驗仍存在，但受 TTL 快取節流。

**5.2 媒體。** `Post.Media []MediaItem{Kind, ThumbURL, URL, Width, Height}`；萃取點 `extended_entities.media`（`legacy.entities` 不做 fallback，首版）；photo 取 sizes 最大、video/gif 取最高位元率 mp4。非同步縮圖：自建 seam `fetchImage func(ctx, url) ([]byte, error)` 注入（正式走獨立 http，單檔上限＋總量上限＋併發 ≤4），完成經 `Services.Invalidate()` 套用；LRU 以解碼位元組計（≤24 MiB 起手，量測後定案），滾出即逐出＋ReleasePixels。`ui/video` 僅存在於展開檢視；收合顯式 `Close()`。GIF 走 mp4 variant（同一解析路徑，需求方裁決）。

**5.3 雙時間線。** `timelineState{posts, cursor, seen, end, busy, err, list}` ×2（不賭 map 泛化——現在有兩個，寫兩份）；`ui.Tabs` 頂部雙 tab；HomeLatestTimeline 解析路徑以 op 名參數化（D1 雙形態語義共用）；seen 上限（如 500）防無界成長。啟動預設 For You；切換保留對方已載內容。

**5.4 搜尋（需求方 2026-11-14 新增範圍）。** 頂列右上角搜尋入口 → 搜尋結果時間線頁。引擎面：`SearchTimeline` 操作——**規格已覆蓋**（REQ-RD-014：SearchTimeline 恆 POST、REQ-RD-001..013 的讀取語義同用），但 registry 快照尚無該操作、queryId 未知 → 併入 P0' 探測輪（與 HomeLatestTimeline 同法萃取＋驗證）。app 面：搜尋結果為第三種 `timelineState`（僅查詢時存在、關閉即棄，不常駐）；Enter 觸發、熱詞不記憶（本輪不做搜尋歷史）。此項使 v1.1 引擎操作數為四：Viewer、HomeTimeline、HomeLatestTimeline、SearchTimeline。

**5.5 水平響應式與頁面骨架（需求方 2026-11-14 佈局裁決）。** 主要內容收斂於**頂列（Toolbar）**：左上角帳號切換、右上角搜尋入口；**不做下方 bar**（僅服務 Desktop；行動端情境不考量）。斷點採需求方提供之 768/1024/1200 階梯（DIP 語義，flexbox 實現）：

| 寬度 | 佈局 |
|---|---|
| <768 | 單欄＋頂列收合（帳號以 `ui.Select` 下拉、搜尋圖示展開）；視窗 MinWidth 設 480 時此 band 才會出現 |
| 768–1023 | 兩欄：左側欄（`ui.Sidebar`，帳號清單＋tabs）＋時間線 |
| 1024–1199 | 完整版型：側欄＋時間線＋展開檢視並列（點帖右側開頁，不切換路由） |
| ≥1200 | 寬內容區：同上，卡片 max-width 640 置中、兩側留白 |

`c.Size()` 判斷＋`Window.OnResize` 觸發重建；`Tester.SetSize` 以 768/1024/1200 三寬度測試進 CI。

**5.6 格式化與 CI。** `cd X` → `gofumpt -w -extra .` → `golangci-lint run` → `go tool mygo build -platform web`（編譯冒煙，每 commit 退出條件）。web 明示**不在資料面驗收範圍**（CORS＋raw TLS 限制；資料面 relay 屬 v2）。

## Goals vs. Non-Goals

**In-Scope Goals:**

* **多帳號**：帳號清單（索引 secret＋每帳號 secret，**上限 5**）、新增（現有 login 流復用）、切換（重驗 Viewer＋有效性 TTL 快取，D12 修訂）、移除（Modal 確認）、失效呈現（credential 錯誤文案＋「重新貼上 Cookie」入口，不移除 key）；**啟動 lazy 憑證預熱＋切走即墓碑**（需求方 2026-11-14）；REQ-WEB-069/070 身分語義；v1 單鍵遷移（先寫後刪＋降級雙寫）；write-through 帳號化與 `engineStore` 廢除；transport Close 接線。
* **媒體**：extended_entities 解析（photo/video/gif，mp4 最高位元率）；列表非同步縮圖（seam 注入、LRU 位元組上限、ReleasePixels）；展開檢視（大圖＋`ui/video` 內嵌播放、收合釋放）；媒體下載獨立出口偏離登錄（CDN 免認證已驗證，§1.7）。
* **雙時間線**：`HomeLatestTimeline` P0' 探測輪（含變數形狀與 features 兩層）→ 快照增補 → 雙 tab、`timelineState` 隔離、seen 上限。
* **搜尋（需求方 2026-11-14 新增）**：`SearchTimeline`（規格 REQ-RD-014 恆 POST）＋頂列搜尋入口＋結果時間線頁（查詢時存在、關閉即棄）。
* **UI/UX**：頂列骨架（左上帳號切換、右上搜尋；**無下方 bar**，Desktop only）；水平響應式（768/1024/1200 斷點＋`Tester.SetSize` 測試）；內建元件兌現（§2.4）。
* **工程**：格式化鏈（gofumpt＋golangci-lint，`cd X`）；web 編譯冒煙進每 commit 退出條件；RAM 量測場景擴充（含媒體 100 帖／播放中峰值）；文件架構待辦（§7）。

**Out-of-Scope Non-Goals（承接 v1 全部 Non-Goals，另加）:**

* 寫入（發帖/按讚/轉貼）、私信、文章、X Chat、帳密登入、Castle、XCTID、CLI、console——維持 v2。
* 並行常駐會話（模型 B）——逃生通道只保留介面空間，不實作。
* 帳號級代理（`WithProxies` 為無消費端死選項，介面預留但不接線）。
* 磁碟媒體快取、多媒體輪播等展開檢視進階互動、GIF 純 Go 解碼路徑。
* X 端登出（僅本機移除帳號）；web 資料面（relay）。
* 媒體上傳。

## Alternatives Weighed

1. **【否決】`ui/video` 延後 v2（zoom-out #7）**——依據當時「GStreamer 系統依賴、生命週期未證」的反對。實測推翻：Windows 用系統 Media Foundation（無額外依賴）、`Close()` 存在、wasm 相容且增量 45 KB。需求方「展開播放」LOCKED 裁決維持。
2. **【採用】GIF 走 mp4 variant（需求方 2026-11-14）**——與 video 同一解析路徑，省一套解碼器；純 Go GIF 解碼（`ui.NewGIFDecoder`）記為事實但不採用（流量考量優先於 CPU）。
3. **【採用，後經墓碑裁決修訂】切換即重驗 Viewer（方案甲，需求方 2026-11-14）**——立即確認 cookie 有效性並取得 UserID；失敗不切換、保留原會話。需求方同日裁決墓碑機制後，修訂為「重驗＋TTL 有效性快取」（TTL 免重驗、加快切換、降低取得次數）。zoom-out 方案乙（不重驗、失效在首次 Home 暴露）記為權衡：省一次請求，但切換當下不知有效性，且切換流需要 UserID 定 key（方案乙仍需一次 Viewer 呼叫，節省有限）。
4. **【否決】`xSession` 工廠化為新抽象層（zoom-out #1 部分）**——`newSession` 已是工廠；v1.1 只做「參數化」（`newSessionFor(acc)`）＋補 `Close()`，不新增 manager/registry 抽象。工廠「存在」不等於「參數化已完成」，此為審查者間的語義誤差，設計文件須寫死簽名。
5. **【並陳】自寫 LRU vs 僅可見列載入＋ReleasePixels**——採後者為基線、LRU 為量測後的補強（zoom-out 紀律：先量測再定上限）。兩案可疊加：可見列載入是行為、LRU 是容量保險。
6. **【否決】水平響應式整項延後 v1.2（zoom-out #11）**——`Window.OnResize`＋`Context.Size()`＋`Tester.SetSize` 三件套存在（core.txt:4584、ui.txt:1369/8453），降級條件已解除。
7. **【採用】媒體獨立 http 客戶端（comprehension 2.4＋implementation-readiness）**——vs 走引擎 transport（design F7 主張）：後者需為媒體開二進位下載面且媒體 403/404 不落六型別語義，成本高於收益；DECISIONS 登錄 TRN-010 偏離。
8. **【採用】oxfmt 從 Go 鏈移除（需求方 2026-11-14）**——oxfmt 不支援 Go 且會改寫凍結規格/vendor；若日後有 JS/TS 資產再單獨採用。
9. **【否決】自建帳號清單遍歷（嘗試列舉 secret keys）**——Secrets 無 List API（實測）；索引 secret 是唯一路徑。
10. **【否決】「快取上限＝80 MiB」字面執行**——與行程峰值驗收線自相矛盾（§1.6）；採解碼位元組 LRU ≤24 MiB 起手＋量測定案。
11. **【並陳】媒體 UI 降級為「解析＋計數標示＋外開」**——若 `fetchImage` seam 的正式路徑（公開 CDN 直載）被 X 風控攔截（4xx/簽章），此為退路：`ui.Link` 外開成本為零。開工前探測輪附帶驗證 CDN 直載可用性。
12. **【維持】web 暫緩發布但可編譯（需求方 2026-11-13）**——作為測試載體；每 commit 編譯冒煙；資料面 relay 屬 v2。

## Key Decisions & Staging for Next Steps

**已定案（D 編號草案，回寫 DECISIONS.md 時定稿；D10 歸屬正規化見 §7）：**

* **D11（草案）多帳號模型 A＋逃生通道**：單活躍會話；索引 secret `x-accounts`＋每帳號 secret；`newSessionFor(acc)`；`xSession` 補 `Close()`；`Verify(cookieLine)`。依據：需求方裁決＋Secrets 實測約束。
* **D12（草案）切換即重驗 Viewer（方案甲＋有效性快取）**：切換時重驗；**重驗結果帶 TTL（建議 10 分鐘），TTL 內免重驗**（需求方 2026-11-14 墓碑機制裁決）；失敗不切換、標記失效、原會話不動；舊 transport 於新會話驗證成功後關閉；啟動 lazy 預熱全部帳號憑證；切走即丟棄時間線狀態（墓碑），切回重新載入。依據：需求方 2026-11-14 兩次裁決（方案甲＋墓碑）。
* **D13（草案）GIF 走 mp4 variant**：與 video 同一解析路徑。依據：需求方 2026-11-14。
* **D14（草案）媒體 RAM 預算**：解碼位元組 LRU ≤24 MiB（量測後定案）＋行程峰值 <80 MiB 驗收；ReleasePixels 槓桿；縮圖 small/medium。依據：L2 基線＋zoom-out 紀律。
* **D15（草案）媒體下載獨立出口**：輕量 http 載公開 CDN＋TRN-010 偏離登錄。依據：§1.7 權衡。
* **D16（草案）HomeLatestTimeline 規格外登錄**：以 P0' 探測＋快照為契約、隨前端漂移；探測結論回寫 DECISIONS。
* **D17（草案）格式化鏈**：`cd X` → `gofumpt -w -extra .` → `golangci-lint run`；oxfmt 移除。依據：需求方 2026-11-14＋工具鏈實測。
* **D18（草案）世代邊界**：切換即 cancel 舊 context 子樹、publish 攜世代號；`timelineState` ×2。依據：§1.1。
* **D19（草案）帳號數上限 5**：新增第 6 個提示清理；目標用戶非專業用戶。依據：需求方 2026-11-14。
* **D20（草案）頂列佈局與斷點**：左上帳號切換、右上搜尋、無下方 bar（Desktop only）；斷點 768/1024/1200（需求方提供階梯）。依據：需求方 2026-11-14。
* **D21（草案）搜尋入 v1.1**：`SearchTimeline`（REQ-RD-014）＋頂列入口＋結果頁；queryId 併 P0' 探測。依據：需求方 2026-11-14（#1 附帶範圍擴張）。
* **D22（草案）媒體 CDN 免認證定案**：`pbs.twimg.com`/mp4 variant 直載（查證＋P0' 實抓收尾）；降級退路保留。依據：需求方 2026-11-14「搜尋並驗證」＋本輪查證（§1.7）。

**階段劃分（commit 粒度：一能力域一 commit，Conventional Commits，退出條件＝`go test -race` 綠＋web 編譯冒煙＋格式化鏈通過）：**

* **P0' 前置閘（開工前）**——(a) `HomeLatestTimeline` 探測輪（兩臂×兩形態×≥20 次）→ D16 定稿；(b) `SearchTimeline` queryId 萃取與請求驗證（同法；REQ-RD-014 恆 POST）；(c) 媒體 CDN 直載可用性驗證（縮圖 URL 實抓；查證已確認免認證，實測收尾）；(d) web「已實證」虛假引用撤回（DECISIONS 修訂）。負責：實作者（探測）＋需求方（提供探測憑證）。
* **C1 `feat(accounts)`**——Account 型別＋accountsStore（索引＋每帳號）＋v1 單鍵遷移（先寫後刪＋雙寫）＋write-through 帳號化＋`engineStore` 廢除。退出：遷移三態測試（新裝/升級/回退）綠。
* **C2 `feat(session)`**——`newSessionFor`＋`Verify`＋切換序列（含失敗回滾與 transport Close）＋`xSession.Close()`。退出：切換流注入測試（成功/失效/崩潰安全）綠；世代邊界測試（in-flight 過期即棄）綠。
* **C3 `feat(timeline)`**——P0' 結論落快照＋`HomeLatestTimeline` 方法（D1 雙形態語義共用）＋`timelineState` ×2＋`ui.Tabs` 雙 tab＋seen 上限。退出：合成 fixture 測試綠；雙 tab 狀態隔離測試綠。
* **C4 `feat(search)`**——`SearchTimeline` 方法（恆 POST）＋搜尋結果 `timelineState`（查詢時存在、關閉即棄）。退出：合成 fixture 測試綠。
* **C5 `feat(media)`**——`Post.Media` 萃取＋`fetchImage` seam＋非同步縮圖＋LRU＋ReleasePixels。退出：萃取 fixture 測試綠；RAM 量測（含媒體 100 帖）<80 MiB。
* **C6 `feat(detail,ui)`**——展開檢視（Route＋大圖＋`ui/video`，收合 Close）＋頂列骨架（左上帳號切換、右上搜尋入口）＋水平響應式（768/1024/1200 斷點）＋`Tester.SetSize` 三寬度測試＋格式化鏈（gofumpt＋golangci-lint 全綠）。退出：全部驗收場景綠；雙平台建置。
* **收尾**——DECISIONS 回寫（D11–D22＋探測結論＋偏離登錄）、README 更新、`design_draft.md` 處置、L2 發現節回註。

**已裁決記錄（需求方 2026-11-14，原開放問題六題全數裁決——清單清空）：**

1. **佈局與斷點（原 #1）→ D20**：不要「只有 timeline」——頂列承載主要內容：左上帳號切換、右上搜尋；不做下方 bar（Desktop only，行動端不考量）；斷點 768/1024/1200（需求方提供階梯）。帶來**搜尋新範圍 → D21**。
2. **帳號數上限（原 #2）→ D19**：3–5、不超過 5；目標用戶非專業用戶。
3. **切換狀態（原 #3）→ D12 修訂**：墓碑機制——啟動 lazy 預熱全部帳號憑證；憑證有效時（TTL 內）加快切換、降低取得次數；切走即丟時間線狀態、切回重載。
4. **LRU 先量測再定案（原 #4）→ D14 維持**：接受。
5. **HomeLatestTimeline 探測（原 #5）**：裁決「測試」＝執行 P0' 探測鏈（空 features → features 回填 → 仍失敗才降級登錄），與建議預設一致。
6. **媒體 CDN（原 #6）→ D22**：裁決「搜尋並驗證」——已查證（§1.7）：官方 API 限流體系（`x-rate-limit-*`、OAuth）不適用於媒體 CDN；`pbs.twimg.com` 免認證、無簽章、無過期參數；403 僅限保護/已刪內容。降級退路保留為應急。

### 與 v1 的差異清單

| v1.1 節/主題 | v1 對應位置 | 變更類型 |
|---|---|---|
| 多帳號（索引 secret＋每帳號 secret＋切換） | v1 Non-Goals（單帳號）；`cookies_store.go` 單 key | **新增** |
| 帳號遷移（先寫後刪＋雙寫） | v1 無 | **新增** |
| write-through 帳號化、`engineStore` 廢除 | v1 `session.go:50`、`main.go:159-161` | **修改** |
| `xSession` 補 `Close()`、transport 接線 | v1 引擎有 Close、app 層未呼叫 | **修改** |
| 世代邊界（cancel＋世代號） | v1 單份 app 狀態（main.go:50-57） | **修改** |
| `Verify(cookieLine)` 取代 `Validate` | v1 `xSession.Validate` | **修改** |
| 媒體（Post.Media＋縮圖＋展開播放） | v1 Non-Goals「媒體（7.10）」 | **新增** |
| GIF 走 mp4 variant | v1 無 | **新增** |
| 雙時間線（HomeLatestTimeline＋Tabs） | v1 僅 HomeTimeline | **新增** |
| 搜尋（SearchTimeline＋頂列入口＋結果頁） | v1 無（讀取路徑僅 HomeTimeline） | **新增** |
| seen 上限 | v1 無界（view_home.go:112） | **修改** |
| 水平響應式（OnResize＋頂列骨架＋768/1024/1200 斷點） | v1 固定單欄 480×720 | **新增** |
| 墓碑機制＋lazy 憑證預熱＋有效性 TTL | v1 無（單帳號無切換） | **新增** |
| 內建元件兌現（Tabs/Sidebar/Select/Modal/Route/List/Avatar） | v1 僅 Text/Column/Row/Input/Button | **新增** |
| 格式化鏈（gofumpt＋golangci-lint，`cd X`） | v1 僅 `go vet`＋`go test -race` | **新增** |
| web 測試載體定位（每 commit 冒煙） | v1 web 建置一次（虛假引用「已實證」） | **修改** |
| 代理解析（D10 直連基線） | v1 REQ-INV-02 fail-closed | **沿用**（D10 已於 v1 修訂） |
| 寫入/DM/文章/帳密/XCTID | v1 Non-Goals | **沿用** |
| 上游授權確認閘（repo 私有） | v1 P5 | **沿用** |

### 文件架構待辦（structural 審查者發現，開工時處置）

1. **v1 計畫檔未入庫**：`docs/superpowers/plans/` 目錄整個缺席（v1 計畫內容折入設計文件 §Detailed Implementation Plan）。裁決放置層後新增 `docs/superpowers/plans/2026-11-14-x-engine-v1.1.md` 並 **commit**（v1.1 不得再犯）。
2. **`design_draft.md` 雙拷貝**：與 `x-engine-v1.md` 逐行幾乎相同（Draft vs Proposed）。v1.1 開輪時裁決歸檔或加 Superseded 標記；**v1.1 不得再產生拷貝**。
3. **DECISIONS.md D10 歸屬正規化**：D1–D9 掛「已定案裁決」、D10 嵌在「L2 驗收記錄」下——續編 D11 前先正規化；定位句放寬為「X 引擎各輪」＋每筆裁決標適用版本（v1/v1.1）。
4. **規格 REQ×版本適用性對照表**：新增「章節域/v1 狀態/v1.1 狀態/裁決編號」表（D10↔REQ-INV-02 翻轉目前無機械查閱點）。
5. **L2 發現節回註**：timeline `legacy`→author `core` 漂移已於 v1 修復（複驗 30/30）——L2 登記待辦節回註「已由 v1.1 輪受理」。
6. **README**：版本路線與 SemVer 對應釘死；web「已實證」表述撤回。
