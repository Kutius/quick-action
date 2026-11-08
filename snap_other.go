//go:build !windows

package main

// pointerDown reports whether the left mouse button is down. Only Windows
// tells the card when a drag of its window ended, so elsewhere the card
// stays where it was let go.
func pointerDown() bool { return false }
