// campaign_runtime.go - campaign flow for projects with "CAMPAIGN yes":
// level intro screen → play → outro (time, record) → loading screen → next
// level intro ..., carry-over state, unlocks, best times, save/continue.

package main

import (
	"log"
	"strings"

	"github.com/go-gl/glfw/v3.3/glfw"

	"aiengine/level"
	"aiengine/progress"
	"aiengine/ui"
)

type campaignPhase int

const (
	phasePlay campaignPhase = iota
	phaseIntro
	phaseOutro
	phaseLoading
	phaseFinished
)

type campaignState struct {
	c       *progress.Campaign
	p       *progress.Progress
	path    string
	phase   campaignPhase
	next    string  // level to load after the outro
	timer   float64 // time in the current phase
	frames  int
	carry   progress.State // state applied when the current level (re)starts
	lastRun float64
	record  bool
}

// campaignText returns a manifest TEXT_<key> override or the default.
func campaignText(key, def string) string {
	if v := strings.TrimSpace(activeManifest.Raw["TEXT_"+key]); v != "" {
		return v
	}
	return def
}

// initCampaign loads progress for campaign projects and wires menu hooks.
func (app *App) initCampaign() {
	c := progress.FromManifest(activeManifest)
	if c == nil || app.game == nil {
		return
	}
	cs := &campaignState{c: c, path: projectPath(progress.FileName)}
	p, err := progress.Load(cs.path, c)
	if err != nil {
		log.Printf("Warning: save: %v", err)
	}
	cs.p = p
	app.cs = cs
	log.Printf("Campaign: %d levels, carry %v, continue=%v", len(c.Levels), c.Carry, p.CanContinue())
	if m := app.ms.menu; m != nil {
		m.Available = func(a ui.Action) bool { return a.Kind != "continue" || cs.p.CanContinue() }
		m.LevelStatus = func(id string) (bool, string) {
			if !cs.p.Unlocked[id] {
				return false, campaignText("LOCKED", "(locked)")
			}
			if best, ok := cs.p.Best[id]; ok {
				return true, progress.FormatTime(best)
			}
			return true, ""
		}
	}
}

func (app *App) saveProgress() {
	if err := app.cs.p.Save(app.cs.path); err != nil {
		log.Printf("Warning: saving progress: %v", err)
	}
}

// campaignStart begins a campaign level with a carried state and shows its
// intro screen.
func (app *App) campaignStart(id string, carry progress.State) {
	cs := app.cs
	entry, ok := activeManifest.FindLevel(id)
	if !ok {
		log.Printf("[Campaign] unknown level %q", id)
		return
	}
	cs.carry = carry
	app.ms.currentLevel = entry
	app.restartLevel()
	cs.phase, cs.timer = phaseIntro, 0
	app.window.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
}

// campaignBlocking reports whether a campaign screen freezes gameplay.
func (app *App) campaignBlocking() bool { return app.cs != nil && app.cs.phase != phasePlay }

// campaignFrame advances the campaign state machine once per frame.
func (app *App) campaignFrame(dt float64) {
	cs := app.cs
	if cs == nil || app.menuActive() {
		return
	}
	cs.timer += dt
	cs.frames++
	if cs.phase == phasePlay && app.game != nil && app.game.session.State == level.Won {
		app.campaignLevelDone()
	}
	if cs.phase == phaseLoading && cs.timer > 0.6 && cs.frames > 2 {
		app.campaignStart(cs.next, cs.p.Carry)
	}
	app.campaignOverlay()
}

// campaignLevelDone records completion and shows the outro.
func (app *App) campaignLevelDone() {
	cs := app.cs
	id := app.ms.currentLevel.ID
	t := app.game.session.Elapsed
	carry := cs.c.Capture(app.scriptEngine)
	next, finished, rec := cs.p.Complete(cs.c, id, t, carry)
	if target := app.scriptEngine.ExitTarget; target != "" && target != "next" && cs.c.Index(target) >= 0 {
		next, finished = target, false // explicit exit_to another level
		cs.p.Current, cs.p.Finished = target, false
		cs.p.Unlocked[target] = true
		cs.p.Carry = carry
	}
	app.saveProgress()
	cs.next, cs.lastRun, cs.record = next, t, rec
	cs.phase, cs.timer, cs.frames = phaseOutro, 0, 0
	if finished {
		cs.next = ""
	}
	app.window.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
	log.Printf("[Campaign] %s done in %s (record %v), next %q", id, progress.FormatTime(t), rec, next)
}

