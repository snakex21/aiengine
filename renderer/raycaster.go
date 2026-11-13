// renderer/raycaster.go — DDA raycasting engine for 2.5D rendering.
// CPU pixel-buffer approach: renders walls+floor/ceiling into a byte buffer,
// uploads to a single OpenGL texture, and draws a fullscreen quad.
// Sprites are rendered as GPU quads on top. Zero FBO, single shader for walls.

package renderer

import (
	"image"
	"image/color"
	"log"
	"math"

	"github.com/go-gl/gl/v4.1-core/gl"
)

// --- Map ---

// MapCell represents a single cell in the 2D grid map.
type MapCell int32

const (
	CellEmpty MapCell = 0
	CellWall1 MapCell = 1 // brick
	CellWall2 MapCell = 2 // stone
	CellWall3 MapCell = 3 // metal
	CellDoor  MapCell = 4 // wood
)

// MapData holds the grid data and dimensions for raycasting.
type MapData struct {
	Grid   [][]MapCell
	Width  int
	Height int

	// Optional per-cell layers (row-major, Width*Height; nil = defaults),
	// see heights.go.
	Floor []float64 // floor height (default 0)
	Ceil  []float64 // ceiling height (default 1)
	Side  []MapCell // texture of the cell's step faces (default stone)
}

func NewMapData(grid [][]MapCell) *MapData {
	if len(grid) == 0 {
		return &MapData{Width: 0, Height: 0}
	}
	return &MapData{
		Grid:   grid,
		Width:  len(grid[0]),
		Height: len(grid),
	}
}

func (m *MapData) GetCell(x, y int) MapCell {
	if x < 0 || x >= m.Width || y < 0 || y >= m.Height {
		return CellWall1
	}
	return m.Grid[y][x]
}

// --- Player ---

// Stance enumerates the player's body posture, which controls eye height and
// movement speed.
type Stance int

const (
	StanceStand Stance = iota
	StanceCrouch
	StanceProne
)

// PlayerState holds the player's position and orientation for the raycaster.
type PlayerState struct {
	X        float64
	Y        float64
	Z        float64 // jump height above ground, in wall-height units (0 = on floor)
	VelZ     float64 // vertical velocity for jumping/falling
	Grounded bool    // true while standing on the ground

	Angle float64 // yaw (left/right look), radians
	Pitch float64 // vertical look, as a fraction of screen height [-PitchLimit, PitchLimit]

	Stance    Stance  // current posture
	EyeOffset float64 // smoothed eye-height offset for the current stance (<= 0)

	// Ground is the absolute height of the floor the player stands on (or
	// falls towards); Z is measured from it. StepOffset smooths the camera
	// after walking up a step (negative, eases back to 0). LastFall is the
	// height of the last completed fall (set on landing; the host reads and
	// clears it for fall damage).
	Ground     float64
	StepOffset float64
	LastFall   float64
	airPeak    float64 // highest absolute feet height of the current flight
	jumpTop    float64 // absolute feet height where the current jump stops rising

	FOV       float64
	MoveSpeed float64
	RotSpeed  float64
}

func NewPlayerState(x, y, angle float64) *PlayerState {
	return &PlayerState{
		X:         x,
		Y:         y,
		Z:         0,
		Grounded:  true,
		Angle:     angle,
		Stance:    StanceStand,
		FOV:       math.Pi / 2.7, // ~66 degrees
		MoveSpeed: 3.0,
		RotSpeed:  3.0,
	}
}

// Jump physics tuning. Z is in wall-height units where 0 is standing on the
// floor. Values are chosen so a jump peaks at roughly half a wall height.
const (
	JumpVelocity = 4.5  // initial upward velocity (units/sec)
	GravityAccel = 16.0 // downward acceleration (units/sec^2)
	MaxJumpZ     = 0.45 // clamp on peak height
)

