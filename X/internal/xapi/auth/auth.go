package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// defaultBearer 為 X 公開 web bearer（REQ-AUTH-002「預設公開 web bearer」）。
// 此值非機密：內嵌於 web 用戶端、每個匿名請求可見，僅作 NewAuth 空
// bearer 時的預設值。
const defaultBearer = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"

// defaultLang 為 lang cookie 缺失時的預設值（REQ-AUTH-006：取指紋檔的
// client_language；v1 指紋值恆為 zh-CN，同 fingerprint.ClientLanguage，
// 並決定 x-twitter-client-language 標頭）。
const defaultLang = "zh-CN"

// CookieStore 是 XAuth 的持久化窄介面：Load 讀回單行 cookie 字串、Save
// 寫回單行 cookie 字串。v1 引擎不主動呼叫 Save——write-through 屬
// Task 9 的 Sender；此處僅宣告接縫（REQ-AUTH-002）。
type CookieStore interface {
	Load(ctx context.Context) (string, error)
	Save(ctx context.Context, cookieLine string) error
}

// authConfig 聚集 NewAuth 的選項（未導出：選項經 AuthOption 調整）。
type authConfig struct {
	proxies map[string]string
	store   CookieStore
}

// AuthOption 調整 authConfig。
type AuthOption func(*authConfig)

// WithProxies 設定代理映射（REQ-AUTH-002）。複製一份映射，呼叫端事後
// 修改不得穿透。
func WithProxies(proxies map[string]string) AuthOption {
	return func(c *authConfig) {
		c.proxies = make(map[string]string, len(proxies))
		for k, v := range proxies {
			c.proxies[k] = v
		}
	}
}

// WithCookieStore 接線 CookieStore（REQ-AUTH-002）。
func WithCookieStore(store CookieStore) AuthOption {
	return func(c *authConfig) { c.store = store }
}

// authSnapshot 是一次發布的不可變鑑權狀態：cookie 映射、排序後的單行
// cookie、ct0 與世代計數。發布後不得修改——更新一律走「複製→修改→
// 原子替換」（REQ-AUTH-014），此即 REQ-HDR-008「CSRFToken 與 CookieHeader
// 內 ct0 同代逐字一致」的結構保證。
type authSnapshot struct {
	cookies map[string]string
	line    string
	ct0     string
	gen     uint64
}

// Auth 是 XAuth：動態憑證的唯一持有者（REQ-AUTH-001）。讀路徑
// （CookieHeader/CSRFToken/IsLoggedIn）僅原子載入單一 snapshot；寫路徑
// （PrepareAuth/UpdateCookies）發布新 snapshot，與讀者可自由並行。
// bearer 與選項（proxies、store）於建構後不變。
type Auth struct {
	bearer string
	cfg    authConfig
	snap   atomic.Pointer[authSnapshot]
	gen    atomic.Uint64
}

// NewAuth 建構 XAuth：bearer 為空時採預設公開 web bearer（REQ-AUTH-002）；
// 初始為空 cookie 映射的 snapshot（gen 0），guest token 與 XCTID 快取留待
// 後續任務（v1 僅保 guest token 方法位）。
func NewAuth(bearer string, opts ...AuthOption) *Auth {
	var cfg authConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if bearer == "" {
		bearer = defaultBearer
	}
	a := &Auth{bearer: bearer, cfg: cfg}
	a.snap.Store(&authSnapshot{cookies: map[string]string{}, line: "", ct0: "", gen: 0})
	return a
}

// PrepareAuth 解析單行 cookie 字串（k=v; k=v，REQ-AUTH-003）：以 ; 分段、
// trim 空白、空段與無 = 段忽略、k=v 以第一個 = 切分（值可含 =，如
// twid=u%3D123）。有 auth_token 而無 ct0 → *xerrors.InvalidInputError
// （ct0 由服務端簽發、不能本地生成，REQ-AUTH-004）；兩者皆無 → 本地產生
// 32 位小寫 hex ct0（crypto/rand，REQ-AUTH-005）。lang 缺失補預設值
// （REQ-AUTH-006）。成功後發布就緒的新 snapshot；失敗則現狀不變。
func (a *Auth) PrepareAuth(cookiesStr string) error {
	cookies := parseCookieLine(cookiesStr)
	if cookies["ct0"] == "" && cookies["auth_token"] != "" {
		return &xerrors.InvalidInputError{
			Msg: "cookie 有 auth_token 但缺 ct0：已登入會話的 ct0 由服務端簽發，不能本地生成，請把瀏覽器 cookie 完整複製過來",
		}
	}
	if cookies["ct0"] == "" && cookies["auth_token"] == "" {
		generated, err := newCt0()
		if err != nil {
			return err
		}
		cookies["ct0"] = generated
	}
	if _, ok := cookies["lang"]; !ok {
		cookies["lang"] = defaultLang
	}
	a.publish(cookies)
	return nil
}

