package game

import "aiengine/level"

// EventKind identifies a game event.
type EventKind int

const (
	EventDamage    EventKind = iota // something took damage (Target, Source, Type, Amount)
	EventDeath                      // an actor died (Target, Type, Source)
	EventLevel                      // a level session event (Level: pickups, movers, switches)
	EventSound                      // a sound at a place (Name, X, Y, Z)
	EventShot                       // a weapon fired (Source, Name = weapon, X/Y/Z = muzzle)
	EventImpact                     // a hitscan ray or projectile stopped (X/Y/Z, Name = weapon)
	EventExplosion                  // splash damage went off (X/Y/Z, Amount = radius, Name = weapon)
)

// Event is one thing that happened during a frame. Events are queued while
// the world updates and dispatched at the end of World.Update: damage and
// deaths to the scripts (onEnemyDamage(id, type, dmg), onEnemyDead(id,
// type)); the host then reads Events for sounds and visuals and calls
// ClearEvents.
type Event struct {
	Kind           EventKind
	Target, Source string // actor ids ("player" for the player)
	Type           string // target actor type
	Name           string // sound / weapon name
	Amount         float64
	X, Y, Z        float64
	Level          level.Event
}

func (w *World) push(ev Event) { w.events = append(w.events, ev) }

// Events returns the events queued since the last ClearEvents.
func (w *World) Events() []Event { return w.events }

// ClearEvents empties the queue (the host calls it after handling them).
func (w *World) ClearEvents() {
	w.events = w.events[:0]
	w.dispatched = 0
}

// dispatch hands new damage and death events to the scripts.
func (w *World) dispatch() {
	for ; w.dispatched < len(w.events); w.dispatched++ {
		ev := w.events[w.dispatched]
		if ev.Target == PlayerID || w.Script == nil {
			continue // player damage already went through SetPlayerHP
		}
		switch ev.Kind {
		case EventDamage:
			w.Script.Emit("onEnemyDamage", ev.Target, ev.Type, ev.Amount)
		case EventDeath:
			w.Script.Emit("onEnemyDead", ev.Target, ev.Type)
		}
	}
}
