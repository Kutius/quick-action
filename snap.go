package main

import (
	"math"
	"time"

	"github.com/egoist/mygo"
)

// snapGap is how close to an edge of the work area the card has to be let go
// to settle flush against it: far enough to catch a careless drag, short
// enough not to take the card from where it was meant to be.
const snapGap = 24

// snapPoll is how often the watcher looks at the button while the card is
// dragged, so that the card settles as soon as it is let go: often enough
// that the glide starts as the hand comes off it, rarely enough that the
// wakeups cost nothing.
const snapPoll = 30 * time.Millisecond

// glideTime is how long the card takes to come to rest against the edge it
// settled on.
const glideTime = 190 * time.Millisecond

// cardMoved wakes the watcher that settles the card, without waiting for it.
func (a *app) cardMoved() {
	select {
	case a.moved <- struct{}{}:
	default:
	}
}

// watchSnap settles the card against the edge it was let go near. The system
// moves the window while it is dragged, so the card cannot tell from its own
// input when the drag ends: it watches the button instead, and looks at where
// the card is once the button is up.
func (a *app) watchSnap() {
	for range a.moved {
		if a.settling.Load() || !pointerDown() {
			continue
		}
		for pointerDown() {
			time.Sleep(snapPoll)
		}
		a.settle()
	}
}

// settle moves the card flush against the edge of the work area it was let go
// near, if it was let go near one.
func (a *app) settle() {
	x, y := a.win.Position()
	w, h := a.win.Size()
	work, ok := workAreaFor(mygo.Rectangle{X: x, Y: y, Width: w, Height: h}, mygo.Screen.Displays())
	if !ok {
		return
	}
	tx, ty, snapped := snapTo(x, y, w, h, work)
	if !snapped {
		return
	}
	a.settling.Store(true)
	defer a.settling.Store(false)
	glide(a.win, x, y, tx, ty)
}

// glide moves the card from where it is to x, y, easing out so that it comes
// to rest against the edge instead of stopping at it. It lets go as soon as
// the user takes the card again.
func glide(w *mygo.Window, fromX, fromY, toX, toY int) {
	start := time.Now()
	for {
		t := float64(time.Since(start)) / float64(glideTime)
		if t >= 1 {
			break
		}
		if pointerDown() {
			return
		}
		e := 1 - math.Pow(1-t, 3)
		w.SetPosition(
			fromX+int(math.Round(float64(toX-fromX)*e)),
			fromY+int(math.Round(float64(toY-fromY)*e)),
		)
		time.Sleep(snapPoll)
	}
	w.SetPosition(toX, toY)
}

// snapTo returns where a card of w by h at x, y settles: flush against the
// edge of work it was let go near, or where it was.
func snapTo(x, y, w, h int, work mygo.Rectangle) (int, int, bool) {
	if work.Width <= 0 || work.Height <= 0 {
		return x, y, false
	}
	snapped := false
	if w <= work.Width {
		if nx, ok := snapAxis(x, w, work.X, work.Width); ok {
			x, snapped = nx, true
		}
	}
	if h <= work.Height {
		if ny, ok := snapAxis(y, h, work.Y, work.Height); ok {
			y, snapped = ny, true
		}
	}
	return x, y, snapped
}

// snapAxis returns where a span of size that starts at start settles along one
// axis of a work area that begins at origin with length, and whether it does.
func snapAxis(start, size, origin, length int) (int, bool) {
	low, high := origin, origin+length-size
	dl, dh := abs(start-low), abs(start-high)
	switch {
	case dl <= snapGap && dl <= dh:
		return low, true
	case dh <= snapGap:
		return high, true
	}
	return start, false
}

// workAreaFor returns the work area of the display the card is the most on:
// its bounds, without the taskbar.
func workAreaFor(card mygo.Rectangle, displays []mygo.Display) (mygo.Rectangle, bool) {
	best, bestArea := mygo.Rectangle{}, 0
	for _, d := range displays {
		if a := overlap(card, d.WorkArea); a > bestArea {
			best, bestArea = d.WorkArea, a
		}
	}
	return best, bestArea > 0
}

// overlap is the area the two rectangles share, zero when they share none.
func overlap(a, b mygo.Rectangle) int {
	w := min(a.X+a.Width, b.X+b.Width) - max(a.X, b.X)
	h := min(a.Y+a.Height, b.Y+b.Height) - max(a.Y, b.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
