// Package level turns a parsed .aie MAP (GRID + PLACE + TRIGGER + SOUND) into
// runtime data: a raycaster grid, the player spawn, placed objects (items,
// zones, enemies, doors, ...), and the trigger/sound definitions.
//
// It is pure Go (no OpenGL calls) so levels can be loaded and played
// headlessly in tests.
//
// GRID symbols (same palette as the native editor):
//
//	.  floor            W  wall 1 (brick)   S  wall 2 (stone)
//	M  wall 3 (metal)   D  door (solid until opened)
//	P  player spawn     E  enemy "grunt"    B  enemy "boss"
//
// Any other single-character symbol is floor and is recorded in Markers.
//
// PLACE lines: PLACE <kind> [type] [id] <x> <y> <z> [key=value ...]
// "PLACE enemy_type <name> 0 0 0 k=v..." defines enemy defaults per type.
// Doors: key=<item> (opens with interact when the player holds the item,
// which is consumed), secret=yes (looks like the wall it replaces), use=yes.
// (doors take only an id: PLACE door <id> <x> <y> <z>; the cell becomes a
// door that unlockDoor/lockDoor and the unlock_door action open and close).
// x and z are map-plane coordinates (x = grid column, z = grid row, 1 unit =
// 1 cell, cell centre = n.5); y is height above the floor.
package level

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"aiengine/parser"
	"aiengine/renderer"
	"aiengine/sector"
)

// Placement is one PLACE line (or an object implied by a GRID symbol).
type Placement struct {
	Kind   string // item, zone, enemy, prop, door, light, weapon, ...
	Type   string // e.g. crystal, exit, grunt
	ID     string // unique id (auto-generated when omitted)
	X, Y   float64
	Height float64
	Params map[string]string
	Line   int
}

