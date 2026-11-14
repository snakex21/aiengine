// Package weapons holds weapon definitions (WEAPON blocks in .aie files),
// the live state of a held weapon (magazine, reload, fire-rate cooldown) and
// a loadout (the weapons and ammo pools of one holder).
//
// All timers run on game time: the host calls Update(dt) once per simulated
// frame and not while the game is paused, so pausing also pauses the fire
// rate and reloads.
//
// Nothing here knows who holds a weapon: the player's loadout, an enemy's
// single weapon and (later) a vehicle turret use the same types; the game
// decides where shots start.
package weapons

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"aiengine/parser"
)

// Kind is how a weapon delivers damage.
type Kind int

const (
	Hitscan    Kind = iota // instant rays (bullets, bolts, lasers)
	Projectile             // flying objects (fireballs, rockets, magic)
)

func (k Kind) String() string {
	if k == Projectile {
		return "projectile"
	}
	return "hitscan"
}

// Def is a weapon template.
type Def struct {
	Name  string
	Title string // name shown on the HUD (default: Name)
	Kind  Kind
	Slot  int // number key that selects it (0 = only by cycling)

	Damage  float64
	Range   float64 // hitscan reach / projectile max distance (map units)
	Spread  float64 // cone half-angle, radians (.aie: degrees)
	Pellets int     // rays or projectiles per shot, at least 1

	FireRate    float64 // shots per second
	Auto        bool    // keeps firing while the trigger is held
	Magazine    int     // rounds per magazine (0 = fires straight from the pool)
	ReloadTime  float64 // seconds
	Ammo        string  // ammo type drawn from the holder's pool ("" = unlimited)
	AmmoPerShot int     // rounds per shot (default 1)
	StartAmmo   int     // rounds that come with the weapon (pool)
	MaxAmmo     int     // pool cap for this ammo type (0 = none)

	// Projectiles.
	Speed       float64 // map units per second
	Gravity     float64 // downward acceleration (units/s^2)
	Lifetime    float64 // seconds before it fizzles (or explodes when it splashes)
	Radius      float64 // collision radius
	Sprite      string  // sprite texture ("orb" = generated glowing ball)
	Scale       float64 // sprite size
	Color       string  // #RRGGBB tint of generated sprites, tracers and lights
	Light       float64 // light the projectile carries (level, 0 = none)
	LightRadius float64

	// Impact.
	Splash        float64 // explosion radius (0 = direct hits only)
	SplashFalloff float64 // fraction of damage at the edge of the splash (0..1)
	Knockback     float64 // push speed given to what it hits

	// Effects and sounds.
	Tracer       bool    // hitscan rays leave a short glowing trail
	MuzzleLight  float64 // camera/shooter light when firing (0..1)
	View         string  // first-person sprite ("" or "player" = the project's hand frames)
	SoundFire    string
	SoundHit     string
	SoundReload  string
	SoundEmpty   string
	SoundExplode string
}

// Default values of a WEAPON block.
func newDef(name string) *Def {
	return &Def{Name: name, Range: 50, FireRate: 1, ReloadTime: 1.5, Pellets: 1, AmmoPerShot: 1,
		Speed: 10, Lifetime: 3, Radius: 0.1, Scale: 0.3, LightRadius: 2.5, Sprite: "orb"}
}

// ParseDef builds a Def from a WEAPON node. Keys (case-insensitive):
//
//	TITLE text           TYPE hitscan|projectile   SLOT n
//	DAMAGE n             RANGE n                   SPREAD degrees
//	PELLETS n (BURST)    FIRE_RATE shots/s         AUTO_FIRE yes|no
//	MAGAZINE n (AMMO_MAX)  RELOAD_TIME s           AMMO type|none
//	AMMO_PER_SHOT n      START_AMMO n (AMMO_RESERVE)  MAX_AMMO n
//	PROJECTILE_SPEED, PROJECTILE_GRAVITY, PROJECTILE_LIFETIME,
//	PROJECTILE_RADIUS, PROJECTILE_SPRITE, PROJECTILE_SCALE,
//	PROJECTILE_LIGHT level, PROJECTILE_LIGHT_RADIUS cells, COLOR #RRGGBB
//	SPLASH_RADIUS n      SPLASH_FALLOFF 0..1       KNOCKBACK n
//	TRACER yes|no        MUZZLE_LIGHT 0..1         VIEW sprite
//	SOUND_FIRE, SOUND_HIT, SOUND_RELOAD, SOUND_EMPTY, SOUND_EXPLODE
//
// It reports unknown keys and bad values as an error (the Def is still
// usable).
func ParseDef(node *parser.AIENode) (*Def, error) {
	return defFromMap(node.Name, Flatten(node))
}

