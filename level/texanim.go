package level

// Animated textures and per-cell textures of grid maps.
//
//	TEXTURE_ANIM woda frames=woda_0,woda_1,woda_2,woda_3 fps=6 scroll=0,0.15
//	TEXTURE_ANIM runy frames=runy_0,runy_1 fps=2
//	TEXTURE_ANIM lawa scroll=0.2,0.05     # scrolling only (no frames)
//
// The animation is bound to its name: any surface using that texture name
// (a sector's floor_tex, a wall symbol palette entry, FLOOR_TEX, ...) shows
// the current frame. fps = frames per second, scroll = u,v texture repeats
// per second. The same lines may sit in a texture descriptor file
// (assets/textures/textures.aie, or textures.aie in the level's TEXTURES
// directory) to animate a texture for every level of a project.
//
// Grid maps choose textures per cell with a palette and up to three layers
// (one symbol per GRID cell, '.' = the map default):
//
//	TEX_SYMBOLS w=woda l=lawa r=runy
//	FLOOR_TEX_GRID     # floor of open cells
//	CEILING_TEX_GRID   # ceiling of open cells
//	WALL_TEX_GRID      # walls of solid cells and step faces of open cells

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"aiengine/parser"
	"aiengine/renderer"
)

// ParseTexAnim parses the text after TEXTURE_ANIM.
func ParseTexAnim(spec string) (renderer.TexAnim, error) {
	var a renderer.TexAnim
	for _, f := range strings.Fields(spec) {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			if a.Name != "" {
				return a, fmt.Errorf("TEXTURE_ANIM: unexpected %q", f)
			}
			a.Name = f
			continue
		}
		switch strings.ToLower(k) {
		case "frames":
			for _, n := range strings.Split(v, ",") {
				if n = strings.TrimSpace(n); n != "" {
					a.Frames = append(a.Frames, n)
				}
			}
		case "fps":
			x, err := strconv.ParseFloat(v, 64)
			if err != nil || x < 0 {
				return a, fmt.Errorf("TEXTURE_ANIM %s: bad fps %q", a.Name, v)
			}
			a.FPS = x
		case "scroll":
			u, w, _ := strings.Cut(v, ",")
			var err1, err2 error
			a.ScrollU, err1 = strconv.ParseFloat(strings.TrimSpace(u), 64)
			if w != "" {
				a.ScrollV, err2 = strconv.ParseFloat(strings.TrimSpace(w), 64)
			}
			if err1 != nil || err2 != nil {
				return a, fmt.Errorf("TEXTURE_ANIM %s: bad scroll %q (u,v)", a.Name, v)
			}
		default:
			return a, fmt.Errorf("TEXTURE_ANIM %s: unknown key %q", a.Name, k)
		}
	}
	if a.Name == "" {
		return a, fmt.Errorf("TEXTURE_ANIM needs a texture name")
	}
	if len(a.Frames) > 0 && a.FPS == 0 {
		a.FPS = 8
	}
	return a, nil
}

// FormatTexAnim writes a TEXTURE_ANIM spec back (round trip).
func FormatTexAnim(a renderer.TexAnim) string {
	var b strings.Builder
	b.WriteString(a.Name)
	if len(a.Frames) > 0 {
		fmt.Fprintf(&b, " frames=%s fps=%g", strings.Join(a.Frames, ","), a.FPS)
	}
	if a.ScrollU != 0 || a.ScrollV != 0 {
		fmt.Fprintf(&b, " scroll=%g,%g", a.ScrollU, a.ScrollV)
	}
	return b.String()
}

// LoadTexAnimFile reads the TEXTURE_ANIM lines of a texture descriptor file
// (other lines and # comments are ignored). A missing file is no error.
func LoadTexAnimFile(path string) ([]renderer.TexAnim, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []renderer.TexAnim
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		t := strings.TrimSpace(sc.Text())
		rest, ok := strings.CutPrefix(t, "TEXTURE_ANIM ")
		if !ok {
			continue
		}
		a, err := ParseTexAnim(rest)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		out = append(out, a)
	}
	return out, sc.Err()
}

// buildTexLayers reads TEX_SYMBOLS and the FLOOR/CEILING/WALL_TEX_GRID
// layers of a grid map.
func (lv *Level) buildTexLayers(mapNode *parser.AIENode) error {
	layers := []struct {
		block string
		dst   *[]uint8
	}{
		{"FLOOR_TEX_GRID", &lv.Map.FloorTex}, {"CEILING_TEX_GRID", &lv.Map.CeilTex}, {"WALL_TEX_GRID", &lv.Map.WallTex},
	}
	any := false
	for _, l := range layers {
		any = any || mapNode.FindChild(l.block) != nil
	}
	if !any {
		return nil
	}
	m := lv.Map
	m.TexNames = []string{""}
	index := map[string]uint8{}
	for _, f := range strings.Fields(lv.Props["TEX_SYMBOLS"]) {
		k, v, ok := strings.Cut(f, "=")
		if !ok || len([]rune(k)) != 1 || v == "" {
			return fmt.Errorf("TEX_SYMBOLS: expected symbol=texture, got %q", f)
		}
		if len(m.TexNames) >= 255 {
			return fmt.Errorf("TEX_SYMBOLS: too many textures")
		}
		index[k] = uint8(len(m.TexNames))
		m.TexNames = append(m.TexNames, v)
	}
	for _, l := range layers {
		n := mapNode.FindChild(l.block)
		if n == nil {
			continue
		}
		dst := make([]uint8, m.Width*m.Height)
		for y, row := range n.Values {
			if y >= m.Height {
				return fmt.Errorf("line %d: %s has more rows than GRID", n.Line, l.block)
			}
			for x, sym := range strings.Fields(row) {
				if x >= m.Width {
					return fmt.Errorf("line %d: %s row %d is wider than GRID", n.Line, l.block, y+1)
				}
				if sym == "." {
					continue
				}
				i, ok := index[sym]
				if !ok {
					return fmt.Errorf("line %d: %s: symbol %q is not in TEX_SYMBOLS", n.Line, l.block, sym)
				}
				dst[y*m.Width+x] = i
			}
		}
		*l.dst = dst
	}
	return nil
}

// TextureNames lists the texture names the level refers to beyond the
// standard set (sector textures, palette entries, animation frames), so the
// host can load <dir>/<name>.png for each.
func (lv *Level) TextureNames() []string {
	seen := map[string]bool{}
	var out []string
	add := func(n string) {
		if n == "" || seen[n] || len([]rune(n)) == 1 {
			return
		}
		seen[n] = true
		out = append(out, n)
	}
	if lv.Map != nil {
		for _, n := range lv.Map.TexNames {
			add(n)
		}
	}
	if lv.Sectors != nil {
		for _, s := range lv.Sectors.Sectors {
			add(s.FloorTex)
			add(s.CeilTex)
			add(s.StepTex)
		}
		for _, l := range lv.Sectors.Lines {
			add(l.Tex)
		}
	}
	for _, a := range lv.TexAnims {
		add(a.Name)
		for _, f := range a.Frames {
			add(f)
		}
	}
	return out
}
