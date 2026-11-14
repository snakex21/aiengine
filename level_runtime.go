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

	"aiengine/anim"
	"aiengine/game"
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
	loops   map[string]moverLoop // looping mover sounds by mover key
}

type levelSprite struct {
	sprite *renderer.Sprite
	baseZ  float64
	bob    float64
	phase  float64
	glow   renderer.PointLight // glow=<level> glow_radius=<cells>: light around the sprite

	texOff, texOn string // switches: texture when off / flipped
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
	app.stopMoverLoops()
	app.sprites = nil
	app.paused = false

	app.player = renderer.NewPlayerState(lv.SpawnX, lv.SpawnY, lv.SpawnAngle)
	if app.ms.settingsActive {
		app.player.FOV = app.ms.settings.FOV * math.Pi / 180
	}
	app.raycaster.Map = lv.Map
	app.raycaster.Sectors = lv.Sectors
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
	app.raycaster.Fog = lv.Fog()
	app.raycaster.Lights = nil
	app.raycaster.Flash = 0

	app.effects = renderer.DefaultEffects()
	app.scriptEngine = scripting.NewEngine()
	app.scriptEngine.Effects = app.effects
	app.scriptEngine.AudioPlayer = app.audioSys
	if hp, err := strconv.ParseFloat(lv.Props["PLAYER_HP"], 64); err == nil && hp > 0 {
		app.scriptEngine.PlayerHP, app.scriptEngine.PlayerMaxHP = hp, hp
	}

	g := &levelGame{lv: lv, sprites: make(map[string]*levelSprite), loops: map[string]moverLoop{}}
	app.w = game.New(lv, nil, app.player, app.scriptEngine, app)
	app.w.God, app.w.Noclip = app.cheat(systems.CheatGod), app.cheat(systems.CheatNoclip)
	app.w.PlacePlayer(lv.SpawnX, lv.SpawnY)
	app.scriptEngine.SetPlayerPosition(app.player.X, app.player.Y, app.player.Feet())
	g.session = level.NewSession(lv, app.scriptEngine)
	app.w.AttachSession(g.session) // movers see the world's bodies; crush damage via ApplyDamage
	g.session.Exec.OnTeleportPlayer = func(x, y, z float64) { app.w.PlacePlayer(x, z) }
	app.scriptEngine.OnRestart = func() { app.restartRequested = true }
	app.scriptEngine.OnSpawn = func(id, typ string, x, y float64) {
		params := map[string]string{}
		for k, v := range lv.EnemyTypes[typ] {
			params[k] = v
		}
		app.spawnEnemy(level.Placement{Kind: "enemy", Type: typ, ID: id, X: x, Y: y, Params: params})
	}
	app.scriptEngine.OnExit = func() { app.window.SetShouldClose(true) }
	app.game = g

	for _, sd := range defaultMoverSounds {
		app.audioSys.RegisterSoundDef(sd)
	}
	for _, snd := range lv.Sounds {
		app.audioSys.RegisterSoundDef(systems.SoundDefFromProps(snd.Name, soundProps(snd.Properties, snd.Children)))
	}

	for i, p := range lv.Placements {
		switch p.Kind {
		case "enemy":
			app.spawnEnemy(p)
		default:
			name := p.Params["sprite"]
			if name == "" && (p.Kind == "item" || p.Kind == "prop") {
				name = p.Type
			}
			tex, texOn := "", ""
			switch {
			case p.Kind == "switch" && name == "":
				tex, texOn = switchTextures(app.texManager)
			case name == "":
				continue
			default:
				tex = app.placementTexture(name)
				if on := p.Params["sprite_on"]; on != "" {
					texOn = app.placementTexture(on)
				}
			}
			scale := p.Float("scale", 0.45)
			baseZ := (1-scale)/2 - p.Height
			s := &renderer.Sprite{X: p.X, Y: p.Y, Z: baseZ, Scale: scale, TexName: tex,
				Alpha: 1, Active: true, EntityID: p.ID}
			app.sprites = append(app.sprites, s)
			g.sprites[p.ID] = &levelSprite{sprite: s, baseZ: baseZ, bob: p.Float("bob", 0), phase: float64(i),
				glow:   renderer.PointLight{X: p.X, Y: p.Y, Level: p.Float("glow", 0), Radius: p.Float("glow_radius", 2)},
				texOff: tex, texOn: texOn}
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

// spawnEnemy adds a level enemy to the world and its sprite to the view.
func (app *App) spawnEnemy(p level.Placement) {
	e := app.w.SpawnEnemy(p)
	app.sprites = append(app.sprites, e.Sprite)
}

// EnemyView builds the sprite and animation of a level enemy (game.Host):
// its atlas (atlas=, else ENEMY_ATLAS) and animation set.
func (app *App) EnemyView(p level.Placement) game.EnemyView {
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
	atlasPath := ""
	if atlas := p.Params["atlas"]; atlas != "" {
		atlasPath = projectPath(filepath.FromSlash(atlas))
	}
	sprite := &renderer.Sprite{Scale: p.Float("scale", 0.85), Alpha: 1, Active: true}
	return game.EnemyView{Sprite: sprite, Anim: anim.NewPlayer(app.enemyAnimSet(atlasPath, prefix)), FramePrefix: prefix}
}

// updateLevel updates the level view after the world step: animated
// textures, picked items, mover sounds, switches, bobbing and glowing
// sprites, and the end screen.
func (app *App) updateLevel(dt float64) {
	g := app.game
	g.clock += dt
	app.raycaster.AnimTime = g.clock
	for _, wev := range app.w.Events() {
		if wev.Kind != game.EventLevel {
			continue
		}
		ev := wev.Level
		switch ev.Kind {
		case level.EventPickup:
			if s := g.sprites[ev.ID]; s != nil {
				s.sprite.Active = false
			}
		case level.EventMover, level.EventSwitch:
			app.handleLevelEvent(ev)
		}
	}
	app.updateMoverLoops()
	lights := app.raycaster.Lights[:0]
	for _, s := range g.sprites {
		if s.bob > 0 {
			s.sprite.Z = s.baseZ - s.bob*(1+math.Sin(g.clock*3+s.phase))/2
		}
	}
	for _, s := range g.sprites {
		if s.glow.Level > 0 && s.sprite.Active {
			lights = append(lights, s.glow)
		}
	}
	app.raycaster.Lights = lights
	if g.session.State != level.Playing {
		app.stopMoverLoops()
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
	app.applyLevelAnimations(lv, dirs)
	app.raycaster.FloorTexture, app.raycaster.CeilingTexture = "", ""
	if tm.Get("floor") != nil {
		app.raycaster.FloorTexture = "floor"
	}
	if tm.Get("ceiling") != nil {
		app.raycaster.CeilingTexture = "ceiling"
	}
}
