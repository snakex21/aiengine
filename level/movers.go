package level

// Animated sectors ("movers"): doors, lifts, crushers and moving floors or
// ceilings. A mover moves one plane (the floor or the ceiling) of every map
// piece carrying its tag: grid cells marked in TAG_GRID (or an animated
// PLACE door's cell) on grid maps, sectors with tag= on sector maps.
//
//	MOVER door  <tag> [speed=1.5] [wait=4] [key=item] [locked=yes] [open=h] [start=open]
//	MOVER lift  <tag> [low=h] [high=h] [speed=1] [wait=2.5]
//	MOVER crusher <tag> [low=h] [speed=1] [wait=0.6] [damage=10] [start=on]
//	MOVER floor <tag> to=h [speed=1] [wait=0] [return=yes] [loop=yes]
//	MOVER ceil  <tag> to=h ...
//
// Common keys: use=yes|no (activated by the use key on the area), walk=<tag>
// (activated when the player enters cells/sectors with that tag; walk=yes =
// its own area), sound_start= sound_move= (looped while moving)
// sound_stop=, reverse=yes|no (go back when a body blocks it).
//
// Heights are plain fields of the map (MapData.Floor/Ceil, Sector.Floor/
// Ceil), so rendering, collision, sight lines and enemy pathfinding follow
// the movement without any rebuild. On sector maps the nav grid sampled from
// the sectors is refreshed for the cells of the moving sectors only.

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"aiengine/renderer"
	"aiengine/sector"
)

// Body is something standing in the world that moving floors carry and
// closing ceilings can block or crush: the player, enemies (and later
// vehicles). Height is the free height it needs above its floor.
type Body struct {
	ID     string
	X, Y   float64
	Radius float64
	Height float64
}

// MoverEventKind tells what a mover did in UpdateMovers.
type MoverEventKind int

const (
	MoverStart   MoverEventKind = iota // started moving (Sound once, Loop while moving)
	MoverStop                          // reached its target (Sound once)
	MoverReverse                       // a body blocked it; it turned back
	MoverCrush                         // a body is being crushed (Body, Damage)
)

// MoverEvent is emitted by UpdateMovers for the host (sounds, damage).
type MoverEvent struct {
	Kind   MoverEventKind
	Tag    string
	Key    string // unique key of the mover (tag + plane), for looping sounds
	Sound  string
	Loop   string
	Body   string
	Damage float64
	X, Y   float64 // centre of the moving area
}

type moverState int

const (
	moverIdle moverState = iota
	moverMoving
	moverWaiting
)

// crushInterval is the time between two crush hits on a blocked body.
const crushInterval = 0.4

// Mover is one animated plane. Fields above the line are configuration
// (from the MOVER line), the rest is run-time state.
type Mover struct {
	Kind    string // door, lift, crusher, floor, ceil
	Tag     string
	Ceil    bool    // moves the ceiling (else the floor)
	Home    float64 // rest position (a door: closed)
	Away    float64 // position after activation (a door: open)
	Speed   float64 // units per second
	Wait    float64 // seconds at Away (and at Home while looping) before moving on
	Return  bool    // go back home after Wait
	Loop    bool    // keep cycling once started (crushers, platforms)
	Reverse bool    // turn back when a body blocks the move
	Damage  float64 // crush damage per hit (0 = no crushing)
	Use     bool    // the use key activates it
	Key     string  // inventory item needed the first time it is used
	Locked  bool    // use never opens it (scripts/switches only)
	Walk    string  // area tag whose entry activates it
	Secret  bool

	SoundStart, SoundMove, SoundStop string

	Pos       float64
	target    float64
	state     moverState
	timer     float64
	crushT    float64
	running   bool
	scripted  bool
	keyUsed   bool
	autostart bool
	area      moverArea
	line      int
}

type moverArea struct {
	cells   []int // grid maps: cell indices (y*W+x)
	sectors []int // sector maps
	raster  []int // sector maps: nav-grid cells to refresh
	cx, cy  float64
}

// IsOpen reports a door that is open or opening (any mover: away from home
// or heading there).
func (m *Mover) IsOpen() bool {
	if m.state == moverMoving {
		return m.target != m.Home
	}
	return m.Pos != m.Home
}

// Moving reports a mover in motion.
func (m *Mover) Moving() bool { return m.state == moverMoving }

