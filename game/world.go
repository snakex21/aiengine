// Package game is the simulation of a running level, without a window:
// player movement, enemies, combat, damage, movers and the per-frame event
// queue. The host (package main) turns devices into an Input, calls
// World.Update once per simulated frame and renders the result; tests drive
// the same World headlessly.
package game

import (
	"math"

	"aiengine/actor"
	"aiengine/level"
	"aiengine/scripting"
	"aiengine/world"
)

// Host is what the world needs from the program around it.
type Host interface {
	// EnemyView builds the sprite (textures loaded) and animation of a new
	// level enemy.
	EnemyView(p level.Placement) EnemyView
	// FrameName names frame i of the classic 3x2 enemy atlas with prefix.
	FrameName(prefix string, i int) string
}

// Input is one frame of player intent.
type Input struct {
	Forward, Back, Left, Right bool
	Jump                       bool // pressed this frame
	Crouch                     bool // held
	ProneToggle                bool // pressed this frame
	Fire                       bool // pressed this frame
	FireHeld                   bool // held (automatic weapons)
	Reload                     bool // pressed this frame
	Interact                   bool // pressed this frame
	Yaw                        float64
	Pitch                      float64
}

// World is one running level (or the sandbox).
type World struct {
	Player *world.PlayerState
	Geo    world.Geometry
	Grid   *world.MapData // the grid map (sandbox chase); nil is fine for sector levels

	Level   *level.Level   // nil in the sandbox
	Session *level.Session // nil in the sandbox
	Script  *scripting.Engine
	Actors  *actor.Set // = Script.Actors
	Host    Host

	// God and Noclip read the console cheats (nil = off).
	God, Noclip func() bool

	// Feedback for the host to show and decay: red flash and camera shake
	// after the player is hurt, muzzle light after a shot.
	HurtFlash, Shake, Flash float64

	// Moving reports whether the player walked this frame.
	Moving bool

	Clock float64 // game time of this world

	enemies    []*Enemy
	events     []Event
	dispatched int
	pushX      float64 // player knockback velocity
	pushY      float64
}

// New creates a world for a level (lv may be nil for the sandbox, then
// grid is its map). It wires the scripting engine's actor registry and
// script damage (killEnemy, setEnemyHP) to the world.
func New(lv *level.Level, grid *world.MapData, player *world.PlayerState, eng *scripting.Engine, host Host) *World {
	w := &World{Player: player, Level: lv, Script: eng, Actors: eng.Actors, Host: host, Grid: grid}
	if lv != nil {
		w.Geo = lv.Geometry()
		w.Grid = lv.Map
	} else {
		w.Geo = grid
	}
	eng.OnDamage = func(id string, amount float64) {
		w.ApplyDamage(id, Damage{Amount: amount, Kind: KindScript})
	}
	return w
}

// AttachSession runs the level session inside the world: movers see the
// world's bodies and crush damage goes through ApplyDamage.
func (w *World) AttachSession(s *level.Session) {
	w.Session = s
	s.Bodies = w.moverBodies
	s.HostDamage = true
}

// PlacePlayer moves the player to (x, y) standing on that spot's floor
// (spawn, teleports): no fall and no step smoothing.
func (w *World) PlacePlayer(x, y float64) {
	w.Player.X, w.Player.Y = x, y
	floor, _ := w.Geo.Support(x, y, BodyRadius)
	w.Player.PlaceAt(floor)
}

// Update advances the simulation by dt seconds of game time. Script timers
// (Engine.Advance) are the host's job, so it can tick other script VMs in
// between.
func (w *World) Update(dt float64, in Input) {
	w.Clock += dt
	w.movePlayer(dt, in)
	if in.Interact && w.Session != nil {
		w.Session.Interact(w.Player.X, w.Player.Y, w.Player.Angle)
	}
	if in.Fire {
		w.fireLegacy()
	}
	w.Script.SetPlayerPosition(w.Player.X, w.Player.Y, w.Player.Feet())
	w.updateEnemies(dt)
	if w.Session != nil {
		w.updateSession(dt)
	}
	w.dispatch()
}

