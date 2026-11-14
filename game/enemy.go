package game

import (
	"math"

	"aiengine/actor"
	"aiengine/anim"
	"aiengine/level"
	"aiengine/renderer"
	"aiengine/world"
)

// enemyPainTime is how long the pain state shows after a hit.
const enemyPainTime = 0.3

// BodyRadius is the collision radius of the player and enemies.
const BodyRadius = 0.15

// Enemy is the game's controller and view of a hostile actor (Actor.Ctl).
type Enemy struct {
	A      *actor.Actor
	Sprite *renderer.Sprite

	// Fighter is the movement and melee brain of level enemies; nil = the
	// sandbox chase of projects without RUNTIME level.
	Fighter     *level.Fighter
	Speed       float64
	Damage      float64
	AttackRange float64
	SightRange  float64 // chase distance (0 = default 6.5)
	HitSound    string
	FramePrefix string // texture prefix of the 3x2 atlas frames ("" = "enemy")

	// Sprite animation; nil = the legacy frame cycle (Host.FrameName).
	Anim *anim.Player

	State       string // idle, walk, attack
	AttackTimer float64
	AnimTimer   float64
	painT       float64
	deathShown  bool
}

// EnemyView is what the host provides for a new enemy: its sprite (already
// textured) and animation.
type EnemyView struct {
	Sprite      *renderer.Sprite
	Anim        *anim.Player
	FramePrefix string
}

func (e *Enemy) sightRange() float64 {
	if e.SightRange > 0 {
		return e.SightRange
	}
	return 6.5
}

// SpawnEnemy creates a level enemy from a placement (parameters already
// merged with its enemy_type template) and registers its actor.
func (w *World) SpawnEnemy(p level.Placement) *Enemy {
	hp := p.Float("hp", 50)
	scale := p.Float("scale", 0.85)
	var view EnemyView
	if w.Host != nil {
		view = w.Host.EnemyView(p)
	}
	if view.Sprite == nil {
		view.Sprite = &renderer.Sprite{Scale: scale, Alpha: 1, Active: true, TexName: "sprite_enemy"}
	}
	s := view.Sprite
	s.X, s.Y, s.EntityID = p.X, p.Y, p.ID
	s.Z = p.Float("z", 0)
	a := &actor.Actor{ID: p.ID, Type: p.Type, Faction: actor.Hostile, X: p.X, Y: p.Y, Radius: BodyRadius,
		Height: p.Float("height", math.Min(world.HeadroomStand, scale)), HP: hp, MaxHP: hp, Alive: true}
	e := &Enemy{A: a, Sprite: s,
		Speed:       p.Float("speed", 1.15),
		Damage:      p.Float("damage", 8),
		AttackRange: p.Float("range", 0.85),
		SightRange:  p.Float("sight", 6.5),
		State:       "idle",
		FramePrefix: view.FramePrefix,
		Fighter:     level.NewFighter(p, int64(len(w.enemies)+1)),
		HitSound:    p.Params["attack_sound"],
		Anim:        view.Anim,
	}
	if e.Anim != nil {
		e.Anim.T = float64(len(w.enemies)) * 0.37 // out of step with each other
	}
	s.Angle = math.Atan2(w.Player.Y-p.Y, w.Player.X-p.X)
	if _, ok := p.Params["angle"]; ok { // initial facing in degrees (0 = +x, 90 = +z)
		s.Angle = p.Float("angle", 0) * math.Pi / 180
	}
	if e.Anim != nil {
		s.TexName, s.Rotations = e.Anim.Textures()
	}
	w.addEnemy(e)
	return e
}

// AddSandboxEnemy adds a sandbox (no level) enemy with the classic chase.
func (w *World) AddSandboxEnemy(id, typ string, x, y float64, sprite *renderer.Sprite) *Enemy {
	sprite.X, sprite.Y, sprite.EntityID = x, y, id
	a := &actor.Actor{ID: id, Type: typ, Faction: actor.Hostile, X: x, Y: y, Radius: BodyRadius,
		Height: math.Min(world.HeadroomStand, sprite.Scale), HP: 50, MaxHP: 50, Alive: true}
	e := &Enemy{A: a, Sprite: sprite, Speed: 1.15, Damage: 8, AttackRange: 0.85, State: "idle"}
	w.addEnemy(e)
	return e
}