// Busy reports a mover that is moving or waiting to move on.
func (m *Mover) Busy() bool { return m.state != moverIdle }

// Plane is "ceil" or "floor".
func (m *Mover) Plane() string {
	if m.Ceil {
		return "ceil"
	}
	return "floor"
}

// Center is the centre of the moving area.
func (m *Mover) Center() (float64, float64) { return m.area.cx, m.area.cy }

// Cells lists the grid cells of the area (grid maps) or the nav-grid cells
// sampled from its sectors (sector maps).
func (m *Mover) Cells(w int) [][2]int {
	src := m.area.cells
	if len(m.area.sectors) > 0 {
		src = m.area.raster
	}
	out := make([][2]int, len(src))
	for i, c := range src {
		out[i] = [2]int{c % w, c / w}
	}
	return out
}

func (m *Mover) key() string { return m.Tag + "/" + m.Plane() }

// moverSpec is a parsed MOVER line before its area is resolved.
type moverSpec struct {
	kind, tag string
	params    map[string]string
	line      int
}

func parseMover(spec string, line int) (moverSpec, error) {
	f := strings.Fields(spec)
	ms := moverSpec{params: map[string]string{}, line: line}
	var names []string
	for _, w := range f {
		if k, v, ok := strings.Cut(w, "="); ok {
			ms.params[strings.ToLower(k)] = v
			continue
		}
		names = append(names, w)
	}
	if len(names) != 2 {
		return ms, fmt.Errorf("line %d: MOVER needs <kind> <tag>, got %q", line, spec)
	}
	ms.kind, ms.tag = strings.ToLower(names[0]), names[1]
	switch ms.kind {
	case "door", "lift", "crusher", "floor", "ceil":
	default:
		return ms, fmt.Errorf("line %d: unknown MOVER kind %q (door, lift, crusher, floor, ceil)", line, ms.kind)
	}
	return ms, nil
}

func (ms moverSpec) float(key string, def float64) (float64, error) {
	v, ok := ms.params[key]
	if !ok {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("line %d: MOVER %s %s: %s=%q is not a number", ms.line, ms.kind, ms.tag, key, v)
	}
	return f, nil
}

func (ms moverSpec) flag(key string, def bool) bool {
	v, ok := ms.params[key]
	if !ok {
		return def
	}
	return isYes(v)
}

// ---- areas ----

// TagAt returns the tag of the map piece under (x, y): the TAG_GRID cell on
// grid maps, the sector's tag on sector maps ("" = none).
func (lv *Level) TagAt(x, y float64) string {
	if lv.Sectors != nil {
		if s := lv.Sectors.SectorAt(x, y); s >= 0 {
			return lv.Sectors.Sectors[s].Tag
		}
		return ""
	}
	return lv.CellTag(int(math.Floor(x)), int(math.Floor(y)))
}

// CellTag returns the tag of grid cell (x, y) ("" = none).
func (lv *Level) CellTag(x, y int) string {
	m := lv.Map
	if lv.cellTags == nil || x < 0 || y < 0 || x >= m.Width || y >= m.Height {
		return ""
	}
	return lv.cellTags[y*m.Width+x]
}

// ensureGridLayers allocates the height, side, flag and tag layers of a grid
// map so movers can change cells one by one.
func (lv *Level) ensureGridLayers() {
	m := lv.Map
	n := m.Width * m.Height
	if !m.HasHeights() {
		m.SetHeights(lv.propFloat("CEILING_HEIGHT", renderer.DefaultCeiling))
	}
	if m.Side == nil {
		m.Side = make([]renderer.MapCell, n)
		for i := range m.Side {
			m.Side[i] = renderer.CellWall2
		}
	}
	if m.Flags == nil {
		m.Flags = make([]uint8, n)
	}
	if lv.cellTags == nil {
		lv.cellTags = make([]string, n)
	}
}

