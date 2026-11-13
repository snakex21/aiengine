// Package sector holds maps made of sectors: polygons with their own floor
// and ceiling height, light and textures, bounded by lines (walls at any
// angle). An edge shared by two sectors is a two-sided line (a portal: you
// see and walk through it, with step faces where heights differ); an edge
// of only one sector is a solid wall.
//
// World units are the same as the grid maps: 1 unit = one grid cell = the
// height of a classic wall; x grows to the right, y downwards (map rows).
//
// The package is pure Go (no OpenGL), so geometry, collision and line of
// sight are tested headlessly. Floor and Ceil are plain fields, so moving
// floors, doors and lifts can change them at run time.
package sector

import (
	"fmt"
	"math"
	"sort"
)

// Vec2 is a point or vector on the map plane.
type Vec2 struct{ X, Y float64 }

func (a Vec2) Sub(b Vec2) Vec2       { return Vec2{a.X - b.X, a.Y - b.Y} }
func (a Vec2) Add(b Vec2) Vec2       { return Vec2{a.X + b.X, a.Y + b.Y} }
func (a Vec2) Scale(k float64) Vec2  { return Vec2{a.X * k, a.Y * k} }
func (a Vec2) Dot(b Vec2) float64    { return a.X*b.X + a.Y*b.Y }
func (a Vec2) Cross(b Vec2) float64  { return a.X*b.Y - a.Y*b.X }
func (a Vec2) Len() float64          { return math.Hypot(a.X, a.Y) }
func (a Vec2) Dist(b Vec2) float64   { return a.Sub(b).Len() }
func (a Vec2) Near(b Vec2) bool      { return math.Abs(a.X-b.X) < 1e-6 && math.Abs(a.Y-b.Y) < 1e-6 }
func lerp(a, b Vec2, t float64) Vec2 { return a.Add(b.Sub(a).Scale(t)) }
func (a Vec2) String() string        { return fmt.Sprintf("(%g,%g)", a.X, a.Y) }
func vkey(a Vec2) [2]int64 {
	return [2]int64{int64(math.Round(a.X * 1e6)), int64(math.Round(a.Y * 1e6))}
}
func ekey(a, b Vec2) [2][2]int64 {
	ka, kb := vkey(a), vkey(b)
	if ka[0] > kb[0] || (ka[0] == kb[0] && ka[1] > kb[1]) {
		ka, kb = kb, ka
	}
	return [2][2]int64{ka, kb}
}

// Sector is one polygonal area.
type Sector struct {
	ID       string
	Floor    float64 // floor height (may change at run time)
	Ceil     float64 // ceiling height (may change at run time)
	Light    float64 // light level, 1 = full
	FloorTex string  // texture names ("" = flat map colour)
	CeilTex  string
	StepTex  string // texture of step faces rising from/down to this sector
	Poly     []Vec2 // outline, in order (either winding)
	Lines    []int  // indices of the lines bounding it
	Tag      string // free tag for scripts (doors, lifts, ...)
	// Peg anchors the texture of step faces into this sector to its
	// ceiling (set for doors, so the door texture moves with the door).
	Peg bool
}

// Line is a wall or portal between two points.
type Line struct {
	A, B  Vec2
	Front int    // a sector on one side
	Back  int    // the sector on the other side, -1 for a solid wall
	Tex   string // wall texture of a solid wall
}

// TwoSided reports a portal line.
func (l *Line) TwoSided() bool { return l.Back >= 0 }

// Other returns the sector on the other side of the line from s (-1 when
// solid).
func (l *Line) Other(s int) int {
	if l.Front == s {
		return l.Back
	}
	return l.Front
}

// Map is a sector map.
type Map struct {
	Sectors []Sector
	Lines   []Line
	Min     Vec2 // bounding box
	Max     Vec2

	ix *index // spatial lookup (index.go)
}

// Def describes one sector for Build: its outline and properties. EdgeTex,
// when set, gives the wall texture of each edge (edge i runs from Poly[i] to
// Poly[i+1]); otherwise WallTex is used for all solid edges.
type Def struct {
	Sector
	WallTex string
	EdgeTex []string
}

