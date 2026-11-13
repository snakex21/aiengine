// enemy_anim.go - sprite animation of level enemies: animation sets per
// enemy type (sheet descriptor, or 8 placeholder rotations generated from the
// classic 3x2 atlas, or a single image), and the mapping of the enemy state
// machine (idle, walk, attack, pain, death) to animation states.

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"aiengine/anim"
	"aiengine/renderer"
)

// placeholderRotations is the number of views generated for atlases
// without rotation art.
const placeholderRotations = 8

// enemyAnimSet returns (building once) the animation set of an enemy type:
// atlasPath is its atlas image ("" = the manifest's ENEMY_ATLAS), prefix the
// texture prefix its 3x2 frames were loaded under.
func (app *App) enemyAnimSet(atlasPath, prefix string) *anim.Set {
	if prefix == "" {
		prefix = "enemy"
		if atlasPath == "" && activeManifest.EnemyAtlas != "" {
			atlasPath = projectPath(activeManifest.EnemyAtlas)
		}
	}
	if s, ok := app.animSets[prefix]; ok {
		return s
	}
	if app.animSets == nil {
		app.animSets = map[string]*anim.Set{}
	}
	s := app.buildAnimSet(atlasPath, prefix)
	app.animSets[prefix] = s
	return s
}

func (app *App) buildAnimSet(atlasPath, prefix string) *anim.Set {
	tm := app.texManager
	// 1. A sheet descriptor next to the atlas: <atlas>.anim.
	if atlasPath != "" {
		desc := strings.TrimSuffix(atlasPath, filepath.Ext(atlasPath)) + ".anim"
		if fileExists(desc) {
			s, err := loadAnimSheet(tm, desc, prefix+"_anim")
			if err == nil {
				log.Printf("[Anim] %s: %d frames x %d rotations, states %v", desc, len(s.Tex), s.Rotations(), clipNames(s))
				return s
			}
			log.Printf("[Anim] %s: %v (using the atlas)", desc, err)
		}
	}
	// 2. The classic 3x2 atlas: placeholder rotations and death frames.
	var frames []*image.RGBA
	for i := 0; i < 6; i++ {
		t := tm.Get(fmt.Sprintf("%s_%d", prefix, i))
		if t == nil || len(t.Pixels) < int(t.Width*t.Height*4) {
			frames = nil
			break
		}
		frames = append(frames, textureImage(t))
	}
	if len(frames) == 6 {
		frames = append(frames, anim.FakeDeath(frames[0], 3)...)
		s := &anim.Set{Clips: map[string]*anim.Clip{}}
		for f, img := range frames {
			views := anim.FakeRotations(img, placeholderRotations)
			names := make([]string, len(views))
			for r, v := range views {
				names[r] = fmt.Sprintf("%s_f%d_r%d", prefix, f, r)
				tm.Register(names[r], imageTexture(v))
			}
			s.Tex = append(s.Tex, names)
		}
		for _, c := range anim.ClassicAtlas() {
			s.Clips[c.Name] = c
		}
		s.Clips[anim.Death] = &anim.Clip{Name: anim.Death, Frames: []int{6, 7, 8}, FPS: 7}
		log.Printf("[Anim] %s: generated %d placeholder rotations for %d frames", prefix, placeholderRotations, len(frames))
		return s
	}
	// 3. A single image.
	return &anim.Set{Tex: [][]string{{"sprite_enemy"}},
		Clips: map[string]*anim.Clip{anim.Idle: {Name: anim.Idle, Frames: []int{0}, Loop: true}}}
}

func clipNames(s *anim.Set) []string {
	var out []string
	for _, n := range []string{anim.Idle, anim.Walk, anim.Attack, anim.Pain, anim.Death} {
		if s.Clips[n] != nil {
			out = append(out, n)
		}
	}
	return out
}

