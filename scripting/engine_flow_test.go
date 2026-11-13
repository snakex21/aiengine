package scripting

import (
	"reflect"
	"testing"
)

func TestTimedMessageAutoHides(t *testing.T) {
	e := NewEngine()
	e.ShowMessageFor("hello", 1.0)
	e.TickMessage(0.6)
	if got := e.CurrentMessage(); got != "hello" {
		t.Fatalf("message after 0.6s = %q, want hello", got)
	}
	e.TickMessage(0.6)
	if got := e.CurrentMessage(); got != "" {
		t.Fatalf("message after 1.2s = %q, want hidden", got)
	}

	e.ShowMessage("sticky")
	e.TickMessage(100)
	if got := e.CurrentMessage(); got != "sticky" {
		t.Fatalf("sticky message = %q", got)
	}
}

func TestHUDTextOrderedByKey(t *testing.T) {
	e := NewEngine()
	lb := NewLuaBridge(e)
	defer lb.Close()
	if err := lb.DoString(`
		setHUDText("2_time", "TIME 10")
		setHUDText("1_items", "ITEMS 0/6")
		setHUDText("3_tmp", "x")
		setHUDText("3_tmp")
	`); err != nil {
		t.Fatal(err)
	}
	want := []string{"ITEMS 0/6", "TIME 10"}
	if got := e.HUDLines(); !reflect.DeepEqual(got, want) {
		t.Fatalf("HUDLines = %v, want %v", got, want)
	}
}

func TestGameFlowCallbacksFromLuaAndJS(t *testing.T) {
	e := NewEngine()
	var wins, loses, restarts int
	var doors []string
	e.OnWin = func() { wins++ }
	e.OnLose = func() { loses++ }
	e.OnRestart = func() { restarts++ }
	e.OnDoor = func(id string, open bool) {
		if open {
			doors = append(doors, "+"+id)
		} else {
			doors = append(doors, "-"+id)
		}
	}

	lb := NewLuaBridge(e)
	defer lb.Close()
	if err := lb.DoString(`unlockDoor("exit") lockDoor("exit") winGame("YOU WIN") restartMap()`); err != nil {
		t.Fatal(err)
	}
	if e.EndText != "YOU WIN" {
		t.Fatalf("EndText = %q", e.EndText)
	}

	jb := NewJSBridge(e)
	if _, err := jb.RunString(`Engine.loseGame(); Engine.restartMap();`); err != nil {
		t.Fatal(err)
	}
	if e.EndText != "" {
		t.Fatalf("EndText after JS loseGame() = %q, want empty", e.EndText)
	}
	if wins != 1 || loses != 1 || restarts != 2 {
		t.Fatalf("wins=%d loses=%d restarts=%d", wins, loses, restarts)
	}
	if !reflect.DeepEqual(doors, []string{"+exit", "-exit"}) {
		t.Fatalf("doors = %v", doors)
	}
}
