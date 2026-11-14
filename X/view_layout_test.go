// view_layout 測試（Task 14）：四帶佈局與帳號選單 UI——layoutBand 帶界
// （768/1024/1200 逐字釘測）、ui.Tester 三寬建構（側欄元素存在、展開檢
// 視的帶閘）、帳號選單（上限提示態、新增帳號流程、移除 Modal→登出）、
// 側欄帳號項（切換／失效重登入口）與搜尋的顯式返回入口（Task 10 UX 註
// 記：已選 tab 點擊不觸發 Changed）。接縫以 fakeSession／seam 注入；
// 夾具一律合成、不打真實網路／真實預設憑證庫 service。
package main

import (
	"context"
	"slices"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"
)

// fiveAccounts 造 maxAccounts（5）個合成帳號（上限測試夾具；非真實身分）。
func fiveAccounts() []Account {
	accs := make([]Account, 0, maxAccounts)
	for i := 1; i <= maxAccounts; i++ {
		uid := "910" + strconv.Itoa(i)
		accs = append(accs, Account{UserID: uid, ScreenName: "goat" + strconv.Itoa(i), Name: "合成帳號" + strconv.Itoa(i)})
	}
	return accs
}

// TestLayoutBands（brief 逐字）：帶界 767.9→0、768→1、1023.9→1、
// 1024→2、1199.9→2、1200→3。
func TestLayoutBands(t *testing.T) {
	a := newTestApp()
	for _, tc := range []struct {
		w    float32
		want int
	}{
		{767.9, 0},
		{768, 1},
		{1023.9, 1},
		{1024, 2},
		{1199.9, 2},
		{1200, 3},
	} {
		if got := a.layoutBand(tc.w); got != tc.want {
			t.Errorf("layoutBand(%v) = %d, want %d", tc.w, got, tc.want)
		}
	}
}

// TestTesterRendersThreeWidths：SetSize(768/1024/1200) 建構 app view 不
// panic；斷言方式為 Tester 的可檢查面——HasText（Texts() 子串比對）：
// 各帶皆應呈側欄元素（時間線 tabs 項「For You」/「Following」與帳號清單
// 「@goated」；band 1 即有側欄，≥1024 屬其子集）。另釘展開檢視的帶閘：
// 1024（band 2）右欄並列（收合按鈕存在）；768（band 1）不渲染且殘留
// detail 即幀收合（T13 語義經 band 閘延續）。
func TestTesterRendersThreeWidths(t *testing.T) {
	a := newTestApp()
	a.sess = &fakeSession{}
	a.accounts = []Account{{UserID: "9001", ScreenName: "goated", Name: "合成帳號"}}
	a.activeID = "9001"
	tt := ui.NewTester(a.view, 768, 720)
	for _, w := range []int{768, 1024, 1200} {
		tt.SetSize(w, 720)
		tt.Frame() // 各帶建構不 panic
		if !tt.HasText("For You") || !tt.HasText("Following") {
			t.Errorf("寬 %d 應呈側欄時間線 tabs，texts %q", w, tt.Texts())
		}
		if !tt.HasText("@goated") {
			t.Errorf("寬 %d 應呈側欄帳號清單，texts %q", w, tt.Texts())
		}
	}
	// ≥1024（band ≥2）：展開檢視右欄並列。
	p := post{WorkID: "d1", WorkURL: "https://x.com/u/status/d1", ScreenName: "u", Title: "無媒體帖d1"}
	tt.SetSize(1024, 720)
	a.openDetail(p)
	tt.Frame()
	if a.detail == nil || !tt.HasText(detailCloseText) {
		t.Fatalf("1024 應並列展開檢視（收合按鈕存在），detail=%v texts %q", a.detail, tt.Texts())
	}
	// 768（band 1）：不渲染展開檢視，殘留即幀收合。
	tt.SetSize(768, 720)
	tt.Frame()
	if a.detail != nil || tt.HasText(detailCloseText) {
		t.Errorf("768 不得渲染展開檢視（殘留即幀收合），detail=%v texts %q", a.detail, tt.Texts())
	}
}

