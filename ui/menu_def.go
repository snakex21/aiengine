// Package ui implements data-driven menus (title screen, pause, settings,
// level select) defined per project in a menu file (see docs/ui-menus.md).
//
// It is pure Go: menus are drawn into the renderer's RGBA UI layer and
// driven by abstract inputs, so navigation is testable without a window.
package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Def is a parsed menu file.
type Def struct {
	Screens   map[string]*Screen
	Order     []string          // screen names in file order
	Sounds    map[string]string // move/select/back → sound name
	SoundDefs []SoundLine       // inline SOUND lines to register
	Style     Style
}

// SoundLine is "SOUND <name> KEY=value ..." (procedural sound parameters).
type SoundLine struct {
	Name  string
	Props map[string]string
}

// Style holds menu colours (RGBA).
type Style struct {
	Title, Text, Focus, FocusBG, Dim, Hint [4]uint8
}

// DefaultStyle is used for colours a menu file does not set.
func DefaultStyle() Style {
	return Style{
		Title:   [4]uint8{255, 214, 120, 255},
		Text:    [4]uint8{215, 215, 225, 255},
		Focus:   [4]uint8{130, 230, 255, 255},
		FocusBG: [4]uint8{40, 60, 100, 170},
		Dim:     [4]uint8{0, 0, 0, 160},
		Hint:    [4]uint8{150, 150, 165, 255},
	}
}

// Screen is one menu page.
type Screen struct {
	Name       string
	Title      string
	Subtitle   string
	Hint       string
	Background Background
	Music      string
	Escape     Action // action for Esc/Back on the root screen (e.g. resume)
	Items      []Item
	Labels     map[string]string // overrides for generated rows (settings, back...)
}

// Background describes what is behind a screen.
type Background struct {
	Kind  string    // none, color, image, dim, map
	Color [4]uint8  // for color
	Image string    // for image (path relative to the project)
	Args  []float64 // for map: x y angle_deg (camera placement)
}

// Item is a selectable row.
type Item struct {
	Label    string
	Action   Action
	Setting  string // settings row key (generated)
	Disabled bool   // shown but cannot be activated (e.g. locked level)
}

// Action is what activating an item asks the host to do.
type Action struct {
	Kind string // new_game, load_level, level_select, settings, resume, restart, main_menu, quit, run_script, menu, back, settings_changed
	Arg  string
}

// Known action kinds (anything else is rejected when parsing).
var actionKinds = map[string]bool{
	"new_game": true, "load_level": true, "level_select": true, "settings": true,
	"resume": true, "restart": true, "main_menu": true, "quit": true,
	"run_script": true, "menu": true, "back": true, "continue": true,
}

// actionsWithArg must have an argument.
var actionsWithArg = map[string]bool{"load_level": true, "run_script": true, "menu": true}

// ParseFile reads a menu file.
func ParseFile(path string) (*Def, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	d, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return d, nil
}

