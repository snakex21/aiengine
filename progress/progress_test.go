package progress

import (
	"path/filepath"
	"strings"
	"testing"

	"aiengine/parser"
	"aiengine/scripting"
)

func campaign(t *testing.T) *Campaign {
	t.Helper()
	m, err := parser.ParseManifest(strings.NewReader(`MANIFEST
  CAMPAIGN yes
  CARRY hp items score
  LEVEL a maps/a.aie Pierwszy
  LEVEL b maps/b.aie Drugi
  LEVEL c maps/c.aie Trzeci
`))
	if err != nil {
		t.Fatal(err)
	}
	c := FromManifest(m)
	if c == nil || len(c.Levels) != 3 {
		t.Fatalf("campaign %+v", c)
	}
	return c
}

func TestNotACampaign(t *testing.T) {
	m, _ := parser.ParseManifest(strings.NewReader("LEVEL a maps/a.aie\n"))
	if FromManifest(m) != nil {
		t.Fatal("projects without CAMPAIGN yes are not campaigns")
	}
}

func TestUnlockProgressionAndBestTimes(t *testing.T) {
	c := campaign(t)
	p := New(c)
	if !p.Unlocked["a"] || p.Unlocked["b"] || p.CanContinue() {
		t.Fatalf("fresh progress %+v", p)
	}
	p.NewGame(c)
	if p.Current != "a" || !p.CanContinue() {
		t.Fatal("new game must start at the first level")
	}
	next, done, rec := p.Complete(c, "a", 95, State{HP: 60})
	if next != "b" || done || !rec || !p.Unlocked["b"] || p.Unlocked["c"] || p.Current != "b" || p.Carry.HP != 60 {
		t.Fatalf("after a: next=%q done=%v %+v", next, done, p)
	}
	// Replaying a faster updates the record; slower does not.
	p.Start("a")
	if _, _, rec := p.Complete(c, "a", 80, State{}); !rec || p.Best["a"] != 80 {
		t.Fatal("faster run must become the record")
	}
	if _, _, rec := p.Complete(c, "a", 120, State{}); rec || p.Best["a"] != 80 {
		t.Fatal("slower run must not replace the record")
	}
	p.Complete(c, "b", 50, State{})
	next, done, _ = p.Complete(c, "c", 70, State{HP: 10})
	if next != "" || !done || !p.Finished || p.CanContinue() || !p.Completed["c"] {
		t.Fatalf("finish: %+v", p)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	c := campaign(t)
	p := New(c)
	p.NewGame(c)
	p.Complete(c, "a", 61.5, State{HP: 42, Items: map[string]int{"relikwia": 2}, Vars: map[string]float64{"score": 350}, Ammo: map[string]int{}})
	path := filepath.Join(t.TempDir(), FileName)
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path, c)
	if err != nil {
		t.Fatal(err)
	}
	if q.Current != "b" || !q.Unlocked["b"] || !q.Completed["a"] || q.Best["a"] != 61.5 ||
		q.Carry.HP != 42 || q.Carry.Items["relikwia"] != 2 || q.Carry.Vars["score"] != 350 {
		t.Fatalf("loaded %+v", q)
	}
	if fresh, err := Load(filepath.Join(t.TempDir(), "none.aie"), c); err != nil || fresh.CanContinue() {
		t.Fatal("missing save must give fresh progress")
	}
}

func TestCaptureAndApplyCarry(t *testing.T) {
	c := campaign(t)
	e := scripting.NewEngine()
	e.SetPlayerHP(55)
	e.GiveItem("relikwia", 3)
	e.SetVar("score", 1200.0)
	e.SetVar("not_carried", 5.0)
	st := c.Capture(e)
	if st.HP != 55 || st.Items["relikwia"] != 3 || st.Vars["score"] != 1200 || len(st.Vars) != 1 {
		t.Fatalf("captured %+v", st)
	}
	e2 := scripting.NewEngine()
	st.Apply(e2)
	if e2.GetPlayerHP() != 55 || e2.ItemCount("relikwia") != 3 || e2.GetVar("score") != 1200.0 || e2.GetVar("not_carried") != nil {
		t.Fatalf("applied: hp %v items %v vars %v", e2.GetPlayerHP(), e2.Inventory, e2.VarsSnapshot())
	}
	if FormatTime(83.4) != "1:23" {
		t.Fatal(FormatTime(83.4))
	}
}
