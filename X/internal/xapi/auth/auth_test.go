package auth

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// ct0HexRE 為 REQ-AUTH-005 的「32 位小寫 hex」斷言。
var ct0HexRE = regexp.MustCompile(`^[0-9a-f]{32}$`)

// parseHeaderLine 把 CookieHeader() 輸出（k=v; k=v）解析回映射，僅供測試
// 斷言使用。
func parseHeaderLine(line string) map[string]string {
	m := make(map[string]string)
	if line == "" {
		return m
	}
	for _, seg := range strings.Split(line, "; ") {
		if k, v, ok := strings.Cut(seg, "="); ok {
			m[k] = v
		}
	}
	return m
}

// TestPrepareAuth_Parse_REQ_AUTH_003 解析矩陣：以 ; 分段、trim 空白、空段
// 忽略、結尾分號容忍，且 k=v 以第一個 = 切分（值可含 =）。（REQ-AUTH-003）
// want 中 ct0 以 "GEN" 標記「期望本地產生的 32 位小寫 hex」（輸入無 ct0 且
// 無 auth_token，REQ-AUTH-005）；lang 缺失一律補 zh-CN（REQ-AUTH-006）。
func TestPrepareAuth_Parse_REQ_AUTH_003(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want map[string]string
	}{
		{"basic_trailing_semicolon", "a=1; b=2;",
			map[string]string{"a": "1", "b": "2", "ct0": "GEN", "lang": "zh-CN"}},
		{"extra_whitespace", "  a=1 ;  b=2  ",
			map[string]string{"a": "1", "b": "2", "ct0": "GEN", "lang": "zh-CN"}},
		{"empty_segments", "a=1;;b=2",
			map[string]string{"a": "1", "b": "2", "ct0": "GEN", "lang": "zh-CN"}},
		{"value_contains_equals", "twid=u%3D123",
			map[string]string{"twid": "u%3D123", "ct0": "GEN", "lang": "zh-CN"}},
		{"first_equals_split", "k=a=b",
			map[string]string{"k": "a=b", "ct0": "GEN", "lang": "zh-CN"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := NewAuth("bearer-test")
			if err := a.PrepareAuth(tc.in); err != nil {
				t.Fatalf("PrepareAuth(%q) error = %v, 期望 nil", tc.in, err)
			}
			got := a.snap.Load().cookies
			if len(got) != len(tc.want) {
				t.Fatalf("PrepareAuth(%q) cookie 鍵數 = %d（%v）, 期望 %d", tc.in, len(got), got, len(tc.want))
			}
			for k, want := range tc.want {
				gotV, ok := got[k]
				if !ok {
					t.Fatalf("PrepareAuth(%q) 缺鍵 %q；got=%v", tc.in, k, got)
				}
				if want == "GEN" {
					if !ct0HexRE.MatchString(gotV) {
						t.Fatalf("PrepareAuth(%q) ct0 = %q, 期望 32 位小寫 hex", tc.in, gotV)
					}
					continue
				}
				if gotV != want {
					t.Fatalf("PrepareAuth(%q) cookie[%q] = %q, 期望 %q", tc.in, k, gotV, want)
				}
			}
		})
	}
}

// TestPrepareAuth_MissingCt0_REQ_AUTH_004 有 auth_token 而無 ct0 → 錯誤，
// 且 errors.As 可判別 *xerrors.InvalidInputError（ct0 由服務端簽發，不能
// 本地生成）。（REQ-AUTH-004）
func TestPrepareAuth_MissingCt0_REQ_AUTH_004(t *testing.T) {
	a := NewAuth("bearer-test")
	err := a.PrepareAuth("auth_token=xyz")
	if err == nil {
		t.Fatal(`PrepareAuth("auth_token=xyz") error = nil, 期望錯誤`)
	}
	var inv *xerrors.InvalidInputError
	if !errors.As(err, &inv) {
		t.Fatalf("PrepareAuth error = %T(%v), 期望 errors.As 命中 *xerrors.InvalidInputError", err, err)
	}
}

// TestPrepareAuth_GuestCt0_REQ_AUTH_005 兩者皆無 → 本地產生 32 位小寫
// hex ct0。（REQ-AUTH-005）
func TestPrepareAuth_GuestCt0_REQ_AUTH_005(t *testing.T) {
	a := NewAuth("bearer-test")
	if err := a.PrepareAuth("foo=bar"); err != nil {
		t.Fatalf("PrepareAuth error = %v, 期望 nil", err)
	}
	if got := a.CSRFToken(); !ct0HexRE.MatchString(got) {
		t.Fatalf("CSRFToken() = %q, 期望 regex ^[0-9a-f]{32}$", got)
	}
}

