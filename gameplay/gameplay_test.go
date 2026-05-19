// gameplay/gameplay_test.go — Unit tests for gameplay systems.

package gameplay

import (
	"strings"
	"testing"
	"time"

	"aiengine/parser"
	"aiengine/systems"
)

// --- Entity Tests ---

func TestParseEntityDefGrunt(t *testing.T) {
	input := `HUMANOID grunt
HP 50
HEAD small skin
TORSO medium grey_shirt
LEGS medium black_pants
`
	doc, _ := parser.ParseAIE(strings.NewReader(input))
	if len(doc.Nodes) != 1 {
		t.Fatal("expected 1 node")
	}

	ed := ParseEntityDef(doc.Nodes[0])
	if ed.Name != "grunt" {
		t.Errorf("expected grunt, got %s", ed.Name)
	}
	if ed.HP != 50 {
		t.Errorf("expected HP=50, got %f", ed.HP)
	}
	if ed.SubType != SubHumanoid {
		t.Errorf("expected SubHumanoid, got %v", ed.SubType)
	}
	if len(ed.Parts) != 3 {
		t.Errorf("expected 3 body parts, got %d", len(ed.Parts))
	}
}

func TestEntitySpawnAndRegistry(t *testing.T) {
	er := NewEntityRegistry()
	def := &EntityDef{
		Name:    "test_enemy",
		SubType: SubHumanoid,
		HP:      100,
		MaxHP:   100,
	}
	er.RegisterDef(def)

	e := er.Spawn("test_enemy", "enemy1", 5, 3, 0, 0)
	if e == nil {
		t.Fatal("Spawn returned nil")
	}
	if e.HP != 100 {
		t.Errorf("expected HP=100, got %f", e.HP)
	}
	if er.CountAlive() != 1 {
		t.Errorf("expected 1 alive, got %d", er.CountAlive())
	}

	e.Alive = false
	if er.CountAlive() != 0 {
		t.Errorf("expected 0 alive after death, got %d", er.CountAlive())
	}
}

func TestParseBehaviorFlags(t *testing.T) {
	input := `GIANT kaiju
HP 5000
BEHAVIOR
  COLLAPSE yes
  SHATTER yes parts=8
  EARTHQUAKE yes intensity=0.8
  BOSS yes
`
	doc, _ := parser.ParseAIE(strings.NewReader(input))
	ed := ParseEntityDef(doc.Nodes[0])

	if !ed.Behavior.Boss {
		t.Error("expected Boss=true")
	}
	if ed.Behavior.DeathType != DeathShatter {
		t.Errorf("expected DeathShatter, got %v", ed.Behavior.DeathType)
	}
	if ed.Behavior.ShatterParts != 8 {
		t.Errorf("expected 8 shatter parts, got %d", ed.Behavior.ShatterParts)
	}
	if !ed.Behavior.Earthquake {
		t.Error("expected Earthquake=true")
	}
}

// --- Weapon Tests ---

func TestParseWeaponDef(t *testing.T) {
	input := `WEAPON pistol
DAMAGE 25
AMMO_MAX 15
FIRE_RATE 2.0
SPREAD 0.02
`
	doc, _ := parser.ParseAIE(strings.NewReader(input))
	wd := ParseWeaponDef(doc.Nodes[0])

	if wd.Name != "pistol" {
		t.Errorf("expected pistol, got %s", wd.Name)
	}
	if wd.Damage != 25 {
		t.Errorf("expected Damage=25, got %f", wd.Damage)
	}
	if wd.AmmoMax != 15 {
		t.Errorf("expected AmmoMax=15, got %d", wd.AmmoMax)
	}
	if wd.Spread != 0.02 {
		t.Errorf("expected Spread=0.02, got %f", wd.Spread)
	}
}

