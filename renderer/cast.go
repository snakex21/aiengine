// renderer/cast.go — parallel column caster for the grid map with per-cell
// floor and ceiling heights, plus CPU sprites clipped against the result.
//
// Each screen column is cast independently (columns are spread over all CPU
// cores). A column keeps an open window of rows [top, bot) that nothing
// nearer has covered yet. Walking the grid front to back, at every cell
// boundary it:
//
//  1. fills the current cell's ceiling (rows above the ceiling's far edge)
//     and floor (rows below the floor's far edge) and shrinks the window;
//  2. if the next cell is solid, fills the rest of the window with the wall
//     and stops;
//  3. otherwise draws an upper step face where the next ceiling is lower
//     and a lower step face where the next floor is higher, shrinking the
//     window again, and stops once it is closed.
//
// After each boundary the window is logged (distance, top, bot); sprites use
// the window of the last boundary nearer than themselves, so they are hidden
// behind ledges, low ceilings and nearer floors exactly like walls are.

package renderer

import (
	"math"
	"runtime"
	"sort"
	"sync"
)

// HorPlusFOV converts a field of view defined for a 16:9 screen into the
// horizontal FOV for the given aspect ratio, keeping the vertical view the
// same ("Hor+"): wider screens see more to the sides instead of the image
// being squashed, narrower ones see less.
func HorPlusFOV(fov16x9, aspect float64) float64 {
	if aspect <= 0 {
		return fov16x9
	}
	return 2 * math.Atan(math.Tan(fov16x9/2)*aspect/(16.0/9.0))
}

// Focal is the projection distance in pixels for a screen width and half
// horizontal FOV. Both walls (vertically) and the horizontal mapping use it,
// so pixels are square at every aspect ratio and FOV.
func Focal(screenW, halfFOV float64) float64 {
	return screenW / 2 / math.Tan(halfFOV)
}

// fov is the horizontal FOV used for this frame's aspect ratio.
func (rc *Raycaster) fov() float64 {
	if rc.screenH == 0 {
		return rc.Player.FOV
	}
	return HorPlusFOV(rc.Player.FOV, float64(rc.screenW)/float64(rc.screenH))
}

// UIScale is the integer pixel scale for HUD/menu text at a render height
// (2 at 720p, 3 at 1080p, 4 at 1440p, 6 at 2160p), so UI stays readable.
func UIScale(sh int) int {
	return max(1, (sh+180)/360)
}

// parallel runs fn over [0,n) split into contiguous chunks, one per CPU.
func parallel(n int, fn func(lo, hi int)) {
	workers := runtime.GOMAXPROCS(0)
	if workers > n {
		workers = n
	}
	if workers <= 1 {
		fn(0, n)
		return
	}
	var wg sync.WaitGroup
	chunk := (n + workers - 1) / workers
	for lo := 0; lo < n; lo += chunk {
		hi := min(lo+chunk, n)
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			fn(lo, hi)
		}(lo, hi)
	}
	wg.Wait()
}

const (
	maxCastSteps = 256  // cells a ray may cross
	nearPlane    = 0.02 // smallest boundary distance (avoids /0 at a cell edge)
)

// Distance shading in 8.8 fixed point: 256 up to distance 1, then 1/sqrt(d).
// Indexed by distance*shadeRes (clamped to the last entry).
const shadeRes = 16

var shadeLUT = func() [64 * shadeRes]int32 {
	var t [64 * shadeRes]int32
	for i := range t {
		d := float64(i) / shadeRes
		s := 1.0
		if d > 1 {
			s = 1 / math.Sqrt(d)
		}
		t[i] = int32(s*256 + 0.5)
	}
	return t
}()

func distShade(d float64) int32 {
	i := int(d * shadeRes)
	if i >= len(shadeLUT) {
		i = len(shadeLUT) - 1
	}
	if i < 0 {
		i = 0
	}
	return shadeLUT[i]
}

// clipEntry is the column's open window after a cell boundary at dist.
type clipEntry struct {
	dist     float64
	top, bot int32
}

