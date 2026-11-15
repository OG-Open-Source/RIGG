// view_detail：展開檢視＋影片播放生命週期（v1.1 設計 §8.6；Task 13）。
//
// 單一活躍播放器：開新帖前先 Close 舊的（openDetail）；離開展開檢視
// （收合按鈕／切換帳號墓碑）經 closeDetail 釋放。列表虛擬化對列表縮圖
// 天然停播（ui.Video 的「view 建構才前進」語義），但展開頁的關閉語義
// 是顯式 Close——播放器不隨檢視消失自動釋放。
//
// Kind 分派：photo → 大圖（fetchImage＋DecodeImage 直取 name=large URL，
// 不走縮圖 LRU——單張、生命週期與展開頁同生共死）；video/gif →
// openPlayer（mp4）＋ui.Video（Ready 前呈媒體縮圖）。媒體 403/404 不落
// 引擎六 kind（app 層語義，Alternative 11 退路）——失敗呈 msgMedia＋外開
// WorkURL 連結。
//
// 宿主形態：band ≥2（≥1024）右欄並列（homeView 分流）；band <2 的窄帶
// 以全寬 detail 頁承接（final review F2 裁決 2026-11-15——§8.8 的
// ui.Router 頁形態不予實作、延後 v1.2，DECISIONS〈v1.1 收尾登錄〉；
// detailView 建構函式為兩形態共用面）。
package main

import (
	"context"
	"errors"
	"os"
	"runtime/debug"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
)

// player 是播放器生命週期的測試 seam 最小面（*ui.VideoPlayer 滿足之）：
// 單一活躍規則只需 Close；渲染處以型別斷言取回 *ui.VideoPlayer 交
// ui.Video（測試替身落空即走 Ready 前縮圖分支）。
type player interface{ Close() }

// openPlayer 是 video 播放器建構接縫（D24：正式＝video.Open(暫存檔路徑)
// ——暫存檔由 fetchMediaToFile 以自有 client＋瀏覽器 UA 先行下載；測試注
// 入記錄替身）。包級函式值形（同 fetchImage／newStore：call-time 解析、
// t.Cleanup 還原）。
var openPlayer = func(path string) (player, error) { return video.Open(path) }

// newGifPlayer 是 GIF 播放器建構接縫（D24：正式＝ui.NewGIFDecoder（純 Go
// 解碼）＋ui.NewVideoPlayer＋SetLoop——記憶體內、不依賴系統媒體框架）。
var newGifPlayer = func(data []byte) (player, error) {
	dec, err := ui.NewGIFDecoder(data)
	if err != nil {
		return nil, err
	}
	pl, err := ui.NewVideoPlayer(dec)
	if err != nil {
		return nil, err
	}
	pl.SetLoop(true)
	return pl, nil
}

// goClose 是播放器非同步關閉的執行接縫（P4：正式＝go f()——Media
// Foundation 收斂可能秒級，同步 Close 卡 UI；測試注入同步執行使 Close
// 計數斷言確定性）。
var goClose = func(f func()) { go f() }

// detailPanelWidth 是右欄並列形態的展開檢視欄寬（DIP；band ≥2 的右欄）。
const detailPanelWidth = float32(380)

// 展開檢視並列（右欄）形態的窗寬下限為 bpLaptop（=1024，Task 14 的佈局
// 帶界，原 Task 13 的 detailMinWidth 常數已併入）：band <2 為窄帶——列
// 點擊同樣開帖，但以全寬 detail 頁承接（final review F2 裁決，homeView
// 分流；feed 於展開期間不可達，收合按鈕即返回路徑，播放器生命週期隨
// detail 不受跨帶縮放影響）。

// detailMediaHeight 是展開檢視媒體區的固定高（DIP）：圖以 Fit(Contain)
// 貼合、播放器未 Ready 時的縮圖／佔位同框，版面不隨載入跳動。
const detailMediaHeight = float32(260)

