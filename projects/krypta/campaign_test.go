package krypta_test

import (
	"path/filepath"
	"testing"

	"aiengine/level"
	"aiengine/nav"
	"aiengine/parser"
	"aiengine/progress"
	"aiengine/renderer"
	"aiengine/scripting"
)

func campaignLevels(t *testing.T) (*parser.Manifest, *progress.Campaign) {
	t.Helper()
	m, err := parser.ParseManifestFile("manifest.aie")
	if err != nil {
		t.Fatal(err)
	}
	c := progress.FromManifest(m)
	if c == nil || len(c.Levels) != 5 {
		t.Fatalf("campaign %+v", c)
	}
	return m, c
}

// solve explores a level like a player would: walk anywhere reachable, pick
// up every item there, open doors whose key was found, open secret walls and
// plain doors, and open the script gate once its condition is met (all
// crystals reachable, or the boss reachable). It returns the reachable cells
// at the end and the order in which items were collected.
func solve(lv *level.Level, boss string) (map[nav.Cell]bool, []level.Placement) {
	keys := map[string]bool{}
	opened := map[string]bool{}
	taken := map[string]bool{}
	var order []level.Placement
	crystalsTotal := len(byType(lv, "crystal"))
	for {
		reach := flood(lv, opened)
		progressMade := false
		for _, p := range lv.Placements {
			if p.Kind != "item" || taken[p.ID] || !reach[nav.CellAt(p.X, p.Y)] {
				continue
			}
			taken[p.ID] = true
			order = append(order, p)
			if give := p.Params["give"]; give != "" {
				keys[give] = true
			}
			progressMade = true
		}
		crystals := 0
		for _, p := range order {
			if p.Type == "crystal" {
				crystals++
			}
		}
		bossReached := false
		for _, p := range lv.ByKind("enemy") {
			if p.ID == boss && reach[nav.CellAt(p.X, p.Y)] {
				bossReached = true
			}
		}
		for _, d := range lv.Doors() {
			if opened[d.ID] || !nextToReach(reach, d) {
				continue
			}
			ok := d.Use || d.Secret || (d.Key != "" && keys[d.Key])
			if d.Key == "" && !d.Use && !d.Secret { // script gate
				ok = (boss == "" && crystals == crystalsTotal) || (boss != "" && bossReached)
			}
			if ok {
				opened[d.ID] = true
				progressMade = true
			}
		}
		if !progressMade {
			return reach, order
		}
	}
}

func byType(lv *level.Level, typ string) []level.Placement {
	var out []level.Placement
	for _, p := range lv.ByKind("item") {
		if p.Type == typ {
			out = append(out, p)
		}
	}
	return out
}

func nextToReach(reach map[nav.Cell]bool, d level.DoorInfo) bool {
	for _, o := range []nav.Cell{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		if reach[nav.Cell{X: d.X + o.X, Y: d.Y + o.Y}] {
			return true
		}
	}
	return false
}

