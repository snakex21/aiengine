# Kampania, przejścia między poziomami i rozbudowane poziomy

Przykład całości: `projects/krypta` (5 poziomów).

## Kampania (manifest)

```
CAMPAIGN yes
CARRY hp score items          # co przechodzi do następnego poziomu
LEVEL przedsionek maps/przedsionek.aie Przedsionek
LEVEL krypta      maps/krypta.aie      Kryształowa Krypta
...
TEXT_LEVEL_COMPLETE Poziom ukończony!
TEXT_LOADING Wczytywanie...
```

- **Kolejność** wyznacza lista `LEVEL`. Pierwszy poziom jest odblokowany na
  starcie, a ukończenie poziomu odblokowuje następny.
- **`CARRY`** wybiera, co przechodzi między poziomami:
  - `hp` – zdrowie gracza,
  - `ammo` – amunicja,
  - `items` – ekwipunek (`giveItem`, przedmioty z `give=`),
  - każda inna nazwa – zmienna skryptu, np. `score`.
- **Zapis postępu** trafia do `save.aie` w katalogu projektu (obok gry, nie w
  AppData; plik jest w `.gitignore`). Zawiera bieżący poziom, odblokowane i
  ukończone poziomy, najlepsze czasy oraz stan z wejścia na bieżący poziom.
  Zapis następuje po każdym ukończonym poziomie, po „Nowej grze” i po wyborze
  poziomu.
- **Menu** (plik menu, zob. `docs/ui-menus.md`):
  - `ITEM "Kontynuuj" continue` – pozycja widoczna tylko wtedy, gdy jest zapis;
  - `new_game` – zaczyna od pierwszego poziomu i zachowuje rekordy;
  - wybór poziomu pokazuje rekordy, a zablokowane poziomy są wyszarzone
    (`TEXT_LOCKED`).

### Przebieg

```
ekran intro (nazwa poziomu + INTRO z mapy) --Enter--> gra
gra --exitLevel()/exit_to/strefa to=--> ekran outro (OUTRO, czas, rekord)
outro --Enter--> ekran "Wczytywanie..." --> intro następnego poziomu
ostatni poziom --> "Kampania ukończona" --Enter--> menu główne
porażka --> ekran końca gry (Enter: restart poziomu z tym samym stanem wejściowym)
```

Ekrany zatrzymują grę i czas. Dalej przechodzi się klawiszem Enter, spacją,
kliknięciem albo przyciskiem A na padzie. Teksty ekranów ustawiają klucze
`TEXT_*` w manifeście: `LEVEL_COMPLETE`, `CAMPAIGN_COMPLETE`, `OUTRO`,
`LOADING`, `START`, `CONTINUE`, `TIME`, `BEST`, `RECORD`, `LOCKED`.

### Wyjście z poziomu

| Sposób | Przykład |
|---|---|
| Lua / JS | `exitLevel()` (następny poziom), `exitLevel("krypta")`, `Engine.exitLevel()` |
| trigger | `ACTION exit_to next` albo `ACTION exit_to <id>` |
| strefa | `PLACE zone exit portal 9.5 0 13.5 to=next` (wejście kończy poziom) |

Poza kampanią `exitLevel(cel, tekst)` działa jak `winGame(tekst)`.

## Mapa: nowe właściwości

```
MAP lodowe_groby
TEXTURES assets/textures/lod     # ściany brick/stone/metal/wood + floor/ceiling
INTRO Linia 1|Linia 2            # "|" = nowa linia
OUTRO Tekst po ukończeniu
VAR_TIME_LIMIT 240               # zmienne skryptu (time_limit)
```

Silnik ustawia też zmienne `level_name`, `items_<typ>` (np.
`items_crystal`) i `secrets_total`.

Tekstury z `TEXTURES` nakładają się na zestaw projektu
(`assets/textures`). Pliki `floor.png` i `ceiling.png` włączają teksturowaną
podłogę i sufit. Wszystkie tekstury muszą być kwadratowe, a ich bok musi być
potęgą 2.

## Drzwi, klucze, sekrety, ekwipunek

