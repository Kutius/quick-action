package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestCardSettles checks that the card stops asking for frames once its
// motion is done: a card that draws forever would cost the computer it
// sits on.
func TestCardSettles(t *testing.T) {
	audio := newFakeAudio()
	a := &app{audio: audio, state: audio.state, wake: make(chan struct{}, 1)}
	tt := ui.NewTester(a.view, widgetWidth, widgetHeight)

	tt.Move(195, 42)
	a.state.Mic.Muted = true
	tt.Frame()
	tt.Advance(5 * time.Second)
	if tt.FramePending() {
		t.Error("the card still asks for frames after its motion")
	}
}