// resolveArea finds the pieces carrying the mover's tag.
func (lv *Level) resolveArea(m *Mover) error {
	if lv.Sectors != nil {
		for i := range lv.Sectors.Sectors {
			if lv.Sectors.Sectors[i].Tag == m.Tag {
				m.area.sectors = append(m.area.sectors, i)
			}
		}
		if len(m.area.sectors) == 0 {
			return fmt.Errorf("line %d: MOVER %s %s: no sector has tag=%s", m.line, m.Kind, m.Tag, m.Tag)
		}
		var cx, cy float64
		for _, s := range m.area.sectors {
			c := lv.Sectors.Centroid(s)
			cx, cy = cx+c.X, cy+c.Y
		}
		n := float64(len(m.area.sectors))
		m.area.cx, m.area.cy = cx/n, cy/n
		lv.rasterArea(m)
		return nil
	}
	if lv.cellTags != nil {
		for i, t := range lv.cellTags {
			if t == m.Tag {
				m.area.cells = append(m.area.cells, i)
			}
		}
	}
	if len(m.area.cells) == 0 {
		return fmt.Errorf("line %d: MOVER %s %s: no TAG_GRID cell has tag %s", m.line, m.Kind, m.Tag, m.Tag)
	}
	var cx, cy float64
	w := lv.Map.Width
	for _, c := range m.area.cells {
		cx += float64(c%w) + 0.5
		cy += float64(c/w) + 0.5
	}
	n := float64(len(m.area.cells))
	m.area.cx, m.area.cy = cx/n, cy/n
	return nil
}

// rasterArea collects the nav-grid cells a sector mover changes: cells
// whose centre lies in one of its sectors, plus (for moving ceilings) cells
// the sectors only partly cover, which take the lower ceiling so enemies
// never path through a closed door narrower than a cell.
func (lv *Level) rasterArea(m *Mover) {
	sm, md := lv.Sectors, lv.Map
	if lv.rasterCentre == nil {
		lv.rasterCentre = map[int]int{}
		lv.rasterExtra = map[int][]int{}
	}
	seen := map[int]bool{}
	for _, s := range m.area.sectors {
		lo, hi := sm.Sectors[s].Poly[0], sm.Sectors[s].Poly[0]
		for _, p := range sm.Sectors[s].Poly {
			lo = sector.Vec2{X: math.Min(lo.X, p.X), Y: math.Min(lo.Y, p.Y)}
			hi = sector.Vec2{X: math.Max(hi.X, p.X), Y: math.Max(hi.Y, p.Y)}
		}
		for y := int(math.Floor(lo.Y)); y <= int(math.Floor(hi.Y)); y++ {
			for x := int(math.Floor(lo.X)); x <= int(math.Floor(hi.X)); x++ {
				if x < 0 || y < 0 || x >= md.Width || y >= md.Height || md.GetCell(x, y) != renderer.CellEmpty {
					continue
				}
				c := y*md.Width + x
				centre := sm.SectorAt(float64(x)+0.5, float64(y)+0.5)
				covered := centre == s
				if !covered && m.Ceil {
					for _, k := range [3]float64{1.0 / 6, 0.5, 5.0 / 6} {
						for _, l := range [3]float64{1.0 / 6, 0.5, 5.0 / 6} {
							covered = covered || sm.Contains(s, sector.Vec2{X: float64(x) + k, Y: float64(y) + l})
						}
					}
					if covered && centre != s {
						lv.rasterExtra[c] = appendUnique(lv.rasterExtra[c], s)
					}
				}
				if !covered {
					continue
				}
				lv.rasterCentre[c] = centre
				if !seen[c] {
					seen[c] = true
					m.area.raster = append(m.area.raster, c)
				}
			}
		}
	}
	sort.Ints(m.area.raster)
}

func appendUnique(xs []int, v int) []int {
	for _, x := range xs {
		if x == v {
			return xs
		}
	}
	return append(xs, v)
}

// refreshRaster resamples one nav-grid cell from its sectors.
func (lv *Level) refreshRaster(c int) {
	s, ok := lv.rasterCentre[c]
	if !ok || s < 0 {
		return
	}
	md := lv.Map
	f, ce := lv.Sectors.Sectors[s].Floor, lv.Sectors.Sectors[s].Ceil
	for _, e := range lv.rasterExtra[c] {
		ce = math.Min(ce, lv.Sectors.Sectors[e].Ceil)
	}
	md.Floor[c], md.Ceil[c] = f, ce
}