// frame holds per-frame constants shared by all column workers.
type frame struct {
	sw, sh            int
	fsw, fsh          float64
	halfFOV           float64
	focal             float64 // projection distance in pixels (same for x and y)
	dirX, dirY        float64 // view direction
	planeX, planeY    float64 // camera plane (half width at distance 1)
	horizon           float64 // screen row of the horizon (float)
	eye               float64 // absolute camera height
	ceil, flr         [4]uint8
	floorTex, ceilTex *Texture2D
	wallTex           [5]*Texture2D // indexed by MapCell
	rowK              []float64     // per row: focal / |row centre - horizon|
	px, py            float64

	// Lighting (light.go): off for maps without any, which then render
	// exactly as before (flat colours unshaded).
	lit, fogOn bool
	fogC       [3]int32
	cellLight  []int32 // per cell, 8.8 fixed point (nil = all 256)
	lut        [len(shadeLUT)]lightEntry
}

// rowAt is the (fractional) screen row of world height h at distance d.
func (f *frame) rowAt(h, d float64) float64 { return f.horizon + (f.eye-h)*f.focal/d }

// rowOf is the first row whose centre lies at or below yf, clamped to the
// screen: spans [rowOf(a), rowOf(b)) tile the screen without gaps.
func (f *frame) rowOf(yf float64) int {
	if yf < 0 {
		return 0
	}
	if yf > f.fsh {
		return f.sh
	}
	return int(math.Ceil(yf - 0.5))
}

// renderToBuffer fills rc.pixels with ceilings, walls, floors and sprites,
// then applies the colour effects.
func (rc *Raycaster) renderToBuffer(sprites []*Sprite) {
	f := &frame{sw: int(rc.screenW), sh: int(rc.screenH)}
	f.fsw, f.fsh = float64(f.sw), float64(f.sh)
	f.halfFOV = rc.fov() / 2
	f.focal = Focal(f.fsw, f.halfFOV)
	f.dirX, f.dirY = math.Cos(rc.Player.Angle), math.Sin(rc.Player.Angle)
	t := math.Tan(f.halfFOV)
	f.planeX, f.planeY = -f.dirY*t, f.dirX*t
	f.horizon = f.fsh/2 + rc.Player.Pitch*f.fsh
	f.eye = rc.Player.EyeZ()
	f.ceil, f.flr = colorToRGBA(rc.CeilColor), colorToRGBA(rc.FloorColor)
	f.px, f.py = rc.Player.X, rc.Player.Y
	if rc.FloorTexture != "" {
		f.floorTex = usableTex(rc.TexManager.Get(rc.FloorTexture))
	}
	if rc.CeilingTexture != "" {
		f.ceilTex = usableTex(rc.TexManager.Get(rc.CeilingTexture))
	}
	for cell, name := range map[MapCell]string{CellWall1: "brick", CellWall2: "stone", CellWall3: "metal", CellDoor: "wood"} {
		f.wallTex[cell] = usableTex(rc.TexManager.Get(name))
	}
	if len(rc.rowK) != f.sh {
		rc.rowK = make([]float64, f.sh)
	}
	f.rowK = rc.rowK
	for y := range f.rowK {
		dy := math.Abs(float64(y) + 0.5 - f.horizon)
		f.rowK[y] = f.focal / math.Max(dy, 1e-3)
	}
	rc.prepareLighting(f)
	if len(rc.zBuffer) != f.sw {
		rc.zBuffer = make([]float64, f.sw)
	}
	if len(rc.clips) != f.sw {
		rc.clips = make([][]clipEntry, f.sw)
	}

	parallel(f.sw, func(lo, hi int) {
		for x := lo; x < hi; x++ {
			rc.castColumn(f, x)
		}
	})
	rc.drawSpritesCPU(f, sprites)

	rc.applyScreenEffects(f.sw, f.sh)
}

// usableTex returns t if it can be sampled with power-of-two wrapping.
func usableTex(t *Texture2D) *Texture2D {
	if t == nil || len(t.Pixels) == 0 || t.Width <= 0 || t.Height <= 0 ||
		t.Width&(t.Width-1) != 0 || t.Height&(t.Height-1) != 0 {
		return nil
	}
	return t
}