// TestAccountMenuShowsFiveAccountsCap（brief 釘測）：5 帳號時「新增帳號」
// 動作呈提示態——選單內呈上限文案（Subtitle，彈出式選單渲染為內窗文字）
// 且項目禁用：ChooseMenuItem 對禁用項回錯、不進入新增帳號流程。
func TestAccountMenuShowsFiveAccountsCap(t *testing.T) {
	a := newTestApp()
	a.sess = &fakeSession{}
	a.accounts = fiveAccounts()
	a.activeID = a.accounts[0].UserID
	tt := ui.NewTester(a.view, 1280, 720)
	tt.Frame()
	// 開啟帳號選單：5 帳號全列＋各帳號的移除項。
	if err := tt.Click("帳號選單"); err != nil {
		t.Fatalf("開啟帳號選單: %v", err)
	}
	menu := tt.Menu()
	for _, acc := range a.accounts {
		if !slices.Contains(menu, "@"+acc.ScreenName) {
			t.Errorf("選單應列帳號 %q，得 %q", "@"+acc.ScreenName, menu)
		}
		if !slices.Contains(menu, "移除 @"+acc.ScreenName) {
			t.Errorf("選單應有移除項 %q，得 %q", "移除 @"+acc.ScreenName, menu)
		}
	}
	// 提示態：上限文案呈現＋項目禁用（選擇被拒）。
	if !tt.HasText(accountCapText) {
		t.Errorf("上限時「新增帳號」應呈提示態文案 %q，texts %q", accountCapText, tt.Texts())
	}
	if err := tt.ChooseMenuItem("新增帳號"); err == nil {
		t.Error("上限時「新增帳號」應禁用（選擇不成立）")
	}
	if a.loginMode != loginModeNone {
		t.Errorf("上限時點「新增帳號」不得進入新增流程，loginMode=%d", a.loginMode)
	}
	tt.CloseMenu()
}

// TestAccountMenuAddUnderCapEntersAddFlow：未達上限時「新增帳號」可選
// ——進入 login view 的新增帳號變體（loginModeAdd）；「返回」復位回 home。
func TestAccountMenuAddUnderCapEntersAddFlow(t *testing.T) {
	a := newTestApp()
	a.sess = &fakeSession{}
	a.accounts = []Account{{UserID: "9201", ScreenName: "goated"}}
	a.activeID = "9201"
	tt := ui.NewTester(a.view, 1280, 720)
	tt.Frame()
	if err := tt.Click("帳號選單"); err != nil {
		t.Fatalf("開啟帳號選單: %v", err)
	}
	if err := tt.ChooseMenuItem("新增帳號"); err != nil {
		t.Fatalf("未達上限應可選「新增帳號」: %v", err)
	}
	if a.loginMode != loginModeAdd {
		t.Fatalf("選擇後應進入新增帳號流程，loginMode=%d", a.loginMode)
	}
	if !tt.HasText("新增帳號") || tt.HasText("時間線") {
		t.Errorf("應呈 login view 的新增帳號變體（home 收起），texts %q", tt.Texts())
	}
	// 返回：復位回 home（首次登入形）。
	if err := tt.Click("返回"); err != nil {
		t.Fatalf("點返回: %v", err)
	}
	if a.loginMode != loginModeNone {
		t.Errorf("返回應復位 loginMode，得 %d", a.loginMode)
	}
	if !tt.HasText("時間線") {
		t.Errorf("返回後應回到 home，texts %q", tt.Texts())
	}
}

// TestRemoveActiveAccountLogsOut（D20 帳號選單）：選單「移除 @xxx」→
// Modal 二次確認 → deleteAccount（帳號鍵刪除＋索引除名）＋記憶體快照除
// 名；被移除者為活躍帳號 → 登出至 login view（a.sess=nil）。
func TestRemoveActiveAccountLogsOut(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("9501"), accountKey("9502"))
	seedSwitchAccounts(t, "9501", "9502")
	a := newTestApp()
	a.sess = &fakeSession{userID: "9501"}
	a.warmup() // 同步 spawn／apply：帳號快照＋活躍快照（索引 Current）載入
	if len(a.accounts) != 2 || a.activeID != "9501" {
		t.Fatalf("前置：warmup 應載 2 帳號且活躍 9501，得 %d/%q", len(a.accounts), a.activeID)
	}
	tt := ui.NewTester(a.view, 1280, 720)
	tt.Frame()
	if err := tt.Click("帳號選單"); err != nil {
		t.Fatalf("開啟帳號選單: %v", err)
	}
	if err := tt.ChooseMenuItem("移除 @u9501"); err != nil {
		t.Fatalf("選移除項: %v", err)
	}
	if !a.removeOpen || a.removeUserID != "9501" {
		t.Fatalf("移除項應開啟確認 Modal，open=%v uid=%q", a.removeOpen, a.removeUserID)
	}
	tt.Frame() // Modal 呈現
	if err := tt.Click("確認移除"); err != nil {
		t.Fatalf("點確認移除: %v", err)
	}
	tt.Frame()
	if a.sess != nil {
		t.Error("移除活躍帳號應登出至 login view（a.sess=nil）")
	}
	if tt.HasText("時間線") || !tt.HasText("直連模式") {
		t.Errorf("登出後應呈 login view，texts %q", tt.Texts())
	}
	if got := len(a.accounts); got != 1 || a.accounts[0].UserID != "9502" {
		t.Errorf("記憶體快照應除名 9501，得 %v", a.accounts)
	}
	idx, err := loadIndex(context.Background())
	if err != nil {
		t.Fatalf("loadIndex: %v", err)
	}
	if idx == nil || !slices.Equal(idx.Order, []string{"9502"}) || idx.Current != "" {
		t.Errorf("索引應除名 9501 且 Current 歸空，得 %+v", idx)
	}
}

