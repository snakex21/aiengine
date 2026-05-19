// scripting/js.go — JavaScript scripting bridge for AIEngine.
// Embeds goja (ECMAScript 5.1+) VM and registers all engine API functions.
// JS primarily controls UI/HUD elements, but can optionally handle game logic.
// See DOCS.md "JS API" section for the full function reference.

package scripting

import (
	"fmt"
	"log"
	"os"

	"github.com/dop251/goja"
)

// JSBridge wraps a goja VM with the AIEngine API.
type JSBridge struct {
	VM     *goja.Runtime
	engine *Engine
	ui     *UIBridge // separate UI object for UI-specific functions
}

// UIBridge exposes UI-only functions as methods on a "UI" global object.
type UIBridge struct {
	engine *Engine
}

// NewJSBridge creates a new JS VM and registers all API functions.
func NewJSBridge(engine *Engine) *JSBridge {
	vm := goja.New()
	ui := &UIBridge{engine: engine}
	jb := &JSBridge{VM: vm, engine: engine, ui: ui}
	jb.registerAPI()
	return jb
}

// registerAPI exposes all engine functions as JS globals.
func (jb *JSBridge) registerAPI() {
	vm := jb.VM
	e := jb.engine

	// --- Engine global object (for both UI and logic access) ---
	engineObj := vm.NewObject()

	// Enemy API.
	engineObj.Set("spawnEnemy", func(call goja.FunctionCall) goja.Value {
		typ := call.Argument(0).String()
		x := call.Argument(1).ToFloat()
		y := call.Argument(2).ToFloat()
		z := 0.0
		if len(call.Arguments) >= 4 {
			z = call.Argument(3).ToFloat()
		}
		id := e.SpawnEnemy(typ, x, y, z)
		return vm.ToValue(id)
	})

	engineObj.Set("killEnemy", func(call goja.FunctionCall) goja.Value {
		e.KillEnemy(call.Argument(0).String())
		return goja.Undefined()
	})

	engineObj.Set("getEnemyHP", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(e.GetEnemyHP(call.Argument(0).String()))
	})

	engineObj.Set("setEnemyHP", func(call goja.FunctionCall) goja.Value {
		e.SetEnemyHP(call.Argument(0).String(), call.Argument(1).ToFloat())
		return goja.Undefined()
	})

	engineObj.Set("allEnemiesDead", func(call goja.FunctionCall) goja.Value {
		zone := ""
		if len(call.Arguments) >= 1 {
			zone = call.Argument(0).String()
		}
		return vm.ToValue(e.AllEnemiesDeadInZone(zone))
	})

	engineObj.Set("getEnemyCount", func(call goja.FunctionCall) goja.Value {
		zone := ""
		if len(call.Arguments) >= 1 {
			zone = call.Argument(0).String()
		}
		return vm.ToValue(e.GetEnemyCountInZone(zone))
	})

	// Player API.
	engineObj.Set("getPlayerHP", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(e.GetPlayerHP())
	})

	engineObj.Set("setPlayerHP", func(call goja.FunctionCall) goja.Value {
		e.SetPlayerHP(call.Argument(0).ToFloat())
		return goja.Undefined()
	})

	engineObj.Set("getPlayerPosition", func(call goja.FunctionCall) goja.Value {
		x, y, z := e.GetPlayerPosition()
		obj := vm.NewObject()
		obj.Set("x", x)
		obj.Set("y", y)
		obj.Set("z", z)
		return obj
	})

	engineObj.Set("setPlayerPosition", func(call goja.FunctionCall) goja.Value {
		e.SetPlayerPosition(
			call.Argument(0).ToFloat(),
			call.Argument(1).ToFloat(),
			call.Argument(2).ToFloat(),
		)
		return goja.Undefined()
	})

	engineObj.Set("getPlayerAmmo", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(e.GetPlayerAmmo(call.Argument(0).String()))
	})

	engineObj.Set("giveWeapon", func(call goja.FunctionCall) goja.Value {
		e.GiveWeapon(call.Argument(0).String())
		return goja.Undefined()
	})

	engineObj.Set("removeWeapon", func(call goja.FunctionCall) goja.Value {
		e.RemoveWeapon(call.Argument(0).String())
		return goja.Undefined()
	})

	engineObj.Set("freezePlayer", func(call goja.FunctionCall) goja.Value {
		e.FreezePlayer()
		return goja.Undefined()
	})

	engineObj.Set("unfreezePlayer", func(call goja.FunctionCall) goja.Value {
		e.UnfreezePlayer()
		return goja.Undefined()
	})

	// Map API.
	engineObj.Set("unlockDoor", func(call goja.FunctionCall) goja.Value {
		e.UnlockDoor(call.Argument(0).String())
		return goja.Undefined()
	})

	engineObj.Set("lockDoor", func(call goja.FunctionCall) goja.Value {
		e.LockDoor(call.Argument(0).String())
		return goja.Undefined()
	})

	// Sound API.
	engineObj.Set("playSound", func(call goja.FunctionCall) goja.Value {
		e.PlaySound(call.Argument(0).String())
		return goja.Undefined()
	})

	engineObj.Set("stopSound", func(call goja.FunctionCall) goja.Value {
		e.StopSound(call.Argument(0).String())
		return goja.Undefined()
	})

	engineObj.Set("playMusic", func(call goja.FunctionCall) goja.Value {
		e.PlayMusic(call.Argument(0).String())
		return goja.Undefined()
	})

	engineObj.Set("stopMusic", func(call goja.FunctionCall) goja.Value {
		e.StopMusic()
		return goja.Undefined()
	})

	engineObj.Set("changeMusic", func(call goja.FunctionCall) goja.Value {
		e.ChangeMusic(call.Argument(0).String())
		return goja.Undefined()
	})

	// Variable API.
	engineObj.Set("getVar", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(e.GetVar(call.Argument(0).String()))
	})

	engineObj.Set("setVar", func(call goja.FunctionCall) goja.Value {
		e.SetVar(call.Argument(0).String(), call.Argument(1).Export())
		return goja.Undefined()
	})

	// Event API.
	engineObj.Set("on", func(call goja.FunctionCall) goja.Value {
		eventName := call.Argument(0).String()
		fn, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			log.Printf("[JS] Engine.on(%s): second argument is not a function", eventName)
			return goja.Undefined()
		}
		e.On(eventName, func(args ...interface{}) {
			jsArgs := make([]goja.Value, len(args))
			for i, arg := range args {
				jsArgs[i] = vm.ToValue(arg)
			}
			if _, err := fn(goja.Undefined(), jsArgs...); err != nil {
				log.Printf("[JS] Event %s error: %v", eventName, err)
			}
		})
		return goja.Undefined()
	})

	engineObj.Set("triggerEvent", func(call goja.FunctionCall) goja.Value {
		e.Emit(call.Argument(0).String())
		return goja.Undefined()
	})

	// Game Flow API.
	engineObj.Set("winGame", func(call goja.FunctionCall) goja.Value {
		e.Emit("onWin")
		if e.OnWin != nil {
			e.OnWin()
		}
		return goja.Undefined()
	})

	engineObj.Set("loseGame", func(call goja.FunctionCall) goja.Value {
		e.Emit("onLose")
		if e.OnLose != nil {
			e.OnLose()
		}
		return goja.Undefined()
	})

	vm.Set("Engine", engineObj)

	// --- UI global object (HUD, menus, effects) ---
	uiObj := vm.NewObject()

	// HUD elements.
	uiObj.Set("setHP", func(call goja.FunctionCall) goja.Value {
		current := call.Argument(0).ToFloat()
		max := call.Argument(1).ToFloat()
		log.Printf("[JS] UI.setHP(%f, %f)", current, max)
		return goja.Undefined()
	})

	uiObj.Set("setAmmo", func(call goja.FunctionCall) goja.Value {
		current := call.Argument(0).ToInteger()
		reserve := call.Argument(1).ToInteger()
		log.Printf("[JS] UI.setAmmo(%d, %d)", current, reserve)
		return goja.Undefined()
	})

	uiObj.Set("setBulletTime", func(call goja.FunctionCall) goja.Value {
		val := call.Argument(0).ToFloat()
		max := call.Argument(1).ToFloat()
		log.Printf("[JS] UI.setBulletTime(%f, %f)", val, max)
		return goja.Undefined()
	})

	uiObj.Set("setCrosshair", func(call goja.FunctionCall) goja.Value {
		size := call.Argument(0).ToFloat()
		log.Printf("[JS] UI.setCrosshair(%f)", size)
		return goja.Undefined()
	})

	uiObj.Set("setMessage", func(call goja.FunctionCall) goja.Value {
		e.ShowMessage(call.Argument(0).String())
		return goja.Undefined()
	})

	uiObj.Set("hideMessage", func(call goja.FunctionCall) goja.Value {
		e.HideMessage()
		return goja.Undefined()
	})

	uiObj.Set("showElement", func(call goja.FunctionCall) goja.Value {
		log.Printf("[JS] UI.showElement(%s) stub", call.Argument(0).String())
		return goja.Undefined()
	})

	uiObj.Set("hideElement", func(call goja.FunctionCall) goja.Value {
		log.Printf("[JS] UI.hideElement(%s) stub", call.Argument(0).String())
		return goja.Undefined()
	})

	// Filters.
	uiObj.Set("setFilter", func(call goja.FunctionCall) goja.Value {
		name := call.Argument(0).String()
		intensity := call.Argument(1).ToFloat()
		log.Printf("[JS] UI.setFilter(%s, %f)", name, intensity)
		if e.Effects != nil {
			switch name {
			case "chromatic_aberration":
				e.Effects.ChromaticAberration = intensity
			case "vignette":
				e.Effects.Vignette = intensity
			case "blur":
				e.Effects.BlurAmount = intensity
			case "red_flash":
				e.Effects.RedTint = intensity
			case "noise":
				e.Effects.FilmGrain = intensity
			}
		}
		return goja.Undefined()
	})

	// Menus.
	uiObj.Set("showMenu", func(call goja.FunctionCall) goja.Value {
		menuType := call.Argument(0).String()
		log.Printf("[JS] UI.showMenu(%s) stub", menuType)
		return goja.Undefined()
	})

	uiObj.Set("hideMenu", func(call goja.FunctionCall) goja.Value {
		log.Println("[JS] UI.hideMenu() stub")
		return goja.Undefined()
	})

	uiObj.Set("showDeathScreen", func(call goja.FunctionCall) goja.Value {
		log.Println("[JS] UI.showDeathScreen() stub")
		return goja.Undefined()
	})

	uiObj.Set("showWinScreen", func(call goja.FunctionCall) goja.Value {
		log.Println("[JS] UI.showWinScreen() stub")
		return goja.Undefined()
	})

	uiObj.Set("showPauseMenu", func(call goja.FunctionCall) goja.Value {
		log.Println("[JS] UI.showPauseMenu() stub")
		return goja.Undefined()
	})

	uiObj.Set("showMainMenu", func(call goja.FunctionCall) goja.Value {
		log.Println("[JS] UI.showMainMenu() stub")
		return goja.Undefined()
	})

	uiObj.Set("showInventory", func(call goja.FunctionCall) goja.Value {
		log.Println("[JS] UI.showInventory() stub")
		return goja.Undefined()
	})

	uiObj.Set("showObjectives", func(call goja.FunctionCall) goja.Value {
		log.Println("[JS] UI.showObjectives() stub")
		return goja.Undefined()
	})

	// Dialog.
	uiObj.Set("showDialog", func(call goja.FunctionCall) goja.Value {
		character := call.Argument(0).String()
		text := call.Argument(1).String()
		log.Printf("[JS] UI.showDialog(%s, %s) stub", character, text)
		return goja.Undefined()
	})

	uiObj.Set("hideDialog", func(call goja.FunctionCall) goja.Value {
		log.Println("[JS] UI.hideDialog() stub")
		return goja.Undefined()
	})

	uiObj.Set("addDialogOption", func(call goja.FunctionCall) goja.Value {
		text := call.Argument(0).String()
		log.Printf("[JS] UI.addDialogOption(%s, fn) stub", text)
		return goja.Undefined()
	})

	// Minimap.
	uiObj.Set("setMinimapVisible", func(call goja.FunctionCall) goja.Value {
		visible := call.Argument(0).ToBoolean()
		log.Printf("[JS] UI.setMinimapVisible(%v) stub", visible)
		return goja.Undefined()
	})

	uiObj.Set("updateMinimap", func(call goja.FunctionCall) goja.Value {
		log.Println("[JS] UI.updateMinimap(data) stub")
		return goja.Undefined()
	})

	uiObj.Set("addMinimapMarker", func(call goja.FunctionCall) goja.Value {
		x := call.Argument(0).ToFloat()
		y := call.Argument(1).ToFloat()
		markerType := call.Argument(2).String()
		log.Printf("[JS] UI.addMinimapMarker(%f, %f, %s) stub", x, y, markerType)
		return goja.Undefined()
	})

	uiObj.Set("removeMinimapMarker", func(call goja.FunctionCall) goja.Value {
		log.Printf("[JS] UI.removeMinimapMarker(%s) stub", call.Argument(0).String())
		return goja.Undefined()
	})

	vm.Set("UI", uiObj)

	// --- Global convenience functions (matching Lua style) ---
	vm.Set("showMessage", func(call goja.FunctionCall) goja.Value {
		e.ShowMessage(call.Argument(0).String())
		return goja.Undefined()
	})

	vm.Set("hideMessage", func(call goja.FunctionCall) goja.Value {
		e.HideMessage()
		return goja.Undefined()
	})

	vm.Set("setFilter", func(call goja.FunctionCall) goja.Value {
		name := call.Argument(0).String()
		intensity := call.Argument(1).ToFloat()
		if e.Effects != nil {
			switch name {
			case "chromatic_aberration":
				e.Effects.ChromaticAberration = intensity
			case "vignette":
				e.Effects.Vignette = intensity
			case "blur":
				e.Effects.BlurAmount = intensity
			case "red_flash":
				e.Effects.RedTint = intensity
			case "noise":
				e.Effects.FilmGrain = intensity
			}
		}
		return goja.Undefined()
	})

	vm.Set("setHP", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(e.GetPlayerHP())
	})

	// --- console.log ---
	consoleObj := vm.NewObject()
	consoleObj.Set("log", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		log.Printf("[JS] %v", args)
		return goja.Undefined()
	})
	vm.Set("console", consoleObj)
}

// RunString executes a JavaScript script string.
func (jb *JSBridge) RunString(script string) (goja.Value, error) {
	val, err := jb.VM.RunString(script)
	if err != nil {
		return nil, fmt.Errorf("js: %w", err)
	}
	return val, nil
}

// RunFile executes a JavaScript script from a file.
func (jb *JSBridge) RunFile(path string) (goja.Value, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("js file %s: %w", path, err)
	}
	return jb.RunString(string(data))
}

// Call invokes a named JS function with arguments.
func (jb *JSBridge) Call(fnName string, args ...interface{}) (goja.Value, error) {
	fn, ok := goja.AssertFunction(jb.VM.Get(fnName))
	if !ok {
		return nil, fmt.Errorf("js function %s not found", fnName)
	}
	jsArgs := make([]goja.Value, len(args))
	for i, arg := range args {
		jsArgs[i] = jb.VM.ToValue(arg)
	}
	return fn(goja.Undefined(), jsArgs...)
}
