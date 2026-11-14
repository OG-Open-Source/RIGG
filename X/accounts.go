// accounts：app 層的帳號資料模型與儲存——多帳號的真源落於系統憑證庫
// （MyGO Secrets，設計 §8.3）。MyGO Secrets 無 List API，故以 x-accounts
// 索引鍵記帳號清單（Order 新增序、Current 活躍者），每帳號一鍵
// x-account-<user_id> 存 Account JSON（身分與憑證綁定，REQ-WEB-069/070：
// 同 UserID 再匯入＝覆蓋同鍵）。憑證材料（CookieLine）僅存系統憑證庫，
// 錯誤字串不含 cookie 值（REQ-INV-01）；索引存在但帳號鍵缺失或 JSON 損毀
// 屬 material 語義——回錯、不靜默跳過、不靜默清空。超過 MaxSecretSize
// 由底層回錯，不靜默截斷。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// accountService 是帳號儲存建構 store 時的 MyGO Secrets service seam：
// 正式路徑為空字串（MyGO 預設 app identifier service）；測試以唯一
// service 名覆寫以隔離、結束還原，絕不打真實預設 service（Task 4 遷移
// 測試複用）。
var accountService = ""

const (
	// indexKey 是系統憑證庫中存放帳號索引的鍵。
	indexKey = "x-accounts"

	// maxAccounts 是帳號數上限（D19）；「新增帳號」入口達上限時呈提示
	// 文案，不解鎖路徑。
	maxAccounts = 5

	// accountIndexVersion 是索引版本，恆 1。
	accountIndexVersion = 1
)

// Account 是一份持久化的憑證與身分（REQ-WEB-069：身分與憑證綁定一致）。
type Account struct {
	UserID     string `json:"user_id"` // Viewer 取得，唯一識別
	ScreenName string `json:"screen_name"`
	Name       string `json:"name"`
	CookieLine string `json:"cookie_line"` // 憑證材料，僅存 secret
}

// accountIndex 是帳號清單真源（MyGO Secrets 無 List API——實測
// core.txt:3736-3790）。
type accountIndex struct {
	Version int      `json:"version"` // 恆 1
	Order   []string `json:"order"`   // user_id 列表（新增序）
	Current string   `json:"current"` // 活躍帳號 user_id（空＝無）
}

// accountKey 是單一帳號在系統憑證庫中的存放鍵（鍵即命名空間）。
func accountKey(userID string) string { return "x-account-" + userID }

// newAccountStore 以 accountService seam 建構指定鍵的 store：正式路徑走
// MyGO 預設 service，測試經 seam 隔離。store 無狀態、即時進出系統憑證庫
// （見 cookies_store.go），故不必長駐，隨用隨建。
func newAccountStore(key string) (*SecretsCookieStore, error) {
	return NewCookieStore(accountService, key)
}

// loadAccounts 讀索引並依 Order 逐鍵讀回帳號。索引不存在回
// ([], accountIndex{}, nil)——首次啟動的正常狀態；索引存在但某帳號鍵缺失
// 或 JSON 無法解析回 material 錯誤（*xerrors.MaterialError），不靜默跳過。
// ctx 僅符介面形狀：系統憑證庫呼叫不受 ctx 控制（同 cookies_store.go）。
func loadAccounts(ctx context.Context) ([]Account, accountIndex, error) {
	idx, err := loadIndex(ctx)
	if err != nil {
		return nil, accountIndex{}, err
	}
	if idx == nil {
		return []Account{}, accountIndex{}, nil
	}
	accounts := make([]Account, 0, len(idx.Order))
	for _, uid := range idx.Order {
		st, err := newAccountStore(accountKey(uid))
		if err != nil {
			return nil, accountIndex{}, err
		}
		raw, err := st.Load(ctx)
		if err != nil {
			return nil, accountIndex{}, fmt.Errorf("accounts: 讀取帳號 %s: %w", uid, err)
		}
		if raw == "" {
			return nil, accountIndex{}, &xerrors.MaterialError{Msg: fmt.Sprintf("帳號 %s 資料無法讀取（系統憑證庫）", uid)}
		}
		var acc Account
		if err := json.Unmarshal([]byte(raw), &acc); err != nil {
			// 損毀 JSON 的解析錯誤可能引述內容片段，不納入錯誤字串
			//（REQ-INV-01）。
			return nil, accountIndex{}, &xerrors.MaterialError{Msg: fmt.Sprintf("帳號 %s 資料無法讀取（系統憑證庫）", uid)}
		}
		accounts = append(accounts, acc)
	}
	return accounts, *idx, nil
}

