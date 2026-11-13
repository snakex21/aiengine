// scripting/engine.go — Shared engine state accessible from both Lua and JS scripts.
// Holds references to all game systems and provides a unified API for scripting bridges.
// Script variables, event listeners, and game state are managed here.

package scripting

import (
	"fmt"
	"log"
	"sort"
	"sync"

	"aiengine/renderer"
)

// Engine is the central state hub for AIEngine scripting.
// Both Lua and JS bridges access the same Engine instance.
type Engine struct {
	mu sync.RWMutex

	// Player state.
	PlayerHP      float64
	PlayerMaxHP   float64
	PlayerX       float64
	PlayerY       float64
	PlayerZ       float64
	PlayerAngle   float64
	PlayerSpeed   float64
	PlayerFrozen  bool
	PlayerAmmo    map[string]int // weapon name → current ammo
	PlayerWeapons []string

	// HUD display values (updated by JS UI.setHP/setAmmo).
	HUDAmmo    int
	HUDAmmoMax int

	// Entities (simple registry for now; will be replaced by gameplay/entities later).
	Entities map[string]*EntityState

	// Map.
	MapName   string
	MapData   *renderer.MapData
	Raycaster *renderer.Raycaster
	Sprites   *[]*renderer.Sprite

	// Triggers.
	Triggers map[string]*TriggerState

	// Script variables (key-value store shared between Lua and JS).
	Vars map[string]interface{}

	// Timers.
	Timers map[string]*TimerState

	// gameTime is simulated time in seconds; it only advances through
	// Advance, so it stops while the game is paused or in a menu.
	gameTime float64
	spawnSeq int

	// Audio placeholders.
	CurrentMusic string
	Volume       float64
	AudioPlayer  AudioPlayer // interface to avoid circular import

	// UI state.
	HUDVisible     bool
	MessageText    string
	MessageVisible bool
	MessageTTL     float64 // seconds left before the message auto-hides (0 = sticky)
	CurrentMenu    string

	// HUDText holds named HUD text lines set by scripts (setHUDText). The
	// renderer draws them sorted by key, so scripts control the order.
	HUDText map[string]string

	// EndText is the optional headline passed to winGame/loseGame.
	EndText string

	// Effects.
	Effects *renderer.EffectsConfig

	// Event listeners.
	listeners map[string][]func(args ...interface{})

	// Callbacks for engine-level actions.
	OnWin       func()
	OnLose      func()
	OnMapChange func(mapName string)
	OnRestart   func()                     // restartMap()
	OnExit      func()                     // exitGame()
	OnDoor      func(id string, open bool) // unlockDoor/lockDoor
}

// quietEvents are emitted every frame; logging them would flood the log.
var quietEvents = map[string]bool{"onUpdate": true}

// EntityState holds minimal entity data for scripting.
type EntityState struct {
	ID      string
	Type    string
	HP      float64
	MaxHP   float64
	X, Y, Z float64
	Frozen  bool
	Alive   bool
}

// TriggerState holds trigger condition and action data.
type TriggerState struct {
	Name      string
	Condition string
	Actions   []string
	Triggered bool
}

// TimerState tracks a named timer.
type TimerState struct {
	Name     string
	Duration float64
	StartAt  float64 // game time (seconds) when the timer was started
	Active   bool
}

// NewEngine creates a new Engine with initialized state.
func NewEngine() *Engine {
	return &Engine{
		PlayerHP:      100,
		PlayerMaxHP:   100,
		PlayerSpeed:   1.0,
		PlayerAmmo:    make(map[string]int),
		PlayerWeapons: []string{"pistol"},
		Entities:      make(map[string]*EntityState),
		Triggers:      make(map[string]*TriggerState),
		Vars:          make(map[string]interface{}),
		Timers:        make(map[string]*TimerState),
		HUDText:       make(map[string]string),
		HUDVisible:    true,
		Volume:        1.0,
		listeners:     make(map[string][]func(args ...interface{})),
	}
}

// --- Event System ---

// On registers an event listener.
func (e *Engine) On(event string, fn func(args ...interface{})) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners[event] = append(e.listeners[event], fn)
}

