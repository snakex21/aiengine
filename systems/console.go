// systems/console.go — Built‑in cheat console and command registry.
// Tilde (~) opens the console overlay; F1 opens the GUI panel.
// Cheats are always available but never active until the player
// explicitly types the command.

package systems

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

// CheatFlag enumerates cheat toggles tracked by the engine.
type CheatFlag int

const (
	CheatGod    CheatFlag = iota // invulnerability
	CheatNoclip                  // walk through walls
	CheatFPS                     // show FPS counter
)

func (c CheatFlag) String() string {
	switch c {
	case CheatGod:
		return "god"
	case CheatNoclip:
		return "noclip"
	case CheatFPS:
		return "fps"
	default:
		return "?"
	}
}

// ConsoleState holds the console input buffer, output history, and cheat flags.
type ConsoleState struct {
	Open    bool
	Input   string
	History []string
	Cursor  int // history navigation position

	Cheats map[CheatFlag]bool

	// Callbacks – set by main to mutate game state.
	OnCommand func(cmd string, args []string) string
}

// NewConsoleState returns a fresh ConsoleState.
func NewConsoleState() *ConsoleState {
	return &ConsoleState{
		Cheats:  make(map[CheatFlag]bool),
		History: make([]string, 0, 64),
	}
}

// Toggle opens or closes the console.
func (cs *ConsoleState) Toggle() {
	cs.Open = !cs.Open
	cs.Input = ""
	cs.Cursor = -1
	if cs.Open {
		log.Println("[Console] opened")
	}
}

// HandleChar appends a printable character to the input buffer.
func (cs *ConsoleState) HandleChar(ch rune) {
	cs.Input += string(ch)
	cs.Cursor = -1
}

// Backspace removes the last character.
func (cs *ConsoleState) Backspace() {
	if len(cs.Input) > 0 {
		cs.Input = cs.Input[:len(cs.Input)-1]
	}
}

// HistoryUp navigates to older history entries.
func (cs *ConsoleState) HistoryUp() {
	if len(cs.History) == 0 {
		return
	}
	if cs.Cursor == -1 {
		cs.Cursor = len(cs.History) - 1
	} else if cs.Cursor > 0 {
		cs.Cursor--
	} else {
		cs.Cursor = len(cs.History) - 1
	}
	cs.Input = cs.History[cs.Cursor]
}

// HistoryDown navigates to newer history entries.
func (cs *ConsoleState) HistoryDown() {
	if cs.Cursor == -1 {
		return
	}
	if cs.Cursor < len(cs.History)-1 {
		cs.Cursor++
		cs.Input = cs.History[cs.Cursor]
	} else {
		cs.Cursor = -1
		cs.Input = ""
	}
}

// Submit executes the current input and appends it to history.
func (cs *ConsoleState) Submit() string {
	line := strings.TrimSpace(cs.Input)
	if line == "" {
		return ""
	}
	cs.History = append(cs.History, line)
	cs.Input = ""
	cs.Cursor = -1

	result := ExecuteCommand(cs, line)
	log.Printf("[Console] > %s  →  %s", line, result)
	return result
}

// ExecuteCommand parses and runs a cheat command.  Returns the result message.
func ExecuteCommand(cs *ConsoleState, line string) string {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return ""
	}
	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "god":
		cs.Cheats[CheatGod] = !cs.Cheats[CheatGod]
		if cs.Cheats[CheatGod] {
			return "god mode ON"
		}
		return "god mode OFF"

	case "noclip":
		cs.Cheats[CheatNoclip] = !cs.Cheats[CheatNoclip]
		if cs.Cheats[CheatNoclip] {
			return "noclip ON"
		}
		return "noclip OFF"

	case "fps":
		cs.Cheats[CheatFPS] = !cs.Cheats[CheatFPS]
		if cs.Cheats[CheatFPS] {
			return "fps counter ON"
		}
		return "fps counter OFF"

	case "speed":
		if cs.OnCommand != nil {
			return cs.OnCommand("speed", args)
		}
		return "speed [multiplier]"

	case "spawn":
		if cs.OnCommand != nil {
			return cs.OnCommand("spawn", args)
		}
		return "spawn <entity> [x] [y]"

	case "give":
		if cs.OnCommand != nil {
			return cs.OnCommand("give", args)
		}
		return "give <weapon|item>"

	case "kill":
		if cs.OnCommand != nil {
			return cs.OnCommand("kill", args)
		}
		return "kill [target]"

	case "tp":
		if cs.OnCommand != nil {
			return cs.OnCommand("tp", args)
		}
		return "tp <x> <y>"

	case "reload":
		if cs.OnCommand != nil {
			return cs.OnCommand("reload", args)
		}
		return "weapon reloaded"

	case "map":
		if cs.OnCommand != nil {
			return cs.OnCommand("map", args)
		}
		return "map <name>"

	case "help":
		return "god noclip fps speed spawn give kill tp reload map help clear hltest"

	case "clear":
		cs.History = cs.History[:0]
		return ""

	default:
		// Try external callback.
		if cs.OnCommand != nil {
			return cs.OnCommand(cmd, args)
		}
		return fmt.Sprintf("unknown command: %s", cmd)
	}
}

// CheatEnabled returns true when a cheat flag is active.
func (cs *ConsoleState) CheatEnabled(flag CheatFlag) bool {
	return cs.Cheats[flag]
}

// --- helper for arg parsing ---

// ArgFloat parses the first argument as float64, returns defaultValue on failure.
func ArgFloat(args []string, idx int, defaultVal float64) float64 {
	if idx >= len(args) {
		return defaultVal
	}
	v, err := strconv.ParseFloat(args[idx], 64)
	if err != nil {
		return defaultVal
	}
	return v
}

// ArgInt parses the first argument as int, returns defaultValue on failure.
func ArgInt(args []string, idx int, defaultVal int) int {
	if idx >= len(args) {
		return defaultVal
	}
	v, err := strconv.Atoi(args[idx])
	if err != nil {
		return defaultVal
	}
	return v
}

// ArgString returns args[idx] or "" if out of bounds.
func ArgString(args []string, idx int) string {
	if idx >= len(args) {
		return ""
	}
	return args[idx]
}