// applyMover sets the moving plane of the whole area to h.
func (lv *Level) applyMover(m *Mover, h float64) {
	if lv.Sectors != nil {
		for _, s := range m.area.sectors {
			if m.Ceil {
				lv.Sectors.Sectors[s].Ceil = h
			} else {
				lv.Sectors.Sectors[s].Floor = h
			}
		}
		for _, c := range m.area.raster {
			lv.refreshRaster(c)
		}
		return
	}
	md := lv.Map
	for _, c := range m.area.cells {
		if m.Ceil {
			md.Ceil[c] = h
		} else {
			md.Floor[c] = h
		}
	}
}

// areaHeights returns the highest floor, lowest ceiling and lowest
// neighbouring floor of a mover's area as currently on the map.
func (lv *Level) areaHeights(m *Mover) (maxFloor, minCeil, lowNeighbour float64) {
	maxFloor, minCeil, lowNeighbour = math.Inf(-1), math.Inf(1), math.Inf(1)
	if lv.Sectors != nil {
		sm := lv.Sectors
		for _, s := range m.area.sectors {
			maxFloor = math.Max(maxFloor, sm.Sectors[s].Floor)
			minCeil = math.Min(minCeil, sm.Sectors[s].Ceil)
			for _, o := range sm.Neighbours(s) {
				if sm.Sectors[o].Tag != m.Tag {
					lowNeighbour = math.Min(lowNeighbour, sm.Sectors[o].Floor)
				}
			}
		}
	} else {
		md := lv.Map
		for _, c := range m.area.cells {
			x, y := c%md.Width, c/md.Width
			maxFloor = math.Max(maxFloor, md.FloorAt(x, y))
			minCeil = math.Min(minCeil, md.CeilAt(x, y))
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if md.GetCell(nx, ny) == renderer.CellEmpty && lv.CellTag(nx, ny) != m.Tag {
					lowNeighbour = math.Min(lowNeighbour, md.FloorAt(nx, ny))
				}
			}
		}
	}
	if math.IsInf(lowNeighbour, 1) {
		lowNeighbour = maxFloor
	}
	return
}

// buildMovers resolves the MOVER lines (after the map, tags and animated
// doors exist) and puts every mover in its start position.
func (lv *Level) buildMovers(specs []moverSpec) error {
	if lv.Sectors == nil && len(specs) > 0 {
		lv.ensureGridLayers()
	}
	for _, ms := range specs {
		m := &Mover{Kind: ms.kind, Tag: ms.tag, line: ms.line}
		if err := lv.resolveArea(m); err != nil {
			return err
		}
		floor, ceil, low := lv.areaHeights(m)
		var err error
		get := func(key string, def float64) float64 {
			v, e := ms.float(key, def)
			if e != nil && err == nil {
				err = e
			}
			return v
		}
		switch ms.kind {
		case "door":
			m.Ceil, m.Home, m.Away = true, floor, get("open", ceil)
			m.Speed, m.Wait = get("speed", 1.5), get("wait", 4)
			m.Return, m.Reverse, m.Use = m.Wait > 0, true, true
		case "lift":
			m.Home, m.Away = get("high", floor), get("low", low)
			m.Speed, m.Wait = get("speed", 1), get("wait", 2.5)
			m.Return, m.Reverse, m.Use = true, true, true
		case "crusher":
			m.Ceil, m.Home, m.Away = true, get("high", ceil), get("low", floor+0.1)
			m.Speed, m.Wait, m.Damage = get("speed", 1), get("wait", 0.6), get("damage", 10)
			m.Loop = true
		case "floor", "ceil":
			m.Ceil = ms.kind == "ceil"
			home := floor
			if m.Ceil {
				home = ceil
			}
			if _, ok := ms.params["to"]; !ok {
				return fmt.Errorf("line %d: MOVER %s %s needs to=<height>", ms.line, ms.kind, ms.tag)
			}
			m.Home, m.Away = get("from", home), get("to", home)
			m.Speed, m.Wait = get("speed", 1), get("wait", 0)
			m.Return, m.Reverse = ms.flag("return", false), true
		}
		m.Damage = get("damage", m.Damage)
		if err != nil {
			return err
		}
		m.Return = ms.flag("return", m.Return)
		m.Loop = ms.flag("loop", m.Loop)
		m.Reverse = ms.flag("reverse", m.Reverse)
		m.Use = ms.flag("use", m.Use)
		m.Key, m.Locked = ms.params["key"], ms.flag("locked", false)
		m.Secret = ms.flag("secret", false)
		if w := ms.params["walk"]; w != "" {
			m.Walk = w
			if isYes(w) {
				m.Walk = m.Tag
			}
		}
		m.SoundStart, m.SoundMove, m.SoundStop = moverSounds(ms)
		if m.Speed <= 0 {
			return fmt.Errorf("line %d: MOVER %s %s: speed must be positive", ms.line, ms.kind, ms.tag)
		}
		if !m.Ceil && math.Max(m.Home, m.Away) > ceil+1e-9 {
			return fmt.Errorf("line %d: MOVER %s %s: floor would rise above the ceiling (%.3g)", ms.line, ms.kind, ms.tag, ceil)
		}
		if m.Ceil && math.Min(m.Home, m.Away) < floor-1e-9 {
			return fmt.Errorf("line %d: MOVER %s %s: ceiling would sink below the floor (%.3g)", ms.line, ms.kind, ms.tag, floor)
		}
		m.Pos = m.Home
		switch strings.ToLower(ms.params["start"]) {
		case "open", "away":
			m.Pos = m.Away
		case "on", "yes", "run":
			m.autostart = true
		}
		m.target = m.Pos
		lv.addMover(m)
		lv.applyMover(m, m.Pos)
		if m.Kind == "door" || m.Kind == "crusher" || m.Kind == "lift" {
			lv.pegArea(m)
		}
	}
	return nil
}

