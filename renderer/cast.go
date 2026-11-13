// renderer/cast.go — parallel column caster: ceiling, walls and floor
// (flat colours or textures) for each screen column, spread over all CPU
// cores. Columns are independent, so each worker owns a contiguous range of
// columns of the pixel buffer and z-buffer.

package renderer

import (
	"math"
	"runtime"
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

// frame holds per-frame constants shared by all column workers.
type frame struct {
	sw, sh            int
	fsw, fsh          float64
	halfFOV           float64
	horizon           float64 // screen row of the horizon (float)
	pitchShift        float64
	camHeight         float64
	ceil, flr         [4]uint8
	floorTex, ceilTex *Texture2D
	wallTex           [5]*Texture2D // indexed by MapCell
	rowDist, rowShade []float64     // per screen row: floor/ceiling distance and shade
	px, py, angle     float64
}

// renderToBuffer fills rc.pixels with ceiling, walls and floor, then applies
// the colour effects.
func (rc *Raycaster) renderToBuffer(sprites []*Sprite) {
	f := &frame{sw: int(rc.screenW), sh: int(rc.screenH)}
	f.fsw, f.fsh = float64(f.sw), float64(f.sh)
	f.halfFOV = rc.fov() / 2
	f.pitchShift = rc.Player.Pitch * f.fsh
	f.horizon = f.fsh/2 + f.pitchShift
	f.camHeight = rc.Player.CameraHeight()
	f.ceil, f.flr = colorToRGBA(rc.CeilColor), colorToRGBA(rc.FloorColor)
	f.px, f.py, f.angle = rc.Player.X, rc.Player.Y, rc.Player.Angle
	if rc.FloorTexture != "" {
		f.floorTex = usableTex(rc.TexManager.Get(rc.FloorTexture))
	}
	if rc.CeilingTexture != "" {
		f.ceilTex = usableTex(rc.TexManager.Get(rc.CeilingTexture))
	}
	for cell, name := range map[MapCell]string{CellWall1: "brick", CellWall2: "stone", CellWall3: "metal", CellDoor: "wood"} {
		f.wallTex[cell] = usableTex(rc.TexManager.Get(name))
	}
	// Floor/ceiling distance and shade depend only on the row: compute once.
	if f.floorTex != nil || f.ceilTex != nil {
		f.rowDist = make([]float64, f.sh)
		f.rowShade = make([]float64, f.sh)
		ceilK := f.fsh * (0.5 - f.camHeight)
		floorK := f.fsh * (0.5 + f.camHeight)
		for y := 0; y < f.sh; y++ {
			var d float64
			if fy := float64(y); fy < f.horizon {
				d = ceilK / (f.horizon - fy)
			} else {
				d = floorK / (fy + 0.5 - f.horizon)
			}
			f.rowDist[y] = d
			f.rowShade[y] = 0.85
			if d > 1 {
				f.rowShade[y] = 0.85 / math.Sqrt(d)
			}
		}
	}
	if len(rc.zBuffer) != f.sw {
		rc.zBuffer = make([]float64, f.sw)
	}

	parallel(f.sw, func(lo, hi int) {
		for x := lo; x < hi; x++ {
			rc.castColumn(f, x)
		}
	})

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
	sw, sh := f.sw, f.sh
	cameraX := 2.0*float64(x)/f.fsw - 1.0
	rayAngle := f.angle + cameraX*f.halfFOV
	rayDirX, rayDirY := math.Cos(rayAngle), math.Sin(rayAngle)

	mapX, mapY := int(math.Floor(f.px)), int(math.Floor(f.py))
	deltaDistX, deltaDistY := math.Abs(1/rayDirX), math.Abs(1/rayDirY)
	if math.IsInf(deltaDistX, 0) {
		deltaDistX = 1e30
	}
	if math.IsInf(deltaDistY, 0) {
		deltaDistY = 1e30
	}
	var sideDistX, sideDistY float64
	var stepX, stepY int
	if rayDirX < 0 {
		stepX, sideDistX = -1, (f.px-float64(mapX))*deltaDistX
	} else {
		stepX, sideDistX = 1, (float64(mapX)+1-f.px)*deltaDistX
	}
	if rayDirY < 0 {
		stepY, sideDistY = -1, (f.py-float64(mapY))*deltaDistY
	} else {
		stepY, sideDistY = 1, (float64(mapY)+1-f.py)*deltaDistY
	}

	hit, side := false, 0
	var wallType MapCell
	for step := 0; step < 96; step++ {
		if sideDistX < sideDistY {
			sideDistX += deltaDistX
			mapX += stepX
			side = 0
		} else {
			sideDistY += deltaDistY
			mapY += stepY
			side = 1
		}
		if cell := rc.Map.GetCell(mapX, mapY); cell != CellEmpty {
			hit, wallType = true, cell
			break
		}
	}

	// Wall span (unclamped float rows) — the floor/ceiling fill the rest.
	wallTop, wallBottom := f.horizon, f.horizon
	var perpDist, wallHeight, vOffset float64
	if hit {
		if side == 0 {
			perpDist = sideDistX - deltaDistX
		} else {
			perpDist = sideDistY - deltaDistY
		}
		perpDist = math.Max(perpDist, 0.001)
		wallHeight = f.fsh / perpDist
		vOffset = f.camHeight*wallHeight + f.pitchShift
		wallTop = (f.fsh-wallHeight)/2 + vOffset
		wallBottom = wallTop + wallHeight
		rc.zBuffer[x] = perpDist
	} else {
		rc.zBuffer[x] = 0
	}

	drawStart := clampInt(int(wallTop), 0, sh)
	drawEnd := clampInt(int(wallTop)+int(wallHeight), 0, sh)
	if !hit {
		drawStart = clampInt(int(f.horizon), 0, sh)
		drawEnd = drawStart
	}

	// Ceiling.
	for y := 0; y < drawStart; y++ {
		idx := (y*sw + x) * 4
		if f.ceilTex == nil || float64(y) >= f.horizon {
			copy(rc.pixels[idx:idx+4], f.ceil[:])
			continue
		}
		d := f.rowDist[y]
		rc.planePixel(idx, f.ceilTex, f.px+rayDirX*d, f.py+rayDirY*d, f.rowShade[y])
	}
	// Floor.
	for y := drawEnd; y < sh; y++ {
		idx := (y*sw + x) * 4
		if f.floorTex == nil || float64(y) <= f.horizon {
			copy(rc.pixels[idx:idx+4], f.flr[:])
			continue
		}
		d := f.rowDist[y]
		rc.planePixel(idx, f.floorTex, f.px+rayDirX*d, f.py+rayDirY*d, f.rowShade[y])
	}
	_ = wallBottom
	if !hit {
		return
	}

	// Wall.
	var wallX float64
	if side == 0 {
		wallX = f.py + perpDist*rayDirY
	} else {
		wallX = f.px + perpDist*rayDirX
	}
	wallX -= math.Floor(wallX)
	shade := 1.0
	if perpDist > 1 {
		shade = 1 / math.Sqrt(perpDist)
	}
	if side == 1 {
		shade *= 0.7
	}
	var tex *Texture2D
	if int(wallType) < len(f.wallTex) {
		tex = f.wallTex[wallType]
	}
	if tex != nil {
		texW, texH := int(tex.Width), int(tex.Height)
		texX := int(wallX*float64(texW)) & (texW - 1)
		step := float64(texH) / wallHeight
		texPos := (float64(drawStart) - f.fsh/2 + wallHeight/2 - vOffset) * step
		for y := drawStart; y < drawEnd; y++ {
			texY := int(texPos) & (texH - 1)
			texPos += step
			src := (texY*texW + texX) * 4
			dst := (y*sw + x) * 4
			rc.pixels[dst] = uint8(float64(tex.Pixels[src]) * shade)
			rc.pixels[dst+1] = uint8(float64(tex.Pixels[src+1]) * shade)
			rc.pixels[dst+2] = uint8(float64(tex.Pixels[src+2]) * shade)
			rc.pixels[dst+3] = 255
		}
		return
	}
	wc := wallColors[wallType]
	stripe := wallX * 255 * 0.3
	r := uint8(float64(wc.R)*shade*0.7 + stripe)
	g := uint8(float64(wc.G)*shade*0.7 + stripe)
	b := uint8(float64(wc.B)*shade*0.7 + stripe)
	for y := drawStart; y < drawEnd; y++ {
		idx := (y*sw + x) * 4
		rc.pixels[idx], rc.pixels[idx+1], rc.pixels[idx+2], rc.pixels[idx+3] = r, g, b, 255
	}
}

// planePixel samples a floor/ceiling texture at world (wx, wy) with a
// precomputed distance shade (one texture repeat per map cell).
func (rc *Raycaster) planePixel(idx int, t *Texture2D, wx, wy, shade float64) {
	tw, th := int(t.Width), int(t.Height)
	tx := int((wx-math.Floor(wx))*float64(tw)) & (tw - 1)
	ty := int((wy-math.Floor(wy))*float64(th)) & (th - 1)
	src := (ty*tw + tx) * 4
	px := t.Pixels[src : src+3 : src+3]
	dst := rc.pixels[idx : idx+4 : idx+4]
	dst[0] = uint8(float64(px[0]) * shade)
	dst[1] = uint8(float64(px[1]) * shade)
	dst[2] = uint8(float64(px[2]) * shade)
	dst[3] = 255
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
