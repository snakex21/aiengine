package game

import (
	"math"

	"aiengine/actor"
	"aiengine/weapons"
	"aiengine/world"
)

// Shot is one trigger pull of a weapon mounted on something: the player,
// an enemy, later a vehicle turret. Shoot does not care who holds it.
type Shot struct {
	Def     *weapons.Def
	Shooter string        // actor id or "player"
	Faction actor.Faction // shots do not hit the shooter's own faction
	X, Y, Z float64       // muzzle (absolute height)
	Yaw     float64       // radians, 0 = +x
	Pitch   float64       // radians, up positive
}

// target is something a shot can hit: a vertical cylinder.
type target struct {
	id         string
	x, y, r    float64
	zMin, zMax float64
	faction    actor.Faction
}

func (t target) centre() (float64, float64, float64) { return t.x, t.y, (t.zMin + t.zMax) / 2 }

// targets lists the hittable bodies for a shot of faction f (nil-faction
// rules: the player's side never hits itself, enemies never hit enemies).
func (w *World) targets(f actor.Faction, buf []target) []target {
	buf = buf[:0]
	if f != actor.Friendly {
		p := w.Player
		buf = append(buf, target{id: PlayerID, x: p.X, y: p.Y, r: 0.25, zMin: p.Feet(), zMax: p.Feet() + p.Headroom() + 0.05, faction: actor.Friendly})
	}
	for _, a := range w.Actors.All() {
		if !a.Alive || a.Faction == f {
			continue
		}
		buf = append(buf, w.actorTarget(a))
	}
	return buf
}

// actorTarget is an actor's hit volume: for enemies the visible sprite
// (centre at its eye height, as tall as the sprite).
func (w *World) actorTarget(a *actor.Actor) target {
	t := target{id: a.ID, x: a.X, y: a.Y, r: math.Max(a.Radius, 0.15), faction: a.Faction}
	floor := world.FloorAt(w.Geo, a.X, a.Y)
	if e, ok := a.Ctl.(*Enemy); ok && e.Sprite != nil {
		c := floor + world.EyeHeight - e.Sprite.Z
		h := e.Sprite.Scale / 2 * 0.9
		t.zMin, t.zMax = c-h, c+h
		t.r = math.Max(t.r, e.Sprite.Scale*0.3)
		return t
	}
	t.zMin, t.zMax = floor+a.Lift, floor+a.Lift+math.Max(a.Height, 0.5)
	return t
}

// hitCylinder returns the first t in [0,1] where the segment p0 + t*d
// enters target t grown by extra (projectile radius), or ok=false.
func hitCylinder(tg target, x0, y0, z0, dx, dy, dz, extra float64) (float64, bool) {
	r := tg.r + extra
	ox, oy := x0-tg.x, y0-tg.y
	a := dx*dx + dy*dy
	b := 2 * (ox*dx + oy*dy)
	c := ox*ox + oy*oy - r*r
	t0, t1 := 0.0, 1.0
	if a < 1e-12 { // vertical segment
		if c > 0 {
			return 0, false
		}
	} else {
		disc := b*b - 4*a*c
		if disc < 0 {
			return 0, false
		}
		s := math.Sqrt(disc)
		t0, t1 = math.Max(0, (-b-s)/(2*a)), math.Min(1, (-b+s)/(2*a))
	}
	zMin, zMax := tg.zMin-extra, tg.zMax+extra
	if math.Abs(dz) < 1e-12 {
		if z0 < zMin || z0 > zMax {
			return 0, false
		}
	} else {
		za, zb := (zMin-z0)/dz, (zMax-z0)/dz
		if za > zb {
			za, zb = zb, za
		}
		t0, t1 = math.Max(t0, za), math.Min(t1, zb)
	}
	if t0 > t1 {
		return 0, false
	}
	return t0, true
}

