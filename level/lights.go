package level

// Lighting of GRID maps (optional; maps without it are fully lit):
//
//	LIGHT_AMBIENT 0.6                    default cell light (1 = full)
//	FOG yes color=#000000 density=0.2    distance fog (factor 1-exp(-density*d))
//	LIGHT_GRID                           one symbol per GRID cell:
//	. . 2 2 9                            '.' = LIGHT_AMBIENT, '0'-'9' = 0..1 (n/9)
//
//	PLACE light <effect> <id> x h z radius=3 level=0.8 speed=6 min=0.2
//	LIGHT_POINT x h z radius=4           (steady light, older syntax)
//
// A light adds level*falloff to the cells it can see within radius. Effects:
// steady, flicker (random dips to min), pulse (smooth between min and 1),
// strobe (on/off), off. speed is changes per second. Scripts change lights
// with setLight(id, level), setLightEffect(id, effect) and
// setCellLight(x, y, level).

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"aiengine/parser"
	"aiengine/renderer"
)

// Light effects.
const (
	LightSteady  = "steady"
	LightFlicker = "flicker"
	LightPulse   = "pulse"
	LightStrobe  = "strobe"
	LightOff     = "off"
)

// Light is a placed light source.
type Light struct {
	ID     string
	X, Y   float64
	Radius float64
	Level  float64
	Effect string
	Speed  float64
	Min    float64 // lowest factor for flicker/pulse

	cells []renderer.LitCell
	seed  uint32
}

// Factor is the light's brightness factor (0..1) at time t.
func (l *Light) Factor(t float64) float64 {
	switch l.Effect {
	case LightOff:
		return 0
	case LightPulse:
		return l.Min + (1-l.Min)*(0.5+0.5*math.Sin(2*math.Pi*t*l.Speed))
	case LightStrobe:
		if math.Mod(t*l.Speed, 1) < 0.5 {
			return 1
		}
		return l.Min
	case LightFlicker:
		// Deterministic noise: a new random value speed times per second;
		// mostly bright with occasional dips (torch-like).
		n := hash32(uint32(int64(math.Floor(t*l.Speed))) ^ l.seed)
		v := float64(n%1000) / 1000
		if v < 0.65 {
			return 1 - 0.15*v
		}
		return l.Min + (1-l.Min)*(v-0.65)/0.35*0.6
	}
	return 1
}

func hash32(x uint32) uint32 {
	x ^= x >> 16
	x *= 0x7feb352d
	x ^= x >> 15
	x *= 0x846ca68b
	x ^= x >> 16
	return x
}

func (lv *Level) buildLights(mapNode *parser.AIENode) error {
	m := lv.Map
	ambient := lv.propFloat("LIGHT_AMBIENT", 1)
	lg := mapNode.FindChild("LIGHT_GRID")
	for _, c := range mapNode.FindChildren("LIGHT_POINT") {
		p, err := parsePlace("light steady "+c.Name, c.Line)
		if err != nil {
			return err
		}
		lv.Placements = append(lv.Placements, p)
	}
	var lights []*Light
	for _, p := range lv.Placements {
		if p.Kind != "light" {
			continue
		}
		effect := strings.ToLower(p.Type)
		switch effect {
		case "", "point", "light":
			effect = LightSteady
		case LightSteady, LightFlicker, LightPulse, LightStrobe, LightOff:
		default:
			return fmt.Errorf("line %d: unknown light effect %q (steady, flicker, pulse, strobe, off)", p.Line, p.Type)
		}
		id := p.ID
		if id == "" {
			id = fmt.Sprintf("light_point_%d", len(lights)+1)
		}
		l := &Light{ID: id, X: p.X, Y: p.Y, Radius: p.Float("radius", 3), Level: p.Float("level", 0.8),
			Effect: effect, Speed: p.Float("speed", 6), Min: p.Float("min", 0.2), seed: hash32(uint32(len(lights) + 1))}
		lights = append(lights, l)
	}
	if lv.Sectors != nil {
		lv.Lights = lights
		lv.buildSectorLights(lights)
		lv.UpdateLights(0)
		return nil
	}
	if lg == nil && ambient == 1 && len(lights) == 0 {
		return nil // fully lit classic map
	}
	lv.baseLight = make([]float64, m.Width*m.Height)
	for i := range lv.baseLight {
		lv.baseLight[i] = ambient
	}
	if lg != nil {
		for y, row := range lg.Values {
			if y >= m.Height {
				return fmt.Errorf("line %d: LIGHT_GRID has more rows than GRID", lg.Line)
			}
			for x, sym := range strings.Fields(row) {
				if x >= m.Width {
					return fmt.Errorf("line %d: LIGHT_GRID row %d is wider than GRID", lg.Line, y+1)
				}
				if sym == "." {
					continue
				}
				if len(sym) != 1 || sym[0] < '0' || sym[0] > '9' {
					return fmt.Errorf("line %d: LIGHT_GRID row %d: unknown light symbol %q (0-9 or .)", lg.Line, y+1, sym)
				}
				lv.baseLight[y*m.Width+x] = float64(sym[0]-'0') / 9
			}
		}
	}
	for _, l := range lights {
		l.cells = renderer.LightFootprint(m, l.X, l.Y, l.Radius)
	}
	lv.Lights = lights
	m.Light = make([]float64, len(lv.baseLight))
	lv.UpdateLights(0)
	return nil
}

