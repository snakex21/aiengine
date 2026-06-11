// renderer/textures.go — Procedural texture generation and OpenGL texture management.
// Implements Perlin noise for generating brick, stone, metal, and other textures.
// Also supports loading external textures from image files.
// All textures are stored as OpenGL texture objects for use by the raycaster.

package renderer

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"

	"github.com/go-gl/gl/v4.1-core/gl"
)

// TexSize is the default resolution for procedural textures.
const TexSize = 64

// Texture2D wraps an OpenGL 2D texture with metadata and CPU-side pixel data.
type Texture2D struct {
	ID     uint32
	Width  int32
	Height int32
	Pixels []uint8 // raw RGBA pixel data for CPU sampling
}

// TextureManager owns all loaded/generated textures, keyed by name.
type TextureManager struct {
	textures map[string]*Texture2D
}

// NewTextureManager creates an empty texture manager.
func NewTextureManager() *TextureManager {
	return &TextureManager{
		textures: make(map[string]*Texture2D),
	}
}

// Get returns a texture by name, or nil if not found.
func (tm *TextureManager) Get(name string) *Texture2D {
	return tm.textures[name]
}

// CreateFromImage uploads an image.Image to an OpenGL texture and registers it.
func (tm *TextureManager) CreateFromImage(name string, img image.Image) (*Texture2D, error) {
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	var texID uint32
	gl.GenTextures(1, &texID)
	gl.BindTexture(gl.TEXTURE_2D, texID)

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)

	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.RGBA,
		int32(bounds.Dx()),
		int32(bounds.Dy()),
		0,
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		gl.Ptr(rgba.Pix),
	)

	tex := &Texture2D{
		ID:     texID,
		Width:  int32(bounds.Dx()),
		Height: int32(bounds.Dy()),
		Pixels: rgba.Pix, // keep CPU-side copy for raycaster sampling
	}
	tm.textures[name] = tex
	return tex, nil
}

// CreateFromColor creates a solid-color 1x1 texture.
func (tm *TextureManager) CreateFromColor(name string, r, g, b float64) (*Texture2D, error) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{
		R: uint8(r * 255),
		G: uint8(g * 255),
		B: uint8(b * 255),
		A: 255,
	})
	return tm.CreateFromImage(name, img)
}

// LoadFromFile loads a texture from a PNG file.
func (tm *TextureManager) LoadFromFile(name, path string) (*Texture2D, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening texture %s: %w", path, err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding texture %s: %w", path, err)
	}

	return tm.CreateFromImage(name, img)
}

// LoadSpriteSheet loads a regular PNG spritesheet and registers each cell as
// namePrefix_0, namePrefix_1, ... in row-major order.
func (tm *TextureManager) LoadSpriteSheet(namePrefix, path string, cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return fmt.Errorf("invalid spritesheet grid %dx%d", cols, rows)
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening spritesheet %s: %w", path, err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return fmt.Errorf("decoding spritesheet %s: %w", path, err)
	}

	b := img.Bounds()
	cellW := b.Dx() / cols
	cellH := b.Dy() / rows
	if cellW <= 0 || cellH <= 0 {
		return fmt.Errorf("spritesheet %s too small for %dx%d grid", path, cols, rows)
	}

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			idx := row*cols + col
			cell := image.NewRGBA(image.Rect(0, 0, cellW, cellH))
			src := image.Rect(b.Min.X+col*cellW, b.Min.Y+row*cellH, b.Min.X+(col+1)*cellW, b.Min.Y+(row+1)*cellH)
			draw.Draw(cell, cell.Bounds(), img, src.Min, draw.Src)
			prepareSpriteFrame(cell)
			cell = trimToVisibleSquare(cell, 8)
			if _, err := tm.CreateFromImage(fmt.Sprintf("%s_%d", namePrefix, idx), cell); err != nil {
				return fmt.Errorf("creating spritesheet frame %d: %w", idx, err)
			}
		}
	}

	return nil
}