func defFromMap(name string, kv map[string]string) (*Def, error) {
	d := newDef(name)
	var errs []string
	num := func(dst *float64, v string) {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			errs = append(errs, fmt.Sprintf("bad number %q", v))
			return
		}
		*dst = f
	}
	whole := func(dst *int, v string) {
		n, err := strconv.Atoi(v)
		if err != nil {
			errs = append(errs, fmt.Sprintf("bad integer %q", v))
			return
		}
		*dst = n
	}
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := kv[k]
		switch k {
		case "TITLE":
			d.Title = v
		case "TYPE":
			switch strings.ToLower(v) {
			case "hitscan", "":
				d.Kind = Hitscan
			case "projectile":
				d.Kind = Projectile
			default:
				errs = append(errs, fmt.Sprintf("TYPE %q (hitscan or projectile)", v))
			}
		case "SLOT":
			whole(&d.Slot, v)
		case "DAMAGE":
			num(&d.Damage, v)
		case "RANGE":
			num(&d.Range, v)
		case "SPREAD":
			var deg float64
			num(&deg, v)
			d.Spread = deg * math.Pi / 180
		case "PELLETS", "BURST":
			whole(&d.Pellets, v)
		case "FIRE_RATE":
			num(&d.FireRate, v)
		case "AUTO_FIRE":
			d.Auto = isYes(v)
		case "MAGAZINE", "AMMO_MAX":
			whole(&d.Magazine, v)
		case "RELOAD_TIME":
			num(&d.ReloadTime, v)
		case "AMMO":
			if strings.EqualFold(v, "none") {
				v = ""
			}
			d.Ammo = v
		case "AMMO_PER_SHOT":
			whole(&d.AmmoPerShot, v)
		case "START_AMMO", "AMMO_RESERVE":
			whole(&d.StartAmmo, v)
		case "MAX_AMMO":
			whole(&d.MaxAmmo, v)
		case "PROJECTILE_SPEED":
			num(&d.Speed, v)
		case "PROJECTILE_GRAVITY":
			num(&d.Gravity, v)
		case "PROJECTILE_LIFETIME":
			num(&d.Lifetime, v)
		case "PROJECTILE_RADIUS":
			num(&d.Radius, v)
		case "PROJECTILE_SPRITE":
			d.Sprite = v
		case "PROJECTILE_SCALE":
			num(&d.Scale, v)
		case "PROJECTILE_LIGHT":
			num(&d.Light, v)
		case "PROJECTILE_LIGHT_RADIUS":
			num(&d.LightRadius, v)
		case "COLOR":
			d.Color = v
		case "SPLASH_RADIUS":
			num(&d.Splash, v)
		case "SPLASH_FALLOFF":
			num(&d.SplashFalloff, v)
		case "KNOCKBACK":
			num(&d.Knockback, v)
		case "TRACER":
			d.Tracer = isYes(v)
		case "MUZZLE_LIGHT":
			num(&d.MuzzleLight, v)
		case "VIEW":
			d.View = v
		case "SOUND_FIRE":
			d.SoundFire = v
		case "SOUND_HIT":
			d.SoundHit = v
		case "SOUND_RELOAD":
			d.SoundReload = v
		case "SOUND_EMPTY":
			d.SoundEmpty = v
		case "SOUND_EXPLODE":
			d.SoundExplode = v
		default:
			errs = append(errs, "unknown key "+k)
		}
	}
	if d.Pellets < 1 {
		d.Pellets = 1
	}
	if d.AmmoPerShot < 1 {
		d.AmmoPerShot = 1
	}
	d.SplashFalloff = math.Max(0, math.Min(1, d.SplashFalloff))
	if len(errs) > 0 {
		return d, fmt.Errorf("WEAPON %s: %s", name, strings.Join(errs, "; "))
	}
	return d, nil
}