// detailCloseText／detailLoadingText／videoPlayText 是展開檢視的收合按
// 鈕、載入佔位與 video 懶建立播放鈕文案（D24）。
const (
	detailCloseText   = "收合"
	detailLoadingText = "載入中…"
	videoPlayText     = "播放"
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
// 躍規則（§8.6）：先 closePlayer() 關舊播放器，再建新狀態。Kind 分派
// （D24 修訂，2026-11-15）：photo 派發大圖下載（會話子樹；完成後解碼，
// 世代＋detail 指標雙檢查，T6 形）；gif 自動下載 bytes 純 Go 解碼播放
// （NewGIFDecoder——mp4 variant 交系統框架在實機無法播放，D13 修訂）；
// video 懶建立——僅呈海報＋播放鈕，點擊才下載暫存檔並建播放器（系統框
// 架初始化是展開卡頓與幻影 RAM 的來源，P4；D24 的自有下載繞過 CDN 對
// 系統框架 UA 的拒絕）。失敗記 playErr 走媒體失敗 UX，不含憑證語義。
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
	case "gif":
		a.startGifPlayback(m.URL)
	case "video":
		// 懶建立：播放器在使用者點擊播放時才建（startVideoPlayback）。
	}
}

// closeDetail 離開展開檢視（收合按鈕／切換帳號墓碑）：Close＋nil＋媒體
// 狀態歸零（§8.6）。冪等：無播放器／detail 時不做事。P5（D26）：
// FreeOSMemory 回收 RSS 高水位（展開期間的大圖解碼與播放器緩衝）。
func (a *app) closeDetail() {
	a.closePlayer()
	a.detail = nil
	a.dmedia = detailMedia{}
	debug.FreeOSMemory()
}

// closePlayer 釋放單一活躍播放器（§8.6）：nil 時不做事（冪等）。P4
// （D26 前奏）：Close 移背景 goroutine——Media Foundation 收斂可能秒級，
// 同步呼叫卡 UI（需求方實測「展開到收合奇慢」的元兇之一）；暫存檔隨
// Close 刪除（D24：playerPath 的生命週期終點）。主執行緒僅做狀態替換。
func (a *app) closePlayer() {
	if a.player == nil {
		return
	}
	pl, path := a.player, a.playerPath
	a.player = nil
	a.playerPath = ""
	goClose(func() {
		pl.Close()
		if path != "" {
			_ = os.Remove(path)
		}
	})
}

// fetchDetailImage 派發展開檢視大圖下載（fetchImage 接縫直取 name=large
// URL——媒體 CDN 獨立客戶端直載、不經引擎 transport，D15/D22 語義沿
// media.go；不走縮圖 LRU）：起跑前捕捉世代與 detail 指標，完成經 publish
// 回主執行緒比對後解碼（ui.DecodeImage 於主執行緒——Bitmap 建構的執行
// 執行緒約束未經查證，主執行緒是安全面，同 finishThumb）並寫 dmedia.img；
// 下載或解碼失敗記 imgFail（呈失敗 UX，不自動重試）。切換帳號的墓碑
// （closeDetail 歸零 detail）與換帖（detail 指標更替）使過期結果整批丟棄。
func (a *app) fetchDetailImage(url string) {
	d, gen := a.detail, a.gen
	a.goSession(func(ctx context.Context) {
		data, err := fetchImage(ctx, url)
		var src ui.ImageSource
		var derr error
		if err == nil {
			// P4：解碼移出主執行緒（2048px 解碼數百 ms 會卡幀）——
			// Bitmap 為資料持有者非 Element，背景建構同 video.Open 的
			// 背景載入語義。
			src, derr = ui.DecodeImage(data)
		}
		a.publish(func() {
			if gen != a.gen || a.detail != d {
				return // 世代過期或展開檢視已收合／換帖：丟棄
			}
			if err != nil || derr != nil {
				a.dmedia.imgFail = true
				return
			}
			a.dmedia.img = src
			// D26 RAM 槓桿：大圖同縮圖——丟 CPU 像素留 GPU 副本（首幀
			// 繪製由保留 data 重上傳）。
			if b, ok := src.(*ui.Bitmap); ok {
				b.ReleasePixels()
			}
		})
	})
}

