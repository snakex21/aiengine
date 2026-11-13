"""Writes the Krypta campaign maps (run with python -I, arg: projects/krypta)."""
import os
import sys

ROOT = sys.argv[1]
MAPS = os.path.join(ROOT, "maps")

COMMON_TYPES = """# Typy przeciwników (wartości domyślne dla PLACE enemy i spawnEnemy)
PLACE enemy_type straznik 0 0 0 hp=50 speed=0.9 damage=10 sight=5 behavior=chase
PLACE enemy_type pajak 0 0 0 hp=20 speed=2.2 damage=6 sight=6 behavior=wander atlas=assets/sprites/pajak.png scale=0.55 z=0.22 range=0.7 attack_rate=0.7
"""

COMMON_SOUNDS = """SOUND ding
  WAVEFORM sine
  FREQUENCY 1320
  DECAY 0.25
  VOLUME 0.5

SOUND gate
  WAVEFORM square
  FREQUENCY 110
  DECAY 0.9
  VOLUME 0.4

SOUND sizzle
  TYPE noise
  FREQUENCY 400
  DECAY 0.15
  VOLUME 0.3

SOUND heal
  WAVEFORM triangle
  FREQUENCY 660
  DECAY 0.4
  VOLUME 0.5

SOUND klucz
  WAVEFORM sine
  FREQUENCY 880
  DECAY 0.35
  VOLUME 0.45

SOUND sekret
  WAVEFORM triangle
  FREQUENCY 520
  DECAY 0.8
  VOLUME 0.45

SOUND zamek
  WAVEFORM square
  FREQUENCY 160
  DECAY 0.12
  VOLUME 0.3

SOUND krypta_drone
  WAVEFORM triangle
  FREQUENCY 55
  DECAY 3.5
  VOLUME 0.25
"""


def write(name, text):
    with open(os.path.join(MAPS, name + ".aie"), "w", encoding="utf-8", newline="\n") as f:
        f.write(text)


def grid_text(rows):
    return "\n".join(" ".join(r) for r in rows)


def gate_trigger(n):
    return f"""TRIGGER brama_otwarta
  CONDITION var_at_least krysztaly {n}
  ACTION unlock_door brama
  ACTION play_sound gate
  ACTION show_message "Brama otwarta! Biegnij do portalu"
"""


# ------------------------------------------------------------------ level 1
L1 = """W W W W W W W W W W W W W W W W W W W W
W P . . . W . . . . . . W . . . . . . W
W . . . . W . . S . . . W . . S . . . W
W . . . . D . . . . . . W . . . . . . W
W W . W W W . . . S . . D . . . . . . W
W . . . . W W W . W W W W W W . W W W W
W . . . . . . W . W . . . . . . . . . W
W . S . . . . W . W . . . . . S . . . W
W . . . . . . W . W . . . . . . . . . W
W W W W W W W W . W W W W W W D W W W W
W . . . . . W . . . W . . . . . . . . W
W . . S . . W . . . W . . . . . . . . W
W . . . . . W . . . W . . . . . . . . W
W W W W W W W W W W W W W W W W W W W W"""

write("przedsionek", f"""# maps/przedsionek.aie — Poziom 1: Przedsionek
# Nauka: drzwi (E), klucz i zamknięte drzwi, ukryty sekret, 4 kryształy.

MAP przedsionek
PLAYER_ANGLE 0
PLAYER_HP 100
FLOOR_COLOR #1E1A24
CEILING_COLOR #0C0B14
VAR_TIME_LIMIT 210
VAR_MUSIC krypta_drone
INTRO Drzwi otwierasz klawiszem E.|Czerwony klucz otworzy czerwone drzwi.|Niektóre ściany skrywają sekrety - sprawdzaj je klawiszem E.
OUTRO Przedsionek za tobą. Dalej czeka właściwa krypta.

GRID
{L1}

{COMMON_TYPES}
PLACE item crystal k1 10.5 0 1.5 bob=0.15
PLACE item crystal k2 17.5 0 1.5 bob=0.15
PLACE item crystal k3 11.5 0 7.5 bob=0.15
PLACE item crystal k4 8.5 0 11.5 bob=0.15
PLACE item klucz_czerwony kc 5.5 0 6.5 give=klucz_czerwony bob=0.1 scale=0.4
PLACE item relikwia r1 2.5 0 11.5 give=relikwia bob=0.1 scale=0.4
PLACE item potion m1 13.5 0 3.5 scale=0.35
PLACE zone acid kw1 10.5 0 3.5 radius=0.45 sprite=acid scale=0.7
PLACE door czerwone 12 0 4 key=klucz_czerwony
PLACE door sekret1 6 0 11 secret=yes
PLACE door brama 15 0 9
PLACE enemy straznik s1 4.5 0 7.5 speed=0.7
PLACE enemy straznik s2 16.5 0 7.5 behavior=guard
PLACE zone exit portal 17.5 0 11.5 radius=0.6 sprite=portal scale=0.9

{COMMON_SOUNDS}
{gate_trigger(4)}""")

