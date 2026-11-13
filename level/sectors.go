package level

// Sector maps (optional instead of GRID): the MAP lists SECTOR lines, each
// a polygon with its own heights, light and textures (see package sector):
//
//	MAP pokaz
//	PLAYER_START 2.5 3.5          # spawn (x z); PLAYER_ANGLE as usual
//	FLOOR_TEX floor               # default floor/ceiling textures of sectors
//	CEILING_TEX ceiling
//	LIGHT_AMBIENT 0.6             # light of sectors without light=
//	SECTOR hala floor=0 ceil=2 wall=W : 0 0 8 0 8 6 0 6
//	SECTOR schody1 floor=0.25 ceil=2 : 8 2 9 2 9 4 8 4
//
// Edges shared by two sectors (same end points) are open passages; the
// others are walls (wall= texture, or edges=T,.,T,... per edge). PLACE,
// TRIGGER, SOUND and lights work as on grid maps (x z are world
// coordinates). The engine renders sector maps with the sector renderer and
// collides against their lines; enemies path-find on a 1-unit grid sampled
// from the sectors. Doors (PLACE door) need GRID maps for now.

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"aiengine/parser"
	"aiengine/renderer"
	"aiengine/sector"
)

// sectorLight is one light's contribution to a sector.
type sectorLight struct {
	sector int
	weight float64
}

func (lv *Level) buildSectors(nodes []*parser.AIENode) error {
	ambient := lv.propFloat("LIGHT_AMBIENT", 1)
	var defs []sector.Def
	for _, n := range nodes {
		d, err := sector.ParseDef(n.Name)
		if err != nil {
			return fmt.Errorf("line %d: %w", n.Line, err)
		}
		if d.ID == "" {
			d.ID = fmt.Sprintf("sector_%d", len(defs)+1)
		}
		if d.Light < 0 {
			d.Light = ambient
		}
		if d.FloorTex == "" {
			d.FloorTex = lv.Props["FLOOR_TEX"]
		}
		if d.CeilTex == "" {
			d.CeilTex = lv.Props["CEILING_TEX"]
		}
		for _, p := range d.Poly {
			if p.X < 0 || p.Y < 0 {
				return fmt.Errorf("line %d: SECTOR %s: coordinates must not be negative", n.Line, d.ID)
			}
		}
		defs = append(defs, d)
	}
	m, err := sector.Build(defs)
	if err != nil {
		return err
	}
	lv.Sectors = m
	start := strings.Fields(lv.Props["PLAYER_START"])
	if len(start) < 2 {
		return fmt.Errorf("sector map needs PLAYER_START x z")
	}
	lv.SpawnX, _ = strconv.ParseFloat(start[0], 64)
	lv.SpawnY, _ = strconv.ParseFloat(start[1], 64)
	if m.SectorAt(lv.SpawnX, lv.SpawnY) < 0 {
		return fmt.Errorf("PLAYER_START %.2f %.2f is outside every sector", lv.SpawnX, lv.SpawnY)
	}
	lv.Map = Rasterize(m)
	return nil
}

// Rasterize samples a sector map on a 1-unit grid (cell centres): cells
// inside a sector are open with its heights and light, the rest are walls.
// It backs the grid-based parts of the runtime (enemy pathfinding, triggers,
// interact) for sector maps.
func Rasterize(m *sector.Map) *renderer.MapData {
	w, h := int(math.Ceil(m.Max.X)), int(math.Ceil(m.Max.Y))
	cells := make([][]renderer.MapCell, h)
	for y := range cells {
		cells[y] = make([]renderer.MapCell, w)
	}
	md := renderer.NewMapData(cells)
	md.SetHeights(renderer.DefaultCeiling)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s := m.SectorAt(float64(x)+0.5, float64(y)+0.5)
			if s < 0 {
				cells[y][x] = renderer.CellWall1
				continue
			}
			md.SetFloor(x, y, m.Sectors[s].Floor)
			md.SetCeil(x, y, m.Sectors[s].Ceil)
		}
	}
	return md
}

