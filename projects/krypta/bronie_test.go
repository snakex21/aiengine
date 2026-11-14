// Weapons of the Krypta campaign played through the engine's game world
// (package game), headlessly: the wand, the crossbow and its bolts, the
// wraith's fireballs, and that every boss can be shot dead.
package krypta_test

import (
	"math"
	"os"
	"strings"
	"testing"

	gm "aiengine/game"
	"aiengine/level"
	"aiengine/parser"
	"aiengine/scripting"
	"aiengine/weapons"
	"aiengine/world"
)

func projectWeapons(t *testing.T) map[string]*weapons.Def {
	t.Helper()
	f, err := os.Open("weapons.aie")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := parser.ParseAIE(f)
	if err != nil {
		t.Fatal(err)
	}
	defs, err := weapons.FromDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	return defs
}

type armedGame struct {
	w   *gm.World
	lua *scripting.LuaBridge
	lv  *level.Level
}

// playMap runs a level like the host: real weapons, PLAYER_WEAPONS from
// the manifest, Lua game logic, enemies in the world.
func playMap(t *testing.T, path string) *armedGame {
	t.Helper()
	lv, err := level.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	eng := scripting.NewEngine()
	eng.Weapons = projectWeapons(t)
	for k, d := range lv.Weapons {
		eng.Weapons[k] = d
	}
	eng.GiveWeapon("rozdzka")
	p := world.NewPlayerState(lv.SpawnX, lv.SpawnY, lv.SpawnAngle)
	w := gm.New(lv, nil, p, eng, nil)
	w.PlacePlayer(lv.SpawnX, lv.SpawnY)
	w.AttachSession(level.NewSession(lv, eng))
	eng.OnSpawn = func(id, typ string, x, y float64) {
		params := map[string]string{}
		for k, v := range lv.EnemyTypes[typ] {
			params[k] = v
		}
		w.SpawnEnemy(level.Placement{Kind: "enemy", Type: typ, ID: id, X: x, Y: y, Params: params})
	}
	for _, pl := range lv.ByKind("enemy") {
		w.SpawnEnemy(pl)
	}
	lua := scripting.NewLuaBridge(eng)
	t.Cleanup(lua.Close)
	if err := lua.DoFile("scripts/krypta.lua"); err != nil {
		t.Fatal(err)
	}
	eng.Emit("onStart")
	return &armedGame{w: w, lua: lua, lv: lv}
}

func (g *armedGame) enemy(t *testing.T, id string) *gm.Enemy {
	t.Helper()
	for _, e := range g.w.Enemies() {
		if e.A.ID == id {
			return e
		}
	}
	t.Fatalf("no enemy %s", id)
	return nil
}

// aim turns the player towards an enemy's aim point and returns the
// input slope for it.
func (g *armedGame) aim(e *gm.Enemy) float64 {
	p := g.w.Player
	p.Angle = math.Atan2(e.A.Y-p.Y, e.A.X-p.X)
	d := math.Hypot(e.A.X-p.X, e.A.Y-p.Y)
	return (g.w.AimHeight(e) - p.EyeZ() + 0.05) / d
}

// shootUntilDead fires at an enemy (re-aiming every frame) for at most
// seconds of game time; it returns the time it took.
func (g *armedGame) shootUntilDead(t *testing.T, e *gm.Enemy, seconds float64) float64 {
	t.Helper()
	const dt = 1.0 / 60
	for tm := 0.0; tm < seconds; tm += dt {
		if !e.A.Alive {
			return tm
		}
		g.w.Script.Advance(dt)
		g.w.Update(dt, gm.Input{FireHeld: true, Fire: true, AimSlope: g.aim(e)})
		g.w.ClearEvents()
	}
	t.Fatalf("%s survived %.0f s of fire (HP %.0f)", e.A.ID, seconds, e.A.HP)
	return 0
}

func TestKryptaWeaponDefinitions(t *testing.T) {
	defs := projectWeapons(t)
	wand, bow, fire := defs["rozdzka"], defs["kusza"], defs["ognista_kula"]
	if wand == nil || wand.Kind != weapons.Projectile || wand.Light <= 0 || wand.Ammo != "" {
		t.Fatalf("wand %+v", wand)
	}
	if bow == nil || bow.Kind != weapons.Hitscan || !bow.Tracer || bow.Ammo != "belty" || bow.Magazine != 1 {
		t.Fatalf("crossbow %+v", bow)
	}
	if fire == nil || fire.Splash <= 0 || fire.Light <= 0 {
		t.Fatalf("fireball %+v", fire)
	}
	for _, d := range defs {
		doc, err := parser.ParseAIE(stringsReader(d.Format()))
		if err != nil {
			t.Fatal(err)
		}
		back, err := weapons.ParseDef(doc.Nodes[0])
		if err != nil || back.Format() != d.Format() {
			t.Fatalf("%s does not round-trip: %v", d.Name, err)
		}
	}
	// Every ranged enemy names a defined weapon.
	_, c := campaignLevels(t)
	for _, l := range c.Levels {
		lv, err := level.LoadFile(l.Path)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range lv.ByKind("enemy") {
			if name := p.Params["weapon"]; name != "" && defs[name] == nil {
				t.Errorf("%s/%s: unknown weapon %q", l.ID, p.ID, name)
			}
		}
	}
}

