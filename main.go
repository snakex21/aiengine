// main.go - AIEngine entry point
// Initializes GLFW window, OpenGL context, renderer, scripting engine, and runs the main game loop.
// Fullscreen toggle via F11. Window size from config.aie (WINDOW_WIDTH/WINDOW_HEIGHT).
// ESC emits "onPause" event — the game (JS/Lua) decides what to do.

package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"

	"aiengine/actor"
	"aiengine/anim"
	"aiengine/game"
	"aiengine/input"
	"aiengine/level"
	"aiengine/parser"
	"aiengine/progress"
	"aiengine/renderer"
	"aiengine/scripting"
	"aiengine/systems"
	"aiengine/ui"
)

const (
	WindowTitle       = "AIEngine"
	DefaultProjectDir = "projects/demo"
)

var activeProjectDir = DefaultProjectDir
var activeManifest = parser.DefaultManifest()

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
	playerAnim DemoPlayerAnim

	// w is the simulation of the running level or sandbox (package game).
	w *game.World

	spriteBuf   []*renderer.Sprite // static sprites + projectiles/effects for this frame
	wheel       float64            // mouse wheel steps since the last frame
	pendingFire bool               // console "fire": shoot on the next frame
	wasReload   bool
	animSets    map[string]*anim.Set // enemy animation sets by texture prefix

	// Scripting.
	scriptEngine *scripting.Engine
	luaBridge    *scripting.LuaBridge
	jsBridge     *scripting.JSBridge

	// Audio.
	audioSys *systems.AudioSystem

	// Console / cheats.
	console   *systems.ConsoleState
	humanLoop *systems.HumanLoop // nil unless the agent bridge is enabled
	tasks     taskQueue          // work posted from other goroutines (human-loop)

	// Config.
	config *parser.Config

	// Mouse look.
	prevMouseX float64
	prevMouseY float64
	mouseInit  bool

	// Menus and player settings (manifest MENU); see menu_runtime.go.
	ms menuState
	// Campaign flow (manifest CAMPAIGN yes); see campaign_runtime.go.
	cs      *campaignState
	advPrev bool // advance key (Enter/Space) held last frame

	// --exec commands by frame ("@N cmd" runs at frame N, others at start).
	delayedExec map[int][]string

	// Level runtime (manifest RUNTIME level); nil for the legacy demo.
	game             *levelGame
	winMode          windowMode
	unfocused        bool
	focusPaused      bool // legacy pause caused by focus loss; a click resumes
	swallowClick     bool // ignore the first click after the window regains focus
	restartRequested bool
	enterWasDown     bool

	// Command-line run options and frame statistics.
	opts       runOptions
	frameCount int
	fpsTimer   float64
	fpsFrames  int
	fps        float64
	totalTime  float64
}

// runOptions are flags for a normal game launch.
type runOptions struct {
	project    string // --project
	screenshot string // --screenshot <file.png>: save a frame, then quit
	frames     int    // --frames N: frame to capture / quit after (0 = run)
	exec       string // --exec "cmd; cmd": console commands after start
	cpuprofile string // --cpuprofile <file>: Go CPU profile of the whole run
	agent      bool   // --agent: start the human-in-the-loop HTTP bridge
}

// parseRunOptions parses launch flags. Unknown flags are an error.
func parseRunOptions(args []string) (runOptions, error) {
	var o runOptions
	need := func(i int, flag string) (string, error) {
		if i+1 >= len(args) {
			return "", fmt.Errorf("%s needs a value", flag)
		}
		return args[i+1], nil
	}
	for i := 0; i < len(args); i++ {
		var err error
		switch args[i] {
		case "--project":
			o.project, err = need(i, args[i])
			i++
		case "--screenshot":
			o.screenshot, err = need(i, args[i])
			i++
		case "--exec":
			o.exec, err = need(i, args[i])
			i++
		case "--cpuprofile":
			o.cpuprofile, err = need(i, args[i])
			i++
		case "--agent":
			o.agent = true
		case "--frames":
			var v string
			if v, err = need(i, args[i]); err == nil {
				o.frames, err = strconv.Atoi(v)
			}
			i++
		default:
			err = fmt.Errorf("unknown flag: %s (use --help)", args[i])
		}
		if err != nil {
			return o, err
		}
	}
	if o.screenshot != "" && o.frames <= 0 {
		o.frames = 60
	}
	return o, nil
}

type DemoPlayerAnim struct {
	IdleFrames   []string
	ShootFrames  []string
	ReloadFrames []string
	MoveTimer    float64
	ShootTimer   float64
	ReloadTimer  float64
	Bob          float64
}

func main() {
	// --- CLI flags (no game loop) ---
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--build":
			runBuild()
			return
		case "--grid2sectors":
			// Convert a GRID map file into a sector map file (no window).
			if len(os.Args) != 4 {
				fmt.Fprintln(os.Stderr, "usage: aiengine --grid2sectors <grid.aie> <sectors.aie>")
				os.Exit(2)
			}
			if err := convertGridMap(os.Args[2], os.Args[3]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		case "--level":
			// Launch specific level — same as normal game, just sets map.
			// Fall through to normal game launch.
			runtime.LockOSThread()
			useExecutableDirectoryWhenNeeded(false)
			app := &App{}
			if err := app.init(); err != nil {
				log.Fatalf("AIEngine initialization failed: %v", err)
			}
			defer app.shutdown()
			if len(os.Args) > 2 {
				log.Printf("Level requested: %s (auto-load not yet implemented)", os.Args[2])
			}
			app.run()
			return
		case "--import":
			runImport()
			return
		case "--help", "-h":
			fmt.Println("AIEngine")
			fmt.Println("  aiengine.exe            run game + cheats")
			fmt.Println("  aiengine.exe --project <name|path>  run selected project")
			fmt.Println("  aiengine.exe --build    pack project to .aiepack")
			fmt.Println("  aiengine.exe --import <.aiepack>  unpack project")
			fmt.Println("  aiengine.exe --editor   launch editor (TBD)")
			fmt.Println("Launch options (combinable):")
			fmt.Println("  --screenshot <file.png> [--frames N]  save frame N (default 60) and quit")
			fmt.Println("  --frames N              quit after N frames (smoke test)")
			fmt.Println("  --exec \"cmd; cmd\"       console commands after start, e.g. \"god; tp 3 4\"")
			fmt.Println("  --cpuprofile <file>     write a Go CPU profile of the run (go tool pprof)")
			fmt.Println("  --agent                 start the agent bridge (HTTP on 127.0.0.1:9876)")
			fmt.Println("  --grid2sectors <in.aie> <out.aie>  convert a GRID map into a sector map")
			return
		}
	}

	opts, err := parseRunOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if opts.project != "" {
		activeProjectDir = normalizeProjectArg(opts.project)
	}

	runtime.LockOSThread()
	useExecutableDirectoryWhenNeeded(opts.project == "")

	app := &App{opts: opts}

	if err := app.init(); err != nil {
		log.Fatalf("AIEngine initialization failed: %v", err)
	}
	defer app.shutdown()
	if opts.cpuprofile != "" {
		if f, err := os.Create(opts.cpuprofile); err == nil {
			if err := pprof.StartCPUProfile(f); err == nil {
				defer pprof.StopCPUProfile()
			}
		}
	}

	app.run()
}

