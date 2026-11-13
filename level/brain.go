package level

import (
	"math"
	"math/rand"

	"aiengine/nav"
	"aiengine/renderer"
)

// Solid implements nav.Grid over the level map (doors count while closed).
func (lv *Level) Solid(x, y int) bool { return lv.Map.GetCell(x, y) != renderer.CellEmpty }

// Enemy behaviours (PLACE enemy ... behavior=<name>):
//
//	chase  – default: chases the player on sight, then searches the last
//	         seen position for a while
//	guard  – like chase, but walks back to its post when it loses the player
//	wander – roams to random nearby spots until it sees the player
//	ambush – stands still (does not even turn) until the player is close
//	         (half the sight range), then chases
const (
	BehaviorChase  = "chase"
	BehaviorGuard  = "guard"
	BehaviorWander = "wander"
	BehaviorAmbush = "ambush"
)

// Brain decides where an enemy wants to go; the host moves it there with
// nav.Steer and handles attacks. It is deterministic for a given seed.
type Brain struct {
	Behavior     string
	HomeX, HomeY float64
	Memory       float64 // seconds a lost player is still pursued (default 4)

	seenX, seenY float64
	lostFor      float64
	hasSeen      bool
	awake        bool
	wanderX      float64
	wanderY      float64
	wanderT      float64
	rng          *rand.Rand
}

// NewBrain creates a brain for an enemy placed at (x, y).
func NewBrain(behavior string, x, y float64, seed int64) *Brain {
	if behavior == "" {
		behavior = BehaviorChase
	}
	return &Brain{Behavior: behavior, HomeX: x, HomeY: y, Memory: 4, rng: rand.New(rand.NewSource(seed))}
}

// Think returns the point the enemy wants to move to this frame and whether
// it wants to move at all. sees is true when the player is in sight range
// with a clear line of sight; dist is the distance to the player.
func (b *Brain) Think(g nav.Grid, ex, ey, px, py, dt, sight, dist float64, sees bool) (tx, ty float64, move bool) {
	if b.Behavior == BehaviorAmbush && !b.awake {
		if sees && dist < sight/2 {
			b.awake = true
		} else {
			return ex, ey, false
		}
	}
	if sees {
		b.seenX, b.seenY, b.hasSeen, b.lostFor = px, py, true, 0
		return px, py, true
	}
	if b.hasSeen {
		b.lostFor += dt
		if b.lostFor < b.Memory && math.Hypot(b.seenX-ex, b.seenY-ey) > 0.3 {
			return b.seenX, b.seenY, true
		}
		b.hasSeen = false
	}
	switch b.Behavior {
	case BehaviorGuard:
		if math.Hypot(b.HomeX-ex, b.HomeY-ey) > 0.3 {
			return b.HomeX, b.HomeY, true
		}
	case BehaviorWander:
		b.wanderT -= dt
		if b.wanderT <= 0 || math.Hypot(b.wanderX-ex, b.wanderY-ey) < 0.3 {
			b.pickWander(g, ex, ey)
		}
		return b.wanderX, b.wanderY, true
	}
	return ex, ey, false
}

// pickWander chooses a random reachable free cell within 4 cells of home.
func (b *Brain) pickWander(g nav.Grid, ex, ey float64) {
	b.wanderT = 3 + b.rng.Float64()*3
	home := nav.CellAt(b.HomeX, b.HomeY)
	for i := 0; i < 20; i++ {
		c := nav.Cell{X: home.X + b.rng.Intn(9) - 4, Y: home.Y + b.rng.Intn(9) - 4}
		if g.Solid(c.X, c.Y) {
			continue
		}
		if nav.FindPath(g, nav.CellAt(ex, ey), c, 256) == nil {
			continue
		}
		b.wanderX, b.wanderY = c.Center()
		return
	}
	b.wanderX, b.wanderY = ex, ey
}
