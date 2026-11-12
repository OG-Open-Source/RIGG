# XApis 行為規格書（Clean-Room Behavioral Specification）

> 本文件是 `XApis-master/` 的唯一行為參考。任何後續實作、審查、測試或移植，**必須**只依據本文件描述的可觀察行為，不得複製原專案原始碼、註解、識別字命名風格或內部實作細節。

---

## 文件控制

| 項目 | 值 |
| --- | --- |
| 文件識別碼 | `XAPIS-BEHAVIOR-SPEC` |
| 版本 | 1.0.0 |
| 狀態 | 凍結（Frozen）— 作為下一輪輸入的唯一參考 |
| 產出日期 | 2026-11-10 |
| 來源基線 | `XApis-master/`（118 個檔案，目錄快照） |
| 來源專案 | XApis（`https://github.com/cv-cat/XApis`） |
| 文件語言 | 繁體中文；所有識別碼、路徑、鍵名、錯誤字串保留原文 |
| 檔案編碼 | UTF-8（無 BOM），LF 換行 |
| 文件授權 | 僅保留署名權（Attribution-only）；見〈來源、授權與致謝〉 |
| 規範依據 | RFC 2119、RFC 8174（規範性關鍵詞）；RFC 3339（時間格式） |

### 修訂紀錄

| 版本 | 日期 | 變更 |
| --- | --- | --- |
| 1.0.0 | 2026-11-10 | 首次由 `XApis-master/` 提取完整行為規格 |

---

## 目錄

