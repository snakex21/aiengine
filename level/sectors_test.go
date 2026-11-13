package level

import (
	"strings"
	"testing"

	"aiengine/nav"
)

const sectorMapSrc = `MAP pokaz
PLAYER_START 2 2
PLAYER_ANGLE 0
LIGHT_AMBIENT 0.5
FLOOR_TEX floor
SECTOR hala floor=0 ceil=2 wall=W : 0 0 4 0 4 1 4 3 4 4 0 4
SECTOR skos floor=0.25 ceil=1.5 light=0.9 wall=S : 4 1 7 0.5 7 3.5 4 3
PLACE light pulse blask 2 1 2 radius=3 level=0.5 speed=1
PLACE item crystal k1 6 0 2
`

func TestLoadSectorMap(t *testing.T) {
	lv := loadSrc(t, sectorMapSrc)
	m := lv.Sectors
	if m == nil || len(m.Sectors) != 2 {
		t.Fatal("sector map not built")
	}
	if lv.SpawnX != 2 || lv.SpawnY != 2 {
		t.Errorf("spawn %v,%v", lv.SpawnX, lv.SpawnY)
	}
	if m.Sectors[0].FloorTex != "floor" || m.Sectors[1].Light < 0.9 {
		t.Errorf("defaults: %+v / %+v", m.Sectors[0], m.Sectors[1])
	}
	// The sampled grid: hall cells open at 0, side room cells at 0.25.
	if lv.Solid(1, 1) || lv.Solid(5, 2) || !lv.Solid(5, 0) || lv.Map.FloorAt(5, 2) != 0.25 {
		t.Error("rasterized grid does not follow the sectors")
	}
	if nav.FindPath(lv, nav.Cell{X: 1, Y: 1}, nav.Cell{X: 6, Y: 2}, 0) == nil {
		t.Error("enemies must find a path into the side room")
	}
}

func TestSectorLightsAnimate(t *testing.T) {
	lv := loadSrc(t, sectorMapSrc)
	hall := &lv.Sectors.Sectors[0]
	lv.UpdateLights(0.25) // pulse peak
	hi := hall.Light
	lv.UpdateLights(0.75) // pulse low
	lo := hall.Light
	if hi <= lo || lo < 0.5 {
		t.Fatalf("hall light should pulse above its base 0.5: %v..%v", lo, hi)
	}
	lv.SetLightEffect("blask", LightOff)
	lv.UpdateLights(0)
	if hall.Light != 0.5 {
		t.Fatalf("light off: base light expected, got %v", hall.Light)
	}
}

func TestSectorMapErrors(t *testing.T) {
	for name, src := range map[string]string{
		"no start":    "MAP x\nSECTOR a : 0 0 1 0 1 1\n",
		"start out":   "MAP x\nPLAYER_START 5 5\nSECTOR a : 0 0 1 0 1 1\n",
		"bad sector":  "MAP x\nPLAYER_START 0.5 0.2\nSECTOR a : 0 0 1 0\n",
		"no geometry": "MAP x\nPLAYER_START 1 1\n",
	} {
		if _, err := FromDocumentString(src); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestGridToSectorsConversion(t *testing.T) {
	out, err := GridToSectors(stairsMap + "GRID_END_MARKER_IGNORED 1\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "FLOOR_GRID") || !strings.Contains(out, "SECTOR c1_1 ") || !strings.Contains(out, "PLAYER_START 1.5 1.5") {
		t.Fatalf("converted map:\n%s", out)
	}
	conv := loadSrc(t, out)
	orig := loadSrc(t, stairsMap)
	for y := 0; y < orig.Map.Height; y++ {
		for x := 0; x < orig.Map.Width; x++ {
			if orig.Solid(x, y) != conv.Solid(x, y) {
				t.Fatalf("cell %d,%d: solid %v vs %v", x, y, orig.Solid(x, y), conv.Solid(x, y))
			}
			if !orig.Solid(x, y) && (orig.FloorAt(x, y) != conv.FloorAt(x, y) || orig.CeilAt(x, y) != conv.CeilAt(x, y)) {
				t.Fatalf("cell %d,%d heights differ", x, y)
			}
		}
	}
	if conv.Sectors == nil {
		t.Fatal("converted map must load as a sector map")
	}
}
