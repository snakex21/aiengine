package level

import (
	"math"
	"strings"
	"testing"

	"aiengine/nav"
	"aiengine/parser"
)

func brainLevel(t *testing.T) *Level {
	t.Helper()
	src := `MAP b
GRID
W W W W W W W W
W P . . . . . W
W . W W W W . W
W . . . . . . W
W W W W W W W W
`
	doc, _ := parser.ParseAIE(strings.NewReader(src))
	lv, err := FromDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	return lv
}

func TestBrainChaseRemembersLastSeen(t *testing.T) {
	lv := brainLevel(t)
	b := NewBrain("", 6.5, 3.5, 1)
	tx, ty, move := b.Think(lv, 6.5, 3.5, 1.5, 3.5, 0.1, 6, 5, true)
	if !move || tx != 1.5 || ty != 3.5 {
		t.Fatalf("chase target %v,%v move=%v", tx, ty, move)
	}
	// Player disappears: keeps going to the last seen spot for Memory seconds.
	tx, _, move = b.Think(lv, 5.5, 3.5, 1.5, 1.5, 1, 6, 5, false)
	if !move || tx != 1.5 {
		t.Fatal("did not search the last seen position")
	}
	for i := 0; i < 5; i++ {
		_, _, move = b.Think(lv, 5.5, 3.5, 1.5, 1.5, 1, 6, 5, false)
	}
	if move {
		t.Fatal("chaser should give up after Memory seconds")
	}
}

func TestBrainGuardReturnsHome(t *testing.T) {
	lv := brainLevel(t)
	b := NewBrain(BehaviorGuard, 6.5, 1.5, 1)
	b.Think(lv, 6.5, 1.5, 4.5, 1.5, 0.1, 6, 2, true)
	for i := 0; i < 6; i++ {
		b.Think(lv, 4.5, 1.5, 1.5, 3.5, 1, 6, 4, false)
	}
	tx, ty, move := b.Think(lv, 4.5, 1.5, 1.5, 3.5, 0.1, 6, 4, false)
	if !move || tx != 6.5 || ty != 1.5 {
		t.Fatalf("guard target %v,%v move=%v, want home", tx, ty, move)
	}
}

func TestBrainWanderPicksReachableFreeCells(t *testing.T) {
	lv := brainLevel(t)
	b := NewBrain(BehaviorWander, 3.5, 3.5, 42)
	for i := 0; i < 30; i++ {
		tx, ty, move := b.Think(lv, 3.5, 3.5, 0, 0, 3.5, 6, 99, false)
		c := nav.CellAt(tx, ty)
		if !move || lv.Solid(c.X, c.Y) {
			t.Fatalf("wander target %v,%v solid or no move", tx, ty)
		}
	}
}

func TestBrainAmbushWaitsForCloseRange(t *testing.T) {
	lv := brainLevel(t)
	b := NewBrain(BehaviorAmbush, 6.5, 3.5, 1)
	if _, _, move := b.Think(lv, 6.5, 3.5, 2.5, 3.5, 0.1, 6, 4, true); move {
		t.Fatal("ambusher moved while the player was far")
	}
	if _, _, move := b.Think(lv, 6.5, 3.5, 4.5, 3.5, 0.1, 6, 2, true); !move {
		t.Fatal("ambusher did not wake up at close range")
	}
	if math.IsNaN(b.HomeX) {
		t.Fatal("unreachable")
	}
}
