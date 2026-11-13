// level_runtime.go - data-driven "level" runtime.
//
// Enabled per project with `RUNTIME level` in manifest.aie. The map, spawn,
// items, zones, doors, enemies, triggers and procedural sounds then come from
// MAIN_MAP (see package level), game logic from MAIN_SCRIPT (Lua) and the
// optional HUD_SCRIPT (JS). Projects without the directive keep the legacy
// hard-coded demo sandbox.

package main

import (
	"fmt"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-gl/glfw/v3.3/glfw"

	"aiengine/level"
	"aiengine/parser"
	"aiengine/renderer"
	"aiengine/scripting"
	"aiengine/systems"
)

// levelGame is the per-play-through state of the level runtime.
type levelGame struct {
	lv      *level.Level
	session *level.Session
	sprites map[string]*levelSprite // placement id → billboard
	clock   float64
}

type levelSprite struct {
	sprite *renderer.Sprite
	baseZ  float64
	bob    float64
	phase  float64
}

func levelRuntimeEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(activeManifest.Raw["RUNTIME"]), "level")
}

// loadLevelFile loads a level map (path relative to the project).
func loadLevelFile(path string) (*level.Level, error) {
	return level.LoadFile(projectPath(path))
}

// startLevel (re)builds all per-level state: map, player, sprites, enemies,
// scripting VMs and the session. It is used for the first start and restarts.
func (app *App) startLevel(lv *level.Level) error {
	if app.luaBridge != nil {
		app.luaBridge.Close()
	}
	app.sprites = nil
	app.enemies = nil
	app.paused = false

	app.player = renderer.NewPlayerState(lv.SpawnX, lv.SpawnY, lv.SpawnAngle)
	if app.ms.settingsActive {
		app.player.FOV = app.ms.settings.FOV * math.Pi / 180
	}
	app.raycaster.Map = lv.Map
	app.raycaster.Player = app.player
	app.raycaster.OverlayTitle = ""
	app.raycaster.OverlaySubtitle = ""
	app.applyLevelTextures(lv)
	if c, ok := parseHexColor(lv.Props["FLOOR_COLOR"]); ok {
		app.raycaster.FloorColor = c
	}
	if c, ok := parseHexColor(lv.Props["CEILING_COLOR"]); ok {
		app.raycaster.CeilColor = c
	}

	app.effects = renderer.DefaultEffects()
	app.scriptEngine = scripting.NewEngine()
	app.scriptEngine.Effects = app.effects
	app.scriptEngine.AudioPlayer = app.audioSys
	if hp, err := strconv.ParseFloat(lv.Props["PLAYER_HP"], 64); err == nil && hp > 0 {
		app.scriptEngine.PlayerHP, app.scriptEngine.PlayerMaxHP = hp, hp
	}
	app.scriptEngine.SetPlayerPosition(app.player.X, app.player.Y, 0)

	g := &levelGame{lv: lv, sprites: make(map[string]*levelSprite)}
	g.session = level.NewSession(lv, app.scriptEngine)
	g.session.Exec.OnTeleportPlayer = func(x, y, z float64) { app.player.X, app.player.Y = x, z }
	app.scriptEngine.OnRestart = func() { app.restartRequested = true }
	app.scriptEngine.OnSpawn = func(id, typ string, x, y float64) {
		params := map[string]string{}
		for k, v := range lv.EnemyTypes[typ] {
			params[k] = v
		}
		app.spawnLevelEnemy(level.Placement{Kind: "enemy", Type: typ, ID: id, X: x, Y: y, Params: params})
	}
	app.scriptEngine.OnExit = func() { app.window.SetShouldClose(true) }
	app.game = g

	for _, snd := range lv.Sounds {
		app.audioSys.RegisterSoundDef(systems.SoundDefFromProps(snd.Name, soundProps(snd.Properties, snd.Children)))
	}

	for i, p := range lv.Placements {
		switch p.Kind {
		case "enemy":
			app.spawnLevelEnemy(p)
		default:
			name := p.Params["sprite"]
			if name == "" && (p.Kind == "item" || p.Kind == "prop") {
				name = p.Type
			}
			if name == "" {
				continue
			}
			scale := p.Float("scale", 0.45)
			baseZ := (1-scale)/2 - p.Height
			s := &renderer.Sprite{X: p.X, Y: p.Y, Z: baseZ, Scale: scale, TexName: app.placementTexture(name),
				Alpha: 1, Active: true, EntityID: p.ID}
			app.sprites = append(app.sprites, s)
			g.sprites[p.ID] = &levelSprite{sprite: s, baseZ: baseZ, bob: p.Float("bob", 0), phase: float64(i)}
		}
	}

	// Campaign: state carried in from the previous level (HP, items, vars).
	if app.cs != nil {
		app.cs.carry.Apply(app.scriptEngine)
	}

	// Scripts: Lua game logic + optional JS HUD.
	app.luaBridge = scripting.NewLuaBridge(app.scriptEngine)
	g.session.CallLua = func(name string) {
		if err := app.luaBridge.Call(name); err != nil {
			log.Printf("[Level] start_lua %s: %v", name, err)
		}
	}
	if script := projectPath(activeManifest.MainScript); fileExists(script) {
		if err := app.luaBridge.DoFile(script); err != nil {
			return err
		}
	}
	app.jsBridge = scripting.NewJSBridge(app.scriptEngine)
	if hud := projectPath(activeManifest.HUDScript); fileExists(hud) {
		if _, err := app.jsBridge.RunFile(hud); err != nil {
			log.Printf("JS %s: %v", hud, err)
		}
	}

	log.Printf("[Level] %s: %dx%d, %d placements, %d triggers, %d sounds",
		lv.Name, lv.Map.Width, lv.Map.Height, len(lv.Placements), g.session.Triggers.Count(), len(lv.Sounds))
	app.scriptEngine.Emit("onStart")
	return nil
}

