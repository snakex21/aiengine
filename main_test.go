package main

import "testing"

func TestParseRunOptions(t *testing.T) {
	o, err := parseRunOptions([]string{"--project", "krypta", "--screenshot", "a.png", "--exec", "god; tp 1 2"})
	if err != nil {
		t.Fatal(err)
	}
	if o.project != "krypta" || o.screenshot != "a.png" || o.frames != 60 || o.exec != "god; tp 1 2" {
		t.Fatalf("opts = %+v", o)
	}
	if o, _ = parseRunOptions([]string{"--frames", "10"}); o.frames != 10 || o.screenshot != "" {
		t.Fatalf("frames only = %+v", o)
	}
	for _, bad := range [][]string{{"--frames"}, {"--frames", "x"}, {"--nope"}, {"--project"}} {
		if _, err := parseRunOptions(bad); err == nil {
			t.Errorf("%v: expected error", bad)
		}
	}
}

func TestParseHexColor(t *testing.T) {
	c, ok := parseHexColor("#1E1A24")
	if !ok || c.R != 0x1E || c.G != 0x1A || c.B != 0x24 || c.A != 255 {
		t.Fatalf("got %v %v", c, ok)
	}
	if _, ok := parseHexColor("red"); ok {
		t.Fatal("expected failure")
	}
}

func TestParseExecFrames(t *testing.T) {
	got := parseExec("menu close; @30 advance; @5 tp 1 2; god")
	if len(got[0]) != 2 || got[0][1] != "god" || got[30][0] != "advance" || got[5][0] != "tp 1 2" {
		t.Fatalf("parseExec = %v", got)
	}
}
