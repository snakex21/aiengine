// gameplay/entities.go — Entity type system for AIEngine.
// Defines all entity categories (HUMANOID, CREATURE, VEHICLE, PROP, WEAPON),
// their subtypes, body parts, behaviors, animations, and spawn management.
// All entity definitions are loaded from .aie files via the parser bridge.

package gameplay

import (
	"strings"

	"aiengine/parser"
	"aiengine/systems"
)

// EntityType enumerates the top-level entity categories.
type EntityType int

const (
	EntHumanoid EntityType = iota
	EntCreature
	EntVehicle
	EntProp
	EntWeaponPickup // weapon as a pickup item (distinct from weapon system)
)

// SubType refines the entity category.
type SubType int

const (
	// Humanoid subtypes.
	SubHumanoid SubType = iota
	SubAlien
	SubMutant
	SubRobot
	SubUndead

	// Creature subtypes.
	SubQuadruped
	SubMultiLeg
	SubFlying
	SubAquatic
	SubAmorphous
	SubSerpentine
	SubGiant

	// Vehicle subtypes (future).
	SubCar
	SubTank
	SubAircraft

	// Prop subtypes.
	SubBarrel
	SubCrate
	SubFurniture
	SubLight
	SubDoor
)

// String returns the subtype as its .aie keyword.
func (st SubType) String() string {
	switch st {
	case SubHumanoid:
		return "HUMANOID"
	case SubAlien:
		return "ALIEN"
	case SubMutant:
		return "MUTANT"
	case SubRobot:
		return "ROBOT"
	case SubUndead:
		return "UNDEAD"
	case SubQuadruped:
		return "QUADRUPED"
	case SubMultiLeg:
		return "MULTI_LEG"
	case SubFlying:
		return "FLYING"
	case SubAquatic:
		return "AQUATIC"
	case SubAmorphous:
		return "AMORPHOUS"
	case SubSerpentine:
		return "SERPENTINE"
	case SubGiant:
		return "GIANT"
	case SubCar:
		return "CAR"
	case SubTank:
		return "TANK"
	case SubAircraft:
		return "AIRCRAFT"
	case SubBarrel:
		return "BARREL"
	case SubCrate:
		return "CRATE"
	case SubFurniture:
		return "FURNITURE"
	case SubLight:
		return "LIGHT"
	case SubDoor:
		return "DOOR"
	default:
		return "UNKNOWN"
	}
}

// ParseSubType converts a .aie entity type string to a SubType.
func ParseSubType(s string) SubType {
	switch s {
	case "HUMANOID":
		return SubHumanoid
	case "ALIEN":
		return SubAlien
	case "MUTANT":
		return SubMutant
	case "ROBOT":
		return SubRobot
	case "UNDEAD":
		return SubUndead
	case "QUADRUPED":
		return SubQuadruped
	case "MULTI_LEG":
		return SubMultiLeg
	case "FLYING":
		return SubFlying
	case "AQUATIC":
		return SubAquatic
	case "AMORPHOUS":
		return SubAmorphous
	case "SERPENTINE":
		return SubSerpentine
	case "GIANT":
		return SubGiant
	case "VEHICLE", "CAR":
		return SubCar
	case "PROP":
		return SubBarrel
	default:
		return SubHumanoid
	}
}

// --- Body Parts ---

// BodyPart represents a named body segment with size, texture, and optional count.
type BodyPart struct {
	Name    string
	Size    string // "small", "medium", "large", "huge", "tiny"
	Texture string
	Count   int // for multi-part (e.g., ARMS 4)
}

// ParseBodyPart creates a BodyPart from a property key and value.
func ParseBodyPart(key, value string) BodyPart {
	parts := splitFields(value)
	count := 1
	size := "medium"
	texture := ""

	if len(parts) >= 1 {
		// First part could be a count or a size.
		if n, ok := parseInt(parts[0]); ok && n > 0 {
			count = n
			if len(parts) >= 2 {
				size = parts[1]
			}
			if len(parts) >= 3 {
				texture = parts[2]
			}
		} else {
			size = parts[0]
			if len(parts) >= 2 {
				texture = parts[1]
			}
		}
	}

	return BodyPart{
		Name:    key,
		Size:    size,
		Texture: texture,
		Count:   count,
	}
}

// --- Animations ---

// AnimKeyframe holds a single keyframe in an animation.
type AnimKeyframe struct {
	Frame      int
	Transforms map[string]Transform // body part → transform
}

