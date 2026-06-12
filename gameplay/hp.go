// gameplay/hp.go — Health point system for AIEngine.
// Manages global HP, per-body-part HP, boss phases, and damage effects.
// Supports WEAK_SPOT multipliers, ARMOR reduction, and REGENERATE.
// Integrates with the entity system for damage application.

package gameplay

import (
	"log"
)

// HPSystem manages health for all entities and the player.
type HPSystem struct {
	entities map[string]*HPState
	player   *HPState
}

// HPState tracks health for a single entity or the player.
type HPState struct {
	Global     float64
	Max        float64
	Parts      map[string]*PartHP
	Phases     []HPPhaseState
	RegenRate  float64
	RegenDelay float64
	RegenTimer float64

	// Armor and weak spots.
	WeakSpots map[string]float64
	Armor     map[string]float64
}

// PartHP tracks health for a single body part.
type PartHP struct {
	Current  float64
	Max      float64
	OnDamage string // effect when damaged
	On50HP   string // effect at 50% HP
	OnDeath  string // effect when destroyed
}

// HPPhaseState tracks a boss HP phase.
type HPPhaseState struct {
	Phase     int
	Threshold float64
	Triggered bool
}

// NewHPSystem creates an empty HP system.
func NewHPSystem() *HPSystem {
	return &HPSystem{
		entities: make(map[string]*HPState),
	}
}

// CreatePlayerHP initializes the player's HP state.
func (hs *HPSystem) CreatePlayerHP(maxHP float64) *HPState {
	hs.player = &HPState{
		Global: maxHP,
		Max:    maxHP,
		Parts:  make(map[string]*PartHP),
	}
	return hs.player
}

// CreateEntityHP initializes HP for an entity from its definition.
func (hs *HPSystem) CreateEntityHP(entityID string, def *EntityDef) *HPState {
	state := &HPState{
		Global:    def.HP,
		Max:       def.MaxHP,
		Parts:     make(map[string]*PartHP),
		WeakSpots: def.WeakSpots,
		Armor:     def.Armor,
	}

	// Copy per-part HP.
	for part, hp := range def.HPParts {
		state.Parts[part] = &PartHP{
			Current: hp,
			Max:     hp,
		}
	}

	// Copy phases.
	for _, phase := range def.HPPhases {
		state.Phases = append(state.Phases, HPPhaseState{
			Phase:     phase.Phase,
			Threshold: phase.HP,
		})
	}

	// Regeneration.
	if def.Behavior.Regenerate {
		state.RegenRate = def.Behavior.RegenerateRate
		state.RegenDelay = 3.0
	}

	hs.entities[entityID] = state
	return state
}

// GetPlayerHP returns the player's current HP.
func (hs *HPSystem) GetPlayerHP() float64 {
	if hs.player == nil {
		return 100
	}
	return hs.player.Global
}

// GetPlayerMaxHP returns the player's maximum HP.
func (hs *HPSystem) GetPlayerMaxHP() float64 {
	if hs.player == nil {
		return 100
	}
	return hs.player.Max
}

// GetEntityHP returns an entity's current HP.
func (hs *HPSystem) GetEntityHP(id string) float64 {
	if state, ok := hs.entities[id]; ok {
		return state.Global
	}
	return 0
}

// --- Damage Application ---

// DamageResult contains the outcome of a damage application.
type DamageResult struct {
	DamageDealt   float64
	PartHit       string
	PartDestroyed bool
	Killed        bool
	PhaseChanged  int // 0 = no change, >0 = new phase number
	Effects       []string
}

// DamagePlayer applies damage to the player.
func (hs *HPSystem) DamagePlayer(amount float64, part string) DamageResult {
	if hs.player == nil {
		return DamageResult{}
	}
	return hs.applyDamage(hs.player, amount, part)
}

// DamageEntity applies damage to an entity by ID.
func (hs *HPSystem) DamageEntity(id string, amount float64, part string) DamageResult {
	state, ok := hs.entities[id]
	if !ok {
		return DamageResult{}
	}
	return hs.applyDamage(state, amount, part)
}