// buildSectorLights computes which sectors each light reaches (centroid
// within radius and in sight of the light) and keeps the sectors' own light
// as the base for UpdateLights.
func (lv *Level) buildSectorLights(lights []*Light) {
	m := lv.Sectors
	lv.sectorBase = make([]float64, len(m.Sectors))
	for i := range m.Sectors {
		lv.sectorBase[i] = m.Sectors[i].Light
	}
	lv.sectorLights = make([][]sectorLight, len(lights))
	for li, l := range lights {
		for s := range m.Sectors {
			c := m.Centroid(s)
			d := math.Hypot(c.X-l.X, c.Y-l.Y)
			if d >= l.Radius || !sightOpen(m, l.X, l.Y, c.X, c.Y) {
				continue
			}
			lv.sectorLights[li] = append(lv.sectorLights[li], sectorLight{s, 1 - d/l.Radius})
		}
	}
}

// sightOpen reports a 2D line crossing no solid wall.
func sightOpen(m *sector.Map, x0, y0, x1, y1 float64) bool {
	p0, p1 := sector.Vec2{X: x0, Y: y0}, sector.Vec2{X: x1, Y: y1}
	for i := range m.Lines {
		if _, _, ok := sector.SegmentHit(p0, p1, &m.Lines[i]); ok && !m.Lines[i].TwoSided() {
			return false
		}
	}
	return true
}

func (lv *Level) updateSectorLights(t float64) {
	m := lv.Sectors
	for i := range m.Sectors {
		m.Sectors[i].Light = lv.sectorBase[i]
	}
	for li, l := range lv.Lights {
		k := l.Level * l.Factor(t)
		if k == 0 || li >= len(lv.sectorLights) {
			continue
		}
		for _, c := range lv.sectorLights[li] {
			m.Sectors[c.sector].Light += k * c.weight
		}
	}
}

// GridToSectors rewrites a GRID map file as a sector map: the GRID and its
// height/light layers become SECTOR lines (one per open cell, see
// sector.GridDefs) and PLAYER_START; everything else is kept. Grid doors
// are not converted (their cells become walls of the neighbours).
func GridToSectors(src string) (string, error) {
	doc, err := parser.ParseAIE(strings.NewReader(src))
	if err != nil {
		return "", err
	}
	lv, err := FromDocument(doc)
	if err != nil {
		return "", err
	}
	if lv.Sectors != nil {
		return "", fmt.Errorf("map is already a sector map")
	}
	var out []string
	skip := false
	inserted := false
	for _, line := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(line)
		upper := strings.ToUpper(strings.Fields(t + " x")[0])
		if parser.IsGridBlock(upper) && len(strings.Fields(t)) == 1 {
			skip = true
			if !inserted {
				inserted = true
				out = append(out, fmt.Sprintf("PLAYER_START %g %g", lv.SpawnX, lv.SpawnY))
				if lv.baseLight != nil { // the cells' own light, not the lights' glow
					lv.Map.Light = lv.baseLight
				}
				for _, d := range sector.GridDefs(renderer.GridSectors(lv.Map)) {
					out = append(out, "SECTOR "+sector.FormatDef(d))
				}
			}
			continue
		}
		if skip {
			fields := strings.Fields(t)
			if len(fields) > 0 && isGridRowLine(fields) {
				continue
			}
			skip = false
		}
		if k := strings.ToUpper(strings.Fields(t + " x")[0]); k == "HEIGHT_STEP" || k == "CEILING_HEIGHT" || k == "STEP_WALL" {
			continue // baked into the sectors
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n"), nil
}

func isGridRowLine(fields []string) bool {
	for _, f := range fields {
		if len([]rune(f)) != 1 {
			return false
		}
	}
	return true
}
