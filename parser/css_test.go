// parser/css_test.go — Unit tests for the CSS parser.

package parser

import (
	"strings"
	"testing"
)

func TestParseCSSBasic(t *testing.T) {
	input := `.grunt {
    color: #666666;
    texture: cloth;
}
`

	doc, err := ParseCSS(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSS failed: %v", err)
	}

	if len(doc.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(doc.Rules))
	}

	rule := doc.Rules[0]
	if rule.Selector != ".grunt" {
		t.Errorf("expected .grunt, got %s", rule.Selector)
	}
	if rule.GetProperty("color") != "#666666" {
		t.Errorf("expected color=#666666, got %s", rule.GetProperty("color"))
	}
	if rule.GetProperty("texture") != "cloth" {
		t.Errorf("expected texture=cloth, got %s", rule.GetProperty("texture"))
	}
}

func TestParseCSSPseudoClass(t *testing.T) {
	input := `.grunt:pain {
    color-flash: #FF0000;
    flash-duration: 0.1;
}
`

	doc, err := ParseCSS(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSS failed: %v", err)
	}

	rule := doc.Rules[0]
	if rule.Selector != ".grunt:pain" {
		t.Errorf("expected .grunt:pain, got %s", rule.Selector)
	}
	if rule.GetProperty("color-flash") != "#FF0000" {
		t.Errorf("expected color-flash=#FF0000, got %s", rule.GetProperty("color-flash"))
	}
}

func TestParseCSSExtends(t *testing.T) {
	input := `.boss-gangster extends .gangster {
    color: #8B0000;
    size-multiplier: 1.5;
}
`

	doc, err := ParseCSS(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSS failed: %v", err)
	}

	rule := doc.Rules[0]
	if rule.Extends != "gangster" {
		t.Errorf("expected extends=gangster, got '%s'", rule.Extends)
	}
	if rule.GetProperty("color") != "#8B0000" {
		t.Errorf("expected color=#8B0000, got %s", rule.GetProperty("color"))
	}
}

func TestParseCSSNested(t *testing.T) {
	input := `.gangster {
    color: #4A4A4A;
    .head {
        color: #D2946A;
        texture: skin;
    }
}
`

	doc, err := ParseCSS(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSS failed: %v", err)
	}

	rule := doc.Rules[0]
	if rule.Selector != ".gangster" {
		t.Errorf("expected .gangster, got %s", rule.Selector)
	}
	if len(rule.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(rule.Children))
	}

	child := rule.Children[0]
	if child.Selector != ".head" {
		t.Errorf("expected .head, got %s", child.Selector)
	}
	if child.GetProperty("color") != "#D2946A" {
		t.Errorf("expected color=#D2946A, got %s", child.GetProperty("color"))
	}
}

func TestParseCSSScreenBlock(t *testing.T) {
	input := `screen {
    vignette: 0.3;
    film-grain: 0.1;
    color-grading: noir;
}
`

	doc, err := ParseCSS(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSS failed: %v", err)
	}

	rule := doc.Rules[0]
	if rule.Selector != "screen" {
		t.Errorf("expected screen, got %s", rule.Selector)
	}
	if rule.GetPropertyFloat("vignette") != 0.3 {
		t.Errorf("expected vignette=0.3, got %f", rule.GetPropertyFloat("vignette"))
	}
}

func TestParseCSSPlayerState(t *testing.T) {
	input := `player:low-hp {
    vignette: 0.8;
    red-tint: 0.3;
    pulse: yes;
    pulse-speed: 1.5;
}
`

	doc, err := ParseCSS(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSS failed: %v", err)
	}

	rule := doc.Rules[0]
	if rule.Selector != "player:low-hp" {
		t.Errorf("expected player:low-hp, got %s", rule.Selector)
	}
	if rule.GetPropertyBool("pulse") != true {
		t.Errorf("expected pulse=true")
	}
}

func TestParseCSSColorHex(t *testing.T) {
	r, g, b, a, ok := ParseHexColor("#FF0000")
	if !ok {
		t.Fatal("ParseHexColor failed")
	}
	if r != 1.0 || g != 0.0 || b != 0.0 || a != 1.0 {
		t.Errorf("expected red (1,0,0,1), got (%f,%f,%f,%f)", r, g, b, a)
	}

	r, g, b, a, ok = ParseHexColor("#80FF8040")
	if !ok {
		t.Fatal("ParseHexColor failed for #RGBA")
	}
	if a < 0.24 || a > 0.26 { // 0x40 = 64, 64/255 ≈ 0.251
		t.Errorf("expected alpha ≈ 0.251, got %f", a)
	}
}

func TestResolveInheritance(t *testing.T) {
	input := `.gangster {
    color: #4A4A4A;
    texture: cloth;
}
.boss-gangster extends .gangster {
    color: #8B0000;
    size-multiplier: 1.5;
}
`

	doc, err := ParseCSS(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSS failed: %v", err)
	}

	doc.ResolveInheritance()

	boss := doc.FindRule(".boss-gangster")
	if boss == nil {
		t.Fatal("expected .boss-gangster rule")
	}

	// texture should be inherited from .gangster.
	if boss.GetProperty("texture") != "cloth" {
		t.Errorf("expected inherited texture=cloth, got '%s'", boss.GetProperty("texture"))
	}

	// color should be overridden.
	if boss.GetProperty("color") != "#8B0000" {
		t.Errorf("expected overridden color=#8B0000, got '%s'", boss.GetProperty("color"))
	}
}
