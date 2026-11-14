// gameplay/gameplay_test.go — Unit tests for map triggers.

package gameplay

import (
	"testing"
)

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
