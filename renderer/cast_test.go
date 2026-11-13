package renderer

import (
	"math"
	"testing"
)

func testRaycaster(w, h int) *Raycaster {
	grid := make([][]MapCell, 16)
	for y := range grid {
		grid[y] = make([]MapCell, 16)
		for x := range grid[y] {
			if x == 0 || y == 0 || x == 15 || y == 15 {
				grid[y][x] = CellWall1
			}
		}
	}
	tm := NewTextureManager()
	checker := func(a, b uint8) *Texture2D {
		px := make([]uint8, 8*8*4)
		for i := 0; i < 64; i++ {
			c := a
			if (i/8+i%8)%2 == 0 {
				c = b
			}
			px[i*4], px[i*4+1], px[i*4+2], px[i*4+3] = c, c, c, 255
		}
		return &Texture2D{Width: 8, Height: 8, Pixels: px}
	}
	tm.textures["brick"] = checker(200, 100)
	tm.textures["floor_t"] = checker(250, 10)
	rc := NewRaycaster(NewMapData(grid), NewPlayerState(3.5, 3.5, 0.3), tm)
	rc.screenW, rc.screenH = int32(w), int32(h)
	rc.pixels = make([]uint8, w*h*4)
	rc.ui = make([]uint8, w*h*4)
	rc.zBuffer = make([]float64, w)
	return rc
}

func TestHorPlusFOV(t *testing.T) {
	f := 66 * math.Pi / 180
	if math.Abs(HorPlusFOV(f, 16.0/9.0)-f) > 1e-12 {
		t.Fatal("16:9 must keep the configured FOV")
	}
	uw := HorPlusFOV(f, 3440.0/1440.0)
	if uw <= f || HorPlusFOV(f, 4.0/3.0) >= f {
		t.Fatal("wider screens must see more, narrower less")
	}
	// Vertical focal length (in screen heights) is the same for all aspects.
	focal := func(fov, aspect float64) float64 { return aspect / 2 / math.Tan(fov/2) }
	if math.Abs(focal(f, 16.0/9.0)-focal(uw, 3440.0/1440.0)) > 1e-9 {
		t.Fatal("Hor+ must keep the vertical scale")
	}
}

func TestUIScaleGrowsWithResolution(t *testing.T) {
	for h, want := range map[int]int{360: 1, 540: 2, 720: 2, 1080: 3, 1440: 4, 2160: 6} {
		if got := UIScale(h); got != want {
			t.Errorf("UIScale(%d) = %d, want %d", h, got, want)
		}
	}
}

func TestTexturedFloorAndWallsRender(t *testing.T) {
	rc := testRaycaster(160, 90)
	rc.FloorTexture = "floor_t"
	rc.renderToBuffer(nil)
	// Bottom row is floor: with a texture it varies, with flat colour it doesn't.
	row := 89
	vals := map[uint8]bool{}
	for x := 0; x < 160; x++ {
		vals[rc.pixels[(row*160+x)*4]] = true
	}
	if len(vals) < 2 {
		t.Fatal("textured floor rendered as a flat colour")
	}
	for x, z := range rc.zBuffer {
		if z <= 0 {
			t.Fatalf("column %d hit nothing in a closed room", x)
		}
	}
	rc.FloorTexture = ""
	rc.renderToBuffer(nil)
	if rc.pixels[(row*160+5)*4] != floorColor.R {
		t.Fatal("flat floor colour not used without a texture")
	}
}

func TestParallelRenderIsDeterministic(t *testing.T) {
	a, b := testRaycaster(320, 180), testRaycaster(320, 180)
	a.FloorTexture, b.FloorTexture = "floor_t", "floor_t"
	a.renderToBuffer(nil)
	b.renderToBuffer(nil)
	for i := range a.pixels {
		if a.pixels[i] != b.pixels[i] {
			t.Fatalf("pixel %d differs between runs", i)
		}
	}
}

func BenchmarkRender1080p(b *testing.B) {
	rc := testRaycaster(1920, 1080)
	rc.FloorTexture = "floor_t"
	for i := 0; i < b.N; i++ {
		rc.renderToBuffer(nil)
	}
}

func BenchmarkRender2160p(b *testing.B) {
	rc := testRaycaster(3840, 2160)
	rc.FloorTexture = "floor_t"
	for i := 0; i < b.N; i++ {
		rc.renderToBuffer(nil)
	}
}

func BenchmarkRender1440p(b *testing.B) {
	rc := testRaycaster(2560, 1440)
	rc.FloorTexture = "floor_t"
	for i := 0; i < b.N; i++ {
		rc.renderToBuffer(nil)
	}
}
