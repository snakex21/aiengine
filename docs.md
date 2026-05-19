# AIEngine Documentation v1.0

## Co to jest AIEngine
Silnik gier 2.5D inspirowany Doomem,
zaprojektowany od podstaw dla współpracy z AI.
Modele, tekstury i logika gry generowane przez LLM.
Renderer raycasting z możliwością upgrade do pełnego 3D
bez zmiany formatu danych.

## Autor
Maks (snakex21)

## Format plików
- .aie - geometria, animacje, mapy
- .css - style, kolory, tekstury
- .lua - logika misji i NPC
- .js - UI i HUD

## Status
Silnik w developmencie.
Gra — TBD.
Przykłady używają nazwy "demo" jako placeholder.

## Sterowanie
Sterowanie konfigurowalne przez plik config.aie
Brak hardkodowanych klawiszy w silniku.

### Przykład config.aie
BIND move_forward W
BIND move_backward S
BIND move_left A
BIND move_right D
BIND shoot MOUSE_LEFT
BIND reload R
BIND fullscreen F11
BIND pause ESCAPE


## Dźwięk

### Obsługiwane formaty
- .mp3
- .ogg  
- .wav
- .flac

### Generowanie przez AI (unikalna funkcja AIEngine)
LLM opisuje dźwięk w .aie formacie,
silnik generuje proceduralnie.

### Przykład generowanego dźwięku
SOUND gunshot
  TYPE impact
  FREQUENCY 800
  DECAY 0.1
  NOISE 0.3
  REVERB 0.2

SOUND footstep_stone
  TYPE click
  FREQUENCY 300
  DECAY 0.05
  RANDOMIZE 0.1

SOUND ambient_wind
  TYPE noise
  FREQUENCY 200
  CONTINUOUS true
  VOLUME 0.3

# POZIOM 1 - prosty
SOUND gunshot TYPE=impact PITCH=high

# POZIOM 2 - parametry  
SOUND gunshot
  TYPE impact
  FREQUENCY 800
  DECAY 0.1

# POZIOM 3 - pełna kontrola
SOUND gunshot
  WAVEFORM sawtooth
  FREQUENCY 800
  ENVELOPE 0.001 0.1 0.3 0.2
  FILTER lowpass 2000
  REVERB 0.2


## Sieć / Multiplayer

### Architektura
- Dedykowany serwer (headless mode)
- Peer-to-peer opcjonalnie
- Protokół: UDP dla gry, TCP dla lobby

### Tryby
- Singleplayer (domyślny)
- Co-op (do 4 graczy)
- Deathmatch
- Własne tryby przez Lua

### Konfiguracja serwera
SERVER port 27015
SERVER max_players 4
SERVER mode coop
SERVER map level1.aie

### Klient łączy się przez
CONNECT 192.168.1.1 27015

### Synchronizacja
- Pozycje graczy
- Stan wrogów
- Eventy (strzały, śmierci, triggery)
- Lua eventy synchronizowane automatycznie

## Platformy
- Windows 10/11 (primary) ✅
- Linux (planned)
- macOS (planned)
- Web/WASM (planned)

## Wymagania minimalne Windows
- Windows 10 64-bit
- OpenGL 3.3+
- 4GB RAM
- Dowolny CPU z SSE2

## Licencja
MIT License - open source, darmowy na zawsze.
Dla ludzkości.

Możesz:
- Używać komercyjnie ✅
- Modyfikować ✅
- Dystrybuować ✅
- Robić closed source gry na tym silniku ✅

Jedyny wymóg:
- Zachowaj informację o autorze w silniku

## Typy obiektów .aie

### Poziom 1 - typy
HUMANOID  - człekokształtna postać
CREATURE  - zwierzę, potwór, abominacja
           (może pełnić rolę pojazdu - można wsiąść)
VEHICLE   - pojazd mechaniczny
PROP      - obiekt statyczny (krzesło, skrzynka, beczka)
WEAPON    - broń
MAP       - mapa poziomu

### Przykłady

# Człowiek
HUMANOID gangster
HEAD small skin
TORSO large grey_suit

# Potwór który służy jako wierzchowiec
CREATURE giant_spider
RIDEABLE yes
BODY huge dark_chitin
LEGS 8 medium dark_chitin

# Pojazd mechaniczny
VEHICLE car
BODY large grey_metal
WHEELS 4 small black_rubber

# Statyczny obiekt
PROP barrel
BODY medium wooden
DESTRUCTIBLE yes

# Broń
WEAPON pistol
BARREL small black_metal
GRIP small brown_leather

## Typy HUMANOID - rozszerzone

HUMANOID  - człowiek, humanoid
ALIEN     - kosmita, obca forma życia humanoidalna
MUTANT    - zmutowana forma humanoidalna
ROBOT     - mechaniczny humanoid
UNDEAD    - zombie, nieumarły humanoid

### Ale wszystkie mają te same części bazowe
HEAD, TORSO, ARMS, LEGS

