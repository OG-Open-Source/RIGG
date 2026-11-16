//go:build snapshot

// snapshot_main：headless 渲染各 view 狀態成 PNG（UI/UX 感知閉環）——
// 需求方回報「你沒法像 HTML 一樣感知嗎」的答案：MyGO Tester 可離線出圖
// （shadcn-ui-mygo make snapshots 同工作流），代理直接讀 PNG 迭代視覺。
// 建置：go build -tags snapshot -o snapshot.exe .
// 執行：snapshot.exe [輸出目錄]（預設 artifacts/ui）。
package main

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

// gradient 造合成圖片（展開檢視大圖用；非真 CDN 內容）。
func gradient(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	return img
}

// gradientPNG 回 PNG 編碼的合成漸層（DecodeImage 的合法輸入）。
func gradientPNG(w, h int) []byte {
	var b []byte
	if img := gradient(w, h); img != nil {
		if enc := png.Encode(&pngWriter{&b}, img); enc != nil {
			_ = enc
		}
	}
	return b
}

// pngWriter 是 io.Writer 最小适配（png.Encode 目標）。
type pngWriter struct{ b *[]byte }

func (w *pngWriter) Write(p []byte) (int, error) {
	*w.b = append(*w.b, p...)
	return len(p), nil
}

// snapSession 是快照用最小會話：狀態由快照端直接種入，會話僅滿足介面。
type snapSession struct{}

func (s *snapSession) Validate(cookie string) (viewerInfo, error) {
	return viewerInfo{UserID: "snap1", ScreenName: "LOVE2ufby7"}, nil
}

func (s *snapSession) Home(cursor string, seen []string) (*timelineResult, error) {
	return &timelineResult{}, nil
}

func (s *snapSession) HomeLatest(cursor string, seen []string) (*timelineResult, error) {
	return &timelineResult{}, nil
}

func (s *snapSession) Search(query, product, cursor string) (*timelineResult, error) {
	return &timelineResult{}, nil
}

func (s *snapSession) Close() error { return nil }

func (a *app) syncAll() {
	a.apply = func(fn func()) { fn() }
	a.spawn = func(fn func(ctx context.Context)) { fn(context.Background()) }
	a.onLoginPersist = func(sess xSession, info viewerInfo, cookieLine string) error {
		return nil
	}
	a.newSess = func() (xSession, error) { return &snapSession{}, nil }
}

func (a *app) seed() {
	a.activeID = "snap1"
	a.accounts = []Account{
		{UserID: "snap1", ScreenName: "LOVE2ufby7", Name: "Love"},
		{UserID: "snap2", ScreenName: "second_acc", Name: "Second"},
		{UserID: "snap3", ScreenName: "third_acc", Name: "Third"},
	}
	a.syncAccountSelection()
	// 兩槽合成貼文（含長文與作者輪替）。
	for _, kind := range []int{feedForYou, feedFollowing} {
		f := a.feeds[kind]
		for n := 0; n < 8; n++ {
			p := post{
				WorkID:     string(rune('a'+kind)) + string(rune('1'+n)),
				WorkURL:    "https://x.com/u/status/" + string(rune('a'+kind)) + string(rune('1'+n)),
				ScreenName: []string{"LOVE2ufby7", "second_acc", "third_acc"}[n%3],
				Title:      "這是合成貼文標題 " + string(rune('1'+n)) + "——包含中日文與 a long English tail to exercise wrapping。",
			}
			f.posts = append(f.posts, p)
			f.seen = append(f.seen, p.WorkID)
		}
		a.feeds[kind] = f
	}
}

func (a *app) shot(t *ui.Tester, dir, name string) {
	img := t.Image()
	if img == nil {
		return
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		return
	}
	defer f.Close()
	_ = png.Encode(f, img)
}

func snapMain() {
	dir := "artifacts/ui"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	_ = os.MkdirAll(dir, 0o755)

	a := newApp()
	a.syncAll()
	a.seed()
	a.sess = &snapSession{}
	a.activeID = "snap1"

	// 合成大圖（展開檢視用；一次解碼、多狀態共用）。
	src, err := ui.DecodeImage(gradientPNG(900, 600))
	if err != nil {
		return
	}

	type size struct {
		name string
		w, h int
	}
	sizes := []size{{"n480", 480, 720}, {"n900", 900, 720}, {"w1280", 1280, 800}, {"f1920", 1920, 1080}}

	for _, dark := range []bool{false, true} {
		tag := "light"
		if dark {
			tag = "dark"
		}
		// 狀態復位（前輪洩漏——v1.1.5 快照實證：dark home 繼承 light 的
		// 搜尋檢視）：搜尋、展開檢視、tab 逐一歸位。
		a.closeSearch()
		a.closeDetail()
		a.activeFeed = feedForYou
		// login。
		a.sess = nil
		a.loginMode = loginModeNone
		for _, s := range sizes {
			t := ui.NewTester(a.view, s.w, s.h)
			t.SetDark(dark)
			t.Frame()
			a.shot(t, dir, "login-"+tag+"-"+s.name)
		}
		// home（已登入；feeds 已種入）。
		a.sess = &snapSession{}
		a.loginMode = loginModeNone
		for _, s := range sizes {
			t := ui.NewTester(a.view, s.w, s.h)
			t.SetDark(dark)
			t.Frame()
			t.Frame()
			a.shot(t, dir, "home-"+tag+"-"+s.name)
		}
		// 展開檢視（photo；1280 右欄＋900 全寬）——直接種 dmedia.img。
		p := a.feeds[feedForYou].posts[1]
		p.Media = []mediaItem{{Kind: "photo", ThumbURL: "", URL: "https://pbs.twimg.test/x", Width: 900, Height: 600}}
		a.detail = &p
		a.dmedia = detailMedia{img: src}
		for _, s := range []size{{"n900", 900, 720}, {"w1280", 1280, 800}} {
			t := ui.NewTester(a.view, s.w, s.h)
			t.SetDark(dark)
			t.Frame()
			t.Frame()
			a.shot(t, dir, "detail-"+tag+"-"+s.name)
		}
		a.closeDetail()
		// 搜尋結果。
		a.submitSearch("合成")
		for _, s := range sizes {
			t := ui.NewTester(a.view, s.w, s.h)
			t.SetDark(dark)
			t.Frame()
			t.Frame()
			a.shot(t, dir, "search-"+tag+"-"+s.name)
		}
		// 移除確認 Modal。
		a.removeUserID = "snap1"
		a.removeOpen = true
		t := ui.NewTester(a.view, 1280, 800)
		t.SetDark(dark)
		t.Frame()
		a.shot(t, dir, "modal-"+tag+"-w1280")
		a.removeOpen = false
	}
}

func main() { snapMain() }
