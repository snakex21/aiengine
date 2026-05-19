// parser/css.go — CSS parser for AIEngine styling.
// Handles .css files that define colors, textures, effects, lighting, and screen post-processing.
// Supports selectors with pseudo-classes, inheritance (extends), and nested rules.
// All rules are stored as pointers to avoid copy-on-append issues.

package parser

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// CSSDocument is the top-level result of parsing a .css file.
type CSSDocument struct {
	Rules []*CSSRule
}

// CSSRule represents a single CSS rule block.
// Selector examples: ".grunt", "#player", ".grunt:pain", ".grunt .head", "screen", "player:low-hp"
type CSSRule struct {
	Selector   string            // full selector string (e.g. ".grunt:pain")
	Properties map[string]string // key: value pairs (value has semicolons stripped)
	Extends    string            // base class name from "extends .base" syntax
	Children   []*CSSRule        // nested rules
	Line       int               // source line number
}

// ParseCSS reads a .css file from reader and returns the parsed document.
func ParseCSS(r io.Reader) (*CSSDocument, error) {
	lines, err := readLines(r)
	if err != nil {
		return nil, fmt.Errorf("reading .css: %w", err)
	}

	doc := &CSSDocument{}
	stack := &cssStack{}

	// currentRule is the innermost open rule; nil means at document level.
	var currentRule *CSSRule

	for i, raw := range lines {
		lineNo := i + 1
		trimmed := strings.TrimSpace(raw)

		// Skip empty lines and comments.
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		// Check for opening brace — selector line.
		if strings.Contains(trimmed, "{") && !strings.HasPrefix(trimmed, "}") {
			braceIdx := strings.Index(trimmed, "{")
			selector := strings.TrimSpace(trimmed[:braceIdx])

			// Check for "extends" syntax.
			extends := ""
			if idx := strings.Index(strings.ToLower(selector), " extends "); idx >= 0 {
				extends = strings.TrimSpace(selector[idx+len(" extends "):])
				selector = strings.TrimSpace(selector[:idx])
				extends = strings.TrimPrefix(extends, ".")
			}

			rule := &CSSRule{
				Selector:   selector,
				Properties: make(map[string]string),
				Extends:    extends,
				Line:       lineNo,
			}

			// Parse inline properties after the brace on the same line.
			rest := strings.TrimSpace(trimmed[braceIdx+1:])
			rest = strings.TrimSuffix(rest, "}")
			if rest != "" {
				parseCSSProperties(rest, rule.Properties)
			}

			// Attach to parent or document.
			if currentRule != nil {
				currentRule.Children = append(currentRule.Children, rule)
			} else {
				doc.Rules = append(doc.Rules, rule)
			}

			// If block is not self-closing, push onto stack.
			if !strings.HasSuffix(strings.TrimSpace(trimmed[braceIdx+1:]), "}") {
				stack.push(currentRule, rule)
				currentRule = rule
			}
			continue
		}

		// Check for closing brace.
		if trimmed == "}" {
			if stack.depth() > 0 {
				entry := stack.pop()
				currentRule = entry.parent
			}
			continue
		}

		// Property line within an open rule.
		if currentRule != nil {
			parseCSSProperties(trimmed, currentRule.Properties)
		}
	}

	return doc, nil
}

// cssStackEntry tracks the parent rule when entering a nested block.
type cssStackEntry struct {
	parent *CSSRule
	child  *CSSRule
}

// cssStack manages nested rule hierarchy during CSS parsing.
type cssStack struct {
	entries []cssStackEntry
}

func (s *cssStack) push(parent *CSSRule, child *CSSRule) {
	s.entries = append(s.entries, cssStackEntry{parent: parent, child: child})
}

func (s *cssStack) pop() cssStackEntry {
	if len(s.entries) == 0 {
		return cssStackEntry{}
	}
	idx := len(s.entries) - 1
	e := s.entries[idx]
	s.entries = s.entries[:idx]
	return e
}

func (s *cssStack) depth() int {
	return len(s.entries)
}

// parseCSSProperties extracts key: value pairs from a CSS property string.
// Each declaration is separated by semicolon.
func parseCSSProperties(line string, props map[string]string) {
	declarations := strings.Split(line, ";")
	for _, decl := range declarations {
		decl = strings.TrimSpace(decl)
		if decl == "" {
			continue
		}

		colonIdx := strings.Index(decl, ":")
		if colonIdx < 0 {
			continue
		}

		key := strings.TrimSpace(decl[:colonIdx])
		value := strings.TrimSpace(decl[colonIdx+1:])
		value = strings.TrimSuffix(value, ";")

		if key != "" {
			props[key] = value
		}
	}
}

// --- CSS Rule helpers ---

