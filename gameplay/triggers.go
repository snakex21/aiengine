// gameplay/triggers.go — Trigger system for AIEngine.
// Triggers are condition-action pairs defined in .aie MAP blocks.
// Conditions are evaluated each frame; when met, actions are executed once.
// Supports Lua integration for complex trigger logic.

package gameplay

import (
	"fmt"
	"log"
	"strings"

	"aiengine/parser"
)

// Trigger is a single condition-action pair from a .aie TRIGGER block.
type Trigger struct {
	Name       string
	Conditions []TriggerCondition
	Actions    []TriggerAction
	Triggered  bool
	Repeatable bool // if true, can fire multiple times
}

// TriggerCondition represents a game state check.
type TriggerCondition struct {
	Type   string // e.g., "all_enemies_dead", "player_enters", "hp_below"
	Param  string // zone name, threshold value, etc.
	Param2 string
}

// TriggerAction represents something to execute when conditions are met.
type TriggerAction struct {
	Type  string // e.g., "unlock_door", "play_sound", "show_message"
	Param string // door ID, sound file, message text, etc.
}

// --- Condition Evaluation ---

// TriggerContext provides the game state necessary for condition evaluation.
type TriggerContext struct {
	PlayerHP       float64
	PlayerX        float64
	PlayerY        float64
	EnemyCount     int
	AllEnemiesDead bool
	Zone           string // current zone being evaluated
	Vars           map[string]interface{}
	Timers         map[string]float64 // named timers → remaining seconds
	ItemPicked     map[string]bool
	ButtonPressed  map[string]bool
	EnemyDead      map[string]bool // specific enemy death tracking
	ZonesInside    map[string]bool // zone id → player currently inside
	ZonesVisited   map[string]bool // zone id → player has been inside at least once
}

// Evaluate checks if all conditions in a trigger are satisfied.
func (t *Trigger) Evaluate(ctx *TriggerContext) bool {
	if t.Triggered && !t.Repeatable {
		return false
	}

	for _, cond := range t.Conditions {
		if !evaluateCondition(&cond, ctx) {
			return false
		}
	}
	return true
}

func evaluateCondition(cond *TriggerCondition, ctx *TriggerContext) bool {
	switch cond.Type {
	case "all_enemies_dead":
		if cond.Param != "" && cond.Param != ctx.Zone {
			return ctx.AllEnemiesDead
		}
		return ctx.AllEnemiesDead

	case "player_enters":
		// True while the player is inside the zone (zone state is fed by the
		// level runtime via SetZoneInside).
		return ctx.ZonesInside[cond.Param]

	case "player_exits":
		// True once the player has been inside the zone and is now outside.
		return ctx.ZonesVisited[cond.Param] && !ctx.ZonesInside[cond.Param]

	case "var_equals":
		// CONDITION var_equals <name> <value>
		v, ok := ctx.Vars[cond.Param]
		return ok && fmt.Sprint(v) == cond.Param2

	case "var_at_least":
		// CONDITION var_at_least <name> <number>
		v, ok := ctx.Vars[cond.Param]
		if !ok {
			return false
		}
		return parseFloat(fmt.Sprint(v)) >= parseFloat(cond.Param2)

	case "item_picked":
		id := cond.Param
		return ctx.ItemPicked[id]

	case "hp_below":
		threshold := parseFloat(cond.Param)
		return ctx.PlayerHP <= threshold

	case "timer_elapsed":
		name := cond.Param
		if remaining, ok := ctx.Timers[name]; ok {
			return remaining <= 0
		}
		return false

	case "button_pressed":
		id := cond.Param
		return ctx.ButtonPressed[id]

	case "enemy_dead":
		id := cond.Param
		return ctx.EnemyDead[id]

	default:
		log.Printf("[Trigger] Unknown condition type: %s", cond.Type)
		return false
	}
}

// --- Action Execution ---

// TriggerExecutor handles running trigger actions against the game state.
type TriggerExecutor struct {
	OnUnlockDoor     func(id string)
	OnLockDoor       func(id string)
	OnPlaySound      func(file string)
	OnPlayMusic      func(file string)
	OnStopMusic      func()
	OnShowMessage    func(text string)
	OnHideMessage    func()
	OnStartLua       func(funcName string)
	OnSetVar         func(name string, value interface{})
	OnWinGame        func()
	OnLoseGame       func()
	OnTeleportPlayer func(x, y, z float64)
	OnEarthquake     func(intensity, duration float64)
	OnChangeMusic    func(file string)
	OnSpawnEnemy     func(typ string, x, y, z float64)
	OnSpawnProp      func(typ string, x, y, z float64)
	OnExitTo         func(target string) // exit_to <level id|next>
	// Animated sectors: activate <tag>, move_floor/move_ceil <tag> <h> [speed].
	OnActivate func(tag string)
	OnMove     func(tag string, ceil bool, h, speed float64)
}

