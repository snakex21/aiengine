// renderer/heights.go — per-cell floor/ceiling heights for the grid map.
//
// Heights are in wall-height units: a classic cell has its floor at 0 and
// its ceiling at 1, and the camera sits 0.5 above the floor the player
// stands on. A map without height data (Floor/Ceil nil) behaves exactly like
// the original flat raycaster.
//
// Between two open cells of different heights the renderer draws "step"
// faces: a lower face where the next floor is higher, an upper face where the
// next ceiling is lower. Those faces use the side texture of the cell they
// belong to (MapData.Side, default DefaultSide).

package renderer

import "math"

// Default vertical layout of a cell without height data.
const (
	DefaultFloor   = 0.0
	DefaultCeiling = 1.0
	EyeHeight      = 0.5 // camera height above the floor while standing
)

// Movement limits shared by the player, enemies and pathfinding.
const (
	MaxStepUp      = 0.3  // highest floor rise a body walks up without jumping
	HeadroomStand  = 0.6  // free height needed to stand / walk upright
	HeadroomCrouch = 0.42 // ... crouched
	HeadroomProne  = 0.25 // ... prone
)

// HasHeights reports whether the map carries any height data.
func (m *MapData) HasHeights() bool { return m.Floor != nil || m.Ceil != nil }

// SetHeights allocates the height layers (filled with the defaults) so they
// can be edited cell by cell. ceiling is the default ceiling height.
func (m *MapData) SetHeights(ceiling float64) {
	n := m.Width * m.Height
	m.Floor = make([]float64, n)
	m.Ceil = make([]float64, n)
	for i := range m.Ceil {
		m.Ceil[i] = ceiling
	}
}

func (m *MapData) inside(x, y int) bool { return x >= 0 && x < m.Width && y >= 0 && y < m.Height }

// FloorAt returns the floor height of cell (x, y) (DefaultFloor outside the
// map or without height data).
func (m *MapData) FloorAt(x, y int) float64 {
	if m.Floor == nil || !m.inside(x, y) {
		return DefaultFloor
	}
	return m.Floor[y*m.Width+x]
}

// CeilAt returns the ceiling height of cell (x, y).
func (m *MapData) CeilAt(x, y int) float64 {
	if m.Ceil == nil || !m.inside(x, y) {
		return DefaultCeiling
	}
	return m.Ceil[y*m.Width+x]
}

// SideAt returns the texture of the step faces of cell (x, y).
func (m *MapData) SideAt(x, y int) MapCell {
	if m.Side == nil || !m.inside(x, y) {
		return CellWall2
	}
	return m.Side[y*m.Width+x]
}

// SetFloor / SetCeil change one cell (allocating the layers on first use).
func (m *MapData) SetFloor(x, y int, h float64) {
	if !m.inside(x, y) {
		return
	}
	if m.Floor == nil {
		m.SetHeights(DefaultCeiling)
	}
	m.Floor[y*m.Width+x] = h
}

func (m *MapData) SetCeil(x, y int, h float64) {
	if !m.inside(x, y) {
		return
	}
	if m.Ceil == nil {
		m.SetHeights(DefaultCeiling)
	}
	m.Ceil[y*m.Width+x] = h
}

// cellAtPoint returns the cell containing world point (x, y).
func cellAtPoint(x, y float64) (int, int) { return int(math.Floor(x)), int(math.Floor(y)) }

// FloorAtPoint is FloorAt for a world position.
func (m *MapData) FloorAtPoint(x, y float64) float64 {
	cx, cy := cellAtPoint(x, y)
	return m.FloorAt(cx, cy)
}

// probe offsets of a round body of radius r: centre and four sides.
func bodyProbes(x, y, r float64) [5][2]float64 {
	return [5][2]float64{{x, y}, {x - r, y}, {x + r, y}, {x, y - r}, {x, y + r}}
}

// Support returns the highest floor and the lowest ceiling under a body of
// radius r at (x, y): a body straddling a ledge stands on the higher side
// (it does not drop into gaps narrower than itself).
func (m *MapData) Support(x, y, r float64) (floor, ceil float64) {
	floor, ceil = math.Inf(-1), math.Inf(1)
	for _, p := range bodyProbes(x, y, r) {
		cx, cy := cellAtPoint(p[0], p[1])
		if m.GetCell(cx, cy) != CellEmpty {
			continue
		}
		floor = math.Max(floor, m.FloorAt(cx, cy))
		ceil = math.Min(ceil, m.CeilAt(cx, cy))
	}
	if math.IsInf(floor, -1) {
		cx, cy := cellAtPoint(x, y)
		return m.FloorAt(cx, cy), m.CeilAt(cx, cy)
	}
	return floor, ceil
}

// CanOccupy reports whether a body of radius r whose feet are at height feet
// may stand at (x, y): no solid cell is touched, no touched floor is more than
// maxStep above the feet, and every touched cell leaves at least headroom
// between the floor the body would stand on and the ceiling.
func (m *MapData) CanOccupy(x, y, r, feet, maxStep, headroom float64) bool {
	stand := feet
	for _, p := range bodyProbes(x, y, r) {
		cx, cy := cellAtPoint(p[0], p[1])
		if m.GetCell(cx, cy) != CellEmpty {
			return false
		}
		f := m.FloorAt(cx, cy)
		if f > feet+maxStep+1e-9 {
			return false
		}
		stand = math.Max(stand, f)
	}
	for _, p := range bodyProbes(x, y, r) {
		cx, cy := cellAtPoint(p[0], p[1])
		if m.CeilAt(cx, cy)-stand < headroom-1e-9 {
			return false
		}
	}
	return true
}

// SightClear reports whether the straight segment from (x0,y0,z0) to
// (x1,y1,z1) passes only through open cells and stays between their floor
// and ceiling (z are absolute heights).
func (m *MapData) SightClear(x0, y0, z0, x1, y1, z1 float64) bool {
	dx, dy := x1-x0, y1-y0
	dist := math.Hypot(dx, dy)
	steps := int(dist/0.05) + 1
	for i := 1; i < steps; i++ {
		t := float64(i) / float64(steps)
		cx, cy := cellAtPoint(x0+dx*t, y0+dy*t)
		if m.GetCell(cx, cy) != CellEmpty {
			return false
		}
		z := z0 + (z1-z0)*t
		if z <= m.FloorAt(cx, cy) || z >= m.CeilAt(cx, cy) {
			return false
		}
	}
	return true
}
