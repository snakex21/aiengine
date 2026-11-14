// movers_runtime.go - host side of animated sectors (level movers): the
// bodies movers must respect, riding lifts, crush damage, positional
// start/stop sounds and move loops, switches and animated textures.

package main

import (
	"log"
	"math"
	"path/filepath"
	"strings"

	"aiengine/level"
	"aiengine/renderer"
	"aiengine/systems"
)

// moverLoop is a looping sound of a moving mover.
type moverLoop struct {
	name string
	x, y float64
}

// Mover sound falloff (map units).
const (
	moverSoundNear = 2.0
	moverSoundFar  = 14.0
)

// defaultMoverSounds are used when a level does not define the names movers
// play by default (mover_start, mover_move, mover_stop).
var defaultMoverSounds = []systems.SoundDef{
	{Name: "mover_start", Waveform: systems.WaveImpact, Frequency: 120, Decay: 0.25, Noise: 0.45, Volume: 0.45},
	{Name: "mover_move", Waveform: systems.WaveSawtooth, Frequency: 48, Decay: 2, Noise: 0.35, Volume: 0.18, Continuous: true},
	{Name: "mover_stop", Waveform: systems.WaveImpact, Frequency: 90, Decay: 0.3, Noise: 0.5, Volume: 0.5},
}

// handleLevelEvent reacts to mover and switch events of the session.
func (app *App) handleLevelEvent(ev level.Event) {
	g := app.game
	switch ev.Kind {
	case level.EventSwitch:
		if s := g.sprites[ev.ID]; s != nil && s.texOn != "" {
			if g.session.SwitchOn(ev.ID) {
				s.sprite.TexName = s.texOn
			} else {
				s.sprite.TexName = s.texOff
			}
		}
	case level.EventMover:
		app.handleMoverEvent(*ev.Mover)
	}
}

func (app *App) handleMoverEvent(ev level.MoverEvent) {
	g := app.game
	vol := systems.DistanceVolume(math.Hypot(ev.X-app.player.X, ev.Y-app.player.Y), moverSoundNear, moverSoundFar)
	switch ev.Kind {
	case level.MoverStart:
		if ev.Sound != "" {
			app.audioSys.PlaySoundVolume(ev.Sound, vol)
		}
		if ev.Loop != "" {
			g.loops[ev.Key] = moverLoop{ev.Loop, ev.X, ev.Y}
			app.audioSys.PlayLoop(ev.Key, ev.Loop, vol)
		}
	case level.MoverStop:
		if _, ok := g.loops[ev.Key]; ok {
			delete(g.loops, ev.Key)
			app.audioSys.StopLoop(ev.Key)
		}
		if ev.Sound != "" {
			app.audioSys.PlaySoundVolume(ev.Sound, vol)
		}
	}
}

// updateMoverLoops follows the player with the loops' volume.
func (app *App) updateMoverLoops() {
	for key, l := range app.game.loops {
		vol := systems.DistanceVolume(math.Hypot(l.x-app.player.X, l.y-app.player.Y), moverSoundNear, moverSoundFar)
		app.audioSys.SetLoopVolume(key, vol)
	}
}

// stopMoverLoops silences every mover loop (level end, restart).
func (app *App) stopMoverLoops() {
	if app.audioSys != nil {
		app.audioSys.StopLoop("")
	}
	if app.game != nil {
		app.game.loops = map[string]moverLoop{}
	}
}

// applyLevelAnimations loads the textures a level names beyond the standard
// set and installs its texture animations (project descriptor first, then
// the level's TEXTURES directory, then the map's own TEXTURE_ANIM lines).
func (app *App) applyLevelAnimations(lv *level.Level, dirs []string) {
	names := lv.TextureNames()
	var anims []renderer.TexAnim
	for _, d := range dirs {
		list, err := level.LoadTexAnimFile(filepath.Join(d, "textures.aie"))
		if err != nil {
			log.Printf("Warning: %v", err)
		}
		anims = append(anims, list...)
		for _, a := range list {
			names = append(names, a.Name)
			names = append(names, a.Frames...)
		}
	}
	anims = append(anims, lv.TexAnims...)
	for _, d := range dirs {
		if loaded, err := app.texManager.LoadTextureSet(d, names); err != nil {
			log.Printf("Warning: textures %s: %v", d, err)
		} else if len(loaded) > 0 {
			log.Printf("[Level] extra textures from %s: %v", d, loaded)
		}
	}
	app.raycaster.SetTexAnims(anims)
	if len(anims) > 0 {
		var list []string
		for _, a := range anims {
			list = append(list, a.Name)
		}
		log.Printf("[Level] animated textures: %s", strings.Join(list, ", "))
	}
}
