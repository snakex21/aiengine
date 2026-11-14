// renderer/texanim.go — animated textures: frame cycling (water, lava,
// blinking lights) and scrolling, plus per-cell texture layers of grid maps.
//
// An animation is bound to a texture name. Every place that resolves a
// texture by name for a frame (walls, floors, ceilings, sector surfaces,
// sprites) gets the animation's current frame instead, so maps simply use
// the animated name. Frames are picked once per rendered frame (one map
// lookup per name); scrolling is a texel offset applied while sampling, so
// the per-pixel cost is a single add.

package renderer

import "math"

// TexAnim animates the texture called Name: it shows Frames in turn at FPS
// (none = Name itself) and scrolls by ScrollU/ScrollV texture repeats per
// second (walls: u along the wall, v downwards; floors and ceilings: u along
// world x, v along world y).
type TexAnim struct {
	Name             string
	Frames           []string
	FPS              float64
	ScrollU, ScrollV float64
}

// FrameAt returns the index into Frames shown at time t (0 without frames).
func (a *TexAnim) FrameAt(t float64) int {
	n := len(a.Frames)
	if n == 0 || a.FPS <= 0 {
		return 0
	}
	i := int(math.Floor(t*a.FPS)) % n
	if i < 0 {
		i += n
	}
	return i
}

// texAnimState is the resolved texture of one animation for this frame.
type texAnimState struct {
	anim TexAnim
	view Texture2D // shallow copy of the current frame with scroll offsets
	ok   bool
}

// SetTexAnims replaces the texture animations (nil = none).
func (rc *Raycaster) SetTexAnims(list []TexAnim) {
	rc.texAnims = nil
	if len(list) == 0 {
		return
	}
	rc.texAnims = make(map[string]*texAnimState, len(list))
	for _, a := range list {
		rc.texAnims[a.Name] = &texAnimState{anim: a}
	}
}

// resolveTexAnims picks every animation's frame for time t.
func (rc *Raycaster) resolveTexAnims(t float64) {
	for _, st := range rc.texAnims {
		a := &st.anim
		name := a.Name
		if len(a.Frames) > 0 {
			name = a.Frames[a.FrameAt(t)]
		}
		base := rc.TexManager.Get(name)
		if base == nil || base.Width <= 0 || base.Height <= 0 {
			st.ok = false
			continue
		}
		st.view = *base
		st.view.OffU = scrollTexels(t*a.ScrollU, base.Width)
		st.view.OffV = scrollTexels(t*a.ScrollV, base.Height)
		st.ok = true
	}
}

func scrollTexels(rep float64, size int32) int32 {
	f := rep - math.Floor(rep)
	return int32(f * float64(size))
}

// tex resolves a texture name for the current frame (animations first).
func (rc *Raycaster) tex(name string) *Texture2D {
	if st, ok := rc.texAnims[name]; ok {
		if st.ok {
			return &st.view
		}
		return nil
	}
	return rc.TexManager.Get(name)
}

// TextureAt resolves a texture name at animation time t as the renderer
// would (for tests and tools).
func (rc *Raycaster) TextureAt(name string, t float64) *Texture2D {
	rc.resolveTexAnims(t)
	return rc.tex(name)
}

// Per-cell texture layers of grid maps (MapData.TexNames + FloorTex/CeilTex/
// WallTex): index 0 = the map default, i > 0 = TexNames[i].

// palette resolves the map's texture palette for this frame.
func (rc *Raycaster) palette(f *frame, m *MapData) {
	f.pal = f.pal[:0]
	for _, n := range m.TexNames {
		var t *Texture2D
		if n != "" {
			if st, ok := symbolTextures[n]; ok {
				n = st.name
			}
			t = usableTex(rc.tex(n))
		}
		f.pal = append(f.pal, t)
	}
}

func layerTex(f *frame, layer []uint8, i int, def *Texture2D) *Texture2D {
	if layer == nil || i < 0 || i >= len(layer) {
		return def
	}
	if v := int(layer[i]); v > 0 && v < len(f.pal) && f.pal[v] != nil {
		return f.pal[v]
	}
	return def
}

func (f *frame) cellIndex(m *MapData, x, y int) int {
	if !m.Inside(x, y) {
		return -1
	}
	return y*m.Width + x
}

// Cell flags (MapData.Flags).
const (
	// CellPeg anchors the step faces into the cell to its moving plane
	// (doors, crushers, lifts): their texture moves with it.
	CellPeg uint8 = 1
)