func moverSounds(ms moverSpec) (start, move, stop string) {
	start, move, stop = "mover_start", "mover_move", "mover_stop"
	if v, ok := ms.params["sound_start"]; ok {
		start = v
	}
	if v, ok := ms.params["sound_move"]; ok {
		move = v
	}
	if v, ok := ms.params["sound_stop"]; ok {
		stop = v
	}
	for _, p := range []*string{&start, &move, &stop} {
		if strings.EqualFold(*p, "none") {
			*p = ""
		}
	}
	return
}

func (lv *Level) addMover(m *Mover) {
	lv.movers = append(lv.movers, m)
	if lv.moverTags == nil {
		lv.moverTags = map[string][]*Mover{}
	}
	lv.moverTags[m.Tag] = append(lv.moverTags[m.Tag], m)
}

// pegArea makes the step faces into the area follow its moving ceiling.
func (lv *Level) pegArea(m *Mover) {
	if lv.Sectors != nil {
		for _, s := range m.area.sectors {
			lv.Sectors.Sectors[s].Peg = true
		}
		return
	}
	for _, c := range m.area.cells {
		lv.Map.Flags[c] |= renderer.CellPeg
	}
}

// Movers lists every mover of the level.
func (lv *Level) Movers() []*Mover { return lv.movers }

// MoversByTag lists the movers of one tag.
func (lv *Level) MoversByTag(tag string) []*Mover { return lv.moverTags[tag] }

// ---- control ----

func (m *Mover) startMove(to float64, out []MoverEvent) []MoverEvent {
	m.target = to
	m.state = moverMoving
	return append(out, MoverEvent{Kind: MoverStart, Tag: m.Tag, Key: m.key(), Sound: m.SoundStart, Loop: m.SoundMove,
		X: m.area.cx, Y: m.area.cy})
}

// activate is the use/switch/walk/script activation of one mover. It
// reports whether the mover reacted.
func (m *Mover) activate() bool {
	switch {
	case m.Kind == "door":
		switch {
		case m.state == moverMoving && m.target == m.Home:
			m.pending(m.Away)
		case m.state == moverMoving:
			return false
		case m.state == moverWaiting || m.Pos != m.Home:
			m.pending(m.Home)
		default:
			m.pending(m.Away)
		}
	case m.Loop:
		if m.running {
			return false
		}
		m.running = true
		m.pending(m.Away)
	default:
		if m.state != moverIdle {
			return false
		}
		if m.Pos == m.Home {
			m.pending(m.Away)
		} else {
			m.pending(m.Home)
		}
	}
	m.scripted = false
	return true
}

// pending starts a move; the start event is emitted by the next update.
func (m *Mover) pending(to float64) {
	if m.state == moverMoving && m.target == to {
		return
	}
	m.target = to
	m.state = moverMoving
	m.timer = -1 // marks "start event not sent yet"
}

