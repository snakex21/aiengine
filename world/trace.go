// world/trace.go — the one geometry query for shots, projectiles and sight.
//
// Trace follows a straight 3D segment through the level and reports the
// first surface it meets: a wall (a solid grid cell or a one-sided sector
// line), a step face (the riser between two floors or two ceilings), a floor
// or a ceiling. Grid maps walk the cells with a DDA (Amanatides & Woo);
// sector maps intersect the segment with the sector lines. Both apply the
// same rules, so the player's aim and enemy vision agree on every map.

package world

import (
	"math"
	"sort"

	"aiengine/sector"
)

// Surface is what a trace hit.
type Surface int

const (
	SurfaceNone    Surface = iota
	SurfaceWall            // solid wall or a step face between two floors/ceilings
	SurfaceFloor           // the floor, from above
	SurfaceCeiling         // the ceiling, from below
)

func (s Surface) String() string {
	switch s {
	case SurfaceWall:
		return "wall"
	case SurfaceFloor:
		return "floor"
	case SurfaceCeiling:
		return "ceiling"
	}
	return "none"
}

// Hit is the result of a trace. Without a hit T is 1 and the point is the
// segment's end.
type Hit struct {
	Surface    Surface
	T          float64 // fraction of the segment travelled (0..1)
	X, Y, Z    float64 // hit point
	NX, NY, NZ float64 // unit surface normal, facing the segment's start
}

// Blocked reports whether the segment hit something.
func (h Hit) Blocked() bool { return h.Surface != SurfaceNone }

// Geometry is the walkable, visible level: a grid map or a sector map.
type Geometry interface {
	// Support returns the highest floor and lowest ceiling under a round
	// body of radius r at (x, y).
	Support(x, y, r float64) (floor, ceil float64)
	// CanOccupy reports whether such a body with its feet at feet may stand
	// at (x, y) (step and headroom rules).
	CanOccupy(x, y, r, feet, maxStep, headroom float64) bool
	// Trace follows the segment (x0,y0,z0)-(x1,y1,z1); z are absolute heights.
	Trace(x0, y0, z0, x1, y1, z1 float64) Hit
}

// LineOfSight reports whether nothing blocks the segment between two points.
func LineOfSight(g Geometry, x0, y0, z0, x1, y1, z1 float64) bool {
	return !g.Trace(x0, y0, z0, x1, y1, z1).Blocked()
}

// Of returns the geometry of a level: its sector map when it has one,
// otherwise its grid.
func Of(grid *MapData, sectors *sector.Map) Geometry {
	if sectors != nil {
		return Sectors{sectors}
	}
	return grid
}

// FloorAt is the floor height under a point (no body radius).
func FloorAt(g Geometry, x, y float64) float64 {
	f, _ := g.Support(x, y, 0)
	return f
}

// eps keeps grazing contacts (a sight line exactly at floor height at its
// end point) from counting as hits.
const eps = 1e-9

func miss(x1, y1, z1 float64) Hit { return Hit{T: 1, X: x1, Y: y1, Z: z1} }

func at(s Surface, t, x0, y0, z0, dx, dy, dz, nx, ny, nz float64) Hit {
	return Hit{Surface: s, T: t, X: x0 + dx*t, Y: y0 + dy*t, Z: z0 + dz*t, NX: nx, NY: ny, NZ: nz}
}

// verticalHit checks the stretch [ta, tb] of the segment that lies in one
// open area with floor f and ceiling c: the segment hits the floor or
// ceiling when it leaves that slab inside the stretch.
func verticalHit(ta, tb, f, c, x0, y0, z0, dx, dy, dz float64) (Hit, bool) {
	zb := z0 + dz*tb
	if dz < 0 && zb < f-eps {
		t := math.Max(ta, (f-z0)/dz)
		return at(SurfaceFloor, t, x0, y0, z0, dx, dy, dz, 0, 0, 1), true
	}
	if dz > 0 && zb > c+eps {
		t := math.Max(ta, (c-z0)/dz)
		return at(SurfaceCeiling, t, x0, y0, z0, dx, dy, dz, 0, 0, -1), true
	}
	return Hit{}, false
}

