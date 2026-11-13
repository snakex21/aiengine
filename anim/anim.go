// Package anim plays sprite animations: named states (idle, walk, attack,
// pain, death) made of frames with their own timing, each frame optionally
// drawn from several view angles (rotations, usually 8). It is pure Go: the
// host maps frames to texture names and hands them to the renderer
// (Sprite.TexName / Sprite.Rotations), which picks the rotation by the
// viewing angle (renderer.RotationIndex).
//
// Sheets are described by a small text file next to the image (see
// ParseDesc):
//
//	# straznik.anim
//	SHEET straznik_8.png     # image, relative to this file
//	GRID 8 7                 # columns rows of equal cells
//	ROTATIONS 8              # columns = rotations (0 front, counter-clockwise), rows = frames
//	MIRROR yes               # optional: only columns 0..4 drawn, 5..7 mirror 3..1
//	ANIM idle   frames=0,1     fps=2
//	ANIM walk   frames=1,2,3,2 fps=7
//	ANIM attack frames=4,5     fps=8 loop=no
//	ANIM pain   frames=6       fps=6 loop=no
//	ANIM death  frames=6,7,8   fps=6 loop=no
//
// With ROTATIONS 1 (default) frames are the cells in row-major order.
package anim

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Standard state names used by the enemy state machine.
const (
	Idle   = "idle"
	Walk   = "walk"
	Attack = "attack"
	Pain   = "pain"
	Death  = "death"
)

// Clip is one animation state: frame indices shown at FPS, looping or
// holding the last frame.
type Clip struct {
	Name   string
	Frames []int
	FPS    float64
	Loop   bool
}

// Duration is the clip's length in seconds (one pass).
func (c *Clip) Duration() float64 {
	if c.FPS <= 0 {
		return 0
	}
	return float64(len(c.Frames)) / c.FPS
}

// FrameAt returns the frame shown t seconds into the clip.
func (c *Clip) FrameAt(t float64) (frame int, done bool) {
	if len(c.Frames) == 0 {
		return 0, true
	}
	if c.FPS <= 0 || t < 0 {
		return c.Frames[0], !c.Loop && c.FPS <= 0
	}
	i := int(t * c.FPS)
	if c.Loop {
		return c.Frames[i%len(c.Frames)], false
	}
	if i >= len(c.Frames) {
		return c.Frames[len(c.Frames)-1], true
	}
	return c.Frames[i], false
}

// Set is the animation data of one sprite type: clips and, per frame, the
// texture of every rotation (Tex[frame][rotation]).
type Set struct {
	Clips map[string]*Clip
	Tex   [][]string
}

// Rotations is the number of view angles of the set (1 = billboard).
func (s *Set) Rotations() int {
	if len(s.Tex) == 0 {
		return 0
	}
	return len(s.Tex[0])
}

// Clip returns the clip for a state, falling back to idle for missing
// walk/attack/pain clips (nil when neither exists; death has no fallback).
func (s *Set) Clip(state string) *Clip {
	if c := s.Clips[state]; c != nil {
		return c
	}
	if state == Death {
		return nil
	}
	return s.Clips[Idle]
}

// Player runs one sprite's animation.
type Player struct {
	Set   *Set
	State string
	T     float64 // seconds into the current state
	clip  *Clip
}

// NewPlayer starts a set in the idle state.
func NewPlayer(s *Set) *Player {
	p := &Player{Set: s}
	p.Play(Idle)
	return p
}

// Play switches to a state; playing the current state again keeps its
// timing (call Restart to start over).
func (p *Player) Play(state string) {
	if state == p.State && p.clip != nil {
		return
	}
	p.Restart(state)
}

// Restart starts a state from its first frame.
func (p *Player) Restart(state string) {
	p.State, p.T, p.clip = state, 0, p.Set.Clip(state)
}

// Update advances the animation clock.
func (p *Player) Update(dt float64) { p.T += dt }

// Done reports a non-looping state that has shown its last frame for its
// full time (or a state without a clip).
func (p *Player) Done() bool {
	if p.clip == nil {
		return true
	}
	if p.clip.Loop {
		return false
	}
	return p.T >= p.clip.Duration()
}

// HasClip reports whether the current state has frames.
func (p *Player) HasClip() bool { return p.clip != nil }

// Frame is the frame index shown now.
func (p *Player) Frame() int {
	if p.clip == nil {
		return 0
	}
	f, _ := p.clip.FrameAt(p.T)
	return f
}

// Textures returns the texture of the current frame (rotation 0) and all
// its rotations (nil for a single view).
func (p *Player) Textures() (name string, rotations []string) {
	f := p.Frame()
	if f < 0 || f >= len(p.Set.Tex) || len(p.Set.Tex[f]) == 0 {
		return "", nil
	}
	views := p.Set.Tex[f]
	if len(views) == 1 {
		return views[0], nil
	}
	return views[0], views
}

// Desc is a parsed sheet descriptor.
type Desc struct {
	Sheet      string
	Cols, Rows int
	Rotations  int
	Mirror     bool
	Clips      []*Clip
}

// Frames is the number of animation frames in the sheet.
func (d *Desc) Frames() int {
	if d.Rotations > 1 {
		return d.Rows
	}
	return d.Cols * d.Rows
}

