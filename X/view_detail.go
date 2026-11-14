// view_detail：展開檢視＋影片播放生命週期（v1.1 設計 §8.6；Task 13）。
//
// 單一活躍播放器：開新帖前先 Close 舊的（openDetail）；離開展開檢視
//（收合按鈕／切換帳號墓碑）經 closeDetail 釋放。列表虛擬化對列表縮圖
// 天然停播（ui.Video 的「view 建構才前進」語義），但展開頁的關閉語義
// 是顯式 Close——播放器不隨檢視消失自動釋放。
//
// Kind 分派：photo → 大圖（fetchImage＋DecodeImage 直取 name=large URL，
// 不走縮圖 LRU——單張、生命週期與展開頁同生共死）；video/gif →
// openPlayer（mp4）＋ui.Video（Ready 前呈媒體縮圖）。媒體 403/404 不落
// 引擎六 kind（app 層語義，Alternative 11 退路）——失敗呈 msgMedia＋外開
// WorkURL 連結。
//
// 宿主形態：≥1024 右欄並列（homeView 分流）；<1024 的 Router 頁形態屬
// Task 14 佈局接線（detailView 建構函式為其重用面）。
package main

import (
	"context"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
)

// player 是播放器生命週期的測試 seam 最小面（*ui.VideoPlayer 滿足之）：
// 單一活躍規則只需 Close；渲染處以型別斷言取回 *ui.VideoPlayer 交
// ui.Video（測試替身落空即走 Ready 前縮圖分支）。
type player interface{ Close() }

// openPlayer 是播放器建構接縫：正式＝video.OpenURL（mp4 URL 背景載入；
// ErrUnsupported（Linux 無 GStreamer）等錯誤一律落媒體失敗 UX）；測試注
// 入記錄替身。包級函式值形（同 fetchImage／newStore：call-time 解析、
// t.Cleanup 還原）。
var openPlayer = func(url string) (player, error) { return video.OpenURL(url) }

// detailPanelWidth 是右欄並列形態的展開檢視欄寬（DIP；Task 14 的
// layoutBand 佈線可再調）。
const detailPanelWidth = float32(380)

// detailMinWidth 是展開檢視並列（右欄）形態的窗寬下限（DIP）：<1024 為
// 窄窗——列點擊不開帖、殘留 detail 於窄幀即時收合（兩處同一閘，見
// homeView 與 homeFeedView 列點擊）：<1024 不渲染展開檢視，收合按鈕
//（只在 detailView）不存在，開帖或殘留將使活躍播放器無任何收合路徑。
const detailMinWidth = float32(1024)

// detailMediaHeight 是展開檢視媒體區的固定高（DIP）：圖以 Fit(Contain)
// 貼合、播放器未 Ready 時的縮圖／佔位同框，版面不隨載入跳動。
const detailMediaHeight = float32(260)

// detailCloseText／detailLoadingText 是展開檢視的收合按鈕與載入佔位文案。
const (
	detailCloseText   = "收合"
	detailLoadingText = "載入中…"
)

// detailMedia 是展開檢視的媒體載入狀態（openDetail 起手歸零，detailView
// 消費）：img 為 photo 大圖的 DecodeImage 產物（nil＝未載入）；imgFail
// 為下載／解碼失敗；playErr 為 openPlayer 失敗——兩者皆呈 msgMedia＋外開。
type detailMedia struct {
	img     ui.ImageSource
	imgFail bool
	playErr error
}

// openDetail 開啟貼文的展開檢視（UI 執行緒：列點擊或後續入口）。單一活
// 躍規則（§8.6）：先 closePlayer() 關舊播放器，再建新狀態。Kind 分派：
// photo 派發大圖下載（會話子樹；完成經 publish 主執行緒解碼，世代＋
// detail 指標雙檢查，T6 形）；video/gif 當場建播放器（openPlayer，mp4
// 背景載入——gif 亦走 mp4，D13；失敗記 playErr 走媒體失敗 UX，不含憑
// 證語義）。
func (a *app) openDetail(p post) {
	a.closePlayer()
	a.dmedia = detailMedia{}
	a.detail = &p
	if len(p.Media) == 0 {
		return
	}
	m := p.Media[0]
	switch m.Kind {
	case "photo":
		a.fetchDetailImage(m.URL)
	case "video", "gif":
		pl, err := openPlayer(m.URL)
		if err != nil {
			a.dmedia.playErr = err
			return
		}
		a.player = pl
	}
}

// closeDetail 離開展開檢視（收合按鈕／切換帳號墓碑）：Close＋nil＋媒體
// 狀態歸零（§8.6）。冪等：無播放器／detail 時不做事。
func (a *app) closeDetail() {
	a.closePlayer()
	a.detail = nil
	a.dmedia = detailMedia{}
}

// closePlayer 釋放單一活躍播放器（§8.6）：nil 時不做事（冪等）。
func (a *app) closePlayer() {
	if a.player == nil {
		return
	}
	a.player.Close()
	a.player = nil
}

