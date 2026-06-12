// gameplay/weapons.go — Weapon system for AIEngine.
// Defines weapon types, firing mechanics (hitscan), ammo management,
// reload system, and damage calculation including per-body-part damage.
// All weapon properties loaded from .aie WEAPON definitions.

package gameplay

import (
	"math"
	"math/rand"
	"time"

	"aiengine/parser"
	"aiengine/systems"
)

// WeaponType enumerates weapon categories.
type WeaponType int

const (
	WpnMelee    WeaponType = iota
	WpnPistol
	WpnShotgun
	WpnRifle
	WpnSniper
	WpnExplosive
	WpnEnergy
	WpnCustom
)

// WeaponDef is a weapon template loaded from .aie.
type WeaponDef struct {
	Name         string
	Type         WeaponType
	Damage       float64
	DamageParts  map[string]float64 // per-body-part damage
	Range        float64
	FireRate     float64 // shots per second
	ReloadTime   float64
	AmmoMax      int // magazine size
	AmmoReserve  int // total spare ammo
	Spread       float64 // radians
	Recoil       float64
	BulletDrop   bool
	BulletDropGravity float64
	BulletSpeed  float64
	Penetration  int    // how many enemies it can pierce
	Noise        float64 // 0-1, lower = quieter
	AutoFire     bool
	Burst        int  // bullets per shot (shotgun pellets)
	Zoom         float64
	Weight       float64
}

// ParseWeaponDef creates a WeaponDef from a parsed .aie WEAPON node.
func ParseWeaponDef(node *parser.AIENode) *WeaponDef {
	wd := &WeaponDef{
		Name:        node.Name,
		DamageParts: make(map[string]float64),
		Range:       50,
		FireRate:    1.0,
		ReloadTime:  1.5,
		AmmoMax:     10,
		AmmoReserve: 30,
		BulletSpeed: 500,
		Penetration: 0,
		Burst:       1,
		Zoom:        1.0,
		Weight:      1.0,
	}

	// Read scalar properties.
	wd.Damage = node.GetPropertyFloat("DAMAGE")
	if v := node.GetPropertyFloat("RANGE"); v > 0 {
		wd.Range = v
	}
	if v := node.GetPropertyFloat("FIRE_RATE"); v > 0 {
		wd.FireRate = v
	}
	if v := node.GetPropertyFloat("RELOAD_TIME"); v > 0 {
		wd.ReloadTime = v
	}
	if v := node.GetPropertyInt("AMMO_MAX"); v > 0 {
		wd.AmmoMax = v
	}
	if v := node.GetPropertyInt("AMMO_RESERVE"); v > 0 {
		wd.AmmoReserve = v
	}
	wd.Spread = node.GetPropertyFloat("SPREAD")
	wd.Recoil = node.GetPropertyFloat("RECOIL")
	wd.BulletDrop = node.GetPropertyBool("BULLET_DROP")
	wd.BulletDropGravity = node.GetPropertyFloat("BULLET_DROP") // "BULLET_DROP yes gravity=9.8"
	// Actually BULLET_DROP line is "yes gravity=9.8". Let's handle it properly.
	if bd := node.GetProperty("BULLET_DROP"); bd != "" {
		wd.BulletDrop = true
		if g := parseParamFloat(bd, "gravity", 9.8); g > 0 {
			wd.BulletDropGravity = g
		}
	}
	wd.BulletSpeed = node.GetPropertyFloat("BULLET_SPEED")
	if wd.BulletSpeed == 0 {
		wd.BulletSpeed = 500
	}
	wd.Penetration = node.GetPropertyInt("PENETRATION")
	wd.Noise = node.GetPropertyFloat("NOISE")
	wd.AutoFire = node.GetPropertyBool("AUTO_FIRE")
	wd.Burst = node.GetPropertyInt("BURST")
	if wd.Burst == 0 {
		wd.Burst = 1
	}
	wd.Zoom = node.GetPropertyFloat("ZOOM")
	if wd.Zoom == 0 {
		wd.Zoom = 1.0
	}
	wd.Weight = node.GetPropertyFloat("WEIGHT")
	if wd.Weight == 0 {
		wd.Weight = 1.0
	}

	// Parse DAMAGE_PARTS child.
	if damageParts := node.FindChild("DAMAGE_PARTS"); damageParts != nil {
		for _, part := range damageParts.Children {
			wd.DamageParts[part.Type] = parseFloat(part.Name)
		}
	}

	// Determine weapon type from entity name or TYPE property.
	typeStr := node.GetProperty("TYPE")
	if typeStr == "" {
		typeStr = node.Name
	}
	wd.Type = parseWeaponType(typeStr)

	return wd
}