// Look and stance tuning.
const (
	PitchLimit = 0.35 // max up/down look as a fraction of screen height

	eyeStand  = 0.0   // eye-height offset per stance (wall-height units, <= 0)
	eyeCrouch = -0.18 // crouched: lower camera
	eyeProne  = -0.34 // prone: lowest camera
	eyeLerp   = 10.0  // eye-height smoothing rate (per second)
	stepLerp  = 18.0  // step-up camera smoothing rate (per second)

	speedCrouch = 0.5  // movement speed multiplier while crouched
	speedProne  = 0.28 // movement speed multiplier while prone
)

// Jump starts an upward jump if the player is currently grounded.
// It is a no-op while airborne (no double jump) or while prone.
func (p *PlayerState) Jump() {
	if p.Grounded && p.Stance != StanceProne {
		p.VelZ = JumpVelocity
		p.Grounded = false
		p.airPeak = p.Ground + p.Z
		p.jumpTop = p.Ground + p.Z + MaxJumpZ
	}
}

// SetStance changes the player's posture. Jumping is only allowed when
// standing, and the eye height eases toward the new stance in UpdateVertical.
func (p *PlayerState) SetStance(s Stance) {
	p.Stance = s
}

// stanceEyeTarget returns the eye-height offset for the current stance.
func (p *PlayerState) stanceEyeTarget() float64 {
	switch p.Stance {
	case StanceCrouch:
		return eyeCrouch
	case StanceProne:
		return eyeProne
	default:
		return eyeStand
	}
}

// SpeedMultiplier returns the movement-speed scale for the current stance.
func (p *PlayerState) SpeedMultiplier() float64 {
	switch p.Stance {
	case StanceCrouch:
		return speedCrouch
	case StanceProne:
		return speedProne
	default:
		return 1.0
	}
}

// CameraHeight is the total camera height above the floor: jump height plus the
// (negative) stance eye offset. Used for distance-scaled vertical parallax.
func (p *PlayerState) CameraHeight() float64 {
	return p.Z + p.EyeOffset + p.StepOffset
}

// Feet is the absolute height of the player's feet.
func (p *PlayerState) Feet() float64 { return p.Ground + p.Z }

// EyeZ is the absolute camera height (feet + eye height + stance/step).
func (p *PlayerState) EyeZ() float64 { return p.Ground + EyeHeight + p.CameraHeight() }

// Headroom is the free height the current stance needs.
func (p *PlayerState) Headroom() float64 {
	switch p.Stance {
	case StanceCrouch:
		return HeadroomCrouch
	case StanceProne:
		return HeadroomProne
	default:
		return HeadroomStand
	}
}

// SetGround tells the player the floor height under it after moving.
// Stepping up (the host's collision allows it up to MaxStepUp, or onto a
// ledge below the feet while jumping) lands the player there with a smooth
// camera; stepping off a ledge starts a fall.
func (p *PlayerState) SetGround(h float64) {
	if h == p.Ground {
		return
	}
	feet := p.Ground + p.Z
	if h > feet { // walked up a step
		p.StepOffset -= h - feet
		p.StepOffset = math.Max(p.StepOffset, -2*MaxStepUp)
		p.Ground, p.Z = h, 0
		if !p.Grounded {
			p.land()
		}
		return
	}
	// Floor at or below the feet: the feet stay where they are.
	p.Ground, p.Z = h, feet-h
	if p.Z <= 1e-9 {
		p.Z = 0
		return
	}
	if p.Grounded {
		p.Grounded = false
		p.VelZ = 0
		p.airPeak = feet
		p.jumpTop = feet
	}
}

// PlaceAt puts the player on the ground at height h (spawn, teleport): no
// fall, no camera smoothing.
func (p *PlayerState) PlaceAt(h float64) {
	p.Ground, p.Z, p.VelZ, p.StepOffset, p.LastFall = h, 0, 0, 0, 0
	p.Grounded = true
}

func (p *PlayerState) land() {
	p.LastFall = math.Max(0, p.airPeak-(p.Ground+p.Z))
	p.Z, p.VelZ, p.Grounded = 0, 0, true
}

