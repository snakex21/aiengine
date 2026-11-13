package ui

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"aiengine/parser"
	"aiengine/renderer"
	"aiengine/settings"
)

// Input is an abstract menu input (keyboard, mouse wheel or gamepad).
type Input int

const (
	Up Input = iota
	Down
	Left
	Right
	Accept
	Back
)

// Settings rows generated for the "settings" screen, in display order.
var settingsRows = []string{
	"mouse_sensitivity", "invert_y", "master_volume", "music_volume",
	"sfx_volume", "background_volume", "resolution", "render_scale", "fullscreen",
	"fullscreen_mode", "fov",
}

// Default (English) labels for generated rows; menu files override them
// with LABEL <key> <text> in the screen.
var defaultLabels = map[string]string{
	"mouse_sensitivity": "Mouse sensitivity",
	"invert_y":          "Invert Y",
	"master_volume":     "Master volume",
	"music_volume":      "Music volume",
	"sfx_volume":        "Effects volume",
	"resolution":        "Resolution",
	"render_scale":      "Render scale",
	"native":            "Native",
	"fullscreen":        "Fullscreen",
	"fullscreen_mode":   "Fullscreen mode",
	"borderless":        "Borderless",
	"exclusive":         "Exclusive",
	"background_volume": "Volume in background",
	"fov":               "Field of view",
	"back":              "Back",
	"on":                "On",
	"off":               "Off",
}

var defaultTitles = map[string]string{"settings": "Settings", "level_select": "Select level", "pause": "Paused"}

// Menu is the runtime state of the menu system.
type Menu struct {
	Def      *Def
	Levels   []parser.LevelEntry
	Settings *settings.Settings
	BaseDir  string            // project directory (for background images)
	OnSound  func(name string) // optional: plays move/select/back sounds

	// Available hides items whose action is not possible right now (e.g.
	// "continue" without a saved game). nil = everything available.
	Available func(a Action) bool
	// LevelStatus describes a level in the generated level list: whether it
	// can be started and a suffix shown after its title (best time,
	// "locked"...). nil = all levels enabled, no suffix.
	LevelStatus func(id string) (enabled bool, suffix string)

	stack  []string
	focus  int
	images map[string]*image.RGBA
}

// New creates a menu over def. levels and s may be nil/empty.
func New(def *Def, levels []parser.LevelEntry, s *settings.Settings, baseDir string) *Menu {
	if def == nil {
		def = &Def{Screens: map[string]*Screen{}, Sounds: map[string]string{}, Style: DefaultStyle()}
	}
	return &Menu{Def: def, Levels: levels, Settings: s, BaseDir: baseDir, images: map[string]*image.RGBA{}}
}

// Has reports whether a screen can be opened (defined, or generated:
// settings and level_select always exist).
func (m *Menu) Has(name string) bool {
	return m.Def.Screens[name] != nil || name == "settings" || name == "level_select"
}

// Active reports whether a menu screen is shown.
func (m *Menu) Active() bool { return len(m.stack) > 0 }

// ScreenName returns the name of the visible screen ("" if none).
func (m *Menu) ScreenName() string {
	if len(m.stack) == 0 {
		return ""
	}
	return m.stack[len(m.stack)-1]
}

// Depth is the number of stacked screens.
func (m *Menu) Depth() int { return len(m.stack) }

// Focus is the index of the highlighted item.
func (m *Menu) Focus() int { return m.focus }

// OpenRoot shows a screen as the only one on the stack (title, pause).
func (m *Menu) OpenRoot(name string) {
	m.stack = m.stack[:0]
	m.push(name)
}

func (m *Menu) push(name string) {
	m.stack = append(m.stack, name)
	m.focus = 0
	m.fixFocus()
}

// Close hides all menu screens.
func (m *Menu) Close() {
	m.stack = m.stack[:0]
	m.focus = 0
}

// PausesGame reports whether gameplay must be frozen (any screen open).
func (m *Menu) PausesGame() bool { return m.Active() }

// Current returns the visible screen with generated rows filled in.
func (m *Menu) Current() *Screen {
	name := m.ScreenName()
	if name == "" {
		return nil
	}
	return m.resolve(name)
}

func (m *Menu) label(s *Screen, key string) string {
	if s != nil {
		if v, ok := s.Labels[key]; ok {
			return v
		}
	}
	return defaultLabels[key]
}