func (rc *Raycaster) castColumn(f *frame, x int) {
	m := rc.Map
	cameraX := 2.0*float64(x)/f.fsw - 1.0
	// Rays through a flat camera plane (not evenly spaced angles), so that
	// the DDA's distance along the un-normalised ray is the perpendicular
	// distance to the plane: straight walls stay straight (no fisheye bow).
	rdx := f.dirX + f.planeX*cameraX
	rdy := f.dirY + f.planeY*cameraX

	mapX, mapY := int(math.Floor(f.px)), int(math.Floor(f.py))
	deltaDistX, deltaDistY := math.Abs(1/rdx), math.Abs(1/rdy)
	if math.IsInf(deltaDistX, 0) {
		deltaDistX = 1e30
	}
	if math.IsInf(deltaDistY, 0) {
		deltaDistY = 1e30
	}
	var sideDistX, sideDistY float64
	var stepX, stepY int
	if rdx < 0 {
		stepX, sideDistX = -1, (f.px-float64(mapX))*deltaDistX
	} else {
		stepX, sideDistX = 1, (float64(mapX)+1-f.px)*deltaDistX
	}
	if rdy < 0 {
		stepY, sideDistY = -1, (f.py-float64(mapY))*deltaDistY
	} else {
		stepY, sideDistY = 1, (float64(mapY)+1-f.py)*deltaDistY
	}

	clips := rc.clips[x][:0]
	defer func() { rc.clips[x] = clips }()
	rc.zBuffer[x] = 0
	top, bot := 0, f.sh
	curF, curC := m.FloorAt(mapX, mapY), m.CeilAt(mapX, mapY)
	curL := f.light(m, mapX, mapY)

	for i := 0; i < maxCastSteps; i++ {
		var d float64
		side := 0
		if sideDistX < sideDistY {
			d = sideDistX
			sideDistX += deltaDistX
			mapX += stepX
		} else {
			d = sideDistY
			sideDistY += deltaDistY
			mapY += stepY
			side = 1
		}
		d = math.Max(d, nearPlane)

		// The current cell's ceiling and floor up to this boundary.
		if e := min(f.rowOf(f.rowAt(curC, d)), bot); e > top {
			rc.drawPlane(f, x, top, e, curC-f.eye, curL, f.ceilTex, f.ceil, rdx, rdy, true)
			top = e
		}
		if s := max(f.rowOf(f.rowAt(curF, d)), top); s < bot {
			rc.drawPlane(f, x, s, bot, f.eye-curF, curL, f.floorTex, f.flr, rdx, rdy, false)
			bot = s
		}

		var wallX float64
		if side == 0 {
			wallX = f.py + d*rdy
		} else {
			wallX = f.px + d*rdx
		}
		wallX -= math.Floor(wallX)

		// Walls and step faces take the light of the cell in front of them.
		cell := m.GetCell(mapX, mapY)
		if cell != CellEmpty {
			if top < bot {
				rc.drawWall(f, x, top, bot, d, curL, cell, side, wallX)
			}
			rc.zBuffer[x] = d
			clips = append(clips, clipEntry{d, int32(top), int32(top)})
			return
		}

		nF, nC := m.FloorAt(mapX, mapY), m.CeilAt(mapX, mapY)
		if nC < curC { // upper step face (lower ceiling ahead)
			if e := min(f.rowOf(f.rowAt(nC, d)), bot); e > top {
				rc.drawWall(f, x, top, e, d, curL, m.SideAt(mapX, mapY), side, wallX)
				top = e
			}
		}
		if nF > curF { // lower step face (higher floor ahead)
			if s := max(f.rowOf(f.rowAt(nF, d)), top); s < bot {
				rc.drawWall(f, x, s, bot, d, curL, m.SideAt(mapX, mapY), side, wallX)
				bot = s
			}
		}
		clips = append(clips, clipEntry{d, int32(top), int32(bot)})
		if top >= bot {
			rc.zBuffer[x] = d
			return
		}
		curF, curC, curL = nF, nC, f.light(m, mapX, mapY)
	}
	// Ran out of steps: the last cell's planes continue to the horizon.
	h := min(max(f.rowOf(f.horizon), top), bot)
	rc.drawPlane(f, x, top, h, curC-f.eye, curL, f.ceilTex, f.ceil, rdx, rdy, true)
	rc.drawPlane(f, x, h, bot, f.eye-curF, curL, f.floorTex, f.flr, rdx, rdy, false)
}

