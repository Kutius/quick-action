package main

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// app is the tool: the microphones and outputs it switches, the card on the
// desktop that shows them, the icon in the notification area that brings the
// card back, and the panel of settings behind the icon.
type app struct {
	audio audioController

	// mu guards what the audio does and what the card remembers of it. The
	// watcher writes it, the view draws it on the main thread, and the
	// shortcuts, the icon and the menu read it from threads of their own,
	// so none of them takes it without the lock.
	mu       sync.Mutex
	state    audioState
	err      error
	previous string // the output the user was on before this one
	lastSeen string // the output the last read found, whoever switched it

	// pending is what the tool remembers waiting for the goroutine that
	// writes it, and saving wakes that goroutine, so that no frame waits
	// for the disk.
	pending settings
	unsaved bool
	saving  chan struct{}

	// switching is set while a switch is under way, so that a double click
	// switches once.
	switching atomic.Bool
	// quitting is set as the app quits, so that the watcher stops handing
	// results to a window that is going away, and closing it lets it go.
	quitting atomic.Bool
	// hiddenByGame is set while the card is out of the way of an app that
	// covers the screen, and hiddenByUser while the user hid it.
	hiddenByGame atomic.Bool
	hiddenByUser atomic.Bool

	win *mygo.Window
	// hwnd is the native window of the card, kept here so that asking the
	// card to step out of a game's way never waits for the main thread.
	hwnd    uintptr
	tray    *mygo.Tray
	mute    *mygo.MenuItem
	outItem *mygo.MenuItem
	panel   *mygo.MenuItem

	// keys are the shortcuts the tool answers to, in the order of the rows
	// of the panel, and settingsWin the panel itself.
	keys        [2]hotkey
	settingsWin *mygo.Window

	// autoStart is whether the tool starts with the user's session, as the
	// system last said, note what the panel says went wrong, and listening
	// the row it waits for a shortcut on, plus one.
	autoStart bool
	note      string
	listening int

	// shownMuted and shownTip are what the icon shows, and lastShown what
	// the card draws, so that a read that finds nothing new redraws
	// nothing. The watcher's goroutine alone touches them.
	shownMuted bool
	shownTip   string
	shownLabel string
	lastShown  cardState
	shown      bool

	// wake asks the watcher to read the audio again.
	wake chan struct{}
}

func main() {
	a := &app{wake: make(chan struct{}, 1), saving: make(chan struct{}, 1)}
	useUpdater()
	mygo.App.WhenReady(a.ready)
	// The tool lives in the notification area: hiding the card, or closing
	// it, leaves it running.
	mygo.App.OnWindowAllClosed(func() {})
	// Before the windows close: the card cancels its own close, and must
	// not cancel the app's.
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { a.quitting.Store(true) })
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		if a.tray != nil {
			a.tray.Destroy()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func (a *app) ready() {
	if c, err := newAudioController(); err != nil {
		log.Print("audio: ", err)
		a.audio = brokenAudio{err}
	} else {
		a.audio = c
	}
	// Read once before the card opens, so that it shows the audio rather
	// than an empty card for its first frame.
	st, err := a.audio.State()
	saved := loadSettings()
	a.mu.Lock()
	a.state, a.err = st, err
	a.lastSeen, a.previous = st.Output.ID, saved.PreviousOutput
	a.pending = saved
	a.mu.Unlock()

	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:         "Quick Action",
		Width:         widgetWidth,
		Height:        widgetHeight,
		MinWidth:      widgetWidth,
		MaxWidth:      widgetWidth,
		MinHeight:     widgetHeight,
		MaxHeight:     widgetHeight,
		Frameless:     true,
		Panel:         true,
		CornerRadius:  14,
		DisableResize: true,
		// Opens where the user left it, and stays where they drag it.
		StateKey: "widget",
		Content:  ui.View(a.view),
	})
	// Closing the card hides it: the tray icon keeps the tool at hand.
	a.win.OnClose(func(e *mygo.CloseEvent) {
		if a.quitting.Load() {
			return
		}
		e.PreventDefault()
		a.win.Hide()
		a.panelShown(false)
	})

	a.setUpKeys(saved.Hotkeys)
	a.setUpTray()
	a.hwnd = a.win.NativeHandle()
	a.avoidGames()
	watchForeground(a.avoidGames)
	go a.writeSettings()
	go a.watch()
}