// resolve builds the effective screen: defined items, generated settings or
// level rows, and hides level_select when there are fewer than two levels.
func (m *Menu) resolve(name string) *Screen {
	src := m.Def.Screens[name]
	s := &Screen{Name: name, Labels: map[string]string{}, Background: Background{Kind: "dim"}}
	if src != nil {
		cp := *src
		s = &cp
		s.Items = nil
	}
	if s.Title == "" {
		s.Title = defaultTitles[name]
	}
	if src != nil {
		for _, it := range src.Items {
			if it.Action.Kind == "level_select" && len(m.Levels) < 2 {
				continue
			}
			if m.Available != nil && !m.Available(it.Action) {
				continue
			}
			s.Items = append(s.Items, it)
		}
	}
	switch name {
	case "settings":
		if len(s.Items) == 0 {
			for _, key := range settingsRows {
				s.Items = append(s.Items, Item{Label: m.label(src, key), Setting: key})
			}
			s.Items = append(s.Items, Item{Label: m.label(src, "back"), Action: Action{Kind: "back"}})
		}
	case "level_select":
		if len(s.Items) == 0 {
			for _, l := range m.Levels {
				it := Item{Label: l.Title, Action: Action{Kind: "load_level", Arg: l.ID}}
				if m.LevelStatus != nil {
					enabled, suffix := m.LevelStatus(l.ID)
					it.Disabled = !enabled
					if suffix != "" {
						it.Label += "  " + suffix
					}
				}
				s.Items = append(s.Items, it)
			}
			s.Items = append(s.Items, Item{Label: m.label(src, "back"), Action: Action{Kind: "back"}})
		}
	}
	return s
}

func (m *Menu) fixFocus() {
	s := m.Current()
	if s == nil || len(s.Items) == 0 {
		m.focus = 0
		return
	}
	if m.focus < 0 {
		m.focus = len(s.Items) - 1
	}
	if m.focus >= len(s.Items) {
		m.focus = 0
	}
}

func (m *Menu) sound(kind string) {
	if m.OnSound != nil {
		if name := m.Def.Sounds[kind]; name != "" {
			m.OnSound(name)
		}
	}
}

// Input applies a navigation input. It returns an action for the host when
// one must be carried out (ok = true). Screen changes inside the menu
// (submenus, back) are handled here and return no action.
func (m *Menu) Input(in Input) (Action, bool) {
	s := m.Current()
	if s == nil {
		return Action{}, false
	}
	switch in {
	case Up, Down:
		if len(s.Items) == 0 {
			return Action{}, false
		}
		if in == Up {
			m.focus--
		} else {
			m.focus++
		}
		m.fixFocus()
		m.sound("move")
		return Action{}, false
	case Back:
		return m.back(s)
	}
	if len(s.Items) == 0 {
		return Action{}, false
	}
	it := s.Items[m.focus]
	if it.Setting != "" {
		step := 1
		if in == Left {
			step = -1
		}
		if m.adjust(it.Setting, step) {
			m.sound("move")
			return Action{Kind: "settings_changed", Arg: it.Setting}, true
		}
		return Action{}, false
	}
	if in != Accept {
		return Action{}, false
	}
	if it.Disabled {
		m.sound("back")
		return Action{}, false
	}
	m.sound("select")
	return m.activate(it.Action)
}

func (m *Menu) back(s *Screen) (Action, bool) {
	if len(m.stack) > 1 {
		m.stack = m.stack[:len(m.stack)-1]
		m.focus = 0
		m.fixFocus()
		m.sound("back")
		return Action{}, false
	}
	if s.Escape.Kind != "" {
		m.sound("back")
		return m.activate(s.Escape)
	}
	return Action{}, false
}

// activate performs menu-internal actions and returns host actions.
func (m *Menu) activate(a Action) (Action, bool) {
	switch a.Kind {
	case "settings", "level_select":
		m.push(a.Kind)
		return Action{}, false
	case "menu":
		if m.Has(a.Arg) {
			m.push(a.Arg)
		}
		return Action{}, false
	case "back":
		return m.back(m.Current())
	case "main_menu":
		if m.Has("main") {
			m.OpenRoot("main")
		}
		return a, true
	case "resume", "new_game", "load_level", "restart", "continue":
		m.Close()
		return a, true
	}
	return a, true // quit, run_script
}