// HUDTitle is the name shown on the HUD.
func (d *Def) HUDTitle() string {
	if d.Title != "" {
		return d.Title
	}
	return d.Name
}

// Format writes the definition as a WEAPON block (ParseDef reads it back).
func (d *Def) Format() string {
	var b strings.Builder
	f := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	yes := func(v bool) string {
		if v {
			return "yes"
		}
		return "no"
	}
	ammo := d.Ammo
	if ammo == "" {
		ammo = "none"
	}
	fmt.Fprintf(&b, "WEAPON %s\n", d.Name)
	line := func(k, v string) { fmt.Fprintf(&b, "  %s %s\n", k, v) }
	if d.Title != "" {
		line("TITLE", d.Title)
	}
	line("TYPE", d.Kind.String())
	line("SLOT", strconv.Itoa(d.Slot))
	line("DAMAGE", f(d.Damage))
	line("RANGE", f(d.Range))
	line("SPREAD", f(d.Spread*180/math.Pi))
	line("PELLETS", strconv.Itoa(d.Pellets))
	line("FIRE_RATE", f(d.FireRate))
	line("AUTO_FIRE", yes(d.Auto))
	line("MAGAZINE", strconv.Itoa(d.Magazine))
	line("RELOAD_TIME", f(d.ReloadTime))
	line("AMMO", ammo)
	line("AMMO_PER_SHOT", strconv.Itoa(d.AmmoPerShot))
	line("START_AMMO", strconv.Itoa(d.StartAmmo))
	line("MAX_AMMO", strconv.Itoa(d.MaxAmmo))
	line("PROJECTILE_SPEED", f(d.Speed))
	line("PROJECTILE_GRAVITY", f(d.Gravity))
	line("PROJECTILE_LIFETIME", f(d.Lifetime))
	line("PROJECTILE_RADIUS", f(d.Radius))
	line("PROJECTILE_SPRITE", d.Sprite)
	line("PROJECTILE_SCALE", f(d.Scale))
	line("PROJECTILE_LIGHT", f(d.Light))
	line("PROJECTILE_LIGHT_RADIUS", f(d.LightRadius))
	line("SPLASH_RADIUS", f(d.Splash))
	line("SPLASH_FALLOFF", f(d.SplashFalloff))
	line("KNOCKBACK", f(d.Knockback))
	line("TRACER", yes(d.Tracer))
	line("MUZZLE_LIGHT", f(d.MuzzleLight))
	for _, kv := range [][2]string{{"COLOR", d.Color}, {"VIEW", d.View}, {"SOUND_FIRE", d.SoundFire},
		{"SOUND_HIT", d.SoundHit}, {"SOUND_RELOAD", d.SoundReload}, {"SOUND_EMPTY", d.SoundEmpty},
		{"SOUND_EXPLODE", d.SoundExplode}} {
		if kv[1] != "" {
			line(kv[0], kv[1])
		}
	}
	return b.String()
}

// Flatten returns a node's properties and single-value children (the .aie
// parser turns some keys, e.g. TYPE, into children) keyed in upper case.
func Flatten(node *parser.AIENode) map[string]string {
	out := make(map[string]string, len(node.Properties)+len(node.Children))
	for k, v := range node.Properties {
		out[strings.ToUpper(k)] = strings.TrimSpace(v)
	}
	for _, c := range node.Children {
		out[strings.ToUpper(c.Type)] = strings.TrimSpace(c.Name)
	}
	return out
}

func isYes(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "yes", "true", "1", "tak", "on":
		return true
	}
	return false
}

