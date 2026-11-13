package krypta_test

import (
	"math"
	"os"
	"testing"

	"aiengine/level"
	"aiengine/nav"
)

func TestPodiumCrystalNeedsTheLift(t *testing.T) {
	lv, err := level.LoadFile("maps/przedsionek.aie")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range lv.Doors() {
		if !d.Anim {
			t.Errorf("door %s must slide (DOOR_STYLE slide)", d.ID)
		}
		lv.SetDoor(d.ID, true)
		settle(lv, d.ID, nil)
	}
	k1 := nav.Cell{X: 10, Y: 1}
	if flood(lv, map[nav.Cell]bool{nav.CellAt(lv.SpawnX, lv.SpawnY): true})[k1] {
		t.Fatal("the podium must be out of reach without the lift")
	}
	lifts := lv.MoversByTag("w")
	if len(lifts) != 1 || lifts[0].Kind != "lift" || !lifts[0].Use {
		t.Fatalf("lift w: %+v", lifts)
	}
	reach, _ := solve(lv, "")
	if !reach[k1] {
		t.Fatal("the lift must take the player up to the podium")
	}
}

func TestCrusherTrapHurtsAndLeverStopsIt(t *testing.T) {
	g := start(t)
	hp := g.eng.GetPlayerHP()
	for i := 0; i < 20; i++ { // 2 s under the crusher
		g.s.Update(0.1, 11.5, 12.5)
	}
	if g.eng.GetPlayerHP() >= hp {
		t.Fatal("the crusher did not hurt the player")
	}
	if g.lv.CeilAt(11, 12) < 0.6-0.25-1e-9 {
		t.Fatalf("crusher went through the player (ceil %.2f over floor %.2f)", g.lv.CeilAt(11, 12), g.lv.FloorAt(11, 12))
	}
	ev := g.s.Interact(8.6, 12.5, 0)
	if len(ev) != 1 || ev[0].Kind != level.EventSwitch || ev[0].ID != "d1" {
		t.Fatalf("lever: %+v", ev)
	}
	for i := 0; i < 40; i++ {
		g.s.Update(0.1, 8.6, 12.5)
	}
	if g.lv.CeilAt(11, 12) != 0.75 || g.lv.MoversByTag("c")[0].Busy() {
		t.Fatalf("stopped crusher must rest up (ceil %.2f)", g.lv.CeilAt(11, 12))
	}
}

func TestNicheGateTellsWhyItIsLocked(t *testing.T) {
	g := startMap(t, "maps/sanktuarium.aie")
	ev := g.s.Interact(12, 7.6, -math.Pi/2)
	if len(ev) != 1 || ev[0].Kind != level.EventDoorLocked || ev[0].ID != "wrota" {
		t.Fatalf("locked gate: %+v", ev)
	}
	if g.eng.CurrentMessage() == "" {
		t.Fatal("the script must explain the locked gate")
	}
}

func TestAnimatedWaterAndLava(t *testing.T) {
	anims, err := level.LoadTexAnimFile("assets/textures/textures.aie")
	if err != nil || len(anims) != 2 {
		t.Fatalf("descriptor: %v %v", anims, err)
	}
	for _, a := range anims {
		if len(a.Frames) != 4 || a.FPS <= 0 {
			t.Errorf("%s: %+v", a.Name, a)
		}
		for _, f := range a.Frames {
			if _, err := os.Stat("assets/textures/" + f + ".png"); err != nil {
				t.Errorf("frame %s: %v", f, err)
			}
		}
	}
	uses := map[string]string{"maps/zalana_kopalnia.aie": "woda", "maps/serce_krypty.aie": "lawa", "maps/sanktuarium.aie": "woda"}
	for path, tex := range uses {
		lv, err := level.LoadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, n := range lv.TextureNames() {
			found = found || n == tex
		}
		if !found {
			t.Errorf("%s does not use %s (%v)", path, tex, lv.TextureNames())
		}
	}
}
