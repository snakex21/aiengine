// menu_runtime.go - hosts the data-driven menus (package ui) and player
// settings (package settings): title screen, pause, settings applied live,
// level select, keyboard/mouse/gamepad navigation.
//
// A project opts in with `MENU menu.aie` in manifest.aie. Without it the
// legacy pause toggle (Esc → onPause event) is unchanged.

package main

import (
	"log"
	"math"
	"os"

	"github.com/go-gl/glfw/v3.3/glfw"

	"aiengine/parser"
	"aiengine/settings"
	"aiengine/systems"
	"aiengine/ui"
)

type menuClick struct {
	x, y  float64
	right bool
}

// menuState is the App's menu/settings bookkeeping.
type menuState struct {
	menu           *ui.Menu
	settings       settings.Settings
	settingsPath   string
	settingsActive bool // apply settings (project has a menu or a settings file)
	inputs         []ui.Input
	clicks         []menuClick
	padPrev        map[glfw.Joystick][16]bool
	currentLevel   parser.LevelEntry
}

// loadSettings reads settings.aie from the project (window size from
// config.aie is the default) and returns the window size to create.
func (app *App) loadSettings(cfgW, cfgH int) (int, int) {
	app.ms.settingsPath = projectPath(settings.FileName)
	s, err := settings.Load(app.ms.settingsPath, settings.Defaults(cfgW, cfgH))
	if err != nil {
		log.Printf("Warning: settings: %v", err)
	}
	app.ms.settings = s
	_, statErr := os.Stat(app.ms.settingsPath)
	app.ms.settingsActive = statErr == nil || activeManifest.Menu != ""
	if !app.ms.settingsActive {
		return cfgW, cfgH
	}
	log.Printf("Settings: %+v", s)
	if s.Native() {
		return cfgW, cfgH // resized to the monitor once GLFW is up (applyVideo)
	}
	return s.Width, s.Height
}

// initMenu loads the project's menu file and opens the title screen.
func (app *App) initMenu() {
	if activeManifest.Menu == "" {
		return
	}
	def, err := ui.ParseFile(projectPath(activeManifest.Menu))
	if err != nil {
		log.Printf("Warning: menu: %v (menus disabled)", err)
		return
	}
	for _, sd := range def.SoundDefs {
		app.audioSys.RegisterSoundDef(systems.SoundDefFromProps(sd.Name, sd.Props))
	}
	app.ms.menu = ui.New(def, activeManifest.LevelList(), &app.ms.settings, activeProjectDir)
	app.ms.menu.OnSound = app.audioSys.PlaySound
	app.ms.padPrev = map[glfw.Joystick][16]bool{}
	log.Printf("Menu: %d screen(s) from %s", len(def.Order), activeManifest.Menu)
	if def.Screens["main"] != nil {
		app.openMenu("main")
	}
}

// menuActive reports whether a menu screen is shown (gameplay frozen).
func (app *App) menuActive() bool { return app.ms.menu != nil && app.ms.menu.Active() }

func (app *App) openMenu(name string) {
	m := app.ms.menu
	if m == nil || !m.Has(name) {
		return
	}
	m.OpenRoot(name)
	app.onMenuScreenChanged()
	app.window.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
	app.scriptEngine.Emit("onPause", true)
}

func (app *App) closeMenu() {
	if app.ms.menu != nil {
		app.ms.menu.Close()
	}
	app.ms.inputs, app.ms.clicks = nil, nil
	app.window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)
	app.mouseInit = false
	// Back to the level's own music (whatever scripts last asked for).
	if track := app.scriptEngine.CurrentMusic; track != "" {
		app.audioSys.PlayMusicTrack(track)
	} else {
		app.audioSys.StopMusicTrack()
	}
	app.scriptEngine.Emit("onPause", false)
}

// onMenuScreenChanged applies per-screen music and map-view camera.
func (app *App) onMenuScreenChanged() {
	s := app.ms.menu.Current()
	if s == nil {
		return
	}
	if s.Music != "" {
		app.audioSys.PlayMusicTrack(s.Music)
	}
	if s.Background.Kind == "map" && len(s.Background.Args) >= 2 {
		app.player.X, app.player.Y = s.Background.Args[0], s.Background.Args[1]
		if len(s.Background.Args) >= 3 {
			app.player.Angle = s.Background.Args[2] * math.Pi / 180
		}
		app.player.Pitch = 0
	}
}