func flood(lv *level.Level, opened map[string]bool) map[nav.Cell]bool {
	passable := func(c nav.Cell) bool {
		if d, ok := lv.DoorAt(c.X, c.Y); ok {
			return opened[d.ID]
		}
		return !lv.Solid(c.X, c.Y)
	}
	start := nav.CellAt(lv.SpawnX, lv.SpawnY)
	seen := map[nav.Cell]bool{start: true}
	queue := []nav.Cell{start}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		for _, o := range []nav.Cell{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
			n := nav.Cell{X: c.X + o.X, Y: c.Y + o.Y}
			if !seen[n] && passable(n) && stepOK(lv, c, n) {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	return seen
}

func TestCampaignLevelsAreSolvable(t *testing.T) {
	_, c := campaignLevels(t)
	for _, l := range c.Levels {
		t.Run(l.ID, func(t *testing.T) {
			lv, err := level.LoadFile(l.Path)
			if err != nil {
				t.Fatal(err)
			}
			boss := lv.Props["VAR_BOSS"]
			reach, order := solve(lv, boss)
			for _, p := range lv.ByKind("item") {
				if !reach[nav.CellAt(p.X, p.Y)] {
					t.Errorf("item %s (%s) unreachable", p.ID, p.Type)
				}
			}
			portal := nav.CellAt(lv.SpawnX, lv.SpawnY)
			for _, z := range lv.ByKind("zone") {
				if z.Type == "exit" {
					portal = nav.CellAt(z.X, z.Y)
				}
			}
			if !reach[portal] {
				t.Errorf("portal unreachable at the end")
			}
			// The portal must not be reachable before the gate opens.
			if flood(lv, map[string]bool{})[portal] {
				t.Errorf("portal reachable without opening any door")
			}
			// Every key must be found before its door (not behind it).
			for _, d := range lv.Doors() {
				if d.Key == "" {
					continue
				}
				found := false
				for _, p := range order {
					if p.Params["give"] == d.Key {
						found = true
					}
				}
				if !found {
					t.Errorf("key %s for door %s never found", d.Key, d.ID)
				}
			}
			for _, e := range lv.ByKind("enemy") {
				if lv.Solid(int(e.X), int(e.Y)) {
					t.Errorf("enemy %s placed inside a wall", e.ID)
				}
			}
			t.Logf("%s: %dx%d, %d items, %d enemies, %d doors", l.ID, lv.Map.Width, lv.Map.Height,
				len(lv.ByKind("item")), len(lv.ByKind("enemy")), len(lv.Doors()))
		})
	}
}

// TestCampaignPlaythrough plays every level headlessly with the real Lua
// script, in order, carrying HP/score/items through progress like the host.
func TestCampaignPlaythrough(t *testing.T) {
	_, c := campaignLevels(t)
	prog := progress.New(c)
	prog.NewGame(c)
	path := filepath.Join(t.TempDir(), progress.FileName)
	for i, l := range c.Levels {
		lv, err := level.LoadFile(l.Path)
		if err != nil {
			t.Fatal(err)
		}
		eng := scripting.NewEngine()
		s := level.NewSession(lv, eng)
		prog.Carry.Apply(eng)
		lua := scripting.NewLuaBridge(eng)
		if err := lua.DoFile("scripts/krypta.lua"); err != nil {
			t.Fatal(err)
		}
		eng.Emit("onStart")
		if i > 0 && eng.GetVar("score") == 0.0 {
			t.Fatalf("%s: score did not carry over", l.ID)
		}

		boss := lv.Props["VAR_BOSS"]
		_, order := solve(lv, boss)
		for _, p := range order {
			s.Update(0.05, p.X, p.Y)
			s.Update(0.05, lv.SpawnX, lv.SpawnY) // step away (potions etc.)
		}
		if boss != "" {
			eng.Emit("onEnemyDead", boss)
			s.Update(0.05, lv.SpawnX, lv.SpawnY)
		}
		for _, z := range lv.ByKind("zone") {
			if z.Type == "exit" {
				s.Update(0.05, z.X, z.Y)
			}
		}
		if s.State != level.Won || eng.ExitTarget != "next" {
			t.Fatalf("%s: state %v exit %q HUD %v", l.ID, s.State, eng.ExitTarget, eng.HUDLines())
		}
		carry := c.Capture(eng)
		next, finished, _ := prog.Complete(c, l.ID, s.Elapsed, carry)
		if err := prog.Save(path); err != nil {
			t.Fatal(err)
		}
		loaded, err := progress.Load(path, c)
		if err != nil {
			t.Fatal(err)
		}
		if i < len(c.Levels)-1 {
			if next != c.Levels[i+1].ID || finished || loaded.Current != next || !loaded.Unlocked[next] {
				t.Fatalf("%s: next %q finished %v loaded %+v", l.ID, next, finished, loaded)
			}
		} else if !finished || !loaded.Finished {
			t.Fatal("campaign not finished after the last level")
		}
		prog = loaded
		lua.Close()
	}
	if prog.Carry.Items["relikwia"] != 0 {
		t.Fatal("carry must be cleared after the campaign ends")
	}
}

func TestBossSummonsSpiders(t *testing.T) {
	lv, err := level.LoadFile("maps/serce_krypty.aie")
	if err != nil {
		t.Fatal(err)
	}
	eng := scripting.NewEngine()
	s := level.NewSession(lv, eng)
	var spawned []string
	eng.OnSpawn = func(id, typ string, x, y float64) { spawned = append(spawned, typ) }
	lua := scripting.NewLuaBridge(eng)
	defer lua.Close()
	if err := lua.DoFile("scripts/krypta.lua"); err != nil {
		t.Fatal(err)
	}
	eng.Emit("onStart")
	for i := 0; i < 200; i++ { // 20 s
		eng.Advance(0.1)
		s.Update(0.1, lv.SpawnX, lv.SpawnY)
	}
	if len(spawned) != 2 || spawned[0] != "pajak" {
		t.Fatalf("spawned %v, want 2 spiders in 20 s", spawned)
	}
	if !lv.Solid(8, 11) {
		t.Fatal("gate must stay closed while the boss lives")
	}
	eng.Emit("onEnemyDead", "boss")
	s.Update(0.1, lv.SpawnX, lv.SpawnY)
	if lv.Solid(8, 11) {
		t.Fatal("gate did not open after the boss died")
	}
	if len(lv.EnemyTypes["pajak"]) == 0 || lv.EnemyTypes["wladca"]["hp"] != "300" {
		t.Fatalf("enemy types %v", lv.EnemyTypes)
	}
}

// stepOK applies the height rules to a move between neighbouring cells
// (ignoring whether the target is a closed door): rises up to a step,
// any drop, and room to stand.
func stepOK(lv *level.Level, from, to nav.Cell) bool {
	ft := lv.FloorAt(to.X, to.Y)
	return ft-lv.FloorAt(from.X, from.Y) <= renderer.MaxStepUp+1e-9 &&
		lv.CeilAt(to.X, to.Y)-ft >= renderer.HeadroomStand-1e-9
}
