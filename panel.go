package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The panel is a card of the material the one on the desktop is made of,
// a row per setting: as wide as two tiles and the padding between them.
const (
	panelWidth      = 288
	panelHeight     = 150
	panelNoteHeight = 174
	panelPadding    = 12
	panelRow        = 38
	panelRadius     = 14
	capRadius       = 6
)

// settingsView draws the panel: whether the tool starts with the user's
// session, and what it answers to.
func (a *app) settingsView(c *ui.Context) {
	t := c.Theme()
	card := cardPlate(t).paint(ui.Column(c).Padding(panelPadding, 14).Radius(panelRadius))
	card.Children(func() {
		a.startRow(c)
		divider(c, t)
		a.keyRow(c, keyMute, "闭麦")
		a.keyRow(c, keyOutput, "切换输出")
		if a.note != "" {
			a.noteRow(c, t)
		}
	})
}

// startRow is whether the tool starts with the user's session.
func (a *app) startRow(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(panelRow).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Text(c, "开机自启").FontSize(13).TextColor(t.Text)
		a.startSwitch(c)
	})
}

// startSwitch is the switch of that row: a track that lights as the tool
// starts with the session, and a knob that slides to the end it is on.
func (a *app) startSwitch(c *ui.Context) {
	t := c.Theme()
	track := ui.Box(c).Size(34, 20).Radius(10).Cursor(ui.CursorPointer).Label("开机自启")
	on := track.AnimateTo("on", amount(a.autoStart), pressSpring)
	track.Background(t.Text.Alpha(0.12).Mix(t.Accent, on)).
		InsetShadow(0, 1, 2, 0, ui.RGB(0, 0, 0).Alpha(0.3*(1-on)))
	track.OnClick(func() { a.setAutoStart(!a.autoStart) })
	track.Children(func() {
		ui.Box(c).Size(16, 16).Radius(8).Background(ui.RGB(255, 255, 255)).
			Shadow(0, 1, 2, 0, ui.RGB(0, 0, 0).Alpha(0.35)).
			Absolute().Top(2).Left(2 + 14*on)
	})
}

// keyRow is a shortcut: the row says what it does, its cap what the tool
// answers to.
func (a *app) keyRow(c *ui.Context, i int, label string) {
	t := c.Theme()
	ui.Row(c).Height(panelRow).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(t.Text)
		a.keyCap(c, i, label)
	})
}

// keyCap is the shortcut itself: a cap that takes the keys pressed while
// the panel listens for one, and shows what the tool answers to otherwise.
func (a *app) keyCap(c *ui.Context, i int, label string) {
	t := c.Theme()
	listening := a.listens(i)

	value, ink := a.keys[i].chord, t.Text
	switch {
	case listening:
		value = "按下组合键"
	case value == "":
		value, ink = "未设置", t.TextMuted
	}

	cap := ui.Row(c).Height(23).Padding(0, 8).Radius(capRadius).AlignItems(ui.Center).
		Cursor(ui.CursorPointer).Focusable().Label(label + "快捷键")
	if listening {
		// Held there every frame the panel listens, so that the keys the
		// user presses land on the cap.
		cap = cap.Focus()
	}
	lift := cap.AnimateTo("lift", amount(cap.Hovered()), liftSpring)
	cap.Translate(0, -lift)

	material := capPlate(t, ui.Color{})
	if listening {
		// The cap lights in the accent color while it listens, as a tile
		// turns to the danger color while the microphone is muted.
		material = capPlate(t, t.Accent)
	}
	material.paint(cap).Shadow(0, 1, 3, 0, material.cast)

	cap.OnClick(func() { a.listen(i) })
	cap.HandleInput(func(ev ui.InputEvent) bool { return a.typed(i, ev) })
	cap.Children(func() {
		ui.Text(c, value).FontSize(12).TextColor(ink).SingleLine()
	})
}

// noteRow says what the panel could not do, in one line under the rows.
func (a *app) noteRow(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(6).AlignItems(ui.Start).Padding(6, 0, 0, 0).Children(func() {
		ui.Box(c).Size(5, 5).Radius(3).Background(t.Warning).Margin(6, 0, 0, 0)
		ui.Text(c, a.note).FontSize(12).TextColor(t.Warning)
	})
}

// capPlate is the material of a shortcut: a tile of the card, a shade
// deeper than the panel in the light appearance, where a tile's face is
// nearly the panel's.
func capPlate(t *ui.Theme, tint ui.Color) plate {
	p := tilePlate(t, tint)
	if t.Dark || tint != (ui.Color{}) {
		return p
	}
	p.face = ui.LinearGradient{Angle: 180, From: ui.Hex("#f7f7fa"), To: ui.Hex("#ebebf1")}
	return p
}

// divider parts the setting the tool starts with from its shortcuts.
func divider(c *ui.Context, t *ui.Theme) {
	ui.Box(c).Height(1).Background(t.Text.Alpha(0.08)).Margin(5, 0)
}

// listen has the panel wait for a key to make the shortcut of row i, or
// stop waiting where it waits for it already.
func (a *app) listen(i int) {
	if a.listens(i) {
		a.listening = 0
	} else {
		a.listening = i + 1
	}
	a.note = ""
	a.invalidateSettings()
}

// listens reports whether the panel waits for a shortcut on row i.
func (a *app) listens(i int) bool {
	return a.listening == i+1
}

// typed takes a key pressed on the cap of row i while the panel listens for
// one: Escape leaves the shortcut as it was, Backspace and Delete clear it,
// and any other key with Ctrl, Alt or Win on it becomes it.
func (a *app) typed(i int, ev ui.InputEvent) bool {
	if !a.listens(i) || ev.Kind != ui.InputKeyDown || ev.Repeat {
		return false
	}
	a.listening = 0
	switch p := pressOf(ev); {
	case p.cancel:
		a.note = ""
	case p.clear:
		a.setKey(i, "")
	case !p.ok:
		// One line of advice, in place of a press that does nothing.
		a.note = "要带 Ctrl、Alt 或 Win"
	default:
		a.setKey(i, p.chord)
	}
	a.invalidateSettings()
	return true
}

// press is what a key pressed on a cap does to the shortcut there.
type press struct {
	chord  string
	ok     bool // a shortcut can be made of it
	clear  bool
	cancel bool
}

// pressOf reads a key pressed on a cap.
func pressOf(ev ui.InputEvent) press {
	switch {
	case ev.Key == ui.KeyEscape && ev.Mods == 0:
		return press{cancel: true}
	case (ev.Key == ui.KeyBackspace || ev.Key == ui.KeyDelete) && ev.Mods == 0:
		return press{clear: true}
	}
	chord, ok := chordOf(ev)
	return press{chord: chord, ok: ok}
}

// setAutoStart starts the tool with the user's session, or stops it, and
// says so where the system will not.
func (a *app) setAutoStart(on bool) {
	if err := mygo.App.SetOpenAtLogin(on); err != nil {
		log.Print("starting at login: ", err)
		a.note = "无法写入启动项"
		a.invalidateSettings()
		return
	}
	a.autoStart, a.note = on, ""
	a.invalidateSettings()
}
