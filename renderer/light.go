// renderer/light.go — sector-style lighting for the grid map.
//
// Every open cell has a light level (MapData.Light, 1 = full, nil = all
// full). On top of it come point lights (Raycaster.Lights: glowing items,
// projectiles, flickering torches driven by the level), which add light to
// the cells they can see within their radius, and the weapon flash
// (Raycaster.Flash), a light at the camera that fades with distance.
// Walls, step faces, floors, ceilings and sprites take the light of the cell
// they belong to, then the distance shading, then distance fog
// (Raycaster.Fog). Lighting is resolved per cell, like the per-sector light
// of classic 2.5D engines: cheap at any resolution.

package renderer

import "math"

// PointLight adds light to the cells around (X, Y) that it can see.
type PointLight struct {
	X, Y   float64
	Radius float64 // cells beyond it get nothing (linear falloff)
	Level  float64 // light added at the centre (1 = a fully lit cell)
}

// Fog blends distant pixels towards Color: factor 1-exp(-Density*distance).
type Fog struct {
	Density float64 // 0 = off
	Color   [3]uint8
}

// LitCell is one cell reached by a light and the light it receives (before
// the light's level is applied).
type LitCell struct {
	Index  int // y*Width + x
	Weight float64
}

// LightFootprint lists the open cells within radius of (x, y) that have a
// clear 2D line from the light to their centre (light does not pass through
// solid cells), with a linear falloff weight.
func LightFootprint(m *MapData, x, y, radius float64) []LitCell {
	if radius <= 0 {
		return nil
	}
	var out []LitCell
	x0, y0 := int(math.Floor(x-radius)), int(math.Floor(y-radius))
	x1, y1 := int(math.Floor(x+radius)), int(math.Floor(y+radius))
	for cy := max(y0, 0); cy <= min(y1, m.Height-1); cy++ {
		for cx := max(x0, 0); cx <= min(x1, m.Width-1); cx++ {
			if m.GetCell(cx, cy) != CellEmpty {
				continue
			}
			// Nearest point of the cell to the light (a light inside a big
			// room lights whole neighbouring cells, not just their centres).
			nx := math.Max(float64(cx), math.Min(x, float64(cx)+1))
			ny := math.Max(float64(cy), math.Min(y, float64(cy)+1))
			d := math.Hypot(nx-x, ny-y)
			if d >= radius || !openLine(m, x, y, float64(cx)+0.5, float64(cy)+0.5) {
				continue
			}
			out = append(out, LitCell{Index: cy*m.Width + cx, Weight: 1 - d/radius})
		}
	}
	return out
}

// openLine reports whether the segment crosses no solid cell.
func openLine(m *MapData, x0, y0, x1, y1 float64) bool {
	dx, dy := x1-x0, y1-y0
	n := int(math.Hypot(dx, dy)/0.2) + 1
	for i := 1; i < n; i++ {
		t := float64(i) / float64(n)
		cx, cy := cellAtPoint(x0+dx*t, y0+dy*t)
		if m.GetCell(cx, cy) != CellEmpty {
			return false
		}
	}
	return true
}

// LightAt returns the base light level of a cell (1 without a light layer).
func (m *MapData) LightAt(x, y int) float64 {
	if m.Light == nil || !m.inside(x, y) {
		return 1
	}
	return m.Light[y*m.Width+x]
}

// SetLight changes one cell's base light (allocating the layer, all 1).
func (m *MapData) SetLight(x, y int, v float64) {
	if !m.inside(x, y) {
		return
	}
	if m.Light == nil {
		m.Light = make([]float64, m.Width*m.Height)
		for i := range m.Light {
			m.Light[i] = 1
		}
	}
	m.Light[y*m.Width+x] = v
}

// lightEntry is the per-distance part of the lighting (8.8 fixed point),
// indexed by distance*shadeRes.
type lightEntry struct {
	dist             int32 // distance shading
	flash            int32 // weapon flash added at this distance
	fog              int32 // fog blend factor
	keep             int32 // 256 - fog
	fogR, fogG, fogB int32 // fog colour * fog
}

// flashRange is how far the weapon flash reaches.
const flashRange = 6.0

// prepareLighting fills the frame's lighting tables for this frame.
func (rc *Raycaster) prepareLighting(f *frame) {
	m := rc.Map
	f.fogOn = rc.Fog.Density > 0
	mapLit := m.Light != nil
	if rc.Sectors != nil {
		mapLit = rc.sectorsLit()
	}
	f.lit = mapLit || len(rc.Lights) > 0 || rc.Flash > 0 || f.fogOn
	if !f.lit {
		return
	}
	f.fogC = [3]int32{int32(rc.Fog.Color[0]), int32(rc.Fog.Color[1]), int32(rc.Fog.Color[2])}
	for i := range f.lut {
		d := float64(i) / shadeRes
		e := &f.lut[i]
		e.dist = shadeLUT[i]
		if rc.Flash > 0 && d < flashRange {
			e.flash = int32(rc.Flash * 256 * (1 - d/flashRange))
		}
		if f.fogOn {
			e.fog = int32(256 * (1 - math.Exp(-rc.Fog.Density*d)))
		}
		e.keep = 256 - e.fog
		e.fogR, e.fogG, e.fogB = f.fogC[0]*e.fog, f.fogC[1]*e.fog, f.fogC[2]*e.fog
	}
	if rc.Sectors != nil || (m.Light == nil && len(rc.Lights) == 0) {
		return
	}
	n := m.Width * m.Height
	if cap(rc.cellLight) < n {
		rc.cellLight = make([]int32, n)
	}
	cl := rc.cellLight[:n]
	for i := range cl {
		cl[i] = 256
		if m.Light != nil {
			cl[i] = int32(m.Light[i] * 256)
		}
	}
	for _, l := range rc.Lights {
		for _, c := range LightFootprint(m, l.X, l.Y, l.Radius) {
			cl[c.Index] += int32(l.Level * c.Weight * 256)
		}
	}
	for i := range cl {
		cl[i] = min(max(cl[i], 0), 768)
	}
	f.cellLight = cl
}

// light returns the light of cell (x, y) in 8.8 fixed point.
func (f *frame) light(m *MapData, x, y int) int32 {
	if f.cellLight == nil || !m.inside(x, y) {
		return 256
	}
	return f.cellLight[y*m.Width+x]
}

// lutIndex maps a distance to the lighting tables.
func lutIndex(d float64) int {
	i := int(d * shadeRes)
	if i >= len(shadeLUT) {
		return len(shadeLUT) - 1
	}
	return max(i, 0)
}

// shadeFor combines cell light L, the distance table entry and the plane
// factor k (256 = 1) into a colour multiplier and a fog factor.
func (f *frame) shadeFor(L int32, i int, k int32) (s, fz int32) {
	e := &f.lut[i]
	return (L*e.dist>>8)*k>>8 + e.flash, e.fog
}

// put writes one shaded, fogged pixel (r, g, b are texel values).
func (f *frame) put(dst []uint8, r, g, b, s, fz int32) {
	r, g, b = r*s>>8, g*s>>8, b*s>>8
	if fz > 0 {
		r = (r*(256-fz) + f.fogC[0]*fz) >> 8
		g = (g*(256-fz) + f.fogC[1]*fz) >> 8
		b = (b*(256-fz) + f.fogC[2]*fz) >> 8
	}
	dst[0], dst[1], dst[2], dst[3] = uint8(min(r, 255)), uint8(min(g, 255)), uint8(min(b, 255)), 255
}
