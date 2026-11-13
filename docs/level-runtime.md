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
starcie (`tp x y [kąt]`, `god`, `fps`, `lua <kod>`, `restart`...).
Przy wyjściu silnik loguje średnie FPS i zużycie pamięci (`[Perf]`).