// applyDamage applies damage to an HPState, computing weak spot/armor modifiers.
func (hs *HPSystem) applyDamage(state *HPState, amount float64, part string) DamageResult {
	result := DamageResult{
		DamageDealt: amount,
		PartHit:     part,
	}

	// Apply weak spot multiplier.
	if mult, ok := state.WeakSpots[part]; ok {
		amount *= mult
		result.DamageDealt = amount
	}

	// Apply armor reduction.
	if reduction, ok := state.Armor[part]; ok {
		amount *= (1.0 - reduction)
		result.DamageDealt = amount
	}

	// Apply to global HP.
	state.Global -= amount

	if state.Global < 0 {
		state.Global = 0
	}

	// Apply to part HP.
	if partHP, ok := state.Parts[part]; ok {
		partHP.Current -= amount
		if partHP.Current <= 0 {
			partHP.Current = 0
			result.PartDestroyed = true
			if partHP.OnDeath != "" {
				result.Effects = append(result.Effects, partHP.OnDeath)
			}
		} else if partHP.Current <= partHP.Max*0.5 && partHP.On50HP != "" {
			result.Effects = append(result.Effects, partHP.On50HP)
		}
		if partHP.OnDamage != "" {
			result.Effects = append(result.Effects, partHP.OnDamage)
		}
	}

	// Check for death.
	if state.Global <= 0 {
		result.Killed = true
	}

	// Check for phase changes.
	for i := range state.Phases {
		if !state.Phases[i].Triggered && state.Global <= state.Phases[i].Threshold {
			state.Phases[i].Triggered = true
			result.PhaseChanged = state.Phases[i].Phase
			break
		}
	}

	return result
}

// --- Regeneration ---

// UpdateRegen advances regeneration timers and applies healing.
func (hs *HPSystem) UpdateRegen(dt float64) {
	// Player regen.
	if hs.player != nil && hs.player.RegenRate > 0 {
		hs.player.RegenTimer += dt
		if hs.player.RegenTimer >= hs.player.RegenDelay && hs.player.Global < hs.player.Max {
			hs.player.Global += hs.player.RegenRate * dt
			if hs.player.Global > hs.player.Max {
				hs.player.Global = hs.player.Max
			}
		}
	}

	// Entity regen.
	for id, state := range hs.entities {
		if state.RegenRate > 0 && state.Global > 0 {
			state.RegenTimer += dt
			if state.RegenTimer >= state.RegenDelay && state.Global < state.Max {
				state.Global += state.RegenRate * dt
				if state.Global > state.Max {
					state.Global = state.Max
				}
			}
		}
		_ = id
	}
}

// --- Effect Application ---

// ApplyEffectsCallback is called when damage effects need to be applied (e.g., blur vision, slow move).
type ApplyEffectsCallback func(entityID string, effects []string)

// --- HP Effects from DOCS ---

// GetPartEffect returns the script event string for a part damage event.
func GetPartEffect(part string, eventType string) string {
	effects := map[string]map[string]string{
		"HEAD": {
			"ON_DAMAGE": "stagger",
			"ON_50HP":   "blur_vision",
			"ON_DEATH":  "instant_kill",
		},
		"TORSO": {
			"ON_DAMAGE": "slow_move",
			"ON_50HP":   "heavy_breathing",
		},
		"ARM_LEFT": {
			"ON_DEATH":  "drop_weapon",
			"ON_DAMAGE": "aim_penalty",
		},
		"ARM_RIGHT": {
			"ON_DEATH":  "drop_weapon",
			"ON_DAMAGE": "aim_penalty",
		},
		"LEG_LEFT": {
			"ON_DEATH":  "limp",
			"ON_DAMAGE": "slow_move",
		},
		"LEG_RIGHT": {
			"ON_DEATH":  "limp",
			"ON_DAMAGE": "slow_move",
		},
	}

	if partEffects, ok := effects[part]; ok {
		if effect, ok := partEffects[eventType]; ok {
			return effect
		}
	}
	return ""
}

// LogDamage logs damage events for debugging.
func LogDamage(sourceID, targetID string, amount float64, part string, killed bool) {
	if killed {
		log.Printf("[HP] %s killed %s (%s, %.0f dmg)", sourceID, targetID, part, amount)
	} else {
		log.Printf("[HP] %s hit %s in %s for %.0f dmg", sourceID, targetID, part, amount)
	}
}
