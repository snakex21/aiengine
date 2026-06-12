// renderer/raycaster.go — DDA raycasting engine for 2.5D rendering.
// CPU pixel-buffer approach: renders walls+floor/ceiling into a byte buffer,
// uploads to a single OpenGL texture, and draws a fullscreen quad.
// Sprites are rendered as GPU quads on top. Zero FBO, single shader for walls.

package renderer

import (
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

// PlayerState holds the player's position and orientation for the raycaster.
type PlayerState struct {
	X         float64
	Y         float64
	Z         float64 // vertical height above ground, in wall-height units (0 = standing)
	VelZ      float64 // vertical velocity for jumping/falling
	Grounded  bool    // true while standing on the ground
	Angle     float64
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

// Jump starts an upward jump if the player is currently grounded.
// It is a no-op while airborne, preventing double jumps.
func (p *PlayerState) Jump() {
	if p.Grounded {
		p.VelZ = JumpVelocity
		p.Grounded = false
	}
}

// UpdateVertical advances the player's vertical position by dt seconds,
// applying gravity while airborne and landing on the ground at Z=0.
func (p *PlayerState) UpdateVertical(dt float64) {
	if p.Grounded {
		return
	}
	p.VelZ -= GravityAccel * dt
	p.Z += p.VelZ * dt
	if p.Z >= MaxJumpZ && p.VelZ > 0 {
		p.Z = MaxJumpZ
		p.VelZ = 0
	}
	if p.Z <= 0 {
		p.Z = 0
		p.VelZ = 0
		p.Grounded = true
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

	// Z-buffer for sprite occlusion.
	zBuffer []float64

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
}

func NewRaycaster(m *MapData, p *PlayerState, tm *TextureManager) *Raycaster {
	return &Raycaster{
		Map:        m,
		Player:     p,
		TexManager: tm,
		effects:    DefaultEffects(),
	}
}

// Init sets up OpenGL resources.
func (rc *Raycaster) Init(screenW, screenH int32) error {
	rc.screenW = screenW
	rc.screenH = screenH
	rc.zBuffer = make([]float64, screenW)
	rc.pixels = make([]uint8, screenW*screenH*4)

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
	quadVerts := []float32{
		-1, -1, 0, 0,
		1, -1, 1, 0,
		1, 1, 1, 1,
		-1, -1, 0, 0,
		1, 1, 1, 1,
		-1, 1, 0, 1,
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

	checkGLError("Init end")
	return nil
}

// Render executes a full frame.
func (rc *Raycaster) Render(sprites []*Sprite) {
	gl.Viewport(0, 0, rc.screenW, rc.screenH)
	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT)

	// 1. CPU: fill pixel buffer with floor, ceiling, and walls.
	rc.renderToBuffer(sprites)

	// 2. Upload to texture.
	gl.BindTexture(gl.TEXTURE_2D, rc.screenTex)
	gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, rc.screenW, rc.screenH, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rc.pixels))

	// 3. Draw fullscreen quad.
	gl.Disable(gl.DEPTH_TEST)
	rc.simpleProg.Use()
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, rc.screenTex)
	rc.simpleProg.SetUniform1i("uTex", 0)
	gl.BindVertexArray(rc.quadVAO)
	gl.DrawArrays(gl.TRIANGLES, 0, 6)
	gl.BindVertexArray(0)

	// 4. Draw sprites as GPU overlay (using Z-buffer from DDA).
	rc.drawSprites(sprites)
	rc.DrawHUDWeapon()
	gl.Enable(gl.DEPTH_TEST)

	checkGLError("Render end")
}

