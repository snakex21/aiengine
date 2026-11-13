package renderer

import (
	"math"
	"testing"
)

// heightRoom is a 16x16 room (walls on the border) seen from (3.5, 8.5)
// looking east, rendered with flat colours only so surfaces can be told
// apart by colour: floor and ceiling flat, step faces metal (no texture).
func heightRoom(w, h int) *Raycaster {
	rc := testRaycaster(w, h)
	rc.TexManager.textures["brick"] = nil
	rc.Map.SetHeights(DefaultCeiling)
	rc.Map.Side = make([]MapCell, rc.Map.Width*rc.Map.Height)
	for i := range rc.Map.Side {
		rc.Map.Side[i] = CellWall3
	}
	rc.Player.X, rc.Player.Y, rc.Player.Angle = 3.5, 8.5, 0
	rc.FloorColor = floorColor
	rc.CeilColor = ceilColor
	return rc
}

func px(rc *Raycaster, x, y int) [3]uint8 {
	i := (y*int(rc.screenW) + x) * 4
	return [3]uint8{rc.pixels[i], rc.pixels[i+1], rc.pixels[i+2]}
}

func isFlat(c [3]uint8, col [4]uint8) bool { return c[0] == col[0] && c[1] == col[1] && c[2] == col[2] }

// isFace reports a step face pixel (metal fallback colour: blue-ish grey,
// distinct from the flat floor/ceiling colours).
func isFace(rc *Raycaster, c [3]uint8) bool {
	return !isFlat(c, colorToRGBA(rc.FloorColor)) && !isFlat(c, colorToRGBA(rc.CeilColor))
}

// frameFor returns the projection constants used by renderToBuffer.
func frameFor(rc *Raycaster) *frame {
	f := &frame{sw: int(rc.screenW), sh: int(rc.screenH)}
	f.fsw, f.fsh = float64(f.sw), float64(f.sh)
	f.focal = Focal(f.fsw, rc.fov()/2)
	f.horizon = f.fsh/2 + rc.Player.Pitch*f.fsh
	f.eye = rc.Player.EyeZ()
	return f
}

// faceRows lists the rows of column x drawn as a step face.
func faceRows(rc *Raycaster, x int) (first, last int) {
	first, last = -1, -1
	for y := 0; y < int(rc.screenH); y++ {
		if isFace(rc, px(rc, x, y)) {
			if first < 0 {
				first = y
			}
			last = y
		}
	}
	return
}

func TestRaisedFloorDrawsLowerFaceAtProjectedRows(t *testing.T) {
	rc := heightRoom(320, 180)
	for y := 1; y < 15; y++ {
		for x := 6; x < 15; x++ {
			rc.Map.SetFloor(x, y, 0.25) // step up at x = 6, 2.5 ahead
		}
	}
	// Close the view with a far wall lowered out of sight: only the step
	// face may be "non-flat" below the horizon in the middle column.
	rc.renderToBuffer(nil)
	f := frameFor(rc)
	wantTop := f.rowOf(f.rowAt(0.25, 2.5))
	wantBot := f.rowOf(f.rowAt(0, 2.5))
	for _, x := range []int{40, 160, 280} {
		// Rows between the horizon and the end of the screen: the face is
		// the only non-flat run below the far wall's bottom edge.
		first := -1
		for y := int(f.horizon) + 1; y < 180; y++ {
			c := px(rc, x, y)
			if isFace(rc, c) && y >= wantTop-1 {
				if first < 0 {
					first = y
				}
			}
		}
		if first != wantTop {
			t.Errorf("column %d: lower face starts at row %d, want %d", x, first, wantTop)
		}
		if !isFlat(px(rc, x, wantBot), colorToRGBA(rc.FloorColor)) || isFlat(px(rc, x, wantBot-1), colorToRGBA(rc.FloorColor)) {
			t.Errorf("column %d: face must end exactly at row %d", x, wantBot)
		}
	}
}