// menuFrame processes queued menu input once per frame.
func (app *App) menuFrame(dt float64) {
	m := app.ms.menu
	if m == nil {
		return
	}
	app.pollGamepads()
	if !m.Active() {
		app.ms.inputs, app.ms.clicks = nil, nil
		return
	}
	sw, sh := app.raycaster.Size()
	ww, wh := app.window.GetSize()
	toBuf := func(x, y float64) (int, int) {
		if ww <= 0 || wh <= 0 {
			return int(x), int(y)
		}
		return int(x * float64(sw) / float64(ww)), int(y * float64(sh) / float64(wh))
	}
	before := m.ScreenName()
	cx, cy := app.window.GetCursorPos()
	if cx != app.prevMouseX || cy != app.prevMouseY {
		bx, by := toBuf(cx, cy)
		m.Hover(bx, by, sw, sh)
		app.prevMouseX, app.prevMouseY = cx, cy
	}
	var acts []ui.Action
	for _, in := range app.ms.inputs {
		if a, ok := m.Input(in); ok {
			acts = append(acts, a)
		}
	}
	for _, c := range app.ms.clicks {
		bx, by := toBuf(c.x, c.y)
		if a, ok := m.Click(bx, by, sw, sh, c.right); ok {
			acts = append(acts, a)
		}
	}
	app.ms.inputs, app.ms.clicks = nil, nil
	for _, a := range acts {
		app.menuAction(a)
	}
	if m.Active() {
		if m.ScreenName() != before {
			app.onMenuScreenChanged()
		}
		// Slow pan for "map" backgrounds.
		if s := m.Current(); s != nil && s.Background.Kind == "map" {
			app.player.Angle += 0.08 * dt
		}
	}
}

// menuAction carries out a host action returned by the menu.
func (app *App) menuAction(a ui.Action) {
	log.Printf("[Menu] action %s %s", a.Kind, a.Arg)
	switch a.Kind {
	case "resume":
		app.closeMenu()
	case "new_game":
		levels := activeManifest.LevelList()
		app.startLevelEntry(levels[0])
		app.closeMenu()
	case "load_level":
		if l, ok := activeManifest.FindLevel(a.Arg); ok {
			app.startLevelEntry(l)
		} else {
			log.Printf("[Menu] unknown level %q", a.Arg)
		}
		app.closeMenu()
	case "restart":
		if app.game != nil {
			app.restartLevel()
		}
		app.closeMenu()
	case "main_menu":
		app.onMenuScreenChanged()
	case "quit":
		app.window.SetShouldClose(true)
	case "run_script":
		if app.luaBridge != nil {
			if err := app.luaBridge.Call(a.Arg); err != nil {
				log.Printf("[Menu] run_script %s: %v", a.Arg, err)
			}
		}
	case "settings_changed":
		app.applySettings()
		app.saveSettings()
	}
}

// startLevelEntry loads and starts one of the project's levels.
func (app *App) startLevelEntry(l parser.LevelEntry) {
	if app.game == nil {
		return // legacy demo: no level runtime to load into
	}
	app.ms.currentLevel = l
	app.restartLevel()
}

func (app *App) saveSettings() {
	if err := app.ms.settings.Save(app.ms.settingsPath); err != nil {
		log.Printf("Warning: saving settings: %v", err)
	}
}

// applySettings pushes settings into audio, camera and video.
func (app *App) applySettings() {
	if !app.ms.settingsActive {
		return
	}
	s := app.ms.settings
	app.audioSys.SetVolumes(s.MasterVolume, s.MusicVolume, s.SfxVolume)
	if app.player != nil {
		app.player.FOV = s.FOV * math.Pi / 180
	}
	app.applyVideo()
}