// startGifPlayback 是 GIF 的 D24 播放路徑：下載 bytes（自有 client＋瀏
// 覽器 UA，≤gifMaxBytes）→ ui.NewGIFDecoder（純 Go 解碼、記憶體內，不
// 依賴系統媒體框架）→ ui.NewVideoPlayer（SetLoop 循環）→ a.player。世
// 代＋detail 指標雙檢查同 fetchDetailImage。
func (a *app) startGifPlayback(url string) {
	d, gen := a.detail, a.gen
	a.playerLoading = true
	a.goSession(func(ctx context.Context) {
		data, err := fetchMediaBytes(ctx, url, gifMaxBytes)
		a.publish(func() {
			a.playerLoading = false
			if gen != a.gen || a.detail != d {
				return // 世代過期或展開檢視已收合／換帖：丟棄
			}
			if err != nil {
				a.dmedia.playErr = err
				return
			}
			// 播放器建構於主執行緒（P4 修訂）：MyGO 的視窗綁定物件不在
			// 主執行緒建構時 Ready/渲染事件不送達（需求方實測「還是一張
			// 圖片」的根因）。NewGIFDecoder 為純 Go 解碼、開銷小。
			pl, perr := newGifPlayer(data)
			if perr != nil {
				a.dmedia.playErr = perr
				return
			}
			a.player = pl
		})
	})
}

// startVideoPlayback 是 video 的 D24 播放路徑（懶建立——使用者點擊播放
// 才走）：下載 mp4 至暫存檔（自有 client＋瀏覽器 UA，串流不整讀）→
// video.Open(path) → a.player（路徑留存 playerPath，收合時隨 player 刪
// 除）。世代＋detail 指標雙檢查同 fetchDetailImage；過期結果的暫存檔
// 即刻刪除。
func (a *app) startVideoPlayback(url string) {
	d, gen := a.detail, a.gen
	a.playerLoading = true
	a.goSession(func(ctx context.Context) {
		path, err := fetchMediaToFile(ctx, url, videoMaxBytes)
		a.publish(func() {
			a.playerLoading = false
			if gen != a.gen || a.detail != d {
				if path != "" {
					_ = os.Remove(path)
				}
				return // 世代過期或展開檢視已收合／換帖：丟棄
			}
			if err != nil {
				a.dmedia.playErr = err
				return
			}
			// 播放器建構於主執行緒（P4 修訂，同 startGifPlayback）；
			// 開啟失敗即刪暫存檔。
			pl, perr := openPlayer(path)
			if perr != nil {
				_ = os.Remove(path)
				a.dmedia.playErr = perr
				return
			}
			a.player = pl
			a.playerPath = path
			// v1.1.3 Ready 逾時 fallback：8 秒未就緒且無 Err → 失敗 UX
			//（外開 fallback 可用），不再無限停在海报。
			vp, _ := pl.(*ui.VideoPlayer)
			a.goSession(func(ctx context.Context) {
				select {
				case <-ctx.Done():
					return
				case <-time.After(8 * time.Second):
				}
				a.publish(func() {
					if gen != a.gen || a.detail != d || a.player != pl {
						return
					}
					if vp != nil && !vp.Ready() && vp.Err() == nil {
						a.dmedia.playErr = errors.New("播放器 8 秒內無法就緒（下載或解碼停滯）")
					}
				})
			})
		})
	})
}

