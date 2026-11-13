package sector

import (
	"fmt"
	"strconv"
	"strings"
)

// Grid is what FromGrid needs from a tile map.
type Grid interface {
	Size() (w, h int)
	Open(x, y int) bool
	Floor(x, y int) float64
	Ceil(x, y int) float64
	Light(x, y int) float64
	WallTex(x, y int) string // texture of the solid cell (x, y)
	StepTex(x, y int) string // texture of the step faces of open cell (x, y)
}

// FromGrid converts a tile map into a sector map: every open cell becomes a
// square sector with the cell's heights and light; walls between an open
// and a solid cell take the solid cell's texture. Rendering the result with
// the sector renderer matches the grid renderer (tested), so it is also the
// reference for checking the sector renderer.
func FromGrid(g Grid, floorTex, ceilTex string) (*Map, error) {
	w, h := g.Size()
	var defs []Def
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !g.Open(x, y) {
				continue
			}
			fx, fy := float64(x), float64(y)
			d := Def{Sector: Sector{
				ID:    fmt.Sprintf("c%d_%d", x, y),
				Floor: g.Floor(x, y), Ceil: g.Ceil(x, y), Light: g.Light(x, y),
				FloorTex: floorTex, CeilTex: ceilTex, StepTex: g.StepTex(x, y),
				Poly: []Vec2{{fx, fy}, {fx + 1, fy}, {fx + 1, fy + 1}, {fx, fy + 1}},
			}}
			// Edges: top, right, bottom, left - each faces that neighbour.
			for _, n := range [4][2]int{{x, y - 1}, {x + 1, y}, {x, y + 1}, {x - 1, y}} {
				tex := ""
				if !inGrid(n[0], n[1], w, h) || !g.Open(n[0], n[1]) {
					tex = g.WallTex(n[0], n[1])
				}
				d.EdgeTex = append(d.EdgeTex, tex)
			}
			defs = append(defs, d)
		}
	}
	if len(defs) == 0 {
		return nil, fmt.Errorf("grid has no open cells")
	}
	return Build(defs)
}

func inGrid(x, y, w, h int) bool { return x >= 0 && y >= 0 && x < w && y < h }

// ParseDef parses one SECTOR line of a map file:
//
//	SECTOR <id> floor=0 ceil=1.5 light=0.8 wall=W step=S floor_tex=floor ceil_tex=ceiling tag=t : x y x y x y ...
//
// The points (at least three pairs) follow the colon; wall= is the texture
// of the sector's solid edges (W, S, M, D or a texture name).
func ParseDef(spec string) (Def, error) {
	d := Def{Sector: Sector{Ceil: 1, Light: 1}}
	head, pts, ok := strings.Cut(spec, ":")
	if !ok {
		return d, fmt.Errorf("SECTOR %q: missing ':' before the points", spec)
	}
	for _, f := range strings.Fields(head) {
		k, v, kv := strings.Cut(f, "=")
		if !kv {
			if d.ID != "" {
				return d, fmt.Errorf("SECTOR %s: unexpected %q", d.ID, f)
			}
			d.ID = f
			continue
		}
		num := func() (float64, error) { return strconv.ParseFloat(v, 64) }
		var err error
		switch strings.ToLower(k) {
		case "floor":
			d.Floor, err = num()
		case "ceil", "ceiling":
			d.Ceil, err = num()
		case "light":
			d.Light, err = num()
		case "wall":
			d.WallTex = v
		case "step":
			d.StepTex = v
		case "floor_tex":
			d.FloorTex = v
		case "ceil_tex", "ceiling_tex":
			d.CeilTex = v
		case "tag":
			d.Tag = v
		default:
			err = fmt.Errorf("unknown key %q", k)
		}
		if err != nil {
			return d, fmt.Errorf("SECTOR %s: %s: %w", d.ID, k, err)
		}
	}
	nums := strings.Fields(pts)
	if len(nums)%2 != 0 || len(nums) < 6 {
		return d, fmt.Errorf("SECTOR %s: need x y pairs (at least 3 points), got %d numbers", d.ID, len(nums))
	}
	for i := 0; i < len(nums); i += 2 {
		x, err1 := strconv.ParseFloat(nums[i], 64)
		y, err2 := strconv.ParseFloat(nums[i+1], 64)
		if err1 != nil || err2 != nil {
			return d, fmt.Errorf("SECTOR %s: bad point %q %q", d.ID, nums[i], nums[i+1])
		}
		d.Poly = append(d.Poly, Vec2{x, y})
	}
	return d, nil
}
