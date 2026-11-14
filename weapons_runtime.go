// weapons_runtime.go - host side of the weapons: loading WEAPON
// definitions (project WEAPONS file + map), the player's start loadout,
// generated textures for projectiles, tracers and explosions, the HUD ammo
// line and first-person weapon sprite, and weapon sounds.

package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"aiengine/game"
	"aiengine/level"
	"aiengine/parser"
	"aiengine/renderer"
	"aiengine/systems"
	"aiengine/weapons"
)

// Positional weapon sound falloff (map units).
const (
	weaponSoundNear = 3.0
	weaponSoundFar  = 22.0
)

// loadWeaponDefs returns the project's WEAPON definitions (manifest WEAPONS
// <file>) with the map's own WEAPON blocks on top, and registers SOUND
// blocks of the weapons file. A project without any gets the built-in
// pistol so the sandbox can still shoot.
func (app *App) loadWeaponDefs(lv *level.Level) map[string]*weapons.Def {
	defs := map[string]*weapons.Def{}
	if file := strings.TrimSpace(activeManifest.Raw["WEAPONS"]); file != "" {
		path := projectPath(filepath.FromSlash(file))
		if f, err := os.Open(path); err != nil {
			log.Printf("Warning: weapons: %v", err)
		} else {
			doc, err := parser.ParseAIE(f)
			f.Close()
			if err != nil {
				log.Printf("Warning: %s: %v", path, err)
			} else {
				list, err := weapons.FromDocument(doc)
				if err != nil {
					log.Printf("Warning: %s: %v", path, err)
				}
				for k, d := range list {
					defs[k] = d
				}
				for _, n := range weapons.Sounds(doc) {
					{
						app.audioSys.RegisterSoundDef(systems.SoundDefFromProps(n.Name, soundProps(n.Properties, n.Children)))
					}
				}
			}
		}
	}
	if lv != nil {
		for k, d := range lv.Weapons {
			defs[k] = d
		}
	}
	if len(defs) == 0 {
		p := weapons.DefaultPistol()
		defs[p.Name] = p
	}
	for _, d := range defs {
		app.weaponTextures(d)
	}
	return defs
}

// startLoadout gives the player the level's (or manifest's)
// PLAYER_WEAPONS, or the built-in pistol in projects without weapons.
func (app *App) startLoadout(lv *level.Level) {
	list := activeManifest.Raw["PLAYER_WEAPONS"]
	if lv != nil && lv.Props["PLAYER_WEAPONS"] != "" {
		list = lv.Props["PLAYER_WEAPONS"]
	}
	names := strings.Fields(list)
	if len(names) == 0 && app.scriptEngine.Weapons["pistol"] != nil && activeManifest.Raw["WEAPONS"] == "" {
		names = []string{"pistol"}
	}
	for _, n := range names {
		app.scriptEngine.GiveWeapon(n)
	}
	if len(names) > 0 {
		app.scriptEngine.SelectWeapon(names[0])
	}
}

// parseColor reads "#RRGGBB" (default: warm white).
func parseColor(s string) color.RGBA {
	if c, ok := parseHexColor(s); ok {
		return c
	}
	return color.RGBA{255, 230, 180, 255}
}

// weaponTextures registers the textures a weapon's projectiles and
// tracers use: assets/sprites/<PROJECTILE_SPRITE>.png when it exists,
// otherwise a generated glowing ball in the weapon's COLOR.
func (app *App) weaponTextures(d *weapons.Def) {
	tm := app.texManager
	col := parseColor(d.Color)
	name := game.ProjectileTexture(d)
	if tm.Get(name) == nil {
		loaded := false
		if d.Sprite != "" && d.Sprite != "orb" {
			if _, err := tm.LoadFromFile(name, projectPath("assets", "sprites", d.Sprite+".png")); err == nil {
				loaded = true
			} else {
				log.Printf("[Weapons] %s: sprite %q: %v (using a glowing orb)", d.Name, d.Sprite, err)
			}
		}
		if !loaded {
			tm.Register(name, imageTexture(glowBall(32, col, 0.55)))
		}
	}
	if tn := game.TracerTexture(d); tm.Get(tn) == nil {
		tm.Register(tn, imageTexture(glowBall(16, col, 0.9)))
	}
	if tm.Get("fx_spark") == nil {
		tm.Register("fx_spark", imageTexture(glowBall(16, color.RGBA{255, 240, 200, 255}, 0.7)))
	}
	if tm.Get("fx_boom_0") == nil {
		for i := 0; i < 4; i++ {
			tm.Register(fmt.Sprintf("fx_boom_%d", i), imageTexture(boomFrame(64, i)))
		}
	}
	if d.View != "" && d.View != "player" && tm.Get("view_"+d.View) == nil {
		for _, suffix := range []string{"", "_fire"} {
			path := projectPath("assets", "sprites", "player", d.View+suffix+".png")
			if fileExists(path) {
				if _, err := tm.LoadFromFile("view_"+d.View+suffix, path); err != nil {
					log.Printf("[Weapons] view %s: %v", path, err)
				}
			}
		}
	}
}

