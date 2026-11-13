package sector

import (
	"math"
	"strings"
	"testing"
)

// Two rooms joined by a doorway portal: a square hall (0..4) and an
// angled room to its east whose floor is a step higher and ceiling lower.
//
//	(0,0)----(4,0)
//	  |  hall  |\
//	  |      (4,1)-(7,0.5)
//	  |      (4,3)  |  side (east, angled walls)
//	  |        |/ (7,3.5)
//	(0,4)----(4,4)
func twoRooms(t *testing.T) *Map {
	t.Helper()
	m, err := Build([]Def{
		{Sector: Sector{ID: "hall", Floor: 0, Ceil: 2, Light: 1, Poly: []Vec2{{0, 0}, {4, 0}, {4, 1}, {4, 3}, {4, 4}, {0, 4}}}, WallTex: "W"},
		{Sector: Sector{ID: "side", Floor: 0.25, Ceil: 1.25, Light: 0.5, Poly: []Vec2{{4, 1}, {7, 0.5}, {7, 3.5}, {4, 3}}}, WallTex: "S"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestBuildMakesPortalsFromSharedEdges(t *testing.T) {
	m := twoRooms(t)
	portals, walls := 0, 0
	for _, l := range m.Lines {
		if l.TwoSided() {
			portals++
			if !(l.A.Near(Vec2{4, 1}) && l.B.Near(Vec2{4, 3}) || l.A.Near(Vec2{4, 3}) && l.B.Near(Vec2{4, 1})) {
				t.Errorf("unexpected portal %v-%v", l.A, l.B)
			}
		} else {
			walls++
		}
	}
	if portals != 1 || walls != 8 {
		t.Fatalf("portals %d walls %d, want 1 and 8", portals, walls)
	}
	if len(m.Sectors[0].Lines) != 6 || len(m.Sectors[1].Lines) != 4 {
		t.Fatal("each sector must list all its lines, the portal included")
	}
}

func TestBuildRejectsBadMaps(t *testing.T) {
	sq := func(x, y, s float64) []Vec2 { return []Vec2{{x, y}, {x + s, y}, {x + s, y + s}, {x, y + s}} }
	cases := map[string][]Def{
		"few points": {{Sector: Sector{ID: "a", Ceil: 1, Poly: []Vec2{{0, 0}, {1, 0}}}}},
		"inverted":   {{Sector: Sector{ID: "a", Floor: 1, Ceil: 0, Poly: sq(0, 0, 1)}}},
		// b touches half of a's east edge without a shared vertex.
		"t-junction": {{Sector: Sector{ID: "a", Ceil: 1, Poly: sq(0, 0, 2)}}, {Sector: Sector{ID: "b", Ceil: 1, Poly: sq(2, 0, 1)}}},
	}
	for name, defs := range cases {
		if _, err := Build(defs); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestSectorAtAngledRoom(t *testing.T) {
	m := twoRooms(t)
	for _, c := range []struct {
		x, y float64
		want string
	}{{1, 1, "hall"}, {3.9, 3.9, "hall"}, {5, 2, "side"}, {6.9, 0.7, "side"}, {6.9, 0.3, ""}, {4.5, 0.5, ""}, {-1, 1, ""}} {
		got := m.SectorAt(c.x, c.y)
		name := ""
		if got >= 0 {
			name = m.Sectors[got].ID
		}
		if name != c.want {
			t.Errorf("SectorAt(%v,%v) = %q, want %q", c.x, c.y, name, c.want)
		}
	}
}

func TestCollisionWithAngledWallsAndSteps(t *testing.T) {
	m := twoRooms(t)
	const r, step, head = 0.15, 0.3, 0.6
	if !m.CanOccupy(2, 2, r, 0, step, head) {
		t.Error("middle of the hall must be free")
	}
	if m.CanOccupy(0.1, 2, r, 0, step, head) {
		t.Error("too close to the west wall")
	}
	// The side room's north wall runs from (4,1) to (7,0.5): a point 0.1
	// below it is too close, 0.3 below is fine.
	y := func(x float64) float64 { return 1 - (x-4)/6 }
	if m.CanOccupy(5.5, y(5.5)+0.1, r, 0.25, step, head) || !m.CanOccupy(5.5, y(5.5)+0.3, r, 0.25, step, head) {
		t.Error("angled wall distance check failed")
	}
	// Walking through the doorway onto the 0.25 step is allowed.
	if !m.CanOccupy(4, 2, r, 0, step, head) {
		t.Error("doorway onto a 0.25 step must be walkable")
	}
	m.Sectors[1].Floor = 0.5 // a floor that moved up (lift)
	if m.CanOccupy(4, 2, r, 0, step, head) {
		t.Error("a 0.5 rise must block")
	}
	if f, _ := m.Support(4.05, 2, r); f != 0.5 {
		t.Errorf("straddling the doorway stands on the higher floor, got %v", f)
	}
}

func TestSightThroughPortal(t *testing.T) {
	m := twoRooms(t)
	if !m.SightClear(1, 2, 0.5, 6, 2, 0.75) {
		t.Error("eye to eye through the doorway must be clear")
	}
	if m.SightClear(1, 2, 0.5, 6, 2, 0) {
		t.Error("a line below the side room's floor edge must be blocked")
	}
	if m.SightClear(1, 2, 0.5, 6, 2, 2) {
		t.Error("a line above the side room's ceiling must be blocked")
	}
	if m.SightClear(1, 0.5, 0.5, 6, 0.8, 0.5) {
		t.Error("through the solid wall north of the doorway must be blocked")
	}
}

type testGrid struct {
	rows  []string
	floor map[[2]int]float64
}

func (g testGrid) Size() (int, int)   { return len(g.rows[0]), len(g.rows) }
func (g testGrid) Open(x, y int) bool { return g.rows[y][x] == '.' }
func (g testGrid) Floor(x, y int) float64 {
	return g.floor[[2]int{x, y}]
}
func (g testGrid) Ceil(x, y int) float64   { return 1 }
func (g testGrid) Light(x, y int) float64  { return 1 }
func (g testGrid) StepTex(x, y int) string { return "S" }
func (g testGrid) WallTex(x, y int) string {
	if y < 0 || y >= len(g.rows) || x < 0 || x >= len(g.rows[0]) {
		return "W"
	}
	return string(g.rows[y][x])
}

func TestFromGrid(t *testing.T) {
	g := testGrid{rows: []string{
		"WWWWW",
		"W..MW",
		"W...W",
		"WWWWW",
	}, floor: map[[2]int]float64{{3, 2}: 0.25}}
	m, err := FromGrid(g, "floor", "ceiling")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Sectors) != 5 {
		t.Fatalf("%d sectors, want one per open cell (5)", len(m.Sectors))
	}
	s := m.SectorAt(3.5, 2.5)
	if s < 0 || m.Sectors[s].Floor != 0.25 {
		t.Fatal("cell heights must carry over")
	}
	metal := 0
	for _, l := range m.Lines {
		if l.Tex == "M" {
			metal++
		}
	}
	if metal != 2 {
		t.Errorf("the metal cell faces 2 open cells, got %d metal walls", metal)
	}
	if !m.CanOccupy(1.5, 1.5, 0.15, 0, 0.3, 0.6) || m.CanOccupy(3.5, 1.5, 0.15, 0, 0.3, 0.6) {
		t.Error("open cell free, solid cell blocked")
	}
}

func TestParseDef(t *testing.T) {
	d, err := ParseDef("hall floor=0.25 ceil=2 light=0.6 wall=M step=S floor_tex=floor tag=lift : 0 0 4 0 4 4 0 4")
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "hall" || d.Floor != 0.25 || d.Ceil != 2 || d.Light != 0.6 || d.WallTex != "M" || d.StepTex != "S" ||
		d.FloorTex != "floor" || d.Tag != "lift" || len(d.Poly) != 4 || d.Poly[2] != (Vec2{4, 4}) {
		t.Fatalf("parsed %+v", d)
	}
	for _, bad := range []string{"a 0 0 1 0 1 1", "a : 0 0 1 0", "a : 0 0 1 0 1", "a nope=1 : 0 0 1 0 1 1", "a floor=x : 0 0 1 0 1 1"} {
		if _, err := ParseDef(bad); err == nil || !strings.Contains(err.Error(), "SECTOR") {
			t.Errorf("%q: expected a SECTOR error, got %v", bad, err)
		}
	}
}

func TestSegmentHit(t *testing.T) {
	l := &Line{A: Vec2{2, -1}, B: Vec2{2, 1}}
	tt, u, ok := SegmentHit(Vec2{0, 0}, Vec2{4, 0}, l)
	if !ok || math.Abs(tt-0.5) > 1e-12 || math.Abs(u-0.5) > 1e-12 {
		t.Fatalf("t=%v u=%v ok=%v", tt, u, ok)
	}
	if _, _, ok := SegmentHit(Vec2{0, 2}, Vec2{4, 2}, l); ok {
		t.Fatal("parallel offset segment must miss")
	}
}
