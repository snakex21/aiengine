// gameplay/cutscenes.go — Cutscene system for AIEngine.
// Supports 4 complexity levels: simple text, text+image, engine animations, full Lua control.
// Cutscenes are defined in .aie CUTSCENE blocks and executed step-by-step.
// Integrates with camera, entity spawning, and the Lua bridge.

package gameplay

import (
	"fmt"
	"log"
	"strings"
	"time"

	"aiengine/parser"
)

// CutsceneCommand represents a single step in a cutscene timeline.
type CutsceneCommand struct {
	Type     string // CAMERA_MOVE, TEXT, SPAWN, WAIT, etc.
	Params   []string
	FloatParams []float64
	Duration float64
	Executed bool
}

// Cutscene is a sequence of commands loaded from .aie.
type Cutscene struct {
	Name     string
	Commands []*CutsceneCommand
	Level    int // 1-4 complexity level

	// Runtime state.
	CurrentCmd  int
	Timer       float64
	Playing     bool
	Paused      bool
	WaitForKey  bool
	KeyPressed  bool
	StartedAt   time.Time
}

// CutscenePlayer manages cutscene playback.
type CutscenePlayer struct {
	cutscenes map[string]*Cutscene
	active    *Cutscene

	// Callbacks for engine integration.
	OnCameraMove     func(x, y, z, duration float64)
	OnCameraLook     func(x, y, z, duration float64)
	OnSpawn          func(typ string, x, y, z float64)
	OnAnimate        func(id, anim string)
	OnShakeCamera    func(intensity, duration float64)
	OnFreezePlayer   func()
	OnUnfreezePlayer func()
	OnShowText       func(text string)
	OnShowImage      func(file string)
	OnPlaySound      func(file string)
	OnPlayMusic      func(file string)
	OnFadeIn         func(duration float64)
	OnFadeOut        func(duration float64)
	OnLuaFunction    func(funcName string)

	// Player state.
	playerFrozen bool
}

// NewCutscenePlayer creates a cutscene player.
func NewCutscenePlayer() *CutscenePlayer {
	return &CutscenePlayer{
		cutscenes: make(map[string]*Cutscene),
	}
}

// LoadCutscene parses a .aie CUTSCENE node and stores it.
func (cp *CutscenePlayer) LoadCutscene(node *parser.AIENode) {
	cs := &Cutscene{
		Name: node.Name,
	}

	// Determine level based on command types.
	hasText := false
	hasImage := false
	hasEngineCmd := false
	hasLua := false

	for _, child := range node.Children {
		cmd := &CutsceneCommand{
			Type:   child.Type,
			Params: []string{child.Name},
		}

		switch child.Type {
		case "TEXT":
			hasText = true
			cmd.Type = "TEXT"
			cmd.Params = []string{child.Name} // text content

		case "IMAGE":
			hasImage = true
			cmd.Type = "IMAGE"
			cmd.Params = []string{child.Name}

		case "MUSIC":
			cmd.Type = "MUSIC"
			cmd.Params = []string{child.Name}

		case "DURATION":
			cmd.Duration = parseFloat(child.Name)

		case "NEXT_ON_KEY":
			cmd.Type = "NEXT_ON_KEY"
			cs.WaitForKey = true

		case "CAMERA_MOVE":
			hasEngineCmd = true
			cmd.Type = "CAMERA_MOVE"
			cmd.Params = strings.Fields(child.Name)
			cmd.FloatParams = parseFloatSlice(cmd.Params)
			if len(cmd.FloatParams) >= 4 {
				cmd.Duration = cmd.FloatParams[3]
			}

		case "CAMERA_LOOK":
			hasEngineCmd = true
			cmd.Type = "CAMERA_LOOK"
			cmd.Params = strings.Fields(child.Name)
			cmd.FloatParams = parseFloatSlice(cmd.Params)
			if len(cmd.FloatParams) >= 4 {
				cmd.Duration = cmd.FloatParams[3]
			}

		case "SPAWN":
			hasEngineCmd = true
			cmd.Type = "SPAWN"
			cmd.Params = strings.Fields(child.Name)

		case "ANIMATE":
			hasEngineCmd = true
			cmd.Type = "ANIMATE"
			cmd.Params = strings.Fields(child.Name)

		case "SHAKE_CAMERA":
			hasEngineCmd = true
			cmd.Type = "SHAKE_CAMERA"
			cmd.Params = strings.Fields(child.Name)
			cmd.FloatParams = parseFloatSlice(cmd.Params)
			if len(cmd.FloatParams) >= 2 {
				cmd.Duration = cmd.FloatParams[1]
			}

		case "FREEZE_PLAYER":
			cmd.Type = "FREEZE_PLAYER"

		case "UNFREEZE_PLAYER":
			cmd.Type = "UNFREEZE_PLAYER"

		case "PLAY_SOUND":
			cmd.Type = "PLAY_SOUND"
			cmd.Params = []string{child.Name}

		case "WAIT":
			cmd.Type = "WAIT"
			cmd.Duration = parseFloat(child.Name)

		case "FADE_IN":
			cmd.Type = "FADE_IN"
			cmd.Duration = parseFloat(child.Name)

		case "FADE_OUT":
			cmd.Type = "FADE_OUT"
			cmd.Duration = parseFloat(child.Name)

		case "LUA":
			hasLua = true
			cmd.Type = "LUA"
			cmd.Params = []string{child.Name}
		}

		cs.Commands = append(cs.Commands, cmd)
	}

	// Determine level.
	switch {
	case hasLua:
		cs.Level = 4
	case hasEngineCmd:
		cs.Level = 3
	case hasImage:
		cs.Level = 2
	case hasText:
		cs.Level = 1
	}

	cp.cutscenes[cs.Name] = cs
}

