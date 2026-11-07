package main

import (
	"bytes"
	stdpng "image/png"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// fakeAudio stands in for the audio of the computer.
type fakeAudio struct {
	state   audioState
	outputs []outputDevice
	err     error

	muted  atomic.Int32 // switches of the microphone
	last   atomic.Bool  // the last of them: muted or not
	chosen atomic.Value // the last output made the default
}

func (f *fakeAudio) State() (audioState, error) { return f.state, f.err }

func (f *fakeAudio) SetMuted(muted bool) error {
	f.muted.Add(1)
	f.last.Store(muted)
	f.state.Mic.Muted = muted
	return nil
}

func (f *fakeAudio) Outputs() ([]outputDevice, error) { return f.outputs, f.err }

func (f *fakeAudio) SetOutput(id string) error {
	f.chosen.Store(id)
	for _, o := range f.outputs {
		if o.ID == id {
			f.state.Output = o
		}
	}
	return nil
}

func newFakeAudio() *fakeAudio {
	f := &fakeAudio{
		state: audioState{
			Mic:     micState{Device: "Microphone (USB Audio)", Count: 1},
			Output:  outputDevice{ID: "headset", Name: "耳机 (Realtek USB Audio)"},
			Outputs: 2,
		},
		outputs: []outputDevice{
			{ID: "headset", Name: "耳机 (Realtek USB Audio)", Default: true},
			{ID: "speaker", Name: "Headphones (鸽鸽法棍面包音响)"},
		},
	}
	f.chosen.Store("")
	return f
}

// TestCard shows both switches, and switches the microphone where its row
// is clicked.
func TestCard(t *testing.T) {
	audio := newFakeAudio()
	a := &app{audio: audio, state: audio.state, wake: make(chan struct{}, 1)}
	tt := ui.NewTester(a.view, widgetWidth, widgetHeight)

	// The tiles name the adapter out: the picker keeps the whole name.
	if !tt.HasText("开启") || !tt.HasText("耳机") {
		t.Fatalf("texts %q, want the microphone on and the headset playing", tt.Texts())
	}

	// What another app, or the system's own settings, do to it.
	audio.state.Mic.Muted = true
	a.state = audio.state
	tt.Frame()
	if !tt.HasText("已静音") {
		t.Fatalf("texts %q, want the microphone muted", tt.Texts())
	}

	if err := tt.Click("切换麦克风"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return audio.muted.Load() == 1 })
	if audio.last.Load() {
		t.Error("the switch muted the microphone again, want it unmuted")
	}
}

// TestCardSwitchesOutput switches to the output the user was on before, the
// one click of the row.
func TestCardSwitchesOutput(t *testing.T) {
	audio := newFakeAudio()
	a := &app{audio: audio, state: audio.state, previous: "speaker", wake: make(chan struct{}, 1)}
	tt := ui.NewTester(a.view, widgetWidth, widgetHeight)

	if err := tt.Click("切换输出"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return audio.chosen.Load() == "speaker" })
}

// TestCardDoesNotGuessOutput asks which output when there is none to go
// back to, rather than guessing one.
func TestCardDoesNotGuessOutput(t *testing.T) {
	audio := newFakeAudio()
	a := &app{audio: audio, state: audio.state, wake: make(chan struct{}, 1)}
	tt := ui.NewTester(a.view, widgetWidth, widgetHeight)

	if err := tt.Click("切换输出"); err != nil {
		t.Fatal(err)
	}
	// Nothing to switch to: the picker opens instead, which needs a
	// window, so the row leaves the output alone.
	time.Sleep(50 * time.Millisecond)
	if got := audio.chosen.Load(); got != "" {
		t.Errorf("switched to %v, want no switch without a known previous output", got)
	}
}

// TestCardWithoutAudio shows why it cannot switch anything.
func TestCardWithoutAudio(t *testing.T) {
	a := &app{audio: brokenAudio{errUnsupported}, wake: make(chan struct{}, 1)}
	a.state, a.err = a.audio.State()
	tt := ui.NewTester(a.view, widgetWidth, widgetHeight)

	if !tt.HasText("不可用") {
		t.Errorf("texts %q, want the audio unavailable", tt.Texts())
	}
}

// TestTrayIcon draws both states of the icon of the notification area.
func TestTrayIcon(t *testing.T) {
	for _, muted := range []bool{false, true} {
		png := trayIconPNG(muted)
		cfg, err := stdpng.DecodeConfig(bytes.NewReader(png))
		if err != nil {
			t.Fatalf("muted %v: %v", muted, err)
		}
		if cfg.Width != trayIconSize || cfg.Height != trayIconSize {
			t.Errorf("muted %v: %dx%d, want %d", muted, cfg.Width, cfg.Height, trayIconSize)
		}
	}
}

// TestShortName keeps the part of a device name a tile shows.
func TestShortName(t *testing.T) {
	for _, c := range []struct{ name, want string }{
		{"耳机 (Realtek USB Audio)", "耳机"},
		{"Headphones (鸽鸽法棍面包音响)", "Headphones"},
		{"LG ULTRAGEAR (NVIDIA High Definition Audio)", "LG ULTRAGEAR"},
		{"扬声器（网易虚拟音频设备）", "扬声器"},
		{"Speakers", "Speakers"},
		{"", ""},
		{"(odd)", "(odd)"},
	} {
		if got := shortName(c.name); got != c.want {
			t.Errorf("shortName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// waitFor runs until cond holds, or fails the test after two seconds.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the switch")
		}
		time.Sleep(time.Millisecond)
	}
}
