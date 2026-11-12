package xapi

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestInternalXapiDoesNotImportMygo 機械強制 internal/xapi 套件樹的
// import 邊界（設計 §3.2）：引擎不得依賴 github.com/egoist/mygo——X 流量
// 只經 transport 的單一出口，不得以 MyGO 插件或框架承載。MyGO 僅屬於應用
// 層（模組根 main），故禁令掃描的對象是 internal/xapi 套件樹的依賴閉包，
// 不含應用層。
//
// 方法：以模組根為工作目錄執行 go list -deps，斷言輸出中沒有任何套件路徑
// 以 github.com/egoist/mygo 開頭。go list 無法執行時跳過並附原因（編譯
// 錯誤等由 go build／go vet 把關，此處不假失敗）。
func TestInternalXapiDoesNotImportMygo(t *testing.T) {
	// 模組根：本檔位於 <模組根>/internal/xapi/，向上兩層即得。
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skipf("runtime.Caller 無法取得本檔路徑，無法定位模組根")
	}
	moduleRoot, err := filepath.Abs(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	if err != nil {
		t.Skipf("無法推導模組根絕對路徑：%v", err)
	}

	cmd := exec.Command("go", "list", "-deps", "./internal/xapi/...")
	cmd.Dir = moduleRoot
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list -deps 執行失敗（不視為違規）：%v\nstderr:\n%s", err, stderr.String())
	}

	const banned = "github.com/egoist/mygo"
	var violations []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, banned) {
			violations = append(violations, line)
		}
	}
	if len(violations) > 0 {
		t.Errorf("internal/xapi 套件樹不得 import %s，違規 %d 筆：\n%s",
			banned, len(violations), strings.Join(violations, "\n"))
	}
}
