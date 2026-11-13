package renderer

import (
	"math"
	"testing"
)

// wallTopEdge returns, per column, the first row that is not ceiling colour
// (the wall's top edge), or -1 when the wall covers the whole column.
func wallTopEdge(rc *Raycaster) []int {
	sw, sh := int(rc.screenW), int(rc.screenH)
	ceil := colorToRGBA(rc.CeilColor)
	out := make([]int, sw)
	for x := 0; x < sw; x++ {
		out[x] = -1
		for y := 0; y < sh/2; y++ {
			i := (y*sw + x) * 4
			if rc.pixels[i] != ceil[0] || rc.pixels[i+1] != ceil[1] || rc.pixels[i+2] != ceil[2] {
				out[x] = y
				break
			}
		}
	}
	return out
}

// maxLineDeviation fits a straight line through the first and last valid
// samples of edge[a:b] and returns the largest deviation in pixels.
func maxLineDeviation(edge []int, a, b int) float64 {
	x0, x1 := a, b-1
	y0, y1 := float64(edge[x0]), float64(edge[x1])
	worst := 0.0
	for x := a; x < b; x++ {
		want := y0 + (y1-y0)*float64(x-x0)/float64(x1-x0)
		worst = math.Max(worst, math.Abs(float64(edge[x])-want))
	}
	return worst
}

// A flat wall must project to straight edges (no fisheye bowing), straight
// on and at an angle, close up and far away, at several FOVs.
func TestFlatWallEdgesStayStraight(t *testing.T) {
	cases := []struct {
		name        string
		x, angleDeg float64
		fovDeg      float64
		from, to    int // column range that sees the x=15 wall face only
	}{
		{"straight far fov66", 10.5, 0, 66, 0, 320},
		{"straight near fov66", 13.6, 0, 66, 0, 320},
		{"straight near fov100", 13.6, 0, 100, 0, 320},
		{"oblique fov66", 11.5, 25, 66, 0, 200},
	}
	for _, c := range cases {
		rc := testRaycaster(320, 180)
		rc.TexManager.textures["brick"] = nil // flat colour walls: clean edges
		rc.Player.X, rc.Player.Y = c.x, 7.5
		rc.Player.Angle = c.angleDeg * math.Pi / 180
		rc.Player.FOV = c.fovDeg * math.Pi / 180
		rc.renderToBuffer(nil)
		edge := wallTopEdge(rc)
		for x := c.from; x < c.to; x++ {
			if edge[x] < 0 {
				t.Fatalf("%s: column %d fully covered by wall; pick another distance", c.name, x)
			}
		}
		if d := maxLineDeviation(edge, c.from, c.to); d > 1.5 {
			t.Errorf("%s: wall top edge bends by %.1f px (want a straight line)", c.name, d)
		}
	}
}

// Walls use the same focal length horizontally and vertically (square
// pixels): a 1x1 wall face seen straight on is as wide as it is tall.
func TestWallAspectIsSquare(t *testing.T) {
	grid := make([][]MapCell, 9)
	for y := range grid {
		grid[y] = make([]MapCell, 9)
	}
	grid[4][7] = CellWall1 // single block 3 cells ahead of the player
	rc := testRaycaster(320, 180)
	rc.Map = NewMapData(grid)
	rc.TexManager.textures["brick"] = nil
	rc.Player.X, rc.Player.Y, rc.Player.Angle = 3.5, 4.5, 0
	rc.renderToBuffer(nil)
	cols := 0
	for _, z := range rc.zBuffer {
		if z > 0 && z < 3.6 { // the block, not the out-of-map boundary
			cols++
		}
	}
	edge := wallTopEdge(rc)
	height := 2 * (90 - edge[160])
	if math.Abs(float64(cols-height)) > 3 {
		t.Fatalf("block is %d px wide but %d px tall", cols, height)
	}
}
