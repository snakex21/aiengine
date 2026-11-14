package weapons

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"aiengine/parser"
)

func parseWeapon(t *testing.T, src string) *Def {
	t.Helper()
	doc, err := parser.ParseAIE(strings.NewReader(src))
	if err != nil || len(doc.Nodes) == 0 {
		t.Fatalf("parse: %v", err)
	}
	d, err := ParseDef(doc.Nodes[0])
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestParseDefDocsKeys(t *testing.T) {
	d := parseWeapon(t, "WEAPON shotgun\nDAMAGE 15\nSPREAD 9\nAMMO_MAX 8\nBURST 8\nRANGE 10.0\nAUTO_FIRE yes\n")
	if d.Name != "shotgun" || d.Damage != 15 || math.Abs(d.Spread-9*math.Pi/180) > 1e-12 || d.Magazine != 8 ||
		d.Pellets != 8 || d.Range != 10 || !d.Auto || d.Kind != Hitscan {
		t.Fatalf("def %+v", d)
	}
}

const fireball = `WEAPON ognista_kula
  TITLE Ognista kula
  TYPE projectile
  DAMAGE 14
  RANGE 12
  FIRE_RATE 0.6
  AMMO none
  PROJECTILE_SPEED 5.5
  PROJECTILE_GRAVITY 0.5
  PROJECTILE_LIFETIME 4
  PROJECTILE_SPRITE orb
  PROJECTILE_SCALE 0.35
  PROJECTILE_LIGHT 0.9
  PROJECTILE_LIGHT_RADIUS 3
  COLOR #FF8030
  SPLASH_RADIUS 1.2
  SPLASH_FALLOFF 0.25
  KNOCKBACK 2.5
  MUZZLE_LIGHT 0.4
  SOUND_FIRE kula_rzut
  SOUND_EXPLODE kula_wybuch
`

// A definition survives Format -> parse unchanged (all keys).
func TestDefFormatRoundTrip(t *testing.T) {
	d := parseWeapon(t, fireball)
	if d.Kind != Projectile || d.Color != "#FF8030" || d.Splash != 1.2 || d.Gravity != 0.5 || d.Ammo != "" || d.Title != "Ognista kula" {
		t.Fatalf("parsed %+v", d)
	}
	cross := &Def{Name: "kusza", Title: "Kusza", Kind: Hitscan, Slot: 2, Damage: 40, Range: 14, Spread: 0.01,
		Pellets: 1, FireRate: 1.2, Magazine: 1, ReloadTime: 0.9, Ammo: "belty", AmmoPerShot: 1, StartAmmo: 6,
		MaxAmmo: 30, Speed: 10, Lifetime: 3, Radius: 0.1, Sprite: "orb", Scale: 0.3, LightRadius: 2.5,
		Tracer: true, MuzzleLight: 0.5, View: "kusza", SoundFire: "kusza_strzal", SoundReload: "kusza_naciag"}
	for _, want := range []*Def{d, cross} {
		got := parseWeapon(t, want.Format())
		if math.Abs(got.Spread-want.Spread) > 1e-12 {
			t.Fatalf("spread %v != %v", got.Spread, want.Spread)
		}
		got.Spread = want.Spread
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("round trip:\n%+v\n%+v\n%s", got, want, want.Format())
		}
	}
}

func TestParseDefReportsUnknownKeys(t *testing.T) {
	doc, _ := parser.ParseAIE(strings.NewReader("WEAPON x\nDAMAGE ten\nWOBBLE 3\n"))
	if _, err := ParseDef(doc.Nodes[0]); err == nil || !strings.Contains(err.Error(), "WOBBLE") || !strings.Contains(err.Error(), "ten") {
		t.Fatalf("err = %v", err)
	}
}

// Fire rate and reload only advance with Update (game time): a paused game
// that stops calling Update keeps the weapon exactly where it was.
func TestCooldownAndReloadRunOnGameTime(t *testing.T) {
	pool := Pool{"b": 4}
	w := NewInstance(&Def{Name: "p", FireRate: 2, Magazine: 2, ReloadTime: 1, Pellets: 1, AmmoPerShot: 1, Ammo: "b"})
	if w.Fire(pool) != 1 || w.Mag != 1 {
		t.Fatalf("first shot: mag %d", w.Mag)
	}
	if w.Fire(pool) != 0 {
		t.Fatal("fired during the cooldown")
	}
	if w.Ready(pool) { // paused: no Update calls, still cooling down
		t.Fatal("ready without game time passing")
	}
	w.Update(0.5, pool)
	if w.Fire(pool) != 1 || w.Mag != 0 || !w.Reloading {
		t.Fatalf("second shot should empty the magazine and start a reload: %+v", w)
	}
	w.Update(0.6, pool)
	if !w.Reloading || w.Mag != 0 {
		t.Fatal("reload finished too early")
	}
	w.Update(0.5, pool)
	if w.Reloading || w.Mag != 2 || pool["b"] != 2 {
		t.Fatalf("after reload: %+v pool %v", w, pool)
	}
}

func TestPoolWeaponsAndUnlimitedAmmo(t *testing.T) {
	pool := Pool{"r": 2}
	w := NewInstance(&Def{Name: "r", FireRate: 10, Ammo: "r", Pellets: 1, AmmoPerShot: 1})
	for i := 0; i < 2; i++ {
		if w.Fire(pool) != 1 {
			t.Fatalf("shot %d failed", i)
		}
		w.Update(0.2, pool)
	}
	if w.Fire(pool) != 0 || w.Reload(pool) || !w.Empty(pool) {
		t.Fatal("fired or reloaded with an empty pool")
	}
	wand := NewInstance(&Def{Name: "wand", FireRate: 100, Pellets: 1, AmmoPerShot: 1})
	for i := 0; i < 50; i++ {
		if wand.Fire(nil) != 1 {
			t.Fatal("unlimited weapon ran dry")
		}
		wand.Update(0.02, nil)
	}
}

func TestLoadoutSlotsCycleAndAmmo(t *testing.T) {
	l := NewLoadout()
	wand := &Def{Name: "wand", Slot: 1, Pellets: 1, AmmoPerShot: 1}
	bow := &Def{Name: "bow", Slot: 2, Ammo: "bolts", StartAmmo: 5, MaxAmmo: 8, Magazine: 1, Pellets: 1, AmmoPerShot: 1}
	l.Give(bow)
	l.Give(wand)
	if l.CurrentWeapon().Def != bow || l.Weapons[0].Def != wand {
		t.Fatal("giving a weapon must keep the selection and sort by slot")
	}
	if !l.SelectSlot(1) || l.CurrentWeapon().Def != wand || l.SelectSlot(7) {
		t.Fatal("slot select")
	}
	l.Cycle(1)
	if l.CurrentWeapon().Def != bow {
		t.Fatal("cycle")
	}
	l.Cycle(-1)
	if l.CurrentWeapon().Def != wand {
		t.Fatal("cycle back")
	}
	if got := l.AddAmmo("bolts", 10); got != 3 || l.Ammo["bolts"] != 8 {
		t.Fatalf("ammo cap: added %d, have %d", got, l.Ammo["bolts"])
	}
	l.Give(bow) // a second pickup of the same weapon only adds ammo (capped)
	if len(l.Weapons) != 2 {
		t.Fatal("duplicate weapon")
	}
}