// Emit fires an event to all registered listeners (synchronous — no goroutines).
// Listeners are called in registration order on the calling goroutine.
// If a listener panics, the panic is caught and logged so other listeners still fire.
func (e *Engine) Emit(event string, args ...interface{}) {
	e.mu.RLock()
	listeners := e.listeners[event]
	e.mu.RUnlock()

	if !quietEvents[event] {
		log.Printf("[Event] %s (listeners: %d)", event, len(listeners))
	}

	for _, fn := range listeners {
		func(f func(args ...interface{})) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Event] %s panic: %v", event, r)
				}
			}()
			f(args...)
		}(fn)
	}
}

// --- Player API ---

// GetPlayerHP returns current player health.
func (e *Engine) GetPlayerHP() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.PlayerHP
}

// SetPlayerHP sets player health and emits onPlayerDamage/onPlayerDead events.
func (e *Engine) SetPlayerHP(hp float64) {
	e.mu.Lock()
	oldHP := e.PlayerHP
	e.PlayerHP = hp
	if e.PlayerHP > e.PlayerMaxHP {
		e.PlayerHP = e.PlayerMaxHP
	}
	if e.PlayerHP < 0 {
		e.PlayerHP = 0
	}
	e.mu.Unlock()

	if hp < oldHP {
		e.Emit("onPlayerDamage", oldHP-hp)
	}
	if e.PlayerHP <= 0 {
		e.Emit("onPlayerDead")
		if e.OnLose != nil {
			e.OnLose()
		}
	}
}

// GetPlayerPosition returns (x, y, z).
func (e *Engine) GetPlayerPosition() (float64, float64, float64) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.PlayerX, e.PlayerY, e.PlayerZ
}

// SetPlayerPosition sets player world position.
func (e *Engine) SetPlayerPosition(x, y, z float64) {
	e.mu.Lock()
	e.PlayerX = x
	e.PlayerY = y
	e.PlayerZ = z
	e.mu.Unlock()
}

// GetPlayerAmmo returns current ammo for a weapon.
func (e *Engine) GetPlayerAmmo(weapon string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.PlayerAmmo[weapon]
}

// SetPlayerAmmo sets ammo count for a weapon.
func (e *Engine) SetPlayerAmmo(weapon string, ammo int) {
	e.mu.Lock()
	e.PlayerAmmo[weapon] = ammo
	e.mu.Unlock()
}

// GiveWeapon adds a weapon to the player's inventory.
func (e *Engine) GiveWeapon(weaponType string) {
	e.mu.Lock()
	// Check if already owned.
	for _, w := range e.PlayerWeapons {
		if w == weaponType {
			e.mu.Unlock()
			return
		}
	}
	e.PlayerWeapons = append(e.PlayerWeapons, weaponType)
	e.PlayerAmmo[weaponType] = 30 // default ammo
	e.mu.Unlock()
}

// RemoveWeapon removes a weapon from inventory.
func (e *Engine) RemoveWeapon(weaponType string) {
	e.mu.Lock()
	for i, w := range e.PlayerWeapons {
		if w == weaponType {
			e.PlayerWeapons = append(e.PlayerWeapons[:i], e.PlayerWeapons[i+1:]...)
			break
		}
	}
	e.mu.Unlock()
}

// FreezePlayer disables player movement.
func (e *Engine) FreezePlayer() {
	e.mu.Lock()
	e.PlayerFrozen = true
	e.mu.Unlock()
}

// UnfreezePlayer enables player movement.
func (e *Engine) UnfreezePlayer() {
	e.mu.Lock()
	e.PlayerFrozen = false
	e.mu.Unlock()
}

// SetPlayerSpeed changes player movement speed multiplier.
func (e *Engine) SetPlayerSpeed(speed float64) {
	e.mu.Lock()
	e.PlayerSpeed = speed
	e.mu.Unlock()
}

// --- Entity (Enemy) API ---

// SpawnEnemy creates a new enemy entity.
func (e *Engine) SpawnEnemy(entityType string, x, y, z float64) string {
	e.mu.Lock()
	e.spawnSeq++
	id := fmt.Sprintf("%s_%d", entityType, e.spawnSeq)
	for e.Entities[id] != nil { // never clobber an existing entity
		e.spawnSeq++
		id = fmt.Sprintf("%s_%d", entityType, e.spawnSeq)
	}
	e.mu.Unlock()
	e.mu.Lock()
	e.Entities[id] = &EntityState{
		ID:    id,
		Type:  entityType,
		HP:    100,
		MaxHP: 100,
		X:     x, Y: y, Z: z,
		Alive: true,
	}
	e.mu.Unlock()
	e.Emit("onEnemySpawn", id, entityType, x, y, z)
	return id
}