# ------------------------------------------------------------------ level 3
L3 = """W W W W W W W W W W W W W W W W W W W W W W
W P . . . . W . . . . . . . W . . . . . . W
W . . S . . W . . S . . S . W . . . . . . W
W . . . . . D . . . . . . . D . . S . . . W
W . . S . . W . . . . . . . W . . . . . . W
W W W . W W W W W W . W W W W W W W . W W W
W . . . . . . . W . . . W . . . . . . . . W
W . S . . S . . W . . . W . . S . . S . . W
W . . . . . . . W . . . W . . . . . . . . W
W . . . . . . . D . . . D . . . . . . . . W
W W W W D W W W W W W W W W W W W W W W . W
W . . . . . . W W W . W W . . . . . . . . W
W . . S . . . W . . W . W . . . . S . . . W
W . . . . . . W . . . . W . . . . . . . . W
W . . . . . . W . . . . D . . . . . . . . W
W W W W W W W W W W W W W W W W W W W W W W"""

write("lodowe_groby", f"""# maps/lodowe_groby.aie — Poziom 3: Lodowe Groby
# Dwa klucze, pająki, ukryta nisza z relikwią, 5 kryształów.

MAP lodowe_groby
PLAYER_ANGLE 0
PLAYER_HP 100
TEXTURES assets/textures/lod
FLOOR_COLOR #8CA8C0
CEILING_COLOR #101828
VAR_TIME_LIMIT 240
VAR_MUSIC krypta_drone
INTRO Mróz skuł groby dawnych strażników.|Pająki są szybkie, ale słabe - wystarczą jeden lub dwa strzały.|Potrzebujesz czerwonego i niebieskiego klucza.
OUTRO Lód pęka za twoimi plecami. Schodzisz głębiej, do starej kopalni.

GRID
{L3}

{COMMON_TYPES}
PLACE item crystal k1 12.5 0 1.5 bob=0.15
PLACE item crystal k2 16.5 0 4.5 bob=0.15
PLACE item crystal k3 14.5 0 8.5 bob=0.15
PLACE item crystal k4 19.5 0 13.5 bob=0.15
PLACE item crystal k5 1.5 0 13.5 bob=0.15
PLACE item klucz_czerwony kc 6.5 0 6.5 give=klucz_czerwony bob=0.1 scale=0.4
PLACE item klucz_niebieski kn 19.5 0 1.5 give=klucz_niebieski bob=0.1 scale=0.4
PLACE item relikwia r1 10.5 0 11.5 give=relikwia bob=0.1 scale=0.4
PLACE item potion m1 20.5 0 6.5 scale=0.35
PLACE item potion m2 5.5 0 13.5 scale=0.35
PLACE zone acid kw1 8.5 0 3.5 radius=0.45 sprite=acid scale=0.7
PLACE zone acid kw2 16.5 0 12.5 radius=0.45 sprite=acid scale=0.7
PLACE door czerwone 14 0 3 key=klucz_czerwony
PLACE door niebieskie 4 0 10 key=klucz_niebieski
PLACE door nisza 10 0 10 secret=yes
PLACE door brama 12 0 14
PLACE enemy pajak p1 3.5 0 8.5
PLACE enemy pajak p2 16.5 0 7.5
PLACE enemy pajak p3 14.5 0 13.5
PLACE enemy straznik s1 19.5 0 3.5 behavior=ambush
PLACE enemy straznik s2 5.5 0 11.5 behavior=guard
PLACE zone exit portal 9.5 0 13.5 radius=0.6 sprite=portal scale=0.9

{COMMON_SOUNDS}
{gate_trigger(5)}""")


