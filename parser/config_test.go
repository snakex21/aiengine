// parser/config_test.go — Unit tests for the config.aie parser.

package parser

import (
	"strings"
	"testing"
)

func TestParseConfigBindings(t *testing.T) {
	input := `BIND move_forward W
BIND move_backward S
BIND move_left A
BIND move_right D
BIND shoot MOUSE_LEFT
BIND reload R
BIND fullscreen F11
BIND pause ESCAPE
`

	cfg, err := ParseConfig(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}

	if len(cfg.Bindings) != 8 {
		t.Fatalf("expected 8 bindings, got %d", len(cfg.Bindings))
	}

	if cfg.GetBinding("move_forward") != "W" {
		t.Errorf("expected move_forward=W, got %s", cfg.GetBinding("move_forward"))
	}
	if cfg.GetBinding("shoot") != "MOUSE_LEFT" {
		t.Errorf("expected shoot=MOUSE_LEFT, got %s", cfg.GetBinding("shoot"))
	}
}

func TestParseConfigServer(t *testing.T) {
	input := `SERVER port 27015
SERVER max_players 4
SERVER mode coop
SERVER map level1.aie
`

	cfg, err := ParseConfig(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}

	if cfg.Server.Port != 27015 {
		t.Errorf("expected port=27015, got %d", cfg.Server.Port)
	}
	if cfg.Server.MaxPlayers != 4 {
		t.Errorf("expected max_players=4, got %d", cfg.Server.MaxPlayers)
	}
	if cfg.Server.Mode != "coop" {
		t.Errorf("expected mode=coop, got %s", cfg.Server.Mode)
	}
	if cfg.Server.Map != "level1.aie" {
		t.Errorf("expected map=level1.aie, got %s", cfg.Server.Map)
	}
}

func TestParseConfigConnect(t *testing.T) {
	input := `CONNECT 192.168.1.1 27015
`

	cfg, err := ParseConfig(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}

	if cfg.Client.Address != "192.168.1.1" {
		t.Errorf("expected address=192.168.1.1, got %s", cfg.Client.Address)
	}
	if cfg.Client.Port != 27015 {
		t.Errorf("expected port=27015, got %d", cfg.Client.Port)
	}
}

func TestParseConfigComments(t *testing.T) {
	input := `# This is a comment
BIND jump SPACE
# Another comment
BIND crouch C
`

	cfg, err := ParseConfig(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}

	if len(cfg.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(cfg.Bindings))
	}
}

func TestParseConfigCaseInsensitive(t *testing.T) {
	input := `bind move_forward w
Bind move_backward s
BIND move_left A
`

	cfg, err := ParseConfig(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}

	if len(cfg.Bindings) != 3 {
		t.Fatalf("expected 3 bindings, got %d", len(cfg.Bindings))
	}

	// GetBinding is case-insensitive.
	if cfg.GetBinding("MOVE_FORWARD") != "w" {
		t.Errorf("expected move_forward=w, got %s", cfg.GetBinding("MOVE_FORWARD"))
	}
}

func TestManifestLevelsAndMenu(t *testing.T) {
	m, err := ParseManifest(strings.NewReader("MANIFEST\n  MAIN_MAP maps/a.aie\n  MENU menu.aie\n  LEVEL a maps/a.aie Kryształowa Krypta\n  LEVEL b maps/b.aie\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Menu != "menu.aie" || len(m.Levels) != 2 || m.Levels[0].Title != "Kryształowa Krypta" || m.Levels[1].Title != "b" {
		t.Fatalf("manifest %+v", m)
	}
	if l, ok := m.FindLevel("B"); !ok || l.Path != "maps/b.aie" {
		t.Fatal("FindLevel")
	}
	single, _ := ParseManifest(strings.NewReader("MAIN_MAP maps/x.aie\n"))
	if ll := single.LevelList(); len(ll) != 1 || ll[0].Path != "maps/x.aie" {
		t.Fatalf("LevelList fallback %+v", ll)
	}
	if _, err := ParseManifest(strings.NewReader("LEVEL onlyid\n")); err == nil {
		t.Fatal("expected error for incomplete LEVEL")
	}
}