// Parse reads a menu definition. See docs/ui-menus.md for the format.
func Parse(r io.Reader) (*Def, error) {
	d := &Def{Screens: map[string]*Screen{}, Sounds: map[string]string{}, Style: DefaultStyle()}
	var cur *Screen
	sc := bufio.NewScanner(r)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := splitQuoted(line)
		key := strings.ToUpper(f[0])
		args := f[1:]
		fail := func(msg string, a ...any) error {
			return fmt.Errorf("line %d: %s", lineNo, fmt.Sprintf(msg, a...))
		}
		switch key {
		case "SCREEN":
			if len(args) < 1 {
				return nil, fail("SCREEN needs a name")
			}
			name := strings.ToLower(args[0])
			if d.Screens[name] != nil {
				return nil, fail("duplicate SCREEN %s", name)
			}
			cur = &Screen{Name: name, Labels: map[string]string{}, Background: Background{Kind: "none"}}
			d.Screens[name] = cur
			d.Order = append(d.Order, name)
		case "SOUND":
			if len(args) < 1 {
				return nil, fail("SOUND needs a name")
			}
			sl := SoundLine{Name: args[0], Props: map[string]string{}}
			for _, kv := range args[1:] {
				if k, v, ok := strings.Cut(kv, "="); ok {
					sl.Props[strings.ToUpper(k)] = v
				}
			}
			d.SoundDefs = append(d.SoundDefs, sl)
		case "SOUNDS":
			for _, kv := range args {
				if k, v, ok := strings.Cut(kv, "="); ok {
					d.Sounds[strings.ToLower(k)] = v
				}
			}
		case "STYLE":
			for _, kv := range args {
				k, v, ok := strings.Cut(kv, "=")
				if !ok {
					continue
				}
				c, ok := ParseColor(v)
				if !ok {
					return nil, fail("bad colour %q", v)
				}
				switch strings.ToLower(k) {
				case "title_color":
					d.Style.Title = c
				case "text_color":
					d.Style.Text = c
				case "focus_color":
					d.Style.Focus = c
				case "focus_bg":
					d.Style.FocusBG = c
				case "dim_color":
					d.Style.Dim = c
				case "hint_color":
					d.Style.Hint = c
				default:
					return nil, fail("unknown STYLE key %q", k)
				}
			}
		default:
			if cur == nil {
				return nil, fail("%s outside a SCREEN", key)
			}
			if err := parseScreenLine(cur, key, args); err != nil {
				return nil, fail("%v", err)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return d, nil
}

func parseScreenLine(s *Screen, key string, args []string) error {
	text := strings.Join(args, " ")
	switch key {
	case "TITLE":
		s.Title = text
	case "SUBTITLE":
		s.Subtitle = text
	case "HINT":
		s.Hint = text
	case "MUSIC":
		s.Music = text
	case "LABEL":
		if len(args) < 2 {
			return fmt.Errorf("LABEL needs <key> <text>")
		}
		s.Labels[strings.ToLower(args[0])] = strings.Join(args[1:], " ")
	case "ESCAPE":
		a, err := parseAction(args)
		if err != nil {
			return err
		}
		s.Escape = a
	case "ITEM":
		if len(args) < 2 {
			return fmt.Errorf("ITEM needs <label> <action> [arg]")
		}
		a, err := parseAction(args[1:])
		if err != nil {
			return err
		}
		s.Items = append(s.Items, Item{Label: args[0], Action: a})
	case "BACKGROUND":
		if len(args) < 1 {
			return fmt.Errorf("BACKGROUND needs a kind")
		}
		bg := Background{Kind: strings.ToLower(args[0])}
		switch bg.Kind {
		case "none", "dim":
		case "color":
			if len(args) < 2 {
				return fmt.Errorf("BACKGROUND color needs #RRGGBB")
			}
			c, ok := ParseColor(args[1])
			if !ok {
				return fmt.Errorf("bad colour %q", args[1])
			}
			bg.Color = c
		case "image":
			if len(args) < 2 {
				return fmt.Errorf("BACKGROUND image needs a path")
			}
			bg.Image = args[1]
		case "map":
			for _, a := range args[1:] {
				v, err := strconv.ParseFloat(a, 64)
				if err != nil {
					return fmt.Errorf("BACKGROUND map expects numbers, got %q", a)
				}
				bg.Args = append(bg.Args, v)
			}
		default:
			return fmt.Errorf("unknown BACKGROUND %q (none, color, image, dim, map)", bg.Kind)
		}
		s.Background = bg
	default:
		return fmt.Errorf("unknown directive %s", key)
	}
	return nil
}

func parseAction(args []string) (Action, error) {
	if len(args) == 0 {
		return Action{}, fmt.Errorf("missing action")
	}
	a := Action{Kind: strings.ToLower(args[0]), Arg: strings.Join(args[1:], " ")}
	if !actionKinds[a.Kind] {
		return a, fmt.Errorf("unknown action %q", a.Kind)
	}
	if actionsWithArg[a.Kind] && a.Arg == "" {
		return a, fmt.Errorf("action %s needs an argument", a.Kind)
	}
	if a.Kind == "menu" {
		a.Arg = strings.ToLower(a.Arg)
	}
	return a, nil
}

// splitQuoted splits on whitespace, keeping "double quoted" text together.
func splitQuoted(line string) []string {
	var out []string
	var cur strings.Builder
	inQ, had := false, false
	for _, r := range line {
		switch {
		case r == '"':
			inQ = !inQ
			had = true
		case (r == ' ' || r == '\t') && !inQ:
			if cur.Len() > 0 || had {
				out = append(out, cur.String())
				cur.Reset()
				had = false
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 || had {
		out = append(out, cur.String())
	}
	return out
}

// ParseColor parses #RRGGBB or #RRGGBBAA.
func ParseColor(s string) ([4]uint8, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 && len(s) != 8 {
		return [4]uint8{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return [4]uint8{}, false
	}
	if len(s) == 6 {
		return [4]uint8{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, true
	}
	return [4]uint8{uint8(v >> 24), uint8(v >> 16), uint8(v >> 8), uint8(v)}, true
}
