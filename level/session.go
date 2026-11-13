package level

import (
	"log"
	"math"
	"strconv"
	"strings"

	"aiengine/gameplay"
	"aiengine/scripting"
)

// EventKind identifies what happened during a Session update.
type EventKind int

const (
	EventPickup    EventKind = iota // player collected an item
	EventZoneEnter                  // player entered a zone
	EventZoneExit                   // player left a zone
)

// Event is emitted by Session.Update.
type Event struct {
	Kind EventKind
	ID   string
	Type string
}

// State is the overall game state of a session.
type State int

const (
	Playing State = iota
	Won
	Lost
)

func (s State) String() string {
	switch s {
	case Won:
		return "won"
	case Lost:
		return "lost"
	default:
		return "playing"
	}
}

// Session runs one play-through of a Level: item pickups, zones, triggers
// and script events. Rendering, enemies and input stay with the host.
//
// Script events emitted (Lua registerEvent / JS Engine.on):
//
//	onItemPickup(id, type)   onZoneEnter(id, type)   onZoneExit(id, type)
//	onUpdate(dt)             onWin / onLose (via winGame/loseGame)
type Session struct {
	Level    *Level
	Script   *scripting.Engine
	Triggers *gameplay.TriggerRegistry
	Exec     *gameplay.TriggerExecutor // host may set extra hooks (teleport, spawn...)

	// CallLua runs a global Lua function for the start_lua trigger action.
	CallLua func(name string)

	State   State
	Elapsed float64 // seconds of play (stops when the game ends)

	picked map[string]bool
	inside map[string]bool
}

// NewSession wires a level to a scripting engine. It installs the engine's
// OnDoor/OnWin/OnLose hooks; the host should not overwrite them.
func NewSession(lv *Level, eng *scripting.Engine) *Session {
	s := &Session{
		Level:    lv,
		Script:   eng,
		Triggers: gameplay.NewTriggerRegistry(),
		picked:   make(map[string]bool),
		inside:   make(map[string]bool),
	}
	eng.OnDoor = func(id string, open bool) {
		if !lv.SetDoor(id, open) {
			log.Printf("[Level] unknown door %q (known: %v)", id, lv.DoorIDs())
		}
	}
	eng.OnWin = func() { s.end(Won) }
	eng.OnLose = func() { s.end(Lost) }

	s.Exec = &gameplay.TriggerExecutor{
		OnUnlockDoor:  eng.UnlockDoor,
		OnLockDoor:    eng.LockDoor,
		OnPlaySound:   eng.PlaySound,
		OnPlayMusic:   eng.PlayMusic,
		OnStopMusic:   eng.StopMusic,
		OnChangeMusic: eng.ChangeMusic,
		OnShowMessage: func(text string) { eng.ShowMessageFor(text, 4) },
		OnHideMessage: eng.HideMessage,
		OnSetVar:      eng.SetVar,
		OnWinGame:     func() { eng.WinGame("") },
		OnLoseGame:    func() { eng.LoseGame("") },
		OnStartLua: func(name string) {
			if s.CallLua != nil {
				s.CallLua(name)
			}
		},
	}
	s.Triggers.SetExecutor(s.Exec)

	// Expose level facts to scripts: level_name, items_<type> counts and
	// every MAP property VAR_<NAME> as variable <name> (number if numeric).
	eng.SetVar("level_name", lv.Name)
	counts := map[string]float64{}
	for _, p := range lv.ByKind("item") {
		counts[p.Type]++
	}
	for typ, n := range counts {
		eng.SetVar("items_"+typ, n)
	}
	for k, v := range lv.Props {
		if name, ok := strings.CutPrefix(k, "VAR_"); ok && name != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				eng.SetVar(strings.ToLower(name), f)
			} else {
				eng.SetVar(strings.ToLower(name), v)
			}
		}
	}
	for _, t := range lv.Triggers {
		s.Triggers.AddTrigger(t)
	}
	return s
}

func (s *Session) end(st State) {
	if s.State == Playing {
		s.State = st
		log.Printf("[Level] game over: %s after %.1fs", st, s.Elapsed)
	}
}

// ItemActive reports whether an item has not been collected yet.
func (s *Session) ItemActive(id string) bool { return !s.picked[id] }

// ItemsLeft returns how many items are still on the map.
func (s *Session) ItemsLeft() int {
	n := 0
	for _, p := range s.Level.ByKind("item") {
		if !s.picked[p.ID] {
			n++
		}
	}
	return n
}

// Update advances the session by dt with the player at (px, py). It returns
// the pickup/zone events of this step (already forwarded to scripts and
// triggers) so the host can update visuals.
func (s *Session) Update(dt, px, py float64) []Event {
	if s.State != Playing {
		return nil
	}
	s.Elapsed += dt
	var events []Event

	for _, p := range s.Level.Placements {
		switch p.Kind {
		case "item":
			if s.picked[p.ID] || !within(p, px, py, 0.5) {
				continue
			}
			s.picked[p.ID] = true
			s.Triggers.MarkItemPicked(p.ID)
			events = append(events, Event{EventPickup, p.ID, p.Type})
			if snd := p.Params["sound"]; snd != "" {
				s.Script.PlaySound(snd)
			}
			s.Script.Emit("onItemPickup", p.ID, p.Type)
		case "zone":
			in := within(p, px, py, 0.5)
			if in == s.inside[p.ID] {
				continue
			}
			s.inside[p.ID] = in
			s.Triggers.SetZoneInside(p.ID, in)
			if in {
				events = append(events, Event{EventZoneEnter, p.ID, p.Type})
				s.Script.Emit("onZoneEnter", p.ID, p.Type)
			} else {
				events = append(events, Event{EventZoneExit, p.ID, p.Type})
				s.Script.Emit("onZoneExit", p.ID, p.Type)
			}
		}
		if s.State != Playing {
			return events
		}
	}

	s.Script.Emit("onUpdate", dt)
	s.Script.TickMessage(dt)
	if s.State != Playing {
		return events
	}

	for k, v := range s.Script.VarsSnapshot() {
		s.Triggers.SetVar(k, v)
	}
	alive := s.Script.GetEnemyCountInZone("")
	s.Triggers.UpdateContext(s.Script.GetPlayerHP(), px, py, alive, alive == 0)
	s.Triggers.Tick()
	return events
}

// within reports whether (px, py) is inside the placement's radius
// (param radius=, default def).
func within(p Placement, px, py, def float64) bool {
	r := p.Float("radius", def)
	return math.Hypot(p.X-px, p.Y-py) <= r
}