// adjust changes a setting by one step; reports whether it changed.
func (m *Menu) adjust(key string, step int) bool {
	s := m.Settings
	if s == nil {
		return false
	}
	before := *s
	f := float64(step)
	switch key {
	case "mouse_sensitivity":
		s.MouseSensitivity = round1(s.MouseSensitivity + 0.1*f)
	case "invert_y":
		s.InvertY = !s.InvertY
	case "master_volume":
		s.MasterVolume = round1(s.MasterVolume + 0.1*f)
	case "music_volume":
		s.MusicVolume = round1(s.MusicVolume + 0.1*f)
	case "sfx_volume":
		s.SfxVolume = round1(s.SfxVolume + 0.1*f)
	case "fullscreen":
		s.Fullscreen = !s.Fullscreen
	case "fullscreen_mode":
		s.Exclusive = !s.Exclusive
	case "background_volume":
		idx := 0
		for i, v := range settings.BackgroundVolumes {
			if v <= s.BackgroundVolume+1e-9 {
				idx = i
			}
		}
		n := len(settings.BackgroundVolumes)
		s.BackgroundVolume = settings.BackgroundVolumes[((idx+step)%n+n)%n]
	case "fov":
		s.FOV += 5 * f
	case "render_scale":
		idx := 0
		for i, v := range settings.RenderScales {
			if v <= s.RenderScale+1e-9 {
				idx = i
			}
		}
		n := len(settings.RenderScales)
		s.RenderScale = settings.RenderScales[((idx+step)%n+n)%n]
	case "resolution":
		idx := -1
		for i, r := range settings.Resolutions {
			if r[0] == s.Width && r[1] == s.Height {
				idx = i
			}
		}
		n := len(settings.Resolutions)
		idx = ((idx+step)%n + n) % n
		s.Width, s.Height = settings.Resolutions[idx][0], settings.Resolutions[idx][1]
	}
	s.Clamp()
	return *s != before
}

func round1(v float64) float64 { return float64(int(v*10+0.5*sign(v))) / 10 }

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// ValueText is the displayed value of a settings row.
func (m *Menu) ValueText(key string) string {
	s := m.Settings
	if s == nil {
		return ""
	}
	scr := m.Current()
	onOff := func(b bool) string {
		if b {
			return m.label(m.Def.Screens[scr.Name], "on")
		}
		return m.label(m.Def.Screens[scr.Name], "off")
	}
	switch key {
	case "mouse_sensitivity":
		return fmt.Sprintf("%.1f", s.MouseSensitivity)
	case "invert_y":
		return onOff(s.InvertY)
	case "master_volume":
		return fmt.Sprintf("%d%%", int(s.MasterVolume*100+0.5))
	case "music_volume":
		return fmt.Sprintf("%d%%", int(s.MusicVolume*100+0.5))
	case "sfx_volume":
		return fmt.Sprintf("%d%%", int(s.SfxVolume*100+0.5))
	case "resolution":
		if s.Native() {
			return m.label(m.Def.Screens[scr.Name], "native")
		}
		return fmt.Sprintf("%dx%d", s.Width, s.Height)
	case "render_scale":
		return fmt.Sprintf("%.0f%%", s.RenderScale*100)
	case "fullscreen":
		return onOff(s.Fullscreen)
	case "fullscreen_mode":
		if s.Exclusive {
			return m.label(m.Def.Screens[scr.Name], "exclusive")
		}
		return m.label(m.Def.Screens[scr.Name], "borderless")
	case "background_volume":
		return fmt.Sprintf("%d%%", int(s.BackgroundVolume*100+0.5))
	case "fov":
		return fmt.Sprintf("%.0f", s.FOV)
	}
	return ""
}

// --- Layout, mouse and drawing ---

// Rect is a screen rectangle in UI-layer pixels.
type Rect struct{ X, Y, W, H int }

func (r Rect) contains(x, y int) bool { return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H }

// Layout holds positions for the current screen at a given resolution.
type Layout struct {
	Unit       int // base pixel scale (1 at 360p)
	TitleScale int
	ItemScale  int
	TitleY     int
	SubtitleY  int
	Items      []Rect
	HintY      int
}

// rowUnits is the height of a text row in font pixels (body + accents + gap).
const rowUnits = renderer.FontH + 2*renderer.FontAccentRows + 3

// Layout computes where everything goes on a sw×sh UI layer.
func (m *Menu) Layout(sw, sh int) Layout {
	s := m.Current()
	l := Layout{Unit: max(1, sh/360)}
	l.TitleScale = 4 * l.Unit
	l.TitleY = sh * 16 / 100
	l.SubtitleY = l.TitleY + (renderer.FontH+2*renderer.FontAccentRows)*l.TitleScale
	if s == nil {
		return l
	}
	top := sh * 40 / 100
	avail := sh*88/100 - top
	n := max(1, len(s.Items))
	l.ItemScale = max(1, min(2*l.Unit, avail/(n*rowUnits)))
	rowH := rowUnits * l.ItemScale
	w := sw * 2 / 3
	if s.Name != "settings" {
		w = sw / 2
	}
	for i := range s.Items {
		l.Items = append(l.Items, Rect{X: (sw - w) / 2, Y: top + i*rowH, W: w, H: rowH})
	}
	l.HintY = sh - (renderer.FontH+6)*l.Unit*2
	return l
}

// Hover moves the focus to the item under (x, y), if any.
func (m *Menu) Hover(x, y, sw, sh int) {
	for i, r := range m.Layout(sw, sh).Items {
		if r.contains(x, y) && i != m.focus {
			m.focus = i
			m.sound("move")
			return
		}
	}
}

