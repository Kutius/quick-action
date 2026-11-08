//go:build windows

package main

var procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")

// vkLeftButton is the virtual key code of the left mouse button.
const vkLeftButton = 0x01

// pointerDown reports whether the left mouse button is down, wherever the
// pointer is. A drag of the card is the system moving the window, which the
// card sees only as moves, so the button is what tells when the drag ended.
func pointerDown() bool {
	// The high bit of the state is set while the button is down.
	v, _, _ := procGetAsyncKeyState.Call(vkLeftButton)
	return uint16(v)&0x8000 != 0
}