func (w *World) addEnemy(e *Enemy) {
	e.A.Ctl = e
	w.Actors.Add(e.A)
	w.enemies = append(w.enemies, e)
}

// Enemies lists the enemies in spawn order (dead ones included).
func (w *World) Enemies() []*Enemy { return w.enemies }

// updateEnemies moves the enemies, resolves their attacks and animates them.
func (w *World) updateEnemies(dt float64) {
	for _, e := range w.enemies {
		a := e.A
		if !a.Alive && e.Sprite.Active {
			if !e.deathShown {
				e.showDeath() // killed outside ApplyDamage (should not happen) - still show it
			}
			if e.Anim != nil {
				w.animate(e, dt, 0, 0) // death animation, then the corpse
			}
			continue
		}
		if !a.Alive || !e.Sprite.Active {
			continue
		}
		prevX, prevY := a.X, a.Y
		if e.Sprite.Alpha < 1.0 {
			e.Sprite.Alpha = min(1, e.Sprite.Alpha+float32(dt*2.5))
		}
		w.applyPush(&a.PushX, &a.PushY, &a.X, &a.Y, dt, func(x, y float64) bool {
			return w.enemyBodyBlocked(prevX, prevY, x, y)
		})
		e.AttackTimer -= dt
		if a.Frozen {
			e.State = "idle"
		} else if e.Fighter != nil && w.Level != nil {
			w.stepFighter(e, dt)
		} else {
			w.stepSandbox(e, dt)
		}
		w.animate(e, dt, a.X-prevX, a.Y-prevY)
		e.Sprite.X, e.Sprite.Y = a.X, a.Y
	}
}

// stepFighter moves one level enemy and resolves its melee attacks with
// level.Fighter (behaviour brain, A* steering, separation, wind-up and
// cooldown).
func (w *World) stepFighter(e *Enemy, dt float64) {
	f, a := e.Fighter, e.A
	f.X, f.Y = a.X, a.Y
	dmg := f.Step(w.Level, w.Player.X, w.Player.Y, dt, func(x, y float64) bool {
		return w.enemyBodyBlocked(a.X, a.Y, x, y) || w.enemyBlocked(e, x, y)
	})
	a.X, a.Y = f.X, f.Y
	switch f.State {
	case "windup", "attack":
		e.State = "attack"
	default:
		e.State = f.State
	}
	if dmg > 0 {
		w.ApplyDamage(PlayerID, Damage{Amount: dmg, Kind: KindMelee, Source: a.ID, X: a.X, Y: a.Y, Sound: e.HitSound})
	}
}

// stepSandbox is the classic chase of the sandbox demo: walk straight at
// the player within sight range, hit once a second within reach.
func (w *World) stepSandbox(e *Enemy, dt float64) {
	a := e.A
	dx, dy := w.Player.X-a.X, w.Player.Y-a.Y
	dist := math.Hypot(dx, dy)
	switch {
	case dist <= e.AttackRange:
		e.State = "attack"
		if e.AttackTimer <= 0 {
			e.AttackTimer = 1.0
			w.ApplyDamage(PlayerID, Damage{Amount: e.Damage, Kind: KindMelee, Source: a.ID, X: a.X, Y: a.Y, Sound: e.HitSound})
		}
	case dist < e.sightRange():
		e.State = "walk"
		if dist > 0.001 {
			stepX, stepY := dx/dist*e.Speed*dt, dy/dist*e.Speed*dt
			if !w.wallAt(a.X+stepX, a.Y) && !w.enemyBlocked(e, a.X+stepX, a.Y) {
				a.X += stepX
			}
			if !w.wallAt(a.X, a.Y+stepY) && !w.enemyBlocked(e, a.X, a.Y+stepY) {
				a.Y += stepY
			}
		}
	default:
		e.State = "idle"
	}
}

