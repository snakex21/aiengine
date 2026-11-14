// renderer/sprites.go — Billboard sprite data types and helpers.
// Sprites are drawn on the CPU (drawSpritesCPU in cast.go), clipped per
// column against walls, steps and floors.

package renderer

// Sprite represents a billboard sprite in the game world.
type Sprite struct {
	X        float64 // world position
	Y        float64
	Z        float64 // height offset from floor
	Scale    float64 // size multiplier
	TexName  string  // texture name in TextureManager
	Alpha    float32 // transparency
	Active   bool    // if false, sprite is not rendered
	EntityID string  // optional reference to entity
	IsWeapon bool    // HUD element – skip Z‑buffer, always on top

	// Optional view-dependent frames: Rotations[i] is the texture seen from
	// rotation i (0 = front, counter-clockwise, any count such as 8), chosen
	// by the angle between Angle (the facing direction, radians) and the
	// camera. Empty = always TexName.
	Angle     float64
	Rotations []string

	// AbsZ: Z is the absolute height of the sprite's centre instead of an
	// offset from its cell's floor (projectiles, effects in the air).
	AbsZ bool
	// Bright sprites glow: they ignore cell light, distance shading and fog
	// (fireballs, tracers, explosions).
	Bright bool
}

// SpriteBatch manages a collection of sprites and renders them efficiently.
type SpriteBatch struct {
	sprites []*Sprite
	texMgr  *TextureManager
	rc      *Raycaster
}

// NewSpriteBatch creates a sprite batch linked to a raycaster.
func NewSpriteBatch(rc *Raycaster) *SpriteBatch {
	return &SpriteBatch{
		rc:     rc,
		texMgr: rc.TexManager,
	}
}

// Add appends a sprite to the batch.
func (sb *SpriteBatch) Add(s *Sprite) {
	sb.sprites = append(sb.sprites, s)
}

// Remove deactivates a sprite by entity ID.
func (sb *SpriteBatch) Remove(entityID string) {
	for _, s := range sb.sprites {
		if s.EntityID == entityID {
			s.Active = false
		}
	}
}

// Clear removes all sprites from the batch.
func (sb *SpriteBatch) Clear() {
	sb.sprites = sb.sprites[:0]
}

// CreateDefaultSprites creates a set of example sprites for testing.
// All positions are on empty cells (0) in the 10x10 test map.
func CreateDefaultSprites() []*Sprite {
	return []*Sprite{
		{X: 1.5, Y: 3.5, Z: 0, Scale: 0.8, TexName: "sprite_enemy", Alpha: 1.0, Active: true, EntityID: "enemy1"},
		{X: 7.5, Y: 2.5, Z: 0, Scale: 0.8, TexName: "sprite_enemy", Alpha: 1.0, Active: true, EntityID: "enemy2"},
		{X: 4.5, Y: 3.5, Z: 0, Scale: 0.7, TexName: "sprite_prop", Alpha: 1.0, Active: true, EntityID: "prop1"},
		{X: 5.5, Y: 3.5, Z: 0, Scale: 0.7, TexName: "sprite_prop", Alpha: 1.0, Active: true, EntityID: "prop2"},
	}
}
