package game

import (
	"math"

	"aiengine/renderer"
	"aiengine/weapons"
)

// Projectile is a flying shot: it moves with its velocity (and gravity),
// stops at the first wall, floor, ceiling or body on its path and then
// hits or explodes.
type Projectile struct {
	Shot
	X, Y, Z    float64
	VX, VY, VZ float64
	Age        float64
	Travel     float64
	Sprite     *renderer.Sprite
	dead       bool
}

// ProjectileTexture is the texture name the host registers for a
// projectile weapon's sprite ("proj_<weapon>").
func ProjectileTexture(d *weapons.Def) string { return "proj_" + d.Name }

// TracerTexture is the texture of a hitscan weapon's tracer dots.
func TracerTexture(d *weapons.Def) string { return "tracer_" + d.Name }

// spawnProjectile launches one projectile of a shot along (dx, dy, dz).
func (w *World) spawnProjectile(s Shot, dx, dy, dz float64) {
	d := s.Def
	p := &Projectile{Shot: s, X: s.X, Y: s.Y, Z: s.Z, VX: dx * d.Speed, VY: dy * d.Speed, VZ: dz * d.Speed}
	p.Sprite = &renderer.Sprite{X: p.X, Y: p.Y, Z: p.Z, AbsZ: true, Bright: true, Scale: d.Scale, Alpha: 1,
		Active: true, TexName: ProjectileTexture(d)}
	w.projectiles = append(w.projectiles, p)
	// The muzzle may already be inside a wall's reach: resolve the first
	// short step right away so a shot point-blank at a wall still hits it.
	w.stepProjectile(p, 0)
}

// Projectiles lists the projectiles in flight.
func (w *World) Projectiles() []*Projectile { return w.projectiles }

// updateProjectiles moves every projectile by dt.
func (w *World) updateProjectiles(dt float64) {
	live := w.projectiles[:0]
	for _, p := range w.projectiles {
		if !p.dead {
			w.stepProjectile(p, dt)
		}
		if !p.dead {
			live = append(live, p)
		}
	}
	clear(w.projectiles[len(live):])
	w.projectiles = live
}

func (w *World) stepProjectile(p *Projectile, dt float64) {
	d := p.Def
	p.Age += dt
	if p.Age > d.Lifetime || p.Travel > d.Range {
		w.expire(p)
		return
	}
	p.VZ -= d.Gravity * dt
	nx, ny, nz := p.X+p.VX*dt, p.Y+p.VY*dt, p.Z+p.VZ*dt
	if dt == 0 { // spawn check: the first 0.25 units in front of the muzzle
		sp := math.Max(math.Hypot(math.Hypot(p.VX, p.VY), p.VZ), 1e-9)
		nx, ny, nz = p.X+p.VX/sp*0.25, p.Y+p.VY/sp*0.25, p.Z+p.VZ/sp*0.25
	}
	hit := w.Geo.Trace(p.X, p.Y, p.Z, nx, ny, nz)
	sx, sy, sz := nx-p.X, ny-p.Y, nz-p.Z
	tg, t, ok := w.firstTarget(p.Faction, p.Shooter, p.X, p.Y, p.Z, sx, sy, sz, d.Radius, hit.T)
	switch {
	case ok:
		p.dead = true
		w.impact(p.Shot, tg, true, p.X+sx*t, p.Y+sy*t, p.Z+sz*t)
	case hit.Blocked():
		p.dead = true
		w.impact(p.Shot, target{}, false, hit.X+hit.NX*0.05, hit.Y+hit.NY*0.05, hit.Z+hit.NZ*0.05)
	default:
		if dt > 0 {
			p.Travel += math.Hypot(math.Hypot(sx, sy), sz)
			p.X, p.Y, p.Z = nx, ny, nz
		}
	}
	p.Sprite.X, p.Sprite.Y, p.Sprite.Z = p.X, p.Y, p.Z
	p.Sprite.Active = !p.dead
}