# ------------------------------------------------------------------ room grids
def room_grid(cols, rows, links, pillars=()):
    """cols x rows rooms with 6x4 interiors. links: ((i,j),(i2,j2),kind)
    kind: '.' open, 'D' door. Returns rows of symbols."""
    W, H = cols * 7 + 1, rows * 5 + 1
    g = [["W"] * W for _ in range(H)]
    for i in range(cols):
        for j in range(rows):
            for y in range(1 + 5 * j, 5 + 5 * j):
                for x in range(1 + 7 * i, 7 + 7 * i):
                    g[y][x] = "."
    cells = {}
    for (a, b, kind) in links:
        (i, j), (i2, j2) = a, b
        if j == j2:  # horizontal neighbours
            x, y = 7 * max(i, i2), 1 + 5 * j + 2
        else:
            x, y = 1 + 7 * i + 3, 5 * max(j, j2)
        g[y][x] = kind
        cells[(a, b)] = (x, y)
    for (x, y) in pillars:
        g[y][x] = "S"
    return g, cells


# ------------------------------------------------------------------ level 4
links = [
    ((0, 0), (1, 0), "."), ((1, 0), (2, 0), "D"), ((0, 0), (0, 1), "."),
    ((0, 1), (0, 2), "D"), ((1, 0), (1, 1), "D"), ((1, 1), (2, 1), "."),
    ((2, 1), (2, 2), "D"), ((1, 1), (1, 2), "W"), ((0, 1), (1, 1), "."),
]
g4, c4 = room_grid(3, 3, links, pillars=[(10, 2), (17, 8), (3, 12), (11, 8)])
g4[2][2] = "P"
red = c4[((1, 0), (2, 0))]
blue = c4[((0, 1), (0, 2))]
gate = c4[((2, 1), (2, 2))]
secret = c4[((1, 1), (1, 2))]
write("zalana_kopalnia", f"""# maps/zalana_kopalnia.aie — Poziom 4: Zalana Kopalnia
# Kwas, zasadzki, dwa klucze, sekretna komora z relikwią, 6 kryształów.

MAP zalana_kopalnia
PLAYER_ANGLE 0
PLAYER_HP 100
TEXTURES assets/textures/kopalnia
FLOOR_COLOR #3A3028
CEILING_COLOR #120C08
VAR_TIME_LIMIT 270
VAR_MUSIC krypta_drone
INTRO Kopalnię zalał żrący kwas.|Strażnicy czekają w zasadzkach i ruszają dopiero, gdy podejdziesz.|Ściany korytarzy kryją niejedną tajemnicę.
OUTRO Za kopalnią bije serce krypty. Strażnik Krypty już na ciebie czeka.

GRID
{grid_text(g4)}

{COMMON_TYPES}
PLACE item crystal k1 5.5 0 1.5 bob=0.15
PLACE item crystal k2 12.5 0 3.5 bob=0.15
PLACE item crystal k3 19.5 0 1.5 bob=0.15
PLACE item crystal k4 1.5 0 13.5 bob=0.15
PLACE item crystal k5 19.5 0 6.5 bob=0.15
PLACE item crystal k6 8.5 0 9.5 bob=0.15
PLACE item klucz_czerwony kc 1.5 0 8.5 give=klucz_czerwony bob=0.1 scale=0.4
PLACE item klucz_niebieski kn 19.5 0 4.5 give=klucz_niebieski bob=0.1 scale=0.4
PLACE item relikwia r1 11.5 0 12.5 give=relikwia bob=0.1 scale=0.4
PLACE item potion m1 5.5 0 13.5 scale=0.35
PLACE item potion m2 15.5 0 9.5 scale=0.35
PLACE zone acid kw1 9.5 0 2.5 radius=0.45 sprite=acid scale=0.7
PLACE zone acid kw2 11.5 0 1.5 radius=0.45 sprite=acid scale=0.7
PLACE zone acid kw3 17.5 0 6.5 radius=0.45 sprite=acid scale=0.7
PLACE zone acid kw4 18.5 0 9.5 radius=0.45 sprite=acid scale=0.7
PLACE zone acid kw5 12.5 0 7.5 radius=0.45 sprite=acid scale=0.7
PLACE door czerwone {red[0]} 0 {red[1]} key=klucz_czerwony
PLACE door niebieskie {blue[0]} 0 {blue[1]} key=klucz_niebieski
PLACE door komora {secret[0]} 0 {secret[1]} secret=yes
PLACE door brama {gate[0]} 0 {gate[1]}
PLACE enemy pajak p1 9.5 0 8.5
PLACE enemy pajak p2 12.5 0 9.5
PLACE enemy pajak p3 4.5 0 7.5
PLACE enemy straznik s1 18.5 0 3.5 behavior=ambush hp=70
PLACE enemy straznik s2 4.5 0 12.5 behavior=ambush hp=70
PLACE enemy straznik s3 16.5 0 8.5 behavior=guard
PLACE zone exit portal 18.5 0 12.5 radius=0.6 sprite=portal scale=0.9

{COMMON_SOUNDS}
{gate_trigger(6)}""")