// Transform describes a spatial transformation for a body part.
type Transform struct {
	Rotate systems.Vec3
	Shake  float64
	Scale  systems.Vec3
}

// Animation is a named sequence of keyframes.
type Animation struct {
	Name   string
	Frames []AnimKeyframe
	Loop   bool
	Speed  float64
}

// ParseAnimation creates an Animation from a parsed .aie ANIM node.
func ParseAnimation(node *parser.AIENode) *Animation {
	anim := &Animation{
		Name:  node.Name,
		Loop:  true,
		Speed: 1.0,
	}

	for _, child := range node.Children {
		if child.Type == "FRAME" {
			frameNum := 0
			if n, ok := parseInt(child.Name); ok {
				frameNum = n
			}
			kf := AnimKeyframe{
				Frame:      frameNum,
				Transforms: make(map[string]Transform),
			}
			for _, prop := range child.Children {
				// prop.Type is the body part, prop.Name is the transform string
				// e.g., Type="HEAD", Name="ROT 0 5 0"
				t := parseTransform(prop.Type, prop.Name)
				kf.Transforms[prop.Type] = t
			}
			// Also check properties directly on the FRAME node.
			for k, v := range child.Properties {
				t := parseTransform(k, v)
				kf.Transforms[k] = t
			}
			anim.Frames = append(anim.Frames, kf)
		}
	}

	return anim
}

func parseTransform(partName, value string) Transform {
	t := Transform{Scale: systems.Vec3{X: 1, Y: 1, Z: 1}}
	fields := splitFields(value)

	if len(fields) < 2 {
		return t
	}

	op := fields[0]
	switch op {
	case "ROT":
		if len(fields) >= 4 {
			t.Rotate = systems.Vec3{
				X: parseFloat(fields[1]),
				Y: parseFloat(fields[2]),
				Z: parseFloat(fields[3]),
			}
		}
	case "SHAKE":
		if len(fields) >= 2 {
			t.Shake = parseFloat(fields[1])
		}
	case "SCALE":
		if len(fields) >= 4 {
			t.Scale = systems.Vec3{
				X: parseFloat(fields[1]),
				Y: parseFloat(fields[2]),
				Z: parseFloat(fields[3]),
			}
		}
	}
	return t
}

// --- Behavior Flags ---

// BehaviorFlags holds special entity behaviors.
type BehaviorFlags struct {
	Rideable               bool
	Boss                   bool
	FireBreath             bool
	SplitOnDeath           bool
	SplitOnDeathCount      int
	SplitOnDamage          bool
	SplitOnDamageThreshold float64
	Segments               bool
	SegmentCount           int
	Regenerate             bool
	RegenerateRate         float64
	PhaseThroughWalls      bool
	Swarm                  bool
	SwarmCount             int
	Burrow                 bool
	ClimbWalls             bool
	Fly                    bool
	FlyHeight              float64

	// Death behavior.
	DeathType           DeathBehaviorType
	ShatterParts        int
	DissolveTime        float64
	Earthquake          bool
	EarthquakeIntensity float64
	Sparks              bool
	SparksDuration      float64
}

// DefaultBehaviorFlags returns sensible defaults for a given subtype.
func DefaultBehaviorFlags(st SubType) BehaviorFlags {
	bf := BehaviorFlags{}

	switch st {
	case SubHumanoid:
		bf.DeathType = DeathRagdoll
	case SubAlien:
		bf.DeathType = DeathDissolve
	case SubMutant:
		bf.DeathType = DeathRagdoll
	case SubRobot:
		bf.DeathType = DeathCollapse
		bf.ShatterParts = 5
		bf.Sparks = true
		bf.SparksDuration = 2.0
	case SubUndead:
		bf.DeathType = DeathRagdoll
	case SubQuadruped:
		bf.DeathType = DeathRagdoll
	case SubMultiLeg:
		bf.DeathType = DeathRagdoll
	case SubFlying:
		bf.DeathType = DeathRagdoll
	case SubAquatic:
		bf.DeathType = DeathSink
	case SubAmorphous:
		bf.DeathType = DeathDissolve
	case SubSerpentine:
		bf.DeathType = DeathRagdoll
	case SubGiant:
		bf.DeathType = DeathCollapse
		bf.ShatterParts = 8
		bf.Earthquake = true
		bf.EarthquakeIntensity = 0.8
	}

	return bf
}