// Activate activates every mover with the tag (switches, walk triggers,
// scripts; keys and locks are not checked). It reports whether any reacted.
func (lv *Level) Activate(tag string) bool {
	ok := false
	for _, m := range lv.moverTags[tag] {
		if m.activate() {
			ok = true
		}
	}
	return ok
}

// SetDoorMovers opens or closes the door movers of a tag (scripts'
// openDoor/closeDoor); other movers with the tag move away (open) or home.
func (lv *Level) SetDoorMovers(tag string, open bool) bool {
	ms := lv.moverTags[tag]
	for _, m := range ms {
		to := m.Home
		if open {
			to = m.Away
		}
		if open && m.Loop {
			m.running = true
		}
		if !open {
			m.running = false
		}
		if (m.state == moverIdle && m.Pos == to) || (m.state == moverWaiting && open && m.Pos == to) {
			continue
		}
		m.pending(to)
	}
	return len(ms) > 0
}

// MoveTo moves the floor (ceil=false) or ceiling of every piece tagged tag
// to height h at speed (<= 0: the mover's speed, or 1). A mover is created
// for tags that have none yet. It reports whether the tag exists.
func (lv *Level) MoveTo(tag string, ceil bool, h, speed float64) bool {
	var m *Mover
	for _, c := range lv.moverTags[tag] {
		if c.Ceil == ceil {
			m = c
		}
	}
	if m == nil {
		kind := "floor"
		if ceil {
			kind = "ceil"
		}
		m = &Mover{Kind: kind, Tag: tag, Ceil: ceil, Speed: 1, Reverse: true}
		if lv.Sectors == nil {
			if lv.cellTags == nil {
				return false
			}
			lv.ensureGridLayers()
		}
		if err := lv.resolveArea(m); err != nil {
			return false
		}
		floor, c, _ := lv.areaHeights(m)
		m.Home = floor
		if ceil {
			m.Home = c
		}
		m.Away, m.Pos, m.target = h, m.Home, m.Home
		m.SoundStart, m.SoundMove, m.SoundStop = "mover_start", "mover_move", "mover_stop"
		lv.addMover(m)
	}
	if speed > 0 {
		m.Speed = speed
	}
	m.running = false
	m.scripted = true
	if m.Pos == h && m.state != moverMoving {
		return true
	}
	m.pending(h)
	return true
}

// StopMovers stops looping movers of a tag; they return home and stay.
func (lv *Level) StopMovers(tag string) bool {
	ms := lv.moverTags[tag]
	for _, m := range ms {
		m.running = false
		if m.Pos != m.Home || m.state == moverMoving {
			m.pending(m.Home)
		}
	}
	return len(ms) > 0
}

// MoverHeight returns the current height of the floor or ceiling mover of a
// tag (ok false when there is none).
func (lv *Level) MoverHeight(tag string, ceil bool) (float64, bool) {
	for _, m := range lv.moverTags[tag] {
		if m.Ceil == ceil {
			return m.Pos, true
		}
	}
	return 0, false
}

// blocksUse reports a wall at (x, y) for the use ray.
func (lv *Level) blocksUse(x, y float64) bool {
	if lv.Sectors != nil {
		return lv.Sectors.SectorAt(x, y) < 0
	}
	return lv.Solid(int(math.Floor(x)), int(math.Floor(y)))
}

// ---- simulation ----

// Support is the floor and ceiling under a body of radius r at (x, y) on
// the level's walkable world (sector or grid map).
func (lv *Level) Support(x, y, r float64) (float64, float64) { return lv.support(x, y, r) }

// support is the floor and ceiling under a body (sector or grid map).
func (lv *Level) support(x, y, r float64) (float64, float64) {
	if lv.Sectors != nil {
		return lv.Sectors.Support(x, y, r)
	}
	return lv.Map.Support(x, y, r)
}

// overlaps reports whether a body touches the mover's area.
func (lv *Level) overlaps(m *Mover, b Body) bool {
	if lv.Sectors != nil {
		var buf [8]int
		ts, _ := lv.Sectors.Touched(b.X, b.Y, b.Radius, buf[:])
		for _, s := range ts {
			if lv.Sectors.Sectors[s].Tag == m.Tag {
				return true
			}
		}
		return false
	}
	for _, p := range [5][2]float64{{b.X, b.Y}, {b.X - b.Radius, b.Y}, {b.X + b.Radius, b.Y}, {b.X, b.Y - b.Radius}, {b.X, b.Y + b.Radius}} {
		if lv.CellTag(int(math.Floor(p[0])), int(math.Floor(p[1]))) == m.Tag {
			return true
		}
	}
	return false
}