// glowBall is a round sprite: a bright core fading to transparent edges.
func glowBall(n int, c color.RGBA, core float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	h := float64(n) / 2
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			d := math.Hypot(float64(x)+0.5-h, float64(y)+0.5-h) / h
			if d >= 1 {
				continue
			}
			a := 1 - d
			white := math.Max(0, 1-d/core) // white-hot centre
			mix := func(v uint8) uint8 { return uint8(float64(v)*(1-white) + 255*white) }
			img.SetRGBA(x, y, color.RGBA{mix(c.R), mix(c.G), mix(c.B), uint8(255 * math.Min(1, a*1.6))})
		}
	}
	return img
}

// boomFrame is frame i of 4 of a generated explosion: a hot ball that
// grows into ragged orange flames and smoke.
func boomFrame(n, i int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	h := float64(n) / 2
	k := float64(i) / 3
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx, dy := float64(x)+0.5-h, float64(y)+0.5-h
			ang := math.Atan2(dy, dx)
			rag := 0.82 + 0.18*math.Sin(ang*7+float64(i)*1.7)*math.Sin(ang*3-float64(i))
			d := math.Hypot(dx, dy) / h / rag
			if d >= 1 {
				continue
			}
			heat := (1 - d) * (1 - 0.6*k)
			r := 255.0
			g := 80 + 175*math.Min(1, heat*1.4)
			b := 30 + 200*math.Max(0, heat-0.55)
			if k > 0.5 && d > 0.6 { // smoky rim late in the blast
				r, g, b = 90, 70, 60
			}
			a := math.Min(1, (1-d)*2.2) * (1 - 0.35*k)
			img.SetRGBA(x, y, color.RGBA{uint8(r), uint8(g), uint8(b), uint8(255 * a)})
		}
	}
	return img
}

// updateWeaponHUD shows the current weapon's name and rounds (scripts can
// still set the ammo bar with UI.setAmmo when the player is unarmed).
func (app *App) updateWeaponHUD() {
	rc := app.raycaster
	lo := app.scriptEngine.Loadout
	wp := lo.CurrentWeapon()
	if wp == nil {
		rc.HUDAmmo, rc.HUDAmmoMax, rc.HUDAmmoText = app.scriptEngine.HUDAmmo, app.scriptEngine.HUDAmmoMax, ""
		return
	}
	loaded, spare, unlimited := lo.Rounds()
	title := strings.ToUpper(wp.Def.HUDTitle())
	switch {
	case unlimited:
		rc.HUDAmmo, rc.HUDAmmoMax = 0, 0
		rc.HUDAmmoText = title
	case wp.Def.Magazine > 0:
		rc.HUDAmmo, rc.HUDAmmoMax = loaded, wp.Def.Magazine
		rc.HUDAmmoText = title + "  " + strconv.Itoa(loaded) + " / " + strconv.Itoa(spare)
		if wp.Reloading {
			rc.HUDAmmoText = title + "  ..."
		}
	default:
		max := wp.Def.MaxAmmo
		if max <= 0 {
			max = int(math.Max(float64(loaded), float64(wp.Def.StartAmmo)))
		}
		rc.HUDAmmo, rc.HUDAmmoMax = loaded, max
		rc.HUDAmmoText = title + "  " + strconv.Itoa(loaded)
	}
}

// weaponView returns the first-person texture of the current weapon when
// it has its own (VIEW), else "" (the project's hand frames are used).
func (app *App) weaponView(firing bool) string {
	wp := app.scriptEngine.Loadout.CurrentWeapon()
	if wp == nil || wp.Def.View == "" || wp.Def.View == "player" {
		return ""
	}
	base := "view_" + wp.Def.View
	if firing && app.texManager.Get(base+"_fire") != nil {
		return base + "_fire"
	}
	if app.texManager.Get(base) != nil {
		return base
	}
	return ""
}

// handleWorldEvents plays positional sounds and starts the weapon
// animation when the player fires.
func (app *App) handleWorldEvents() {
	for _, ev := range app.w.Events() {
		switch ev.Kind {
		case game.EventSound:
			d := math.Hypot(ev.X-app.player.X, ev.Y-app.player.Y)
			app.audioSys.PlaySoundVolume(ev.Name, systems.DistanceVolume(d, weaponSoundNear, weaponSoundFar))
		case game.EventShot:
			if ev.Source == game.PlayerID {
				app.playerAnim.ShootTimer = 0.18
			}
		}
	}
}

// renderSprites is the static sprites plus projectiles and effects.
func (app *App) renderSprites() []*renderer.Sprite {
	app.spriteBuf = append(app.spriteBuf[:0], app.sprites...)
	if app.w != nil {
		app.spriteBuf = app.w.Sprites(app.spriteBuf)
	}
	return app.spriteBuf
}