func runBuild() {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("getwd: %v", err)
	}
	name := filepath.Base(dir)
	output := name + ".aiepack"

	if len(os.Args) > 2 {
		output = os.Args[2]
	}

	fmt.Printf("Packing project '%s' → %s ...\n", name, output)
	if err := systems.PackProject(dir, name, output); err != nil {
		log.Fatalf("build failed: %v", err)
	}
	fmt.Println("Done.")
}

func runImport() {
	packPath := ""
	if len(os.Args) > 2 {
		packPath = os.Args[2]
	} else {
		// Try to find the first .aiepack in current directory.
		entries, _ := os.ReadDir(".")
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".aiepack" {
				packPath = e.Name()
				break
			}
		}
	}

	if packPath == "" {
		log.Fatal("usage: aiengine.exe --import <file.aiepack>")
	}

	dest := filepath.Base(packPath)
	dest = dest[:len(dest)-len(filepath.Ext(dest))] // strip .aiepack

	fmt.Printf("Importing %s → %s/ ...\n", packPath, dest)
	if err := systems.ImportProject(packPath, dest); err != nil {
		log.Fatalf("import failed: %v", err)
	}
	fmt.Println("Done.")
}

func useExecutableDirectoryWhenNeeded(allowProjectPicker bool) {
	if envProject := strings.TrimSpace(os.Getenv("AIENGINE_PROJECT")); envProject != "" {
		activeProjectDir = filepath.Clean(envProject)
	}
	useProjectBaseDirectoryWhenNeeded()
	if strings.TrimSpace(os.Getenv("AIENGINE_PROJECT")) == "" && allowProjectPicker && activeProjectDir == DefaultProjectDir {
		activeProjectDir = chooseProjectDir()
	}
	if _, err := os.Stat(projectPath("config.aie")); err == nil {
		return
	}
	useProjectBaseDirectoryWhenNeeded()
}

func useProjectBaseDirectoryWhenNeeded() {
	if _, err := os.Stat(projectPath("config.aie")); err == nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	candidates := []string{dir, filepath.Dir(dir)}
	for _, base := range candidates {
		if _, err := os.Stat(filepath.Join(base, activeProjectDir, "config.aie")); err == nil {
			if chdirErr := os.Chdir(base); chdirErr != nil {
				log.Printf("Warning: could not switch to project base directory %s: %v", base, chdirErr)
			}
			return
		}
	}
}

func normalizeProjectArg(arg string) string {
	arg = filepath.Clean(strings.TrimSpace(arg))
	if arg == "." || arg == "" {
		return DefaultProjectDir
	}
	if filepath.IsAbs(arg) || strings.Contains(arg, string(filepath.Separator)) || strings.Contains(arg, "/") {
		return arg
	}
	return filepath.Join("projects", arg)
}

func chooseProjectDir() string {
	projects := listProjects("projects")
	if len(projects) == 0 {
		return DefaultProjectDir
	}
	if len(projects) == 1 {
		return projects[0]
	}

	fmt.Println("Available AIEngine projects:")
	for i, p := range projects {
		fmt.Printf("  %d) %s\n", i+1, filepath.Base(p))
	}
	fmt.Printf("Select project [1-%d] (Enter = 1): ", len(projects))
	var choice string
	if _, err := fmt.Scanln(&choice); err == nil {
		var index int
		if _, err := fmt.Sscanf(choice, "%d", &index); err == nil && index >= 1 && index <= len(projects) {
			return projects[index-1]
		}
	}
	return projects[0]
}

func listProjects(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	projects := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(path, "config.aie")); err == nil {
			projects = append(projects, path)
		}
	}
	sort.Strings(projects)
	return projects
}

func projectPath(parts ...string) string {
	all := append([]string{activeProjectDir}, parts...)
	return filepath.Join(all...)
}

