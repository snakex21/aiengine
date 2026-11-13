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
	rc := &Raycaster{pixels: make([]uint8, sw*sh*4)}
	for i := range rc.pixels {
		rc.pixels[i] = 90
	}
	rc.OverlayTitle = "WIN"
	rc.OverlaySubtitle = "line 1\nline 2"
	rc.HUDLines = []string{"ITEMS 1/6"}
	rc.drawHUDText(sw, sh)
	if rc.pixels[0] != 30 { // corner darkened to a third
		t.Fatalf("corner = %d, want 30", rc.pixels[0])
	}
	bright := 0
	for i := 0; i < len(rc.pixels); i += 4 {
		if rc.pixels[i] > 200 {
			bright++
		}
	}
	if bright == 0 {
		t.Fatal("overlay text was not drawn")
	}
}