// fetchDetailImage 派發展開檢視大圖下載（fetchImage 接縫直取 name=large
// URL——媒體 CDN 獨立客戶端直載、不經引擎 transport，D15/D22 語義沿
// media.go；不走縮圖 LRU）：起跑前捕捉世代與 detail 指標，完成經 publish
// 回主執行緒比對後解碼（ui.DecodeImage 於主執行緒——Bitmap 建構的執行
// 執行緒約束未經查證，主執行緒是安全面，同 finishThumb）並寫 dmedia.img；
// 下載或解碼失敗記 imgFail（呈失敗 UX，不自動重試）。切換帳號的墓碑
//（closeDetail 歸零 detail）與換帖（detail 指標更替）使過期結果整批丟棄。
func (a *app) fetchDetailImage(url string) {
	d, gen := a.detail, a.gen
	a.goSession(func(ctx context.Context) {
		data, err := fetchImage(ctx, url)
		a.publish(func() {
			if gen != a.gen || a.detail != d {
				return // 世代過期或展開檢視已收合／換帖：丟棄
			}
			if err != nil {
				a.dmedia.imgFail = true
				return
			}
			src, derr := ui.DecodeImage(data)
			if derr != nil {
				a.dmedia.imgFail = true
				return
			}
			a.dmedia.img = src
		})
	})
}

// detailView 建構展開檢視（右欄形態本體；Task 14 將它掛進 <1024 的
// Router 頁）：標題列（作者＋收合）、貼文文字、WorkURL 與媒體區（Kind
// 分派見 detailMediaView）。顯示欄位經 Sanitize 後上屏（view 層契約）。
// 收合點擊觸發的 closeDetail 歸零 player／dmedia：MyGO 對建構中處理的
// 輸入於同幀重建（consumed → 下一 pass），呈現幀不殘留關閉後狀態。
func (a *app) detailView(c *ui.Context, p post) {
	t := c.Theme()
	ui.Column(c).Padding(12, 16).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, Sanitize("@"+p.ScreenName)).TextColor(t.TextMuted).Grow(1)
			if ui.Button(c, detailCloseText).Clicked() {
				a.closeDetail() // 收合：顯式 Close（§8.6 離開展開檢視）
			}
		})
		if p.Title != "" {
			ui.Text(c, Sanitize(p.Title))
		}
		if p.WorkURL != "" {
			ui.Link(c, "開啟貼文", Sanitize(p.WorkURL))
		}
		a.detailMediaView(c, p)
	})
}

// detailMediaView 是展開檢視的媒體區（Kind 分派，§8.6）：photo → 大圖
//（ui.Image；未載入 → 佔位、失敗 → msgMedia＋外開）；video/gif →
// ui.Video（openPlayer 失敗 → 失敗 UX；Ready 前呈媒體縮圖——thumbLRU
// 命中即縮圖、未命中佔位，不派發縮圖下載：展開頁的載入對象是大圖／播
// 放器，縮圖管線屬列表框，見 media.go）。
func (a *app) detailMediaView(c *ui.Context, p post) {
	if len(p.Media) == 0 {
		return
	}
	m := p.Media[0]
	switch m.Kind {
	case "photo":
		switch {
		case a.dmedia.img != nil:
			a.detailMediaBox(c, func() {
				ui.Image(c, a.dmedia.img).Fill().Fit(ui.Contain)
			})
		case a.dmedia.imgFail:
			a.mediaFailView(c, p)
		default:
			a.detailPlaceholder(c)
		}
	case "video", "gif":
		if a.dmedia.playErr != nil || a.player == nil {
			a.mediaFailView(c, p)
			return
		}
		if vp, ok := a.player.(*ui.VideoPlayer); ok && vp.Ready() {
			a.detailMediaBox(c, func() {
				ui.Video(c, vp).Fill().Fit(ui.Contain)
			})
			return
		}
		// Ready 前：媒體縮圖（thumbLRU 命中；未命中佔位）。
		if m.ThumbURL != "" {
			if src, ok := thumbLRU.get(m.ThumbURL); ok {
				a.detailMediaBox(c, func() {
					ui.Image(c, src).Fill().Fit(ui.Contain)
				})
				return
			}
		}
		a.detailPlaceholder(c)
	}
}

// detailMediaBox 是媒體區的固定高容器（背景色塊；內容 Fit(Contain) 貼合）。
func (a *app) detailMediaBox(c *ui.Context, content func()) {
	t := c.Theme()
	ui.Column(c).Fill().Height(detailMediaHeight).Background(t.Surface).Children(content)
}

// detailPlaceholder 是媒體載入中的佔位（主題色塊＋載入文字）。
func (a *app) detailPlaceholder(c *ui.Context) {
	t := c.Theme()
	a.detailMediaBox(c, func() {
		ui.Text(c, detailLoadingText).TextColor(t.TextMuted)
	})
}

// mediaFailView 是媒體失敗 UX（msgMedia＋外開 WorkURL）：媒體 403/404 不
// 落引擎六 kind（app 層語義）——固定文案，不拼接錯誤原始訊息（REQ-INV-01
// 同形：無憑證材料可能）。
func (a *app) mediaFailView(c *ui.Context, p post) {
	t := c.Theme()
	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, msgMedia).TextColor(t.Danger)
		if p.WorkURL != "" {
			ui.Link(c, msgOpenExternal, Sanitize(p.WorkURL))
		}
	})
}