func parseWeaponType(s string) WeaponType {
	switch s {
	case "MELEE":
		return WpnMelee
	case "PISTOL":
		return WpnPistol
	case "SHOTGUN":
		return WpnShotgun
	case "RIFLE":
		return WpnRifle
	case "SNIPER":
		return WpnSniper
	case "EXPLOSIVE":
		return WpnExplosive
	case "ENERGY":
		return WpnEnergy
	case "CUSTOM":
		return WpnCustom
	default:
		return WpnPistol
	}
}

// --- Weapon Instance ---

// WeaponInstance is a live weapon held by the player.
type WeaponInstance struct {
	Def          *WeaponDef
	AmmoCurrent  int
	AmmoReserve  int
	Reloading    bool
	ReloadTimer  float64
	FireCooldown float64
	LastFired    time.Time
}

// NewWeaponInstance creates a weapon from its definition.
func NewWeaponInstance(def *WeaponDef) *WeaponInstance {
	return &WeaponInstance{
		Def:         def,
		AmmoCurrent: def.AmmoMax,
		AmmoReserve: def.AmmoReserve,
	}
}

// CanFire returns true if the weapon is ready to shoot.
func (wi *WeaponInstance) CanFire() bool {
	if wi.Reloading {
		return false
	}
	if wi.AmmoCurrent <= 0 {
		return false
	}
	elapsed := time.Since(wi.LastFired).Seconds()
	return elapsed >= 1.0/wi.Def.FireRate
}

// Fire attempts to shoot the weapon. Returns the number of bullets fired.
func (wi *WeaponInstance) Fire() int {
	if !wi.CanFire() {
		return 0
	}

	bullets := wi.Def.Burst
	if bullets < 1 {
		bullets = 1
	}
	if wi.AmmoCurrent < bullets {
		bullets = wi.AmmoCurrent
	}

	wi.AmmoCurrent -= bullets
	wi.LastFired = time.Now()

	if wi.AmmoCurrent <= 0 {
		wi.Reload()
	}

	return bullets
}

// Reload starts or queues a reload.
func (wi *WeaponInstance) Reload() {
	if wi.Reloading {
		return
	}
	if wi.AmmoCurrent >= wi.Def.AmmoMax {
		return // already full
	}
	if wi.AmmoReserve <= 0 {
		return // no spare ammo
	}

	wi.Reloading = true
	wi.ReloadTimer = wi.Def.ReloadTime
}

// Update advances weapon timers (reload, cooldown).
func (wi *WeaponInstance) Update(dt float64) {
	if wi.Reloading {
		wi.ReloadTimer -= dt
		if wi.ReloadTimer <= 0 {
			// Complete reload.
			needed := wi.Def.AmmoMax - wi.AmmoCurrent
			if wi.AmmoReserve < needed {
				needed = wi.AmmoReserve
			}
			wi.AmmoCurrent += needed
			wi.AmmoReserve -= needed
			wi.Reloading = false
			wi.ReloadTimer = 0
		}
	}
}

// --- Hitscan Trace ---

// HitscanResult holds the result of a weapon trace.
type HitscanResult struct {
	Hit       bool
	Entity    *Entity
	Distance  float64
	Point     systems.Vec3
	BodyPart  string // which body part was hit (if entity)
}