// Build creates a map from sector outlines. Edges with the same two end
// points in two sectors become portals; edges of one sector become walls.
// Sectors must meet at shared vertices (no T-junctions), must not overlap,
// and need at least 3 points.
func Build(defs []Def) (*Map, error) {
	m := &Map{Min: Vec2{math.Inf(1), math.Inf(1)}, Max: Vec2{math.Inf(-1), math.Inf(-1)}}
	type owner struct{ sector, line int }
	edges := map[[2][2]int64]owner{}
	for si, d := range defs {
		if len(d.Poly) < 3 {
			return nil, fmt.Errorf("sector %q: needs at least 3 points", d.ID)
		}
		if math.Abs(polyArea(d.Poly)) < 1e-9 {
			return nil, fmt.Errorf("sector %q: zero area", d.ID)
		}
		if d.Ceil < d.Floor {
			return nil, fmt.Errorf("sector %q: ceiling %.3g below floor %.3g", d.ID, d.Ceil, d.Floor)
		}
		s := d.Sector
		s.Poly = append([]Vec2(nil), d.Poly...)
		s.Lines = nil
		for i := range s.Poly {
			a, b := s.Poly[i], s.Poly[(i+1)%len(s.Poly)]
			if a.Near(b) {
				return nil, fmt.Errorf("sector %q: repeated point %v", d.ID, a)
			}
			m.Min = Vec2{math.Min(m.Min.X, a.X), math.Min(m.Min.Y, a.Y)}
			m.Max = Vec2{math.Max(m.Max.X, a.X), math.Max(m.Max.Y, a.Y)}
			k := ekey(a, b)
			if o, ok := edges[k]; ok {
				l := &m.Lines[o.line]
				if l.Back >= 0 {
					return nil, fmt.Errorf("edge %v-%v is shared by more than two sectors", a, b)
				}
				if o.sector == si {
					return nil, fmt.Errorf("sector %q: edge %v-%v used twice", d.ID, a, b)
				}
				l.Back = si
				s.Lines = append(s.Lines, o.line)
				continue
			}
			tex := d.WallTex
			if i < len(d.EdgeTex) && d.EdgeTex[i] != "" {
				tex = d.EdgeTex[i]
			}
			edges[k] = owner{si, len(m.Lines)}
			s.Lines = append(s.Lines, len(m.Lines))
			m.Lines = append(m.Lines, Line{A: a, B: b, Front: si, Back: -1, Tex: tex})
		}
		m.Sectors = append(m.Sectors, s)
	}
	if err := m.checkTJunctions(); err != nil {
		return nil, err
	}
	m.buildIndex()
	return m, nil
}

// checkTJunctions rejects solid walls that overlap another collinear edge
// without sharing both end points (sectors that touch along part of an edge:
// they would be sealed off from each other).
func (m *Map) checkTJunctions() error {
	idx := make([]int, 0, len(m.Lines))
	for i := range m.Lines {
		if !m.Lines[i].TwoSided() {
			idx = append(idx, i)
		}
	}
	sort.Slice(idx, func(a, b int) bool {
		return math.Min(m.Lines[idx[a]].A.X, m.Lines[idx[a]].B.X) < math.Min(m.Lines[idx[b]].A.X, m.Lines[idx[b]].B.X)
	})
	for ii, i := range idx {
		li := &m.Lines[i]
		maxX := math.Max(li.A.X, li.B.X)
		for _, j := range idx[ii+1:] {
			lj := &m.Lines[j]
			if math.Min(lj.A.X, lj.B.X) > maxX+1e-9 {
				break
			}
			if li.Front == lj.Front || !collinearOverlap(li.A, li.B, lj.A, lj.B) {
				continue
			}
			return fmt.Errorf("sectors %q and %q touch along %v-%v without shared vertices (split the edges)",
				m.Sectors[li.Front].ID, m.Sectors[lj.Front].ID, li.A, li.B)
		}
	}
	return nil
}

func collinearOverlap(a, b, c, d Vec2) bool {
	ab := b.Sub(a)
	if math.Abs(ab.Cross(c.Sub(a))) > 1e-9 || math.Abs(ab.Cross(d.Sub(a))) > 1e-9 {
		return false
	}
	l2 := ab.Dot(ab)
	t0, t1 := c.Sub(a).Dot(ab)/l2, d.Sub(a).Dot(ab)/l2
	if t0 > t1 {
		t0, t1 = t1, t0
	}
	return math.Min(1, t1)-math.Max(0, t0) > 1e-9
}

func polyArea(p []Vec2) float64 {
	a := 0.0
	for i := range p {
		a += p[i].Cross(p[(i+1)%len(p)])
	}
	return a / 2
}

// Contains reports whether point p lies inside sector s (even-odd rule over
// its lines; points exactly on an edge may go either way).
func (m *Map) Contains(s int, p Vec2) bool {
	in := false
	for _, li := range m.Sectors[s].Lines {
		a, b := m.Lines[li].A, m.Lines[li].B
		if (a.Y > p.Y) != (b.Y > p.Y) {
			x := a.X + (p.Y-a.Y)*(b.X-a.X)/(b.Y-a.Y)
			if x > p.X {
				in = !in
			}
		}
	}
	return in
}

// SectorAt returns the sector containing (x, y), or -1 outside the map.
func (m *Map) SectorAt(x, y float64) int {
	p := Vec2{x, y}
	if x < m.Min.X || y < m.Min.Y || x > m.Max.X || y > m.Max.Y {
		return -1
	}
	if m.ix != nil {
		c := m.ix.cell(x, y)
		if c < 0 {
			return -1
		}
		for _, s := range m.ix.sectors[c] {
			if m.Contains(int(s), p) {
				return int(s)
			}
		}
		return -1
	}
	for i := range m.Sectors {
		if m.Contains(i, p) {
			return i
		}
	}
	return -1
}

// SectorByID finds a sector by its id.
func (m *Map) SectorByID(id string) int {
	for i := range m.Sectors {
		if m.Sectors[i].ID == id {
			return i
		}
	}
	return -1
}