// planeK darkens floors and ceilings a little against walls (0.85).
const planeK = 218

// drawPlane draws rows [y0, y1) of column x with a horizontal plane whose
// height differs from the eye by dh (positive when the plane faces the
// camera), lit by cell light L. Rows on the wrong side of the horizon, or
// without a texture, get the flat colour (shaded only on lit maps).
func (rc *Raycaster) drawPlane(f *frame, x, y0, y1 int, dh float64, L int32, tex *Texture2D, flat [4]uint8, rdx, rdy float64, above bool) {
	sw := f.sw
	pix := rc.pixels
	if tex == nil || dh <= 0 {
		for y := y0; y < y1; y++ {
			i := (y*sw + x) * 4
			if f.lit && dh > 0 && (float64(y)+0.5 < f.horizon) == above {
				s, fz := f.shadeFor(L, lutIndex(dh*f.rowK[y]), planeK)
				f.put(pix[i:i+4:i+4], int32(flat[0]), int32(flat[1]), int32(flat[2]), s, fz)
				continue
			}
			pix[i], pix[i+1], pix[i+2], pix[i+3] = flat[0], flat[1], flat[2], 255
		}
		return
	}
	tw, th := int(tex.Width), int(tex.Height)
	fw, fh := float64(tw), float64(th)
	for y := y0; y < y1; y++ {
		i := (y*sw + x) * 4
		if (float64(y)+0.5 < f.horizon) != above {
			pix[i], pix[i+1], pix[i+2], pix[i+3] = flat[0], flat[1], flat[2], 255
			continue
		}
		d := dh * f.rowK[y]
		wx, wy := f.px+rdx*d, f.py+rdy*d
		tx := int(wx*fw) & (tw - 1) // one repeat per cell (power-of-two wrap)
		ty := int(wy*fh) & (th - 1)
		src := (ty*tw + tx) * 4
		tp := tex.Pixels[src : src+3 : src+3]
		if f.lit { // f.put, inlined by hand (hot loop)
			e := &f.lut[lutIndex(d)]
			s := (L*e.dist>>8)*planeK>>8 + e.flash
			r, g, b := int32(tp[0])*s>>8, int32(tp[1])*s>>8, int32(tp[2])*s>>8
			if e.fog > 0 {
				r = (r*e.keep + e.fogR) >> 8
				g = (g*e.keep + e.fogG) >> 8
				b = (b*e.keep + e.fogB) >> 8
			}
			dst := pix[i : i+4 : i+4]
			dst[0], dst[1], dst[2], dst[3] = uint8(min(r, 255)), uint8(min(g, 255)), uint8(min(b, 255)), 255
			continue
		}
		s := distShade(d) * planeK >> 8
		pix[i] = uint8(int32(tp[0]) * s >> 8)
		pix[i+1] = uint8(int32(tp[1]) * s >> 8)
		pix[i+2] = uint8(int32(tp[2]) * s >> 8)
		pix[i+3] = 255
	}
}

