package level

import (
	"math"
	"strings"
	"testing"

	"aiengine/nav"
	"aiengine/renderer"
	"aiengine/scripting"
)

func moverSession(t *testing.T, src string) (*Level, *scripting.Engine, *Session) {
	t.Helper()
	lv, err := FromDocumentString(src)
	if err != nil {
		t.Fatal(err)
	}
	eng := scripting.NewEngine()
	return lv, eng, NewSession(lv, eng)
}

// runFor steps the session for secs with the player at (px, py) and returns
// the mover events.
func runFor(s *Session, secs, px, py float64) []MoverEvent {
	var out []MoverEvent
	for t := 0.0; t < secs; t += 0.05 {
		for _, e := range s.Update(0.05, px, py) {
			if e.Kind == EventMover {
				out = append(out, *e.Mover)
			}
		}
	}
	return out
}

func hasEvent(evs []MoverEvent, k MoverEventKind) bool {
	for _, e := range evs {
		if e.Kind == k {
			return true
		}
	}
	return false
}

const doorMap = `MAP drzwi
TAGS d=brama k=skarbiec
GRID
W W W W W W W W W
W P . . . . . . W
W W W W W W W . W
W . . . . . . . W
W W W W W W W W W
TAG_GRID
. . . . . . . . .
. . . d . . . . .
. . . . . . . . .
. . . . . k . . .
. . . . . . . . .
MOVER door brama speed=2 wait=1
MOVER door skarbiec key=klucz wait=0
`

func TestDoorOpensWaitsAndCloses(t *testing.T) {
	lv, eng, s := moverSession(t, doorMap)
	var opened []string
	eng.On("onDoorOpen", func(a ...interface{}) { opened = append(opened, a[0].(string)) })
	if lv.CeilAt(3, 1) != lv.FloorAt(3, 1) || lv.Solid(3, 1) {
		t.Fatalf("door must start closed as an open cell with ceiling = floor (ceil %.2f)", lv.CeilAt(3, 1))
	}
	if nav.FindPath(lv, nav.Cell{X: 1, Y: 1}, nav.Cell{X: 5, Y: 1}, 0) != nil {
		t.Fatal("enemies must not path through a closed door")
	}
	ev := s.Interact(2.5, 1.5, 0)
	if len(ev) != 1 || ev[0].Kind != EventMoverUsed || ev[0].ID != "brama" || len(opened) != 1 {
		t.Fatalf("use: %+v opened %v", ev, opened)
	}
	evs := runFor(s, 0.6, 1.5, 1.5)
	if !hasEvent(evs, MoverStart) || !hasEvent(evs, MoverStop) || lv.CeilAt(3, 1) != 1 {
		t.Fatalf("door did not open: ceil %.2f events %+v", lv.CeilAt(3, 1), evs)
	}
	if nav.FindPath(lv, nav.Cell{X: 1, Y: 1}, nav.Cell{X: 5, Y: 1}, 0) == nil {
		t.Fatal("open door must be walkable for enemies")
	}
	// wait=1: it closes again by itself.
	runFor(s, 2, 1.5, 1.5)
	if lv.CeilAt(3, 1) != 0 {
		t.Fatalf("door did not close after wait (ceil %.2f)", lv.CeilAt(3, 1))
	}
}

func TestDoorReversesWhenBlocked(t *testing.T) {
	lv, _, s := moverSession(t, doorMap)
	s.Interact(2.5, 1.5, 0)
	runFor(s, 0.6, 3.5, 1.5) // open, player now stands in the doorway
	evs := runFor(s, 3, 3.5, 1.5)
	if !hasEvent(evs, MoverReverse) {
		t.Fatalf("closing door did not reverse on the player: %+v", evs)
	}
	if lv.CeilAt(3, 1) < renderer.HeadroomStand-1e-9 {
		t.Fatalf("door squeezed the player: ceil %.2f", lv.CeilAt(3, 1))
	}
	// Once the player leaves, it closes.
	runFor(s, 3, 1.5, 1.5)
	if lv.CeilAt(3, 1) != 0 {
		t.Fatalf("door stuck open (ceil %.2f)", lv.CeilAt(3, 1))
	}
}

