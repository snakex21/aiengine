// renderer/sector_cast.go — column renderer for sector maps (package
// sector): polygons with their own floor/ceiling heights joined by portal
// lines, walls at any angle.
//
// Each screen column follows its ray front to back through the sector
// graph: in the current sector it finds the nearest line the ray crosses,
// fills the sector's ceiling and floor up to that distance, then either
// draws a solid wall and stops, or draws the upper/lower step faces of the
// portal and continues in the sector behind it. The per-column open window
// and its log (the same as the grid renderer's) clip everything farther
// away, including sprites. Floors and ceilings are drawn as per-column
// spans with the distance of every row.

package renderer

import (
	"math"

	"aiengine/sector"
)

// sectorFrame holds per-frame data for a sector map.
type sectorFrame struct {
	m         *sector.Map
	cam       int // sector of the camera (-1 = outside the map)
	floorTex  []*Texture2D
	ceilTex   []*Texture2D
	stepTex   []*Texture2D
	stepCell  []MapCell // flat-colour fallback of the step faces
	light     []int32   // per sector, 8.8 fixed point
	lineTex   []*Texture2D
	lineCell  []MapCell
	lineLen   []float64
	lineK     []int32 // directional factor (x-facing walls brighter)
	lineAxisX []bool  // vertical line: texture u follows world y
	lineAxisY []bool  // horizontal line: texture u follows world x
}

// symbolTextures maps the grid symbols to the wall texture names.
var symbolTextures = map[string]struct {
	name string
	cell MapCell
}{
	"W": {"brick", CellWall1}, "S": {"stone", CellWall2}, "M": {"metal", CellWall3}, "D": {"wood", CellDoor},
}

// sectorsLit reports whether any sector has a light level other than 1.
func (rc *Raycaster) sectorsLit() bool {
	for i := range rc.Sectors.Sectors {
		if rc.Sectors.Sectors[i].Light != 1 {
			return true
		}
	}
	return false
}

// prepareSectors resolves textures, lights and line constants for a frame.
func (rc *Raycaster) prepareSectors(f *frame) {
	m := rc.Sectors
	sf := rc.secFrame
	if sf == nil {
		sf = &sectorFrame{}
		rc.secFrame = sf
	}
	sf.m = m
	sf.cam = m.SectorAt(f.px, f.py)
	ns, nl := len(m.Sectors), len(m.Lines)
	grow := func(n int) {
		if cap(sf.floorTex) < ns {
			sf.floorTex, sf.ceilTex, sf.stepTex = make([]*Texture2D, n), make([]*Texture2D, n), make([]*Texture2D, n)
			sf.stepCell, sf.light = make([]MapCell, n), make([]int32, n)
		}
	}
	grow(ns)
	sf.floorTex, sf.ceilTex, sf.stepTex = sf.floorTex[:ns], sf.ceilTex[:ns], sf.stepTex[:ns]
	sf.stepCell, sf.light = sf.stepCell[:ns], sf.light[:ns]
	if cap(sf.lineTex) < nl {
		sf.lineTex, sf.lineCell = make([]*Texture2D, nl), make([]MapCell, nl)
		sf.lineLen, sf.lineK = make([]float64, nl), make([]int32, nl)
		sf.lineAxisX, sf.lineAxisY = make([]bool, nl), make([]bool, nl)
	}
	sf.lineTex, sf.lineCell, sf.lineLen, sf.lineK = sf.lineTex[:nl], sf.lineCell[:nl], sf.lineLen[:nl], sf.lineK[:nl]
	sf.lineAxisX, sf.lineAxisY = sf.lineAxisX[:nl], sf.lineAxisY[:nl]

	cache := map[string]*Texture2D{}
	resolve := func(name string) (*Texture2D, MapCell) {
		cell := CellWall2
		if st, ok := symbolTextures[name]; ok {
			name, cell = st.name, st.cell
		}
		if name == "" {
			return nil, cell
		}
		t, ok := cache[name]
		if !ok {
			t = usableTex(rc.tex(name))
			cache[name] = t
		}
		return t, cell
	}
	for i := range m.Sectors {
		s := &m.Sectors[i]
		sf.floorTex[i], _ = resolve(s.FloorTex)
		sf.ceilTex[i], _ = resolve(s.CeilTex)
		step := s.StepTex
		if step == "" {
			step = "S"
		}
		sf.stepTex[i], sf.stepCell[i] = resolve(step)
		sf.light[i] = int32(s.Light * 256)
	}
	// Dynamic point lights add light to sectors around them (by the
	// distance to the sector's centre, without occlusion).
	for _, l := range rc.Lights {
		for i := range m.Sectors {
			c := m.Centroid(i)
			if d := math.Hypot(c.X-l.X, c.Y-l.Y); d < l.Radius {
				sf.light[i] += int32(l.Level * (1 - d/l.Radius) * 256)
			}
		}
	}
	for i := range sf.light {
		sf.light[i] = min(max(sf.light[i], 0), 768)
	}
	for i := range m.Lines {
		l := &m.Lines[i]
		sf.lineTex[i], sf.lineCell[i] = resolve(l.Tex)
		if l.Tex == "" {
			sf.lineTex[i], sf.lineCell[i] = resolve("W")
		}
		dx, dy := l.B.X-l.A.X, l.B.Y-l.A.Y
		ln := math.Hypot(dx, dy)
		sf.lineLen[i] = ln
		// Walls facing along x (vertical lines) are lit fully, walls facing
		// along y at 0.7, angled walls in between (as the grid's sides).
		sf.lineK[i] = 179 + int32(77*math.Abs(dy)/ln+0.5)
		sf.lineAxisX[i] = math.Abs(dx) < 1e-9
		sf.lineAxisY[i] = math.Abs(dy) < 1e-9
	}
	f.sec = sf
}