// ParseBehaviorFlags reads behavior properties from a child node.
func ParseBehaviorFlags(child *parser.AIENode, bf *BehaviorFlags) {
	deathType := bf.DeathType
	deathPriority := 0
	for k, v := range child.Properties {
		switch k {
		case "COLLAPSE":
			if v == "yes" || strings.HasPrefix(v, "yes") {
				if deathPriority < 1 {
					deathType = DeathCollapse
					deathPriority = 1
				}
			}
		case "SHATTER":
			deathType = DeathShatter
			deathPriority = 2
			bf.ShatterParts = parseParamInt(v, "parts", 5)
		case "DISSOLVE":
			if v == "yes" || strings.HasPrefix(v, "yes") {
				deathType = DeathDissolve
				deathPriority = 2
			}
		case "EARTHQUAKE":
			bf.Earthquake = v == "yes" || strings.HasPrefix(v, "yes")
			bf.EarthquakeIntensity = parseParamFloat(v, "intensity", 0.5)
		case "SPARKS":
			bf.Sparks = v == "yes" || strings.HasPrefix(v, "yes")
			bf.SparksDuration = parseParamFloat(v, "duration", 2.0)
		case "SPLIT_ON_DEATH":
			bf.SplitOnDeath = v == "yes" || strings.HasPrefix(v, "yes")
			bf.SplitOnDeathCount = parseParamInt(v, "count", 2)
		case "SPLIT_ON_DAMAGE":
			bf.SplitOnDamage = v == "yes" || strings.HasPrefix(v, "yes")
			bf.SplitOnDamageThreshold = parseParamFloat(v, "threshold", 50)
		case "SEGMENTS":
			bf.Segments = v == "yes" || strings.HasPrefix(v, "yes")
			bf.SegmentCount = parseParamInt(v, "count", 5)
		case "REGENERATE":
			bf.Regenerate = v == "yes" || strings.HasPrefix(v, "yes")
			bf.RegenerateRate = parseParamFloat(v, "rate", 5)
		case "BURROW":
			bf.Burrow = v == "yes" || strings.HasPrefix(v, "yes")
		case "CLIMB_WALLS":
			bf.ClimbWalls = v == "yes" || strings.HasPrefix(v, "yes")
		case "FLY":
			bf.Fly = v == "yes" || strings.HasPrefix(v, "yes")
			bf.FlyHeight = parseParamFloat(v, "height", 2.0)
		case "RIDEABLE":
			bf.Rideable = v == "yes" || strings.HasPrefix(v, "yes")
		case "BOSS":
			bf.Boss = v == "yes" || strings.HasPrefix(v, "yes")
		case "FIRE_BREATH":
			bf.FireBreath = v == "yes" || strings.HasPrefix(v, "yes")
		}
	}
	bf.DeathType = deathType
}

// --- Entity Definition ---

// EntityDef is the template for creating an entity instance.
// Built from .aie parsing.
type EntityDef struct {
	Name    string
	Type    EntityType
	SubType SubType
	HP      float64
	MaxHP   float64

	// Body parts.
	Parts []BodyPart

	// HP per body part.
	HPParts map[string]float64

	// HP phases (boss fights).
	HPPhases []HPPhase

	// Damage modifiers.
	WeakSpots map[string]float64 // part → multiplier
	Armor     map[string]float64 // part → damage reduction (0-1)

	// Behaviors.
	Behavior BehaviorFlags

	// Animations.
	Animations map[string]*Animation

	// Weapons (for weapon pickups).
	WeaponDef *WeaponDef

	// CSS class name for styling.
	CSSClass string
}

// HPPhase defines a boss phase triggered at a specific HP threshold.
type HPPhase struct {
	Phase int
	HP    float64
}