// KillEnemy marks an enemy as dead and emits event.
func (e *Engine) KillEnemy(id string) {
	e.mu.Lock()
	ent, ok := e.Entities[id]
	if !ok {
		e.mu.Unlock()
		return
	}
	ent.Alive = false
	entType := ent.Type
	e.mu.Unlock()
	e.Emit("onEnemyDead", id, entType)
}

// GetEnemyHP returns the HP of an enemy.
func (e *Engine) GetEnemyHP(id string) float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if ent, ok := e.Entities[id]; ok {
		return ent.HP
	}
	return 0
}

// SetEnemyHP sets enemy HP and emits damage/death events.
func (e *Engine) SetEnemyHP(id string, hp float64) {
	e.mu.Lock()
	ent, ok := e.Entities[id]
	if !ok {
		e.mu.Unlock()
		return
	}
	oldHP := ent.HP
	ent.HP = hp
	if ent.HP <= 0 {
		ent.HP = 0
		ent.Alive = false
	}
	entType := ent.Type
	e.mu.Unlock()

	if hp < oldHP {
		e.Emit("onEnemyDamage", id, entType, oldHP-hp)
	}
	if ent.HP <= 0 {
		e.Emit("onEnemyDead", id, entType)
	}
}

// AllEnemiesDeadInZone checks if all enemies in a zone are dead (stub).
func (e *Engine) AllEnemiesDeadInZone(zone string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	// Stub: check all entities; zone filtering will be added later.
	for _, ent := range e.Entities {
		if ent.Alive {
			// TODO: zone check.
			return false
		}
	}
	return true
}

// GetEnemyCountInZone returns enemy count in a zone (stub).
func (e *Engine) GetEnemyCountInZone(zone string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	count := 0
	for _, ent := range e.Entities {
		if ent.Alive {
			count++
		}
	}
	return count
}

// MoveEnemy moves an enemy to a new position.
func (e *Engine) MoveEnemy(id string, x, y, z float64) {
	e.mu.Lock()
	if ent, ok := e.Entities[id]; ok {
		ent.X = x
		ent.Y = y
		ent.Z = z
	}
	e.mu.Unlock()
}

// FreezeEnemy disables enemy AI.
func (e *Engine) FreezeEnemy(id string) {
	e.mu.Lock()
	if ent, ok := e.Entities[id]; ok {
		ent.Frozen = true
	}
	e.mu.Unlock()
}

// UnfreezeEnemy enables enemy AI.
func (e *Engine) UnfreezeEnemy(id string) {
	e.mu.Lock()
	if ent, ok := e.Entities[id]; ok {
		ent.Frozen = false
	}
	e.mu.Unlock()
}

// --- Map API ---

// UnlockDoor opens a door. The host (level runtime) applies it to the map via
// OnDoor; without a host it only emits the event.
func (e *Engine) UnlockDoor(id string) {
	log.Printf("[Script] UnlockDoor: %s", id)
	if e.OnDoor != nil {
		e.OnDoor(id, true)
	}
	e.Emit("onDoorOpen", id)
}

// LockDoor closes a door via OnDoor.
func (e *Engine) LockDoor(id string) {
	log.Printf("[Script] LockDoor: %s", id)
	if e.OnDoor != nil {
		e.OnDoor(id, false)
	}
}

// --- Game flow API ---

// WinGame ends the game with a win. text is an optional end-screen headline.
func (e *Engine) WinGame(text string) {
	e.mu.Lock()
	e.EndText = text
	e.mu.Unlock()
	e.Emit("onWin")
	if e.OnWin != nil {
		e.OnWin()
	}
}

// LoseGame ends the game with a loss. text is an optional end-screen headline.
func (e *Engine) LoseGame(text string) {
	e.mu.Lock()
	e.EndText = text
	e.mu.Unlock()
	e.Emit("onLose")
	if e.OnLose != nil {
		e.OnLose()
	}
}

// RestartMap asks the host to reload the current level.
func (e *Engine) RestartMap() {
	if e.OnRestart != nil {
		e.OnRestart()
	}
}