// init sets up GLFW, OpenGL, input, renderer, scripting, and test content.
func (app *App) init() error {
	manifest, err := parser.ParseManifestFile(projectPath("manifest.aie"))
	if err != nil {
		log.Printf("Warning: could not load manifest.aie: %v (using defaults)", err)
		manifest = parser.DefaultManifest()
	}
	activeManifest = manifest
	log.Printf("Project: %s (%s)", activeManifest.Project, activeProjectDir)

	// --- Load config FIRST ---
	cfg, err := parser.ParseConfigFile(projectPath(activeManifest.Config))
	if err != nil {
		log.Printf("Warning: could not load config.aie: %v (using defaults)", err)
		cfg = &parser.Config{}
	}
	app.config = cfg

	winW, winH := app.loadSettings(cfg.GetWindowWidth(), cfg.GetWindowHeight())
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
		app.inputManager.LoadConfig(projectPath(activeManifest.Config))
	} else {
		app.inputManager.SetDefaults()
	}

	// --- Texture manager ---
	app.texManager = renderer.NewTextureManager()
	if err := app.texManager.GenerateDefaultTextures(); err != nil {
		return fmt.Errorf("texture generation: %w", err)
	}
	if names, err := app.texManager.LoadWallOverrides(projectPath("assets", "textures")); err != nil {
		log.Printf("Warning: wall textures: %v", err)
	} else if len(names) > 0 {
		log.Printf("Project wall textures: %v", names)
	}
	app.loadDemoSpriteSheets()

	// --- Audio system (register sounds on main thread, init backend in goroutine) ---
	app.audioSys = systems.NewAudioSystem()
	app.audioSys.SoundPaths = []string{projectPath("sounds"), activeProjectDir, "."}
	app.audioSys.RegisterSoundDef(systems.SoundDef{
		Name: "gunshot", Waveform: systems.WaveImpact,
		Frequency: 800, Decay: 0.1, Noise: 0.3, Volume: 0.5,
	})
	app.audioSys.RegisterSoundDef(systems.SoundDef{
		Name: "player_hurt", Waveform: systems.WaveImpact,
		Frequency: 180, Decay: 0.18, Noise: 0.5, Volume: 0.6,
	})
	app.audioSys.RegisterSoundDef(systems.SoundDef{
		Name: "footstep", Waveform: systems.WaveClick,
		Frequency: 300, Decay: 0.05, Volume: 0.3,
	})
	go func() {
		if err := app.audioSys.InitBackend(); err != nil {
			log.Printf("Audio backend init failed (no sound): %v", err)
			return
		}
		log.Println("Audio system ready")
	}()

	// --- Console / Cheats ---
	app.console = systems.NewConsoleState()
	app.console.OnCommand = app.consoleCommand

	// --- Human‑in‑the‑loop (agent bridge): only with --agent, AIENGINE_AGENT=1
	// or AGENT_BRIDGE yes in config.aie ---
	if app.agentEnabled() {
		app.startHumanLoop()
	}

	// --- Map & Player ---
	var mapData *renderer.MapData
	var lv *level.Level
	if levelRuntimeEnabled() {
		app.ms.currentLevel = activeManifest.LevelList()[0]
		lv, err = loadLevelFile(app.ms.currentLevel.Path)
		if err != nil {
			return fmt.Errorf("level: %w", err)
		}
		mapData = lv.Map
		app.player = renderer.NewPlayerState(lv.SpawnX, lv.SpawnY, lv.SpawnAngle)
	} else {
		mapData = createTestMap()
		app.player = renderer.NewPlayerState(2.5, 2.5, 0.0)
	}

	// --- Raycaster ---
	app.raycaster = renderer.NewRaycaster(mapData, app.player, app.texManager)
	if err := app.raycaster.Init(int32(winW), int32(winH)); err != nil {
		return fmt.Errorf("raycaster init: %w", err)
	}

	if lv != nil {
		if err := app.startLevel(lv); err != nil {
			return err
		}
	} else {
		app.initDemoContent()
	}

	// --- GLFW callbacks ---
	window.SetKeyCallback(app.keyCallback)
	window.SetCharCallback(app.charCallback)
	window.SetMouseButtonCallback(app.mouseButtonCallback)
	window.SetScrollCallback(func(_ *glfw.Window, _, dy float64) {
		if !app.menuActive() {
			app.wheel += dy // weapon switching (game input)
		}
	})
	window.SetSizeCallback(app.sizeCallback)
	window.SetFocusCallback(app.focusCallback)
	window.SetCloseCallback(func(w *glfw.Window) {
		w.SetShouldClose(true)
	})
	window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)

	app.applySettings()
	app.initMenu()
	app.initCampaign()

	app.delayedExec = parseExec(app.opts.exec)
	app.runExec(0)

	app.lastFrameTime = time.Now()
	log.Println("AIEngine initialized successfully.")
	return nil
}

// agentEnabled reports whether the human-in-the-loop HTTP bridge should run.
func (app *App) agentEnabled() bool {
	if app.opts.agent || os.Getenv("AIENGINE_AGENT") == "1" {
		return true
	}
	return app.config != nil && strings.EqualFold(strings.TrimSpace(app.config.Raw["AGENT_BRIDGE"]), "yes")
}

// startHumanLoop starts the agent bridge. Questions arrive on the HTTP
// goroutine and are handed to the main loop through app.tasks.
func (app *App) startHumanLoop() {
	app.humanLoop = systems.NewHumanLoop()
	app.humanLoop.OnAsk = func(question string, options []string) {
		msg := fmt.Sprintf("[AGENT] %s", question)
		if len(options) > 0 {
			msg += fmt.Sprintf("  [%s]", strings.Join(options, "/"))
		}
		log.Printf("[HumanLoop] %s", msg)
		app.tasks.post(func() {
			app.console.History = append(app.console.History, msg)
			app.console.Open = true // show the question immediately
		})
	}
	if err := app.humanLoop.Start(9876); err != nil {
		log.Printf("Warning: HumanLoop on :9876 failed: %v — trying :9877", err)
		if err2 := app.humanLoop.Start(9877); err2 != nil {
			log.Printf("Warning: HumanLoop failed to start: %v (agent bridge unavailable)", err2)
			app.humanLoop = nil
		}
	}
}

