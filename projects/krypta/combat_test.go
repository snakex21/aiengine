package krypta_test

import (
	"math"
	"testing"

	"aiengine/level"
	"aiengine/nav"
	"aiengine/progress"
	"aiengine/scripting"
	"aiengine/settings"
	"aiengine/ui"
)

func blockedBy(lv *level.Level) func(x, y float64) bool {
	return func(x, y float64) bool {
		for _, o := range [][2]float64{{0, 0}, {0.15, 0}, {-0.15, 0}, {0, 0.15}, {0, -0.15}} {
			c := nav.CellAt(x+o[0], y+o[1])
			if lv.Solid(c.X, c.Y) {
				return true
			}
		}
		return false
	}
}

// spotNextTo finds a free player position 0.6 from (x, y) with line of sight.
func spotNextTo(t *testing.T, lv *level.Level, x, y float64) (float64, float64) {
	t.Helper()
	for a := 0.0; a < 2*math.Pi; a += math.Pi / 8 {
		px, py := x+0.6*math.Cos(a), y+0.6*math.Sin(a)
		if !blockedBy(lv)(px, py) && nav.LineClear(lv, x, y, px, py, 0) {
			return px, py
		}
	}
	t.Fatalf("no free spot next to %.1f,%.1f", x, y)
	return 0, 0
}

// hitsIn is the number of hits a fighter lands in T seconds when the player
// stays in reach: the first after the wind-up, then one per (rate + wind-up).
func hitsIn(f *level.Fighter, T float64) int {
	return int((T-f.Windup)/(f.Rate+f.Windup)) + 1
}

// Every enemy placed in every campaign level must hurt a player standing
// next to it, by the expected amount.
func TestEveryEnemyDealsDamage(t *testing.T) {
	_, c := campaignLevels(t)
	for _, l := range c.Levels {
		lv, err := level.LoadFile(l.Path)
		if err != nil {
			t.Fatal(err)
		}
		for i, p := range lv.ByKind("enemy") {
			f := level.NewFighter(p, int64(i+1))
			if f.Damage <= 0 {
				t.Errorf("%s/%s: no damage configured", l.ID, p.ID)
				continue
			}
			px, py := spotNextTo(t, lv, p.X, p.Y)
			const T = 5.0
			dealt := 0.0
			for tm := 0.0; tm < T; tm += 0.02 {
				dealt += f.Step(lv, px, py, 0.02, blockedBy(lv))
			}
			want := float64(hitsIn(f, T)) * f.Damage
			if math.Abs(dealt-want) > f.Damage+0.01 {
				t.Errorf("%s/%s (%s, %s): dealt %.0f in %.0f s, want about %.0f", l.ID, p.ID, p.Type,
					p.Params["behavior"], dealt, T, want)
			}
			if d := math.Hypot(px-f.X, py-f.Y); d < level.Separation-0.01 {
				t.Errorf("%s/%s overlaps the player (%.2f)", l.ID, p.ID, d)
			}
		}
	}
}

// Enemies that have to walk to the player (all behaviours, from a few cells
// away, player hugging a wall) still reach and hurt them.
func TestEnemiesChaseAndHitPlayerAgainstWall(t *testing.T) {
	lv, err := level.LoadFile("maps/serce_krypty.aie")
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"pajak", "straznik", "wladca"} {
		for _, beh := range []string{"chase", "guard", "wander", "ambush"} {
			params := map[string]string{"behavior": beh}
			for k, v := range lv.EnemyTypes[typ] {
				if k != "behavior" {
					params[k] = v
				}
			}
			f := level.NewFighter(level.Placement{ID: typ, Type: typ, X: 5.5, Y: 6.5, Params: params}, 3)
			px, py := 2.84, 6.5 // pressed against a pillar-free stretch near the west wall
			if beh == "ambush" {
				px = 4.5 // ambushers only wake at half their sight range
			}
			dealt := 0.0
			for tm := 0.0; tm < 12; tm += 0.02 {
				dealt += f.Step(lv, px, py, 0.02, blockedBy(lv))
			}
			if dealt <= 0 {
				t.Errorf("%s/%s never hit the player (ended at %.2f,%.2f)", typ, beh, f.X, f.Y)
			}
		}
	}
}