// TestPrepareAuth_LangDefault_REQ_AUTH_006 無 lang → 補 lang=zh-CN。
// （REQ-AUTH-006）
func TestPrepareAuth_LangDefault_REQ_AUTH_006(t *testing.T) {
	a := NewAuth("bearer-test")
	if err := a.PrepareAuth("foo=bar"); err != nil {
		t.Fatalf("PrepareAuth error = %v, 期望 nil", err)
	}
	if got := a.CookieHeader(); !strings.Contains(got, "lang=zh-CN") {
		t.Fatalf("CookieHeader() = %q, 期望含 lang=zh-CN", got)
	}
}

// TestUpdateCookies_Merge_REQ_AUTH_014 合併非空值、不刪既有鍵、空值鍵
// 不新增。（REQ-AUTH-014）
func TestUpdateCookies_Merge_REQ_AUTH_014(t *testing.T) {
	a := NewAuth("bearer-test")
	if err := a.PrepareAuth("a=1; ct0=x"); err != nil {
		t.Fatalf("PrepareAuth error = %v, 期望 nil", err)
	}
	a.UpdateCookies(map[string]string{"b": "2", "ct0": "y", "drop": ""})
	header := a.CookieHeader()
	got := parseHeaderLine(header)
	if got["a"] != "1" || got["b"] != "2" || got["ct0"] != "y" {
		t.Fatalf("CookieHeader() = %q, 期望合併後 a=1,b=2,ct0=y", header)
	}
	if _, exists := got["drop"]; exists {
		t.Fatalf("CookieHeader() = %q, 期望空值鍵 drop 不出現", header)
	}
	if strings.Contains(header, "drop") {
		t.Fatalf("CookieHeader() = %q, 期望無 drop", header)
	}
}

// TestIsLoggedIn_REQ_AUTH_007 等價於 auth_token 非空：有／無兩態。
// （REQ-AUTH-007）
func TestIsLoggedIn_REQ_AUTH_007(t *testing.T) {
	loggedIn := NewAuth("bearer-test")
	if err := loggedIn.PrepareAuth("auth_token=abc; ct0=x"); err != nil {
		t.Fatalf("PrepareAuth error = %v, 期望 nil", err)
	}
	if !loggedIn.IsLoggedIn() {
		t.Fatal("IsLoggedIn() = false, 期望 true（auth_token 非空）")
	}
	guest := NewAuth("bearer-test")
	if err := guest.PrepareAuth("ct0=x"); err != nil {
		t.Fatalf("PrepareAuth error = %v, 期望 nil", err)
	}
	if guest.IsLoggedIn() {
		t.Fatal("IsLoggedIn() = true, 期望 false（無 auth_token）")
	}
}

// TestCookieHeader_CSRF_Exact_REQ_HDR_008 CookieHeader() 內的 ct0= 值與
// CSRFToken() 逐字相等（同代數保證）。（REQ-HDR-008）
func TestCookieHeader_CSRF_Exact_REQ_HDR_008(t *testing.T) {
	a := NewAuth("bearer-test")
	if err := a.PrepareAuth("ct0=abc123def; foo=bar"); err != nil {
		t.Fatalf("PrepareAuth error = %v, 期望 nil", err)
	}
	ct0InHeader := parseHeaderLine(a.CookieHeader())["ct0"]
	if csrf := a.CSRFToken(); csrf != ct0InHeader {
		t.Fatalf("CSRFToken() = %q, 與 CookieHeader 內 ct0 = %q 不一致（REQ-HDR-008）", csrf, ct0InHeader)
	}
}