// AddPitch adjusts the vertical look angle and clamps it to the allowed range.
func (p *PlayerState) AddPitch(delta float64) {
	p.Pitch += delta
	if p.Pitch > PitchLimit {
		p.Pitch = PitchLimit
	}
	if p.Pitch < -PitchLimit {
		p.Pitch = -PitchLimit
	}
}

// UpdateVertical advances the player's vertical position by dt seconds,
// applying gravity while airborne and easing the eye height toward the current
// stance. Landing settles the player on the ground at Z=0.
func (p *PlayerState) UpdateVertical(dt float64) {
	// Ease eye height toward the stance target.
	target := p.stanceEyeTarget()
	t := eyeLerp * dt
	if t > 1 {
		t = 1
	}
	p.EyeOffset += (target - p.EyeOffset) * t
	// Ease the camera back up after a step (fast, ~0.1 s).
	if p.StepOffset != 0 {
		p.StepOffset -= p.StepOffset * math.Min(1, stepLerp*dt)
		if math.Abs(p.StepOffset) < 1e-3 {
			p.StepOffset = 0
		}
	}

	if p.Grounded {
		return
	}
	p.VelZ -= GravityAccel * dt
	p.Z += p.VelZ * dt
	if p.VelZ > 0 && p.Ground+p.Z >= p.jumpTop {
		p.Z = p.jumpTop - p.Ground
		p.VelZ = 0
	}
	p.airPeak = math.Max(p.airPeak, p.Ground+p.Z)
	if p.Z <= 0 {
		p.Z = 0
		p.land()
	}
}

// --- Wall colors (fallback, no texture needed) ---

var wallColors = map[MapCell]color.RGBA{
	CellWall1: {180, 100, 60, 255},  // brick: brown-orange
	CellWall2: {140, 140, 140, 255}, // stone: grey
	CellWall3: {160, 160, 180, 255}, // metal: blue-grey
	CellDoor:  {139, 90, 43, 255},   // wood: brown
}

var ceilColor = color.RGBA{20, 20, 30, 255}  // dark blue-grey
var floorColor = color.RGBA{40, 35, 30, 255} // dark brown

// --- Raycaster ---

type Raycaster struct {
	Map        *MapData
	Player     *PlayerState
	TexManager *TextureManager

	screenW int32
	screenH int32

	// CPU pixel buffer (RGBA).
	pixels []uint8

	// OpenGL texture for the pixel buffer.
	screenTex uint32

	// Simple fullscreen-quad shader.
	simpleProg *ShaderProgram
	quadVAO    uint32
	quadVBO    uint32

	// Sprite rendering (GPU overlay).
	spriteProg *ShaderProgram
	wallVAO    uint32
	wallVBO    uint32

	// Per column: distance of the solid wall hit (0 = none).
	zBuffer []float64
	// Per column: open-window log for sprite clipping (see cast.go).
	clips       [][]clipEntry
	rowK        []float64
	spriteItems []spriteItem

	// Effects (applied via simple post-processing).
	effects *EffectsConfig

	// Console overlay (cheat console).
	ConsoleOpen    bool
	ConsoleInput   string
	ConsoleHistory []string

	// HUD weapon (first‑person view).
	HUDWeaponTex string
	HUDWeaponBob float64

	HUDEnabled bool
	HUDHP      int
	HUDMaxHP   int
	HUDAmmo    int
	HUDAmmoMax int

	// Scripted HUD text: lines drawn top-left, a centred message, and a
	// full-screen overlay (end screen) that hides sprites and the weapon.
	HUDLines        []string
	HUDMessage      string
	OverlayTitle    string
	OverlaySubtitle string
	OverlayColor    [4]uint8
	OverlayOpaque   bool // fully cover the world (loading screens)

	// HideHUD suppresses crosshair, bars, HUD text and the weapon (menus).
	HideHUD bool
	// UIDraw, when set, draws extra UI (menus) into the UI layer each frame.
	UIDraw func(buf []uint8, sw, sh int)
	// ViewportW/H is the framebuffer size the image is stretched to
	// (0 = same as the internal render size).
	ViewportW, ViewportH int32

	// UI layer: RGBA with alpha, drawn after sprites and the weapon so HUD
	// text, menus and the console are always on top.
	ui    []uint8
	uiTex uint32

	// Optional floor/ceiling textures (TextureManager names, power-of-two
	// sizes); empty = flat CeilColor/FloorColor.
	FloorTexture   string
	CeilingTexture string

	// Floor/ceiling fill colours (defaults set by NewRaycaster).
	CeilColor  color.RGBA
	FloorColor color.RGBA
}