### Różnice między typami
HUMANOID - standardowa anatomia
ALIEN     - może mieć dodatkowe części
           TENTACLE, ANTENNA, EXTRA_EYES
MUTANT    - zniekształcone części
           DEFORMED_HEAD, EXTRA_ARM
ROBOT     - metaliczne części
           SENSOR, ANTENNA, JOINT_VISIBLE
UNDEAD    - zniszczone części
           MISSING_ARM, EXPOSED_BONE

### Przykłady
ALIEN grey_alien
HEAD small grey_skin
EYES large black
TORSO small grey_skin
ARMS 4 tiny grey_skin
LEGS 2 small grey_skin
ANTENNA 2 tiny grey_skin

ROBOT terminator
HEAD medium chrome_metal
SENSOR red
TORSO large chrome_metal
ARMS 2 large chrome_metal
LEGS 2 large chrome_metal
JOINT_VISIBLE yes

## Typy CREATURE

### Podtypy
QUADRUPED   - czworonóg (pies, wilk, koń, lew)
MULTI_LEG   - wielonożny (pająk, owad, skorpion)
FLYING      - latający (ptak, smok, nietoperz)
AQUATIC     - wodny (ryba, rekin, ośmiornica)
AMORPHOUS   - bezkształtny (slime, blob, chmura)
SERPENTINE  - wężowaty (wąż, robak, larwa)
GIANT       - gigantyczny (boss, kaiju)

### Przykłady

# Czworonóg
QUADRUPED wolf
BODY medium grey_fur
HEAD small grey_fur
LEGS 4 small grey_fur
TAIL medium grey_fur
RIDEABLE no

# Wielonożny
MULTI_LEG giant_spider
BODY large dark_chitin
HEAD small dark_chitin
LEGS 8 medium dark_chitin
FANGS 2 tiny black
RIDEABLE yes

# Latający
FLYING dragon
BODY huge red_scales
HEAD large red_scales
WINGS 2 huge red_scales
LEGS 2 large red_scales
TAIL large red_scales
RIDEABLE yes
FIRE_BREATH yes

# Bezkształtny
AMORPHOUS slime
BODY medium green_slime
EYES 2 small black
RIDEABLE no
SPLIT_ON_DEATH yes

# Wężowaty
SERPENTINE giant_worm
BODY_SEGMENTS 8 large brown_skin
HEAD large brown_skin
RIDEABLE yes

# Gigantyczny boss
GIANT kaiju
BODY huge dark_scales
HEAD huge dark_scales
ARMS 2 huge dark_scales
LEGS 2 huge dark_scales
TAIL huge dark_scales
RIDEABLE no
BOSS yes

## Specjalne zachowania (BEHAVIOR)

### Zachowania śmierci
RAGDOLL           - fizyczne opadanie ciała (humanoid domyślnie)
DISSOLVE          - rozpuszczanie się (slime domyślnie)
EXPLODE           - eksplozja przy śmierci
SPLIT             - podział na mniejsze kopie
COLLAPSE          - zawalenie się (robot, giant)
EVAPORATE         - parowanie (duch, gaz)
SINK              - tonięcie (aquatic)
SHATTER           - rozbicie na kawałki (robot, crystal)

### Zachowania specjalne
SPLIT_ON_DEATH yes count=2    - dzieli się na 2 kopie
SPLIT_ON_DAMAGE yes threshold=50  - dzieli się gdy HP < 50%
SEGMENTS yes count=8          - ruch segmentowy (wąż)
REGENERATE yes rate=5         - regeneruje HP
PHASE_THROUGH_WALLS yes       - przechodzi przez ściany
SWARM yes count=10            - porusza się w grupie
BURROW yes                    - chowa się pod ziemią
CLIMB_WALLS yes               - wspina się po ścianach
FLY yes height=2.0            - lata na danej wysokości

### Domyślne zachowania per typ
HUMANOID    → RAGDOLL
ALIEN       → DISSOLVE lub RAGDOLL
MUTANT      → RAGDOLL + EXPLODE opcjonalnie
ROBOT       → COLLAPSE + SHATTER
UNDEAD      → RAGDOLL + REGENERATE opcjonalnie
QUADRUPED   → RAGDOLL
MULTI_LEG   → RAGDOLL + SHATTER opcjonalnie
FLYING      → RAGDOLL (spada z nieba)
AQUATIC     → SINK
AMORPHOUS   → DISSOLVE + SPLIT opcjonalnie
SERPENTINE  → RAGDOLL (segmenty opadają)
GIANT       → COLLAPSE + trzęsienie ziemi

### Przykłady

# Slime który się dzieli
AMORPHOUS slime
BODY medium green_slime
BEHAVIOR
  SPLIT_ON_DEATH yes count=2
  DISSOLVE yes delay=3.0
  MIN_SIZE tiny