// UpdateCookies 將非空值合併進當前 cookie 映射並發布新 snapshot
// （REQ-AUTH-014）：不刪既有鍵（v1 不處理來源明確清除）、空值鍵跳過；
// 每次呼叫產生新 snapshot 並原子替換。
func (a *Auth) UpdateCookies(m map[string]string) {
	cur := a.snap.Load()
	next := make(map[string]string, len(cur.cookies)+len(m))
	for k, v := range cur.cookies {
		next[k] = v
	}
	for k, v := range m {
		if k == "" || v == "" {
			continue
		}
		next[k] = v
	}
	a.publish(next)
}

// IsLoggedIn 等價於當前 snapshot 的 auth_token 非空（REQ-AUTH-007）。
func (a *Auth) IsLoggedIn() bool {
	return a.snap.Load().cookies["auth_token"] != ""
}

// CookieHeader 回當前 snapshot 的單行 cookie（k=v; k=v，鍵依字母序的
// 決定性輸出，方便測試與除錯；Cookie 是單一標頭，不受 transport 的
// Header-Order 影響）。呼叫端（Sender）將其注入請求——transport 不持
// cookie 狀態（DECISIONS D9）。未 Prepare 前為空字串。
//
// REQ-HDR-008 的同代保證僅及於單次呼叫：回傳行整體來自一次原子載入的
// 同一 snapshot。請求層級的 ct0 ↔ cookie 一致由呼叫端保證（Task 9
// caller rule）：每個請求僅呼叫本方法一次，將回傳行注入 Cookie 標頭，
// 並自「同一行」解析出的 ct0= 值作為 x-csrf-token——不得另呼
// CSRFToken() 配對（兩次獨立原子載入之間若恰逢發布，會取得舊 ct0 配
// 新 cookie 行的跨代組合）。
func (a *Auth) CookieHeader() string {
	return a.snap.Load().line
}

// CSRFToken 回當前 snapshot 的 ct0。與 CookieHeader 內 ct0 的同代逐字
// 一致（REQ-HDR-008）僅於單次呼叫內成立：本方法與 CookieHeader 是兩次
// 獨立原子載入，兩者之間若恰逢發布，會取得跨代組合（舊 ct0 配新
// cookie 行）。請求層級的 x-csrf-token 契約由呼叫端履行——自單次
// CookieHeader() 載入的行內解析 ct0=，勿以本方法與 CookieHeader() 配對
// （參見 CookieHeader 的 caller rule）。
func (a *Auth) CSRFToken() string {
	return a.snap.Load().ct0
}

// Bearer 回建構時的 bearer（空值已於 NewAuth 代換為預設公開 web bearer）。
func (a *Auth) Bearer() string {
	return a.bearer
}

// Store 回建構時經 WithCookieStore 接線的 CookieStore（未接線回 nil）。
// 唯讀存取：store 於建構後不變。XAuth 本身不主動 Save（REQ-AUTH-002），
// 僅交出接線供呼叫端落盤——Task 9 的 Sender 於回應 Set-Cookie 合併後，
// 以此執行同步 write-through（設計 §3.6：在回應處理完成前寫入儲存）。
func (a *Auth) Store() CookieStore {
	return a.cfg.store
}

// GuestToken 是 guest token 的方法位：v1 不實作兌換（REQ-AUTH-009/010
// 延後），恆回空字串與 *xerrors.InvalidInputError，保留介面供 v2 登入流。
func (a *Auth) GuestToken(ctx context.Context) (string, error) {
	return "", &xerrors.InvalidInputError{Msg: "guest token not implemented in v1"}
}

// parseCookieLine 解析單行 cookie 字串為映射（REQ-AUTH-003）：以 ; 分段、
// trim 空白、空段／無 = 段／空鍵段忽略、以第一個 = 切分。空輸入回空映射。
func parseCookieLine(s string) map[string]string {
	cookies := make(map[string]string)
	for _, seg := range strings.Split(s, ";") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		k, v, ok := strings.Cut(seg, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		cookies[k] = strings.TrimSpace(v)
	}
	return cookies
}

// newCt0 以 crypto/rand 產生 32 位小寫 hex 的 ct0（REQ-AUTH-005）。
func newCt0() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: 產生 ct0 失敗: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// publish 以「複製→修改→原子替換」路徑發布新 snapshot：鍵排序組成單行
// cookie、攜帶映射內的 ct0 與新世代計數；發布後 snapshot 不可變。
func (a *Auth) publish(cookies map[string]string) {
	keys := make([]string, 0, len(cookies))
	for k := range cookies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+cookies[k])
	}
	a.snap.Store(&authSnapshot{
		cookies: cookies,
		line:    strings.Join(parts, "; "),
		ct0:     cookies["ct0"],
		gen:     a.gen.Add(1),
	})
}
