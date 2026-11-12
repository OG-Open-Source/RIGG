// cookies_store：app 層的 CookieStore 實作——系統憑證庫（MyGO Secrets）。
// 儲存選型依 OQ7 fallback 鏈：MyGO secrets API（存在且形狀吻合：Set/Get/
// Delete 對應 Save/Load，單行 cookie 遠小於 MaxSecretSize=2560，且
// 「work before Run」使測試可用）→ MyGO SQLite 插件 → JSON 檔，本實作取
// 鏈首。憑證（cookie 字串）僅存於系統加密憑證庫，絕不進日誌或錯誤訊息
// （REQ-INV-01）；錯誤包裝僅附固定文字與底層儲存層訊息，不承載值本身。
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/egoist/mygo"

	xapi "github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"
	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// cookieLineKey 是系統憑證庫中存放單行 cookie 的鍵（≤256 位元組）。
const cookieLineKey = "x-cookie-line"

// 編譯期斷言：SecretsCookieStore 實作 xapi.CookieStore（REQ-AUTH-002）。
var _ xapi.CookieStore = (*SecretsCookieStore)(nil)

// SecretsCookieStore 以系統憑證庫（Windows Credential Manager／macOS
// Keychain／Linux Secret Service）持久化單行 cookie 字串。零值不可用，
// 經 NewCookieStore 建構。型別名隨選型結果（MyGO Secrets）而得；若鏈首
// 不成立而走 SQLite／JSON fallback，型別名應相應改動，介面契約不變。
type SecretsCookieStore struct {
	// secrets 是作用域化（Service）後的 MyGO Secrets 模組；零 service
	// 即 app 自身的 identifier。
	secrets mygo.SecretsModule
	// key 是憑證庫中的存放鍵。
	key string
}

// NewCookieStore 建構以系統憑證庫落盤的 CookieStore。
//
//   - service 為空字串時使用 MyGO 預設 service（app identifier；`mygo dev`
//     下自動與正式 app 區隔），非空時以 Secrets.Service 作用域化——測試
//     以唯一 service 名隔離，正式呼叫傳空字串即可。
//   - key 不得為空（系統憑證庫的帳目鍵）。
//
// 不開啟連線、不做 IO：系統憑證庫呼叫即時進行（MyGO Secrets 文件：
// 「safe from any goroutine and work before Run」），故無可關閉資源，
// 不提供 Close。
func NewCookieStore(service, key string) (*SecretsCookieStore, error) {
	if key == "" {
		return nil, &xerrors.InvalidInputError{Msg: "cookie store: key 不得為空"}
	}
	s := mygo.Secrets
	if service != "" {
		s = s.Service(service)
	}
	return &SecretsCookieStore{secrets: s, key: key}, nil
}

// Load 讀回單行 cookie 字串。未存放任何憑證（ErrSecretNotFound）回
// ("", nil)——首次啟動的正常狀態，不是錯誤；呼叫端以空字串判斷「尚無
// 憑證」。ctx 僅符介面形狀：系統憑證庫呼叫不受 ctx 控制（文件：可能於
// 系統詢問使用者時阻塞，如解鎖 keyring）。
func (s *SecretsCookieStore) Load(ctx context.Context) (string, error) {
	b, err := s.secrets.Get(s.key)
	if err != nil {
		if errors.Is(err, mygo.ErrSecretNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("cookie store: 讀取失敗: %w", err)
	}
	return string(b), nil
}

// Save 寫回單行 cookie 字串（取代既有值）；空字串視為清除（登出語義）。
// 超過 MyGO MaxSecretSize 的行由底層拒絕並回錯誤——絕不靜默截斷或改存
// 他處。
func (s *SecretsCookieStore) Save(ctx context.Context, cookieLine string) error {
	if cookieLine == "" {
		return s.Delete()
	}
	if err := s.secrets.Set(s.key, []byte(cookieLine)); err != nil {
		return fmt.Errorf("cookie store: 寫入失敗: %w", err)
	}
	return nil
}

// Delete 清除存放的 cookie（登出、測試清理用）；對不存在的鍵不視為錯誤。
func (s *SecretsCookieStore) Delete() error {
	if err := s.secrets.Delete(s.key); err != nil {
		return fmt.Errorf("cookie store: 清除失敗: %w", err)
	}
	return nil
}