# Wąż z segmentami
SERPENTINE giant_worm
BODY_SEGMENTS 8 large brown_skin
HEAD large brown_skin
BEHAVIOR
  SEGMENTS yes count=8
  BURROW yes
  SPLIT_ON_DEATH yes count=3

# Robot który się rozpada
ROBOT terminator
HEAD medium chrome_metal
TORSO large chrome_metal
BEHAVIOR
  COLLAPSE yes
  SHATTER yes parts=5
  SPARKS yes duration=2.0

## System zdrowia (HP)

### Domyślny - tylko globalne HP
HUMANOID gangster
HP 100

### Rozszerzony - globalne + części ciała
HUMANOID gangster
HP 100
HP_PARTS
  HEAD 25      # headshot = instant kill jeśli > globalne
  TORSO 60
  ARM_LEFT 20
  ARM_RIGHT 20
  LEG_LEFT 25
  LEG_RIGHT 25

### Efekty uszkodzenia części
HEAD
  ON_DAMAGE stagger        # zatacza się
  ON_50HP blur_vision      # zamazany wzrok
  ON_DEATH instant_kill    # śmierć natychmiastowa

TORSO
  ON_DAMAGE slow_move      # wolniejszy ruch
  ON_50HP heavy_breathing  # ciężki oddech

ARM_LEFT
  ON_DEATH drop_weapon     # upuszcza broń
  ON_DAMAGE aim_penalty    # gorsze celowanie

LEG_LEFT
  ON_DEATH limp            # kuleje
  ON_DAMAGE slow_move      # spowalnia

### Przykłady

# Prosty wróg - tylko globalne
HUMANOID grunt
HP 50

# Średni wróg - globalne + kluczowe części
HUMANOID soldier  
HP 150
HP_PARTS
  HEAD 40
  LEGS 60

# Boss - pełny system
GIANT kaiju
HP 5000
HP_PARTS
  HEAD 800     # słaby punkt
  TORSO 2000
  ARM_LEFT 600
  ARM_RIGHT 600
  LEG_LEFT 500
  LEG_RIGHT 500
HP_PHASES
  PHASE 2 hp=3000   # zmienia zachowanie przy 3000 HP
  PHASE 3 hp=1000   # staje się agresywniejszy

### Flagi
WEAK_SPOT HEAD multiplier=3.0   # headshot = 3x obrażenia
ARMOR TORSO reduction=0.5       # pancerz redukuje 50% dmg
REGENERATE rate=5 delay=3.0     # regeneruje 5HP/s po 3s

## System broni (WEAPON)

### Zmienne dostępne - używasz tych które chcesz
DAMAGE          - obrażenia bazowe
DAMAGE_PARTS    - obrażenia per część ciała
RANGE           - zasięg
FIRE_RATE       - szybkość strzału (strzały/sekundę)
RELOAD_TIME     - czas przeładowania
AMMO_MAX        - max amunicja w magazynku
AMMO_RESERVE    - zapas amunicji
SPREAD          - rozrzut
RECOIL          - odrzut
BULLET_DROP     - opad pocisku (opcjonalne)
BULLET_SPEED    - prędkość pocisku
PENETRATION     - przebicie przez ściany/wrogów
NOISE           - hałas przy strzale
AUTO_FIRE       - automatyczny ogień yes/no
BURST           - seria (ile strzałów)
ZOOM            - przybliżenie (celownik)
WEIGHT          - waga (wpływa na ruch gracza)

### Przykłady

# Prosty pistolet - minimum zmiennych
WEAPON pistol
DAMAGE 25
AMMO_MAX 15

# Karabin - więcej parametrów
WEAPON rifle
DAMAGE 40
RANGE 50.0
FIRE_RATE 0.1
RELOAD_TIME 2.5
AMMO_MAX 30
AMMO_RESERVE 90
SPREAD 0.02
RECOIL 0.3
AUTO_FIRE yes

# Snajperka - z opadem
WEAPON sniper
DAMAGE 150
RANGE 200.0
FIRE_RATE 1.5
RELOAD_TIME 3.0
AMMO_MAX 5
AMMO_RESERVE 20
BULLET_DROP yes gravity=9.8
BULLET_SPEED 900.0
ZOOM 4.0
NOISE 0.1
WEIGHT 5.0

# Shotgun - obrażenia per część
WEAPON shotgun
DAMAGE 15
DAMAGE_PARTS
  HEAD 40
  TORSO 15
  LEGS 8
SPREAD 0.15
AMMO_MAX 8
BURST 8
RANGE 10.0

# Broń cicha
WEAPON silenced_pistol
DAMAGE 20
NOISE 0.05
AMMO_MAX 12

### Typy broni
MELEE     - broń biała (nóż, miecz, pięści)
PISTOL    - pistolet
SHOTGUN   - strzelba
RIFLE     - karabin
SNIPER    - snajperka
EXPLOSIVE - wybuchowa (granat, rocket launcher)
ENERGY    - energetyczna (laser, plasma)
CUSTOM    - własny typ