func TestLowCeilingDrawsUpperFace(t *testing.T) {
	rc := heightRoom(320, 180)
	for y := 1; y < 15; y++ {
		for x := 6; x < 15; x++ {
			rc.Map.SetCeil(x, y, 0.75)
		}
	}
	rc.renderToBuffer(nil)
	f := frameFor(rc)
	wantTop := f.rowOf(f.rowAt(1, 2.5))
	wantBot := f.rowOf(f.rowAt(0.75, 2.5))
	for _, x := range []int{40, 160, 280} {
		if !isFlat(px(rc, x, wantTop-1), colorToRGBA(rc.CeilColor)) {
			t.Errorf("column %d: row above the face must be ceiling", x)
		}
		for y := wantTop; y < wantBot; y++ {
			if !isFace(rc, px(rc, x, y)) {
				t.Fatalf("column %d row %d: expected upper face (rows %d..%d)", x, y, wantTop, wantBot)
			}
		}
		if !isFlat(px(rc, x, wantBot), colorToRGBA(rc.CeilColor)) {
			t.Errorf("column %d: row %d below the face must be the low ceiling", x, wantBot)
		}
	}
}

func TestPitShowsFarSideAndHidesNearSide(t *testing.T) {
	rc := heightRoom(320, 180)
	for y := 1; y < 15; y++ {
		for x := 6; x < 9; x++ {
			rc.Map.SetFloor(x, y, -0.5) // pit from x=6 to x=9
		}
	}
	rc.renderToBuffer(nil)
	f := frameFor(rc)
	// The far side of the pit (x = 9, 5.5 ahead) rises from -0.5 to 0.
	wantTop := f.rowOf(f.rowAt(0, 5.5))
	wantBot := f.rowOf(f.rowAt(-0.5, 5.5))
	nearEdge := f.rowOf(f.rowAt(0, 2.5))
	if wantBot > nearEdge {
		wantBot = nearEdge // the near floor hides the bottom of the far side
	}
	first, last := -1, -1
	for y := int(f.horizon) + 1; y < 180; y++ {
		if isFace(rc, px(rc, 160, y)) && y >= wantTop {
			if first < 0 {
				first = y
			}
			last = y
		}
	}
	if first != wantTop || last != wantBot-1 {
		t.Fatalf("far pit side rows %d..%d, want %d..%d", first, last, wantTop, wantBot-1)
	}
}

func TestStepEdgeIsStraight(t *testing.T) {
	rc := heightRoom(640, 360)
	rc.Player.Angle = 0.35 // look at the step at an angle
	for y := 1; y < 15; y++ {
		for x := 7; x < 15; x++ {
			rc.Map.SetFloor(x, y, 0.25)
		}
	}
	rc.renderToBuffer(nil)
	edge := make([]int, 640)
	for x := range edge {
		edge[x] = -1
		for y := 181; y < 360; y++ {
			if isFace(rc, px(rc, x, y)) {
				edge[x] = y
				break
			}
		}
	}
	// The edge is a straight line where the face is visible.
	from, to := -1, -1
	for x, e := range edge {
		if e > 0 && x > 0 && edge[x-1] > 0 {
			if from < 0 {
				from = x
			}
			to = x
		}
	}
	if to-from < 100 {
		t.Fatalf("step face visible only in columns %d..%d", from, to)
	}
	if d := maxLineDeviation(edge, from+5, to-5); d > 1.5 {
		t.Errorf("step edge bends by %.1f px", d)
	}
}

func TestEyeFollowsGroundHeight(t *testing.T) {
	low := heightRoom(320, 180)
	low.renderToBuffer(nil)
	lowEdge := wallTopEdge(low)

	high := heightRoom(320, 180)
	high.Map.SetHeights(2)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			high.Map.SetFloor(x, y, 0.5)
			high.Map.SetCeil(x, y, 1.5)
		}
	}
	high.Player.PlaceAt(0.5)
	high.renderToBuffer(nil)
	// Same room shifted up by 0.5 with the camera: identical image.
	for i := range low.pixels {
		if low.pixels[i] != high.pixels[i] {
			t.Fatalf("raising floor, ceiling and camera together changed pixel %d", i)
		}
	}
	_ = lowEdge
}