# ------------------------------------------------------------------ level 5 (boss)
W5, H5 = 19, 16
g5 = [["W"] * W5 for _ in range(H5)]
for y in range(1, 11):
    for x in range(1, 16):
        g5[y][x] = "."
for y in range(12, 15):
    for x in range(5, 13):
        g5[y][x] = "."
g5[11][8] = "D"           # gate to the exit chamber
g5[5][17] = "."           # secret niche behind (16,5)
for (x, y) in [(4, 3), (12, 3), (4, 8), (12, 8)]:
    g5[y][x] = "S"
g5[9][8] = "P"
write("serce_krypty", f"""# maps/serce_krypty.aie — Poziom 5: Serce Krypty (walka z bossem)
# Pokonaj Strażnika Krypty; co kilka sekund przywołuje pająki.

MAP serce_krypty
PLAYER_ANGLE -90
PLAYER_HP 100
TEXTURES assets/textures/serce
FLOOR_COLOR #301018
CEILING_COLOR #0A0406
VAR_TIME_LIMIT 300
VAR_BOSS boss
VAR_SUMMON 9
VAR_MUSIC krypta_drone
INTRO Serce krypty pulsuje czerwonym światłem.|Strażnik Krypty przywołuje pająki - pokonaj go, a brama do portalu się otworzy.|Kryj się za filarami.
OUTRO Strażnik pada, a krypta cichnie. Kryształy są twoje.

GRID
{grid_text(g5)}

{COMMON_TYPES}
PLACE enemy_type wladca 0 0 0 hp=300 speed=0.8 damage=15 sight=9 range=1.2 attack_rate=1.2 behavior=chase atlas=assets/sprites/straznik_krypty.png scale=1.35 z=-0.15
PLACE enemy wladca boss 8.5 0 3.5
PLACE enemy pajak p1 2.5 0 2.5
PLACE enemy pajak p2 14.5 0 2.5
PLACE item potion m1 1.5 0 9.5 scale=0.35
PLACE item potion m2 15.5 0 9.5 scale=0.35
PLACE item potion m3 17.5 0 5.5 scale=0.35
PLACE item relikwia r1 17.5 0 5.5 give=relikwia bob=0.1 scale=0.4
PLACE door nisza 16 0 5 secret=yes
PLACE door brama 8 0 11
PLACE zone exit portal 8.5 0 13.5 radius=0.6 sprite=portal scale=0.9

{COMMON_SOUNDS}
TRIGGER boss_pokonany
  CONDITION enemy_dead boss
  ACTION unlock_door brama
  ACTION play_sound gate
  ACTION show_message "Strażnik Krypty pokonany! Brama otwarta"
""")
print("maps written")
