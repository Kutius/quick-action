package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The card is two tiles, as wide and as tall as they are.
const (
	widgetWidth  = 264
	widgetHeight = 84
)

// The card lays its tiles out in a row of that padding and gap; each tile
// fills the rest.
const (
	cardPadding = 10
	cardGap     = 7
	cardRadius  = 13
	tileRadius  = 11
)

// The card's motion: a quick spring for what the pointer does, which should
// feel like the card answered, and a slower one for what the audio does,
// which is worth watching. The second fades colors rather than moving
// anything, so it keeps its shape while the desktop asks for less motion.
var (
	liftSpring  = ui.Animation{Spring: ui.SpringOptions{Duration: 260 * time.Millisecond, Bounce: 0.1}}
	pressSpring = ui.Animation{Spring: ui.SpringOptions{Duration: 180 * time.Millisecond, Bounce: 0.2}}
	stateFade   = ui.Animation{Duration: 240 * time.Millisecond, Ease: ui.EaseInOut, NonMotion: true}
)

// plate is the material a piece of the card is made of: the two shades its
// face runs between, the hairline that edges it, the light that falls inside
// its top edge, the shadow its bottom edge keeps, and the shadow it casts.
type plate struct {
	face               ui.LinearGradient
	edge               ui.LinearGradient
	shine, glow, shade ui.Color
	cast               ui.Color
}

// paint gives an element the plate: a face that runs a shade darker from its
// top to its bottom, a hairline lit from above, the light that spills from
// that hairline, and the shade its bottom edge keeps.
func (p plate) paint(e ui.Element) ui.Element {
	e = e.LinearGradient(p.face).Border(1, ui.Color{}).BorderLinearGradient(p.edge)
	e = e.InsetShadow(0, 1, 0, 0, p.shine)
	e = e.InsetShadow(0, 2, 12, 0, p.glow)
	return e.InsetShadow(0, -1, 1, 0, p.shade)
}

// lit is the plate with the light a pointer over it gives.
func (p plate) lit(dark bool) plate {
	toward := ui.RGB(0, 0, 0)
	if dark {
		toward = ui.RGB(255, 255, 255)
	}
	p.face.From = p.face.From.Mix(toward, 0.07)
	p.face.To = p.face.To.Mix(toward, 0.07)
	return p
}

// mix is the plate f of the way from p to o, for a material that changes
// with the audio rather than blinking into its next state.
func (p plate) mix(o plate, f float32) plate {
	return plate{
		face: ui.LinearGradient{Angle: p.face.Angle,
			From: p.face.From.Mix(o.face.From, f),
			To:   p.face.To.Mix(o.face.To, f)},
		edge: ui.LinearGradient{Angle: p.edge.Angle,
			From: p.edge.From.Mix(o.edge.From, f),
			To:   p.edge.To.Mix(o.edge.To, f)},
		shine: p.shine.Mix(o.shine, f),
		glow:  p.glow.Mix(o.glow, f),
		shade: p.shade.Mix(o.shade, f),
		cast:  p.cast.Mix(o.cast, f),
	}
}

// cardPlate is the material of the card itself, which sits on the desktop
// with the shadow Windows gives its window.
func cardPlate(t *ui.Theme) plate {
	white, black := ui.RGB(255, 255, 255), ui.RGB(0, 0, 0)
	if t.Dark {
		return plate{
			face:  ui.LinearGradient{Angle: 180, From: ui.Hex("#2b2b31"), To: ui.Hex("#1b1b1f")},
			edge:  ui.LinearGradient{Angle: 180, From: white.Alpha(0.16), To: white.Alpha(0.02)},
			shine: white.Alpha(0.12),
			glow:  white.Alpha(0.05),
			shade: black.Alpha(0.5),
		}
	}
	return plate{
		face:  ui.LinearGradient{Angle: 180, From: ui.Hex("#fdfdfe"), To: ui.Hex("#f0f0f4")},
		edge:  ui.LinearGradient{Angle: 180, From: white, To: black.Alpha(0.12)},
		shine: white.Alpha(0.95),
		glow:  white.Alpha(0.35),
		shade: black.Alpha(0.05),
	}
}