func spriteTex(r, g, b uint8) *Texture2D {
	p := make([]uint8, 4*4*4)
	for i := 0; i < 16; i++ {
		p[i*4], p[i*4+1], p[i*4+2], p[i*4+3] = r, g, b, 255
	}
	return &Texture2D{Width: 4, Height: 4, Pixels: p}
}

func TestSpriteClippedByNearerLedge(t *testing.T) {
	rc := heightRoom(320, 180)
	rc.TexManager.textures["magenta"] = spriteTex(255, 0, 255)
	for y := 1; y < 15; y++ {
		rc.Map.SetFloor(6, y, 0.4) // a low wall-like ledge 2.5 ahead
	}
	s := &Sprite{X: 8.5, Y: 8.5, Scale: 1, TexName: "magenta", Alpha: 1, Active: true}
	rc.renderToBuffer([]*Sprite{s})
	f := frameFor(rc)
	ledgeTop := f.rowOf(f.rowAt(0.4, 2.5))
	ledgeFar := f.rowOf(f.rowAt(0.4, 3.5)) // far edge of the ledge top
	isMagenta := func(c [3]uint8) bool { return c[1] == 0 && c[0] > 50 && c[0] == c[2] }
	if !isMagenta(px(rc, 160, ledgeFar-3)) {
		t.Fatalf("sprite above the ledge should be visible at row %d: %v", ledgeFar-3, px(rc, 160, ledgeFar-3))
	}
	for y := ledgeFar; y < 180; y++ {
		if isMagenta(px(rc, 160, y)) {
			t.Fatalf("sprite drawn over the nearer ledge at row %d (ledge top %d)", y, ledgeTop)
		}
	}
}

func TestSpriteStandsOnItsCellFloor(t *testing.T) {
	bottomRow := func(floor float64) int {
		rc := heightRoom(320, 180)
		rc.TexManager.textures["magenta"] = spriteTex(255, 0, 255)
		rc.Map.SetFloor(7, 8, floor)
		s := &Sprite{X: 7.5, Y: 8.5, Scale: 0.5, Z: 0.25, TexName: "magenta", Alpha: 1, Active: true}
		rc.renderToBuffer([]*Sprite{s})
		last := -1
		for y := 0; y < 180; y++ {
			if c := px(rc, 160, y); c[1] == 0 && c[0] == c[2] && c[0] > 50 {
				last = y
			}
		}
		return last
	}
	f := frameFor(heightRoom(320, 180))
	if got, want := bottomRow(0), f.rowOf(f.rowAt(0, 4))-1; got != want {
		t.Errorf("sprite on floor 0: bottom row %d, want %d", got, want)
	}
	if got, want := bottomRow(0.3), f.rowOf(f.rowAt(0.3, 4))-1; got != want {
		t.Errorf("sprite on a 0.3 platform: bottom row %d, want %d", got, want)
	}
}

func TestSpriteRotationIndex(t *testing.T) {
	// Facing east (0); viewer east of it sees the front (0), north (+pi/2)
	// sees rotation 2 of 8, behind sees 4.
	cases := []struct {
		facing, view float64
		want         int
	}{{0, 0, 0}, {0, math.Pi / 2, 2}, {0, math.Pi, 4}, {0, -math.Pi / 2, 6}, {math.Pi, 0, 4}, {0, 0.3, 0}, {0, 0.5, 1}}
	for _, c := range cases {
		if got := RotationIndex(c.facing, c.view, 8); got != c.want {
			t.Errorf("RotationIndex(%v, %v) = %d, want %d", c.facing, c.view, got, c.want)
		}
	}
}