// wallAt reports a solid grid cell under (x, y) or 0.15 around it (sandbox).
func (w *World) wallAt(x, y float64) bool {
	const margin = 0.15
	m := w.Grid
	if m == nil {
		return !w.Geo.CanOccupy(x, y, margin, world.FloorAt(w.Geo, x, y), world.MaxStepUp, 0)
	}
	cell := func(x, y float64) bool { return m.GetCell(int(math.Floor(x)), int(math.Floor(y))) != world.CellEmpty }
	return cell(x, y) || cell(x-margin, y) || cell(x+margin, y) || cell(x, y-margin) || cell(x, y+margin)
}

// enemyBlocked reports another living enemy too close to (x, y).
func (w *World) enemyBlocked(current *Enemy, x, y float64) bool {
	for _, o := range w.enemies {
		if o == current || !o.A.Alive {
			continue
		}
		if dx, dy := o.A.X-x, o.A.Y-y; dx*dx+dy*dy < 0.28*0.28 {
			return true
		}
	}
	return false
}

// enemyBodyBlocked reports whether an enemy body standing on the floor of
// (fromX, fromY) may not move to (x, y).
func (w *World) enemyBodyBlocked(fromX, fromY, x, y float64) bool {
	feet, _ := w.Geo.Support(fromX, fromY, BodyRadius)
	return !w.Geo.CanOccupy(x, y, BodyRadius, feet, world.MaxStepUp, world.HeadroomStand)
}

// AimHeight is the absolute height shots aim at on an enemy (its eyes).
func (w *World) AimHeight(e *Enemy) float64 {
	return world.FloorAt(w.Geo, e.A.X, e.A.Y) + world.EyeHeight - e.Sprite.Z
}

// showDeath starts the death animation (the body stays as a corpse) or
// hides the enemy.
func (e *Enemy) showDeath() {
	e.deathShown = true
	if e.Anim != nil && e.Anim.Set.Clip(anim.Death) != nil {
		e.Sprite.Alpha = 1
		e.Anim.Restart(anim.Death)
	} else {
		e.Sprite.Active = false
	}
}

// animate maps the enemy's state to its animation and facing: walk/idle
// from the brain, attack (wind-up and hit), pain after damage, death until
// the corpse frame. (dx, dy) is this frame's movement.
func (w *World) animate(e *Enemy, dt, dx, dy float64) {
	a := e.Anim
	if a == nil {
		e.AnimTimer += dt
		if w.Host != nil {
			e.Sprite.TexName = w.Host.FrameName(e.FramePrefix, e.legacyFrame())
		}
		return
	}
	state := anim.Idle
	switch {
	case !e.A.Alive:
		state = anim.Death
	case e.painT > 0:
		e.painT -= dt
		state = anim.Pain
	case e.State == "walk":
		state = anim.Walk
	case e.State == "attack":
		state = anim.Attack
	}
	a.Play(state)
	a.Update(dt)
	if e.A.Alive {
		if e.State == "attack" {
			e.Sprite.Angle = math.Atan2(w.Player.Y-e.A.Y, w.Player.X-e.A.X)
		} else if dx*dx+dy*dy > 1e-10 {
			e.Sprite.Angle = math.Atan2(dy, dx)
		}
	}
	e.Sprite.TexName, e.Sprite.Rotations = a.Textures()
}

var (
	walkFrames   = [...]int{1, 2, 3, 2}
	attackFrames = [...]int{4, 5}
	idleFrames   = [...]int{0, 1}
)

// legacyFrame is the frame of the classic 3x2 atlas for the current state.
func (e *Enemy) legacyFrame() int {
	switch e.State {
	case "walk":
		return walkFrames[int(e.AnimTimer/0.14)%len(walkFrames)]
	case "attack":
		return attackFrames[int(e.AnimTimer/0.12)%len(attackFrames)]
	default:
		return idleFrames[int(e.AnimTimer/0.45)%len(idleFrames)]
	}
}