// FromDocument collects the WEAPON blocks of a parsed .aie document.
func FromDocument(doc *parser.AIEDocument) (map[string]*Def, error) {
	out := map[string]*Def{}
	var errs []string
	for _, n := range doc.Nodes {
		if !n.IsWeapon() {
			continue
		}
		d, err := ParseDef(n)
		if err != nil {
			errs = append(errs, fmt.Sprintf("line %d: %v", n.Line, err))
		}
		out[d.Name] = d
	}
	if len(errs) > 0 {
		return out, fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return out, nil
}

// Instance is a weapon in someone's hands.
type Instance struct {
	Def       *Def
	Mag       int // rounds in the magazine
	Reloading bool

	reloadLeft float64 // seconds until the reload completes
	cooldown   float64 // seconds until the next shot is allowed
}

// NewInstance creates a weapon with a full magazine.
func NewInstance(def *Def) *Instance {
	return &Instance{Def: def, Mag: def.Magazine}
}

// Pool is the spare ammunition of a holder by ammo type. A nil pool (an
// enemy's weapon) never runs out.
type Pool map[string]int

func (p Pool) has(typ string, n int) bool { return p == nil || typ == "" || p[typ] >= n }

func (p Pool) take(typ string, n int) int {
	if p == nil || typ == "" {
		return n
	}
	n = min(n, p[typ])
	p[typ] -= n
	return n
}

// Cooldown is the time left before the next shot.
func (w *Instance) Cooldown() float64 { return math.Max(0, w.cooldown) }

// Ready reports whether a shot can be fired now.
func (w *Instance) Ready(pool Pool) bool {
	if w.Reloading || w.cooldown > 0 {
		return false
	}
	return w.loaded(pool)
}

// loaded reports whether a shot's rounds are available (ignoring timers).
func (w *Instance) loaded(pool Pool) bool {
	if w.Def.Magazine > 0 {
		return w.Mag >= w.Def.AmmoPerShot
	}
	return pool.has(w.Def.Ammo, w.Def.AmmoPerShot)
}

// Empty reports that the weapon cannot fire until it gets ammunition.
func (w *Instance) Empty(pool Pool) bool {
	return !w.loaded(pool) && !w.Reloading && !(w.Def.Magazine > 0 && pool.has(w.Def.Ammo, 1))
}

// Fire spends a shot's rounds and starts the cooldown. It returns the
// number of pellets fired (0 when the weapon is not ready). An empty
// magazine starts a reload when the pool has rounds.
func (w *Instance) Fire(pool Pool) int {
	if !w.Ready(pool) {
		return 0
	}
	if w.Def.Magazine > 0 {
		w.Mag -= w.Def.AmmoPerShot
	} else {
		pool.take(w.Def.Ammo, w.Def.AmmoPerShot)
	}
	if w.Def.FireRate > 0 {
		w.cooldown = 1 / w.Def.FireRate
	}
	if w.Def.Magazine > 0 && w.Mag < w.Def.AmmoPerShot {
		w.Reload(pool)
	}
	return w.Def.Pellets
}

// Reload starts a reload when the magazine is not full and the pool has
// rounds. It reports whether a reload started.
func (w *Instance) Reload(pool Pool) bool {
	if w.Reloading || w.Def.Magazine <= 0 || w.Mag >= w.Def.Magazine || !pool.has(w.Def.Ammo, 1) {
		return false
	}
	w.Reloading = true
	w.reloadLeft = w.Def.ReloadTime
	return true
}

// Update advances the cooldown and the reload by dt seconds of game time.
func (w *Instance) Update(dt float64, pool Pool) {
	if w.cooldown > 0 {
		w.cooldown -= dt
	}
	if !w.Reloading {
		return
	}
	w.reloadLeft -= dt
	if w.reloadLeft > 0 {
		return
	}
	w.Mag += pool.take(w.Def.Ammo, w.Def.Magazine-w.Mag)
	w.Reloading, w.reloadLeft = false, 0
}

// Loadout is what one holder carries: weapons (in slot order), the
// selected one and the ammo pools.
type Loadout struct {
	Weapons []*Instance
	Current int
	Ammo    Pool
	Max     map[string]int // pool caps by ammo type
}

// NewLoadout creates an empty loadout.
func NewLoadout() *Loadout { return &Loadout{Ammo: Pool{}, Max: map[string]int{}} }

// Has reports whether a weapon is carried.
func (l *Loadout) Has(name string) bool { return l.Find(name) != nil }

// Find returns a carried weapon by name.
func (l *Loadout) Find(name string) *Instance {
	for _, w := range l.Weapons {
		if w.Def.Name == name {
			return w
		}
	}
	return nil
}

// Give adds a weapon (with its start ammo); a weapon already carried only
// adds the ammo. It reports whether the weapon is new.
func (l *Loadout) Give(def *Def) bool {
	if def.MaxAmmo > 0 && def.Ammo != "" {
		l.Max[def.Ammo] = max(l.Max[def.Ammo], def.MaxAmmo)
	}
	l.AddAmmo(def.Ammo, def.StartAmmo)
	if l.Has(def.Name) {
		return false
	}
	cur := l.CurrentWeapon()
	l.Weapons = append(l.Weapons, NewInstance(def))
	sort.SliceStable(l.Weapons, func(i, j int) bool { return slotKey(l.Weapons[i].Def) < slotKey(l.Weapons[j].Def) })
	l.Current = 0
	for i, w := range l.Weapons {
		if w == cur {
			l.Current = i
		}
	}
	return true
}

func slotKey(d *Def) int {
	if d.Slot <= 0 {
		return 100
	}
	return d.Slot
}

// Remove drops a weapon.
func (l *Loadout) Remove(name string) {
	for i, w := range l.Weapons {
		if w.Def.Name == name {
			l.Weapons = append(l.Weapons[:i], l.Weapons[i+1:]...)
			if l.Current >= len(l.Weapons) {
				l.Current = max(0, len(l.Weapons)-1)
			}
			return
		}
	}
}

// AddAmmo adds rounds of a type (up to its cap) and returns how many fit.
func (l *Loadout) AddAmmo(typ string, n int) int {
	if typ == "" || n <= 0 {
		return 0
	}
	if c := l.Max[typ]; c > 0 {
		n = min(n, c-l.Ammo[typ])
	}
	if n <= 0 {
		return 0
	}
	l.Ammo[typ] += n
	return n
}

// CurrentWeapon returns the selected weapon (nil when unarmed).
func (l *Loadout) CurrentWeapon() *Instance {
	if l.Current < 0 || l.Current >= len(l.Weapons) {
		return nil
	}
	return l.Weapons[l.Current]
}

// SelectSlot selects the first weapon of a slot (repeated presses cycle
// through weapons sharing the slot). It reports whether the choice changed.
func (l *Loadout) SelectSlot(slot int) bool {
	var idx []int
	for i, w := range l.Weapons {
		if w.Def.Slot == slot {
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		return false
	}
	next := idx[0]
	for k, i := range idx {
		if i == l.Current {
			next = idx[(k+1)%len(idx)]
		}
	}
	return l.selectIndex(next)
}

// Select selects a weapon by name.
func (l *Loadout) Select(name string) bool {
	for i, w := range l.Weapons {
		if w.Def.Name == name {
			return l.selectIndex(i)
		}
	}
	return false
}

// Cycle selects the next (dir > 0) or previous weapon.
func (l *Loadout) Cycle(dir int) bool {
	n := len(l.Weapons)
	if n < 2 || dir == 0 {
		return false
	}
	step := 1
	if dir < 0 {
		step = n - 1
	}
	return l.selectIndex((l.Current + step) % n)
}

func (l *Loadout) selectIndex(i int) bool {
	if i == l.Current {
		return false
	}
	if w := l.CurrentWeapon(); w != nil && w.Reloading {
		w.Reloading, w.reloadLeft = false, 0 // switching cancels a reload
	}
	l.Current = i
	return true
}

// Update advances every carried weapon's cooldown and the current one's
// reload.
func (l *Loadout) Update(dt float64) {
	for _, w := range l.Weapons {
		w.Update(dt, l.Ammo)
	}
}

// Rounds returns the rounds of the current weapon: in the magazine (or the
// pool for weapons without one), the pool, and whether ammo is unlimited.
func (l *Loadout) Rounds() (loaded, spare int, unlimited bool) {
	w := l.CurrentWeapon()
	if w == nil {
		return 0, 0, false
	}
	if w.Def.Ammo == "" {
		return w.Mag, 0, true
	}
	if w.Def.Magazine > 0 {
		return w.Mag, l.Ammo[w.Def.Ammo], false
	}
	return l.Ammo[w.Def.Ammo], 0, false
}

// DefaultPistol is the weapon of projects that define none: a hitscan
// pistol with unlimited ammo.
func DefaultPistol() *Def {
	d := newDef("pistol")
	d.Title = "Pistol"
	d.Slot = 1
	d.Damage = 25
	d.Range = 20
	d.FireRate = 3
	d.Spread = 0.6 * math.Pi / 180
	d.MuzzleLight = 1
	d.SoundFire = "gunshot"
	return d
}