## Format mapy (MAP)

### Trzy tryby - używasz tego który chcesz

# TRYB 1 - prosta siatka
MAP level1
GRID
W W W W W W
W P . . . W
W . B . E W
W . . . . W
W W W W W W

Symbole:
W = ściana
. = podłoga
P = spawn gracza
E = spawn wroga
B = prop (barrel domyślnie)
D = drzwi
L = lampa
# = komentarz

# TRYB 2 - proceduralna
MAP level1
GENERATE
  SIZE 50x50
  ROOMS 8
  CORRIDORS yes
  ENEMY_COUNT 10
  PROP_COUNT 20
  SEED 42

# TRYB 3 - hybrydowa
MAP level1
# Ręczna część główna
GRID
W W W W W
W P . . W
W W D W W
W . . . W
W W W W W

# Proceduralna strefa
ZONE zone2
  GENERATE size=30x30 rooms=4 seed=7

# Obiekty na mapie
PLACE enemy gangster 5.0 0.0 3.0
PLACE prop barrel 2.0 0.0 2.0
PLACE light 4.0 1.0 4.0 color=#FF9932
PLACE weapon pistol 3.0 0.0 1.0
PLACE vehicle car 8.0 0.0 5.0

# Triggery i eventy
TRIGGER zone1_clear
  CONDITION all_enemies_dead zone1
  ACTION unlock_door door_zone2
  ACTION play_sound checkpoint.ogg
  ACTION show_message "Strefa 1 oczyszczona!"

# Oświetlenie mapy
LIGHT_AMBIENT 0.1
LIGHT_POINT 5.0 1.0 3.0 radius=4.0 color=#FF9932

# Muzyka
MUSIC dark_ambient.ogg loop=yes volume=0.5

# Właściwości mapy
MAP_PROPS
  GRAVITY 9.8
  FOG yes color=#000000 density=0.3
  SKYBOX none
  FLOOR_TEXTURE stone
  CEILING_TEXTURE concrete
  WALL_TEXTURE brick

## Triggery i Eventy

### Prosty trigger w .aie
Używaj gdy logika jest prosta - jeden warunek, jedna akcja
TRIGGER nazwa
  CONDITION warunek
  ACTION akcja

### Dostępne warunki (CONDITION)
all_enemies_dead zona     - wszyscy wrogowie martwi
player_enters zona        - gracz wchodzi do strefy
player_exits zona         - gracz wychodzi ze strefy
item_picked id            - gracz podniósł przedmiot
hp_below wartość          - HP gracza poniżej wartości
timer_elapsed sekundy     - upłynął czas
button_pressed id         - gracz nacisnął przycisk
enemy_dead id             - konkretny wróg martwy

### Dostępne akcje (ACTION)
unlock_door id            - otwiera drzwi
lock_door id              - zamyka drzwi
spawn_enemy typ punkt     - spawnuje wroga
spawn_prop typ pozycja    - spawnuje obiekt
play_sound plik           - odgrywa dźwięk
play_music plik           - odgrywa muzykę
stop_music                - zatrzymuje muzykę
show_message tekst        - pokazuje wiadomość
hide_message              - ukrywa wiadomość
start_lua funkcja         - wywołuje funkcję Lua
set_var nazwa wartość     - ustawia zmienną
win_game                  - koniec gry - wygrana
lose_game                 - koniec gry - przegrana
teleport_player punkt     - teleportuje gracza
earthquake siła czas      - trzęsienie ziemi
change_music plik         - zmienia muzykę

### Przykład prostego triggera w .aie
TRIGGER zone1_clear
  CONDITION all_enemies_dead zone1
  ACTION unlock_door door_zone2
  ACTION play_sound checkpoint.ogg
  ACTION show_message "Strefa 1 oczyszczona!"

TRIGGER player_low_hp
  CONDITION hp_below 20
  ACTION play_sound heartbeat.ogg
  ACTION show_message "Ostrzeżenie: niskie HP!"

### Złożona logika w Lua
-- Używaj gdy potrzebujesz warunków, pętli, zmiennych
-- Lua ma pełny dostęp do silnika

function onZone1Clear()
  -- złożona logika
  if getVar("difficulty") == "hard" then
    spawnEnemy("boss", "boss_spawn")
    showMessage("Boss przybywa!")
  else
    unlockDoor("door_zone2")
    showMessage("Strefa oczyszczona!")
  end
end

function onPlayerDamage(amount, part)
  -- reakcja na obrażenia per część ciała
  if part == "HEAD" then
    addEffect("blur_vision", 2.0)
  elseif part == "LEG_LEFT" or part == "LEG_RIGHT" then
    setPlayerSpeed(0.5)
  end
end

function onEnemyDead(id, type)
  -- licznik zabić
  kills = getVar("kills") + 1
  setVar("kills", kills)
  if kills >= 10 then
    triggerEvent("wave_clear")
  end