- [文件控制](#文件控制)
- [1. 範圍與目的](#1-範圍與目的)
  - [1.1 目的](#11-目的)
  - [1.2 範圍內](#12-範圍內)
  - [1.3 範圍外](#13-範圍外)
  - [1.4 讀者與使用方式](#14-讀者與使用方式)
- [2. 規範性語言與慣例](#2-規範性語言與慣例)
- [3. Clean-Room 章程](#3-clean-room-章程)
  - [3.1 允許的輸入](#31-允許的輸入)
  - [3.2 禁止的行為](#32-禁止的行為)
  - [3.3 衍生作品邊界](#33-衍生作品邊界)
  - [3.4 證據等級](#34-證據等級)
- [4. 系統概觀](#4-系統概觀)
  - [4.1 元件地圖](#41-元件地圖)
  - [4.2 信任邊界](#42-信任邊界)
  - [4.3 執行模式](#43-執行模式)
- [5. 術語與正規化資料型別](#5-術語與正規化資料型別)
- [6. 外部介面契約](#6-外部介面契約)
  - [6.1 X Web 介面](#61-x-web-介面)
  - [6.2 Go TLS 閘道 RPC](#62-go-tls-閘道-rpc)
  - [6.3 AI 供應商契約](#63-ai-供應商契約)
  - [6.4 資訊來源契約](#64-資訊來源契約)
  - [6.5 MySQL 結構](#65-mysql-結構)
- [7. 行為規格：核心 SDK](#7-行為規格核心-sdk)
  - [7.1 傳輸層與瀏覽器指紋](#71-傳輸層與瀏覽器指紋)
  - [7.2 鑑權狀態](#72-鑑權狀態)
  - [7.3 請求標頭建造](#73-請求標頭建造)
  - [7.4 GraphQL 操作註冊表與參數裝配](#74-graphql-操作註冊表與參數裝配)
  - [7.5 XCTID 純算](#75-xctid-純算)
  - [7.6 Castle token](#76-castle-token)
  - [7.7 登入](#77-登入)
  - [7.8 讀取 API](#78-讀取-api)
  - [7.9 寫入 API](#79-寫入-api)
  - [7.10 媒體 API](#710-媒體-api)
  - [7.11 文章 API 與 Markdown 映射](#711-文章-api-與-markdown-映射)
  - [7.12 X Chat 私信 API](#712-x-chat-私信-api)
  - [7.13 資料處理工具](#713-資料處理工具)
  - [7.14 命令列介面](#714-命令列介面)
  - [7.15 快速開始腳本](#715-快速開始腳本)
- [8. 行為規格：Web 控制台](#8-行為規格web-控制台)
- [9. 行為規格：前端工作台](#9-行為規格前端工作台)
- [10. 行為規格：Go 閘道與部署](#10-行為規格go-閘道與部署)
- [11. 橫切不變量](#11-橫切不變量)
- [12. 錯誤分類與重試語義](#12-錯誤分類與重試語義)
- [13. 設定矩陣](#13-設定矩陣)
- [14. 驗收準則](#14-驗收準則)
- [15. 符合性與驗證矩陣](#15-符合性與驗證矩陣)
- [16. 已知限制與排除項](#16-已知限制與排除項)
- [17. 對 OG-Open-Source README 標準的符合性登錄](#17-對-og-open-source-readme-標準的符合性登錄)
- [18. 需求索引](#18-需求索引)
- [19. 來源、授權與致謝](#19-來源授權與致謝)

---

## 1. 範圍與目的

### 1.1 目的

本文件把 `XApis-master/` 的可觀察行為，轉換為**可獨立重寫**的規範性敘述。其判準是：一位未接觸原始碼的實作者，只讀本文件，即應能重建出在下列面向與原專案等價的系統：

1. 對 X（Twitter）Web 端點的請求形態與鑑權語義。
2. 登入、讀取、寫入、媒體、文章、私信各鏈路的輸入輸出與失敗分類。
3. Web 控制台的 HTTP 介面、持久化模型、配額與養護狀態機。
4. 部署拓撲、設定鍵與其優先序。
5. 測試所固化的驗收行為。

### 1.2 範圍內

| 代號 | 子系統 | 來源路徑 |
| --- | --- | --- |
| `TRN` | 傳輸、指紋、閘道客戶端 | `builder/client.py`、`builder/gateway.py`、`utils/fingerprint.py` |
| `AUTH` | 鑑權狀態容器 | `builder/auth.py` |
| `HDR` | 請求標頭建造與排序 | `builder/header.py` |
| `GQL` | GraphQL 註冊表與參數 | `builder/params.py`、`utils/graphql_registry.py`、`utils/frontend.py` |
| `XCT` | XCTID 純算 | `utils/transaction.py` |
| `CST` | Castle token（Node 回放／純算） | `utils/castle_token.py`、`utils/castle_crypto.py`、`utils/castle_refresh.py`、`utils/chromium_deflate.py` |
| `LOG` | 登入流程 | `x_apis/login_api.py` |
| `RD` | 讀取 API | `x_apis/x_api.py` |
| `WR` | 寫入 API | `x_apis/x_write_api.py` |
| `MED` | 媒體上傳 | `x_apis/x_media_api.py` |
| `ART` | 文章與 Markdown 映射 | `x_apis/x_article_api.py`、`utils/article_util.py` |
| `DM` | X Chat 私信 | `x_apis/x_dm_api.py`、`console/chat.py` |
| `DAT` | 資料處理與匯出 | `utils/data_util.py`、`utils/x_util.py` |
| `CLI` | 命令列與快速開始 | `main.py`、`quickstart.py` |
| `WEB` | Web 控制台後端 | `console/` |
| `FE` | 前端工作台 | `console/static/`、`frontend/` |
| `GW` | Go TLS 閘道 | `gateway/` |
| `DEP` | 容器與編排 | `Dockerfile`、`compose*.yaml`、`scripts/` |

### 1.3 範圍外

1. `static/graphql.json`、`static/transaction_l1.json`、`static/castle/*` 的**內容資料**（屬線上擷取素材，非行為）。
2. 第三方元件的內部行為：Castle.io SDK、`chat-xdk`、curl_cffi 的 TLS 實作、MySQL、Go `tls-client`。
3. 任何憑證、Cookie、代理位址、金鑰的實際值。
4. X 平台自身的風控、驗證碼與設備證明政策。
5. 圖形資產（`assets/xapis-logo.png`）。

### 1.4 讀者與使用方式

1. 先讀第 3 章（Clean-Room 章程）與第 5 章（術語），再進入第 7 章之後。
2. 實作時以需求識別碼（`REQ-<AREA>-<NNN>`）為工作單元；第 18 章提供索引。
3. 驗收時以第 14 章與第 15 章為準；不得以「與原始碼相同」作為驗收理由。

---

## 2. 規範性語言與慣例

1. 本文件使用 RFC 2119／RFC 8174 關鍵詞，對應如下，且僅以大寫中文詞呈現時具規範力：

| 關鍵詞 | 英文 | 強度 |
| --- | --- | --- |
| 必須 / 不得 | MUST / MUST NOT | 絕對要求 |
| 應 / 不應 | SHOULD / SHOULD NOT | 強烈建議，例外需記錄理由 |
| 可 | MAY | 允許選項 |
| 需要 | REQUIRED | 前置條件 |

2. 識別碼格式：`REQ-<AREA>-<三位數>`；`AREA` 取自 1.2 表。
3. 路徑一律以 `/` 分隔並相對於專案根目錄；X 端點以絕對 URL 表示。
4. 時間：對外介面使用 RFC 3339／UTC；控制台配額以**北京時間（UTC+8）自然日**計算。
5. 位元組單位：`KiB = 1024`、`MiB = 1024²`；媒體上限以十進位 `MB` 表述時沿用來源文件的 `50 MB`。
6. 欄位名、環境變數、路由、錯誤字串**必須逐字保留**（含大小寫與底線）。
7. 表格中「證據」欄為事實溯源，不代表可複製該處程式碼。

---

## 3. Clean-Room 章程

### 3.1 允許的輸入

1. 本文件。
2. 公開可得的**介面事實**：HTTP 方法、路徑、標頭名稱、JSON 欄位名、狀態碼、GraphQL 操作名與 `queryId`。
3. 公開標準：RFC 2119／8174、RFC 3339、Semantic Versioning 2.0.0、Conventional Commits 1.0.0、Keep a Changelog 1.1.0、twitter-text v3 計重規則、Draft.js 資料形狀（公開文件所述部分）。
4. 第三方元件的公開授權與 API 文件。

### 3.2 禁止的行為

1. 不得複製、改寫或翻譯原專案的函式體、類別結構、私有輔助函式、註解文字。
2. 不得沿用原專案的檔案內部組織作為設計理由；檔案佈局可不同。
3. 不得把 `static/` 內的擷取素材（`graphql.json`、`transaction_l1.json`、`castle/*`）視為規格；它們是**可替換的執行期資料**。
4. 不得在文件中記錄任何真實憑證、Cookie 值、代理認證或金鑰。
5. 不得以本文件為由，重製第三方元件的授權檔案或原始碼。

### 3.3 衍生作品邊界

| 類別 | 處置 |
| --- | --- |
| 原專案自有程式碼之行為 | 可自由重寫；**應**在衍生作品標註 `XApis` 與 `OG-Open-Source` 署名 |
| 第三方元件（Castle SDK、`chat-xdk`、curl_cffi、Go 模組） | 各自授權約束；不得視為 MIT；不得修改其授權檔 |
| X 平台介面事實 | 屬事實性資料，不受著作權保護；仍**應**標註來源 |
| 文件本身 | 僅保留署名權；引用時**必須**標註 `OG-Open-Source` |

### 3.4 證據等級

| 等級 | 定義 | 本文件用法 |
| --- | --- | --- |
| E1 | 由多個測試或文件交叉確認 | 標記「已驗證」 |
| E2 | 由單一實作路徑確認 | 預設等級 |
| E3 | 由註解或文件名稱推得，未見執行路徑 | 標記「推定」 |
| E4 | 平台行為，無法由本專案保證 | 標記「外部依賴」 |

---

## 4. 系統概觀

### 4.1 元件地圖

```text
┌──────────────────────────── 使用者介面 ────────────────────────────┐
│  CLI (main.py / quickstart.py)      Web 控制台 (console/static)     │
└───────────────┬───────────────────────────────┬────────────────────┘
                │ Python 呼叫                    │ HTTP + CSRF
                ▼                                ▼
┌──────────────────────────────┐   ┌──────────────────────────────────┐
│  SDK 層                      │   │  console.app（FastAPI，單 worker）│
│  x_apis/{x,write,media,      │◄──┤  帳號／佇列／養護／文章／私信     │
│  article,dm,login}_api       │   │  MySQL 8.4 持久化                 │
│  builder/{client,header,     │   └───────────────┬──────────────────┘
│  params,auth,gateway}        │                   │ 內部 RPC
│  utils/{transaction,castle,  │                   ▼
│  graphql_registry,article}   │   ┌──────────────────────────────────┐
└───────────────┬──────────────┘   │  Go TLS 閘道（tls-client）        │
                │                   │  帳號＋代理連線池、共享密鑰        │
                ▼                   └───────────────┬──────────────────┘
        curl_cffi（Chrome 150 TLS 模板）             │ HTTP/SOCKS5
                └───────────────────┬───────────────┘
                                    ▼
                            X Web 端點（x.com / api.x.com / jf.x.com / upload.x.com）
```

### 4.2 信任邊界

| 邊界 | 內側 | 外側 | 控制 |
| --- | --- | --- | --- |
| B1 | Web 控制台 | 瀏覽器 | HttpOnly／SameSite 會話 Cookie、CSRF、來源檢查、scrypt 密碼雜湊 |
| B2 | Web 控制台 | MySQL | 應用層 Fernet 加密敏感欄位；資料庫憑證以檔案注入 |
| B3 | Web 控制台 | Go 閘道 | 共享 Bearer 密鑰；閘道不對宿主映射埠 |
| B4 | Go 閘道 | X | 帳號專屬代理；**不得**回退直連 |
| B5 | Web 控制台 | AI／資訊源 | 拒絕內網與 loopback；固定 DNS 解析 IP；限制回應大小；不跟隨重定向 |
| B6 | 瀏覽器 | X 金鑰伺服器（私信解鎖） | CSP 禁止直連；由 Worker 經受控後端通道 |

### 4.3 執行模式

| 模式 | 觸發 | 必要條件 | 行為 |
| --- | --- | --- | --- |
| `SDK-COOKIE` | `load_env()` 讀到 `X_COOKIES` | `auth_token` 與 `ct0` 同時存在 | 直接以 Cookie 會話呼叫讀寫 API |
| `SDK-PASSWORD` | `main.py login` / `quickstart.py` | 帳密 + 完整 Castle profile | jetfuel 純算登入，成功後回寫 Cookie |
| `SDK-NODE` | 未提供 profile 且未強制純算 | 本機 `node` 可執行 | 以 Node 回放 Castle SDK 產生 token（相容模式） |
| `CONSOLE` | `uvicorn console.app:create_app` | MySQL、閘道密鑰、初始化令牌 | 多使用者 Web 工作台 |
| `TEST` | `compose.test.yaml` + `pytest` | 測試 MySQL、模擬 X／AI、測試 CA 與 CONNECT 代理 | 不觸及真實帳號 |

---

## 5. 術語與正規化資料型別

| 術語 | 定義 |
| --- | --- |
| `XAuth` | 會話狀態容器：bearer、proxy、cookie 映射、guest token、XCTID 產生器 |
| `auth_token` | X 會話 Cookie；存在即視為「已登入」 |
| `ct0` | CSRF Cookie；**必須**與 `x-csrf-token` 標頭逐字相同 |
| `twid` | 使用者識別 Cookie，形如 `u%3D<數字>` |
| `guest_token` | 匿名請求憑證；已登入會話不需要 |
| `XCTID` | `x-client-transaction-id`，每請求一次性簽章 |
| `Castle token` | 登入表單的 `$castle_token`，每步一次性 |
| `weight` | twitter-text v3 的推文計重值 |
| `long post` / `note tweet` | 權重 > 280 時使用的 `CreateNoteTweet` |
| `thread` | 多則推文逐條回覆串接 |
| `article` | X 文章（Draft.js `content_state`） |
| `maintenance` / 養護 | 帳號首次成功登入後的前 N 天限制期 |
| `quota day` | 北京時間自然日（UTC+8） |
| `source` | RSS／Atom 或靜態網頁熱點來源 |
| `rule` | 自動化規則：帳號 × 來源 × 提示詞 × 間隔 × 每日上限 |
| `pool` | 閘道內以「帳號 + 代理」為鍵的連線池 |

### 5.1 正規化型別

| 型別 | 定義 | 範例 |
| --- | --- | --- |
| `TweetId` | 純十進位數字字串 | `"1718802931036622848"` |
| `ScreenName` | 1–15 字元 `[A-Za-z0-9_]`，小寫 | `"xapis"` |
| `Cursor` | X 回傳的不透明字串 | `"DAABCgAB..."` |
| `MediaId` | 十進位字串 | `"1234567890"` |
| `ArticleId` | 十進位字串；relay id 為 `base64("ArticleEntity:<id>")` | `"1800000000000000000"` |
| `ProxyUrl` | `scheme://[user:pass@]host:port`；scheme ∈ {http, https, socks5, socks5h} | `socks5h://h:1080` |
| `Result3` | `(success: bool, message: str, payload)` | `(True, "成功", {...})` |
| `Result2` | `(ok: bool, message: str)` | — |

---

## 6. 外部介面契約

### 6.1 X Web 介面

| 主機 | 用途 | 備註 |
| --- | --- | --- |
| `https://x.com` | app shell、GraphQL（`/i/api/graphql`）、REST（`/i/api/1.1`）、文章編輯器 | 主要來源 |
| `https://api.x.com` | 舊 onboarding、X Chat GraphQL（`/graphql`） | 私信與 legacy 登入 |
| `https://jf.x.com` | jetfuel 登入（`/onboarding/web`） | 現行登入主路徑 |
| `https://upload.x.com` | 媒體分片上傳（`/i/media/upload.json`） | **不帶** XCTID |
| `https://abs.twimg.com` | webpack 分片、SDK 素材 | 公開靜態資源 |

#### 6.1.1 GraphQL 請求形態

1. `REQ-GQL-001`：查詢型操作**必須**以 `GET` 送出，參數置於 query string：`variables`、`features`（可選）、`fieldToggles`（可選）。
2. `REQ-GQL-002`：`variables`、`features`、`fieldToggles` **必須**以緊湊 JSON 序列化（分隔符 `,` 與 `:`，不轉義非 ASCII）。
3. `REQ-GQL-003`：mutation 型操作**必須**以 `POST` 送出 JSON body，鍵序為 `variables` → `features`（若該操作宣告）→ `fieldToggles`（若解析後非空）→ `queryId`。
4. `REQ-GQL-004`：`features` **不得**在該操作未宣告任何 feature switch 時出現（例如 `FavoriteTweet`、`CreateRetweet`）。
5. `REQ-GQL-005`：`HomeTimeline` 與 `SearchTimeline` 即使屬 query 型，**必須**以 `POST` 送出（URL 長度與現行前端行為）。
6. `REQ-GQL-006`：X 的 GraphQL 失敗可能以 HTTP 200 + `errors` 表達；讀取路徑僅在**完全沒有 `data`** 時拋錯，寫入路徑見到 `errors` 即拋錯。
7. `REQ-GQL-007`：錯誤物件**必須**保留 `operation_name`、`errors`、`res_json` 與首個錯誤的 `code`，訊息格式為 `<operation>: <message1>; <message2>`。

#### 6.1.2 已知錯誤碼

| code | 語義 | 處置 |
| --- | --- | --- |
| `32` | 會話失效 | 視為憑證無效，清除 Cookie 並要求重登 |
| `89` | 憑證無效 | 同上 |
| `215` | 驗證失敗 | 同上 |
| `326` | 帳號鎖定 | 同上 |
| `226` | 疑似自動化 | 退避，不重試 |
| `344` | 限流 | 退避，不重試 |
| `353` | CSRF Cookie 與標頭不匹配 | 重建會話標頭後重試一次 |

### 6.2 Go TLS 閘道 RPC

1. `REQ-GW-001`：閘道**必須**提供 `POST /v1/request`，以 `Authorization: Bearer <token>` 認證。
2. `REQ-GW-002`：請求體**必須**包含 `method`、`url`、`headers`（映射）、`body`（Base64）、`proxy`、`account`（連線池鍵）。
3. `REQ-GW-003`：成功回應**必須**為 `{ "status": <int>, "headers": {<name>: [<value>...]}, "body": "<base64>" }`。
4. `REQ-GW-004`：閘道**必須**將上游 `Set-Cookie` 以陣列形式保留，不得合併為單一字串。
5. `REQ-GW-005`：客戶端**不得**在閘道啟用時直連 X；代理缺失時**必須**失敗而非降級。
6. `REQ-GW-006`：客戶端**必須**將閘道錯誤碼映射為固定語義（見 12.2）。

### 6.3 AI 供應商契約

1. `REQ-WEB-AI-001`：**必須**支援 `POST <base>/chat/completions`，非串流，取 `choices[0].message.content`。
2. `REQ-WEB-AI-002`：輸出預算參數**必須**可選 `max_completion_tokens`（OpenAI 官方與推理模型）或 `max_tokens`（相容服務）。
3. `REQ-WEB-AI-003`：短貼文預算為 `4096`（新版參數）／`500`（舊版參數）；長文為 `8192`／`2000`；新版預算**包含**推理 token。
4. `REQ-WEB-AI-004`：**不得**強制送出 `temperature`；是否送出由「服務相容選項」決定。
5. `REQ-WEB-AI-005`：API Key 留空時**必須**保留既有金鑰；無金鑰的本地服務可首次留空。

### 6.4 資訊來源契約

1. `REQ-WEB-SRC-001`：RSS／Atom **必須**以 feed 解析器讀取；條目去重鍵為**條目連結**。
2. `REQ-WEB-SRC-002`：網頁來源**必須**只讀靜態 HTML，**不得**執行 JavaScript；去重鍵為**正文摘要雜湊**。
3. `REQ-WEB-SRC-003`：每次執行**最多**總結 3 條新內容。
4. `REQ-WEB-SRC-004`：來源位址**不得**指向內網、loopback 或链路本地位址；重定向**不得**跟隨（部署者應填最終位址）。

### 6.5 MySQL 結構

> 完整資料表、欄位、約束與索引見〈8.2 資料模型〉。

1. `REQ-WEB-DB-001`：**必須**使用 MySQL 8.4 與 InnoDB；每個資料庫**僅允許一個** Uvicorn worker。
2. `REQ-WEB-DB-002`：排程器**必須**以 MySQL 具名鎖確保全域唯一；失去鎖後**必須**重新取得才能續跑。
3. `REQ-WEB-DB-003`：原子領取與配額扣減**必須**以行鎖 + 交易完成。
4. `REQ-WEB-DB-004`：所有時間**必須**以 UTC 儲存，配額日界以 UTC+8 換算。

---

## 7. 行為規格：核心 SDK

### 7.1 傳輸層與瀏覽器指紋

#### 7.1.1 指紋檔案

| 需求 | 內容 |
| --- | --- |
| `REQ-TRN-001` | 程序內**必須**只有一份指紋檔案；螢幕尺寸在程序啟動後**不得**變動。 |
| `REQ-TRN-002` | 檔案**必須**包含：UA、`sec-ch-ua`、`sec-ch-ua-mobile`、`sec-ch-ua-platform`、`accept-language`、`client_language`、`timezone`、`screen_width`、`screen_height`。 |
| `REQ-TRN-003` | UA **必須**為 `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Safari/537.36`；`sec-ch-ua` **必須**為 `"Google Chrome";v="153", "Not_A Brand";v="8", "Chromium";v="153"`；platform 為 `"Windows"`；mobile 為 `?0`。 |
| `REQ-TRN-004` | `accept-language` **必須**為 `zh-CN,zh;q=0.9,en;q=0.8,zh-TW;q=0.7,ja;q=0.6`；`client_language` 為 `zh-CN`；`timezone` 為 `Asia/Shanghai`。 |
| `REQ-TRN-005` | 螢幕尺寸**必須**自 `(1920,1080)`、`(1536,864)`、`(1440,900)`、`(1366,768)` 均勻隨機選一，並以字串保存。 |
| `REQ-TRN-006` | **應**提供測試用重設入口，使下一次取用時重新抽樣。 |

#### 7.1.2 HTTP 客戶端

| 需求 | 內容 |
| --- | --- |
| `REQ-TRN-010` | 所有 X 請求**必須**經單一客戶端出口，不得由各 API 模組自建連線。 |
| `REQ-TRN-011` | 傳輸**必須**使用 curl_cffi 的 `chrome150` impersonate 模板；應用層 UA／Client Hints **必須**覆蓋為 7.1.1 的 Chrome 153 檔案。 |
| `REQ-TRN-012` | 每個請求**必須**補齊下列預設（呼叫端已給值則不覆蓋）：`user-agent`、`sec-ch-ua`、`sec-ch-ua-mobile`、`sec-ch-ua-platform`、`accept: */*`、`accept-language`、`priority: u=1, i`。 |
| `REQ-TRN-013` | **必須**移除僅屬導覽請求的 `upgrade-insecure-requests` 與 `sec-fetch-user`。 |
| `REQ-TRN-014` | **必須**設定 `sec-fetch-mode: cors`、`sec-fetch-dest: empty`。 |
| `REQ-TRN-015` | `sec-fetch-site` **必須**依目標主機推導：`x.com`／`www.x.com` → `same-origin`；`*.x.com`、`twitter.com`、`*.twitter.com` → `same-site`；其他 → `cross-site`。 |
| `REQ-TRN-016` | 若環境變數 `X_GATEWAY_URL` 非空，**必須**改走閘道客戶端（見 7.1.3）；否則直連。 |
| `REQ-TRN-017` | 直連時，代理映射中**必須**剔除所有以 `_` 開頭的內部鍵。 |
| `REQ-TRN-018` | 若 `X_REQUIRE_PROXY=1` 且缺少 `https` 代理，**必須**拋出 `必須綁定代理後才能請求 X`。 |
| `REQ-TRN-019` | 需要跨請求保存 Cookie 時（登入流），**必須**使用同一 session 物件。 |
| `REQ-TRN-020` | session 的 impersonate 版本**可**由呼叫端覆寫；登入流固定使用 `chrome150` 傳輸模板。 |

#### 7.1.3 閘道客戶端（Python 側）

| 需求 | 內容 |
| --- | --- |
| `REQ-TRN-030` | 啟用條件**僅**為 `X_GATEWAY_URL` 非空。 |
| `REQ-TRN-031` | 認證權杖**必須**先取 `X_GATEWAY_TOKEN`；為空時讀 `X_GATEWAY_TOKEN_FILE`（預設 `data/gateway.token`）；兩者皆空**必須**失敗。 |
| `REQ-TRN-032` | 請求**必須**先套用 7.1.2 的 fetch 標頭，再序列化為 RPC；`accept-encoding` **必須**固定為 `gzip`。 |
| `REQ-TRN-033` | **必須**要求 `https` 或 `http` 代理；缺失時錯誤訊息為 `X account proxy is required before login or any X request`。 |
| `REQ-TRN-034` | 連線池鍵**必須**優先採用代理映射的 `_account`；否則為代理字串的 SHA-256 十六進位。 |
| `REQ-TRN-035` | 逾時**必須**取 `timeout`（預設 30 秒）；若提供 `_deadline`（單調時鐘秒數），**必須**取兩者較小值；已逾期**必須**拋 `login_timeout`。 |
| `REQ-TRN-036` | **必須**在送出前呼叫 `_on_request(method, url)`；回應後**必須**呼叫 `_on_response(method, url, response, elapsed, field_names)`，其例外**不得**中斷主流程。 |
| `REQ-TRN-037` | **必須**自回應標頭重建 `Set-Cookie`，並同時寫入回應物件與 session 的 Cookie jar。 |
| `REQ-TRN-038` | **必須**在回應物件附掛診斷資訊：`csrf_matches_cookie`、`cookie_names`、`set_cookie`（名稱與是否清除）、`cookie_sync`、`cookie_sync_result`。 |
| `REQ-TRN-039` | 若提供 `_on_cookies(url, cookie_lines)`，**必須**以原始 `Set-Cookie` 行陣列呼叫；其例外**必須**在回應處理完成後重新拋出。 |
| `REQ-TRN-040` | 重定向**僅**允許 `GET`；**必須**最多 5 跳；跨主機時**必須**移除 `authorization`、`cookie`、`x-csrf-token` 並丟棄 cookies。寫入請求**不得**重放。 |
| `REQ-TRN-041` | 回應物件**必須**提供 `status_code`、`reason`、`url`、`content`、`text`（UTF-8）、`headers`（大小寫不敏感，多值以 `, ` 合併）、`cookies`、`request`。 |

### 7.2 鑑權狀態

| 需求 | 內容 |
| --- | --- |
| `REQ-AUTH-001` | 鑑權狀態**必須**是動態憑證的唯一持有者；API 方法**不得**自行拼裝 Cookie 或簽章素材。 |
| `REQ-AUTH-002` | 建構時**必須**接受 `bearer`（預設公開 web bearer）與 `proxies`，並初始化空的 cookie 映射、guest token、XCTID 快取。 |
| `REQ-AUTH-003` | `prepare_auth(cookies_str)` **必須**解析 `k=v; k=v` 形式，容忍空字串、多餘空白與結尾分號。 |
| `REQ-AUTH-004` | 若解析後無 `ct0` 且有 `auth_token`，**必須**拋出 `ValueError`，訊息為 `cookie 裡有 auth_token 但缺 ct0。已登入會話的 ct0 由服務端簽發，不能本地生成，請把瀏覽器 cookie 完整複製過來`。 |
| `REQ-AUTH-005` | 若無 `ct0` 且無 `auth_token`，**必須**本地產生 32 位小寫十六進位 `ct0`。 |
| `REQ-AUTH-006` | `lang` Cookie **必須**預設為指紋檔案的 `client_language`；該值同時決定 `x-twitter-client-language` 標頭。 |
| `REQ-AUTH-007` | `is_logged_in` **必須**等價於 `auth_token` 非空。 |
| `REQ-AUTH-008` | `user_id` **必須**自 `twid` 去除 `u%3D`、`u=` 與引號後取得。 |
| `REQ-AUTH-009` | `guest_token` **必須**惰性取得；快取有效期為 `3 × 3600` 秒；過期或未取得時**必須**重新兌換。 |
| `REQ-AUTH-010` | 兌換**必須**以 `POST https://api.x.com/1.1/guest/activate.json`、FORM 標頭與 bearer 發出，取回應 `guest_token`，並寫回 `gt` Cookie 與 cookie 字串。 |
| `REQ-AUTH-011` | XCTID 產生器**必須**惰性建立；優先自網路取得素材，失敗時**必須**回退至本地快取素材。 |
| `REQ-AUTH-012` | 建立網路素材時，代理映射中**必須**剔除 `_on_cookies`，避免匿名素材請求回寫帳號 Cookie。 |
| `REQ-AUTH-013` | 對外**必須**提供「強制重建 XCTID 素材」入口，供前端發版後使用。 |
| `REQ-AUTH-014` | `update_cookies(new)` **必須**合併非空值回 cookie 映射並重算 cookie 字串；**不得**刪除既有鍵（除非來源明確清除）。 |

### 7.3 請求標頭建造

| 需求 | 內容 |
| --- | --- |
| `REQ-HDR-001` | **必須**提供六種標頭族：GraphQL GET、GraphQL POST、REST GET、REST FORM、UPLOAD、DOC。 |
| `REQ-HDR-002` | 標頭輸出**必須**依各族宣告順序排列；未登記者**必須**依字母序追加於後。 |
| `REQ-HDR-003` | GraphQL 族順序**必須**為：`content-type`、`authorization`、`x-twitter-auth-type`、`x-csrf-token`、`x-twitter-client-language`、`x-twitter-active-user`、`x-client-transaction-id`、`x-guest-token`、`x-att`、`origin`、`referer`。 |
| `REQ-HDR-004` | REST GET 族**不得**包含 `content-type`；順序為 `x-twitter-polling`、`authorization`、`x-twitter-auth-type`、`x-csrf-token`、`x-twitter-client-language`、`x-twitter-active-user`、`x-client-transaction-id`、`x-guest-token`、`x-att`、`origin`、`referer`。 |
| `REQ-HDR-005` | REST FORM 族**必須**將 `content-type` 置於 `x-twitter-active-user` 之後、`x-client-transaction-id` 之前；值為 `application/x-www-form-urlencoded; charset=UTF-8`。 |
| `REQ-HDR-006` | UPLOAD 族順序**必須**為 `authorization`、`x-twitter-auth-type`、`referer`、`x-csrf-token`、`content-type`、`origin`，且**不得**含 XCTID、client-language、active-user。 |
| `REQ-HDR-007` | DOC 族**必須**含 `upgrade-insecure-requests: 1`、`sec-fetch-dest: document`、`sec-fetch-mode: navigate`、`sec-fetch-site: none`、`sec-fetch-user: ?1`。 |
| `REQ-HDR-008` | `x-csrf-token` **必須**逐字等於當前 `ct0`；若在計算 XCTID 前已設置，**必須**於計算後以最終狀態重取。 |
| `REQ-HDR-009` | `x-twitter-auth-type: OAuth2Session` **僅**在已登入時加入；未登入加入會被服務端拒絕。 |
| `REQ-HDR-010` | `x-twitter-client-language` **必須**取自 cookie 的 `lang`（小寫）；缺失則不送。 |
| `REQ-HDR-011` | `x-att` **僅**在 cookie 存在 `att` 時加入（登入流一次性令牌）。 |
| `REQ-HDR-012` | XCTID 的 path 輸入**必須**為不含 query 的 pathname。 |
| `REQ-HDR-013` | 所有標頭名**必須**小寫輸出。 |

### 7.4 GraphQL 操作註冊表與參數裝配

#### 7.4.1 註冊表檔案

| 需求 | 內容 |
| --- | --- |
| `REQ-GQL-010` | 註冊表**必須**為單一 JSON 檔（預設 `static/graphql.json`），執行期唯讀。 |
| `REQ-GQL-011` | 頂層**必須**含 `generatedAt`（`%Y-%m-%dT%H:%M:%S%z`）、`frontendSha`（可為 `null`）、`operations`（依名稱排序）、`featureDefaults`（僅含被引用開關，依名稱排序）。 |
| `REQ-GQL-012` | 每個操作**必須**含 `queryId`、`operationType`（`query`｜`mutation`）、`featureSwitches`（字串陣列）、`fieldToggles`（字串陣列）。 |
| `REQ-GQL-013` | 註冊表**必須**快取於記憶體；提供強制重載。 |
| `REQ-GQL-014` | 查無操作時**必須**拋 `KeyError`，訊息含操作名並提示重新產生註冊表。 |
| `REQ-GQL-015` | `features` **必須**由操作宣告的 switch 清單逐一取 `featureDefaults`，缺省為 `False`；呼叫端覆寫優先。 |
| `REQ-GQL-016` | `fieldToggles` 的全集僅供保守回退；與瀏覽器逐欄位對齊**必須**使用 7.4.2 的子集表。 |

#### 7.4.2 瀏覽器實際送出的 `fieldToggles` 子集

| 操作 | 子集 |
| --- | --- |
| `SearchTimeline`、`HomeTimeline`、`CreateTweet`、`CreateNoteTweet` | **不送** `fieldToggles` |
| `ArticleEntityDraftCreate`、`ArticleEntityUpdateTitle`、`ArticleEntityUpdateContent`、`ArticleEntityUpdateCoverMedia`、`ArticleEntityPublish`、`ArticleEntityDelete`、`ArticleEntitiesSlice` | **不送** |
| `UserByScreenName` | `withPayments=false`、`withAuxiliaryUserLabels=true` |
| `UserTweets` | `withArticlePlainText=false` |
| `UserOriginalsTimeline` | `withPayments=false`、`withArticlePlainText=false` |
| `TweetDetail` | `withPayments=false`、`withArticleRichContentState=true`、`withArticlePlainText=false`、`withArticleSummaryText=true`、`withArticleVoiceOver=true`、`withGrokAnalyze=false`、`withDisallowedReplyControls=false` |
| `TweetResultByRestId` | `withArticleRichContentState=true`、`withArticlePlainText=false`、`withArticleSummaryText=true`、`withArticleVoiceOver=true`、`withPayments=false` |
| 其他 | 回退註冊表全集 |

#### 7.4.3 操作物件

| 需求 | 內容 |
| --- | --- |
| `REQ-GQL-020` | `path()` **必須**回傳 `/i/api/graphql/<queryId>/<name>`。 |
| `REQ-GQL-021` | `url()` **必須**回傳 `https://x.com/i/api/graphql/<queryId>/<name>`。 |
| `REQ-GQL-022` | `is_mutation` **必須**等價於 `operationType == "mutation"`。 |
| `REQ-GQL-023` | 解析 `fieldToggles` 的優先序**必須**為：呼叫端顯式值（`None` 表示不送）→ 瀏覽器子集（可疊加覆寫）→ 註冊表全集。 |
| `REQ-GQL-024` | `features` 解析後為空時，query string 與 JSON body **皆不得**帶該鍵。 |
| `REQ-GQL-025` | 一般 REST 參數裝配**必須**保持插入順序，並可輸出為 `k=v&k=v`。 |

#### 7.4.4 註冊表重建

| 需求 | 內容 |
| --- | --- |
| `REQ-GQL-030` | 重建**必須**先取得 app shell HTML（`https://x.com/i/flow/login`）。 |
| `REQ-GQL-031` | shell 取得**必須**重試，直到 HTML 含 `responsive-web/client-web`；次數上限取 `X_SHELL_ATTEMPTS`（預設 12，夾在 1–12）。耗盡**必須**拋錯並附最後 URL。 |
| `REQ-GQL-032` | **必須**解析全部 JS 資產：webpack chunk 清單 + HTML 入口 script；掃描時**必須**跳過名稱以 `icons.` 或 `i18n/` 開頭者。 |
| `REQ-GQL-033` | chunk 清單**必須**由兩張 id 表（名稱表、hash 表）重建；id 的指數寫法（如 `9e3`）**必須**正規化為十進位；檔名格式為 `<name>.<hash>a.js`，基底為 `https://abs.twimg.com/responsive-web/client-web/`。 |
| `REQ-GQL-034` | 操作萃取**必須**辨識 `queryId`、`operationName`、`operationType`、`metadata.featureSwitches`、`metadata.fieldToggles` 五欄。 |
| `REQ-GQL-035` | feature 取值**必須**先取 `defaultConfig`，再以登入態 shell 的 `user.config` 覆寫（與瀏覽器實際送出值一致）。 |
| `REQ-GQL-036` | 登入態 shell 取得失敗時**必須**回退未登入 `defaultConfig`，且不得中斷重建。 |
| `REQ-GQL-037` | 分片抓取**應**併發（預設 16 個 worker）；單一分片失敗**不得**中止整體，**必須**計入失敗清單。 |
| `REQ-GQL-038` | 寫檔**必須**以 UTF-8、`ensure_ascii=false`、兩空白縮排輸出，並更新記憶體快取。 |

### 7.5 XCTID 純算

| 需求 | 內容 |
| --- | --- |
| `REQ-XCT-001` | 產生器**必須**由三項素材構成：`key`（Base64，解出 48 位元組）、4 個動畫幀的 path 字串、4 個整數下標。 |
| `REQ-XCT-002` | 時間原點**必須**為 `1682924400` 秒；固定鹽**必須**為 `obfiowerehiring`；payload 末位常數**必須**為 `3`；動畫時間分母**必須**為 `4096`。 |
| `REQ-XCT-003` | 幀選擇**必須**為 `frames[key[5] % 4]`；列索引**必須**為 `key[indices[0]] % 16`；幀時間**必須**為 `indices[1..]` 各 `key[i] % 16` 的連乘。 |
| `REQ-XCT-004` | 幀 path **必須**自第 10 個字元起以 `C` 分段，各段僅保留數字作為整數列。 |
| `REQ-XCT-005` | 顏色/旋轉/曲線求解**必須**依 cubic-bezier 求值；旋轉角映射區間為 `[60, 360]` 並向下取整；其餘曲線參數以 `_is_odd(i)` 為下界、`1.0` 為上界，四捨五入至小數 2 位。 |
| `REQ-XCT-006` | 動畫鍵**必須**為「顏色前三通道十六進位 + 旋轉矩陣四元素十六進位 + `0` + `0`」，再移除所有 `.` 與 `-`。浮點轉十六進位**必須**逐位展開小數。 |
| `REQ-XCT-007` | 摘要**必須**為 `sha256("<METHOD>!<PATH>!<t><keyword><anim_key>")`，其中 `t = floor(now) - 1682924400`。 |
| `REQ-XCT-008` | payload **必須**為 `key(48) + t 的 4 位元組小端 + digest 前 16 位元組 + 3`。 |
| `REQ-XCT-009` | 輸出**必須**為 `base64([rnd, payload[i] XOR rnd ...])`，並去除尾端 `=`。 |
| `REQ-XCT-010` | `rnd` **必須**為 `0..255` 的隨機整數；同輸入**不得**假設同輸出。 |
| `REQ-XCT-011` | **必須**允許覆寫時間與隨機位元組，以支援逐位元組對拍測試。 |
| `REQ-XCT-012` | 本地快取素材**必須**為含 `key`（字串）、`frames`（陣列）、`indices`（陣列）的 JSON；預設路徑 `static/transaction_l1.json`，可由 `XCTID_PROFILE_FILE` 覆寫。結構不符**必須**拋 `ValueError`。 |
| `REQ-XCT-013` | 素材抓取**必須**自 shell 的 `twitter-site-verification` meta 取 key、自 4 個 `loading-x-anim-{0..3}` 各取第 2 條 `<path d>`，並自 `ondemand.s` 分片以 `\(\w\[(\d{1,2})\],\s*16\)` 取前 4 個下標。任一不足**必須**拋錯。 |

### 7.6 Castle token

#### 7.6.1 兩種後端

| 需求 | 內容 |
| --- | --- |
| `REQ-CST-001` | **必須**提供兩條互斥路徑：Node 回放（相容）與純算（strict）。 |
| `REQ-CST-002` | 純算路徑**不得**啟動 Node、**不得**建立 DOM 環境、**不得**產生網路 I/O。 |
| `REQ-CST-003` | 純算路徑要求完整 `Rl` 向量；槽數**必須**為 `632`（legacy）或 `749`（現行 x-web UMD）；其他長度**必須**拒絕。 |
| `REQ-CST-004` | 部分輸入**不得**產生「看似合理但服務端無效」的 token。 |

#### 7.6.2 Node 回放

| 需求 | 內容 |
| --- | --- |
| `REQ-CST-010` | **必須**以子程序執行 `static/castle/run.js`，工作目錄為 `static/castle/`。 |
| `REQ-CST-011` | SDK 檔選擇：檔名以 `castle.umd-` 開頭時走 UMD（`CASTLE_UMD_FILE`，可搭配 `CASTLE_UMD_RUNTIME`）；否則走 `CASTLE_FILE`（預設 `castle.js`）。 |
| `REQ-CST-012` | 公鑰來源優先序：呼叫端參數 → `static/castle/castle_meta.json` 的 `public_key` → runner 內建值。 |
| `REQ-CST-013` | 頁面狀態**必須**以 UTF-8 後 Base64 傳入：cookie、localStorage、sessionStorage；未傳時**不得**沿用上一個帳號的值。 |
| `REQ-CST-014` | 環境快照**必須**支援 `own_keys`、`enum_keys`、`query_collection`、`all_elements`、`scripts`、`style_counts`、`computed_keys`、`performance_marks`；`all_elements` 可為 `{meta, count}` 形狀。 |
| `REQ-CST-015` | 若提供完整 749 槽向量且呼叫端未指定 SDK，**必須**自動切換至現行 UMD 套件。 |
| `REQ-CST-016` | 舊版 `rl_overrides` **必須**為數值鍵映射；非映射**必須**拋 `ValueError`。 |
| `REQ-CST-017` | 子程序輸出**必須**以 `__CASTLE_TOKEN__=(\S+)` 擷取；找不到**必須**拋錯並附 stderr 末 400 字元。 |
| `REQ-CST-018` | token 在回傳前**必須**通過結構檢查：含 `|` 分隔、前半為前綴、後半可 Base64 解碼且可解析 payload。 |
| `REQ-CST-019` | 找不到 Node 可執行檔時**必須**拋錯並提示 `NODE_BIN`。 |
| `REQ-CST-020` | 所有暫存檔**必須**在結束後刪除（含例外路徑）。 |

#### 7.6.3 純算與密碼學原語

| 需求 | 內容 |
| --- | --- |
| `REQ-CST-030` | 純算入口**必須**接受：完整 profile 物件（`{"rl": ...}`）、序列化擷取（`{"raw": ...}` 或 `rawB64`）、數值字串鍵映射、或已排序的 list/tuple。 |
| `REQ-CST-031` | 前綴預設：632 槽使用 `e8bl5yQW`；749 槽使用 `uq1seNrd`；profile 可覆寫。 |
| `REQ-CST-032` | 壓縮後端預設 `native`（內附 Chromium zlib profile）；`fallback` 使用 Castle 自帶串流。native 不可用時**必須**失敗，**不得**靜默改用 fallback。 |
| `REQ-CST-033` | 時間戳**必須**可由 `timestamp_ms`／`timestamp` 覆寫；未提供時使用當前毫秒。 |
| `REQ-CST-034` | 內層加密**必須**為 32 位元 xorshift 串流：每字先加「位置 + `0x4742E2C7`」，加 `0xF50EE6A7`，左移 13／右移 17／左移 5 三次 XOR，最後加 `0xA847C680`；輸出為小端 32 位元字組。所有運算**必須**以 32 位元無號截斷。 |
| `REQ-CST-035` | `d` 傳輸編碼**必須**為每來源位元組 `x` 產生兩 byte：`0xD0 | (x >> 4)`、`((x & 0x0F) << 4) | 0x03`；解碼**必須**驗證高位 `0xD0` 與低位 `0x03`。 |
| `REQ-CST-036` | payload 框架**必須**為「內層長度（2 byte 分割）+ nonce + 密文」；現行框架使用完整 16 byte nonce。 |
| `REQ-CST-037` | native 壓縮**必須**以 raw DEFLATE（windowBits `-15`）、level `6`、memLevel `8`、strategy `0` 執行；後端為 `static/castle/chromium_zlib.dll`，可由 `CASTLE_CHROMIUM_ZLIB` 覆寫。 |
| `REQ-CST-038` | 內附 native 後端目前**僅支援 Windows**；非 Windows **必須**明確報錯。 |

#### 7.6.4 SDK 抗漂移

| 需求 | 內容 |
| --- | --- |
| `REQ-CST-040` | `refresh()` **必須**：定位線上 castle 分片 → 下載原始碼 → 取得公鑰與 webpack module id → 先嘗試原始碼自檢 → 失敗才套相容補丁 → 自檢成功後**原子替換** `castle.js` → 寫入 meta。 |
| `REQ-CST-041` | 自檢 token **必須**符合 `^[A-Za-z0-9_-]{6,16}\|.+` 且長度 > `4000`。 |
| `REQ-CST-042` | 相容補丁**必須**恰命中 6 處（1 處強制觸發 + 5 個 deferred 工廠），每處命中數恰為 1；不符**必須**中止且不改動 `castle.js`。 |
| `REQ-CST-043` | 公鑰**必須**自 `responsive_web_castle_public_key` 取值；module id **必須**自 `{<數字>(...)` 形式取得。 |
| `REQ-CST-044` | meta **必須**含 `generatedAt`、`source_url`、`orig_sha256`、`public_key`、`token_prefix`、`patch_counts`、`patch_mode`（`unpatched`｜`patched`）、`module_id`。 |
| `REQ-CST-045` | `check()` **必須**唯讀，回報 `sdk_changed`、`pk_changed`、`module_changed`、`patch_ok`、`patch_counts`、`has_meta`、`drifted`。 |
| `REQ-CST-046` | `drifted` **必須**為「SDK 內容或公鑰或 module id 變更」或「本地基線曾用補丁且補丁不再命中」。 |
| `REQ-CST-047` | CLI `refresh-castle --check` 在 `drifted=true` 時**必須**以退出碼 1 結束。 |

### 7.7 登入

#### 7.7.1 Jetfuel 主路徑（`jf.x.com`）

| 需求 | 內容 |
| --- | --- |
| `REQ-LOG-001` | 端點**必須**為：入口 `https://x.com/i/jf/onboarding/web?redirect_after_login=%2Fhome&mode=login`、視圖 `https://jf.x.com/onboarding/web?redirect_after_login=%2Fhome&mode=login`、passkey `https://jf.x.com/onboarding/web/remotes/passkey_one_fa`、動作 `https://jf.x.com/onboarding/web/actions/{begin_login,login_enter_password}`。 |
| `REQ-LOG-002` | 預熱**必須**依序：GET 入口頁（導覽標頭）→ 設定 `dnt=1`、必要時設定 `__cuid`（UUID v4）→ 取得或兌換 `guest_token` → GET 視圖（含 XCTID）→ GET passkey（含 XCTID）。 |
| `REQ-LOG-003` | 動作請求標頭**必須**依序為：`x-jf-client-theme: light`、`authorization`、`referer: https://x.com/`、`accept-language: zh-cn`、`timezone`（指紋）、`x-twitter-client-language: zh-cn`、`x-twitter-active-user: yes`、`x-client-transaction-id`、`x-guest-token`、`x-jf-v: JP-5`、`content-type`（表單時），再接 `origin`、`accept: */*`、`sec-fetch-site: same-site`、`sec-fetch-mode: cors`、`sec-fetch-dest: empty`。 |
| `REQ-LOG-004` | 每個動作（含預熱 GET）**必須**帶新算 XCTID，path 為 `/onboarding/web` 或 `/onboarding/web/actions/<action>`。 |
| `REQ-LOG-005` | 表單**必須**以 `application/x-www-form-urlencoded` 送出，`$castle_token` **必須**置於所有動作欄位之後。 |
| `REQ-LOG-006` | `begin_login` **必須**送出 `username_or_email` 與**空字串** `password`。 |
| `REQ-LOG-007` | `login_enter_password` **必須**送出 `username`、`password`、`session_token`。 |
| `REQ-LOG-008` | 每步 token **必須**為新鮮值；不得重用前一步 token。 |
| `REQ-LOG-009` | 回應**必須**先嘗試 gunzip，再以 TLV（1 byte 長度 + 內容）解出可列印字串；錯誤文案可為 UTF-8 中文。 |
| `REQ-LOG-010` | 下一步**必須**由回應內文的 `onboarding/web/actions/<name>`（可含 `i/jfapi/` 前綴）依出現順序去重取得，**不得**硬編碼挑戰名稱。 |
| `REQ-LOG-011` | `session_token` **必須**優先取「`session_token` 後緊接 UUID」者，否則取第一個符合 UUID 格式的字串。 |
| `REQ-LOG-012` | 主流程**必須**最多 8 步；每步檢查：已有 `auth_token` 或內含 `login_success` → 成功；內含 `errors` → 分類失敗；否則推進至主 next action（排除 `begin_login`、`begin_password_recovery`）。 |
| `REQ-LOG-013` | 遇需人工輸入的 action 且未提供回呼時，**必須**拋 `LoginChallenge`，攜帶 action 名、`flow_token`（= session_token）與原始字串。 |
| `REQ-LOG-014` | 提供回呼時**必須**以 `{**回傳值, session_token}` 續跑。 |
| `REQ-LOG-015` | 限流判定**必須**涵蓋 HTTP 429 及下列子串（不分大小寫）：`imit`、`rate limit`、`temporarily limited`、`临时限制`、`暂时限制`、`暂时被限制`、`登录限制`、`稍后重试`、`稍后再试`；命中**必須**拋 `LoginRateLimited` 並附 `retry-after`／`x-rate-limit-reset` 提示。 |
| `REQ-LOG-016` | 挑戰判定**必須**以下列子串對應語義：`two_factor`、`verification`、`acid`、`arkose`、`deny`、`alternate`。 |
| `REQ-LOG-017` | 成功後**必須**合併所有非空 Cookie；缺 `ct0` **必須**本地產生；`lang` **必須**補為 `zh-cn`；缺 `auth_token` **必須**拋 `LoginFailed`。 |
| `REQ-LOG-018` | 對外便利入口**必須**回傳 `(success, message, auth)`；`LoginChallenge` 回傳 `False` 並附 `需要人工處理：...`。 |
| `REQ-LOG-019` | strict 入口在未提供 profile 時**必須**拋 `ValueError`，且**不得**回退 Node。 |
| `REQ-LOG-020` | profile 形狀**必須**支援 `raw_by_action`／`rawByAction`（鍵 `begin_login`、`login_enter_password`，或 `default`／`*`）與 `raw_sequence`／`rawSequence`（至少 2 枚）；缺少必要步驟**必須**拋錯。 |
| `REQ-LOG-021` | 每步 profile 選擇**必須**保留外層 `prefix`、`compression`、`inner_variant`、`timestamp_ms`／`timestamp` 的中繼資料。 |
| `REQ-LOG-022` | profile 可為無參可呼叫物件，每次請求回傳新向量；可呼叫物件優先於靜態值。 |

#### 7.7.2 舊行動端路徑（對照用，非主路徑）

| 需求 | 內容 |
| --- | --- |
| `REQ-LOG-030` | 端點**必須**為 `https://api.x.com/1.1/onboarding/task.json`，guest 兌換為 `/1.1/guest/activate.json`。 |
| `REQ-LOG-031` | **必須**使用 Android bearer 與 Android UA，且**不得**送 XCTID。 |
| `REQ-LOG-032` | 狀態機**必須**處理：`LoginEnterUserIdentifier`、`LoginEnterUserIdentifierSSO`、`LoginEnterPassword`、`AccountDuplicationCheck`、`LoginJsInstrumentationSubtask`、`LoginSuccessSubtask`。 |
| `REQ-LOG-033` | 挑戰 subtask **必須**拋 `LoginChallenge`：`LoginAcid`、`LoginTwoFactorAuthChallenge`、`ArkoseLogin`、`DenyLoginSubtask`、`LoginEnterAlternateIdentifierSubtask`。 |
| `REQ-LOG-034` | 最多 12 步；`att` 標頭**必須**逐步回傳；`Set-Cookie` **必須**逐步合併。 |
| `REQ-LOG-035` | 回應含 `AttestationDenied` **必須**拋 `LoginAttestationBlocked`（設備證明牆，硬體級不可偽造）。 |
| `REQ-LOG-036` | 成功時若 body 的 `open_account.user.id_str` 存在，**必須**補 `twid=u%3D<id>`；缺 `ct0` **必須**本地產生。 |
| `REQ-LOG-037` | 此路徑**不得**作為預設登入策略。 |

#### 7.7.3 CLI 登入編排

| 需求 | 內容 |
| --- | --- |
| `REQ-LOG-040` | 強制重登條件**必須**為 `--force` 或 `X_LOGIN_FORCE ∈ {1,true,yes}`（不分大小寫）。 |
| `REQ-LOG-041` | 未強制且已有 `auth_token` 時，**必須**以真實 Viewer 請求驗證；有效則直接復用並結束（退出碼 0）。 |
| `REQ-LOG-042` | 驗證時**必須**先確保 XCTID 素材可用（缺則載入本地快取），**不得**因素材抓取失敗而清除憑證。 |
| `REQ-LOG-043` | 僅在下列情況視為憑證失效：GraphQL 錯誤碼 ∈ {32, 89, 215, 326}；訊息含 `unauthorized`／`authentication`／`expired token`／`invalid token`／`not authorized`；HTTP 401／403。逾時或代理故障**不得**清除憑證。 |
| `REQ-LOG-044` | 判定失效後**必須**自 `.env` 移除 `X_COOKIES` 行，再進入帳密流程。 |
| `REQ-LOG-045` | 帳密來源**必須**為命令列參數優先，其次 `X_USERNAME`／`X_PASSWORD`；缺任一**必須**退出碼 1。 |
| `REQ-LOG-046` | 帳密流程預設**必須**使用乾淨會話；僅當 `X_LOGIN_BROWSER_COOKIES ∈ {1,true,yes}` 時才注入 `.env` Cookie。 |
| `REQ-LOG-047` | `X_LOGIN_VISIBLE_COOKIES` 或 `X_LOGIN_VISIBLE_COOKIES_FILE` **必須**可覆寫瀏覽器可見 Cookie；注入時**必須**排除 `auth_token`、`twid`、`ct0`。 |
| `REQ-LOG-048` | `--pure-rl` 優先於 `CASTLE_PURE_RL_FILE`；profile 讀取**必須**容忍最多 3 層 JSON 字串包裹；`rl` 欄位**必須**為物件或陣列。 |
| `REQ-LOG-049` | 純算要求條件**必須**為：未指定 `--compat-node`，或提供 profile，或 `--pure-only`，或 `CASTLE_PURE_ONLY ∈ {1,true,yes}`。提供 profile 時一律 strict。 |
| `REQ-LOG-050` | 成功後**必須**把 cookie 字串寫回 `.env` 的 `X_COOKIES='...'`，並記錄 `user_id`。 |
| `REQ-LOG-051` | `LoginChallenge` **必須**以退出碼 1 結束並提示需在瀏覽器完成人工驗證。 |

### 7.8 讀取 API

#### 7.8.1 統一發送

| 需求 | 內容 |
| --- | --- |
| `REQ-RD-001` | 所有 query 型 GraphQL **必須**經同一發送函式：以 POST 族標頭、`GET` 方法、query string 參數、`timeout=30` 發出。 |
| `REQ-RD-002` | `referer` 預設為 `https://x.com/home`，可逐方法覆寫。 |
| `REQ-RD-003` | `raise_for_status()` **必須**先執行；解析 JSON 後僅在「有 `errors` 且無 `data`」時拋 `GraphQLError`。 |
| `REQ-RD-004` | POST 型 query **必須**以 `POST` 族標頭、JSON body 發出，body 結構同 mutation。 |

#### 7.8.2 方法契約

| 方法 | 操作 | 必要 variables | 其他 |
| --- | --- | --- | --- |
| `get_work_info(auth, work_id, cursor=None, referrer=None)` | `TweetDetail` | `focalTweetId`、`with_rux_injections=false`、`rankingMode=Relevance`、`includePromotedContent=true`、`withCommunity=true`、`withQuickPromoteEligibilityTweetFields=true`、`withBirdwatchNotes=true`、`withVoice=true`；`cursor` 可選 | `referrer` 有值時**必須**緊接 `focalTweetId` 之後插入；referer 為 `https://x.com/i/status/<id>` |
| `get_work_comments(...)` | 同上 | 同上 | 僅為語義別名 |
| `get_work_result(auth, work_id)` | `TweetResultByRestId` | `tweetId`、`includePromotedContent=true`、`withBirdwatchNotes=true`、`withVoice=true`、`withCommunity=true` | referer 同詳情 |
| `get_all_work_comments(auth, work_id, max_pages=20)` | `TweetDetail` | — | 回傳每頁原始回應；游標取 `Bottom`，無游標即停 |
| `search_work(auth, query, cursor=None, product='Top', count=20)` | `SearchTimeline` | `rawQuery`、`count`、`querySource=typed_query`、`product`、`withGrokTranslatedBio=true`、`withQuickPromoteEligibilityTweetFields=false`；`cursor` 有值時置於 `count` 之後 | **必須**以 POST 送出；referer 為 `https://x.com/search?q=<urlencoded>&src=typed_query` |
| `get_user_info(auth, user_name)` | `UserByScreenName` | `screen_name`、`withGrokTranslatedBio=true` | 使用者輸入可為連結、`@handle`、裸名 |
| `get_user_id(auth, user_name)` | 同上 | — | 回傳 `data.user.result.rest_id` |
| `get_user_post_note(auth, user_id, cursor=None, count=20, operation='UserOriginalsTimeline')` | 預設 `UserOriginalsTimeline`（可切 `UserTweets`） | `userId`、`count`、`includePromotedContent=true`、`withQuickPromoteEligibilityTweetFields=true`、`withVoice=true`；`cursor` 置於 `count` 之後 | 輸入為數字 `rest_id`，非 screen name |
| `get_user_all_post_note(auth, user_id, max_pages=20)` | 同上 | — | 游標取 `Bottom`；新游標為空或與舊值相同即停 |
| `get_home_timeline(auth, cursor=None, count=20, seen_tweet_ids=None)` | `HomeTimeline` | `count`、`includePromotedContent=true`、`withCommunity=true`；無 cursor 時加 `requestContext=launch` | 無已讀記錄 → **GET** 且**不含** `seenTweetIds`；有已讀 → **POST** 且 `seenTweetIds` 置於 variables 末 |
| `get_viewer(auth)` | `Viewer` | `withCommunitiesMemberships=true` | 用於驗證會話 |
| `get_my_user_id(auth)` | — | — | 優先回傳 cookie 的 `twid`；否則呼叫 `Viewer` |

| 需求 | 內容 |
| --- | --- |
| `REQ-RD-010` | 解析函式**必須**接受完整推文連結或純數字 id；無法解析**必須**拋 `ValueError`。 |
| `REQ-RD-011` | 使用者名稱解析**必須**接受 `x.com/@name/`、`twitter.com/name`、`@name`、裸名，並回傳小寫。 |
| `REQ-RD-012` | 游標萃取**必須**深度遍歷任意 timeline 結構，於 `content` 或 `content.itemContent` 中尋找 `cursorType == <direction>` 且 `value` 非空者；找不到回 `None`。 |
| `REQ-RD-013` | 推文條目萃取**必須**只取 `entryId` 以 `tweet-` 開頭者，並回傳 `content.itemContent.tweet_results.result`。 |
| `REQ-RD-014` | `SearchTimeline` 的 GET 路由目前回 404；實作**不得**改用 GET。 |

### 7.9 寫入 API

| 需求 | 內容 |
| --- | --- |
| `REQ-WR-001` | 所有 mutation **必須**以 POST 族標頭、JSON body、`timeout=30` 送出，且**必須**帶 XCTID。 |
| `REQ-WR-002` | mutation 回應含任何 `errors` **必須**拋 `GraphQLError`（與讀取路徑不同）。 |
| `REQ-WR-003` | 舊 REST 寫入**必須**以 FORM 族標頭、表單編碼送出，path 為 `/i/api/1.1<api>`。 |
| `REQ-WR-004` | `CreateTweet` 與 `CreateNoteTweet` 的 variables **必須**完全相同。 |
| `REQ-WR-005` | 推文 variables **必須**含：`tweet_text`、`media.media_entities[{media_id, tagged_users: []}]`、`media.possibly_sensitive=false`、`semantic_annotation_ids: []`、`disallowed_reply_options: null`、`semantic_annotation_options.source="UniversalLink"`。 |
| `REQ-WR-006` | 回覆時**必須**加 `reply.in_reply_to_tweet_id` 與 `reply.exclude_reply_user_ids`（預設空陣列）。 |
| `REQ-WR-007` | 引用時**必須**加 `attachment_url`（完整推文連結）。 |
| `REQ-WR-008` | `post_tweet(auth, text, images=None, reply_to=None, quote_url=None, note=None)` **必須**：先上傳圖片取得 media id → 依 `note` 或 `weight > 280` 選 `CreateNoteTweet`／`CreateTweet` → 回傳 `(True, "成功", res_json)`；失敗回傳 `(False, message, res_json|null)`。 |
| `REQ-WR-009` | `post_thread(auth, texts, images=None, reply_to=None, interval=2.0)` **必須**逐條發送，後一條回覆前一條；每條之間間隔 `interval` 秒（首條不等待）；單條超重自動走長推。 |
| `REQ-WR-010` | thread 中途失敗**必須**回傳 `(False, "第 <n> 條失敗：<原因>", 已發出的 ids)`；成功回傳 `(True, "成功", ids)`。 |
| `REQ-WR-011` | `images` 可為與 `texts` 等長清單，元素為單一路徑、路徑清單或 `None`。 |
| `REQ-WR-012` | 新推文 id **必須**自 `data.create_tweet.tweet_results.result` 或 `data.notetweet_create.tweet_results.result` 取得 `rest_id`；缺失時回退 `legacy.id_str`；兩者皆無**必須**拋 `KeyError`。 |
| `REQ-WR-013` | 刪除**必須**只送 `tweet_id`（**不得**帶 `dark_request`）。 |
| `REQ-WR-014` | 互動操作**必須**使用：`FavoriteTweet`／`UnfavoriteTweet`（`tweet_id`）、`CreateRetweet`／`DeleteRetweet`（`tweet_id`／`source_tweet_id` + `dark_request=false`）、`CreateBookmark`／`DeleteBookmark`（`tweet_id`）。 |
| `REQ-WR-015` | 關係操作**必須**使用 `/friendships/create.json` 與 `/friendships/destroy.json`，表單固定含 `include_profile_interstitial_type=1`、`include_blocking=1`、`include_blocked_by=1`、`include_followed_by=1`、`include_want_retweets=1`、`include_mute_edge=1`、`include_can_dm=1`、`include_can_media_tag=1`、`skip_status=1`、`user_id`。 |

### 7.10 媒體 API

| 需求 | 內容 |
| --- | --- |
| `REQ-MED-001` | 上傳**必須**走 `https://upload.x.com/i/media/upload.json`，命令參數置於 query string，**不得**置於 body。 |
| `REQ-MED-002` | 上傳標頭**必須**使用 UPLOAD 族（無 XCTID），referer 為 `https://x.com/`。 |
| `REQ-MED-003` | 分片大小**必須**為 `4 MiB`；`APPEND` 的 `segment_index` **必須**自 0 遞增。 |
| `REQ-MED-004` | `INIT` 參數**必須**為 `command=INIT`、`total_bytes`、`media_type`、`media_category`；回傳 `media_id_string`。 |
| `REQ-MED-005` | `APPEND` **必須**以 multipart 欄位名 `media`、檔名 `blob`、`application/octet-stream` 送出。 |
| `REQ-MED-006` | `FINALIZE` **必須**帶 `original_md5`（整檔 MD5 十六進位）。 |
| `REQ-MED-007` | 影片**必須**輪詢 `STATUS` 直到 `processing_info.state` 不為 `pending`／`in_progress`；等待秒數取 `max(check_after_secs, 1)`；預設逾時 `180` 秒。`failed` **必須**拋錯。 |
| `REQ-MED-008` | 副檔名對應**必須**為：`.jpg/.jpeg → image/jpeg`、`.png → image/png`、`.gif → image/gif`、`.webp → image/webp`、`.mp4 → video/mp4`、`.mov → video/quicktime`；其他 → `image/jpeg`。 |
| `REQ-MED-009` | 類別對應**必須**為：`image/gif → tweet_gif`、`video/mp4|video/quicktime → tweet_video`、其他 → `tweet_image`。 |
| `REQ-MED-010` | 一般發文流程**必須**在 `FINALIZE` 後呼叫 `POST https://x.com/i/api/1.1/media/metadata/create.json`，body 為 `{media_id, allow_download_status:{allow_download:"true"|"false"}}`；此端點回應為空，**不得**解析 JSON。 |
| `REQ-MED-011` | 文章編輯器（封面／插圖）**必須**略過 metadata 步驟。 |
| `REQ-MED-012` | 批次上傳收到單一字串**必須**視為單一檔案，**不得**逐字元迭代。 |
| `REQ-MED-013` | 控制台**僅**保存 media id；排程不得超過上傳後 `23` 小時；單檔上限 `50 MB`。 |

### 7.11 文章 API 與 Markdown 映射

#### 7.11.1 操作鏈路

| 需求 | 內容 |
| --- | --- |
| `REQ-ART-001` | 完整鏈路**必須**為：`ArticleEntityDraftCreate` → `ArticleEntityUpdateTitle` → `ArticleEntityUpdateContent` →（可選）媒體上傳 → `ArticleEntityUpdateCoverMedia` →（可選）`ArticleEntityPublish`。 |
| `REQ-ART-002` | 列表**必須**使用 `ArticleEntitiesSlice`（GET），`lifecycle ∈ {Draft, Published}`，`count` 預設 20，`userId` 預設為會話 `twid`。 |
| `REQ-ART-003` | 刪除**必須**使用 `ArticleEntityDelete`，只送 `articleEntityId`。 |
| `REQ-ART-004` | 同一文章 id 的鍵名**必須**依操作區分：標題／封面／發布／刪除用 `articleEntityId`；正文用 `article_entity`。 |
| `REQ-ART-005` | 此組操作**不得**送 `fieldToggles`。 |
| `REQ-ART-006` | referer **必須**為 `https://x.com/compose/articles` 或 `https://x.com/compose/articles/edit/<id>`。 |
| `REQ-ART-007` | 建草稿**必須**送 `content_state`（預設 `{blocks: [], entity_map: []}`）與 `title`。 |
| `REQ-ART-008` | 文章 id 萃取**必須**接受 `rest_id`，或把 `id` 視為 `base64("ArticleEntity:<rest_id>")` 解碼；皆無**必須**拋 `KeyError`。 |
| `REQ-ART-009` | 發布 variables **必須**含 `articleEntityId`、`visibilitySetting`（預設 `Public`）；`tweetText` **僅**在非空時加入；`conversationControl` **僅**在 `reply_mode` 非空且非 `All` 時加入。 |
| `REQ-ART-010` | `reply_mode` **必須**限於 `All`、`Community`、`Verified`、`ByInvitation`；非法值**必須**拋 `ValueError`。 |
| `REQ-ART-011` | 發布後推文 id **必須**自 `data.articleentity_publish.article_entity_results.result.metadata.tweet_results.result.rest_id` 取得。 |
| `REQ-ART-012` | 發布說明的長度上限為 `256` 字（由平台約束）。 |
| `REQ-ART-013` | 編輯連結**必須**為 `https://x.com/compose/articles/edit/<article_id>`。 |
| `REQ-ART-014` | 已發布文章刪除後，發布時產生的推文亦會消失（外部行為）。 |
| `REQ-ART-015` | `post_article` **必須**回傳 `(success, message, info)`；`info` 至少含 `article_id`、`edit_url`，發布時另含 `tweet_id` 與 `raw`。 |
| `REQ-ART-016` | `post_article` 的執行順序**必須**為：解析標題與正文 → 建草稿（或沿用 `article_id`）→ 更新標題 → 更新正文 →（可選）上傳並設封面 →（可選）發布。 |
| `REQ-ART-017` | 標題為空**必須**拋 `ValueError`，訊息為 `文章標題不能為空：傳 title，或讓正文第一行是 \`# 標題\``。 |
| `REQ-ART-018` | 封面相對路徑**必須**以 `base_dir` 解析；絕對路徑直接使用。 |

#### 7.11.2 `content_state` 結構

| 需求 | 內容 |
| --- | --- |
| `REQ-ART-020` | 頂層**必須**為 `{"blocks": [...], "entity_map": [...]}`；鍵名**必須**為 snake_case。 |
| `REQ-ART-021` | 每個 block **必須**含 `data`（物件）、`text`、`key`（5 字元 base36）、`type`、`entity_ranges`、`inline_style_ranges`。 |
| `REQ-ART-022` | `entity_map` **必須**為 `[{key: "<序號字串>", value: {data, type, mutability}}]` 陣列，**不得**為物件。 |
| `REQ-ART-023` | 行內樣式名**必須**為 `Bold`、`Italic`、`Strikethrough`。 |
| `REQ-ART-024` | `offset`／`length` **必須**以 Unicode 碼點計，**不得**用 UTF-16 索引。 |
| `REQ-ART-025` | 首尾相接的同款式區間**必須**合併；輸出**必須**依 `(offset, style)` 排序。 |
| `REQ-ART-026` | `atomic` block 的文字**必須**固定為單一空格，實體範圍為 `offset=0,length=1`。 |
| `REQ-ART-027` | 實體型別與資料**必須**為：`LINK{url}`（mutability `Mutable`）、`DIVIDER{}`、`MEDIA{entity_key: uuid4, media_items:[{local_media_id, media_category:"DraftTweetImage", media_id}]}`、`TWEET{tweet_id}`、`MARKDOWN{markdown:"```<lang>\n<code>\n```"}`（`MARKDOWN` 為 `Mutable`）。 |
| `REQ-ART-028` | 一個 `MEDIA` 區塊**最多** 4 張圖；`local_media_id` **必須**自 1 遞增。 |

#### 7.11.3 Markdown 子集

| 行首語法 | 產出 |
| --- | --- |
| `# 標題` | `header-one` |
| `## 標題`、`### 標題` 以上 | `header-two` |
| `- 項`、`* 項`、`+ 項` | `unordered-list-item` |
| `1. 項`、`1) 項` | `ordered-list-item` |
| `> 引用` | `blockquote` |
| `---`、`***`、`___`（3 個以上） | `DIVIDER` |
| `![](path)` 獨占一行 | `MEDIA`（需上傳回呼） |
| `https://x.com/<user>/status/<id>` 獨占一行（可被 `<>` 包裹） | `TWEET` |
| ` ```lang ` … ` ``` ` 或 `~~~` | `MARKDOWN` 代碼塊 |
| 其他非空行 | `unstyled` |
| 空行 | 僅作分隔，不產生 block |

| 需求 | 內容 |
| --- | --- |
| `REQ-ART-030` | 行內語法**必須**支援 `**粗體**`、`*斜體*`、`~~刪除線~~`、`[文字](url)`、反斜線轉義；`**` 優先於 `*`；**必須**支援嵌套（如粗體內含斜體或連結）。 |
| `REQ-ART-031` | 行內解析**必須**回傳純文字、樣式區間、連結區間；連結區間為 `(offset, length, url)`。 |
| `REQ-ART-032` | 圖片行**必須**先以 `base_dir` 解析路徑，再呼叫上傳回呼；未提供回呼**必須**拋 `ValueError`（不得靜默丟圖）。 |
| `REQ-ART-033` | 代碼圍欄**必須**讀到對應結尾圍欄（或檔尾）為止，內容以 LF 連接。 |
| `REQ-ART-034` | `split_title` **必須**僅在「第一行（去前導換行後）符合 `# 標題`」時拆出標題並回傳剩餘正文；否則標題為 `None`。 |
| `REQ-ART-035` | 換行**必須**先正規化為 LF；每行**必須**去除右側空白。 |

### 7.12 X Chat 私信 API

| 需求 | 內容 |
| --- | --- |
| `REQ-DM-001` | 傳輸**必須**區分兩個來源：`https://api.x.com/graphql`（Apollo 風格）與 `https://x.com/i/api/graphql`（web GraphQL）。 |
| `REQ-DM-002` | 固定操作 id **必須**為：`GenerateXChatTokenMutation=Qh3fZRjPPtPoHYR_2sCZsA`、`xChatDmSettingsQuery=mRmm_3aCzCcbpzBkhyhCDg`、`GetUsersByIdsForXChat=4zbPmRwsca859ORbYCLyzg`、`GetPublicKeysQuery=C-YphxeIcPLOGtFNGhkuOw`、`GetInitialXChatPageQuery=m1gzpOV8JFOTaFH0Xq7lMQ`、`GetConversationPageQuery=GX9ZijkxG8AqRMQVD7hMnQ`、`GetInboxPageRequestQuery=txrXftx8RcSvw9jCjwRZWA`、`SendMessageCreateMutation=TWRPP7gnKwV_R8-tE-Dd3Q`、`AddEncryptedConversationKeysMutation=hEhvYw2gxbE4YlBjr6JlXA`。 |
| `REQ-DM-003` | 預設查詢設定**必須**為：`conversation_event_limit=200`、`inbox_conversation_event_limit=5`、`inbox_conversation_limit=20`、`user_event_limit=500`。 |
| `REQ-DM-004` | 外部 GraphQL 標頭順序**必須**為：`authorization`、`x-csrf-token`、`referer`、`apollo-require-preflight`、`x-client-transaction-id`、`x-twitter-auth-type`、`accept`、`content-type`、`origin`。 |
| `REQ-DM-005` | GET 型操作**必須**把 `variables` 以緊湊 JSON 放入 query string；POST 型**必須**放 JSON body 的 `variables`。 |
| `REQ-DM-006` | 回應**必須**驗證為物件；含 `errors` **必須**拋 `GraphQLError`。 |
| `REQ-DM-007` | `generate_chat_token` **必須**自 `data.user_get_x_chat_auth_token.token` 取短期 token，且**不得**落盤；缺失**必須**拋錯。 |
| `REQ-DM-008` | `bootstrap` **必須**依序組合：chat token → DM 設定 → 初始頁 → 成員 → 公鑰 →（可選）會話頁，並回傳 `passcode_required` 欄位（預設 `None`）。 |
| `REQ-DM-009` | `send_message` **必須**只接受已簽章密文（`encoded_message_create_event`、`encoded_message_event_signature`、可選 `conversation_token`）；**不得**接受 PIN、明文或私鑰。 |
| `REQ-DM-010` | 發送回應若無 `data.xchat_send_create_message_event.encoded_message_event`，**必須**拋錯並提示「不能重複發送」。 |
| `REQ-DM-011` | `add_conversation_keys` 回應含 `error_code` **必須**拋 `GraphQLError`；缺 `conversation_key_change_sequence_id` **必須**拋錯。 |
| `REQ-DM-012` | 私信頁行為**必須**符合第 8 章 `WEB-CHAT` 與第 9 章 `FE-CHAT`。 |

### 7.13 資料處理工具

#### 7.13.1 推文計重（twitter-text v3）

| 需求 | 內容 |
| --- | --- |
| `REQ-DAT-001` | 上限**必須**為 `280`。 |
| `REQ-DAT-002` | 連結（`https?://` 後接非空白且非全角標點）**必須**整段計為 `23`，不計實際長度。 |
| `REQ-DAT-003` | 碼點落在 `[0,4351]`、`[8192,8205]`、`[8208,8223]`、`[8242,8247]` **必須**計 1；其餘計 2。 |
| `REQ-DAT-004` | emoji 修飾碼點（`U+200D`、`U+FE0E`、`U+FE0F`、`U+20E3`）**不得**單獨計數；`U+200D` 之後緊接的一個碼點**不得**計數；膚色修飾（`U+1F3FB..U+1F3FF`）**不得**計數。 |
| `REQ-DAT-005` | 連結結束判定**必須**排除 CJK 標點（`U+3000..U+303F`）與全角字元（`U+FF00..U+FFEF`）。 |

#### 7.13.2 結構化與匯出

| 需求 | 內容 |
| --- | --- |
| `REQ-DAT-010` | 推文正規化**必須**先解開 `TweetWithVisibilityResults.tweet`。 |
| `REQ-DAT-011` | 輸出欄位**必須**為：`work_id`、`work_url`、`work_type`、`title`、`create_time`、`lang`、`user_id`、`nickname`、`screen_name`、`user_url`、`like_count`、`retweet_count`、`reply_count`、`quote_count`、`bookmark_count`、`view_count`、`image_urls`、`video_urls`。 |
| `REQ-DAT-012` | `work_url` **必須**為 `https://x.com/<screen_name>/status/<id>`；`user_url` 為 `https://x.com/<screen_name>`。 |
| `REQ-DAT-013` | `work_type` **必須**為 `video`（有影片）、`image`（有圖無影片）或 `text`。 |
| `REQ-DAT-014` | 標題**必須**優先取 `note_tweet.note_tweet_results.result.text`，否則取 `legacy.full_text`（長推的 `full_text` 僅為截斷預覽）。 |
| `REQ-DAT-015` | 媒體**必須**優先讀 `legacy.extended_entities.media`，缺失才讀 `entities.media`。 |
| `REQ-DAT-016` | 影片**必須**選 `content_type == video/mp4` 中位元率最高者。 |
| `REQ-DAT-017` | 資料夾名稱**必須**為 `<screen_name>_<title>`，非法字元（`\ / : * ? " < > | CR LF TAB`）替換為 `_`，長度上限 60，空值為 `untitled`。 |
| `REQ-DAT-018` | 圖片下載**必須**加 `?format=jpg&name=orig`；檔名為 `<序號>.jpg`；影片為 `<序號>.mp4`。 |
| `REQ-DAT-019` | xlsx 匯出**必須**第一列為欄位名，清單欄位以 LF 連接。 |

### 7.14 命令列介面

| 需求 | 內容 |
| --- | --- |
| `REQ-CLI-001` | 入口**必須**為 `python main.py <command> [args]`；無子命令**必須**報錯退出。 |
| `REQ-CLI-002` | 所有命令**必須**先載入 `.env` 並建立鑑權狀態；資料目錄為 `datas/media_datas` 與 `datas/excel_datas`（不存在則建立）。 |

| 子命令 | 參數 | 行為 | 退出碼 |
| --- | --- | --- | --- |
| `whoami` | — | 需 `auth_token`；印出 `@screen_name (rest_id) name` | 0；未登入 1 |
| `detail` | `<work>` | 取詳情、取第一條推文、印 JSON；印出同頁評論數 | 0；無條目 1 |
| `user` | `<name>` | 印 `rest_id`、`name`、`screen_name`、`followers`、`tweets`、`description` | 0 |
| `search` | `<query>` `[--product Top\|Latest\|People\|Media]`（預設 `Top`） | 印 `work_url` 與標題前 60 字，最後印總數 | 0 |
| `post` | `[text]` `[--file]` `[--image ...]` `[--reply-to]` `[--quote]` `[--note]` | 正文為空 → 1；超重或 `--note` 走長推；印 `https://x.com/i/status/<id>` | 0；失敗 1 |
| `thread` | `[texts...]` `[--file]` `[--reply-to]` `[--interval 2.0]` | 檔案以單獨一行 `---` 分隔；少於 2 條 → 1；逐條印連結；失敗印已發部分 | 0；失敗 1 |
| `article` | `<file>` `[--title]` `[--cover]` `[--publish]` `[--caption]` `[--reply-mode]` `[--article-id]` | 預設只存草稿；印 `edit_url`；發布時印推文連結 | 0；失敗 1 |
| `article-list` | `[--published]` `[--count 20]` | 印 `id`、標題、`lifecycle` 與總數 | 0 |
| `article-delete` | `<article_id>` | 印原始回應 | 0 |
| `delete` | `<work>` | 印原始回應 | 0 |
| `spider` | `<name>` `[--pages 2]` `[--save all\|excel\|media\|media-image\|media-video\|none]`（預設 `excel`） | 逐頁抓取至無新游標；可下載媒體與寫 xlsx | 0 |
| `refresh-graphql` | — | 重建註冊表，印操作數 | 0 |
| `refresh-castle` | `[--check]` | 重建或漂移檢測；`--check` 有漂移 → 1 | 0／1 |
| `login` | `[--username]` `[--password]` `[--pure-rl FILE]` `[--pure-only]` `[--compat-node]` `[--force]` | 見 7.7.3 | 0／1 |

| 需求 | 內容 |
| --- | --- |
| `REQ-CLI-010` | `thread --file` **必須**以正則 `^\s*---\s*$`（多行模式）分割，並去除空段。 |
| `REQ-CLI-011` | `spider` **必須**在「無新游標」或「游標與前一頁相同」時停止。 |
| `REQ-CLI-012` | `spider --save` 為 `media*` 時**必須**下載媒體；為 `all` 或 `excel` 時**必須**寫入 `<excel>/<screen_name>.xlsx`。 |
| `REQ-CLI-013` | 所有結構化輸出**必須**以 `ensure_ascii=false`、兩空白縮排的 JSON 印出。 |

### 7.15 快速開始腳本

| 需求 | 內容 |
| --- | --- |
| `REQ-CLI-020` | 預設查詢為 `美国`，產品為 `Top`，終端顯示上限 10 條。 |
| `REQ-CLI-021` | 帳密解析順序**必須**為：腳本常數 → 環境變數 → 互動輸入（使用者名稱 `input`、密碼 `getpass`）；任一為空**必須**拋 `ValueError`。 |
| `REQ-CLI-022` | Castle profile 路徑**必須**取腳本常數或 `CASTLE_PURE_RL_FILE`；為空**必須**拋錯並說明需要同一登入頁的完整 profile。 |
| `REQ-CLI-023` | profile 載入**必須**展開 `~`、解析為絕對路徑、容忍最多 3 層 JSON 字串包裹，且必須為 dict／list／tuple。 |
| `REQ-CLI-024` | 代理**必須**取 `X_PROXY`，否則探測系統代理；皆無則直連。 |
| `REQ-CLI-025` | 登入**必須**使用 strict 純算入口；失敗**必須**拋 `RuntimeError("登录失败：<message>")`。 |
| `REQ-CLI-026` | 登入成功**必須**把 cookie 寫回 `.env`。 |
| `REQ-CLI-027` | 查詢詞與產品**必須**可由 `X_SEARCH_QUERY`／`X_SEARCH_PRODUCT` 覆寫；產品**必須**限於 `Top`、`Latest`、`People`、`Media`。 |
| `REQ-CLI-028` | 搜尋**必須**使用本地快取 XCTID 素材，**不得**依賴瀏覽器或額外 shell 抓取。 |
| `REQ-CLI-029` | 輸出**必須**為 `搜索成功：'<query>'，类型=<product>，结果=<n> 条`，續行格式為 `<序號>. @<screen_name> <標題前 120 字>` 與縮排 4 空白的 `work_url`。 |
| `REQ-CLI-030` | 標準輸出／錯誤**必須**在 Windows 終端以 UTF-8（`errors=replace`）重新配置。 |
| `REQ-CLI-031` | `KeyboardInterrupt` **必須**回傳 130；其他例外**必須**印 `快速搜索失败：<type>: <msg>` 並回傳 1。 |

## 8. 行為規格：Web 控制台

### 8.1 執行時契約

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-001` | 控制台**必須**以 FastAPI 應用提供服務，入口為工廠函式 `create_app(data_dir=None, start_worker=True)`；正式部署**必須**以 `--factory` 載入。 |
| `REQ-WEB-002` | 正式部署**必須**只啟動**一個** Uvicorn worker；不得以多 worker 共用同一 MySQL 資料庫。 |
| `REQ-WEB-003` | 應用**不得**暴露 OpenAPI、Swagger UI 或 ReDoc；`docs_url`、`redoc_url`、`openapi_url` **必須**停用。 |
| `REQ-WEB-004` | 啟動時若啟用背景工作者，**必須**先取得 MySQL 具名鎖 `<database>:scheduler`；取鎖失敗**必須**拒絕啟動。 |
| `REQ-WEB-005` | 啟動時**必須**把殘留的 `accounts.status='logging_in'` 重設為 `pending`，避免永久卡在登入中。 |
| `REQ-WEB-006` | 背景迴圈**必須**每 3 秒執行一次 `tick`；失去排程鎖時**必須**先釋放、重新取得並執行復原，不得在無鎖狀態續跑。 |
| `REQ-WEB-007` | `GET /healthz` **不得**要求登入；**必須**執行 `SELECT 1`，並在啟用工作者時確認排程執行緒存活。成功回 `200 {"status":"ok","database":"mysql"}`；排程器未運行回 `503`。 |
| `REQ-WEB-008` | `GET /` **必須**回傳靜態 `index.html`；`/static` **必須**以靜態檔案目錄掛載。 |
| `REQ-WEB-009` | 應用啟動時**必須**套用 `mysql.sql` 的建表語句，並對舊資料庫補齊 `account_policies.physical_xid`、`maintenance_logs.details`、`account_maintenance.read_home`、`account_maintenance.home_pages_per_cycle`。 |
| `REQ-WEB-010` | 資料庫名稱**必須**匹配 `[A-Za-z0-9_]+`；不合法時**必須**拒絕啟動。 |

### 8.2 資料模型

所有時間欄位**必須**以 Unix epoch 秒（`DOUBLE`）儲存；所有資料表**必須**使用 InnoDB 與 `utf8mb4`。

| 資料表 | 主鍵／唯一鍵 | 關鍵欄位 | 用途與約束 |
| --- | --- | --- | --- |
| `transaction_lock` | `id` | 固定一列 `id=1` | 全域交易互斥；`SELECT ... FOR UPDATE` 序列化初始化、佇列領取與配額計數 |
| `users` | `id`；`username` 唯一 | `password`、`role`、`disabled`、`created` | 工作台使用者；`role ∈ {admin, member}` |
| `sessions` | `token` | `user_id`、`csrf`、`expires` | `token` 為原始會話 token 的 SHA-256 十六進位；`csrf` 為明文隨機值；逾期即無效 |
| `attempts` | `key` | `count`、`until` | 限流計數；`key` 為 `sha256(語意鍵:來源IP)`；視窗預設 900 秒 |
| `account_policies` | `id`；`x_id` 唯一 | `owner`、`first_login`、`warmup_until`、`limit_until` | 以 X 數字 ID 為物理身分；同一 X 帳號跨工作台使用者共用 |
| `accounts` | `id`；`(owner,x_id)` 唯一 | `handle`、`name`、`avatar`、`secret`、`status`、`warmup_days`、`policy_id`、`session_version`、`cookie_updated` | `secret` 為 Fernet 密文；`status ∈ {pending, logging_in, connected, expired}` |
| `ai_settings` | `owner` | `base_url`、`model`、`secret`、`instruction` | 每位使用者一列；`secret` 內含 API Key 與 token 參數偏好 |
| `sources` | `id` | `owner`、`name`、`url`、`kind`、`enabled`、`last_fetch`、`error` | `kind ∈ {rss, web}` |
| `entries` | `id`；`(source_id,url_hash)` 唯一 | `title`、`url`、`summary`、`published`、`fetched` | `url_hash` 為 `SHA2(url,256)` 生成欄 |
| `rules` | `id` | `owner`、`account_id`、`name`、`source_ids` JSON、`prompt`、`mode`、`interval_minutes`、`daily_limit`、`enabled`、`next_run`、`running`、`last_run`、`last_result` | `mode ∈ {draft, publish}`；`running` 為單次執行旗標 |
| `jobs` | `id`；`(owner,request_id)` 唯一 | `account_id`、`payload` JSON、`state`、`scheduled`、`result` JSON、`error`、`rule_id`、`created`、`updated` | 索引 `(state,scheduled)`；狀態見 8.13 |
| `publication_ledger` | `id` | `policy_id`、`day` DATE、`units`、`confirmed`、`state`、`created`、`updated` | 每筆發布請求一列；`state ∈ {reserved, sent, rejected, uncertain}` |
| `rule_entries` | `(rule_id,entry_id)` | — | 自動化去重；同一規則同一來源條目只處理一次 |
| `media` | `id` | `owner`、`account_id`、`x_id`、`name`、`mime`、`created` | 只保存 X 媒體 ID，不保存二進位內容 |
| `keyword_groups` | `id` | `owner`、`name`、`keywords` JSON、`created` | 每組 1–100 個關鍵詞 |
| `account_maintenance` | `account_id` | `group_ids` JSON、`enabled`、`interval_minutes`、`pages_per_cycle`、`delay_min_seconds`、`delay_max_seconds`、`read_comments`、`comment_pages`、`read_home`、`home_pages_per_cycle`、`next_run`、`running`、`state` JSON、`last_run`、`last_result`、`read_count` | 每帳號一列；`state` 保存游標與輪次 |
| `maintenance_logs` | `id` 自增 | `account_id`、`keyword`、`operation`、`page`、`item_count`、`status`、`message`、`details` JSON、`created` | 保留 30 天；索引 `(account_id,created)` |
| `audit` | `id` 自增 | `owner`、`action`、`detail`、`created` | `owner` 可為 NULL；`detail` 截斷 500 字元 |
| `account_logins` | `account_id` | `stage`、`message`、`started`、`updated` | 登入進度；每位帳號一列 |
| `request_logs` | `id` 自增 | `account_id`、`attempt_id`、`method`、`path`、`status`、`duration_ms`、`request_fields` JSON、`response_headers` JSON、`response_excerpt`、`error_type`、`created` | 登入診斷；保留 30 天；索引 `(account_id,created)` |
| `chat_dispatches` | `(account_id,message_id)` | `payload_hash`、`operation`、`state`、`created`、`updated` | 私信冪等；`state ∈ {sending, responded, rejected, uncertain}` |

**資料完整性不變量**

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-DB-010` | 所有外鍵**必須**啟用；刪除 `users`、`accounts`、`sources`、`rules` **必須**串聯刪除其附屬資料，`audit.owner` 例外設為 NULL。 |
| `REQ-WEB-DB-011` | `publication_ledger.id` **必須**由呼叫端提供且全域唯一；同一請求 ID 重複寫入**必須**被主鍵拒絕。 |
| `REQ-WEB-DB-012` | `jobs.(owner,request_id)` **必須**唯一；重複提交**不得**產生第二列。 |
| `REQ-WEB-DB-013` | `account_policies.x_id` **必須**唯一，確保同一物理 X 帳號只有一份保護期與配額。 |

### 8.3 HTTP 封套、標頭與請求限制

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-020` | 所有 API 回應**必須**為 UTF-8 JSON；錯誤**必須**為 `{"detail": <string>}`。 |
| `REQ-WEB-021` | 欄位驗證失敗**必須**回 `422`，格式為 `{"detail":"请检查输入字段","fields":[{"field":"<路徑>","message":"<訊息>"}]}`；**不得**回顯原始輸入值。 |
| `REQ-WEB-022` | 非 `GET`／`HEAD`／`OPTIONS` 請求**必須**帶 `X-Requested-With: XApis`；若有 `Origin`，**必須**等於 `CONSOLE_PUBLIC_URL`（去尾端 `/`）或請求基底 URL；否則回 `403 {"detail":"请求来源验证失败，请从控制台页面操作"}`。 |
| `REQ-WEB-023` | 請求若帶 `Content-Length`，**必須**為純數字且不得超過 `55 × 1024 × 1024` 位元組；否則回 `413 {"detail":"请求过大（最大 50 MB 媒体）"}`。 |
| `REQ-WEB-024` | 請求體**必須**在路由前限制：一般路徑 `2 MiB`，路徑以 `/media` 結尾者 `55 MiB`；超限回 `413 {"detail":"请求体超过大小限制"}`。分塊傳輸亦同。 |
| `REQ-WEB-025` | 每個回應**必須**加上：`X-Content-Type-Options: nosniff`、`X-Frame-Options: DENY`、`Referrer-Policy: no-referrer`、`Cache-Control: no-store`。 |
| `REQ-WEB-026` | `Content-Security-Policy` **必須**為：`default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; worker-src 'self'; style-src 'self'; img-src 'self' https://*.twimg.com data: blob:; media-src https://*.twimg.com blob:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'`。 |
| `REQ-WEB-027` | 未捕捉的上游例外**必須**轉為 `502 {"detail":"<安全化訊息>"}`；**不得**把含 URL、Cookie 或憑證的原始例外寫入回應或日誌。 |
| `REQ-WEB-028` | 會話 Cookie 名稱**必須**為 `xapis_session`；屬性**必須**為 `HttpOnly`、`SameSite=Strict`、`Path=/`、`Max-Age=604800`；`Secure` 僅在 `CONSOLE_COOKIE_SECURE=1` 時設定。 |
| `REQ-WEB-029` | 會話有效期**必須**為 7 天；資料庫只保存 token 的 SHA-256；每次登入**必須**先清除逾期會話。 |
| `REQ-WEB-030` | 非 `GET`／`HEAD` 的已認證請求**必須**比對 `x-csrf-token` 與會話 `csrf`（常數時間比較）；不符回 `403 {"detail":"会话验证失败，请刷新页面"}`。 |
| `REQ-WEB-031` | 未認證或會話逾期**必須**回 `401 {"detail":"请先登录"}`；非管理員存取管理端點**必須**回 `403 {"detail":"需要管理员权限"}`。 |
| `REQ-WEB-032` | 資源不屬於目前使用者時**必須**回 `404 {"detail":"记录不存在"}`，**不得**揭露該資源是否存在。 |
| `REQ-WEB-033` | 限流**必須**以 `sha256(語意鍵:來源IP)` 計數，視窗 900 秒；超限回 `429 {"detail":"操作过于频繁，请在 15 分钟后重试"}`。 |
| `REQ-WEB-034` | 登入限流**必須**同時套用來源 IP（20 次）與小寫使用者名稱（20 次）；後者超限訊息為 `登录尝试过多，请稍后重试`。 |
| `REQ-WEB-035` | 控制台**不得**信任任意轉發標頭；正式部署**必須**由反向代理保留原始 `Host`，且 `CONSOLE_PUBLIC_URL` 與瀏覽器位址一致。 |

### 8.4 鑑權與工作台使用者

| 方法與路徑 | 權限 | 行為 |
| --- | --- | --- |
| `GET /api/auth/status` | 公開 | 回 `{"initialized": <bool>}`；以是否存在任何 `users` 判定 |
| `POST /api/auth/setup` | 公開 | 建立首位管理員；需初始化令牌；成功即建立會話 |
| `POST /api/auth/login` | 公開 | 帳密登入；成功建立會話 |
| `GET /api/auth/me` | 會話 | 回目前使用者 `id`、`username`、`role` 與 `csrf` |
| `POST /api/auth/logout` | 會話 | 刪除目前會話並清除 Cookie |
| `POST /api/auth/password` | 會話 | 驗證目前密碼後更新；刪除該使用者全部會話 |
| `GET /api/users` | 管理員 | 列出 `id,username,role,disabled,created`，依 `created` 排序 |
| `POST /api/users` | 管理員 | 建立使用者；`role` 預設 `member` |
| `PATCH /api/users/{user_id}` | 管理員 | 更新 `role` 與 `disabled`；刪除該使用者全部會話 |

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-040` | 初始化**必須**先比對 `CONSOLE_SETUP_TOKEN` 或 `data/setup.token`；不符回 `403 {"detail":"初始化令牌不正确"}`。 |
| `REQ-WEB-041` | 初始化**必須**在單一交易中檢查 `users` 是否為空；已初始化回 `409 {"detail":"控制台已初始化，请登录"}`。首位使用者**必須**為 `admin`。 |
| `REQ-WEB-042` | 初始化令牌長度**必須**為 16–256 字元；使用者名稱 3–40 字元且匹配 `^[a-zA-Z0-9_.@-]+$`；密碼 12–128 字元。 |
| `REQ-WEB-043` | 登入**必須**對不存在使用者執行等成本的虛擬密碼檢查，避免以時間差列舉使用者。 |
| `REQ-WEB-044` | 登入失敗、使用者停用或密碼錯誤**必須**一律回 `401 {"detail":"账号或密码不正确"}`。 |
| `REQ-WEB-045` | 登入表單的密碼長度**必須**允許 1–128 字元，以便驗證既有雜湊；建立與變更密碼仍套用 12 字元下限。 |
| `REQ-WEB-046` | 密碼雜湊**必須**為 `scrypt`，參數 `N=2^15`、`r=8`、`p=1`、`maxmem=64 MiB`，16 位元組隨機鹽；編碼格式 `scrypt$<salt-hex>$<digest-hex>`。 |
| `REQ-WEB-047` | 變更密碼**必須**先驗證目前密碼；錯誤回 `403 {"detail":"当前密码不正确"}`。成功後**必須**刪除該使用者全部會話並清除目前 Cookie。 |
| `REQ-WEB-048` | 建立使用者時名稱重複**必須**回 `409 {"detail":"用户名已存在"}`。 |
| `REQ-WEB-049` | 更新使用者時**不得**停用或降級最後一位啟用中的管理員；違反回 `409 {"detail":"不能停用或降级最后一位管理员"}`。 |
| `REQ-WEB-050` | `PATCH /api/users/{id}` **必須**視為完整替換：未提供 `role` 時預設 `member`，未提供 `disabled` 時預設 `false`；客戶端**應**明確送出兩欄。 |
| `REQ-WEB-051` | 所有登入、登出、初始化、使用者變更**必須**寫入 `audit`；動作名稱分別為 `auth.setup`、`auth.login`、`user.create`、`user.update`、`auth.local-recovery`。 |
| `REQ-WEB-052` | 管理員**不得**透過任何端點讀取他人的 X Cookie、代理密碼或 AI Key；管理端點只暴露使用者中繼資料。 |

### 8.5 帳號、代理與 X 登入

| 方法與路徑 | 行為 |
| --- | --- |
| `GET /api/accounts` | 列出目前使用者帳號；去除 `secret` 與 `owner`；`proxy` 只回 `scheme://host:port`；附 `policy` 狀態 |
| `POST /api/accounts` | 建立帳號；需 `name`、完整 `proxy`、`warmup_days`；初始 `status=pending`、`x_id=pending-<id>` |
| `PUT /api/accounts/{id}/proxy` | 更換代理；使會話失效、暫停自動化、佇列轉草稿 |
| `POST /api/accounts/{id}/login` | 以 Cookie 或帳密登入 X；成功後綁定保護期 |
| `GET /api/accounts/{id}/login-status` | 回 `account_logins` 的 `stage,message,started,updated`；無資料回 `{"stage":"idle","message":"尚未开始登录"}` |
| `GET /api/accounts/{id}/request-logs` | 回最近 100 筆登入診斷；`request_fields` 與 `response_headers` 已解析為物件 |
| `POST /api/accounts/{id}/proxy-test` | 經綁定代理請求 `https://x.com/`，20 秒逾時；回 `status`、`seconds`、訊息 |
| `DELETE /api/accounts/{id}` | 刪除帳號；登入中或有 `sending` 任務時拒絕 |

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-060` | 建立帳號**必須**驗證代理：scheme ∈ {`http`,`https`,`socks5`,`socks5h`}、主機與埠存在、無 query／fragment、path 為空或 `/`、不含 CR/LF；否則回 `400 {"detail":"请绑定完整代理地址，例如 http://用户名:密码@代理主机:端口"}`。 |
| `REQ-WEB-061` | `name` 長度**必須**為 1–80；`warmup_days` 為 1–90，預設 4。 |
| `REQ-WEB-062` | 更換代理前**必須**確認帳號非 `logging_in` 且無 `sending` 任務；否則回 `409`（`账号正在登录，请稍后重试` 或 `账号正在发布，请稍后重试`）。 |
| `REQ-WEB-063` | 更換代理**必須**：保留 `chat_pin`、清除其他 `secret` 內容（含 Cookie）、`session_version += 1`、`status='pending'`、停用該帳號所有規則、把 `queued` 任務轉為 `draft`。回應訊息**必須**為 `代理已更新，请重新登录；自动化已暂停，待发布内容已转为草稿`。 |
| `REQ-WEB-064` | 登入端點**必須**以 `xlogin:<user_id>` 限流 8 次；並在 `sending` 任務存在時回 `409 {"detail":"请等待当前发布完成"}`。 |
| `REQ-WEB-065` | 登入**必須**以條件更新 `status='logging_in'` 奪取所有權；若已是 `logging_in` 回 `409 {"detail":"账号正在登录"}`。失敗時**必須**把狀態回復為 `pending`。 |
| `REQ-WEB-066` | Cookie 登入**必須**要求 `auth_token` 與 `ct0` 同時存在；否則回 `400 {"detail":"Cookie 需要包含 auth_token 和 ct0"}`。帳密登入**必須**要求非空使用者名稱與密碼；否則回 `400 {"detail":"请输入 X 用户名和密码"}`。 |
| `REQ-WEB-067` | 登入請求**必須**有 180 秒總期限；代理、X 使用者名稱、密碼、Cookie 與代理認證**不得**寫入日誌或回應。 |
| `REQ-WEB-068` | 登入成功前**必須**以 Viewer 驗證身分；未取得 `x_id`、`handle` 或有效會話回 `502 {"detail":"X 未返回有效用户身份，请检查会话"}`。 |
| `REQ-WEB-069` | 若帳號已綁定非 `pending-` 的 `x_id`，重新登入的 X 身分**必須**相同；不同回 `409 {"detail":"登录身份与此账号原绑定的 X 用户不同，请新增账号，避免已有任务发往错误账号"}`。 |
| `REQ-WEB-070` | 同一 `owner` 下重複綁定同一 X 帳號**必須**回 `409 {"detail":"此 X 账号已经添加"}`。 |
| `REQ-WEB-071` | X 要求驗證碼、2FA 或設備證明時**必須**回 `409 {"detail":"X 要求验证码、2FA 或设备验证，请在浏览器完成后导入 Cookie"}`。 |
| `REQ-WEB-072` | X 登入失敗**必須**回 `409`，訊息以 `X 登录失败：` 開頭，後接經脫敏且截斷至 350 字元的細節。 |
| `REQ-WEB-073` | 登入成功**必須**寫入 `x_id`、`handle`、`name`、`avatar`、加密後的 Cookie、`status='connected'`、`policy_id`、`cookie_updated`；並以 `INSERT IGNORE` 建立 `account_maintenance`，`next_run = now + 30`。 |
| `REQ-WEB-074` | 登入成功**必須**以 X 數字 ID 綁定 `account_policies`：`first_login=now`、`warmup_until=now+warmup_days×86400`、`limit_until=now+7×86400`；既有物理 ID 政策**不得**重設。 |
| `REQ-WEB-075` | 登入診斷**必須**記錄請求方法、主機＋路徑、狀態碼、耗時、欄位名、脫敏回應與 `Set-Cookie` 名稱；**不得**記錄 Cookie 值、密碼、代理認證或令牌。 |
| `REQ-WEB-076` | 登入階段**必須**依路徑映射：`Viewer` → `verifying`；`/actions/` → `credentials`；空路徑或 `/` → `connecting`；其他 → `preparing`；另有 `starting`、`connected`、`failed`。 |
| `REQ-WEB-077` | 刪除帳號前**必須**確認非 `logging_in` 且無 `sending` 任務；否則回 `409 {"detail":"账号正忙，请稍后重试"}`。 |
| `REQ-WEB-078` | 代理測試**必須**走與正式請求相同的閘道與帳號代理；**不得**在代理失敗時回退直連。 |
| `REQ-WEB-079` | 讀取任何需 X 會話的端點前**必須**確認 `status='connected'` 且 Cookie 可登入；否則標記 `expired`、停用養護與規則、把 `queued` 轉 `draft`，回 `409 {"detail":"请先绑定代理并完成账号登录"}` 或 `X 会话已过期，请重新登录账号`。 |
| `REQ-WEB-080` | 帳號列表的 `policy` **必須**包含 `stage`、`can_write`、`warmup_days`、`first_login`、`warmup_until`、`limit_until`、`daily_limit`、`today_used`、`remaining`、`timezone`。 |

### 8.6 保護期與每日發布配額

保護期以「該 X 數字 ID 第一次成功登入此工作台」為起點，**不得**因重新登入、換代理、刪除重加或刪除紀錄而重設。

| 階段 | 判定 | 讀取 | X 寫入 | 每北京日發布上限 |
| --- | --- | --- | --- | --- |
| `not_started` | 尚無 `policy_id` | 允許 | 禁止 | 0 |
| `warming` | `now < warmup_until` | 允許 | 禁止 | 0 |
| `limited` | `warmup_until ≤ now < limit_until` | 允許 | 允許 | 2 |
| `ready` | `now ≥ limit_until` | 允許 | 允許 | 無工作台上限 |

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-081` | `can_write` **必須**為 `not warming and status='connected'`；`not_started` 時為 `false`。 |
| `REQ-WEB-082` | 配額日**必須**以 `Asia/Shanghai`（UTC+8）自然日計算；`day` 存為 DATE；隔日邊界為當地午夜。 |
| `REQ-WEB-083` | `today_used` **必須**為當日 `publication_ledger.units` 總和，且排除 `state='rejected'`；`remaining` 僅在 `limited` 階段為 `max(0, 2 - today_used)`，其他階段為 `null`。 |
| `REQ-WEB-084` | 寫入前**必須**以 `publication_ledger` 原子預留額度；同一 `request_id` 已存在時回 `409 {"detail":"发布请求已有记录，请先核对结果，不能重复发送"}`。 |
| `REQ-WEB-085` | 未滿 7 天且 `used + units > 2` 時**必須**拋出可重排的等待：若 `units > 2` 等到 `limit_until`，否則等到 `min(下一個北京日, limit_until)`；狀態碼 `429`，訊息為 `账号首次登录后 7 天内每天最多 2 篇，今天额度不足；串文按每条计数`。 |
| `REQ-WEB-086` | 只讀期內的寫入**必須**拋出等待（`423`），訊息含到期時間，格式 `账号处于只读养护期，MM月DD日 HH:MM（北京时间）自动释放 X 写权限`。 |
| `REQ-WEB-087` | 一則貼文、回覆、引用、長帖、文章發布**必須**各計 1 單位；串文**必須**每條各計 1 單位；文章草稿不發布時計 0。 |
| `REQ-WEB-088` | X 明確拒絕時**必須**把預留額度歸還（`units=0, confirmed=0, state='rejected'`）。 |
| `REQ-WEB-089` | 結果不明時**必須**保留 1 單位並標記 `state='uncertain'`；已確認的單位數寫入 `confirmed`。 |
| `REQ-WEB-090` | 同一物理 X 帳號在不同工作台使用者之間**必須**共用同一 `account_policies` 與同一 ledger。 |
| `REQ-WEB-091` | 佇列因保護期或額度不足而等待時，**必須**把 `scheduled` 更新為 `retry_at` 並維持 `queued`；**不得**標記失敗。 |

### 8.7 時間線、貼文、互動、文章、探索與媒體

| 方法與路徑 | 權限 | 行為 |
| --- | --- | --- |
| `GET /api/accounts/{id}/timeline` | 會話 | 依 `kind` 讀取 `home`、`mine`、`search`、`detail`、`user`；回 `posts`、`users`、`cursor` |
| `GET /api/accounts/{id}/posts/{post_id}` | 會話 | 讀取單一貼文討論；回 `post`、`context`、`replies`、`cursor` |
| `POST /api/accounts/{id}/action` | 會話＋寫入權 | 執行按讚、轉推、書籤、刪除、追蹤等互動 |
| `POST /api/accounts/{id}/article-action` | 會話＋寫入權 | 刪除文章或發布既有文章 |
| `GET /api/capabilities` | 會話 | 回可用的讀取操作註冊表與傳輸模式 |
| `POST /api/accounts/{id}/explore` | 會話 | 依白名單操作與參數呼叫低階讀取 API |
| `POST /api/accounts/{id}/media` | 會話＋寫入權 | 上傳圖片、GIF 或影片，回本地媒體 ID |

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-100` | `kind` **必須**限於 `home`、`mine`、`search`、`detail`、`user`；其他值回 `400 {"detail":"未知时间线"}`。 |
| `REQ-WEB-101` | 查詢字串 `q` **不得**超過 500 字元，游標**不得**超過 4000 字元；`product` **必須**限於 `Top`、`Latest`、`People`、`Media`、`Lists`。 |
| `REQ-WEB-102` | `kind=search` 時 `q` **必須**非空，否則回 `400 {"detail":"请输入搜索关键词"}`；`kind=detail` 時 `q` **必須**非空，否則回 `400 {"detail":"请输入贴文 ID"}`。 |
| `REQ-WEB-103` | `kind=user` **必須**先解析使用者；查無使用者回 `404 {"detail":"用户不存在"}`；成功時回應**必須**在時間線物件外附 `profile`。 |
| `REQ-WEB-104` | 時間線輸出**必須**排除促銷條目（`promotedMetadata` 或 `entryId` 以 `promoted` 開頭），並以 `work_id` 去重。 |
| `REQ-WEB-105` | 貼文詳情**必須**只接受 ASCII 數字且長度 ≤30 的 `post_id`；否則回 `400 {"detail":"贴文 ID 无效"}`；游標長度限制 4000。 |
| `REQ-WEB-106` | 討論輸出**必須**分離三部分：`post` 為焦點貼文、`context` 為由遠祖到父帖的鏈、`replies` 為同會話的回覆；焦點缺席時 `post` 為 `null`，回覆仍**必須**回傳。 |
| `REQ-WEB-107` | 互動動作**必須**限於 `like`、`unlike`、`repost`、`unrepost`、`bookmark`、`unbookmark`、`delete`、`follow`、`unfollow`。 |
| `REQ-WEB-108` | `follow`／`unfollow` 的 `target` **必須**為純數字使用者 ID；其他動作**必須**先以貼文 ID 解析器處理，解析後仍**必須**為數字。 |
| `REQ-WEB-109` | 每次成功互動**必須**寫入稽核，動作名稱為 `x.<action>`，明細為目標字串；回應固定為 `{"ok": true}`。 |
| `REQ-WEB-110` | 文章 `delete` **不得**預留發布額度；文章 `publish` **必須**以 `article:<request_id>` 預留 1 單位，成功後確認 1 單位。 |
| `REQ-WEB-111` | 文章發布若收到 GraphQL 明確拒絕，**必須**釋放額度並標記 `rejected`；其他例外**必須**標記 `uncertain` 並保留 1 單位。 |
| `REQ-WEB-112` | 文章動作成功後**必須**寫入稽核 `article.<action>`，回應**必須**為去除敏感鍵後的原始結果。 |
| `REQ-WEB-113` | `capabilities.reads` **必須**逐項回傳 `id`、`label`、`params`；`dm_send` **必須**為 `true`；`proxy_required` **必須**為 `true`；`gateway` 依是否設定 `X_GATEWAY_URL` 回 `go-tls-client` 或 `curl_cffi`。 |
| `REQ-WEB-114` | `explore.operation` **必須**存在於讀取註冊表；參數鍵**必須**為該操作宣告欄位的子集；未知操作或參數回 `400`（`未支持的操作`／`参数不支持`）。 |
| `REQ-WEB-115` | `explore.params.user_ids` **必須**為 1–50 個純數字字串；其他參數**必須**為 ≤4000 字元的字串；空字串**必須**在呼叫前移除。 |
| `REQ-WEB-116` | `explore` 的 `lifecycle` 若存在**必須**為 `Draft` 或 `Published`；缺少必填參數回 `400 {"detail":"缺少必填参数"}`。 |
| `REQ-WEB-117` | `explore` 回應**必須**遞迴移除鍵名含 `token`、`cookie`、`secret`、`authorization`、`passcode`、`private_key`、`chat_pin` 的欄位。 |
| `REQ-WEB-118` | 媒體上傳**必須**接受 `.jpg`、`.jpeg`、`.png`、`.gif`、`.webp`、`.mp4`、`.mov`；`purpose` **必須**為 `post` 或 `article`；文章用途僅允許 JPG、PNG、WebP。 |
| `REQ-WEB-119` | 媒體檔**必須**非空且 ≤50 MiB；超限回 `413 {"detail":"文件不得超过 50 MB"}`；成功回 `{"id","name","mime"}`，且只保存 X 媒體 ID 與中繼資料。 |

### 8.8 X Chat 私信

| 方法與路徑 | 行為 |
| --- | --- |
| `GET /api/accounts/{id}/chat/inbox` | 讀取收件箱首頁或下一頁；首頁會快取系統事件中繼資料 |
| `GET /api/accounts/{id}/chat/status` | 回是否已存 PIN、是否支援傳送、目前是否可寫 |
| `GET /api/accounts/{id}/chat/recipient?handle=` | 解析單一使用者並建立一對一會話識別 |
| `GET /api/accounts/{id}/chat/public-keys?ids=` | 取得 1–100 位成員的聊天公鑰 |
| `PUT /api/accounts/{id}/chat/password` | 加密保存聊天 PIN |
| `DELETE /api/accounts/{id}/chat/password` | 移除聊天 PIN 並作廢解鎖上下文 |
| `GET /api/accounts/{id}/chat/conversations/{cid}` | 讀取會話分頁事件 |
| `POST /api/accounts/{id}/chat/send` | 送出已由瀏覽器加密的訊息事件 |
| `POST /api/accounts/{id}/chat/conversation-keys` | 為新會話提交成員金鑰封裝 |
| `POST /api/accounts/{id}/chat/unlock-context` | 建立短效金鑰恢復上下文 |
| `POST /api/accounts/{id}/chat/relay` | 經帳號代理轉送允許的密鑰服務請求 |
| `DELETE /api/accounts/{id}/chat/unlock-context/{ctx}` | 主動銷毀解鎖上下文 |

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-120` | 收件箱游標**不得**超過 5000 字元；游標**必須**為 URL-safe Base64 的 JSON，僅含 `cursor_id`、`graph_snapshot_id`、`graph_snapshot_restarted`，前兩者為 1–1000 字元字串，第三者為布林。 |
| `REQ-WEB-121` | 收件箱回應**必須**包含 `conversations`、`item_count`、`message_requests`、`complete`、`unrecognized_items`、`cursor`、`synced_at`、`send_supported`；無法辨識的結構**必須**回 `502`，**不得**當成空收件箱。 |
| `REQ-WEB-122` | 收件箱**必須**跳過 `is_deleted_by_viewer` 項目；同一 `conversation_id` 只輸出一次；成員去重後最多 100 位；標題優先序為群組名稱 → `群聊` → 對方名稱／帳號 → `给自己留言` → `私信会话`。 |
| `REQ-WEB-123` | 只有 `XChatGetInboxPageContinueCursor` 且具 `cursor_id` 時**必須**產生下一頁游標；`XChatGetInboxPageEndCursor` 時 `complete=true` 且 `cursor=null`。 |
| `REQ-WEB-124` | `chat/status` 的 `has_saved_pin` **必須**由解密後的 `secret.chat_pin` 判定；`can_write` **必須**套用保護期規則；`send_supported` 固定 `true`。 |
| `REQ-WEB-125` | `recipient` **必須**去除開頭 `@` 與空白，使用者名稱**必須**匹配 `^[A-Za-z0-9_]{1,15}$`；查無使用者回 `404 {"detail":"未找到该聊天用户"}`。 |
| `REQ-WEB-126` | 一對一會話 ID **必須**為雙方數字 ID 以數值升序、以 `:` 連接；自己與自己對話時**必須**重複同一 ID 兩次。 |
| `REQ-WEB-127` | `public-keys` 的 `ids` **必須**先去重，數量 1–100，每項為 ≤30 位 ASCII 數字；否則回 `400 {"detail":"聊天成员 ID 无效"}`；查詢**不得**要求 Juicebox token。 |
| `REQ-WEB-128` | 聊天 PIN 長度**必須**為 4–128；保存與刪除**必須**寫入稽核 `chat.password.saved`／`chat.password.removed`；變更 PIN **必須**作廢該帳號所有解鎖上下文。 |
| `REQ-WEB-129` | 會話 ID **必須**匹配 `^[a-zA-Z0-9:-]{1,200}$`；`before` 與 `key_before` 可省略，否則**必須**為 ≤20 位 ASCII 數字；省略時以 64 位有號整數上限作為起點。 |
| `REQ-WEB-130` | 會話回應**必須**包含去重後的事件、當頁事件、`before`、`has_more`、`send_supported`、`message_request_state`；事件總數上限 500。 |
| `REQ-WEB-131` | 與自己的會話**必須**過濾 X 自身的設定同步事件，並以 `system_event_count` 回報被過濾數量；中繼資料快取有效期 900 秒且綁定 `session_version`。 |
| `REQ-WEB-132` | 傳送前**必須**通過保護期檢查；`message_id` **必須**為標準 UUID 格式；`conversation_id` **必須**匹配 `^[a-zA-Z0-9:-]+$` 且 ≤200 字元。 |
| `REQ-WEB-133` | 加密事件與簽章**必須**為標準 Base64，建立事件 4–100000 字元、簽章 4–10000 字元；**不得**接受明文訊息欄位。 |
| `REQ-WEB-134` | 同一帳號同一 `message_id` **必須**只派送一次；重複回 `409 {"detail":"这条聊天请求已提交，请先刷新会话核对结果，不能重复发送"}`。派送紀錄只保存 `payload_hash`，**不得**保存密文或明文。 |
| `REQ-WEB-135` | 派送結果**必須**記為 `sending` → `responded`；GraphQL 明確拒絕記為 `rejected`；其他例外記為 `uncertain` 且**不得**自動重試。 |
| `REQ-WEB-136` | 新會話金鑰設定**必須**要求目前帳號在會話成員中，且成員集合與金鑰接收者完全一致；既有會話已有事件或 `has_more=true` 時回 `409`，**不得**覆寫原金鑰。 |
| `REQ-WEB-137` | 解鎖上下文**必須**先有已保存 PIN，否則回 `409 {"detail":"请先保存已在 X Chat 设置的聊天密码"}`；`unlock-context` 限流 6 次。 |
| `REQ-WEB-138` | 解鎖上下文**必須**只回傳 X 提供的最新金鑰版本、PIN、realm token、公鑰與恢復設定；**不得**在伺服器產生、替換或推導私鑰。 |
| `REQ-WEB-139` | 解鎖上下文有效期**必須**為 300 秒，同時存在上限 128 個；realm 位址**必須**為 HTTPS、無帳密／查詢／片段、埠為空或 443，主機限於 `realm-b.x.com`、`realm-east1.x.com`、`realm-west1.x.com`。 |
| `REQ-WEB-140` | 中繼請求**必須**驗證上下文屬於該帳號、未過期、`session_version` 一致且帳號 `connected`；否則回 `409 {"detail":"聊天解锁会话已失效，请重新解锁"}`。 |
| `REQ-WEB-141` | 中繼僅允許兩個目標：上下文允許清單內的 `POST`，或 `https://api.x.com/2/chat/features` 的 `GET`；其他回 `400 {"detail":"不允许访问此聊天服务地址"}`。 |
| `REQ-WEB-142` | 中繼 body **必須**為標準 Base64 且解碼後 ≤1 MB；標頭只允許 `content-type`、`accept`、`x-juicebox-version`，每值 ≤200 字元且不得含 CR/LF。 |
| `REQ-WEB-143` | 中繼**必須**由伺服器依 realm 選擇 `Authorization: Bearer`，**不得**轉送瀏覽器任意授權標頭；請求**必須**經帳號代理與閘道，30 秒逾時且不跟隨重定向。 |
| `REQ-WEB-144` | 中繼回應**必須** ≤2 MB，並以 `{"status","body","content_type"}` 回傳，`body` 為 Base64；超過上限回 `502`。 |
| `REQ-WEB-145` | 銷毀上下文**必須**只作用於目前使用者的帳號，且對不存在或已過期的上下文仍回 `{"ok": true}`。 |

### 8.9 AI 設定、資訊來源與條目

| 方法與路徑 | 行為 |
| --- | --- |
| `GET /api/ai` | 回目前使用者的 AI 設定；未設定時回預設值 |
| `PUT /api/ai` | 建立或更新 AI 設定；空白 API Key 保留原值 |
| `POST /api/ai/generate` | 依提示詞與至多 5 條條目產生貼文 |
| `GET /api/sources` | 列出目前使用者的來源，依建立時間遞減 |
| `POST /api/sources` | 新增 RSS 或網頁來源 |
| `DELETE /api/sources/{id}` | 刪除未被自動化規則引用的來源 |
| `POST /api/sources/{id}/refresh` | 立即抓取一次來源 |
| `GET /api/entries` | 列出最近 100 條條目 |

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-150` | 未設定 AI 時，`GET /api/ai` **必須**回 `base_url=https://api.openai.com/v1`、空 `model`、空 `instruction`、`configured=false`、`token_parameter=auto`。 |
| `REQ-WEB-151` | 已設定時**必須**回 `base_url`、`model`、`instruction`、`configured=true`、`has_key`、`token_parameter`；**不得**回傳 `owner` 或 `secret`。 |
| `REQ-WEB-152` | 保存 AI 設定**必須**驗證 `base_url` 為 HTTP(S)、無帳密、無片段、無反斜線／CR／LF；**不得**含查詢參數；私網主機僅在 `CONSOLE_PRIVATE_AI_HOSTS` 明列時允許。 |
| `REQ-WEB-153` | `base_url` 長度 8–500、`model` 1–100、`api_key` ≤1000、`instruction` ≤2000；`token_parameter` **必須**為 `auto`、`max_tokens`、`max_completion_tokens` 之一。 |
| `REQ-WEB-154` | `PUT /api/ai` 的 `api_key` 為空且已有設定時**必須**保留既有金鑰；首次設定可留空以連接無金鑰的本地服務；保存時**必須**移除 `base_url` 尾端 `/`。 |
| `REQ-WEB-155` | AI 產生**必須**限流 30 次／900 秒；`prompt` 1–4000 字元；`entry_ids` ≤5；`style` ≤500；`long` 為布林。 |
| `REQ-WEB-156` | 產生時每個 `entry_id` **必須**屬於目前使用者的來源；否則回 `404 {"detail":"热点不存在"}`；輸出**必須**為 `{"text","weight","sources"}`。 |
| `REQ-WEB-157` | AI 輸出**必須**非空且 ≤20000 字元；回應格式**必須**為 OpenAI `choices[0].message.content`；格式不符回 `502`。 |
| `REQ-WEB-158` | 產生提示**必須**把外部條目視為不可信資料、要求不捏造事實／數字／來源、保留至少一個原始連結；長文模式上限 3000 字，短文模式要求總權重 ≤280。 |
| `REQ-WEB-159` | 來源 `name` 長度 1–80、`url` 長度 8–1000；`kind` **必須**為 `rss` 或 `web`；`enabled` 預設 `true`。 |
| `REQ-WEB-160` | 新增來源**必須**先執行 SSRF 檢查：僅 HTTP(S)、無帳密／片段／反斜線／CR／LF，解析後所有位址**必須**為全域可路由；否則回 `400`。 |
| `REQ-WEB-161` | 刪除來源前**必須**檢查目前使用者的所有規則；任一規則引用即回 `409 {"detail":"请先移除引用此来源的自动化任务"}`。 |
| `REQ-WEB-162` | 刷新來源**必須**限流 30 次／900 秒；成功回 `{"count": <n>}`；失敗**必須**把安全化訊息寫入 `sources.error` 並回 `502`。 |
| `REQ-WEB-163` | RSS／Atom 每次**必須**最多處理 30 條；標題截斷 500、摘要截斷 12000；連結僅接受 `http`／`https`；無發佈時間時允許 `null`。 |
| `REQ-WEB-164` | 網頁來源**必須**只取靜態 HTML，移除 `script`、`style`、`nav`、`footer`、`header`、`form`、`noscript`、`iframe` 後取文字；正文截斷 12000。 |
| `REQ-WEB-165` | 網頁來源的去重連結**必須**為 `<原始 URL 去除片段>#snapshot-<正文 SHA-256 前 16 位十六進位>`，使內容變更才產生新條目。 |
| `REQ-WEB-166` | 條目寫入**必須**以 `(source_id,url_hash)` 唯一鍵忽略重複；成功抓取後**必須**更新 `last_fetch` 並清除 `error`。 |
| `REQ-WEB-167` | `GET /api/entries` **必須**以 `COALESCE(published,fetched)` 遞減排序，最多 100 筆，並附 `source_name`。 |

### 8.10 任務佇列與發布

| 方法與路徑 | 行為 |
| --- | --- |
| `GET /api/jobs` | 列出最近 200 筆任務，附帳號 `handle` 與 `name` |
| `POST /api/jobs` | 建立草稿或排入佇列；以 `request_id` 冪等 |
| `PUT /api/jobs/{id}` | 僅編輯 `draft` 任務 |
| `POST /api/jobs/{id}/cancel` | 僅將 `queued` 轉回 `draft` |
| `DELETE /api/jobs/{id}` | 僅刪除 `draft` 任務 |

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-170` | `GET /api/jobs` **必須**解析 `payload` 與 `result` 為 JSON 物件，依 `created` 遞減，最多 200 筆。 |
| `REQ-WEB-171` | 建立任務**必須**先驗證帳號屬於目前使用者；`state` **必須**為 `draft` 或 `queued`；`request_id` 長度 16–100。 |
| `REQ-WEB-172` | `state=queued` 時**必須**要求帳號 `connected`，否則回 `409 {"detail":"请先完成账号登录"}`；並以排程時間預先檢查保護期與額度。 |
| `REQ-WEB-173` | 任務正文**必須**非空：`thread` 使用 `texts`，其他使用 `text`；每段去除空白後**必須**非空且 ≤25000 字元。 |
| `REQ-WEB-174` | 非文章且未啟用長文時，每段**必須**滿足 `tweet_weight ≤ 280`；否則回 `400 {"detail":"普通贴文超过 280 权重，请缩短或启用 Premium 长贴文"}`。 |
| `REQ-WEB-175` | 文章任務**必須**有非空 `title`；`reply_to` 解析後**必須**為數字；`article_id` 若存在**必須**為純數字；`quote_url` **必須**為 `https` 且主機為 `x.com`／`twitter.com`，並可解析出貼文 ID。 |
| `REQ-WEB-176` | 排程時間**必須**落在「現在 − 300 秒」至「現在 + 366 天」之間；否則回 `400 {"detail":"发布时间必须在未来一年内"}`。 |
| `REQ-WEB-177` | 媒體 ID **必須**屬於目前使用者與該帳號；排入佇列時每項媒體**必須**在建立後 23 小時內，否則回 `400`（`媒体不存在或不属于该账号`／`X 媒体约 24 小时后过期，请在发布前重新上传`）。 |
| `REQ-WEB-178` | 多個媒體**必須**全為 `image/jpeg`、`image/png`、`image/webp`；影片或 GIF **不得**與其他媒體混合，否則回 `400 {"detail":"视频或 GIF 不能与其他媒体混合"}`。 |
| `REQ-WEB-179` | `POST /api/jobs` 遇到同 `owner` 同 `request_id` **必須**回既有 `{"id","duplicate":true}`，**不得**建立第二列；新任務**必須**寫入稽核 `job.<state>`。 |
| `REQ-WEB-180` | 編輯任務**必須**以條件更新 `WHERE state='draft'`；非草稿回 `409 {"detail":"仅草稿可以编辑，请先取消排期"}`。 |
| `REQ-WEB-181` | 取消**必須**以條件更新 `queued → draft`；否則回 `409 {"detail":"任务已开始或不在队列，无法取消"}`。 |
| `REQ-WEB-182` | 刪除**必須**以條件刪除 `state='draft'`；否則回 `409 {"detail":"只能删除草稿"}`。 |
| `REQ-WEB-183` | 排程器每次 tick **必須**只領取一筆符合條件的任務：`queued`、`scheduled ≤ now`、使用者未停用、帳號 `connected`，依 `scheduled` 排序；領取**必須**以交易將狀態改為 `sending`。 |
| `REQ-WEB-184` | 發布前**必須**重新確認帳號存在、可寫、媒體未過期（23 小時）；任一不符**必須**失敗且不呼叫 X 寫入。 |
| `REQ-WEB-185` | 發布單位數**必須**為：一般貼文 1、串文每條 1、文章發布 1、文章僅存草稿 0；有單位時**必須**先原子預留，再執行 X 寫入。 |
| `REQ-WEB-186` | 串文**必須**逐條送出，第一條帶媒體與引用，其餘回覆前一條；每條之間**必須**可被停止事件打斷（最長等待 2 秒）；每成功一條**必須**立即確認額度並持久化已得 `tweet_ids`。 |
| `REQ-WEB-187` | 文章任務**必須**以 Markdown 轉內容狀態；插圖路徑 `media:<本地媒體 ID>` **必須**解析為同帳號未過期媒體；未提供 `article_id` 時**必須**先建立草稿；標題、內容、封面依序更新。 |
| `REQ-WEB-188` | 文章發布成功**必須**記錄 `tweet_ids`、`article_id`、`edit_url`；`edit_url` 格式**必須**為 `https://x.com/compose/articles/edit/<article_id>`。 |
| `REQ-WEB-189` | 發布成功**必須**把任務設為 `sent`；GraphQL 明確拒絕或 HTTP 業務錯誤設為 `failed`；已開始寫入後的其他例外設為 `uncertain`。 |
| `REQ-WEB-190` | `uncertain` 任務**不得**自動重試；錯誤訊息**必須**安全化，且結果與錯誤**必須**持久化後才寫稽核 `publish.<state>`。 |
| `REQ-WEB-191` | 保護期或額度等待**必須**把任務保持 `queued`，將 `scheduled` 更新為 `retry_at`，並記錄等待訊息；**不得**計為失敗。 |
| `REQ-WEB-192` | 發布途中遇到 GraphQL `32` 或 `89` **必須**將帳號設為 `expired`、停用該帳號所有規則、把 `queued` 任務轉為 `draft`。 |
| `REQ-WEB-193` | 程序重啟**必須**把殘留 `sending` 任務標為 `uncertain`，錯誤固定為「服務曾在發送期間中斷，請先在 X 核對；不會自動重發」。 |
| `REQ-WEB-194` | 程序重啟**必須**把 `publication_ledger.state='reserved'` 標為 `uncertain`，避免未確認的發布被當成未使用額度。 |
| `REQ-WEB-195` | 自動化規則每次執行**必須**先計算該規則最近 24 小時建立的任務數；達 `daily_limit` 即停止並記錄「已达到最近 24 小时生成上限」。 |
| `REQ-WEB-196` | 自動化**必須**只讀取啟用中的來源，每來源取最新 5 條未處理條目，合併後取最新 3 條；無候選即結束且不建立任務。 |
| `REQ-WEB-197` | 自動化產生內容**必須**滿足權重 ≤280；超過時**不得**發布，且**不得**消耗來源去重紀錄。 |
| `REQ-WEB-198` | 自動化建立任務前**必須**在交易內重新讀取規則；規則已刪除或停用**必須**放棄，且**不得**寫入 `rule_entries`。 |
| `REQ-WEB-199` | 自動化任務**必須**在 `mode=publish` 且帳號可寫、當日額度未用盡時為 `queued`，否則為 `draft`；`request_id` **必須**等於任務 ID。 |
| `REQ-WEB-200` | 自動化結束**必須**重設 `running=0`，寫入 `last_run`、`last_result`，並稽核 `automation.run`；結果字串**必須**區分「已加入发布队列」、「已生成待审核草稿」與暫停／上限／錯誤原因。 |

### 8.11 自動化規則、關鍵詞與帳號養護

| 方法與路徑 | 行為 |
| --- | --- |
| `GET /api/rules` | 列出目前使用者規則，`source_ids` 解析為陣列 |
| `POST /api/rules` | 建立規則；下次執行時間為現在加間隔 |
| `PUT /api/rules/{id}` | 更新規則；執行中拒絕 |
| `POST /api/rules/{id}/run` | 要求立即執行一次 |
| `POST /api/rules/{id}/pause` | 停用規則並把其 `queued` 任務轉草稿 |
| `DELETE /api/rules/{id}` | 刪除規則；執行中拒絕 |
| `GET /api/keyword-groups` | 列出關鍵詞分組 |
| `POST /api/keyword-groups` | 建立分組 |
| `PUT /api/keyword-groups/{id}` | 更新分組 |
| `DELETE /api/keyword-groups/{id}` | 刪除未被帳號綁定的分組 |
| `GET /api/maintenance` | 列出各帳號養護設定、狀態與保護期 |
| `PUT /api/accounts/{id}/maintenance` | 建立或更新養護設定 |
| `POST /api/accounts/{id}/maintenance/run` | 要求立即執行一步 |
| `POST /api/accounts/{id}/maintenance/pause` | 停用養護 |
| `GET /api/accounts/{id}/maintenance/logs` | 回最近 100 筆養護日誌 |

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-210` | 規則 `name` 長度 1–100；`source_ids` 1–10；`prompt` 1–4000；`interval_minutes` 15–43200；`daily_limit` 1–24；`mode ∈ {draft, publish}`；`enabled` 預設 `false`。 |
| `REQ-WEB-211` | 建立或更新規則**必須**確認帳號屬於目前使用者；`enabled=true` 時帳號**必須** `connected`，否則回 `409 {"detail":"请先完成账号登录"}`。 |
| `REQ-WEB-212` | 規則**必須**要求目前使用者已有 AI 設定，否則回 `409 {"detail":"请先连接 AI"}`；每個來源 ID **必須**屬於目前使用者。 |
| `REQ-WEB-213` | 建立規則**必須**把 `next_run` 設為 `now + interval_minutes × 60`；更新規則**必須**重設同一間隔。 |
| `REQ-WEB-214` | 規則 `running=true` 時**不得**更新或刪除，分別回 `409`（`任务正在执行，请完成后编辑`／`请等待任务运行完成`）。 |
| `REQ-WEB-215` | 立即執行**必須**要求 `enabled=true` 且 `running=false`，否則回 `409`；成功只把 `next_run` 設為現在，實際執行由排程器完成。 |
| `REQ-WEB-216` | 暫停或刪除規則**必須**在同一交易把該規則所有 `queued` 任務轉為 `draft`；刪除後規則列消失。 |
| `REQ-WEB-217` | 關鍵詞分組 `name` 長度 1–100、`keywords` 1–100；每個關鍵詞**必須**去除前後空白、忽略空字串、去重，且長度 ≤200；否則回 `400`。 |
| `REQ-WEB-218` | 刪除關鍵詞分組前**必須**檢查目前使用者所有帳號養護設定；任一綁定即回 `409 {"detail":"请先解除账号对此关键词分组的绑定"}`。 |
| `REQ-WEB-219` | 養護設定的 `group_ids` ≤20 且每個**必須**屬於目前使用者；`interval_minutes` 15–1440；`pages_per_cycle` 1–10；`home_pages_per_cycle` 1–10；`delay_min_seconds` 5–600；`delay_max_seconds` 5–1800；`comment_pages` 1–5。 |
| `REQ-WEB-220` | 養護設定**必須**要求 `delay_max_seconds ≥ delay_min_seconds`，否則回 `400 {"detail":"随机延迟最大值不能小于最小值"}`；`running=true` 時回 `409 {"detail":"正在读取当前页，请完成后再修改配置"}`。 |
| `REQ-WEB-221` | 更新養護設定**必須**重設游標狀態為空物件，`next_run` 設為 `now + delay_min_seconds`，並稽核 `maintenance.configure`。 |
| `REQ-WEB-222` | 手動養護**必須**要求帳號 `connected`、設定存在且啟用、`running=false`、距上次執行 ≥ `delay_min_seconds`，且狀態為空或 `next_run ≤ now`；否則回 `409`。 |
| `REQ-WEB-223` | 手動養護成功只把 `next_run` 設為現在；**不得**同步執行 X 請求。暫停只把 `enabled` 設為 `0`。 |
| `REQ-WEB-224` | 養護日誌**必須**依 `id` 遞減取 100 筆，`details` 解析為物件；保留期 30 天，逾期刪除。 |
| `REQ-WEB-225` | 一輪養護的來源順序**必須**為：`read_home=true` 時的 `home`，其後為 `search`（僅當有可用關鍵詞）；關鍵詞**必須**從綁定分組去重後隨機選一個。 |
| `REQ-WEB-226` | `home` 每頁 20 條、上限 `home_pages_per_cycle`；`search` 每頁 20 條、產品固定 `Latest`、上限 `pages_per_cycle`。 |
| `REQ-WEB-227` | 停止翻頁原因**必須**為：達到頁數上限 `page_limit`、無下一頁游標 `no_cursor`、游標與本頁輸入相同 `repeated_cursor`。 |
| `REQ-WEB-228` | 若 `read_comments=true` 且有貼文，**必須**隨機選 1 條，讀取其評論至 `comment_pages` 上限；評論讀完才推進來源。 |
| `REQ-WEB-229` | 每步之間**必須**等待 `[delay_min_seconds, delay_max_seconds]` 均勻隨機秒數；一輪結束後**必須**等待 `interval_minutes × 60 × U(0.8,1.2)`。 |
| `REQ-WEB-230` | 領取養護任務時**必須**先把 `next_run` 設為 `now + 180 + delay_max_seconds`，避免程序中斷後立即重複領取。 |
| `REQ-WEB-231` | 養護失敗**必須**至少退避 900 秒或 `interval_minutes × 60`（取大者）；遇到 `88`、`226`、`344` 或 HTTP 429 **必須**退避 6 小時。 |
| `REQ-WEB-232` | 養護遇到 GraphQL `32`／`89` 或 HTTP 401 **必須**把帳號設為 `expired`，停用養護與所有規則，並清空游標狀態。 |
| `REQ-WEB-233` | 無任何來源且無關鍵詞時，**必須**只執行 `viewer` 身份保活；回傳身分**必須**與綁定 `x_id` 相同，否則記為失敗。 |
| `REQ-WEB-234` | 養護日誌狀態**必須**為 `ok`、`warning`、`error`、`running`、`interrupted`；X 回傳部分錯誤時為 `warning` 並保留請求明細。 |
| `REQ-WEB-235` | 程序中斷時**必須**把 `running` 日誌改為 `interrupted`、`running` 旗標歸零，並在 `details.interrupted=true`；已收到的結果**不得**刪除。 |

### 8.12 稽核、統計與輸入驗證矩陣

| 方法與路徑 | 行為 |
| --- | --- |
| `GET /api/audit` | 回目前使用者最近 100 筆稽核，欄位為 `action`、`detail`、`created` |
| `GET /api/stats` | 回帳號數、任務狀態計數、啟用中規則數 |

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-240` | 稽核查詢**必須**只回目前使用者資料，依 `id` 遞減，最多 100 筆；`detail` 寫入時**必須**截斷至 500 字元。 |
| `REQ-WEB-241` | 統計**必須**回 `{"accounts": <int>, "jobs": {<state>: <count>}, "rules": <enabled count>}`；沒有任務時 `jobs` 為空物件。 |

**輸入模型驗證矩陣**

所有請求模型**必須**拒絕未知欄位（`extra=forbid`），字串預設去除前後空白；密碼欄位例外，**不得**去除空白。

| 模型 | 欄位與限制 |
| --- | --- |
| `Credentials` | `username` 3–40、`^[a-zA-Z0-9_.@-]+$`；`password` 12–128 |
| `Setup` | 繼承 `Credentials`；`token` 16–256 |
| `LoginCredentials` | 繼承 `Credentials`；`password` 改為 1–128 |
| `UserCreate` | 繼承 `Credentials`；`role ∈ {admin, member}`，預設 `member` |
| `UserUpdate` | `disabled` 布林預設 `false`；`role ∈ {admin, member}` 預設 `member` |
| `Password` | `current` 1–128；`new` 12–128 |
| `AccountCreate` | `name` 1–80；`proxy` 8–1000；`warmup_days` 1–90，預設 4 |
| `Proxy` | `proxy` 8–1000 |
| `LoginAccount` | `username` ≤100；`password` ≤256；`cookies` ≤20000；`profile` 為物件、陣列或 `null` |
| `KeywordGroup` | `name` 1–100；`keywords` 1–100 項 |
| `Maintenance` | 見 `REQ-WEB-219`；`enabled`、`read_home`、`read_comments` 預設 `true` |
| `ChatPassword` | `pin` 4–128 |
| `ChatRelay` | `context_id` 20–100；`url` 8–500；`method ∈ {GET, POST}`；`headers` ≤10 鍵；`body` ≤1,400,000 |
| `ChatSend` | `conversation_id` 1–200、`^[a-zA-Z0-9:-]+$`；`message_id` UUID；兩個 Base64 欄位見 `REQ-WEB-133` |
| `ChatParticipantKey` | `user_id`、`public_key_version` 為 1–30 位數字；`encrypted_conversation_key` 4–10000 Base64 |
| `ChatSignature` | `signing_public_key` 4–1000 Base64；`public_key_version` 1–30 位數字；`signature_version` 1–10 位數字；`signature` 4–1000 Base64 |
| `ChatKeyAction` | `message_id` UUID；`encoded_message_event_detail` 4–100000 Base64；`message_event_signature` 為 `ChatSignature` |
| `ChatKeySetup` | `conversation_id` 為 `<數字>:<數字>`；`conversation_key_version` 1–30 位數字；參與者金鑰 1–2；動作簽章恰 1 |
| `AIConfig` | `base_url` 8–500；`model` 1–100；`api_key` ≤1000；`instruction` ≤2000；`token_parameter` 三選一 |
| `Generate` | `prompt` 1–4000；`entry_ids` ≤5；`style` ≤500；`long` 布林 |
| `Source` | `name` 1–80；`url` 8–1000；`kind ∈ {rss, web}`；`enabled` 布林 |
| `Rule` | 見 `REQ-WEB-210` |
| `Payload` | `kind ∈ {post, thread, article}`；`text` ≤25000；`texts` ≤25；`title` ≤250；`media` ≤4；`reply_to` ≤200；`quote_url` ≤300；`article_id` ≤100；`caption` ≤1000；`reply_mode ∈ {"", All, Community, Verified, ByInvitation}`；`sources` ≤5 |
| `Job` | `account_id` 字串；`request_id` 16–100；`payload` 為 `Payload`；`state ∈ {draft, queued}`；`scheduled` 可為 `null` |
| `Action` | 九種動作；`target` 1–250 |
| `Explorer` | `operation` 字串；`params` 物件 |
| `ArticleAction` | `action ∈ {delete, publish}`；`article_id` 純數字；`caption` ≤1000；`request_id` 16–60，預設 32 位十六進位 |

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-242` | 驗證錯誤**必須**以 `loc[1:]` 的路徑以 `.` 連接作為 `field`，`message` 使用驗證器訊息；**不得**包含原始輸入值。 |
| `REQ-WEB-243` | 所有非 `GET`／`HEAD`／`OPTIONS` 的 API 請求**必須**先通過 `X-Requested-With` 與 Origin 檢查，再進入模型驗證；兩者失敗優先於 `422`。 |

### 8.13 狀態機

**任務（`jobs.state`）**

| 來源狀態 | 事件 | 目的狀態 | 備註 |
| --- | --- | --- | --- |
| — | 建立草稿 | `draft` | 不檢查帳號可寫 |
| `draft` | 建立或編輯為 `queued` | `queued` | 需 `connected` 且通過保護期預檢 |
| `queued` | 取消／換代理／規則暫停／會話失效 | `draft` | 條件更新 |
| `queued` | 排程器原子領取 | `sending` | 僅一筆 |
| `sending` | X 全部成功 | `sent` | 逐條確認額度 |
| `sending` | 明確拒絕或 HTTP 業務錯誤 | `failed` | 不重試 |
| `sending` | 已寫入後傳輸例外 | `uncertain` | 保留額度，不重試 |
| `sending` | 保護期／額度等待 | `queued` | `scheduled=retry_at` |
| `sending` | 程序中斷後復原 | `uncertain` | 重啟時批次處理 |

**規則（`rules`）**

| 欄位／狀態 | 轉移 |
| --- | --- |
| `enabled` | 建立預設 `false`；可切換 `true`／`false`；暫停、帳號失效、會話過期時強制 `false` |
| `running` | 排程器以交易 `0 → 1` 並同時推進 `next_run`；執行完或中斷復原時 `1 → 0` |
| `next_run` | 建立／更新為 `now + interval`；立即執行為 `now`；領取時先推進至下一個間隔 |
| `last_result` | 每次執行結束寫入固定結果字串或安全化錯誤 |

**養護（`account_maintenance`）**

| 欄位／狀態 | 轉移 |
| --- | --- |
| `state` | 空物件代表新一輪；每步保存 `sources`、`source_index`、`phase`、頁碼、游標、`feed_done`、`cycle_id`；完成一輪回到空物件 |
| `running` | 領取時 `0 → 1`；`finally` 區塊 `1 → 0`；中斷復原強制 `0` |
| `next_run` | 領取時 `now + 180 + delay_max`；頁間 `now + U(delay_min,delay_max)`；輪末 `now + interval×60×U(0.8,1.2)`；錯誤退避 |
| `enabled` | 設定時可為 `true`／`false`；暫停、帳號失效時 `false` |

**發布帳本（`publication_ledger.state`）**

| 來源狀態 | 事件 | 目的狀態 | `units` |
| --- | --- | --- | --- |
| — | 原子預留 | `reserved` | 請求單位數 |
| `reserved` | 全部確認 | `sent` | `confirmed` |
| `reserved` | 明確拒絕且未確認 | `rejected` | `0` |
| `reserved` | 部分確認後不明 | `uncertain` | `confirmed + 1` |
| `reserved` | 程序中斷復原 | `uncertain` | 保留原值 |

**聊天派送（`chat_dispatches.state`）**

`insert → sending → responded`；GraphQL 明確拒絕為 `rejected`；其他例外為 `uncertain`。`sending` 與 `uncertain` **不得**自動重送。

**帳號（`accounts.status`）**

| 來源狀態 | 事件 | 目的狀態 |
| --- | --- | --- |
| `pending` | 開始登入（條件更新） | `logging_in` |
| `logging_in` | 登入成功 | `connected` |
| `logging_in` | 登入失敗 | `pending` |
| `connected` | 換代理 | `pending` |
| `connected` | Cookie 無效／X 回 32、89、401 | `expired` |
| `logging_in` | 程序重啟 | `pending` |

**登入階段（`account_logins.stage`）**

`starting → preparing → connecting → credentials → verifying → connected`；任何階段失敗為 `failed`；無資料時查詢回 `idle`。階段由請求路徑映射，**不得**由使用者輸入決定。

**解鎖上下文（記憶體）**

`建立（300 秒 TTL）→ 使用（中繼）→ 銷毀（DELETE／PIN 變更／TTL 到期）`。上下文**必須**綁定帳號、`session_version` 與允許的 URL 集合；過期即不可用。

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-250` | 所有狀態轉移**必須**以條件更新或交易完成；**不得**先讀後寫而無鎖。 |
| `REQ-WEB-251` | 任何 `uncertain` 狀態**不得**由系統自動轉回可重試狀態；只能由人員核對後建立新請求。 |
| `REQ-WEB-252` | 狀態機**不得**允許 `draft → sending`、`sent → queued`、`failed → sending`、`rejected → sent` 等跳躍。 |

### 8.14 併發、原子性與資源不變量

| 需求 | 內容 |
| --- | --- |
| `REQ-WEB-260` | 初始化、佇列領取、配額預留、限流計數**必須**在 `transaction_lock` 的單列 `FOR UPDATE` 下序列化；交易提交或連線中斷時**必須**自動釋放。 |
| `REQ-WEB-261` | 全域排程**必須**以 MySQL 具名鎖 `<database>:scheduler` 保證唯一；鎖連線**必須**持續存活並定期 `ping`；失去鎖**必須**停止續跑、重新取得並執行復原。 |
| `REQ-WEB-262` | 同一資料庫**不得**啟動第二個排程器；第二個實例取鎖失敗**必須**拒絕啟動，不得以唯讀模式共用排程。 |
| `REQ-WEB-263` | 任務建立、聊天派送、發布請求**必須**以資料庫唯一鍵實現冪等；重複請求**必須**回既有結果或 `409`，**不得**重送上游。 |
| `REQ-WEB-264` | 每次 X 寫入成功後**必須**先持久化已確認結果，再繼續下一段；程序中斷時已確認部分**不得**遺失。 |
| `REQ-WEB-265` | 帳號 `session_version` **必須**在換代理時遞增；舊版本的 Cookie 回呼**不得**覆寫新會話，聊天上下文**必須**同步失效。 |
| `REQ-WEB-266` | 所有使用者資源查詢**必須**同時以 `id` 與 `owner` 限定；跨使用者存取一律回 `404`，不得以 `403` 洩漏存在性。 |
| `REQ-WEB-267` | 所有請求體**必須**在路由與 multipart 解析前受限；一般 2 MiB、`/media` 55 MiB；分塊傳輸不得繞過。 |
| `REQ-WEB-268` | 所有上游請求**必須**經帳號代理與 Go 閘道；代理缺失、閘道不可用或憑證錯誤**必須**失敗關閉，**不得**回退直連。 |
| `REQ-WEB-269` | 所有祕密（Cookie、代理帳密、AI Key、聊天 PIN、realm token、私鑰）**不得**出現在回應、日誌、稽核、診斷或前端狀態中。 |
| `REQ-WEB-270` | 聊天私鑰與明文**不得**離開瀏覽器 Worker；伺服器只處理密文事件、封裝後的金鑰與中繼位元組。 |
| `REQ-WEB-271` | 時間欄位**必須**以 Unix epoch 秒儲存；配額日界**必須**以 `Asia/Shanghai` 計算；顯示時間**必須**以北京時間標示。 |
| `REQ-WEB-272` | 所有清單端點**必須**有固定上限（任務 200、稽核 100、日誌 100、條目 100、收件箱事件 500），不得無限成長。 |
| `REQ-WEB-273` | 所有外部輸入**必須**經白名單（操作、產品、動作、媒體副檔名、realm 主機、中繼 URL、AI 主機）而非黑名單。 |
| `REQ-WEB-274` | 稽核與診斷寫入**不得**因失敗而中斷主要業務；但主要業務的狀態與結果**必須**在回應前持久化。 |
| `REQ-WEB-275` | 任何快取（聊天中繼資料、解鎖上下文）**必須**有 TTL 與數量上限，且不得跨帳號或跨 `session_version` 重用。 |

## 9. 行為規格：前端工作台

前端是無框架的單頁應用，由 `index.html` 載入 ES 模組 `app.js`，所有狀態保存在瀏覽器記憶體；唯一持久化的前端狀態是 `localStorage["xapis-account"]`。

### 9.1 路由與頁面

| 頁面識別 | 導覽標題 | 用途 |
| --- | --- | --- |
| `home` | 主页 | 總覽、快速發布、近期任務 |
| `mine` | 我的贴文 | 目前帳號的貼文時間線 |
| `explore` | 探索 | 搜尋、使用者搜尋、貼文詳情入口 |
| `post` | 贴文详情 | 焦點貼文、祖先與回覆 |
| `jobs` | 内容工作室 | 草稿、排程、發布結果 |
| `articles` | 文章 | 文章草稿與發布 |
| `messages` | 私信 | X Chat 收件箱與會話 |
| `sources` | 热点雷达 | RSS／網頁來源與條目 |
| `rules` | 自动化 | 規則與執行結果 |
| `accounts` | 账号管理 | 帳號、代理、登入 |
| `settings` | 设置 | AI、密碼、使用者管理 |
| `tools` | API 工具箱 | 低階讀取操作 |
| `maintenance` | 账号养护 | 只讀養護設定與日誌 |
| `keywords` | 关键词目录 | 關鍵詞分組 |

| 需求 | 內容 |
| --- | --- |
| `REQ-FE-001` | 前端**必須**為單一 `index.html` 搭配外部 ES 模組；**不得**使用內嵌 script、內嵌事件屬性或 `eval`。 |
| `REQ-FE-002` | 路由**必須**使用 URL fragment；`#<page>` 中 `<page>` 不屬於 14 個頁面集合時**必須**回 `home`。 |
| `REQ-FE-003` | `#post/<id>` **必須**解析為 `post` 頁與 `detailId`；`id` 非 1–30 位數字時**必須**回 `home`。 |
| `REQ-FE-004` | `#explore` 的 `q`、`kind`、`product` **必須**可還原；`kind` 僅 `user` 或 `search`（預設 `search`）；`product` 僅 `Latest`、`Top`、`People`、`Media`、`Lists`（預設 `Latest`）。 |
| `REQ-FE-005` | 路由序列化**必須**省略空 `q`、`kind=search`、`product=Latest`；非 `explore` 頁面**不得**附查詢字串。 |
| `REQ-FE-006` | 貼文連結解析**必須**接受純數字或 `x.com`、`www.x.com`、`twitter.com`、`www.twitter.com` 的 `/status/<id>`；其他主機或格式回 `null`。 |
| `REQ-FE-007` | 導覽**必須**以 `history.pushState` 寫入新路由；由瀏覽器前後鍵或直接載入時**必須**以 `replaceState` 還原，且 `hashchange` 與 `popstate` 皆**必須**觸發路由解析。 |
| `REQ-FE-008` | 貼文詳情返回**必須**保存來源路由；無有效來源時回 `#home`。 |
| `REQ-FE-009` | 分頁合併**必須**以 `work_id` 去重，保留既有順序，只追加未出現項目，且**不得**修改原陣列。 |
| `REQ-FE-010` | 帳號切換**必須**寫入 `localStorage["xapis-account"]`；載入時若保存值不存在於帳號清單，**必須**改用第一個帳號或空字串。 |

### 9.2 API 包裝與錯誤呈現

| 需求 | 內容 |
| --- | --- |
| `REQ-FE-020` | 所有 API 呼叫**必須**經單一包裝函式，路徑前綴固定 `/api`，並帶 `credentials: "same-origin"`。 |
| `REQ-FE-021` | 每個請求**必須**帶 `X-Requested-With: XApis`；取得 CSRF 後**必須**帶 `X-CSRF-Token`；JSON body **必須**設定 `Content-Type: application/json`；`FormData` **不得**設定 Content-Type。 |
| `REQ-FE-022` | 回應**必須**先解析 JSON；解析失敗**必須**拋出「服務暫時不可用，請稍後重試」。 |
| `REQ-FE-023` | 非 2xx **必須**以 `detail` 為主要訊息；若有 `fields`，**必須**逐項以 `<field>: <message>`、以「；」連接附加。 |
| `REQ-FE-024` | 已登入狀態收到 `401` **必須**清除聊天金庫、聊天狀態與使用者，並回到登入畫面；**不得**自動重送原請求。 |
| `REQ-FE-025` | 登入與初始化成功**必須**以回應更新 `user` 與 `csrf`，重新載入基礎資料後導向 `home`。 |
| `REQ-FE-026` | 啟動流程**必須**為：`GET /api/auth/status` → 未初始化顯示初始化表單；已初始化則 `GET /api/auth/me` → 失敗顯示登入；成功後載入帳號、統計、條目、能力，再解析目前路由。 |
| `REQ-FE-027` | 所有顯示於 HTML 的外部字串**必須**經 HTML 跳脫；**不得**以字串拼接未跳脫的使用者內容。 |
| `REQ-FE-028` | 前端**不得**把 Cookie、代理密碼、AI Key、聊天 PIN、realm token 或私鑰寫入 DOM、`localStorage`、`sessionStorage` 或 URL。 |

### 9.3 輪詢與即時更新

| 場景 | 間隔 | 條件與行為 |
| --- | --- | --- |
| X 登入進度 | 1.5 秒 | 登入請求進行中且表單仍在畫面；顯示階段訊息與已等待秒數；前一次未完成不重疊 |
| 任務與規則頁 | 10 秒 | 已登入、頁面可見、無開啟中的 modal；僅在 `jobs` 或 `rules` 頁；以版本號避免舊回應覆寫；內容有變才重繪 |
| 養護日誌 modal | 5 秒 | modal 仍開啟且仍為同一帳號；只讀本地日誌，不觸發 X 請求；保留展開狀態與捲動位置 |
| 聊天收件箱與會話 | 30 秒 | 已解鎖、頁面為 `messages`、分頁可見、非忙碌／傳送中／載入中、焦點不在收件箱表單、輸入框為空 |

| 需求 | 內容 |
| --- | --- |
| `REQ-FE-030` | 輪詢**必須**在頁面隱藏、modal 開啟或目前操作未完成時跳過，且**不得**並行重入。 |
| `REQ-FE-031` | 輪詢回應**必須**以版本號或狀態物件比對，較舊的回應**不得**覆寫較新的頁面。 |
| `REQ-FE-032` | 輪詢失敗**不得**替換目前頁面或清空資料；**不得**自動重試寫入操作。 |
| `REQ-FE-033` | 聊天輪詢**不得**自動解鎖、自動送出或自動重送任何訊息。 |

### 9.4 聊天金庫與 Worker 協定

| 需求 | 內容 |
| --- | --- |
| `REQ-FE-040` | 聊天金庫**必須**在專用 Web Worker 中執行；主執行緒只持有 Worker 控制代碼與顯示用訊息。 |
| `REQ-FE-041` | Worker 請求**必須**有單調遞增 `id`；同一時間只允許一個步驟；每個請求 120 秒逾時，逾時**必須**終止 Worker 並拒絕所有待處理請求。 |
| `REQ-FE-042` | Worker 協定**必須**支援 `unlock`、`decrypt`、`send`、`lock` 四種訊息；未知訊息**必須**回錯誤。 |
| `REQ-FE-043` | `unlock` **必須**先取得解鎖上下文，將 PIN 轉為位元組後立即從上下文物件刪除，載入加密 SDK、解鎖、驗證公鑰版本與身分，成功後回 `{unlocked, context_id}`。 |
| `REQ-FE-044` | `unlock` 失敗**必須**釋放 SDK、刪除伺服器上下文並回具分類的錯誤；PIN 位元組**必須**在 `finally` 清零。 |
| `REQ-FE-045` | `decrypt` **必須**要求已解鎖，先取得參與者簽章金鑰，解密事件後只回傳顯示用訊息、分頁游標、失敗數與脱敏診斷。 |
| `REQ-FE-046` | `send` **必須**要求已解鎖且 `chat/status.can_write`；文字 1–10000 字元；既有會話**必須**先完整驗證事件，新會話**必須**先提交成員金鑰封裝。 |
| `REQ-FE-047` | `send` **必須**在送出後確認回傳事件中存在本次 `message_id`；未確認**必須**要求使用者刷新核對，**不得**重送。 |
| `REQ-FE-048` | Worker 內所有跨來源 `fetch` **必須**改寫為 `chat/relay`；同源請求**必須**直接放行；未初始化時**必須**拒絕跨來源請求。 |
| `REQ-FE-049` | Worker 錯誤**必須**映射為 `invalid_pin`、`not_registered`、`invalid_auth`、`protocol_version`、`rate_limit`、`chat_error`、`network`、`unlock_failed`；診斷字串**必須**去除長 token、長數字與位元組陣列。 |
| `REQ-FE-050` | `lock` **必須**釋放 SDK、刪除伺服器上下文、清空本地上下文，回 `{unlocked:false}`；即使刪除失敗仍**必須**完成本地清理。 |

### 9.5 建置與資產

| 需求 | 內容 |
| --- | --- |
| `REQ-FE-060` | 前端建置**必須**使用 Node 22 與 `esbuild` 0.28.2，將聊天 SDK 打包為單一 ESM `console/static/vendor/chat-sdk.mjs`，目標 `es2022`、壓縮、瀏覽器平台。 |
| `REQ-FE-061` | 建置**必須**複製 `chat_xdk_wasm_bg.wasm`、`juicebox-sdk_bg.wasm`，並產生 `NOTICE.txt` 保留上游 MIT 授權與第三方聲明。 |
| `REQ-FE-062` | 依賴**必須**以 lockfile 固定；`chat-xdk` 使用本地打包檔 `chat-xdk-b4ce88b.tgz`，版本標示為 `0.5.2-web.b4ce88b`，**不得**改為未經驗證的公開版本。 |
| `REQ-FE-063` | 靜態資產**必須**由 `/static` 提供；`index.html` 只引用 `/static/app.css`、`/static/workspace.css`、`/static/app.js` 與 `/static/mark.svg`。 |
| `REQ-FE-064` | 前端**不得**依賴 CDN、遠端字型或執行時下載程式碼；所有指令碼與樣式**必須**同源。 |
| `REQ-FE-065` | 前端**必須**在 CSP `script-src 'self' 'wasm-unsafe-eval'`、`worker-src 'self'`、`connect-src 'self'` 下可運作；不得要求放寬 `unsafe-inline` 或 `unsafe-eval`。 |

## 10. 行為規格：Go 閘道與部署

### 10.1 Go TLS 閘道執行時契約

| 項目 | 規格 |
| --- | --- |
| 監聽 | `:8090`，僅內部網路 |
| `POST /v1/request` | 唯一代理 RPC；JSON 請求與 JSON 回應 |
| `GET /healthz` | 回 `{"status":"ok","profile":"chrome_150","proxy_required":true}` |
| 認證 | `Authorization: Bearer <token>`，常數時間比較 |
| 併發 | 64 個同時請求；滿載回 `503 gateway busy` |
| 請求上限 | 16 MiB；未知 JSON 欄位拒絕 |
| 回應上限 | 16 MiB；超過回 `502` |
| TLS 指紋 | Chrome 150 profile、隨機 TLS 擴充順序 |
| 逾時 | 標頭 5 秒、讀 30 秒、寫 130 秒、閒置 60 秒、上游 120 秒 |
| 標頭上限 | 16 KiB |

| 需求 | 內容 |
| --- | --- |
| `REQ-DEP-001` | 閘道**必須**只接受 `POST /v1/request`；其他方法回 `405`。 |
| `REQ-DEP-002` | 請求 JSON **必須**恰含 `method`、`url`、`headers`、`body`、`proxy`、`account`；`method` 僅 `GET`／`POST`；`body` 為 Base64；`account` 非空且 ≤128。 |
| `REQ-DEP-003` | 目標 URL **必須**為 HTTPS、無帳密、無片段、埠為空或 443，主機為 `x.com`、`twitter.com`、`twimg.com` 或其子網域；否則回 `400`。 |
| `REQ-DEP-004` | 代理**必須**為 `http`、`https`、`socks5`、`socks5h`，具主機與埠，無查詢／片段，路徑為空或 `/`；否則回 `400`。 |
| `REQ-DEP-005` | 連線池**必須**以 `sha256(account + NUL + proxy)` 為鍵；同一鍵重用；閒置超過 15 分鐘或超過 256 個時**必須**關閉最舊連線。 |
| `REQ-DEP-006` | `socks5h://` **必須**在建立客戶端前正規化為 `socks5://`；代理**不得**回退直連。 |
| `REQ-DEP-007` | 轉送標頭**必須**過濾 `host`、`connection`、`content-length`、`transfer-encoding`、`proxy-authorization` 及含 CR/LF 的鍵或值。 |
| `REQ-DEP-008` | 上游請求**必須**強制 `Accept-Encoding: gzip`，並以固定標頭順序送出；回應為 gzip 時**必須**解壓後再回傳。 |
| `REQ-DEP-009` | 回應**必須**保留 `Set-Cookie` 為陣列，不得合併；回應標頭**必須**移除 `Content-Length`。 |
| `REQ-DEP-010` | 閘道**不得**跟隨重定向；重定向由呼叫端以安全規則處理，寫入請求**不得**重放。 |
| `REQ-DEP-011` | 認證 token **必須**取自 `X_GATEWAY_TOKEN`，否則 `X_GATEWAY_TOKEN_FILE`（預設 `/shared/gateway.token`）；長度**必須** ≥32。 |
| `REQ-DEP-012` | token 檔不存在時**必須**產生 32 位元組隨機值、以小寫十六進位寫入、權限 `0600`、父目錄 `0700`；**不得**覆寫既有檔。 |
| `REQ-DEP-013` | 閘道**不得**記錄 URL、標頭、Cookie、代理帳密或 body；啟動日誌只允許固定描述。 |
| `REQ-DEP-014` | `healthcheck` 子命令**必須**以 3 秒逾時請求 `http://127.0.0.1:8090/healthz`，非 200 即退出碼 1。 |

### 10.2 容器拓撲

| 服務 | 映像／建置 | 網路 | 持久化 | 關鍵限制 |
| --- | --- | --- | --- | --- |
| `init-secrets` | `xapis-web:local`，執行 `python -m console.bootstrap` | `none` | `mysql-user-secret`、`mysql-root-secret` | 一次性、唯讀、`cap_drop: ALL` |
| `mysql` | `mysql:8.4` | `internal` | `mysql-data`、兩個密碼 volume | utf8mb4、1 GiB、健康檢查 |
| `gateway` | `./gateway` | `internal`、`outbound` | `gateway-secret` | 唯讀、512 MiB、UID 非 root |
| `web` | `.` | `internal`、`outbound` | `console-data`、`gateway-secret`、`mysql-user-secret` | 唯讀、1 GiB、`tmpfs /tmp` 256 MiB |

| 需求 | 內容 |
| --- | --- |
| `REQ-DEP-020` | `init-secrets` **必須**在 MySQL 啟動前成功完成，產生資料庫使用者與 root 密碼檔；**不得**有網路存取。 |
| `REQ-DEP-021` | MySQL **必須**使用 `MYSQL_PASSWORD_FILE` 與 `MYSQL_ROOT_PASSWORD_FILE`，**不得**把密碼寫入環境變數或映像。 |
| `REQ-DEP-022` | `internal` 網路**必須**設為 `internal: true`；只有 `gateway` 與 `web` 可同時加入 `outbound`。 |
| `REQ-DEP-023` | MySQL **不得**發布宿主機埠；正式部署只發布 `web` 的 `${CONSOLE_BIND:-127.0.0.1}:${CONSOLE_PORT:-8000}:8000`。 |
| `REQ-DEP-024` | `web` 與 `gateway` **必須** `read_only: true`、`cap_drop: [ALL]`、`security_opt: [no-new-privileges:true]`、`init: true`。 |
| `REQ-DEP-025` | `web` 容器**必須**以 UID／GID `10001` 執行；映像內不得有可寫入的應用目錄，除 `/app/data` 與 `/tmp`。 |
| `REQ-DEP-026` | 所有服務**必須**設定日誌輪替（`max-size: 10m`、`max-file: 3`）與記憶體上限。 |
| `REQ-DEP-027` | `web` **必須**等待 `gateway` 與 `mysql` 健康後才啟動；`gateway` 健康檢查**必須**使用內建 `healthcheck`。 |
| `REQ-DEP-028` | 本機別名 `compose.local.yaml` **必須**只額外發布 `127.0.0.1:8765:8000`，不得改變其他服務。 |
| `REQ-DEP-029` | 測試用 `compose.test.yaml` **必須**使用獨立資料庫名稱與一次性密碼，僅發布 `127.0.0.1:3307:3306`，且**不得**用於正式環境。 |

### 10.3 映像建置契約

| 需求 | 內容 |
| --- | --- |
| `REQ-DEP-030` | 建置**必須**為多階段：Node 22 `bookworm-slim` 產生前端 bundle，Python 3.12 `slim-bookworm` 產生執行映像。 |
| `REQ-DEP-031` | Node 階段**必須**以 `npm ci --ignore-scripts` 安裝，建置後只複製 `console/static/vendor` 到執行映像。 |
| `REQ-DEP-032` | Python 階段**必須**以 `requirements.lock` 安裝，不得使用未鎖定版本；不得在映像中保留建置快取。 |
| `REQ-DEP-033` | 執行映像**必須**建立 `xapis` 群組與使用者（10001），預建 `/app/data`、`/shared`、`/secrets/mysql`、`/secrets/root` 並設為該使用者所有。 |
| `REQ-DEP-034` | 啟動命令**必須**為 `uvicorn console.app:create_app --factory --host 0.0.0.0 --port 8000 --workers 1 --no-access-log --no-proxy-headers`。 |
| `REQ-DEP-035` | 映像健康檢查**必須**每 30 秒請求 `/healthz`，逾時 5 秒，啟動寬限 15 秒，連續 3 次失敗視為不健康。 |
| `REQ-DEP-036` | 映像**不得**包含 `.env`、`data/`、`node_modules`、測試資料、備份檔或任何真實憑證。 |

### 10.4 管理、備份與還原

| 命令 | 行為 |
| --- | --- |
| `python -m console.manage setup-token` | 僅在尚無使用者時印出初始化令牌；已初始化則退出碼 1 |
| `python -m console.manage reset-password --username <name>` | 互動輸入兩次新密碼；長度 12–128；更新雜湊、啟用使用者、刪除全部會話、稽核 `auth.local-recovery` |
| `python -m console.manage backup --output <dir>` | 匯出一致性 MySQL 快照、加密金鑰與初始化令牌 |
| `python -m console.manage restore --source <dir>` | 僅能還原到空資料庫；還原後需重啟 Web |
| `python -m console.manage migrate-sqlite --source <file>` | 唯讀讀取舊 SQLite，匯入空 MySQL；來源檔保留 |

| 需求 | 內容 |
| --- | --- |
| `REQ-DEP-040` | 備份**必須**在 `REPEATABLE READ` 與 `START TRANSACTION WITH CONSISTENT SNAPSHOT` 下讀取所有業務表，輸出 `mysql-backup.json`（`version=1`、`created`、`tables`）。 |
| `REQ-DEP-041` | 備份表集合**必須**為 `users`、`sessions`、`attempts`、`account_policies`、`accounts`、`account_logins`、`request_logs`、`ai_settings`、`sources`、`entries`、`rules`、`jobs`、`publication_ledger`、`rule_entries`、`media`、`keyword_groups`、`account_maintenance`、`maintenance_logs`、`audit`。 |
| `REQ-DEP-042` | 備份**必須**移除生成欄 `entries.url_hash`，並複製 `encryption.key` 與 `setup.token`；所有檔案權限 `0600`、目錄 `0700`。 |
| `REQ-DEP-043` | 還原**必須**拒絕非空資料庫（已有任何 `users`）；版本非 1 拒絕；匯入前**必須**以備份金鑰成功解密所有帳號密文。 |
| `REQ-DEP-044` | 還原成功**必須**把備份金鑰與初始化令牌寫回資料目錄，並以新金鑰取代執行中 cipher；**不得**在還原期間接受寫入。 |
| `REQ-DEP-045` | SQLite 遷移**必須**以唯讀 URI 開啟、只匯入兩邊都存在的表、先以現行金鑰解密帳號密文、匯入後保留來源檔。 |
| `REQ-DEP-046` | 遷移或還原後，`connected` 但無 `policy_id` 的帳號**必須**建立新保護期，並建立 `next_run = now + 30` 的養護列。 |
| `REQ-DEP-047` | 正式對外**必須**由反向代理終止 TLS，設定 `CONSOLE_PUBLIC_URL` 與瀏覽器位址一致；啟用 HTTPS 時**必須**設 `CONSOLE_COOKIE_SECURE=1`。 |
| `REQ-DEP-048` | 備份目錄含加密金鑰，**必須**視為與正式資料同等敏感；不得提交版本控制或放入映像。 |

## 11. 橫切不變量

以下不變量橫跨所有子系統；任何一條被違反即視為整體不符合本規格。

| 需求 | 內容 |
| --- | --- |
| `REQ-INV-01` | 憑證最小暴露：Cookie、代理帳密、AI Key、聊天 PIN、realm token、私鑰、X 密碼**不得**出現於回應、日誌、稽核、診斷、前端狀態或 URL；所有對外輸出**必須**經脫敏函式。 |
| `REQ-INV-02` | 失敗關閉：閘道啟用或 `X_REQUIRE_PROXY=1` 時，代理缺失、閘道不可用或認證失敗**必須**直接失敗；**不得**存在任何直連回退路徑。 |
| `REQ-INV-03` | 冪等：任務、文章發布、聊天派送**必須**以資料庫唯一鍵綁定請求 ID；重複請求**必須**回既有結果或 `409`，**不得**對上游重送。 |
| `REQ-INV-04` | 不確定性即終態：`uncertain` 狀態**不得**被系統自動重試、自動標記失敗或自動回復；僅能由人員核對後以新請求處理。 |
| `REQ-INV-05` | 原子性：配額預留／確認／退回、佇列領取、初始化、限流計數**必須**在交易與條件更新下完成；**不得**以「先讀後寫無鎖」實作。 |
| `REQ-INV-06` | 白名單優先：操作、產品、動作、媒體副檔名、聊天 realm 主機、中繼 URL、AI 私網主機**必須**以明確白名單判定；黑名單僅作為補充。 |
| `REQ-INV-07` | 編碼一致性：對外 JSON **必須**為 UTF-8、緊湊序列化、不轉義非 ASCII；文章 offset／length **必須**以 Unicode 碼點計。 |
| `REQ-INV-08` | 時間語義：儲存一律 Unix epoch 秒；配額日界以 `Asia/Shanghai` 自然日計算；對人顯示**必須**標注北京時間；註冊表中繼資料使用 `%Y-%m-%dT%H:%M:%S%z`。 |
| `REQ-INV-09` | 指紋單一真源：UA／Client Hints／語言／時區／螢幕**必須**取自同一指紋檔案；應用層版本與傳輸層 TLS 模板版本可以不同，但**必須**各自自洽。 |
| `REQ-INV-10` | 一次性令牌：XCTID 每請求現算；Castle token 每登入步驟現算；`session_token` 於登入流內推進；**不得**重用前步值。 |
| `REQ-INV-11` | 隨機性不影響契約：XCTID 混入隨機位元組、養護延遲隨機化、延遲區間抖動**不得**改變成功判定；**必須**提供固定隨機源入口以供對拍。 |
| `REQ-INV-12` | 平台差異隔離：Node 執行器、瀏覽器 Worker、ctypes 原生庫**必須**是可替換的隔離邊界；隔離層失效**必須**明確報錯，**不得**靜默降級。 |
| `REQ-INV-13` | 可觀察性：診斷與日誌**必須**記錄方法、路徑、狀態、耗時與脫敏回應；HTTP 200 內的 GraphQL 錯誤**必須**可見；保留期 30 天。 |
| `REQ-INV-14` | 資源上限：請求體（2 MiB／55 MiB）、回應（16 MiB）、清單（200／100／500）、連線池（256）、快取（TTL 300／900 秒、上限 128／1000）**必須**存在且不可繞過。 |
| `REQ-INV-15` | 保護期屬物理身分：以 X 數字 ID 為鍵，跨工作台使用者共用；重新登入、換代理、刪除重加、刪除紀錄**不得**重設起點或額度。 |
| `REQ-INV-16` | 匿名請求與帳號會話隔離：公開簽章素材抓取**不得**繼承帳號 Cookie 回寫回呼；會話標頭**必須**於素材初始化後以最終 Cookie 狀態重組。 |

## 12. 錯誤分類與重試語義

### 12.1 例外分類

| 例外 | 觸發 | 攜帶 | 語義 |
| --- | --- | --- | --- |
| `GraphQLError` | HTTP 200 內含 `errors`（讀取：且無 `data`） | `operation_name`、`errors`、`res_json`、`code`（首錯誤碼） | 上游業務失敗；**不得**視為成功 |
| `LoginChallenge` | 登入流遇需人工輸入的 action／subtask | `subtask_id`、`flow_token`、`payload` | 交回人工；**不得**自動重試 |
| `LoginFailed` | 登入流不可復原失敗 | 訊息 | 終止登入 |
| `LoginRateLimited` | HTTP 429 或限流文案命中 | `retry-after`／`x-rate-limit-reset` 提示 | 退避後重試，非欄位校驗失敗 |
| `LoginAttestationBlocked` | 回應含 `AttestationDenied` | — | 硬體設備證明牆；改用 Cookie 匯入 |
| `GatewayError` | 閘道 RPC 失敗 | `code` | 見 12.2 |
| `PolicyWait` | 保護期或每日額度約束 | `retry_at` | 可重排等待，非失敗 |
| `TimeoutError` | 媒體轉碼逾時（預設 180 秒） | media id | 上游處理未完成 |
| `KeyError` | 回應缺推文／文章 id | 訊息含原始回應摘要 | 結構不符 |
| `ValueError` | 輸入契約違反（Cookie 缺 ct0、profile 形狀、reply_mode 等） | 訊息 | 呼叫端修正輸入 |

### 12.2 閘道錯誤碼映射

| 傳輸狀態 | `code` | 訊息 |
| --- | --- | --- |
| 401 | `gateway_auth` | 出網閘道認證失敗，請檢查共享密鑰配置 |
| 400 | `proxy_configuration` | 代理或出網參數無效，請檢查代理地址和協議 |
| 502 | `proxy_connection` | 代理無法完成與 X 的連線；請確認協議、認證及出口可用性 |
| 503 | `gateway_busy` | 出網閘道繁忙，請稍後重試 |
| 504 | `proxy_timeout` | 透過代理連線 X 超時；請檢查代理協議和出口線路 |
| 其他 | `gateway_error` | 出網閘道請求失敗，請檢查服務狀態 |
| `requests.Timeout` | `proxy_timeout` | 透過代理連線 X 超時；請檢查協議、認證及出口線路 |
| `requests.RequestException` | `gateway_unavailable` | 無法連線出網閘道，請檢查閘道服務 |
| 期限已過 | `login_timeout` | 登入驗證超時，請檢查代理出口後重試，或匯入已登入的 Cookie |

### 12.3 GraphQL 錯誤碼語義

| code | 語義 | 系統處置 |
| --- | --- | --- |
| `32` | 會話失效 | 帳號標記 `expired`；停用養護與規則；`queued` 任務轉 `draft`；CLI 視為憑證無效 |
| `89` | 憑證無效 | 同 `32` |
| `215`、`326` | 驗證失敗／帳號鎖定 | CLI 登入有效性檢查視為憑證失效；控制台安全化提示 |
| `88`、`344` | 限流 | 養護退避 6 小時；其他路徑退避不重試 |
| `226` | 疑似自動化 | 退避；不重試 |
| `353` | CSRF Cookie 與標頭不匹配 | 會話標頭以最終 Cookie 狀態重組；控制台提示檢查會話 |
| `179` | 禁止操作 | 安全化提示；養護記為 `warning` |
| `187` | 重複內容 | 任務 `failed`；提示重複 |
| `385` | 需要 Premium | 提示啟用長貼文；不得靜默升級 |
| `239` | guest token 無效 | 記入診斷；重建會話 |

### 12.4 控制台 HTTP 狀態語義

| 狀態碼 | 語義 | 代表情境 |
| --- | --- | --- |
| `400` | 格式或白名單違規 | 代理 scheme 非法、未知探索操作、媒體過期、正文超重、貼文 ID 無效 |
| `401` | 未認證 | 無會話或會話逾期 |
| `403` | 授權／CSRF／來源失敗 | CSRF 不符、Origin 非法、缺 `X-Requested-With`、非管理員、最後一位管理員保護（部分為 409） |
| `404` | 資源不存在（隱藏他使用者資源） | 跨使用者存取一律 404 |
| `409` | 狀態衝突 | 重複初始化、重複發送、既有會話金鑰、登入中／發布中、身分綁定不符、來源仍被引用 |
| `413` | 請求過大 | 超過 2 MiB／55 MiB 或 55 MB Content-Length |
| `422` | 欄位驗證失敗 | 缺代理、聊天送出含明文、中繼多餘欄位、延遲 min>max |
| `423` | 政策阻止（只讀期） | 養護期內寫入、媒體上傳、聊天發送、金鑰設定；附 `retry_at` |
| `429` | 限流或每日額度用盡 | 登入 20 次限流；前 7 天每日 2 篇額度不足；附 `retry_at` |
| `502` | 上游失敗或結果不明 | 閘道逾時、未知收件箱結構、聊天結果不明 |

### 12.5 重試與退避語義

| 情境 | 語義 |
| --- | --- |
| `uncertain` 任務／帳本／聊天派送 | **不得**自動重試；僅人工核對 |
| GraphQL 明確拒絕 | 終態 `failed`／`rejected`；退回預留額度 |
| 保護期或額度等待 | 任務保持 `queued`，`scheduled` 更新為 `retry_at`；不計失敗 |
| 養護一般錯誤 | 退避 `max(900, interval_minutes×60)` 秒 |
| 養護限流（88／226／344 或 HTTP 429） | 退避 6 小時 |
| 養護會話失效（32／89 或 HTTP 401） | 帳號 `expired`，停用養護與規則，不重試 |
| 登入限流（`LoginRateLimited`） | 人工退避；控制台不得自動重登 |
| XCTID／Castle 素材漂移 | 經 `refresh-graphql`／`refresh-castle` 重建後繼續；生成前自檢 |
| guest token | 快取 3 小時內重用；逾期重新兌換 |
| 瞬時傳輸錯誤 | **不得**清除仍有效的 Cookie（僅 401／403 或明確 auth 錯誤碼可清除） |

## 13. 設定矩陣

### 13.1 SDK 與 CLI 環境變數

| 變數 | 預設 | 語義 |
| --- | --- | --- |
| `X_COOKIES` | — | 會話 Cookie 串（必需鍵 `auth_token`、`ct0`）；登入成功後回寫 |
| `X_USERNAME`／`X_PASSWORD` | — | 帳密登入憑證 |
| `X_PROXY` | — | 全域代理；未設時探測系統代理 |
| `X_SEARCH_QUERY`／`X_SEARCH_PRODUCT` | `美国`／`Top` | quickstart 查詢覆寫 |
| `CASTLE_PURE_RL_FILE` | — | 純算 Castle profile 路徑 |
| `CASTLE_PURE_ONLY` | — | `1|true|yes` 時無 profile 即失敗，拒絕 Node 回退 |
| `CASTLE_PROFILE_FILE` | — | Node 回放路徑的瀏覽器採集向量檔 |
| `CASTLE_CHROMIUM_ZLIB` | `static/castle/chromium_zlib.dll` | 原生 deflate 後端覆寫 |
| `X_LOGIN_FORCE` | — | `1|true|yes` 等價 `--force` |
| `X_LOGIN_BROWSER_COOKIES` | — | `1|true|yes` 時把 `.env` Cookie 注入登入流（對拍用） |
| `X_LOGIN_VISIBLE_COOKIES`／`X_LOGIN_VISIBLE_COOKIES_FILE` | — | 覆寫瀏覽器可見 Cookie（排除 `auth_token`／`twid`／`ct0`） |
| `X_GATEWAY_URL` | — | 非空即啟用 Go 閘道 RPC |
| `X_GATEWAY_TOKEN`／`X_GATEWAY_TOKEN_FILE` | 檔案 `data/gateway.token` | 閘道共享密鑰；兩者皆空必須失敗 |
| `X_REQUIRE_PROXY` | — | `1` 時缺 https 代理即拒絕請求 |
| `XCTID_PROFILE_FILE` | `static/transaction_l1.json` | 本地 XCTID 素材覆寫 |
| `X_SHELL_ATTEMPTS` | `12`（SDK）；控制台設 `2` | app shell A/B 重試上限（夾 1–12） |
| `NODE_BIN` | `node` | Node 可執行檔路徑 |

### 13.2 控制台環境變數

| 變數 | 預設 | 語義 |
| --- | --- | --- |
| `CONSOLE_BIND`／`CONSOLE_PORT` | `127.0.0.1`／`8000` | 宿主發布位址 |
| `CONSOLE_PUBLIC_URL` | — | 預期 Origin；HTTPS 部署必填，與瀏覽器位址一致、無尾端 `/` |
| `CONSOLE_COOKIE_SECURE` | `0` | `1` 時會話 Cookie 加 `Secure` |
| `CONSOLE_PRIVATE_AI_HOSTS` | 空 | 允許私網位址的主機名（僅 AI，逗號分隔） |
| `CONSOLE_DATA_DIR` | `data`（容器 `/app/data`） | 存放 `encryption.key` 與 `setup.token` |
| `CONSOLE_SETUP_TOKEN` | — | 覆寫初始化令牌檔 |
| `CONSOLE_DB_HOST`／`CONSOLE_DB_PORT`／`CONSOLE_DB_USER`／`CONSOLE_DB_NAME` | `mysql`／`3306`／`xapis`／`xapis` | MySQL 連線；資料庫名須匹配 `[A-Za-z0-9_]+` |
| `CONSOLE_DB_PASSWORD`／`CONSOLE_DB_PASSWORD_FILE` | 檔案 `/secrets/mysql/password` | 資料庫密碼二擇一 |
| `XAPIS_TEST_DB_HOST`／`XAPIS_TEST_DB_PORT` | `127.0.0.1`／`3307` | 測試資料庫 |

### 13.3 檔案型設定與執行期資料

| 檔案 | 角色 | 生成者 |
| --- | --- | --- |
| `.env`（範本 `.env.example`） | SDK 憑證與控制台開關 | 使用者；登入流程回寫 `X_COOKIES` |
| `requirements.txt`／`requirements.lock` | 依賴宣告與鎖定 | 維護者；鎖檔必須提交 |
| `data/encryption.key` | Fernet 金鑰（0600） | 首次 `Store()` 初始化 |
| `data/setup.token` | 初始化令牌（0600） | 首次 `Store()` 初始化 |
| `/shared/gateway.token` | 閘道共享密鑰（0600） | 閘道首次啟動自生成 |
| `/secrets/mysql/password`、`/secrets/root/password` | 資料庫密碼（0600，`token_urlsafe(48)`） | `console.bootstrap` |
| `static/graphql.json` | GraphQL 操作註冊表（執行期資料） | `refresh-graphql` |
| `static/transaction_l1.json` | XCTID 本地素材基線 | 隨倉庫提供；可由 profile 覆寫 |
| `static/castle/castle_meta.json` | Castle 基線（公鑰、sha、module id、前綴） | `refresh-castle` |
| `console/static/vendor/*` | 瀏覽器聊天加密 SDK 產物 | `frontend` 建置（Docker 自動） |

### 13.4 解析優先序

1. 代理：`X_PROXY` → 標準環境變數（`HTTPS_PROXY`/`ALL_PROXY`）→ Windows 註冊表（`ProxyEnable` 開啟時）→ 本機常見埠探測（7897、7890、10809、10808、1080、8080）→ 直連。
2. 閘道密鑰：`X_GATEWAY_TOKEN` → `X_GATEWAY_TOKEN_FILE` → 失敗。
3. 初始化令牌：`CONSOLE_SETUP_TOKEN` → `data/setup.token`。
4. Castle 公鑰：呼叫端參數 → `castle_meta.json` → runner 內建。
5. 純算 profile：`--pure-rl` → `CASTLE_PURE_RL_FILE`；提供 profile 一律 strict。
6. XCTID 素材：呼叫端路徑 → `XCTID_PROFILE_FILE` → `static/transaction_l1.json`；網路素材失敗時回退本地快取。

## 14. 驗收準則

### 14.1 算法驗收

| 等級 | 準則 |
| --- | --- |
| L1 | 以固定隨機源與固定時間戳，XCTID 與 Castle token 的輸出**必須**與瀏覽器擷取基線逐位元組一致 |
| L2 | 以真實隨機源產生的令牌送出後**必須**被 X 服務端接受（登入成功、請求不 404／不 403） |
| 登入成功判定 | 會話 Cookie 含 `auth_token`，且 Viewer 回傳身分與綁定帳號一致 |
| 註冊表驗收 | 重建後操作數 > 0、被引用開關全數收錄、失敗分片清單為空或已記錄 |

### 14.2 測試套件驗收

| 指令 | 環境 | 通過條件 |
| --- | --- | --- |
| `pytest -q` | `compose.test.yaml`（`xapis_test`，埠 3307） | 全數通過；不觸及真實 X／AI |
| `go test -race ./...`（`gateway/`） | 本地 Go 1.27 | 全數通過 |
| `node --test tests/test_chat_crypto.mjs` | Node 22，離線（網路 stub 為拋錯） | 加解密往返、竄改偵測、游標精度全數通過 |

### 14.3 不可違反驗收（任一成立即整體驗收失敗）

1. 任何回應、日誌、稽核或前端狀態出現明文憑證。
2. 閘道啟用或要求代理時存在直連流量。
3. `uncertain` 被自動重試或自動改判。
4. 同一 `request_id`／`message_id` 造成第二次上游寫入。
5. 瞬時傳輸錯誤導致有效 Cookie 被清除。
6. 只讀期內發生任何 X 寫入（含媒體上傳、聊天發送、金鑰設定）。
7. 跨使用者資源以 403 或內容洩漏存在性。

### 14.4 行為抽檢清單

| 情境 | 預期 |
| --- | --- |
| 141 個中文字元普通發文 | `400`（權重 > 280）；同文 `long=true` → `200` |
| 第 5 天第三次發布 | 額度原子拒絕：2 成功、1 `PolicyWait`，任務排至下一北京日 |
| 3 段串文於第 5 天 | 任務排至第 7 天界；不部分發送 |
| 3 併發同一 `request_id` | 一個 `id`，第二次回應 `duplicate=true` |
| 閘道停止後發請求 | 例外向上傳播，直連路徑零呼叫 |
| 上游 302 至 `abs.twimg.com` | 跟隨請求不含 `authorization`、`x-csrf-token`、`auth_token` |
| POST 收到 302 | 不重放（上游恰 1 次呼叫） |
| 匿名 `/i/flow/login` 清除 `ct0` | 帳號 `ct0` 與 CSRF 標頭保持一致 |
| Viewer 回 `{}` | 養護記 `error`，不標記身分驗證成功 |
| GraphQL code 32 於發布中 | 帳號 `expired`、規則停用、`queued` 轉 `draft`、暫停任務入 `draft` |
| 登入 20 次失敗後第 21 次 | `429` |
| 兩個併發初始化 | 恰一個 200、一個 409、一列使用者 |
| 排程中任務引用 25 小時前上傳的媒體 | `400`（過期） |
| 養護游標連續相同 | `stop_reason=repeated_cursor`，狀態重置 |
| 程序於 `sending` 中斷後重啟 | 任務 `uncertain`、帳本 `uncertain`、執行中日誌 `interrupted` |
| 跨宿主聊天 realm（查詢附加、私網、查 `api.x.com/2/tweets`） | 全部 `400` |

## 15. 符合性與驗證矩陣

### 15.1 需求域 → 證據映射

| 規格章節 | 主要驗證證據 | 等級 |
| --- | --- | --- |
| 7.1 傳輸與閘道客戶端 | `tests/test_gateway.py`、`tests/test_network.py` | E1 |
| 7.2 鑑權狀態 | `tests/test_cookies.py`、`tests/test_accounts.py` | E1 |
| 7.3 標頭族 | `builder/header.py` 行為；測試間接覆蓋（CSRF 一致性） | E2 |
| 7.4 註冊表 | 單一實作路徑 + `static/graphql.json` 結構 | E2 |
| 7.5 XCTID | 單一實作路徑；對拍入口（固定時間／隨機位元組） | E2 |
| 7.6 Castle | 單一實作路徑 + `castle_meta.json` 基線 + 自檢邏輯 | E2 |
| 7.7 登入 | `tests/test_login_diagnostics.py`、`docs/LOGIN_FLOW_ANALYSIS.md` | E1 |
| 7.8／7.9 讀寫 | `tests/test_api_capabilities.py`、`tests/test_post_details.py`、`tests/test_accounts.py` | E1 |
| 7.10 媒體 | `tests/test_gateway.py`、`tests/test_api_capabilities.py`、`tests/test_jobs.py` | E1 |
| 7.11 文章 | `tests/test_jobs.py` | E1 |
| 7.12 X Chat | `tests/test_chat.py`、`tests/test_chat_crypto.mjs` | E1 |
| 7.13 資料處理 | `tests/test_jobs.py`（計重）、`tests/test_accounts.py`（時間線解析、去重） | E1 |
| 7.14／7.15 CLI | `main.py`／`quickstart.py` 行為與文件 | E2 |
| 8.3／8.4 控制台安全 | `tests/test_auth.py` | E1 |
| 8.5 帳號與登入 | `tests/test_accounts.py`、`tests/test_login_diagnostics.py` | E1 |
| 8.6 保護期與配額 | `tests/test_policy.py` | E1 |
| 8.7 時間線與媒體端點 | `tests/test_api_capabilities.py`、`tests/test_post_details.py` | E1 |
| 8.8 聊天端點 | `tests/test_chat.py` | E1 |
| 8.9 AI 與來源 | `tests/test_content.py`、`tests/test_network.py` | E1 |
| 8.10 任務佇列 | `tests/test_jobs.py` | E1 |
| 8.11 規則與養護 | `tests/test_maintenance.py`、`tests/test_content.py` | E1 |
| 8.12–8.14 狀態機與不變量 | `tests/test_mysql_backup.py`、上述全部 | E1 |
| 9 前端 | `tests/navigation.test.mjs`、`tests/browser_fixture.py` | E1／E2 |
| 10 閘道與部署 | `gateway/`（含 `main_test.go`）、`compose*.yaml`、`tests/test_gateway.py` | E1 |

### 15.2 文件與測試交叉一致的關鍵數值

| 數值 | 文件出處 | 測試證據 |
| --- | --- | --- |
| 預設 4 天只讀養護 | `docs/CONSOLE.md` | `tests/test_policy.py`（`warming` 邊界） |
| 前 7 天每日最多 2 篇 | `docs/CONSOLE.md` | `tests/test_policy.py`（原子並發、日界重置、第 7 天解除） |
| 媒體排程窗 23 小時、單檔 50 MB | `docs/CONSOLE.md` | `tests/test_api_capabilities.py`、`tests/test_jobs.py` |
| 養護頁間隨機 30–90 秒 | `docs/CONSOLE.md` | `tests/test_maintenance.py`（區間內） |
| 養護週期 ±20% 抖動 | `docs/CONSOLE.md` | `tests/test_maintenance.py`（≥48 分） |
| 規則間隔最小 15 分鐘 | `docs/CONSOLE.md` | `tests/test_content.py` + 模型驗證 |
| 每次執行最多 3 條新內容、滾動 24 小時上限 | `docs/CONSOLE.md` | `tests/test_content.py` |
| 登入限流（IP 與使用者名各 20 次／900 秒） | — | `tests/test_auth.py` |
| 日誌保留 30 天 | `docs/CONSOLE.md` | `tests/test_maintenance.py`、`tests/test_login_diagnostics.py` |
| 聊天收件箱刷新 30 秒、解鎖上下文 300 秒 | `docs/CONSOLE.md` | `tests/test_chat.py`；前端輪詢（E2） |
| 首次登入後第 4 天前寫入全禁、讀取允許 | `docs/CONSOLE.md` | `tests/test_policy.py`（`423` vs 讀取 `200`） |

## 16. 已知限制與排除項

1. `static/` 內的線上擷取素材（`graphql.json`、`transaction_l1.json`、`castle/*`）為**可替換執行期資料**，隨 X 前端發版漂移；實作**必須**內建重建路徑，不得視其為不變式。
2. 行動端登入路徑受硬體設備證明（App Attest／Play Integrity）阻擋，**不得**作為預設策略；僅保留對照與 `LoginAttestationBlocked` 分類。
3. 純算 Castle 要求完整 632／749 槽 `Rl` 向量；部分輸入**必須**拒絕。內附 Chromium-zlib 原生後端目前僅支援 Windows。
4. `x.com/i/flow/login` 存在 A/B 路由：僅舊 client-web shell 含 XCTID 素材；抓取**必須**帶重試預算並以 HTML 特徵判定。
5. Castle／XCTID 的服務端校驗屬外部依賴；指紋相容**不保證**通過驗證碼或設備證明。
6. 聊天加密 SDK 為建置期依賴（`console/static/vendor/` 不入版本控制；Docker 建置自動產生）；缺漏時聊天解鎖必須明確失敗。
7. `ui_metrics` 與 Castle Node 回放需要本機 Node（`NODE_BIN`）；純算路徑不依賴 Node。
8. 控制台限流計數以**嘗試次數**計（成功登入亦消耗額度），非僅失敗次數。
9. 會話 CSRF token 於資料庫明文保存（僅 token 本身雜湊）；仰賴 Cookie 屬性與常數時間比較補償，屬已知設計取捨。
10. 「每日上限」存在兩種語義：規則的 `daily_limit` 為**生成**上限（滾動 24 小時）；政策 2 篇／日僅適用首次登入後 7 天內。
11. `kind=detail` 時間線與 `/posts/{id}` 功能重疊（皆走 `TweetDetail`）；前者回扁平形狀，屬相容保留。
12. X 平台行為（風控、限流、GraphQL 變更）為外部依賴；本規格僅約束客戶端可觀察行為。
13. HAR／抓包素材含憑證，**必須**排除於版本控制與映像之外（`.gitignore`／`.dockerignore` 已含規則）。
14. 上游快照未包含 `/LICENSE` 檔（見第 19 章）；衍生作品授權處理**必須**遵循第 17 章登錄。

## 17. 對 OG-Open-Source README 標準的符合性登錄

| 標準條款 | 強度 | 本文件／衍生作品處置 | 狀態 |
| --- | --- | --- | --- |
| 僅以 MIT 作為唯一授權並記錄於 `/LICENSE`（第三方元件除外） | MUST | 上游快照未觀察到 `/LICENSE`；衍生作品**必須**新增 MIT `/LICENSE`，並將第三方元件排除於該要求外（見 §19.2） | 待辦（衍生作品） |
| 署名（Attribution） | SHOULD | 本文件 §19 署名來源專案 XApis 與整理方 OG-Open-Source；任何引用**必須**保留 | 符合 |
| 環境需求逐項給出版本 | SHOULD | Python 3.12（容器）、Node 22、Go 1.27、MySQL 8.4、依賴見 `requirements.lock` 與 §13 | 符合 |
| 以 submodule 掛載 `.github` 於 `/.github/` | MUST | 上游快照未使用 submodule（觀察所得）；衍生作品**必須**依標準加入 | 待辦（衍生作品） |
| 套件管理器安裝第三方庫並提交 lockfile | MUST | `requirements.lock` 與 `frontend/package-lock.json` 皆隨快照提交 | 符合 |
| 無套件管理器提供之庫以 submodule 入 `/vendor/` | MUST | 聊天加密 SDK 以 npm `file:` 依賴（`frontend/vendor/chat-xdk-b4ce88b.tgz`）經建置打包，屬套件管理器路徑；`static/castle/*` 為執行期資料而非程式庫 | 符合（型態差異已登錄） |
| 第三方元件保留原始授權檔 | MUST | 聊天 SDK 建置產生 `NOTICE.txt`（含上游 LICENSE 與第三方聲明） | 符合 |
| 每個主要功能至少一個使用範例 | SHOULD | §7.14 CLI 表與上游 README 快速開始；衍生 README **應**附命令與預期輸出 | 符合 |
| 接受 PR 的倉庫**必須**連結 Code of Conduct | MUST | 衍生作品**必須**連結 `/CODE_OF_CONDUCT.md`（Contributor Covenant 3.0） | 待辦（衍生作品） |
| 開發指引採用免費共識標準；RFC 適用時優先 | MUST | 本文件採用 RFC 2119／8174（關鍵詞）、RFC 3339（時間）；twitter-text v3、SemVer 2.0.0、Keep a Changelog 1.1.0、Conventional Commits 1.0.0 | 符合 |
| Commit 訊息可循 Conventional Commits 1.0.0 | CAN | 建議衍生作品採用 | 建議 |
| 變更可記錄於 `/CHANGELOG.md`（Keep a Changelog 1.1.0） | CAN | 本文件含〈修訂紀錄〉；衍生作品**應**建立 `/CHANGELOG.md` | 建議 |
| 提交前通過專案定義的 linter | SHOULD | 衍生作品**應**定義 linter 並納入 CI | 待辦（衍生作品） |
| 文件以 Markdown 撰寫 | MUST | 本文件為 Markdown | 符合 |
| 檔案以 UTF-8 儲存 | MUST | 本文件 UTF-8（無 BOM） | 符合 |
| 特殊檔名匹配 `^[A-Z_]+\.md$` | MUST | 本檔名 `XAPIS_BEHAVIOR_SPEC.md` 符合；上游 `docs/` 文件非特殊檔，不受此限 | 符合 |
| 版本化：三選一；標籤匹配 `[a-z0-9\.-]+` | MUST | 選定 Semantic Versioning 2.0.0（本文件 1.0.0） | 符合 |
| Issue 範本含環境／重現步驟／期望／實際 | SHOULD | 衍生作品**應**提供範本 | 建議 |

## 18. 需求索引

| 區域 | ID 範圍 | 所在章節 |
| --- | --- | --- |
| `TRN` | 001–006、010–020、030–041 | 7.1 |
| `AUTH` | 001–014 | 7.2 |
| `HDR` | 001–013 | 7.3 |
| `GQL` | 001–007、010–016、020–025、030–038 | 6.1.1、7.4 |
| `GW` | 001–006 | 6.2 |
| `WEB-AI` | 001–005 | 6.3、8.9 |
| `WEB-SRC` | 001–004 | 6.4、8.9 |
| `WEB-DB` | 001–004、010–013 | 6.5、8.2 |
| `XCT` | 001–013 | 7.5 |
| `CST` | 001–004、010–020、030–038、040–047 | 7.6 |
| `LOG` | 001–022、030–037、040–051 | 7.7 |
| `RD` | 001–004、010–014 | 7.8 |
| `WR` | 001–015 | 7.9 |
| `MED` | 001–013 | 7.10 |
| `ART` | 001–018、020–028、030–035 | 7.11 |
| `DM` | 001–012 | 7.12 |
| `DAT` | 001–005、010–019 | 7.13 |
| `CLI` | 001–002、010–013、020–031 | 7.14、7.15 |
| `WEB` | 001–010、020–035、040–052、060–091、100–119、120–145、150–167、170–200、210–235、240–243、250–252、260–275 | 8 |
| `FE` | 001–010、020–028、030–033、040–050、060–065 | 9 |
| `DEP` | 001–014、020–029、030–036、040–048 | 10 |
| `INV` | 01–16 | 11 |

## 19. 來源、授權與致謝

### 19.1 來源聲明

1. 本文件的行為描述提取自 `XApis-master/` 目錄快照（118 個檔案），上游專案為 [XApis](https://github.com/cv-cat/XApis)（cv-cat，2026-11-10 觀察）。
2. 提取方法為閱讀原始碼、文件與測試後的**行為層重述**：本文件不含任何原專案函式體、類別結構或註解的複製、改寫或翻譯；演算法僅以其輸入輸出契約與公開常數描述。
3. 於觀察時點，上游倉庫未附 `/LICENSE` 檔案，GitHub 介面亦未標示授權。衍生作品散布前**必須**先確認上游授權狀態，並依第 17 章補上 MIT `/LICENSE` 與署名。
4. 上游自我定位為「Twitter(X) 推特算法逆向」，並聲明僅供學習、研究與經授權的自動化使用。本規格繼承同一使用邊界。

### 19.2 第三方元件登錄

| 元件 | 角色 | 引入方式 | 授權狀態 |
| --- | --- | --- | --- |
| `curl_cffi` | Chrome TLS/HTTP2 指紋傳輸 | `requirements.lock` | 以其上游授權宣告為準 |
| FastAPI／Starlette／Pydantic／uvicorn | Web 框架與驗證 | `requirements.lock` | 以其上游授權宣告為準 |
| `PyMySQL` | MySQL 驅動 | `requirements.lock` | 以其上游授權宣告為準 |
| `loguru`、`python-dotenv`、`openpyxl` | 日誌、環境、xlsx | `requirements.lock` | 以其上游授權宣告為準 |
| `feedparser`（含 `feedparser-sgmllib`）、`beautifulsoup4`（含 `soupsieve`） | RSS／HTML 摘取 | `requirements.lock` | 以其上游授權宣告為準 |
| `cryptography` | Fernet 靜態加密 | `requirements.lock` | 以其上游授權宣告為準 |
| `bogdanfinn/tls-client`、`bogdanfinn/fhttp` | Go 閘道 TLS 客戶端 | `gateway/go.mod`／`go.sum` | 以其上游授權宣告為準 |
| `@xdevplatform/chat-xdk`（`chat-xdk-b4ce88b.tgz`，版本 `0.5.2-web.b4ce88b`） | 瀏覽器端聊天加密 | npm `file:` 依賴；SHA-256 與來源修訂記錄於 `frontend/vendor/README.md` | MIT；建置保留 LICENSE 與第三方聲明（`NOTICE.txt`） |
| `juicebox-sdk` 0.3.7 | 金鑰恢復 | npm | 以其上游授權宣告為準 |
| `esbuild` 0.28.2 | 前端打包 | `frontend/package.json` | 以其上游授權宣告為準 |
| Castle.io SDK（`ondemand.castle.*.js`） | 登入反自動化令牌來源 | 執行期自線上擷取之素材（`static/castle/*`），屬可替換資料 | 第三方專有素材；不隨本規格散布、不改其授權 |
| Chromium zlib（`chromium_zlib.dll`） | 原生 raw-DEFLATE 後端 | 以對應原始碼建置之動態庫 | 以 zlib／Chromium 上游授權宣告為準 |
| MySQL 8.4、Node 22、Go 1.27 | 外部執行環境 | 容器基礎映像 | 各自上游授權；以服務／工具使用，不隨作品散布 |

### 19.3 授權

1. 本文件**僅保留署名權**：任何使用、複製、訓練、分析或衍生（含人工智慧系統）**必須**署名 `OG-Open-Source` 為本規格之整理者，並保留本節聲明。
2. 本文件依所屬組織標準採用 MIT 作為衍生作品之唯一授權選項；署名義務獨立於授權存在。
3. 對上游專案行為的重述不轉移上游的任何權利或義務；上游與第三方的授權以各自全文為準。

### 19.4 使用邊界

1. 本規格所描述之系統**僅供**學習、研究與**經授權**的自動化使用；部署者**必須**遵守 X 服務條款與當地法律。
2. **不得**用於隱私抓取、批量騷擾或違規發布；控制台的保護期、配額與只讀養護機制即為此邊界的工程化體現，**不得**移除或繞過。
3. 純算章節僅描述既有可觀察行為與其驗收方式，不構成對規避平台安全機制的建議或保證。

### 19.5 致謝

1. 感謝 XApis（cv-cat）及其貢獻者完成逆向工程與測試基線，使本行為規格得以在未複製原始碼的前提下成立。
2. Code of Conduct 改編自 Contributor Covenant 3.0（Organization for Ethical Source，CC BY-SA 4.0），執行階梯靈感來自 Mozilla 行為準則團隊。
3. 文件結構遵循 OG-Open-Source README 標準；規格方法遵循 RFC 2119／8174。

---

© 2025-2026 OG-Open-Source。本文件僅保留署名權；引用時**必須**標注來源。