// restartLevel reloads the current level from disk (so edits are picked up)
// and starts over.
func (app *App) restartLevel() {
	app.restartRequested = false
	if app.ms.currentLevel.Path == "" {
		app.ms.currentLevel = activeManifest.LevelList()[0]
	}
	lv, err := loadLevelFile(app.ms.currentLevel.Path)
	if err != nil {
		log.Printf("[Level] restart failed: %v", err)
		return
	}
	if err := app.startLevel(lv); err != nil {
		log.Printf("[Level] restart failed: %v", err)
	}
	app.mouseInit = false
}

// placementTexture loads assets/sprites/<name>.png once, falling back to the
// generated prop sprite.
func (app *App) placementTexture(name string) string {
	tex := "placed_" + name
	if app.texManager.Get(tex) != nil {
		return tex
	}
	path := projectPath("assets", "sprites", name+".png")
	if _, err := app.texManager.LoadFromFile(tex, path); err != nil {
		log.Printf("[Level] sprite %q: %v (using default)", name, err)
		return "sprite_prop"
	}
	return tex
}

func (app *App) spawnLevelEnemy(p level.Placement) {
	hp := p.Float("hp", 50)
	prefix := ""
	if atlas := p.Params["atlas"]; atlas != "" {
		prefix = "atlas_" + strings.NewReplacer("/", "_", "\\", "_", ".", "_").Replace(atlas)
		if app.texManager.Get(prefix+"_0") == nil {
			if err := app.texManager.LoadSpriteSheet(prefix, projectPath(filepath.FromSlash(atlas)), 3, 2); err != nil {
				log.Printf("[Level] enemy atlas %s: %v", atlas, err)
				prefix = ""
			}
		}
	}
	sprite := &renderer.Sprite{X: p.X, Y: p.Y, Scale: p.Float("scale", 0.85), Alpha: 1, Active: true, EntityID: p.ID}
	sprite.Z = p.Float("z", 0)
	enemy := &DemoEnemy{
		ID: p.ID, Sprite: sprite, HP: hp, MaxHP: hp,
		Speed:       p.Float("speed", 1.15),
		Damage:      p.Float("damage", 8),
		AttackRange: p.Float("range", 0.85),
		AttackRate:  p.Float("attack_rate", 1.0),
		SightRange:  p.Float("sight", 6.5),
		State:       "idle", Alive: true,
		FramePrefix: prefix,
		Fighter:     level.NewFighter(p, int64(len(app.enemies)+1)),
		HitSound:    p.Params["attack_sound"],
	}
	sprite.TexName = app.enemyFrameFor(prefix, 0)
	app.enemies = append(app.enemies, enemy)
	app.sprites = append(app.sprites, sprite)
	app.scriptEngine.Entities[p.ID] = &scripting.EntityState{ID: p.ID, Type: p.Type, HP: hp, MaxHP: hp, X: p.X, Y: p.Y, Alive: true}
}

// updateLevelEnemy moves one level enemy and resolves its melee attacks
// with level.Fighter (behaviour brain, A* steering, separation, wind-up and
// cooldown).
func (app *App) updateLevelEnemy(e *DemoEnemy, dt float64) {
	f := e.Fighter
	f.X, f.Y = e.Sprite.X, e.Sprite.Y
	dmg := f.Step(app.game.lv, app.player.X, app.player.Y, dt, func(x, y float64) bool {
		return app.isWallAt(x, y) || app.enemyBlocked(e, x, y)
	})
	e.Sprite.X, e.Sprite.Y = f.X, f.Y
	switch f.State {
	case "windup", "attack":
		e.State = "attack"
	default:
		e.State = f.State
	}
	if dmg > 0 {
		app.playerHit(e, dmg)
	}
}

