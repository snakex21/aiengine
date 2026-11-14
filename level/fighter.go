package level

import (
	"math"

	"aiengine/nav"
	"aiengine/world"
)

// Separation is the closest an enemy's centre comes to the player's centre:
// enemies stop here instead of walking into (and overlapping) the player.
const Separation = 0.5

// minReach is how far beyond Separation every melee attack must reach, so an
// enemy that stopped at Separation can always hit.
const minReach = 0.2

// Fighter is the movement and melee logic of one enemy, independent of the
// renderer so it can be stepped headlessly.
type Fighter struct {
	ID     string
	Type   string
	X, Y   float64
	Speed  float64
	Damage float64
	Range  float64 // melee reach (centre to centre), at least Separation+minReach
	Rate   float64 // seconds between hits
	Windup float64 // telegraph time before a hit lands
	Sight  float64
	Brain  *Brain

	State string // idle, walk, windup, attack (for animation)

	cooldown float64
	winding  float64
}

// NewFighter builds a fighter from an enemy placement (params already merged
// with its enemy_type template).
func NewFighter(p Placement, seed int64) *Fighter {
	f := &Fighter{
		ID: p.ID, Type: p.Type, X: p.X, Y: p.Y,
		Speed:  p.Float("speed", 1.15),
		Damage: p.Float("damage", 8),
		Range:  p.Float("range", 0.85),
		Rate:   p.Float("attack_rate", 1.0),
		Windup: p.Float("windup", 0.3),
		Sight:  p.Float("sight", 6.5),
		Brain:  NewBrain(p.Params["behavior"], p.X, p.Y, seed),
		State:  "idle",
	}
	if f.Range < Separation+minReach {
		f.Range = Separation + minReach
	}
	if f.Rate <= 0 {
		f.Rate = 1
	}
	return f
}

// Step advances the fighter by dt with the player at (px, py). blocked
// reports positions the body may not enter (walls, other enemies). It
// returns the damage dealt to the player in this step (0 or Damage).
// Sight uses the level geometry (world.Trace, the same query as the
// player's shots); paths use the level's nav grid.
func (f *Fighter) Step(lv *Level, px, py, dt float64, blocked func(x, y float64) bool) float64 {
	var g nav.Grid = lv
	if f.cooldown > 0 {
		f.cooldown -= dt
	}
	dist := math.Hypot(px-f.X, py-f.Y)
	geo := lv.Geometry()
	ef, pf := world.FloorAt(geo, f.X, f.Y), world.FloorAt(geo, px, py)
	sees := dist < f.Sight && f.Sees(geo, ef, px, py, pf)
	reach := math.Abs(ef-pf) <= MeleeHeight

	// Melee: telegraph (wind-up), then hit if the player is still in reach.
	if sees && reach && dist <= f.Range {
		if f.winding > 0 {
			f.winding -= dt
			f.State = "windup"
			if f.winding <= 0 {
				f.winding = 0
				f.cooldown = f.Rate
				f.State = "attack"
				return f.Damage
			}
			return 0
		}
		if f.cooldown <= 0 && f.Damage > 0 {
			f.winding = math.Max(f.Windup, 1e-6)
			f.State = "windup"
			return 0
		}
		f.State = "attack"
		f.Brain.Think(g, f.X, f.Y, px, py, dt, f.Sight, dist, true) // keep memory fresh
		f.approach(g, px, py, px, py, dt, blocked)
		return 0
	}
	f.winding = 0 // player escaped the telegraphed hit

	tx, ty, move := f.Brain.Think(g, f.X, f.Y, px, py, dt, f.Sight, dist, sees)
	if !move {
		f.State = "idle"
		return 0
	}
	f.approach(g, px, py, tx, ty, dt, blocked)
	return 0
}

// approach moves towards (tx, ty) but never closer than Separation to the
// player at (px, py).
func (f *Fighter) approach(g nav.Grid, px, py, tx, ty, dt float64, blocked func(x, y float64) bool) {
	dx, dy, ok := nav.Steer(g, f.X, f.Y, tx, ty, 0.2)
	if !ok {
		f.State = "idle"
		return
	}
	step := f.Speed * dt
	nx, ny := f.X+dx*step, f.Y+dy*step
	if math.Hypot(px-nx, py-ny) < Separation {
		// Stop at the separation ring instead of overlapping the player.
		if math.Hypot(px-f.X, py-f.Y) <= Separation {
			if f.State != "attack" {
				f.State = "idle"
			}
			return
		}
		nx, ny = f.X, f.Y
		for i := 0; i < 8 && math.Hypot(px-(nx+dx*step/8), py-(ny+dy*step/8)) >= Separation; i++ {
			nx, ny = nx+dx*step/8, ny+dy*step/8
		}
	}
	moved := false
	if blocked == nil || !blocked(nx, f.Y) {
		f.X, moved = nx, true
	}
	if blocked == nil || !blocked(f.X, ny) {
		f.Y, moved = ny, true
	}
	if f.State != "attack" {
		f.State = "walk"
		if !moved {
			f.State = "idle"
		}
	}
}

// EyeLevel is the height of enemy and player eyes above their floor for
// line-of-sight checks; MeleeHeight is the largest floor difference a melee
// hit bridges (an enemy below a ledge cannot hit a player standing on it).
const (
	EyeLevel    = 0.5
	MeleeHeight = 0.45
)

// Sees reports whether the fighter (feet at height ef) has a clear line
// from its eyes to the player's eyes (player feet at pf).
func (f *Fighter) Sees(geo world.Geometry, ef, px, py, pf float64) bool {
	return world.LineOfSight(geo, f.X, f.Y, ef+EyeLevel, px, py, pf+EyeLevel)
}
