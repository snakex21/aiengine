package level

// Tags and sliding doors.
//
// Grid maps mark cells for movers with a TAG_GRID layer (one symbol per
// GRID cell, '.' = none). A tag is the symbol itself unless TAGS names it:
//
//	TAGS w=winda p=pulapka
//	TAG_GRID
//	. . w w .
//	. . . p .
//
// Doors (GRID 'D' and PLACE door) slide open like a classic door - their
// ceiling rises from the floor - with PLACE door ... anim=yes, or for all
// doors of a map with DOOR_STYLE slide (anim=no keeps one instant). Keys,
// secrets and scripts work as for instant doors. On sector maps PLACE door
// always slides: it animates the sector under the given point.

import (
	"fmt"
	"sort"
	"strings"

	"aiengine/parser"
	"aiengine/renderer"
)

func (lv *Level) buildTags(mapNode *parser.AIENode) error {
	tg := mapNode.FindChild("TAG_GRID")
	if tg == nil {
		return nil
	}
	names := map[string]string{}
	for _, f := range strings.Fields(lv.Props["TAGS"]) {
		k, v, ok := strings.Cut(f, "=")
		if !ok || len([]rune(k)) != 1 || v == "" {
			return fmt.Errorf("TAGS: expected symbol=name, got %q", f)
		}
		names[k] = v
	}
	m := lv.Map
	lv.cellTags = make([]string, m.Width*m.Height)
	for y, row := range tg.Values {
		if y >= m.Height {
			return fmt.Errorf("line %d: TAG_GRID has more rows than GRID", tg.Line)
		}
		for x, sym := range strings.Fields(row) {
			if x >= m.Width {
				return fmt.Errorf("line %d: TAG_GRID row %d is wider than GRID", tg.Line, y+1)
			}
			if sym == "." {
				continue
			}
			if n, ok := names[sym]; ok {
				sym = n
			}
			lv.cellTags[y*m.Width+x] = sym
		}
	}
	return nil
}

// slides reports whether a door is animated.
func (lv *Level) slides(d *door) bool {
	if lv.Sectors != nil {
		return true
	}
	if v, ok := d.params["anim"]; ok {
		return isYes(v)
	}
	return strings.EqualFold(strings.TrimSpace(lv.Props["DOOR_STYLE"]), "slide")
}

// buildSlidingDoors turns animated doors into door movers.
func (lv *Level) buildSlidingDoors() error {
	ids := make([]string, 0, len(lv.doors))
	for id := range lv.doors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		d := lv.doors[id]
		if !lv.slides(d) {
			continue
		}
		ms := moverSpec{kind: "door", tag: d.ID, params: map[string]string{"wait": "0", "use": "no"}}
		for k, v := range d.params {
			ms.params[k] = v
		}
		ms.params["use"], ms.params["key"], ms.params["locked"] = "no", "", "" // the door logic handles use
		if lv.Sectors != nil {
			s := lv.Sectors.SectorAt(float64(d.X)+0.5, float64(d.Y)+0.5)
			if s < 0 {
				return fmt.Errorf("PLACE door %s: (%d, %d) is outside every sector", d.ID, d.X, d.Y)
			}
			sec := &lv.Sectors.Sectors[s]
			if sec.Tag == "" {
				sec.Tag = d.ID
			}
			ms.tag = sec.Tag
			if !d.Secret {
				sec.StepTex = "D"
			}
			lv.Map.Grid[d.Y][d.X] = renderer.CellEmpty // undo the raster door
		} else {
			lv.ensureGridLayers()
			m := lv.Map
			i := d.Y*m.Width + d.X
			m.Grid[d.Y][d.X] = renderer.CellEmpty
			m.Side[i] = d.Cell
			lv.cellTags[i] = d.ID
		}
		before := len(lv.movers)
		if err := lv.buildMovers([]moverSpec{ms}); err != nil {
			return fmt.Errorf("PLACE door %s: %w", d.ID, err)
		}
		d.mover = lv.movers[before]
		d.Anim = true
	}
	return nil
}