// Float returns a numeric parameter or def when absent/invalid.
func (p Placement) Float(key string, def float64) float64 {
	if v, ok := p.Params[key]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

// Level is a loaded, playable map.
type Level struct {
	Name       string
	Map        *renderer.MapData
	SpawnX     float64
	SpawnY     float64
	SpawnAngle float64 // radians; from MAP property PLAYER_ANGLE (degrees)

	Placements []Placement
	Markers    map[string][][2]int // unknown GRID symbol → cells
	Props      map[string]string   // MAP-level properties (FLOOR_COLOR, ...)
	Triggers   []*parser.AIENode
	Sounds     []*parser.AIENode

	// EnemyTypes are templates from "PLACE enemy_type <name> 0 0 0 k=v...":
	// default parameters for enemies of that type (also for spawnEnemy).
	EnemyTypes map[string]map[string]string

	// Lights are the placed light sources (PLACE light, LIGHT_POINT); see
	// lights.go. baseLight is the per-cell light before them (nil = lit map).
	Lights    []*Light
	baseLight []float64

	// Sectors is set for sector maps (sectors.go); Map is then a 1-unit
	// grid sampled from it for pathfinding and triggers.
	Sectors      *sector.Map
	sectorBase   []float64
	sectorLights [][]sectorLight

	doors map[string]*door
}

// DoorInfo describes a door for the host and scripts.
type DoorInfo struct {
	ID     string
	X, Y   int
	Key    string // inventory item needed to open it with "interact"
	Secret bool   // looks like a wall; opens with "interact"
	Use    bool   // opens with "interact" (plain grid doors)
	Open   bool
}

type door struct {
	DoorInfo
	Cell renderer.MapCell
}

// LoadFile parses a .aie file and builds the first MAP in it.
func LoadFile(path string) (*Level, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	doc, err := parser.ParseAIE(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	lv, err := FromDocument(doc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return lv, nil
}

// FromDocument builds a Level from the first MAP node of a parsed document.
func FromDocument(doc *parser.AIEDocument) (*Level, error) {
	var mapNode *parser.AIENode
	for _, n := range doc.Nodes {
		if n.IsMap() {
			mapNode = n
			break
		}
	}
	if mapNode == nil {
		return nil, fmt.Errorf("no MAP block")
	}
	grid := mapNode.FindChild("GRID")
	sectorDefs := mapNode.FindChildren("SECTOR")
	if len(sectorDefs) == 0 && (grid == nil || len(grid.Values) == 0) {
		return nil, fmt.Errorf("MAP %s has no GRID rows and no SECTOR lines", mapNode.Name)
	}

	lv := &Level{
		Name:    mapNode.Name,
		Markers: make(map[string][][2]int),
		Props:   make(map[string]string),
		doors:   make(map[string]*door),

		EnemyTypes: map[string]map[string]string{},
	}
	for k, v := range mapNode.Properties {
		lv.Props[strings.ToUpper(k)] = v
	}
	if deg, err := strconv.ParseFloat(lv.Props["PLAYER_ANGLE"], 64); err == nil {
		lv.SpawnAngle = deg * math.Pi / 180
	}

	if len(sectorDefs) > 0 {
		if err := lv.buildSectors(sectorDefs); err != nil {
			return nil, err
		}
	} else {
		if err := lv.buildGrid(grid.Values); err != nil {
			return nil, err
		}
		if err := lv.buildHeights(mapNode); err != nil {
			return nil, err
		}
	}

	counters := map[string]int{}
	for _, child := range mapNode.Children {
		switch strings.ToUpper(child.Type) {
		case "PLACE":
			p, err := parsePlace(child.Name, child.Line)
			if err != nil {
				return nil, err
			}
			if p.ID == "" {
				counters[p.Kind]++
				p.ID = fmt.Sprintf("%s_%s_%d", p.Kind, p.Type, counters[p.Kind])
			}
			if p.Kind == "enemy_type" {
				lv.EnemyTypes[p.Type] = p.Params
				continue
			}
			if p.Kind == "door" {
				cx, cy := int(math.Floor(p.X)), int(math.Floor(p.Y))
				cell := lv.Map.GetCell(cx, cy)
				if cell == renderer.CellEmpty {
					cell = renderer.CellDoor
					lv.Map.Grid[cy][cx] = cell
				}
				// A named door replaces the anonymous grid door on its cell.
				delete(lv.doors, fmt.Sprintf("door_%d_%d", cx, cy))
				lv.doors[p.ID] = &door{DoorInfo: DoorInfo{
					ID: p.ID, X: cx, Y: cy, Key: p.Params["key"],
					Secret: isYes(p.Params["secret"]), Use: isYes(p.Params["use"]),
				}, Cell: cell}
			}
			lv.Placements = append(lv.Placements, p)
		case "TRIGGER":
			lv.Triggers = append(lv.Triggers, child)
		case "SOUND":
			lv.Sounds = append(lv.Sounds, child)
		}
	}
	if err := lv.buildLights(mapNode); err != nil {
		return nil, err
	}
	// Enemy templates fill in parameters an enemy does not set itself.
	for i, p := range lv.Placements {
		if p.Kind != "enemy" {
			continue
		}
		for k, v := range lv.EnemyTypes[p.Type] {
			if _, set := p.Params[k]; !set {
				lv.Placements[i].Params[k] = v
			}
		}
	}
	return lv, nil
}

func isYes(v string) bool {
	switch strings.ToLower(v) {
	case "yes", "true", "1", "tak":
		return true
	}
	return false
}

func (lv *Level) buildGrid(rows []string) error {
	width := 0
	parsed := make([][]string, len(rows))
	for i, r := range rows {
		parsed[i] = strings.Fields(r)
		if len(parsed[i]) > width {
			width = len(parsed[i])
		}
	}
	cells := make([][]renderer.MapCell, len(rows))
	spawnFound := false
	enemyN := 0
	for y, row := range parsed {
		cells[y] = make([]renderer.MapCell, width)
		for x := 0; x < width; x++ {
			sym := "W" // pad ragged rows with wall
			if x < len(row) {
				sym = row[x]
			}
			cx, cy := float64(x)+0.5, float64(y)+0.5
			switch sym {
			case ".":
			case "W":
				cells[y][x] = renderer.CellWall1
			case "S":
				cells[y][x] = renderer.CellWall2
			case "M":
				cells[y][x] = renderer.CellWall3
			case "D":
				cells[y][x] = renderer.CellDoor
				id := fmt.Sprintf("door_%d_%d", x, y)
				lv.doors[id] = &door{DoorInfo: DoorInfo{ID: id, X: x, Y: y, Use: true}, Cell: renderer.CellDoor}
			case "P":
				lv.SpawnX, lv.SpawnY = cx, cy
				spawnFound = true
			case "E", "B":
				enemyN++
				typ := "grunt"
				if sym == "B" {
					typ = "boss"
				}
				lv.Placements = append(lv.Placements, Placement{
					Kind: "enemy", Type: typ, ID: fmt.Sprintf("grid_enemy_%d", enemyN),
					X: cx, Y: cy, Params: map[string]string{},
				})
			default:
				lv.Markers[sym] = append(lv.Markers[sym], [2]int{x, y})
			}
		}
	}
	if !spawnFound {
		return fmt.Errorf("GRID has no player spawn 'P'")
	}
	lv.Map = renderer.NewMapData(cells)
	return nil
}

// parsePlace parses "kind [type] [id] x y z [key=value...]".
func parsePlace(spec string, line int) (Placement, error) {
	fields := strings.Fields(spec)
	p := Placement{Params: map[string]string{}, Line: line}
	if len(fields) == 0 {
		return p, fmt.Errorf("line %d: empty PLACE", line)
	}
	p.Kind = strings.ToLower(fields[0])
	var names []string
	var nums []float64
	for _, f := range fields[1:] {
		if k, v, ok := strings.Cut(f, "="); ok {
			p.Params[strings.ToLower(k)] = v
			continue
		}
		if n, err := strconv.ParseFloat(f, 64); err == nil {
			nums = append(nums, n)
			continue
		}
		if len(nums) > 0 {
			return p, fmt.Errorf("line %d: PLACE %s: name %q after coordinates", line, p.Kind, f)
		}
		names = append(names, f)
	}
	if len(nums) != 3 {
		return p, fmt.Errorf("line %d: PLACE %s needs x y z coordinates, got %d numbers", line, p.Kind, len(nums))
	}
	switch {
	case p.Kind == "door" && len(names) == 1:
		// Doors have no type: "PLACE door <id> x y z".
		p.Type, p.ID = "door", names[0]
	case len(names) > 0:
		p.Type = names[0]
		if len(names) > 1 {
			p.ID = names[1]
		}
	}
	p.X, p.Height, p.Y = nums[0], nums[1], nums[2]
	return p, nil
}

// ByKind returns all placements of the given kind, in file order.
func (lv *Level) ByKind(kind string) []Placement {
	var out []Placement
	for _, p := range lv.Placements {
		if p.Kind == kind {
			out = append(out, p)
		}
	}
	return out
}

// DoorIDs lists all known door ids.
func (lv *Level) DoorIDs() []string {
	ids := make([]string, 0, len(lv.doors))
	for id := range lv.doors {
		ids = append(ids, id)
	}
	return ids
}

// SetDoor opens (clears the cell) or closes (restores it) a door. It returns
// false for an unknown id.
func (lv *Level) SetDoor(id string, open bool) bool {
	d, ok := lv.doors[id]
	if !ok {
		return false
	}
	if open {
		lv.Map.Grid[d.Y][d.X] = renderer.CellEmpty
	} else {
		lv.Map.Grid[d.Y][d.X] = d.Cell
	}
	d.Open = open
	return true
}

// Door returns information about a door.
func (lv *Level) Door(id string) (DoorInfo, bool) {
	d, ok := lv.doors[id]
	if !ok {
		return DoorInfo{}, false
	}
	return d.DoorInfo, true
}

// DoorAt returns the door on a cell, if any.
func (lv *Level) DoorAt(x, y int) (DoorInfo, bool) {
	for _, d := range lv.doors {
		if d.X == x && d.Y == y {
			return d.DoorInfo, true
		}
	}
	return DoorInfo{}, false
}

// Doors lists all doors.
func (lv *Level) Doors() []DoorInfo {
	out := make([]DoorInfo, 0, len(lv.doors))
	for _, d := range lv.doors {
		out = append(out, d.DoorInfo)
	}
	return out
}

// IsSolid reports whether the map cell under (x, y) blocks movement.
func (lv *Level) IsSolid(x, y float64) bool {
	return lv.Map.GetCell(int(math.Floor(x)), int(math.Floor(y))) != renderer.CellEmpty
}

// FromDocumentString parses .aie source and builds its first MAP.
func FromDocumentString(src string) (*Level, error) {
	doc, err := parser.ParseAIE(strings.NewReader(src))
	if err != nil {
		return nil, err
	}
	return FromDocument(doc)
}