// tilePlate is the material of a tile: a button raised from the card, or a
// muted one that keeps the light of the danger color.
func tilePlate(t *ui.Theme, tint ui.Color) plate {
	white, black := ui.RGB(255, 255, 255), ui.RGB(0, 0, 0)
	if tint != (ui.Color{}) {
		if t.Dark {
			return plate{
				face:  ui.LinearGradient{Angle: 180, From: tint.Alpha(0.26), To: tint.Alpha(0.13)},
				edge:  ui.LinearGradient{Angle: 180, From: tint.Alpha(0.5), To: tint.Alpha(0.16)},
				shine: white.Alpha(0.16),
				glow:  white.Alpha(0.04),
				shade: black.Alpha(0.22),
				cast:  tint.Alpha(0.22),
			}
		}
		return plate{
			face:  ui.LinearGradient{Angle: 180, From: tint.Alpha(0.15), To: tint.Alpha(0.08)},
			edge:  ui.LinearGradient{Angle: 180, From: tint.Alpha(0.34), To: tint.Alpha(0.14)},
			shine: white.Alpha(0.7),
			glow:  white.Alpha(0.18),
			shade: tint.Alpha(0.10),
			cast:  tint.Alpha(0.18),
		}
	}
	if t.Dark {
		return plate{
			face:  ui.LinearGradient{Angle: 180, From: ui.Hex("#34343a"), To: ui.Hex("#27272c")},
			edge:  ui.LinearGradient{Angle: 180, From: white.Alpha(0.13), To: white.Alpha(0.03)},
			shine: white.Alpha(0.12),
			glow:  white.Alpha(0.04),
			shade: black.Alpha(0.32),
			cast:  black.Alpha(0.40),
		}
	}
	return plate{
		face:  ui.LinearGradient{Angle: 180, From: white, To: ui.Hex("#f3f3f7")},
		edge:  ui.LinearGradient{Angle: 180, From: white, To: black.Alpha(0.08)},
		shine: white.Alpha(0.95),
		glow:  white.Alpha(0.30),
		shade: black.Alpha(0.04),
		cast:  black.Alpha(0.10),
	}
}

// view draws the card: the microphone and the output, each a tile that does
// its switch where it is clicked. The card's edge moves the window, as a
// panel of macOS moves by what it is not, so that the tiles stay free to be
// switches.
func (a *app) view(c *ui.Context) {
	// One read of what the audio does for the whole frame, so that the two
	// tiles cannot show two different states.
	st, err := a.snapshot()
	t := c.Theme()
	card := cardPlate(t).paint(ui.Row(c).Fill().
		Padding(cardPadding).Gap(cardGap).AlignItems(ui.Stretch).Radius(cardRadius))
	card.Cursor(ui.CursorGrab).DragWindow().Children(func() {
		a.micTile(c, st, err)
		a.outputTile(c, st, err)
	})
}

// micTile is the microphone: a click mutes or unmutes it. The card turns to
// the danger color as it does, and the microphone in it gives way to the
// one with a line through it, both on the way the spring carries.
func (a *app) micTile(c *ui.Context, st audioState, err error) {
	t := c.Theme()
	muted := st.Mic.Muted && err == nil

	value, valueColor := "开启", t.Text
	if muted {
		value = "已静音"
	}
	switch {
	case err != nil:
		value, valueColor = "不可用", t.TextMuted
	case st.Mic.Device == "":
		value, valueColor = "未找到麦克风", t.TextMuted
	}

	tile := cardTile(c, t.Danger, muted, "切换麦克风")
	mute := tile.mute
	ink := t.Text.Mix(t.Danger, mute)

	tile.OnClick(a.toggle).Children(func() {
		// The two microphones sit in one place, one fading in as the other
		// fades out, so that the tile changes rather than blinks.
		ui.Box(c).Size(21, 21).Children(func() {
			for _, glyph := range []struct {
				svg  *ui.SVG
				fade float32
			}{
				// Each fades out of the way squared, so that the two are
				// bright together where they cross rather than washing out.
				{micOnSVG, mute * mute},
				{micOffSVG, (1 - mute) * (1 - mute)},
			} {
				icon := ui.Box(c).Absolute().Fill().Center().Opacity(1 - glyph.fade)
				icon.Children(func() { ui.Icon(c, glyph.svg).FontSize(21).TextColor(ink) })
			}
		})
		ui.Text(c, value).FontSize(11).TextColor(valueColor.Mix(t.Danger, mute)).SingleLine().Ellipsis("…")
	})
}

