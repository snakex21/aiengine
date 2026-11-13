package krypta_test

import (
	"path/filepath"
	"strings"
	"testing"

	"aiengine/parser"
	"aiengine/progress"
	"aiengine/settings"
	"aiengine/ui"
)

func loadMenu(t *testing.T, s *settings.Settings) (*ui.Menu, *parser.Manifest) {
	t.Helper()
	m, err := parser.ParseManifestFile("manifest.aie")
	if err != nil {
		t.Fatal(err)
	}
	def, err := ui.ParseFile(m.Menu)
	if err != nil {
		t.Fatal(err)
	}
	return ui.New(def, m.LevelList(), s, "."), m
}

func TestTitleMenuNavigation(t *testing.T) {
	s := settings.Defaults(1280, 720)
	menu, man := loadMenu(t, &s)
	camp := progress.FromManifest(man)
	prog := progress.New(camp)
	// Same hooks as the engine host (campaign_runtime.go).
	menu.Available = func(a ui.Action) bool { return a.Kind != "continue" || prog.CanContinue() }
	menu.LevelStatus = func(id string) (bool, string) {
		if !prog.Unlocked[id] {
			return false, "(zablokowany)"
		}
		if b, ok := prog.Best[id]; ok {
			return true, progress.FormatTime(b)
		}
		return true, ""
	}

	menu.OpenRoot("main")
	want := "main: [Nowa gra] | Wybór poziomu | Jak grać | Ustawienia | Wyjście"
	if got := menu.Describe(); got != want {
		t.Fatalf("main menu without a save\n got %s\nwant %s", got, want)
	}

	// Finish level 1: "Kontynuuj" appears and level 2 unlocks with level 1's record.
	prog.NewGame(camp)
	prog.Complete(camp, "przedsionek", 83, progress.State{})
	menu.OpenRoot("main")
	if got := menu.Describe(); !strings.HasPrefix(got, "main: [Kontynuuj] | Nowa gra") {
		t.Fatalf("main menu with a save: %s", got)
	}
	if a, ok := menu.Input(ui.Accept); !ok || a.Kind != "continue" {
		t.Fatalf("continue: %+v", a)
	}

	menu.OpenRoot("main")
	menu.Input(ui.Down)
	menu.Input(ui.Down)
	menu.Input(ui.Accept)
	want = "level_select: [Przedsionek  1:23] | Kryształowa Krypta | Lodowe Groby  (zablokowany) | Zalana Kopalnia  (zablokowany) | Serce Krypty  (zablokowany) | Wróć"
	if got := menu.Describe(); got != want {
		t.Fatalf("level select\n got %s\nwant %s", got, want)
	}
	menu.Input(ui.Down)
	menu.Input(ui.Down) // Lodowe Groby: locked
	if _, ok := menu.Input(ui.Accept); ok {
		t.Fatal("a locked level started")
	}
	menu.Input(ui.Up)
	a, ok := menu.Input(ui.Accept)
	if !ok || a.Kind != "load_level" || a.Arg != "krypta" || menu.Active() {
		t.Fatalf("load level: %+v ok=%v", a, ok)
	}

	// Jak grać → Wróć; Esc on the title screen does nothing.
	menu.OpenRoot("main")
	for i := 0; i < 3; i++ {
		menu.Input(ui.Down)
	}
	menu.Input(ui.Accept)
	if menu.ScreenName() != "instrukcja" {
		t.Fatalf("screen %s", menu.ScreenName())
	}
	menu.Input(ui.Accept) // Wróć
	if _, ok := menu.Input(ui.Back); ok || menu.ScreenName() != "main" {
		t.Fatal("Esc on the title screen must keep it open")
	}
	menu.Input(ui.Up) // focus was reset to the first item: wraps to Wyjście
	if a, ok := menu.Input(ui.Accept); !ok || a.Kind != "quit" {
		t.Fatalf("quit: %+v", a)
	}
}

func TestSettingsMenuIsPolishAndPersists(t *testing.T) {
	s := settings.Defaults(1280, 720)
	menu, _ := loadMenu(t, &s)
	menu.OpenRoot("pause")
	menu.Input(ui.Down)
	menu.Input(ui.Down)
	menu.Input(ui.Accept) // Ustawienia
	if !strings.HasPrefix(menu.Describe(), "settings: [Czułość myszy=1.0] | Odwróć oś Y=Nie | Głośność ogólna=100%") {
		t.Fatalf("settings: %s", menu.Describe())
	}
	menu.Input(ui.Right)
	menu.Input(ui.Down)
	menu.Input(ui.Accept)
	path := filepath.Join(t.TempDir(), settings.FileName)
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	back, err := settings.Load(path, settings.Defaults(1, 1))
	if err != nil || back.MouseSensitivity != 1.1 || !back.InvertY {
		t.Fatalf("persisted %+v err=%v", back, err)
	}
	menu.Input(ui.Back)
	if a, ok := menu.Input(ui.Back); !ok || a.Kind != "resume" {
		t.Fatalf("Esc in pause should resume: %+v", a)
	}
}

// The host only steps the session (and game time) while no menu is open;
// this mirrors that loop and checks the level clock and Lua timers freeze.
func TestPauseFreezesTimeAndScriptTimers(t *testing.T) {
	g := start(t)
	s := settings.Defaults(1280, 720)
	menu, _ := loadMenu(t, &s)
	if err := g.lua.DoString(`fired = false
		registerEvent("onTimer", function(n) fired = true end)
		startTimer("t", 2)`); err != nil {
		t.Fatal(err)
	}
	frame := func(dt float64) {
		if menu.PausesGame() {
			return
		}
		g.eng.Advance(dt)
		g.s.Update(dt, g.lv.SpawnX, g.lv.SpawnY)
	}
	for i := 0; i < 10; i++ {
		frame(0.1)
	}
	menu.OpenRoot("pause")
	for i := 0; i < 100; i++ { // 10 s "paused"
		frame(0.1)
	}
	if !hudHas(g.eng, "Czas 2:59") || g.lua.L.GetGlobal("fired").String() != "false" || g.eng.GetTime() > 1.01 {
		t.Fatalf("time ran while paused: HUD %v, time %.1f, fired %s", g.eng.HUDLines(), g.eng.GetTime(), g.lua.L.GetGlobal("fired"))
	}
	menu.Input(ui.Accept) // Wznów
	for i := 0; i < 11; i++ {
		frame(0.1)
	}
	if g.lua.L.GetGlobal("fired").String() != "true" || !hudHas(g.eng, "Czas 2:57") {
		t.Fatalf("after resume: HUD %v fired %s", g.eng.HUDLines(), g.lua.L.GetGlobal("fired"))
	}
}