// LoadWeaponHandAtlas extracts the user's Glock hand atlas into named frames:
// player_idle_0..4, player_shoot_0..3, player_reload_0..4. The source image is
// a labelled atlas, not a uniform 3x2 sheet, so regions are defined in original
// 1600x1000 atlas coordinates and scaled to the actual PNG size.
func (tm *TextureManager) LoadWeaponHandAtlas(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening weapon atlas %s: %w", path, err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return fmt.Errorf("decoding weapon atlas %s: %w", path, err)
	}

	b := img.Bounds()
	sx := float64(b.Dx()) / 1600.0
	sy := float64(b.Dy()) / 1000.0

	type atlasFrame struct {
		name       string
		x, y, w, h int
	}

	frames := []atlasFrame{
		// IDLE row, 5 columns.
		{"player_idle_0", 35, 150, 285, 230},
		{"player_idle_1", 325, 150, 285, 230},
		{"player_idle_2", 635, 150, 285, 230},
		{"player_idle_3", 945, 150, 285, 230},
		{"player_idle_4", 1260, 150, 285, 230},

		// SHOOT row. Includes muzzle flash and casing where present.
		{"player_shoot_0", 45, 435, 285, 230},
		{"player_shoot_1", 345, 405, 285, 260},
		{"player_shoot_2", 640, 420, 300, 245},
		{"player_shoot_3", 950, 435, 285, 230},

		// RELOAD row, selected clean key poses.
		{"player_reload_0", 25, 720, 235, 250},
		{"player_reload_1", 245, 720, 235, 250},
		{"player_reload_2", 480, 720, 260, 250},
		{"player_reload_3", 700, 700, 245, 290},
		{"player_reload_4", 880, 720, 300, 250},
		{"player_reload_5", 1120, 700, 285, 270},
		{"player_reload_6", 1325, 720, 245, 250},
	}

	for _, fr := range frames {
		src := image.Rect(
			b.Min.X+int(float64(fr.x)*sx),
			b.Min.Y+int(float64(fr.y)*sy),
			b.Min.X+int(float64(fr.x+fr.w)*sx),
			b.Min.Y+int(float64(fr.y+fr.h)*sy),
		)
		src = src.Intersect(b)
		if src.Empty() {
			return fmt.Errorf("empty atlas frame %s", fr.name)
		}
		cell := image.NewRGBA(image.Rect(0, 0, src.Dx(), src.Dy()))
		draw.Draw(cell, cell.Bounds(), img, src.Min, draw.Src)
		prepareSpriteFrame(cell)
		cell = trimToVisibleSquare(cell, 16)
		if _, err := tm.CreateFromImage(fr.name, cell); err != nil {
			return fmt.Errorf("creating atlas frame %s: %w", fr.name, err)
		}
	}

	return nil
}

func prepareSpriteFrame(img *image.RGBA) {
	bg := img.RGBAAt(0, 0)
	applyTopLeftColorKey(img, 48)
	if bg.A >= 250 {
		removeDarkBackground(img, 42)
	}
	removeSmallAlphaComponents(img, 160)
}

