# Runtime poziomów (`RUNTIME level`)

Projekt z linią `RUNTIME level` w `manifest.aie` jest uruchamiany z danych:
mapa, obiekty, triggery i dźwięki pochodzą z `MAIN_MAP`, logika z
`MAIN_SCRIPT` (Lua), opcjonalny HUD z `HUD_SCRIPT` (JS). Projekty bez tej
linii (np. `demo`) działają jak dotąd (wbudowana piaskownica).
Przykład: `projects/krypta`.

## Mapa

```
MAP nazwa
PLAYER_ANGLE 15          # kierunek startowy w stopniach (0 = +x, 90 = +z)
PLAYER_HP 100
FLOOR_COLOR #1E1A24
CEILING_COLOR #0C0B14

GRID
W W W W
W P . W
W W W W
```

Symbole `GRID` (paleta edytora): `.` podłoga, `W` ściana 1 (brick),
`S` ściana 2 (stone), `M` ściana 3 (metal), `D` drzwi (wood, zamknięte),
`P` start gracza, `E` wróg `grunt`, `B` wróg `boss`. Inne pojedyncze znaki
to podłoga zapisana w `Level.Markers`.

Tekstury ścian można podmienić plikami
`assets/textures/{brick,stone,metal,wood}.png` (kwadrat, bok = potęga 2).

### Wysokości podłogi i sufitu (opcjonalne)

Każda komórka może mieć własną wysokość podłogi i sufitu: schody, podesty,
balkony, przepaście, niskie tunele, wysokie sale. Mapy bez tych bloków
działają jak dawniej (podłoga 0, sufit 1, oko 0.5 nad podłogą).

```
HEIGHT_STEP 0.125      # jednostka wysokości w wysokościach ściany (domyślnie 1/8)
CEILING_HEIGHT 1.25    # domyślny sufit (wysokości ściany, domyślnie 1)
STEP_WALL S            # tekstura ścianek stopni: W, S, M lub D (domyślnie S)
FALL_DAMAGE 40         # HP za każdą jednostkę ściany upadku ponad FALL_SAFE
FALL_SAFE 0.6          # bezpieczna wysokość upadku (domyślnie 0.6)

FLOOR_GRID             # ten sam układ co GRID, jeden znak na komórkę
. . 2 4 6 8 .          # . = 0, 0-9 = 0..9 jednostek, A-Z = 10..35
. . . . . h .          # a-z = -1..-26 (doły), tu h = -8 = -1.0
CEILING_GRID           # wysokość sufitu bezwzględnie; . = CEILING_HEIGHT
. K K K K . .          # K = 20 jednostek = 2.5
```

Zasady ruchu (gracz i wrogowie): wejście na stopień do 0.3 (2 jednostki)
bez skoku, wyżej tylko skokiem (szczyt skoku 0.45), zejście z dowolnej
wysokości (upadek, z obrażeniami przy `FALL_DAMAGE`, zdarzenie
`onPlayerFall(wysokość, obrażenia)`). Potrzebne miejsce pod sufitem: 0.6
w pionie, 0.42 w kucki, 0.25 leżąc; pod niskim sufitem gracz nie wstanie.
Wrogowie szukają drogi A* po stopniach, widzą gracza tylko przy czystej
linii na wysokości oczu i nie zadają ciosów przez krawędź wyższą niż 0.45.
Sprite'y stoją na podłodze swojej komórki (`PLACE ... y` to wysokość nad
nią). Strzał wymaga czystej linii 3D (nie przez krawędź półki).

Przykład: `maps/testy/wysokosci.aie` (z konsoli: `map maps/testy/wysokosci.aie`).

### Oświetlenie (opcjonalne)

Światło jest liczone na komórkę (jak światło sektora w klasycznych silnikach
2.5D): ściany i ścianki stopni biorą światło komórki przed nimi, podłoga,
sufit i sprite'y - komórki, w której są. Na to nakłada się zaciemnienie
z odległością i mgła. Mapy bez tych linii wyglądają jak dawniej.

```
LIGHT_AMBIENT 0.35                    # domyślne światło komórki (1 = pełne)
FOG yes color=#06050A density=0.12    # mgła: udział 1-exp(-density*odległość)
LIGHT_GRID                            # jeden znak na komórkę GRID
. 6 7 9 9 .                           # . = LIGHT_AMBIENT, 0-9 = 0..1 (n/9)

PLACE light flicker pochodnia 1.5 0.7 7.5 radius=3.5 level=1.1 speed=9 min=0.3
PLACE light pulse otchlan 14.5 0 3.5 radius=3 level=0.9 speed=0.5 min=0.1
PLACE light strobe alarm 15.5 0 8.5 radius=1.5 level=1 speed=2 min=0
LIGHT_POINT 5.0 1.0 3.0 radius=4.0    # stałe światło (starsza składnia)
PLACE item crystal k1 5.5 0 1.5 glow=0.6 glow_radius=2.5   # świecący przedmiot
```