// applyVideo sets the internal resolution and window/fullscreen mode.
func (app *App) applyVideo() {
	s := app.ms.settings
	monitor := glfw.GetPrimaryMonitor()
	mode := monitor.GetVideoMode()
	outW, outH := s.Output(mode.Width, mode.Height)
	if s.Fullscreen {
		outW, outH = mode.Width, mode.Height // fullscreen always uses the monitor mode
		if !s.Native() {
			outW, outH = s.Width, s.Height // ...but renders the chosen size, stretched
		}
	}
	rw, rh := s.RenderSize(outW, outH)
	app.raycaster.Resize(int32(rw), int32(rh))
	switch {
	case s.Fullscreen && !app.fullscreen:
		app.windowedX, app.windowedY = app.window.GetPos()
		app.window.SetMonitor(monitor, 0, 0, mode.Width, mode.Height, mode.RefreshRate)
		app.fullscreen = true
	case !s.Fullscreen && app.fullscreen:
		app.window.SetMonitor(nil, app.windowedX, app.windowedY, outW, outH, 0)
		app.fullscreen = false
	case !s.Fullscreen:
		if w, h := app.window.GetSize(); w != outW || h != outH {
			app.window.SetSize(outW, outH)
			app.window.SetPos(max(0, (mode.Width-outW)/2), max(0, (mode.Height-outH)/2))
		}
	}
	log.Printf("Video: output %dx%d fullscreen=%v, render %dx%d (%.0f%%), monitor %dx%d",
		outW, outH, s.Fullscreen, rw, rh, s.RenderScale*100, mode.Width, mode.Height)
	app.windowedW, app.windowedH = outW, outH
}

// menuKey maps a key press to a menu input.
func menuKey(key glfw.Key) (ui.Input, bool) {
	switch key {
	case glfw.KeyUp, glfw.KeyW:
		return ui.Up, true
	case glfw.KeyDown, glfw.KeyS:
		return ui.Down, true
	case glfw.KeyLeft, glfw.KeyA:
		return ui.Left, true
	case glfw.KeyRight, glfw.KeyD:
		return ui.Right, true
	case glfw.KeyEnter, glfw.KeyKPEnter, glfw.KeySpace:
		return ui.Accept, true
	case glfw.KeyEscape, glfw.KeyBackspace:
		return ui.Back, true
	}
	return 0, false
}

// pollGamepads turns gamepad buttons/stick into menu inputs, and Start into
// the pause menu during play.
func (app *App) pollGamepads() {
	for joy := glfw.Joystick1; joy <= glfw.Joystick4; joy++ {
		if !joy.Present() || !joy.IsGamepad() {
			continue
		}
		st := joy.GetGamepadState()
		if st == nil {
			continue
		}
		var now [16]bool
		for i := 0; i < 15; i++ {
			now[i] = st.Buttons[i] == glfw.Press
		}
		stickY := st.Axes[glfw.AxisLeftY]
		now[15] = stickY < -0.6 || stickY > 0.6
		prev := app.ms.padPrev[joy]
		app.ms.padPrev[joy] = now
		pressed := func(b glfw.GamepadButton) bool { return now[b] && !prev[b] }
		if !app.menuActive() {
			if pressed(glfw.ButtonStart) {
				app.pauseRequested()
			}
			continue
		}
		push := func(in ui.Input) { app.ms.inputs = append(app.ms.inputs, in) }
		switch {
		case pressed(glfw.ButtonDpadUp):
			push(ui.Up)
		case pressed(glfw.ButtonDpadDown):
			push(ui.Down)
		case pressed(glfw.ButtonDpadLeft):
			push(ui.Left)
		case pressed(glfw.ButtonDpadRight):
			push(ui.Right)
		case pressed(glfw.ButtonA):
			push(ui.Accept)
		case pressed(glfw.ButtonB), pressed(glfw.ButtonStart):
			push(ui.Back)
		case now[15] && !prev[15]:
			if stickY < 0 {
				push(ui.Up)
			} else {
				push(ui.Down)
			}
		}
	}
}

// pauseRequested handles the pause key/button: with a menu file it opens the
// pause screen (or the title screen after the level ended); without one it
// falls back to the legacy pause toggle.
func (app *App) pauseRequested() {
	m := app.ms.menu
	if m == nil {
		app.paused = !app.paused
		if app.paused {
			app.window.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
		} else {
			app.window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)
			app.mouseInit = false
		}
		app.scriptEngine.Emit("onPause", app.paused)
		log.Printf("[Engine] Pause toggled: %v", app.paused)
		return
	}
	if app.levelEnded() && m.Has("main") && m.Def.Screens["main"] != nil {
		app.openMenu("main")
		return
	}
	if m.Def.Screens["pause"] != nil {
		app.openMenu("pause")
	}
}