// ExitGame asks the host to close the game.
func (e *Engine) ExitGame() {
	if e.OnExit != nil {
		e.OnExit()
	}
}

// AudioPlayer is the interface for audio playback (implemented by systems.AudioSystem).
type AudioPlayer interface {
	PlaySound(name string)
	StopAll()
}

// --- Sound API ---

// PlaySound plays a sound effect.
func (e *Engine) PlaySound(file string) {
	log.Printf("[Script] PlaySound: %s", file)
	if e.AudioPlayer != nil {
		e.AudioPlayer.PlaySound(file)
	}
}

// StopSound stops all sounds.
func (e *Engine) StopSound(file string) {
	log.Printf("[Script] StopSound: %s", file)
	if e.AudioPlayer != nil {
		e.AudioPlayer.StopAll()
	}
}

// MusicPlayer is implemented by audio players that can loop music.
type MusicPlayer interface {
	PlayMusicTrack(name string)
	StopMusicTrack()
}

// PlayMusic starts looping background music (a SOUND name or audio file)
// when the audio player supports it.
func (e *Engine) PlayMusic(file string) {
	e.mu.Lock()
	e.CurrentMusic = file
	e.mu.Unlock()
	log.Printf("[Script] PlayMusic: %s", file)
	if mp, ok := e.AudioPlayer.(MusicPlayer); ok {
		mp.PlayMusicTrack(file)
	}
}

// StopMusic stops background music.
func (e *Engine) StopMusic() {
	e.mu.Lock()
	e.CurrentMusic = ""
	e.mu.Unlock()
	log.Printf("[Script] StopMusic")
	if mp, ok := e.AudioPlayer.(MusicPlayer); ok {
		mp.StopMusicTrack()
	}
}

// ChangeMusic transitions to new music (placeholder).
func (e *Engine) ChangeMusic(file string) {
	e.PlayMusic(file)
}

// SetVolume sets global audio volume.
func (e *Engine) SetVolume(vol float64) {
	e.mu.Lock()
	e.Volume = vol
	e.mu.Unlock()
}

// --- UI API ---

// ShowMessage displays a HUD message until hidden or replaced.
func (e *Engine) ShowMessage(text string) {
	e.ShowMessageFor(text, 0)
}

// ShowMessageFor displays a HUD message that auto-hides after seconds (0 = sticky).
func (e *Engine) ShowMessageFor(text string, seconds float64) {
	if seconds != seconds || seconds < 0 { // NaN (e.g. JS undefined) or negative
		seconds = 0
	}
	e.mu.Lock()
	e.MessageText = text
	e.MessageVisible = true
	e.MessageTTL = seconds
	e.mu.Unlock()
	log.Printf("[Script] ShowMessage: %s", text)
}

// TickMessage counts down a timed message; call once per frame with dt.
func (e *Engine) TickMessage(dt float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.MessageVisible && e.MessageTTL > 0 {
		e.MessageTTL -= dt
		if e.MessageTTL <= 0 {
			e.MessageTTL = 0
			e.MessageVisible = false
		}
	}
}

// CurrentMessage returns the visible HUD message, or "".
func (e *Engine) CurrentMessage() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if !e.MessageVisible {
		return ""
	}
	return e.MessageText
}

// SetHUDText sets (or with empty text, clears) a named HUD text line.
func (e *Engine) SetHUDText(key, text string) {
	e.mu.Lock()
	if text == "" {
		delete(e.HUDText, key)
	} else {
		e.HUDText[key] = text
	}
	e.mu.Unlock()
}

// HUDLines returns the HUD text lines ordered by key.
func (e *Engine) HUDLines() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	keys := make([]string, 0, len(e.HUDText))
	for k := range e.HUDText {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, e.HUDText[k])
	}
	return lines
}

// HideMessage hides the HUD message.
func (e *Engine) HideMessage() {
	e.mu.Lock()
	e.MessageVisible = false
	e.mu.Unlock()
}

// SetHUDVisible toggles the entire HUD.
func (e *Engine) SetHUDVisible(visible bool) {
	e.mu.Lock()
	e.HUDVisible = visible
	e.mu.Unlock()
}

// DefaultMessageSeconds is how long script messages stay up when the script
// does not pass a duration (pass 0 for a sticky message).
const DefaultMessageSeconds = 4.0