// Centroid is the average of a sector's outline points.
func (m *Map) Centroid(s int) Vec2 {
	var c Vec2
	for _, p := range m.Sectors[s].Poly {
		c = c.Add(p)
	}
	return c.Scale(1 / float64(len(m.Sectors[s].Poly)))
}

// segDist is the distance from p to segment ab.
func segDist(p, a, b Vec2) float64 {
	ab := b.Sub(a)
	t := p.Sub(a).Dot(ab) / ab.Dot(ab)
	t = math.Max(0, math.Min(1, t))
	return p.Dist(lerp(a, b, t))
}

// touched returns the sectors a body of radius r at p overlaps: the one
// containing p and those on both sides of every line within r. solid is
// true when a solid wall is within r or p is outside the map.
func (m *Map) touched(p Vec2, r float64, buf []int) (out []int, solid bool) {
	out = buf[:0]
	add := func(s int) {
		for _, o := range out {
			if o == s {
				return
			}
		}
		out = append(out, s)
	}
	s := m.SectorAt(p.X, p.Y)
	if s < 0 {
		return out, true
	}
	add(s)
	solid = false
	m.ix.linesNear(p, r, func(i int) {
		l := &m.Lines[i]
		if solid || segDist(p, l.A, l.B) >= r {
			return
		}
		if !l.TwoSided() {
			solid = true
			return
		}
		add(l.Front)
		add(l.Back)
	})
	return out, solid
}

// Touched lists the sectors a body of radius r at (x, y) overlaps (see
// touched); solid reports a wall within r or a point outside the map.
func (m *Map) Touched(x, y, r float64, buf []int) (sectors []int, solid bool) {
	return m.touched(Vec2{x, y}, r, buf)
}

// Neighbours lists the sectors sharing a portal line with sector s.
func (m *Map) Neighbours(s int) []int {
	var out []int
	for _, li := range m.Sectors[s].Lines {
		if o := m.Lines[li].Other(s); o >= 0 {
			dup := false
			for _, e := range out {
				dup = dup || e == o
			}
			if !dup {
				out = append(out, o)
			}
		}
	}
	return out
}

// Support returns the highest floor and lowest ceiling under a body of
// radius r at (x, y) (it stands on the highest floor it overlaps).
func (m *Map) Support(x, y, r float64) (floor, ceil float64) {
	var buf [8]int
	ts, _ := m.touched(Vec2{x, y}, r, buf[:])
	if len(ts) == 0 {
		return 0, 0
	}
	floor, ceil = math.Inf(-1), math.Inf(1)
	for _, s := range ts {
		floor = math.Max(floor, m.Sectors[s].Floor)
		ceil = math.Min(ceil, m.Sectors[s].Ceil)
	}
	return floor, ceil
}

// CanOccupy reports whether a body of radius r with its feet at height feet
// may stand at (x, y): it touches no solid wall, no overlapped floor is
// more than maxStep above the feet, and it has headroom under every
// overlapped ceiling (same rules as the grid maps).
func (m *Map) CanOccupy(x, y, r, feet, maxStep, headroom float64) bool {
	var buf [8]int
	ts, solid := m.touched(Vec2{x, y}, r, buf[:])
	if solid {
		return false
	}
	stand := feet
	for _, s := range ts {
		f := m.Sectors[s].Floor
		if f > feet+maxStep+1e-9 {
			return false
		}
		stand = math.Max(stand, f)
	}
	for _, s := range ts {
		if m.Sectors[s].Ceil-stand < headroom-1e-9 {
			return false
		}
	}
	return true
}

// SegmentHit intersects segment p0-p1 with line l: t along p0-p1 and u
// along the line, ok when they cross.
func SegmentHit(p0, p1 Vec2, l *Line) (t, u float64, ok bool) {
	r := p1.Sub(p0)
	s := l.B.Sub(l.A)
	den := r.Cross(s)
	if math.Abs(den) < 1e-12 {
		return 0, 0, false
	}
	q := l.A.Sub(p0)
	t = q.Cross(s) / den
	u = q.Cross(r) / den
	return t, u, t >= 0 && t <= 1 && u >= 0 && u <= 1
}

// SightClear reports whether an eye at (x0,y0,z0) sees (x1,y1,z1): every
// line crossed is a portal, and at the crossing the sight line passes above
// both floors and below both ceilings.
func (m *Map) SightClear(x0, y0, z0, x1, y1, z1 float64) bool {
	p0, p1 := Vec2{x0, y0}, Vec2{x1, y1}
	for i := range m.Lines {
		l := &m.Lines[i]
		t, _, ok := SegmentHit(p0, p1, l)
		if !ok {
			continue
		}
		if !l.TwoSided() {
			return false
		}
		z := z0 + (z1-z0)*t
		a, b := &m.Sectors[l.Front], &m.Sectors[l.Back]
		if z <= math.Max(a.Floor, b.Floor) || z >= math.Min(a.Ceil, b.Ceil) {
			return false
		}
	}
	return true
}