end

### Łączenie .aie z Lua
# W .aie wywołujesz Lua gdy potrzebujesz złożonej logiki
TRIGGER boss_spawn
  CONDITION all_enemies_dead zone2
  ACTION start_lua onBossSpawn    # wywołuje funkcję Lua

# W Lua.lua
function onBossSpawn()
  -- złożona logika bossa
  spawnEnemy("kaiju", "boss_spawn")
  changeMusic("boss_theme.ogg")
  showMessage("BOSS!")
  startCutscene("boss_intro")
end

## Cutscenki (CUTSCENE)

### POZIOM 1 - prosty tekst
CUTSCENE intro
  TEXT "Miasto opanowane przez gangi..."
  TEXT "Twoja misja: oczyścić ulice."
  NEXT_ON_KEY yes

### POZIOM 2 - tekst z obrazem
CUTSCENE intro
  IMAGE title_screen.png
  TEXT "Miasto opanowane przez gangi..."
  MUSIC dark_ambient.ogg
  DURATION 5.0

### POZIOM 3 - silnikowe animacje
CUTSCENE boss_intro
  CAMERA_MOVE 0.0 2.0 -5.0 duration=3.0
  SPAWN kaiju boss_spawn
  ANIMATE kaiju roar duration=2.0
  SHAKE_CAMERA intensity=0.5 duration=1.0
  TEXT "BOSS PRZYBYWA!"
  PLAY_SOUND roar.ogg

### POZIOM 4 - pełna kontrola przez Lua
CUTSCENE final_boss
  LUA onFinalBossCutscene

-- final_boss.lua
function onFinalBossCutscene()
  freezePlayer()
  moveCamera(0.0, 5.0, -10.0, 3.0)
  spawnEnemy("kaiju", "boss_spawn")
  playAnimation("kaiju", "roar")
  shakeCamera(0.5, 2.0)
  showMessage("TO JEST KONIEC!")
  wait(3.0)
  unfreezePlayer()
  startCombat()
end

### Dostępne komendy cutscenki
CAMERA_MOVE x y z duration   - przesuń kamerę
CAMERA_LOOK x y z duration   - obróć kamerę
SPAWN typ punkt               - spawnuj obiekt
ANIMATE id animacja           - odtwórz animację
SHAKE_CAMERA siła czas        - trzęsienie kamery
FREEZE_PLAYER                 - zablokuj gracza
UNFREEZE_PLAYER               - odblokuj gracza
TEXT tekst                    - pokaż tekst
IMAGE plik                    - pokaż obraz
PLAY_SOUND plik               - odtwórz dźwięk
MUSIC plik                    - odtwórz muzykę
WAIT sekundy                  - czekaj
FADE_IN czas                  - rozjaśnij ekran
FADE_OUT czas                 - przyciemnij ekran
NEXT_ON_KEY yes               - czekaj na klawisz
LUA funkcja                   - wywołaj Lua

## Lua API

### Wbudowane funkcje silnika (zawsze dostępne)

#### Wrogowie
spawnEnemy(typ, punkt)
killEnemy(id)
getEnemyHP(id)
setEnemyHP(id, wartość)
allEnemiesDead(zona)
getEnemyCount(zona)
moveEnemy(id, x, y, z)
freezeEnemy(id)
unfreezeEnemy(id)

#### Gracz
getPlayerHP()
setPlayerHP(wartość)
getPlayerPosition()
setPlayerPosition(x, y, z)
getPlayerAmmo(broń)
setPlayerAmmo(broń, wartość)
giveWeapon(typ)
removeWeapon(typ)
freezePlayer()
unfreezePlayer()
setPlayerSpeed(wartość)

#### Mapa
unlockDoor(id)
lockDoor(id)
openDoor(id)
closeDoor(id)
spawnProp(typ, x, y, z)
removeProp(id)
changeTexture(id, tekstura)

#### Dźwięk i muzyka
playSound(plik)
stopSound(plik)
playMusic(plik)
stopMusic()
changeMusic(plik)
setVolume(wartość)

#### UI i komunikaty
showMessage(tekst)
hideMessage()
showImage(plik)
hideImage()
setHUDVisible(yes/no)

#### Zmienne i stan gry
getVar(nazwa)
setVar(nazwa, wartość)
startTimer(nazwa, sekundy)
stopTimer(nazwa)
getTime()

#### Kamera
moveCamera(x, y, z, czas)
shakeCamera(siła, czas)
lookAt(x, y, z, czas)

#### Eventy
registerEvent(nazwa, funkcja)
triggerEvent(nazwa)
wait(sekundy)

#### Gra
winGame()
loseGame()
loadMap(plik)
restartMap()
exitGame()

### Własne funkcje - piszesz sam
-- Rejestrujesz własne funkcje jako eventy
function onGameStart()
  -- twoja logika startu
end
registerEvent("onStart", onGameStart)

