# Menu, pauza i ustawienia (`MENU menu.aie`)

Projekt włącza system menu linią w `manifest.aie`:

```
MENU menu.aie
LEVEL krypta  maps/krypta.aie  Kryształowa Krypta
LEVEL trening maps/trening.aie Trening
```

`LEVEL <id> <mapa> [tytuł]` (powtarzalne) tworzy listę poziomów dla
`new_game` (pierwszy poziom), `load_level <id>` i ekranu wyboru poziomu. Bez
`LEVEL` jedynym poziomem jest `MAIN_MAP`.

Projekty bez `MENU` (np. `demo`) działają jak dawniej: Esc przełącza pauzę i
wysyła zdarzenie `onPause`.

Przykład: `projects/krypta/menu.aie`.

## Format pliku menu

Jedna dyrektywa na linię, `#` zaczyna komentarz, tekst ze spacjami w
cudzysłowie. Tekst może zawierać polskie znaki.

```
SOUND menu_move WAVEFORM=square FREQUENCY=660 DECAY=0.04 VOLUME=0.2
SOUNDS move=menu_move select=menu_select back=menu_back
STYLE title_color=#7FE0FF text_color=#D8D4E8 focus_color=#FFE08A focus_bg=#3A2A6AB0 dim_color=#05040AA0 hint_color=#8C88A0

SCREEN main
  TITLE "Kryształowa Krypta"
  SUBTITLE "Zbierz kryształy"
  BACKGROUND map 7.4 2.6 160
  MUSIC menu_drone
  HINT "Strzałki: wybór   Enter: zatwierdź"
  ITEM "Nowa gra" new_game
  ITEM "Wybór poziomu" level_select
  ITEM "Jak grać" menu instrukcja
  ITEM "Ustawienia" settings
  ITEM "Wyjście" quit

SCREEN pause
  TITLE "Pauza"
  BACKGROUND dim
  ESCAPE resume
  ITEM "Wznów" resume
  ITEM "Restart" restart
  ITEM "Ustawienia" settings
  ITEM "Menu główne" main_menu
```

### Dyrektywy globalne

| Dyrektywa | Znaczenie |
|---|---|
| `SOUND <nazwa> KLUCZ=wartość ...` | dźwięk proceduralny (te same klucze co blok `SOUND` w `.aie`: `WAVEFORM`, `FREQUENCY`, `DECAY`, `VOLUME`, `TYPE`, `ENVELOPE`, ...) |
| `SOUNDS move= select= back=` | dźwięki nawigacji |
| `STYLE klucz=#RRGGBB[AA] ...` | `title_color`, `text_color`, `focus_color`, `focus_bg`, `dim_color`, `hint_color` |
| `SCREEN <nazwa>` | początek ekranu; dalsze linie należą do niego |

### Dyrektywy ekranu

| Dyrektywa | Znaczenie |
|---|---|
| `TITLE`, `SUBTITLE`, `HINT` | tytuł, podtytuł, podpowiedź na dole |
| `BACKGROUND none` | tylko napisy na tle gry |
| `BACKGROUND dim` | przyciemniony, zatrzymany widok gry |
| `BACKGROUND color #RRGGBB` | jednolity kolor |
| `BACKGROUND image <plik.png>` | obraz rozciągnięty na ekran (ścieżka względem projektu) |
| `BACKGROUND map [x z kąt]` | widok poziomu z podanego miejsca, kamera powoli się obraca |
| `MUSIC <dźwięk>` | zapętlona muzyka ekranu (`SOUND` z menu lub mapy albo plik audio); po zamknięciu menu wraca muzyka poziomu |
| `ITEM "<tekst>" <akcja> [argument]` | pozycja menu |
| `ESCAPE <akcja>` | co robi Esc na ekranie głównym stosu (np. `resume` w pauzie) |
| `LABEL <klucz> "<tekst>"` | tłumaczenie wierszy generowanych (zob. niżej) |

### Akcje

