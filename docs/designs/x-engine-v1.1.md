# MyGO X 引擎 v1.1 設計文件（RIGG/X）

* **Author:** 控制器（依需求方 2026-11-14 裁決與 `concepts_brief_v1.1.md` 撰寫）
* **Date:** 2026-11-14
* **Status:** Proposed——待需求方簽核後入計畫
* **上游文件：** `docs/designs/concepts_brief_v1.1.md`（構想與 D11–D22 草案）、`docs/designs/x-engine-v1.md`（v1 設計，本文僅記載**差異**，v1 未觸及處沿用）、`docs/DECISIONS.md`（D1–D10 已定案）、`XAPIS_BEHAVIOR_SPEC.md`（凍結規格）

> 本文所有「§n」指本文件章節；「簡報 §n」指構想簡報章節。實測證據（檔案:行、go doc 行號）已在簡報載明，本文僅於關鍵處重引。

## Overview

v1.1 在 v1 的單帳號唯讀引擎上擴充五域：多帳號（模型 A＋墓碑機制）、媒體（縮圖＋展開播放）、雙時間線（For You／Following）、搜尋（`SearchTimeline`）、頂列響應式 UI。引擎層新增三個讀取操作與 `Post.Media` 資料面；app 層新增帳號儲存／會話工廠／媒體管線／頂列佈局。RAM 行程峰值 <80 MiB 為全程驗收線。

## The Problem

v1 有四個結構性缺口，v1.1 逐一補上：

1. **單帳號硬編碼**——`engineStore` 全域單例（session.go:50）＋單 key `x-cookie-line`（cookies_store.go:21）＋quit flush 固定目標（main.go:159-161）：無法容納第二個帳號。
2. **app 層單份狀態**——posts/cursor/seen/list 無帳號與時間線歸屬（main.go:50-57）：無法容納第二個帳號的會話，也無法容納第二條時間線。
3. **`Post` 無媒體資料面**（timeline.go:23-28）與 transport 無 app 層關閉路徑（client.go:168 有 Close、無人呼叫）。
4. **固定 480×720 單欄佈局**、無頂列、無搜尋。

## Goals

* **多帳號**（上限 5）：新增（現有 login 流復用）、切換（重驗＋TTL 有效性快取，墓碑機制）、移除（Modal 確認）；v1 單鍵遷移（先寫後刪＋降級雙寫）；REQ-WEB-069/070 身分語義；write-through 帳號化；transport Close 接線。
* **媒體**：`extended_entities` 解析（photo/video/gif，mp4 最高位元率）、非同步縮圖（LRU 位元組上限＋ReleasePixels 槓桿）、展開檢視（大圖＋`ui/video`，收合 `Close()`）。
* **雙時間線**：For You（`HomeTimeline`）／Following（`HomeLatestTimeline`，P0' 探測後落快照）雙 tab，狀態完全隔離。
* **搜尋**：頂列入口 → `SearchTimeline`（恆 POST，REQ-RD-014）結果頁，查詢時存在、關閉即棄。
* **UI/UX**：頂列骨架（左上帳號、右上搜尋，無下方 bar）；768/1024/1200 斷點；`Tester.SetSize` 測試。
* **工程**：格式化鏈（`cd X` → `gofumpt -w -extra .` → `golangci-lint run`）＋web 編譯冒煙進每 commit 退出條件。

## Non-Goals

承接 v1 全部 Non-Goals（寫入／私信／文章／帳密登入／Castle／XCTID／CLI／console）；另加：並行常駐會話（模型 B）、帳號級代理（`WithProxies` 不接線）、磁碟媒體快取、多媒體輪播、GIF 純 Go 解碼路徑、X 端登出、搜尋歷史、下方 bar／行動端佈局、媒體上傳。

## Critical User Journeys (v1.1 新增)

### CUJ 4: 多帳號新增、切換、移除

新增：home 頂列帳號選單 →「新增帳號」→ login view 變體（保留返回鈕）→ 貼 cookie → Viewer 驗證 → 取得 UserID → 寫帳號 secret＋索引 → 自動切換為活躍帳號。同 UserID 重複匯入＝覆蓋同 key（REQ-WEB-070）。