// loadIndex 讀回帳號索引；索引不存在回 (nil, nil)——由呼叫端決定語義
// （loadAccounts 視為空清單，deleteAccount 視為無索引可除名）。
func loadIndex(ctx context.Context) (*accountIndex, error) {
	st, err := newAccountStore(indexKey)
	if err != nil {
		return nil, err
	}
	raw, err := st.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("accounts: 讀取帳號索引: %w", err)
	}
	if raw == "" {
		return nil, nil
	}
	var idx accountIndex
	if err := json.Unmarshal([]byte(raw), &idx); err != nil {
		// 損毀 JSON 的解析錯誤可能引述內容片段，不納入錯誤字串
		//（REQ-INV-01）。
		return nil, &xerrors.MaterialError{Msg: "帳號索引無法讀取（系統憑證庫）"}
	}
	return &idx, nil
}

// saveAccount 原子寫帳號鍵（Account JSON 取代既有值）：同 UserID 再匯入
// ＝覆蓋同鍵（REQ-WEB-070）。超過 MaxSecretSize 由底層回錯，不靜默截斷。
func saveAccount(ctx context.Context, acc Account) error {
	b, err := json.Marshal(acc)
	if err != nil {
		return fmt.Errorf("accounts: 序列化帳號 %s: %w", acc.UserID, err)
	}
	st, err := newAccountStore(accountKey(acc.UserID))
	if err != nil {
		return err
	}
	if err := st.Save(ctx, string(b)); err != nil {
		return fmt.Errorf("accounts: 寫入帳號 %s: %w", acc.UserID, err)
	}
	return nil
}

// saveIndex 寫索引鍵（accountIndex JSON 取代既有值）；索引是帳號清單
// 真源，呼叫端負責與帳號鍵的一致性（遷移「先寫後刪」、上限檢查等）。
func saveIndex(ctx context.Context, idx accountIndex) error {
	b, err := json.Marshal(idx)
	if err != nil {
		return fmt.Errorf("accounts: 序列化帳號索引: %w", err)
	}
	st, err := newAccountStore(indexKey)
	if err != nil {
		return err
	}
	if err := st.Save(ctx, string(b)); err != nil {
		return fmt.Errorf("accounts: 寫入帳號索引: %w", err)
	}
	return nil
}

// deleteAccount 刪帳號鍵並自索引除名；被刪者為 Current 時 Current 歸空
// （移除活躍帳號導回 login 由呼叫端接線）。索引不存在時僅刪帳號鍵。
// 帳號鍵或索引原本就不存在皆不視為錯誤（冪等）。
func deleteAccount(ctx context.Context, userID string) error {
	st, err := newAccountStore(accountKey(userID))
	if err != nil {
		return err
	}
	if err := st.Delete(); err != nil {
		return fmt.Errorf("accounts: 刪除帳號 %s: %w", userID, err)
	}
	idx, err := loadIndex(ctx)
	if err != nil {
		return err
	}
	if idx == nil {
		return nil
	}
	kept := make([]string, 0, len(idx.Order))
	for _, id := range idx.Order {
		if id != userID {
			kept = append(kept, id)
		}
	}
	if idx.Current == userID {
		idx.Current = ""
	}
	idx.Order = kept
	return saveIndex(ctx, *idx)
}

// accountCookieStore 以帳號鍵包裝 auth.CookieStore 的 app 層 adapter
// （控制器裁決 2026-11-14：鍵內容碰撞）。auth.CookieStore 寫 raw cookie 行、
// saveAccount 寫 Account JSON，兩者共用 x-account-<uid> 鍵——引擎
// write-through 與 quit flush 未經本 adapter 時，第一次落盤即以 raw 行
// 覆蓋 Account JSON，loadAccounts 全清單 MaterialError。本 adapter 把
// raw 行讀寫收斂為 Account JSON 的 CookieLine 欄位讀寫：
//
//   - Load：讀 secret → Unmarshal Account JSON → 回 .CookieLine。鍵缺失 →
//     ("", nil)（沿底層 ErrSecretNotFound 語義）；JSON 損毀 → MaterialError
//     （讀路徑的中繼資料損毀必須浮現，同 loadAccounts；錯誤字串不含內容
//     片段，REQ-INV-01）。
//   - Save：read-modify-write——讀回現值（缺失或損毀 → 最小重建
//     Account{UserID, CookieLine: line}——憑證輪替不可被中繼資料缺失或
//     損毀阻斷，documented trade-off），僅更新 .CookieLine 後寫回。寫回值
//     恆為 Account JSON，不觸發底層「空行＝刪除」語義。
//   - Delete：刪鍵。
//
// 內部走 newStore(accountService, accountKey(userID))——即
// newAccountStore 的帳號鍵建構路徑經 newStore seam（T5 的記錄器替身測試
// 由此注入；adapter 包於 seam 之下，seam 本體保留）。鍵恆帶 x-account-
// 前綴不為空，建構錯誤（僅空鍵可能）於首次操作時原樣向上，不靜默吞。
type accountCookieStore struct {
	userID   string
	inner    auth.CookieStore // 帳號鍵底層儲存層
	buildErr error            // 底層建構錯誤（首次操作時原樣向上）
}

