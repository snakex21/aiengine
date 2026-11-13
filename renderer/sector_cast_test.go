package renderer

import (
	"math"
	"testing"

	"aiengine/sector"
)

// richRoom is a grid room with steps, a pit, a low ceiling, a pillar and
// varied light: the reference image for the sector renderer.
func richRoom(w, h int) *Raycaster {
	rc := testRaycaster(w, h)
	rc.FloorTexture, rc.CeilingTexture = "floor_t", "floor_t"
	m := rc.Map
	m.SetHeights(1.5)
	for y := 1; y < 15; y++ {
		m.SetFloor(8, y, 0.25)
		m.SetFloor(9, y, 0.5)
		m.SetCeil(11, y, 0.9)
		m.SetFloor(12, y, -0.5)
		for x := 1; x < 15; x++ {
			m.SetLight(x, y, 0.4+0.6*float64((x*7+y*3)%5)/4)
		}
	}
	m.Grid[5][6] = CellWall3
	m.Grid[10][6] = CellWall3
	rc.Player.X, rc.Player.Y, rc.Player.Angle = 3.5, 7.7, 0.15
	return rc
}

func sectorTwin(t testing.TB, rc *Raycaster) *Raycaster {
	sm, err := sector.FromGrid(GridSectors(rc.Map), "", "")
	if err != nil {
		t.Fatal(err)
	}
	for i := range sm.Sectors {
		sm.Sectors[i].FloorTex, sm.Sectors[i].CeilTex = rc.FloorTexture, rc.CeilingTexture
	}
	tw := testRaycaster(int(rc.screenW), int(rc.screenH))
	tw.FloorTexture, tw.CeilingTexture = rc.FloorTexture, rc.CeilingTexture
	tw.Map = rc.Map
	tw.Sectors = sm
	tw.Player = rc.Player
	return tw
}

// The sector renderer draws a converted grid map like the grid renderer
// (only pixels exactly on cell corners may differ).
func TestSectorRendererMatchesGridRenderer(t *testing.T) {
	for _, angle := range []float64{0.15, 1.2, 2.9, 4.4} {
		g := richRoom(320, 180)
		g.Player.Angle = angle
		s := sectorTwin(t, g)
		g.renderToBuffer(nil)
		s.renderToBuffer(nil)
		diff := 0
		for i := 0; i < len(g.pixels); i += 4 {
			if g.pixels[i] != s.pixels[i] || g.pixels[i+1] != s.pixels[i+1] || g.pixels[i+2] != s.pixels[i+2] {
				diff++
			}
		}
		if total := len(g.pixels) / 4; diff*1000 > total*5 {
			t.Errorf("angle %.2f: %d of %d pixels differ (want < 0.5%%)", angle, diff, total)
		}
		for x := range g.zBuffer {
			if math.Abs(g.zBuffer[x]-s.zBuffer[x]) > 1e-6 {
				t.Fatalf("angle %.2f column %d: wall distance %v vs %v", angle, x, g.zBuffer[x], s.zBuffer[x])
			}
		}
	}
}

// An angled wall projects to a straight edge, and a portal into a higher
// sector shows its lower face with the projected height.
func TestSectorAngledWallAndPortal(t *testing.T) {
	m, err := sector.Build([]sector.Def{
		{Sector: sector.Sector{ID: "a", Floor: 0, Ceil: 1, Light: 1, Poly: pts(0, 0, 6, 0, 6, 2, 6, 4, 0, 4)}, WallTex: "W"},
		{Sector: sector.Sector{ID: "b", Floor: 0.25, Ceil: 1, Light: 1, StepTex: "M", Poly: pts(6, 2, 6, 0, 10, 1, 10, 3, 6, 4)}, WallTex: "S"},
	})
	if err != nil {
		t.Fatal(err)
	}
	rc := heightRoom(640, 360)
	rc.Sectors = m
	rc.Player.X, rc.Player.Y, rc.Player.Angle = 2, 2, 0
	rc.renderToBuffer(nil)
	f := frameFor(rc)
	// Lower face of the portal at x = 6 (4 ahead): rows of 0.25..0 at d=4.
	top, bot := f.rowOf(f.rowAt(0.25, 4)), f.rowOf(f.rowAt(0, 4))
	for y := top; y < bot; y++ {
		if !isFace(rc, px(rc, 320, y)) {
			t.Fatalf("row %d of the portal's lower face (rows %d..%d) not drawn", y, top, bot)
		}
	}
	if isFace(rc, px(rc, 320, bot)) {
		t.Error("near floor must start right below the lower face")
	}
	// The far wall of b (x = 10, 8 ahead) is a solid wall: z-buffer 8.
	if math.Abs(rc.zBuffer[320]-8) > 1e-6 {
		t.Fatalf("centre column wall distance %v, want 8", rc.zBuffer[320])
	}
	// The angled walls of b: their top edge on screen is a straight line.
	edge := wallTopEdge(rc)
	from, to := -1, -1
	for x := 330; x < 640; x++ {
		if z := rc.zBuffer[x]; z > 4 && z < 8-1e-6 && edge[x] > 0 {
			if from < 0 {
				from = x
			}
			to = x
		}
	}
	if to-from < 20 {
		t.Fatalf("angled wall not visible (columns %d..%d)", from, to)
	}
	if d := maxLineDeviation(edge, from+2, to-2); d > 1.5 {
		t.Errorf("angled wall top edge bends by %.1f px", d)
	}
}

func TestSectorSpritesStandOnSectorFloor(t *testing.T) {
	m, err := sector.Build([]sector.Def{
		{Sector: sector.Sector{ID: "low", Floor: 0, Ceil: 2, Light: 1, Poly: pts(0, 0, 5, 0, 5, 4, 0, 4)}, WallTex: "W"},
		{Sector: sector.Sector{ID: "high", Floor: 0.5, Ceil: 2, Light: 1, Poly: pts(5, 0, 9, 0, 9, 4, 5, 4)}, WallTex: "W"},
	})
	if err != nil {
		t.Fatal(err)
	}
	rc := heightRoom(320, 180)
	rc.Sectors = m
	rc.TexManager.textures["magenta"] = spriteTex(255, 0, 255)
	rc.Player.X, rc.Player.Y, rc.Player.Angle = 1, 2, 0
	s := &Sprite{X: 7, Y: 2, Scale: 0.5, Z: 0.25, TexName: "magenta", Alpha: 1, Active: true}
	rc.renderToBuffer([]*Sprite{s})
	f := frameFor(rc)
	last := -1
	for y := 0; y < 180; y++ {
		if c := px(rc, 160, y); c[1] == 0 && c[0] == c[2] && c[0] > 50 {
			last = y
		}
	}
	if want := f.rowOf(f.rowAt(0.5, 6)) - 1; last != want {
		t.Fatalf("sprite bottom row %d, want %d (standing on the 0.5 sector)", last, want)
	}
}

func BenchmarkSectorRender1080p(b *testing.B) {
	g := richRoom(1920, 1080)
	s := sectorTwin(b, g)
	for i := 0; i < b.N; i++ {
		s.renderToBuffer(nil)
	}
}

func BenchmarkGridRichRender1080p(b *testing.B) {
	g := richRoom(1920, 1080)
	for i := 0; i < b.N; i++ {
		g.renderToBuffer(nil)
	}
}

// pts builds a polygon from x, y pairs.
func pts(xy ...float64) []sector.Vec2 {
	out := make([]sector.Vec2, 0, len(xy)/2)
	for i := 0; i+1 < len(xy); i += 2 {
		out = append(out, sector.Vec2{X: xy[i], Y: xy[i+1]})
	}
	return out
}