// playerHit applies enemy damage with feedback: hurt sound, red flash and a
// short camera shake. Death goes through SetPlayerHP (onPlayerDead → lose).
func (app *App) playerHit(e *DemoEnemy, dmg float64) {
	if app.console != nil && app.console.CheatEnabled(systems.CheatGod) {
		return
	}
	log.Printf("[Combat] %s hits player for %.0f", e.ID, dmg)
	snd := e.HitSound
	if snd == "" {
		snd = "player_hurt"
	}
	app.scriptEngine.PlaySound(snd)
	app.hurtFlash = 0.6
	app.shake = 0.25
	app.scriptEngine.SetPlayerHP(app.scriptEngine.GetPlayerHP() - dmg)
}

// updateLevel runs the session after player movement.
func (app *App) updateLevel(dt float64) {
	g := app.game
	g.clock += dt
	events := g.session.Update(dt, app.player.X, app.player.Y)
	for _, ev := range events {
		if ev.Kind == level.EventPickup {
			if s := g.sprites[ev.ID]; s != nil {
				s.sprite.Active = false
			}
		}
	}
	for _, s := range g.sprites {
		if s.bob > 0 {
			s.sprite.Z = s.baseZ - s.bob*(1+math.Sin(g.clock*3+s.phase))/2
		}
	}
	if g.session.State != level.Playing {
		app.showEndScreen()
	}
}

func (app *App) showEndScreen() {
	if app.raycaster.OverlayTitle != "" {
		return
	}
	if app.cs != nil && app.game.session.State == level.Won {
		return // campaigns show the outro screen instead
	}
	text := strings.TrimSpace(app.scriptEngine.EndText)
	title, rest, _ := strings.Cut(text, "\n")
	if title == "" {
		title = "GAME OVER"
		if app.game.session.State == level.Won {
			title = "YOU WIN"
		}
	}
	hint := activeManifest.Raw["RESTART_HINT"]
	if hint == "" {
		hint = "ENTER - RESTART"
	}
	sub := strings.TrimSpace(rest)
	if sub != "" {
		sub += "\n\n"
	}
	app.raycaster.OverlayTitle = title
	app.raycaster.OverlaySubtitle = sub + hint
	app.raycaster.OverlayColor = [4]uint8{255, 90, 80, 255}
	if app.game.session.State == level.Won {
		app.raycaster.OverlayColor = [4]uint8{120, 255, 140, 255}
	}
}

// levelEnded reports whether the level runtime is showing its end screen.
func (app *App) levelEnded() bool {
	return app.game != nil && app.game.session.State != level.Playing
}

// restartPressed checks the "restart" binding, falling back to Enter.
func (app *App) restartPressed() bool {
	if app.inputManager.IsJustPressed("restart") {
		return true
	}
	if app.config != nil && app.config.HasBinding("restart") {
		return false
	}
	pressed := app.window.GetKey(glfw.KeyEnter) == glfw.Press
	just := pressed && !app.enterWasDown
	app.enterWasDown = pressed
	return just
}

// soundProps flattens a SOUND block (properties and value-children such as
// FREQUENCY 800) into the map SoundDefFromProps expects.
func soundProps(props map[string]string, children []*parser.AIENode) map[string]string {
	out := make(map[string]string, len(props)+len(children))
	for k, v := range props {
		out[strings.ToUpper(k)] = v
	}
	for _, c := range children {
		out[strings.ToUpper(c.Type)] = c.Name
	}
	return out
}

func parseHexColor(s string) (c color.RGBA, ok bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return c, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return c, false
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, true
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// saveScreenshot writes the current frame to a PNG file.
func (app *App) saveScreenshot(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, app.raycaster.ReadScreen()); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	log.Printf("Screenshot saved: %s", path)
	return nil
}

// applyLevelTextures loads the project texture set (assets/textures) and then
// the level's own set (MAP property TEXTURES <dir>): walls brick/stone/metal/
// wood plus optional floor/ceiling. Overrides from the previous level are
// undone first.
func (app *App) applyLevelTextures(lv *level.Level) {
	tm := app.texManager
	tm.ResetOverrides()
	dirs := []string{projectPath("assets", "textures")}
	if d := strings.TrimSpace(lv.Props["TEXTURES"]); d != "" {
		dirs = append(dirs, projectPath(filepath.FromSlash(d)))
	}
	for _, d := range dirs {
		if names, err := tm.LoadTextureSet(d, renderer.SurfaceTextureNames); err != nil {
			log.Printf("Warning: textures %s: %v", d, err)
		} else if len(names) > 0 {
			log.Printf("[Level] textures from %s: %v", d, names)
		}
	}
	app.raycaster.FloorTexture, app.raycaster.CeilingTexture = "", ""
	if tm.Get("floor") != nil {
		app.raycaster.FloorTexture = "floor"
	}
	if tm.Get("ceiling") != nil {
		app.raycaster.CeilingTexture = "ceiling"
	}
}
