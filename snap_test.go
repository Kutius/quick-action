package main

import (
	"testing"

	"github.com/egoist/mygo"
)

// TestSnapTo checks where a card of the card size settles when it is let go
// at a place of the work area, flush to an edge or where it was.
func TestSnapTo(t *testing.T) {
	work := mygo.Rectangle{Width: 1920, Height: 1040}
	cases := []struct {
		name         string
		x, y         int
		wantX, wantY int
		wantSnapped  bool
	}{
		{"free", 800, 500, 800, 500, false},
		{"out of reach", 30, 500, 30, 500, false},
		{"left edge", 12, 500, 0, 500, true},
		{"past the left edge", -6, 500, 0, 500, true},
		{"right edge", 1920 - 264 - 9, 500, 1920 - 264, 500, true},
		{"top edge", 800, 5, 800, 0, true},
		{"bottom edge", 800, 1040 - 84 + 3, 800, 1040 - 84, true},
		{"corner", 10, 8, 0, 0, true},
	}
	for _, c := range cases {
		x, y, snapped := snapTo(c.x, c.y, 264, 84, work)
		if x != c.wantX || y != c.wantY || snapped != c.wantSnapped {
			t.Errorf("%s: snapTo(%d, %d) = %d, %d, %v; want %d, %d, %v",
				c.name, c.x, c.y, x, y, snapped, c.wantX, c.wantY, c.wantSnapped)
		}
	}
}

// TestSnapToNearest checks that a card small enough to be within reach of both
// edges of its work area settles on the nearer one.
func TestSnapToNearest(t *testing.T) {
	work := mygo.Rectangle{Width: 100, Height: 100}
	if x, _, _ := snapTo(58, 40, 40, 20, work); x != 60 {
		t.Errorf("near the far edge: x = %d, want 60", x)
	}
	if x, _, _ := snapTo(2, 40, 40, 20, work); x != 0 {
		t.Errorf("near the near edge: x = %d, want 0", x)
	}
}

// TestSnapToLargerThanWork checks that a card that does not fit its work area
// is left alone along the axis it does not fit.
func TestSnapToLargerThanWork(t *testing.T) {
	work := mygo.Rectangle{Width: 200, Height: 1040}
	x, y, snapped := snapTo(120, 500, 264, 84, work)
	if x != 120 || y != 500 || snapped {
		t.Errorf("snapTo = %d, %d, %v; want 120, 500, false", x, y, snapped)
	}
}

// TestWorkAreaFor checks that the card settles within the work area of the
// display it is the most on.
func TestWorkAreaFor(t *testing.T) {
	first := mygo.Rectangle{Width: 1920, Height: 1040}
	second := mygo.Rectangle{X: 1920, Width: 1280, Height: 984}
	displays := []mygo.Display{{WorkArea: first}, {WorkArea: second}}

	work, ok := workAreaFor(mygo.Rectangle{X: 1800, Y: 200, Width: 264, Height: 84}, displays)
	if !ok || work != second {
		t.Errorf("mostly on the second display: %+v, %v; want %+v, true", work, ok, second)
	}
	work, ok = workAreaFor(mygo.Rectangle{X: 600, Y: 200, Width: 264, Height: 84}, displays)
	if !ok || work != first {
		t.Errorf("mostly on the first display: %+v, %v; want %+v, true", work, ok, first)
	}
	if _, ok := workAreaFor(mygo.Rectangle{X: -900, Y: -900, Width: 264, Height: 84}, displays); ok {
		t.Error("off every display: settled in one anyway")
	}
	if _, ok := workAreaFor(mygo.Rectangle{X: 10, Y: 10, Width: 264, Height: 84}, nil); ok {
		t.Error("no displays: settled in one anyway")
	}
}
