// main.go - AIEngine entry point
// Initializes GLFW window, OpenGL context, renderer, scripting engine, and runs the main game loop.
// Fullscreen toggle via F11. Window size from config.aie (WINDOW_WIDTH/WINDOW_HEIGHT).
// ESC emits "onPause" event — the game (JS/Lua) decides what to do.

package main

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"

	"aiengine/input"
	"aiengine/parser"
	"aiengine/renderer"
	"aiengine/scripting"
)

const (
	WindowTitle = "AIEngine"
)

// App holds the top-level engine state for the main loop.
type App struct {
	window        *glfw.Window
	inputManager  *input.Manager
	fullscreen    bool
	windowedW     int
	windowedH     int
	windowedX     int
	windowedY     int
	lastFrameTime time.Time
	deltaTime     float64
	paused        bool

	// Renderer.
	raycaster  *renderer.Raycaster
	texManager *renderer.TextureManager
	sprites    []*renderer.Sprite
	player     *renderer.PlayerState
	effects    *renderer.EffectsConfig

	// Scripting.
	scriptEngine *scripting.Engine
	luaBridge    *scripting.LuaBridge
	jsBridge     *scripting.JSBridge

	// Config.
	config *parser.Config

	// Mouse look.
	prevMouseX float64
	prevMouseY float64
	mouseInit  bool
}

func main() {
	runtime.LockOSThread()

	app := &App{}

	if err := app.init(); err != nil {
		log.Fatalf("AIEngine initialization failed: %v", err)
	}
	defer app.shutdown()

	app.run()
}

// init sets up GLFW, OpenGL, input, renderer, scripting, and test content.
func (app *App) init() error {
	// --- Load config FIRST ---
	cfg, err := parser.ParseConfigFile("demo/config.aie")
	if err != nil {
		log.Printf("Warning: could not load config.aie: %v (using defaults)", err)
		cfg = &parser.Config{}
	}
	app.config = cfg

	winW := cfg.GetWindowWidth()
	winH := cfg.GetWindowHeight()
	app.windowedW = winW
	app.windowedH = winH
	log.Printf("Window size from config: %dx%d", winW, winH)

	// --- GLFW ---
	if err := glfw.Init(); err != nil {
		return fmt.Errorf("glfw.Init: %w", err)
	}

	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 3)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	glfw.WindowHint(glfw.Resizable, glfw.False)

	window, err := glfw.CreateWindow(winW, winH, WindowTitle, nil, nil)
	if err != nil {
		glfw.Terminate()
		return fmt.Errorf("glfw.CreateWindow: %w", err)
	}
	app.window = window
	window.MakeContextCurrent()
	glfw.SwapInterval(0)

	// Center window on primary monitor.
	monitor := glfw.GetPrimaryMonitor()
	mode := monitor.GetVideoMode()
	window.SetPos(
		(mode.Width-winW)/2,
		(mode.Height-winH)/2,
	)

	// --- OpenGL ---
	if err := gl.Init(); err != nil {
		window.Destroy()
		glfw.Terminate()
		return fmt.Errorf("gl.Init: %w", err)
	}

	log.Printf("OpenGL Vendor:   %s", gl.GoStr(gl.GetString(gl.VENDOR)))
	log.Printf("OpenGL Renderer: %s", gl.GoStr(gl.GetString(gl.RENDERER)))
	log.Printf("OpenGL Version:  %s", gl.GoStr(gl.GetString(gl.VERSION)))

	gl.Viewport(0, 0, int32(winW), int32(winH))
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)

	// --- Input system ---
	app.inputManager = input.NewManager()
	// Pass bindings from config to input manager.
	if cfg != nil && len(cfg.Bindings) > 0 {
		app.inputManager.SetDefaults() // ensure all defaults exist
		// Override with config bindings via re-loading (inputManager reads config.aie).
		app.inputManager.LoadConfig("demo/config.aie")
	} else {
		app.inputManager.SetDefaults()
	}

	// --- Texture manager ---
	app.texManager = renderer.NewTextureManager()
	if err := app.texManager.GenerateDefaultTextures(); err != nil {
		return fmt.Errorf("texture generation: %w", err)
	}

	// --- Map & Player ---
	mapData := createTestMap()
	app.player = renderer.NewPlayerState(2.5, 2.5, 0.0)

	// --- Raycaster ---
	app.raycaster = renderer.NewRaycaster(mapData, app.player, app.texManager)
	if err := app.raycaster.Init(int32(winW), int32(winH)); err != nil {
		return fmt.Errorf("raycaster init: %w", err)
	}

	// --- Sprites & Effects ---
	app.sprites = renderer.CreateDefaultSprites()
	app.effects = renderer.DefaultEffects()

	// --- Scripting Engine ---
	app.scriptEngine = scripting.NewEngine()
	app.scriptEngine.Effects = app.effects

	// Lua bridge.
	app.luaBridge = scripting.NewLuaBridge(app.scriptEngine)
	// Try loading demo Lua scripts.
	if err := app.luaBridge.DoFile("demo/scripts/level1.lua"); err != nil {
		log.Printf("Lua level1.lua: %v", err)
	}
	if err := app.luaBridge.DoFile("demo/scripts/boss_kaiju.lua"); err != nil {
		log.Printf("Lua boss_kaiju.lua: %v", err)
	}

	// JS bridge.
	app.jsBridge = scripting.NewJSBridge(app.scriptEngine)
	if _, err := app.jsBridge.RunFile("demo/scripts/hud.js"); err != nil {
		log.Printf("JS hud.js: %v", err)
	}

	// Emit onStart event.
	app.scriptEngine.Emit("onStart")

	// --- GLFW callbacks ---
	window.SetKeyCallback(app.keyCallback)
	window.SetMouseButtonCallback(app.mouseButtonCallback)
	window.SetSizeCallback(app.sizeCallback)
	window.SetCloseCallback(func(w *glfw.Window) {
		w.SetShouldClose(true)
	})
	window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)

	app.lastFrameTime = time.Now()
	log.Println("AIEngine initialized successfully.")
	return nil
}