// Execute runs all actions for a trigger.
func (te *TriggerExecutor) Execute(trigger *Trigger) {
	if trigger.Triggered && !trigger.Repeatable {
		return
	}

	trigger.Triggered = true

	for _, action := range trigger.Actions {
		te.executeAction(&action)
	}
}

func (te *TriggerExecutor) executeAction(action *TriggerAction) {
	log.Printf("[Trigger] Executing: %s %s", action.Type, action.Param)

	switch action.Type {
	case "unlock_door":
		if te.OnUnlockDoor != nil {
			te.OnUnlockDoor(action.Param)
		}
	case "lock_door":
		if te.OnLockDoor != nil {
			te.OnLockDoor(action.Param)
		}
	case "spawn_enemy":
		// Parse x y z from param: "grunt 5.0 0.0 2.0"
		parts := strings.Fields(action.Param)
		if len(parts) >= 4 && te.OnSpawnEnemy != nil {
			x := parseFloat(parts[1])
			y := parseFloat(parts[2])
			z := parseFloat(parts[3])
			te.OnSpawnEnemy(parts[0], x, y, z)
		}
	case "spawn_prop":
		parts := strings.Fields(action.Param)
		if len(parts) >= 4 && te.OnSpawnProp != nil {
			x := parseFloat(parts[1])
			y := parseFloat(parts[2])
			z := parseFloat(parts[3])
			te.OnSpawnProp(parts[0], x, y, z)
		}
	case "play_sound":
		if te.OnPlaySound != nil {
			te.OnPlaySound(action.Param)
		}
	case "play_music":
		if te.OnPlayMusic != nil {
			te.OnPlayMusic(action.Param)
		}
	case "stop_music":
		if te.OnStopMusic != nil {
			te.OnStopMusic()
		}
	case "show_message":
		if te.OnShowMessage != nil {
			te.OnShowMessage(action.Param)
		}
	case "hide_message":
		if te.OnHideMessage != nil {
			te.OnHideMessage()
		}
	case "start_lua":
		if te.OnStartLua != nil {
			te.OnStartLua(action.Param)
		}
	case "set_var":
		parts := strings.Fields(action.Param)
		if len(parts) >= 2 && te.OnSetVar != nil {
			te.OnSetVar(parts[0], parts[1])
		}
	case "win_game":
		if te.OnWinGame != nil {
			te.OnWinGame()
		}
	case "lose_game":
		if te.OnLoseGame != nil {
			te.OnLoseGame()
		}
	case "teleport_player":
		parts := strings.Fields(action.Param)
		if len(parts) >= 3 && te.OnTeleportPlayer != nil {
			te.OnTeleportPlayer(
				parseFloat(parts[0]),
				parseFloat(parts[1]),
				parseFloat(parts[2]),
			)
		}
	case "earthquake":
		parts := strings.Fields(action.Param)
		if len(parts) >= 2 && te.OnEarthquake != nil {
			te.OnEarthquake(
				parseFloat(parts[0]),
				parseFloat(parts[1]),
			)
		}
	case "exit_to":
		if te.OnExitTo != nil {
			target := action.Param
			if target == "" {
				target = "next"
			}
			te.OnExitTo(target)
		}
	case "change_music":
		if te.OnChangeMusic != nil {
			te.OnChangeMusic(action.Param)
		}
	case "activate":
		if te.OnActivate != nil {
			te.OnActivate(strings.TrimSpace(action.Param))
		}
	case "move_floor", "move_ceil":
		parts := strings.Fields(action.Param)
		if len(parts) >= 2 && te.OnMove != nil {
			speed := 0.0
			if len(parts) >= 3 {
				speed = parseFloat(parts[2])
			}
			te.OnMove(parts[0], action.Type == "move_ceil", parseFloat(parts[1]), speed)
		}
	default:
		log.Printf("[Trigger] Unknown action type: %s", action.Type)
	}
}

