package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The view runs without a window in tests. The window content dispatches
// between the login and home views: login first, home once a session is set.
func TestView(t *testing.T) {
	a := newTestApp()
	tt := ui.NewTester(a.view, 480, 720)
	if !tt.HasText("Cookie") {
		t.Errorf("初始畫面應為 login（Cookie 輸入），texts %q", tt.Texts())
	}
	a.sess = &fakeSession{}
	tt.Frame()
	if !tt.HasText("時間線") {
		t.Errorf("有會話後畫面應為 home，texts %q", tt.Texts())
	}
}