// TestSidebarAccountSwitchAndRelogin（D20 側欄）：失效帳號項（切換驗證失
// 敗的呈現語義）點擊走「重新貼上 Cookie」入口（login view 變體，目標帳
// 號預標；失效者不可為選擇——呈現回歸活躍帳號）；標記清除後點同項即切
// 換（switchTo 序列）。
func TestSidebarAccountSwitchAndRelogin(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("9601"), accountKey("9602"))
	seedSwitchAccounts(t, "9601", "9602")
	fresh := &fakeSession{userID: "9602"}
	withSwitchFactory(t, fresh)
	a := newTestApp()
	a.sess = &fakeSession{userID: "9601"}
	a.warmup()
	tt := ui.NewTester(a.view, 1280, 720)
	tt.Frame()
	// 失效帳號項 → 重登入口。
	a.accInvalid["9602"] = true
	tt.Frame() // 標籤隨下一幀更新（失效者標記為呈現面）
	if err := tt.Click("@u9602（失效）"); err != nil {
		t.Fatalf("點失效帳號項: %v", err)
	}
	if a.loginMode != loginModeRelogin || a.loginUserID != "9602" {
		t.Fatalf("失效帳號項應走重登入口：mode=%d uid=%q", a.loginMode, a.loginUserID)
	}
	if !tt.HasText("重新貼上 Cookie") {
		t.Errorf("應呈重新貼上 Cookie 變體，texts %q", tt.Texts())
	}
	if a.sideSel != "9601" {
		t.Errorf("失效者不可為選擇：側欄呈現應回歸活躍帳號，sideSel=%q", a.sideSel)
	}
	// 返回（入口復位）→ 標記清除（重登成功後的語義）→ 點同項即切換。
	a.resetLoginForm()
	delete(a.accInvalid, "9602")
	tt.Frame()
	if err := tt.Click("@u9602"); err != nil {
		t.Fatalf("點側欄帳號項: %v", err)
	}
	if a.sess != fresh || a.activeID != "9602" || a.gen != 1 {
		t.Fatalf("側欄帳號項應觸發切換：sess=%v activeID=%q gen=%d", a.sess, a.activeID, a.gen)
	}
}