// Cell returns the sheet cell (column, row) of frame f seen from rotation
// r and whether it must be drawn mirrored.
func (d *Desc) Cell(f, r int) (col, row int, mirror bool) {
	if d.Rotations <= 1 {
		return f % d.Cols, f / d.Cols, false
	}
	if d.Mirror && r > d.Rotations/2 {
		return d.Rotations - r, f, true
	}
	return r, f, false
}

// ParseDesc reads a sheet descriptor.
func ParseDesc(r io.Reader) (*Desc, error) {
	d := &Desc{Rotations: 1}
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		bad := func(msg string) error { return fmt.Errorf("line %d: %s", n, msg) }
		switch strings.ToUpper(f[0]) {
		case "SHEET":
			if len(f) != 2 {
				return nil, bad("SHEET <image>")
			}
			d.Sheet = f[1]
		case "GRID":
			if len(f) != 3 {
				return nil, bad("GRID <columns> <rows>")
			}
			c, err1 := strconv.Atoi(f[1])
			r, err2 := strconv.Atoi(f[2])
			if err1 != nil || err2 != nil || c <= 0 || r <= 0 {
				return nil, bad("GRID needs two positive numbers")
			}
			d.Cols, d.Rows = c, r
		case "ROTATIONS":
			v, err := strconv.Atoi(strings.Join(f[1:], ""))
			if err != nil || v < 1 {
				return nil, bad("ROTATIONS <n>")
			}
			d.Rotations = v
		case "MIRROR":
			d.Mirror = len(f) > 1 && isYes(f[1])
		case "ANIM":
			c, err := parseClip(f[1:])
			if err != nil {
				return nil, bad(err.Error())
			}
			d.Clips = append(d.Clips, c)
		default:
			return nil, bad("unknown keyword " + f[0])
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	switch {
	case d.Sheet == "":
		return nil, fmt.Errorf("descriptor has no SHEET")
	case d.Cols == 0:
		return nil, fmt.Errorf("descriptor has no GRID")
	case d.Rotations > 1 && d.Mirror && d.Cols < d.Rotations/2+1:
		return nil, fmt.Errorf("MIRROR with %d rotations needs %d columns", d.Rotations, d.Rotations/2+1)
	case d.Rotations > 1 && !d.Mirror && d.Cols < d.Rotations:
		return nil, fmt.Errorf("%d rotations need %d columns, GRID has %d", d.Rotations, d.Rotations, d.Cols)
	}
	for _, c := range d.Clips {
		for _, fr := range c.Frames {
			if fr < 0 || fr >= d.Frames() {
				return nil, fmt.Errorf("ANIM %s: frame %d outside the sheet (0..%d)", c.Name, fr, d.Frames()-1)
			}
		}
	}
	return d, nil
}

func parseClip(f []string) (*Clip, error) {
	if len(f) == 0 {
		return nil, fmt.Errorf("ANIM <state> frames=... fps=...")
	}
	c := &Clip{Name: strings.ToLower(f[0]), FPS: 8, Loop: true}
	if c.Name == Death || c.Name == Pain || c.Name == Attack {
		c.Loop = c.Name == Attack
	}
	for _, kv := range f[1:] {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("ANIM %s: expected key=value, got %q", c.Name, kv)
		}
		switch strings.ToLower(k) {
		case "frames":
			for _, s := range strings.Split(v, ",") {
				n, err := strconv.Atoi(strings.TrimSpace(s))
				if err != nil {
					return nil, fmt.Errorf("ANIM %s: bad frame %q", c.Name, s)
				}
				c.Frames = append(c.Frames, n)
			}
		case "fps":
			x, err := strconv.ParseFloat(v, 64)
			if err != nil || x < 0 {
				return nil, fmt.Errorf("ANIM %s: bad fps %q", c.Name, v)
			}
			c.FPS = x
		case "loop":
			c.Loop = isYes(v)
		default:
			return nil, fmt.Errorf("ANIM %s: unknown key %q", c.Name, k)
		}
	}
	if len(c.Frames) == 0 {
		return nil, fmt.Errorf("ANIM %s has no frames", c.Name)
	}
	return c, nil
}

func isYes(v string) bool {
	switch strings.ToLower(v) {
	case "yes", "true", "1", "tak":
		return true
	}
	return false
}

// NewSet builds a set from a descriptor; name(f, r) gives the texture name
// registered for frame f, rotation r.
func NewSet(d *Desc, name func(f, r int) string) *Set {
	s := &Set{Clips: map[string]*Clip{}}
	rot := d.Rotations
	for f := 0; f < d.Frames(); f++ {
		views := make([]string, rot)
		for r := range views {
			views[r] = name(f, r)
		}
		s.Tex = append(s.Tex, views)
	}
	for _, c := range d.Clips {
		s.Clips[c.Name] = c
	}
	return s
}

// ClassicAtlas is the clip layout of the engine's 3x2 enemy atlases (frames
// 0..5: idle 0-1, walk 1-2-3-2, attack 4-5; pain shows 4, death has none).
func ClassicAtlas() []*Clip {
	return []*Clip{
		{Name: Idle, Frames: []int{0, 1}, FPS: 1 / 0.45, Loop: true},
		{Name: Walk, Frames: []int{1, 2, 3, 2}, FPS: 1 / 0.14, Loop: true},
		{Name: Attack, Frames: []int{4, 5}, FPS: 1 / 0.12, Loop: true},
		{Name: Pain, Frames: []int{4}, FPS: 1 / 0.3, Loop: false},
	}
}