func NewRaycaster(m *MapData, p *PlayerState, tm *TextureManager) *Raycaster {
	return &Raycaster{
		Map:        m,
		Player:     p,
		TexManager: tm,
		effects:    DefaultEffects(),
		CeilColor:  ceilColor,
		FloorColor: floorColor,
	}
}

// Init sets up OpenGL resources.
func (rc *Raycaster) Init(screenW, screenH int32) error {
	rc.screenW = screenW
	rc.screenH = screenH
	rc.zBuffer = make([]float64, screenW)
	rc.pixels = make([]uint8, screenW*screenH*4)
	rc.ui = make([]uint8, screenW*screenH*4)

	log.Printf("[Renderer] Initializing %dx%d (pixel buffer: %d bytes)",
		screenW, screenH, len(rc.pixels))

	// --- Simple passthrough shader (vertex + fragment) ---
	const simpleVert = `#version 330 core
layout(location=0) in vec2 aPos;
layout(location=1) in vec2 aTexCoord;
out vec2 vTexCoord;
void main(){gl_Position=vec4(aPos,0,1);vTexCoord=aTexCoord;}
` + "\x00"

	const simpleFrag = `#version 330 core
in vec2 vTexCoord;
out vec4 FragColor;
uniform sampler2D uTex;
void main(){FragColor=texture(uTex,vTexCoord);}
` + "\x00"

	var err error
	rc.simpleProg, err = NewShaderProgram(simpleVert, simpleFrag)
	if err != nil {
		log.Printf("[Renderer] Simple shader compile FAILED: %v", err)
		return err
	}
	log.Printf("[Renderer] Simple shader compiled OK (ID=%d)", rc.simpleProg.ID)

	// --- Sprite shader (keep from shader.go) ---
	rc.spriteProg, err = NewShaderProgram(spriteVertexShader, spriteFragmentShader)
	if err != nil {
		log.Printf("[Renderer] Sprite shader compile FAILED: %v", err)
		return err
	}
	log.Printf("[Renderer] Sprite shader compiled OK (ID=%d)", rc.spriteProg.ID)

	// --- Fullscreen quad ---
	// The CPU buffer is top-down (row 0 = top of screen) while texture
	// row 0 maps to v=0, so v runs 1 at the bottom to 0 at the top.
	quadVerts := []float32{
		-1, -1, 0, 1,
		1, -1, 1, 1,
		1, 1, 1, 0,
		-1, -1, 0, 1,
		1, 1, 1, 0,
		-1, 1, 0, 0,
	}
	gl.GenVertexArrays(1, &rc.quadVAO)
	gl.GenBuffers(1, &rc.quadVBO)
	gl.BindVertexArray(rc.quadVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, rc.quadVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(quadVerts)*4, gl.Ptr(quadVerts), gl.STATIC_DRAW)
	gl.VertexAttribPointer(0, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(2*4))
	gl.EnableVertexAttribArray(1)
	gl.BindVertexArray(0)
	log.Printf("[Renderer] Fullscreen quad VAO=%d", rc.quadVAO)

	// --- Wall column quad (for sprites) ---
	wallVerts := []float32{
		-0.5, 0, 0, 1,
		0.5, 0, 1, 1,
		0.5, 1, 1, 0,
		-0.5, 0, 0, 1,
		0.5, 1, 1, 0,
		-0.5, 1, 0, 0,
	}
	gl.GenVertexArrays(1, &rc.wallVAO)
	gl.GenBuffers(1, &rc.wallVBO)
	gl.BindVertexArray(rc.wallVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, rc.wallVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(wallVerts)*4, gl.Ptr(wallVerts), gl.STATIC_DRAW)
	gl.VertexAttribPointer(0, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(2*4))
	gl.EnableVertexAttribArray(1)
	gl.BindVertexArray(0)

	// --- Screen texture ---
	gl.GenTextures(1, &rc.screenTex)
	gl.BindTexture(gl.TEXTURE_2D, rc.screenTex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, screenW, screenH, 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	log.Printf("[Renderer] Screen texture ID=%d (%dx%d)", rc.screenTex, screenW, screenH)

	gl.GenTextures(1, &rc.uiTex)
	gl.BindTexture(gl.TEXTURE_2D, rc.uiTex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, screenW, screenH, 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	checkGLError("Init end")
	return nil
}

// Render executes a full frame.
func (rc *Raycaster) Render(sprites []*Sprite) {
	// The internal image (screenW x screenH) is stretched over the whole
	// framebuffer, so fullscreen/resize never leave it in a corner.
	vw, vh := rc.viewport()
	gl.Viewport(0, 0, vw, vh)
	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT)

	// 1. CPU: world (floor, ceiling, walls, sprites, colour effects) and
	// the UI layer.
	rc.renderToBuffer(sprites)
	rc.renderUI()

	// 2. World image as a fullscreen quad.
	gl.Disable(gl.DEPTH_TEST)
	rc.drawLayer(rc.screenTex, rc.pixels, false)

	// 3. The first-person weapon (GPU quad).
	if !rc.HideHUD && rc.OverlayTitle == "" {
		rc.DrawHUDWeapon()
	}

	// 4. UI layer on top of everything.
	rc.drawLayer(rc.uiTex, rc.ui, true)
	gl.Enable(gl.DEPTH_TEST)

	checkGLError("Render end")
}

func (rc *Raycaster) viewport() (int32, int32) {
	if rc.ViewportW > 0 && rc.ViewportH > 0 {
		return rc.ViewportW, rc.ViewportH
	}
	return rc.screenW, rc.screenH
}

// drawLayer uploads a CPU buffer to tex and draws it as a fullscreen quad.
func (rc *Raycaster) drawLayer(tex uint32, pix []uint8, blend bool) {
	gl.Disable(gl.DEPTH_TEST) // drawSprites re-enables it; the depth buffer is never cleared
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, rc.screenW, rc.screenH, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pix))
	if blend {
		gl.Enable(gl.BLEND)
		gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	}
	rc.simpleProg.Use()
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	rc.simpleProg.SetUniform1i("uTex", 0)
	gl.BindVertexArray(rc.quadVAO)
	gl.DrawArrays(gl.TRIANGLES, 0, 6)
	gl.BindVertexArray(0)
	if blend {
		gl.Disable(gl.BLEND)
	}
}

