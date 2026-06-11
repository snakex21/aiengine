// tools/sprites.go — Sprite sheet cutting tools for AI agents.
// Pure cut – no colour manipulation, no background removal.
// The sprite sheet must already have a transparent background.
// AutoCrop removes empty margins from cut frames.

package tools

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

// EnsureDir creates a directory and all parents if they do not exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// AutoCrop scans the image from all four edges and finds the tightest
// bounding box that contains non‑empty pixels.  A pixel is considered
// "content" when its alpha > 0 AND R+G+B > 30 (excludes nearly‑black
// garbage and fully transparent areas).  Returns the cropped image
// with zero margin.  If the image is entirely empty a 1×1 transparent
// image is returned.
func AutoCrop(img image.Image) image.Image {
	b := img.Bounds()
	minX, minY := b.Max.X, b.Max.Y
	maxX, maxY := b.Min.X, b.Min.Y
	found := false

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bv, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			if int(r>>8)+int(g>>8)+int(bv>>8) <= 30 {
				continue
			}
			found = true
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}

	if !found {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}

	w := maxX - minX + 1
	h := maxY - minY + 1
	cropped := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(cropped, cropped.Bounds(), img, image.Point{X: minX, Y: minY}, draw.Src)
	return cropped
}

// CutSprite extracts a single (x, y, w, h) region from a PNG sprite sheet
// and saves it as a PNG.  The existing alpha channel is preserved as‑is.
// The destination directory is created if it does not exist.
func CutSprite(src string, x, y, w, h int, dst string) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("invalid frame dimensions: %dx%d", w, h)
	}

	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return fmt.Errorf("decoding %s: %w", src, err)
	}

	b := img.Bounds()
	if x < b.Min.X || y < b.Min.Y || x+w > b.Max.X || y+h > b.Max.Y {
		return fmt.Errorf("frame (%d,%d %dx%d) exceeds image bounds %s", x, y, w, h, b)
	}

	cut := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(cut, cut.Bounds(), img, image.Point{X: x, Y: y}, draw.Src)

	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("creating directory for %s: %w", dst, err)
	}

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	defer out.Close()

	if err := png.Encode(out, cut); err != nil {
		return fmt.Errorf("encoding %s: %w", dst, err)
	}

	return nil
}

// CutSpriteSheet extracts all frames from a sprite sheet using a row‑based
// layout.  Each row can have a different number of columns (cols[i]).
//
// Parameters:
//   - src: path to the sprite sheet PNG.
//   - rows: total number of rows.
//   - cols: number of columns per row; len(cols) must equal rows.
//   - prefix: name prefix per row (e.g. "idle", "walk"); len(prefix) must equal rows.
//   - dst: output directory; files are saved as dst/prefix[row]_col.png.
func CutSpriteSheet(src string, rows int, cols []int, prefix []string, dst string) error {
	if rows <= 0 {
		return fmt.Errorf("rows must be > 0, got %d", rows)
	}
	if len(cols) != rows {
		return fmt.Errorf("cols length %d does not match rows %d", len(cols), rows)
	}
	if len(prefix) != rows {
		return fmt.Errorf("prefix length %d does not match rows %d", len(prefix), rows)
	}

	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return fmt.Errorf("decoding %s: %w", src, err)
	}

	b := img.Bounds()
	imgW := b.Dx()
	imgH := b.Dy()
	cellH := imgH / rows
	if cellH <= 0 {
		return fmt.Errorf("image height %d too small for %d rows", imgH, rows)
	}

	for row := 0; row < rows; row++ {
		colCount := cols[row]
		if colCount <= 0 {
			return fmt.Errorf("cols[%d] must be > 0, got %d", row, colCount)
		}
		cellW := imgW / colCount
		if cellW <= 0 {
			return fmt.Errorf("image width %d too small for %d columns", imgW, colCount)
		}

		pref := prefix[row]
		for col := 0; col < colCount; col++ {
			x := b.Min.X + col*cellW
			y := b.Min.Y + row*cellH
			outPath := filepath.Join(dst, fmt.Sprintf("%s_%d.png", pref, col))

			if err := CutSpriteRegion(img, x, y, cellW, cellH, outPath); err != nil {
				return fmt.Errorf("row %d col %d: %w", row, col, err)
			}
		}
	}

	return nil
}

// CutSpriteRegion is like CutSprite but works on an already‑decoded image.
func CutSpriteRegion(img image.Image, x, y, w, h int, dst string) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("invalid cell size %dx%d", w, h)
	}

	cut := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(cut, cut.Bounds(), img, image.Point{X: x, Y: y}, draw.Src)

	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("creating directory for %s: %w", dst, err)
	}

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	defer out.Close()

	if err := png.Encode(out, cut); err != nil {
		return fmt.Errorf("encoding %s: %w", dst, err)
	}

	return nil
}

// compile‑time check that unused imports are intentional.
var _ = color.RGBA{}