// --- Trigger Registry ---

// TriggerRegistry manages all triggers in the current level.
type TriggerRegistry struct {
	triggers []*Trigger
	executor *TriggerExecutor
	context  *TriggerContext
}

// NewTriggerRegistry creates an empty trigger registry.
func NewTriggerRegistry() *TriggerRegistry {
	return &TriggerRegistry{
		context: &TriggerContext{
			Vars:          make(map[string]interface{}),
			Timers:        make(map[string]float64),
			ItemPicked:    make(map[string]bool),
			ButtonPressed: make(map[string]bool),
			EnemyDead:     make(map[string]bool),
			ZonesInside:   make(map[string]bool),
			ZonesVisited:  make(map[string]bool),
		},
	}
}

// SetExecutor assigns the action executor.
func (tr *TriggerRegistry) SetExecutor(exec *TriggerExecutor) {
	tr.executor = exec
}

// AddTrigger registers a trigger from a parsed .aie node.
func (tr *TriggerRegistry) AddTrigger(node *parser.AIENode) {
	t := &Trigger{
		Name: node.Name,
	}

	for _, child := range node.Children {
		switch child.Type {
		case "CONDITION":
			parts := strings.SplitN(child.Name, " ", 3)
			cond := TriggerCondition{
				Type: parts[0],
			}
			if len(parts) >= 2 {
				cond.Param = parts[1]
			}
			if len(parts) >= 3 {
				cond.Param2 = parts[2]
			}
			t.Conditions = append(t.Conditions, cond)

		case "ACTION":
			parts := strings.SplitN(child.Name, " ", 2)
			action := TriggerAction{
				Type: parts[0],
			}
			if len(parts) >= 2 {
				action.Param = parts[1]
			}
			t.Actions = append(t.Actions, action)
		}
	}

	tr.triggers = append(tr.triggers, t)
}

// UpdateContext refreshes the evaluation context with current game state.
func (tr *TriggerRegistry) UpdateContext(playerHP float64, playerX, playerY float64, enemyCount int, allDead bool) {
	tr.context.PlayerHP = playerHP
	tr.context.PlayerX = playerX
	tr.context.PlayerY = playerY
	tr.context.EnemyCount = enemyCount
	tr.context.AllEnemiesDead = allDead
}

// SetVar sets a script variable for trigger evaluation.
func (tr *TriggerRegistry) SetVar(name string, value interface{}) {
	tr.context.Vars[name] = value
}

// MarkEnemyDead records a specific enemy death for trigger conditions.
func (tr *TriggerRegistry) MarkEnemyDead(id string) {
	tr.context.EnemyDead[id] = true
}

// MarkItemPicked records an item pickup for item_picked conditions.
func (tr *TriggerRegistry) MarkItemPicked(id string) {
	tr.context.ItemPicked[id] = true
}

// SetZoneInside records whether the player is inside a zone, for
// player_enters / player_exits conditions.
func (tr *TriggerRegistry) SetZoneInside(id string, inside bool) {
	tr.context.ZonesInside[id] = inside
	if inside {
		tr.context.ZonesVisited[id] = true
	}
}

// SetTimer stores the remaining seconds of a named timer for timer_elapsed.
func (tr *TriggerRegistry) SetTimer(name string, remaining float64) {
	tr.context.Timers[name] = remaining
}

// Count returns the number of registered triggers.
func (tr *TriggerRegistry) Count() int {
	return len(tr.triggers)
}

// Tick evaluates all triggers and executes any whose conditions are met.
func (tr *TriggerRegistry) Tick() {
	if tr.executor == nil {
		return
	}

	for _, t := range tr.triggers {
		if t.Evaluate(tr.context) {
			tr.executor.Execute(t)
		}
	}
}

// parseFloat reads a number leniently: an optional leading '-', digits and
// one decimal point; any other character is skipped ("30s" = 30).
func parseFloat(s string) float64 {
	var result float64
	frac := 1.0
	decimal := false
	sign := 1.0
	for i, ch := range s {
		if i == 0 && ch == '-' {
			sign = -1.0
			continue
		}
		if ch == '.' {
			decimal = true
			continue
		}
		if ch < '0' || ch > '9' {
			continue
		}
		digit := float64(ch - '0')
		if decimal {
			frac *= 0.1
			result += digit * frac
		} else {
			result = result*10 + digit
		}
	}
	return result * sign
}
