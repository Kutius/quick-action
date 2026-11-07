//go:build !windows

package main

// newAudioController fails on systems whose audio this tool does not switch
// yet: macOS and Linux need their own code here, the card and the tray icon
// need none.
func newAudioController() (audioController, error) { return nil, errUnsupported }
