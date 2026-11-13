// parser/aie_test.go — Unit tests for the .aie parser.

package parser

import (
	"strings"
	"testing"
)

func TestParseAIESimpleEntity(t *testing.T) {
	input := `HUMANOID grunt
HP 50
HEAD small skin
TORSO medium grey_shirt
LEGS medium black_pants
`

	doc, err := ParseAIE(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAIE failed: %v", err)
	}

	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Nodes))
	}

	n := doc.Nodes[0]
	if n.Type != "HUMANOID" {
		t.Errorf("expected HUMANOID, got %s", n.Type)
	}
	if n.Name != "grunt" {
		t.Errorf("expected grunt, got %s", n.Name)
	}
	if n.GetPropertyInt("HP") != 50 {
		t.Errorf("expected HP=50, got %d", n.GetPropertyInt("HP"))
	}
	if n.GetProperty("HEAD") != "small skin" {
		t.Errorf("expected HEAD='small skin', got '%s'", n.GetProperty("HEAD"))
	}
}

func TestParseAIEMap(t *testing.T) {
	input := `MAP level1
MUSIC dark_ambient.ogg loop=yes
LIGHT_AMBIENT 0.1
GRID
W W W
W P W
W W W
PLACE enemy grunt 5.0 0.0 2.0
TRIGGER zone_clear
  CONDITION all_enemies_dead zone1
  ACTION unlock_door door_zone2
`

	doc, err := ParseAIE(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAIE failed: %v", err)
	}

	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 MAP node, got %d", len(doc.Nodes))
	}

	m := doc.Nodes[0]
	if m.Type != "MAP" {
		t.Errorf("expected MAP, got %s", m.Type)
	}

	// Check GRID child.
	grid := m.FindChild("GRID")
	if grid == nil {
		t.Fatal("expected GRID child")
	}
	if len(grid.Values) != 3 {
		t.Errorf("expected 3 grid rows, got %d", len(grid.Values))
	}

	// Check PLACE children.
	places := m.FindChildren("PLACE")
	if len(places) != 1 {
		t.Errorf("expected 1 PLACE, got %d", len(places))
	}

	// Check TRIGGER child of MAP.
	trigger := m.FindChild("TRIGGER")
	if trigger == nil {
		t.Fatal("expected TRIGGER child inside MAP")
	}
	if trigger.Name != "zone_clear" {
		t.Errorf("expected zone_clear, got %s", trigger.Name)
	}

	condition := trigger.FindChild("CONDITION")
	if condition == nil || condition.Name != "all_enemies_dead zone1" {
		t.Error("expected CONDITION child with 'all_enemies_dead zone1'")
	}

	action := trigger.FindChild("ACTION")
	if action == nil || action.Name != "unlock_door door_zone2" {
		t.Error("expected ACTION child with 'unlock_door door_zone2'")
	}
}

func TestParseAIEWeapon(t *testing.T) {
	input := `WEAPON pistol
DAMAGE 25
AMMO_MAX 15
`

	doc, err := ParseAIE(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAIE failed: %v", err)
	}

	w := doc.Nodes[0]
	if w.Type != "WEAPON" {
		t.Errorf("expected WEAPON, got %s", w.Type)
	}
	if w.GetPropertyInt("DAMAGE") != 25 {
		t.Errorf("expected DAMAGE=25, got %d", w.GetPropertyInt("DAMAGE"))
	}
}

func TestParseAIEBossKaiju(t *testing.T) {
	input := `GIANT kaiju
HP 5000
HP_PARTS
  HEAD 800
  TORSO 2000
  ARM_LEFT 600
HP_PHASES
  PHASE 2 hp=3000
  PHASE 3 hp=1000
WEAK_SPOT HEAD multiplier=3.0
BEHAVIOR
  COLLAPSE yes
  SHATTER yes parts=8
BODY huge dark_scales
HEAD huge dark_scales
ANIM idle
  FRAME 0
    HEAD ROT 0 5 0
  FRAME 30
    HEAD ROT 0 -5 0
END
`

	doc, err := ParseAIE(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAIE failed: %v", err)
	}

	n := doc.Nodes[0]
	if n.Type != "GIANT" {
		t.Errorf("expected GIANT, got %s", n.Type)
	}

	// HP_PARTS block should have HEAD, TORSO, ARM_LEFT children.
	hpParts := n.FindChild("HP_PARTS")
	if hpParts == nil {
		t.Fatal("expected HP_PARTS child")
	}
	if len(hpParts.Children) != 3 {
		t.Errorf("expected 3 HP part entries, got %d", len(hpParts.Children))
	}
	// Verify values: HEAD child should have Name = "800"
	headPart := hpParts.FindChild("HEAD")
	if headPart == nil || headPart.Name != "800" {
		t.Errorf("expected HEAD part with value 800, got Name=%q", headPart.Name)
	}
	torsoPart := hpParts.FindChild("TORSO")
	if torsoPart == nil || torsoPart.Name != "2000" {
		t.Errorf("expected TORSO part with value 2000")
	}

	// HP_PHASES block.
	hpPhases := n.FindChild("HP_PHASES")
	if hpPhases == nil {
		t.Fatal("expected HP_PHASES child")
	}
	phases := hpPhases.FindChildren("PHASE")
	if len(phases) != 2 {
		t.Errorf("expected 2 PHASE entries, got %d", len(phases))
	}

	// BEHAVIOR block.
	behavior := n.FindChild("BEHAVIOR")
	if behavior == nil {
		t.Fatal("expected BEHAVIOR child")
	}
	if behavior.GetPropertyBool("COLLAPSE") != true {
		t.Error("expected COLLAPSE=yes")
	}

	// ANIM block.
	anim := n.FindChild("ANIM")
	if anim == nil || anim.Name != "idle" {
		t.Fatal("expected ANIM idle")
	}
	frames := anim.FindChildren("FRAME")
	if len(frames) != 2 {
		t.Errorf("expected 2 FRAME entries, got %d", len(frames))
	}
}