func TestKeyDoorMover(t *testing.T) {
	lv, eng, s := moverSession(t, doorMap)
	var locked []string
	eng.On("onDoorLocked", func(a ...interface{}) { locked = append(locked, a[0].(string)+":"+a[1].(string)) })
	ev := s.Interact(4.5, 3.5, 0)
	if len(ev) != 1 || ev[0].Kind != EventDoorLocked || len(locked) != 1 || locked[0] != "skarbiec:klucz" {
		t.Fatalf("locked: %+v %v", ev, locked)
	}
	eng.GiveItem("klucz", 1)
	ev = s.Interact(4.5, 3.5, 0)
	if len(ev) != 1 || ev[0].Kind != EventMoverUsed || eng.ItemCount("klucz") != 0 {
		t.Fatalf("key door: %+v keys %d", ev, eng.ItemCount("klucz"))
	}
	runFor(s, 1, 4.5, 3.5)
	if lv.CeilAt(5, 3) != 1 {
		t.Fatal("key door did not open")
	}
	// wait=0: stays open; use again closes it (no key needed any more).
	runFor(s, 5, 4.5, 3.5)
	if lv.CeilAt(5, 3) != 1 {
		t.Fatal("wait=0 door must stay open")
	}
	s.Interact(4.5, 3.5, 0)
	runFor(s, 1, 4.5, 3.5)
	if lv.CeilAt(5, 3) != 0 || eng.ItemCount("klucz") != 0 {
		t.Fatal("using an open door must close it")
	}
}

const liftMap = `MAP winda
CEILING_HEIGHT 2
FLOOR_GRID
. . . . . .
. . . 8 8 .
. . . . . .
TAG_GRID
. . . . . .
. . . w . .
. . . . . .
GRID
W W W W W W
W P . . . W
W W W W W W
MOVER lift w wait=0.5 speed=1
`

func TestLiftCarriesThePlayer(t *testing.T) {
	lv, _, s := moverSession(t, liftMap)
	if lv.FloorAt(3, 1) != 1 {
		t.Fatalf("lift must rest up at 1.0, got %.2f", lv.FloorAt(3, 1))
	}
	// From below the ledge, use the lift: it comes down.
	if ev := s.Interact(2.5, 1.5, 0); len(ev) != 1 || ev[0].Kind != EventMoverUsed {
		t.Fatalf("use lift: %+v", ev)
	}
	runFor(s, 1.2, 2.5, 1.5)
	if lv.FloorAt(3, 1) != 0 {
		t.Fatalf("lift did not come down: %.2f", lv.FloorAt(3, 1))
	}
	// Step on, ride up: the player stays on the floor without falling.
	p := renderer.NewPlayerState(3.5, 1.5, 0)
	f, _ := lv.Support(p.X, p.Y, 0.15)
	p.PlaceAt(f)
	carried := false
	for i := 0; i < 80; i++ {
		before, _ := lv.Support(p.X, p.Y, 0.15)
		s.Update(0.05, p.X, p.Y)
		after, _ := lv.Support(p.X, p.Y, 0.15)
		carried = p.Ride(before, after) || carried
		p.SetGround(after)
		p.UpdateVertical(0.05)
	}
	if !carried || p.Ground != 1 || p.Feet() != 1 || !p.Grounded || p.LastFall != 0 {
		t.Fatalf("ride: carried %v ground %.2f feet %.2f grounded %v fall %.2f", carried, p.Ground, p.Feet(), p.Grounded, p.LastFall)
	}
	// Up there the ledge is level with the lift.
	if !lv.Map.CanOccupy(4.5, 1.5, 0.15, p.Feet(), renderer.MaxStepUp, renderer.HeadroomStand) {
		t.Fatal("ledge not reachable from the lift")
	}
	// Using the lift while standing on it takes it down again.
	if ev := s.Interact(p.X, p.Y, math.Pi); len(ev) != 1 || ev[0].Kind != EventMoverUsed {
		t.Fatalf("use from the lift: %+v", ev)
	}
}