切換：帳號選單點選目標 → `switchTo`：寫回當前 cookie（flush）→ 索引 `current` 寫目標 → 有效性檢查（TTL 內跳過重驗）→ `newSessionFor` → （TTL 外時）`Validate` → 成功：替換 sess/auth/store、cancel 舊 context、關閉舊 transport、時間線歸零（墓碑）→ home 重載 For You。失敗：標記該帳號失效（不清除 secret）、索引回寫原帳號、原會話不動、錯誤區呈現 credential 文案＋「重新貼上 Cookie」入口。

移除：帳號選單「移除」→ Modal 二次確認 → 刪帳號 secret＋索引除名 → 若為活躍帳號則登出至 login（其餘帳號仍可選）。

### CUJ 5: 媒體瀏覽

時間線列顯示縮圖（`ui.Image`，非同步載入、失敗顯示可重試佔位）→ 點帖 → 展開檢視（≥1024 右欄並列；<1024 Router 導頁 `/status/{id}`）→ 大圖（`name=large`）／影片 GIF 以 `ui.Video` 播放最高位元率 mp4（`video.OpenURL`）→ 離開展開檢視 → `player.Close()`。

### CUJ 6: 時間線分流與搜尋

頂列下方 tabs（`ui.Tabs`）：For You ↔ Following，各自 cursor/seen/list 獨立、切換保留對方已載內容、各自 AtEnd 觸發翻頁。頂列右上搜尋框（Enter）→ 搜尋結果 feed（第三狀態，僅查詢時存在）→ 關閉即棄。

## Technical Plan (Architecture)

### 8.1 總體拓撲（delta）

```
X/
├── main.go                 [修改] app struct：gen／feeds／accounts／switchTo/warmup
├── session.go              [修改] xSession 介面擴充、newSessionFor、廢 engineStore
├── accounts.go             [新增] Account／accountsStore／遷移／憑證預熱與 TTL 快取
├── media.go                [新增] thumbStore（LRU）／fetchImage seam
├── view_home.go            [修改] 頂列、tabs、feeds 接線、世代 publish
├── view_detail.go          [新增] 展開檢視（大圖／video 播放、生命週期）
├── view_search.go          [新增] 搜尋列（頂列右上）
├── messages.go             [修改] 新錯誤文案（失效帳號、媒體載入失敗）
├── app_state.go            [修改] per-session context 子樹
└── internal/xapi/
    ├── api/timeline.go     [修改] Post.Media、HomeLatestTimeline、SearchTimeline
    ├── api/media.go        [新增] extended_entities 萃取（REQ-DAT-013/015/016/018）
    └── …（v1 七層不動）
```

引擎七層分層與 import 禁令（`internal/xapi` 不得 import MyGO）不變。

### 8.2 引擎層變更

**操作面**——`api.Sender` 新增兩個讀取方法（沿用 v1 的單一 `CookieHeader()` 載入 caller rule、`HeaderOrderKey` 線序、六型別錯誤分類）：

```go
// HomeLatestTimeline 與 HomeTimeline 同構（D1 雙形態語義共用）：
// 首頁 GET 無 seen；有 seen（cursor 或 seen 非空）走 POST。實際形態依
// P0' 探測結果定稿（DECISIONS D16），此簽名不隨形態改變。
func (s *Sender) HomeLatestTimeline(cursor string, count int, seenTweetIDs []string) (*TimelineResult, error)

// SearchTimeline 恆 POST（REQ-RD-014），variables 含 query；無 seen。
func (s *Sender) SearchTimeline(query, cursor string, count int) (*TimelineResult, error)
```

註冊表快照（`X/static/graphql.json`）增補 `HomeLatestTimeline` 與 `SearchTimeline` 兩操作（queryId／operationName／features 依 P0' 探測結果；`featureSwitches` 沿用現有快照格式）。快照格式不變（含 `generatedAt`），兩處出貨機制（mygo.json `resources`＋`registryPath()` 兩級查找）已就緒不動。

**資料面**——`Post` 增媒體欄位：