// snapshot is what the audio does, as the last read found it.
func (a *app) snapshot() (audioState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state, a.err
}

// previousOutput is the output the user was on before this one, which a
// click on the card switches back to.
func (a *app) previousOutput() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.previous
}

func (a *app) setUpTray() {
	st, audioErr := a.snapshot()
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:      "mic",
		Icon:    trayIconPNG(st.Mic.Muted),
		ToolTip: trayTip(st, audioErr),
	})
	if err != nil {
		log.Print("notification area: ", err)
		return
	}
	a.tray = tray
	a.shownMuted, a.shownTip = st.Mic.Muted, trayTip(st, audioErr)
	a.shownLabel = switchLabel(st.Mic.Muted)
	// What the icon is for: a click on it shows the card, or puts it
	// away again, as the icons of the notification area do.
	tray.OnClick(a.togglePanel)

	a.mute = &mygo.MenuItem{
		Label:    switchLabel(st.Mic.Muted),
		Subtitle: a.keys[keyMute].chord,
		Click:    func(*mygo.MenuItem, *mygo.Window) { a.toggle() },
	}
	a.outItem = &mygo.MenuItem{
		Label:    "切换输出",
		Subtitle: a.keys[keyOutput].chord,
		Click:    func(*mygo.MenuItem, *mygo.Window) { a.switchOutput() },
	}
	a.panel = &mygo.MenuItem{
		ID: "panel", Label: "显示面板", Type: mygo.MenuItemCheckbox, Checked: true,
		Click: func(*mygo.MenuItem, *mygo.Window) { a.togglePanel() },
	}
	tray.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		a.mute,
		a.outItem,
		{Label: "选择输出…", Click: func(*mygo.MenuItem, *mygo.Window) { a.chooseOutput() }},
		mygo.Separator(),
		a.panel,
		{Label: "设置…", Click: func(*mygo.MenuItem, *mygo.Window) { a.openSettings() }},
		{Label: "检查更新…", Click: func(*mygo.MenuItem, *mygo.Window) { a.checkUpdates() }},
		mygo.Separator(),
		{Label: "退出", Role: mygo.RoleQuit},
	}))
}

// checkUpdates looks for a newer version, in front of the card: the window
// that offers it must not open behind it.
func (a *app) checkUpdates() {
	mygo.App.Focus()
	checkForUpdates()
}

// openSettings shows the panel next to the icon of the notification area,
// asking the system what it says about the tool as it does: it starts the
// tool at login, or it does not.
func (a *app) openSettings() {
	if a.quitting.Load() || a.settingsWin != nil {
		return
	}
	a.listening = 0
	a.autoStart = mygo.App.OpenAtLogin()
	a.note = a.keyNote()
	opts := mygo.FlyoutOptions{
		Placement: mygo.PlacementTopEnd,
		Gap:       8,
		Width:     panelWidth,
		Height:    a.panelSize(),
		Focusable: true,
		Shadow:    true,
		Content:   ui.View(a.settingsView),
	}
	switch {
	case a.tray != nil:
		opts.Tray = a.tray
	case a.win != nil:
		// Without an icon to hang it on, the panel opens beside the card.
		opts.Parent = a.win
		opts.Placement = mygo.PlacementBottomStart
		opts.Anchor = mygo.Rectangle{Width: widgetWidth, Height: 1}
	default:
		return
	}
	a.settingsWin = mygo.NewFlyout(opts)
	a.settingsWin.OnClosed(func() { a.settingsWin = nil })
}