// campaignAdvance handles Enter/click/gamepad A on a campaign screen.
func (app *App) campaignAdvance() {
	cs := app.cs
	switch cs.phase {
	case phaseIntro:
		cs.phase = phasePlay
		app.window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)
		app.mouseInit = false
	case phaseOutro:
		if cs.next == "" {
			cs.phase, cs.timer = phaseFinished, 0
		} else {
			cs.phase, cs.timer, cs.frames = phaseLoading, 0, 0
		}
	case phaseFinished:
		cs.phase = phasePlay
		app.openMenu("main")
	}
	app.raycaster.OverlayTitle = ""
}

// campaignOverlay draws the current campaign screen through the overlay.
func (app *App) campaignOverlay() {
	cs := app.cs
	rc := app.raycaster
	rc.OverlayOpaque = false
	cont := campaignText("CONTINUE", "Enter - continue")
	switch cs.phase {
	case phasePlay:
		return
	case phaseIntro:
		rc.OverlayTitle = app.ms.currentLevel.Title
		rc.OverlaySubtitle = joinText(levelText(app.game.lv, "INTRO"), campaignText("START", "Enter - start"))
		rc.OverlayColor = [4]uint8{130, 230, 255, 255}
	case phaseOutro:
		stats := campaignText("TIME", "Time") + " " + progress.FormatTime(cs.lastRun)
		if cs.record {
			stats += "   " + campaignText("RECORD", "New record!")
		} else if best, ok := cs.p.Best[app.ms.currentLevel.ID]; ok {
			stats += "   " + campaignText("BEST", "Best") + " " + progress.FormatTime(best)
		}
		rc.OverlayTitle = campaignText("LEVEL_COMPLETE", "Level complete")
		rc.OverlaySubtitle = joinText(levelText(app.game.lv, "OUTRO"), stats, cont)
		rc.OverlayColor = [4]uint8{120, 255, 140, 255}
	case phaseLoading:
		title := cs.next
		if e, ok := activeManifest.FindLevel(cs.next); ok {
			title = e.Title
		}
		rc.OverlayTitle = campaignText("LOADING", "Loading...")
		rc.OverlaySubtitle = title
		rc.OverlayOpaque = true
		rc.OverlayColor = [4]uint8{200, 200, 220, 255}
	case phaseFinished:
		rc.OverlayTitle = campaignText("CAMPAIGN_COMPLETE", "Campaign complete!")
		total := 0.0
		for _, l := range cs.c.Levels {
			total += cs.p.Best[l.ID]
		}
		rc.OverlaySubtitle = joinText(campaignText("OUTRO", ""),
			campaignText("BEST", "Best")+" "+progress.FormatTime(total), cont)
		rc.OverlayColor = [4]uint8{255, 214, 120, 255}
	}
}

// levelText reads a MAP text property; "|" separates lines.
func levelText(lv *level.Level, key string) string {
	return strings.ReplaceAll(strings.TrimSpace(lv.Props[key]), "|", "\n")
}

func joinText(parts ...string) string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n\n")
}

// campaignMenuAction handles menu actions in campaign projects; it reports
// whether the action was consumed.
func (app *App) campaignMenuAction(kind, arg string) bool {
	cs := app.cs
	if cs == nil {
		return false
	}
	switch kind {
	case "continue":
		if cs.p.CanContinue() {
			app.campaignStart(cs.p.Current, cs.p.Carry)
		}
	case "new_game":
		cs.p.NewGame(cs.c)
		app.saveProgress()
		app.campaignStart(cs.p.Current, progress.State{})
	case "load_level":
		if !cs.p.Unlocked[arg] {
			return true
		}
		cs.p.Start(arg)
		app.saveProgress()
		app.campaignStart(arg, progress.State{})
	default:
		return false
	}
	return true
}