```go
// MediaItem 是單一媒體項（REQ-DAT-011/013/015/016/018 的 v1.1 子集）。
type MediaItem struct {
    Kind     string // "photo" | "video" | "gif"（DAT-013 三分）
    ThumbURL string // 列表縮圖：photo → media_url_https?format=jpg&name=small；
                    // video/gif → video_info.poster（或 media_url_https）同名參數
    URL      string // photo → name=large；video/gif → 最高位元率 mp4 variant（DAT-016）
    Width    int
    Height   int
}
// Post 增：Media []MediaItem（零值＝無媒體，不影響既有欄位與測試 fixture）
```

萃取規則（`api/media.go`，`extended_entities.media` 為唯一錨點——`legacy.entities` 不做 fallback，首版）：

* photo：`media_url_https` 為基底；`sizes` 取 `small`（縮圖）與 `large`（展開）；`Width/Height` 自 `sizes` 對應項。`format` 參數自 URL 副檔名推導（`jpg`/`png`/`webp`——探測實證：PNG 來源以 `format=jpg` 請求回 404，必須沿用 API 給予的副檔名）。
* video／gif：`video_info.variants` 過濾 `content_type == "video/mp4"`，取 `bitrate` 最高者為 `URL`（REQ-DAT-016；GIF 走 mp4——D13）；縮圖＝`media_url_https?format=jpg&name=small`。`duration_millis` 不取（展開檢視播放器自帶控制）。
* 缺 `extended_entities` 的帖：`Media` 為 nil（非空 slice 零長度不允許）。

**介面紀律**——引擎公開 API 僅增不改：`HomeTimeline`/`Viewer`/`Send` 簽名不動；`Post` 增欄位為向後相容變更（既有 fixture 不受影響）。

### 8.3 帳號資料模型與儲存（app 層）

```go
// Account 是一份持久化的憑證與身分（REQ-WEB-069：身分與憑證綁定一致）。
type Account struct {
    UserID     string `json:"user_id"`     // Viewer 取得，唯一識別
    ScreenName string `json:"screen_name"`
    Name       string `json:"name"`
    CookieLine string `json:"cookie_line"` // 憑證材料，僅存 secret
}

// accountIndex 是帳號清單真源（MyGO Secrets 無 List API——實測 core.txt:3736-3790）。
type accountIndex struct {
    Version int      `json:"version"` // 恆 1
    Order   []string `json:"order"`   // user_id 列表（新增序）
    Current string   `json:"current"` // 活躍帳號 user_id（空＝無）
}
```

儲存鍵（service 沿用 MyGO 預設 app identifier）：

| 鍵 | 內容 | 上限 |
|---|---|---|
| `x-accounts` | `accountIndex` JSON（≤300 bytes） | MaxSecretSize 2560 |
| `x-account-<user_id>` | `Account` JSON（實測 ≈303 bytes；整份 Cookie header 可能逼近 2560 → 超限回錯誤並提示精簡貼上，不做加密落檔降級——v1.1 記為已知限制，v2 再議） | MaxSecretSize 2560 |

`accountsStore`（`X/accounts.go`）：

```go
type accountsStore struct{} // 無狀態：鍵即命名空間
func loadAccounts(ctx context.Context) ([]Account, accountIndex, error) // 讀索引＋逐鍵讀帳號
func saveAccount(ctx context.Context, acc Account) error                // 原子寫帳號鍵
func saveIndex(ctx context.Context, idx accountIndex) error
func deleteAccount(ctx context.Context, userID string) error            // 刪帳號鍵＋索引除名
```

**寫回（write-through）帳號化**——廢除 `engineStore` 全域（session.go:50）：`newSessionFor` 內以 `NewCookieStore("", "x-account-"+acc.UserID)` 建每帳號 store 接入該會話 Auth；quit flush（main.go:159-161）目標改為**活躍帳號的 store**（`app.activeStore auth.CookieStore`，切換時替換）。

**遷移（v1 → v1.1，Blocker）**——序列「先寫後刪」（crash-safe）：