// detailView 建構展開檢視（band ≥2 右欄並列形態本體）：標題列（作者＋
// 收合）、貼文文字、WorkURL 與媒體區（Kind 分派見 detailMediaView）。
// 顯示欄位經 Sanitize 後上屏（view 層契約）。收合點擊觸發的 closeDetail
// 歸零 player／dmedia：MyGO 對建構中處理的輸入於同幀重建（consumed →
// 下一 pass），呈現幀不殘留關閉後狀態。
func (a *app) detailView(c *ui.Context, p post) {
	t := c.Theme()
	// v1.1.5：內容包原生 ui.Scroll（mygo-components 對照：捲動區域）——
	// 直式影片的媒體盒可達 994 DIP，超出視窗的內容先前被裁切（收合鈕
	// 不可達＝「卡住」）；Scroll 使溢出可捲。
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(12, 16).Gap(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				// 作者頭像（MyGO 內建 ui.Avatar——mygo-components 對照：原生
				// widget；name 播種、無圖走文字 fallback）。
				ui.Avatar(c, p.ScreenName, nil)
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
	})
}

// detailMediaView 是展開檢視的媒體區（Kind 分派，§8.6＋D24 狀態機）：
// photo → 大圖（ui.Image；未載入 → 佔位、失敗 → msgMedia＋外開）；video
// → 懶建立狀態機（海報＋播放鈕 → 下載暫存檔中 → ui.Video；D24）；gif →
// 自動播放（openDetail 已派發下載解碼；完成前佔位）。媒體縮圖沿 thumbLRU
// 命中呈現（不派發縮圖下載：展開頁的載入對象是大圖／播放器）。
func (a *app) detailMediaView(c *ui.Context, p post) {
	if len(p.Media) == 0 {
		return
	}
	m := p.Media[0]
	switch m.Kind {
	case "photo":
		switch {
		case a.dmedia.img != nil:
			// Lobe display.Image（mygo-components 對照：MyGO ui.Image 無
			// 預覽檢視器——真缺口）：圓角框＋點擊飛出預覽（滾輪縮放、拖
			// 曳平移、Escape 關閉）。自然比例（寬受限於容器、高隨比例）
			// ——不再固定高 letterbox。
			display.Image(c, a.dmedia.img, display.ImageProps{
				Contain: true,
				Alt:     Sanitize("@" + p.ScreenName + " 的貼文圖片"),
			})
		case a.dmedia.imgFail:
			a.mediaFailView(c, p)
		default:
			a.detailPlaceholder(c)
		}
	case "video":
		switch {
		case a.dmedia.playErr != nil:
			a.mediaFailDetail(c, a.dmedia.playErr.Error()) // v1.1.3 診斷浮面
		case a.player != nil:
			a.playerView(c, m)
		case a.playerLoading:
			a.detailPlaceholder(c) // 下載暫存檔中（D24）
		default:
			a.videoPosterView(c, m.ThumbURL, m.URL) // 懶建立：海報＋播放鈕
		}
	case "gif":
		switch {
		case a.dmedia.playErr != nil:
			a.mediaFailDetail(c, a.dmedia.playErr.Error())
		case a.player != nil:
			a.playerView(c, m)
		default:
			a.detailPlaceholder(c) // 自動下載／解碼中（openDetail 已派發，D24）
		}
	}
}

// playerView 渲染活躍播放器（v1.1.5 修訂）：**ui.Video 恆建構、每幀必
// 建**——系統引擎（Media Foundation）為渲染驅動：首幀送達需要 ui.Video
// 元素存在（「its first picture came, which builds a frame of the window
// that shows it」）。v1.1.3 以 Ready 閘門（元素不建 → 死鎖）、v1.1.4 以
// 海報分支（!Ready 時 ui.Image return，元素又不建）兩度重演死鎖——本版
// 起播放中狀態一律直建 ui.Video（本機檔首幀 <1 秒，黑框短暫可接受；開
// 帖前的海報＋播放鈕已提供視覺連續性）。Err → 失敗 UX 帶真實錯誤描述。
// 媒體盒高按影片比例自適應（v1.1.4：固定高對直式影片 letterbox）。
func (a *app) playerView(c *ui.Context, m mediaItem) {
	vp, ok := a.player.(*ui.VideoPlayer)
	if !ok {
		a.detailPlaceholder(c)
		return
	}
	if e := vp.Err(); e != nil {
		a.mediaFailDetail(c, "播放器解碼失敗："+e.Error())
		return
	}
	h := mediaBoxHeight(m.Width, m.Height)
	ui.Column(c).Fill().Height(h).Background(c.Theme().Surface).Children(func() {
		ui.Video(c, vp).Fill().Fit(ui.Contain)
	})
}