// SetFilter sets a screen effect by name (shared by Lua setFilter and JS
// UI.setFilter). Unknown names are ignored.
func (e *Engine) SetFilter(name string, v float64) {
	if e.Effects == nil || v != v {
		return
	}
	switch name {
	case "chromatic_aberration":
		e.Effects.ChromaticAberration = v
	case "vignette":
		e.Effects.Vignette = v
	case "blur":
		e.Effects.BlurAmount = v
	case "red_flash", "red_tint":
		e.Effects.RedTint = v
	case "noise", "film_grain":
		e.Effects.FilmGrain = v
	case "desaturate":
		e.Effects.Desaturate = v
	case "brightness":
		e.Effects.Brightness = v
	default:
		log.Printf("[Script] setFilter: unknown filter %q", name)
	}
}

// --- Variable API ---

// GetVar retrieves a script variable.
func (e *Engine) GetVar(name string) interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.Vars[name]
}

// SetVar stores a script variable.
func (e *Engine) SetVar(name string, value interface{}) {
	e.mu.Lock()
	e.Vars[name] = value
	e.mu.Unlock()
}

// VarsSnapshot returns a copy of all script variables.
func (e *Engine) VarsSnapshot() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make(map[string]interface{}, len(e.Vars))
	for k, v := range e.Vars {
		out[k] = v
	}
	return out
}

// --- Timer API ---

// StartTimer creates and starts a named timer.
func (e *Engine) StartTimer(name string, duration float64) {
	e.mu.Lock()
	e.Timers[name] = &TimerState{
		Name:     name,
		Duration: duration,
		StartAt:  e.gameTime,
		Active:   true,
	}
	e.mu.Unlock()
}

// StopTimer stops a named timer.
func (e *Engine) StopTimer(name string) {
	e.mu.Lock()
	if t, ok := e.Timers[name]; ok {
		t.Active = false
	}
	e.mu.Unlock()
}

// IsTimerElapsed checks if a timer has expired.
func (e *Engine) IsTimerElapsed(name string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	t, ok := e.Timers[name]
	if !ok || !t.Active {
		return false
	}
	return e.gameTime-t.StartAt >= t.Duration
}

// GetTime returns the game time in seconds: time the simulation has run,
// excluding pauses and menus (it used to be the wall clock).
func (e *Engine) GetTime() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.gameTime
}

// Advance moves game time forward by dt seconds and fires elapsed timers
// (onTimer). The host calls it once per simulated frame and not while paused.
func (e *Engine) Advance(dt float64) {
	if dt > 0 {
		e.mu.Lock()
		e.gameTime += dt
		e.mu.Unlock()
	}
	e.TickTimers()
}

// --- Camera API (callbacks to renderer) ---

// OnMoveCamera is a callback for camera movement (placeholder).
var OnMoveCamera func(x, y, z, duration float64)

// OnShakeCamera is a callback for camera shake (placeholder).
var OnShakeCamera func(intensity, duration float64)

// OnLookAt is a callback for camera look target (placeholder).
var OnLookAt func(x, y, z, duration float64)

// --- Timer Tick ---

// TickTimers emits onTimer for timers that have elapsed in game time. It does
// not advance time; use Advance from the game loop.
func (e *Engine) TickTimers() {
	e.mu.Lock()
	var elapsed []string
	for name, t := range e.Timers {
		if t.Active && e.gameTime-t.StartAt >= t.Duration {
			elapsed = append(elapsed, name)
			t.Active = false
		}
	}
	e.mu.Unlock()

	for _, name := range elapsed {
		e.Emit("onTimer", name)
	}
}

// SyncPlayerFromRaycaster copies player position/angle from the renderer's player state.
func (e *Engine) SyncPlayerFromRaycaster(player *renderer.PlayerState) {
	e.mu.Lock()
	e.PlayerX = player.X
	e.PlayerY = player.Y
	e.PlayerAngle = player.Angle
	e.mu.Unlock()
}

// SyncPlayerToRaycaster copies script-driven position back to the raycaster player.
func (e *Engine) SyncPlayerToRaycaster(player *renderer.PlayerState) {
	e.mu.RLock()
	player.X = e.PlayerX
	player.Y = e.PlayerY
	e.mu.RUnlock()
}