// Spiders summoned by the boss script (spawnEnemy) use their template and
// deal damage; the player's HP drops in the engine and death loses the level.
func TestBossSummonedSpidersHurtAndKill(t *testing.T) {
	lv, err := level.LoadFile("maps/serce_krypty.aie")
	if err != nil {
		t.Fatal(err)
	}
	eng := scripting.NewEngine()
	s := level.NewSession(lv, eng)
	var fighters []*level.Fighter
	eng.OnSpawn = func(id, typ string, x, y float64) {
		params := map[string]string{}
		for k, v := range lv.EnemyTypes[typ] {
			params[k] = v
		}
		fighters = append(fighters, level.NewFighter(level.Placement{ID: id, Type: typ, X: x, Y: y, Params: params}, 7))
	}
	lua := scripting.NewLuaBridge(eng)
	defer lua.Close()
	if err := lua.DoFile("scripts/krypta.lua"); err != nil {
		t.Fatal(err)
	}
	eng.Emit("onStart")
	px, py := 4.5, 7.0 // near the first summon point (4.5, 5.5)
	for tm := 0.0; tm < 60 && s.State == level.Playing; tm += 0.05 {
		eng.Advance(0.05)
		s.Update(0.05, px, py)
		for _, f := range fighters {
			if d := f.Step(lv, px, py, 0.05, blockedBy(lv)); d > 0 {
				eng.SetPlayerHP(eng.GetPlayerHP() - d)
			}
		}
	}
	if len(fighters) == 0 || fighters[0].Type != "pajak" || fighters[0].Damage != 6 {
		t.Fatalf("summoned %d fighters: %+v", len(fighters), fighters)
	}
	if s.State != level.Lost || eng.GetPlayerHP() != 0 {
		t.Fatalf("player should have been killed by spiders: state %v HP %.0f", s.State, eng.GetPlayerHP())
	}
}

// Damage keeps working after a level transition (carried HP) and stops
// while the pause menu is open, resuming afterwards.
func TestDamageAfterTransitionAndPause(t *testing.T) {
	_, c := campaignLevels(t)
	prog := progress.New(c)
	prog.NewGame(c)
	prog.Complete(c, "przedsionek", 60, progress.State{HP: 70})

	lv, err := level.LoadFile("maps/krypta.aie")
	if err != nil {
		t.Fatal(err)
	}
	eng := scripting.NewEngine()
	level.NewSession(lv, eng)
	prog.Carry.Apply(eng)
	if eng.GetPlayerHP() != 70 {
		t.Fatalf("carried HP %v", eng.GetPlayerHP())
	}
	p := lv.ByKind("enemy")[0]
	f := level.NewFighter(p, 1)
	px, py := spotNextTo(t, lv, p.X, p.Y)

	s := settings.Defaults(1280, 720)
	menu, _ := loadMenu(t, &s)
	frame := func(dt float64) {
		if menu.PausesGame() { // the host skips the whole simulation step
			return
		}
		if d := f.Step(lv, px, py, dt, blockedBy(lv)); d > 0 {
			eng.SetPlayerHP(eng.GetPlayerHP() - d)
		}
	}
	for i := 0; i < 50; i++ { // 1 s
		frame(0.02)
	}
	afterPlay := eng.GetPlayerHP()
	if afterPlay >= 70 {
		t.Fatal("no damage after the level transition")
	}
	menu.OpenRoot("pause")
	for i := 0; i < 500; i++ { // 10 s paused
		frame(0.02)
	}
	if eng.GetPlayerHP() != afterPlay {
		t.Fatal("damage while paused")
	}
	menu.Input(ui.Accept) // Wznów
	for i := 0; i < 100; i++ {
		frame(0.02)
	}
	if eng.GetPlayerHP() >= afterPlay {
		t.Fatal("no damage after resuming")
	}
}
