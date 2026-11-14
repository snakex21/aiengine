package world

import (
	"math"
	"math/rand"
	"testing"

	"aiengine/sector"
)

// testGrid is a 10x10 room with border walls, a pillar at (5,2), a raised
// floor (0.5) at x=7..8, y=5..8, a low ceiling (0.7) at (2,7) and a pit
// (-0.5) at (4,7).
func testGrid() *MapData {
	g := make([][]MapCell, 10)
	for y := range g {
		g[y] = make([]MapCell, 10)
		for x := range g[y] {
			if x == 0 || y == 0 || x == 9 || y == 9 {
				g[y][x] = CellWall1
			}
		}
	}
	g[2][5] = CellWall2
	m := NewMapData(g)
	m.SetHeights(1)
	for y := 5; y <= 8; y++ {
		m.SetFloor(7, y, 0.5)
		m.SetFloor(8, y, 0.5)
		m.SetCeil(7, y, 1.5)
		m.SetCeil(8, y, 1.5)
	}
	m.SetCeil(2, 7, 0.7)
	m.SetFloor(4, 7, -0.5)
	return m
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestGridTraceWallFloorCeilingAndSteps(t *testing.T) {
	m := testGrid()
	// Wall: east along y=2.5 at eye height stops at the pillar's west face.
	h := m.Trace(1.5, 2.5, 0.5, 8.5, 2.5, 0.5)
	if h.Surface != SurfaceWall || !near(h.X, 5) || h.NX != -1 || h.NY != 0 {
		t.Fatalf("pillar: %+v", h)
	}
	// Floor: a shot aimed down lands on the floor with an upward normal.
	h = m.Trace(1.5, 4.5, 0.5, 3.5, 4.5, -0.5)
	if h.Surface != SurfaceFloor || !near(h.Z, 0) || !near(h.X, 2.5) || h.NZ != 1 {
		t.Fatalf("floor: %+v", h)
	}
	// Ceiling.
	h = m.Trace(1.5, 4.5, 0.5, 3.5, 4.5, 1.5)
	if h.Surface != SurfaceCeiling || !near(h.Z, 1) || h.NZ != -1 {
		t.Fatalf("ceiling: %+v", h)
	}
	// Step face: low shot into the raised floor stops at x=7.
	h = m.Trace(3.5, 6.5, 0.3, 8.5, 6.5, 0.3)
	if h.Surface != SurfaceWall || !near(h.X, 7) || h.NX != -1 {
		t.Fatalf("step face: %+v", h)
	}
	// ... a higher one flies over the step.
	if h = m.Trace(3.5, 6.5, 0.7, 8.5, 6.5, 0.7); h.Blocked() {
		t.Fatalf("over the step: %+v", h)
	}
	// Low ceiling: a shot at z=0.8 hits its riser at x=2 (coming from x>2).
	h = m.Trace(3.5, 7.5, 0.8, 1.5, 7.5, 0.8)
	if h.Surface != SurfaceWall || !near(h.X, 3) || h.NX != 1 {
		t.Fatalf("low ceiling riser: %+v", h)
	}
	// Into the pit and out: the far rim blocks a line skimming the bottom.
	if h = m.Trace(3.5, 7.5, 0.5, 4.5, 7.5, -0.4); h.Blocked() {
		t.Fatalf("looking into the pit: %+v", h)
	}
	if h = m.Trace(4.5, 7.5, -0.3, 6.5, 7.5, -0.3); h.Surface != SurfaceWall || !near(h.X, 5) {
		t.Fatalf("pit rim: %+v", h)
	}
	if !LineOfSight(m, 1.5, 1.5, 0.5, 8.5, 1.5, 0.5) {
		t.Fatal("open corridor blocked")
	}
}

func TestSectorTraceWallStepAndFloor(t *testing.T) {
	sq := func(id string, x0, y0, x1, y1, f, c float64) sector.Def {
		return sector.Def{Sector: sector.Sector{ID: id, Floor: f, Ceil: c,
			Poly: []sector.Vec2{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}}}
	}
	sm, err := sector.Build([]sector.Def{sq("a", 0, 0, 4, 4, 0, 2), sq("b", 4, 0, 8, 4, 0.5, 2)})
	if err != nil {
		t.Fatal(err)
	}
	g := Of(nil, sm)
	h := g.Trace(1, 2, 0.3, 7, 2, 0.3)
	if h.Surface != SurfaceWall || !near(h.X, 4) || h.NX != -1 {
		t.Fatalf("step face: %+v", h)
	}
	if h = g.Trace(1, 2, 1, 9, 2, 1); h.Surface != SurfaceWall || !near(h.X, 8) || h.NX != -1 {
		t.Fatalf("outer wall: %+v", h)
	}
	if h = g.Trace(5, 1, 1, 7, 1, 0); h.Surface != SurfaceFloor || !near(h.Z, 0.5) {
		t.Fatalf("raised floor: %+v", h)
	}
	if h = g.Trace(7, 1, 1, 1, 3, 1.2); h.Blocked() {
		t.Fatalf("clear shot: %+v", h)
	}
}

// The grid and its sector conversion must agree on every segment (the
// old code used a sampled raster for enemies and polygons for the player).
func TestGridAndSectorTracesAgree(t *testing.T) {
	m := testGrid()
	sm, err := sector.FromGrid(gridSrc{m}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	sec := Of(nil, sm)
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 3000; i++ {
		x0, y0 := 1+r.Float64()*8, 1+r.Float64()*8
		x1, y1 := 1+r.Float64()*8, 1+r.Float64()*8
		if m.GetCell(int(x0), int(y0)) != CellEmpty {
			continue
		}
		f0, f1 := FloorAt(m, x0, y0), FloorAt(m, x1, y1)
		z0, z1 := f0+0.05+r.Float64()*0.6, f1+0.05+r.Float64()*0.6
		a, b := m.Trace(x0, y0, z0, x1, y1, z1), sec.Trace(x0, y0, z0, x1, y1, z1)
		if a.Surface != b.Surface || math.Abs(a.T-b.T) > 1e-6 {
			t.Fatalf("(%.3f,%.3f,%.3f)->(%.3f,%.3f,%.3f): grid %+v sectors %+v", x0, y0, z0, x1, y1, z1, a, b)
		}
		// Sight is symmetric.
		if a.Blocked() != m.Trace(x1, y1, z1, x0, y0, z0).Blocked() && m.GetCell(int(x1), int(y1)) == CellEmpty {
			t.Fatalf("asymmetric sight (%.3f,%.3f,%.3f)<->(%.3f,%.3f,%.3f)", x0, y0, z0, x1, y1, z1)
		}
	}
}

type gridSrc struct{ m *MapData }

func (g gridSrc) Size() (int, int)        { return g.m.Width, g.m.Height }
func (g gridSrc) Open(x, y int) bool      { return g.m.GetCell(x, y) == CellEmpty }
func (g gridSrc) Floor(x, y int) float64  { return g.m.FloorAt(x, y) }
func (g gridSrc) Ceil(x, y int) float64   { return g.m.CeilAt(x, y) }
func (g gridSrc) Light(x, y int) float64  { return 1 }
func (g gridSrc) WallTex(x, y int) string { return "W" }
func (g gridSrc) StepTex(x, y int) string { return "S" }

// Sight through a sector portal onto a 0.25 step into a lower-ceilinged
// side room (moved from the sector package with SightClear).
func TestSightThroughPortal(t *testing.T) {
	m, err := sector.Build([]sector.Def{
		{Sector: sector.Sector{ID: "hall", Floor: 0, Ceil: 2, Light: 1, Poly: []sector.Vec2{{X: 0, Y: 0}, {X: 4, Y: 0}, {X: 4, Y: 1}, {X: 4, Y: 3}, {X: 4, Y: 4}, {X: 0, Y: 4}}}, WallTex: "W"},
		{Sector: sector.Sector{ID: "side", Floor: 0.25, Ceil: 1.25, Light: 0.5, Poly: []sector.Vec2{{X: 4, Y: 1}, {X: 7, Y: 0.5}, {X: 7, Y: 3.5}, {X: 4, Y: 3}}}, WallTex: "S"},
	})
	if err != nil {
		t.Fatal(err)
	}
	g := Of(nil, m)
	if !LineOfSight(g, 1, 2, 0.5, 6, 2, 0.75) {
		t.Error("eye to eye through the doorway must be clear")
	}
	if LineOfSight(g, 1, 2, 0.5, 6, 2, 0) {
		t.Error("a line below the side room's floor edge must be blocked")
	}
	if LineOfSight(g, 1, 2, 0.5, 6, 2, 2) {
		t.Error("a line above the side room's ceiling must be blocked")
	}
	if LineOfSight(g, 1, 0.5, 0.5, 6, 0.8, 0.5) {
		t.Error("through the solid wall north of the doorway must be blocked")
	}
}