```
啟動：loadAccounts() 後若索引不存在且舊鍵 x-cookie-line 有值：
  1. 以舊鍵值組臨時 session → Viewer 取 UserID 與身分（Validate 語義）
  2. 失敗 → 保留舊鍵、呈 login（絕不清除）
  3. 成功 → saveAccount(Account{…}) → saveIndex(Order=[uid], Current=uid)
     → 雙寫期（v1.1 全程）：活躍帳號 Save 時同步寫舊鍵（降級回 v1 二進位不丟會話）
  4. 成功後刪除舊鍵？——不刪：降級雙寫需要它；v1.2 移除雙寫時一併清理
```

（對簡報 §5.1 的修訂：舊鍵**保留**作為降級雙寫目標，不於遷移後刪除——否則雙寫無處落。）

**身分語義（REQ-WEB-069/070）**——匯入時以 Viewer 回應的 UserID 定鍵；同 UserID 再匯入＝`saveAccount` 覆蓋同鍵並提示「已更新」；不同 cookie（不同 UserID）→ 新鍵。

### 8.4 會話工廠與世代邊界

```go
// xSession 介面擴充（session.go）——v1 的 Validate/Home 不動，增三方法：
type xSession interface {
    Validate(cookieLine string) error                                  // PrepareAuth＋Viewer
    Home(cursor string, seen []string) (*timelineResult, error)
    HomeLatest(cursor string, seen []string) (*timelineResult, error)  // [新增]
    Search(query, cursor string) (*timelineResult, error)              // [新增]
    Close() error                                                      // [新增] 關閉 transport
}

// engineSession 增 tr 欄位持有 transport；Close() 呼叫 tr.Close()（冪等）。
func newSessionFor(acc Account, proxyURL string) (*engineSession, error)
// 組裝鏈同 v1 newSession，差異：store = NewCookieStore("", "x-account-"+acc.UserID)
// → auth.WithCookieStore(store)；PrepareAuth(acc.CookieLine)（cookie 行已知）。
```

**世代邊界（D18，簡報 §1.1）**——`app` 增：

```go
gen        int               // 每次成功切換 +1
sessCtx    context.Context   // 活躍會話 context（state.ctx 的子樹）
sessCancel context.CancelFunc
```

* feed 背景工作以 `sessCtx` 派發（`app_state.go` 增 `GoSession`）；切換序列先 `sessCancel()`。
* publish 攜世代：`loadMore` 起跑前捕捉 `gen := a.gen` 與 feed 指標；回寫前 `if gen != a.gen { return }`（雙保險——cancel 已斬斷在途，gen 防已回程的 publish）。
* 假實作（`fakeSession`）同步擴充三方法；世代檢查在 app 層測試以注入 `apply` 延遲驗證。

**切換序列（D12，含收斂態）**——`switchTo(userID string)`（背景執行，UI 顯示 spinner、期間禁翻頁與再切換）：

```
1. flush 當前：activeStore.Save(當前 cookieLine)（write-through 已同步，此為兜底）
2. 索引 Current = 目標 → saveIndex
3. sessCancel()（斬斷舊在途）
4. newSessionFor(目標)（無網路；失敗 → 回滾索引、原會話不動、錯誤區呈現）
5. 有效性檢查：TTL 內（credCache，§8.5）→ 跳過；TTL 外 → sess.Validate(acc.CookieLine)
   失敗 → 標記帳號失效（accInvalid map）、回滾索引至原帳號、原會話不動、錯誤文案
6. 成功：舊 sess.Close()（transport 收斂——簡報 §1.3 實測根因）
7. 舊 sessCancel 已執行；替換 a.sess/a.auth/a.activeStore；a.gen++
8. feeds 歸零（墓碑）；自動載入 For You 首頁
```

崩潰安全：索引先寫（2）、會話後換（7）——中途崩潰下次啟動以索引 Current 為準重載。

### 8.5 墓碑與憑證預熱（D12 修訂、D19）

```go
// 憑證有效性快取（TTL 內切換免重驗——需求方「降低取得次數、加快切換」）。
const verifyTTL = 10 * time.Minute
type credCache struct {
    mu       sync.Mutex
    verified map[string]time.Time // user_id → 最後驗證時刻
}
func (c *credCache) fresh(userID string) bool
func (c *credCache) mark(userID string)
func (c *credCache) invalidate(userID string) // 切換驗證失敗時
```

