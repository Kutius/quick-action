package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestChordOf writes the keys the user presses as the desktop takes them,
// and refuses the ones that would take a key from the app in front.
func TestChordOf(t *testing.T) {
	for _, c := range []struct {
		mods ui.Modifiers
		key  ui.Key
		want string
		ok   bool
	}{
		{ui.Ctrl | ui.Alt, ui.KeyM, "Ctrl+Alt+M", true},
		{ui.Ctrl | ui.Alt, ui.KeyO, "Ctrl+Alt+O", true},
		{ui.Super, ui.KeySpace, "Super+Space", true},
		{ui.Ctrl | ui.Alt | ui.Shift, ui.KeyF9, "Ctrl+Alt+Shift+F9", true},
		{ui.Ctrl | ui.Alt, ui.KeyPageDown, "Ctrl+Alt+PageDown", true},
		{ui.Ctrl | ui.Alt, ui.KeyMinus, "Ctrl+Alt+-", true},
		// Without a modifier a shortcut would take the key from the app in
		// front, and a modifier on its own is no key at all.
		{0, ui.KeyM, "", false},
		{ui.Shift, ui.KeyM, "", false},
		{ui.Ctrl, ui.KeyUnknown, "", false},
	} {
		got, ok := chordOf(ui.InputEvent{Key: c.key, Mods: c.mods})
		if got != c.want || ok != c.ok {
			t.Errorf("chordOf(%v, %v) = %q, %v; want %q, %v", c.mods, c.key, got, ok, c.want, c.ok)
		}
	}
}

// TestPressOf reads what a key pressed on a cap does: Escape leaves the
// shortcut, Backspace and Delete clear it, and a key with a modifier on it
// becomes it.
func TestPressOf(t *testing.T) {
	for _, c := range []struct {
		name string
		ev   ui.InputEvent
		want press
	}{
		{"escape", ui.InputEvent{Key: ui.KeyEscape}, press{cancel: true}},
		{"backspace", ui.InputEvent{Key: ui.KeyBackspace}, press{clear: true}},
		{"delete", ui.InputEvent{Key: ui.KeyDelete}, press{clear: true}},
		{"letter", ui.InputEvent{Key: ui.KeyM}, press{}},
		{"chord", ui.InputEvent{Key: ui.KeyM, Mods: ui.Ctrl | ui.Alt}, press{chord: "Ctrl+Alt+M", ok: true}},
		// A modifier on Escape is a shortcut, not the way out.
		{"ctrl+escape", ui.InputEvent{Key: ui.KeyEscape, Mods: ui.Ctrl}, press{chord: "Ctrl+Escape", ok: true}},
	} {
		if got := pressOf(c.ev); got != c.want {
			t.Errorf("%s: pressOf = %+v, want %+v", c.name, got, c.want)
		}
	}
}
