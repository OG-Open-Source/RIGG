package fingerprint

import (
	"testing"
)

// TestFingerprint_Constants_REQ-TRN-003 逐字斷言指紋常數契約。
func TestFingerprint_Constants_REQ_TRN_003(t *testing.T) {
	fp := Get()

	const wantUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Safari/537.36"
	if fp.UA != wantUA {
		t.Errorf("UA = %q, 期望 %q", fp.UA, wantUA)
	}

	const wantSecChUA = "\"Google Chrome\";v=\"153\", \"Not_A Brand\";v=\"8\", \"Chromium\";v=\"153\""
	if fp.SecChUA != wantSecChUA {
		t.Errorf("SecChUA = %q, 期望 %q", fp.SecChUA, wantSecChUA)
	}

	const wantSecChUAMobile = "?0"
	if fp.SecChUAMobile != wantSecChUAMobile {
		t.Errorf("SecChUAMobile = %q, 期望 %q", fp.SecChUAMobile, wantSecChUAMobile)
	}

	const wantSecChUAPlatform = "\"Windows\""
	if fp.SecChUAPlatform != wantSecChUAPlatform {
		t.Errorf("SecChUAPlatform = %q, 期望 %q", fp.SecChUAPlatform, wantSecChUAPlatform)
	}
}

// TestFingerprint_Language_REQ-TRN-004 逐字斷言語言與時區常數。
func TestFingerprint_Language_REQ_TRN_004(t *testing.T) {
	fp := Get()

	const wantAcceptLanguage = "zh-CN,zh;q=0.9,en;q=0.8,zh-TW;q=0.7,ja;q=0.6"
	if fp.AcceptLanguage != wantAcceptLanguage {
		t.Errorf("AcceptLanguage = %q, 期望 %q", fp.AcceptLanguage, wantAcceptLanguage)
	}

	const wantClientLanguage = "zh-CN"
	if fp.ClientLanguage != wantClientLanguage {
		t.Errorf("ClientLanguage = %q, 期望 %q", fp.ClientLanguage, wantClientLanguage)
	}

	const wantTimezone = "Asia/Shanghai"
	if fp.Timezone != wantTimezone {
		t.Errorf("Timezone = %q, 期望 %q", fp.Timezone, wantTimezone)
	}
}

// TestFingerprint_ScreenSampling_REQ-TRN-005 斷言螢幕抽樣 200 次皆命中
// 四選一集合，且欄位以字串保存。
func TestFingerprint_ScreenSampling_REQ_TRN_005(t *testing.T) {
	// 欄位以字串保存（型別斷言確認）。
	fp := Get()
	if _, ok := any(fp.ScreenWidth).(string); !ok {
		t.Errorf("ScreenWidth 型別 = %T，期望 string", fp.ScreenWidth)
	}
	if _, ok := any(fp.ScreenHeight).(string); !ok {
		t.Errorf("ScreenHeight 型別 = %T，期望 string", fp.ScreenHeight)
	}

	allowed := map[[2]string]bool{
		{"1920", "1080"}: true,
		{"1536", "864"}:  true,
		{"1440", "900"}:  true,
		{"1366", "768"}:  true,
	}

	// 單例下 Get() 只抽一次，故直接對內部抽樣函式測 200 次。
	for i := 0; i < 200; i++ {
		w, h := sampleScreen()
		if !allowed[[2]string{w, h}] {
			t.Fatalf("第 %d 次抽樣 (%q, %q) 不在四選一集合內", i, w, h)
		}
	}
}

// TestFingerprint_Singleton_REQ-TRN-001 斷言 Get() 兩次回傳同指標，
// 且欄位值不變。
func TestFingerprint_Singleton_REQ_TRN_001(t *testing.T) {
	first := Get()
	second := Get()
	if first != second {
		t.Fatalf("Get() 回傳不同指標：%p vs %p", first, second)
	}
	if *first != *second {
		t.Errorf("Get() 欄位值變動：%+v vs %+v", *first, *second)
	}
}