**lazy 預熱**——`app.warmup()`（`sessCtx` 派發，啟動後執行一次）：`loadAccounts` 讀索引＋全部帳號 secret 進 `a.accounts []Account`（記憶體快照；MyGO Secrets 可從任意 goroutine 呼叫——實測文件）。登入/登出/移除/遷移成功後重跑（冪等）。UI 的帳號選單讀 `a.accounts`（預熱完成前顯示「載入中…」）。

**墓碑**——切走即丟：`a.feeds` 整體重建（posts/cursor/seen/list 歸零）；不保留任何舊帳號的時間線狀態。切回帳號 = 重新 Home 首頁（憑證與中繼資料在記憶體，切換無 secret IO）。

**上限（D19）**——`maxAccounts = 5`；「新增帳號」入口在 `len(accounts) >= 5` 時呈提示文案（不解鎖路徑）。

### 8.6 媒體管線（app 層）

```go
// fetchImage seam：測試注入；正式路徑＝獨立輕量 http.Client（公開 CDN 直載，
// D15/D22；timeout 10s、單檔 ≤8 MiB、Content-Type 必須 image/*）。
var fetchImage = func(ctx context.Context, url string) ([]byte, error)
const thumbConcurrency = 4 // 跨 feed 全域信號量（簡報 F10）

// thumbStore：解碼位元組 LRU（D14——≤24 MiB 起手，量測後定案常數）。
type thumbStore struct {
    mu      sync.Mutex
    entries map[string]*thumbEntry // key = ThumbURL
    order   []string               // LRU 序（觸碰即移尾）
    bytes   int
}
type thumbEntry struct {
    src   ui.ImageSource // DecodeImage 產物（Bitmap）
    bytes int            // 解碼後估算（w*h*4）
}
func (t *thumbStore) get(url string) (ui.ImageSource, bool)   // 主執行緒；命中即觸碰
func (t *thumbStore) put(url string, src ui.ImageSource)      // 主執行緒；超限逐出最舊
```

載入流程（view 建構縮圖格時）：`get` 命中 → 直接顯示；未命中 → 佔位（主題色塊＋「載入中」）＋ 若該 URL 無在途任務 → `spawn`（semaphore 4、`sessCtx`）→ `fetchImage` → `apply`（主執行緒）→ `ui.DecodeImage` → `put` → 重繪（`Services.Invalidate()` 或下一次 build）。失敗 → 佔位改「可重試」文案，點擊重試（不自動重試，避免風暴）。

記憶體槓桿：LRU 逐出即 Go GC 回收（`Bitmap` 不可複用、不持 GPU 句柄管理責任）。`Bitmap.ReleasePixels()`（ui.txt:784-795）列為量測後補強開關（常數 `releasePixels bool`，預設 false；量測顯示 CPU 像素佔比過高再開）。

**展開檢視與播放器生命週期**——單一活躍播放器：`app.player *ui.VideoPlayer`；開啟新帖展開前 `Close()` 舊的；離開展開檢視（Route pop／收合）`Close()`。`video.OpenURL(mp4URL)` 於展開建構時呼叫（背景載入，`Ready` 前顯示縮圖）。媒體 403/404（保護/已刪內容）→ 錯誤映射為 app 層 `mediaFailed` 文案＋外開連結（`ui.Link`，Alternative 11 退路）。

### 8.7 雙時間線與搜尋（app 層）

```go
type timelineKind int
const (feedForYou timelineKind = iota; feedFollowing; feedSearch)

type timelineState struct {
    posts  []post
    cursor string
    seen   []string // 上限 500（F10）：append 後 len>500 → 截尾保留最近
    end    bool
    busy   bool
    err    error
    list   ui.ListState // Key=WorkID（回填不跳位——ui.txt:5770-5775）
}
// app.feeds [3]*timelineState（固定三槽，非 map）；feedSearch 僅查詢時填充。
```

