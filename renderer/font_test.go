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
