package main

// windowRect is a rectangle of the screen, in pixels.
type windowRect struct {
	left, top, right, bottom int32
}

// windowFacts is what the card asks of a window: where it is, the display it
// is on, and whether it is shown and maximized.
type windowFacts struct {
	bounds  windowRect
	monitor windowRect
	visible bool
	zoomed  bool
}

// coversScreen reports whether front covers the whole display card is on, as
// a game in fullscreen does.
//
// A maximized window does not count: it is one the user works in, with the
// taskbar still there, and the card belongs over it. Games in fullscreen
// cover the display without being maximized, which tells them apart.
func coversScreen(card, front windowFacts) bool {
	if !front.visible || front.zoomed {
		return false
	}
	if front.monitor == (windowRect{}) || front.monitor != card.monitor {
		return false
	}
	return front.bounds.left <= front.monitor.left &&
		front.bounds.top <= front.monitor.top &&
		front.bounds.right >= front.monitor.right &&
		front.bounds.bottom >= front.monitor.bottom
}