* 翻頁：各 feed 的 List `AtEnd` 各自觸發（For You／Following 走 `Home`/`HomeLatest`＋seen；Search 走 `Search` 無 seen）。
* tab 切換：僅切換活躍指標，狀態保留（同帳號內；跨帳號由墓碑統一歸零）。
* 搜尋：`app.searchQuery string`；提交 → `feedSearch` 歸零起跑；關閉（tabs 點回或清空）→ `feedSearch` 歸 nil 槽。搜尋結果不進 seen、不影響 For You／Following 的去重。
* 失效帳號的時間線：該帳號空時間線＋credential 錯誤文案＋「重新貼上 Cookie」按鈕（導 login 變體，更新該帳號 cookie 後原地重試切換）——不移除 secret。

### 8.8 佈局與響應式（D20）

頂列（`ui.Toolbar`）＋內容區：

```
┌ Toolbar ────────────────────────────────────────────────┐
│ [帳號 MenuButton ▾]   [Tabs: For You|Following]   [搜尋框] │
├─────────────────────────────────────────────────────────┤
│ <768：單欄時間線（帳號→Select、tabs 保持、搜尋框收窄）        │
│ 768–1023：Sidebar(帳號清單) │ 時間線（tabs 移入側欄或保持頂列）│
│ 1024–1199：Sidebar │ 時間線 │ 展開檢視並列（點帖右欄開）      │
│ ≥1200：同上，卡片 max-width 640 置中、兩側留白              │
└─────────────────────────────────────────────────────────┘
```

* 判斷：`w, _ := c.Size()`（DIP）＋`Window.OnResize` → `Invalidate`（core.txt:4584）；斷點常數 768/1024/1200。
* 帳號選單：`ui.MenuButton`（寬窗恆顯清單＋新增＋移除；窄窗 `ui.Select` 同構項）。搜尋框：`ui.TextField`＋`Submitted()`。
* 展開檢視雙形態：≥1024 右欄並列（`a.detail *post` 非 nil）；<1024 `ui.Router` 頁 `/status/{id}`。兩形態共用同一 `detailView` 建構函式與播放器生命週期（§8.6）。
* 測試：`Tester.SetSize(768/1024/1200)` 三寬度斷言（單欄有 Sidebar 隱藏、雙欄出現、並列出現）。

### 8.9 錯誤面

無新 `kind`（v1 六型別足夠）——新增的是**同 kind 的 app 層文案**（messages.go）：

| 情境 | kind | 文案要點 |
|---|---|---|
| 切換時 Viewer 失敗 | credential | 「@X 的 Cookie 已失效——重新貼上 Cookie 或移除此帳號」；帳號標記失效 |
| 索引/帳號 secret 損毀 | material | 「帳號資料無法讀取（系統憑證庫）」；不靜默清空 |
| 媒體下載/解碼失敗 | —（app 層） | 縮圖佔位可重試；展開檢視顯示「無法載入媒體」＋外開連結 |
| 帳號數已滿 | —（app 層） | 「最多 5 個帳號——請先移除一個」 |

### 8.10 併發與生命週期

* context 樹：`state.ctx`（生命週期）→ `sessCtx`（活躍會話；切換 cancel）→ feed 工作與媒體下載皆其子樹。
* 主執行緒紀律：goroutine 絕不改 `app`——一律 `apply`（Window.Update）；`thumbStore` 的讀寫都在主執行緒段（`get`/`put` 由 apply 執行），信號量計數用 `sync` 於 goroutine 側。
* quit flush：`OnBeforeQuit` → 活躍帳號 store flush（`activeStore`）→ `sess.Close()` → AppState 收斂（順序不變，目標帳號化）。
* `-race` 進 CI（沿 v1）。

## Alternatives Considered

已於簡報 §Alternatives Weighed 十二案完整載明（ui/video 延後否決、GIF mp4 採用、重驗方案甲＋墓碑修訂、工廠化不新增抽象、LRU 先量測、水平響應式否決延後、媒體獨立出口、oxfmt 移除、Secrets 無 List、80 MiB 字面矛盾、媒體降級退路、web 暫緩）。本文不重複。

## Non-Functional Requirements

