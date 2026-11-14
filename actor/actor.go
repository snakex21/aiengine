// Package actor is the single registry of things that live in a level:
// enemies, later vehicles and other bodies. It replaces the separate
// enemy lists of the host (DemoEnemy), the scripting engine (Entities) and
// the old gameplay entities. Scripts, the game simulation, movers and
// projectiles all read and write the same Actor values.
//
// The package is pure data (no rendering, no scripting); game-specific
// state hangs off Actor.Ctl.
package actor

import (
	"fmt"
	"sort"
)

// Faction says who an actor fights for.
type Faction int

const (
	Neutral Faction = iota
	Friendly
	Hostile
)

// Actor is one body in the level.
type Actor struct {
	ID      string
	Type    string // enemy type / template name
	Faction Faction

	X, Y   float64 // position on the map plane
	Lift   float64 // height of the body's bottom above its floor (flyers, sprite z=)
	Radius float64 // collision radius
	Height float64 // body height (headroom under doors and crushers, hit box)

	HP, MaxHP float64
	Alive     bool
	Frozen    bool // scripts paused its AI

	// Knockback velocity (map units per second), decays over time.
	PushX, PushY float64

	// Ctl is the owner's per-actor state (the game's brain, sprite and
	// weapon of an enemy).
	Ctl any
}

// TakeDamage lowers HP by amount (not below 0). It returns the HP actually
// removed and whether this damage killed the actor. Dead actors take none.
func (a *Actor) TakeDamage(amount float64) (dealt float64, died bool) {
	if !a.Alive || amount <= 0 {
		return 0, false
	}
	dealt = min(amount, a.HP)
	a.HP -= dealt
	if a.HP <= 0 {
		a.HP, a.Alive = 0, false
		return dealt, true
	}
	return dealt, false
}

// Set is the registry: actors in insertion order plus lookup by id.
type Set struct {
	list []*Actor
	byID map[string]*Actor
	seq  int
}

// NewSet creates an empty registry.
func NewSet() *Set { return &Set{byID: map[string]*Actor{}} }

// Add registers an actor (an actor with an id already in the set replaces it).
func (s *Set) Add(a *Actor) {
	if old := s.byID[a.ID]; old != nil {
		for i, o := range s.list {
			if o == old {
				s.list[i] = a
			}
		}
	} else {
		s.list = append(s.list, a)
	}
	s.byID[a.ID] = a
}

// Get returns the actor with the given id, or nil.
func (s *Set) Get(id string) *Actor { return s.byID[id] }

// All returns every actor, dead ones included, in insertion order. The
// slice belongs to the set.
func (s *Set) All() []*Actor { return s.list }

// Len is the number of actors (alive or not).
func (s *Set) Len() int { return len(s.list) }

// CountAlive counts living actors of a faction.
func (s *Set) CountAlive(f Faction) int {
	n := 0
	for _, a := range s.list {
		if a.Alive && a.Faction == f {
			n++
		}
	}
	return n
}

// NewID returns an unused id "<prefix>_<n>".
func (s *Set) NewID(prefix string) string {
	for {
		s.seq++
		id := fmt.Sprintf("%s_%d", prefix, s.seq)
		if s.byID[id] == nil {
			return id
		}
	}
}

// IDs returns the ids of all actors, sorted.
func (s *Set) IDs() []string {
	out := make([]string, 0, len(s.list))
	for _, a := range s.list {
		out = append(out, a.ID)
	}
	sort.Strings(out)
	return out
}

// Clear removes every actor.
func (s *Set) Clear() {
	s.list = s.list[:0]
	clear(s.byID)
}