// outputTile is the output: a click switches to the one the user was on
// before, and the arrow picks another.
func (a *app) outputTile(c *ui.Context, st audioState, err error) {
	t := c.Theme()
	value, valueColor := shortName(st.Output.Name), t.Text
	switch {
	case err != nil:
		value, valueColor = "不可用", t.TextMuted
	case value == "":
		value, valueColor = "未找到输出设备", t.TextMuted
	}

	tile := cardTile(c, t.Danger, false, "切换输出")

	tile.OnClick(a.switchOutput).Children(func() {
		ui.Icon(c, speakerSVG).FontSize(21).TextColor(t.Text)
		ui.Text(c, value).FontSize(11).TextColor(valueColor).SingleLine().Ellipsis("…")
		// The way to another output rises with the pointer, so that the
		// tile is nothing but the output at rest.
		if st.Outputs > 1 {
			more := ui.Box(c).Size(18, 18).Radius(5).Center().
				Absolute().Top(3).Right(3).Cursor(ui.CursorPointer).Label("选择输出").
				Opacity(tile.lift)
			if more.Hovered() {
				more.Background(t.Text.Alpha(0.08))
			}
			more.OnClick(a.chooseOutput).Children(func() {
				ui.Icon(c, chevronSVG).FontSize(10).TextColor(t.TextMuted)
			})
		}
	})
}

// control is a tile of the card, with the values its own motion runs on:
// how far it has turned to the danger color, and how far the pointer has
// lifted it.
type control struct {
	ui.Element
	mute, lift float32
}

// cardTile is a tile of the card: a switch raised from it, which lifts as
// the pointer arrives and sets down under a press, and turns to tint as the
// audio does.
func cardTile(c *ui.Context, tint ui.Color, muted bool, label string) control {
	t := c.Theme()
	tile := ui.Column(c).Grow(1).Gap(6).Center().Radius(tileRadius).
		Cursor(ui.CursorPointer).Label(label)
	mute := tile.AnimateTo("mute", amount(muted), stateFade)
	lift := tile.AnimateTo("lift", amount(tile.Hovered()), liftSpring)
	press := tile.AnimateTo("press", amount(tile.Pressed()), pressSpring)
	// The pointer lifts the tile a pixel and a press sets it down. Both are
	// applied here, from the values the springs carry, rather than posed
	// towards values that are themselves still moving, which would lag.
	set := 1 - 0.03*press
	tile.Translate(0, -lift).Scale(set, set)

	material := tilePlate(t, ui.Color{}).mix(tilePlate(t, tint), mute)
	material = material.mix(material.lit(t.Dark), lift)
	material.paint(tile).Shadow(0, 1, 3, 0, material.cast)
	return control{Element: tile, mute: mute, lift: lift}
}

// amount is a state as the value its animations run to.
func amount(on bool) float32 {
	if on {
		return 1
	}
	return 0
}

// shortName is a device name without the adapter Windows adds in brackets:
// "耳机 (Realtek USB Audio)" is "耳机", which is what a tile shows. The
// picker shows the whole name.
func shortName(name string) string {
	for _, bracket := range []string{" (", "（"} {
		if i := strings.Index(name, bracket); i > 0 {
			return name[:i]
		}
	}
	return name
}
