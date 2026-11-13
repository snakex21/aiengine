package renderer

import (
	"math"
	"testing"
)

func solidTex(r, g, b uint8, n int) *Texture2D {
	px := make([]uint8, n*n*4)
	for i := 0; i < n*n; i++ {
		px[i*4], px[i*4+1], px[i*4+2], px[i*4+3] = r, g, b, 255
	}
	return &Texture2D{Width: int32(n), Height: int32(n), Pixels: px}
}

func TestAnimatedTextureFrameSelection(t *testing.T) {
	rc := testRaycaster(64, 36)
	tm := rc.TexManager
	tm.Register("woda_0", solidTex(0, 0, 200, 8))
	tm.Register("woda_1", solidTex(0, 100, 200, 8))
	tm.Register("woda_2", solidTex(0, 200, 200, 8))
	rc.SetTexAnims([]TexAnim{{Name: "woda", Frames: []string{"woda_0", "woda_1", "woda_2"}, FPS: 4}})
	for _, c := range []struct {
		t    float64
		want uint8
	}{{0, 0}, {0.24, 0}, {0.26, 100}, {0.6, 200}, {0.76, 0}, {10.75, 100}} {
		if g := rc.TextureAt("woda", c.t).Pixels[1]; g != c.want {
			t.Errorf("t=%.2f: green %d, want %d", c.t, g, c.want)
		}
	}
	a := TexAnim{Frames: []string{"a", "b"}, FPS: 2}
	if a.FrameAt(-0.1) != 1 || a.FrameAt(1.2) != 0 {
		t.Fatal("FrameAt must wrap in both directions")
	}
	// Not animated: the plain texture.
	if rc.TextureAt("woda_1", 3) != tm.Get("woda_1") {
		t.Fatal("plain names must resolve unchanged")
	}
}

func TestScrollingFloorAndCellTextures(t *testing.T) {
	rc := testRaycaster(96, 54)
	stripes := solidTex(0, 0, 0, 8)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if x < 4 {
				stripes.Pixels[(y*8+x)*4] = 255
			}
		}
	}
	rc.TexManager.Register("lawa", stripes)
	m := rc.Map
	m.TexNames = []string{"", "lawa"}
	m.FloorTex = make([]uint8, m.Width*m.Height)
	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			m.FloorTex[y*m.Width+x] = 1
		}
	}
	rc.SetTexAnims([]TexAnim{{Name: "lawa", ScrollU: 0.5}})
	frame := func(at float64) []uint8 {
		rc.AnimTime = at
		rc.renderToBuffer(nil)
		return append([]uint8(nil), rc.pixels...)
	}
	a, b := frame(0), frame(0.5) // half a stripe period later
	row := 53
	red := 0
	diff := 0
	for x := 0; x < 96; x++ {
		i := (row*96 + x) * 4
		if a[i] > 0 {
			red++
		}
		if a[i] != b[i] {
			diff++
		}
	}
	if red == 0 || red == 96 {
		t.Fatal("per-cell floor texture not used")
	}
	if diff < 20 {
		t.Fatalf("scrolling floor did not move (%d pixels changed)", diff)
	}
}

func TestPeggedDoorTextureMovesWithTheDoor(t *testing.T) {
	// A door cell ahead (ceiling = floor + h) drawn as an upper step face:
	// pegged, its texture's bottom row stays at the door's bottom edge.
	rc := testRaycaster(64, 64)
	grad := solidTex(0, 0, 0, 8)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			grad.Pixels[(y*8+x)*4+1] = uint8(30 * y) // green grows downwards
		}
	}
	rc.TexManager.Register("stone", grad)
	m := rc.Map
	m.SetHeights(1)
	m.Flags = make([]uint8, m.Width*m.Height)
	door := 3*m.Width + 6
	m.Flags[door] = CellPeg
	rc.Player.X, rc.Player.Y, rc.Player.Angle = 3.5, 3.5, 0
	bottomGreen := func(open float64) uint8 {
		m.Ceil[door] = open
		rc.renderToBuffer(nil)
		// Lowest door row in the centre column: the face ends at the row of
		// height `open` at distance 2.5.
		f := 64/2 + (0.5-open)*Focal(64, rc.fov()/2)/2.5
		y := int(math.Ceil(f-0.5)) - 1
		return rc.pixels[(y*64+32)*4+1]
	}
	// Bottom texture row (green 210) shaded by distance 2.5.
	want := uint8(int32(210) * distShade(2.5) >> 8)
	g1, g2 := bottomGreen(0.2), bottomGreen(0.45)
	if g1 != want || g2 != want {
		t.Fatalf("pegged door bottom must show the texture's bottom row: %d %d, want %d", g1, g2, want)
	}
	// Unpegged faces stay anchored to the world: the bottom row changes.
	m.Flags[door] = 0
	if bottomGreen(0.2) == bottomGreen(0.45) {
		t.Fatal("unpegged face must not follow the door")
	}
}

func TestRotationPickDrawsTheRightView(t *testing.T) {
	rc := testRaycaster(64, 64)
	views := []string{"v0", "v1", "v2", "v3", "v4", "v5", "v6", "v7"}
	for i, n := range views {
		rc.TexManager.Register(n, solidTex(uint8(10+30*i), 0, 0, 8))
	}
	rc.Player.X, rc.Player.Y, rc.Player.Angle = 3.5, 3.5, 0
	s := &Sprite{X: 6.5, Y: 3.5, Scale: 0.8, Alpha: 1, Active: true, TexName: "v0", Rotations: views}
	for _, c := range []struct {
		facing float64
		view   int
	}{{math.Pi, 0}, {0, 4}, {math.Pi / 2, 2}, {-math.Pi / 2, 6}, {3 * math.Pi / 4, 1}} {
		s.Angle = c.facing
		rc.renderToBuffer([]*Sprite{s})
		want := uint8(int32(10+30*c.view) * distShade(3) >> 8)
		if r := rc.pixels[(32*64+32)*4]; r != want {
			t.Errorf("facing %.2f: drew red %d, want view %d", c.facing, r, c.view)
		}
	}
}