// GetProperty retrieves a property by key (case-insensitive).
func (r *CSSRule) GetProperty(key string) string {
	for k, v := range r.Properties {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// GetPropertyFloat retrieves a float property.
func (r *CSSRule) GetPropertyFloat(key string) float64 {
	v := r.GetProperty(key)
	if v == "" {
		return 0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return f
}

// GetPropertyBool retrieves a boolean property.
func (r *CSSRule) GetPropertyBool(key string) bool {
	v := strings.ToLower(r.GetProperty(key))
	return v == "true" || v == "yes" || v == "1"
}

// GetColor parses a CSS color value (#RRGGBB or #RRGGBBAA) from a property.
func (r *CSSRule) GetColor(key string) (float64, float64, float64, float64, bool) {
	v := r.GetProperty(key)
	if v == "" {
		return 0, 0, 0, 1, false
	}
	return ParseHexColor(v)
}

// FindChild finds a direct child rule by selector.
func (r *CSSRule) FindChild(selector string) *CSSRule {
	for _, c := range r.Children {
		if c.Selector == selector {
			return c
		}
	}
	return nil
}

// --- Document-level helpers ---

// FindRule searches the document (including nested rules) for a rule by selector.
func (doc *CSSDocument) FindRule(selector string) *CSSRule {
	var search func(rules []*CSSRule) *CSSRule
	search = func(rules []*CSSRule) *CSSRule {
		for _, r := range rules {
			if r.Selector == selector {
				return r
			}
			if found := search(r.Children); found != nil {
				return found
			}
		}
		return nil
	}
	return search(doc.Rules)
}

// ResolveInheritance merges properties from extended base rules into derived rules.
// The derived rule's own properties take precedence.
func (doc *CSSDocument) ResolveInheritance() {
	// Build a map of selectors to rules for quick lookup.
	ruleMap := make(map[string]*CSSRule)
	var index func(rules []*CSSRule)
	index = func(rules []*CSSRule) {
		for _, r := range rules {
			ruleMap[r.Selector] = r
			index(r.Children)
		}
	}
	index(doc.Rules)

	var resolve func(rules []*CSSRule)
	resolve = func(rules []*CSSRule) {
		for _, r := range rules {
			if r.Extends != "" {
				base, ok := ruleMap["."+r.Extends]
				if !ok {
					base = ruleMap[r.Extends]
				}
				if base != nil {
					for k, v := range base.Properties {
						if _, exists := r.Properties[k]; !exists {
							r.Properties[k] = v
						}
					}
				}
				r.Extends = ""
			}
			resolve(r.Children)
		}
	}
	resolve(doc.Rules)
}

// --- Color parsing utility ---

// ParseHexColor parses a hex color string (#RGB, #RRGGBB, #RRGGBBAA) and returns RGBA floats (0-1).
func ParseHexColor(hex string) (float64, float64, float64, float64, bool) {
	hex = strings.TrimPrefix(hex, "#")

	var r, g, b uint8
	a := uint8(255)

	switch len(hex) {
	case 3:
		r = hexToByte(hex[0:1] + hex[0:1])
		g = hexToByte(hex[1:2] + hex[1:2])
		b = hexToByte(hex[2:3] + hex[2:3])
	case 4:
		r = hexToByte(hex[0:1] + hex[0:1])
		g = hexToByte(hex[1:2] + hex[1:2])
		b = hexToByte(hex[2:3] + hex[2:3])
		a = hexToByte(hex[3:4] + hex[3:4])
	case 6:
		r = hexToByte(hex[0:2])
		g = hexToByte(hex[2:4])
		b = hexToByte(hex[4:6])
	case 8:
		r = hexToByte(hex[0:2])
		g = hexToByte(hex[2:4])
		b = hexToByte(hex[4:6])
		a = hexToByte(hex[6:8])
	default:
		return 0, 0, 0, 1, false
	}

	return float64(r) / 255.0, float64(g) / 255.0, float64(b) / 255.0, float64(a) / 255.0, true
}

func hexToByte(s string) uint8 {
	var val uint8
	for _, ch := range s {
		val <<= 4
		switch {
		case ch >= '0' && ch <= '9':
			val |= uint8(ch - '0')
		case ch >= 'a' && ch <= 'f':
			val |= uint8(ch - 'a' + 10)
		case ch >= 'A' && ch <= 'F':
			val |= uint8(ch - 'A' + 10)
		default:
			return 0
		}
	}
	return val
}

// NamedColor maps CSS color names to hex strings.
var NamedColor = map[string]string{
	"black":   "#000000",
	"white":   "#FFFFFF",
	"red":     "#FF0000",
	"green":   "#00FF00",
	"blue":    "#0000FF",
	"yellow":  "#FFFF00",
	"cyan":    "#00FFFF",
	"magenta": "#FF00FF",
	"gray":    "#808080",
	"grey":    "#808080",
	"orange":  "#FFA500",
	"purple":  "#800080",
	"pink":    "#FFC0CB",
	"brown":   "#A52A2A",
	"navy":    "#000080",
	"teal":    "#008080",
	"maroon":  "#800000",
	"olive":   "#808000",
	"lime":    "#00FF00",
	"silver":  "#C0C0C0",
	"gold":    "#FFD700",
	"coral":   "#FF7F50",
	"crimson": "#DC143C",
	"indigo":  "#4B0082",
	"violet":  "#EE82EE",
}

// ResolveColor parses any color value: hex (#RGB), named color, or returns false.
func ResolveColor(value string) (float64, float64, float64, float64, bool) {
	if strings.HasPrefix(value, "#") {
		return ParseHexColor(value)
	}
	if hex, ok := NamedColor[strings.ToLower(value)]; ok {
		return ParseHexColor(hex)
	}
	return 0, 0, 0, 1, false
}