const crusherMap = `MAP pulapka
CEILING_HEIGHT 1.5
TAG_GRID
. . . . .
. . c . .
. . . . .
GRID
W W W W W
W P . . W
W W W W W
MOVER crusher c speed=2 damage=15 walk=yes
`

func TestCrusherDamagesBodies(t *testing.T) {
	lv, eng, s := moverSession(t, crusherMap)
	var crushed []string
	eng.On("onCrush", func(a ...interface{}) { crushed = append(crushed, a[0].(string)) })
	s.Bodies = func(px, py float64, buf []Body) []Body {
		buf = append(buf, Body{ID: PlayerBody, X: px, Y: py, Radius: 0.15, Height: renderer.HeadroomStand})
		return append(buf, Body{ID: "pajak", X: 2.5, Y: 1.4, Radius: 0.15, Height: 0.3})
	}
	runFor(s, 0.2, 1.5, 1.5)
	if lv.CeilAt(2, 1) != 1.5 {
		t.Fatal("crusher must wait at its top until triggered")
	}
	// Walking into the crusher's cell starts it (walk=yes).
	evs := runFor(s, 2, 2.5, 1.5)
	dmg := map[string]float64{}
	for _, e := range evs {
		if e.Kind == MoverCrush {
			dmg[e.Body] += e.Damage
		}
	}
	if dmg[PlayerBody] < 15 || len(crushed) == 0 {
		t.Fatalf("crush damage %v events %v", dmg, crushed)
	}
	if c := lv.CeilAt(2, 1); c < renderer.HeadroomStand-1e-9 || c > renderer.HeadroomStand+0.1 {
		t.Fatalf("crusher must press on the player's head, ceil %.2f", c)
	}
	// The player escapes; the smaller spider is pressed further down.
	s.Bodies = func(px, py float64, buf []Body) []Body {
		return append(buf, Body{ID: "pajak", X: 2.5, Y: 1.4, Radius: 0.15, Height: 0.3})
	}
	evs = runFor(s, 1, 1.2, 1.5)
	for _, e := range evs {
		if e.Kind == MoverCrush && e.Body == "pajak" {
			dmg["pajak"] += e.Damage
		}
	}
	if dmg["pajak"] < 15 || math.Abs(lv.CeilAt(2, 1)-0.3) > 0.1 {
		t.Fatalf("spider: damage %v ceil %.2f", dmg, lv.CeilAt(2, 1))
	}
	// Player and spider leave: it goes all the way down, then back up, and
	// keeps cycling.
	s.Bodies = nil
	low, high := 9.0, 0.0
	for i := 0; i < 120; i++ {
		runFor(s, 0.05, 1.2, 1.5)
		low, high = math.Min(low, lv.CeilAt(2, 1)), math.Max(high, lv.CeilAt(2, 1))
	}
	if math.Abs(low-0.1) > 1e-9 || high != 1.5 {
		t.Fatalf("crusher cycle %.2f..%.2f, want 0.1..1.5", low, high)
	}
	// stopMover: it returns up and stays.
	eng.OnStopMover("c")
	runFor(s, 3, 1.2, 1.5)
	if lv.CeilAt(2, 1) != 1.5 || lv.MoversByTag("c")[0].Busy() {
		t.Fatal("stopped crusher must rest at the top")
	}
}

const sectorLiftMap = `MAP sektory
PLAYER_START 1 1
SECTOR dol floor=0 ceil=3 : 0 0 4 0 4 2 1 2 0 2
SECTOR winda floor=1 ceil=3 tag=lift : 4 0 6 0 6 2 4 2
SECTOR gora floor=1 ceil=3 : 6 0 8 0 8 2 6 2
SECTOR wnek floor=0 ceil=1 tag=drzwi : 0 2 1 2 1 3 0 3
SECTOR pokoj floor=0 ceil=1 : 0 3 1 3 2 3 2 5 0 5
MOVER lift lift wait=1
MOVER door drzwi wait=0
`

