package main

import (
	"bytes"
	"image/color"
	"image/png"
	"sync"

	"github.com/egoist/mygo/ui"
)

// The icons of the card and of the notification area: Lucide's, drawn in the
// color they are given.
var (
	micOnSVG   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="6" height="13" x="9" y="2" rx="3"/><path d="M19 10v2a7 7 0 0 1-14 0v-2"/><path d="M12 19v3"/></svg>`))
	micOffSVG  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="2" x2="22" y1="2" y2="22"/><path d="M18.89 13.23A7.12 7.12 0 0 0 19 12v-2"/><path d="M5 10v2a7 7 0 0 0 12 5"/><path d="M15 9.34V5a3 3 0 0 0-5.68-1.33"/><path d="M9 9v3a3 3 0 0 0 5.12 2.12"/><path d="M12 19v3"/></svg>`))
	speakerSVG = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 5 6 9H2v6h4l5 4z"/><path d="M15.54 8.46a5 5 0 0 1 0 7.07"/><path d="M19.07 4.93a10 10 0 0 1 0 14.14"/></svg>`))
	chevronSVG = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>`))
)

// micIcon is the microphone as the card draws it.
func micIcon(muted bool) *ui.SVG {
	if muted {
		return micOffSVG
	}
	return micOnSVG
}

// trayIconSize is the size of the icon of the notification area in pixels:
// the 16 the taskbar asks for, drawn at twice that for its scaled displays.
const trayIconSize = 32

// trayIconPNG draws the microphone for the notification area. Both states
// keep a color that reads on the light and the dark taskbar alike, so that
// the icon itself says which it shows.
//
// There are two of them and neither changes, so each is drawn once: drawing
// one rasterizes its SVG and encodes a PNG.
func trayIconPNG(muted bool) []byte {
	if muted {
		return trayIconMuted()
	}
	return trayIconShown()
}

var (
	trayIconShown = sync.OnceValue(func() []byte { return drawTrayIcon(false) })
	trayIconMuted = sync.OnceValue(func() []byte { return drawTrayIcon(true) })
)

func drawTrayIcon(muted bool) []byte {
	ink := color.NRGBA{R: 0x8b, G: 0x93, B: 0xa7, A: 0xff}
	if muted {
		ink = color.NRGBA{R: 0xe5, G: 0x48, B: 0x4d, A: 0xff}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, micIcon(muted).Image(trayIconSize, trayIconSize, ink)); err != nil {
		return nil
	}
	return buf.Bytes()
}
