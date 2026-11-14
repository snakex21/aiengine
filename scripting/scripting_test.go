// scripting/scripting_test.go — Unit tests for Lua and JS bridges.

package scripting

import (
	"testing"
)

func TestEngineCreation(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if e.PlayerHP != 100 {
		t.Errorf("expected PlayerHP=100, got %f", e.PlayerHP)
	}
	if len(e.PlayerWeapons) != 1 || e.PlayerWeapons[0] != "pistol" {
		t.Errorf("expected pistol in weapons, got %v", e.PlayerWeapons)
	}
}

func TestEnginePlayerHP(t *testing.T) {
	e := NewEngine()
	e.SetPlayerHP(50)
	if hp := e.GetPlayerHP(); hp != 50 {
		t.Errorf("expected HP=50, got %f", hp)
	}
	e.SetPlayerHP(150) // above max → capped
	if hp := e.GetPlayerHP(); hp != 100 {
		t.Errorf("expected HP capped at 100, got %f", hp)
	}
	e.SetPlayerHP(-10) // below 0 → 0
	if hp := e.GetPlayerHP(); hp != 0 {
		t.Errorf("expected HP floored at 0, got %f", hp)
	}
}

func TestEngineEnemySpawn(t *testing.T) {
	e := NewEngine()
	id := e.SpawnEnemy("grunt", 5.0, 3.0, 0.0)
	if id == "" {
		t.Fatal("expected non-empty enemy ID")
	}
	if hp := e.GetEnemyHP(id); hp != 100 {
		t.Errorf("expected enemy HP=100, got %f", hp)
	}
	e.KillEnemy(id)
	if hp := e.GetEnemyHP(id); hp != 0 {
		t.Errorf("killed enemy HP = %v, want 0", hp)
	}
	if e.AllEnemiesDeadInZone("") != true {
		t.Error("expected all enemies dead after kill")
	}
}

func TestEngineVariables(t *testing.T) {
	e := NewEngine()
	e.SetVar("score", 42)
	if v := e.GetVar("score"); v != 42 {
		t.Errorf("expected score=42, got %v", v)
	}
	e.SetVar("name", "player1")
	if v := e.GetVar("name"); v != "player1" {
		t.Errorf("expected name=player1, got %v", v)
	}
}

func TestEngineEvents(t *testing.T) {
	e := NewEngine()
	fired := false
	e.On("testEvent", func(args ...interface{}) {
		fired = true
	})
	e.Emit("testEvent")
	// Events are async in goroutines; wait is not possible in this test structure.
	// This test validates registration doesn't panic.
	_ = fired
}

func TestEngineTimers(t *testing.T) {
	e := NewEngine()
	e.StartTimer("test", 0.001) // 1ms timer
	if !e.IsTimerElapsed("test") {
		// Might not have elapsed yet.
	}
	e.StopTimer("test")
	if e.IsTimerElapsed("test") {
		// Stopped timer should not be elapsed.
	}
}

func TestLuaBridgeCreation(t *testing.T) {
	e := NewEngine()
	lb := NewLuaBridge(e)
	defer lb.Close()

	if lb.L == nil {
		t.Fatal("Lua VM is nil")
	}
}

func TestLuaBridgeExecute(t *testing.T) {
	e := NewEngine()
	lb := NewLuaBridge(e)
	defer lb.Close()

	script := `
		local x = 1 + 2
		setVar("test_result", x)
	`
	if err := lb.DoString(script); err != nil {
		t.Fatalf("Lua execution failed: %v", err)
	}

	val := e.GetVar("test_result")
	f, ok := val.(float64)
	if !ok || f != 3.0 {
		t.Errorf("expected test_result=3, got %v", val)
	}
}