Światło dodaje `level * (1 - odległość/radius)` komórkom, które widzi (nie
przechodzi przez ściany). Efekty: `steady`, `flicker` (losowe przygaśnięcia
do `min`), `pulse` (płynnie między `min` a 1), `strobe` (wł./wył.), `off`;
`speed` = zmiany na sekundę. Strzał rozjaśnia na chwilę otoczenie gracza.
Przedmiot z `glow=` świeci, dopóki nie zostanie podniesiony.

Skrypty (Lua, w JS jako `engine.*`): `setLight(id, poziom)`,
`setLightEffect(id, "flicker"|"pulse"|"strobe"|"steady"|"off")`,
`setCellLight(x, y, poziom)` (np. własne pulsowanie w `onUpdate`).
Przykład: `maps/testy/swiatlo.aie`.
Edytor zachowuje bloki `FLOOR_GRID`/`CEILING_GRID` przy zapisie (malowanie
wysokości w edytorze jeszcze nie istnieje).

## PLACE

```
PLACE <rodzaj> [typ] [id] <x> <y> <z> [klucz=wartość ...]
```

`x` = kolumna, `z` = wiersz siatki (środek pola = n.5), `y` = wysokość.
Bez `id` silnik nada własne (`item_crystal_1`).

| rodzaj | znaczenie | parametry |
|---|---|---|
| `item` | znajdźka, zbierana po wejściu | `radius` (0.5), `sound`, `sprite` (domyślnie = typ), `scale`, `bob` |
| `zone` | strefa wejścia/wyjścia | `radius` (0.5), `sprite`, `scale` |
| `enemy` | wróg (atlas `ENEMY_ATLAS`) | `hp`, `speed`, `damage`, `range`, `sight`, `scale` |
| `door` | `PLACE door <id> x y z` – drzwi otwierane `unlockDoor(id)` | |
| `prop` | dekoracja z obrazkiem `assets/sprites/<typ>.png` | `sprite`, `scale` |

## Zdarzenia i API skryptów

Zdarzenia (Lua `registerEvent`, JS `Engine.on`): `onStart`,
`onUpdate(dt)`, `onItemPickup(id, typ)`, `onZoneEnter(id, typ)`,
`onZoneExit(id, typ)`, `onPlayerDamage(ile)`, `onPlayerDead`,
`onEnemyDead(id)`, `onWin`, `onLose`, `onDoorOpen(id)`.

Nowe/naprawione funkcje: `setHUDText(klucz, tekst)` (linie HUD sortowane po
kluczu), `showMessage(tekst, sekundy)` (domyślnie 4 s, 0 = na stałe),
`setFilter(nazwa, wartość)` (`red_flash`, `vignette`, `desaturate`,
`brightness`), `winGame(tekst)` / `loseGame(tekst)` (pierwsza linia to
tytuł ekranu końca, reszta to podtytuł), `restartMap()`, `exitGame()`,
`unlockDoor(id)` / `lockDoor(id)`. W JS działają `setTimeout`/`setInterval`.

Triggery w mapie mają działające warunki `item_picked`, `player_enters`,
`player_exits`, `hp_below`, `all_enemies_dead`, `enemy_dead` oraz nowe
`var_equals <nazwa> <wartość>` i `var_at_least <nazwa> <liczba>`
(zmienne z `setVar`).

Ekran końca gry: restart klawiszem z `BIND restart ...` (domyślnie Enter),
podpowiedź z `RESTART_HINT` w manifeście.

## Uruchamianie automatyczne

```
aiengine.exe --project <nazwa> [--screenshot plik.png] [--frames N] [--exec "komenda; komenda"]
```

`--screenshot` zapisuje klatkę N (domyślnie 60) i zamyka grę, `--frames`
sam w sobie kończy po N klatkach, `--exec` wykonuje komendy konsoli po
starcie (`tp x y [kąt]`, `god`, `fps`, `lua <kod>`, `restart`,
`map <plik.aie>` - dowolna mapa projektu...).
Przy wyjściu silnik loguje średnie FPS i zużycie pamięci (`[Perf]`).
