package scripting

import (
	"aiengine/renderer"
	"reflect"
	"testing"
	"time"
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

func TestJSSetTimeoutRunsOnTick(t *testing.T) {
	e := NewEngine()
	jb := NewJSBridge(e)
	if _, err := jb.RunString(`
		var log = [];
		setTimeout(function() { log.push("a"); }, 100);
		var cancelled = setTimeout(function() { log.push("x"); }, 50);
		clearTimeout(cancelled);
		var n = 0;
		var iv = setInterval(function() { n++; if (n == 3) clearInterval(iv); }, 20);
	`); err != nil {
		t.Fatal(err)
	}
	jb.Tick(0.05)
	if got := jb.VM.Get("log").String(); got != "" {
		t.Fatalf("log after 50ms = %q", got)
	}
	for i := 0; i < 10; i++ {
		jb.Tick(0.02)
	}
	if got := jb.VM.Get("log").String(); got != "a" {
		t.Fatalf("log = %q, want a", got)
	}
	if n := jb.VM.Get("n").ToInteger(); n != 3 {
		t.Fatalf("interval ran %d times, want 3", n)
	}
}

func TestSetFilterFromLuaAndDefaultMessageDuration(t *testing.T) {
	e := NewEngine()
	e.Effects = renderer.DefaultEffects()
	lb := NewLuaBridge(e)
	defer lb.Close()
	if err := lb.DoString(`setFilter("red_flash", 0.5) setFilter("desaturate", 1) showMessage("hi")`); err != nil {
		t.Fatal(err)
	}
	if e.Effects.RedTint != 0.5 || e.Effects.Desaturate != 1 {
		t.Fatalf("effects = %+v", e.Effects)
	}
	if e.MessageTTL != DefaultMessageSeconds {
		t.Fatalf("MessageTTL = %v", e.MessageTTL)
	}
}

func TestTimersRunOnGameTimeOnly(t *testing.T) {
	e := NewEngine()
	lb := NewLuaBridge(e)
	defer lb.Close()
	if err := lb.DoString(`
		fired = 0
		registerEvent("onTimer", function(name) if name == "boom" then fired = fired + 1 end end)
		startTimer("boom", 1.0)
	`); err != nil {
		t.Fatal(err)
	}
	e.Advance(0.6)
	time.Sleep(30 * time.Millisecond) // "paused": wall time passes, game time does not
	e.TickTimers()
	if lb.L.GetGlobal("fired").String() != "0" || e.IsTimerElapsed("boom") {
		t.Fatal("timer fired before 1 s of game time")
	}
	if got := e.GetTime(); got != 0.6 {
		t.Fatalf("GetTime = %v, want 0.6", got)
	}
	e.Advance(0.5)
	if lb.L.GetGlobal("fired").String() != "1" {
		t.Fatal("timer did not fire after 1.1 s of game time")
	}
}

func TestSpawnEnemyIDsAreUnique(t *testing.T) {
	e := NewEngine()
	a := e.SpawnEnemy("grunt", 1, 1, 0)
	b := e.SpawnEnemy("grunt", 2, 2, 0)
	if a == b || len(e.Entities) != 2 {
		t.Fatalf("ids %q %q, entities %d", a, b, len(e.Entities))
	}
}

type fakeMusic struct{ track string }

func (f *fakeMusic) PlaySound(string)        {}
func (f *fakeMusic) StopAll()                {}
func (f *fakeMusic) PlayMusicTrack(n string) { f.track = n }
func (f *fakeMusic) StopMusicTrack()         { f.track = "" }

func TestPlayMusicReachesMusicPlayer(t *testing.T) {
	e := NewEngine()
	fm := &fakeMusic{}
	e.AudioPlayer = fm
	lb := NewLuaBridge(e)
	defer lb.Close()
	lb.DoString(`playMusic("drone")`)
	if fm.track != "drone" {
		t.Fatalf("track = %q", fm.track)
	}
	lb.DoString(`stopMusic()`)
	if fm.track != "" {
		t.Fatal("stopMusic did not stop")
	}
}