function onEnemyDead(id, typ)
  -- twoja logika śmierci wroga
end
registerEvent("onEnemyDead", onEnemyDead)

-- Dostępne eventy silnika
onStart          - start mapy
onPlayerDead     - śmierć gracza
onPlayerDamage   - obrażenia gracza
onEnemyDead      - śmierć wroga
onEnemySpawn     - spawn wroga
onItemPickup     - podniesienie przedmiotu
onDoorOpen       - otwarcie drzwi
onZoneEnter      - wejście do strefy
onZoneExit       - wyjście ze strefy
onTimer          - upłynął timer
onWin            - wygrana
onLose           - przegrana

## JS API

### Dwie role JS w AIEngine

#### Rola 1 - UI (główna)
Kontroluje wszystkie elementy wizualne HUD i menu.

#### Rola 2 - Logika (opcjonalna)
Może zastąpić Lua jeśli wolisz JS do logiki misji.
Albo współpracować z Lua.

### Podział zależy od ciebie:
# Opcja A - JS tylko UI, Lua logika
# Opcja B - JS wszystko (UI + logika)
# Opcja C - JS UI + logika, Lua pomocniczo
# Opcja D - mieszaj jak chcesz

### JS API - UI

#### Elementy HUD
setHP(wartość, max)
setAmmo(obecna, zapas)
setBulletTime(wartość, max)
setCrosshair(rozmiar)
setMessage(tekst)
hideMessage()
showElement(id)
hideElement(id)
setFilter(nazwa, intensywność)

#### Filtry ekranu
setFilter("chromatic_aberration", 0.8)
setFilter("vignette", 0.6)
setFilter("blur", 0.3)
setFilter("red_flash", 0.5)
setFilter("noise", 0.2)

#### Menu
showMenu(typ)
hideMenu()
showDeathScreen()
showWinScreen()
showPauseMenu()
showMainMenu()
showInventory()
showObjectives()

#### Minimap
setMinimapVisible(yes/no)
updateMinimap(dane)
addMinimapMarker(x, y, typ)
removeMinimapMarker(id)

#### Dialog NPC
showDialog(postać, tekst)
hideDialog()
addDialogOption(tekst, funkcja)

### JS API - Logika (opcjonalna)

#### Dostęp do silnika z JS
Engine.spawnEnemy(typ, punkt)
Engine.killEnemy(id)
Engine.getPlayerHP()
Engine.setPlayerHP(wartość)
Engine.unlockDoor(id)
Engine.playSound(plik)
Engine.getVar(nazwa)
Engine.setVar(nazwa, wartość)
Engine.triggerEvent(nazwa)

#### Eventy w JS
Engine.on("onStart", function() {
    showMessage("Gra się zaczyna!")
})

Engine.on("onPlayerDamage", function(amount, part) {
    setFilter("red_flash", 0.5)
    setTimeout(() => setFilter("red_flash", 0), 100)
})

Engine.on("onEnemyDead", function(id, typ) {
    kills++
    updateKillCounter(kills)
})

### Przykład - JS robi wszystko
// missions/level1.js
let kills = 0
let zone = 1

Engine.on("onStart", function() {
    UI.showMessage("Oczyść wszystkie strefy!")
    Engine.spawnEnemy("gangster", "zone1_spawn1")
})

Engine.on("onEnemyDead", function(id, typ) {
    kills++
    if (Engine.allEnemiesDead("zone" + zone)) {
        zone++
        if (zone > 3) {
            UI.showWinScreen()
        } else {
            Engine.unlockDoor("door_zone" + zone)
            UI.showMessage("Strefa " + (zone-1) + " oczyszczona!")
        }
    }
})

### Przykład - JS tylko UI, Lua logika
// ui/hud.js - tylko wizualia
Engine.on("onPlayerDamage", function(amount) {
    setFilter("red_flash", 0.8)
    shakeElement("crosshair", 0.2)
})

-- missions/level1.lua - logika
function onEnemyDead(id)
    if allEnemiesDead("zone1") then
        unlockDoor("door_zone2")
    end
end

## CSS - Pełna specyfikacja

### Kolory i tekstury obiektów
.gangster {
    color: #4A4A4A;
    texture: cloth;
    texture-scale: 1.0;
    shine: 0.2;
    roughness: 0.8;
}

.gangster .head {
    color: #D2946A;
    texture: skin;
}

### Dziedziczenie
.boss-gangster extends .gangster {
    color: #8B0000;
    size-multiplier: 1.5;
    shine: 0.9;
}

### Stany obiektów
.gangster:idle {
    animation-speed: 1.0;
}

.gangster:walk {
    animation-speed: 1.5;
}

.gangster:pain {
    color-flash: #FF0000;
    flash-duration: 0.1;
}

.gangster:dead {
    color-darken: 0.5;
    ragdoll: true;
}

### Tekstury proceduralne
.wall {
    texture: noise(perlin, scale=2.0, seed=42);
    color: #888888;
}