// TestConcurrentSnapshot_Race 1 writer goroutine 連續 UpdateCookies 500 次
// ＋4 reader goroutine 連續讀 CookieHeader()/CSRFToken()/IsLoggedIn()：
// -race 下無報告，且讀者取得的（CSRFToken, CookieHeader 內 ct0）為同一
// snapshot 代數的一致對——writer 與單次讀取交錯造成的跨代對以重試取得
// 一致對（writer 有限次更新、結束後必收斂），writer 結束後斷言最終
// snapshot 為第 500 次合併結果且 header 與 CSRF 同代逐字一致。
func TestConcurrentSnapshot_Race(t *testing.T) {
	a := NewAuth("bearer-test")
	if err := a.PrepareAuth("auth_token=t; a=1; ct0=base"); err != nil {
		t.Fatalf("PrepareAuth error = %v, 期望 nil", err)
	}

	const iterations = 500
	writerDone := make(chan struct{})
	var writeWG sync.WaitGroup
	writeWG.Add(1)
	go func() {
		defer writeWG.Done()
		defer close(writerDone)
		for i := 0; i < iterations; i++ {
			// ct0 與 k 同次更新且值同源（ct0-NNN ↔ vNNN）：一致對內兩者必同代。
			a.UpdateCookies(map[string]string{
				"ct0": fmt.Sprintf("ct0-%03d", i),
				"k":   fmt.Sprintf("v%03d", i),
			})
		}
	}()

	var readWG sync.WaitGroup
	var consistent, retried atomic.Int64
	for range 4 {
		readWG.Add(1)
		go func() {
			defer readWG.Done()
			localRetry := 0
			for {
				select {
				case <-writerDone:
					return
				default:
				}
				csrf := a.CSRFToken()
				line := a.CookieHeader()
				_ = a.IsLoggedIn()
				parsed := parseHeaderLine(line)
				ct0InLine := parsed["ct0"]
				if ct0InLine != csrf {
					// 跨代交錯（兩次獨立原子讀取間恰逢發布）：重試取得同代對。
					localRetry++
					retried.Add(1)
					if localRetry > 1_000_000 {
						t.Errorf("讀者持續觀察到跨代不一致對：CSRFToken=%q, header ct0=%q", csrf, ct0InLine)
						return
					}
					continue
				}
				// 同代對：writer 世代（ct0-NNN）行內 ct0-NNN 與 k=vNNN 必同源
				// （snapshot 原子性證據）；初始 Prepare 世代（ct0=base）無 k。
				if rest, ok := strings.CutPrefix(ct0InLine, "ct0-"); ok {
					if wantK := "v" + rest; parsed["k"] != wantK {
						t.Errorf("同代對行內不一致：ct0=%q, k=%q, 期望 k=%q；line=%q", ct0InLine, parsed["k"], wantK, line)
						return
					}
				}
				consistent.Add(1)
				localRetry = 0
			}
		}()
	}
	writeWG.Wait()
	readWG.Wait()

	if consistent.Load() == 0 {
		t.Fatal("併發期間未取得任何同代一致讀數對")
	}

	// 靜止後：最終 snapshot 必為第 500 次合併結果（合併不刪既有鍵），
	// 且 CSRFToken 與 CookieHeader 內 ct0 同代逐字一致。
	wantCt0 := fmt.Sprintf("ct0-%03d", iterations-1)
	if got := a.CSRFToken(); got != wantCt0 {
		t.Fatalf("CSRFToken() = %q, 期望 %q", got, wantCt0)
	}
	header := a.CookieHeader()
	got := parseHeaderLine(header)
	if got["ct0"] != wantCt0 || got["k"] != fmt.Sprintf("v%03d", iterations-1) ||
		got["a"] != "1" || got["auth_token"] != "t" {
		t.Fatalf("CookieHeader() = %q, 期望最終合併結果 ct0=%s, k=v499, a=1, auth_token=t", header, wantCt0)
	}
	if got := a.CSRFToken(); parseHeaderLine(a.CookieHeader())["ct0"] != got {
		t.Fatalf("靜止後 CSRFToken() 與 CookieHeader 內 ct0 不一致")
	}
	t.Logf("同代一致讀數對 %d 次，跨代交錯重試 %d 次", consistent.Load(), retried.Load())
}

