// proxy：app 層的代理解析——引擎唯一的環境變數讀取點（架構鐵律：env
// 讀取全部集中在本檔，不得外溢至 internal/xapi）。解析鏈依 DECISIONS D6：
// X_PROXY → HTTPS_PROXY/ALL_PROXY → MyGO netproxy 系統偵測 → 皆無則
// fail-closed（("", error)）；無註冊表層、無埠掃描探測層（D6 裁決移除，
// 本檔的預檢僅對「已偵測到的單一位址」做可達性確認，非探測發現）。
package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/egoist/mygo/netproxy"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// 代理鏈的環境變數名（僅本檔讀取）。
const (
	envXProxy     = "X_PROXY"
	envHTTPSProxy = "HTTPS_PROXY"
	envAllProxy   = "ALL_PROXY"
)

// proxyPreflightTimeout 是系統偵測命中後的預檢短逾時（brief 規定 5 秒）。
const proxyPreflightTimeout = 5 * time.Second

// detectSystemProxy 是系統代理偵測的注入縫：包住 MyGO netproxy 對引擎
// 主要 host（x.com）的系統設定詢問（Windows 走 WinHTTP，含 PAC/WPAD）。
// 回 (proxyURL, true) 表偵測命中。測試以替身替換本變數；netproxy 亦自旁
// 讀標準代理環境變數，但本縫僅於 X_PROXY/HTTPS_PROXY/ALL_PROXY 皆空時
// 才會被詢問，其 env 語義與鏈首兩層一致（詳見 ResolveProxy）。
var detectSystemProxy = func() (string, bool) {
	req, err := http.NewRequest(http.MethodGet, "https://x.com/", nil)
	if err != nil {
		return "", false
	}
	u, err := netproxy.Proxy(req)
	if err != nil || u == nil || u.Host == "" {
		return "", false
	}
	return u.String(), true
}

// ResolveProxy 依 D6 鏈解析引擎出口代理，回 "scheme://host:port" 形式
// 字串（transport.Options.ProxyURL 契約）。
//
//   - X_PROXY → HTTPS_PROXY → ALL_PROXY：使用者顯式配置，命中即回、不經
//     預檢（配置錯誤由 transport 於 New 時 fail-closed 呈現）。空值視同
//     未設定。
//   - 系統偵測（netproxy）：啟發式結果，命中後對偵測位址做 5 秒短逾時
//     預檢，通過才採用；預檢失敗回 ("", error)，不靜默退回直連。
//   - 皆無：("", error)——fail-closed（REQ-INV-02），絕不回直連。
//
// 錯誤分類：無可用代理為可重試的 bootstrap 素材缺失（kind=material）；
// 偵測位址預檢失敗為傳輸層（kind=transport）。訊息不含任何憑證材料。
func ResolveProxy() (string, error) {
	for _, name := range []string{envXProxy, envHTTPSProxy, envAllProxy} {
		if v := os.Getenv(name); v != "" {
			return v, nil
		}
	}
	if p, ok := detectSystemProxy(); ok && p != "" {
		if err := proxyPreflight(p); err != nil {
			return "", err
		}
		return p, nil
	}
	return "", &xerrors.MaterialError{Msg: "無可用代理：X_PROXY／HTTPS_PROXY／ALL_PROXY 皆未設定且系統偵測無結果（fail-closed，REQ-INV-02、D6）"}
}

// proxyPreflight 對偵測位址做 TCP 短逾時預檢：僅驗「單一已知位址」的
// 可達性（D6 禁止的是以掃描候選埠發現代理的探測層，非對已得位址的
// 連線確認）。位址無法解析或連不上即回 *xerrors.TransportError。
func proxyPreflight(proxy string) error {
	u, err := url.Parse(proxy)
	if err != nil || u.Host == "" {
		return &xerrors.TransportError{Msg: fmt.Sprintf("代理預檢失敗：偵測位址無效（fail-closed）：%q", proxy)}
	}
	conn, err := net.DialTimeout("tcp", u.Host, proxyPreflightTimeout)
	if err != nil {
		return &xerrors.TransportError{Msg: fmt.Sprintf("代理預檢失敗：%s 於 %v 內不可達：%v", u.Host, proxyPreflightTimeout, err)}
	}
	_ = conn.Close()
	return nil
}