.floor {
    texture: tiles(size=0.5, grout-color=#333333);
    color: #AAAAAA;
}

.metal {
    texture: noise(simplex, scale=0.5, seed=7);
    shine: 0.9;
    color: #AAAAAA;
}

### Oświetlenie przez CSS
.level-dark {
    ambient: 0.05;
    fog-color: #000000;
    fog-density: 0.8;
}

.level-outdoor {
    ambient: 0.8;
    sun-color: #FFF5E0;
    sun-angle: 45;
}

.level-sunset {
    ambient: 0.4;
    sun-color: #FF6B35;
    sun-angle: 10;
    fog-color: #FF8C42;
    fog-density: 0.2;
}

### Efekty ekranu globalne
screen {
    vignette: 0.3;
    film-grain: 0.1;
    color-grading: warm;
    contrast: 1.1;
    saturation: 0.9;
    brightness: 1.0;
}

### Efekty per stan gracza
player:normal {
    vignette: 0.2;
    film-grain: 0.05;
}

player:low-hp {
    vignette: 0.8;
    red-tint: 0.3;
    blur: 0.2;
    pulse: yes;
    pulse-speed: 1.5;
}

player:dead {
    desaturate: 1.0;
    blur: 0.8;
    darken: 0.5;
}

player:bullet-time {
    chromatic-aberration: 0.8;
    vignette: 0.6;
    saturation: 1.3;
    time-blur: 0.3;
}

player:underwater {
    blur: 0.3;
    blue-tint: 0.4;
    distortion: 0.2;
    vignette: 0.5;
}

### Dostępne właściwości CSS

#### Kolor i tekstura
color: #RRGGBB
texture: nazwa lub noise(typ, scale, seed)
texture-scale: liczba
shine: 0.0-1.0
roughness: 0.0-1.0
opacity: 0.0-1.0
color-flash: #RRGGBB
flash-duration: sekundy
color-darken: 0.0-1.0
size-multiplier: liczba

#### Oświetlenie sceny
ambient: 0.0-1.0
sun-color: #RRGGBB
sun-angle: stopnie
fog-color: #RRGGBB
fog-density: 0.0-1.0

#### Efekty ekranu
vignette: 0.0-1.0
film-grain: 0.0-1.0
blur: 0.0-1.0
chromatic-aberration: 0.0-1.0
red-tint: 0.0-1.0
blue-tint: 0.0-1.0
desaturate: 0.0-1.0
contrast: liczba
saturation: liczba
brightness: liczba
distortion: 0.0-1.0
time-blur: 0.0-1.0
pulse: yes/no
pulse-speed: liczba
color-grading: warm/cold/noir/sepia/custom

#### Animacje
animation-speed: liczba
ragdoll: yes/no

## Przykłady kompletne

### Przykład 1 - Prosty wróg (minimum)
# entities/grunt.aie
HUMANOID grunt
HP 50
HEAD small skin
TORSO medium grey_shirt
LEGS medium black_pants

# styles/grunt.css
.grunt {
    color: #666666;
    texture: cloth;
}

# scripts/grunt.lua
function onDead(id)
    playSound("death.ogg")
end
registerEvent("onEnemyDead", onDead)

---

### Przykład 2 - Złożony boss
# entities/boss_kaiju.aie
GIANT kaiju
HP 5000
HP_PARTS
  HEAD 800
  TORSO 2000
  ARM_LEFT 600
  ARM_RIGHT 600
  LEG_LEFT 500
  LEG_RIGHT 500
HP_PHASES
  PHASE 2 hp=3000
  PHASE 3 hp=1000
WEAK_SPOT HEAD multiplier=3.0
ARMOR TORSO reduction=0.5
BEHAVIOR
  COLLAPSE yes
  SHATTER yes parts=8
  EARTHQUAKE yes intensity=0.8

BODY huge dark_scales
HEAD huge dark_scales
ARMS 2 huge dark_scales
LEGS 2 huge dark_scales
TAIL huge dark_scales

ANIM idle
  FRAME 0
    HEAD ROT 0 5 0
  FRAME 30
    HEAD ROT 0 -5 0
  FRAME 60
    HEAD ROT 0 5 0
END

ANIM roar
  FRAME 0
    HEAD ROT -20 0 0
  FRAME 10
    HEAD ROT 20 0 0
  FRAME 20
    HEAD ROT 0 0 0
END

ANIM phase2
  FRAME 0
    BODY SHAKE 0.5
  FRAME 5
    BODY SHAKE 0.0
END

# styles/boss_kaiju.css
.kaiju {
    color: #1A1A2E;
    texture: noise(perlin, scale=3.0, seed=666)
    shine: 0.7;
    roughness: 0.3;
}

.kaiju:pain {
    color-flash: #FF0000;
    flash-duration: 0.2;
}

.kaiju:phase2 {
    color: #4A0000;
    shine: 0.9;
    red-tint: 0.3;
}

.kaiju:dead {
    color-darken: 0.8;
    collapse: yes;
    shatter: yes;
}

# scripts/boss_kaiju.lua
local phase = 1

function onBossSpawn(id)
    changeMusic("boss_theme.ogg")
    showMessage("KAIJU PRZYBYWA!")
    shakeCamera(0.8, 2.0)
end

function onBossDamage(id, amount, part)
    if part == "HEAD" then
        playSound("head_hit.ogg")
        shakeCamera(0.3, 0.5)
    end
    
    local hp = getEnemyHP(id)
    
    if hp <= 3000 and phase == 1 then
        phase = 2
        playAnimation(id, "roar")
        showMessage("FAZA 2!")
        changeMusic("boss_theme_intense.ogg")
        setEnemySpeed(id, 1.5)
    end
    
    if hp <= 1000 and phase == 2 then
        phase = 3
        showMessage("FAZA OSTATECZNA!")
        setEnemyDamage(id, 2.0)
        spawnEnemy("grunt", "spawn_1")
        spawnEnemy("grunt", "spawn_2")
    end
end

function onBossDead(id)
    stopMusic()
    playSound("victory.ogg")
    shakeCamera(1.0, 3.0)
    showMessage("ZWYCIĘSTWO!")
    wait(3.0)
    winGame()
end

registerEvent("onEnemySpawn", onBossSpawn)
registerEvent("onEnemyDamage", onBossDamage)
registerEvent("onEnemyDead", onBossDead)

---

### Przykład 3 - Prosta mapa z triggerami
# maps/level1.aie
MAP level1
MUSIC dark_ambient.ogg loop=yes
LIGHT_AMBIENT 0.1
FOG yes color=#000000 density=0.2

GRID
W W W W W W W W W W
W P . . . W . . . W
W . B . . D . E . W
W . . . . W . . . W
W W W W W W W W W W

# Obiekty
PLACE enemy grunt zone1_spawn1 5.0 0.0 2.0
PLACE enemy grunt zone1_spawn2 5.0 0.0 3.0
PLACE enemy boss_kaiju boss_spawn 8.0 0.0 2.0
PLACE prop barrel 3.0 0.0 2.0
PLACE weapon pistol 2.0 0.0 1.0
PLACE light 5.0 1.0 2.0 color=#FF9932 radius=4.0

# Triggery
TRIGGER zone1_clear
  CONDITION all_enemies_dead zone1
  ACTION unlock_door door_zone2
  ACTION play_sound checkpoint.ogg
  ACTION show_message "Strefa 1 oczyszczona!"
  ACTION start_lua onZone1Clear

TRIGGER boss_trigger
  CONDITION player_enters boss_zone
  ACTION start_lua onBossSpawn
  ACTION change_music boss_theme.ogg

TRIGGER low_hp_warning
  CONDITION hp_below 20
  ACTION play_sound heartbeat.ogg
  ACTION show_message "Ostrzeżenie: niskie HP!"

# styles/level1.css
screen {
    vignette: 0.3;
    film-grain: 0.1;
    color-grading: noir;
}

player:low-hp {
    vignette: 0.8;
    red-tint: 0.3;
    pulse: yes;
    pulse-speed: 1.5;
}

.wall {
    texture: brick;
    color: #444444;
}

.floor {
    texture: tiles(size=0.5, grout-color=#222222);
    color: #333333;
}

# scripts/level1.lua
function onZone1Clear()
    showMessage("Znajdź bossa!")
    addMinimapMarker(8.0, 2.0, "boss")
end

function onBossSpawn()
    freezePlayer()
    moveCamera(8.0, 2.0, 0.0, 2.0)
    wait(2.0)
    unfreezePlayer()
end

registerEvent("onZone1Clear", onZone1Clear)
registerEvent("onBossSpawn", onBossSpawn)


## Architektura techniczna

### Jak działa AIEngine
Natywna aplikacja Windows (.exe)
Zero przeglądarki, zero zewnętrznych runtime'ów.

### Stack wewnętrzny
Go core
├── GLFW    - natywne okno Windows
├── OpenGL  - renderer raycasting
├── goja    - JS engine (wbudowany w exe)
├── gopher-lua - Lua engine (wbudowany w exe)
└── CSS parser - własny (wbudowany w exe)

### Co robi każdy język
Go  - silnik, renderer, fizyka, parser .aie
Lua - logika misji, AI wrogów, eventy
JS  - UI, HUD, menu, opcjonalnie logika
CSS - kolory, tekstury, efekty ekranu

### Instalacja dla gracza
Jeden plik: aiengine.exe
Zero instalowania Go, Lua, JS, CSS
Wszystko wbudowane w exe

### Instalacja dla dewelopera
1. Pobierz Go z golang.org
2. git clone aiengine
3. go mod tidy
4. go build -o aiengine.exe
5. Gotowe