func TestParseAIECutscene(t *testing.T) {
	input := `CUTSCENE intro
  TEXT "Miasto opanowane przez gangi..."
  TEXT "Twoja misja: oczyscic ulice."
  NEXT_ON_KEY yes
`

	doc, err := ParseAIE(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAIE failed: %v", err)
	}

	c := doc.Nodes[0]
	if c.Type != "CUTSCENE" {
		t.Errorf("expected CUTSCENE, got %s", c.Type)
	}
	if c.Name != "intro" {
		t.Errorf("expected intro, got %s", c.Name)
	}

	texts := c.FindChildren("TEXT")
	if len(texts) != 2 {
		t.Errorf("expected 2 TEXT entries, got %d", len(texts))
	}
}

func TestParseAIEComments(t *testing.T) {
	input := `# This is a comment
HUMANOID grunt
# Another comment
HP 50
`

	doc, err := ParseAIE(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAIE failed: %v", err)
	}

	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Nodes))
	}
}

func TestParseAIESound(t *testing.T) {
	input := `SOUND gunshot
  TYPE impact
  FREQUENCY 800
  DECAY 0.1
  NOISE 0.3
`

	doc, err := ParseAIE(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAIE failed: %v", err)
	}

	s := doc.Nodes[0]
	if s.Type != "SOUND" {
		t.Errorf("expected SOUND, got %s", s.Type)
	}

	// FREQUENCY is stored as a child with Name="800".
	freq := s.FindChild("FREQUENCY")
	if freq == nil {
		t.Fatal("expected FREQUENCY child")
	}
	if freq.Name != "800" {
		t.Errorf("expected FREQUENCY=800, got Name=%q", freq.Name)
	}

	// TYPE child.
	typeChild := s.FindChild("TYPE")
	if typeChild == nil || typeChild.Name != "impact" {
		t.Errorf("expected TYPE=impact, got Name=%q", typeChild.Name)
	}
}

func TestParseAIEStandaloneTrigger(t *testing.T) {
	// TRIGGER without a MAP should be a standalone top-level node.
	input := `TRIGGER win_game
  CONDITION timer_elapsed 10
  ACTION win_game
`

	doc, err := ParseAIE(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAIE failed: %v", err)
	}

	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 standalone TRIGGER node, got %d", len(doc.Nodes))
	}

	tr := doc.Nodes[0]
	if tr.Type != "TRIGGER" || tr.Name != "win_game" {
		t.Errorf("expected TRIGGER win_game, got %s %s", tr.Type, tr.Name)
	}
}

func TestHeightAndLightGridBlocks(t *testing.T) {
	src := `MAP h
GRID
W W W
W P W
W W W
FLOOR_GRID
. . .
. 2 .
. . a
CEILING_GRID
. G .
LIGHT_GRID
9 9 9
PLACE item x 1.5 0 1.5
`
	doc, err := ParseAIE(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	m := doc.Nodes[0]
	for name, rows := range map[string]int{"GRID": 3, "FLOOR_GRID": 3, "CEILING_GRID": 1, "LIGHT_GRID": 1} {
		n := m.FindChild(name)
		if n == nil || len(n.Values) != rows {
			t.Fatalf("%s: got %+v, want %d rows", name, n, rows)
		}
	}
	if m.FindChild("FLOOR_GRID").Values[2] != ". . a" {
		t.Errorf("row kept verbatim, got %q", m.FindChild("FLOOR_GRID").Values[2])
	}
	if len(m.FindChildren("PLACE")) != 1 {
		t.Error("PLACE after a grid block must still parse")
	}
}
