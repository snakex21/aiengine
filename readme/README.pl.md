# AIEngine — Silnik Gier 2.5D

**Tworzony przez AI. Napędzany przez AI. Open source na zawsze.**

AIEngine to silnik gier 2.5D w stylu Dooma, napisany od podstaw w Go.
Modele, tekstury, dźwięki i logika gry są projektowane do generowania przez LLM.
Renderer używa raycastingu z CPU pixel-bufferem — zero problemów z shaderami GPU,
gwarantowane działanie na każdej karcie OpenGL 3.3+.

---

## Spis treści

- [Szybki start](#szybki-start)
- [Architektura](#architektura)
- [Formaty plików](#formaty-plików)
- [Skryptowanie](#skryptowanie)
- [Kluczowe funkcje](#kluczowe-funkcje)
- [Struktura projektu](#struktura-projektu)
- [Kompilacja](#kompilacja)
- [Konfiguracja](#konfiguracja)
- [Demo](#demo)
- [Licencja](#licencja)

---

## Szybki start

```bash
git clone https://github.com/snakex21/aiengine.git
cd aiengine
go mod tidy
go build -o aiengine.exe .
./aiengine.exe
```

**Wymagania:** Go 1.26+, OpenGL 3.3+, Windows 10/11 64-bit.

---

## Architektura

```
Go Core (silnik, renderer, parser)
├── GLFW          Natywne okno
├── OpenGL 3.3    Renderer raycasting
├── gopher-lua    Maszyna Lua (logika misji, AI)
├── goja          Maszyna JS  (UI, HUD, menu)
└── Własny parser .aie / .css / config
```

| Język | Rola |
|-------|------|
| **Go** | Silnik, renderer, fizyka, parser |
| **Lua** | Logika misji, AI wrogów, eventy |
| **JS** | UI, HUD, menu, efekty |
| **CSS** | Kolory, tekstury, efekty ekranu |

Wszystko skompilowane do **jednego .exe** — zero zewnętrznych runtime'ów.

---

## Formaty plików

### .aie — Encje, Mapy, Bronie, Dźwięki
Format oparty na wcięciach z 3 poziomami złożoności:

```aie
# Poziom 1 — prosty
HUMANOID grunt
HP 50
HEAD small skin

# Poziom 2 — bloki
HP_PARTS
  HEAD 25
  TORSO 60

# Poziom 3 — pełna kontrola
ANIM idle
  FRAME 0
    HEAD ROT 0 5 0
  FRAME 30
    HEAD ROT 0 -5 0
END
```

Obsługiwane typy encji: `HUMANOID`, `ALIEN`, `MUTANT`, `ROBOT`, `UNDEAD`, `QUADRUPED`, `MULTI_LEG`, `FLYING`, `AQUATIC`, `AMORPHOUS`, `SERPENTINE`, `GIANT`, `VEHICLE`, `PROP`, `WEAPON`, `MAP`.

### .css — Stylizacja i efekty

```css
.gangster {
    color: #4A4A4A;
    texture: cloth;
}

screen {
    vignette: 0.3;
    film-grain: 0.1;
}

player:low-hp {
    vignette: 0.8;
    red-tint: 0.3;
    pulse: yes;
}
```

Wspiera: dziedziczenie (`extends`), pseudo-klasy (`:pain`, `:dead`), tekstury proceduralne (`noise(perlin, scale=2.0, seed=42)`), efekty ekranu.

### config.aie — Sterowanie i ustawienia

```
WINDOW_WIDTH 1280
WINDOW_HEIGHT 720

BIND move_forward W
BIND shoot MOUSE_LEFT
BIND fullscreen F11
BIND pause ESCAPE
```

---

## Skryptowanie

### Lua — Logika misji

```lua
function onEnemyDead(id, enemyType)
    local kills = getVar("kills") + 1
    setVar("kills", kills)
    if kills >= 10 then
        unlockDoor("door_boss")
        showMessage("Drzwi do bossa otwarte!")
    end
end
registerEvent("onEnemyDead", onEnemyDead)
```

**45+ funkcji API:** `spawnEnemy()`, `getPlayerHP()`, `unlockDoor()`, `playSound()`, `showMessage()`, `getVar()`, `registerEvent()`, `winGame()`...

### JavaScript — UI i HUD

```js
Engine.on("onPlayerDamage", function(amount) {
    UI.setFilter("red_flash", 0.8);
    UI.setHP(Engine.getPlayerHP(), 100);
});

Engine.on("onPause", function(paused) {
    if (paused) UI.showPauseMenu();
    else UI.hideMenu();
});
```

**`Engine.*`** (25+ metod) + **`UI.*`** (25+ metod): HUD, menu, filtry ekranu, minimapa, dialogi.

---

## Kluczowe funkcje

| Funkcja | Status |
|---------|--------|
| **DDA Raycasting** | ✅ Działa — CPU pixel-buffer, teksturowane ściany |
| **Billboard Sprite'y** | ✅ Z-buffer occlusion, pozycjonowanie kątowe |
| **Kolizje predykcyjne** | ✅ Per-axis sliding, zero drgań |
| **Tekstury Perlin Noise** | ✅ Cegła, kamień, metal, drewno, skóra, tkanina |
| **Dźwięk proceduralny** | ✅ Sine/square/sawtooth/noise, obwiednia ADSR |
| **Skryptowanie Lua** | ✅ 45+ funkcji API, system eventów |
| **Skryptowanie JS** | ✅ 50+ funkcji API, kontrola UI |
| **Parser CSS** | ✅ Dziedziczenie, pseudo-klasy, efekty |
| **Parser .aie** | ✅ 3 poziomy, wszystkie typy encji, mapy |
| **System HP bossów** | ✅ HP per część ciała, fazy, słabe punkty, pancerz |
| **System broni** | ✅ 8 typów, hitscan, rozrzut, przeładowanie, 4 presety |
| **System triggerów** | ✅ 9 warunków, 15 akcji |
| **System cutscenek** | ✅ 4 poziomy złożoności, timeline player |
| **Fizyka ragdoll** | ✅ Łańcuch Verleta, solver więzów |
| **Sieć/Multiplayer** | ✅ Protokół UDP, serwer/klient, serializacja |
| **Post-processing** | 🔜 Planowany — shader gotowy, potrzebny GPU renderer |
| **Odtwarzanie audio** | 🔜 Stub — generator działa, potrzebne wyjście audio |

---

## Struktura projektu

```
aiengine/
├── main.go                    Punkt wejścia, pętla gry
├── parser/                    Parsery .aie, .css, config
│   ├── aie.go                 Parser encji/map/triggerów/cutscenek
│   ├── css.go                 Parser stylów i efektów
│   └── config.go              Parser sterowania i ustawień
├── renderer/                  Silnik renderowania 2.5D
│   ├── raycaster.go           DDA raycasting + CPU pixel-buffer
│   ├── sprites.go             Billboard sprite'y
│   ├── textures.go            Tekstury proceduralne Perlin noise
│   ├── effects.go             Konfiguracja post-processingu
│   └── shader.go              Kompilator shaderów GLSL
├── scripting/                 Mostki Lua + JS
│   ├── engine.go              Współdzielony stan silnika i system eventów
│   ├── lua.go                 Mostek gopher-lua (45+ API)
│   └── js.go                  Mostek goja (50+ API)
├── systems/                   Podsystemy silnika
│   ├── physics.go             Grawitacja, kolizje, ragdoll
│   ├── audio.go               Generowanie dźwięku proceduralnego
│   └── network.go             Protokół multiplayer UDP
├── gameplay/                  Systemy logiki gry
│   ├── entities.go            Typy encji, animacje, zachowania
│   ├── weapons.go             Typy broni, hitscan, amunicja
│   ├── hp.go                  HP per część, fazy, pancerz
│   ├── triggers.go            Ewaluacja warunek-akcja
│   └── cutscenes.go           Odtwarzacz cutscenek timeline
├── input/                     Manager sterowania
├── demo/                      Demo assetów
│   ├── config.aie
│   ├── maps/level1.aie
│   ├── entities/              grunt.aie, boss_kaiju.aie
│   ├── scripts/               level1.lua, boss_kaiju.lua, hud.js
│   └── styles/                grunt.css, level1.css
└── readme/                    Dokumentacja (EN/PL)
```

**41 plików źródłowych, 74 testy, zero zewnętrznych zależności runtime.**

---

## Kompilacja

```bash
# Wymagania: Go 1.26+, CGO włączone (dla GLFW)

git clone https://github.com/snakex21/aiengine.git
cd aiengine
go mod tidy
go build -o aiengine.exe .

# Uruchom testy
go test ./...       # 74 testów
go vet ./...        # analiza statyczna
```

### Zależności deweloperskie
- `github.com/go-gl/glfw` — zarządzanie oknem
- `github.com/go-gl/gl` — bindings OpenGL
- `github.com/yuin/gopher-lua` — maszyna Lua
- `github.com/dop251/goja` — maszyna JavaScript

Wszystkie zależności są pobierane automatycznie przez `go mod tidy`.

---

## Konfiguracja

Edytuj `demo/config.aie`:

```aie
# Okno
WINDOW_WIDTH 1280
WINDOW_HEIGHT 720

# Sterowanie
BIND move_forward W
BIND move_backward S
BIND move_left A
BIND move_right D
BIND shoot MOUSE_LEFT
BIND fullscreen F11
BIND pause ESCAPE
```

Wszystkie nazwy klawiszy z GLFW są wspierane: `W`, `SPACE`, `ESCAPE`, `F11`, `MOUSE_LEFT`, `LEFT_SHIFT` itd.

---

## Demo

Demo zawiera:
- **Mapę testową 10×10** ze ścianami z cegły, kamienia i metalu
- **4 sprite'y** (2 wrogów, 2 propsy)
- **Gracza** z ruchem WASD + mysz
- **F11** przełączanie fullscreen
- **ESC** pauza z uwolnieniem kursora
- **Skrypty Lua** do logiki misji i walk z bossami
- **HUD JS** z efektami obrażeń i menu pauzy

Sterowanie:
- `WASD` — Ruch
- `Mysz` — Rozglądanie
- `F11` — Pełny ekran
- `ESC` — Pauza / Wznów

---

## Licencja

**MIT License** — darmowy na zawsze, dla ludzkości.

Możesz:
- ✅ Używać komercyjnie
- ✅ Modyfikować
- ✅ Dystrybuować
- ✅ Tworzyć gry closed-source na tym silniku

Jedyny wymóg:
- Zachowaj informację o autorze w silniku

---

## Autor

**Maks (snakex21)**

---

*"Modele, tekstury i logika gry generowane przez LLM."*