// TestReloginUpdatesCookieAndSwitches（D20 失效重登，submitLogin 的
// relogin 分支）：身分不符（REQ-WEB-069）→ 錯誤呈現、不切換、secret 不
// 動；身分相符 → 以新 cookie 行覆蓋同鍵（REQ-WEB-070）、失效標記清除，
// 沿 switchTo 原地重試切換。
func TestReloginUpdatesCookieAndSwitches(t *testing.T) {
	withAccountService(t)
	cleanupSecrets(t, indexKey, accountKey("9701"), accountKey("9702"))
	seedSwitchAccounts(t, "9701", "9702")
	old := &fakeSession{userID: "9701"}
	bad := &fakeSession{userID: "9702", validate: func(string) (viewerInfo, error) {
		return viewerInfo{}, &xerrors.CredentialInvalidError{Msg: "HTTP 401"}
	}}
	a := newTestApp()
	a.sess = old
	a.warmup() // 帳號快照（重登入口的目標解析自快照——選單驅動的真實前提）
	withSwitchFactory(t, bad)
	a.switchTo("9702") // 切換驗證失敗 → 標記失效（重登入口的前提）
	if !a.accInvalid["9702"] || a.sess != old {
		t.Fatalf("前置：切換失敗應標記失效且原會話不動，invalid=%v sess=%v", a.accInvalid["9702"], a.sess)
	}
	// 身分不符：貼上他帳號身分的 cookie → 錯誤、停留變體、secret 不動。
	a.openRelogin("9702")
	a.cookieInput = "auth_token=synthetic-9702x; ct0=synthetic-9702y"
	a.newSess = func() (xSession, error) {
		return &fakeSession{validate: func(string) (viewerInfo, error) {
			return viewerInfo{UserID: "9999", ScreenName: "other"}, nil
		}}, nil
	}
	a.submitLogin()
	if a.loginErr == nil {
		t.Fatal("身分不符應呈現錯誤")
	}
	if a.loginMode != loginModeRelogin || a.sess != old || !a.accInvalid["9702"] {
		t.Fatalf("身分不符不得切換或清除標記：mode=%d sess=%v invalid=%v", a.loginMode, a.sess, a.accInvalid["9702"])
	}
	accs, _, err := loadAccounts(context.Background())
	if err != nil {
		t.Fatalf("loadAccounts: %v", err)
	}
	for _, acc := range accs {
		if acc.UserID == "9702" && acc.CookieLine != "auth_token=synthetic-9702; ct0=synthetic-9702b" {
			t.Errorf("身分不符不得動 secret：CookieLine = %q", acc.CookieLine)
		}
	}
	// 身分相符：貼上 9702 的新 cookie → 覆蓋同鍵、標記清除、原地切換。
	good := &fakeSession{userID: "9702"}
	a.newSess = func() (xSession, error) {
		return &fakeSession{validate: func(string) (viewerInfo, error) {
			return viewerInfo{UserID: "9702", ScreenName: "u9702", Name: "合成帳號9702"}, nil
		}}, nil
	}
	withSwitchFactory(t, good)
	a.cookieInput = "auth_token=synthetic-9702-new; ct0=synthetic-9702c"
	a.submitLogin()
	if a.loginErr != nil {
		t.Fatalf("身分相符應完成重登，得 %v", a.loginErr)
	}
	if a.sess != good || a.activeID != "9702" || a.accInvalid["9702"] {
		t.Fatalf("重登後應沿 switchTo 原地切換：sess=%v activeID=%q invalid=%v", a.sess, a.activeID, a.accInvalid["9702"])
	}
	accs, _, err = loadAccounts(context.Background())
	if err != nil {
		t.Fatalf("loadAccounts: %v", err)
	}
	for _, acc := range accs {
		if acc.UserID == "9702" {
			if acc.CookieLine != "auth_token=synthetic-9702-new; ct0=synthetic-9702c" {
				t.Errorf("重登應以新 cookie 行覆蓋同鍵：CookieLine = %q", acc.CookieLine)
			}
			if acc.ScreenName != "u9702" || acc.Name != "合成帳號9702" {
				t.Errorf("重登不得摧毀中繼資料：得 %+v", acc)
			}
		}
	}
}

// TestSearchBackAffordanceClosesSearch（Task 10 UX 註記釘測）：自預設
// tab（For You）開啟搜尋——已選 tab 的點擊不觸發 Changed，顯式「返回時
// 間線」是返回路徑：關閉搜尋（ephemeral 槽歸零）、For You 槽不受影響。
func TestSearchBackAffordanceClosesSearch(t *testing.T) {
	fake := &fakeSession{
		home: pageQueue(pageOf(mkPosts("a1"), "")),
		search: func(string, string, string) (*timelineResult, error) {
			return pageOf(mkPosts("s1"), ""), nil
		},
	}
	a, tt := homeFixture(t, fake)
	for i := 0; i < 3; i++ {
		tt.Frame() // For You 首頁載入
	}
	typeSearch(t, tt, "MyGO")
	if !a.searchOpen {
		t.Fatal("前置：搜尋應已開啟")
	}
	if err := tt.Click(searchBackText); err != nil {
		t.Fatalf("點返回時間線: %v", err)
	}
	tt.Frame()
	if a.searchOpen {
		t.Error("返回時間線應關閉搜尋")
	}
	fs := a.feeds[feedSearch]
	if len(fs.posts) != 0 || len(fs.seen) != 0 || fs.busy {
		t.Errorf("ephemeral 槽應歸零：posts=%d seen=%d busy=%v", len(fs.posts), len(fs.seen), fs.busy)
	}
	if !sameStrings(workIDs(a.feeds[feedForYou].posts), []string{"a1"}) {
		t.Errorf("For You 槽應不受影響，得 %q", workIDs(a.feeds[feedForYou].posts))
	}
	if !tt.HasText("時間線") {
		t.Errorf("返回後應回到時間線標示，texts %q", tt.Texts())
	}
}