```
PLACE item klucz_czerwony kc 6.5 0 6.5 give=klucz_czerwony
PLACE door czerwone 14 0 3 key=klucz_czerwony   # otwiera E z kluczem (klucz znika)
PLACE door nisza 10 0 10 secret=yes             # wygląda jak ściana, otwiera E
PLACE door brama 12 0 14                        # tylko skrypt/trigger
```

- **Drzwi `D` z siatki** bez `PLACE` otwiera się klawiszem `interact` (E).
- **Zdarzenia:**
  - `onDoorOpen(id)`,
  - `onSecretFound(id)`,
  - `onDoorLocked(id, klucz)` – drzwi wymagające klucza albo otwierane tylko
    skryptem; pozwala wyświetlić komunikat,
  - `onInventory(nazwa, ilość)`.
- **Lua:** `giveItem`, `takeItem`, `itemCount`, `hasItem`.

## Przeciwnicy

```
PLACE enemy_type pajak 0 0 0 hp=20 speed=2.2 damage=6 sight=6 behavior=wander atlas=assets/sprites/pajak.png scale=0.55 z=0.22
PLACE enemy pajak p1 3.5 0 8.5            # bierze wartości z typu
PLACE enemy straznik s1 19.5 0 3.5 behavior=ambush hp=70
```

| Parametr | Znaczenie |
|---|---|
| `hp`, `speed`, `damage`, `range`, `attack_rate`, `sight` | statystyki |
| `atlas` | własny atlas 3×2 (klatki: stoi, 3× chód, 2× atak); domyślnie `ENEMY_ATLAS` |
| `scale`, `z` | rozmiar sprite'a i przesunięcie w pionie (`z=(1-scale)/2` stawia go na podłodze) |
| `behavior` | `chase` (domyślne: goni i szuka w miejscu, gdzie ostatnio widział gracza), `guard` (wraca na posterunek), `wander` (krąży w okolicy), `ambush` (stoi, aż gracz podejdzie na połowę zasięgu wzroku) |

Ruch przeciwnika:
- Gdy droga jest wolna, idzie prosto do celu.
- Gdy nie jest, korzysta z **A\*** po siatce (pakiet `nav`) i omija narożniki,
  zamiast się na nich zatrzymywać.
- Widzi gracza tylko wtedy, gdy między nimi nie ma ściany.

`spawnEnemy(typ, x, y)` z Lua tworzy widocznego przeciwnika według typu z
`enemy_type`. Krypta korzysta z tego w walce z bossem.

## Grafika i okno

- **Rozdzielczości:** 960×540 … 1920×1080, 2560×1440, 3440×1440, 3840×2160 i
  „Natywna” (rozmiar monitora).
- **`RENDER_SCALE` (50–200%)** określa, jaką część rozdzielczości silnik
  faktycznie renderuje; obraz jest rozciągany na okno.
- **FOV** jest zdefiniowany dla 16:9 i działa w trybie Hor+: ekran
  ultrawide widzi więcej na boki, a obraz nie jest zgniatany.
- **HUD i menu** skalują się z wysokością renderu.
- **Pełny ekran:**
  - domyślnie bez ramki (okno na cały monitor, bez zmiany trybu wideo, więc
    Alt+Tab i klawisz Win działają od razu),
  - `FULLSCREEN_MODE exclusive` – tryb wyłączny,
  - przełączanie klawiszem F11 albo Alt+Enter.
- **Utrata fokusu** otwiera pauzę, zwalnia kursor i wycisza dźwięk
  (`BACKGROUND_VOLUME`, domyślnie 0%). Kursor wraca do gry dopiero po
  kliknięciu.
- Gdy okno jest zminimalizowane, gra nie renderuje.

## Testy i automatyzacja

```
go test ./progress/ ./nav/ ./level/ ./projects/krypta/
aiengine.exe --project krypta --screenshot x.png --frames 12 --exec "level zalana_kopalnia play; lua exitLevel(); @5 advance"
```

Komendy konsoli dla `--exec`:
- `level <id> [play]` – uruchamia poziom kampanii (bez `play` zostaje na ekranie
  intro),
- `advance` – przechodzi dalej z ekranu intro/outro,
- `focus off|on` – symuluje utratę i odzyskanie fokusu,
- `@N komenda` – wykonuje komendę w klatce N.
