package main

import (
	"log"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The two things the tool answers to a shortcut for, in the order of the
// rows of the panel.
const (
	keyMute = iota
	keyOutput
)

// defaultKeys are what the tool answers to until the user picks others, and
// spareKeys what it falls back to where another program has a default
// already: a tool that was just installed answers to something either way.
var (
	defaultKeys = [2]string{"Ctrl+Alt+M", "Ctrl+Alt+O"}
	spareKeys   = [2]string{"Ctrl+Alt+Shift+M", "Ctrl+Alt+Shift+O"}
)

// hotkey is a shortcut of the tool: the accelerator it answers to, and why
// the desktop would not take it where it did not.
type hotkey struct {
	chord string
	err   error
}

// action is what the shortcut of row i does.
func (a *app) action(i int) func() {
	if i == keyMute {
		return a.toggle
	}
	return a.switchOutput
}

// bind makes the tool answer to chord in row i, and reports what the desktop
// said: another program may hold it already.
func (a *app) bind(i int, chord string) error {
	if chord == "" {
		a.keys[i] = hotkey{}
		return nil
	}
	if err := mygo.GlobalShortcut.Register(chord, a.action(i)); err != nil {
		a.keys[i] = hotkey{chord: chord, err: err}
		return err
	}
	a.keys[i] = hotkey{chord: chord}
	return nil
}

// setUpKeys binds the shortcuts the user picked, or the tool's own where
// they picked none. A default another program holds gives way to a spare,
// which the tool then keeps, so that a fresh install answers to something.
func (a *app) setUpKeys(saved []string) {
	for i := range a.keys {
		chosen := i < len(saved)
		chord := defaultKeys[i]
		if chosen {
			chord = saved[i]
		}
		if chord == "" {
			// The user cleared it: the tool answers to nothing there.
			a.keys[i] = hotkey{}
			continue
		}
		if err := a.bind(i, chord); err == nil {
			continue
		}
		log.Printf("shortcut %s: %v", chord, a.keys[i].err)
		if chosen {
			// The panel says why it is off, and the user picks another.
			continue
		}
		if spare := spareKeys[i]; spare != "" && a.bind(i, spare) == nil {
			// Kept in memory only: the next start asks for the default
			// again, in case the program that had it is gone.
			log.Printf("shortcut %s: answering to %s instead", chord, spare)
		}
	}
	a.keyChanged()
}

// setKey makes the tool answer to chord in place of what it answered to,
// and keeps what it had where the desktop will not take the new one.
func (a *app) setKey(i int, chord string) {
	old := a.keys[i].chord
	if old != "" {
		mygo.GlobalShortcut.Unregister(old)
	}
	if err := a.bind(i, chord); err != nil {
		a.note = taken(chord)
		if old != "" {
			// Back to what worked; the panel says what did not.
			_ = a.bind(i, old)
		}
		return
	}
	a.note = ""
	a.rememberKeys()
	a.keyChanged()
}

// keyChanged tells the menu of the icon what the tool answers to now.
func (a *app) keyChanged() {
	if a.mute != nil {
		a.mute.SetSubtitle(a.keys[keyMute].chord)
	}
	if a.outItem != nil {
		a.outItem.SetSubtitle(a.keys[keyOutput].chord)
	}
}

// rememberKeys keeps the shortcuts the tool answers to between runs, the
// ones the desktop refused among them: it may take them next time.
func (a *app) rememberKeys() {
	a.remember(func(s *settings) {
		s.Hotkeys = []string{a.keys[keyMute].chord, a.keys[keyOutput].chord}
	})
}

// taken says why a shortcut is off, in the words the panel uses.
func taken(chord string) string {
	return chord + " 已被其它程序占用"
}

// keyNote is why a shortcut is off, if one is.
func (a *app) keyNote() string {
	for i := range a.keys {
		if a.keys[i].err != nil {
			return taken(a.keys[i].chord)
		}
	}
	return ""
}

// chordOf writes a key with the modifiers held as the desktop takes it, as
// in "Ctrl+Alt+O", and reports false for a press that is no shortcut: a
// modifier on its own, or a key without Ctrl, Alt or Win, which would take
// the key from the app the user is in.
func chordOf(ev ui.InputEvent) (string, bool) {
	name := keyName(ev.Key)
	if name == "" || ev.Mods&(ui.Ctrl|ui.Alt|ui.Super) == 0 {
		return "", false
	}
	var b strings.Builder
	for _, m := range []struct {
		mod  ui.Modifiers
		name string
	}{{ui.Super, "Super"}, {ui.Ctrl, "Ctrl"}, {ui.Alt, "Alt"}, {ui.Shift, "Shift"}} {
		if ev.Mods&m.mod != 0 {
			b.WriteString(m.name)
			b.WriteByte('+')
		}
	}
	b.WriteString(name)
	return b.String(), true
}

// keyName names a key as an accelerator does, or nothing for a key that
// names no shortcut.
func keyName(k ui.Key) string {
	switch {
	case k >= ui.KeyA && k <= ui.KeyZ:
		return string(rune('A' + k - ui.KeyA))
	case k >= ui.Key0 && k <= ui.Key9:
		return string(rune('0' + k - ui.Key0))
	case k >= ui.KeyF1 && k <= ui.KeyF12:
		return "F" + strconv.Itoa(int(k-ui.KeyF1)+1)
	}
	return namedKeys[k]
}

// namedKeys are the keys a shortcut names rather than types.
var namedKeys = map[ui.Key]string{
	ui.KeyEnter:        "Enter",
	ui.KeyTab:          "Tab",
	ui.KeySpace:        "Space",
	ui.KeyBackspace:    "Backspace",
	ui.KeyDelete:       "Delete",
	ui.KeyInsert:       "Insert",
	ui.KeyEscape:       "Escape",
	ui.KeyHome:         "Home",
	ui.KeyEnd:          "End",
	ui.KeyPageUp:       "PageUp",
	ui.KeyPageDown:     "PageDown",
	ui.KeyUp:           "Up",
	ui.KeyDown:         "Down",
	ui.KeyLeft:         "Left",
	ui.KeyRight:        "Right",
	ui.KeyMinus:        "-",
	ui.KeyEqual:        "=",
	ui.KeyComma:        ",",
	ui.KeyPeriod:       ".",
	ui.KeySlash:        "/",
	ui.KeySemicolon:    ";",
	ui.KeyQuote:        "'",
	ui.KeyBracketLeft:  "[",
	ui.KeyBracketRight: "]",
	ui.KeyBackslash:    "\\",
	ui.KeyBackquote:    "`",
}