// loadAnimSheet loads a sheet descriptor and registers its frames.
func loadAnimSheet(tm *renderer.TextureManager, descPath, prefix string) (*anim.Set, error) {
	f, err := os.Open(descPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	d, err := anim.ParseDesc(f)
	if err != nil {
		return nil, err
	}
	imgFile, err := os.Open(filepath.Join(filepath.Dir(descPath), filepath.FromSlash(d.Sheet)))
	if err != nil {
		return nil, err
	}
	defer imgFile.Close()
	img, err := png.Decode(imgFile)
	if err != nil {
		return nil, err
	}
	cells := anim.CutSheet(img, d)
	return anim.NewSet(d, func(fr, r int) string {
		name := fmt.Sprintf("%s_f%d_r%d", prefix, fr, r)
		tm.Register(name, imageTexture(cells[fr][r]))
		return name
	}), nil
}

func textureImage(t *renderer.Texture2D) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, int(t.Width), int(t.Height)))
	copy(img.Pix, t.Pixels)
	return img
}

func imageTexture(img *image.RGBA) *renderer.Texture2D {
	return &renderer.Texture2D{Width: int32(img.Bounds().Dx()), Height: int32(img.Bounds().Dy()), Pixels: img.Pix}
}

// enemyPainTime is how long the pain state shows after a hit.
const enemyPainTime = 0.3

// animateEnemy maps the enemy's state to its animation and facing:
// walk/idle from the fighter, attack (wind-up and hit), pain after damage,
// death until the corpse frame. (dx, dy) is this frame's movement.
func (app *App) animateEnemy(e *DemoEnemy, dt, dx, dy float64) {
	a := e.Anim
	if a == nil {
		e.AnimTimer += dt
		e.Sprite.TexName = app.enemyAnimFrame(e)
		return
	}
	state := anim.Idle
	switch {
	case !e.Alive:
		state = anim.Death
	case e.painT > 0:
		e.painT -= dt
		state = anim.Pain
	case e.State == "walk":
		state = anim.Walk
	case e.State == "attack":
		state = anim.Attack
	}
	a.Play(state)
	a.Update(dt)
	if e.Alive {
		if e.State == "attack" {
			e.Sprite.Angle = math.Atan2(app.player.Y-e.Sprite.Y, app.player.X-e.Sprite.X)
		} else if dx*dx+dy*dy > 1e-10 {
			e.Sprite.Angle = math.Atan2(dy, dx)
		}
	}
	e.Sprite.TexName, e.Sprite.Rotations = a.Textures()
}

// switchTextures returns the generated off/on textures of a switch without
// its own sprite (a lever on a plate).
func switchTextures(tm *renderer.TextureManager) (off, on string) {
	off, on = "sprite_switch_off", "sprite_switch_on"
	if tm.Get(off) != nil {
		return
	}
	for i, name := range []string{off, on} {
		img := image.NewRGBA(image.Rect(0, 0, 32, 32))
		plate := color.RGBA{70, 66, 72, 255}
		for y := 8; y < 30; y++ {
			for x := 9; x < 23; x++ {
				c := plate
				if x == 9 || x == 22 || y == 8 || y == 29 {
					c = color.RGBA{40, 38, 44, 255}
				}
				img.SetRGBA(x, y, c)
			}
		}
		// The lever: up-left when off, down-right (and lit) when on.
		lamp := color.RGBA{150, 40, 30, 255}
		dir := -1.0
		if i == 1 {
			lamp, dir = color.RGBA{60, 220, 90, 255}, 1
		}
		for t := 0.0; t < 9; t += 0.25 {
			x, y := 16+dir*t*0.6, 19-t
			if i == 1 {
				y = 19 + t*0.9
			}
			for o := -1; o <= 1; o++ {
				img.SetRGBA(int(x)+o, int(y), color.RGBA{190, 180, 160, 255})
			}
		}
		for y := 11; y < 14; y++ {
			for x := 12; x < 15; x++ {
				img.SetRGBA(x, y, lamp)
			}
		}
		tm.Register(name, imageTexture(img))
	}
	return
}