// firstTarget returns the first body the segment meets before tMax.
func (w *World) firstTarget(f actor.Faction, skip string, x0, y0, z0, dx, dy, dz, extra, tMax float64) (target, float64, bool) {
	w.tgtBuf = w.targets(f, w.tgtBuf)
	best, bestT, found := target{}, tMax, false
	for _, tg := range w.tgtBuf {
		if tg.id == skip {
			continue
		}
		if t, ok := hitCylinder(tg, x0, y0, z0, dx, dy, dz, extra); ok && t <= bestT {
			best, bestT, found = tg, t, true
		}
	}
	return best, bestT, found
}

// spreadDir returns a unit direction around (yaw, pitch) inside a cone of
// half-angle spread (uniform over the cone's disc).
func (w *World) spreadDir(yaw, pitch, spread float64) (float64, float64, float64) {
	if spread > 0 {
		r := spread * math.Sqrt(w.rng.Float64())
		a := 2 * math.Pi * w.rng.Float64()
		yaw += r * math.Cos(a)
		pitch += r * math.Sin(a)
	}
	cp := math.Cos(pitch)
	return cp * math.Cos(yaw), cp * math.Sin(yaw), math.Sin(pitch)
}

// Shoot fires pellets rays or projectiles of a shot.
func (w *World) Shoot(s Shot, pellets int) {
	d := s.Def
	w.push(Event{Kind: EventShot, Source: s.Shooter, Name: d.Name, X: s.X, Y: s.Y, Z: s.Z})
	w.sound(d.SoundFire, s.X, s.Y, s.Z)
	if d.MuzzleLight > 0 {
		if s.Shooter == PlayerID {
			w.Flash = math.Max(w.Flash, d.MuzzleLight)
		} else {
			w.addEffect(&Effect{X: s.X, Y: s.Y, Z: s.Z, Light: d.MuzzleLight, LightRadius: 2.5, TTL: 0.12})
		}
	}
	for i := 0; i < max(1, pellets); i++ {
		dx, dy, dz := w.spreadDir(s.Yaw, s.Pitch, d.Spread)
		if d.Kind == weapons.Projectile {
			w.spawnProjectile(s, dx, dy, dz)
		} else {
			w.hitscan(s, dx, dy, dz)
		}
	}
}

// hitscan traces one ray: the nearest body before the first wall takes
// the damage; splash weapons explode where the ray stops.
func (w *World) hitscan(s Shot, dx, dy, dz float64) {
	d := s.Def
	ex, ey, ez := s.X+dx*d.Range, s.Y+dy*d.Range, s.Z+dz*d.Range
	hit := w.Geo.Trace(s.X, s.Y, s.Z, ex, ey, ez)
	sx, sy, sz := ex-s.X, ey-s.Y, ez-s.Z
	tg, t, ok := w.firstTarget(s.Faction, s.Shooter, s.X, s.Y, s.Z, sx, sy, sz, 0, hit.T)
	px, py, pz := s.X+sx*t, s.Y+sy*t, s.Z+sz*t
	if !ok {
		px, py, pz = hit.X+hit.NX*0.02, hit.Y+hit.NY*0.02, hit.Z+hit.NZ*0.02
	}
	if d.Tracer {
		w.tracer(d, s.X, s.Y, s.Z, px, py, pz)
	}
	if !ok && !hit.Blocked() {
		return // out of range in open air
	}
	w.impact(s, tg, ok, px, py, pz)
}

// impact resolves a hit at (x, y, z): a direct hit on tg (when direct) and,
// for splash weapons, an explosion.
func (w *World) impact(s Shot, tg target, direct bool, x, y, z float64) {
	d := s.Def
	w.push(Event{Kind: EventImpact, Source: s.Shooter, Target: tg.id, Name: d.Name, X: x, Y: y, Z: z})
	w.sound(d.SoundHit, x, y, z)
	if d.Splash > 0 {
		w.explode(s, x, y, z)
		return
	}
	w.addEffect(&Effect{X: x, Y: y, Z: z, Tex: "fx_spark", Scale: 0.12, TTL: 0.15, Fade: true, Light: 0.25, LightRadius: 1})
	if direct {
		w.ApplyDamage(tg.id, Damage{Amount: d.Damage, Kind: KindShot, Source: s.Shooter, X: s.X, Y: s.Y, Knockback: d.Knockback})
	}
}