// Click activates the item under (x, y). Right-clicking a settings row
// steps its value backwards.
func (m *Menu) Click(x, y, sw, sh int, right bool) (Action, bool) {
	for i, r := range m.Layout(sw, sh).Items {
		if r.contains(x, y) {
			m.focus = i
			s := m.Current()
			if s.Items[i].Setting != "" {
				if right {
					return m.Input(Left)
				}
				return m.Input(Right)
			}
			if right {
				return Action{}, false
			}
			return m.Input(Accept)
		}
	}
	return Action{}, false
}

// Draw renders the current screen into an RGBA UI layer.
func (m *Menu) Draw(buf []uint8, sw, sh int) {
	s := m.Current()
	if s == nil {
		return
	}
	st := m.Def.Style
	switch s.Background.Kind {
	case "color":
		renderer.FillRect(buf, sw, sh, 0, 0, sw, sh, s.Background.Color)
	case "image":
		if !m.blitImage(buf, sw, sh, s.Background.Image) {
			renderer.FillRect(buf, sw, sh, 0, 0, sw, sh, [4]uint8{0, 0, 0, 255})
		}
	case "dim":
		renderer.FillRect(buf, sw, sh, 0, 0, sw, sh, st.Dim)
	case "map":
		dim := st.Dim
		dim[3] /= 2
		renderer.FillRect(buf, sw, sh, 0, 0, sw, sh, dim)
	}
	l := m.Layout(sw, sh)
	centred := func(text string, y, scale int, col [4]uint8) {
		renderer.DrawTextScaled(buf, sw, sh, (sw-renderer.TextWidth(text, scale))/2, y, text, col, scale)
	}
	if s.Title != "" {
		centred(s.Title, l.TitleY, l.TitleScale, st.Title)
	}
	if s.Subtitle != "" {
		centred(s.Subtitle, l.SubtitleY, l.Unit+1, st.Text)
	}
	pad := 2 * l.ItemScale
	for i, it := range s.Items {
		r := l.Items[i]
		col := st.Text
		if it.Disabled {
			col = st.Hint
		}
		focused := i == m.focus
		if focused {
			renderer.FillRect(buf, sw, sh, r.X, r.Y, r.W, r.H-pad, st.FocusBG)
			col = st.Focus
		}
		textY := r.Y + (r.H-pad-renderer.FontH*l.ItemScale)/2
		if it.Setting != "" {
			renderer.DrawTextScaled(buf, sw, sh, r.X+3*pad, textY, it.Label, col, l.ItemScale)
			val := m.ValueText(it.Setting)
			if focused {
				val = "< " + val + " >"
			}
			renderer.DrawTextScaled(buf, sw, sh, r.X+r.W-3*pad-renderer.TextWidth(val, l.ItemScale), textY, val, col, l.ItemScale)
			continue
		}
		label := it.Label
		if focused {
			label = "> " + label + " <"
		}
		renderer.DrawTextScaled(buf, sw, sh, (sw-renderer.TextWidth(label, l.ItemScale))/2, textY, label, col, l.ItemScale)
	}
	if s.Hint != "" {
		centred(s.Hint, l.HintY, l.Unit, st.Hint)
	}
}

// blitImage scales a PNG (cached) over the whole layer, nearest-neighbour.
func (m *Menu) blitImage(buf []uint8, sw, sh int, rel string) bool {
	img, ok := m.images[rel]
	if !ok {
		img = loadRGBA(filepath.Join(m.BaseDir, filepath.FromSlash(rel)))
		m.images[rel] = img
	}
	if img == nil {
		return false
	}
	b := img.Bounds()
	for y := 0; y < sh; y++ {
		sy := b.Min.Y + y*b.Dy()/sh
		for x := 0; x < sw; x++ {
			sx := b.Min.X + x*b.Dx()/sw
			si := img.PixOffset(sx, sy)
			di := (y*sw + x) * 4
			copy(buf[di:di+3], img.Pix[si:si+3])
			buf[di+3] = 255
		}
	}
	return true
}

func loadRGBA(path string) *image.RGBA {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil
	}
	dst := image.NewRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst
}

// Describe returns a one-line text summary of the visible screen (for logs
// and tests): "screen: [item] item ...", focus in brackets.
func (m *Menu) Describe() string {
	s := m.Current()
	if s == nil {
		return "(closed)"
	}
	parts := make([]string, len(s.Items))
	for i, it := range s.Items {
		text := it.Label
		if it.Setting != "" {
			text += "=" + m.ValueText(it.Setting)
		}
		if i == m.focus {
			text = "[" + text + "]"
		}
		parts[i] = text
	}
	return s.Name + ": " + strings.Join(parts, " | ")
}
