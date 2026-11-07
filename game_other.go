//go:build !windows

package main

// screenTaken reports whether an app in front covers the screen, which only
// the Windows card looks for.
func screenTaken(uintptr) bool { return false }

// watchForeground calls fn when the window in front changes, which only the
// Windows card looks for.
func watchForeground(func()) {}