// drawWall fills rows [y0, y1) of column x with a wall face at distance d
// lit by cell light L. Textures are anchored to world height (one repeat per
// unit, top of a classic 0..1 wall at v=0), so faces of different heights
// line up.
func (rc *Raycaster) drawWall(f *frame, x, y0, y1 int, d float64, L int32, cell MapCell, side int, wallX float64) {
	sw := f.sw
	pix := rc.pixels
	k := int32(256)
	if side == 1 {
		k = 179 // 0.7: cheap directional light
	}
	s, fz := distShade(d)*k>>8, int32(0)
	if f.lit {
		s, fz = f.shadeFor(L, lutIndex(d), k)
	}
	var tex *Texture2D
	if int(cell) >= 0 && int(cell) < len(f.wallTex) {
		tex = f.wallTex[cell]
	}
	if tex == nil {
		wc, ok := wallColors[cell]
		if !ok {
			wc = wallColors[CellWall2]
		}
		stripe := wallX * 255 * 0.3
		fs := float64(s) / 256
		var px [4]uint8
		f.put(px[:], int32(clampByte(float64(wc.R)*fs*0.7+stripe)), int32(clampByte(float64(wc.G)*fs*0.7+stripe)),
			int32(clampByte(float64(wc.B)*fs*0.7+stripe)), 256, fz)
		for y := y0; y < y1; y++ {
			i := (y*sw + x) * 4
			copy(pix[i:i+4], px[:])
		}
		return
	}
	texW, texH := int(tex.Width), int(tex.Height)
	texX := int(wallX*float64(texW)) & (texW - 1)
	step := float64(texH) * d / f.focal
	hTop := f.eye - (float64(y0)+0.5-f.horizon)*d/f.focal
	texPos := (1-hTop)*float64(texH) + float64(texH)*4096 // keep positive
	for y := y0; y < y1; y++ {
		texY := int(texPos) & (texH - 1)
		texPos += step
		src := (texY*texW + texX) * 4
		i := (y*sw + x) * 4
		if f.lit {
			r, g, b := int32(tex.Pixels[src])*s>>8, int32(tex.Pixels[src+1])*s>>8, int32(tex.Pixels[src+2])*s>>8
			if fz > 0 {
				r = (r*(256-fz) + f.fogC[0]*fz) >> 8
				g = (g*(256-fz) + f.fogC[1]*fz) >> 8
				b = (b*(256-fz) + f.fogC[2]*fz) >> 8
			}
			dst := pix[i : i+4 : i+4]
			dst[0], dst[1], dst[2], dst[3] = uint8(min(r, 255)), uint8(min(g, 255)), uint8(min(b, 255)), 255
			continue
		}
		pix[i] = uint8(int32(tex.Pixels[src]) * s >> 8)
		pix[i+1] = uint8(int32(tex.Pixels[src+1]) * s >> 8)
		pix[i+2] = uint8(int32(tex.Pixels[src+2]) * s >> 8)
		pix[i+3] = 255
	}
}

// --- Sprites ---

type spriteItem struct {
	tex      *Texture2D
	alpha    int32 // 0..256
	depth    float64
	x0, size float64 // left edge and size on screen (square)
	yTop     float64
	shade    int32 // light of the sprite's cell and distance (8.8)
	fog      int32 // fog factor (8.8)
}

// spriteTexture picks the texture for a sprite seen from the camera: one of
// its rotation frames (when it has them) by the viewing angle, otherwise
// TexName.
func (rc *Raycaster) spriteTexture(s *Sprite) *Texture2D {
	name := s.TexName
	if n := len(s.Rotations); n > 0 {
		name = s.Rotations[RotationIndex(s.Angle, math.Atan2(rc.Player.Y-s.Y, rc.Player.X-s.X), n)]
	}
	return rc.TexManager.Get(name)
}

// RotationIndex selects one of n rotation frames for a thing facing angle
// facing, seen from direction toViewer (the angle from the thing to the
// camera). Frame 0 is the front (thing faces the viewer), frames go
// counter-clockwise around the thing.
func RotationIndex(facing, toViewer float64, n int) int {
	a := toViewer - facing
	a = math.Mod(a, 2*math.Pi)
	if a < 0 {
		a += 2 * math.Pi
	}
	seg := 2 * math.Pi / float64(n)
	return int((a+seg/2)/seg) % n
}