// blocked appends the indices of bodies in the area that no longer fit
// between floor and ceiling.
func (lv *Level) blocked(m *Mover, bodies []Body, out []int) []int {
	for i, b := range bodies {
		if !lv.overlaps(m, b) {
			continue
		}
		f, c := lv.support(b.X, b.Y, b.Radius)
		if c-f < b.Height-1e-6 {
			out = append(out, i)
		}
	}
	return out
}

// UpdateMovers advances every mover by dt. bodies are the things in the
// world (player, enemies): moving planes stop or turn back when they would
// squeeze a body, crushers hurt it instead. Events are appended to out.
func (lv *Level) UpdateMovers(dt float64, bodies []Body, out []MoverEvent) []MoverEvent {
	for _, m := range lv.movers {
		out = lv.UpdateMover(m, dt, bodies, out)
	}
	return out
}

// UpdateMover advances one mover (see UpdateMovers).
func (lv *Level) UpdateMover(m *Mover, dt float64, bodies []Body, out []MoverEvent) []MoverEvent {
	if m.autostart {
		m.autostart = false
		m.activate()
	}
	switch m.state {
	case moverIdle:
		return out
	case moverWaiting:
		m.timer -= dt
		if m.timer > 0 {
			return out
		}
		to := m.Home
		if m.Pos == m.Home {
			to = m.Away
		}
		return m.startMove(to, out)
	}
	if m.timer < 0 { // started by activate/pending: announce it
		m.timer = 0
		out = m.startMove(m.target, out)
	}
	next := m.Pos
	step := m.Speed * dt
	if m.target > m.Pos {
		next = math.Min(m.target, m.Pos+step)
	} else {
		next = math.Max(m.target, m.Pos-step)
	}
	// Only a move that narrows the gap (ceiling down, floor up) can block.
	if (m.Ceil && next < m.Pos) || (!m.Ceil && next > m.Pos) {
		lv.applyMover(m, next)
		var buf [8]int
		hit := lv.blocked(m, bodies, buf[:0])
		if len(hit) > 0 {
			lv.applyMover(m, m.Pos)
			switch {
			case m.Damage > 0:
				m.crushT -= dt
				if m.crushT <= 0 {
					m.crushT = crushInterval
					for _, i := range hit {
						out = append(out, MoverEvent{Kind: MoverCrush, Tag: m.Tag, Key: m.key(), Body: bodies[i].ID,
							Damage: m.Damage, X: m.area.cx, Y: m.area.cy})
					}
				}
			case m.Reverse:
				back := m.Home
				if m.target == m.Home {
					back = m.Away
				}
				m.target = back
				out = append(out, MoverEvent{Kind: MoverReverse, Tag: m.Tag, Key: m.key(), X: m.area.cx, Y: m.area.cy})
			}
			return out
		}
	} else {
		lv.applyMover(m, next)
	}
	m.crushT = 0
	m.Pos = next
	if next == m.target {
		out = append(out, MoverEvent{Kind: MoverStop, Tag: m.Tag, Key: m.key(), Sound: m.SoundStop, X: m.area.cx, Y: m.area.cy})
		m.arrive()
	}
	return out
}

// arrive decides what happens at the end of a move.
func (m *Mover) arrive() {
	atAway := m.Pos == m.Away && m.Pos != m.Home
	switch {
	case m.scripted:
		m.state = moverIdle
	case atAway && (m.Return || (m.Loop && m.running)):
		m.state, m.timer = moverWaiting, m.Wait
	case !atAway && m.Loop && m.running:
		m.state, m.timer = moverWaiting, m.Wait
	default:
		m.state = moverIdle
	}
}

// FormatMover writes a MOVER line's text (after the keyword) from its kind,
// tag and parameters, keys sorted (the inverse of the parser).
func FormatMover(kind, tag string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(kind + " " + tag)
	for _, k := range keys {
		b.WriteString(" " + k + "=" + params[k])
	}
	return b.String()
}
