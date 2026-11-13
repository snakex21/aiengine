package level

// Height layers of a GRID map (optional, backwards compatible):
//
//	HEIGHT_STEP 0.125        size of one height unit in wall heights (default 1/8)
//	CEILING_HEIGHT 1         default ceiling (wall heights, default 1)
//	STEP_WALL S              texture of step faces: W, S, M or D (default S)
//	FALL_DAMAGE 40           HP lost per wall height fallen beyond FALL_SAFE
//	FALL_SAFE 0.6            falls up to this height are harmless
//
//	FLOOR_GRID               one symbol per GRID cell, same layout:
//	. . 1 2 3                '.' = 0, '0'-'9' = 0..9 units, 'A'-'Z' = 10..35,
//	. . a b .                'a'-'z' = -1..-26 (pits)
//	CEILING_GRID             absolute ceiling per cell, '.' = CEILING_HEIGHT
//	. . 6 6 G
//
// The player walks up rises of at most renderer.MaxStepUp (0.3, i.e. two
// units of 1/8), needs renderer.HeadroomStand (0.6) under the ceiling to stand
// (less when crouching or prone), and falls down any drop.

import (
	"fmt"
	"strconv"
	"strings"

	"aiengine/nav"
	"aiengine/parser"
	"aiengine/renderer"
)

// HeightUnits decodes a height-grid symbol into units; ok is false for '.'
// (default) and unknown symbols.
func HeightUnits(sym string) (units int, ok bool) {
	if len(sym) != 1 {
		return 0, false
	}
	c := sym[0]
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'A' && c <= 'Z':
		return 10 + int(c-'A'), true
	case c >= 'a' && c <= 'z':
		return -1 - int(c-'a'), true
	}
	return 0, false
}

// HeightSymbol encodes units as a height-grid symbol ('.' for 0).
func HeightSymbol(units int) string {
	switch {
	case units == 0:
		return "."
	case units > 0 && units <= 9:
		return string(rune('0' + units))
	case units >= 10 && units <= 35:
		return string(rune('A' + units - 10))
	case units < 0 && units >= -26:
		return string(rune('a' - 1 - units))
	}
	return "?"
}

func (lv *Level) propFloat(key string, def float64) float64 {
	if v, err := strconv.ParseFloat(strings.TrimSpace(lv.Props[key]), 64); err == nil {
		return v
	}
	return def
}

// HeightStep is the size of one FLOOR_GRID/CEILING_GRID unit.
func (lv *Level) HeightStep() float64 { return lv.propFloat("HEIGHT_STEP", 0.125) }

// FallDamage returns the damage for a fall of the given height (0 when the
// map has no FALL_DAMAGE or the fall is within FALL_SAFE).
func (lv *Level) FallDamage(fall float64) float64 {
	k := lv.propFloat("FALL_DAMAGE", 0)
	safe := lv.propFloat("FALL_SAFE", 0.6)
	if k <= 0 || fall <= safe {
		return 0
	}
	return k * (fall - safe)
}

func (lv *Level) buildHeights(mapNode *parser.AIENode) error {
	fg, cg := mapNode.FindChild("FLOOR_GRID"), mapNode.FindChild("CEILING_GRID")
	m := lv.Map
	if side, ok := lv.Props["STEP_WALL"]; ok {
		cell, err := wallSymbol(strings.TrimSpace(side))
		if err != nil {
			return fmt.Errorf("STEP_WALL: %w", err)
		}
		m.Side = make([]renderer.MapCell, m.Width*m.Height)
		for i := range m.Side {
			m.Side[i] = cell
		}
	}
	ceiling := lv.propFloat("CEILING_HEIGHT", renderer.DefaultCeiling)
	if fg == nil && cg == nil && ceiling == renderer.DefaultCeiling {
		return nil // flat classic map
	}
	step := lv.HeightStep()
	m.SetHeights(ceiling)
	read := func(n *parser.AIENode, dst []float64, def float64) error {
		if n == nil {
			return nil
		}
		for y, row := range n.Values {
			if y >= m.Height {
				return fmt.Errorf("line %d: %s has more rows than GRID", n.Line, n.Type)
			}
			for x, sym := range strings.Fields(row) {
				if x >= m.Width {
					return fmt.Errorf("line %d: %s row %d is wider than GRID", n.Line, n.Type, y+1)
				}
				u, ok := HeightUnits(sym)
				if !ok {
					if sym != "." {
						return fmt.Errorf("line %d: %s row %d: unknown height symbol %q", n.Line, n.Type, y+1, sym)
					}
					dst[y*m.Width+x] = def
					continue
				}
				dst[y*m.Width+x] = float64(u) * step
			}
		}
		return nil
	}
	if err := read(fg, m.Floor, 0); err != nil {
		return err
	}
	if err := read(cg, m.Ceil, ceiling); err != nil {
		return err
	}
	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			if m.GetCell(x, y) == renderer.CellEmpty && m.CeilAt(x, y) < m.FloorAt(x, y) {
				return fmt.Errorf("cell (%d,%d): ceiling %.3g below floor %.3g", x, y, m.CeilAt(x, y), m.FloorAt(x, y))
			}
		}
	}
	return nil
}

func wallSymbol(s string) (renderer.MapCell, error) {
	switch strings.ToUpper(s) {
	case "W":
		return renderer.CellWall1, nil
	case "S":
		return renderer.CellWall2, nil
	case "M":
		return renderer.CellWall3, nil
	case "D":
		return renderer.CellDoor, nil
	}
	return 0, fmt.Errorf("unknown wall symbol %q (W, S, M or D)", s)
}

// FloorAt / CeilAt give cell heights (nav.Heights).
func (lv *Level) FloorAt(x, y int) float64 { return lv.Map.FloorAt(x, y) }
func (lv *Level) CeilAt(x, y int) float64  { return lv.Map.CeilAt(x, y) }

// CanStep reports whether a walker may move from one open cell to a
// neighbouring one (nav.Stepper): rises up to MaxStepUp, any drop, and
// standing headroom in the target cell.
func (lv *Level) CanStep(from, to nav.Cell) bool {
	if lv.Solid(to.X, to.Y) {
		return false
	}
	ft := lv.Map.FloorAt(to.X, to.Y)
	if ft-lv.Map.FloorAt(from.X, from.Y) > renderer.MaxStepUp+1e-9 {
		return false
	}
	return lv.Map.CeilAt(to.X, to.Y)-ft >= renderer.HeadroomStand-1e-9
}