// explode deals splash damage around (x, y, z): full damage at the centre
// falling linearly to SplashFalloff at the edge, only to bodies in line of
// sight of the blast (walls and floors shield), with knockback.
func (w *World) explode(s Shot, x, y, z float64) {
	d := s.Def
	r := d.Splash
	w.push(Event{Kind: EventExplosion, Source: s.Shooter, Name: d.Name, Amount: r, X: x, Y: y, Z: z})
	w.sound(d.SoundExplode, x, y, z)
	w.addEffect(&Effect{X: x, Y: y, Z: z, Tex: "fx_boom", Frames: 4, Scale: r * 0.9, Grow: r * 0.6, TTL: 0.4,
		Light: 1.4, LightRadius: r*2 + 1.5, Fade: true})
	if pd := math.Hypot(w.Player.X-x, w.Player.Y-y); pd < r+3 {
		w.Shake = math.Max(w.Shake, 0.25*(1-pd/(r+3)))
	}
	w.tgtBuf = w.targets(actor.Neutral, w.tgtBuf) // everyone: splash hurts all sides
	hits := append([]target(nil), w.tgtBuf...)
	for _, tg := range hits {
		cx, cy, cz := tg.centre()
		// Distance from the blast to the nearest point of the body.
		zc := math.Max(tg.zMin, math.Min(z, tg.zMax))
		h := math.Max(0, math.Hypot(cx-x, cy-y)-tg.r)
		dist := math.Hypot(h, zc-z)
		if dist >= r || !world.LineOfSight(w.Geo, x, y, z, cx, cy, cz) {
			continue
		}
		k := 1 - (1-d.SplashFalloff)*dist/r
		w.ApplyDamage(tg.id, Damage{Amount: d.Damage * k, Kind: KindSplash, Source: s.Shooter, X: x, Y: y, Knockback: d.Knockback * k})
	}
}

// sound queues a positional sound ("" = none).
func (w *World) sound(name string, x, y, z float64) {
	if name != "" {
		w.push(Event{Kind: EventSound, Name: name, X: x, Y: y, Z: z})
	}
}

// updatePlayerWeapon handles switching, reloading and firing the player's
// current weapon.
func (w *World) updatePlayerWeapon(dt float64, in Input) {
	lo := w.Script.Loadout
	switch {
	case in.Slot > 0:
		lo.SelectSlot(in.Slot)
	case in.Cycle != 0:
		lo.Cycle(in.Cycle)
	}
	lo.Update(dt)
	wp := lo.CurrentWeapon()
	if wp == nil {
		return
	}
	p := w.Player
	if in.Reload && wp.Reload(lo.Ammo) {
		w.sound(wp.Def.SoundReload, p.X, p.Y, p.EyeZ())
	}
	if !in.Fire && !(in.FireHeld && wp.Def.Auto) {
		return
	}
	if wp.Empty(lo.Ammo) {
		if in.Fire {
			w.sound(wp.Def.SoundEmpty, p.X, p.Y, p.EyeZ())
		}
		return
	}
	n := wp.Fire(lo.Ammo)
	if n == 0 {
		return
	}
	w.Shoot(Shot{Def: wp.Def, Shooter: PlayerID, Faction: actor.Friendly, X: p.X, Y: p.Y, Z: p.EyeZ() - 0.05,
		Yaw: p.Angle, Pitch: math.Atan(in.AimSlope)}, n)
	if wp.Reloading {
		w.sound(wp.Def.SoundReload, p.X, p.Y, p.EyeZ())
	}
}