// renderUI clears the UI layer and draws HUD, scripted text, menus and the
// console into it.
func (rc *Raycaster) renderUI() {
	sw, sh := int(rc.screenW), int(rc.screenH)
	if len(rc.ui) != sw*sh*4 {
		rc.ui = make([]uint8, sw*sh*4)
	}
	clear(rc.ui)
	if !rc.HideHUD {
		rc.drawHUD(sw, sh)
		rc.drawHUDText(sw, sh)
	}
	if rc.UIDraw != nil {
		rc.UIDraw(rc.ui, sw, sh)
	}
	if rc.ConsoleOpen {
		rc.drawConsole(sw, sh)
	}
}

// Resize changes the internal render resolution (pixel buffers, z-buffer and
// textures). The image is still stretched to ViewportW x ViewportH.
func (rc *Raycaster) Resize(w, h int32) {
	if w <= 0 || h <= 0 || (w == rc.screenW && h == rc.screenH) {
		return
	}
	rc.screenW, rc.screenH = w, h
	rc.pixels = make([]uint8, w*h*4)
	rc.ui = make([]uint8, w*h*4)
	rc.zBuffer = make([]float64, w)
	for _, tex := range []uint32{rc.screenTex, rc.uiTex} {
		gl.BindTexture(gl.TEXTURE_2D, tex)
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	}
	log.Printf("[Renderer] Resized to %dx%d", w, h)
}