func TestSectorNavUpdatesOnlyMovedCells(t *testing.T) {
	lv, eng, s := moverSession(t, sectorLiftMap)
	before := append([]float64(nil), lv.Map.Floor...)
	beforeC := append([]float64(nil), lv.Map.Ceil...)
	if lv.FloorAt(4, 1) != 1 || nav.FindPath(lv, nav.Cell{X: 1, Y: 1}, nav.Cell{X: 7, Y: 1}, 0) != nil {
		t.Fatal("raised lift must block the way up")
	}
	if !eng.MoveFloor("lift", 0, 4) {
		t.Fatal("moveFloor on a sector tag failed")
	}
	runFor(s, 0.4, 1.5, 1.5)
	changed := map[int]bool{}
	for i := range before {
		if lv.Map.Floor[i] != before[i] || lv.Map.Ceil[i] != beforeC[i] {
			changed[i] = true
		}
	}
	w := lv.Map.Width
	if len(changed) != 4 || !changed[0*w+4] || !changed[1*w+5] {
		t.Fatalf("nav cells changed: %v, want exactly the lift's 4", changed)
	}
	if lv.FloorAt(4, 1) != 0 || lv.Sectors.Sectors[1].Floor != 0 {
		t.Fatal("lift did not come down")
	}
	// The way up is now a single step at the far side (1.0 > MaxStepUp):
	// still blocked, but the lift itself is walkable.
	if !lv.CanStep(nav.Cell{X: 3, Y: 1}, nav.Cell{X: 4, Y: 1}) {
		t.Fatal("lowered lift must be walkable")
	}
	// The closed door sector blocks the room; opening it frees the path.
	if nav.FindPath(lv, nav.Cell{X: 0, Y: 1}, nav.Cell{X: 1, Y: 4}, 0) != nil {
		t.Fatal("closed sector door must block nav")
	}
	if ev := s.Interact(0.5, 1.5, math.Pi/2); len(ev) != 1 || ev[0].Kind != EventMoverUsed {
		t.Fatalf("use sector door: %+v", ev)
	}
	runFor(s, 1, 0.5, 1.5)
	if nav.FindPath(lv, nav.Cell{X: 0, Y: 1}, nav.Cell{X: 1, Y: 4}, 0) == nil {
		t.Fatal("open sector door must let enemies through")
	}
	if !lv.Sectors.CanOccupy(0.5, 2.5, 0.15, 0, renderer.MaxStepUp, renderer.HeadroomStand) {
		t.Fatal("player cannot pass the open sector door")
	}
}

func TestSwitchesZonesAndScripts(t *testing.T) {
	src := doorMap + `
PLACE switch dzwignia s1 2.5 0.5 1.2 tag=skarbiec action=open once=yes
PLACE zone trap z1 6.5 0 3.5 radius=0.4 activate=brama
`
	lv, eng, s := moverSession(t, src)
	var flipped []string
	eng.On("onSwitch", func(a ...interface{}) { flipped = append(flipped, a[0].(string)) })
	// Switch opens the key door without the key (switches ignore keys).
	ev := s.Interact(2.5, 1.5, -math.Pi/2)
	if len(ev) != 1 || ev[0].Kind != EventSwitch || len(flipped) != 1 || !s.SwitchOn("s1") {
		t.Fatalf("switch: %+v %v", ev, flipped)
	}
	runFor(s, 1, 1.5, 1.5)
	if lv.CeilAt(5, 3) != 1 {
		t.Fatal("switch did not open the door")
	}
	if ev := s.Interact(2.5, 1.5, -math.Pi/2); len(ev) != 0 || len(flipped) != 1 {
		t.Fatal("once=yes switch flipped twice")
	}
	// A zone with activate= opens brama.
	runFor(s, 0.1, 6.5, 3.5)
	runFor(s, 0.6, 1.5, 1.5)
	if lv.CeilAt(3, 1) != 1 {
		t.Fatal("zone activate= did not open brama")
	}
	// Lua: moveCeil and getCeil.
	lb := scripting.NewLuaBridge(eng)
	defer lb.Close()
	if err := lb.DoString(`moveCeil("brama", 0.5, 10)`); err != nil {
		t.Fatal(err)
	}
	runFor(s, 0.3, 1.5, 1.5)
	if err := lb.DoString(`h = getCeil("brama")`); err != nil {
		t.Fatal(err)
	}
	if h := lb.L.GetGlobal("h").String(); h != "0.5" {
		t.Fatalf("getCeil = %s", h)
	}
	// Trigger action activate.
	s.Exec.OnActivate("skarbiec")
	runFor(s, 1, 1.5, 1.5)
	if lv.CeilAt(5, 3) != 0 {
		t.Fatal("activate on an open door must close it")
	}
}

