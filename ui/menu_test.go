package ui

import (
	"strings"
	"testing"

	"aiengine/parser"
	"aiengine/settings"
)

const testMenu = `
# test menu
SOUND blip WAVEFORM=square FREQUENCY=880 DECAY=0.05
SOUNDS move=blip select=blip back=blip
STYLE focus_color=#FF0000 focus_bg=#00000080

SCREEN main
  TITLE "Gra Testowa"
  SUBTITLE "Zażółć gęślą jaźń"
  BACKGROUND map 3.5 2.5 90
  MUSIC drone
  ITEM "Nowa gra" new_game
  ITEM "Wybór poziomu" level_select
  ITEM "Ustawienia" settings
  ITEM "Skrypt" run_script onSecret
  ITEM "Wyjście" quit

SCREEN pause
  TITLE Pauza
  BACKGROUND dim
  ESCAPE resume
  ITEM "Wznów" resume
  ITEM "Restart" restart
  ITEM "Ustawienia" settings
  ITEM "Menu główne" main_menu

SCREEN settings
  TITLE Ustawienia
  LABEL mouse_sensitivity "Czułość myszy"
  LABEL on Tak
  LABEL off Nie
  LABEL back Wróć
`

func newTestMenu(t *testing.T, levels int) (*Menu, *settings.Settings, *[]string) {
	t.Helper()
	def, err := Parse(strings.NewReader(testMenu))
	if err != nil {
		t.Fatal(err)
	}
	var ls []parser.LevelEntry
	for i := 0; i < levels; i++ {
		ls = append(ls, parser.LevelEntry{ID: string(rune('a' + i)), Title: "Poziom " + string(rune('A'+i))})
	}
	s := settings.Defaults(1280, 720)
	m := New(def, ls, &s, ".")
	var sounds []string
	m.OnSound = func(n string) { sounds = append(sounds, n) }
	return m, &s, &sounds
}

func TestParseMenu(t *testing.T) {
	m, _, _ := newTestMenu(t, 2)
	d := m.Def
	if len(d.Order) != 3 || d.Screens["main"].Title != "Gra Testowa" || d.Screens["main"].Music != "drone" {
		t.Fatalf("screens %+v", d.Order)
	}
	bg := d.Screens["main"].Background
	if bg.Kind != "map" || len(bg.Args) != 3 || bg.Args[2] != 90 {
		t.Fatalf("background %+v", bg)
	}
	if d.Screens["pause"].Escape.Kind != "resume" {
		t.Fatal("ESCAPE not parsed")
	}
	if d.Style.Focus != [4]uint8{255, 0, 0, 255} || d.Style.FocusBG[3] != 0x80 {
		t.Fatalf("style %+v", d.Style)
	}
	if len(d.SoundDefs) != 1 || d.SoundDefs[0].Props["FREQUENCY"] != "880" {
		t.Fatalf("sounds %+v", d.SoundDefs)
	}
}