| Akcja | Działanie |
|---|---|
| `new_game` | uruchamia pierwszy poziom od nowa |
| `load_level <id>` | uruchamia poziom z listy `LEVEL` |
| `level_select` | ekran wyboru poziomu (pozycja ukryta, gdy poziomów jest mniej niż 2) |
| `settings` | ekran ustawień |
| `menu <ekran>` | otwiera dowolny ekran z pliku (podmenu) |
| `back` | wraca do poprzedniego ekranu |
| `resume` | zamyka menu i wraca do gry |
| `restart` | restart bieżącego poziomu |
| `main_menu` | ekran `main` |
| `quit` | zamyka grę |
| `run_script <funkcja>` | wywołuje globalną funkcję Lua (np. `run_script pokazSekret`) |

### Ekrany specjalne

- `main` otwiera się przy starcie gry. Gra i czas gry stoją, dopóki gracz nie
  wybierze `new_game` albo `load_level`.
- `pause` otwiera się klawiszem `pause` (Esc) lub przyciskiem Start pada.
  Po wygranej lub przegranej Esc otwiera `main`.
- `settings` jest generowany. Jeśli ekran nie ma `ITEM`, silnik dodaje wiersze
  `mouse_sensitivity`, `invert_y`, `master_volume`, `music_volume`,
  `sfx_volume`, `resolution`, `fullscreen`, `fov` i `back`. Etykiety
  (domyślnie angielskie) i teksty `on`/`off` zmienisz przez `LABEL`.
- `level_select` jest generowany z listy `LEVEL` (plus `back`).

## Sterowanie w menu

| Wejście | Działanie |
|---|---|
| ↑ ↓ / W S, d-pad, lewa gałka | zmiana zaznaczenia |
| ← → / A D, d-pad | zmiana wartości ustawienia |
| Enter / Spacja, A na padzie | zatwierdzenie |
| Esc / Backspace, B lub Start na padzie | wstecz (albo `ESCAPE` ekranu) |
| mysz | najechanie zaznacza; lewy klik zatwierdza lub zwiększa wartość, prawy zmniejsza |

Zaznaczona pozycja ma pasek w kolorze `focus_bg`, tekst w `focus_color` i
znaczniki `> <`. Przy wartościach ustawień widać `< wartość >`.

## Pauza i czas gry

Gdy menu jest otwarte (także pauza), silnik nie wykonuje kroku gry: stoją
ruch, wrogowie, `onUpdate`, timery Lua (`startTimer`/`onTimer`), JS
(`setTimeout`/`setInterval`) i `getTime()`. Wszystkie liczą **czas gry**, a nie
zegar ścienny. Konsola (`~`) też zatrzymuje czas.

## Ustawienia

Plik `settings.aie` powstaje w katalogu projektu (obok gry, nie w AppData)
przy pierwszej zmianie i jest wczytywany przy starcie. Zmiany działają od
razu:

```
MOUSE_SENSITIVITY 1.00   # 0.2–3
INVERT_Y no
MASTER_VOLUME 1.00       # 0–1, efekty = master*sfx, muzyka = master*music
MUSIC_VOLUME 0.70
SFX_VOLUME 1.00
RESOLUTION 1280x720      # 960x540, 1280x720, 1600x900, 1920x1080
FULLSCREEN no            # także F11
FOV 66                   # 50–110 stopni
```

`RESOLUTION` to rozdzielczość renderowania. W oknie jest to rozmiar okna, a na
pełnym ekranie obraz jest rozciągany na cały monitor.

## Czcionka

Bitmapowa czcionka 5×7 (tylko wielkie litery) obsługuje UTF-8 i polskie
znaki `ąćęłńóśźż ĄĆĘŁŃÓŚŹŻ`. Akcenty wystają 2 piksele (×skala) ponad i pod
literę. HUD, komunikaty, menu i konsola są rysowane na osobnej warstwie UI,
nad sprite'ami i bronią.

## Automatyzacja

Komendy konsoli (też przez `--exec`): `menu <ekran>`, `menu close`,
`menukey up down left right enter esc`, `fullscreen`. Przykład:

```
aiengine.exe --project krypta --screenshot ustawienia.png --frames 30 --exec "menu settings; menukey down right"
```

Testy bez okna: `go test ./ui/ ./settings/ ./projects/krypta/`.
