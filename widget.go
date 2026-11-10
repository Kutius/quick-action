package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The card is one row: the microphone and the output, each a zone of it.
const (
	widgetWidth  = 200
	widgetHeight = 48
)

// The card lays its zones out in a row of that padding and gap, with a
// hairline between them; each zone is as tall as the row, and the microphone
// one keeps its width so that the output does not move as the microphone
// changes state.
//
// cardRadius is the radius Windows rounds the window itself to. DWM cuts a
// frameless window to the corners of its own windows, whatever radius it is
// given, so the card draws the same radius to meet the edge it is cut at;
// docs/window-corners.md has the alternatives and their tradeoffs.
const (
	cardPadding = 8
	cardGap     = 4
	cardRadius  = 8
	zoneHeight  = 32
	zoneRadius  = 8
	zonePadding = 7
	zoneGap     = 4
	micWidth    = 72
	outputIcon  = 16
	chevronBox  = 16
	// nameWidth is what the output's name may take: the output zone's width,
	// without its padding, its icon, the chevron and the gaps between them.
	nameWidth = widgetWidth - 2*cardPadding - 2*cardGap - 1 - micWidth -
		2*zonePadding - outputIcon - chevronBox - 3*zoneGap
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

// view draws the card: the microphone and the output, each a zone that does
// its switch where it is clicked. The card's edge moves the window, as a
// panel of macOS moves by what it is not, so that the zones stay free to be
// switches.
func (a *app) view(c *ui.Context) {
	// One read of what the audio does for the whole frame, so that the two
	// zones cannot show two different states.
	st, err := a.snapshot()
	t := c.Theme()
	p := cardPlate(t)
	// The face runs under the card too: Windows rounds the window itself,
	// and a corner it rounds a hair wider than the card would otherwise
	// show the background behind it.
	ui.Box(c).Fill().LinearGradient(p.face).Children(func() {
		a.card(c, st, err, t, p)
	})
}

// card draws the card: the microphone and the output, each a zone that does
// its switch where it is clicked. The card's edge moves the window, as a
// panel of macOS moves by what it is not, so that the zones stay free to be
// switches.
func (a *app) card(c *ui.Context, st audioState, err error, t *ui.Theme, p plate) {
	card := p.paint(ui.Row(c).Fill().
		Padding(cardPadding).Gap(cardGap).AlignItems(ui.Center).Radius(cardRadius))
	card.Cursor(ui.CursorGrab).DragWindow().Children(func() {
		a.micZone(c, st, err)
		cardDivider(c, t)
		a.outputZone(c, st, err)
	})
}

// cardDivider parts the microphone from the output.
func cardDivider(c *ui.Context, t *ui.Theme) {
	ui.Box(c).Width(1).Height(16).Background(t.Text.Alpha(0.10))
}

// micZone is the microphone: a click mutes or unmutes it. The zone turns to
// the danger color as it does, and the microphone in it gives way to the
// one with a line through it, both on the way the spring carries.
func (a *app) micZone(c *ui.Context, st audioState, err error) {
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
		value, valueColor = "未找到", t.TextMuted
	}

	zone, ctl := cardZone(c, t.Danger, muted, "切换麦克风")
	zone.Width(micWidth)
	mute := ctl.mute
	ink := t.Text.Mix(t.Danger, mute)

	zone.OnClick(a.toggle).Children(func() {
		// The two microphones sit in one place, one fading in as the other
		// fades out, so that the zone changes rather than blinks.
		ui.Box(c).Size(17, 17).Children(func() {
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
				icon.Children(func() { ui.Icon(c, glyph.svg).FontSize(17).TextColor(ink) })
			}
		})
		ui.Text(c, value).FontSize(11).TextColor(valueColor.Mix(t.Danger, mute)).SingleLine().Ellipsis("…")
	})
}

// outputZone is the output: a click switches to the one the user was on
// before, and the arrow picks another.
func (a *app) outputZone(c *ui.Context, st audioState, err error) {
	t := c.Theme()
	value, valueColor := shortName(st.Output.Name), t.Text
	switch {
	case err != nil:
		value, valueColor = "不可用", t.TextMuted
	case value == "":
		value, valueColor = "未找到", t.TextMuted
	}

	zone, ctl := cardZone(c, ui.Color{}, false, "切换输出")
	zone.Grow(1)

	zone.OnClick(a.switchOutput).Children(func() {
		ui.Icon(c, speakerSVG).FontSize(outputIcon).TextColor(t.Text)
		ui.Text(c, value).FontSize(11).TextColor(valueColor).SingleLine().
			Ellipsis("…").MaxWidth(nameWidth)
		ui.Spacer(c)
		// The way to another output, faint at rest and lit with the pointer,
		// as the zone washes.
		if st.Outputs > 1 {
			more := ui.Box(c).Size(chevronBox, chevronBox).Radius(5).Center().
				Cursor(ui.CursorPointer).Label("选择输出").Opacity(0.55 + 0.45*ctl.lift)
			if more.Hovered() {
				more.Background(t.Text.Alpha(0.08))
			}
			more.OnClick(a.chooseOutput).Children(func() {
				ui.Icon(c, chevronSVG).FontSize(11).TextColor(t.TextMuted)
			})
		}
	})
}

// control is what a zone's motion runs on: how far it has turned to the
// danger color, and how far the pointer has washed it.
type control struct {
	mute, lift float32
}

// cardZone is a zone of the card: the pointer washes it, a press deepens the
// wash and sets it down, and a state that is on, as a muted microphone, tints
// it. The element comes back on its own, rather than in control: a build
// value kept in a struct outlives the frame that made it.
func cardZone(c *ui.Context, tint ui.Color, muted bool, label string) (ui.Element, control) {
	t := c.Theme()
	zone := ui.Row(c).Height(zoneHeight).Gap(zoneGap).Padding(0, zonePadding).
		AlignItems(ui.Center).Radius(zoneRadius).
		Cursor(ui.CursorPointer).Label(label)
	mute := zone.AnimateTo("mute", amount(muted), stateFade)
	lift := zone.AnimateTo("lift", amount(zone.Hovered()), liftSpring)
	press := zone.AnimateTo("press", amount(zone.Pressed()), pressSpring)

	// The wash of the pointer, deeper under a press; a state that is on
	// tints the zone, and the wash deepens that tint.
	wash := 0.05*lift + 0.06*press
	background := t.Text.Alpha(wash)
	if tint != (ui.Color{}) {
		background = background.Mix(tint.Alpha(0.13+wash), mute)
	}
	zone.Background(background)

	// The press sets the zone down, from the value the spring carries rather
	// than towards one that is itself still moving, which would lag.
	set := 1 - 0.03*press
	zone.Scale(set, set)
	return zone, control{mute: mute, lift: lift}
}

// amount is a state as the value its animations run to.
func amount(on bool) float32 {
	if on {
		return 1
	}
	return 0
}

// shortName is a device name without the adapter Windows adds in brackets:
// "耳机 (Realtek USB Audio)" is "耳机", which is what the card shows. The
// picker shows the whole name.
func shortName(name string) string {
	for _, bracket := range []string{" (", "（"} {
		if i := strings.Index(name, bracket); i > 0 {
			return name[:i]
		}
	}
	return name
}
