// Package world is the game's model of space, independent of rendering:
// the grid map (cells, floor/ceiling heights, light layer), the player's
// body (position, stance, jumping) and the geometry queries shared by
// movement, aiming, enemy sight and projectiles (trace.go). Grid maps and
// sector maps answer the same queries through the Geometry interface.
package world

// MapCell represents a single cell in the 2D grid map.
type MapCell int32

const (
	CellEmpty MapCell = 0
	CellWall1 MapCell = 1 // brick
	CellWall2 MapCell = 2 // stone
	CellWall3 MapCell = 3 // metal
	CellDoor  MapCell = 4 // wood
)

// MapData holds the grid data and dimensions for raycasting.
type MapData struct {
	Grid   [][]MapCell
	Width  int
	Height int

	// Optional per-cell layers (row-major, Width*Height; nil = defaults),
	// see heights.go.
	Floor []float64 // floor height (default 0)
	Ceil  []float64 // ceiling height (default 1)
	Side  []MapCell // texture of the cell's step faces (default stone)
	Light []float64 // light level (default 1), see light.go
	Flags []uint8   // CellPeg etc. (texanim.go)

	// Optional per-cell textures (texanim.go): TexNames is a palette
	// (index 0 = map default), the layers index it per cell.
	TexNames []string
	FloorTex []uint8
	CeilTex  []uint8
	WallTex  []uint8 // walls of a solid cell, step faces of an open one
}

func NewMapData(grid [][]MapCell) *MapData {
	if len(grid) == 0 {
		return &MapData{Width: 0, Height: 0}
	}
	return &MapData{
		Grid:   grid,
		Width:  len(grid[0]),
		Height: len(grid),
	}
}

func (m *MapData) GetCell(x, y int) MapCell {
	if x < 0 || x >= m.Width || y < 0 || y >= m.Height {
		return CellWall1
	}
	return m.Grid[y][x]
}

// LightAt returns the base light level of a cell (1 without a light layer).
func (m *MapData) LightAt(x, y int) float64 {
	if m.Light == nil || !m.inside(x, y) {
		return 1
	}
	return m.Light[y*m.Width+x]
}

// SetLight changes one cell's base light (allocating the layer, all 1).
func (m *MapData) SetLight(x, y int, v float64) {
	if !m.inside(x, y) {
		return
	}
	if m.Light == nil {
		m.Light = make([]float64, m.Width*m.Height)
		for i := range m.Light {
			m.Light[i] = 1
		}
	}
	m.Light[y*m.Width+x] = v
}
