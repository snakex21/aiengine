package krypta_test

import (
	"path/filepath"
	"strings"
	"testing"

	"aiengine/level"
	"aiengine/parser"
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
	menu.OpenRoot("main")
	want := "main: [Nowa gra] | Wybór poziomu | Jak grać | Ustawienia | Wyjście"
	if got := menu.Describe(); got != want {
		t.Fatalf("main menu\n got %s\nwant %s", got, want)
	}

	// Wybór poziomu → Trening → load_level trening.
	menu.Input(ui.Down)
	menu.Input(ui.Accept)
	if !strings.HasPrefix(menu.Describe(), "level_select: [Kryształowa Krypta] | Trening (3 kryształy) | Wróć") {
		t.Fatalf("level select: %s", menu.Describe())
	}
	menu.Input(ui.Down)
	a, ok := menu.Input(ui.Accept)
	if !ok || a.Kind != "load_level" || a.Arg != "trening" || menu.Active() {
		t.Fatalf("load level: %+v ok=%v", a, ok)
	}
	if l, ok := man.FindLevel(a.Arg); !ok || l.Path != "maps/trening.aie" {
		t.Fatalf("level entry %+v", l)
	}

	// Jak grać → Wróć; Esc on the title screen does nothing.
	menu.OpenRoot("main")
	menu.Input(ui.Down)
	menu.Input(ui.Down)
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

func TestTrainingLevelWin(t *testing.T) {
	g := startMap(t, "maps/trening.aie")
	if !hudHas(g.eng, "Kryształy 0/3") || !hudHas(g.eng, "Czas 2:00") {
		t.Fatalf("HUD %v", g.eng.HUDLines())
	}
	for _, id := range []string{"t1", "t2", "t3"} {
		g.visit(t, id)
	}
	if g.lv.IsSolid(6.5, 3.5) {
		t.Fatal("gate closed after 3 crystals")
	}
	if !reachable(g.lv)[cell(g.at(t, "portal"))] {
		t.Fatal("portal unreachable")
	}
	g.visit(t, "portal")
	if g.s.State != level.Won {
		t.Fatalf("state %v", g.s.State)
	}
}
