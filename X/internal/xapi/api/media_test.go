package api

// media 測試：Post.Media 萃取語義（v1.1 設計 §8.2、REQ-DAT-013/015/016/018）：
// photo 以 media_url_https 為基底、format 沿用其路徑副檔名（P0' 實證：PNG
// 來源誤用 format=jpg → 404——Review Focus 5 釘測）、sizes small/large 對應
// ThumbURL/URL、Width/Height 取 large；video／gif 過濾
// content_type=="video/mp4" 取 bitrate 最高（m3u8 不採——D13）；未知 type
// 跳過；缺 extended_entities → Media 為 nil（非 nil 必 len>0）。fixture 一律
// 合成（欄位名沿 REQ 語義，值全自造）；經 tlData／tlEntry／tlItem 家族掛
// 條目樹，交共用解析尾段 parseTimelineResult 解析（wire 形態由
// timeline_test.go 覆蓋，此處聚焦萃取）。

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// parseFixturePosts 將合成條目樹 marshal 為 {"data":…} 信封，交共用解析尾段
// parseTimelineResult 解析（與 Send 之後的解析路徑同一函式）並回傳 Posts。
func parseFixturePosts(t *testing.T, data map[string]any) []Post {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("fixture 序列化失敗：%v", err)
	}
	res, err := parseTimelineResult("HomeTimeline", []byte(`{"data":`+string(raw)+`}`))
	if err != nil {
		t.Fatalf("parseTimelineResult error = %v, 期望 nil", err)
	}
	return res.Posts
}

// tlMediaEntry 合成掛媒體的 tweet- 條目：extended_entities.media 錨點掛在
// result 上（設計 §8.2 契約形；REQ-DAT-015 的 legacy.extended_entities 實際
// 路徑由 extractMedia 相容，另於 TestMediaExtractionPhoto 驗證）。
func tlMediaEntry(entryID, restID, fullText string, media ...map[string]any) map[string]any {
	result := tlResult(restID, fullText)
	nodes := make([]any, 0, len(media))
	for _, m := range media {
		nodes = append(nodes, m)
	}
	result["extended_entities"] = map[string]any{"media": nodes}
	return tlEntry(entryID, tlItem(result))
}

// tlPhotoNode 合成 photo 媒體節（欄位名沿 REQ-DAT-015/018 語義，值全自造）。
func tlPhotoNode(mediaURL string, smallW, smallH, largeW, largeH int) map[string]any {
	return map[string]any{
		"type":            "photo",
		"media_url_https": mediaURL,
		"sizes": map[string]any{
			"small": map[string]any{"w": smallW, "h": smallH, "resize": "fit"},
			"large": map[string]any{"w": largeW, "h": largeH, "resize": "fit"},
		},
	}
}

// tlVariant 合成 video_info.variants 項（REQ-DAT-016 欄位語義）。
func tlVariant(contentType string, bitrate int, url string) map[string]any {
	return map[string]any{"content_type": contentType, "bitrate": bitrate, "url": url}
}

// tlVideoNode 合成帶 video_info.variants 的媒體節（video／animated_gif 共用，
// 亦供未知 type 用例）；mediaURL 為空 → 節點不帶 media_url_https（驗證
// 「ThumbURL 留空」規則）。
func tlVideoNode(mediaType, mediaURL string, variants ...map[string]any) map[string]any {
	nodes := make([]any, 0, len(variants))
	for _, v := range variants {
		nodes = append(nodes, v)
	}
	node := map[string]any{
		"type":       mediaType,
		"video_info": map[string]any{"variants": nodes},
	}
	if mediaURL != "" {
		node["media_url_https"] = mediaURL
	}
	return node
}