// renderToBuffer fills rc.pixels with ceiling, floor, and wall columns.
func (rc *Raycaster) renderToBuffer(sprites []*Sprite) {
	sw := int(rc.screenW)
	sh := int(rc.screenH)

	// Fill ceiling and floor.
	ceil := colorToRGBA(ceilColor)
	flr := colorToRGBA(floorColor)

	for y := 0; y < sh/2; y++ {
		for x := 0; x < sw; x++ {
			idx := (y*sw + x) * 4
			rc.pixels[idx] = ceil[0]
			rc.pixels[idx+1] = ceil[1]
			rc.pixels[idx+2] = ceil[2]
			rc.pixels[idx+3] = ceil[3]
		}
	}
	for y := sh / 2; y < sh; y++ {
		for x := 0; x < sw; x++ {
			idx := (y*sw + x) * 4
			rc.pixels[idx] = flr[0]
			rc.pixels[idx+1] = flr[1]
			rc.pixels[idx+2] = flr[2]
			rc.pixels[idx+3] = flr[3]
		}
	}

	// Reset Z-buffer.
	for i := range rc.zBuffer {
		rc.zBuffer[i] = 0
	}

	fsw := float64(sw)
	fsh := float64(sh)
	halfFOV := rc.Player.FOV / 2.0

	// Raycast each column.
	for x := 0; x < sw; x++ {
		cameraX := 2.0*float64(x)/fsw - 1.0
		rayAngle := rc.Player.Angle + cameraX*halfFOV

		rayDirX := math.Cos(rayAngle)
		rayDirY := math.Sin(rayAngle)

		mapX := int(math.Floor(rc.Player.X))
		mapY := int(math.Floor(rc.Player.Y))

		deltaDistX := math.Abs(1.0 / rayDirX)
		deltaDistY := math.Abs(1.0 / rayDirY)
		if math.IsInf(deltaDistX, 0) {
			deltaDistX = 1e30
		}
		if math.IsInf(deltaDistY, 0) {
			deltaDistY = 1e30
		}

		var sideDistX, sideDistY float64
		var stepX, stepY int

		if rayDirX < 0 {
			stepX = -1
			sideDistX = (rc.Player.X - float64(mapX)) * deltaDistX
		} else {
			stepX = 1
			sideDistX = (float64(mapX) + 1.0 - rc.Player.X) * deltaDistX
		}
		if rayDirY < 0 {
			stepY = -1
			sideDistY = (rc.Player.Y - float64(mapY)) * deltaDistY
		} else {
			stepY = 1
			sideDistY = (float64(mapY) + 1.0 - rc.Player.Y) * deltaDistY
		}

		var hit bool
		var side int
		var wallType MapCell
		const maxSteps = 64

		for step := 0; step < maxSteps; step++ {
			if sideDistX < sideDistY {
				sideDistX += deltaDistX
				mapX += stepX
				side = 0
			} else {
				sideDistY += deltaDistY
				mapY += stepY
				side = 1
			}
			cell := rc.Map.GetCell(mapX, mapY)
			if cell != CellEmpty {
				hit = true
				wallType = cell
				break
			}
		}

		if !hit {
			continue
		}

		// Perpendicular distance.
		var perpDist float64
		if side == 0 {
			perpDist = sideDistX - deltaDistX
		} else {
			perpDist = sideDistY - deltaDistY
		}
		if perpDist < 0.001 {
			perpDist = 0.001
		}

		rc.zBuffer[x] = perpDist

		// Wall height on screen.
		wallHeight := fsh / perpDist
		// Vertical camera offset for jumping: a world-space height of Z
		// projects to Z*wallHeight pixels at this distance. Raising the camera
		// (Z>0) shifts walls downward on screen.
		vOffset := rc.Player.Z * wallHeight
		drawStart := int((fsh-wallHeight)/2.0 + vOffset)
		drawEnd := int(drawStart) + int(wallHeight)
		if drawStart < 0 {
			drawStart = 0
		}
		if drawEnd >= sh {
			drawEnd = sh
		}

		// Compute texture X coordinate.
		var wallX float64
		if side == 0 {
			wallX = rc.Player.Y + perpDist*rayDirY
		} else {
			wallX = rc.Player.X + perpDist*rayDirX
		}
		wallX -= math.Floor(wallX)

		// Distance shading.
		shade := 1.0
		if perpDist > 1.0 {
			shade = 1.0 / math.Sqrt(perpDist)
		}
		if side == 1 {
			shade *= 0.7 // Y-side darker
		}

		// Select texture by wall type.
		texName := "brick"
		switch wallType {
		case CellWall1:
			texName = "brick"
		case CellWall2:
			texName = "stone"
		case CellWall3:
			texName = "metal"
		case CellDoor:
			texName = "wood"
		}
		tex := rc.TexManager.Get(texName)

		if tex != nil && len(tex.Pixels) > 0 {
			texW := int(tex.Width)
			texH := int(tex.Height)
			texX := int(wallX*float64(texW)) % texW
			if texX < 0 {
				texX += texW
			}
			step := float64(texH) / wallHeight
			texPos := (float64(drawStart) - fsh/2.0 + wallHeight/2.0 - vOffset) * step

			for y := drawStart; y < drawEnd; y++ {
				texY := int(texPos) & (texH - 1)
				texPos += step
				srcIdx := (texY*texW + texX) * 4
				dstIdx := (y*sw + x) * 4
				rc.pixels[dstIdx] = uint8(float64(tex.Pixels[srcIdx]) * shade)
				rc.pixels[dstIdx+1] = uint8(float64(tex.Pixels[srcIdx+1]) * shade)
				rc.pixels[dstIdx+2] = uint8(float64(tex.Pixels[srcIdx+2]) * shade)
				rc.pixels[dstIdx+3] = 255
			}
		} else {
			// Fallback solid color.
			wc := wallColors[wallType]
			r := uint8(float64(wc.R) * shade)
			g := uint8(float64(wc.G) * shade)
			b := uint8(float64(wc.B) * shade)
			texStripe := uint8(float64(wallX) * 255.0)
			r = uint8(float64(r)*0.7 + float64(texStripe)*0.3)
			g = uint8(float64(g)*0.7 + float64(texStripe)*0.3)
			b = uint8(float64(b)*0.7 + float64(texStripe)*0.3)
			for y := drawStart; y < drawEnd; y++ {
				idx := (y*sw + x) * 4
				rc.pixels[idx] = r
				rc.pixels[idx+1] = g
				rc.pixels[idx+2] = b
				rc.pixels[idx+3] = 255
			}
		}
	}

	// Draw HUD overlay.
	rc.drawHUD(sw, sh)

	// Draw console overlay on top of everything.
	if rc.ConsoleOpen {
		rc.drawConsole(sw, sh)
	}
}