// invalidateSettings draws the panel again, for what it shows changed: the
// line it says a shortcut is taken on makes it taller.
func (a *app) invalidateSettings() {
	if a.settingsWin == nil || a.quitting.Load() {
		return
	}
	a.settingsWin.SetSize(panelWidth, a.panelSize())
	a.settingsWin.Invalidate()
}

// panelSize is how tall the panel is: taller with the line that says why a
// shortcut is off.
func (a *app) panelSize() int {
	if a.note != "" {
		return panelNoteHeight
	}
	return panelHeight
}

// remember changes what the tool keeps between runs, and has the writer
// write it: the thread that draws must not wait for the disk.
func (a *app) remember(change func(*settings)) {
	a.mu.Lock()
	change(&a.pending)
	a.unsaved = true
	a.mu.Unlock()
	select {
	case a.saving <- struct{}{}:
	default:
	}
}

// togglePanel shows the card, or puts it away, as a click on the icon of
// the notification area does, and tells the menu which it does.
func (a *app) togglePanel() {
	if a.win == nil || a.quitting.Load() {
		return
	}
	if a.win.IsVisible() {
		a.hiddenByUser.Store(true)
		a.win.Hide()
		a.panelShown(false)
		return
	}
	a.hiddenByUser.Store(false)
	a.win.Show()
	a.panelShown(true)
}

// panelShown tells the menu of the icon whether the card shows.
func (a *app) panelShown(shown bool) {
	if a.panel != nil {
		a.panel.SetChecked(shown)
	}
}

// toggle mutes or unmutes the microphones, and tells the card and the icon
// what they do once they did.
func (a *app) toggle() {
	if a.switching.Swap(true) {
		return
	}
	st, _ := a.snapshot()
	muted := !st.Mic.Muted
	go func() {
		defer a.switching.Store(false)
		if err := a.audio.SetMuted(muted); err != nil {
			log.Print("switching the microphone: ", err)
		}
		a.wakeUp()
	}()
}

// switchOutput switches to the output the user was on before this one: the
// one click of the card. Without one, it asks which.
func (a *app) switchOutput() {
	st, _ := a.snapshot()
	previous, current := a.previousOutput(), st.Output.ID
	if previous == "" || previous == current {
		a.chooseOutput()
		return
	}
	go func() {
		if err := a.audio.SetOutput(previous); err != nil {
			// The output the user came from is gone: ask which instead.
			log.Print("switching the output: ", err)
			a.chooseOutput()
			return
		}
		a.wakeUp()
	}()
}

// setOutput makes an output the one the computer plays to.
func (a *app) setOutput(id string) {
	if st, _ := a.snapshot(); id == "" || id == st.Output.ID {
		return
	}
	go func() {
		if err := a.audio.SetOutput(id); err != nil {
			log.Print("switching the output: ", err)
		}
		a.wakeUp()
	}()
}

// chooseOutput lists the outputs for the user to pick one.
func (a *app) chooseOutput() {
	st, _ := a.snapshot()
	current := st.Output.ID
	go func() {
		outputs, err := a.audio.Outputs()
		if err != nil {
			log.Print("listing the outputs: ", err)
			return
		}
		if len(outputs) < 2 {
			return
		}
		items := make([]*mygo.MenuItem, 0, len(outputs))
		for _, out := range outputs {
			id := out.ID
			items = append(items, &mygo.MenuItem{
				Label:   out.Name,
				Type:    mygo.MenuItemRadio,
				Checked: id == current,
				Click:   func(*mygo.MenuItem, *mygo.Window) { a.setOutput(id) },
			})
		}
		// The system's own menu, which may grow past the card.
		mygo.NewMenu(items).Popup(a.win)
	}()
}

// wakeUp asks the watcher to read the audio again.
func (a *app) wakeUp() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