// initDemoContent sets up the legacy hard-coded demo sandbox (projects
// without RUNTIME level).
func (app *App) initDemoContent() {
	// --- Sprites & Effects ---
	app.sprites = []*renderer.Sprite{
		{X: 4.5, Y: 3.5, Z: 0, Scale: 0.7, TexName: "sprite_prop", Alpha: 1.0, Active: true, EntityID: "prop1"},
		{X: 5.5, Y: 3.5, Z: 0, Scale: 0.7, TexName: "sprite_prop", Alpha: 1.0, Active: true, EntityID: "prop2"},
	}
	app.effects = renderer.DefaultEffects()

	// --- Scripting Engine ---
	app.scriptEngine = scripting.NewEngine()
	app.scriptEngine.Effects = app.effects
	app.scriptEngine.AudioPlayer = app.audioSys // wire audio to scripts
	app.scriptEngine.Weapons = app.loadWeaponDefs(nil)
	app.startLoadout(nil)
	app.w = game.New(nil, app.raycaster.Map, app.player, app.scriptEngine, app)
	app.w.God, app.w.Noclip = app.cheat(systems.CheatGod), app.cheat(systems.CheatNoclip)
	for _, sp := range []struct {
		id   string
		x, y float64
	}{{"enemy1", 1.5, 3.5}, {"enemy2", 7.5, 2.5}, {"enemy3", 7.5, 7.5}} {
		app.addSandboxEnemy(sp.id, "grunt", sp.x, sp.y)
	}

	// Lua bridge.
	app.luaBridge = scripting.NewLuaBridge(app.scriptEngine)
	// Try loading demo Lua scripts.
	if err := app.luaBridge.DoFile(projectPath(activeManifest.MainScript)); err != nil {
		log.Printf("Lua level1.lua: %v", err)
	}
	// Boss scripts are intentionally not loaded in this demo variant: one living enemy type only.

	// JS bridge.
	app.jsBridge = scripting.NewJSBridge(app.scriptEngine)
	if _, err := app.jsBridge.RunFile(projectPath(activeManifest.HUDScript)); err != nil {
		log.Printf("JS hud.js: %v", err)
	}

	// Emit onStart event.
	app.scriptEngine.Emit("onStart")
}

// cheat returns a reader of a console cheat (for game.World).
func (app *App) cheat(name systems.CheatFlag) func() bool {
	return func() bool { return app.console != nil && app.console.CheatEnabled(name) }
}

