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
	EventPickup     EventKind = iota // player collected an item
	EventZoneEnter                   // player entered a zone
	EventZoneExit                    // player left a zone
	EventDoorOpened                  // interact opened a door
	EventDoorLocked                  // interact hit a locked/script door
	EventMover                       // a mover started/stopped/crushed (Mover set)
	EventSwitch                      // interact flipped a switch (ID = switch id)
	EventMoverUsed                   // interact activated a mover (ID = tag)
)

// Event is emitted by Session.Update.
type Event struct {
	Kind  EventKind
	ID    string
	Type  string
	Mover *MoverEvent // EventMover
}

// PlayerBody is the id of the player in mover bodies.
const PlayerBody = "player"

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

	// Bodies, when set, lists the things movers must respect (player,
	// enemies, ...); without it only the player is considered.
	Bodies func(px, py float64, buf []Body) []Body
	// HostDamage leaves crush damage to the host (EventMover events);
	// otherwise the session takes it from the player's HP itself.
	HostDamage bool

	picked   map[string]bool
	inside   map[string]bool
	switched map[string]bool
	lastTag  string
	bodyBuf  []Body
	mevBuf   []MoverEvent
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
	s.switched = make(map[string]bool)
	eng.OnDoor = func(id string, open bool) {
		if !lv.SetDoor(id, open) {
			log.Printf("[Level] unknown door %q (known: %v)", id, lv.DoorIDs())
		}
	}
	eng.OnActivate = lv.Activate
	eng.OnMove = func(tag, plane string, h, speed float64) bool {
		return lv.MoveTo(tag, plane == "ceil", h, speed)
	}
	eng.OnStopMover = lv.StopMovers
	eng.OnMoverHeight = func(tag, plane string) (float64, bool) { return lv.MoverHeight(tag, plane == "ceil") }
	eng.OnLight = func(id string, level float64, effect string) bool {
		ok := lv.LightByID(id) != nil
		if ok && level >= 0 {
			lv.SetLightLevel(id, level)
		}
		if ok && effect != "" {
			ok = lv.SetLightEffect(id, effect)
		}
		if !ok {
			log.Printf("[Level] setLight: unknown light %q or effect %q", id, effect)
		}
		lv.UpdateLights(s.Elapsed)
		return ok
	}
	eng.OnCellLight = func(x, y int, level float64) bool {
		ok := lv.SetCellLight(x, y, level)
		lv.UpdateLights(s.Elapsed)
		return ok
	}
	eng.OnWin = func() { s.end(Won) }
	eng.ExitTarget = ""
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
		OnActivate: func(tag string) { lv.Activate(tag) },
		OnMove: func(tag string, ceil bool, h, speed float64) {
			lv.MoveTo(tag, ceil, h, speed)
		},
	}
	s.Triggers.SetExecutor(s.Exec)

	// Expose level facts to scripts: level_name, items_<type> counts and
	// every MAP property VAR_<NAME> as variable <name> (number if numeric).
	eng.SetVar("level_name", lv.Name)
	secrets := 0.0
	for _, d := range lv.Doors() {
		if d.Secret {
			secrets++
		}
	}
	eng.SetVar("secrets_total", secrets)
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
	s.Exec.OnExitTo = func(target string) { eng.ExitLevel(target, "") }
	// Enemy deaths feed enemy_dead / all_enemies_dead conditions.
	eng.On("onEnemyDead", func(args ...interface{}) {
		if len(args) > 0 {
			if id, ok := args[0].(string); ok {
				s.Triggers.MarkEnemyDead(id)
			}
		}
	})
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
	s.Level.UpdateLights(s.Elapsed)
	var events []Event

	for _, p := range s.Level.Placements {
		switch p.Kind {
		case "item":
			if s.picked[p.ID] || !within(p, px, py, 0.5) {
				continue
			}
			s.picked[p.ID] = true
			s.Triggers.MarkItemPicked(p.ID)
			if give := p.Params["give"]; give != "" {
				s.Script.GiveItem(give, 1)
			}
			events = append(events, Event{Kind: EventPickup, ID: p.ID, Type: p.Type})
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
				events = append(events, Event{Kind: EventZoneEnter, ID: p.ID, Type: p.Type})
				s.Script.Emit("onZoneEnter", p.ID, p.Type)
				if to := p.Params["to"]; to != "" && s.State == Playing {
					s.Script.ExitLevel(to, "") // exit zone straight to a level
				}
				if tag := p.Params["activate"]; tag != "" {
					s.Level.Activate(tag)
				}
			} else {
				events = append(events, Event{Kind: EventZoneExit, ID: p.ID, Type: p.Type})
				s.Script.Emit("onZoneExit", p.ID, p.Type)
			}
		}
		if s.State != Playing {
			return events
		}
	}
	// Walk triggers: entering a tagged area activates movers with walk=<tag>.
	if tag := s.Level.TagAt(px, py); tag != s.lastTag {
		s.lastTag = tag
		if tag != "" {
			for _, m := range s.Level.movers {
				if m.Walk == tag {
					m.activate()
				}
			}
			s.Script.Emit("onAreaEnter", tag)
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
	return s.updateMovers(dt, px, py, events)
}

// updateMovers steps the level's movers and reports their events (also to
// scripts: onMoverStart(tag), onMoverStop(tag), onCrush(body, damage)).
func (s *Session) updateMovers(dt, px, py float64, events []Event) []Event {
	if len(s.Level.movers) == 0 {
		return events
	}
	if s.Bodies != nil {
		s.bodyBuf = s.Bodies(px, py, s.bodyBuf[:0])
	} else {
		s.bodyBuf = append(s.bodyBuf[:0], Body{ID: PlayerBody, X: px, Y: py, Radius: 0.15, Height: 0.6})
	}
	s.mevBuf = s.Level.UpdateMovers(dt, s.bodyBuf, s.mevBuf[:0])
	for i := range s.mevBuf {
		ev := s.mevBuf[i]
		switch ev.Kind {
		case MoverStart:
			s.Script.Emit("onMoverStart", ev.Tag)
		case MoverStop:
			s.Script.Emit("onMoverStop", ev.Tag)
		case MoverCrush:
			if ev.Body == PlayerBody && !s.HostDamage {
				s.Script.SetPlayerHP(s.Script.GetPlayerHP() - ev.Damage)
			}
			s.Script.Emit("onCrush", ev.Body, ev.Damage)
		}
		events = append(events, Event{Kind: EventMover, ID: ev.Tag, Mover: &ev})
	}
	return events
}

// Interact is the player's "use" action at (px, py) facing angle: it opens
// the first closed door within reach in front of the player when allowed
// (plain doors, secret walls, or key doors when the key is in the
// inventory - the key is consumed). Locked and script-only doors emit
// onDoorLocked(id, key). Opening emits onDoorOpen(id) and, for secrets,
// onSecretFound(id).
func (s *Session) Interact(px, py, angle float64) []Event {
	if s.State != Playing {
		return nil
	}
	if ev, ok := s.useSwitch(px, py, angle); ok {
		return ev
	}
	// A lift or platform the player stands on.
	if ev, ok := s.useMover(px, py, false); ok {
		return ev
	}
	for _, reach := range []float64{0.5, 0.8, 1.1, 1.4} {
		fx, fy := px+math.Cos(angle)*reach, py+math.Sin(angle)*reach
		x, y := int(math.Floor(fx)), int(math.Floor(fy))
		d, ok := s.Level.DoorAt(x, y)
		if !ok {
			if ev, ok := s.useMover(fx, fy, true); ok {
				return ev
			}
			if s.Level.blocksUse(fx, fy) {
				return nil // a plain wall is in the way
			}
			continue
		}
		if d.Open {
			continue
		}
		switch {
		case d.Key != "":
			if s.Script.ItemCount(d.Key) <= 0 {
				s.Script.Emit("onDoorLocked", d.ID, d.Key)
				return []Event{{Kind: EventDoorLocked, ID: d.ID, Type: d.Key}}
			}
			s.Script.TakeItem(d.Key, 1)
		case d.Secret, d.Use:
		default:
			s.Script.Emit("onDoorLocked", d.ID, "")
			return []Event{{Kind: EventDoorLocked, ID: d.ID}}
		}
		s.Script.UnlockDoor(d.ID)
		if d.Secret {
			s.Script.Emit("onSecretFound", d.ID)
		}
		return []Event{{Kind: EventDoorOpened, ID: d.ID, Type: d.Key}}
	}
	return nil
}

// useMover activates the usable movers of the area at (x, y) (doors only
// when doors is set). Keys are consumed on first use; locked movers and
// missing keys emit onDoorLocked(tag, key).
func (s *Session) useMover(x, y float64, doors bool) ([]Event, bool) {
	tag := s.Level.TagAt(x, y)
	if tag == "" {
		return nil, false
	}
	var used *Mover
	for _, m := range s.Level.moverTags[tag] {
		if m.Use && (doors || m.Kind != "door") {
			used = m
			break
		}
	}
	if used == nil {
		return nil, false
	}
	if used.Locked {
		s.Script.Emit("onDoorLocked", tag, "")
		return []Event{{Kind: EventDoorLocked, ID: tag}}, true
	}
	if used.Key != "" && !used.keyUsed {
		if s.Script.ItemCount(used.Key) <= 0 {
			s.Script.Emit("onDoorLocked", tag, used.Key)
			return []Event{{Kind: EventDoorLocked, ID: tag, Type: used.Key}}, true
		}
		s.Script.TakeItem(used.Key, 1)
		used.keyUsed = true
	}
	opening := used.Kind == "door" && !used.IsOpen()
	if !used.activate() {
		return nil, true
	}
	if opening {
		s.Script.Emit("onDoorOpen", tag)
		if used.Secret {
			s.Script.Emit("onSecretFound", tag)
		}
	}
	return []Event{{Kind: EventMoverUsed, ID: tag, Type: used.Kind}}, true
}

// switchReach and switchCone limit which switch the player can use.
const (
	switchReach = 1.3
	switchCone  = 0.6 // radians either side of the view direction
)

// useSwitch flips the nearest PLACE switch in front of the player:
// activates the movers of its tag= (action=activate, open, close, stop),
// emits onSwitch(id, tag). once=yes switches work one time.
func (s *Session) useSwitch(px, py, angle float64) ([]Event, bool) {
	best, bestD := -1, math.Inf(1)
	for i, p := range s.Level.Placements {
		if p.Kind != "switch" {
			continue
		}
		d := math.Hypot(p.X-px, p.Y-py)
		if d > switchReach || d >= bestD {
			continue
		}
		a := math.Atan2(p.Y-py, p.X-px) - angle
		a = math.Atan2(math.Sin(a), math.Cos(a))
		if math.Abs(a) > switchCone && d > 0.35 {
			continue
		}
		best, bestD = i, d
	}
	if best < 0 {
		return nil, false
	}
	p := s.Level.Placements[best]
	if isYes(p.Params["once"]) && s.switched[p.ID] {
		return nil, true
	}
	if key := p.Params["key"]; key != "" && !s.switched[p.ID] {
		if s.Script.ItemCount(key) <= 0 {
			s.Script.Emit("onDoorLocked", p.ID, key)
			return []Event{{Kind: EventDoorLocked, ID: p.ID, Type: key}}, true
		}
		s.Script.TakeItem(key, 1)
	}
	s.switched[p.ID] = !s.switched[p.ID] || isYes(p.Params["once"])
	for _, tag := range strings.Split(p.Params["tag"], ",") {
		if tag == "" {
			continue
		}
		switch strings.ToLower(p.Params["action"]) {
		case "open":
			s.Level.SetDoor(tag, true)
		case "close":
			s.Level.SetDoor(tag, false)
		case "stop":
			s.Level.StopMovers(tag)
		default:
			s.Level.Activate(tag)
		}
	}
	if snd := p.Params["sound"]; snd != "" {
		s.Script.PlaySound(snd)
	}
	s.Script.Emit("onSwitch", p.ID, p.Params["tag"])
	return []Event{{Kind: EventSwitch, ID: p.ID, Type: p.Params["tag"]}}, true
}

// SwitchOn reports whether a switch is in its flipped state.
func (s *Session) SwitchOn(id string) bool { return s.switched[id] }

// within reports whether (px, py) is inside the placement's radius
// (param radius=, default def).
func within(p Placement, px, py, def float64) bool {
	r := p.Float("radius", def)
	return math.Hypot(p.X-px, p.Y-py) <= r
}
