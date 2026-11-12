package fingerprint

import (
	"math/rand/v2"
	"sync"
)

// 指紋常數為規格 REQ-TRN-003/004 逐字契約，僅能跟隨上游規格版本更新
// （REQ-INV-09：與傳輸層 chrome150 各自自洽）。
const (
	uaChrome153 = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Safari/537.36"

	secChUAChrome153 = "\"Google Chrome\";v=\"153\", \"Not_A Brand\";v=\"8\", \"Chromium\";v=\"153\""

	secChUAMobile = "?0"

	secChUAPlatform = "\"Windows\""

	acceptLanguage = "zh-CN,zh;q=0.9,en;q=0.8,zh-TW;q=0.7,ja;q=0.6"

	clientLanguage = "zh-CN"

	timezone = "Asia/Shanghai"
)

// Fingerprint 是九欄位指紋檔：UA、Client Hints、語言、時區與四選一
// 螢幕抽樣。程序內單一真源、啟動後不變、不持久化（REQ-TRN-001..006）。
type Fingerprint struct {
	UA              string
	SecChUA         string
	SecChUAMobile   string
	SecChUAPlatform string
	AcceptLanguage  string
	ClientLanguage  string
	Timezone        string
	ScreenWidth     string
	ScreenHeight    string
}

// screenOptions 為四選一螢幕抽樣集合，皆以字串保存（REQ-TRN-005）。
var screenOptions = [4][2]string{
	{"1920", "1080"},
	{"1536", "864"},
	{"1440", "900"},
	{"1366", "768"},
}

// sampleScreen 以 math/rand/v2 均勻抽四選一，以字串回傳。
func sampleScreen() (string, string) {
	pick := screenOptions[rand.IntN(len(screenOptions))]
	return pick[0], pick[1]
}

var (
	once   sync.Once
	shared *Fingerprint
)

// Get 回傳程序內唯一指紋（sync.Once 建構，啟動後不變，REQ-TRN-001）。
func Get() *Fingerprint {
	once.Do(func() {
		shared = newFingerprintWithScreen()
	})
	return shared
}

func newFingerprintWithScreen() *Fingerprint {
	w, h := sampleScreen()
	fp := &Fingerprint{
		UA:              uaChrome153,
		SecChUA:         secChUAChrome153,
		SecChUAMobile:   secChUAMobile,
		SecChUAPlatform: secChUAPlatform,
		AcceptLanguage:  acceptLanguage,
		ClientLanguage:  clientLanguage,
		Timezone:        timezone,
		ScreenWidth:     w,
		ScreenHeight:    h,
	}
	return fp
}

// NewForTest 僅測試注入用：依 screen 參數取集合值，非法值 panic。
func NewForTest(screen string) *Fingerprint {
	fp := &Fingerprint{
		UA:              uaChrome153,
		SecChUA:         secChUAChrome153,
		SecChUAMobile:   secChUAMobile,
		SecChUAPlatform: secChUAPlatform,
		AcceptLanguage:  acceptLanguage,
		ClientLanguage:  clientLanguage,
		Timezone:        timezone,
	}
	for _, opt := range screenOptions {
		if opt[0] == screen {
			fp.ScreenWidth = opt[0]
			fp.ScreenHeight = opt[1]
			return fp
		}
	}
	panic("fingerprint: NewForTest: 非法螢幕值 " + screen)
}