// Trace walks the grid cells crossed by the segment (exact DDA).
func (m *MapData) Trace(x0, y0, z0, x1, y1, z1 float64) Hit {
	dx, dy, dz := x1-x0, y1-y0, z1-z0
	cx, cy := int(math.Floor(x0)), int(math.Floor(y0))
	stepX, stepY := 1, 1
	tMaxX, tMaxY := math.Inf(1), math.Inf(1)
	tDX, tDY := math.Inf(1), math.Inf(1)
	if dx < 0 {
		stepX = -1
	}
	if dy < 0 {
		stepY = -1
	}
	if dx != 0 {
		tDX = math.Abs(1 / dx)
		next := float64(cx)
		if dx > 0 {
			next++
		}
		tMaxX = (next - x0) / dx
	}
	if dy != 0 {
		tDY = math.Abs(1 / dy)
		next := float64(cy)
		if dy > 0 {
			next++
		}
		tMaxY = (next - y0) / dy
	}
	tEnter := 0.0
	var nx, ny float64 // normal of the last cell boundary crossed
	for guard := 0; guard < 1<<16; guard++ {
		tExit := math.Min(math.Min(tMaxX, tMaxY), 1)
		f, c := m.FloorAt(cx, cy), m.CeilAt(cx, cy)
		zIn := z0 + dz*tEnter
		switch {
		case m.GetCell(cx, cy) != CellEmpty:
			return at(SurfaceWall, tEnter, x0, y0, z0, dx, dy, dz, nx, ny, 0)
		case tEnter > 0 && (zIn < f-eps || zIn > c+eps):
			return at(SurfaceWall, tEnter, x0, y0, z0, dx, dy, dz, nx, ny, 0) // step face
		}
		if h, ok := verticalHit(tEnter, tExit, f, c, x0, y0, z0, dx, dy, dz); ok {
			return h
		}
		if tExit >= 1 {
			return miss(x1, y1, z1)
		}
		if tMaxX < tMaxY {
			cx += stepX
			tEnter = tMaxX
			tMaxX += tDX
			nx, ny = float64(-stepX), 0
		} else {
			cy += stepY
			tEnter = tMaxY
			tMaxY += tDY
			nx, ny = 0, float64(-stepY)
		}
	}
	return miss(x1, y1, z1)
}

// SightClear reports whether the segment between two points is unblocked
// (kept for older callers; same as LineOfSight on the grid).
func (m *MapData) SightClear(x0, y0, z0, x1, y1, z1 float64) bool {
	return !m.Trace(x0, y0, z0, x1, y1, z1).Blocked()
}

// Sectors adapts a sector map to Geometry (Support and CanOccupy come from
// sector.Map, Trace is implemented here).
type Sectors struct{ *sector.Map }

type crossing struct {
	t    float64
	line int
}

// Trace intersects the segment with the sector lines: a one-sided line is
// a wall, a portal whose floor/ceiling gap the segment misses is a step
// face, and between crossings the segment may hit the floor or ceiling of
// the sector it is in.
func (s Sectors) Trace(x0, y0, z0, x1, y1, z1 float64) Hit {
	m := s.Map
	dx, dy, dz := x1-x0, y1-y0, z1-z0
	p0, p1 := sector.Vec2{X: x0, Y: y0}, sector.Vec2{X: x1, Y: y1}
	if m.SectorAt(x0, y0) < 0 {
		return at(SurfaceWall, 0, x0, y0, z0, dx, dy, dz, 0, 0, 0)
	}
	minX, maxX := math.Min(x0, x1), math.Max(x0, x1)
	minY, maxY := math.Min(y0, y1), math.Max(y0, y1)
	var buf [32]crossing
	cs := buf[:0]
	for i := range m.Lines {
		l := &m.Lines[i]
		if math.Max(l.A.X, l.B.X) < minX || math.Min(l.A.X, l.B.X) > maxX ||
			math.Max(l.A.Y, l.B.Y) < minY || math.Min(l.A.Y, l.B.Y) > maxY {
			continue
		}
		if t, _, ok := sector.SegmentHit(p0, p1, l); ok {
			cs = append(cs, crossing{t, i})
		}
	}
	sort.Slice(cs, func(a, b int) bool { return cs[a].t < cs[b].t })
	ta := 0.0
	for _, c := range cs {
		if c.t > ta {
			// The open stretch before this line.
			mx, my := x0+dx*(ta+c.t)/2, y0+dy*(ta+c.t)/2
			if sec := m.SectorAt(mx, my); sec >= 0 {
				sc := &m.Sectors[sec]
				if h, ok := verticalHit(ta, c.t, sc.Floor, sc.Ceil, x0, y0, z0, dx, dy, dz); ok {
					return h
				}
			}
		}
		l := &m.Lines[c.line]
		// Normal of the line, facing the segment's start.
		nx, ny := l.B.Y-l.A.Y, -(l.B.X - l.A.X)
		if n := math.Hypot(nx, ny); n > 0 {
			nx, ny = nx/n, ny/n
		}
		if nx*dx+ny*dy > 0 {
			nx, ny = -nx, -ny
		}
		if !l.TwoSided() {
			return at(SurfaceWall, c.t, x0, y0, z0, dx, dy, dz, nx, ny, 0)
		}
		z := z0 + dz*c.t
		a, b := &m.Sectors[l.Front], &m.Sectors[l.Back]
		if z < math.Max(a.Floor, b.Floor)-eps || z > math.Min(a.Ceil, b.Ceil)+eps {
			return at(SurfaceWall, c.t, x0, y0, z0, dx, dy, dz, nx, ny, 0)
		}
		ta = math.Max(ta, c.t)
	}
	if sec := m.SectorAt(x0+dx*(ta+1)/2, y0+dy*(ta+1)/2); sec >= 0 {
		sc := &m.Sectors[sec]
		if h, ok := verticalHit(ta, 1, sc.Floor, sc.Ceil, x0, y0, z0, dx, dy, dz); ok {
			return h
		}
	}
	return miss(x1, y1, z1)
}