// movePlayer applies walking (with wall sliding), stance, jumping, gravity,
// fall damage and mouse look.
func (w *World) movePlayer(dt float64, in Input) {
	p := w.Player
	speed := p.MoveSpeed * p.SpeedMultiplier() * dt
	if w.Script.PlayerFrozen {
		speed = 0
	}
	moveX, moveY := 0.0, 0.0
	if in.Forward {
		moveX += math.Cos(p.Angle) * speed
		moveY += math.Sin(p.Angle) * speed
	}
	if in.Back {
		moveX -= math.Cos(p.Angle) * speed
		moveY -= math.Sin(p.Angle) * speed
	}
	if in.Left {
		moveX += math.Sin(p.Angle) * speed
		moveY -= math.Cos(p.Angle) * speed
	}
	if in.Right {
		moveX -= math.Sin(p.Angle) * speed
		moveY += math.Cos(p.Angle) * speed
	}
	// Predictive collision per axis (wall sliding). Noclip skips walls.
	noclip := w.Noclip != nil && w.Noclip()
	if nx := p.X + moveX; noclip || !w.playerBlocked(nx, p.Y) {
		p.X = nx
	}
	if ny := p.Y + moveY; noclip || !w.playerBlocked(p.X, ny) {
		p.Y = ny
	}
	w.applyPush(&w.pushX, &w.pushY, &p.X, &p.Y, dt, w.playerBlocked)
	floor, _ := w.Geo.Support(p.X, p.Y, BodyRadius)
	p.SetGround(floor)

	// Stance: hold crouch to crouch; toggle prone. Jumping stands up.
	switch {
	case in.ProneToggle:
		if p.Stance == world.StanceProne {
			p.SetStance(world.StanceStand)
		} else {
			p.SetStance(world.StanceProne)
		}
	case in.Crouch:
		p.SetStance(world.StanceCrouch)
	case p.Stance == world.StanceCrouch && w.canStandUp():
		p.SetStance(world.StanceStand) // crouch released, not under a low ceiling
	}
	if p.Stance == world.StanceStand && !w.canStandUp() {
		p.SetStance(world.StanceCrouch)
	}
	if in.Jump {
		if p.Stance != world.StanceStand {
			p.SetStance(world.StanceStand)
		}
		p.Jump()
	}
	p.UpdateVertical(dt)
	w.applyFallDamage()

	if in.Yaw != 0 {
		p.Angle += in.Yaw
	}
	if in.Pitch != 0 {
		p.AddPitch(in.Pitch)
	}
	w.Moving = moveX != 0 || moveY != 0
}

// applyPush moves a body by its knockback velocity (blocked positions stop
// it) and lets the velocity decay.
func (w *World) applyPush(vx, vy, x, y *float64, dt float64, blocked func(x, y float64) bool) {
	if *vx == 0 && *vy == 0 {
		return
	}
	if nx := *x + *vx*dt; !blocked(nx, *y) {
		*x = nx
	} else {
		*vx = 0
	}
	if ny := *y + *vy*dt; !blocked(*x, ny) {
		*y = ny
	} else {
		*vy = 0
	}
	k := math.Exp(-8 * dt)
	*vx, *vy = *vx*k, *vy*k
	if math.Hypot(*vx, *vy) < 0.05 {
		*vx, *vy = 0, 0
	}
}

// playerBlocked reports whether the player may not stand at (x, y): walls,
// floors more than a step above the feet, or too little headroom.
func (w *World) playerBlocked(x, y float64) bool {
	p := w.Player
	return !w.Geo.CanOccupy(x, y, BodyRadius, p.Feet(), world.MaxStepUp, p.Headroom())
}

// canStandUp reports whether the ceiling above the player leaves room to
// stand (crawl spaces keep the player crouched/prone).
func (w *World) canStandUp() bool {
	_, ceil := w.Geo.Support(w.Player.X, w.Player.Y, BodyRadius)
	return ceil-w.Player.Feet() >= world.HeadroomStand-1e-9
}

// applyFallDamage consumes the last landing and hurts the player (map
// FALL_DAMAGE); scripts get onPlayerFall(height, damage).
func (w *World) applyFallDamage() {
	fall := w.Player.LastFall
	w.Player.LastFall = 0
	if fall <= 0 || w.Level == nil {
		return
	}
	dmg := math.Round(w.Level.FallDamage(fall))
	if dmg <= 0 {
		return
	}
	w.ApplyDamage(PlayerID, Damage{Amount: dmg, Kind: KindFall})
	w.Script.Emit("onPlayerFall", fall, dmg)
}

// updateSession runs pickups, zones, triggers and movers after the player
// moved. A player standing on a moving floor rides it.
func (w *World) updateSession(dt float64) {
	p := w.Player
	before, _ := w.Level.Support(p.X, p.Y, BodyRadius)
	events := w.Session.Update(dt, p.X, p.Y)
	after, _ := w.Level.Support(p.X, p.Y, BodyRadius)
	p.Ride(before, after)
	for _, ev := range events {
		if ev.Kind == level.EventMover && ev.Mover.Kind == level.MoverCrush {
			w.ApplyDamage(ev.Mover.Body, Damage{Amount: ev.Mover.Damage, Kind: KindCrush})
		}
		w.push(Event{Kind: EventLevel, Level: ev})
	}
}

// moverBodies lists the player and the living actors for the movers.
func (w *World) moverBodies(px, py float64, buf []level.Body) []level.Body {
	buf = append(buf, level.Body{ID: level.PlayerBody, X: px, Y: py, Radius: BodyRadius, Height: w.Player.Headroom()})
	for _, a := range w.Actors.All() {
		if a.Alive {
			buf = append(buf, level.Body{ID: a.ID, X: a.X, Y: a.Y, Radius: BodyRadius, Height: a.Height})
		}
	}
	return buf
}