// addSandboxEnemy adds a classic chasing enemy to the sandbox.
func (app *App) addSandboxEnemy(id, typ string, x, y float64) {
	sprite := &renderer.Sprite{Z: 0, Scale: 0.85, TexName: app.FrameName("", 0), Alpha: 1.0, Active: true}
	app.w.AddSandboxEnemy(id, typ, x, y, sprite)
	app.sprites = append(app.sprites, sprite)
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

func (app *App) loadDemoSpriteSheets() {
	app.playerAnim.IdleFrames = []string{"player_idle_0", "player_idle_1", "player_idle_2", "player_idle_3", "player_idle_4"}
	app.playerAnim.ShootFrames = []string{"player_shoot_0", "player_shoot_1", "player_shoot_2", "player_shoot_3"}
	app.playerAnim.ReloadFrames = []string{"player_reload_0", "player_reload_1", "player_reload_2", "player_reload_3", "player_reload_4", "player_reload_5", "player_reload_6"}
	if !app.loadPlayerAnimationFiles(projectPath(activeManifest.PlayerAssets)) {
		if err := app.texManager.LoadWeaponHandAtlas(projectPath(activeManifest.PlayerAtlas)); err != nil {
			log.Printf("Warning: could not load gracz.png spritesheet: %v", err)
			app.playerAnim.IdleFrames = []string{"sprite_default"}
			app.playerAnim.ShootFrames = []string{"sprite_default"}
			app.playerAnim.ReloadFrames = []string{"sprite_default"}
		}
	}
	if err := app.texManager.LoadSpriteSheet("enemy", projectPath(activeManifest.EnemyAtlas), 3, 2); err != nil {
		log.Printf("Warning: could not load przeciwnik.png spritesheet: %v", err)
	}
}

func (app *App) loadPlayerAnimationFiles(dir string) bool {
	frames := map[string][]string{
		"player_idle":   app.playerAnim.IdleFrames,
		"player_shoot":  app.playerAnim.ShootFrames,
		"player_reload": app.playerAnim.ReloadFrames,
	}
	filePrefixes := map[string]string{
		"player_idle":   "idle",
		"player_shoot":  "shoot",
		"player_reload": "reload",
	}

	for texPrefix, frameNames := range frames {
		filePrefix := filePrefixes[texPrefix]
		for i, texName := range frameNames {
			path := filepath.Join(dir, fmt.Sprintf("%s_%d.png", filePrefix, i))
			if _, err := os.Stat(path); err != nil {
				return false
			}
			tex, err := app.texManager.LoadFromFile(texName, path)
			if err != nil {
				log.Printf("Warning: could not load player animation frame %s: %v", path, err)
				return false
			}
			if a := renderer.EdgeAlpha(tex); a > 8 {
				log.Printf("Warning: %s: visible pixels (alpha %d) touch the frame edge - the art (e.g. a glow) will look cut off; add transparent margin", path, a)
			}
		}
	}

	log.Printf("Loaded player animation frames from %s", dir)
	return true
}

// FrameName names frame index of a classic 3x2 enemy atlas (game.Host).
func (app *App) FrameName(prefix string, index int) string { return app.enemyFrameFor(prefix, index) }

func (app *App) enemyFrameFor(prefix string, index int) string {
	if prefix == "" {
		prefix = "enemy"
	}
	name := fmt.Sprintf("%s_%d", prefix, index)
	if app.texManager != nil && app.texManager.Get(name) != nil {
		return name
	}
	return "sprite_enemy"
}

// run executes the main game loop.
func (app *App) run() {
	for !app.window.ShouldClose() {
		// Minimised: nothing to draw; sleep until restored (the framebuffer
		// is 0x0, and a huge dt on restore is avoided).
		if app.window.GetAttrib(glfw.Iconified) == glfw.True {
			glfw.WaitEventsTimeout(0.1)
			app.lastFrameTime = time.Now()
			app.mouseInit = false
			continue
		}
		app.calculateDeltaTime()
		app.trackFPS(app.deltaTime)

		glfw.PollEvents()
		app.tasks.run()
		if app.restartRequested {
			app.restartLevel()
		}
		app.handleInput()
		app.menuFrame(app.deltaTime)
		app.campaignFrame(app.deltaTime)

		if !app.paused {
			app.update(app.deltaTime)
		}
		app.render()

		app.frameCount++
		app.runExec(app.frameCount)
		if app.opts.frames > 0 && app.frameCount >= app.opts.frames {
			if app.opts.screenshot != "" {
				if err := app.saveScreenshot(app.opts.screenshot); err != nil {
					log.Printf("Screenshot failed: %v", err)
				}
			}
			app.window.SetShouldClose(true)
		}

		// CRITICAL: Update input state at END of frame, so IsJustPressed
		// sees the difference between previous (last frame) and held (just set by PollEvents).
		app.inputManager.Update()

		app.window.SwapBuffers()
	}
}

// trackFPS keeps a once-per-second FPS estimate and total run time.
func (app *App) trackFPS(dt float64) {
	app.totalTime += dt
	app.fpsTimer += dt
	app.fpsFrames++
	if app.fpsTimer >= 1 {
		app.fps = float64(app.fpsFrames) / app.fpsTimer
		app.fpsTimer, app.fpsFrames = 0, 0
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
	// Fullscreen works always.
	if app.inputManager.IsJustPressed("fullscreen") {
		app.toggleFullscreen()
	}

	// Menus take all other input (see keyCallback / menuFrame).
	if app.menuActive() {
		return
	}

	// Pause: pause menu when the project has one, else the legacy toggle.
	if app.inputManager.IsJustPressed("pause") {
		app.pauseRequested()
		if app.menuActive() {
			return
		}
	}

	// When paused, ignore all gameplay input.
	if app.paused {
		return
	}

	// Campaign screens (intro/outro/loading): Enter/Space/click advance.
	if app.campaignBlocking() {
		if app.advancePressed() && app.cs.phase != phaseLoading {
			app.campaignAdvance()
		}
		return
	}

	// Level end screen: only restart is accepted.
	if app.levelEnded() {
		if app.restartPressed() {
			app.restartLevel()
		}
		return
	}

}

func (app *App) toggleFullscreen() {
	if app.ms.settingsActive {
		app.ms.settings.Fullscreen = !app.fullscreen
		app.applyVideo()
		app.saveSettings()
		return
	}
	// No settings (legacy projects): toggle borderless fullscreen.
	if app.fullscreen {
		app.setWindowMode(windowed, app.windowedW, app.windowedH)
	} else {
		app.windowedW, app.windowedH = app.window.GetSize()
		app.setWindowMode(borderlessFullscreen, 0, 0)
	}
	log.Printf("Fullscreen: %v", app.fullscreen)
}

// update runs one simulated frame: script time, the world (player,
// enemies, combat, movers - package game) and the level view.
func (app *App) update(dt float64) {
	// Pause gameplay while the console or a menu is open (this also freezes
	// game time, so script timers stop).
	if app.console != nil && app.console.Open {
		return
	}
	if app.menuActive() || app.campaignBlocking() {
		return
	}
	if app.levelEnded() {
		return
	}
	// Game time: script timers (Lua startTimer, JS setTimeout) and getTime
	// only advance here, so pauses, menus and the console freeze them.
	app.scriptEngine.Advance(dt)
	if app.jsBridge != nil {
		app.jsBridge.Tick(dt)
	}
	if app.game == nil {
		app.scriptEngine.TickMessage(dt)
	}

	in := app.gameInput()
	app.w.Update(dt, in)
	app.handleWorldEvents()
	if wp := app.scriptEngine.Loadout.CurrentWeapon(); wp != nil {
		if wp.Reloading && !app.wasReload {
			app.playerAnim.ReloadTimer = 0.75
		}
		app.wasReload = wp.Reloading
	}
	app.updatePlayerAnimation(dt, app.w.Moving)
	if app.game != nil {
		app.updateLevel(dt)
	} else {
		app.raycaster.Lights = app.w.Lights(app.raycaster.Lights[:0])
	}
	app.w.ClearEvents()
}

// gameInput reads the bound actions and the mouse into a game.Input.
func (app *App) gameInput() game.Input {
	im := app.inputManager
	in := game.Input{
		Forward: im.IsPressed("move_forward"), Back: im.IsPressed("move_backward"),
		Left: im.IsPressed("move_left"), Right: im.IsPressed("move_right"),
		Jump: im.IsJustPressed("jump"), Crouch: im.IsPressed("crouch"), ProneToggle: im.IsJustPressed("prone"),
		Fire: im.IsJustPressed("shoot"), FireHeld: im.IsPressed("shoot"),
		Reload: im.IsJustPressed("reload"), Interact: im.IsJustPressed("interact"),
		AimSlope: app.raycaster.AimSlope(),
	}
	if app.pendingFire {
		in.Fire, app.pendingFire = true, false
	}
	for k := 1; k <= 9; k++ {
		if im.IsJustPressed("weapon_" + strconv.Itoa(k)) {
			in.Slot = k
		}
	}
	switch {
	case im.IsJustPressed("next_weapon") || app.wheel <= -1:
		in.Cycle = 1
	case im.IsJustPressed("prev_weapon") || app.wheel >= 1:
		in.Cycle = -1
	}
	app.wheel = 0
	cx, cy := app.window.GetCursorPos()
	if !app.mouseInit {
		app.prevMouseX, app.prevMouseY = cx, cy
		app.mouseInit = true
	}
	dx, dy := cx-app.prevMouseX, cy-app.prevMouseY
	app.prevMouseX, app.prevMouseY = cx, cy
	sens, invert := 1.0, false
	if app.ms.settingsActive {
		sens, invert = app.ms.settings.MouseSensitivity, app.ms.settings.InvertY
	}
	in.Yaw = dx * 0.003 * sens
	// Moving the mouse up (negative dy) looks up: the world shifts downward
	// on screen and more ceiling is revealed (positive pitch = up).
	in.Pitch = -dy * 0.0015 * sens
	if invert {
		in.Pitch = -in.Pitch
	}
	return in
}

func (app *App) updatePlayerAnimation(dt float64, moving bool) {
	if len(app.playerAnim.IdleFrames) == 0 {
		return
	}
	if app.playerAnim.ReloadTimer > 0 {
		app.playerAnim.ReloadTimer -= dt
		frames := app.playerAnim.ReloadFrames
		idx := int((0.75 - app.playerAnim.ReloadTimer) / 0.10)
		if idx < 0 {
			idx = 0
		}
		if idx >= len(frames) {
			idx = len(frames) - 1
		}
		app.raycaster.HUDWeaponTex = frames[idx]
		app.raycaster.HUDWeaponBob = 0
		return
	}
	if view := app.weaponView(app.playerAnim.ShootTimer > 0); view != "" {
		// A weapon with its own first-person sprite (VIEW).
		app.playerAnim.ShootTimer = math.Max(0, app.playerAnim.ShootTimer-dt)
		app.playerAnim.ReloadTimer = 0
		if moving {
			app.playerAnim.Bob += dt * 12
		} else {
			app.playerAnim.Bob = 0
		}
		app.raycaster.HUDWeaponTex, app.raycaster.HUDWeaponBob = view, app.playerAnim.Bob
		return
	}
	if app.playerAnim.ShootTimer > 0 {
		app.playerAnim.ShootTimer -= dt
		frames := app.playerAnim.ShootFrames
		idx := int((0.18 - app.playerAnim.ShootTimer) / 0.045)
		if idx < 0 {
			idx = 0
		}
		if idx >= len(frames) {
			idx = len(frames) - 1
		}
		app.raycaster.HUDWeaponTex = frames[idx]
		app.raycaster.HUDWeaponBob = app.playerAnim.Bob
		return
	}
	if moving {
		app.playerAnim.MoveTimer += dt
		app.playerAnim.Bob += dt * 12
		idx := int(app.playerAnim.MoveTimer/0.12) % len(app.playerAnim.IdleFrames)
		app.raycaster.HUDWeaponTex = app.playerAnim.IdleFrames[idx]
	} else {
		app.playerAnim.Bob = 0
		app.raycaster.HUDWeaponTex = app.playerAnim.IdleFrames[0]
	}
	app.raycaster.HUDWeaponBob = app.playerAnim.Bob
}

// render draws a frame.
func (app *App) render() {
	// Stretch the internal image over the real framebuffer (fullscreen and
	// window resizes previously left it in the bottom-left corner).
	fbW, fbH := app.window.GetFramebufferSize()
	app.raycaster.ViewportW, app.raycaster.ViewportH = int32(fbW), int32(fbH)

	// Sync HUD values from scripting engine.
	app.raycaster.HUDHP = int(app.scriptEngine.GetPlayerHP())
	app.raycaster.HUDMaxHP = int(app.scriptEngine.PlayerMaxHP)
	app.updateWeaponHUD()
	if app.levelEnded() {
		app.showEndScreen() // also covers endings triggered outside update (console, events)
	}
	app.raycaster.HUDMessage = app.scriptEngine.CurrentMessage()
	lines := app.scriptEngine.HUDLines()
	if app.console != nil && app.console.CheatEnabled(systems.CheatFPS) {
		lines = append(lines, fmt.Sprintf("FPS %.0f", app.fps))
	}
	app.raycaster.HUDLines = lines

	// Sync console state.
	if app.console != nil {
		app.raycaster.ConsoleOpen = app.console.Open
		app.raycaster.ConsoleInput = app.console.Input
		app.raycaster.ConsoleHistory = app.console.History
	}

	// Menus draw into the UI layer and hide the gameplay HUD.
	if app.menuActive() {
		app.raycaster.UIDraw = app.ms.menu.Draw
		app.raycaster.HideHUD = true
	} else {
		app.raycaster.UIDraw = nil
		app.raycaster.HideHUD = false
	}

	// Hit feedback: red flash on top of script effects and a short shake.
	fx := *app.effects
	w := app.w
	if w.HurtFlash > 0 {
		fx.RedTint = math.Max(fx.RedTint, w.HurtFlash)
		w.HurtFlash = math.Max(0, w.HurtFlash-app.deltaTime*2.5)
	}
	angle, pitch := app.player.Angle, app.player.Pitch
	if w.Shake > 0 && !app.menuActive() {
		k := w.Shake * 0.06
		app.player.Angle += k * math.Sin(app.totalTime*90)
		app.player.Pitch += k * math.Cos(app.totalTime*77)
		w.Shake = math.Max(0, w.Shake-app.deltaTime)
	}
	// Weapon flash: a short light at the camera after each shot.
	app.raycaster.Flash = 0.6 * w.Flash
	w.Flash = math.Max(0, w.Flash-app.deltaTime*9)
	app.raycaster.ApplyEffects(&fx)
	app.raycaster.Render(app.renderSprites())
	app.player.Angle, app.player.Pitch = angle, pitch
}

// shutdown cleans up resources.
func (app *App) shutdown() {
	if app.frameCount > 0 && app.totalTime > 0 {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		log.Printf("[Perf] %d frames in %.1fs: avg %.1f FPS (%.2f ms/frame), heap %.1f MB, sys %.1f MB",
			app.frameCount, app.totalTime, float64(app.frameCount)/app.totalTime,
			1000*app.totalTime/float64(app.frameCount), float64(ms.HeapAlloc)/1e6, float64(ms.Sys)/1e6)
	}
	if app.audioSys != nil {
		app.audioSys.Shutdown()
	}
	if app.humanLoop != nil {
		app.humanLoop.Stop()
	}
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
	// Alt+Enter toggles fullscreen everywhere; F11 also inside menus (in
	// gameplay F11 comes through the "fullscreen" binding).
	if action == glfw.Press && ((key == glfw.KeyEnter || key == glfw.KeyKPEnter) && mods&glfw.ModAlt != 0 ||
		key == glfw.KeyF11 && app.menuActive()) {
		app.toggleFullscreen()
		return
	}
	// Console takes all input when open.
	if app.console != nil && app.console.Open {
		if action == glfw.Press || action == glfw.Repeat {
			app.consoleKeyInput(key)
		}
		return
	}

	// Tilde toggles console.
	if key == glfw.KeyGraveAccent && action == glfw.Press {
		app.console.Toggle()
		return
	}

	// Menus take keyboard input; releases still reach the game so keys held
	// when a menu opened do not stay "pressed".
	if app.menuActive() {
		if action == glfw.Press || action == glfw.Repeat {
			if in, ok := menuKey(key); ok {
				app.ms.inputs = append(app.ms.inputs, in)
			}
		} else if action == glfw.Release {
			app.inputManager.OnKeyEvent(key, scancode, action, mods)
		}
		return
	}

	// F1 GUI panel (placeholder).
	if key == glfw.KeyF1 && action == glfw.Press {
		log.Println("[Cheats] F1 GUI panel (not yet implemented)")
		return
	}

	app.inputManager.OnKeyEvent(key, scancode, action, mods)
}

func (app *App) mouseButtonCallback(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
	// After a focus-loss pause (no pause menu): a click resumes and
	// re-captures the cursor; the click itself is not passed to the game.
	if action == glfw.Press && !app.menuActive() {
		app.swallowClick = false
	}
	if app.campaignBlocking() && !app.menuActive() && action == glfw.Press {
		if app.cs.phase != phaseLoading {
			app.campaignAdvance()
		}
		return
	}
	if app.focusPaused && action == glfw.Press {
		app.focusPaused, app.paused, app.mouseInit = false, false, false
		w.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)
		app.scriptEngine.Emit("onPause", false)
		return
	}
	if app.menuActive() && action == glfw.Press {
		if app.swallowClick {
			app.swallowClick = false
			return
		}
		x, y := w.GetCursorPos()
		app.ms.clicks = append(app.ms.clicks, menuClick{x: x, y: y, right: button == glfw.MouseButtonRight})
		return
	}
	app.inputManager.OnMouseButtonEvent(button, action, mods)
}

func (app *App) sizeCallback(w *glfw.Window, width int, height int) {
	if !app.fullscreen {
		gl.Viewport(0, 0, int32(width), int32(height))
	}
}

// --- Console input handling ---

func (app *App) consoleKeyInput(key glfw.Key) {
	switch key {
	case glfw.KeyEscape, glfw.KeyGraveAccent:
		app.console.Toggle()
	case glfw.KeyEnter, glfw.KeyKPEnter:
		input := strings.TrimSpace(app.console.Input)
		// Route to human‑loop if there's a pending question.
		if app.humanLoop != nil && app.humanLoop.AnswerFromConsole(input) {
			app.console.History = append(app.console.History, "> "+input)
			app.console.History = append(app.console.History, "[OK]")
			app.console.Input = ""
			app.console.Cursor = -1
			return
		}
		app.console.Submit()
	case glfw.KeyBackspace:
		app.console.Backspace()
	case glfw.KeyUp:
		app.console.HistoryUp()
	case glfw.KeyDown:
		app.console.HistoryDown()
	case glfw.KeyV:
		// Ctrl+V paste (simplified — GLFW doesn't give clipboard easily here)
	default:
		// Printable characters are handled via char callback.
	}
}

func (app *App) charCallback(w *glfw.Window, char rune) {
	if app.console != nil && app.console.Open {
		if char >= 32 && char < 127 {
			app.console.HandleChar(char)
		}
	}
}

// consoleCommand handles cheat commands that need game state.
func (app *App) consoleCommand(cmd string, args []string) string {
	switch cmd {
	case "speed":
		v := systems.ArgFloat(args, 0, 2.0)
		app.player.MoveSpeed = v
		return fmt.Sprintf("speed set to %.1f", v)

	case "spawn":
		entType := systems.ArgString(args, 0)
		x := systems.ArgFloat(args, 1, app.player.X+1)
		y := systems.ArgFloat(args, 2, app.player.Y)
		if entType == "" {
			return "spawn <entity> [x] [y]"
		}
		if app.game != nil {
			// Level runtime: the level's template for the type (like spawnEnemy).
			app.scriptEngine.SpawnEnemy(entType, x, y, 0)
		} else {
			app.addSandboxEnemy(app.scriptEngine.Actors.NewID("spawn"), entType, x, y)
		}
		return fmt.Sprintf("spawned %s at (%.1f, %.1f)", entType, x, y)

	case "give":
		item := systems.ArgString(args, 0)
		switch {
		case item == "":
			return "give <weapon> | give ammo <type> [n] | give <item> [n]"
		case item == "ammo":
			typ := systems.ArgString(args, 1)
			n := app.scriptEngine.GiveAmmo(typ, int(systems.ArgFloat(args, 2, 20)))
			return fmt.Sprintf("gave %d %s", n, typ)
		case app.scriptEngine.Weapons[item] != nil:
			app.scriptEngine.GiveWeapon(item)
			app.scriptEngine.SelectWeapon(item)
		default:
			app.scriptEngine.GiveItem(item, int(systems.ArgFloat(args, 1, 1)))
		}
		return fmt.Sprintf("gave %s", item)

	case "weapon":
		name := systems.ArgString(args, 0)
		if !app.scriptEngine.SelectWeapon(name) && !app.scriptEngine.HasWeapon(name) {
			return "weapon <name> (carried: " + app.carriedWeapons() + ")"
		}
		return "weapon " + name

	case "fire":
		app.pendingFire = true
		return "fire"

	case "kill":
		target := systems.ArgString(args, 0)
		if target != "" {
			// Kill specific enemy.
			if a := app.w.Actors.Get(target); a != nil && a.Alive {
				app.w.ApplyDamage(target, game.Damage{Amount: a.HP, Kind: game.KindScript})
				return fmt.Sprintf("killed %s", target)
			}
			return fmt.Sprintf("target not found: %s", target)
		}
		// Kill all.
		count := 0
		for _, a := range app.w.Actors.All() {
			if a.Alive && a.Faction == actor.Hostile {
				app.w.ApplyDamage(a.ID, game.Damage{Amount: a.HP, Kind: game.KindScript})
				count++
			}
		}
		return fmt.Sprintf("killed %d enemies", count)

	case "tp":
		x := systems.ArgFloat(args, 0, app.player.X)
		y := systems.ArgFloat(args, 1, app.player.Y)
		app.w.PlacePlayer(x, y)
		if len(args) > 2 { // optional facing in degrees (0 = +x, 90 = +y)
			app.player.Angle = systems.ArgFloat(args, 2, 0) * math.Pi / 180
		}
		return fmt.Sprintf("teleported to (%.1f, %.1f)", x, y)

	case "reload":
		lo := app.scriptEngine.Loadout
		if wp := lo.CurrentWeapon(); wp != nil && wp.Reload(lo.Ammo) {
			return "reloading " + wp.Def.Name
		}
		return "nothing to reload"

	case "map":
		name := systems.ArgString(args, 0)
		if name == "" {
			return "map <file.aie>"
		}
		if app.game == nil {
			return "map needs RUNTIME level"
		}
		// Load any map file of the project (test maps outside the campaign).
		lv, err := loadLevelFile(name)
		if err != nil {
			return err.Error()
		}
		app.closeMenu()
		app.ms.currentLevel = parser.LevelEntry{ID: lv.Name, Path: name, Title: lv.Name}
		if err := app.startLevel(lv); err != nil {
			return err.Error()
		}
		return "map " + name

	case "lua":
		if app.luaBridge == nil {
			return "lua not initialized"
		}
		if err := app.luaBridge.DoString(strings.Join(args, " ")); err != nil {
			return err.Error()
		}
		return "ok"

	case "menu":
		name := systems.ArgString(args, 0)
		if app.ms.menu == nil {
			return "no menu file (manifest MENU)"
		}
		if name == "" || name == "close" {
			app.closeMenu()
			return "menu closed"
		}
		if !app.ms.menu.Has(name) {
			return "unknown screen: " + name
		}
		app.openMenu(name)
		return "menu " + name

	case "menukey":
		// Queue menu inputs (for scripted tests): menukey down down right enter esc
		keys := map[string]ui.Input{"up": ui.Up, "down": ui.Down, "left": ui.Left, "right": ui.Right, "enter": ui.Accept, "esc": ui.Back}
		for _, k := range args {
			in, ok := keys[strings.ToLower(k)]
			if !ok {
				return "menukey up|down|left|right|enter|esc ..."
			}
			app.ms.inputs = append(app.ms.inputs, in)
		}
		return fmt.Sprintf("queued %d menu input(s)", len(args))

	case "level":
		// level <id> [play]: start a campaign level (intro screen unless "play").
		if app.cs == nil {
			return "level needs CAMPAIGN yes"
		}
		id := systems.ArgString(args, 0)
		if _, ok := activeManifest.FindLevel(id); !ok {
			return "unknown level: " + id
		}
		app.closeMenu()
		app.cs.p.Start(id)
		app.campaignStart(id, progress.State{})
		if systems.ArgString(args, 1) == "play" {
			app.campaignAdvance()
		}
		return "level " + id

	case "advance":
		// Press "continue" on a campaign intro/outro screen (for tests).
		if app.campaignBlocking() {
			app.campaignAdvance()
			return "advanced"
		}
		return "no campaign screen"

	case "focus":
		app.onFocusChange(systems.ArgString(args, 0) != "off")
		return "focus " + systems.ArgString(args, 0)

	case "fullscreen":
		app.toggleFullscreen()
		return fmt.Sprintf("fullscreen %v", app.fullscreen)

	case "restart":
		if app.game == nil {
			return "restart needs RUNTIME level"
		}
		app.restartRequested = true
		return "restarting level"

	case "hltest":
		if app.humanLoop == nil {
			return "human-loop not running (start with --agent)"
		}
		go func() {
			pq := app.humanLoop.Ask("HL test — czy dziala? (tak/nie)", []string{"tak", "nie"})
			answer := <-pq.Answer
			app.tasks.post(func() {
				app.console.History = append(app.console.History, fmt.Sprintf("[HL answer] %s", answer))
			})
		}()
		return "human-loop test sent — type 'tak' or 'nie'"

	default:
		return fmt.Sprintf("unknown: %s", cmd)
	}
}

// carriedWeapons lists the player's weapons.
func (app *App) carriedWeapons() string {
	var names []string
	for _, w := range app.scriptEngine.Loadout.Weapons {
		names = append(names, w.Def.Name)
	}
	return strings.Join(names, " ")
}

// advancePressed reports a fresh press of Enter, keypad Enter or Space.
func (app *App) advancePressed() bool {
	down := app.window.GetKey(glfw.KeyEnter) == glfw.Press ||
		app.window.GetKey(glfw.KeyKPEnter) == glfw.Press ||
		app.window.GetKey(glfw.KeySpace) == glfw.Press
	pressed := down && !app.advPrev
	app.advPrev = down
	return pressed
}

// parseExec splits --exec into commands per frame: "@30 advance" runs at
// frame 30, plain commands right after start (frame 0).
func parseExec(spec string) map[int][]string {
	out := map[int][]string{}
	for _, cmd := range strings.Split(spec, ";") {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		frame := 0
		if strings.HasPrefix(cmd, "@") {
			head, rest, _ := strings.Cut(cmd[1:], " ")
			if n, err := strconv.Atoi(head); err == nil {
				frame, cmd = n, strings.TrimSpace(rest)
			}
		}
		out[frame] = append(out[frame], cmd)
	}
	return out
}

func (app *App) runExec(frame int) {
	for _, cmd := range app.delayedExec[frame] {
		log.Printf("[exec@%d] %s -> %s", frame, cmd, systems.ExecuteCommand(app.console, cmd))
	}
	delete(app.delayedExec, frame)
}
