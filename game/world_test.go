package game

import (
	"testing"

	"aiengine/level"
	"aiengine/scripting"
	"aiengine/weapons"
	"aiengine/world"
)

const arenaMap = `MAP arena
GRID
W W W W W W W W W W
W P . . . . . . . W
W . . . . . . . . W
W . . . . . . . . W
W W W W W W W W W W
PLACE enemy_type grunt 0 0 0 hp=50 damage=10 sight=8
PLACE enemy grunt g1 6.5 0 1.5
`

// newTestWorld loads a map and runs it headlessly like the host does.
func newTestWorld(t *testing.T, src string) (*World, *scripting.LuaBridge) {
	t.Helper()
	return newTestWorldWith(t, src, nil)
}

// newTestWorldWith also registers weapon definitions before the enemies
// spawn (ranged enemies look up their weapon).
func newTestWorldWith(t *testing.T, src string, defs map[string]*weapons.Def) (*World, *scripting.LuaBridge) {
	t.Helper()
	lv, err := level.FromDocumentString(src)
	if err != nil {
		t.Fatal(err)
	}
	eng := scripting.NewEngine()
	if defs != nil {
		eng.Weapons = defs
	}
	p := world.NewPlayerState(lv.SpawnX, lv.SpawnY, lv.SpawnAngle)
	w := New(lv, nil, p, eng, nil)
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
	return w, lua
}

func step(w *World, seconds float64, in Input) {
	for t := 0.0; t < seconds; t += 1.0 / 60 {
		w.Script.Advance(1.0 / 60)
		w.Update(1.0/60, in)
		in.Fire, in.Jump, in.Reload, in.Interact = false, false, false, false
		w.ClearEvents()
	}
}

// Lua killEnemy/setEnemyHP used to be undone the next frame (the host
// copied its own HP back); now the actor stays dead and the script hears
// onEnemyDead(id, type) once.
func TestLuaKillEnemyStaysDead(t *testing.T) {
	w, lua := newTestWorld(t, arenaMap)
	if err := lua.DoString(`
		deaths = ""
		registerEvent("onEnemyDead", function(id, typ) deaths = deaths .. id .. ":" .. typ .. ";" end)
		killEnemy("g1")
		extra = spawnEnemy("grunt", 4.5, 3.5)
		setEnemyHP(extra, 0)
	`); err != nil {
		t.Fatal(err)
	}
	step(w, 1, Input{})
	for _, e := range w.Enemies() {
		if e.A.Alive || e.A.HP != 0 {
			t.Fatalf("%s alive after kill (HP %v)", e.A.ID, e.A.HP)
		}
	}
	if got := lua.L.GetGlobal("deaths").String(); got != "g1:grunt;grunt_1:grunt;" {
		t.Fatalf("onEnemyDead calls %q", got)
	}
	if !w.Script.AllEnemiesDeadInZone("") {
		t.Fatal("trigger count still sees living enemies")
	}
}

// One damage path: god mode blocks melee, crush and fall damage alike.
func TestApplyDamageGodModeAndEvents(t *testing.T) {
	w, _ := newTestWorld(t, arenaMap)
	god := true
	w.God = func() bool { return god }
	for _, kind := range []string{KindMelee, KindCrush, KindFall, KindSplash} {
		if d := w.ApplyDamage(PlayerID, Damage{Amount: 10, Kind: kind}); d != 0 {
			t.Fatalf("%s hurt the player in god mode", kind)
		}
	}
	god = false
	if d := w.ApplyDamage(PlayerID, Damage{Amount: 10, Kind: KindCrush}); d != 10 || w.Script.GetPlayerHP() != 90 {
		t.Fatalf("crush dealt %v, HP %v", d, w.Script.GetPlayerHP())
	}
	w.ApplyDamage("g1", Damage{Amount: 60, Kind: KindShot, Source: PlayerID})
	var kinds []EventKind
	for _, ev := range w.Events() {
		kinds = append(kinds, ev.Kind)
	}
	if len(kinds) != 3 || kinds[1] != EventDamage || kinds[2] != EventDeath || w.Events()[1].Amount != 50 {
		t.Fatalf("events %+v", w.Events())
	}
}

// Enemies chase and hit the player through the world update.
func TestEnemyHitsPlayerThroughWorld(t *testing.T) {
	w, _ := newTestWorld(t, arenaMap)
	step(w, 8, Input{})
	if w.Script.GetPlayerHP() >= 100 {
		t.Fatal("the grunt never hit the player")
	}
}