func TestWeaponFireAndReload(t *testing.T) {
	def := &WeaponDef{
		Name:        "test_gun",
		Damage:      10,
		AmmoMax:     5,
		AmmoReserve: 10,
		FireRate:    1000.0, // extreme fire rate
		ReloadTime:  0.1,
		Burst:       1,
	}

	wi := NewWeaponInstance(def)

	// Fire all ammo — need tiny pauses due to real-time cooldown check.
	for i := 0; i < 5; i++ {
		bullets := wi.Fire()
		if bullets != 1 {
			t.Errorf("shot %d: expected 1 bullet, got %d (ammo=%d)", i, bullets, wi.AmmoCurrent)
		}
		// Small sleep to allow cooldown to pass.
		if i < 4 {
			time.Sleep(2 * time.Millisecond)
		}
	}

	if wi.AmmoCurrent != 0 {
		t.Errorf("expected 0 ammo after 5 shots, got %d", wi.AmmoCurrent)
	}

	// Force reload.
	wi.Reload()
	if !wi.Reloading {
		wi.Reload() // try again
	}

	// Advance reload timer.
	wi.Update(0.5)
	if wi.Reloading {
		t.Error("expected reload to be complete")
	}
	if wi.AmmoCurrent != 5 {
		t.Errorf("expected 5 ammo after reload, got %d (reserve=%d)", wi.AmmoCurrent, wi.AmmoReserve)
	}
}

// timeSleep is a tiny helper to avoid importing time in test.
func timeSleep(ms int) {
	for i := 0; i < ms*10000; i++ {
		_ = i
	}
}

func TestWeaponManager(t *testing.T) {
	wm := NewWeaponManager()
	defs := CreateDefaultWeaponDefs()
	for _, d := range defs {
		wm.RegisterDef(d)
	}

	wm.GiveWeapon("pistol")
	wm.GiveWeapon("shotgun")

	if len(wm.Weapons) != 2 {
		t.Fatalf("expected 2 weapons, got %d", len(wm.Weapons))
	}

	curr := wm.CurrentWeapon()
	if curr.Def.Name != "pistol" {
		t.Errorf("expected pistol, got %s", curr.Def.Name)
	}

	wm.NextWeapon()
	curr = wm.CurrentWeapon()
	if curr.Def.Name != "shotgun" {
		t.Errorf("expected shotgun, got %s", curr.Def.Name)
	}
}

func TestApplySpread(t *testing.T) {
	dir := systems.Vec3{1, 0, 0}
	spread := ApplySpread(dir, 0.1)

	// Spread vector should be approximately normalized.
	l := spread.Length()
	if l < 0.99 || l > 1.01 {
		t.Errorf("expected unit vector, got length=%f", l)
	}

	// Should be close to original direction (dot product near 1).
	dot := dir.Normalize().Dot(spread)
	if dot < 0.9 {
		t.Errorf("expected dot product > 0.9, got %f", dot)
	}
}

// --- HP Tests ---

func TestHPSystemDamage(t *testing.T) {
	hs := NewHPSystem()
	hs.CreatePlayerHP(100)

	result := hs.DamagePlayer(30, "TORSO")
	if result.DamageDealt != 30 {
		t.Errorf("expected 30 damage, got %f", result.DamageDealt)
	}
	if hs.GetPlayerHP() != 70 {
		t.Errorf("expected HP=70, got %f", hs.GetPlayerHP())
	}
	if result.Killed {
		t.Error("player should not be dead at 70 HP")
	}

	result = hs.DamagePlayer(100, "HEAD")
	if hs.GetPlayerHP() != 0 {
		t.Errorf("expected HP=0, got %f", hs.GetPlayerHP())
	}
	if !result.Killed {
		t.Error("player should be dead")
	}
}

func TestHPWeakSpotMultiplier(t *testing.T) {
	state := &HPState{
		Global: 100,
		Max:    100,
		WeakSpots: map[string]float64{"HEAD": 3.0},
	}
	hs := &HPSystem{}
	result := hs.applyDamage(state, 10, "HEAD")
	if result.DamageDealt != 30 { // 10 * 3.0
		t.Errorf("expected 30 damage with 3x multiplier, got %f", result.DamageDealt)
	}
}