// TestMediaExtractionPhoto：photo 節 → ThumbURL/URL 為
// media_url_https?format=<ext>&name=small|large，Width/Height 取 sizes.large
// （展開檢視的對應項）；無副檔名 → 預設 jpg；REQ-DAT-015 的
// legacy.extended_entities 實際路徑同錨相容。
func TestMediaExtractionPhoto(t *testing.T) {
	data := tlData([]map[string]any{
		tlMediaEntry(
			"tweet-1", "1001", "photo post",
			tlPhotoNode("https://pbs.twimg.test/media/PHOTO1.jpg", 120, 80, 2048, 1365),
		),
		tlMediaEntry(
			"tweet-2", "1002", "no extension",
			tlPhotoNode("https://pbs.twimg.test/media/BARE2", 60, 40, 640, 427),
		),
	})
	posts := parseFixturePosts(t, data)
	if len(posts) != 2 {
		t.Fatalf("應得 2 條貼文，得到 %d：%+v", len(posts), posts)
	}
	want1 := []MediaItem{{
		Kind:     "photo",
		ThumbURL: "https://pbs.twimg.test/media/PHOTO1.jpg?format=jpg&name=small",
		URL:      "https://pbs.twimg.test/media/PHOTO1.jpg?format=jpg&name=large",
		Width:    2048,
		Height:   1365,
	}}
	if !reflect.DeepEqual(posts[0].Media, want1) {
		t.Errorf("Posts[0].Media = %+v, 期望 %+v（small/large 的 format 與 name 參數、Width/Height 取 large）", posts[0].Media, want1)
	}
	want2 := []MediaItem{{
		Kind:     "photo",
		ThumbURL: "https://pbs.twimg.test/media/BARE2?format=jpg&name=small",
		URL:      "https://pbs.twimg.test/media/BARE2?format=jpg&name=large",
		Width:    640,
		Height:   427,
	}}
	if !reflect.DeepEqual(posts[1].Media, want2) {
		t.Errorf("Posts[1].Media = %+v, 期望 %+v（media_url_https 無副檔名 → 預設 jpg）", posts[1].Media, want2)
	}

	// REQ-DAT-015 的實際路徑：extended_entities 掛在 legacy 之下——同一錨
	// 點鍵，extractMedia 應相容萃取。
	nested := tlResult("1003", "legacy nested")
	legacy, _ := nested["legacy"].(map[string]any)
	legacy["extended_entities"] = map[string]any{"media": []any{
		tlPhotoNode("https://pbs.twimg.test/media/NESTED3.jpg", 30, 20, 300, 200),
	}}
	postsNested := parseFixturePosts(t, tlData([]map[string]any{tlEntry("tweet-3", tlItem(nested))}))
	if len(postsNested) != 1 {
		t.Fatalf("應得 1 條貼文，得到 %d", len(postsNested))
	}
	want3 := []MediaItem{{
		Kind:     "photo",
		ThumbURL: "https://pbs.twimg.test/media/NESTED3.jpg?format=jpg&name=small",
		URL:      "https://pbs.twimg.test/media/NESTED3.jpg?format=jpg&name=large",
		Width:    300,
		Height:   200,
	}}
	if !reflect.DeepEqual(postsNested[0].Media, want3) {
		t.Errorf("legacy.extended_entities 路徑應同錨相容：got %+v, want %+v", postsNested[0].Media, want3)
	}
}

// TestMediaExtractionPreservesFormat：media_url_https 以 .png 結尾 →
// ThumbURL/URL 用 format=png（沿用來源副檔名），且不得出現 format=jpg
// （P0' 實證：PNG 來源誤用 format=jpg → 404——Review Focus 5 釘測）。
func TestMediaExtractionPreservesFormat(t *testing.T) {
	data := tlData([]map[string]any{
		tlMediaEntry(
			"tweet-1", "1101", "png post",
			tlPhotoNode("https://pbs.twimg.test/media/PNGSRC.png", 100, 100, 900, 900),
		),
	})
	posts := parseFixturePosts(t, data)
	if len(posts) != 1 {
		t.Fatalf("應得 1 條貼文，得到 %d", len(posts))
	}
	got := posts[0].Media
	if len(got) != 1 {
		t.Fatalf("應得 1 個媒體項，得到 %d：%+v", len(got), got)
	}
	if got[0].ThumbURL != "https://pbs.twimg.test/media/PNGSRC.png?format=png&name=small" {
		t.Errorf("ThumbURL = %q, 期望 format=png（沿用來源副檔名）", got[0].ThumbURL)
	}
	if got[0].URL != "https://pbs.twimg.test/media/PNGSRC.png?format=png&name=large" {
		t.Errorf("URL = %q, 期望 format=png（沿用來源副檔名）", got[0].URL)
	}
	if strings.Contains(got[0].ThumbURL, "format=jpg") || strings.Contains(got[0].URL, "format=jpg") {
		t.Errorf("PNG 來源不得出現 format=jpg（P0' 實證 404）：thumb=%q url=%q", got[0].ThumbURL, got[0].URL)
	}
}

