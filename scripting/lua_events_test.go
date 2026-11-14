package scripting

import "testing"

// Lua callbacks registered with registerEvent must receive exactly the
// arguments passed to Emit, in order.
func TestLuaRegisterEventReceivesArgs(t *testing.T) {
	e := NewEngine()
	lb := NewLuaBridge(e)
	defer lb.Close()

	err := lb.DoString(`
		got = ""
		calls = 0
		registerEvent("onEnemyDead", function(id, typ)
			calls = calls + 1
			got = tostring(id) .. "/" .. tostring(typ)
		end)
		registerEvent("onStart", function() calls = calls + 10 end)
	`)
	if err != nil {
		t.Fatal(err)
	}
	e.Emit("onEnemyDead", "e1", "grunt")
	e.Emit("onStart")

	if got := lb.L.GetGlobal("got").String(); got != "e1/grunt" {
		t.Fatalf("callback args = %q, want %q", got, "e1/grunt")
	}
	if calls := lb.L.GetGlobal("calls").String(); calls != "11" {
		t.Fatalf("calls = %s, want 11", calls)
	}
}

// killEnemy/setEnemyHP act on the shared actor registry: the actor really
// dies (nothing copies old HP back) and onEnemyDead gets id and type.
func TestLuaKillEnemyActsOnActorRegistry(t *testing.T) {
	e := NewEngine()
	lb := NewLuaBridge(e)
	defer lb.Close()
	routed := 0
	err := lb.DoString(`
		dead = ""
		registerEvent("onEnemyDead", function(id, typ) dead = dead .. id .. "/" .. typ .. ";" end)
		a = spawnEnemy("pajak", 2, 3, 0)
		b = spawnEnemy("straznik", 4, 3, 0)
		killEnemy(a)
		setEnemyHP(b, 40)
		hpB = getEnemyHP(b)
		setEnemyHP(b, 0)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got := lb.L.GetGlobal("dead").String(); got != "pajak_1/pajak;straznik_2/straznik;" {
		t.Fatalf("onEnemyDead calls %q", got)
	}
	if hp := lb.L.GetGlobal("hpB").String(); hp != "40" {
		t.Fatalf("hp after setEnemyHP = %s", hp)
	}
	for _, a := range e.Actors.All() {
		if a.Alive || a.HP != 0 {
			t.Fatalf("%s still alive (%v HP)", a.ID, a.HP)
		}
	}
	if !e.AllEnemiesDeadInZone("") {
		t.Fatal("enemies alive")
	}
	// With a host, script damage goes through its damage path.
	e.OnDamage = func(id string, amount float64) { routed++ }
	id := e.SpawnEnemy("grunt", 1, 1, 0)
	e.KillEnemy(id)
	if routed != 1 {
		t.Fatal("killEnemy did not use the host damage path")
	}
}