// createTestMap builds a simple test map for the raycaster.
func createTestMap() *renderer.MapData {
	grid := [][]renderer.MapCell{
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		{1, 0, 0, 0, 0, 1, 0, 0, 0, 1},
		{1, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		{1, 0, 0, 2, 0, 0, 0, 3, 0, 1},
		{1, 0, 0, 2, 0, 1, 0, 0, 0, 1},
		{1, 0, 0, 0, 0, 1, 0, 0, 0, 1},
		{1, 0, 0, 0, 0, 0, 0, 2, 0, 1},
		{1, 0, 3, 0, 0, 0, 0, 2, 0, 1},
		{1, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	}
	return renderer.NewMapData(grid)
}

// run executes the main game loop.
func (app *App) run() {
	for !app.window.ShouldClose() {
		app.calculateDeltaTime()

		glfw.PollEvents()
		app.handleInput()

		if !app.paused {
			app.update(app.deltaTime)
		}
		app.render()

		// CRITICAL: Update input state at END of frame, so IsJustPressed
		// sees the difference between previous (last frame) and held (just set by PollEvents).
		app.inputManager.Update()

		// Tick script timers.
		app.scriptEngine.TickTimers()

		app.window.SwapBuffers()
	}
}

func (app *App) calculateDeltaTime() {
	now := time.Now()
	dt := now.Sub(app.lastFrameTime).Seconds()
	app.lastFrameTime = now
	const maxDT = 0.1
	if dt > maxDT {
		dt = maxDT
	}
	app.deltaTime = dt
}

func (app *App) handleInput() {
	if app.inputManager.IsJustPressed("fullscreen") {
		app.toggleFullscreen()
	}
	// ESC emits "onPause" event — game (JS/Lua) decides what to do.
	if app.inputManager.IsJustPressed("pause") {
		app.paused = !app.paused
		if app.paused {
			app.window.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
		} else {
			app.window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)
			// Reset mouse delta tracking to avoid jump on resume.
			app.mouseInit = false
		}
		app.scriptEngine.Emit("onPause", app.paused)
		log.Printf("[Engine] Pause toggled: %v", app.paused)
	}
}

func (app *App) toggleFullscreen() {
	if app.fullscreen {
		app.window.SetMonitor(nil, app.windowedX, app.windowedY, app.windowedW, app.windowedH, 0)
		gl.Viewport(0, 0, int32(app.windowedW), int32(app.windowedH))
		app.fullscreen = false
		log.Printf("Switched to windowed mode (%dx%d)", app.windowedW, app.windowedH)
	} else {
		app.windowedX, app.windowedY = app.window.GetPos()
		app.windowedW, app.windowedH = app.window.GetSize()
		monitor := glfw.GetPrimaryMonitor()
		mode := monitor.GetVideoMode()
		app.window.SetMonitor(monitor, 0, 0, mode.Width, mode.Height, mode.RefreshRate)
		app.fullscreen = true
		log.Printf("Switched to fullscreen (%dx%d)", mode.Width, mode.Height)
	}
}

// update handles per-frame game logic: player movement, sprite updates.
func (app *App) update(dt float64) {
	speed := app.player.MoveSpeed * dt

	moveX := 0.0
	moveY := 0.0

	if app.inputManager.IsPressed("move_forward") {
		moveX += math.Cos(app.player.Angle) * speed
		moveY += math.Sin(app.player.Angle) * speed
	}
	if app.inputManager.IsPressed("move_backward") {
		moveX -= math.Cos(app.player.Angle) * speed
		moveY -= math.Sin(app.player.Angle) * speed
	}
	if app.inputManager.IsPressed("move_left") {
		moveX += math.Sin(app.player.Angle) * speed
		moveY -= math.Cos(app.player.Angle) * speed
	}
	if app.inputManager.IsPressed("move_right") {
		moveX -= math.Sin(app.player.Angle) * speed
		moveY += math.Cos(app.player.Angle) * speed
	}

	// Predictive collision per-axis (wall sliding).
	newX := app.player.X + moveX
	if !app.isWallAt(newX, app.player.Y) {
		app.player.X = newX
	}
	newY := app.player.Y + moveY
	if !app.isWallAt(app.player.X, newY) {
		app.player.Y = newY
	}

	// Mouse look.
	cx, cy := app.window.GetCursorPos()
	if !app.mouseInit {
		app.prevMouseX = cx
		app.prevMouseY = cy
		app.mouseInit = true
	}
	dx := cx - app.prevMouseX
	dy := cy - app.prevMouseY
	if dx != 0 || dy != 0 {
		app.player.Angle += dx * 0.003
	}
	app.prevMouseX = cx
	app.prevMouseY = cy
}

func (app *App) isWallAt(x, y float64) bool {
	const margin = 0.15
	cx := int(math.Floor(x))
	cy := int(math.Floor(y))
	if app.raycaster.Map.GetCell(cx, cy) != renderer.CellEmpty {
		return true
	}
	if app.raycaster.Map.GetCell(int(math.Floor(x-margin)), cy) != renderer.CellEmpty {
		return true
	}
	if app.raycaster.Map.GetCell(int(math.Floor(x+margin)), cy) != renderer.CellEmpty {
		return true
	}
	if app.raycaster.Map.GetCell(cx, int(math.Floor(y-margin))) != renderer.CellEmpty {
		return true
	}
	if app.raycaster.Map.GetCell(cx, int(math.Floor(y+margin))) != renderer.CellEmpty {
		return true
	}
	return false
}

// render draws a frame.
func (app *App) render() {
	app.raycaster.ApplyEffects(app.effects)
	app.raycaster.Render(app.sprites)
}

// shutdown cleans up resources.
func (app *App) shutdown() {
	if app.luaBridge != nil {
		app.luaBridge.Close()
	}
	if app.raycaster != nil {
		app.raycaster.Shutdown()
	}
	if app.texManager != nil {
		app.texManager.Delete()
	}
	if app.inputManager != nil {
		app.inputManager.Shutdown()
	}
	if app.window != nil {
		app.window.Destroy()
	}
	glfw.Terminate()
	log.Println("AIEngine shut down.")
}

// --- GLFW Callbacks ---

func (app *App) keyCallback(w *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	app.inputManager.OnKeyEvent(key, scancode, action, mods)
}

func (app *App) mouseButtonCallback(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
	app.inputManager.OnMouseButtonEvent(button, action, mods)
}

func (app *App) sizeCallback(w *glfw.Window, width int, height int) {
	if !app.fullscreen {
		gl.Viewport(0, 0, int32(width), int32(height))
	}
}
