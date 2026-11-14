// heights_runtime.go - player/enemy movement over maps with floor and
// ceiling heights (see renderer/heights.go and level/heights.go).

package main

import (
	"fmt"
	"math"
	"os"

	"aiengine/level"
	"aiengine/renderer"
	"aiengine/systems"
	"aiengine/world"
)

// bodyRadius is the collision radius of the player and enemies.
const bodyRadius = 0.15

// world is the walkable world: the grid map or, for sector maps, the
// sectors (both apply the same step, headroom and sight rules).
func (app *App) world() world.Geometry {
	return world.Of(app.raycaster.Map, app.raycaster.Sectors)
}

// placePlayer moves the player to (x, y) standing on that spot's floor (spawn,
// teleports): no fall and no step smoothing.
func (app *App) placePlayer(x, y float64) {
	app.player.X, app.player.Y = x, y
	floor, _ := app.world().Support(x, y, bodyRadius)
	app.player.PlaceAt(floor)
}

// playerBlocked reports whether the player may not stand at (x, y): solid
// cells, floors more than a step above the feet, or too little headroom for
// the current stance.
func (app *App) playerBlocked(x, y float64) bool {
	p := app.player
	return !app.world().CanOccupy(x, y, bodyRadius, p.Feet(), renderer.MaxStepUp, p.Headroom())
}

// canStandUp reports whether the ceiling above the player leaves room to
// stand (crawl spaces keep the player crouched/prone).
func (app *App) canStandUp() bool {
	_, ceil := app.world().Support(app.player.X, app.player.Y, bodyRadius)
	return ceil-app.player.Feet() >= renderer.HeadroomStand-1e-9
}

// updatePlayerGround follows the floor under the player after it moved and
// applies fall damage (map FALL_DAMAGE) when it lands.
func (app *App) updatePlayerGround() {
	floor, _ := app.world().Support(app.player.X, app.player.Y, bodyRadius)
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
	m := app.world()
	feet, _ := m.Support(fromX, fromY, bodyRadius)
	return !m.CanOccupy(x, y, bodyRadius, feet, renderer.MaxStepUp, renderer.HeadroomStand)
}

// enemyAimHeight is the absolute height the player aims at on an enemy.
func (app *App) enemyAimHeight(s *renderer.Sprite) float64 {
	floor, _ := app.world().Support(s.X, s.Y, 0)
	return floor + renderer.EyeHeight - s.Z
}

// convertGridMap writes the sector version of a GRID map file.
func convertGridMap(in, out string) error {
	src, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	res, err := level.GridToSectors(string(src))
	if err != nil {
		return fmt.Errorf("%s: %w", in, err)
	}
	return os.WriteFile(out, []byte(res), 0o644)
}
