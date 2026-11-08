package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// panelApp is the panel of a tool that answers to the shortcuts it was
// made with.
func panelApp() *app {
	a := &app{}
	a.keys[keyMute] = hotkey{chord: "Ctrl+Alt+M"}
	a.keys[keyOutput] = hotkey{chord: "Ctrl+Alt+O"}
	return a
}

// TestPanel shows the settings and the shortcuts the tool answers to.
func TestPanel(t *testing.T) {
	tt := ui.NewTester(panelApp().settingsView, panelWidth, panelHeight)
	for _, want := range []string{"开机自启", "闭麦", "切换输出", "Ctrl+Alt+M", "Ctrl+Alt+O"} {
		if !tt.HasText(want) {
			t.Errorf("texts %q, want %q", tt.Texts(), want)
		}
	}
	if tt.HasText("未设置") || tt.HasText("按下组合键") {
		t.Errorf("texts %q, want the shortcuts as they are", tt.Texts())
	}
}

// TestPanelStates shows a shortcut waiting for a key, one cleared, and the
// line that says why a shortcut is off.
func TestPanelStates(t *testing.T) {
	a := panelApp()
	a.listening = keyMute + 1
	a.keys[keyOutput] = hotkey{}
	a.note = "Ctrl+Alt+O 已被其它程序占用"

	tt := ui.NewTester(a.settingsView, panelWidth, panelNoteHeight)
	for _, want := range []string{"按下组合键", "未设置", "Ctrl+Alt+O 已被其它程序占用"} {
		if !tt.HasText(want) {
			t.Errorf("texts %q, want %q", tt.Texts(), want)
		}
	}
}

// TestPanelSettles checks that the panel stops asking for frames once its
// motion is done, as the card does.
func TestPanelSettles(t *testing.T) {
	a := panelApp()
	tt := ui.NewTester(a.settingsView, panelWidth, panelHeight)
	tt.Move(240, 75)
	a.autoStart = true
	tt.Frame()
	tt.Advance(5 * time.Second)
	if tt.FramePending() {
		t.Error("the panel still asks for frames after its motion")
	}
}