// TestMediaExtractionVideoHighestBitrate：variants 兩 mp4 一 m3u8 → URL 取
// bitrate 較高的 mp4（REQ-DAT-016），m3u8 不入選（D13：不採 HLS）；縮圖＝
// media_url_https?format=<ext>&name=small；Width/Height 留 0（aspect_ratio
// 無法換算實際值，寬高不強求）；節點無 media_url_https → ThumbURL 留空。
func TestMediaExtractionVideoHighestBitrate(t *testing.T) {
	data := tlData([]map[string]any{
		tlMediaEntry(
			"tweet-1", "1201", "video post",
			tlVideoNode(
				"video", "https://pbs.twimg.test/media/VIDPOST1.jpg",
				tlVariant("application/x-mpegURL", 2880000, "https://video.twimg.test/vid/M3U8A.m3u8"),
				tlVariant("video/mp4", 832000, "https://video.twimg.test/vid/MP4LOW.mp4"),
				tlVariant("video/mp4", 2176000, "https://video.twimg.test/vid/MP4HIGH.mp4"),
			),
			tlVideoNode(
				"video", "",
				tlVariant("video/mp4", 632000, "https://video.twimg.test/vid/MP4B.mp4"),
			),
		),
	})
	posts := parseFixturePosts(t, data)
	if len(posts) != 1 {
		t.Fatalf("應得 1 條貼文，得到 %d", len(posts))
	}
	got := posts[0].Media
	if len(got) != 2 {
		t.Fatalf("應得 2 個媒體項，得到 %d：%+v", len(got), got)
	}
	if got[0].Kind != "video" {
		t.Errorf("Kind = %q, 期望 \"video\"", got[0].Kind)
	}
	if got[0].URL != "https://video.twimg.test/vid/MP4HIGH.mp4" {
		t.Errorf("URL = %q, 期望 bitrate 較高的 mp4（REQ-DAT-016）", got[0].URL)
	}
	if strings.Contains(got[0].URL, "m3u8") {
		t.Errorf("m3u8 不得入選（D13）：URL = %q", got[0].URL)
	}
	if got[0].ThumbURL != "https://pbs.twimg.test/media/VIDPOST1.jpg?format=jpg&name=small" {
		t.Errorf("ThumbURL = %q, 期望 media_url_https?format=jpg&name=small", got[0].ThumbURL)
	}
	if got[0].Width != 0 || got[0].Height != 0 {
		t.Errorf("video 寬高不強求（aspect_ratio 無法換算實際值），得到 %d×%d", got[0].Width, got[0].Height)
	}
	if got[1].ThumbURL != "" {
		t.Errorf("節點無 media_url_https → ThumbURL 留空，得到 %q", got[1].ThumbURL)
	}
	if got[1].URL != "https://video.twimg.test/vid/MP4B.mp4" {
		t.Errorf("URL = %q, 期望僅存的 mp4 variant", got[1].URL)
	}
}

// TestMediaExtractionGifUsesMP4：animated_gif → Kind=="gif"（D13），URL 為
// mp4 variant（不採 m3u8）。
func TestMediaExtractionGifUsesMP4(t *testing.T) {
	// D24 修訂：gif 優先取 image/gif variant（NewGIFDecoder 需 GIF 本體）；
	// m3u8 恆不入選。
	data := tlData([]map[string]any{
		tlMediaEntry(
			"tweet-1", "1301", "gif post",
			tlVideoNode(
				"animated_gif", "https://pbs.twimg.test/media/GIFPOST1.jpg",
				tlVariant("application/x-mpegURL", 1280000, "https://video.twimg.test/vid/GIFA.m3u8"),
				tlVariant("video/mp4", 960000, "https://video.twimg.test/vid/GIFB.mp4"),
				tlVariant("image/gif", 0, "https://video.twimg.test/vid/GIFC.gif"),
			),
		),
	})
	posts := parseFixturePosts(t, data)
	if len(posts) != 1 {
		t.Fatalf("應得 1 條貼文，得到 %d", len(posts))
	}
	got := posts[0].Media
	if len(got) != 1 {
		t.Fatalf("應得 1 個媒體項，得到 %d：%+v", len(got), got)
	}
	if got[0].Kind != "gif" {
		t.Errorf("Kind = %q, 期望 \"gif\"（animated_gif → gif）", got[0].Kind)
	}
	if got[0].URL != "https://video.twimg.test/vid/GIFC.gif" {
		t.Errorf("URL = %q, 期望 image/gif variant（D24：GIF 走純 Go 解碼）", got[0].URL)
	}
}

