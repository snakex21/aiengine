// parser/integration_test.go — Integration tests parsing real demo files.

package parser

import (
	"os"
	"testing"
)

func TestParseDemoGrunt(t *testing.T) {
	f, err := os.Open("../demo/entities/grunt.aie")
	if err != nil {
		t.Skipf("demo file not found: %v", err)
		return
	}
	defer f.Close()

	doc, err := ParseAIE(f)
	if err != nil {
		t.Fatalf("ParseAIE failed: %v", err)
	}

	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Nodes))
	}

	grunt := doc.Nodes[0]
	if grunt.Type != "HUMANOID" || grunt.Name != "grunt" {
		t.Errorf("expected HUMANOID grunt, got %s %s", grunt.Type, grunt.Name)
	}

	// Should have ANIM children.
	anims := grunt.FindChildren("ANIM")
	if len(anims) < 2 {
		t.Errorf("expected at least 2 ANIM blocks, got %d", len(anims))
	}
}

func TestParseDemoBossKaiju(t *testing.T) {
	f, err := os.Open("../demo/entities/boss_kaiju.aie")
	if err != nil {
		t.Skipf("demo file not found: %v", err)
		return
	}
	defer f.Close()

	doc, err := ParseAIE(f)
	if err != nil {
		t.Fatalf("ParseAIE failed: %v", err)
	}

	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Nodes))
	}

	kaiju := doc.Nodes[0]
	if kaiju.Type != "GIANT" || kaiju.Name != "kaiju" {
		t.Errorf("expected GIANT kaiju, got %s %s", kaiju.Type, kaiju.Name)
	}

	// Verify HP_PARTS exist.
	hpParts := kaiju.FindChild("HP_PARTS")
	if hpParts == nil {
		t.Fatal("expected HP_PARTS child")
	}
	if len(hpParts.Children) != 6 {
		t.Errorf("expected 6 HP parts, got %d", len(hpParts.Children))
	}
}

func TestParseDemoLevel1Map(t *testing.T) {
	f, err := os.Open("../demo/maps/level1.aie")
	if err != nil {
		t.Skipf("demo file not found: %v", err)
		return
	}
	defer f.Close()

	doc, err := ParseAIE(f)
	if err != nil {
		t.Fatalf("ParseAIE failed: %v", err)
	}

	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 MAP node, got %d", len(doc.Nodes))
	}

	level := doc.Nodes[0]
	if level.Type != "MAP" || level.Name != "level1" {
		t.Errorf("expected MAP level1, got %s %s", level.Type, level.Name)
	}

	// Verify GRID exists with rows.
	grid := level.FindChild("GRID")
	if grid == nil {
		t.Fatal("expected GRID child")
	}
	if len(grid.Values) != 5 {
		t.Errorf("expected 5 grid rows, got %d", len(grid.Values))
	}

	// Verify PLACE entries.
	places := level.FindChildren("PLACE")
	if len(places) != 6 {
		t.Errorf("expected 6 PLACE entries, got %d", len(places))
	}

	// Verify TRIGGER entries.
	triggers := level.FindChildren("TRIGGER")
	if len(triggers) != 3 {
		t.Errorf("expected 3 TRIGGER entries, got %d", len(triggers))
	}
}

func TestParseDemoConfig(t *testing.T) {
	cfg, err := ParseConfigFile("../demo/config.aie")
	if err != nil {
		t.Skipf("demo config not found: %v", err)
		return
	}

	if len(cfg.Bindings) < 10 {
		t.Errorf("expected at least 10 bindings, got %d", len(cfg.Bindings))
	}

	// Verify specific bindings.
	if cfg.GetBinding("move_forward") != "W" {
		t.Errorf("expected move_forward=W, got %s", cfg.GetBinding("move_forward"))
	}
	if cfg.GetBinding("fullscreen") != "F11" {
		t.Errorf("expected fullscreen=F11, got %s", cfg.GetBinding("fullscreen"))
	}
}

func TestParseDemoCSSGrunt(t *testing.T) {
	f, err := os.Open("../demo/styles/grunt.css")
	if err != nil {
		t.Skipf("demo file not found: %v", err)
		return
	}
	defer f.Close()

	doc, err := ParseCSS(f)
	if err != nil {
		t.Fatalf("ParseCSS failed: %v", err)
	}

	rule := doc.FindRule(".grunt")
	if rule == nil {
		t.Fatal("expected .grunt rule")
	}
	if rule.GetProperty("color") != "#666666" {
		t.Errorf("expected color=#666666, got %s", rule.GetProperty("color"))
	}
}

func TestParseDemoCSSLevel1(t *testing.T) {
	f, err := os.Open("../demo/styles/level1.css")
	if err != nil {
		t.Skipf("demo file not found: %v", err)
		return
	}
	defer f.Close()

	doc, err := ParseCSS(f)
	if err != nil {
		t.Fatalf("ParseCSS failed: %v", err)
	}

	// Check screen rule.
	screen := doc.FindRule("screen")
	if screen == nil {
		t.Fatal("expected screen rule")
	}
	if screen.GetPropertyFloat("vignette") != 0.3 {
		t.Errorf("expected vignette=0.3, got %f", screen.GetPropertyFloat("vignette"))
	}

	// Check player:low-hp rule.
	lowHP := doc.FindRule("player:low-hp")
	if lowHP == nil {
		t.Fatal("expected player:low-hp rule")
	}
}
