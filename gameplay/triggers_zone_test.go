package gameplay

import (
	"strings"
	"testing"

	"aiengine/parser"
)

func mustTrigger(t *testing.T, src string) *parser.AIENode {
	t.Helper()
	doc, err := parser.ParseAIE(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	return doc.Nodes[0]
}

func TestTriggerZoneEnterExitAndVars(t *testing.T) {
	tr := NewTriggerRegistry()
	var fired []string
	tr.SetExecutor(&TriggerExecutor{
		OnShowMessage: func(text string) { fired = append(fired, text) },
	})
	tr.AddTrigger(mustTrigger(t, "TRIGGER enter\n  CONDITION player_enters exit\n  ACTION show_message entered\n"))
	tr.AddTrigger(mustTrigger(t, "TRIGGER leave\n  CONDITION player_exits exit\n  ACTION show_message left\n"))
	tr.AddTrigger(mustTrigger(t, "TRIGGER enough\n  CONDITION var_at_least crystals 3\n  ACTION show_message enough\n"))
	tr.AddTrigger(mustTrigger(t, "TRIGGER mode\n  CONDITION var_equals mode hard\n  ACTION show_message hard\n"))

	tr.Tick()
	if len(fired) != 0 {
		t.Fatalf("fired too early: %v", fired)
	}

	tr.SetZoneInside("exit", true)
	tr.SetVar("crystals", 2.0)
	tr.Tick()
	tr.SetZoneInside("exit", false)
	tr.SetVar("crystals", 3.0)
	tr.SetVar("mode", "hard")
	tr.Tick()
	tr.Tick() // non-repeatable triggers must not fire twice

	want := "entered,left,enough,hard"
	if got := strings.Join(fired, ","); got != want {
		t.Fatalf("fired = %s, want %s", got, want)
	}
}
