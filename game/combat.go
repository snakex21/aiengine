package game

import (
	"math"

	"aiengine/world"
)

// fireLegacy is the fixed sandbox shot: the nearest living enemy within
// 7 cells and 0.18 rad of the view, with a clear line from the eye, takes
// 25 damage.
func (w *World) fireLegacy() {
	p := w.Player
	w.Flash = 1
	w.Script.PlaySound("gunshot")
	best := math.MaxFloat64
	var target *Enemy
	for _, e := range w.enemies {
		if !e.A.Alive || !e.Sprite.Active {
			continue
		}
		dx, dy := e.A.X-p.X, e.A.Y-p.Y
		dist := math.Hypot(dx, dy)
		if dist > 7.0 || dist >= best {
			continue
		}
		angle := math.Remainder(math.Atan2(dy, dx)-p.Angle, 2*math.Pi)
		if math.Abs(angle) > 0.18 {
			continue
		}
		if !world.LineOfSight(w.Geo, p.X, p.Y, p.EyeZ(), e.A.X, e.A.Y, w.AimHeight(e)) {
			continue
		}
		best, target = dist, e
	}
	if target != nil {
		w.ApplyDamage(target.A.ID, Damage{Amount: 25, Kind: KindShot, Source: PlayerID, X: p.X, Y: p.Y})
	}
}
