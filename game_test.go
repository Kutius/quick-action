package main

import "testing"

// TestCoversScreen tells a game in fullscreen apart from the windows a user
// works in.
func TestCoversScreen(t *testing.T) {
	display := windowRect{0, 0, 2048, 1152}
	other := windowRect{2048, 0, 4096, 1152}
	card := windowFacts{bounds: windowRect{900, 500, 1200, 596}, monitor: display, visible: true}

	for _, c := range []struct {
		name  string
		front windowFacts
		want  bool
	}{
		{
			name:  "a game covering the display",
			front: windowFacts{bounds: display, monitor: display, visible: true},
			want:  true,
		},
		{
			name:  "a game beyond the display's edges",
			front: windowFacts{bounds: windowRect{-8, -8, 2056, 1160}, monitor: display, visible: true},
			want:  true,
		},
		{
			name:  "a maximized window, which keeps the taskbar",
			front: windowFacts{bounds: windowRect{0, 0, 2048, 1104}, monitor: display, visible: true},
			want:  false,
		},
		{
			name:  "a window marked maximized, whatever its bounds",
			front: windowFacts{bounds: display, monitor: display, visible: true, zoomed: true},
			want:  false,
		},
		{
			name:  "a window of the size a user works in",
			front: windowFacts{bounds: windowRect{376, 141, 1671, 970}, monitor: display, visible: true},
			want:  false,
		},
		{
			name:  "a game on another display",
			front: windowFacts{bounds: other, monitor: other, visible: true},
			want:  false,
		},
		{
			name:  "a hidden window",
			front: windowFacts{bounds: display, monitor: display},
			want:  false,
		},
		{
			name:  "a window of no display known",
			front: windowFacts{bounds: display, visible: true},
			want:  false,
		},
	} {
		if got := coversScreen(card, c.front); got != c.want {
			t.Errorf("%s: coversScreen = %v, want %v", c.name, got, c.want)
		}
	}
}
