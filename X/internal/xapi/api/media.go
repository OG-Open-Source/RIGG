// media 萃取：Post.Media 資料面（v1.1 設計 §8.2；REQ-DAT-013/015/016/018
// 語義的 v1.1 子集）。extended_entities.media 為唯一錨點——legacy.entities
// 不做 fallback（首版）；photo 取 media_url_https 為基底＋sizes small/large，
// video／gif 取 video_info.variants 中 content_type=="video/mp4" 的最高
// bitrate（REQ-DAT-016；D13：GIF 走 mp4；m3u8 不採——P0' 實證 HLS 為
// application/x-mpegURL）。CDN URL 的 format 參數必須沿用 media_url_https
// 的路徑副檔名（P0' 實證：PNG 來源誤用 format=jpg → 404——DECISIONS
// 〈v1.1 P0' 探測〉媒體節、Review Focus 5）。本檔僅做節點讀取，不發網路
// 請求；無效項（未知 type／無基底 URL／無 mp4 variant）一律跳過，全數無效
// 或缺錨點回 nil——Media 非 nil 必 len>0。
package api

import (
	"strconv"
	"strings"
)

// MediaItem 是單一媒體項（REQ-DAT-011/013/015/016/018 的 v1.1 子集）。
type MediaItem struct {
	Kind     string // "photo" | "video" | "gif"（DAT-013 三分）
	ThumbURL string // 列表縮圖
	URL      string // 展開檢視：photo → name=large；video/gif → 最高位元率 mp4
	Width    int
	Height   int
}

// extractMedia 自貼文 result 節點萃取媒體：extended_entities.media 為唯一
// 錨點（設計 §8.2——legacy.entities 不做 fallback，REQ-DAT-015 語義的 v1.1
// 形態）。錨點位置相容兩形：result.extended_entities.media（契約形）與
// result.legacy.extended_entities.media（REQ-DAT-015 的線上實際路徑）。
// 回 nil 表示無媒體：缺 extended_entities、media 為空陣列、或項目全數無效
// （未知 type／無基底 URL／無 mp4 variant）——非 nil 必 len>0。
func extractMedia(result map[string]any) []MediaItem {
	entities, _ := result["extended_entities"].(map[string]any)
	if entities == nil {
		if legacy, ok := result["legacy"].(map[string]any); ok {
			entities, _ = legacy["extended_entities"].(map[string]any)
		}
	}
	if entities == nil {
		return nil
	}
	mediaList, _ := entities["media"].([]any)
	var out []MediaItem
	for _, raw := range mediaList {
		node, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		var item MediaItem
		var valid bool
		switch node["type"] { // Kind 判定：其他值跳過該項（設計 §8.2）
		case "photo":
			item, valid = extractPhotoMedia(node)
		case "video":
			item, valid = extractVideoMedia(node, "video")
		case "animated_gif":
			item, valid = extractVideoMedia(node, "gif")
		}
		if valid {
			out = append(out, item)
		}
	}
	return out
}

// extractPhotoMedia 萃取 photo 節：media_url_https 為基底，ThumbURL/URL 為
// <base>?format=<ext>&name=small|large（ext 沿用基底副檔名——P0' 實證）；
// Width/Height 取 sizes.large（展開檢視的對應項，缺則 0）。缺 media_url_https
// 視為結構不符，回 invalid 跳過該項。
func extractPhotoMedia(node map[string]any) (MediaItem, bool) {
	base, _ := node["media_url_https"].(string)
	if base == "" {
		return MediaItem{}, false
	}
	item := MediaItem{
		Kind:     "photo",
		ThumbURL: mediaCDNURL(base, "small"),
		URL:      mediaCDNURL(base, "large"),
	}
	if sizes, ok := node["sizes"].(map[string]any); ok {
		if large, ok := sizes["large"].(map[string]any); ok {
			item.Width = nodeInt(large["w"])
			item.Height = nodeInt(large["h"])
		}
	}
	return item, true
}