// drawSpritesCPU draws the sprites (far to near) into the pixel buffer,
// each column clipped to the window the walls left open in front of it.
func (rc *Raycaster) drawSpritesCPU(f *frame, sprites []*Sprite) {
	items := rc.spriteItems[:0]
	for _, s := range sprites {
		if s == nil || !s.Active {
			continue
		}
		tex := rc.spriteTexture(s)
		if tex == nil || tex.Width <= 0 || tex.Height <= 0 || len(tex.Pixels) < int(tex.Width*tex.Height*4) {
			continue
		}
		dx, dy := s.X-f.px, s.Y-f.py
		depth := dx*f.dirX + dy*f.dirY
		if depth < 0.1 {
			continue
		}
		lateral := -dx*f.dirY + dy*f.dirX
		size := s.Scale * f.focal / depth
		sx := f.fsw/2 + f.focal*lateral/depth
		x0 := sx - size/2
		if x0+size < 0 || x0 >= f.fsw {
			continue
		}
		// World top of the sprite: it stands on its cell's floor; Z > 0
		// lowers it (legacy convention), Scale is its height.
		topH := rc.Map.FloorAtPoint(s.X, s.Y) + EyeHeight - s.Z + s.Scale/2
		yTop := f.rowAt(topH, depth)
		if yTop >= f.fsh || yTop+size < 0 {
			continue
		}
		alpha := int32(s.Alpha*256 + 0.5)
		if alpha <= 0 {
			continue
		}
		shade, fog := distShade(depth), int32(0)
		if f.lit { // sprites are lit by the cell they stand in
			cx, cy := cellAtPoint(s.X, s.Y)
			shade, fog = f.shadeFor(f.light(rc.Map, cx, cy), lutIndex(depth), 256)
		}
		items = append(items, spriteItem{tex: tex, alpha: min(alpha, 256), depth: depth, x0: x0, size: size,
			yTop: yTop, shade: shade, fog: fog})
	}
	rc.spriteItems = items
	if len(items) == 0 {
		return
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].depth > items[j].depth })
	parallel(f.sw, func(lo, hi int) {
		for i := range items {
			rc.drawSpriteColumns(f, &items[i], lo, hi)
		}
	})
}

func (rc *Raycaster) drawSpriteColumns(f *frame, it *spriteItem, lo, hi int) {
	xa := max(lo, int(math.Ceil(it.x0-0.5)))
	xb := min(hi, int(math.Ceil(it.x0+it.size-0.5)))
	if xa >= xb {
		return
	}
	tw, th := int(it.tex.Width), int(it.tex.Height)
	ya := f.rowOf(it.yTop)
	yb := f.rowOf(it.yTop + it.size)
	pix := rc.pixels
	tp := it.tex.Pixels
	// Texture rows step in 16.16 fixed point down each column.
	vStep := int64(float64(th) / it.size * 65536)
	s, keep := it.shade, 256-it.fog
	fogR, fogG, fogB := f.fogC[0]*it.fog, f.fogC[1]*it.fog, f.fogC[2]*it.fog
	for x := xa; x < xb; x++ {
		// Window left open by everything nearer than the sprite.
		top, bot := int32(0), int32(f.sh)
		for _, c := range rc.clips[x] {
			if c.dist >= it.depth {
				break
			}
			top, bot = c.top, c.bot
		}
		y0, y1 := max(ya, int(top)), min(yb, int(bot))
		if y0 >= y1 {
			continue
		}
		tx := int((float64(x) + 0.5 - it.x0) / it.size * float64(tw))
		tx = min(max(tx, 0), tw-1)
		v := int64((float64(y0) + 0.5 - it.yTop) / it.size * float64(th) * 65536)
		for y := y0; y < y1; y, v = y+1, v+vStep {
			ty := min(max(int(v>>16), 0), th-1)
			src := (ty*tw + tx) * 4
			a := int32(tp[src+3]) * it.alpha >> 8
			if a < 1 {
				continue
			}
			r := int32(tp[src]) * s >> 8
			g := int32(tp[src+1]) * s >> 8
			b := int32(tp[src+2]) * s >> 8
			if it.fog > 0 {
				r, g, b = (r*keep+fogR)>>8, (g*keep+fogG)>>8, (b*keep+fogB)>>8
			}
			r, g, b = min(r, 255), min(g, 255), min(b, 255)
			i := (y*f.sw + x) * 4
			dst := pix[i : i+3 : i+3]
			if a >= 255 {
				dst[0], dst[1], dst[2] = uint8(r), uint8(g), uint8(b)
				continue
			}
			dst[0] = uint8((r*a + int32(dst[0])*(255-a)) / 255)
			dst[1] = uint8((g*a + int32(dst[1])*(255-a)) / 255)
			dst[2] = uint8((b*a + int32(dst[2])*(255-a)) / 255)
		}
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