// drawHUD renders crosshair, HP bar and ammo counter.
func (rc *Raycaster) drawHUD(sw, sh int) {
	cx := sw / 2
	cy := sh / 2
	crossLen := 8
	crossGap := 2
	white := [4]uint8{255, 255, 255, 255}

	for x := cx - crossLen; x < cx-crossGap; x++ {
		if x >= 0 && x < sw {
			idx := (cy*sw + x) * 4
			rc.pixels[idx], rc.pixels[idx+1], rc.pixels[idx+2], rc.pixels[idx+3] = white[0], white[1], white[2], white[3]
		}
	}
	for x := cx + crossGap; x < cx+crossLen; x++ {
		if x >= 0 && x < sw {
			idx := (cy*sw + x) * 4
			rc.pixels[idx], rc.pixels[idx+1], rc.pixels[idx+2], rc.pixels[idx+3] = white[0], white[1], white[2], white[3]
		}
	}
	for y := cy - crossLen; y < cy-crossGap; y++ {
		if y >= 0 && y < sh {
			idx := (y*sw + cx) * 4
			rc.pixels[idx], rc.pixels[idx+1], rc.pixels[idx+2], rc.pixels[idx+3] = white[0], white[1], white[2], white[3]
		}
	}
	for y := cy + crossGap; y < cy+crossLen; y++ {
		if y >= 0 && y < sh {
			idx := (y*sw + cx) * 4
			rc.pixels[idx], rc.pixels[idx+1], rc.pixels[idx+2], rc.pixels[idx+3] = white[0], white[1], white[2], white[3]
		}
	}

	// HP bar.
	barX, barY, barW, barH := 10, sh-30, 150, 12
	borderColor := [4]uint8{60, 60, 60, 255}
	bgColor := [4]uint8{30, 30, 30, 200}
	for y := barY - 1; y <= barY+barH; y++ {
		for x := barX - 1; x <= barX+barW; x++ {
			if y >= 0 && y < sh && x >= 0 && x < sw {
				idx := (y*sw + x) * 4
				rc.pixels[idx], rc.pixels[idx+1], rc.pixels[idx+2], rc.pixels[idx+3] = borderColor[0], borderColor[1], borderColor[2], borderColor[3]
			}
		}
	}
	for y := barY; y < barY+barH; y++ {
		for x := barX; x < barX+barW; x++ {
			idx := (y*sw + x) * 4
			rc.pixels[idx], rc.pixels[idx+1], rc.pixels[idx+2], rc.pixels[idx+3] = bgColor[0], bgColor[1], bgColor[2], bgColor[3]
		}
	}
	if rc.HUDMaxHP > 0 {
		ratio := float64(rc.HUDHP) / float64(rc.HUDMaxHP)
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
		fillW := int(float64(barW) * ratio)
		for y := barY; y < barY+barH; y++ {
			for x := barX; x < barX+fillW; x++ {
				frac := float64(x-barX) / float64(barW)
				r := uint8(255 * (1 - frac))
				g := uint8(255 * frac)
				idx := (y*sw + x) * 4
				rc.pixels[idx], rc.pixels[idx+1], rc.pixels[idx+2] = r, g, 0
				rc.pixels[idx+3] = 220
			}
		}
	}

	// Ammo counter.
	ammoX, ammoY, ammoW, ammoH := sw-140, sh-30, 130, 12
	for y := ammoY - 1; y <= ammoY+ammoH; y++ {
		for x := ammoX - 1; x <= ammoX+ammoW; x++ {
			if y >= 0 && y < sh && x >= 0 && x < sw {
				idx := (y*sw + x) * 4
				rc.pixels[idx], rc.pixels[idx+1], rc.pixels[idx+2], rc.pixels[idx+3] = borderColor[0], borderColor[1], borderColor[2], borderColor[3]
			}
		}
	}
	for y := ammoY; y < ammoY+ammoH; y++ {
		for x := ammoX; x < ammoX+ammoW; x++ {
			idx := (y*sw + x) * 4
			rc.pixels[idx], rc.pixels[idx+1], rc.pixels[idx+2], rc.pixels[idx+3] = bgColor[0], bgColor[1], bgColor[2], bgColor[3]
		}
	}
	if rc.HUDAmmoMax > 0 {
		ratio := float64(rc.HUDAmmo) / float64(rc.HUDAmmoMax)
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
		fillW := int(float64(ammoW) * ratio)
		for y := ammoY; y < ammoY+ammoH; y++ {
			for x := ammoX; x < ammoX+fillW; x++ {
				idx := (y*sw + x) * 4
				rc.pixels[idx], rc.pixels[idx+1], rc.pixels[idx+2] = uint8(220), uint8(160), uint8(40)
				rc.pixels[idx+3] = 220
			}
		}
	}
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
			rc.pixels[idx] = bg[0]
			rc.pixels[idx+1] = bg[1]
			rc.pixels[idx+2] = bg[2]
			rc.pixels[idx+3] = bg[3]
		}
	}

	prompt := "> " + rc.ConsoleInput + "_"
	DrawConsoleString(rc.pixels, sw, 8, sh-14, prompt, green)

	visible := (sh/2 - 20) / FontH
	if visible > len(rc.ConsoleHistory) {
		visible = len(rc.ConsoleHistory)
	}
	for i := 0; i < visible; i++ {
		line := rc.ConsoleHistory[len(rc.ConsoleHistory)-visible+i]
		DrawConsoleString(rc.pixels, sw, 8, sh-14-(visible-i)*FontH, line, white)
	}
}