| 項 | 驗收線 | 量測場景 |
|---|---|---|
| RAM（行程峰值） | <80 MiB | (a) 閒置；(b) 100 帖含縮圖捲動到底；(c) 展開檢視播放影片中 |
| 切換延遲 | TTL 內 ≤500 ms（無網路）；TTL 外 ≤3 s（含 Viewer） | 5 帳號輪切 |
| 縮圖併發 | ≤4 | 捲動含媒體 feed |
| seen 上限 | 500 | 長翻頁 |
| 每帳號 secret | ≤2560 bytes | 整份 header 貼上時的錯誤路徑 |
| 測試 | `go test -race` 綠（9+ packages）；web 編譯冒煙；gofumpt/golangci-lint 綠 | 每 commit |

## Detailed Implementation Plan

### P0' 前置閘（探測輪——臨時程式 `X/cmd/probe11/`，結論回寫 DECISIONS 後可留作維運工具）

1. **HomeLatestTimeline**：以引擎 sender 實測（掛真實 cookie）——臂一：空 features GET，n≥20；臂二：空 features POST（cursor/seen）；臂三：宣告 features（自前端 chunk 抽取）重跑失敗臂。判準（D1 判準）：全數 200 且 JSON 含 `data` → 採空 features 形態；部分失敗 → features 回填；全失敗 → D16 登錄降級（v1.1 單時間線）。
2. **SearchTimeline**：自 X 前端 main chunk 重建 queryId map（v1 探測節同法）→ 驗證恆 POST n≥20（簡單 query 一枚、含 `since:` 過濾一枚）。判準同上。
3. **媒體 CDN**：自探測回應實抓縮圖（`name=small`）n≥10、mp4 variant n≥5——驗證免認證與 Content-Type（D22 收尾）。
4. **撤回虛假引用**：DECISIONS「web 已實證」改為「web 編譯冒煙（2026-11-13 起）＋資料面未驗證（CORS/TLS）」。

### C1 `feat(accounts)`——帳號資料模型與儲存

* `X/accounts.go`：Account／accountIndex／loadAccounts/saveAccount/saveIndex/deleteAccount／遷移序列／雙寫。
* `X/session.go`：廢 `engineStore`；`newSessionFor`；`xSession` 增 `HomeLatest`/`Search`/`Close`（engine 側先以 stub 實作 Search/HomeLatest？——否：C1 只做帳號域，stub 於 C3/C4 補足；C1 的 `newSessionFor` 用 `HomeTimeline` 即可測）。
* `X/main.go`：`a.accounts`／warmup 接線／quit flush 目標帳號化。
* 測試：遷移三態（無舊鍵/有舊鍵+Viewer 成功/有舊鍵+Viewer 失敗）、同 UserID 覆蓋（REQ-WEB-070）、secret 上限錯誤路徑。
* 退出：遷移測試綠；`go test -race` 綠。

### C2 `feat(session)`——切換序列與世代邊界

* `X/main.go`：`switchTo`／`gen`／`sessCtx`／`credCache`／`accInvalid`／帳號選單動作（新增/切換/移除）。
* `X/app_state.go`：`GoSession`（per-session context 子樹）。
* `X/session.go`：`engineSession.tr`＋`Close()`。
* 測試：切換成功（transport Close 被呼叫、gen+1、feeds 歸零）、切換失敗（回滾、原會話不動、標記失效）、TTL 免重驗、in-flight publish 世代過期即棄、crash 安全（索引先寫）。
* 退出：切換流測試綠；世代邊界測試綠。

### C3 `feat(timeline)`——雙時間線

* P0' 結論落 `X/static/graphql.json`＋`api/timeline.go`：`HomeLatestTimeline`（形態依探測）。
* `X/view_home.go`：`feeds [3]*timelineState`／`ui.Tabs`／seen 截尾／各 feed AtEnd。
* 測試：合成 fixture（`HomeLatestTimeline` 沿用 `tlItem` 家族）、雙 tab 狀態隔離、seen 上限。
* 退出：fixture 測試綠；雙 tab 隔離測試綠。

### C4 `feat(search)`——搜尋

* `X/static/graphql.json`＋`api/timeline.go`：`SearchTimeline`（恆 POST）。
* `X/view_search.go`：頂列搜尋框接線、`feedSearch` 生命週期（查詢時存在、關閉即棄）。
* 測試：fixture 測試、搜尋不污染 For You seen。
* 退出：fixture 測試綠。

