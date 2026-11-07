//go:build windows

package main

import (
	"runtime"
	"syscall"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")

	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procMonitorFromWindow   = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procIsZoomed            = user32.NewProc("IsZoomed")
	procGetShellWindow      = user32.NewProc("GetShellWindow")
	procGetDesktopWindow    = user32.NewProc("GetDesktopWindow")
	procSetWinEventHook     = user32.NewProc("SetWinEventHook")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
)

type monitorInfo struct {
	cbSize    uint32
	rcMonitor windowRect
	rcWork    windowRect
	dwFlags   uint32
}

const (
	monitorDefaultToNearest = 2

	eventSystemForeground     = 0x0003
	eventObjectLocationChange = 0x800B
	wineventOutOfContext      = 0x0000
)

// screenTaken reports whether an app in front covers the whole display the
// window hwnd is on, as a game in fullscreen does.
func screenTaken(hwnd uintptr) bool {
	front, _, _ := procGetForegroundWindow.Call()
	if front == 0 || front == hwnd {
		return false
	}
	// The desktop and the taskbar are not games.
	if shell, _, _ := procGetShellWindow.Call(); front == shell {
		return false
	}
	if desktop, _, _ := procGetDesktopWindow.Call(); front == desktop {
		return false
	}
	return coversScreen(factsOf(hwnd), factsOf(front))
}

// factsOf reads where a window is, and the display it is on.
func factsOf(hwnd uintptr) windowFacts {
	var facts windowFacts
	if visible, _, _ := procIsWindowVisible.Call(hwnd); visible != 0 {
		facts.visible = true
	}
	if zoomed, _, _ := procIsZoomed.Call(hwnd); zoomed != 0 {
		facts.zoomed = true
	}
	if ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&facts.bounds))); ok == 0 {
		return windowFacts{}
	}
	monitor, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	if monitor == 0 {
		return facts
	}
	var info monitorInfo
	info.cbSize = uint32(unsafe.Sizeof(info))
	if ok, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); ok != 0 {
		facts.monitor = info.rcMonitor
	}
	return facts
}

// watchForeground calls fn whenever the window in front changes, or the one
// in front covers the screen. Windows tells a thread of its own, which
// sleeps in its message loop until it does, so the card asks nothing of the
// computer while nothing happens: no timer, no polling.
func watchForeground(fn func()) {
	events := make(chan struct{}, 1)
	// The callback runs on the thread below, inside its message loop, and
	// only hands the news over: moving the card waits for the main thread.
	callback := syscall.NewCallback(func(_, event, hwnd, idObject, _, _, _ uintptr) uintptr {
		if idObject != 0 { // OBJID_WINDOW: the window itself, not a part of it
			return 0
		}
		if event == eventObjectLocationChange {
			// A window that is not in front cannot cover the screen: a
			// resize matters only for the one that is.
			if front, _, _ := procGetForegroundWindow.Call(); hwnd != front {
				return 0
			}
		}
		select {
		case events <- struct{}{}:
		default:
		}
		return 0
	})

	go func() {
		for range events {
			fn()
		}
	}()
	go func() {
		// The hooks belong to the thread that sets them, and Windows
		// delivers their events through that thread's message queue.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		foreground, _, _ := procSetWinEventHook.Call(eventSystemForeground, eventSystemForeground, 0,
			callback, 0, 0, wineventOutOfContext)
		moved, _, _ := procSetWinEventHook.Call(eventObjectLocationChange, eventObjectLocationChange, 0,
			callback, 0, 0, wineventOutOfContext)
		if foreground == 0 && moved == 0 {
			return
		}
		var message [64]byte // MSG, which is 48 bytes on 64-bit Windows
		for {
			if r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message[0])), 0, 0, 0); int32(r) <= 0 {
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&message[0])))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message[0])))
		}
	}()
}