// watch reads the audio once a second, and as soon as a switch ends, so that
// the card and the icon follow what other apps, the system's own settings
// and the devices themselves do. A game in fullscreen is not polled for:
// watchForeground hears of it instead.
func (a *app) watch() {
	audio := time.NewTicker(time.Second)
	defer audio.Stop()
	a.refresh()
	for {
		select {
		case <-audio.C:
			a.refresh()
		case <-a.wake:
			a.refresh()
		}
		if a.quitting.Load() {
			return
		}
	}
}

// avoidGames takes the card out of the way of an app that covers its
// display, as a game in fullscreen does, and brings it back after, unless
// the user hid it. Windows calls it as the window in front changes.
func (a *app) avoidGames() {
	if a.hwnd == 0 || a.quitting.Load() {
		return
	}
	if !screenTaken(a.hwnd) {
		if a.hiddenByGame.Swap(false) && !a.hiddenByUser.Load() {
			// Shown without taking the focus from what the user went back
			// to.
			a.win.ShowInactive()
			a.panelShown(true)
		}
		return
	}
	if !a.hiddenByGame.Swap(true) {
		a.win.Hide()
		a.panelShown(false)
	}
}

func (a *app) refresh() {
	st, err := a.audio.State()
	a.publish(st, err)
}

// cardState is what the card draws: the audio, and whether reading it
// failed.
type cardState struct {
	audio  audioState
	failed bool
}

// publish shows a state on the icon and hands it to the view, which draws
// again only when there is something new to draw.
func (a *app) publish(st audioState, err error) {
	a.showTray(st, err)

	next := cardState{audio: st, failed: err != nil}
	redraw := !a.shown || next != a.lastShown
	a.lastShown, a.shown = next, true

	a.mu.Lock()
	// A switch, whoever made it, makes the output it left the one a click
	// goes back to.
	switched := st.Output.ID != "" && a.lastSeen != "" && st.Output.ID != a.lastSeen
	if switched {
		a.previous = a.lastSeen
		a.pending.PreviousOutput = a.previous
		a.unsaved = true
	}
	if st.Output.ID != "" {
		a.lastSeen = st.Output.ID
	}
	a.state, a.err = st, err
	a.mu.Unlock()

	if switched {
		// The writer takes the newest settings: it has the ones before, and
		// a read that follows this one replaces them again.
		select {
		case a.saving <- struct{}{}:
		default:
		}
	}
	if redraw && a.win != nil && !a.quitting.Load() {
		a.win.Invalidate()
	}
}

// writeSettings keeps what the tool remembers between runs, off the main
// thread, which must not wait for the disk while it draws.
func (a *app) writeSettings() {
	for range a.saving {
		a.mu.Lock()
		s, unsaved := a.pending, a.unsaved
		a.unsaved = false
		a.mu.Unlock()
		if unsaved {
			saveSettings(s)
		}
	}
}

func (a *app) showTray(st audioState, err error) {
	if a.tray == nil {
		return
	}
	if st.Mic.Muted != a.shownMuted {
		a.shownMuted = st.Mic.Muted
		if err := a.tray.SetIcon(trayIconPNG(st.Mic.Muted), false); err != nil {
			log.Print("tray icon: ", err)
		}
	}
	if tip := trayTip(st, err); tip != a.shownTip {
		a.shownTip = tip
		a.tray.SetToolTip(tip)
	}
	if label := switchLabel(st.Mic.Muted); a.mute != nil && label != a.shownLabel {
		a.shownLabel = label
		a.mute.SetLabel(label)
	}
}

// switchLabel names what a switch does, for the menu of the icon.
func switchLabel(muted bool) string {
	if muted {
		return "取消静音"
	}
	return "静音麦克风"
}

// trayTip says what the audio does, in the tooltip of the icon.
func trayTip(st audioState, err error) string {
	switch {
	case err != nil:
		return "麦克风不可用"
	case st.Mic.Muted:
		return "麦克风已静音"
	}
	return "麦克风开启"
}