// TestSnapshotPublishCoherence_Race 白盒釘死發布層一致性（REQ-HDR-008
// 的結構保證）：與 TestConcurrentSnapshot_Race 同一併發設置（1 writer
// × 500 次 UpdateCookies，ct0 與 k 值同源），但讀者僅一個、且直讀內部
// 原子指標——單次 s := a.snap.Load() 即取得單一 snapshot，隨即斷言
// s.ct0 == s.cookies["ct0"]，且解析 s.line 所得映射與 s.cookies 逐對相
// 等（reflect.DeepEqual）。無重試、不容跨代：單一指標載入把「一次發布
// 的 line/ct0/map 同源」釘死——若實作把三者存於分離的原子（兩次獨立
// 載入、靠重試收斂的弱一致），本測試在 -race 下必翻。
func TestSnapshotPublishCoherence_Race(t *testing.T) {
	a := NewAuth("bearer-test")
	if err := a.PrepareAuth("auth_token=t; a=1; ct0=base"); err != nil {
		t.Fatalf("PrepareAuth error = %v, 期望 nil", err)
	}

	const iterations = 500
	writerDone := make(chan struct{})
	var writeWG sync.WaitGroup
	writeWG.Add(1)
	go func() {
		defer writeWG.Done()
		defer close(writerDone)
		for i := 0; i < iterations; i++ {
			a.UpdateCookies(map[string]string{
				"ct0": fmt.Sprintf("ct0-%03d", i),
				"k":   fmt.Sprintf("v%03d", i),
			})
		}
	}()

	var readWG sync.WaitGroup
	var loads atomic.Int64
	readWG.Add(1)
	go func() {
		defer readWG.Done()
		for {
			// 白盒：先載入、後檢查 writerDone——即使 writer 已結束，
			// 靜止的最終 snapshot 仍必須通過同一組斷言（至少載入一次）。
			s := a.snap.Load()
			loads.Add(1)
			if s.ct0 != s.cookies["ct0"] {
				t.Errorf("snapshot 內部不一致（gen %d）：s.ct0 = %q, s.cookies[ct0] = %q", s.gen, s.ct0, s.cookies["ct0"])
				return
			}
			parsed := parseHeaderLine(s.line)
			if !reflect.DeepEqual(parsed, s.cookies) {
				t.Errorf("s.line 與 s.cookies 不同源（gen %d）：line=%q 解析=%v, cookies=%v", s.gen, s.line, parsed, s.cookies)
				return
			}
			select {
			case <-writerDone:
				return
			default:
			}
		}
	}()
	writeWG.Wait()
	readWG.Wait()

	if n := loads.Load(); n == 0 {
		t.Fatal("讀者未完成任何載入")
	} else {
		t.Logf("單次指標載入＋斷言 %d 次（每次皆同一 snapshot 的 line/ct0/map）", n)
	}
}

// TestGuestTokenStub guest token 方法位：恆回空字串＋錯誤（v1 不實作）。
func TestGuestTokenStub(t *testing.T) {
	a := NewAuth("bearer-test")
	got, err := a.GuestToken(context.Background())
	if got != "" {
		t.Fatalf("GuestToken() = %q, 期望空字串", got)
	}
	if err == nil {
		t.Fatal("GuestToken() error = nil, 期望錯誤")
	}
	var inv *xerrors.InvalidInputError
	if !errors.As(err, &inv) {
		t.Fatalf("GuestToken error = %T(%v), 期望 errors.As 命中 *xerrors.InvalidInputError", err, err)
	}
}

// fakeStore 為 CookieStore 的測試替身（編譯期驗證介面契合）。
type fakeStore struct{ loadLine string }

var _ CookieStore = (*fakeStore)(nil)

func (f *fakeStore) Load(ctx context.Context) (string, error) { return f.loadLine, nil }
func (f *fakeStore) Save(ctx context.Context, cookieLine string) error {
	return nil
}

// TestCookieStore_Interface 以 fake store 驗 WithCookieStore／WithProxies
// 的選項接線（REQ-AUTH-002：建構接受 bearer 與 proxies）。v1 引擎不主動
// 呼叫 Save——write-through 屬 Task 9 的 Sender，此處僅驗接線。
func TestCookieStore_Interface(t *testing.T) {
	store := &fakeStore{loadLine: "a=1"}
	proxies := map[string]string{"p1": "http://127.0.0.1:7890"}
	a := NewAuth("bearer-test", WithCookieStore(store), WithProxies(proxies))
	if a.cfg.store != CookieStore(store) {
		t.Fatalf("cfg.store = %v, 期望接線 fakeStore", a.cfg.store)
	}
	if !reflect.DeepEqual(a.cfg.proxies, proxies) {
		t.Fatalf("cfg.proxies = %v, 期望 %v", a.cfg.proxies, proxies)
	}
	// 別名防護：呼叫端事後修改映射不得穿透已接線的選項。
	proxies["p1"] = "mutated"
	if a.cfg.proxies["p1"] != "http://127.0.0.1:7890" {
		t.Fatal("WithProxies 應複製映射，呼叫端修改不得穿透")
	}
	// REQ-AUTH-002：建構時接受 bearer——傳入值逐字回傳；空值採預設公開
	// web bearer。
	if got := a.Bearer(); got != "bearer-test" {
		t.Fatalf("Bearer() = %q, 期望建構時傳入的 %q", got, "bearer-test")
	}
	if got := NewAuth("").Bearer(); got != defaultBearer {
		t.Fatalf(`NewAuth("").Bearer() = %q, 期望預設公開 web bearer`, got)
	}
}
