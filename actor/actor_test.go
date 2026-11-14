package actor

import "testing"

func TestSetAndDamage(t *testing.T) {
	s := NewSet()
	a := &Actor{ID: "a", Faction: Hostile, HP: 30, MaxHP: 30, Alive: true}
	s.Add(a)
	s.Add(&Actor{ID: "b", Faction: Friendly, HP: 10, Alive: true})
	if s.Get("a") != a || s.Len() != 2 || s.CountAlive(Hostile) != 1 {
		t.Fatal("registry")
	}
	if id := s.NewID("a"); id == "a" || s.Get(id) != nil {
		t.Fatalf("NewID %q", id)
	}
	if d, died := a.TakeDamage(20); d != 20 || died {
		t.Fatal("first hit")
	}
	if d, died := a.TakeDamage(50); d != 10 || !died || a.Alive || a.HP != 0 {
		t.Fatalf("killing hit dealt %v", d)
	}
	if d, _ := a.TakeDamage(5); d != 0 {
		t.Fatal("dead actors take no damage")
	}
	if s.CountAlive(Hostile) != 0 {
		t.Fatal("count after death")
	}
}