// mediaBoxHeight 以媒體比例估算展開檢視的媒體盒高（DIP）：基準寬 560
// （內容欄寬的中值），夾在 [160, 640]；無尺寸資訊回 detailMediaHeight。
func mediaBoxHeight(w, h int) float32 {
	if w <= 0 || h <= 0 {
		return detailMediaHeight
	}
	eh := 560 * float32(h) / float32(w)
	if eh < 160 {
		return 160
	}
	if eh > 640 {
		return 640
	}
	return eh
}

// videoPosterView 是 video 的懶建立海報（D24）：縮圖命中即呈、未命中佔
// 位；播放鈕疊於海報中央（可發現性——v1.1.2 的下方按鈕被解讀為「還是
// 一張圖片」）。點擊才 startVideoPlayback（下載暫存檔＋建播放器）。
func (a *app) videoPosterView(c *ui.Context, thumbURL, url string) {
	a.detailMediaBox(c, func() {
		shown := false
		if thumbURL != "" {
			if src, ok := thumbLRU.get(thumbURL); ok {
				ui.Image(c, src).Fill().Fit(ui.Contain)
				shown = true
			}
		}
		if !shown {
			t := c.Theme()
			ui.Text(c, detailLoadingText).TextColor(t.TextMuted)
		}
	})
	if ui.PrimaryButton(c, videoPlayText).Clicked() {
		a.startVideoPlayback(url)
	}
}

// detailMediaBox 是媒體區的固定高容器（背景色塊；內容 Fit(Contain) 貼合）
// ——detailPlaceholder 佔位用；媒體本體改走比例自適應高（mediaBoxHeight）。
func (a *app) detailMediaBox(c *ui.Context, content func()) {
	t := c.Theme()
	ui.Column(c).Fill().Height(detailMediaHeight).Background(t.Surface).Children(content)
}

// detailPlaceholder 是媒體載入中的佔位（主題色塊＋原生 Spinner＋載入文
// 字——mygo-components 對照：進行中的原生 widget）。
func (a *app) detailPlaceholder(c *ui.Context) {
	t := c.Theme()
	a.detailMediaBox(c, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c)
			ui.Text(c, detailLoadingText).TextColor(t.TextMuted)
		})
	})
}

// mediaFailView 是媒體失敗 UX（Lobe Alert——mygo-components 對照：MyGO
// 無橫幅 widget，真缺口；msgMedia＋外開 WorkURL）：媒體 403/404 不落引
// 擎六 kind（app 層語義）——固定文案，不拼接錯誤原始訊息（REQ-INV-01
// 同形：無憑證材料可能）。
func (a *app) mediaFailView(c *ui.Context, p post) {
	a.mediaFailDetail(c, "")
}

// mediaFailDetail 是帶診斷描述的媒體失敗 UX（v1.1.3 診斷浮面）：desc 非
// 空 → Alert Description 呈現真實錯誤（app 層錯誤不含憑證材料），使使
// 用者回報即可定位下載／解碼／系統框架哪一層。
func (a *app) mediaFailDetail(c *ui.Context, desc string) {
	t := c.Theme()
	p := post{}
	if a.detail != nil {
		p = *a.detail
	}
	_ = t
	feedback.Alert(c, msgMedia, feedback.AlertProps{
		Type:        feedback.Warning,
		Description: desc,
		Selectable:  true,
	})
	if p.WorkURL != "" {
		ui.Link(c, msgOpenExternal, Sanitize(p.WorkURL))
	}
}
