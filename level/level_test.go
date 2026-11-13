package level

import (
	"math"
	"strings"
	"testing"

	"aiengine/parser"
	"aiengine/renderer"
	"aiengine/scripting"
)

const testMap = `MAP test
PLAYER_ANGLE 90
FLOOR_COLOR #102030

GRID
W W W W W W
W P . * . W
W . E . D W
W W W W W W

PLACE item gem g1 2.5 0.0 1.5 sound=ding
PLACE item gem g2 3.5 0 1.5 radius=0.3
PLACE zone exit goal 4.5 0 1.5 radius=0.4
PLACE door exit_door 4 0 2
PLACE light 3.0 1.0 2.0 color=#FF9932

SOUND ding
  FREQUENCY 900

TRIGGER got_all
  CONDITION item_picked g2
  ACTION unlock_door exit_door
  ACTION show_message "Door open"
`

func loadTest(t *testing.T) *Level {
	t.Helper()
	doc, err := parser.ParseAIE(strings.NewReader(testMap))
	if err != nil {
		t.Fatal(err)
	}
	lv, err := FromDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	return lv
}

func TestLevelFromGrid(t *testing.T) {
	lv := loadTest(t)
	if lv.Map.Width != 6 || lv.Map.Height != 4 {
		t.Fatalf("size %dx%d", lv.Map.Width, lv.Map.Height)
	}
	if lv.SpawnX != 1.5 || lv.SpawnY != 1.5 {
		t.Fatalf("spawn %v,%v", lv.SpawnX, lv.SpawnY)
	}
	if math.Abs(lv.SpawnAngle-math.Pi/2) > 1e-9 {
		t.Fatalf("angle %v", lv.SpawnAngle)
	}
	if lv.Props["FLOOR_COLOR"] != "#102030" {
		t.Fatalf("props %v", lv.Props)
	}
	if got := lv.Map.GetCell(4, 2); got != renderer.CellDoor {
		t.Fatalf("door cell = %v", got)
	}
	if len(lv.Markers["*"]) != 1 {
		t.Fatalf("markers %v", lv.Markers)
	}
	enemies := lv.ByKind("enemy")
	if len(enemies) != 1 || enemies[0].Type != "grunt" || enemies[0].X != 2.5 || enemies[0].Y != 2.5 {
		t.Fatalf("enemies %+v", enemies)
	}
	items := lv.ByKind("item")
	if len(items) != 2 || items[0].ID != "g1" || items[0].Type != "gem" || items[0].Params["sound"] != "ding" {
		t.Fatalf("items %+v", items)
	}
	lights := lv.ByKind("light")
	if len(lights) != 1 || lights[0].Height != 1.0 || lights[0].ID == "" {
		t.Fatalf("lights %+v", lights)
	}
	if len(lv.Triggers) != 1 || len(lv.Sounds) != 1 {
		t.Fatalf("triggers %d sounds %d", len(lv.Triggers), len(lv.Sounds))
	}
}

func TestLevelErrors(t *testing.T) {
	for name, src := range map[string]string{
		"no map":    "PROP barrel\n",
		"no grid":   "MAP x\nLIGHT_AMBIENT 0.1\n",
		"no spawn":  "MAP x\nGRID\nW W W\nW . W\nW W W\n",
		"bad place": "MAP x\nGRID\nW W W\nW P W\nW W W\nPLACE item gem 1.5 0\n",
	} {
		doc, err := parser.ParseAIE(strings.NewReader(src))
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		if _, err := FromDocument(doc); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestSessionPickupsZonesTriggersAndWin(t *testing.T) {
	lv := loadTest(t)
	eng := scripting.NewEngine()
	s := NewSession(lv, eng)
	lua := scripting.NewLuaBridge(eng)
	defer lua.Close()
	s.CallLua = func(name string) { _ = lua.Call(name) }

	if err := lua.DoString(`
		picked = 0
		registerEvent("onItemPickup", function(id, typ) picked = picked + 1 end)
		registerEvent("onZoneEnter", function(id, typ)
			if typ == "exit" then winGame("done") end
		end)
	`); err != nil {
		t.Fatal(err)
	}

	// Door closed: the exit cell beyond is unreachable.
	if !lv.IsSolid(4.5, 2.5) {
		t.Fatal("door should start closed")
	}
	ev := s.Update(0.1, 2.5, 1.5) // on g1
	if len(ev) != 1 || ev[0].Kind != EventPickup || ev[0].ID != "g1" {
		t.Fatalf("events %+v", ev)
	}
	if s.Update(0.1, 2.5, 1.5) != nil {
		t.Fatal("item picked twice")
	}
	s.Update(0.1, 3.5, 1.5) // g2 → trigger opens door
	if lv.IsSolid(4.5, 2.5) {
		t.Fatal("trigger should have opened the door")
	}
	if eng.CurrentMessage() != "Door open" {
		t.Fatalf("message = %q", eng.CurrentMessage())
	}
	if s.ItemsLeft() != 0 || lua.L.GetGlobal("picked").String() != "2" {
		t.Fatalf("items left %d, lua picked %s", s.ItemsLeft(), lua.L.GetGlobal("picked"))
	}
	if s.State != Playing {
		t.Fatal("won too early")
	}
	s.Update(0.1, 4.5, 1.5) // exit zone
	if s.State != Won || eng.EndText != "done" {
		t.Fatalf("state %v end %q", s.State, eng.EndText)
	}
	elapsed := s.Elapsed
	s.Update(5, 1.5, 1.5)
	if s.Elapsed != elapsed {
		t.Fatal("clock must stop after the game ends")
	}
}

func TestSessionLoseOnDeath(t *testing.T) {
	lv := loadTest(t)
	eng := scripting.NewEngine()
	s := NewSession(lv, eng)
	eng.SetPlayerHP(0)
	if s.State != Lost {
		t.Fatalf("state %v", s.State)
	}
}
