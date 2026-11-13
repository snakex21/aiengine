// Package progress implements campaign progression: an ordered list of
// levels, unlocking on completion, best times, state carried between levels
// (HP, ammo, inventory, chosen script variables) and a save file kept next
// to the game (save.aie in the project directory, never in AppData).
//
// Manifest:
//
//	CAMPAIGN yes
//	CARRY hp items score     # hp, ammo, items (inventory) or variable names
//	LEVEL l1 maps/l1.aie Przedsionek
//	LEVEL l2 maps/l2.aie Kryształowa Krypta
package progress

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"aiengine/parser"
	"aiengine/scripting"
)

// FileName is the save file created in the project directory.
const FileName = "save.aie"

// Campaign is the ordered level list plus what carries over.
type Campaign struct {
	Levels []parser.LevelEntry
	Carry  []string // "hp", "ammo", "items" or variable names
}

// FromManifest returns the campaign of a manifest, or nil when the project
// is not a campaign (no "CAMPAIGN yes").
func FromManifest(m *parser.Manifest) *Campaign {
	switch strings.ToLower(strings.TrimSpace(m.Raw["CAMPAIGN"])) {
	case "yes", "true", "1", "tak":
	default:
		return nil
	}
	return &Campaign{Levels: m.LevelList(), Carry: strings.Fields(strings.ToLower(m.Raw["CARRY"]))}
}

// Index returns the position of a level id (-1 if unknown).
func (c *Campaign) Index(id string) int {
	for i, l := range c.Levels {
		if strings.EqualFold(l.ID, id) {
			return i
		}
	}
	return -1
}

// Next returns the level after id ("" for the last one).
func (c *Campaign) Next(id string) string {
	if i := c.Index(id); i >= 0 && i+1 < len(c.Levels) {
		return c.Levels[i+1].ID
	}
	return ""
}

func (c *Campaign) carries(what string) bool {
	for _, k := range c.Carry {
		if k == what {
			return true
		}
	}
	return false
}

// State is what carries over from one level into the next.
type State struct {
	HP    float64 // 0 = not carried
	Ammo  map[string]int
	Items map[string]int
	Vars  map[string]float64
}

// Empty reports whether nothing is carried.
func (s State) Empty() bool {
	return s.HP == 0 && len(s.Ammo) == 0 && len(s.Items) == 0 && len(s.Vars) == 0
}

// Capture reads the carried state from the scripting engine.
func (c *Campaign) Capture(e *scripting.Engine) State {
	st := State{Ammo: map[string]int{}, Items: map[string]int{}, Vars: map[string]float64{}}
	if c.carries("hp") {
		st.HP = e.GetPlayerHP()
	}
	if c.carries("ammo") {
		for _, w := range e.PlayerWeapons {
			st.Ammo[w] = e.GetPlayerAmmo(w)
		}
	}
	if c.carries("items") {
		for k, v := range e.Inventory {
			st.Items[k] = v
		}
	}
	vars := e.VarsSnapshot()
	for _, k := range c.Carry {
		if k == "hp" || k == "ammo" || k == "items" {
			continue
		}
		if v, ok := vars[k].(float64); ok {
			st.Vars[k] = v
		}
	}
	return st
}

// Apply writes a carried state into a fresh scripting engine (call it after
// the level's scripts are loaded, before onStart).
func (s State) Apply(e *scripting.Engine) {
	if s.HP > 0 {
		e.PlayerHP = s.HP
	}
	for w, a := range s.Ammo {
		e.SetPlayerAmmo(w, a)
	}
	for k, v := range s.Items {
		e.Inventory[k] = v
	}
	for k, v := range s.Vars {
		e.SetVar(k, v)
	}
}

// Progress is the saved campaign state.
type Progress struct {
	Current   string             // level to continue from ("" = no game)
	Unlocked  map[string]bool    // playable from level select
	Completed map[string]bool    // finished at least once
	Best      map[string]float64 // best completion time per level (seconds)
	Finished  bool               // the whole campaign was completed
	Carry     State              // state at the start of Current
}

// New returns fresh progress with the first level unlocked.
func New(c *Campaign) *Progress {
	p := &Progress{Unlocked: map[string]bool{}, Completed: map[string]bool{}, Best: map[string]float64{}}
	if len(c.Levels) > 0 {
		p.Unlocked[c.Levels[0].ID] = true
	}
	return p
}

// CanContinue reports whether a game is in progress.
func (p *Progress) CanContinue() bool { return p.Current != "" && !p.Finished }

