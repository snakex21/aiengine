package sector

import "math"

// index is a uniform grid (1 unit cells) over the map's bounding box that
// lists, per cell, the sectors and lines whose bounding boxes overlap it.
type index struct {
	x0, y0  int
	w, h    int
	sectors [][]int32
	lines   [][]int32
}

func (m *Map) buildIndex() {
	ix := &index{x0: int(math.Floor(m.Min.X)), y0: int(math.Floor(m.Min.Y))}
	ix.w = int(math.Floor(m.Max.X)) - ix.x0 + 1
	ix.h = int(math.Floor(m.Max.Y)) - ix.y0 + 1
	ix.sectors = make([][]int32, ix.w*ix.h)
	ix.lines = make([][]int32, ix.w*ix.h)
	box := func(pts []Vec2, fn func(c int)) {
		lo, hi := pts[0], pts[0]
		for _, p := range pts[1:] {
			lo = Vec2{math.Min(lo.X, p.X), math.Min(lo.Y, p.Y)}
			hi = Vec2{math.Max(hi.X, p.X), math.Max(hi.Y, p.Y)}
		}
		for y := int(math.Floor(lo.Y)) - ix.y0; y <= int(math.Floor(hi.Y))-ix.y0; y++ {
			for x := int(math.Floor(lo.X)) - ix.x0; x <= int(math.Floor(hi.X))-ix.x0; x++ {
				if x >= 0 && y >= 0 && x < ix.w && y < ix.h {
					fn(y*ix.w + x)
				}
			}
		}
	}
	for i := range m.Sectors {
		box(m.Sectors[i].Poly, func(c int) { ix.sectors[c] = append(ix.sectors[c], int32(i)) })
	}
	for i := range m.Lines {
		l := &m.Lines[i]
		box([]Vec2{l.A, l.B}, func(c int) { ix.lines[c] = append(ix.lines[c], int32(i)) })
	}
	m.ix = ix
}

func (ix *index) cell(x, y float64) int {
	cx, cy := int(math.Floor(x))-ix.x0, int(math.Floor(y))-ix.y0
	if cx < 0 || cy < 0 || cx >= ix.w || cy >= ix.h {
		return -1
	}
	return cy*ix.w + cx
}

// linesNear calls fn for every line whose bounding box overlaps the square
// of half-size r around p (a line may be reported more than once).
func (ix *index) linesNear(p Vec2, r float64, fn func(li int)) {
	for y := int(math.Floor(p.Y-r)) - ix.y0; y <= int(math.Floor(p.Y+r))-ix.y0; y++ {
		for x := int(math.Floor(p.X-r)) - ix.x0; x <= int(math.Floor(p.X+r))-ix.x0; x++ {
			if x < 0 || y < 0 || x >= ix.w || y >= ix.h {
				continue
			}
			for _, li := range ix.lines[y*ix.w+x] {
				fn(int(li))
			}
		}
	}
}