// TraceHitscan performs a hitscan from origin in direction.
// Checks against walls (physics world) and entities.
func TraceHitscan(
	origin systems.Vec3,
	direction systems.Vec3,
	maxRange float64,
	world *systems.PhysicsWorld,
	entities []*Entity,
	shooterID string,
) HitscanResult {
	ray := systems.Ray{
		Origin:    origin,
		Direction: direction.Normalize(),
	}

	// Check world collision first.
	worldHit := world.Raycast(ray)
	worldDist := maxRange
	if worldHit.Hit {
		worldDist = worldHit.Distance
	}

	// Check entity hits.
	var closestEnt *Entity
	closestDist := maxRange
	var closestPart string

	for _, e := range entities {
		if !e.Alive || e.ID == shooterID {
			continue
		}

		// Simple sphere-AABB test for each part.
		entPos := systems.Vec3{X: e.X, Y: e.Y, Z: e.Z}
		dist := entPos.Sub(origin).Length()
		if dist > maxRange {
			continue
		}

		// Check if ray passes near the entity.
		// Simplified: check if ray passes within 0.5 units of entity center.
		toEnt := entPos.Sub(origin)
		proj := toEnt.Dot(direction)
		if proj < 0 || proj > maxRange {
			continue
		}

		closestPoint := origin.Add(direction.Scale(proj))
		perpDist := entPos.Sub(closestPoint).Length()

		if perpDist < 0.5 && proj < closestDist {
			closestDist = proj
			closestEnt = e

			// Determine which body part was hit.
			closestPart = determineHitPart(e, closestPoint)
		}
	}

	// Determine actual hit.
	if closestEnt != nil && closestDist < worldDist {
		return HitscanResult{
			Hit:      true,
			Entity:   closestEnt,
			Distance: closestDist,
			Point:    origin.Add(direction.Scale(closestDist)),
			BodyPart: closestPart,
		}
	}

	if worldHit.Hit {
		return HitscanResult{
			Hit:      true,
			Distance: worldDist,
			Point:    worldHit.Point,
		}
	}

	return HitscanResult{Hit: false}
}

// determineHitPart guesses which body part was hit based on vertical position.
func determineHitPart(e *Entity, hitPoint systems.Vec3) string {
	relZ := hitPoint.Z - e.Z
	height := 1.8 // default humanoid height

	switch {
	case relZ > height*0.8:
		return "HEAD"
	case relZ > height*0.5:
		return "TORSO"
	case relZ > height*0.3:
		return "ARMS"
	default:
		return "LEGS"
	}
}

// --- Damage Calculation ---

// CalculateDamage computes final damage to an entity considering weak spots, armor, and per-part HP.
func CalculateDamage(baseDamage float64, partName string, def *EntityDef) float64 {
	damage := baseDamage

	// Check per-part damage from weapon.
	if def != nil {
		// Weak spot multiplier.
		if mult, ok := def.WeakSpots[partName]; ok {
			damage *= mult
		}
		// Armor reduction.
		if reduction, ok := def.Armor[partName]; ok {
			damage *= (1.0 - reduction)
		}
	}

	if damage < 1 {
		damage = 1
	}
	return damage
}

// ApplyDamage applies damage to an entity, handling per-part HP and death.
func ApplyDamage(entity *Entity, damage float64, partName string) {
	// Apply to global HP.
	entity.HP -= damage

	// Apply to per-part HP if tracked.
	if hp, ok := entity.HPParts[partName]; ok {
		entity.HPParts[partName] = hp - damage
		if entity.HPParts[partName] < 0 {
			entity.HPParts[partName] = 0
		}
	}

	if entity.HP <= 0 {
		entity.HP = 0
		entity.Alive = false
		// Trigger death behavior.
		entity.DeathBehavior = &DeathBehaviorState{
			Type:   entity.Def.Behavior.DeathType,
			Active: true,
		}
	}
}

// --- Spread Calculation ---

// ApplySpread adds random angular deviation to a direction vector.
func ApplySpread(direction systems.Vec3, spreadRadians float64) systems.Vec3 {
	if spreadRadians <= 0 {
		return direction.Normalize()
	}

	// Generate random angles.
	theta := rand.Float64() * 2 * math.Pi
	phi := rand.Float64() * spreadRadians

	// Compute perpendicular vectors to the direction.
	dir := direction.Normalize()
	up := systems.Vec3{X: 0, Y: 0, Z: 1}
	if math.Abs(dir.Dot(up)) > 0.99 {
		up = systems.Vec3{X: 0, Y: 1, Z: 0}
	}
	right := dir.Cross(up).Normalize()
	perpendicularUp := right.Cross(dir).Normalize()

	// Apply spread.
	spreadDir := dir.Scale(math.Cos(phi)).
		Add(right.Scale(math.Sin(phi) * math.Cos(theta))).
		Add(perpendicularUp.Scale(math.Sin(phi) * math.Sin(theta)))

	return spreadDir.Normalize()
}