// 編譯期斷言：accountCookieStore 實作引擎 write-through 的儲存層介面。
var _ auth.CookieStore = (*accountCookieStore)(nil)

// newAccountCookieStore 建構指定帳號的 adapter store（見型別註解）。
func newAccountCookieStore(userID string) auth.CookieStore {
	inner, err := newStore(accountService, accountKey(userID))
	return &accountCookieStore{userID: userID, inner: inner, buildErr: err}
}

// Load 實作 auth.CookieStore：回帳號 JSON 的 .CookieLine（見型別註解）。
func (s *accountCookieStore) Load(ctx context.Context) (string, error) {
	if s.buildErr != nil {
		return "", s.buildErr
	}
	raw, err := s.inner.Load(ctx)
	if err != nil || raw == "" {
		return "", err // 鍵缺失（"", nil）或底層讀取錯誤原樣向上
	}
	var acc Account
	if err := json.Unmarshal([]byte(raw), &acc); err != nil {
		// 損毀 JSON 的解析錯誤可能引述內容片段，不納入錯誤字串（REQ-INV-01）。
		return "", &xerrors.MaterialError{Msg: fmt.Sprintf("帳號 %s 資料無法讀取（系統憑證庫）", s.userID)}
	}
	return acc.CookieLine, nil
}

// Save 實作 auth.CookieStore：read-modify-write 僅更新 .CookieLine（見
// 型別註解）。
func (s *accountCookieStore) Save(ctx context.Context, line string) error {
	if s.buildErr != nil {
		return s.buildErr
	}
	raw, err := s.inner.Load(ctx)
	if err != nil {
		return err
	}
	acc := Account{UserID: s.userID, CookieLine: line} // 缺失或損毀 → 最小重建
	if raw != "" {
		var cur Account
		if err := json.Unmarshal([]byte(raw), &cur); err == nil {
			cur.CookieLine = line
			acc = cur
		}
	}
	b, err := json.Marshal(acc)
	if err != nil {
		return fmt.Errorf("accounts: 序列化帳號 %s: %w", s.userID, err)
	}
	if err := s.inner.Save(ctx, string(b)); err != nil {
		return fmt.Errorf("accounts: 寫入帳號 %s: %w", s.userID, err)
	}
	return nil
}

// Delete 刪除帳號鍵（登出／移除流程用；底層實作皆載 Delete）。
func (s *accountCookieStore) Delete() error {
	if s.buildErr != nil {
		return s.buildErr
	}
	if d, ok := s.inner.(interface{ Delete() error }); ok {
		if err := d.Delete(); err != nil {
			return fmt.Errorf("accounts: 刪除帳號 %s: %w", s.userID, err)
		}
		return nil
	}
	return fmt.Errorf("accounts: 帳號 %s 的底層儲存層不支援刪除", s.userID)
}

// verifyTTL 是憑證有效性快取的存活時間（§8.5：TTL 內切換免重驗——需求方
// 「降低取得次數、加快切換」）。
const verifyTTL = 10 * time.Minute

// credCache 是憑證有效性 TTL 快取：user_id → 最後驗證時刻。切換序列於
// TTL 內跳過 Validate（§8.4 步驟 5）；驗證成功 mark、失敗 invalidate。
// 零值可用（mark 惰性建表）；全方法併發安全（mutex——切換於背景
// goroutine 執行）。
type credCache struct {
	mu       sync.Mutex
	verified map[string]time.Time
}

// fresh 回該帳號憑證是否仍在 TTL 內（從未驗證或已過期回 false）。
func (c *credCache) fresh(userID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.verified[userID]
	return ok && time.Since(t) < verifyTTL
}

// mark 記錄驗證成功時刻（TTL 起算點）。
func (c *credCache) mark(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.verified == nil {
		c.verified = make(map[string]time.Time)
	}
	c.verified[userID] = time.Now()
}

// invalidate 清除驗證記錄（切換驗證失敗時——下次切換強制重驗）。
func (c *credCache) invalidate(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.verified, userID)
}
