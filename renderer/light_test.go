package renderer

import (
	"testing"
)

func lum(c [3]uint8) int { return int(c[0]) + int(c[1]) + int(c[2]) }

func litRoom() *Raycaster {
	rc := testRaycaster(320, 180)
	rc.FloorTexture, rc.CeilingTexture = "floor_t", "floor_t"
	return rc
}

func TestFullLightLayerMatchesUnlitRender(t *testing.T) {
	a, b := litRoom(), litRoom()
	for y := 0; y < b.Map.Height; y++ {
		for x := 0; x < b.Map.Width; x++ {
			b.Map.SetLight(x, y, 1)
		}
	}
	a.renderToBuffer(nil)
	b.renderToBuffer(nil)
	for i := range a.pixels {
		if a.pixels[i] != b.pixels[i] {
			t.Fatalf("light 1 everywhere must look like no lighting (pixel %d: %d vs %d)", i, a.pixels[i], b.pixels[i])
		}
	}
}

func TestDarkCellsRenderDarker(t *testing.T) {
	bright, dark := litRoom(), litRoom()
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			dark.Map.SetLight(x, y, 0.25)
		}
	}
	bright.renderToBuffer(nil)
	dark.renderToBuffer(nil)
	for _, p := range [][2]int{{160, 10}, {160, 90}, {160, 170}, {20, 120}} {
		lb, ld := lum(px(bright, p[0], p[1])), lum(px(dark, p[0], p[1]))
		if ld*3 > lb || ld == 0 && lb == 0 {
			t.Errorf("pixel %v: dark %d vs bright %d (want about 1/4)", p, ld, lb)
		}
	}
}

func TestWallTakesLightOfCellInFront(t *testing.T) {
	rc := litRoom()
	rc.Player.X, rc.Player.Y, rc.Player.Angle = 3.5, 8.5, 0
	ref := litRoom()
	ref.Player.X, ref.Player.Y, ref.Player.Angle = 3.5, 8.5, 0
	// Darken the cells right in front of the east wall (x = 14).
	for y := 0; y < 16; y++ {
		rc.Map.SetLight(14, y, 0.2)
	}
	rc.renderToBuffer(nil)
	ref.renderToBuffer(nil)
	if lum(px(rc, 160, 90))*2 > lum(px(ref, 160, 90)) {
		t.Fatalf("wall facing a dark cell must be dark: %v vs %v", px(rc, 160, 90), px(ref, 160, 90))
	}
}

func TestFogBlendsDistanceTowardsColour(t *testing.T) {
	rc := litRoom()
	rc.Fog = Fog{Density: 2, Color: [3]uint8{200, 0, 0}}
	rc.Player.X, rc.Player.Y, rc.Player.Angle = 1.5, 8.5, 0 // 13 cells to the far wall
	rc.renderToBuffer(nil)
	c := px(rc, 160, 90)
	if c[0] < 190 || c[1] > 10 {
		t.Fatalf("far wall in thick fog should be fog-red, got %v", c)
	}
	near := px(rc, 160, 179) // floor at the player's feet
	if near[0] > 190 {
		t.Fatalf("nearby floor should not be fully fogged: %v", near)
	}
}

func TestPointLightLightsVisibleCellsOnly(t *testing.T) {
	m := NewMapData([][]MapCell{
		{0, 0, 0, 1, 0},
		{0, 0, 0, 1, 0},
	})
	fp := LightFootprint(m, 1.5, 0.5, 3)
	lit := map[int]float64{}
	for _, c := range fp {
		lit[c.Index] = c.Weight
	}
	if lit[1] != 1 {
		t.Errorf("the light's own cell gets full weight, got %v", lit[1])
	}
	if lit[2] <= 0 || lit[2] >= 1 {
		t.Errorf("neighbour cell should get partial light, got %v", lit[2])
	}
	if _, ok := lit[4]; ok {
		t.Error("cell behind the wall must stay dark")
	}
	if _, ok := lit[3]; ok {
		t.Error("solid cells are not lit")
	}
}

func TestPointLightAndFlashBrighten(t *testing.T) {
	base := litRoom()
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			base.Map.SetLight(x, y, 0.2)
		}
	}
	base.renderToBuffer(nil)
	ref := lum(px(base, 160, 170))

	withLight := litRoom()
	withLight.Map = base.Map
	withLight.Lights = []PointLight{{X: 3.5, Y: 3.5, Radius: 3, Level: 1}}
	withLight.renderToBuffer(nil)
	if lum(px(withLight, 160, 170)) <= ref*2 {
		t.Errorf("point light at the player should brighten the floor: %d vs %d", lum(px(withLight, 160, 170)), ref)
	}

	flash := litRoom()
	flash.Map = base.Map
	flash.Flash = 1
	flash.renderToBuffer(nil)
	if lum(px(flash, 160, 170)) <= ref*2 {
		t.Errorf("weapon flash should brighten nearby floor: %d vs %d", lum(px(flash, 160, 170)), ref)
	}
}

func TestSpriteLitByItsCell(t *testing.T) {
	shot := func(light float64) int {
		rc := heightRoom(320, 180)
		rc.TexManager.textures["white"] = spriteTex(200, 200, 200)
		rc.Map.SetLight(6, 8, light)
		s := &Sprite{X: 6.5, Y: 8.5, Scale: 0.6, Z: 0.2, TexName: "white", Alpha: 1, Active: true}
		rc.renderToBuffer([]*Sprite{s})
		return lum(px(rc, 160, 95))
	}
	if b, d := shot(1), shot(0.3); d*2 > b {
		t.Fatalf("sprite in a dark cell must be darker: %d vs %d", d, b)
	}
}

func benchLit(b *testing.B, w, h int) {
	rc := testRaycaster(w, h)
	rc.FloorTexture, rc.CeilingTexture = "floor_t", "floor_t"
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			rc.Map.SetLight(x, y, 0.3+float64((x+y)%5)*0.15)
		}
	}
	rc.Lights = []PointLight{{X: 5, Y: 5, Radius: 4, Level: 0.8}, {X: 10, Y: 9, Radius: 3, Level: 0.6}}
	rc.Fog = Fog{Density: 0.15, Color: [3]uint8{10, 10, 20}}
	rc.Flash = 0.5
	for i := 0; i < b.N; i++ {
		rc.renderToBuffer(nil)
	}
}

func BenchmarkRenderLit1080p(b *testing.B) { benchLit(b, 1920, 1080) }
func BenchmarkRenderLit1440p(b *testing.B) { benchLit(b, 2560, 1440) }
func BenchmarkRenderLit2160p(b *testing.B) { benchLit(b, 3840, 2160) }

// A sprite right in front of the camera covering most of the screen.
func BenchmarkRenderBigSprite1080p(b *testing.B) {
	rc := testRaycaster(1920, 1080)
	rc.FloorTexture = "floor_t"
	big := &Texture2D{Width: 256, Height: 256, Pixels: make([]uint8, 256*256*4)}
	for i := 3; i < len(big.Pixels); i += 4 {
		big.Pixels[i-3], big.Pixels[i] = 200, 255
	}
	rc.TexManager.textures["big"] = big
	s := &Sprite{X: 4.2, Y: 3.7, Scale: 0.9, TexName: "big", Alpha: 1, Active: true}
	for i := 0; i < b.N; i++ {
		rc.renderToBuffer([]*Sprite{s})
	}
}
