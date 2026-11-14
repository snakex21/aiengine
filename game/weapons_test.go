package game

import (
	"math"
	"strings"
	"testing"

	"aiengine/actor"
	"aiengine/parser"
	"aiengine/weapons"
)

// corridor: 14x5, player at (1.5, 2.5) looking east, a pillar at (6, 1).
const corridorMap = `MAP corridor
PLAYER_ANGLE 0
GRID
W W W W W W W W W W W W W W
W . . . . . W . . . . . . W
W P . . . . . . . . . . . W
W . . . . . . . . . . . . W
W W W W W W W W W W W W W W
`

// sectorCorridor is the same corridor as a sector map (no pillar): x 1..13,
// y 1..4, player at (1.5, 2.5).
const sectorCorridor = `MAP sektor
PLAYER_START 1.5 2.5
PLAYER_ANGLE 0
SECTOR a floor=0 ceil=1.5 : 1 1 7 1 7 4 1 4
SECTOR b floor=0 ceil=1.5 : 7 1 13 1 13 4 7 4
`

func defs(t *testing.T, src string) map[string]*weapons.Def {
	t.Helper()
	doc, err := parser.ParseAIE(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	m, err := weapons.FromDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

const testWeapons = `
WEAPON rifle
  TYPE hitscan
  SLOT 1
  DAMAGE 10
  RANGE 6
  FIRE_RATE 4
  AUTO_FIRE yes
  MAGAZINE 4
  RELOAD_TIME 1
  AMMO bullets
  START_AMMO 8
  TRACER yes
WEAPON shotgun
  TYPE hitscan
  SLOT 2
  DAMAGE 2
  RANGE 20
  SPREAD 12
  PELLETS 200
  AMMO none
WEAPON rocket
  TYPE projectile
  SLOT 3
  DAMAGE 100
  RANGE 30
  AMMO none
  FIRE_RATE 1
  PROJECTILE_SPEED 5
  PROJECTILE_LIGHT 1
  SPLASH_RADIUS 2
  SPLASH_FALLOFF 0.5
  KNOCKBACK 3
WEAPON fireball
  TYPE projectile
  DAMAGE 12
  RANGE 15
  AMMO none
  FIRE_RATE 0.8
  PROJECTILE_SPEED 6
`

// armed builds a world on a map with the test weapons, gives the player
// the named ones and selects the first.
func armed(t *testing.T, src string, give ...string) *World {
	t.Helper()
	w, _ := newTestWorld(t, src)
	w.Script.Weapons = defs(t, testWeapons)
	for _, n := range give {
		w.Script.GiveWeapon(n)
	}
	w.Script.SelectWeapon(give[0])
	return w
}

// dummy places a hostile actor (no brain) at (x, y): a 0.6 tall body on
// the floor, hit radius 0.3.
func dummy(w *World, id string, x, y, hp float64) *actor.Actor {
	a := &actor.Actor{ID: id, Type: "dummy", Faction: actor.Hostile, X: x, Y: y, Radius: 0.3, Height: 0.8, HP: hp, MaxHP: hp, Alive: true}
	w.Actors.Add(a)
	return a
}

func fire(w *World) { w.Update(1.0/60, Input{Fire: true}) }

func TestHitscanRangeAndWalls(t *testing.T) {
	w := armed(t, corridorMap, "rifle")
	near := dummy(w, "near", 6.5, 2.5, 100) // 5 cells away, in range 6
	fire(w)
	if near.HP != 90 {
		t.Fatalf("in-range target HP %v", near.HP)
	}
	near.X = 8.5 // 7 cells: out of range
	step(w, 0.5, Input{})
	fire(w)
	if near.HP != 90 {
		t.Fatalf("out-of-range target hit (HP %v)", near.HP)
	}
	// Behind the pillar at (6, 1): the wall stops the ray.
	near.X, near.Y = 7.5, 1.5
	w.Player.X, w.Player.Y = 4.5, 1.5
	step(w, 0.5, Input{})
	fire(w)
	if near.HP != 90 {
		t.Fatal("shot through the pillar")
	}
	var tracers int
	for _, e := range w.effects {
		if e.Tex == TracerTexture(w.Script.Weapons["rifle"]) {
			tracers++
		}
	}
	if tracers == 0 {
		t.Fatal("no tracer effect")
	}
}

// Spread: a wide cone puts only part of the pellets on a narrow target,
// and the share falls with distance (about 1/d^2).
func TestHitscanSpread(t *testing.T) {
	w := armed(t, corridorMap, "shotgun")
	a := dummy(w, "t", 3.5, 2.5, 1e6)
	fire(w)
	hitNear := (1e6 - a.HP) / 2
	a.HP, a.X = 1e6, 9.5
	step(w, 1.1, Input{})
	fire(w)
	hitFar := (1e6 - a.HP) / 2
	if hitNear <= 0 || hitNear >= 200 || hitFar <= 0 || hitFar >= hitNear/3 {
		t.Fatalf("pellets on target: near %v far %v of 200", hitNear, hitFar)
	}
}

// Projectiles fly at their speed and stop at walls on grid and sector
// maps alike.
func TestProjectileFlightAndWallHit(t *testing.T) {
	for _, src := range []string{corridorMap, sectorCorridor} {
		w := armed(t, src, "rocket")
		fire(w)
		if len(w.Projectiles()) != 1 {
			t.Fatalf("%d projectiles", len(w.Projectiles()))
		}
		p := w.Projectiles()[0]
		x0 := p.X
		w.ClearEvents()
		w.Update(0.5, Input{})
		if math.Abs(p.X-x0-2.5) > 1e-6 || p.Sprite.X != p.X || !p.Sprite.AbsZ {
			t.Fatalf("after 0.5 s at speed 5: moved %.3f", p.X-x0)
		}
		var boom *Event
		for i := 0; i < 200 && boom == nil; i++ {
			w.ClearEvents()
			w.Update(1.0/60, Input{})
			for _, ev := range w.Events() {
				if ev.Kind == EventExplosion {
					ev := ev
					boom = &ev
				}
			}
		}
		if boom == nil || math.Abs(boom.X-13) > 0.1 || len(w.Projectiles()) != 0 {
			t.Fatalf("%s: explosion %+v", src[:12], boom)
		}
	}
}

// Floors and ceilings stop projectiles too (gravity drop / steep shot).
func TestProjectileHitsFloorAndCeiling(t *testing.T) {
	w := armed(t, sectorCorridor, "rocket")
	w.Script.Weapons["rocket"].Gravity = 20
	fire(w)
	var hitZ float64 = -1
	for i := 0; i < 120; i++ {
		w.ClearEvents()
		w.Update(1.0/60, Input{})
		for _, ev := range w.Events() {
			if ev.Kind == EventExplosion {
				hitZ = ev.Z
			}
		}
	}
	if math.Abs(hitZ-0.05) > 0.02 {
		t.Fatalf("falling rocket should burst on the floor, z=%v", hitZ)
	}
	w.Script.Weapons["rocket"].Gravity = 0
	step(w, 1.1, Input{})
	w.ClearEvents()
	w.Update(1.0/60, Input{Fire: true, AimSlope: 3})
	for i := 0; i < 60; i++ {
		w.Update(1.0/60, Input{})
	}
	found := false
	for _, ev := range w.Events() {
		if ev.Kind == EventExplosion && math.Abs(ev.Z-1.45) < 0.02 {
			found = true
		}
	}
	if !found {
		t.Fatalf("steep shot should burst at the 1.5 ceiling: %+v", w.Events())
	}
}

// Splash: damage falls from full at the centre to SplashFalloff at the
// edge; a wall between the blast and a body shields it.
func TestSplashFalloffAndLineOfSight(t *testing.T) {
	w := armed(t, corridorMap, "rocket")
	d := w.Script.Weapons["rocket"]
	shot := Shot{Def: d, Shooter: PlayerID, Faction: actor.Friendly}
	centre := dummy(w, "centre", 4.5, 2.5, 1000)
	mid := dummy(w, "mid", 4.5, 3.6, 1000)   // body edge 0.8 from the blast
	shield := dummy(w, "behind", 6.5, 1.5, 1000) // behind the pillar
	far := dummy(w, "far", 8.5, 2.5, 1000)
	w.Player.X = 1.5
	w.explode(shot, 4.5, 2.5, 0.5)
	if got := 1000 - centre.HP; math.Abs(got-100) > 1e-9 {
		t.Fatalf("centre took %v, want 100", got)
	}
	// mid: distance to its body = hypot(1.1-0.3, 0) = 0.8 -> k = 1 - 0.5*0.8/2 = 0.8
	if got := 1000 - mid.HP; math.Abs(got-80) > 1e-6 {
		t.Fatalf("mid took %v, want 80", got)
	}
	if shield.HP != 1000 || far.HP != 1000 {
		t.Fatalf("shielded %v, far %v must be untouched", shield.HP, far.HP)
	}
	if mid.PushY <= 0 {
		t.Fatal("knockback should push away from the blast")
	}
	// The same body in the open at the same distance is hit.
	shield.X, shield.Y = 5.5, 2.5 // 1 - 0.3 = 0.7 from the blast
	w.explode(shot, 4.5, 2.5, 0.5)
	if shield.HP >= 1000 {
		t.Fatal("unshielded body not hit")
	}
}

// Ammo, magazine, reload and fire rate run on game time: while the host
// does not update the world (pause) nothing advances.
func TestAmmoReloadAndFireRateUnderPause(t *testing.T) {
	w := armed(t, corridorMap, "rifle")
	lo := w.Script.Loadout
	shots := 0
	for i := 0; i < 60; i++ { // 1 s holding the trigger: 4 rounds, then a reload
		w.ClearEvents()
		w.Update(1.0/60, Input{FireHeld: true})
		for _, ev := range w.Events() {
			if ev.Kind == EventShot {
				shots++
			}
		}
	}
	wp := lo.CurrentWeapon()
	if shots != 4 || !wp.Reloading || wp.Mag != 0 || lo.Ammo["bullets"] != 8 {
		t.Fatalf("shots %d reloading %v mag %d pool %d", shots, wp.Reloading, wp.Mag, lo.Ammo["bullets"])
	}
	// Paused for a long time: no Update, so the reload does not progress.
	for i := 0; i < 1000; i++ {
		_ = wp.Ready(lo.Ammo)
	}
	if !wp.Reloading {
		t.Fatal("reload finished without game time")
	}
	step(w, 1.1, Input{})
	if wp.Reloading || wp.Mag != 4 || lo.Ammo["bullets"] != 4 {
		t.Fatalf("after reload: mag %d pool %d", wp.Mag, lo.Ammo["bullets"])
	}
	// Ammo pickups refill the pool; switching keys select by slot.
	w.Script.GiveAmmo("bullets", 3)
	if lo.Ammo["bullets"] != 7 {
		t.Fatal("ammo pickup")
	}
	w.Script.GiveWeapon("rocket")
	w.Update(1.0/60, Input{Slot: 3})
	if lo.CurrentWeapon().Def.Name != "rocket" {
		t.Fatal("slot 3 should select the rocket launcher")
	}
	w.Update(1.0/60, Input{Cycle: -1})
	if lo.CurrentWeapon().Def.Name != "rifle" {
		t.Fatal("wheel back should select the rifle")
	}
}

const rangedMap = `MAP ranged
PLAYER_ANGLE 0
GRID
W W W W W W W W W W W W
W P . . . . . . . . . W
W . . . . . . . . . . W
W W W W W W W W W W W W
PLACE enemy_type upior 0 0 0 hp=40 damage=0 sight=12 weapon=fireball windup=0.3
PLACE enemy upior u1 9.5 0 1.5
`

// A ranged enemy keeps its distance and its fireballs hurt the player.
func TestEnemyProjectileHitsPlayer(t *testing.T) {
	w, _ := newTestWorldWith(t, rangedMap, defs(t, testWeapons))
	u := w.Enemies()[0]
	if u.Weapon == nil {
		t.Fatal("upior has no weapon")
	}
	shots := 0
	for i := 0; i < 60*5 && w.Script.GetPlayerHP() == 100; i++ {
		w.ClearEvents()
		w.Update(1.0/60, Input{})
		for _, ev := range w.Events() {
			if ev.Kind == EventShot && ev.Source == "u1" {
				shots++
			}
		}
	}
	if shots == 0 || w.Script.GetPlayerHP() != 88 {
		t.Fatalf("shots %d, player HP %v (want 88 after one fireball)", shots, w.Script.GetPlayerHP())
	}
	if d := math.Hypot(u.A.X-w.Player.X, u.A.Y-w.Player.Y); d < u.Keep-0.01 {
		t.Fatalf("ranged enemy closed in to %.2f (keep %.2f)", d, u.Keep)
	}
	// God mode: fireballs do nothing.
	w.God = func() bool { return true }
	step(w, 5, Input{})
	if w.Script.GetPlayerHP() != 88 {
		t.Fatal("fireball hurt the player in god mode")
	}
}
