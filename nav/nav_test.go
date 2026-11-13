package nav

import (
	"math"
	"strings"
	"testing"
)

type strGrid []string

func (g strGrid) Solid(x, y int) bool {
	if y < 0 || y >= len(g) || x < 0 || x >= len(g[y]) {
		return true
	}
	return g[y][x] == '#'
}

func grid(s string) strGrid { return strings.Split(strings.TrimSpace(s), "\n") }

var room = grid(`
##########
#S.......#
#.######.#
#.#....#.#
#.#.G..#.#
#.####.#.#
#........#
##########`)

func find(g strGrid, ch byte) Cell {
	for y, row := range g {
		if x := strings.IndexByte(row, ch); x >= 0 {
			return Cell{x, y}
		}
	}
	panic("missing")
}

func TestFindPathAroundWalls(t *testing.T) {
	s, goal := find(room, 'S'), find(room, 'G')
	p := FindPath(room, s, goal, 0)
	if len(p) == 0 || p[0] != s || p[len(p)-1] != goal {
		t.Fatalf("path %v", p)
	}
	for i, c := range p {
		if room.Solid(c.X, c.Y) {
			t.Fatalf("path goes through wall at %v", c)
		}
		if i > 0 {
			d := Cell{c.X - p[i-1].X, c.Y - p[i-1].Y}
			if d.X != 0 && d.Y != 0 && (room.Solid(p[i-1].X+d.X, p[i-1].Y) || room.Solid(p[i-1].X, p[i-1].Y+d.Y)) {
				t.Fatalf("corner cut between %v and %v", p[i-1], c)
			}
		}
	}
	// The only way in is the gap at the bottom (x=6,y=5).
	through := false
	for _, c := range p {
		if c == (Cell{6, 5}) {
			through = true
		}
	}
	if !through {
		t.Fatalf("path did not use the only entrance: %v", p)
	}
}

func TestFindPathUnreachable(t *testing.T) {
	g := grid(`
#####
#S#G#
#####`)
	if p := FindPath(g, find(g, 'S'), find(g, 'G'), 0); p != nil {
		t.Fatalf("expected no path, got %v", p)
	}
	if p := FindPath(g, find(g, 'S'), Cell{2, 1}, 0); p != nil {
		t.Fatal("goal in a wall must give no path")
	}
}

func TestSteerGoesAroundCornerInsteadOfIntoWall(t *testing.T) {
	g := grid(`
#######
#E....#
#####.#
#P....#
#######`)
	ex, ey := find(g, 'E').Center()
	px, py := find(g, 'P').Center()
	dx, dy, ok := Steer(g, ex, ey, px, py, 0.2)
	if !ok {
		t.Fatal("no steering")
	}
	// Straight at the player is down (blocked); the path leads right first.
	if dx < 0.7 || math.Abs(dy) > 0.7 {
		t.Fatalf("steer (%.2f, %.2f), want mostly +x", dx, dy)
	}
	// Simulate walking: the enemy must reach the player without entering walls.
	x, y := ex, ey
	for i := 0; i < 400 && math.Hypot(px-x, py-y) > 0.3; i++ {
		dx, dy, ok := Steer(g, x, y, px, py, 0.2)
		if !ok {
			t.Fatalf("lost path at %.2f,%.2f", x, y)
		}
		x, y = x+dx*0.05, y+dy*0.05
		if c := CellAt(x, y); g.Solid(c.X, c.Y) {
			t.Fatalf("walked into wall at %.2f,%.2f", x, y)
		}
	}
	if math.Hypot(px-x, py-y) > 0.3 {
		t.Fatalf("did not reach the player, stopped at %.2f,%.2f", x, y)
	}
}

func TestLineClear(t *testing.T) {
	if !LineClear(room, 1.5, 1.5, 8.5, 1.5, 0.2) {
		t.Fatal("open corridor reported blocked")
	}
	if LineClear(room, 1.5, 1.5, 1.5, 6.5, 0.4) == LineClear(room, 1.5, 1.5, 4.5, 4.5, 0.2) {
		// first is clear (column 1 open), second crosses walls
		t.Fatal("LineClear inconsistent")
	}
}

// Regression: near a wall LineClear (with body radius) fails, and when the
// enemy was already in the player's cell FindPath returned a 1-cell path, so
// Steer gave up and the enemy stood still out of attack range.
func TestSteerFinalApproachInsideTargetCell(t *testing.T) {
	g := grid(`
#####
#...#
#####`)
	// Player hugs the north wall of cell (2,1); enemy at the south-west of the
	// same cell.
	dx, dy, ok := Steer(g, 2.1, 1.85, 2.85, 1.12, 0.2)
	if !ok || dx <= 0 || dy >= 0 {
		t.Fatalf("steer (%.2f, %.2f) ok=%v, want towards the player", dx, dy, ok)
	}
	// Adjacent cell, player close to a wall: must head to the player, not
	// just to the centre of the player's cell.
	dx, dy, ok = Steer(g, 1.5, 1.5, 2.9, 1.12, 0.2)
	if !ok || dx <= 0 {
		t.Fatalf("adjacent steer (%.2f, %.2f) ok=%v", dx, dy, ok)
	}
}
