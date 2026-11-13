package level

import (
	"math"
	"testing"

	"aiengine/scripting"
)

const lightMap = `MAP l
LIGHT_AMBIENT 0.3
FOG yes color=#102030 density=0.25
GRID
W W W W W W W
W P . W . . W
W . . W . . W
W W W W W W W
LIGHT_GRID
. . . . . . .
. 9 . . . 0 .
. . 4 . . . .
PLACE light flicker torch 1.5 0.8 1.5 radius=2 level=0.5 speed=8 min=0.1
PLACE light pulse glow 4.5 0.5 2.5 radius=1.5 level=1 speed=1
LIGHT_POINT 5.5 0.5 1.5 radius=1
`

func TestLightGridAmbientAndFog(t *testing.T) {
	lv := loadSrc(t, lightMap)
	if lv.baseLight[1*7+1] != 1 || lv.baseLight[2*7+2] != 4.0/9 || lv.baseLight[1*7+5] != 0 || lv.baseLight[1*7+2] != 0.3 {
		t.Fatalf("base light wrong: %v", lv.baseLight)
	}
	f := lv.Fog()
	if f.Density != 0.25 || f.Color != [3]uint8{0x10, 0x20, 0x30} {
		t.Fatalf("fog %+v", f)
	}
	if len(lv.Lights) != 3 || lv.LightByID("torch") == nil || lv.LightByID("light_point_3") == nil {
		t.Fatalf("lights: %+v", lv.Lights)
	}
}

func TestLightsDoNotPassWalls(t *testing.T) {
	lv := loadSrc(t, lightMap)
	lv.SetLightEffect("torch", LightSteady)
	lv.UpdateLights(0)
	// The torch room (x 1..2) is brighter than ambient next to the torch;
	// the room behind the wall (x 4..5) gets nothing from the torch.
	if lv.Map.LightAt(2, 1) <= 0.3 {
		t.Error("cell next to the torch must be brighter than ambient")
	}
	lv.SetLightEffect("glow", LightOff)
	lv.SetLightEffect("light_point_3", LightOff)
	lv.UpdateLights(0)
	if got := lv.Map.LightAt(4, 1); got != 0.3 {
		t.Errorf("cell behind the wall lit by the torch: %v", got)
	}
}

func TestLightEffects(t *testing.T) {
	l := &Light{Effect: LightPulse, Speed: 1, Min: 0.2}
	if math.Abs(l.Factor(0.25)-1) > 1e-9 || math.Abs(l.Factor(0.75)-0.2) > 1e-9 {
		t.Errorf("pulse peaks %v / %v", l.Factor(0.25), l.Factor(0.75))
	}
	l = &Light{Effect: LightStrobe, Speed: 2, Min: 0}
	if l.Factor(0.1) != 1 || l.Factor(0.3) != 0 {
		t.Error("strobe must alternate on and off")
	}
	l = &Light{Effect: LightFlicker, Speed: 10, Min: 0.1, seed: 7}
	low, high, same := 1.0, 0.0, true
	for i := 0; i < 200; i++ {
		v := l.Factor(float64(i) / 10)
		low, high = math.Min(low, v), math.Max(high, v)
		if v != l.Factor(float64(i)/10) {
			same = false
		}
	}
	if !same || low > 0.5 || high < 0.9 || low < 0.1 {
		t.Errorf("flicker should be deterministic, mostly bright with dips: %v..%v", low, high)
	}
	if (&Light{Effect: LightOff}).Factor(1) != 0 || (&Light{Effect: LightSteady}).Factor(1) != 1 {
		t.Error("off/steady")
	}
}

func TestScriptsControlLights(t *testing.T) {
	lv := loadSrc(t, lightMap)
	eng := scripting.NewEngine()
	s := NewSession(lv, eng)
	if !eng.SetLight("torch", -1, "off") || !eng.SetLight("glow", -1, "off") || !eng.SetLight("light_point_3", 0, "") {
		t.Fatal("setLight on known lights must succeed")
	}
	s.Update(0.1, 1.5, 1.5)
	if got := lv.Map.LightAt(4, 2); got != 0.3 {
		t.Errorf("all lights off: ambient expected, got %v", got)
	}
	if eng.SetLight("nope", 1, "") || eng.SetLight("torch", -1, "disco") {
		t.Error("unknown light or effect must fail")
	}
	eng.SetCellLight(4, 2, 0.9)
	if got := lv.Map.LightAt(4, 2); got != 0.9 {
		t.Errorf("setCellLight: %v", got)
	}
}

func TestUnlitMapStaysUnlit(t *testing.T) {
	lv := arena(t)
	if lv.Map.Light != nil || lv.Fog().Density != 0 {
		t.Fatal("maps without lighting must stay fully lit (renderer fast path)")
	}
}

func TestLuaLightAPI(t *testing.T) {
	lv := loadSrc(t, lightMap)
	eng := scripting.NewEngine()
	NewSession(lv, eng)
	lb := scripting.NewLuaBridge(eng)
	defer lb.Close()
	if err := lb.DoString(`ok1 = setLightEffect("glow", "strobe"); ok2 = setLight("torch", 0.1); ok3 = setCellLight(1, 2, 0)`); err != nil {
		t.Fatal(err)
	}
	if lv.LightByID("glow").Effect != LightStrobe || lv.LightByID("torch").Level != 0.1 || lv.baseLight[2*7+1] != 0 {
		t.Fatal("Lua light calls not applied")
	}
}