// ParseEntityDef creates an EntityDef from a parsed .aie entity node.
func ParseEntityDef(node *parser.AIENode) *EntityDef {
	ed := &EntityDef{
		Name:       node.Name,
		SubType:    ParseSubType(node.Type),
		Parts:      make([]BodyPart, 0),
		HPParts:    make(map[string]float64),
		HPPhases:   make([]HPPhase, 0),
		WeakSpots:  make(map[string]float64),
		Armor:      make(map[string]float64),
		Animations: make(map[string]*Animation),
		CSSClass:   node.Name,
	}

	// Determine top-level type.
	switch ed.SubType {
	case SubHumanoid, SubAlien, SubMutant, SubRobot, SubUndead:
		ed.Type = EntHumanoid
	case SubQuadruped, SubMultiLeg, SubFlying, SubAquatic, SubAmorphous, SubSerpentine, SubGiant:
		ed.Type = EntCreature
	case SubCar, SubTank, SubAircraft:
		ed.Type = EntVehicle
	case SubBarrel, SubCrate, SubFurniture, SubLight, SubDoor:
		ed.Type = EntProp
	}

	ed.Behavior = DefaultBehaviorFlags(ed.SubType)

	// Parse top-level properties.
	ed.HP = node.GetPropertyFloat("HP")
	ed.MaxHP = ed.HP

	// Parse body parts from properties.
	for k, v := range node.Properties {
		if isBodyPartKey(k) {
			ed.Parts = append(ed.Parts, ParseBodyPart(k, v))
		}
		if k == "RIDEABLE" {
			ed.Behavior.Rideable = v == "yes"
		}
		if k == "BOSS" {
			ed.Behavior.Boss = v == "yes"
		}
	}

	// Parse Children (HP_PARTS, HP_PHASES, BEHAVIOR, ANIM, WEAK_SPOT, ARMOR).
	for _, child := range node.Children {
		switch child.Type {
		case "HP_PARTS":
			for _, part := range child.Children {
				ed.HPParts[part.Type] = part.GetPropertyFloat(part.Type)
				// Also try Name as value.
				if v := parseFloat(part.Name); v > 0 {
					ed.HPParts[part.Type] = v
				}
			}
		case "HP_PHASES":
			for _, phase := range child.Children {
				hp := parseFloat(phase.Name)
				// Phase name might include "hp=3000"
				if hp == 0 {
					hp = parseParamFloat(phase.Name, "hp", 0)
				}
				phaseNum := 1
				if n, ok := parseInt(phase.Name); ok {
					phaseNum = n
				}
				ed.HPPhases = append(ed.HPPhases, HPPhase{Phase: phaseNum, HP: hp})
			}
		case "BEHAVIOR":
			ParseBehaviorFlags(child, &ed.Behavior)
		case "ANIM":
			anim := ParseAnimation(child)
			if anim != nil {
				ed.Animations[anim.Name] = anim
			}
		case "WEAK_SPOT":
			partName := child.Name
			mult := parseParamFloat(child.Name, "multiplier", 3.0)
			// WEAK_SPOT HEAD multiplier=3.0 → Name may be "HEAD multiplier=3.0"
			if idx := indexField(child.Name, "multiplier"); idx > 0 {
				partName = child.Name[:idx]
				mult = parseParamFloat(child.Name, "multiplier", 3.0)
			}
			ed.WeakSpots[strings.TrimSpace(partName)] = mult
		case "ARMOR":
			partName := child.Name
			reduction := parseParamFloat(child.Name, "reduction", 0.5)
			if idx := indexField(child.Name, "reduction"); idx > 0 {
				partName = child.Name[:idx]
				reduction = parseParamFloat(child.Name, "reduction", 0.5)
			}
			ed.Armor[strings.TrimSpace(partName)] = reduction
		}
	}

	return ed
}

// --- Entity Instance ---

// Entity is a live instance of an EntityDef in the game world.
type Entity struct {
	Def     *EntityDef
	ID      string
	X, Y, Z float64
	Angle   float64

	HP      float64
	HPParts map[string]float64 // current HP per body part
	Alive   bool
	Frozen  bool

	CurrentAnim string
	AnimFrame   int
	AnimTimer   float64

	// Physics.
	Rigidbody *systems.Rigidbody

	// Death state.
	DeathBehavior *DeathBehaviorState

	// For creatures: rider entity ID (if being ridden).
	RiderID string
}

// DeathBehaviorState tracks death animation progress.
type DeathBehaviorState struct {
	Type     DeathBehaviorType
	Timer    float64
	Duration float64
	Active   bool
	Ragdoll  *systems.Ragdoll
}

// DeathBehaviorType mirrors the systems package enum locally.
type DeathBehaviorType int

const (
	DeathRagdoll DeathBehaviorType = iota
	DeathDissolve
	DeathExplode
	DeathCollapse
	DeathShatter
	DeathSink
	DeathEvaporate
)

