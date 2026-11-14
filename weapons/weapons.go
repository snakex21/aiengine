// Package weapons holds weapon definitions (WEAPON blocks in .aie files) and
// the live state of a held weapon: magazine, reload and fire-rate cooldown.
//
// All timers run on game time: the host calls Update(dt) once per simulated
// frame and not while the game is paused, so pausing also pauses the fire
// rate and reloads (the old gameplay version used the wall clock).
package weapons

import (
	"strconv"
	"strings"

	"aiengine/parser"
)

// Def is a weapon template.
type Def struct {
	Name        string
	Damage      float64
	Range       float64
	FireRate    float64 // shots per second
	ReloadTime  float64 // seconds
	Magazine    int     // rounds per magazine (0 = fires straight from the reserve)
	AmmoReserve int     // spare rounds a new weapon comes with
	Spread      float64 // cone half-angle, radians
	Pellets     int     // projectiles/rays per shot (shotgun), at least 1
	Auto        bool    // keeps firing while the trigger is held
}

// ParseDef builds a Def from a WEAPON node (keys as in docs/engine.md:
// DAMAGE, RANGE, FIRE_RATE, RELOAD_TIME, AMMO_MAX, AMMO_RESERVE, SPREAD,
// BURST, AUTO_FIRE). Missing keys keep the defaults.
func ParseDef(node *parser.AIENode) *Def {
	kv := Flatten(node)
	d := &Def{Name: node.Name, Range: 50, FireRate: 1, ReloadTime: 1.5, Magazine: 10, AmmoReserve: 30, Pellets: 1}
	num := func(key string, dst *float64) {
		if v, err := strconv.ParseFloat(kv[key], 64); err == nil {
			*dst = v
		}
	}
	whole := func(key string, dst *int) {
		if v, err := strconv.Atoi(kv[key]); err == nil {
			*dst = v
		}
	}
	num("DAMAGE", &d.Damage)
	num("RANGE", &d.Range)
	num("FIRE_RATE", &d.FireRate)
	num("RELOAD_TIME", &d.ReloadTime)
	whole("AMMO_MAX", &d.Magazine)
	whole("AMMO_RESERVE", &d.AmmoReserve)
	num("SPREAD", &d.Spread)
	whole("BURST", &d.Pellets)
	d.Auto = isYes(kv["AUTO_FIRE"])
	if d.Pellets < 1 {
		d.Pellets = 1
	}
	return d
}

// Flatten returns a node's properties and single-value children (the .aie
// parser turns some keys, e.g. TYPE, into children) keyed in upper case.
func Flatten(node *parser.AIENode) map[string]string {
	out := make(map[string]string, len(node.Properties)+len(node.Children))
	for k, v := range node.Properties {
		out[strings.ToUpper(k)] = strings.TrimSpace(v)
	}
	for _, c := range node.Children {
		out[strings.ToUpper(c.Type)] = strings.TrimSpace(c.Name)
	}
	return out
}

func isYes(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "yes", "true", "1", "tak", "on":
		return true
	}
	return false
}

// Instance is a weapon in someone's hands.
type Instance struct {
	Def       *Def
	Mag       int // rounds in the magazine
	Reserve   int // spare rounds
	Reloading bool

	reloadLeft float64 // seconds until the reload completes
	cooldown   float64 // seconds until the next shot is allowed
}

// NewInstance creates a weapon with a full magazine and the default reserve.
func NewInstance(def *Def) *Instance {
	return &Instance{Def: def, Mag: def.Magazine, Reserve: def.AmmoReserve}
}

// Ready reports whether a shot can be fired now.
func (w *Instance) Ready() bool {
	if w.Reloading || w.cooldown > 0 {
		return false
	}
	if w.Def.Magazine > 0 {
		return w.Mag > 0
	}
	return w.Reserve > 0
}

// Fire spends one round and starts the cooldown. It returns the number of
// pellets fired (0 when the weapon is not ready). An empty magazine starts
// a reload.
func (w *Instance) Fire() int {
	if !w.Ready() {
		return 0
	}
	if w.Def.Magazine > 0 {
		w.Mag--
	} else {
		w.Reserve--
	}
	if w.Def.FireRate > 0 {
		w.cooldown = 1 / w.Def.FireRate
	}
	if w.Def.Magazine > 0 && w.Mag <= 0 {
		w.Reload()
	}
	return w.Def.Pellets
}

// Reload starts a reload when the magazine is not full and spare rounds
// are left. It reports whether a reload started.
func (w *Instance) Reload() bool {
	if w.Reloading || w.Def.Magazine <= 0 || w.Mag >= w.Def.Magazine || w.Reserve <= 0 {
		return false
	}
	w.Reloading = true
	w.reloadLeft = w.Def.ReloadTime
	return true
}

// Update advances the cooldown and the reload by dt seconds of game time.
func (w *Instance) Update(dt float64) {
	if w.cooldown > 0 {
		w.cooldown -= dt
	}
	if !w.Reloading {
		return
	}
	w.reloadLeft -= dt
	if w.reloadLeft > 0 {
		return
	}
	need := min(w.Def.Magazine-w.Mag, w.Reserve)
	w.Mag += need
	w.Reserve -= need
	w.Reloading, w.reloadLeft = false, 0
}
