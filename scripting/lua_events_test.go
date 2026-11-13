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