// Play starts a named cutscene.
func (cp *CutscenePlayer) Play(name string) {
	cs, ok := cp.cutscenes[name]
	if !ok {
		log.Printf("[Cutscene] Not found: %s", name)
		return
	}

	cs.Playing = true
	cs.CurrentCmd = 0
	cs.Timer = 0
	cs.StartedAt = time.Now()
	cp.active = cs

	log.Printf("[Cutscene] Starting: %s (level %d, %d commands)", name, cs.Level, len(cs.Commands))
}

// Stop ends the active cutscene.
func (cp *CutscenePlayer) Stop() {
	if cp.active == nil {
		return
	}
	cp.active.Playing = false
	if cp.playerFrozen {
		if cp.OnUnfreezePlayer != nil {
			cp.OnUnfreezePlayer()
		}
		cp.playerFrozen = false
	}
	log.Printf("[Cutscene] Stopped: %s", cp.active.Name)
	cp.active = nil
}

// IsPlaying returns true if a cutscene is active.
func (cp *CutscenePlayer) IsPlaying() bool {
	return cp.active != nil && cp.active.Playing
}

// KeyAdvance signals that the player pressed the advance key (for NEXT_ON_KEY).
func (cp *CutscenePlayer) KeyAdvance() {
	if cp.active != nil && cp.active.WaitForKey {
		cp.active.KeyPressed = true
	}
}

// Update advances the cutscene timeline.
func (cp *CutscenePlayer) Update(dt float64) {
	if cp.active == nil || !cp.active.Playing {
		return
	}

	cs := cp.active
	cs.Timer += dt

	// Process commands that should happen at the current time.
	for cs.CurrentCmd < len(cs.Commands) {
		cmd := cs.Commands[cs.CurrentCmd]

		if cmd.Executed {
			cs.CurrentCmd++
			continue
		}

		// Check if it's time to execute this command.
		if cmd.Duration > 0 && cs.Timer < cmd.Duration {
			break
		}

		cp.executeCommand(cmd)
		cmd.Executed = true

		// If this is a WAIT command, break and wait for its duration.
		if cmd.Type == "WAIT" {
			cs.Timer = 0
			cs.CurrentCmd++
			break
		}

		cs.CurrentCmd++
	}

	// Check if all commands are done.
	allDone := true
	for _, cmd := range cs.Commands {
		if !cmd.Executed {
			allDone = false
			break
		}
	}

	if allDone && !cs.WaitForKey {
		cp.Stop()
	}
}