// thingFloorLight returns the floor height and light (8.8) under a sprite.
func (rc *Raycaster) thingFloorLight(f *frame, x, y float64) (float64, int32) {
	if f.sec != nil {
		s := f.sec.m.SectorAt(x, y)
		if s < 0 {
			return 0, 256
		}
		return f.sec.m.Sectors[s].Floor, f.sec.light[s]
	}
	cx, cy := cellAtPoint(x, y)
	return rc.Map.FloorAt(cx, cy), f.light(rc.Map, cx, cy)
}

// maxSectorSteps bounds the portals one ray may pass.
const maxSectorSteps = 512

func (rc *Raycaster) castSectorColumn(f *frame, x int) {
	sf := f.sec
	m := sf.m
	cameraX := 2.0*float64(x)/f.fsw - 1.0
	rdx := f.dirX + f.planeX*cameraX
	rdy := f.dirY + f.planeY*cameraX

	clips := rc.clips[x][:0]
	defer func() { rc.clips[x] = clips }()
	rc.zBuffer[x] = 0
	top, bot := 0, f.sh
	s := sf.cam
	if s < 0 { // camera outside the map (noclip): black
		for y := 0; y < f.sh; y++ {
			i := (y*f.sw + x) * 4
			rc.pixels[i], rc.pixels[i+1], rc.pixels[i+2], rc.pixels[i+3] = 0, 0, 0, 255
		}
		clips = append(clips, clipEntry{nearPlane, 0, 0})
		return
	}
	from, prev := -1, -1
	tPrev := 0.0
	for step := 0; step < maxSectorSteps; step++ {
		sec := &m.Sectors[s]
		L := sf.light[s]
		// Nearest line of this sector the ray leaves through.
		best, bestU, bestLine := math.Inf(1), 0.0, -1
		for _, li := range sec.Lines {
			if li == from {
				continue
			}
			l := &m.Lines[li]
			ex, ey := l.B.X-l.A.X, l.B.Y-l.A.Y
			den := rdx*ey - rdy*ex
			if math.Abs(den) < 1e-12 {
				continue
			}
			qx, qy := l.A.X-f.px, l.A.Y-f.py
			t := (qx*ey - qy*ex) / den
			u := (qx*rdy - qy*rdx) / den
			if u < -1e-9 || u > 1+1e-9 || t < tPrev-1e-9 || t >= best {
				continue
			}
			// Through a vertex the ray touches several lines at once: never
			// step straight back into the sector it came from.
			if t < tPrev+1e-7 && prev >= 0 && l.Other(s) == prev {
				continue
			}
			best, bestU, bestLine = t, u, li
		}
		if bestLine < 0 { // open sector (bad data): planes to the horizon
			h := min(max(f.rowOf(f.horizon), top), bot)
			rc.drawPlane(f, x, top, h, sec.Ceil-f.eye, L, sf.ceilTex[s], f.ceil, rdx, rdy, true)
			rc.drawPlane(f, x, h, bot, f.eye-sec.Floor, L, sf.floorTex[s], f.flr, rdx, rdy, false)
			return
		}
		d := math.Max(best, nearPlane)

		// This sector's ceiling and floor up to the line.
		if e := min(f.rowOf(f.rowAt(sec.Ceil, d)), bot); e > top {
			rc.drawPlane(f, x, top, e, sec.Ceil-f.eye, L, sf.ceilTex[s], f.ceil, rdx, rdy, true)
			top = e
		}
		if b := max(f.rowOf(f.rowAt(sec.Floor, d)), top); b < bot {
			rc.drawPlane(f, x, b, bot, f.eye-sec.Floor, L, sf.floorTex[s], f.flr, rdx, rdy, false)
			bot = b
		}

		// Texture u: world-aligned on axis lines (matches the grid
		// renderer), distance along the line otherwise.
		var wallX float64
		switch {
		case sf.lineAxisX[bestLine]:
			wallX = f.py + d*rdy
		case sf.lineAxisY[bestLine]:
			wallX = f.px + d*rdx
		default:
			wallX = bestU * sf.lineLen[bestLine]
		}
		wallX -= math.Floor(wallX)

		l := &m.Lines[bestLine]
		k := sf.lineK[bestLine]
		o := l.Other(s)
		if o < 0 { // solid wall
			if top < bot {
				rc.drawWallTex(f, x, top, bot, d, L, sf.lineTex[bestLine], sf.lineCell[bestLine], k, wallX, 1)
			}
			rc.zBuffer[x] = d
			clips = append(clips, clipEntry{d, int32(top), int32(top)})
			return
		}
		next := &m.Sectors[o]
		if next.Ceil < sec.Ceil { // upper step face
			if e := min(f.rowOf(f.rowAt(next.Ceil, d)), bot); e > top {
				a := 1.0
				if next.Peg {
					a = next.Ceil
				}
				rc.drawWallTex(f, x, top, e, d, L, sf.stepTex[o], sf.stepCell[o], k, wallX, a)
				top = e
			}
		}
		if next.Floor > sec.Floor { // lower step face
			if b := max(f.rowOf(f.rowAt(next.Floor, d)), top); b < bot {
				a := 1.0
				if next.Peg {
					a = next.Floor
				}
				rc.drawWallTex(f, x, b, bot, d, L, sf.stepTex[o], sf.stepCell[o], k, wallX, a)
				bot = b
			}
		}
		clips = append(clips, clipEntry{d, int32(top), int32(bot)})
		if top >= bot {
			rc.zBuffer[x] = d
			return
		}
		s, prev, from, tPrev = o, s, bestLine, best
	}
}

// GridSectors adapts a grid map to sector.FromGrid (walls take the texture
// of their solid cell, step faces the cell's side texture).
func GridSectors(m *MapData) sector.Grid { return gridSource{m} }

type gridSource struct{ m *MapData }

var cellSymbols = map[MapCell]string{CellWall1: "W", CellWall2: "S", CellWall3: "M", CellDoor: "D"}

func (g gridSource) Size() (int, int)        { return g.m.Width, g.m.Height }
func (g gridSource) Open(x, y int) bool      { return g.m.GetCell(x, y) == CellEmpty }
func (g gridSource) Floor(x, y int) float64  { return g.m.FloorAt(x, y) }
func (g gridSource) Ceil(x, y int) float64   { return g.m.CeilAt(x, y) }
func (g gridSource) Light(x, y int) float64  { return g.m.LightAt(x, y) }
func (g gridSource) WallTex(x, y int) string { return cellSymbols[g.m.GetCell(x, y)] }
func (g gridSource) StepTex(x, y int) string { return cellSymbols[g.m.SideAt(x, y)] }
