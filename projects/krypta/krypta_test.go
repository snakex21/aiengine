// Headless play-through tests for "Krysztalowa Krypta". They load the real
// project files (map + Lua) through the engine's level runtime, without a
// window, and drive the player by position.
package krypta_test

import (
	"math"
	"strings"
	"testing"

	"aiengine/level"
	"aiengine/nav"
	"aiengine/scripting"
)

type game struct {
	lv  *level.Level
	eng *scripting.Engine
	s   *level.Session
	lua *scripting.LuaBridge
}

func start(t *testing.T) *game {
	t.Helper()
	return startMap(t, "maps/krypta.aie")
}

func startMap(t *testing.T, path string) *game {
	t.Helper()
	lv, err := level.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	eng := scripting.NewEngine()
	s := level.NewSession(lv, eng)
	lua := scripting.NewLuaBridge(eng)
	t.Cleanup(lua.Close)
	s.CallLua = func(name string) {
		if err := lua.Call(name); err != nil {
			t.Errorf("lua %s: %v", name, err)
		}
	}
	if err := lua.DoFile("scripts/krypta.lua"); err != nil {
		t.Fatal(err)
	}
	eng.Emit("onStart")
	return &game{lv: lv, eng: eng, s: s, lua: lua}
}

// at returns the placement with the given id.
func (g *game) at(t *testing.T, id string) level.Placement {
	t.Helper()
	for _, p := range g.lv.Placements {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("no placement %q", id)
	return level.Placement{}
}

func (g *game) visit(t *testing.T, id string) {
	p := g.at(t, id)
	g.s.Update(0.1, p.X, p.Y)
}

func hudHas(eng *scripting.Engine, text string) bool {
	return strings.Contains(strings.Join(eng.HUDLines(), "|"), text)
}

// reachable runs a flood fill over walkable cells from the spawn.
func reachable(lv *level.Level) map[[2]int]bool {
	seen := map[[2]int]bool{}
	start := [2]int{int(lv.SpawnX), int(lv.SpawnY)}
	queue := [][2]int{start}
	seen[start] = true
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := [2]int{c[0] + d[0], c[1] + d[1]}
			if seen[n] || lv.IsSolid(float64(n[0])+0.5, float64(n[1])+0.5) ||
				!stepOK(lv, nav.Cell{X: c[0], Y: c[1]}, nav.Cell{X: n[0], Y: n[1]}) {
				continue
			}
			seen[n] = true
			queue = append(queue, n)
		}
	}
	return seen
}

func cell(p level.Placement) [2]int { return [2]int{int(p.X), int(p.Y)} }

func TestLevelLoadsWithExpectedContent(t *testing.T) {
	g := start(t)
	if n := len(g.lv.ByKind("item")); n != 8 {
		t.Fatalf("items = %d, want 8 (6 crystals + 2 potions)", n)
	}
	if n := len(g.lv.ByKind("enemy")); n != 3 {
		t.Fatalf("enemies = %d", n)
	}
	if n := len(g.lv.Sounds); n != 5 {
		t.Fatalf("sounds = %d", n)
	}
	if !hudHas(g.eng, "Kryształy 0/6") || !hudHas(g.eng, "Czas 3:00") {
		t.Fatalf("HUD = %v", g.eng.HUDLines())
	}
}

func TestEverythingReachableButPortalNeedsTheGate(t *testing.T) {
	g := start(t)
	open := reachable(g.lv)
	for _, p := range g.lv.Placements {
		if p.Kind == "door" || p.Kind == "light" || p.ID == "portal" {
			continue
		}
		if !open[cell(p)] {
			t.Errorf("%s at %v is unreachable from the spawn", p.ID, cell(p))
		}
	}
	if open[cell(g.at(t, "portal"))] {
		t.Fatal("portal reachable while the gate is closed")
	}
	g.lv.SetDoor("brama", true)
	settle(g.lv, "brama", nil)
	if !reachable(g.lv)[cell(g.at(t, "portal"))] {
		t.Fatal("portal unreachable with the gate open")
	}
}

func TestWinByCollectingAllCrystalsAndEnteringPortal(t *testing.T) {
	g := start(t)

	g.visit(t, "portal") // asleep: must not win, and the gate is closed anyway
	if g.s.State != level.Playing {
		t.Fatal("won without crystals")
	}

	for i, id := range []string{"k1", "k2", "k3", "k4", "k5"} {
		g.visit(t, id)
		if g.s.ItemsLeft() != 8-(i+1) {
			t.Fatalf("after %s items left = %d", id, g.s.ItemsLeft())
		}
	}
	if d, _ := g.lv.Door("brama"); d.Open {
		t.Fatal("gate opened before the 6th crystal")
	}
	g.visit(t, "k6")
	if d, _ := g.lv.Door("brama"); !d.Open {
		t.Fatal("gate did not open after 6 crystals")
	}
	if !hudHas(g.eng, "Kryształy 6/6") || !strings.Contains(g.eng.CurrentMessage(), "Brama otwarta") {
		t.Fatalf("HUD %v message %q", g.eng.HUDLines(), g.eng.CurrentMessage())
	}

	g.s.Update(0.1, 0, 0) // leave the portal zone first
	g.visit(t, "portal")
	if g.s.State != level.Won {
		t.Fatalf("state = %v, want won", g.s.State)
	}
	if !strings.HasPrefix(g.eng.EndText, "Ucieczka udana") {
		t.Fatalf("end text %q", g.eng.EndText)
	}
}

func TestAcidHurtsAndPotionHeals(t *testing.T) {
	g := start(t)
	acid := g.at(t, "kwas1")
	for i := 0; i < 10; i++ { // 1 s in acid
		g.s.Update(0.1, acid.X, acid.Y)
	}
	hp := g.eng.GetPlayerHP()
	if math.Abs(hp-80) > 5.01 {
		t.Fatalf("HP after 1 s in acid = %v, want about 80", hp)
	}
	g.s.Update(0.1, acid.X+2, acid.Y) // step out
	before := g.eng.GetPlayerHP()
	g.s.Update(1, acid.X+2, acid.Y)
	if g.eng.GetPlayerHP() != before {
		t.Fatal("acid kept hurting after leaving it")
	}
	g.visit(t, "m2")
	if g.eng.GetPlayerHP() <= before {
		t.Fatalf("potion did not heal: %v -> %v", before, g.eng.GetPlayerHP())
	}
}

func TestLoseByDyingInAcid(t *testing.T) {
	g := start(t)
	acid := g.at(t, "kwas2")
	for i := 0; i < 100 && g.s.State == level.Playing; i++ {
		g.s.Update(0.1, acid.X, acid.Y)
	}
	if g.s.State != level.Lost || !strings.HasPrefix(g.eng.EndText, "Poległeś") {
		t.Fatalf("state %v end %q", g.s.State, g.eng.EndText)
	}
}

func TestLoseWhenTimeRunsOut(t *testing.T) {
	g := start(t)
	for i := 0; i < 200 && g.s.State == level.Playing; i++ {
		g.s.Update(1, g.lv.SpawnX, g.lv.SpawnY)
	}
	if g.s.State != level.Lost || !strings.HasPrefix(g.eng.EndText, "Koniec czasu") {
		t.Fatalf("state %v end %q", g.s.State, g.eng.EndText)
	}
	if g.s.Elapsed < 179 || g.s.Elapsed > 181 {
		t.Fatalf("lost after %.0f s, want 180", g.s.Elapsed)
	}
}
