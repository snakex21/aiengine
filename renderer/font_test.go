package renderer

import "testing"

// litColumns returns which of the 5 glyph columns have any pixel set when
// ch is drawn at the origin.
func litColumns(t *testing.T, ch string) [FontW - 1]bool {
	t.Helper()
	const sw, sh = 16, 16
	px := make([]uint8, sw*sh*4)
	DrawConsoleString(px, sw, 0, 0, ch, [4]uint8{255, 255, 255, 255})
	var cols [FontW - 1]bool
	for y := 0; y < sh; y++ {
		for x := 0; x < FontW-1; x++ {
			if px[(y*sw+x)*4] != 0 {
				cols[x] = true
			}
		}
	}
	return cols
}

func TestConsoleFontDrawsAllFiveColumns(t *testing.T) {
	// 'H' has strokes in its leftmost and rightmost columns.
	cols := litColumns(t, "H")
	if !cols[0] || !cols[4] {
		t.Fatalf("H columns lit = %v, want both outer columns", cols)
	}
}

func TestDrawTextScaledClipsAndScales(t *testing.T) {
	const sw, sh = 40, 20
	px := make([]uint8, sw*sh*4)
	// Partly off-screen on every side must not panic.
	DrawTextScaled(px, sw, sh, -5, -5, "HELLO, WORLD (1/2) 100%", [4]uint8{255, 0, 0, 255}, 3)
	if TextWidth("AB", 3) != 2*FontW*3 {
		t.Fatalf("TextWidth = %d", TextWidth("AB", 3))
	}
}

func TestPolishGlyphsHaveMarks(t *testing.T) {
	for _, r := range "ąćęłńóśźżĄĆĘŁŃÓŚŹŻ" {
		if !HasGlyph(r) {
			t.Errorf("missing glyph %q", r)
		}
	}
	base, _ := lookupGlyph('O')
	acc, _ := lookupGlyph('Ó')
	if acc == base || acc[0] == 0 || acc[1] == 0 {
		t.Fatalf("Ó should be O plus an accent: %v vs %v", acc, base)
	}
	if g, _ := lookupGlyph('ą'); g[9] == 0 {
		t.Fatal("ą needs an ogonek below the body")
	}
	if g, _ := lookupGlyph('ż'); g[0] == 0 {
		t.Fatal("ż needs a dot above")
	}
	if a, b := mustGlyph(t, 'ł'), mustGlyph(t, 'L'); a == b {
		t.Fatal("ł must differ from L")
	}
	// ASCII stays intact.
	if g, _ := lookupGlyph('H'); g[2] != consoleFont['H'][0] || g[0] != 0 {
		t.Fatal("ASCII glyph changed")
	}
}

func mustGlyph(t *testing.T, r rune) glyph {
	t.Helper()
	g, ok := lookupGlyph(r)
	if !ok {
		t.Fatalf("no glyph %q", r)
	}
	return g
}

func TestPolishTextWidthCountsRunes(t *testing.T) {
	if TextWidth("Wyjście", 2) != 7*FontW*2 {
		t.Fatalf("TextWidth = %d", TextWidth("Wyjście", 2))
	}
	const sw, sh = 120, 30
	px := make([]uint8, sw*sh*4)
	DrawTextScaled(px, sw, sh, 2, 4, "ŻÓŁĆ", [4]uint8{255, 255, 255, 255}, 1)
	// The dot of Ż sits two rows above the body top (y=4 → y=2).
	if px[(2*sw+2+2)*4] != 255 {
		t.Fatal("Ż dot not drawn above the letter")
	}
}