// Size returns the internal render resolution.
func (rc *Raycaster) Size() (int, int) { return int(rc.screenW), int(rc.screenH) }

// drawHUD renders crosshair, HP bar and ammo counter, scaled with the
// render height so they stay the same relative size at any resolution.
func (rc *Raycaster) drawHUD(sw, sh int) {
	u := UIScale(sh)
	white := [4]uint8{255, 255, 255, 255}
	cx, cy := sw/2, sh/2
	l, g := 4*u, u+1
	FillRect(rc.ui, sw, sh, cx-l, cy, l-g, max(1, u/2), white)
	FillRect(rc.ui, sw, sh, cx+g, cy, l-g, max(1, u/2), white)
	FillRect(rc.ui, sw, sh, cx, cy-l, max(1, u/2), l-g, white)
	FillRect(rc.ui, sw, sh, cx, cy+g, max(1, u/2), l-g, white)

	border := [4]uint8{60, 60, 60, 255}
	bg := [4]uint8{30, 30, 30, 200}
	bar := func(x, y, w, h int, ratio float64, fill func(frac float64) [4]uint8) {
		FillRect(rc.ui, sw, sh, x-1, y-1, w+2, h+2, border)
		FillRect(rc.ui, sw, sh, x, y, w, h, bg)
		ratio = clamp01(ratio)
		fw := int(float64(w) * ratio)
		for i := 0; i < fw; i++ {
			FillRect(rc.ui, sw, sh, x+i, y, 1, h, fill(float64(i)/float64(w)))
		}
	}
	m := 5 * u
	bw, bh := 75*u, 6*u
	if rc.HUDMaxHP > 0 {
		bar(m, sh-m-bh, bw, bh, float64(rc.HUDHP)/float64(rc.HUDMaxHP), func(f float64) [4]uint8 {
			return [4]uint8{uint8(255 * (1 - f)), uint8(255 * f), 0, 220}
		})
	}
	if rc.HUDAmmoMax > 0 { // hidden when the game has no ammo
		aw := 65 * u
		bar(sw-m-aw, sh-m-bh, aw, bh, float64(rc.HUDAmmo)/float64(rc.HUDAmmoMax), func(float64) [4]uint8 {
			return [4]uint8{220, 160, 40, 220}
		})
	}
}

