// Package auth 實作 XAuth：不可變 snapshot 與原子替換（版本化計數），
// 提供 PrepareAuth／UpdateCookies／IsLoggedIn／CookieHeader／CSRFToken；
// 內含預設公開 web bearer 常數、guest token 方法位（回 unimplemented），
// 並宣告 CookieStore 窄介面（REQ-AUTH-001..014）。
package auth
