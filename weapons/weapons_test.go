package weapons

import (
	"strings"
	"testing"

	"aiengine/parser"
)

func parseWeapon(t *testing.T, src string) *Def {
	t.Helper()
	doc, err := parser.ParseAIE(strings.NewReader(src))
	if err != nil || len(doc.Nodes) == 0 {
		t.Fatalf("parse: %v", err)
	}
	return ParseDef(doc.Nodes[0])
}

func TestParseDefDocsKeys(t *testing.T) {
	d := parseWeapon(t, "WEAPON shotgun\nDAMAGE 15\nSPREAD 0.15\nAMMO_MAX 8\nBURST 8\nRANGE 10.0\nAUTO_FIRE yes\n")
	if d.Name != "shotgun" || d.Damage != 15 || d.Spread != 0.15 || d.Magazine != 8 || d.Pellets != 8 || d.Range != 10 || !d.Auto {
		t.Fatalf("def %+v", d)
	}
}

// Fire rate and reload only advance with Update (game time): a paused game
// that stops calling Update keeps the weapon exactly where it was.
func TestCooldownAndReloadRunOnGameTime(t *testing.T) {
	w := NewInstance(&Def{Name: "p", FireRate: 2, Magazine: 2, AmmoReserve: 4, ReloadTime: 1, Pellets: 1})
	if w.Fire() != 1 || w.Mag != 1 {
		t.Fatalf("first shot: mag %d", w.Mag)
	}
	if w.Fire() != 0 {
		t.Fatal("fired during the cooldown")
	}
	// Paused for "a long time": no Update calls, still cooling down.
	if w.Ready() {
		t.Fatal("ready without game time passing")
	}
	w.Update(0.5)
	if w.Fire() != 1 || w.Mag != 0 || !w.Reloading {
		t.Fatalf("second shot should empty the magazine and start a reload: %+v", w)
	}
	w.Update(0.6)
	if !w.Reloading || w.Mag != 0 {
		t.Fatal("reload finished too early")
	}
	w.Update(0.5)
	if w.Reloading || w.Mag != 2 || w.Reserve != 2 {
		t.Fatalf("after reload: %+v", w)
	}
}

func TestNoMagazineFiresFromReserve(t *testing.T) {
	w := NewInstance(&Def{Name: "r", FireRate: 10, AmmoReserve: 2, Pellets: 1})
	for i := 0; i < 2; i++ {
		if w.Fire() != 1 {
			t.Fatalf("shot %d failed", i)
		}
		w.Update(0.2)
	}
	if w.Fire() != 0 || w.Reload() {
		t.Fatal("fired or reloaded with an empty reserve")
	}
}
