package level

import (
	"math"
	"strings"
	"testing"

	"aiengine/parser"
	"aiengine/scripting"
)

const doorsMap = `MAP drzwi
GRID
W W W W W W W W W
W P . . D . . . W
W . W W W W W . W
W . . . . . . . W
W W W W W W W W W

PLACE item key klucz 1.5 0 3.5 give=klucz_czerwony
PLACE door czerwone 4 0 1 key=klucz_czerwony
PLACE door ukryte 3 0 2 secret=yes
PLACE door brama 7 0 2
PLACE zone exit wyjscie 7.5 0 1.5 to=next
PLACE enemy_type pajak 0 0 0 hp=20 speed=2.5 behavior=wander
PLACE enemy pajak p1 5.5 0 3.5
PLACE enemy pajak p2 6.5 0 3.5 hp=40

TRIGGER boss
  CONDITION enemy_dead p1
  ACTION unlock_door brama

TRIGGER wyjdz
  CONDITION item_picked never
  ACTION exit_to poziom2
`

func doorsSession(t *testing.T) (*Level, *scripting.Engine, *Session) {
	t.Helper()
	doc, err := parser.ParseAIE(strings.NewReader(doorsMap))
	if err != nil {
		t.Fatal(err)
	}
	lv, err := FromDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	eng := scripting.NewEngine()
	return lv, eng, NewSession(lv, eng)
}

func TestEnemyTemplates(t *testing.T) {
	lv, _, _ := doorsSession(t)
	en := lv.ByKind("enemy")
	if len(en) != 2 || len(lv.ByKind("enemy_type")) != 0 {
		t.Fatalf("enemies %+v", en)
	}
	if en[0].Params["hp"] != "20" || en[0].Params["behavior"] != "wander" || en[1].Params["hp"] != "40" || en[1].Params["speed"] != "2.5" {
		t.Fatalf("template merge: %+v / %+v", en[0].Params, en[1].Params)
	}
}

func TestKeyDoorSecretAndInteract(t *testing.T) {
	lv, eng, s := doorsSession(t)
	var locked, secrets []string
	eng.On("onDoorLocked", func(a ...interface{}) { locked = append(locked, a[0].(string)+":"+a[1].(string)) })
	eng.On("onSecretFound", func(a ...interface{}) { secrets = append(secrets, a[0].(string)) })
	if eng.GetVar("secrets_total") != 1.0 {
		t.Fatalf("secrets_total = %v", eng.GetVar("secrets_total"))
	}

	// Facing the red door (east of the player) without the key.
	ev := s.Interact(3.5, 1.5, 0)
	if len(ev) != 1 || ev[0].Kind != EventDoorLocked || !lv.Solid(4, 1) {
		t.Fatalf("locked door: %+v", ev)
	}
	// Facing a plain wall: nothing.
	if ev := s.Interact(1.5, 1.5, math.Pi/2*0+math.Pi); ev != nil {
		t.Fatalf("wall interact: %+v", ev)
	}
	// Pick up the key (give=), then the door opens and the key is used up.
	s.Update(0.1, 1.5, 3.5)
	if eng.ItemCount("klucz_czerwony") != 1 {
		t.Fatal("key not added to inventory")
	}
	ev = s.Interact(3.5, 1.5, 0)
	if len(ev) != 1 || ev[0].Kind != EventDoorOpened || lv.Solid(4, 1) || eng.ItemCount("klucz_czerwony") != 0 {
		t.Fatalf("key door: %+v solid=%v keys=%d", ev, lv.Solid(4, 1), eng.ItemCount("klucz_czerwony"))
	}
	// Secret wall south of (3.5,1.5) looks like a wall but opens.
	if lv.Map.GetCell(3, 2) == 0 {
		t.Fatal("secret door must keep its wall look")
	}
	ev = s.Interact(3.5, 1.5, math.Pi/2)
	if len(ev) != 1 || ev[0].Kind != EventDoorOpened || len(secrets) != 1 {
		t.Fatalf("secret: %+v %v", ev, secrets)
	}
	// Script-only door (brama) refuses interact.
	ev = s.Interact(7.5, 3.5, -math.Pi/2)
	if len(ev) != 1 || ev[0].Kind != EventDoorLocked || locked[len(locked)-1] != "brama:" {
		t.Fatalf("script door: %+v %v", ev, locked)
	}
}

func TestEnemyDeathTriggerAndExitZone(t *testing.T) {
	lv, eng, s := doorsSession(t)
	eng.Emit("onEnemyDead", "p1")
	s.Update(0.1, 1.5, 1.5)
	if lv.Solid(7, 2) {
		t.Fatal("enemy_dead trigger did not open the gate")
	}
	s.Update(0.1, 7.5, 1.5) // exit zone with to=next
	if s.State != Won || eng.ExitTarget != "next" {
		t.Fatalf("state %v exit %q", s.State, eng.ExitTarget)
	}
}

func TestExitLevelFromLua(t *testing.T) {
	_, eng, s := doorsSession(t)
	lb := scripting.NewLuaBridge(eng)
	defer lb.Close()
	if err := lb.DoString(`giveItem("relikwia", 2) takeItem("relikwia") exitLevel("poziom3")`); err != nil {
		t.Fatal(err)
	}
	if s.State != Won || eng.ExitTarget != "poziom3" || eng.ItemCount("relikwia") != 1 {
		t.Fatalf("state %v target %q items %v", s.State, eng.ExitTarget, eng.Inventory)
	}
}