// SpawnEntity creates a live entity from a definition.
func SpawnEntity(def *EntityDef, id string, x, y, z, angle float64) *Entity {
	e := &Entity{
		Def:     def,
		ID:      id,
		X:       x,
		Y:       y,
		Z:       z,
		Angle:   angle,
		HP:      def.HP,
		HPParts: make(map[string]float64),
		Alive:   true,
	}

	// Copy HP parts.
	for part, hp := range def.HPParts {
		e.HPParts[part] = hp
	}

	// Create rigidbody.
	pos := systems.Vec3{X: x, Y: y, Z: z}
	e.Rigidbody = systems.NewRigidbody(pos, 70) // 70kg default

	return e
}

// --- Entity Registry ---

// EntityRegistry manages all spawned entities.
type EntityRegistry struct {
	entities map[string]*Entity
	defs     map[string]*EntityDef
}

// NewEntityRegistry creates an empty registry.
func NewEntityRegistry() *EntityRegistry {
	return &EntityRegistry{
		entities: make(map[string]*Entity),
		defs:     make(map[string]*EntityDef),
	}
}

// RegisterDef stores an entity definition for later spawning.
func (er *EntityRegistry) RegisterDef(def *EntityDef) {
	er.defs[def.Name] = def
}

// GetDef returns a definition by name.
func (er *EntityRegistry) GetDef(name string) *EntityDef {
	return er.defs[name]
}

// Spawn creates and registers a new entity from a named definition.
func (er *EntityRegistry) Spawn(defName, id string, x, y, z, angle float64) *Entity {
	def := er.defs[defName]
	if def == nil {
		return nil
	}
	e := SpawnEntity(def, id, x, y, z, angle)
	er.entities[id] = e
	return e
}

// Get returns a live entity by ID.
func (er *EntityRegistry) Get(id string) *Entity {
	return er.entities[id]
}

// Remove marks an entity as dead and removes it from the registry.
func (er *EntityRegistry) Remove(id string) {
	delete(er.entities, id)
}

// GetAll returns all alive entities.
func (er *EntityRegistry) GetAll() []*Entity {
	var result []*Entity
	for _, e := range er.entities {
		if e.Alive {
			result = append(result, e)
		}
	}
	return result
}

// CountAlive returns the number of alive entities.
func (er *EntityRegistry) CountAlive() int {
	count := 0
	for _, e := range er.entities {
		if e.Alive {
			count++
		}
	}
	return count
}

// --- Utility ---

func isBodyPartKey(k string) bool {
	switch k {
	case "HEAD", "TORSO", "ARMS", "LEGS", "TAIL",
		"BODY", "WINGS", "FANGS", "EYES", "ANTENNA",
		"BODY_SEGMENTS", "TENTACLE", "EXTRA_EYES",
		"DEFORMED_HEAD", "EXTRA_ARM", "SENSOR", "JOINT_VISIBLE",
		"MISSING_ARM", "EXPOSED_BONE",
		"BARREL", "GRIP", "WHEELS":
		return true
	}
	return false
}

func splitFields(s string) []string {
	var parts []string
	current := ""
	inQuote := false
	for _, ch := range s {
		if ch == '"' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && (ch == ' ' || ch == '\t') {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func parseInt(s string) (int, bool) {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + int(ch-'0')
	}
	return n, true
}

func parseFloat(s string) float64 {
	var result float64
	var frac float64 = 1.0
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

func findParam(s, param string) int {
	search := param + "="
	for i := 0; i <= len(s)-len(search); i++ {
		if s[i:i+len(search)] == search {
			return i
		}
	}
	return -1
}

func parseParamInt(s, param string, defaultVal int) int {
	search := param + "="
	idx := findParam(s, param)
	if idx < 0 {
		return defaultVal
	}
	start := idx + len(search)
	end := start
	for end < len(s) && s[end] != ' ' {
		end++
	}
	n, ok := parseInt(s[start:end])
	if !ok {
		return defaultVal
	}
	return n
}

func parseParamFloat(s, param string, defaultVal float64) float64 {
	search := param + "="
	idx := findParam(s, param)
	if idx < 0 {
		return defaultVal
	}
	start := idx + len(search)
	end := start
	for end < len(s) && s[end] != ' ' {
		end++
	}
	return parseFloat(s[start:end])
}

func indexField(s, field string) int {
	for i := 0; i <= len(s)-len(field); i++ {
		if s[i:i+len(field)] == field {
			return i
		}
	}
	return -1
}