func TestSlidingPlaceDoors(t *testing.T) {
	src := strings.Replace(doorsMap, "MAP drzwi\n", "MAP drzwi\nDOOR_STYLE slide\n", 1)
	lv, eng, s := moverSession(t, src)
	d, _ := lv.Door("czerwone")
	if !d.Anim || d.Open || lv.Solid(4, 1) || lv.Map.SideAt(4, 1) != renderer.CellDoor || lv.CeilAt(4, 1) != 0 {
		t.Fatalf("sliding door %+v solid %v", d, lv.Solid(4, 1))
	}
	if lv.Map.SideAt(3, 2) == renderer.CellDoor || lv.Map.SideAt(3, 2) == renderer.CellWall2 && lv.Map.Grid[2][3] != 0 {
		t.Fatal("secret sliding door must keep its wall look")
	}
	eng.GiveItem("klucz_czerwony", 1)
	if ev := s.Interact(3.5, 1.5, 0); len(ev) != 1 || ev[0].Kind != EventDoorOpened {
		t.Fatalf("interact: %+v", ev)
	}
	if d, _ := lv.Door("czerwone"); !d.Open {
		t.Fatal("door must report open while opening")
	}
	runFor(s, 1, 1.5, 1.5)
	if lv.CeilAt(4, 1) != 1 {
		t.Fatal("sliding door did not open")
	}
	// Script gate via unlock_door (trigger) slides too.
	eng.Emit("onEnemyDead", "p1")
	runFor(s, 1, 1.5, 1.5)
	if lv.CeilAt(7, 2) != 1 {
		t.Fatal("gate did not slide open")
	}
	// Old maps without DOOR_STYLE keep instant doors.
	lv2, _, _ := moverSession(t, doorsMap)
	if d, _ := lv2.Door("czerwone"); d.Anim || !lv2.Solid(4, 1) {
		t.Fatal("instant doors must stay the default")
	}
}

func TestMoverParseRoundTrip(t *testing.T) {
	for _, spec := range []string{
		"door brama speed=2 wait=1 key=klucz",
		"lift w low=0 high=1.5 wait=2",
		"crusher c damage=20 start=on",
		"floor plyta to=0.5 loop=yes",
	} {
		ms, err := parseMover(spec, 1)
		if err != nil {
			t.Fatal(err)
		}
		again, err := parseMover(FormatMover(ms.kind, ms.tag, ms.params), 1)
		if err != nil || again.kind != ms.kind || again.tag != ms.tag || len(again.params) != len(ms.params) {
			t.Fatalf("%q: %+v vs %+v (%v)", spec, ms, again, err)
		}
		for k, v := range ms.params {
			if again.params[k] != v {
				t.Fatalf("%q: %s=%q lost", spec, k, v)
			}
		}
	}
	if _, err := parseMover("winda w", 3); err == nil {
		t.Fatal("unknown kind accepted")
	}
	if _, err := FromDocumentString(strings.Replace(doorMap, "MOVER door brama", "MOVER door nieistnieje", 1)); err == nil {
		t.Fatal("mover without tagged cells accepted")
	}
	a, err := ParseTexAnim("woda frames=w0,w1,w2 fps=6 scroll=0,0.25")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseTexAnim(FormatTexAnim(a))
	if err != nil || b.Name != "woda" || len(b.Frames) != 3 || b.FPS != 6 || b.ScrollV != 0.25 {
		t.Fatalf("texanim round trip %+v (%v)", b, err)
	}
}