// expire ends a projectile at its range or lifetime: splash weapons burst
// in the air, others fizzle.
func (w *World) expire(p *Projectile) {
	p.dead = true
	if p.Def.Splash > 0 {
		w.explode(p.Shot, p.X, p.Y, p.Z)
	}
}

// Effect is a short-lived visual: an impact spark, an explosion, a tracer
// dot or a muzzle light. It may carry a light that fades with it.
type Effect struct {
	X, Y, Z     float64
	Tex         string // texture ("" = light only); with Frames: Tex_0..Tex_<n-1>
	Frames      int
	Scale       float64
	Grow        float64 // scale added over the lifetime
	TTL         float64
	Fade        bool // alpha fades out
	Light       float64
	LightRadius float64

	age    float64
	sprite *renderer.Sprite
}

func (w *World) addEffect(e *Effect) {
	if e.Tex != "" {
		e.sprite = &renderer.Sprite{X: e.X, Y: e.Y, Z: e.Z, AbsZ: true, Bright: true, Scale: e.Scale, Alpha: 1, Active: true}
		e.sprite.TexName = e.frameTex()
	}
	w.effects = append(w.effects, e)
}

func (e *Effect) frameTex() string {
	if e.Frames <= 1 {
		return e.Tex
	}
	i := min(int(e.age/e.TTL*float64(e.Frames)), e.Frames-1)
	return e.Tex + "_" + string(rune('0'+i))
}

func (w *World) updateEffects(dt float64) {
	live := w.effects[:0]
	for _, e := range w.effects {
		e.age += dt
		if e.age >= e.TTL {
			continue
		}
		if s := e.sprite; s != nil {
			k := e.age / e.TTL
			s.TexName = e.frameTex()
			s.Scale = e.Scale + e.Grow*k
			if e.Fade {
				s.Alpha = float32(1 - k*k)
			}
		}
		live = append(live, e)
	}
	clear(w.effects[len(live):])
	w.effects = live
}

// tracer leaves a line of glowing dots from the muzzle to the hit point.
func (w *World) tracer(d *weapons.Def, x0, y0, z0, x1, y1, z1 float64) {
	dx, dy, dz := x1-x0, y1-y0, z1-z0
	n := math.Hypot(math.Hypot(dx, dy), dz)
	const gap = 0.3
	for s := 0.45; s < n; s += gap {
		k := s / n
		w.addEffect(&Effect{X: x0 + dx*k, Y: y0 + dy*k, Z: z0 + dz*k, Tex: TracerTexture(d), Scale: 0.07,
			TTL: 0.09 + 0.05*k, Fade: true})
	}
	if n > 0.5 {
		w.addEffect(&Effect{X: x0 + dx*0.5, Y: y0 + dy*0.5, Z: z0 + dz*0.5, Light: 0.35, LightRadius: 1.5, TTL: 0.08})
	}
}

// Sprites appends the sprites of projectiles and effects (for rendering).
func (w *World) Sprites(buf []*renderer.Sprite) []*renderer.Sprite {
	for _, p := range w.projectiles {
		buf = append(buf, p.Sprite)
	}
	for _, e := range w.effects {
		if e.sprite != nil {
			buf = append(buf, e.sprite)
		}
	}
	return buf
}

// Lights appends the dynamic lights of projectiles and effects.
func (w *World) Lights(buf []renderer.PointLight) []renderer.PointLight {
	for _, p := range w.projectiles {
		if p.Def.Light > 0 {
			buf = append(buf, renderer.PointLight{X: p.X, Y: p.Y, Level: p.Def.Light, Radius: p.Def.LightRadius})
		}
	}
	for _, e := range w.effects {
		if e.Light > 0 {
			k := 1 - e.age/e.TTL
			buf = append(buf, renderer.PointLight{X: e.X, Y: e.Y, Level: e.Light * k, Radius: e.LightRadius})
		}
	}
	return buf
}