// drawHUDText draws script HUD lines, the centred message and the overlay.
func (rc *Raycaster) drawHUDText(sw, sh int) {
	white := [4]uint8{235, 235, 235, 255}
	scale := UIScale(sh)
	if rc.OverlayTitle == "" {
		for i, line := range rc.HUDLines {
			DrawTextScaled(rc.ui, sw, sh, 6*scale, 6*scale+FontAccentRows*scale+i*(FontH+FontAccentRows+1)*scale, line, white, scale)
		}
	}

	if rc.OverlayTitle != "" {
		veil := [4]uint8{0, 0, 0, 175}
		if rc.OverlayOpaque {
			veil[3] = 255
		}
		FillRect(rc.ui, sw, sh, 0, 0, sw, sh, veil)
		col := rc.OverlayColor
		if col[3] == 0 {
			col = [4]uint8{255, 220, 90, 255}
		}
		big := scale * 3
		DrawTextScaled(rc.ui, sw, sh, (sw-TextWidth(rc.OverlayTitle, big))/2, sh/3, rc.OverlayTitle, col, big)
		y := sh/3 + FontH*big + 24
		for _, line := range splitLines(rc.OverlaySubtitle) {
			DrawTextScaled(rc.ui, sw, sh, (sw-TextWidth(line, scale))/2, y, line, white, scale)
			y += (FontH+FontAccentRows*2)*scale + 4
		}
		return
	}

	if rc.HUDMessage != "" {
		ms := scale + 1
		yellow := [4]uint8{255, 230, 120, 255}
		y := sh / 4
		for _, line := range splitLines(rc.HUDMessage) {
			DrawTextScaled(rc.ui, sw, sh, (sw-TextWidth(line, ms))/2, y, line, yellow, ms)
			y += (FontH+FontAccentRows*2)*ms + 2
		}
	}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// applyScreenEffects applies the colour parts of EffectsConfig to the CPU
// buffer. Neutral settings cost nothing. (Blur, grain and chromatic
// aberration are not implemented yet.)
func (rc *Raycaster) applyScreenEffects(sw, sh int) {
	fx := rc.effects
	if fx == nil {
		return
	}
	red := clamp01(fx.RedTint)
	desat := clamp01(fx.Desaturate)
	vig := clamp01(fx.Vignette)
	bright := fx.Brightness
	if bright <= 0 {
		bright = 1
	}
	if red == 0 && desat == 0 && vig == 0 && bright == 1 {
		return
	}
	cx, cy := float64(sw)/2, float64(sh)/2
	maxD2 := cx*cx + cy*cy
	parallel(sh, func(y0, y1 int) {
		rc.effectRows(y0, y1, sw, red, desat, vig, bright, cx, cy, maxD2)
	})
}

func (rc *Raycaster) effectRows(y0, y1, sw int, red, desat, vig, bright, cx, cy, maxD2 float64) {
	for y := y0; y < y1; y++ {
		dy := float64(y) - cy
		for x := 0; x < sw; x++ {
			i := (y*sw + x) * 4
			r, g, b := float64(rc.pixels[i]), float64(rc.pixels[i+1]), float64(rc.pixels[i+2])
			if desat > 0 {
				l := 0.299*r + 0.587*g + 0.114*b
				r += (l - r) * desat
				g += (l - g) * desat
				b += (l - b) * desat
			}
			if red > 0 {
				r += (255 - r) * red * 0.6
				g *= 1 - red*0.5
				b *= 1 - red*0.5
			}
			k := bright
			if vig > 0 {
				dx := float64(x) - cx
				k *= 1 - vig*(dx*dx+dy*dy)/maxD2
			}
			rc.pixels[i] = clampByte(r * k)
			rc.pixels[i+1] = clampByte(g * k)
			rc.pixels[i+2] = clampByte(b * k)
		}
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clampByte(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v)
}

// ReadScreen reads the current framebuffer (call after Render, before swap)
// into a top-down RGBA image.
func (rc *Raycaster) ReadScreen() *image.RGBA {
	vw, vh := rc.viewport()
	w, h := int(vw), int(vh)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	buf := make([]uint8, w*h*4)
	gl.PixelStorei(gl.PACK_ALIGNMENT, 1)
	gl.ReadPixels(0, 0, vw, vh, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(buf))
	for y := 0; y < h; y++ { // OpenGL rows are bottom-up
		copy(img.Pix[y*w*4:(y+1)*w*4], buf[(h-1-y)*w*4:(h-y)*w*4])
	}
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	return img
}

// drawConsole renders the cheat console overlay on the pixel buffer.
func (rc *Raycaster) drawConsole(sw, sh int) {
	bg := [4]uint8{0, 0, 0, 200}
	white := [4]uint8{220, 220, 220, 255}
	green := [4]uint8{80, 255, 80, 255}

	startY := sh / 2
	for y := startY; y < sh; y++ {
		for x := 0; x < sw; x++ {
			idx := (y*sw + x) * 4
			rc.ui[idx] = bg[0]
			rc.ui[idx+1] = bg[1]
			rc.ui[idx+2] = bg[2]
			rc.ui[idx+3] = bg[3]
		}
	}

	u := max(1, UIScale(sh)/2)
	lineH := (FontH + 2) * u
	prompt := "> " + rc.ConsoleInput + "_"
	DrawTextScaled(rc.ui, sw, sh, 8*u, sh-lineH-4*u, prompt, green, u)

	visible := (sh/2 - 20*u) / lineH
	if visible > len(rc.ConsoleHistory) {
		visible = len(rc.ConsoleHistory)
	}
	for i := 0; i < visible; i++ {
		line := rc.ConsoleHistory[len(rc.ConsoleHistory)-visible+i]
		DrawTextScaled(rc.ui, sw, sh, 8*u, sh-lineH-4*u-(visible-i)*lineH, line, white, u)
	}
}

// DrawHUDWeapon renders the player's animated first-person sprite over the HUD.
func (rc *Raycaster) DrawHUDWeapon() {
	if rc.HUDWeaponTex == "" {
		return
	}
	tex := rc.TexManager.Get(rc.HUDWeaponTex)
	if tex == nil {
		return
	}

	sw := float32(rc.screenW)
	sh := float32(rc.screenH)
	height := sh * 0.42
	width := height * (float32(tex.Width) / float32(tex.Height))
	x := sw*0.60 - width*0.25
	y := sh*0.58 + float32(math.Sin(rc.HUDWeaponBob))*8.0

	rc.spriteProg.Use()
	rc.spriteProg.SetUniform1i("uFlipY", 1)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Disable(gl.DEPTH_TEST)
	rc.spriteProg.SetUniform2f("uScreenSize", sw, sh)
	rc.spriteProg.SetUniform2f("uSpritePos", x, y)
	rc.spriteProg.SetUniform2f("uSpriteSize", width, height)
	rc.spriteProg.SetUniform1f("uAlpha", 1.0)
	rc.spriteProg.SetUniform1f("uShade", 1.0)
	rc.spriteProg.SetUniform1f("uTexX", 0.0)
	rc.spriteProg.SetUniform1i("uUseFixedTexX", 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, tex.ID)
	rc.spriteProg.SetUniform1i("uTexture", 0)
	gl.BindVertexArray(rc.wallVAO)
	gl.DrawArrays(gl.TRIANGLES, 0, 6)
	gl.BindVertexArray(0)
	gl.Disable(gl.BLEND)
	gl.Enable(gl.DEPTH_TEST)
}

// ApplyEffects stores the effects config for the next frame.
func (rc *Raycaster) ApplyEffects(cfg *EffectsConfig) {
	rc.effects = cfg
}

// Shutdown releases OpenGL resources.
func (rc *Raycaster) Shutdown() {
	if rc.simpleProg != nil {
		rc.simpleProg.Delete()
	}
	if rc.spriteProg != nil {
		rc.spriteProg.Delete()
	}
	if rc.quadVAO != 0 {
		gl.DeleteVertexArrays(1, &rc.quadVAO)
	}
	if rc.quadVBO != 0 {
		gl.DeleteBuffers(1, &rc.quadVBO)
	}
	if rc.wallVAO != 0 {
		gl.DeleteVertexArrays(1, &rc.wallVAO)
	}
	if rc.wallVBO != 0 {
		gl.DeleteBuffers(1, &rc.wallVBO)
	}
	if rc.screenTex != 0 {
		gl.DeleteTextures(1, &rc.screenTex)
	}
	if rc.uiTex != 0 {
		gl.DeleteTextures(1, &rc.uiTex)
	}
}

// checkGLError logs OpenGL errors.
func checkGLError(tag string) {
	err := gl.GetError()
	if err != gl.NO_ERROR {
		log.Printf("[GL Error] %s: 0x%x", tag, err)
	}
}

// colorToRGBA converts a color.RGBA to [4]uint8.
func colorToRGBA(c color.RGBA) [4]uint8 {
	return [4]uint8{c.R, c.G, c.B, c.A}
}

// FillRect overwrites a clipped rectangle with col (alpha included); used for
// drawing into the UI layer.
func FillRect(buf []uint8, sw, sh, x0, y0, w, h int, col [4]uint8) {
	x1, y1 := x0+w, y0+h
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > sw {
		x1 = sw
	}
	if y1 > sh {
		y1 = sh
	}
	if x0 >= x1 {
		return
	}
	for y := y0; y < y1; y++ {
		row := buf[(y*sw+x0)*4 : (y*sw+x1)*4]
		for i := 0; i < len(row); i += 4 {
			row[i], row[i+1], row[i+2], row[i+3] = col[0], col[1], col[2], col[3]
		}
	}
}