// drawSprites renders sprites as colored columns using the Z-buffer.
// Uses angular approach: compute angle to sprite, delta from player angle,
// then perpendicular distance for Z-buffer and screen height.
func (rc *Raycaster) drawSprites(sprites []*Sprite) {
	if len(sprites) == 0 {
		return
	}
	sw := float64(rc.screenW)
	sh := float64(rc.screenH)
	halfFOV := rc.Player.FOV / 2.0

	// Pre-compute sprite data: Euclidean distance + perpendicular distance.
	type spriteData struct {
		s        *Sprite
		euclDist float64 // Euclidean distance (for sorting)
		perpDist float64 // perpendicular distance (for Z-buffer & height)
		screenX  float64 // screen X center
		angle    float64 // world angle to sprite
	}
	var sorted []spriteData

	for _, s := range sprites {
		if !s.Active {
			continue
		}
		dx := s.X - rc.Player.X
		dy := s.Y - rc.Player.Y
		euclDist := math.Sqrt(dx*dx + dy*dy)
		angle := math.Atan2(dy, dx)

		// Delta angle (normalized to [-PI, PI]).
		delta := angle - rc.Player.Angle
		for delta > math.Pi {
			delta -= 2 * math.Pi
		}
		for delta < -math.Pi {
			delta += 2 * math.Pi
		}

		// Sprite must be within FOV.
		if math.Abs(delta) > halfFOV+0.2 {
			continue
		}

		// Perpendicular distance (projection onto view direction).
		perpDist := euclDist * math.Cos(delta)
		if perpDist < 0.1 {
			continue
		}

		// Screen X: map angular offset to screen columns.
		screenX := (sw / 2.0) + (delta/halfFOV)*(sw/2.0)

		sorted = append(sorted, spriteData{
			s:        s,
			euclDist: euclDist,
			perpDist: perpDist,
			screenX:  screenX,
			angle:    angle,
		})
	}

	// Sort by perpendicular distance: FAR → NEAR (paint far sprites first).
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].perpDist < sorted[j].perpDist {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	rc.spriteProg.Use()
	rc.spriteProg.SetUniform1i("uFlipY", 0)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Disable(gl.DEPTH_TEST)

	for _, sd := range sorted {
		s := sd.s
		perpDist := sd.perpDist

		// Sprite height on screen (proportional to 1/distance).
		spriteHeight := (sh / perpDist) * s.Scale
		spriteWidth := spriteHeight // square sprites

		// Vertical center offset by sprite Z, plus player jump height so
		// grounded sprites stay anchored to the floor as the camera rises.
		drawStartY := (sh-spriteHeight)/2.0 + s.Z*sh/perpDist + rc.Player.Z*sh/perpDist
		drawEndY := drawStartY + spriteHeight

		// Horizontal bounds.
		drawStartX := sd.screenX - spriteWidth/2.0
		drawEndX := sd.screenX + spriteWidth/2.0

		// Clipping.
		if drawEndX < 0 || drawStartX >= sw || drawEndY < 0 || drawStartY >= sh {
			continue
		}

		// Distance-based shading.
		shade := float32(1.0)
		if perpDist > 1.0 {
			shade = float32(1.0 / math.Sqrt(perpDist))
		}

		tex := rc.TexManager.Get(s.TexName)
		texID := uint32(0)
		if tex != nil {
			texID = tex.ID
		}

		// Render sprite column by column with Z-buffer check.
		startCol := int(math.Max(0, drawStartX))
		endCol := int(math.Min(sw-1, drawEndX))

		for stripe := startCol; stripe <= endCol; stripe++ {
			// Z-buffer: skip if wall at this column is closer.
			if perpDist > rc.zBuffer[stripe] && rc.zBuffer[stripe] > 0 {
				continue
			}

			texX := (float64(stripe) - drawStartX) / spriteWidth
			if texX < 0 {
				texX = 0
			}
			if texX > 1.0 {
				texX = 1.0
			}

			rc.spriteProg.SetUniform2f("uScreenSize", float32(sw), float32(sh))
			rc.spriteProg.SetUniform2f("uSpritePos", float32(stripe), float32((drawStartY+drawEndY)/2.0))
			rc.spriteProg.SetUniform2f("uSpriteSize", 1.0, float32((drawEndY-drawStartY)/2.0))
			rc.spriteProg.SetUniform1f("uAlpha", s.Alpha)
			rc.spriteProg.SetUniform1f("uShade", shade)
			rc.spriteProg.SetUniform1f("uTexX", float32(texX))
			rc.spriteProg.SetUniform1i("uUseFixedTexX", 1)

			gl.ActiveTexture(gl.TEXTURE0)
			gl.BindTexture(gl.TEXTURE_2D, texID)
			rc.spriteProg.SetUniform1i("uTexture", 0)

			gl.BindVertexArray(rc.wallVAO)
			gl.DrawArrays(gl.TRIANGLES, 0, 6)
		}
	}

	gl.BindVertexArray(0)
	gl.Disable(gl.BLEND)
	gl.Enable(gl.DEPTH_TEST)
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