// UpdateLights recomputes the map's cell lights at time t (base light plus
// every light's current contribution). Call once per frame.
func (lv *Level) UpdateLights(t float64) {
	if lv.Sectors != nil {
		lv.updateSectorLights(t)
		return
	}
	if lv.baseLight == nil {
		return
	}
	copy(lv.Map.Light, lv.baseLight)
	for _, l := range lv.Lights {
		k := l.Level * l.Factor(t)
		if k == 0 {
			continue
		}
		for _, c := range l.cells {
			lv.Map.Light[c.Index] += k * c.Weight
		}
	}
}

// LightByID returns a placed light.
func (lv *Level) LightByID(id string) *Light {
	for _, l := range lv.Lights {
		if l.ID == id {
			return l
		}
	}
	return nil
}

// SetLightLevel changes a light's level (false for an unknown id).
func (lv *Level) SetLightLevel(id string, level float64) bool {
	l := lv.LightByID(id)
	if l == nil {
		return false
	}
	l.Level = level
	return true
}

// SetLightEffect changes a light's effect (false for an unknown id/effect).
func (lv *Level) SetLightEffect(id, effect string) bool {
	l := lv.LightByID(id)
	switch effect = strings.ToLower(effect); effect {
	case LightSteady, LightFlicker, LightPulse, LightStrobe, LightOff:
	default:
		return false
	}
	if l == nil {
		return false
	}
	l.Effect = effect
	return true
}

// SetCellLight changes the base light of a cell (enabling lighting on a
// fully lit map).
func (lv *Level) SetCellLight(x, y int, level float64) bool {
	m := lv.Map
	if x < 0 || y < 0 || x >= m.Width || y >= m.Height {
		return false
	}
	if lv.baseLight == nil {
		lv.baseLight = make([]float64, m.Width*m.Height)
		for i := range lv.baseLight {
			lv.baseLight[i] = 1
		}
		m.Light = make([]float64, len(lv.baseLight))
	}
	lv.baseLight[y*m.Width+x] = level
	return true
}

// Fog returns the map's fog (FOG yes color=#rrggbb density=0.2, or
// FOG_DENSITY / FOG_COLOR); zero density = no fog.
func (lv *Level) Fog() renderer.Fog {
	var f renderer.Fog
	spec := strings.Fields(lv.Props["FOG"])
	on := len(spec) > 0 && isYes(spec[0])
	density, color := "0.2", "#000000"
	for _, kv := range spec {
		if k, v, ok := strings.Cut(kv, "="); ok {
			switch strings.ToLower(k) {
			case "density":
				density = v
			case "color", "colour":
				color = v
			}
		}
	}
	if v, ok := lv.Props["FOG_DENSITY"]; ok {
		on, density = true, v
	}
	if v, ok := lv.Props["FOG_COLOR"]; ok {
		color = v
	}
	if !on {
		return f
	}
	f.Density, _ = strconv.ParseFloat(density, 64)
	c := strings.TrimPrefix(strings.TrimSpace(color), "#")
	if v, err := strconv.ParseUint(c, 16, 32); err == nil && len(c) == 6 {
		f.Color = [3]uint8{uint8(v >> 16), uint8(v >> 8), uint8(v)}
	}
	return f
}
