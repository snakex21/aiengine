package level

import (
	"math"
	"strings"
	"testing"

	"aiengine/nav"
	"aiengine/parser"
)

func arena(t *testing.T) *Level {
	t.Helper()
	doc, _ := parser.ParseAIE(strings.NewReader(`MAP a
GRID
W W W W W W W W
W P . . . . . W
W . . . . . . W
W . . . . . . W
W W W W W W W W
`))
	lv, err := FromDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	return lv
}

func wallBlocked(lv *Level) func(x, y float64) bool {
	return func(x, y float64) bool {
		for _, o := range [][2]float64{{0, 0}, {0.15, 0}, {-0.15, 0}, {0, 0.15}, {0, -0.15}} {
			c := nav.CellAt(x+o[0], y+o[1])
			if lv.Solid(c.X, c.Y) {
				return true
			}
		}
		return false
	}
}

func run(f *Fighter, lv *Level, px, py, seconds float64) (damage float64, minDist float64) {
	minDist = math.Inf(1)
	for t := 0.0; t < seconds; t += 0.02 {
		damage += f.Step(lv, px, py, 0.02, wallBlocked(lv))
		minDist = math.Min(minDist, math.Hypot(px-f.X, py-f.Y))
	}
	return
}

func TestFighterWalksUpAndHitsPlayerHuggingAWall(t *testing.T) {
	lv := arena(t)
	// Small reach (like the spiders' range=0.7) and the player pressed into
	// the top-right corner: the old logic stopped at the cell centre, out of
	// reach, and never attacked.
	f := NewFighter(Placement{ID: "s", X: 1.5, Y: 3.5, Params: map[string]string{
		"speed": "2.2", "damage": "6", "range": "0.7", "attack_rate": "0.7", "behavior": "wander"}}, 1)
	dmg, minDist := run(f, lv, 6.84, 1.16, 6)
	if dmg < 6*4 {
		t.Fatalf("damage %.0f in 6 s, want several hits", dmg)
	}
	if minDist < Separation-0.01 {
		t.Fatalf("enemy overlapped the player: %.2f < %.2f", minDist, Separation)
	}
}

func TestFighterWindupCooldownAndEscape(t *testing.T) {
	lv := arena(t)
	f := NewFighter(Placement{ID: "g", X: 3.5, Y: 2.5, Params: map[string]string{"damage": "10", "attack_rate": "1", "windup": "0.3"}}, 1)
	// Player right next to it: first hit after the wind-up, then one per second.
	dmg, _ := run(f, lv, 4.1, 2.5, 2.35)
	if dmg != 30 { // hits at ~0.3 s, ~1.6 s ... (wind-up + rate) -> 2 or 3
		if dmg != 20 {
			t.Fatalf("damage after 2.35 s = %.0f", dmg)
		}
	}
	// Player steps out during the wind-up: no damage.
	f2 := NewFighter(Placement{ID: "g2", X: 3.5, Y: 2.5, Params: map[string]string{"damage": "10", "windup": "0.3", "speed": "0.1"}}, 1)
	f2.Step(lv, 4.1, 2.5, 0.02, nil)
	if f2.State != "windup" {
		t.Fatalf("state %s, want windup", f2.State)
	}
	if d := f2.Step(lv, 6.5, 2.5, 0.5, nil); d != 0 {
		t.Fatal("hit landed although the player escaped")
	}
}

func TestFighterRangeNeverBelowSeparation(t *testing.T) {
	f := NewFighter(Placement{Params: map[string]string{"range": "0.1"}}, 1)
	if f.Range < Separation+minReach {
		t.Fatalf("range %.2f", f.Range)
	}
}

func TestFighterNoLineOfSightNoHit(t *testing.T) {
	doc, _ := parser.ParseAIE(strings.NewReader("MAP b\nGRID\nW W W W W\nW P W . W\nW W W W W\n"))
	lv, _ := FromDocument(doc)
	f := NewFighter(Placement{ID: "x", X: 3.5, Y: 1.5, Params: map[string]string{"range": "2.5"}}, 1)
	if d, _ := run(f, lv, 1.5, 1.5, 3); d != 0 {
		t.Fatal("hit through a wall")
	}
}
