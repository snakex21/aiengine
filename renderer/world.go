// renderer/world.go — the map and player types now live in package world;
// these aliases keep the renderer's (and older callers') names working.

package renderer

import (
	"math"

	"aiengine/world"
)

type (
	MapCell     = world.MapCell
	MapData     = world.MapData
	PlayerState = world.PlayerState
	Stance      = world.Stance
)

const (
	CellEmpty = world.CellEmpty
	CellWall1 = world.CellWall1
	CellWall2 = world.CellWall2
	CellWall3 = world.CellWall3
	CellDoor  = world.CellDoor

	DefaultFloor   = world.DefaultFloor
	DefaultCeiling = world.DefaultCeiling
	EyeHeight      = world.EyeHeight

	MaxStepUp      = world.MaxStepUp
	HeadroomStand  = world.HeadroomStand
	HeadroomCrouch = world.HeadroomCrouch
	HeadroomProne  = world.HeadroomProne

	StanceStand  = world.StanceStand
	StanceCrouch = world.StanceCrouch
	StanceProne  = world.StanceProne

	JumpVelocity = world.JumpVelocity
	GravityAccel = world.GravityAccel
	MaxJumpZ     = world.MaxJumpZ
	PitchLimit   = world.PitchLimit
)

// NewMapData and NewPlayerState create world types.
func NewMapData(grid [][]MapCell) *MapData { return world.NewMapData(grid) }

func NewPlayerState(x, y, angle float64) *PlayerState { return world.NewPlayerState(x, y, angle) }

// cellAtPoint returns the cell containing world point (x, y).
func cellAtPoint(x, y float64) (int, int) { return int(math.Floor(x)), int(math.Floor(y)) }
