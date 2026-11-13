package anim

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
)

const desc = `# test sheet
SHEET straznik_8.png
GRID 5 4
ROTATIONS 8
MIRROR yes
ANIM idle   frames=0,1   fps=2
ANIM walk   frames=1,2,3,2 fps=8
ANIM attack frames=2,3 fps=10 loop=no
ANIM death  frames=3 fps=4
`

func testSet(t *testing.T) (*Desc, *Set) {
	t.Helper()
	d, err := ParseDesc(strings.NewReader(desc))
	if err != nil {
		t.Fatal(err)
	}
	return d, NewSet(d, func(f, r int) string { return "f" + string(rune('0'+f)) + "r" + string(rune('0'+r)) })
}

func TestParseDescAndCells(t *testing.T) {
	d, s := testSet(t)
	if d.Sheet != "straznik_8.png" || d.Frames() != 4 || s.Rotations() != 8 || len(s.Clips) != 4 {
		t.Fatalf("desc %+v rotations %d clips %d", d, s.Rotations(), len(s.Clips))
	}
	if c := s.Clips["attack"]; c.Loop || c.FPS != 10 {
		t.Fatalf("attack clip %+v", c)
	}
	if !s.Clips["walk"].Loop || s.Clips["death"].Loop {
		t.Fatal("walk loops by default, death does not")
	}
	// Mirrored views: rotation 6 is column 2 flipped, rotation 4 column 4.
	if c, r, m := d.Cell(3, 6); c != 2 || r != 3 || !m {
		t.Fatalf("cell(3,6) = %d %d %v", c, r, m)
	}
	if c, _, m := d.Cell(0, 4); c != 4 || m {
		t.Fatal("rotation 4 must be drawn, not mirrored")
	}
	for _, bad := range []string{
		"GRID 4 4\nROTATIONS 8\n",                           // no sheet
		"SHEET a.png\nGRID 4 4\nROTATIONS 8\n",              // too few columns
		"SHEET a.png\nGRID 2 2\nANIM idle frames=0,9\n",     // frame outside
		"SHEET a.png\nGRID 2 2\nANIM idle frames=0 fpx=2\n", // unknown key
	} {
		if _, err := ParseDesc(strings.NewReader(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestStateTransitionsAndFrameTiming(t *testing.T) {
	_, s := testSet(t)
	p := NewPlayer(s)
	if p.State != Idle || p.Frame() != 0 {
		t.Fatalf("start %s frame %d", p.State, p.Frame())
	}
	p.Update(0.49)
	if p.Frame() != 0 {
		t.Fatal("idle at 2 fps shows frame 0 for 0.5 s")
	}
	p.Update(0.02)
	if p.Frame() != 1 {
		t.Fatal("idle frame 1 after 0.5 s")
	}
	p.Update(0.5)
	if p.Frame() != 0 || p.Done() {
		t.Fatal("idle loops")
	}
	// Walk: 8 fps through 1,2,3,2; playing the same state keeps the time.
	p.Play(Walk)
	p.Update(0.13)
	p.Play(Walk)
	if p.Frame() != 2 {
		t.Fatalf("walk at 0.13 s = frame %d, want 2", p.Frame())
	}
	// Attack does not loop: holds its last frame, then Done.
	p.Play(Attack)
	p.Update(0.15)
	if p.Frame() != 3 || p.Done() {
		t.Fatalf("attack at 0.15 s: frame %d done %v", p.Frame(), p.Done())
	}
	p.Update(0.1)
	if p.Frame() != 3 || !p.Done() {
		t.Fatal("attack must end on its last frame")
	}
	// Pain is missing: falls back to idle. Death has no fallback.
	p.Restart(Pain)
	if !p.HasClip() || p.Frame() != 0 {
		t.Fatal("missing pain must fall back to idle")
	}
	delete(s.Clips, Death)
	p.Restart(Death)
	if p.HasClip() || !p.Done() {
		t.Fatal("missing death must report done without frames")
	}
	// Textures: frame 3 with all 8 views.
	p.Restart(Attack)
	p.Update(1)
	name, views := p.Textures()
	if name != "f3r0" || len(views) != 8 || views[5] != "f3r5" {
		t.Fatalf("textures %s %v", name, views)
	}
}

func TestFakeRotationsAndDeath(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 2; y < 16; y++ {
		for x := 4; x < 12; x++ {
			c := color.RGBA{200, 120, 60, 255}
			if x < 8 {
				c = color.RGBA{60, 200, 90, 255} // left half green: shows mirroring
			}
			src.SetRGBA(x, y, c)
		}
	}
	views := FakeRotations(src, 8)
	if len(views) != 8 || views[0].RGBAAt(5, 8) != src.RGBAAt(5, 8) {
		t.Fatal("view 0 must be the original")
	}
	width := func(img *image.RGBA) int {
		n := 0
		for x := 0; x < 16; x++ {
			if img.RGBAAt(x, 8).A > 0 {
				n++
			}
		}
		return n
	}
	if width(views[2]) >= width(views[0]) || width(views[2]) != width(views[6]) {
		t.Fatalf("side views must be narrower and symmetric: %d %d %d", width(views[0]), width(views[2]), width(views[6]))
	}
	// Views 1 and 7 are mirror images.
	for x := 0; x < 16; x++ {
		if views[1].RGBAAt(x, 8) != views[7].RGBAAt(15-x, 8) {
			t.Fatal("views 1 and 7 must mirror each other")
		}
	}
	// The back view is darker than the front.
	lum := func(c color.RGBA) int { return int(c.R) + int(c.G) + int(c.B) }
	if lum(views[4].RGBAAt(8, 8)) >= lum(views[0].RGBAAt(8, 8)) {
		t.Fatal("back view must be darker")
	}
	death := FakeDeath(src, 3)
	top := func(img *image.RGBA) int {
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				if img.RGBAAt(x, y).A > 0 {
					return y
				}
			}
		}
		return 16
	}
	if !(top(death[0]) > top(src) && top(death[2]) > top(death[0])) {
		t.Fatalf("death frames must sink: %d %d %d", top(src), top(death[0]), top(death[2]))
	}
}

func TestCutSheetMirrorsAndSquares(t *testing.T) {
	d, _ := testSet(t)
	img := image.NewRGBA(image.Rect(0, 0, 50, 48)) // 5x4 cells of 10x12
	for r := 0; r < 4; r++ {
		for c := 0; c < 5; c++ {
			img.SetRGBA(c*10, r*12+11, color.RGBA{uint8(c * 40), uint8(r * 60), 0, 255}) // marker bottom-left
		}
	}
	cells := CutSheet(img, d)
	if len(cells) != 4 || len(cells[0]) != 8 || cells[0][0].Bounds().Dx() != 12 || cells[0][0].Bounds().Dy() != 12 {
		t.Fatal("cells must be 4 frames x 8 views of 12x12")
	}
	// Marker of column 2, row 1 is bottom-left (x = 1 after centring);
	// rotation 6 mirrors column 2: bottom-right.
	if cells[1][2].RGBAAt(1, 11).R != 80 || cells[1][6].RGBAAt(10, 11).R != 80 || math.Abs(float64(cells[1][6].RGBAAt(10, 11).G)-60) > 0 {
		t.Fatalf("mirror: %v %v", cells[1][2].RGBAAt(1, 11), cells[1][6].RGBAAt(10, 11))
	}
}