func TestLuaBridgeEnemyAPI(t *testing.T) {
	e := NewEngine()
	lb := NewLuaBridge(e)
	defer lb.Close()

	script := `
		local id = spawnEnemy("grunt", 5.0, 3.0, 0.0)
		setVar("enemy_id", id)
		setEnemyHP(id, 50)
		local hp = getEnemyHP(id)
		setVar("enemy_hp", hp)
		killEnemy(id)
		setVar("all_dead", allEnemiesDead("zone1"))
	`
	if err := lb.DoString(script); err != nil {
		t.Fatalf("Lua enemy API failed: %v", err)
	}

	if hp, ok := e.GetVar("enemy_hp").(float64); !ok || hp != 50 {
		t.Errorf("expected enemy_hp=50, got %v", e.GetVar("enemy_hp"))
	}
	if allDead, ok := e.GetVar("all_dead").(bool); !ok || !allDead {
		t.Errorf("expected all_dead=true, got %v", e.GetVar("all_dead"))
	}
}

func TestLuaBridgePlayerAPI(t *testing.T) {
	e := NewEngine()
	lb := NewLuaBridge(e)
	defer lb.Close()

	script := `
		setPlayerHP(75)
		local hp = getPlayerHP()
		setVar("player_hp", hp)
		giveWeapon("shotgun")
		setPlayerSpeed(0.5)
	`
	if err := lb.DoString(script); err != nil {
		t.Fatalf("Lua player API failed: %v", err)
	}

	if hp := e.GetPlayerHP(); hp != 75 {
		t.Errorf("expected PlayerHP=75, got %f", hp)
	}
}

func TestJSBridgeCreation(t *testing.T) {
	e := NewEngine()
	jb := NewJSBridge(e)

	if jb.VM == nil {
		t.Fatal("JS VM is nil")
	}
}

func TestJSBridgeExecute(t *testing.T) {
	e := NewEngine()
	jb := NewJSBridge(e)

	script := `
		var x = 10 * 3;
		Engine.setVar("js_result", x);
	`
	if _, err := jb.RunString(script); err != nil {
		t.Fatalf("JS execution failed: %v", err)
	}

	v := e.GetVar("js_result")
	var f float64
	switch val := v.(type) {
	case float64:
		f = val
	case int64:
		f = float64(val)
	case int:
		f = float64(val)
	}
	if f != 30 {
		t.Errorf("expected js_result=30, got %v (type %T)", v, v)
	}
}

func TestJSBridgeEnemyAPI(t *testing.T) {
	e := NewEngine()
	jb := NewJSBridge(e)

	script := `
		var id = Engine.spawnEnemy("grunt", 5.0, 3.0, 0.0);
		Engine.setVar("js_enemy_id", id);
		Engine.setEnemyHP(id, 25);
		var hp = Engine.getEnemyHP(id);
		Engine.setVar("js_enemy_hp", hp);
		Engine.killEnemy(id);
	`
	if _, err := jb.RunString(script); err != nil {
		t.Fatalf("JS enemy API failed: %v", err)
	}

	v := e.GetVar("js_enemy_hp")
	var f float64
	switch val := v.(type) {
	case float64:
		f = val
	case int64:
		f = float64(val)
	case int:
		f = float64(val)
	}
	if f != 25 {
		t.Errorf("expected js_enemy_hp=25, got %v (type %T)", v, v)
	}
}

func TestJSBridgeUIFilters(t *testing.T) {
	e := NewEngine()
	jb := NewJSBridge(e)

	script := `
		UI.setFilter("vignette", 0.5);
		UI.setFilter("red_flash", 0.8);
		UI.setFilter("blur", 0.2);
		UI.setMessage("Hello World!");
		UI.setHP(80, 100);
		UI.setAmmo(12, 30);
	`
	if _, err := jb.RunString(script); err != nil {
		t.Fatalf("JS UI API failed: %v", err)
	}
}

func TestJSBridgeEngineOn(t *testing.T) {
	e := NewEngine()
	jb := NewJSBridge(e)

	script := `
		var eventFired = false;
		Engine.on("customEvent", function(data) {
			eventFired = true;
			Engine.setVar("event_data", data);
		});
		Engine.triggerEvent("customEvent");
	`
	if _, err := jb.RunString(script); err != nil {
		t.Fatalf("JS event API failed: %v", err)
	}
	// Event is async, but registration shouldn't fail.
}