// extractVideoMedia 萃取 video／gif 節：URL 取 video_info.variants 中
// content_type=="video/mp4" 的最高 bitrate（REQ-DAT-016；GIF 走 mp4——D13；
// m3u8 不採）；無 mp4 variant → 展開檢視無來源，回 invalid 跳過該項。縮圖
// ＝media_url_https?format=<ext>&name=small（media_url_https 為 video_info
// 的 poster 形——節點無之則 ThumbURL 留空）；Width/Height 留 0
// （video_info.aspect_ratio 無法換算實際值，寬高不強求）。
func extractVideoMedia(node map[string]any, kind string) (MediaItem, bool) {
	info, _ := node["video_info"].(map[string]any)
	var variants []any
	if info != nil {
		variants, _ = info["variants"].([]any)
	}
	var best map[string]any
	if kind == "gif" {
		// D24 修訂：GIF 走純 Go GIF 解碼（NewGIFDecoder 需 GIF 本體）——
		// URL 取 variants 中的 image/gif variant（animated_gif 的
		// video_info 恆含之）；缺 gif variant 時退回 mp4（呼叫端以
		// content_type 判別後續處理）。
		best = bestGifVariant(variants)
		if best == nil {
			best = bestMP4Variant(variants)
		}
	} else {
		best = bestMP4Variant(variants)
	}
	if best == nil {
		return MediaItem{}, false
	}
	item := MediaItem{Kind: kind, URL: nodeString(best["url"])}
	// 影片尺寸自 variant URL 解析（形如 /vid/<codec>/<w>x<h>/…）——展開
	// 檢視的媒體盒以比例自適應高（v1.1.4：固定高造成直式影片 letterbox）。
	if w, h, ok := videoDimsFromURL(item.URL); ok {
		item.Width, item.Height = w, h
	}
	if base, _ := node["media_url_https"].(string); base != "" {
		item.ThumbURL = mediaCDNURL(base, "small")
	}
	return item, true
}

// videoDimsFromURL 自 video variant URL 解析像素尺寸（X 的 variant 路徑
// 恆含 /vid/<codec>/<w>x<h>/ 段）；無此段回 ok=false。
func videoDimsFromURL(url string) (int, int, bool) {
	i := strings.LastIndex(url, "/vid/")
	if i < 0 {
		return 0, 0, false
	}
	segs := strings.Split(url[i+5:], "/")
	if len(segs) < 2 {
		return 0, 0, false
	}
	parts := strings.SplitN(segs[1], "x", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	w, err1 := strconv.Atoi(parts[0])
	h, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

// bestMP4Variant 自 variants 過濾 content_type=="video/mp4"（REQ-DAT-016；
// m3u8 為 application/x-mpegURL 不入選——P0' 實證），回 bitrate 最高者；
// 並列取遍歷序首個（嚴格大於方替換）；無 mp4 回 nil。
func bestMP4Variant(variants []any) map[string]any {
	var best map[string]any
	bestBitrate := 0.0
	for _, v := range variants {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if nodeString(m["content_type"]) != "video/mp4" {
			continue
		}
		if bitrate := float64(nodeInt(m["bitrate"])); best == nil || bitrate > bestBitrate {
			best = m
			bestBitrate = bitrate
		}
	}
	return best
}

// bestGifVariant 自 variants 過濾 content_type=="image/gif"（D24：GIF 走
// 純 Go GIF 解碼，需 GIF 本體；animated_gif 的 video_info 恆含 gif
// variant）；並列取遍歷序首個；無 gif variant 回 nil（呼叫端退回 mp4）。
func bestGifVariant(variants []any) map[string]any {
	for _, v := range variants {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if nodeString(m["content_type"]) == "image/gif" {
			return m
		}
	}
	return nil
}

// mediaCDNURL 組媒體 CDN URL：<base>?format=<ext>&name=<name>。<ext> 沿用
// base 的路徑副檔名（P0' 實證：format 必須與 media_url_https 副檔名一致，
// PNG 誤用 jpg → 404——DECISIONS〈v1.1 P0' 探測〉媒體節），無副檔名預設
// jpg（設計 §8.2）；僅取路徑末段推導（遇 '?' 截斷，防查詢串混入 ext）。
func mediaCDNURL(base, name string) string {
	segment := base
	if i := strings.IndexByte(segment, '?'); i >= 0 {
		segment = segment[:i]
	}
	if i := strings.LastIndexByte(segment, '/'); i >= 0 {
		segment = segment[i+1:]
	}
	ext := "jpg"
	if i := strings.LastIndexByte(segment, '.'); i >= 0 && segment[i+1:] != "" {
		ext = segment[i+1:]
	}
	return base + "?format=" + ext + "&name=" + name
}

// nodeString 讀節點字串欄位（非字串回空字串）。
func nodeString(v any) string {
	s, _ := v.(string)
	return s
}

// nodeInt 讀節點數值欄位：JSON unmarshal 後為 float64，容忍 Go 原生
// int／int64（直組 map 的呼叫端）；無法解讀回 0。
func nodeInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}
