// heights_runtime.go - player/enemy movement over maps with floor and
// ceiling heights (see renderer/heights.go and level/heights.go).

package main

import (
	"math"

	"aiengine/renderer"
	"aiengine/systems"
)

// bodyRadius is the collision radius of the player and enemies.
const bodyRadius = 0.15

// placePlayer moves the player to (x, y) standing on that spot's floor (spawn,
// teleports): no fall and no step smoothing.
func (app *App) placePlayer(x, y float64) {
	app.player.X, app.player.Y = x, y
	floor, _ := app.raycaster.Map.Support(x, y, bodyRadius)
	app.player.PlaceAt(floor)
}

// playerBlocked reports whether the player may not stand at (x, y): solid
// cells, floors more than a step above the feet, or too little headroom for
// the current stance.
func (app *App) playerBlocked(x, y float64) bool {
	p := app.player
	return !app.raycaster.Map.CanOccupy(x, y, bodyRadius, p.Feet(), renderer.MaxStepUp, p.Headroom())
}

// canStandUp reports whether the ceiling above the player leaves room to
// stand (crawl spaces keep the player crouched/prone).
func (app *App) canStandUp() bool {
	_, ceil := app.raycaster.Map.Support(app.player.X, app.player.Y, bodyRadius)
	return ceil-app.player.Feet() >= renderer.HeadroomStand-1e-9
}

// updatePlayerGround follows the floor under the player after it moved and
// applies fall damage (map FALL_DAMAGE) when it lands.
func (app *App) updatePlayerGround() {
	floor, _ := app.raycaster.Map.Support(app.player.X, app.player.Y, bodyRadius)
	app.player.SetGround(floor)
}

// applyFallDamage consumes the last landing and hurts the player.
func (app *App) applyFallDamage() {
	fall := app.player.LastFall
	app.player.LastFall = 0
	if fall <= 0 || app.game == nil {
		return
	}
	dmg := math.Round(app.game.lv.FallDamage(fall))
	if dmg <= 0 {
		return
	}
	app.scriptEngine.PlaySound("player_hurt")
	app.hurtFlash = math.Min(0.8, 0.3+dmg/50)
	app.shake = 0.2
	if app.console == nil || !app.console.CheatEnabled(systems.CheatGod) {
		app.scriptEngine.SetPlayerHP(app.scriptEngine.GetPlayerHP() - dmg)
	}
	app.scriptEngine.Emit("onPlayerFall", fall, dmg)
}

// enemyBodyBlocked reports whether an enemy body standing on the floor of
// (fromX, fromY) may not move to (x, y).
func (app *App) enemyBodyBlocked(fromX, fromY, x, y float64) bool {
	m := app.raycaster.Map
	feet, _ := m.Support(fromX, fromY, bodyRadius)
	return !m.CanOccupy(x, y, bodyRadius, feet, renderer.MaxStepUp, renderer.HeadroomStand)
}

// enemyAimHeight is the absolute height the player aims at on an enemy.
func (app *App) enemyAimHeight(s *renderer.Sprite) float64 {
	return app.raycaster.Map.FloorAtPoint(s.X, s.Y) + renderer.EyeHeight - s.Z
}
