package game

import (
	"log"
	"math"

	"aiengine/anim"
	"aiengine/level"
)

// PlayerID is the id of the player as a damage target and source.
const PlayerID = level.PlayerBody

// Damage kinds.
const (
	KindMelee  = "melee"
	KindShot   = "shot"
	KindSplash = "splash"
	KindCrush  = "crush"
	KindFall   = "fall"
	KindScript = "script"
)

// Damage describes one hit.
type Damage struct {
	Amount    float64
	Kind      string
	Source    string  // attacker id ("player", an actor id, "" = the level)
	X, Y      float64 // where it came from (knockback pushes away from it)
	Knockback float64 // push speed in map units per second
	Sound     string  // hurt sound for the player ("" = player_hurt)
}

// ApplyDamage is the one place damage is applied, to the player
// (target "player") or an actor. God mode protects the player from every
// kind of damage. It returns the HP actually removed and queues
// EventDamage/EventDeath.
func (w *World) ApplyDamage(target string, d Damage) float64 {
	if d.Amount <= 0 {
		return 0
	}
	if target == PlayerID {
		return w.damagePlayer(d)
	}
	a := w.Actors.Get(target)
	if a == nil {
		return 0
	}
	dealt, died := a.TakeDamage(d.Amount)
	if dealt <= 0 {
		return 0
	}
	if d.Knockback > 0 {
		knock(&a.PushX, &a.PushY, a.X-d.X, a.Y-d.Y, d.Knockback)
	}
	e, _ := a.Ctl.(*Enemy)
	if e != nil {
		e.hurt(died)
	}
	w.push(Event{Kind: EventDamage, Target: a.ID, Type: a.Type, Source: d.Source, Amount: dealt, X: a.X, Y: a.Y})
	if died {
		w.push(Event{Kind: EventDeath, Target: a.ID, Type: a.Type, Source: d.Source, X: a.X, Y: a.Y})
	}
	return dealt
}

// knock adds a push of the given speed along (dx, dy).
func knock(vx, vy *float64, dx, dy, speed float64) {
	if n := math.Hypot(dx, dy); n > 1e-6 {
		*vx += dx / n * speed
		*vy += dy / n * speed
	}
}

func (w *World) damagePlayer(d Damage) float64 {
	if w.God != nil && w.God() {
		return 0
	}
	p := w.Player
	switch d.Kind {
	case KindMelee:
		log.Printf("[Combat] %s hits player for %.0f", d.Source, d.Amount)
		w.HurtFlash, w.Shake = 0.6, 0.25
	case KindFall:
		w.HurtFlash, w.Shake = math.Min(0.8, 0.3+d.Amount/50), 0.2
	default:
		w.HurtFlash, w.Shake = math.Min(0.8, 0.3+d.Amount/40), 0.25
	}
	snd := d.Sound
	if snd == "" {
		snd = "player_hurt"
	}
	if w.Script != nil {
		w.Script.PlaySound(snd)
	}
	if d.Knockback > 0 {
		knock(&w.pushX, &w.pushY, p.X-d.X, p.Y-d.Y, d.Knockback)
	}
	before := w.Script.GetPlayerHP()
	w.Script.SetPlayerHP(before - d.Amount)
	dealt := before - w.Script.GetPlayerHP()
	w.push(Event{Kind: EventDamage, Target: PlayerID, Source: d.Source, Amount: dealt, X: p.X, Y: p.Y})
	return dealt
}

// hurt shows pain or death on an enemy that took damage.
func (e *Enemy) hurt(died bool) {
	if e.Sprite != nil {
		e.Sprite.Alpha = 0.75
	}
	if died {
		e.showDeath()
		return
	}
	if e.Anim != nil {
		e.painT = enemyPainTime
		e.Anim.Restart(anim.Pain)
	}
}