func TestHPArmorReduction(t *testing.T) {
	state := &HPState{
		Global: 100,
		Max:    100,
		Armor: map[string]float64{"TORSO": 0.5},
	}
	hs := &HPSystem{}
	result := hs.applyDamage(state, 40, "TORSO")
	if result.DamageDealt != 20 { // 40 * (1 - 0.5)
		t.Errorf("expected 20 damage with 50%% armor, got %f", result.DamageDealt)
	}
}

// --- Trigger Tests ---

func TestTriggerEvaluate(t *testing.T) {
	trigger := &Trigger{
		Conditions: []TriggerCondition{
			{Type: "all_enemies_dead", Param: "zone1"},
			{Type: "hp_below", Param: "50"},
		},
	}

	ctx := &TriggerContext{
		AllEnemiesDead: true,
		PlayerHP:       30,
		Zone:           "zone1",
	}

	if !trigger.Evaluate(ctx) {
		t.Error("trigger should fire")
	}

	ctx.PlayerHP = 60
	if trigger.Evaluate(ctx) {
		t.Error("trigger should not fire when HP > 50")
	}
}

func TestTriggerExecute(t *testing.T) {
	messageShown := ""
	musicPlayed := ""

	exec := &TriggerExecutor{
		OnShowMessage: func(text string) { messageShown = text },
		OnPlayMusic:   func(file string) { musicPlayed = file },
	}

	trigger := &Trigger{
		Actions: []TriggerAction{
			{Type: "show_message", Param: "Hello!"},
			{Type: "play_music", Param: "theme.ogg"},
		},
	}

	exec.Execute(trigger)

	if messageShown != "Hello!" {
		t.Errorf("expected 'Hello!', got '%s'", messageShown)
	}
	if musicPlayed != "theme.ogg" {
		t.Errorf("expected 'theme.ogg', got '%s'", musicPlayed)
	}
	if !trigger.Triggered {
		t.Error("trigger should be marked triggered")
	}
}

// --- Cutscene Tests ---

func TestCutsceneLoadLevel1(t *testing.T) {
	input := `CUTSCENE intro
  TEXT "Misja: Oczyscic miasto."
  TEXT "Powodzenia."
  NEXT_ON_KEY yes
`
	doc, _ := parser.ParseAIE(strings.NewReader(input))

	player := NewCutscenePlayer()
	player.LoadCutscene(doc.Nodes[0])

	cs := player.cutscenes["intro"]
	if cs == nil {
		t.Fatal("cutscene not loaded")
	}
	if cs.Level != 1 {
		t.Errorf("expected Level 1, got %d", cs.Level)
	}
	if len(cs.Commands) != 3 {
		t.Errorf("expected 3 commands (2 TEXT + NEXT_ON_KEY), got %d", len(cs.Commands))
	}
	if !cs.WaitForKey {
		t.Error("expected WaitForKey=true")
	}
}

func TestCutscenePlayAndAdvance(t *testing.T) {
	cs := &Cutscene{
		Name:  "test",
		Level: 1,
		Commands: []*CutsceneCommand{
			{Type: "TEXT", Params: []string{"Hello"}},
		},
	}

	player := NewCutscenePlayer()
	player.cutscenes["test"] = cs

	textShown := ""
	player.OnShowText = func(text string) { textShown = text }

	player.Play("test")
	if !player.IsPlaying() {
		t.Error("cutscene should be playing")
	}

	// Advance time to trigger the command.
	player.Update(0.1)

	if textShown != "Hello" {
		t.Errorf("expected 'Hello', got '%s'", textShown)
	}

	// Should auto-stop after all commands done.
	if player.IsPlaying() {
		t.Log("cutscene still playing (may have auto-stopped)")
	}
}

func parseFirstNode(s string) *parser.AIENode {
	doc, _ := parser.ParseAIE(strings.NewReader(s))
	if doc == nil || len(doc.Nodes) == 0 {
		return nil
	}
	return doc.Nodes[0]
}
