// probe_findings 記錄 v1.1 P0' 探測輪的結論常數（2026-11-14，執行證據見
// docs/DECISIONS.md 〈v1.1 P0' 探測〉節）：僅為設計裁決的程式內錨點，無
// 邏輯；線上語義的變動仍以執行期註冊表快照（static/graphql.json）為準。
// 本檔不得承載 queryId——queryId 的家是快照，雙寫必漂移。
package api

// P0' 探測判準：每臂 ≥20 次、跨 2 會話形態、以「HTTP 200 且回應 data 含
// 貼文」為通過（D1 判準沿用）。
const (
	// ProbeHomeLatestMode 是 HomeLatestTimeline 的送出形態結論："dual"——
	// 與 HomeTimeline 同構（首頁 GET 無 seen；cursor/seen 非空 → POST 帶
	// seenTweetIds）。實證：GET 20/20、POST 20/20、零失敗，空 features。
	ProbeHomeLatestMode = "dual"

	// ProbeSearchAlwaysPOST：SearchTimeline 恆 POST（REQ-GQL-005 語義，
	// 實證 20/20——本探測僅以 POST 送出，GET 臂未投）。
	ProbeSearchAlwaysPOST = true

	// ProbeSearchProductRequired：SearchTimeline variables 必須含 product
	//（"Top"/"Latest"）——缺product 回 422 GRAPHQL_VALIDATION_FAILED
	// "must be defined: variable product"（P0' 實證）。空 features 可行。
	ProbeSearchProductRequired = true

	// ProbeMediaCDNDirectOK：媒體 CDN（pbs.twimg.com／video.twimg.com）免
	// 認證直載可行——縮圖 ?format=<ext>&name=small 10/10（200 image/*）、
	// mp4 variant HEAD 3/3（video/mp4）、m3u8 2/2（application/x-mpegURL）。
	// format 參數必須沿用 media_url_https 的副檔名（社群文檔與本輪一致）。
	ProbeMediaCDNDirectOK = true
)