// NewGame starts over from the first level, keeping unlocks and records.
func (p *Progress) NewGame(c *Campaign) {
	p.Current, p.Finished, p.Carry = "", false, State{}
	if len(c.Levels) > 0 {
		p.Current = c.Levels[0].ID
	}
}

// Start records that a level is being played from scratch (level select).
func (p *Progress) Start(id string) {
	p.Current, p.Finished, p.Carry = id, false, State{}
}

// Complete records finishing level id in t seconds with the given carried
// state. It unlocks and moves to the next level; finished is true after the
// last level. isRecord reports a new best time.
func (p *Progress) Complete(c *Campaign, id string, t float64, carry State) (next string, finished, isRecord bool) {
	p.Completed[id] = true
	if best, ok := p.Best[id]; !ok || t < best {
		p.Best[id] = t
		isRecord = true
	}
	next = c.Next(id)
	if next == "" {
		p.Current, p.Finished, p.Carry = "", true, State{}
		return "", true, isRecord
	}
	p.Unlocked[next] = true
	p.Current, p.Carry = next, carry
	return next, false, isRecord
}

// FormatTime renders seconds as m:ss.
func FormatTime(t float64) string {
	s := int(t + 0.5)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// Load reads a save file; a missing file gives fresh progress.
func Load(path string, c *Campaign) (*Progress, error) {
	p := New(c)
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fl := strings.Fields(sc.Text())
		if len(fl) < 2 || strings.HasPrefix(fl[0], "#") {
			continue
		}
		switch strings.ToUpper(fl[0]) {
		case "CURRENT":
			p.Current = fl[1]
		case "FINISHED":
			p.Finished = fl[1] == "yes"
		case "UNLOCKED":
			for _, id := range fl[1:] {
				p.Unlocked[id] = true
			}
		case "COMPLETED":
			for _, id := range fl[1:] {
				p.Completed[id] = true
			}
		case "BEST":
			if len(fl) >= 3 {
				if v, err := strconv.ParseFloat(fl[2], 64); err == nil {
					p.Best[fl[1]] = v
				}
			}
		case "CARRY":
			p.loadCarry(fl[1:])
		}
	}
	if p.Current != "" && c.Index(p.Current) < 0 {
		p.Current = "" // level removed from the campaign
	}
	return p, sc.Err()
}

func (p *Progress) loadCarry(f []string) {
	num := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	if p.Carry.Ammo == nil {
		p.Carry = State{Ammo: map[string]int{}, Items: map[string]int{}, Vars: map[string]float64{}}
	}
	switch {
	case f[0] == "HP" && len(f) >= 2:
		p.Carry.HP = num(f[1])
	case f[0] == "AMMO" && len(f) >= 3:
		p.Carry.Ammo[f[1]] = int(num(f[2]))
	case f[0] == "ITEM" && len(f) >= 3:
		p.Carry.Items[f[1]] = int(num(f[2]))
	case f[0] == "VAR" && len(f) >= 3:
		p.Carry.Vars[f[1]] = num(f[2])
	}
}

// Save writes the progress atomically.
func (p *Progress) Save(path string) error {
	var b strings.Builder
	b.WriteString("# AIEngine campaign progress - written by the game\n")
	fmt.Fprintf(&b, "CURRENT %s\n", orDash(p.Current))
	fmt.Fprintf(&b, "FINISHED %s\n", map[bool]string{true: "yes", false: "no"}[p.Finished])
	if ids := keys(p.Unlocked); len(ids) > 0 {
		fmt.Fprintf(&b, "UNLOCKED %s\n", strings.Join(ids, " "))
	}
	if ids := keys(p.Completed); len(ids) > 0 {
		fmt.Fprintf(&b, "COMPLETED %s\n", strings.Join(ids, " "))
	}
	for _, id := range sortedKeys(p.Best) {
		fmt.Fprintf(&b, "BEST %s %.2f\n", id, p.Best[id])
	}
	if p.Carry.HP > 0 {
		fmt.Fprintf(&b, "CARRY HP %.1f\n", p.Carry.HP)
	}
	for _, k := range sortedKeys(p.Carry.Ammo) {
		fmt.Fprintf(&b, "CARRY AMMO %s %d\n", k, p.Carry.Ammo[k])
	}
	for _, k := range sortedKeys(p.Carry.Items) {
		fmt.Fprintf(&b, "CARRY ITEM %s %d\n", k, p.Carry.Items[k])
	}
	for _, k := range sortedKeys(p.Carry.Vars) {
		fmt.Fprintf(&b, "CARRY VAR %s %g\n", k, p.Carry.Vars[k])
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
