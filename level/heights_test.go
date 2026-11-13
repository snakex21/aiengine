package level

import (
	"math"
	"strings"
	"testing"

	"aiengine/nav"
	"aiengine/parser"
	"aiengine/renderer"
)

func loadSrc(t *testing.T, src string) *Level {
	t.Helper()
	doc, err := parser.ParseAIE(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	lv, err := FromDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	return lv
}

// stairs: a corridor climbing 2 units per cell (0.25) to a platform at 1.0,
// next to a 0.5-high wall-like ledge (row 3) and a pit (row 1, x=7).
const stairsMap = `MAP stairs
HEIGHT_STEP 0.125
CEILING_HEIGHT 2
STEP_WALL M
FALL_DAMAGE 40
GRID
W W W W W W W W W W
W P . . . . . . . W
W . . . . . . . . W
W . . . . . . . . W
W W W W W W W W W W
FLOOR_GRID
. . . . . . . . . .
. . 2 4 6 8 8 h 8 .
. . . . . . . . . .
. . 4 4 4 4 4 4 4 .
CEILING_GRID
. . . . . . . . . .
. . . . . . . . . .
. . . . . . . . . .
. . . . . . . . 6 .
`

func TestHeightSymbols(t *testing.T) {
	for sym, want := range map[string]int{"0": 0, "9": 9, "A": 10, "Z": 35, "a": -1, "z": -26} {
		got, ok := HeightUnits(sym)
		if !ok || got != want {
			t.Errorf("HeightUnits(%q) = %d,%v want %d", sym, got, ok, want)
		}
		if sym != "0" && HeightSymbol(want) != sym {
			t.Errorf("HeightSymbol(%d) = %q want %q", want, HeightSymbol(want), sym)
		}
	}
	if _, ok := HeightUnits("."); ok {
		t.Error("'.' is the default, not a number")
	}
}

func TestLoadHeightGrids(t *testing.T) {
	lv := loadSrc(t, stairsMap)
	m := lv.Map
	checks := []struct {
		x, y       int
		floor, cei float64
	}{
		{1, 1, 0, 2}, {2, 1, 0.25, 2}, {5, 1, 1, 2}, {7, 1, -1, 2}, {3, 3, 0.5, 2}, {8, 3, 0.5, 0.75},
	}
	for _, c := range checks {
		if f, ce := m.FloorAt(c.x, c.y), m.CeilAt(c.x, c.y); f != c.floor || ce != c.cei {
			t.Errorf("cell (%d,%d): floor %v ceil %v, want %v %v", c.x, c.y, f, ce, c.floor, c.cei)
		}
	}
	if m.SideAt(3, 3) != renderer.CellWall3 {
		t.Error("STEP_WALL M must set the step face texture")
	}
	if d := lv.FallDamage(1.0); math.Abs(d-16) > 1e-9 {
		t.Errorf("FallDamage(1) = %v, want 40*(1-0.6)=16", d)
	}
	if lv.FallDamage(0.5) != 0 {
		t.Error("short falls are harmless")
	}
}

func TestClassicMapHasNoHeights(t *testing.T) {
	lv := arena(t)
	if lv.Map.HasHeights() {
		t.Fatal("a map without height grids must stay flat (old maps unchanged)")
	}
}

func TestHeightGridErrors(t *testing.T) {
	bad := map[string]string{
		"symbol":  "MAP x\nGRID\nW P W\nFLOOR_GRID\n. ? .\n",
		"rows":    "MAP x\nGRID\nW P W\nFLOOR_GRID\n. . .\n. . .\n",
		"inverse": "MAP x\nGRID\nW P W\nFLOOR_GRID\n. 9 .\n",
	}
	for name, src := range bad {
		doc, err := parser.ParseAIE(strings.NewReader(src))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := FromDocument(doc); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestCanStepAndPathOverStairs(t *testing.T) {
	lv := loadSrc(t, stairsMap)
	if !lv.CanStep(nav.Cell{X: 1, Y: 1}, nav.Cell{X: 2, Y: 1}) {
		t.Error("a 0.25 stair must be climbable")
	}
	if lv.CanStep(nav.Cell{X: 1, Y: 2}, nav.Cell{X: 2, Y: 3}) {
		t.Error("a 0.5 ledge must not be climbable")
	}
	if !lv.CanStep(nav.Cell{X: 2, Y: 3}, nav.Cell{X: 2, Y: 2}) {
		t.Error("dropping down is always allowed")
	}
	if lv.CanStep(nav.Cell{X: 7, Y: 3}, nav.Cell{X: 8, Y: 3}) {
		t.Error("0.25 headroom is too low to walk")
	}
	// Path from the start up the stairs to the platform end walks along
	// row 1 (rising 0.25 per cell).
	path := nav.FindPath(lv, nav.Cell{X: 1, Y: 2}, nav.Cell{X: 6, Y: 1}, 0)
	if path == nil {
		t.Fatal("platform must be reachable by the stairs")
	}
	for i := 1; i < len(path); i++ {
		if lv.FloorAt(path[i].X, path[i].Y)-lv.FloorAt(path[i-1].X, path[i-1].Y) > renderer.MaxStepUp {
			t.Fatalf("path climbs too high between %v and %v", path[i-1], path[i])
		}
	}
	// The 0.5 ledge row is only reachable by dropping from... nowhere: no path.
	if nav.FindPath(lv, nav.Cell{X: 1, Y: 2}, nav.Cell{X: 4, Y: 3}, 0) != nil {
		t.Error("ledge row 3 is not reachable on foot")
	}
}

func TestFighterRespectsHeights(t *testing.T) {
	lv := loadSrc(t, stairsMap)
	// Enemy below the 0.5 ledge, player standing on it right next to it:
	// it sees the player but cannot hit across the height difference.
	f := NewFighter(Placement{ID: "e", Type: "g", X: 3.5, Y: 2.5, Params: map[string]string{"windup": "0.01"}}, 1)
	dealt := 0.0
	for i := 0; i < 120; i++ {
		dealt += f.Step(lv, 3.5, 3.3, 1.0/60, nil)
	}
	if dealt != 0 {
		t.Fatalf("enemy hit a player %v units above it", lv.FloorAt(3, 3))
	}
	// Same floor: hits land.
	g := NewFighter(Placement{ID: "e2", Type: "g", X: 2.5, Y: 2.5, Params: map[string]string{"windup": "0.01"}}, 1)
	dealt = 0
	for i := 0; i < 120; i++ {
		dealt += g.Step(lv, 3.2, 2.5, 1.0/60, nil)
	}
	if dealt == 0 {
		t.Fatal("enemy on the same floor must hit")
	}
}

func TestSightOverLedge(t *testing.T) {
	lv := loadSrc(t, stairsMap)
	// Eyes at floor + 0.5 on row 2 see each other over the 0.5 ledge row?
	// No - the line stays in row 2 (floor 0). Across row 3 (floor 0.5) an
	// eye-level line from row 2 to beyond would touch the ledge top.
	if !nav.SightClear(lv, 1.5, 2.5, 0.5, 8.5, 2.5, 0.5) {
		t.Error("open row must be visible")
	}
	if nav.SightClear(lv, 1.5, 1.5, 0.5, 6.5, 1.5, 0.5) {
		t.Error("eye-level line into the rising stairs must hit the steps")
	}
	if !nav.SightClear(lv, 1.5, 1.5, 0.5, 6.5, 1.5, 1.5) {
		t.Error("looking up the stairs at the eye on the platform is clear")
	}
}
