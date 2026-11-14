// migrate：v1 單鍵（x-cookie-line）→ v1.1 多帳號模型的遷移（設計 §8.3）。
// 序列「先寫後刪」（crash-safe）：任何失敗點絕不清除舊鍵；成功後舊鍵亦
// 保留——降級雙寫（dualWrite）的目標，v1.2 移除雙寫時一併清理。錯誤字串
// 不含 cookie 值（REQ-INV-01）。
package main

import (
	"context"
	"fmt"
)

// migrateSessionFactory 是遷移組臨時會話的工廠 seam：正式路徑以 store=nil
// 組裝（遷移只驗身分，不接 write-through 儲存層）；測試注入 fakeSession
// 隔離（不打真實 X）。
var migrateSessionFactory = func(cookieLine, proxyURL string) (xSession, error) {
	return assembleSession(cookieLine, proxyURL, nil)
}

// migrateLegacyStore 檢查 v1 舊鍵並遷移至帳號模型（§8.3 序列）：
//
//   - 舊鍵無值 → 無事可遷，nil（首次啟動的正常狀態）。
//   - 索引已存在 → no-op：不組臨時會話、不動舊鍵（雙寫目標仍需它），nil。
//   - 以舊鍵值組臨時會話 → Validate 取得身分；失敗 → 保留舊鍵、回 error
//     （呈 login，絕不清除）。
//   - 成功 → saveAccount（身分與憑證綁定，REQ-WEB-069）→ saveIndex
//     (Order=[uid], Current=uid)；舊鍵保留作降級雙寫目標（v1.2 清理）。
//
// 啟動接線（loadAccounts 之後）屬 Task 6 warmup；本函式只承載遷移序列。
func migrateLegacyStore(ctx context.Context, proxyURL string) error {
	legacy, err := legacyCookieLine(ctx)
	if err != nil {
		return err
	}
	if legacy == "" {
		return nil
	}
	idx, err := loadIndex(ctx)
	if err != nil {
		return err
	}
	if idx != nil {
		return nil // 索引已存在：已遷移，no-op（舊鍵為雙寫目標，保留）
	}
	sess, err := migrateSessionFactory(legacy, proxyURL)
	if err != nil {
		return err
	}
	info, err := sess.Validate(legacy)
	if err != nil {
		return fmt.Errorf("migrate: 舊憑證驗證失敗（保留舊鍵）: %w", err)
	}
	// 先寫（帳號鍵 → 索引）後續；全程不刪舊鍵。中途失敗時索引不存在，
	// 下次啟動重遷同鍵覆蓋（REQ-WEB-070），無需回滾。
	acc := Account{UserID: info.UserID, ScreenName: info.ScreenName, Name: info.Name, CookieLine: legacy}
	if err := saveAccount(ctx, acc); err != nil {
		return err
	}
	return saveIndex(ctx, accountIndex{Version: accountIndexVersion, Order: []string{acc.UserID}, Current: acc.UserID})
}

// legacyCookieLine 讀回 v1 舊鍵現值（經 accountService seam——正式路徑為
// MyGO 預設 service，與 v1 寫入點 NewCookieStore("", cookieLineKey) 同一
// 作用域；測試經 seam 隔離）。缺失回空字串（首啟正常狀態，非錯誤）。
func legacyCookieLine(ctx context.Context) (string, error) {
	st, err := newAccountStore(cookieLineKey)
	if err != nil {
		return "", err
	}
	line, err := st.Load(ctx)
	if err != nil {
		return "", fmt.Errorf("migrate: 讀取舊鍵失敗: %w", err)
	}
	return line, nil
}

// dualWrite 把活躍帳號的 cookie 行同步寫入 v1 舊鍵 x-cookie-line（降級
// 雙寫期：v1.1 全程，二進位降級不丟會話；v1.2 移除雙寫時清理）。寫入經
// accountService seam（正式＝MyGO 預設 service，同 v1 鍵作用域）。
func dualWrite(activeCookieLine string) error {
	st, err := newAccountStore(cookieLineKey)
	if err != nil {
		return err
	}
	if err := st.Save(context.Background(), activeCookieLine); err != nil {
		return fmt.Errorf("migrate: 雙寫舊鍵失敗: %w", err)
	}
	return nil
}