// The wand kills a guardian (50 HP, 25 per bolt of light) and the script
// gets onEnemyDead (score +50).
func TestWandKillsGuardian(t *testing.T) {
	g := playMap(t, "maps/krypta.aie")
	g.w.God = func() bool { return true }
	s3 := g.enemy(t, "s3")
	g.w.PlacePlayer(s3.A.X-2.2, s3.A.Y)
	s3.A.Frozen = true // stand still for the test
	g.shootUntilDead(t, s3, 5)
	g.w.Update(1.0/60, gm.Input{})
	if score := g.w.Script.GetVar("score"); score != 50.0 {
		t.Fatalf("score after the kill = %v", score)
	}
}

// Picking up the crossbow selects it with 6 bolts; a shot spends the bolt,
// reloads from the pool and kills a guardian in one hit (50 damage).
func TestCrossbowPickupBoltsAndShot(t *testing.T) {
	g := playMap(t, "maps/krypta.aie")
	g.w.God = func() bool { return true }
	lo := g.w.Script.Loadout
	pick := func(id string) {
		for _, p := range g.lv.Placements {
			if p.ID == id {
				g.w.PlacePlayer(p.X, p.Y)
			}
		}
		g.w.Update(1.0/60, gm.Input{})
		g.w.ClearEvents()
	}
	pick("kusza")
	if cur := lo.CurrentWeapon(); cur == nil || cur.Def.Name != "kusza" || lo.Ammo["belty"] != 6 || cur.Mag != 1 {
		t.Fatalf("after pickup: %+v pool %v", cur, lo.Ammo)
	}
	pick("b1")
	if lo.Ammo["belty"] != 12 {
		t.Fatalf("bolt bundle: pool %v", lo.Ammo)
	}
	s3 := g.enemy(t, "s3")
	s3.A.Frozen = true
	g.w.PlacePlayer(s3.A.X, s3.A.Y-2)
	g.w.Update(1.0/60, gm.Input{Fire: true, AimSlope: g.aim(s3)})
	if s3.A.Alive {
		t.Fatalf("crossbow bolt did not kill the guardian (HP %v)", s3.A.HP)
	}
	for i := 0; i < 60; i++ {
		g.w.Update(1.0/60, gm.Input{})
	}
	if lo.CurrentWeapon().Mag != 1 || lo.Ammo["belty"] != 11 {
		t.Fatalf("after one shot and a reload: mag %d pool %d", lo.CurrentWeapon().Mag, lo.Ammo["belty"])
	}
	g.w.Update(1.0/60, gm.Input{Slot: 1})
	if lo.CurrentWeapon().Def.Name != "rozdzka" {
		t.Fatal("key 1 should select the wand")
	}
}

// The wraith in the flooded mine keeps its distance and its fireballs hurt;
// it can still be shot down with the wand.
func TestWraithFireballsHurtAndItCanBeKilled(t *testing.T) {
	g := playMap(t, "maps/zalana_kopalnia.aie")
	u := g.enemy(t, "u1")
	if u.Weapon == nil || u.Weapon.Def.Name != "ognista_kula" {
		t.Fatal("wraith without its fireball")
	}
	g.w.PlacePlayer(u.A.X-4, u.A.Y) // in its tunnel, in sight past the pillar
	hp := g.w.Script.GetPlayerHP()
	fireballs := 0
	for i := 0; i < 60*8 && g.w.Script.GetPlayerHP() == hp; i++ {
		g.w.Script.Advance(1.0 / 60)
		g.w.Update(1.0/60, gm.Input{})
		for _, ev := range g.w.Events() {
			if ev.Kind == gm.EventShot && ev.Source == "u1" {
				fireballs++
			}
		}
		g.w.ClearEvents()
	}
	if fireballs == 0 || g.w.Script.GetPlayerHP() >= hp {
		t.Fatalf("fireballs %d, HP %v -> %v", fireballs, hp, g.w.Script.GetPlayerHP())
	}
	g.w.God = func() bool { return true }
	g.shootUntilDead(t, u, 15)
}

// Completability with weapons: every boss can be killed with the wand the
// player always has (unlimited ammo), and its death opens the way.
func TestEveryBossCanBeShotDead(t *testing.T) {
	_, c := campaignLevels(t)
	for _, l := range c.Levels {
		g := playMap(t, l.Path)
		boss := g.lv.Props["VAR_BOSS"]
		if boss == "" {
			continue
		}
		g.w.God = func() bool { return true }
		e := g.enemy(t, boss)
		g.w.PlacePlayer(e.A.X, e.A.Y+3)
		took := g.shootUntilDead(t, e, 60)
		if g.w.Script.GetVar("score").(float64) < 1000 {
			t.Fatalf("%s: boss death not scored", l.ID)
		}
		t.Logf("%s: boss %s down in %.1f s of wand fire", l.ID, boss, took)
	}
}

func stringsReader(s string) *strings.Reader { return strings.NewReader(s) }

// The shooting-range test map loads with its own WEAPON block on top of
// the project's weapons.
func TestShootingRangeMap(t *testing.T) {
	g := playMap(t, "maps/testy/bronie.aie")
	if d := g.lv.Weapons["rakieta"]; d == nil || d.Splash != 1.8 || d.Ammo != "rakiety" {
		t.Fatalf("map weapon %+v", d)
	}
	if len(g.lv.ByKind("enemy")) != 3 {
		t.Fatal("enemies")
	}
}
