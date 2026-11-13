// scripting/lua.go — Lua scripting bridge for AIEngine.
// Embeds gopher-lua VM and registers all engine API functions as Lua globals.
// Lua scripts control mission logic, NPC behavior, events, and complex game flow.
// See DOCS.md "Lua API" section for the full function reference.

package scripting

import (
	"fmt"
	"log"

	lua "github.com/yuin/gopher-lua"
)

// LuaBridge wraps a gopher-lua LState with the AIEngine API.
type LuaBridge struct {
	L      *lua.LState
	engine *Engine
}

// NewLuaBridge creates a new Lua VM and registers all API functions.
func NewLuaBridge(engine *Engine) *LuaBridge {
	L := lua.NewState()
	lb := &LuaBridge{L: L, engine: engine}
	lb.registerAPI()
	return lb
}

// registerAPI exposes all engine functions as Lua globals.
func (lb *LuaBridge) registerAPI() {
	L := lb.L
	e := lb.engine

	// --- Enemy API ---
	L.SetGlobal("spawnEnemy", L.NewFunction(func(L *lua.LState) int {
		typ := L.CheckString(1)
		x := float64(L.CheckNumber(2))
		y := float64(L.CheckNumber(3))
		z := float64(0.0)
		if L.GetTop() >= 4 {
			z = float64(L.CheckNumber(4))
		}
		id := e.SpawnEnemy(typ, x, y, z)
		L.Push(lua.LString(id))
		return 1
	}))

	L.SetGlobal("killEnemy", L.NewFunction(func(L *lua.LState) int {
		e.KillEnemy(L.CheckString(1))
		return 0
	}))

	L.SetGlobal("getEnemyHP", L.NewFunction(func(L *lua.LState) int {
		hp := e.GetEnemyHP(L.CheckString(1))
		L.Push(lua.LNumber(hp))
		return 1
	}))

	L.SetGlobal("setEnemyHP", L.NewFunction(func(L *lua.LState) int {
		e.SetEnemyHP(L.CheckString(1), float64(L.CheckNumber(2)))
		return 0
	}))

	L.SetGlobal("allEnemiesDead", L.NewFunction(func(L *lua.LState) int {
		zone := ""
		if L.GetTop() >= 1 {
			zone = L.CheckString(1)
		}
		allDead := e.AllEnemiesDeadInZone(zone)
		L.Push(lua.LBool(allDead))
		return 1
	}))

	L.SetGlobal("getEnemyCount", L.NewFunction(func(L *lua.LState) int {
		zone := ""
		if L.GetTop() >= 1 {
			zone = L.CheckString(1)
		}
		count := e.GetEnemyCountInZone(zone)
		L.Push(lua.LNumber(count))
		return 1
	}))

	L.SetGlobal("moveEnemy", L.NewFunction(func(L *lua.LState) int {
		id := L.CheckString(1)
		x := float64(L.CheckNumber(2))
		y := float64(L.CheckNumber(3))
		z := float64(0.0)
		if L.GetTop() >= 4 {
			z = float64(L.CheckNumber(4))
		}
		e.MoveEnemy(id, x, y, z)
		return 0
	}))

	L.SetGlobal("freezeEnemy", L.NewFunction(func(L *lua.LState) int {
		e.FreezeEnemy(L.CheckString(1))
		return 0
	}))

	L.SetGlobal("unfreezeEnemy", L.NewFunction(func(L *lua.LState) int {
		e.UnfreezeEnemy(L.CheckString(1))
		return 0
	}))

	// --- Player API ---
	L.SetGlobal("getPlayerHP", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(e.GetPlayerHP()))
		return 1
	}))

	L.SetGlobal("setPlayerHP", L.NewFunction(func(L *lua.LState) int {
		e.SetPlayerHP(float64(L.CheckNumber(1)))
		return 0
	}))

	L.SetGlobal("getPlayerPosition", L.NewFunction(func(L *lua.LState) int {
		x, y, z := e.GetPlayerPosition()
		L.Push(lua.LNumber(x))
		L.Push(lua.LNumber(y))
		L.Push(lua.LNumber(z))
		return 3
	}))

	L.SetGlobal("setPlayerPosition", L.NewFunction(func(L *lua.LState) int {
		e.SetPlayerPosition(
			float64(L.CheckNumber(1)),
			float64(L.CheckNumber(2)),
			float64(L.CheckNumber(3)),
		)
		return 0
	}))

	L.SetGlobal("getPlayerAmmo", L.NewFunction(func(L *lua.LState) int {
		ammo := e.GetPlayerAmmo(L.CheckString(1))
		L.Push(lua.LNumber(ammo))
		return 1
	}))

	L.SetGlobal("setPlayerAmmo", L.NewFunction(func(L *lua.LState) int {
		e.SetPlayerAmmo(L.CheckString(1), int(L.CheckNumber(2)))
		return 0
	}))

	L.SetGlobal("giveWeapon", L.NewFunction(func(L *lua.LState) int {
		e.GiveWeapon(L.CheckString(1))
		return 0
	}))

	L.SetGlobal("removeWeapon", L.NewFunction(func(L *lua.LState) int {
		e.RemoveWeapon(L.CheckString(1))
		return 0
	}))

	L.SetGlobal("freezePlayer", L.NewFunction(func(L *lua.LState) int {
		e.FreezePlayer()
		return 0
	}))

	L.SetGlobal("unfreezePlayer", L.NewFunction(func(L *lua.LState) int {
		e.UnfreezePlayer()
		return 0
	}))

	L.SetGlobal("setPlayerSpeed", L.NewFunction(func(L *lua.LState) int {
		e.SetPlayerSpeed(float64(L.CheckNumber(1)))
		return 0
	}))

	// --- Map API ---
	L.SetGlobal("unlockDoor", L.NewFunction(func(L *lua.LState) int {
		e.UnlockDoor(L.CheckString(1))
		return 0
	}))

	L.SetGlobal("setLight", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(e.SetLight(L.CheckString(1), float64(L.CheckNumber(2)), "")))
		return 1
	}))

	L.SetGlobal("setLightEffect", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(e.SetLight(L.CheckString(1), -1, L.CheckString(2))))
		return 1
	}))

	L.SetGlobal("setCellLight", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(e.SetCellLight(L.CheckInt(1), L.CheckInt(2), float64(L.CheckNumber(3)))))
		return 1
	}))

	L.SetGlobal("lockDoor", L.NewFunction(func(L *lua.LState) int {
		e.LockDoor(L.CheckString(1))
		return 0
	}))

	L.SetGlobal("openDoor", L.NewFunction(func(L *lua.LState) int {
		e.UnlockDoor(L.CheckString(1)) // alias
		return 0
	}))

	L.SetGlobal("closeDoor", L.NewFunction(func(L *lua.LState) int {
		e.LockDoor(L.CheckString(1)) // alias
		return 0
	}))

	// --- Sound & Music API ---
	L.SetGlobal("playSound", L.NewFunction(func(L *lua.LState) int {
		e.PlaySound(L.CheckString(1))
		return 0
	}))

	L.SetGlobal("stopSound", L.NewFunction(func(L *lua.LState) int {
		e.StopSound(L.CheckString(1))
		return 0
	}))

	L.SetGlobal("playMusic", L.NewFunction(func(L *lua.LState) int {
		e.PlayMusic(L.CheckString(1))
		return 0
	}))

	L.SetGlobal("stopMusic", L.NewFunction(func(L *lua.LState) int {
		e.StopMusic()
		return 0
	}))

	L.SetGlobal("changeMusic", L.NewFunction(func(L *lua.LState) int {
		e.ChangeMusic(L.CheckString(1))
		return 0
	}))

	L.SetGlobal("setVolume", L.NewFunction(func(L *lua.LState) int {
		e.SetVolume(float64(L.CheckNumber(1)))
		return 0
	}))

	// --- UI API ---
	L.SetGlobal("showMessage", L.NewFunction(func(L *lua.LState) int {
		e.ShowMessageFor(L.CheckString(1), float64(L.OptNumber(2, DefaultMessageSeconds)))
		return 0
	}))

	L.SetGlobal("setFilter", L.NewFunction(func(L *lua.LState) int {
		e.SetFilter(L.CheckString(1), float64(L.CheckNumber(2)))
		return 0
	}))

	L.SetGlobal("hideMessage", L.NewFunction(func(L *lua.LState) int {
		e.HideMessage()
		return 0
	}))

	L.SetGlobal("showImage", L.NewFunction(func(L *lua.LState) int {
		log.Printf("[Lua] showImage: %s (stub)", L.CheckString(1))
		return 0
	}))

	L.SetGlobal("hideImage", L.NewFunction(func(L *lua.LState) int {
		log.Printf("[Lua] hideImage (stub)")
		return 0
	}))

	L.SetGlobal("setHUDVisible", L.NewFunction(func(L *lua.LState) int {
		visible := true
		if L.GetTop() >= 1 {
			switch L.CheckAny(1).(type) {
			case lua.LBool:
				visible = bool(L.CheckBool(1))
			case lua.LString:
				visible = L.CheckString(1) == "yes"
			}
		}
		e.SetHUDVisible(visible)
		return 0
	}))

	// --- Variable API ---
	L.SetGlobal("getVar", L.NewFunction(func(L *lua.LState) int {
		val := e.GetVar(L.CheckString(1))
		switch v := val.(type) {
		case float64:
			L.Push(lua.LNumber(v))
		case int:
			L.Push(lua.LNumber(v))
		case string:
			L.Push(lua.LString(v))
		case bool:
			L.Push(lua.LBool(v))
		default:
			L.Push(lua.LNil)
		}
		return 1
	}))

	L.SetGlobal("setVar", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		val := L.CheckAny(2)
		switch v := val.(type) {
		case lua.LNumber:
			e.SetVar(name, float64(v))
		case lua.LString:
			e.SetVar(name, string(v))
		case lua.LBool:
			e.SetVar(name, bool(v))
		default:
			e.SetVar(name, val.String())
		}
		return 0
	}))

	// --- Timer API ---
	L.SetGlobal("startTimer", L.NewFunction(func(L *lua.LState) int {
		e.StartTimer(L.CheckString(1), float64(L.CheckNumber(2)))
		return 0
	}))

	L.SetGlobal("stopTimer", L.NewFunction(func(L *lua.LState) int {
		e.StopTimer(L.CheckString(1))
		return 0
	}))

	L.SetGlobal("getTime", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(e.GetTime()))
		return 1
	}))

	// --- Camera API ---
	L.SetGlobal("moveCamera", L.NewFunction(func(L *lua.LState) int {
		x := float64(L.CheckNumber(1))
		y := float64(L.CheckNumber(2))
		z := float64(L.CheckNumber(3))
		duration := float64(L.CheckNumber(4))
		if OnMoveCamera != nil {
			OnMoveCamera(x, y, z, duration)
		}
		log.Printf("[Lua] moveCamera(%f, %f, %f, %f) stub", x, y, z, duration)
		return 0
	}))

	L.SetGlobal("shakeCamera", L.NewFunction(func(L *lua.LState) int {
		intensity := float64(L.CheckNumber(1))
		duration := float64(L.CheckNumber(2))
		if OnShakeCamera != nil {
			OnShakeCamera(intensity, duration)
		}
		return 0
	}))

	L.SetGlobal("lookAt", L.NewFunction(func(L *lua.LState) int {
		x := float64(L.CheckNumber(1))
		y := float64(L.CheckNumber(2))
		z := float64(L.CheckNumber(3))
		duration := float64(0.0)
		if L.GetTop() >= 4 {
			duration = float64(L.CheckNumber(4))
		}
		if OnLookAt != nil {
			OnLookAt(x, y, z, duration)
		}
		return 0
	}))

	// --- Event API ---
	L.SetGlobal("registerEvent", L.NewFunction(func(L *lua.LState) int {
		eventName := L.CheckString(1)
		fn := L.CheckFunction(2)
		// Keep a reference to prevent GC.
		fnRef := fn
		e.On(eventName, func(args ...interface{}) {
			// Push the callback followed by exactly the emitted args.
			// (Previously the event name was pushed as an extra value, so
			// PCall tried to call that string and every Lua event failed.)
			L.Push(fnRef)
			for _, arg := range args {
				switch v := arg.(type) {
				case float64:
					L.Push(lua.LNumber(v))
				case int:
					L.Push(lua.LNumber(v))
				case string:
					L.Push(lua.LString(v))
				case bool:
					L.Push(lua.LBool(v))
				default:
					L.Push(lua.LString(fmt.Sprintf("%v", v)))
				}
			}
			nargs := len(args)
			if err := L.PCall(nargs, 0, nil); err != nil {
				log.Printf("[Lua] Event %s error: %v", eventName, err)
			}
		})
		return 0
	}))

	L.SetGlobal("triggerEvent", L.NewFunction(func(L *lua.LState) int {
		e.Emit(L.CheckString(1))
		return 0
	}))

	// --- Game Flow API ---
	L.SetGlobal("winGame", L.NewFunction(func(L *lua.LState) int {
		log.Println("[Lua] winGame called")
		e.WinGame(L.OptString(1, ""))
		return 0
	}))

	L.SetGlobal("loseGame", L.NewFunction(func(L *lua.LState) int {
		log.Println("[Lua] loseGame called")
		e.LoseGame(L.OptString(1, ""))
		return 0
	}))

	L.SetGlobal("loadMap", L.NewFunction(func(L *lua.LState) int {
		mapName := L.CheckString(1)
		log.Printf("[Lua] loadMap: %s", mapName)
		if e.OnMapChange != nil {
			e.OnMapChange(mapName)
		}
		return 0
	}))

	L.SetGlobal("restartMap", L.NewFunction(func(L *lua.LState) int {
		e.RestartMap()
		return 0
	}))

	L.SetGlobal("exitGame", L.NewFunction(func(L *lua.LState) int {
		log.Println("[Lua] exitGame called")
		e.ExitGame()
		return 0
	}))

	L.SetGlobal("exitLevel", L.NewFunction(func(L *lua.LState) int {
		e.ExitLevel(L.OptString(1, "next"), L.OptString(2, ""))
		return 0
	}))

	L.SetGlobal("giveItem", L.NewFunction(func(L *lua.LState) int {
		e.GiveItem(L.CheckString(1), L.OptInt(2, 1))
		return 0
	}))

	L.SetGlobal("takeItem", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(e.TakeItem(L.CheckString(1), L.OptInt(2, 1))))
		return 1
	}))

	L.SetGlobal("itemCount", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(e.ItemCount(L.CheckString(1))))
		return 1
	}))

	L.SetGlobal("hasItem", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(e.ItemCount(L.CheckString(1)) > 0))
		return 1
	}))

	L.SetGlobal("setHUDText", L.NewFunction(func(L *lua.LState) int {
		e.SetHUDText(L.CheckString(1), L.OptString(2, ""))
		return 0
	}))

	// --- Utility ---
	L.SetGlobal("wait", L.NewFunction(func(L *lua.LState) int {
		duration := float64(L.CheckNumber(1))
		// In a real engine, this would pause script execution.
		// For now, log it.
		log.Printf("[Lua] wait(%f) stub", duration)
		return 0
	}))

	// spawnProp stub.
	L.SetGlobal("spawnProp", L.NewFunction(func(L *lua.LState) int {
		propType := L.CheckString(1)
		x := float64(L.CheckNumber(2))
		y := float64(L.CheckNumber(3))
		z := float64(L.CheckNumber(4))
		log.Printf("[Lua] spawnProp(%s, %f, %f, %f) stub", propType, x, y, z)
		return 0
	}))

	// removeProp stub.
	L.SetGlobal("removeProp", L.NewFunction(func(L *lua.LState) int {
		log.Printf("[Lua] removeProp(%s) stub", L.CheckString(1))
		return 0
	}))

	// changeTexture stub.
	L.SetGlobal("changeTexture", L.NewFunction(func(L *lua.LState) int {
		id := L.CheckString(1)
		tex := L.CheckString(2)
		log.Printf("[Lua] changeTexture(%s, %s) stub", id, tex)
		return 0
	}))
}

// DoString executes a Lua script string.
func (lb *LuaBridge) DoString(script string) error {
	if err := lb.L.DoString(script); err != nil {
		return fmt.Errorf("lua: %w", err)
	}
	return nil
}

// DoFile executes a Lua script from a file.
func (lb *LuaBridge) DoFile(path string) error {
	if err := lb.L.DoFile(path); err != nil {
		return fmt.Errorf("lua file %s: %w", path, err)
	}
	return nil
}

// Call invokes a named Lua function with optional arguments.
func (lb *LuaBridge) Call(fnName string, args ...lua.LValue) error {
	fn := lb.L.GetGlobal(fnName)
	if fn == lua.LNil {
		return fmt.Errorf("lua function %s not found", fnName)
	}
	lb.L.Push(fn)
	for _, arg := range args {
		lb.L.Push(arg)
	}
	if err := lb.L.PCall(len(args), 0, nil); err != nil {
		return fmt.Errorf("lua call %s: %w", fnName, err)
	}
	return nil
}

// Close shuts down the Lua VM.
func (lb *LuaBridge) Close() {
	if lb.L != nil {
		lb.L.Close()
	}
}
