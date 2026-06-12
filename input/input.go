// Package input handles keyboard, mouse, and action binding for AIEngine.
// Keybindings are loaded from a config.aie file (BIND lines).
// The Manager tracks per-action state and exposes IsPressed / IsJustPressed / IsJustReleased.
// No hardcoded keybindings exist — everything is driven by config.aie or defaults.

package input

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-gl/glfw/v3.3/glfw"
)

// ActionState records the current and previous frame state for a single action.
type actionState struct {
	held     bool // true while the key/button is physically down
	previous bool // held value from last frame
}

// Manager owns the keybinding table and per-action state.
type Manager struct {
	// bindings maps action name → GLFW key/button.
	// Key bindings use positive Key values; mouse bindings use negative values
	// to distinguish them (e.g. -1 = MouseButton1).
	bindings map[string]int32

	// state tracks current+previous for each known action.
	state map[string]*actionState

	// keyToAction is a reverse map: GLFW key → action name (for fast lookup on key events).
	keyToAction map[glfw.Key]string
	// mouseToAction is a reverse map: GLFW mouse button → action name.
	mouseToAction map[glfw.MouseButton]string

	// cursor position (for future mouse-look).
	cursorX, cursorY float64
}

// NewManager allocates and returns an empty input Manager.
func NewManager() *Manager {
	return &Manager{
		bindings:      make(map[string]int32),
		state:         make(map[string]*actionState),
		keyToAction:   make(map[glfw.Key]string),
		mouseToAction: make(map[glfw.MouseButton]string),
	}
}