// --- Player Weapon Manager ---

// WeaponManager manages the player's weapon inventory.
type WeaponManager struct {
	Weapons     []*WeaponInstance
	CurrentIdx  int
	defs        map[string]*WeaponDef
}

// NewWeaponManager creates a weapon manager with default weapon defs.
func NewWeaponManager() *WeaponManager {
	return &WeaponManager{
		defs: make(map[string]*WeaponDef),
	}
}

// RegisterDef stores a weapon definition.
func (wm *WeaponManager) RegisterDef(def *WeaponDef) {
	wm.defs[def.Name] = def
}

// GiveWeapon adds a weapon to the player's inventory.
func (wm *WeaponManager) GiveWeapon(name string) {
	def, ok := wm.defs[name]
	if !ok {
		return
	}
	// Check if already owned.
	for _, w := range wm.Weapons {
		if w.Def.Name == name {
			// Refill ammo.
			w.AmmoReserve += def.AmmoReserve
			return
		}
	}
	inst := NewWeaponInstance(def)
	wm.Weapons = append(wm.Weapons, inst)
	if len(wm.Weapons) == 1 {
		wm.CurrentIdx = 0
	}
}

// CurrentWeapon returns the currently selected weapon.
func (wm *WeaponManager) CurrentWeapon() *WeaponInstance {
	if wm.CurrentIdx < 0 || wm.CurrentIdx >= len(wm.Weapons) {
		return nil
	}
	return wm.Weapons[wm.CurrentIdx]
}

// NextWeapon cycles to the next weapon.
func (wm *WeaponManager) NextWeapon() {
	if len(wm.Weapons) == 0 {
		return
	}
	wm.CurrentIdx = (wm.CurrentIdx + 1) % len(wm.Weapons)
}

// Update advances all weapon timers.
func (wm *WeaponManager) Update(dt float64) {
	for _, w := range wm.Weapons {
		w.Update(dt)
	}
}

// CreateDefaultWeaponDefs returns a set of built-in weapon definitions.
func CreateDefaultWeaponDefs() map[string]*WeaponDef {
	return map[string]*WeaponDef{
		"pistol": {
			Name:        "pistol",
			Type:        WpnPistol,
			Damage:      25,
			Range:       30,
			FireRate:    2.0,
			ReloadTime:  1.5,
			AmmoMax:     15,
			AmmoReserve: 45,
			Spread:      0.02,
			BulletSpeed: 800,
			Noise:       0.3,
			Burst:       1,
		},
		"shotgun": {
			Name:        "shotgun",
			Type:        WpnShotgun,
			Damage:      15,
			Range:       10,
			FireRate:    1.0,
			ReloadTime:  2.5,
			AmmoMax:     8,
			AmmoReserve: 24,
			Spread:      0.15,
			BulletSpeed: 400,
			Noise:       0.8,
			Burst:       8,
		},
		"rifle": {
			Name:        "rifle",
			Type:        WpnRifle,
			Damage:      40,
			Range:       50,
			FireRate:    8.0,
			ReloadTime:  2.5,
			AmmoMax:     30,
			AmmoReserve: 90,
			Spread:      0.03,
			Recoil:      0.3,
			BulletSpeed: 900,
			Noise:       0.5,
			AutoFire:    true,
			Burst:       1,
		},
		"sniper": {
			Name:        "sniper",
			Type:        WpnSniper,
			Damage:      150,
			Range:       200,
			FireRate:    0.7,
			ReloadTime:  3.0,
			AmmoMax:     5,
			AmmoReserve: 20,
			BulletDrop:  true,
			BulletDropGravity: 9.8,
			BulletSpeed: 900,
			Zoom:        4.0,
			Noise:       0.1,
			Weight:      5.0,
			Burst:       1,
		},
	}
}