### C5 `feat(media)`——媒體解析與縮圖

* `X/internal/xapi/api/media.go`＋`timeline.go`：`Post.Media`／萃取規則（§8.2）。
* `X/media.go`：`fetchImage`／`thumbStore`／併發信號量。
* `X/view_home.go`：列表縮圖格（佔位→載入→重試）。
* 測試：萃取 fixture（photo/video/gif/缺 entities/mp4 選擇）、LRU 逐出、失敗佔位。
* 退出：萃取測試綠；RAM 量測 (b) <80 MiB。

### C6 `feat(detail,ui)`——展開檢視、頂列佈局、響應式、格式化

* `X/view_detail.go`：展開檢視（大圖／`video.OpenURL`＋`ui.Video`／Close 生命週期／Route 與右欄雙形態）。
* `X/view_home.go`＋`main.go`：頂列（Toolbar＋MenuButton/Select＋TextField）＋768/1024/1200 斷點＋`Window.OnResize`。
* `X/messages.go`：§8.9 文案。
* 格式化：`cd X` → `gofumpt -w -extra .`（現況 6 檔）→ `golangci-lint run` 全綠。
* 測試：`Tester.SetSize` 三寬度、播放器 Close 呼叫序列、媒體失敗文案。
* 退出：全部驗收場景綠；雙平台建置（windows-amd64＋web 冒煙）。

### 收尾

* `docs/DECISIONS.md`：D10 歸屬正規化、D11–D22 定稿（含 P0' 探測結論、TRN-010 偏離、web 引用撤回、定位句放寬＋REQ×版本對照表）、L2 發現節回註。
* `docs/designs/design_draft.md` 加 Superseded 標記（避免雙拷貝歧義）。
* README：v1.1 能力與 SemVer 對應、web 定位（測試載體）。
* 計畫檔：`docs/superpowers/plans/2026-11-14-x-engine-v1.1.md` 隨本文 commit 入庫。

### Target Files to Create/Modify

| 檔案 | 動作 | 域 |
|---|---|---|
| `X/accounts.go`＋`accounts_test.go` | Create | C1 |
| `X/session.go` | Modify | C1/C2 |
| `X/main.go` | Modify | C1/C2/C6 |
| `X/app_state.go` | Modify | C2 |
| `X/view_home.go`＋`view_home_test.go` | Modify | C3/C5/C6 |
| `X/internal/xapi/api/timeline.go`＋`timeline_test.go` | Modify | C3/C4/C5 |
| `X/internal/xapi/api/media.go`＋`media_test.go` | Create | C5 |
| `X/static/graphql.json` | Modify | C3/C4（P0' 後） |
| `X/media.go`＋`media_test.go` | Create | C5 |
| `X/view_search.go` | Create | C4 |
| `X/view_detail.go` | Create | C6 |
| `X/messages.go` | Modify | C6 |
| `X/cmd/probe11/` | Create（臨時工具） | P0' |
| `docs/DECISIONS.md`／`README.md` | Modify | 收尾 |

## Verification Plan

1. **單元**：每域測試（§8 各節）`-race` 綠；遷移三態；切換收斂；世代隔離；萃取規則；LRU；播放器生命週期。
2. **佈局**：`Tester.SetSize(768/1024/1200)` 斷言三形態。
3. **L2 驗收（真實憑證，需求方提供）**：
   - 帳號：新增兩帳號 → 來回切換 ×5（TTL 內外各半）→ 資料零交叉污染（肉眼＋錄影）；失效 cookie 呈現與復原。
   - 時間線：For You／Following 各 10 頁（298 帖級）零跨頁重複；seen ≤500。
   - 搜尋：兩查詢詞，結果正確、關閉後 For You 狀態不變。
   - 媒體：含圖/影/GIF 的 feed 捲動 100 帖——縮圖全載、無佔位滯留；展開播放 mp4 有聲畫同步；離開後 RAM 回落。
   - RAM：三場景峰值 <80 MiB（L2 法：閒置/捲動/播放）。
   - 切換延遲：TTL 內 ≤500 ms、外 ≤3 s。
4. **建置**：windows-amd64 出貨（graphql.json 隨 exe）＋web 編譯冒煙。