func TestParseErrors(t *testing.T) {
	for name, src := range map[string]string{
		"item outside screen": "ITEM \"A\" quit\n",
		"unknown action":      "SCREEN s\nITEM \"A\" fly\n",
		"missing arg":         "SCREEN s\nITEM \"A\" load_level\n",
		"bad background":      "SCREEN s\nBACKGROUND plasma\n",
		"duplicate":           "SCREEN s\nSCREEN s\n",
		"bad colour":          "STYLE title_color=#zz\n",
	} {
		if _, err := Parse(strings.NewReader(src)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestMainMenuNavigation(t *testing.T) {
	m, _, sounds := newTestMenu(t, 2)
	m.OpenRoot("main")
	if !m.PausesGame() || m.Describe() != "main: [Nowa gra] | Wybór poziomu | Ustawienia | Skrypt | Wyjście" {
		t.Fatalf("describe %q", m.Describe())
	}
	m.Input(Up) // wraps to the last item
	if a, ok := m.Input(Accept); !ok || a.Kind != "quit" {
		t.Fatalf("quit: %+v %v", a, ok)
	}
	m.Input(Up) // Skrypt
	if a, ok := m.Input(Accept); !ok || a != (Action{Kind: "run_script", Arg: "onSecret"}) {
		t.Fatalf("run_script: %+v", a)
	}
	// Level select lists the levels plus Back; Back returns to main.
	m.OpenRoot("main")
	m.Input(Down)
	m.Input(Accept)
	if m.ScreenName() != "level_select" || m.Describe() != "level_select: [Poziom A] | Poziom B | Back" {
		t.Fatalf("level select %q", m.Describe())
	}
	m.Input(Down)
	if a, ok := m.Input(Accept); !ok || a != (Action{Kind: "load_level", Arg: "b"}) || m.Active() {
		t.Fatalf("load_level: %+v active=%v", a, m.Active())
	}
	m.OpenRoot("main")
	m.Input(Down)
	m.Input(Accept)
	m.Input(Back)
	if m.ScreenName() != "main" {
		t.Fatalf("back -> %s", m.ScreenName())
	}
	if a, ok := m.Input(Accept); !ok || a.Kind != "new_game" || m.Active() {
		t.Fatalf("new_game: %+v", a)
	}
	if len(*sounds) == 0 {
		t.Fatal("no menu sounds played")
	}
}

func TestLevelSelectHiddenWithOneLevel(t *testing.T) {
	m, _, _ := newTestMenu(t, 1)
	m.OpenRoot("main")
	if strings.Contains(m.Describe(), "Wybór poziomu") {
		t.Fatalf("level select shown with one level: %s", m.Describe())
	}
}

func TestPauseScreenEscapeResumes(t *testing.T) {
	m, _, _ := newTestMenu(t, 1)
	m.OpenRoot("pause")
	m.Input(Down)
	m.Input(Down)
	m.Input(Accept) // settings
	if m.ScreenName() != "settings" {
		t.Fatal("settings not opened")
	}
	if _, ok := m.Input(Back); ok || m.ScreenName() != "pause" {
		t.Fatal("Back from settings should return to pause")
	}
	if a, ok := m.Input(Back); !ok || a.Kind != "resume" || m.Active() {
		t.Fatalf("Esc on pause: %+v active=%v", a, m.Active())
	}
	m.OpenRoot("pause")
	m.Input(Down)
	m.Input(Down)
	m.Input(Down)
	if a, ok := m.Input(Accept); !ok || a.Kind != "main_menu" || m.ScreenName() != "main" {
		t.Fatalf("main_menu: %+v screen=%s", a, m.ScreenName())
	}
}

func TestSettingsRowsAdjustAndClamp(t *testing.T) {
	m, s, _ := newTestMenu(t, 1)
	m.OpenRoot("settings")
	if !strings.HasPrefix(m.Describe(), "settings: [Czułość myszy=1.0] | Invert Y=Nie") {
		t.Fatalf("describe %q", m.Describe())
	}
	if a, ok := m.Input(Right); !ok || a.Kind != "settings_changed" || s.MouseSensitivity != 1.1 {
		t.Fatalf("sensitivity: %+v %v", a, s.MouseSensitivity)
	}
	for i := 0; i < 50; i++ {
		m.Input(Left)
	}
	if s.MouseSensitivity != 0.2 {
		t.Fatalf("clamped sensitivity = %v", s.MouseSensitivity)
	}
	if _, ok := m.Input(Left); ok {
		t.Fatal("no change at the limit must not report settings_changed")
	}
	m.Input(Down)
	m.Input(Accept) // invert Y toggles
	if !s.InvertY {
		t.Fatal("invert Y not toggled")
	}
	m.Input(Down) // master
	m.Input(Left)
	if s.MasterVolume != 0.9 {
		t.Fatalf("master %v", s.MasterVolume)
	}
	for i := 0; i < 4; i++ {
		m.Input(Down)
	} // music, sfx, background volume, resolution
	m.Input(Left)
	m.Input(Left)
	if !s.Native() || m.ValueText("resolution") != "Native" {
		t.Fatalf("native: %+v", s)
	}
	m.Input(Right)
	m.Input(Right)
	m.Input(Right)
	if s.Width != 1600 || s.Height != 900 {
		t.Fatalf("resolution %dx%d", s.Width, s.Height)
	}
	m.Input(Down) // render scale
	m.Input(Right)
	if s.RenderScale != 1.25 || m.ValueText("render_scale") != "125%" {
		t.Fatalf("render scale %v", s.RenderScale)
	}
	for i := 0; i < 20; i++ {
		m.Input(Left) // wraps around the list
	}
	m.Input(Down)
	m.Input(Accept) // fullscreen on
	m.Input(Down)
	m.Input(Accept) // exclusive mode
	if !s.Exclusive || m.ValueText("fullscreen_mode") != "Exclusive" {
		t.Fatalf("fullscreen mode %+v", s)
	}
	m.Input(Down)
	m.Input(Right)
	if !s.Fullscreen || s.FOV != 71 {
		t.Fatalf("fullscreen %v fov %v", s.Fullscreen, s.FOV)
	}
	m.Input(Down) // Wróć
	if _, ok := m.Input(Accept); ok || !m.Active() {
		t.Fatal("Back on a root settings screen with no ESCAPE should close nothing and return no action")
	}
}

func TestMouseHoverAndClick(t *testing.T) {
	m, s, _ := newTestMenu(t, 2)
	m.OpenRoot("main")
	const sw, sh = 1280, 720
	l := m.Layout(sw, sh)
	r := l.Items[2] // Ustawienia
	m.Hover(r.X+5, r.Y+5, sw, sh)
	if m.Focus() != 2 {
		t.Fatalf("hover focus = %d", m.Focus())
	}
	m.Click(r.X+5, r.Y+5, sw, sh, false)
	if m.ScreenName() != "settings" {
		t.Fatal("click did not open settings")
	}
	l = m.Layout(sw, sh)
	row := l.Items[0]
	m.Click(row.X+row.W-4, row.Y+2, sw, sh, true) // right-click: step back
	if s.MouseSensitivity != 0.9 {
		t.Fatalf("sensitivity after right-click = %v", s.MouseSensitivity)
	}
	if _, ok := m.Click(1, 1, sw, sh, false); ok {
		t.Fatal("click outside items must do nothing")
	}
}

func TestDrawRendersTextAndFocus(t *testing.T) {
	m, _, _ := newTestMenu(t, 2)
	m.OpenRoot("pause")
	const sw, sh = 640, 360
	buf := make([]uint8, sw*sh*4)
	m.Draw(buf, sw, sh)
	if buf[3] != m.Def.Style.Dim[3] {
		t.Fatalf("dim background alpha = %d", buf[3])
	}
	r := m.Layout(sw, sh).Items[0]
	if buf[((r.Y+1)*sw+r.X+1)*4+3] != m.Def.Style.FocusBG[3] {
		t.Fatal("focus band not drawn")
	}
	red := 0
	for i := 0; i < len(buf); i += 4 {
		if buf[i] == 255 && buf[i+1] == 0 && buf[i+2] == 0 {
			red++
		}
	}
	if red == 0 {
		t.Fatal("focused item text not drawn in focus colour")
	}
}

func TestContinueHiddenAndLockedLevels(t *testing.T) {
	def, err := Parse(strings.NewReader("SCREEN main\n  ITEM Kontynuuj continue\n  ITEM \"Nowa gra\" new_game\n  ITEM Poziomy level_select\n"))
	if err != nil {
		t.Fatal(err)
	}
	levels := []parser.LevelEntry{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}}
	m := New(def, levels, nil, ".")
	saved := false
	m.Available = func(a Action) bool { return a.Kind != "continue" || saved }
	m.LevelStatus = func(id string) (bool, string) {
		if id == "a" {
			return true, "1:23"
		}
		return false, "(zablokowany)"
	}
	m.OpenRoot("main")
	if strings.Contains(m.Describe(), "Kontynuuj") {
		t.Fatal("continue shown without a save")
	}
	saved = true
	m.OpenRoot("main")
	if a, ok := m.Input(Accept); !ok || a.Kind != "continue" || m.Active() {
		t.Fatalf("continue: %+v", a)
	}
	m.OpenRoot("main")
	m.Input(Down)
	m.Input(Down)
	m.Input(Accept)
	if m.Describe() != "level_select: [A  1:23] | B  (zablokowany) | Back" {
		t.Fatalf("levels: %s", m.Describe())
	}
	m.Input(Down)
	if _, ok := m.Input(Accept); ok || m.ScreenName() != "level_select" {
		t.Fatal("a locked level must not start")
	}
}
