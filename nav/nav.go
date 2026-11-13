// Package nav provides grid pathfinding (A*) and steering for enemies on the
// raycaster's tile map, so they walk around walls and corners instead of
// pushing straight at the player.
package nav

import (
	"container/heap"
	"math"
)

// Grid is a walkability map: Solid reports blocked cells (out of range must
// report true).
type Grid interface {
	Solid(x, y int) bool
}

// Cell is a grid coordinate (x = column, y = row).
type Cell struct{ X, Y int }

// CellAt returns the cell containing world point (x, y).
func CellAt(x, y float64) Cell { return Cell{int(math.Floor(x)), int(math.Floor(y))} }

// Center returns the world centre of a cell.
func (c Cell) Center() (float64, float64) { return float64(c.X) + 0.5, float64(c.Y) + 0.5 }

var dirs = [8]Cell{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}

// FindPath returns the cells from start to goal (both included) using A* with
// 8-way moves; diagonal steps are only allowed when both adjacent orthogonal
// cells are free (no corner cutting). maxNodes bounds the search (0 = 4096).
// It returns nil when the goal is unreachable or blocked.
func FindPath(g Grid, start, goal Cell, maxNodes int) []Cell {
	if g.Solid(goal.X, goal.Y) || g.Solid(start.X, start.Y) {
		return nil
	}
	if start == goal {
		return []Cell{start}
	}
	if maxNodes <= 0 {
		maxNodes = 4096
	}
	open := &pq{}
	heap.Push(open, &node{c: start, f: octile(start, goal)})
	gScore := map[Cell]float64{start: 0}
	came := map[Cell]Cell{}
	closed := map[Cell]bool{}
	for open.Len() > 0 && len(closed) < maxNodes {
		cur := heap.Pop(open).(*node).c
		if cur == goal {
			path := []Cell{cur}
			for cur != start {
				cur = came[cur]
				path = append(path, cur)
			}
			for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
				path[i], path[j] = path[j], path[i]
			}
			return path
		}
		if closed[cur] {
			continue
		}
		closed[cur] = true
		for _, d := range dirs {
			n := Cell{cur.X + d.X, cur.Y + d.Y}
			if g.Solid(n.X, n.Y) || closed[n] {
				continue
			}
			cost := 1.0
			if d.X != 0 && d.Y != 0 {
				if g.Solid(cur.X+d.X, cur.Y) || g.Solid(cur.X, cur.Y+d.Y) {
					continue // would clip a corner
				}
				cost = math.Sqrt2
			}
			ng := gScore[cur] + cost
			if old, ok := gScore[n]; ok && ng >= old {
				continue
			}
			gScore[n] = ng
			came[n] = cur
			heap.Push(open, &node{c: n, f: ng + octile(n, goal)})
		}
	}
	return nil
}

func octile(a, b Cell) float64 {
	dx := math.Abs(float64(a.X - b.X))
	dy := math.Abs(float64(a.Y - b.Y))
	return math.Max(dx, dy) + (math.Sqrt2-1)*math.Min(dx, dy)
}

// LineClear reports whether a body of the given radius can move in a straight
// line from (x0,y0) to (x1,y1) without touching solid cells (sampled).
func LineClear(g Grid, x0, y0, x1, y1, radius float64) bool {
	dx, dy := x1-x0, y1-y0
	dist := math.Hypot(dx, dy)
	steps := int(dist/0.1) + 1
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x, y := x0+dx*t, y0+dy*t
		for _, o := range [5][2]float64{{0, 0}, {radius, 0}, {-radius, 0}, {0, radius}, {0, -radius}} {
			c := CellAt(x+o[0], y+o[1])
			if g.Solid(c.X, c.Y) {
				return false
			}
		}
	}
	return true
}

// Steer returns a unit direction to move from (x,y) towards (tx,ty): straight
// when the way is clear for a body of the given radius, otherwise towards the
// first useful cell centre of an A* path. ok is false when there is no path.
func Steer(g Grid, x, y, tx, ty, radius float64) (dx, dy float64, ok bool) {
	if LineClear(g, x, y, tx, ty, radius) {
		return unit(tx-x, ty-y)
	}
	from, goal := CellAt(x, y), CellAt(tx, ty)
	// Same cell, or the next cell is the target's: the remaining way is
	// inside open cells, so head straight for the target point (aiming at a
	// cell centre - or giving up - left enemies standing out of reach when
	// the player hugged a wall).
	if from == goal || (math.Abs(float64(from.X-goal.X)) <= 1 && math.Abs(float64(from.Y-goal.Y)) <= 1 &&
		!g.Solid(goal.X, from.Y) && !g.Solid(from.X, goal.Y)) {
		return unit(tx-x, ty-y)
	}
	path := FindPath(g, from, goal, 0)
	if len(path) < 2 {
		return 0, 0, false
	}
	// Skip ahead to the farthest path cell reachable in a straight line
	// (smooths the zig-zag of grid paths).
	next := path[1]
	for i := len(path) - 1; i >= 1; i-- {
		cx, cy := path[i].Center()
		if LineClear(g, x, y, cx, cy, radius) {
			next = path[i]
			break
		}
	}
	cx, cy := next.Center()
	return unit(cx-x, cy-y)
}

func unit(dx, dy float64) (float64, float64, bool) {
	l := math.Hypot(dx, dy)
	if l < 1e-9 {
		return 0, 0, false
	}
	return dx / l, dy / l, true
}

// --- priority queue ---

type node struct {
	c Cell
	f float64
}

type pq []*node

func (q pq) Len() int            { return len(q) }
func (q pq) Less(i, j int) bool  { return q[i].f < q[j].f }
func (q pq) Swap(i, j int)       { q[i], q[j] = q[j], q[i] }
func (q *pq) Push(x interface{}) { *q = append(*q, x.(*node)) }
func (q *pq) Pop() interface{} {
	old := *q
	n := old[len(old)-1]
	*q = old[:len(old)-1]
	return n
}