// LoadConfig reads a config.aie file and populates keybindings.
// Expected format: BIND <action_name> <key_name>
// Lines starting with # are comments; empty lines are ignored.
func (m *Manager) LoadConfig(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if !strings.HasPrefix(line, "BIND ") {
			continue // not a BIND line; silently skip
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			log.Printf("config.aie:%d: malformed BIND line: %q", lineNo, line)
			continue
		}

		actionName := fields[1]
		bindingStr := fields[2]

		code, ok := parseBinding(bindingStr)
		if !ok {
			log.Printf("config.aie:%d: unknown key/button: %q", lineNo, bindingStr)
			continue
		}

		m.bindings[actionName] = code
		m.ensureState(actionName)

		// Populate reverse maps for fast event lookup.
		if code > 0 {
			m.keyToAction[glfw.Key(code)] = actionName
		} else {
			// Mouse buttons: code is negative GLFW mouse button constant.
			m.mouseToAction[glfw.MouseButton(-code)] = actionName
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	log.Printf("Loaded %d keybindings from %s", len(m.bindings), path)
	return nil
}

// SetDefaults applies the built-in fallback bindings when no config.aie is found.
func (m *Manager) SetDefaults() {
	defaults := []struct {
		action string
		key    glfw.Key
	}{
		{"move_forward", glfw.KeyW},
		{"move_backward", glfw.KeyS},
		{"move_left", glfw.KeyA},
		{"move_right", glfw.KeyD},
		{"shoot", glfw.KeyLeftControl}, // represented as MOUSE_LEFT in config, but GLFW mouse is separate
		{"reload", glfw.KeyR},
		{"fullscreen", glfw.KeyF11},
		{"pause", glfw.KeyEscape},
		{"interact", glfw.KeyE},
		{"jump", glfw.KeySpace},
		{"crouch", glfw.KeyC},
		{"prone", glfw.KeyZ},
		{"sprint", glfw.KeyLeftShift},
		{"next_weapon", glfw.KeyTab},
		{"use_item", glfw.KeyF},
	}

	for _, d := range defaults {
		code := int32(d.key)
		m.bindings[d.action] = code
		m.ensureState(d.action)
		m.keyToAction[d.key] = d.action
	}

	// Default mouse bindings.
	mouseDefaults := []struct {
		action string
		button glfw.MouseButton
	}{
		{"shoot", glfw.MouseButton1},
		{"alt_attack", glfw.MouseButton2},
	}

	for _, md := range mouseDefaults {
		code := int32(-md.button)
		m.bindings[md.action] = code
		m.ensureState(md.action)
		m.mouseToAction[md.button] = md.action
	}

	log.Printf("Using %d default keybindings", len(m.bindings))
}

// ensureState creates a state entry for an action if it does not exist.
func (m *Manager) ensureState(action string) {
	if _, ok := m.state[action]; !ok {
		m.state[action] = &actionState{}
	}
}

// Update must be called once per frame to shift previous/current states.
func (m *Manager) Update() {
	for _, st := range m.state {
		st.previous = st.held
	}
}

// IsPressed returns true while the action's binding is held down.
func (m *Manager) IsPressed(action string) bool {
	st, ok := m.state[action]
	if !ok {
		return false
	}
	return st.held
}

// IsJustPressed returns true only on the first frame the action's binding is pressed.
func (m *Manager) IsJustPressed(action string) bool {
	st, ok := m.state[action]
	if !ok {
		return false
	}
	return st.held && !st.previous
}

// IsJustReleased returns true only on the frame the action's binding is released.
func (m *Manager) IsJustReleased(action string) bool {
	st, ok := m.state[action]
	if !ok {
		return false
	}
	return !st.held && st.previous
}

// CursorPos returns the current mouse cursor position.
func (m *Manager) CursorPos() (float64, float64) {
	return m.cursorX, m.cursorY
}

// --- GLFW event handlers ---

// OnKeyEvent is called from the GLFW key callback.
// It updates the internal state for any action bound to this key.
func (m *Manager) OnKeyEvent(key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	_ = scancode
	_ = mods

	actionName, bound := m.keyToAction[key]
	if !bound {
		return
	}

	st := m.state[actionName] // guaranteed to exist because we create state on bind

	switch action {
	case glfw.Press, glfw.Repeat:
		st.held = true
	case glfw.Release:
		st.held = false
	}
}

// OnMouseButtonEvent is called from the GLFW mouse button callback.
func (m *Manager) OnMouseButtonEvent(button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
	_ = mods

	actionName, bound := m.mouseToAction[button]
	if !bound {
		return
	}

	st := m.state[actionName]

	switch action {
	case glfw.Press:
		st.held = true
	case glfw.Release:
		st.held = false
	}
}

// OnCursorPos is called from the GLFW cursor position callback.
func (m *Manager) OnCursorPos(xpos, ypos float64) {
	m.cursorX = xpos
	m.cursorY = ypos
}

// Shutdown releases any resources held by the input manager.
func (m *Manager) Shutdown() {
	// Currently no resources to release; placeholder for future.
}

// --- Key name parsing ---

// parseBinding converts a config.aie binding string (e.g. "W", "MOUSE_LEFT", "ESCAPE")
// to an internal code. Returns the code and true on success.
// Positive values = GLFW Key, negative values = GLFW MouseButton (negated).
func parseBinding(name string) (int32, bool) {
	upper := strings.ToUpper(name)

	// Mouse buttons first.
	switch upper {
	case "MOUSE_LEFT", "MOUSE1":
		return int32(-glfw.MouseButton1), true
	case "MOUSE_RIGHT", "MOUSE2":
		return int32(-glfw.MouseButton2), true
	case "MOUSE_MIDDLE", "MOUSE3":
		return int32(-glfw.MouseButton3), true
	case "MOUSE4":
		return int32(-glfw.MouseButton4), true
	case "MOUSE5":
		return int32(-glfw.MouseButton5), true
	}

	// Keyboard keys.
	key, ok := keyNameToGLFW(upper)
	return int32(key), ok
}

// keyNameToGLFW maps human-readable key names (matching config.aie syntax)
// to GLFW key constants.
func keyNameToGLFW(name string) (glfw.Key, bool) {
	switch name {
	case "A":
		return glfw.KeyA, true
	case "B":
		return glfw.KeyB, true
	case "C":
		return glfw.KeyC, true
	case "D":
		return glfw.KeyD, true
	case "E":
		return glfw.KeyE, true
	case "F":
		return glfw.KeyF, true
	case "G":
		return glfw.KeyG, true
	case "H":
		return glfw.KeyH, true
	case "I":
		return glfw.KeyI, true
	case "J":
		return glfw.KeyJ, true
	case "K":
		return glfw.KeyK, true
	case "L":
		return glfw.KeyL, true
	case "M":
		return glfw.KeyM, true
	case "N":
		return glfw.KeyN, true
	case "O":
		return glfw.KeyO, true
	case "P":
		return glfw.KeyP, true
	case "Q":
		return glfw.KeyQ, true
	case "R":
		return glfw.KeyR, true
	case "S":
		return glfw.KeyS, true
	case "T":
		return glfw.KeyT, true
	case "U":
		return glfw.KeyU, true
	case "V":
		return glfw.KeyV, true
	case "W":
		return glfw.KeyW, true
	case "X":
		return glfw.KeyX, true
	case "Y":
		return glfw.KeyY, true
	case "Z":
		return glfw.KeyZ, true

	case "0":
		return glfw.Key0, true
	case "1":
		return glfw.Key1, true
	case "2":
		return glfw.Key2, true
	case "3":
		return glfw.Key3, true
	case "4":
		return glfw.Key4, true
	case "5":
		return glfw.Key5, true
	case "6":
		return glfw.Key6, true
	case "7":
		return glfw.Key7, true
	case "8":
		return glfw.Key8, true
	case "9":
		return glfw.Key9, true

	case "F1":
		return glfw.KeyF1, true
	case "F2":
		return glfw.KeyF2, true
	case "F3":
		return glfw.KeyF3, true
	case "F4":
		return glfw.KeyF4, true
	case "F5":
		return glfw.KeyF5, true
	case "F6":
		return glfw.KeyF6, true
	case "F7":
		return glfw.KeyF7, true
	case "F8":
		return glfw.KeyF8, true
	case "F9":
		return glfw.KeyF9, true
	case "F10":
		return glfw.KeyF10, true
	case "F11":
		return glfw.KeyF11, true
	case "F12":
		return glfw.KeyF12, true

	case "ESCAPE", "ESC":
		return glfw.KeyEscape, true
	case "SPACE":
		return glfw.KeySpace, true
	case "ENTER", "RETURN":
		return glfw.KeyEnter, true
	case "TAB":
		return glfw.KeyTab, true
	case "BACKSPACE":
		return glfw.KeyBackspace, true
	case "DELETE":
		return glfw.KeyDelete, true
	case "INSERT":
		return glfw.KeyInsert, true
	case "HOME":
		return glfw.KeyHome, true
	case "END":
		return glfw.KeyEnd, true
	case "PAGE_UP", "PAGEUP":
		return glfw.KeyPageUp, true
	case "PAGE_DOWN", "PAGEDOWN":
		return glfw.KeyPageDown, true

	case "LEFT_SHIFT", "LSHIFT":
		return glfw.KeyLeftShift, true
	case "RIGHT_SHIFT", "RSHIFT":
		return glfw.KeyRightShift, true
	case "LEFT_CONTROL", "LCTRL":
		return glfw.KeyLeftControl, true
	case "RIGHT_CONTROL", "RCTRL":
		return glfw.KeyRightControl, true
	case "LEFT_ALT", "LALT":
		return glfw.KeyLeftAlt, true
	case "RIGHT_ALT", "RALT":
		return glfw.KeyRightAlt, true
	case "LEFT_SUPER", "LSUPER":
		return glfw.KeyLeftSuper, true
	case "RIGHT_SUPER", "RSUPER":
		return glfw.KeyRightSuper, true

	case "UP", "ARROW_UP":
		return glfw.KeyUp, true
	case "DOWN", "ARROW_DOWN":
		return glfw.KeyDown, true
	case "LEFT", "ARROW_LEFT":
		return glfw.KeyLeft, true
	case "RIGHT", "ARROW_RIGHT":
		return glfw.KeyRight, true

	case "CAPS_LOCK", "CAPSLOCK":
		return glfw.KeyCapsLock, true
	case "NUM_LOCK", "NUMLOCK":
		return glfw.KeyNumLock, true
	case "SCROLL_LOCK", "SCROLLLOCK":
		return glfw.KeyScrollLock, true
	case "PRINT_SCREEN", "PRINTSCREEN":
		return glfw.KeyPrintScreen, true
	case "PAUSE":
		return glfw.KeyPause, true

	case "KP_0":
		return glfw.KeyKP0, true
	case "KP_1":
		return glfw.KeyKP1, true
	case "KP_2":
		return glfw.KeyKP2, true
	case "KP_3":
		return glfw.KeyKP3, true
	case "KP_4":
		return glfw.KeyKP4, true
	case "KP_5":
		return glfw.KeyKP5, true
	case "KP_6":
		return glfw.KeyKP6, true
	case "KP_7":
		return glfw.KeyKP7, true
	case "KP_8":
		return glfw.KeyKP8, true
	case "KP_9":
		return glfw.KeyKP9, true
	case "KP_ADD":
		return glfw.KeyKPAdd, true
	case "KP_SUBTRACT":
		return glfw.KeyKPSubtract, true
	case "KP_MULTIPLY":
		return glfw.KeyKPMultiply, true
	case "KP_DIVIDE":
		return glfw.KeyKPDivide, true
	case "KP_DECIMAL":
		return glfw.KeyKPDecimal, true
	case "KP_ENTER":
		return glfw.KeyKPEnter, true

	case "MINUS":
		return glfw.KeyMinus, true
	case "EQUAL":
		return glfw.KeyEqual, true
	case "LEFT_BRACKET":
		return glfw.KeyLeftBracket, true
	case "RIGHT_BRACKET":
		return glfw.KeyRightBracket, true
	case "BACKSLASH":
		return glfw.KeyBackslash, true
	case "SEMICOLON":
		return glfw.KeySemicolon, true
	case "APOSTROPHE":
		return glfw.KeyApostrophe, true
	case "COMMA":
		return glfw.KeyComma, true
	case "PERIOD":
		return glfw.KeyPeriod, true
	case "SLASH":
		return glfw.KeySlash, true
	case "GRAVE_ACCENT":
		return glfw.KeyGraveAccent, true

	default:
		return 0, false
	}
}