func (cp *CutscenePlayer) executeCommand(cmd *CutsceneCommand) {
	switch cmd.Type {
	case "TEXT":
		if cp.OnShowText != nil && len(cmd.Params) > 0 {
			cp.OnShowText(cmd.Params[0])
		}

	case "IMAGE":
		if cp.OnShowImage != nil && len(cmd.Params) > 0 {
			cp.OnShowImage(cmd.Params[0])
		}

	case "MUSIC":
		if cp.OnPlayMusic != nil && len(cmd.Params) > 0 {
			cp.OnPlayMusic(cmd.Params[0])
		}

	case "CAMERA_MOVE":
		if cp.OnCameraMove != nil && len(cmd.FloatParams) >= 4 {
			cp.OnCameraMove(
				cmd.FloatParams[0],
				cmd.FloatParams[1],
				cmd.FloatParams[2],
				cmd.FloatParams[3],
			)
		}

	case "CAMERA_LOOK":
		if cp.OnCameraLook != nil && len(cmd.FloatParams) >= 4 {
			cp.OnCameraLook(
				cmd.FloatParams[0],
				cmd.FloatParams[1],
				cmd.FloatParams[2],
				cmd.FloatParams[3],
			)
		}

	case "SPAWN":
		if cp.OnSpawn != nil && len(cmd.Params) >= 4 {
			cp.OnSpawn(
				cmd.Params[0],
				parseFloat(cmd.Params[1]),
				parseFloat(cmd.Params[2]),
				parseFloat(cmd.Params[3]),
			)
		}

	case "ANIMATE":
		if cp.OnAnimate != nil && len(cmd.Params) >= 2 {
			cp.OnAnimate(cmd.Params[0], cmd.Params[1])
		}

	case "SHAKE_CAMERA":
		if cp.OnShakeCamera != nil && len(cmd.FloatParams) >= 2 {
			cp.OnShakeCamera(cmd.FloatParams[0], cmd.FloatParams[1])
		}

	case "FREEZE_PLAYER":
		cp.playerFrozen = true
		if cp.OnFreezePlayer != nil {
			cp.OnFreezePlayer()
		}

	case "UNFREEZE_PLAYER":
		cp.playerFrozen = false
		if cp.OnUnfreezePlayer != nil {
			cp.OnUnfreezePlayer()
		}

	case "PLAY_SOUND":
		if cp.OnPlaySound != nil && len(cmd.Params) > 0 {
			cp.OnPlaySound(cmd.Params[0])
		}

	case "FADE_IN":
		if cp.OnFadeIn != nil {
			cp.OnFadeIn(cmd.Duration)
		}

	case "FADE_OUT":
		if cp.OnFadeOut != nil {
			cp.OnFadeOut(cmd.Duration)
		}

	case "LUA":
		if cp.OnLuaFunction != nil && len(cmd.Params) > 0 {
			cp.OnLuaFunction(cmd.Params[0])
		}

	case "WAIT", "NEXT_ON_KEY", "DURATION":
		// These are timing commands, handled in Update.
	}
}

// --- Cutscene Presets ---

// CreateIntroCutscene builds a simple level-1 intro cutscene.
func CreateIntroCutscene(title string, messages []string) *Cutscene {
	cs := &Cutscene{
		Name:      "intro",
		Level:     1,
		WaitForKey: true,
	}

	for _, msg := range messages {
		cs.Commands = append(cs.Commands, &CutsceneCommand{
			Type:   "TEXT",
			Params: []string{msg},
		})
	}

	return cs
}

// CreateBossIntroCutscene builds a level-3 boss intro cutscene.
func CreateBossIntroCutscene(bossType string, spawnX, spawnY, spawnZ float64) *Cutscene {
	cs := &Cutscene{
		Name:  "boss_intro",
		Level: 3,
	}

	cs.Commands = append(cs.Commands,
		&CutsceneCommand{Type: "FREEZE_PLAYER"},
		&CutsceneCommand{
			Type:        "CAMERA_MOVE",
			FloatParams: []float64{0, 2, -5, 3},
			Duration:    3.0,
		},
		&CutsceneCommand{
			Type:   "SPAWN",
			Params: []string{bossType, ftoa(spawnX), ftoa(spawnY), ftoa(spawnZ)},
		},
		&CutsceneCommand{
			Type:   "ANIMATE",
			Params: []string{bossType, "roar"},
		},
		&CutsceneCommand{
			Type:        "SHAKE_CAMERA",
			FloatParams: []float64{0.5, 1.0},
			Duration:    1.0,
		},
		&CutsceneCommand{
			Type:   "TEXT",
			Params: []string{"BOSS PRZYBYWA!"},
		},
		&CutsceneCommand{
			Type:   "PLAY_SOUND",
			Params: []string{"roar.ogg"},
		},
		&CutsceneCommand{Type: "WAIT", Duration: 2.0},
		&CutsceneCommand{Type: "UNFREEZE_PLAYER"},
	)

	return cs
}

func ftoa(f float64) string {
	return strings.TrimRight(
		strings.TrimRight(
			fmt.Sprintf("%.2f", f), "0",
		), ".",
	)
}

// parseFloatSlice converts a string slice to float64 slice.
func parseFloatSlice(ss []string) []float64 {
	result := make([]float64, len(ss))
	for i, s := range ss {
		result[i] = parseFloat(s)
	}
	return result
}