func TestCanOccupyAndSupport(t *testing.T) {
	m := NewMapData([][]MapCell{
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 1, 0},
	})
	m.SetFloor(1, 0, 0.25) // walkable step
	m.SetFloor(2, 0, 0.5)  // too high without jumping
	m.SetCeil(2, 0, 1.5)
	m.SetCeil(3, 1, 0.5)   // crawl space
	if !m.CanOccupy(1.5, 0.5, 0.15, 0, MaxStepUp, HeadroomStand) {
		t.Error("a 0.25 step must be walkable")
	}
	if m.CanOccupy(2.5, 0.5, 0.15, 0, MaxStepUp, HeadroomStand) {
		t.Error("a 0.5 ledge must block a walking body")
	}
	if !m.CanOccupy(2.5, 0.5, 0.15, 0.25, MaxStepUp, HeadroomStand) {
		t.Error("from the 0.25 step the 0.5 ledge is reachable")
	}
	if m.CanOccupy(3.5, 1.5, 0.15, 0, MaxStepUp, HeadroomStand) || !m.CanOccupy(3.5, 1.5, 0.15, 0, MaxStepUp, HeadroomProne) {
		t.Error("crawl space: blocked standing, open prone")
	}
	if m.CanOccupy(2.5, 2.5, 0.15, 0, MaxStepUp, HeadroomStand) {
		t.Error("solid cell must block")
	}
	if f, _ := m.Support(1.9, 0.5, 0.15); f != 0.5 {
		t.Errorf("body straddling a ledge stands on the higher floor, got %v", f)
	}
}

func TestSightClearRespectsHeights(t *testing.T) {
	m := NewMapData([][]MapCell{{0, 0, 0, 0, 0}})
	m.SetFloor(2, 0, 0.6) // a waist-high block in the middle
	if m.SightClear(0.5, 0.5, 0.5, 4.5, 0.5, 0.5) {
		t.Error("eye-level line through a 0.6 block must be blocked")
	}
	if !m.SightClear(0.5, 0.5, 0.9, 4.5, 0.5, 0.9) {
		t.Error("line above the block must be clear")
	}
	m.SetCeil(3, 0, 0.8)
	if m.SightClear(0.5, 0.5, 0.9, 4.5, 0.5, 0.9) {
		t.Error("low ceiling must block the high line")
	}
}

func TestPlayerStepUpSmoothsCamera(t *testing.T) {
	p := NewPlayerState(0, 0, 0)
	p.SetGround(0.25)
	if !p.Grounded || p.Ground != 0.25 || p.Z != 0 {
		t.Fatalf("step up must keep the player grounded on the new floor: %+v", p)
	}
	if math.Abs(p.EyeZ()-0.5) > 1e-9 {
		t.Fatalf("camera must not jump on the step: eye %v", p.EyeZ())
	}
	for i := 0; i < 30; i++ {
		p.UpdateVertical(1.0 / 60)
	}
	if math.Abs(p.EyeZ()-0.75) > 1e-3 {
		t.Fatalf("camera should settle at the new eye height, got %v", p.EyeZ())
	}
}

func TestPlayerFallsOffLedgeAndReportsFall(t *testing.T) {
	p := NewPlayerState(0, 0, 0)
	p.PlaceAt(1)
	p.SetGround(0)
	if p.Grounded || p.Feet() != 1 {
		t.Fatalf("walking off a ledge must start a fall at the old height: %+v", p)
	}
	for i := 0; i < 120 && !p.Grounded; i++ {
		p.UpdateVertical(1.0 / 60)
	}
	if !p.Grounded || p.Feet() != 0 {
		t.Fatal("player never landed")
	}
	if math.Abs(p.LastFall-1) > 1e-9 {
		t.Fatalf("fall height %v, want 1", p.LastFall)
	}
}

func TestJumpOntoLedge(t *testing.T) {
	p := NewPlayerState(0, 0, 0)
	p.Jump()
	for i := 0; i < 8; i++ {
		p.UpdateVertical(1.0 / 60)
	}
	if p.Feet() < 0.4 {
		t.Fatalf("jump should be near its peak, feet %v", p.Feet())
	}
	p.SetGround(0.4) // moved over a 0.4 ledge below the feet
	for i := 0; i < 60 && !p.Grounded; i++ {
		p.UpdateVertical(1.0 / 60)
	}
	if !p.Grounded || p.Ground != 0.4 || p.Feet() != 0.4 {
		t.Fatalf("player should land on the ledge: %+v", p)
	}
	if p.LastFall > 0.1 {
		t.Fatalf("a short drop onto a ledge is no fall: %v", p.LastFall)
	}
}