func applyTopLeftColorKey(img *image.RGBA, threshold uint8) {
	if img.Bounds().Empty() {
		return
	}
	bg := img.RGBAAt(0, 0)
	if bg.A < 250 {
		return
	}
	thr := int(threshold)
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			c := img.RGBAAt(x, y)
			if absInt(int(c.R)-int(bg.R)) <= thr && absInt(int(c.G)-int(bg.G)) <= thr && absInt(int(c.B)-int(bg.B)) <= thr {
				c.A = 0
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func removeDarkBackground(img *image.RGBA, maxRGB uint8) {
	limit := int(maxRGB)
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			if int(c.R) <= limit && int(c.G) <= limit && int(c.B) <= limit {
				c.A = 0
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func removeSmallAlphaComponents(img *image.RGBA, minArea int) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	visited := make([]bool, w*h)
	queue := make([]int, 0, w*h/8)
	component := make([]int, 0, 1024)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			start := y*w + x
			if visited[start] || img.RGBAAt(b.Min.X+x, b.Min.Y+y).A == 0 {
				continue
			}
			visited[start] = true
			queue = append(queue[:0], start)
			component = append(component[:0], start)

			for head := 0; head < len(queue); head++ {
				idx := queue[head]
				cx := idx % w
				cy := idx / w
				neighbors := [][2]int{{cx - 1, cy}, {cx + 1, cy}, {cx, cy - 1}, {cx, cy + 1}}
				for _, n := range neighbors {
					nx, ny := n[0], n[1]
					if nx < 0 || nx >= w || ny < 0 || ny >= h {
						continue
					}
					nidx := ny*w + nx
					if visited[nidx] || img.RGBAAt(b.Min.X+nx, b.Min.Y+ny).A == 0 {
						continue
					}
					visited[nidx] = true
					queue = append(queue, nidx)
					component = append(component, nidx)
				}
			}

			if len(component) < minArea {
				for _, idx := range component {
					px := b.Min.X + idx%w
					py := b.Min.Y + idx/w
					c := img.RGBAAt(px, py)
					c.A = 0
					img.SetRGBA(px, py, c)
				}
			}
		}
	}
}

func trimToVisibleSquare(img *image.RGBA, padding int) *image.RGBA {
	b := img.Bounds()
	minX, minY := b.Max.X, b.Max.Y
	maxX, maxY := b.Min.X, b.Min.Y
	found := false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.RGBAAt(x, y).A == 0 {
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
		return img
	}
	minX = maxInt(b.Min.X, minX-padding)
	minY = maxInt(b.Min.Y, minY-padding)
	maxX = minInt(b.Max.X, maxX+padding+1)
	maxY = minInt(b.Max.Y, maxY+padding+1)
	cropped := image.NewRGBA(image.Rect(0, 0, maxX-minX, maxY-minY))
	draw.Draw(cropped, cropped.Bounds(), img, image.Point{X: minX, Y: minY}, draw.Src)
	side := maxInt(cropped.Bounds().Dx(), cropped.Bounds().Dy())
	out := image.NewRGBA(image.Rect(0, 0, side, side))
	off := image.Point{X: (side - cropped.Bounds().Dx()) / 2, Y: (side - cropped.Bounds().Dy()) / 2}
	draw.Draw(out, image.Rect(off.X, off.Y, off.X+cropped.Bounds().Dx(), off.Y+cropped.Bounds().Dy()), cropped, image.Point{}, draw.Src)
	return out
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (tm *TextureManager) GenerateEnemyBillboard(name string) (*Texture2D, error) {
	img := image.NewRGBA(image.Rect(0, 0, TexSize, TexSize))
	body := color.RGBA{170, 45, 45, 255}
	dark := color.RGBA{70, 20, 20, 255}
	eye := color.RGBA{255, 220, 80, 255}
	for y := 0; y < TexSize; y++ {
		for x := 0; x < TexSize; x++ {
			dx := float64(x-32) / 16.0
			dy := float64(y-20) / 14.0
			if dx*dx+dy*dy <= 1.0 {
				img.SetRGBA(x, y, body)
			}
			bdx := float64(x-32) / 18.0
			bdy := float64(y-44) / 20.0
			if bdx*bdx+bdy*bdy <= 1.0 {
				img.SetRGBA(x, y, body)
			}
			if (x > 12 && x < 24 && y > 34 && y < 58) || (x > 40 && x < 52 && y > 34 && y < 58) {
				img.SetRGBA(x, y, dark)
			}
		}
	}
	img.SetRGBA(25, 19, eye)
	img.SetRGBA(39, 19, eye)
	return tm.CreateFromImage(name, img)
}

// GenerateBrick creates a procedural brick wall texture using a simple pattern.
func (tm *TextureManager) GenerateBrick(name string, baseColor color.RGBA, mortarColor color.RGBA) (*Texture2D, error) {
	img := image.NewRGBA(image.Rect(0, 0, TexSize, TexSize))

	brickH := int32(TexSize / 4) // 4 rows of bricks
	brickW := int32(TexSize / 2) // 2 bricks per row (with offset)

	for y := int32(0); y < TexSize; y++ {
		for x := int32(0); x < TexSize; x++ {
			row := y / brickH
			offset := int32(0)
			if row%2 == 1 {
				offset = brickW / 2
			}

			col := ((x - offset + TexSize) % TexSize) / brickW

			// Mortar lines
			isMortarX := ((x-offset+TexSize)%TexSize)%brickW < 1
			isMortarY := y%brickH < 1

			if isMortarX || isMortarY {
				img.Set(int(x), int(y), mortarColor)
			} else {
				// Add slight variation to brick color.
				noise := uint8((perlin2D(float64(x)/16.0, float64(y)/16.0)*20 - 10))
				c := color.RGBA{
					R: clampU8(int(baseColor.R) + int(noise)),
					G: clampU8(int(baseColor.G) + int(noise)),
					B: clampU8(int(baseColor.B) + int(noise)),
					A: 255,
				}
				img.Set(int(x), int(y), c)
			}
			_ = col
		}
	}

	return tm.CreateFromImage(name, img)
}

// GenerateNoise creates a Perlin-noise-based texture.
func (tm *TextureManager) GenerateNoise(name string, scale float64, seed int64, baseColor color.RGBA) (*Texture2D, error) {
	img := image.NewRGBA(image.Rect(0, 0, TexSize, TexSize))

	// Use seed to offset sampling.
	seedOff := float64(seed) * 127.1

	for y := int32(0); y < TexSize; y++ {
		for x := int32(0); x < TexSize; x++ {
			nx := (float64(x) + seedOff) / float64(TexSize) * scale
			ny := (float64(y) + seedOff) / float64(TexSize) * scale
			noise := perlin2D(nx, ny) // range [-1, 1]
			// Map to [0,1] and apply to base color.
			brightness := (noise + 1.0) / 2.0 // [0, 1]
			c := color.RGBA{
				R: clampU8(int(float64(baseColor.R) * brightness)),
				G: clampU8(int(float64(baseColor.G) * brightness)),
				B: clampU8(int(float64(baseColor.B) * brightness)),
				A: 255,
			}
			img.Set(int(x), int(y), c)
		}
	}

	return tm.CreateFromImage(name, img)
}

// GenerateTiles creates a tile floor texture with grout lines.
func (tm *TextureManager) GenerateTiles(name string, tileSize float64, groutColor, tileColor color.RGBA) (*Texture2D, error) {
	img := image.NewRGBA(image.Rect(0, 0, TexSize, TexSize))
	tilePx := int32(tileSize * float64(TexSize))

	for y := int32(0); y < TexSize; y++ {
		for x := int32(0); x < TexSize; x++ {
			isGrout := x%tilePx < 1 || y%tilePx < 1
			if isGrout {
				img.Set(int(x), int(y), groutColor)
			} else {
				// Slight variation.
				noise := uint8((perlin2D(float64(x)/8.0, float64(y)/8.0) * 10))
				c := color.RGBA{
					R: clampU8(int(tileColor.R) + int(noise)),
					G: clampU8(int(tileColor.G) + int(noise)),
					B: clampU8(int(tileColor.B) + int(noise)),
					A: 255,
				}
				img.Set(int(x), int(y), c)
			}
		}
	}

	return tm.CreateFromImage(name, img)
}

// GenerateDefaultTextures creates a set of built-in procedural textures.
func (tm *TextureManager) GenerateDefaultTextures() error {
	generators := []struct {
		name string
		fn   func() error
	}{
		{"brick", func() error {
			_, err := tm.GenerateBrick("brick",
				color.RGBA{180, 100, 60, 255},
				color.RGBA{80, 80, 80, 255})
			return err
		}},
		{"stone", func() error {
			_, err := tm.GenerateNoise("stone", 3.0, 42,
				color.RGBA{140, 140, 140, 255})
			return err
		}},
		{"metal", func() error {
			_, err := tm.GenerateNoise("metal", 4.0, 7,
				color.RGBA{160, 160, 170, 255})
			return err
		}},
		{"tiles", func() error {
			_, err := tm.GenerateTiles("tiles", 0.25,
				color.RGBA{50, 50, 50, 255},
				color.RGBA{120, 120, 120, 255})
			return err
		}},
		{"wood", func() error {
			_, err := tm.GenerateNoise("wood", 8.0, 99,
				color.RGBA{139, 90, 43, 255})
			return err
		}},
		{"concrete", func() error {
			_, err := tm.GenerateNoise("concrete", 2.0, 13,
				color.RGBA{128, 128, 128, 255})
			return err
		}},
		{"skin", func() error {
			_, err := tm.GenerateNoise("skin", 6.0, 77,
				color.RGBA{210, 148, 106, 255})
			return err
		}},
		{"cloth", func() error {
			_, err := tm.GenerateNoise("cloth", 5.0, 33,
				color.RGBA{102, 102, 102, 255})
			return err
		}},
		{"sprite_default", func() error {
			_, err := tm.CreateFromColor("sprite_default", 1.0, 0.0, 0.0)
			return err
		}},
		{"sprite_enemy", func() error {
			_, err := tm.GenerateEnemyBillboard("sprite_enemy")
			return err
		}},
		{"sprite_prop", func() error {
			_, err := tm.CreateFromColor("sprite_prop", 0.6, 0.4, 0.2)
			return err
		}},
	}

	for _, g := range generators {
		if err := g.fn(); err != nil {
			return fmt.Errorf("generating %s: %w", g.name, err)
		}
	}

	return nil
}

// Delete releases all textures.
func (tm *TextureManager) Delete() {
	for _, tex := range tm.textures {
		gl.DeleteTextures(1, &tex.ID)
	}
	tm.textures = make(map[string]*Texture2D)
}

// --- Perlin noise implementation (2D) ---

// perm is the permutation table for Perlin noise.
var perm = [512]int{}

func init() {
	// Standard Perlin permutation table.
	p := [256]int{
		151, 160, 137, 91, 90, 15, 131, 13, 201, 95, 96, 53, 194, 233, 7, 225,
		140, 36, 103, 30, 69, 142, 8, 99, 37, 240, 21, 10, 23, 190, 6, 148,
		247, 120, 234, 75, 0, 26, 197, 62, 94, 252, 219, 203, 117, 35, 11, 32,
		57, 177, 33, 88, 237, 149, 56, 87, 174, 20, 125, 136, 171, 168, 68, 175,
		74, 165, 71, 134, 139, 48, 27, 166, 77, 146, 158, 231, 83, 111, 229, 122,
		60, 211, 133, 230, 220, 105, 92, 41, 55, 46, 245, 40, 244, 102, 143, 54,
		65, 25, 63, 161, 1, 216, 80, 73, 209, 76, 132, 187, 208, 89, 18, 169,
		200, 196, 135, 130, 116, 188, 159, 86, 164, 100, 109, 198, 173, 186, 3, 64,
		52, 217, 226, 250, 124, 123, 5, 202, 38, 147, 118, 126, 255, 82, 85, 212,
		207, 206, 59, 227, 47, 16, 58, 17, 182, 189, 28, 42, 223, 183, 170, 213,
		119, 248, 152, 2, 44, 154, 163, 70, 221, 153, 101, 155, 167, 43, 172, 9,
		129, 22, 39, 253, 19, 98, 108, 110, 79, 113, 224, 232, 178, 185, 112, 104,
		218, 246, 97, 228, 251, 34, 242, 193, 238, 210, 144, 12, 191, 179, 162, 241,
		81, 51, 145, 235, 249, 14, 239, 107, 49, 192, 214, 31, 181, 199, 106, 157,
		184, 84, 204, 176, 115, 121, 50, 45, 127, 4, 150, 254, 138, 236, 205, 93,
		222, 114, 67, 29, 24, 72, 243, 141, 128, 195, 78, 66, 215, 61, 156, 180,
	}
	for i := 0; i < 512; i++ {
		perm[i] = p[i&255]
	}
}

func fade(t float64) float64 {
	return t * t * t * (t*(t*6-15) + 10)
}

func lerp(a, b, t float64) float64 {
	return a + t*(b-a)
}

func grad(hash int, x, y float64) float64 {
	h := hash & 3
	u := 0.0
	v := 0.0
	if h == 0 {
		u, v = x, y
	} else if h == 1 {
		u, v = y, -x
	} else if h == 2 {
		u, v = -x, -y
	} else {
		u, v = -y, x
	}
	return u + v
}

// perlin2D returns Perlin noise value for 2D coordinates in range [-1, 1].
func perlin2D(x, y float64) float64 {
	xi := int(math.Floor(x)) & 255
	yi := int(math.Floor(y)) & 255

	x -= math.Floor(x)
	y -= math.Floor(y)

	u := fade(x)
	v := fade(y)

	aa := perm[perm[xi]+yi]
	ab := perm[perm[xi]+yi+1]
	ba := perm[perm[xi+1]+yi]
	bb := perm[perm[xi+1]+yi+1]

	return lerp(
		lerp(grad(aa, x, y), grad(ba, x-1, y), u),
		lerp(grad(ab, x, y-1), grad(bb, x-1, y-1), u),
		v,
	)
}

// clampU8 clamps an int to [0, 255].
func clampU8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
