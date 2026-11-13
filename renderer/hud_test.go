package renderer

import (
	"reflect"
	"testing"
)

func TestSplitLines(t *testing.T) {
	if got := splitLines("A\nB"); !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Fatalf("splitLines = %v", got)
	}
	if splitLines("") != nil {
		t.Fatal("empty string should give no lines")
	}
}

func TestScreenEffectsNeutralIsNoop(t *testing.T) {
	rc := &Raycaster{effects: DefaultEffects(), pixels: []uint8{10, 20, 30, 255}}
	rc.applyScreenEffects(1, 1)
	if !reflect.DeepEqual(rc.pixels, []uint8{10, 20, 30, 255}) {
		t.Fatalf("neutral effects changed pixels: %v", rc.pixels)
	}
}

func TestScreenEffectsRedTintAndDesaturate(t *testing.T) {
	fx := DefaultEffects()
	fx.RedTint = 1
	rc := &Raycaster{effects: fx, pixels: []uint8{100, 100, 100, 255}}
	rc.applyScreenEffects(1, 1)
	if !(rc.pixels[0] > 100 && rc.pixels[1] < 100 && rc.pixels[2] < 100) {
		t.Fatalf("red tint result %v", rc.pixels[:3])
	}

	fx = DefaultEffects()
	fx.Desaturate = 1
	rc = &Raycaster{effects: fx, pixels: []uint8{255, 0, 0, 255}}
	rc.applyScreenEffects(1, 1)
	if rc.pixels[0] != rc.pixels[1] || rc.pixels[1] != rc.pixels[2] {
		t.Fatalf("desaturated pixel not grey: %v", rc.pixels[:3])
	}
}

func TestHUDTextOverlayDarkensAndDraws(t *testing.T) {
	const sw, sh = 200, 120
	rc := &Raycaster{ui: make([]uint8, sw*sh*4)}
	rc.OverlayTitle = "WIN"
	rc.OverlaySubtitle = "line 1\nline 2"
	rc.HUDLines = []string{"ITEMS 1/6"}
	rc.drawHUDText(sw, sh)
	if rc.ui[3] != 175 || rc.ui[0] != 0 { // corner: translucent black veil
		t.Fatalf("corner = %v, want black at alpha 175", rc.ui[:4])
	}
	bright := 0
	for i := 0; i < len(rc.ui); i += 4 {
		if rc.ui[i] > 200 {
			bright++
		}
	}
	if bright == 0 {
		t.Fatal("overlay text was not drawn")
	}
}

func TestHUDGoesToUILayerNotWorld(t *testing.T) {
	const sw, sh = 160, 100
	rc := &Raycaster{pixels: make([]uint8, sw*sh*4), ui: make([]uint8, sw*sh*4), HUDMaxHP: 100, HUDHP: 50}
	rc.HUDLines = []string{"ŻÓŁW"}
	rc.drawHUD(sw, sh)
	rc.drawHUDText(sw, sh)
	for _, v := range rc.pixels {
		if v != 0 {
			t.Fatal("HUD drew into the world buffer")
		}
	}
	lit := 0
	for i := 3; i < len(rc.ui); i += 4 {
		if rc.ui[i] != 0 {
			lit++
		}
	}
	if lit == 0 {
		t.Fatal("HUD missing from UI layer")
	}
}

func TestFillRectClips(t *testing.T) {
	buf := make([]uint8, 4*4*4)
	FillRect(buf, 4, 4, -2, 3, 10, 10, [4]uint8{1, 2, 3, 4})
	if buf[(3*4+0)*4+3] != 4 || buf[(2*4+0)*4+3] != 0 {
		t.Fatal("FillRect clipping wrong")
	}
	FillRect(buf, 4, 4, 9, 9, 2, 2, [4]uint8{9, 9, 9, 9}) // fully outside: no panic
}

func TestEdgeAlphaFindsClippedArt(t *testing.T) {
	tex := &Texture2D{Width: 4, Height: 4, Pixels: make([]uint8, 4*4*4)}
	tex.Pixels[(2*4+1)*4+3] = 255 // interior pixel only
	if EdgeAlpha(tex) != 0 {
		t.Fatal("interior pixel reported as edge")
	}
	tex.Pixels[(3*4+1)*4+3] = 255 // bottom row: allowed for weapon art
	if EdgeAlpha(tex) != 0 {
		t.Fatal("bottom edge must be ignored")
	}
	tex.Pixels[(0*4+2)*4+3] = 120 // top row glow
	if EdgeAlpha(tex) != 120 {
		t.Fatalf("EdgeAlpha = %d", EdgeAlpha(tex))
	}
}
