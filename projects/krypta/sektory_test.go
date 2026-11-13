package krypta

import (
	"math"
	"testing"

	"aiengine/level"
	"aiengine/renderer"
)

// walkSectors moves a player across a sector level like the host does
// (per-axis collision against the sector lines, then floor follow).
func walkSectors(lv *level.Level, p *renderer.PlayerState, dx, dy float64, frames int) {
	const dt = 1.0 / 60
	m := lv.Sectors
	for i := 0; i < frames; i++ {
		nx := p.X + dx*3*dt
		if m.CanOccupy(nx, p.Y, 0.15, p.Feet(), renderer.MaxStepUp, p.Headroom()) {
			p.X = nx
		}
		ny := p.Y + dy*3*dt
		if m.CanOccupy(p.X, ny, 0.15, p.Feet(), renderer.MaxStepUp, p.Headroom()) {
			p.Y = ny
		}
		f, _ := m.Support(p.X, p.Y, 0.15)
		p.SetGround(f)
		p.UpdateVertical(dt)
	}
}

func TestSanktuariumIsASectorMap(t *testing.T) {
	lv, err := level.LoadFile("maps/sanktuarium.aie")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Sectors == nil || len(lv.Sectors.Sectors) < 15 {
		t.Fatal("sanktuarium must load as a sector map")
	}
	angled := 0
	for _, l := range lv.Sectors.Lines {
		if !l.TwoSided() && math.Abs(l.A.X-l.B.X) > 1e-9 && math.Abs(l.A.Y-l.B.Y) > 1e-9 {
			angled++
		}
	}
	if angled < 8 {
		t.Errorf("showcase needs angled walls, found %d", angled)
	}
}

func TestSanktuariumStairsBalconyAndPool(t *testing.T) {
	lv, err := level.LoadFile("maps/sanktuarium.aie")
	if err != nil {
		t.Fatal(err)
	}
	// East: walk from the hall up the stairs onto the balcony (floor 1).
	p := renderer.NewPlayerState(15.5, 12, 0)
	p.PlaceAt(0)
	walkSectors(lv, p, 1, 0, 200)
	if p.Ground != 1 || p.X < 21 {
		t.Fatalf("stairs: ended at x=%.2f on floor %.2f, want the balcony (1.0)", p.X, p.Ground)
	}
	// West: walk down the ramp into the sunken pool (floor -0.75).
	p = renderer.NewPlayerState(8.5, 12, math.Pi)
	p.PlaceAt(0)
	walkSectors(lv, p, -1, 0, 200)
	if p.Ground != -0.75 || p.X > 4.5 {
		t.Fatalf("pool: ended at x=%.2f on floor %.2f, want the pool (-0.75)", p.X, p.Ground)
	}
	// The angled pool walls stop the player.
	if p.X < 1.1 {
		t.Fatalf("walked through the pool's west wall (x=%.2f)", p.X)
	}
	// North: the niche gate is closed (a door sector, opened by the
	// crystals trigger); open it, then walk into the portal niche (0.25)
	// up to its angled back walls.
	p = renderer.NewPlayerState(12, 9, -math.Pi/2)
	p.PlaceAt(0)
	walkSectors(lv, p, 0, -1, 200)
	if p.Y < 7 {
		t.Fatalf("walked through the closed niche gate (y=%.2f)", p.Y)
	}
	lv.SetDoor("wrota", true)
	for i := 0; i < 60; i++ {
		lv.UpdateMovers(0.05, nil, nil)
	}
	p = renderer.NewPlayerState(12, 9, -math.Pi/2)
	p.PlaceAt(0)
	walkSectors(lv, p, 0, -1, 200)
	if p.Ground != 0.25 || p.Y > 4 || p.Y < 3.1 {
		t.Fatalf("niche: ended at y=%.2f on floor %.2f", p.Y, p.Ground)
	}
}
