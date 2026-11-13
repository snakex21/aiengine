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

### Mapy sektorowe (`SECTOR` zamiast `GRID`)

Zamiast siatki mapa może składać się z sektorów: wielokątów z własną
wysokością podłogi i sufitu, światłem i teksturami. Ściany biegną pod
dowolnym kątem. Silnik sam wybiera renderer: mapa z `SECTOR` idzie przez
renderer sektorowy, mapa z `GRID` przez dotychczasowy raycaster.

```
MAP sanktuarium
PLAYER_START 12 20            # start gracza (x z) - wymagany
PLAYER_ANGLE -90
FLOOR_TEX floor               # domyślne tekstury podłogi/sufitu sektorów
CEILING_TEX ceiling
LIGHT_AMBIENT 0.55            # światło sektorów bez light=
SECTOR przedsionek floor=0 ceil=1.5 light=0.5 wall=W : 10 17 14 17 15 19 14 21 10 21 9 19
SECTOR oltarz floor=0.25 ceil=2.25 step=M : 11 10 13 10 14 11 14 13 13 14 11 14 10 13 10 11
```

Po dwukropku są punkty obrysu (x z, co najmniej 3, w dowolnym kierunku).
Krawędź o tych samych dwóch końcach w dwóch sektorach to przejście (portal),
pozostałe to ściany. Sektory muszą stykać się wspólnymi wierzchołkami,
bez „T" w połowie krawędzi (błąd przy wczytaniu). Klucze: `floor`, `ceil`,
`light`, `wall` (tekstura ścian: W, S, M, D albo nazwa tekstury), `edges`
(tekstura każdej krawędzi po kolei, `.` = `wall`), `step` (ścianki stopni
prowadzących do tego sektora), `floor_tex`, `ceil_tex`, `tag`. Współrzędne
nie mogą być ujemne. `PLACE`, `TRIGGER`, `SOUND` i światła działają jak na
siatce. Kolizje, stopnie, upadki i linia wzroku liczą się na liniach
sektorów; wrogowie szukają drogi po siatce 1×1 próbkowanej z sektorów.
Drzwi (`PLACE door`) na mapie sektorowej zawsze się przesuwają: animują
sektor pod podanym punktem (patrz „Animowane sektory").

Konwersja mapy GRID na sektorową (każda wolna kratka staje się sektorem):

```
aiengine --grid2sectors maps/krypta.aie maps/krypta_sektory.aie
```

Przykład: poziom `maps/sanktuarium.aie` kampanii Krypty.
Edytor zachowuje bloki `FLOOR_GRID`/`CEILING_GRID`, `TAG_GRID`,
`*_TEX_GRID` oraz linie `MOVER`/`TEXTURE_ANIM` przy zapisie (malowanie
wysokości i tagów w edytorze jeszcze nie istnieje).

### Animowane sektory: drzwi, windy, zgniatacze, ruchome podłogi

„Mover" przesuwa podłogę albo sufit wszystkich kawałków mapy z danym tagiem:
komórek zaznaczonych w `TAG_GRID` (mapa GRID) albo sektorów z `tag=` (mapa
sektorowa). Wysokości to zwykłe pola mapy, więc renderer, kolizje, linia
wzroku i A* wrogów od razu widzą zmianę (na mapie sektorowej odświeżane są
tylko komórki siatki nawigacji leżące w ruchomych sektorach).

```
TAGS w=winda c=pulapka        # opcjonalne nazwy symboli (bez tego tag = symbol)
TAG_GRID                      # jeden znak na komórkę GRID, . = brak tagu
. . . w . .
. . . c . .
MOVER door  brama   speed=1.5 wait=4 key=klucz_czerwony
MOVER lift  winda   low=0 high=1 speed=1 wait=2.5
MOVER crusher pulapka low=0.1 speed=1 wait=0.6 damage=10 start=on
MOVER floor plyta   to=0.5 speed=0.5 loop=yes wait=1
MOVER ceil  strop   to=0.8 return=yes wait=3
```

| rodzaj | rusza | domyślnie |
|---|---|---|
| `door` | sufit od podłogi (zamknięte) do `open=` (domyślnie sufit z mapy) | `speed=1.5 wait=4` (0 = zostają otwarte), `use=yes`, cofa się, gdy ktoś stoi w przejściu |
| `lift` | podłogę z `high=` (pozycja spoczynkowa, domyślnie podłoga z mapy) do `low=` (domyślnie najniższa podłoga sąsiadów), czeka `wait`, wraca | `speed=1 wait=2.5 use=yes`, cofa się, gdy zgniótłby ciało o sufit |
| `crusher` | sufit z `high=` do `low=` (domyślnie podłoga+0.1) i z powrotem, w kółko | `speed=1 wait=0.6 damage=10`, przy oporze stoi i zadaje obrażenia co 0.4 s |
| `floor` / `ceil` | podłogę / sufit do `to=` | `speed=1`, `return=yes` wraca po `wait`, `loop=yes` jeździ w kółko |

Wspólne klucze: `use=yes|no` (aktywacja klawiszem E: stojąc przed obszarem
albo na nim - winda), `walk=<tag>` (aktywacja, gdy gracz wejdzie na komórki
/sektory z tym tagiem; `walk=yes` = własny obszar), `key=<przedmiot>`
(zużywany przy pierwszym użyciu), `locked=yes` (E tylko pokazuje
`onDoorLocked`; otwierają skrypty i przełączniki), `start=on` (rusza od
startu, np. zgniatacz), `start=open` (drzwi otwarte na starcie),
`reverse=yes|no`, `damage=`, `sound_start=`, `sound_move=` (pętla w czasie
ruchu), `sound_stop=` (`none` = cisza; domyślnie `mover_start`,
`mover_move`, `mover_stop` - poziom może zdefiniować własne `SOUND` o tych
nazwach). Głośność zależy od odległości gracza.

Gracz i wrogowie jeżdżą na windach (gracz nie spada, kamera się nie
„schodkuje"), zamykające się drzwi i windy cofają się przed ciałem,
zgniatacz naciska i rani (gracza i wrogów). Sterowanie:

- klawisz E (interact) - na obszarze przed sobą lub pod sobą (windy),
- `PLACE switch <typ> <id> x y z tag=<tag>[,<tag>] action=activate|open|close|stop once=yes key=<przedmiot> sound=<dźwięk>`
  (przełącznik: E w zasięgu 1.3 przed sobą; bez `sprite=` silnik rysuje
  dźwignię, `sprite_on=` = obrazek po przełączeniu; zdarzenie `onSwitch(id, tag)`),
- `PLACE zone ... activate=<tag>` - wejście w strefę,
- akcje triggerów `activate <tag>`, `move_floor <tag> <wys> [prędkość]`,
  `move_ceil <tag> <wys> [prędkość]`, `unlock_door <tag>` / `lock_door <tag>`,
- skrypty (Lua, w JS jako `engine.*`): `activate(tag)`, `moveFloor(tag, wys[, prędkość])`,
  `moveCeil(tag, wys[, prędkość])`, `stopMover(tag)`, `getFloor(tag)`,
  `getCeil(tag)`, `openDoor(tag)` / `closeDoor(tag)` (też `unlockDoor`/`lockDoor`).
  `moveFloor`/`moveCeil` działają też dla tagu bez `MOVER`.

Zdarzenia: `onMoverStart(tag)`, `onMoverStop(tag)`, `onCrush(ciało, obrażenia)`
(`player` albo id wroga), `onAreaEnter(tag)`, `onDoorOpen(tag)`, `onDoorLocked(tag, klucz)`.

Drzwi z `PLACE door` i `D` w GRID przesuwają się z `anim=yes` przy drzwiach
albo dla całej mapy z `DOOR_STYLE slide` (`anim=no` zostawia dawne,
natychmiastowe). Klucze, sekrety (`secret=yes` - wyglądają jak ściana) i
`unlockDoor` działają jak dotąd; `speed=`, `wait=` jak przy `MOVER door`.
Mapy bez tych linii działają jak wcześniej.

Przykłady: Krypta - winda w Przedsionku, zgniatacz z dźwignią w Krypcie,
wrota niszy (sektor) w Sanktuarium.

### Animowane tekstury

```
TEXTURE_ANIM woda frames=woda_0,woda_1,woda_2,woda_3 fps=5 scroll=0.05,0.12
TEXTURE_ANIM runy frames=runy_0,runy_1 fps=2
TEXTURE_ANIM lawa scroll=0.2,0.05          # samo przesuwanie
```

Animacja jest przypisana do nazwy tekstury: każda powierzchnia z tą nazwą
pokazuje bieżącą klatkę (`fps` klatek na sekundę) przesuniętą o `scroll`
(u,v powtórzeń tekstury na sekundę; ściany: u wzdłuż ściany, podłogi: u =
x świata). Koszt: wybór klatki raz na klatkę obrazu, przesunięcie to jedno
dodawanie przy próbkowaniu. Linie mogą stać w mapie albo w deskryptorze
`assets/textures/textures.aie` (oraz `textures.aie` w katalogu `TEXTURES`
poziomu) - wtedy działają we wszystkich poziomach. Pliki klatek:
`<katalog tekstur>/<nazwa>.png` (kwadrat, bok potęga 2).

Mapa GRID wybiera tekstury per komórka paletą i warstwami (`.` = domyślna):

```
TEX_SYMBOLS w=woda l=lawa r=runy
FLOOR_TEX_GRID       # podłogi komórek
CEILING_TEX_GRID     # sufity komórek
WALL_TEX_GRID        # ściany pełnych komórek i ścianki stopni wolnych
```

Mapa sektorowa: `floor_tex=woda` itd. przy `SECTOR`.

### Animacja sprite'ów wrogów

Wrogowie mają stany animacji `idle`, `walk`, `attack` (zamach i cios),
`pain` (po trafieniu) i `death` (ciało zostaje na ziemi) i 8 kątów widzenia
(widok wybiera kąt między kierunkiem, w którym wróg patrzy, a kamerą).
Arkusz z rotacjami opisuje plik tekstowy obok atlasu (`<atlas>.anim`):

```
SHEET straznik_8.png      # obrazek (względem pliku)
GRID 8 7                  # kolumny wiersze równych komórek
ROTATIONS 8               # kolumny = kąty (0 przód, dalej przeciwnie do zegara), wiersze = klatki
MIRROR yes                # opcjonalnie: narysowane tylko kolumny 0..4, 5..7 to odbicia 3..1
ANIM idle   frames=0,1     fps=2
ANIM walk   frames=1,2,3,2 fps=7
ANIM attack frames=4,5     fps=8
ANIM pain   frames=6       fps=6 loop=no
ANIM death  frames=6,7,8   fps=6 loop=no
```

Bez deskryptora silnik bierze klasyczny atlas 3×2 (`ENEMY_ATLAS`,
`atlas=`) i generuje z niego zastępcze 8 rotacji (zwężenie przy obrocie,
ciemniejsza strona odwrócona, ciemny tył) i klatki śmierci. Wróg bez atlasu
to pojedynczy obrazek jak dotąd. `PLACE enemy ... height=` to wysokość
ciała dla drzwi i zgniataczy (domyślnie min(0.6, scale)).

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
| `door` | `PLACE door <id> x y z` – drzwi otwierane `unlockDoor(id)` | `key`, `secret`, `use`, `anim`, `speed`, `wait` |
| `switch` | przełącznik (E) uruchamiający movery tagu | `tag`, `action`, `once`, `key`, `sprite`, `sprite_on`, `sound` |
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