// TestMediaExtractionGifFallsBackToMP4（D24 退路）：variants 缺 image/gif
// 時退回最高位元率 mp4（m3u8 仍不入選）。
func TestMediaExtractionGifFallsBackToMP4(t *testing.T) {
	data := tlData([]map[string]any{
		tlMediaEntry(
			"tweet-2", "1302", "gif post no gif variant",
			tlVideoNode(
				"animated_gif", "https://pbs.twimg.test/media/GIFPOST2.jpg",
				tlVariant("application/x-mpegURL", 1280000, "https://video.twimg.test/vid/GIFA.m3u8"),
				tlVariant("video/mp4", 960000, "https://video.twimg.test/vid/GIFB.mp4"),
			),
		),
	})
	posts := parseFixturePosts(t, data)
	if len(posts) != 1 {
		t.Fatalf("應得 1 條貼文，得到 %d", len(posts))
	}
	got := posts[0].Media
	if len(got) != 1 {
		t.Fatalf("應得 1 個媒體項，得到 %d：%+v", len(got), got)
	}
	if got[0].URL != "https://video.twimg.test/vid/GIFB.mp4" {
		t.Errorf("URL = %q, 期望退回 mp4 variant（無 gif variant 時）", got[0].URL)
	}
}

// TestNoEntitiesYieldsNilMedia：缺 extended_entities 的帖 → Media 為 nil；
// extended_entities.media 為空陣列 → 同為 nil（非 nil 零長度不允許）。
func TestNoEntitiesYieldsNilMedia(t *testing.T) {
	empty := tlResult("1402", "empty media")
	empty["extended_entities"] = map[string]any{"media": []any{}}
	data := tlData([]map[string]any{
		tlTweetEntry("tweet-1", "1401", "no media"),
		tlEntry("tweet-2", tlItem(empty)),
	})
	posts := parseFixturePosts(t, data)
	if len(posts) != 2 {
		t.Fatalf("應得 2 條貼文，得到 %d", len(posts))
	}
	if posts[0].Media != nil {
		t.Errorf("無 extended_entities → Media 應為 nil，得到 %+v", posts[0].Media)
	}
	if posts[1].Media != nil {
		t.Errorf("media 空陣列 → Media 應為 nil（非 nil 零長度不允許），得到 %+v", posts[1].Media)
	}
}

// TestUnknownTypeSkipped：type 非 photo/video/animated_gif 的項目跳過
// （設計 §8.2 Kind 判定）；全為未知 type 的帖 → Media nil（非 nil 必 len>0）。
func TestUnknownTypeSkipped(t *testing.T) {
	unknownOnly := tlResult("1502", "unknown only")
	unknownOnly["extended_entities"] = map[string]any{"media": []any{
		map[string]any{"type": "xyz", "media_url_https": "https://pbs.twimg.test/media/XYZ1.jpg"},
	}}
	data := tlData([]map[string]any{
		tlMediaEntry(
			"tweet-1", "1501", "mixed media",
			tlVideoNode(
				"xyz", "https://pbs.twimg.test/media/BADPOST.jpg",
				tlVariant("video/mp4", 1000, "https://video.twimg.test/vid/BAD.mp4"),
			),
			tlPhotoNode("https://pbs.twimg.test/media/KEPT1.jpg", 10, 20, 300, 400),
		),
		tlEntry("tweet-2", tlItem(unknownOnly)),
	})
	posts := parseFixturePosts(t, data)
	if len(posts) != 2 {
		t.Fatalf("應得 2 條貼文，得到 %d", len(posts))
	}
	want := []MediaItem{{
		Kind:     "photo",
		ThumbURL: "https://pbs.twimg.test/media/KEPT1.jpg?format=jpg&name=small",
		URL:      "https://pbs.twimg.test/media/KEPT1.jpg?format=jpg&name=large",
		Width:    300,
		Height:   400,
	}}
	if !reflect.DeepEqual(posts[0].Media, want) {
		t.Errorf("type xyz 應跳過、photo 照收：got %+v, want %+v", posts[0].Media, want)
	}
	if posts[1].Media != nil {
		t.Errorf("全為未知 type → Media 應為 nil（非 nil 必 len>0），得到 %+v", posts[1].Media)
	}
}
