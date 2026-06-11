// tools/sprites_test.go — Tests for sprite sheet cutting tools.

package tools

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func saveTempPNG(t *testing.T, img image.Image, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp PNG: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode temp PNG: %v", err)
	}
	return path
}

func pngDecodeFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// ---------------------------------------------------------------------------
// AutoCrop

func TestAutoCrop_TrimsEmptyEdges(t *testing.T) {
	// 10×10 image, only a 2×2 block in the center is non‑empty.
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img.SetRGBA(4, 4, color.RGBA{255, 255, 255, 255})
	img.SetRGBA(5, 4, color.RGBA{255, 255, 255, 255})
	img.SetRGBA(4, 5, color.RGBA{255, 255, 255, 255})
	img.SetRGBA(5, 5, color.RGBA{255, 255, 255, 255})

	out := AutoCrop(img)
	b := out.Bounds()
	if b.Dx() != 2 || b.Dy() != 2 {
		t.Fatalf("expected 2×2 after crop, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestAutoCrop_ExcludesBlack(t *testing.T) {
	// Black pixel (R+G+B = 0) should be treated as empty margin.
	img := image.NewRGBA(image.Rect(0, 0, 5, 5))
	img.SetRGBA(0, 0, color.RGBA{0, 0, 0, 255})         // black – ignored
	img.SetRGBA(4, 4, color.RGBA{255, 0, 0, 255})       // red – content

	out := AutoCrop(img)
	b := out.Bounds()
	if b.Dx() != 1 || b.Dy() != 1 {
		t.Fatalf("black should be ignored, expected 1×1 got %dx%d", b.Dx(), b.Dy())
	}
	r, _, _, _ := out.At(0, 0).RGBA()
	if r>>8 != 255 {
		t.Fatal("red pixel lost")
	}
}

func TestAutoCrop_KeepsDarkButNotBlack(t *testing.T) {
	// Dark brown (R+G+B ≈ 100) should survive.
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	img.SetRGBA(1, 1, color.RGBA{60, 30, 10, 255}) // sum = 100 > 30

	out := AutoCrop(img)
	b := out.Bounds()
	if b.Dx() != 1 || b.Dy() != 1 {
		t.Fatal("dark brown should survive AutoCrop")
	}
}

func TestAutoCrop_AllEmpty(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	// Completely transparent — no content.
	out := AutoCrop(img)
	b := out.Bounds()
	if b.Dx() != 1 || b.Dy() != 1 {
		t.Fatalf("empty image should return 1×1, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestAutoCrop_AllBlack(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 6, 6))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
	out := AutoCrop(img)
	b := out.Bounds()
	if b.Dx() != 1 || b.Dy() != 1 {
		t.Fatalf("all‑black should return 1×1, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestAutoCrop_ZeroMargin(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 15))
	// Single pixel at exact position.
	img.SetRGBA(7, 5, color.RGBA{200, 100, 50, 255})

	out := AutoCrop(img)
	b := out.Bounds()
	if b.Min.X != 0 || b.Min.Y != 0 || b.Dx() != 1 || b.Dy() != 1 {
		t.Fatalf("expected exactly 1×1 at origin, got %s", b)
	}
}

// ---------------------------------------------------------------------------
// CutSprite  (now includes AutoCrop)

func TestCutSprite_Basic(t *testing.T) {
	// Create a sheet with a coloured block surrounded by transparent area.
	img := image.NewRGBA(image.Rect(0, 0, 100, 80))
	// Red block at (30,20) → (60,50)
	for y := 20; y < 50; y++ {
		for x := 30; x < 60; x++ {
			img.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	src := saveTempPNG(t, img, "basic.png")
	dst := filepath.Join(t.TempDir(), "out", "frame.png")

	// Cut covers the block plus margin.
	if err := CutSprite(src, 10, 5, 80, 60, dst); err != nil {
		t.Fatalf("CutSprite: %v", err)
	}

	read, err := pngDecodeFile(dst)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	// Without AutoCrop the output is the raw cut size: 80×60.
	b := read.Bounds()
	if b.Dx() != 80 || b.Dy() != 60 {
		t.Fatalf("expected 80x60, got %dx%d", b.Dx(), b.Dy())
	}
	// Verify the red block is present (offset by cut origin: 30-10=20, 20-5=15).
	r, _, _, _ := read.At(20, 15).RGBA()
	if r>>8 != 255 {
		t.Fatal("red content missing")
	}
}

func TestCutSprite_PreservesAlpha(t *testing.T) {
	// Semi‑transparent red block on fully transparent background.
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 3; y < 7; y++ {
		for x := 3; x < 7; x++ {
			img.SetRGBA(x, y, color.RGBA{255, 0, 0, 64})
		}
	}
	src := saveTempPNG(t, img, "alpha.png")
	dst := filepath.Join(t.TempDir(), "out", "alpha.png")

	if err := CutSprite(src, 0, 0, 10, 10, dst); err != nil {
		t.Fatalf("CutSprite: %v", err)
	}

	read, err := pngDecodeFile(dst)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Without AutoCrop: 10×10 (the raw cut size).
	b := read.Bounds()
	if b.Dx() != 10 || b.Dy() != 10 {
		t.Fatalf("expected 10x10, got %dx%d", b.Dx(), b.Dy())
	}
	// Pixel (3,3) corresponds to original (3,3) — part of the red block.
	_, _, _, a := read.At(3, 3).RGBA()
	if a>>8 != 64 {
		t.Fatalf("alpha not preserved: expected 64, got %d", a>>8)
	}
}

func TestCutSprite_InvalidDimensions(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "bad.png")
	err := CutSprite("nonexistent.png", 0, 0, 0, 10, dst)
	if err == nil {
		t.Fatal("expected error for zero width")
	}
}

func TestCutSprite_OutOfBounds(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	src := saveTempPNG(t, img, "small.png")
	dst := filepath.Join(t.TempDir(), "out", "oob.png")

	err := CutSprite(src, 25, 0, 20, 10, dst)
	if err == nil {
		t.Fatal("expected error for out‑of‑bounds frame")
	}
}

func TestCutSprite_SourceNotFound(t *testing.T) {
	err := CutSprite("nonexistent.png", 0, 0, 10, 10, "/tmp/out.png")
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

// ---------------------------------------------------------------------------
// CutSpriteSheet

func TestCutSpriteSheet_Uniform(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 120, 60))
	// Row 0: three small coloured dots at different X offsets.
	img.SetRGBA(20, 10, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(60, 10, color.RGBA{0, 255, 0, 255})
	img.SetRGBA(100, 10, color.RGBA{0, 0, 255, 255})
	// Row 1: two dots.
	img.SetRGBA(20, 40, color.RGBA{255, 255, 0, 255})
	img.SetRGBA(60, 40, color.RGBA{0, 255, 255, 255})

	src := saveTempPNG(t, img, "sheet.png")
	dst := filepath.Join(t.TempDir(), "sprites")

	if err := CutSpriteSheet(src, 2, []int{3, 3}, []string{"idle", "walk"}, dst); err != nil {
		t.Fatalf("CutSpriteSheet: %v", err)
	}

	expected := []string{"idle_0.png", "idle_1.png", "idle_2.png", "walk_0.png", "walk_1.png", "walk_2.png"}
	for _, name := range expected {
		p := filepath.Join(dst, name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("missing file: %s", p)
		}
	}
}

func TestCutSpriteSheet_NonUniformRows(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 300, 100))
	// Row 0: 3 dots, Row 1: 2 dots.
	img.SetRGBA(50, 25, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(150, 25, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(250, 25, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(75, 75, color.RGBA{0, 0, 255, 255})
	img.SetRGBA(225, 75, color.RGBA{0, 0, 255, 255})

	src := saveTempPNG(t, img, "nonuniform.png")
	dst := filepath.Join(t.TempDir(), "sprites")

	if err := CutSpriteSheet(src, 2, []int{3, 2}, []string{"run", "death"}, dst); err != nil {
		t.Fatalf("CutSpriteSheet: %v", err)
	}

	for _, name := range []string{"run_0.png", "run_1.png", "run_2.png", "death_0.png", "death_1.png"} {
		p := filepath.Join(dst, name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("missing file: %s", p)
		}
	}
}

func TestCutSpriteSheet_ErrorRowsMismatch(t *testing.T) {
	err := CutSpriteSheet("x.png", 3, []int{2, 2}, []string{"a"}, "out")
	if err == nil {
		t.Fatal("expected error for cols/prefix mismatch")
	}
}

func TestCutSpriteSheet_ErrorZeroCols(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	src := saveTempPNG(t, img, "tiny.png")
	err := CutSpriteSheet(src, 2, []int{0, 3}, []string{"a", "b"}, t.TempDir())
	if err == nil {
		t.Fatal("expected error for zero columns")
	}
}

func TestCutSpriteSheet_ErrorZeroRows(t *testing.T) {
	err := CutSpriteSheet("x.png", 0, nil, nil, "out")
	if err == nil {
		t.Fatal("expected error for zero rows")
	}
}

func TestCutSpriteSheet_SourceNotFound(t *testing.T) {
	err := CutSpriteSheet("missing.png", 2, []int{2, 2}, []string{"a", "b"}, "out")
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

// ---------------------------------------------------------------------------
// CutSpriteRegion

func TestCutSpriteRegion(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	img.SetRGBA(8, 8, color.RGBA{100, 200, 50, 255})
	dst := filepath.Join(t.TempDir(), "region.png")

	if err := CutSpriteRegion(img, 5, 5, 10, 10, dst); err != nil {
		t.Fatalf("CutSpriteRegion: %v", err)
	}

	r, err := pngDecodeFile(dst)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Without AutoCrop: 10×10.
	if b := r.Bounds(); b.Dx() != 10 || b.Dy() != 10 {
		t.Fatalf("expected 10×10, got %dx%d", b.Dx(), b.Dy())
	}
	// Pixel (3,3) in cut = original (8,8) — should be opaque.
	_, _, _, a := r.At(3, 3).RGBA()
	if a == 0 {
		t.Fatal("expected opaque pixel")
	}
}
