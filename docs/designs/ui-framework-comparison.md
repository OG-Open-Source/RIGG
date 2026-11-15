# MyGo UI 框架對比（v1.1.5 後的基底裁決輸入）

* **Date:** 2026-11-16
* **Status:** 對比完成——待需求方裁決基底框架後，進重構計畫
* **方法：** 六候選全數取得一手證據——五框架＋原版**同模組並存編譯冒煙（native＋wasm 雙建置全綠，mygo v0.9.0 下互相無衝突）**；模組 go.mod／LICENSE／commit 活躍度（atom feed）／skill 文件／套件結構逐一核對。

## 候選總表

| 框架 | 版本/狀態 | mygo 相容 | 授權 | 活躍度 | wasm | 定位 |
|---|---|---|---|---|---|---|
| **原版 egoist/mygo** | v0.9.0（釘選） | —（基底） | MIT（egoist） | 上游活躍 | ✅（長期驗證） | 原生 widget 層＋Tester——我們的既有基底 |
| **Lobe MyGo**（現用） | v0.0.0-20261110 | ≥v0.8.4-20261108 ✅（實測） | MIT | 活躍 | ✅（實測） | 設計語言＋補位元件（Alert/Skeleton/Empty/Image 預覽/Toast） |
| **Cyenoch/mygoui** | v0.3.1（tag＝main 頭） | **v0.9.0 精確對準**（docs/mygo-0.9.md） | MIT（JGBingZi） | 活躍 | ✅（實測） | typed-options 元件庫＋**視窗外殼（Scaffold/PaneToggle）**＋flyout_host＋shaders＋locale/fonts＋**重度測試文化**（74 檔 form 含測試） |
| **Pylogmon/shadcn-ui-mygo** | 活躍頭（昨日 commit） | **v0.9.0 精確對準** | MIT＋完整第三方 notices 紀律 | **極活躍**（日多 commit） | ✅（實測） | **最完整 shadcn 移植**：theme(OKLCH+偏好儲存)/control/display/overlay/navigation/**desktop(原生視窗 flyout)**/**conversation(訊息/氣泡/附件/MessageScroller 虛擬捲動)**/charts(70 例)/fonts(26 族)/motion |
| **oneo-me/onevra/source** | v0.2026.18 | v0.7.4（舊） | 未確認 | 低 | ✅（實測） | 小型設計實驗（chart/chat/command/flow/gesture/graph/shaders/treemap＋2 shell） |
| **xingkaixin/mygo-native-ui** | v0.4.0 | v0.8.1+ ✅（實測） | MIT（XingKaiXin） | 活躍 | ✅（實測） | **100+ 專業 widget 集**：ui/**player**（媒體傳輸控制）、ui/gif（disposal/循環）、ui/lightbox（縮放預覽）、ui/carousel（多圖輪播）、ui/thumbgrid（背景解碼縮圖格）、ui/skeleton、ui/settings、ui/swipeactions、ui/palette... |

## 對 X 客戶端需求的面相對比

| 需求 | 原版 | Lobe（現用） | mygoui | shadcn-ui-mygo | onevra | mygo-native-ui |
|---|---|---|---|---|---|---|
| 虛擬化 feed 列表 | ✅ ui.List＋Key | —（用原生） | ListBox（非虛擬化？待驗） | MessageScroller（虛擬捲動＋錨定＋跳轉） | — | thumbgrid（背景解碼格） |
| 設計系統/token | — | ✅ theme.Apply＋Tokens | ✅ theme＋Space* | ✅ OKLCH＋作用域主題＋偏好儲存 | theme | — |
| 錯誤橫幅/骨架/空狀態 | — | ✅ Alert/Skeleton/Empty | ✅ Banner/Skeleton/EmptyState | ✅ Alert/Skeleton/Empty（13+ 配置） | — | ✅ Skeleton |
| 圖片預覽（縮放/平移） | — | ✅ Image viewer | —（待查） | —（待查） | — | ✅ lightbox（＋carousel 多圖） |
| **影片播放控制** | VideoPlayer 原語 | — | — | — | — | ✅ **ui/player**（scrubber/音量/速度/章節） |
| GIF 播放 | ✅ NewGIFDecoder（已用） | — | — | — | — | ✅ ui/gif（disposal 完整） |
| 多圖輪播 | — | — | — | —（待查） | — | ✅ carousel（彈簧吸附） |
| 帳號側欄/視窗外殼 | Sidebar/MenuButton | ✅ DraggableSideNav（現用） | ✅ **Scaffold/PaneToggle/SplitView** | ✅ Sidebar＋desktop flyout | shells/sidebar | settings（列式） |
| 帳號下拉/選單 | MenuButton/Select | —（待查） | ✅ ComboBox/overlay | ✅ DropdownMenu（27 配置）＋desktop 原生 | — | — |
| 訊息/貼文呈現 | — | — | — | ✅ **conversation 套件**（Message/Bubble/Attachment/Marker） | chat | — |
| 測試文化 | ✅ Tester | — | ✅ 每元件測試 | ✅ parity tests＋消費者驗證 | — | — |

## 各候選強弱（對本專案）

* **原版**：基底不可動（engine 邊界、Tester、虛擬化）；缺設計系統與媒體 UX。
* **Lobe（現用）**：已運作、淺、學習成本低；缺捲動 UX、媒體控制、視窗外殼；元件面中等。
* **mygoui**：**測試文化最重**（幾乎每元件一測試檔）、Scaffold 視窗外殼獨有、typed options 一致性佳；v0.9.0 精確對準有專章（docs/mygo-0.9.md）。媒體面缺。
* **shadcn-ui-mygo**：**覆蓋最廣＋工程紀律最高**（parity tests、消費者驗證、第三方 notices 逐項 manifest）；conversation 套件的 MessageScroller（虛擬捲動＋錨定＋跳轉）直擊 D30 的捲動 UX 痛點；desktop 原生 flyout 為獨有。風險：API 面大、破壞性變更頻繁（近期有 ! commits）、wasm 雖實測通過但元件深度使用未驗。
* **onevra**：面窄、mygo 版本舊——不採。
* **mygo-native-ui**：**媒體與專業 widget 的即戰力**（player/lightbox/carousel/gif/thumbgrid）；無設計系統（需配 theme 層）；單一維護者。

## 建議（待需求方裁決）

* **案甲（建議）——混合基底，漸進重構**：保留原生 ui.List＋Lobe theme 為底，**引入 shadcn-ui-mygo 的 conversation＋ScrollArea**（捲動 UX 重構的主答案）＋**xingkaixin 的 ui/player＋ui/lightbox＋ui/carousel＋ui/gif**（媒體控制列與多圖，直接兌現 D30 播放器控制列）。五框架並存已實證；重構面集中在 detail 媒體區與捲動行為，feed 列表結構不動。
* **案乙——全量換底 shadcn-ui-mygo**：theme/conversation/ScrollArea/Modal/Toast 全換 shadcn，Lobe 退役。一致性最高；工程面最大（theme 遷移＋全部測試改寫），且承受其 API 破壞性變更節奏。
* **案丙——維持現狀＋Lobe 補齊**：捲動 UX 與播放控制以 Lobe/原生手寫。工程最小，但 D30 兩項痛點沒有現成答案。

## 發布註記

現況（cf56e45＋後續 fix）已由需求方裁定可發布 `X/v0.2.0`（D30）；本對比與重構屬 v1.2 輪，不阻塞 tag。
