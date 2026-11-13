"""Generate original pixel-art assets for projects/krypta (run with python -I)."""
import math
import os
import random
import sys

from PIL import Image, ImageDraw, ImageFilter

ROOT = sys.argv[1]  # projects/krypta
SPR = os.path.join(ROOT, "assets", "sprites")
TEX = os.path.join(ROOT, "assets", "textures")
HAND = os.path.join(ROOT, "assets", "sprites", "player")
for d in (SPR, TEX, HAND):
    os.makedirs(d, exist_ok=True)

rnd = random.Random(1337)


def save(img, *path):
    img.save(os.path.join(*path))


# ---------------------------------------------------------------- textures
def crypt_blocks(base, mortar, seed, moss=None, rows=4):
    r = random.Random(seed)
    im = Image.new("RGBA", (64, 64), mortar)
    d = ImageDraw.Draw(im)
    bh = 64 // rows
    for row in range(rows):
        off = (row % 2) * 16
        for col in range(-1, 3):
            x0 = col * 32 + off + 1
            y0 = row * bh + 1
            shade = r.randint(-18, 18)
            c = tuple(max(0, min(255, v + shade)) for v in base) + (255,)
            d.rectangle([x0, y0, x0 + 29, y0 + bh - 3], fill=c)
            # chipped highlight / shadow edge
            hl = tuple(min(255, v + 25) for v in c[:3]) + (255,)
            sh = tuple(max(0, v - 30) for v in c[:3]) + (255,)
            d.line([x0, y0, x0 + 29, y0], fill=hl)
            d.line([x0, y0 + bh - 3, x0 + 29, y0 + bh - 3], fill=sh)
    px = im.load()
    for _ in range(260):  # grit
        x, y = r.randrange(64), r.randrange(64)
        c = px[x, y]
        k = r.randint(-25, 15)
        px[x, y] = tuple(max(0, min(255, v + k)) for v in c[:3]) + (255,)
    if moss:
        for _ in range(90):
            x = r.randrange(64)
            y = r.randrange(40, 64)
            px[x, y] = moss + (255,)
    return im


save(crypt_blocks((78, 82, 102), (34, 34, 44), 1), TEX, "brick.png")
save(crypt_blocks((92, 96, 84), (38, 40, 34), 2, moss=(60, 110, 50), rows=3), TEX, "stone.png")

# "metal": ancient bronze panel with a glowing rune
im = Image.new("RGBA", (64, 64), (96, 72, 40, 255))
d = ImageDraw.Draw(im)
for i in range(0, 64, 4):
    d.line([0, i, 63, i], fill=(104 + (i % 8), 78, 44, 255))
d.rectangle([3, 3, 60, 60], outline=(60, 44, 24, 255), width=2)
for x, y in ((7, 7), (56, 7), (7, 56), (56, 56)):
    d.ellipse([x - 2, y - 2, x + 2, y + 2], fill=(150, 120, 70, 255))
d.line([32, 16, 32, 48], fill=(90, 230, 255, 255), width=3)
d.line([22, 24, 42, 40], fill=(90, 230, 255, 255), width=2)
d.line([42, 24, 22, 40], fill=(90, 230, 255, 255), width=2)
save(im, TEX, "metal.png")

# "wood": the sealed gate (door) - dark planks with iron bands
im = Image.new("RGBA", (64, 64), (70, 44, 26, 255))
d = ImageDraw.Draw(im)
for x in range(0, 64, 8):
    d.rectangle([x, 0, x + 6, 63], fill=(88 + rnd.randint(-8, 8), 56, 32, 255))
for y in (10, 52):
    d.rectangle([0, y, 63, y + 4], fill=(48, 48, 56, 255))
    for x in range(4, 64, 12):
        d.ellipse([x, y + 1, x + 2, y + 3], fill=(120, 120, 130, 255))
d.ellipse([26, 26, 38, 38], outline=(90, 230, 255, 255), width=2)  # rune lock
save(im, TEX, "wood.png")


# ---------------------------------------------------------------- pickups
def crystal():
    im = Image.new("RGBA", (64, 64), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    glow = Image.new("RGBA", (64, 64), (0, 0, 0, 0))
    ImageDraw.Draw(glow).ellipse([12, 8, 52, 60], fill=(60, 200, 255, 90))
    im = Image.alpha_composite(im, glow.filter(ImageFilter.GaussianBlur(6)))
    d = ImageDraw.Draw(im)
    d.polygon([(32, 6), (46, 26), (32, 60), (18, 26)], fill=(70, 210, 255, 255))
    d.polygon([(32, 6), (32, 60), (18, 26)], fill=(40, 150, 220, 255))
    d.polygon([(32, 6), (46, 26), (32, 26)], fill=(180, 245, 255, 255))
    d.line([(18, 26), (46, 26)], fill=(210, 250, 255, 255))
    d.polygon([(32, 6), (46, 26), (32, 60), (18, 26)], outline=(10, 70, 120, 255))
    return im


def potion():
    im = Image.new("RGBA", (64, 64), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    d.rectangle([27, 12, 37, 26], fill=(190, 210, 220, 200))
    d.rectangle([25, 8, 39, 13], fill=(120, 80, 40, 255))
    d.ellipse([14, 22, 50, 60], fill=(200, 30, 50, 255), outline=(90, 10, 20, 255), width=2)
    d.ellipse([20, 28, 30, 38], fill=(255, 150, 160, 255))
    d.line([20, 46, 44, 46], fill=(150, 20, 40, 255))
    return im


def acid():
    im = Image.new("RGBA", (64, 64), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    d.ellipse([2, 46, 62, 63], fill=(60, 200, 40, 230), outline=(20, 90, 10, 255), width=2)
    d.ellipse([10, 49, 40, 57], fill=(140, 255, 90, 230))
    for x, y, rr in ((18, 40, 3), (40, 34, 2), (30, 26, 2), (48, 44, 3)):
        d.ellipse([x - rr, y - rr, x + rr, y + rr], outline=(150, 255, 100, 255))
    return im


def portal():
    im = Image.new("RGBA", (64, 64), (0, 0, 0, 0))
    glow = Image.new("RGBA", (64, 64), (0, 0, 0, 0))
    ImageDraw.Draw(glow).ellipse([6, 2, 58, 63], fill=(170, 80, 255, 140))
    im = Image.alpha_composite(im, glow.filter(ImageFilter.GaussianBlur(4)))
    d = ImageDraw.Draw(im)
    for i, c in enumerate([(120, 40, 200), (170, 90, 255), (220, 170, 255), (255, 255, 255)]):
        d.ellipse([12 + i * 5, 6 + i * 6, 52 - i * 5, 62 - i * 6], outline=c + (255,), width=3)
    return im


save(crystal(), SPR, "crystal.png")
save(potion(), SPR, "potion.png")
save(acid(), SPR, "acid.png")
save(portal(), SPR, "portal.png")


def glow_layer(size, box, rgba, blur):
    """Soft glow: blur only the alpha mask over a solid colour (no dark fringe)."""
    mask = Image.new("L", size, 0)
    ImageDraw.Draw(mask).ellipse(box, fill=rgba[3])
    g = Image.new("RGBA", size, rgba[:3] + (0,))
    g.putalpha(mask.filter(ImageFilter.GaussianBlur(blur)))
    return g



# ---------------------------------------------------------------- guardian atlas (3x2)
def guardian(phase, attack=False):
    s = 128
    im = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    gc = (255, 80, 60, 120) if attack else (140, 90, 255, 110)
    im = Image.alpha_composite(im, glow_layer((s, s), [18, 10, 110, 120], gc, 10))
    d = ImageDraw.Draw(im)
    body = (90, 60, 160, 255) if not attack else (150, 50, 90, 255)
    sway = int(6 * math.sin(phase))
    # hooded body tapering into wisps
    d.polygon([(64, 12), (96, 50), (92 + sway, 104), (64, 92), (36 - sway, 104), (32, 50)], fill=body)
    for k in range(3):
        x = 44 + k * 20 + sway
        d.polygon([(x - 8, 96), (x + 8, 96), (x + int(4 * math.sin(phase + k)), 122)], fill=body)
    d.ellipse([42, 30, 86, 66], fill=(20, 10, 30, 255))  # hood opening
    eye = (255, 230, 80, 255) if not attack else (255, 60, 40, 255)
    d.ellipse([50, 42, 59, 50], fill=eye)
    d.ellipse([69, 42, 78, 50], fill=eye)
    if attack:
        d.line([20, 60, 4, 40], fill=(255, 120, 80, 255), width=5)
        d.line([108, 60, 124, 40], fill=(255, 120, 80, 255), width=5)
    return im


atlas = Image.new("RGBA", (128 * 3, 128 * 2), (0, 0, 0, 0))
frames = [guardian(0), guardian(1.0), guardian(2.2), guardian(3.4), guardian(0.5, True), guardian(2.0, True)]
for i, f in enumerate(frames):
    atlas.paste(f, ((i % 3) * 128, (i // 3) * 128))
save(atlas, ROOT, "straznik.png")


# ---------------------------------------------------------------- first-person wand
def hand(glow=0.0, lower=0, flash=False):
    # The canvas leaves room around the crystal tip so the glow (radius up to
    # ~58 px plus blur) is never cut off at the image edge.
    w, h = 320, 320
    X, Y = 23, 84  # offset of the original 274x236 drawing inside the canvas
    im = Image.new("RGBA", (w, h), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    oy = lower + Y
    # sleeve + hand
    d.polygon([(150 + X, h), (236 + X, h), (196 + X, 150 + oy), (150 + X, 160 + oy)], fill=(60, 40, 90, 255))
    d.ellipse([140 + X, 128 + oy, 196 + X, 176 + oy], fill=(214, 168, 130, 255), outline=(120, 80, 60, 255), width=2)
    # wand shaft
    d.line([(168 + X, 150 + oy), (118 + X, 40 + oy)], fill=(96, 60, 30, 255), width=10)
    d.line([(166 + X, 148 + oy), (118 + X, 42 + oy)], fill=(140, 96, 52, 255), width=3)
    # fingers over the shaft
    for k in range(3):
        d.ellipse([150 + X + k * 6, 132 + oy + k * 9, 172 + X + k * 6, 148 + oy + k * 9], fill=(224, 178, 140, 255))
    # crystal tip with glow
    tx, ty = 116 + X, 32 + oy
    r = int(18 + 14 * glow + (26 if flash else 0))
    gcol = (255, 255, 220, 200) if flash else (80, 220, 255, int(90 + 100 * glow))
    # Blur only the alpha mask over a solid glow colour: blurring an RGBA
    # layer on transparent black darkens the fringe into a dark ring.
    mask = Image.new("L", (w, h), 0)
    ImageDraw.Draw(mask).ellipse([tx - r, ty - r, tx + r, ty + r], fill=gcol[3])
    g = Image.new("RGBA", (w, h), gcol[:3] + (0,))
    g.putalpha(mask.filter(ImageFilter.GaussianBlur(6)))
    im = Image.alpha_composite(im, g)
    d = ImageDraw.Draw(im)
    d.polygon([(tx, ty - 18), (tx + 10, ty), (tx, ty + 14), (tx - 10, ty)], fill=(120, 230, 255, 255), outline=(20, 90, 140, 255))
    a = im.getchannel("A")
    edges = [a.crop((0, 0, w, 1)), a.crop((0, 0, 1, h)), a.crop((w - 1, 0, w, h))]
    assert max(max(e.getdata()) for e in edges) == 0, "wand frame art touches the image edge"
    return im


for i in range(5):
    save(hand(glow=0.5 + 0.5 * math.sin(i / 5 * 2 * math.pi)), HAND, f"idle_{i}.png")
for i in range(4):
    save(hand(glow=1.0, lower=-6 + i * 2, flash=i < 2), HAND, f"shoot_{i}.png")
for i, low in enumerate([10, 30, 50, 60, 50, 30, 10]):
    save(hand(glow=0.2, lower=low), HAND, f"reload_{i}.png")


# ================================================================ campaign assets
def noise_fill(im, base, spread, seed, count=900):
    r = random.Random(seed)
    px = im.load()
    w, h = im.size
    for _ in range(count):
        x, y = r.randrange(w), r.randrange(h)
        k = r.randint(-spread, spread)
        px[x, y] = tuple(max(0, min(255, v + k)) for v in base) + (255,)


def tiles(base, grout, seed, n=4, jitter=14):
    r = random.Random(seed)
    im = Image.new("RGBA", (64, 64), grout + (255,))
    d = ImageDraw.Draw(im)
    s = 64 // n
    for ty in range(n):
        for tx in range(n):
            k = r.randint(-jitter, jitter)
            c = tuple(max(0, min(255, v + k)) for v in base) + (255,)
            d.rectangle([tx * s + 1, ty * s + 1, tx * s + s - 2, ty * s + s - 2], fill=c)
    return im


def rock(base, seed, cracks=(0, 0, 0)):
    r = random.Random(seed)
    im = Image.new("RGBA", (64, 64), base + (255,))
    noise_fill(im, base, 22, seed, 1600)
    d = ImageDraw.Draw(im)
    for _ in range(6):
        x, y = r.randrange(64), r.randrange(64)
        pts = [(x, y)]
        for _ in range(4):
            x, y = (x + r.randint(-10, 10)) % 64, (y + r.randint(-10, 10)) % 64
            pts.append((x, y))
        d.line(pts, fill=cracks + (255,), width=1)
    return im


def theme(dirname, walls, floor, ceiling):
    out = os.path.join(TEX, dirname) if dirname else TEX
    os.makedirs(out, exist_ok=True)
    for name, img in walls.items():
        save(img, out, name + ".png")
    save(floor, out, "floor.png")
    save(ceiling, out, "ceiling.png")


# Default crypt set: existing walls + flagstone floor and dark vault ceiling.
theme("", {}, tiles((66, 62, 78), (30, 28, 36), 11, n=2, jitter=10), rock((24, 22, 32), 12, (12, 10, 18)))


# Ice graves
def ice_blocks(seed):
    im = crypt_blocks((150, 190, 225), (90, 120, 160), seed, rows=4)
    d = ImageDraw.Draw(im)
    r = random.Random(seed)
    for _ in range(8):
        x = r.randrange(64)
        d.line([x, 0, x + r.randint(-6, 6), 63], fill=(220, 240, 255, 255))
    return im


im = Image.new("RGBA", (64, 64), (120, 170, 210, 255))
noise_fill(im, (120, 170, 210), 18, 31, 1200)
d = ImageDraw.Draw(im)
for cx in (16, 48):
    d.polygon([(cx, 8), (cx + 10, 32), (cx, 56), (cx - 10, 32)], fill=(200, 240, 255, 255), outline=(80, 130, 190, 255))
ice_rune = im
im = Image.new("RGBA", (64, 64), (70, 100, 140, 255))
d = ImageDraw.Draw(im)
for x in range(0, 64, 8):
    d.rectangle([x, 0, x + 6, 63], fill=(90, 125, 165, 255))
d.rectangle([0, 28, 63, 34], fill=(200, 230, 250, 255))
d.ellipse([26, 24, 38, 38], outline=(255, 255, 255, 255), width=2)
ice_door = im
ceil = rock((20, 30, 50), 33, (60, 90, 130))
d = ImageDraw.Draw(ceil)
for x in range(4, 64, 12):
    d.polygon([(x, 0), (x + 4, 0), (x + 2, 10)], fill=(180, 220, 250, 255))
theme("lod", {"brick": ice_blocks(30), "stone": rock((110, 140, 170), 32, (70, 95, 125)), "metal": ice_rune, "wood": ice_door},
      tiles((190, 215, 235), (130, 160, 190), 34, n=4, jitter=8), ceil)


# Flooded mine (rust and timber)
def timber_rock(seed):
    im = rock((110, 70, 45), seed, (60, 35, 20))
    d = ImageDraw.Draw(im)
    d.rectangle([0, 0, 7, 63], fill=(95, 60, 30, 255))
    d.rectangle([56, 0, 63, 63], fill=(95, 60, 30, 255))
    d.rectangle([0, 0, 63, 7], fill=(110, 72, 38, 255))
    return im


im = Image.new("RGBA", (64, 64), (120, 70, 40, 255))
noise_fill(im, (120, 70, 40), 25, 41, 1500)
d = ImageDraw.Draw(im)
for y in (8, 40):
    d.rectangle([0, y, 63, y + 16], outline=(70, 40, 20, 255), width=2)
    for x in range(6, 64, 14):
        d.ellipse([x, y + 6, x + 4, y + 10], fill=(160, 120, 80, 255))
rust_plate = im
im = Image.new("RGBA", (64, 64), (80, 52, 28, 255))
d = ImageDraw.Draw(im)
for y in range(0, 64, 8):
    d.rectangle([0, y, 63, y + 6], fill=(100, 66, 36, 255))
d.line([0, 0, 63, 63], fill=(60, 38, 20, 255), width=5)
d.line([63, 0, 0, 63], fill=(60, 38, 20, 255), width=5)
mine_door = im
gravel = Image.new("RGBA", (64, 64), (70, 58, 48, 255))
noise_fill(gravel, (70, 58, 48), 30, 43, 2600)
ceil = rock((35, 25, 20), 44, (20, 12, 8))
d = ImageDraw.Draw(ceil)
d.rectangle([0, 28, 63, 36], fill=(90, 58, 30, 255))
theme("kopalnia", {"brick": rock((125, 80, 50), 40, (70, 40, 20)), "stone": timber_rock(42), "metal": rust_plate, "wood": mine_door},
      gravel, ceil)


# Heart of the crypt (obsidian with red crystal veins)
def veins(base, vein, seed):
    im = rock(base, seed, base)
    d = ImageDraw.Draw(im)
    r = random.Random(seed)
    for _ in range(5):
        x, y = r.randrange(64), r.randrange(64)
        pts = [(x, y)]
        for _ in range(5):
            x, y = (x + r.randint(-12, 12)) % 64, (y + r.randint(-12, 12)) % 64
            pts.append((x, y))
        d.line(pts, fill=vein + (255,), width=2)
    return im


im = Image.new("RGBA", (64, 64), (40, 20, 25, 255))
d = ImageDraw.Draw(im)
d.rectangle([3, 3, 60, 60], outline=(200, 160, 60, 255), width=3)
d.polygon([(32, 10), (50, 32), (32, 54), (14, 32)], outline=(255, 210, 90, 255), width=3)
d.ellipse([27, 27, 37, 37], fill=(255, 60, 60, 255))
gold_rune = im
im = Image.new("RGBA", (64, 64), (200, 190, 170, 255))
noise_fill(im, (200, 190, 170), 20, 52, 900)
d = ImageDraw.Draw(im)
for y in range(6, 64, 14):
    d.line([4, y, 60, y], fill=(150, 140, 120, 255), width=3)
d.ellipse([24, 22, 40, 40], outline=(120, 20, 20, 255), width=3)
bone_door = im
floor = tiles((70, 20, 28), (25, 8, 12), 53, n=4, jitter=12)
ceil = rock((15, 8, 10), 54, (60, 10, 15))
d = ImageDraw.Draw(ceil)
for x, y in ((10, 12), (40, 20), (22, 46), (52, 50)):
    d.ellipse([x, y, x + 3, y + 3], fill=(255, 70, 70, 255))
theme("serce", {"brick": veins((30, 22, 30), (200, 30, 50), 50), "stone": veins((15, 12, 18), (120, 20, 40), 51),
                "metal": gold_rune, "wood": bone_door}, floor, ceil)


# --- pickups: keys and relic
def key(color, dark):
    im = Image.new("RGBA", (64, 64), (0, 0, 0, 0))
    im = Image.alpha_composite(im, glow_layer((64, 64), [8, 8, 56, 56], color + (90,), 6))
    d = ImageDraw.Draw(im)
    d.ellipse([12, 14, 34, 36], outline=color + (255,), width=6)
    d.ellipse([18, 20, 28, 30], outline=dark + (255,), width=1)
    d.line([31, 31, 52, 52], fill=color + (255,), width=6)
    d.line([44, 44, 50, 38], fill=color + (255,), width=5)
    d.line([50, 50, 56, 44], fill=color + (255,), width=5)
    return im


save(key((230, 60, 60), (120, 20, 20)), SPR, "klucz_czerwony.png")
save(key((70, 140, 255), (20, 50, 130)), SPR, "klucz_niebieski.png")

im = Image.new("RGBA", (64, 64), (0, 0, 0, 0))
im = Image.alpha_composite(im, glow_layer((64, 64), [6, 6, 58, 58], (255, 220, 120, 110), 7))
d = ImageDraw.Draw(im)
d.polygon([(32, 8), (44, 22), (40, 52), (24, 52), (20, 22)], fill=(230, 190, 80, 255), outline=(120, 80, 20, 255))
d.ellipse([27, 24, 37, 34], fill=(200, 40, 60, 255))
d.rectangle([22, 50, 42, 56], fill=(150, 110, 40, 255))
save(im, SPR, "relikwia.png")


# --- spider atlas (3x2): idle, 3 walk frames, 2 attack frames
def spider(phase, attack=False):
    s = 128
    im = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    body = (60, 50, 70, 255) if not attack else (110, 40, 50, 255)
    leg = (35, 30, 40, 255)
    for k in range(4):
        a = 0.5 * math.sin(phase + k * 1.3)
        for side in (-1, 1):
            x0, y0 = 64 + side * 14, 76 + k * 6 - 8
            x1, y1 = 64 + side * (40 + 6 * a * side), 58 + k * 10
            x2, y2 = 64 + side * (56 + 4 * a), 112 - k * 2
            d.line([x0, y0, x1, y1, x2, y2], fill=leg, width=5)
    d.ellipse([40, 62, 88, 104], fill=body)          # abdomen
    d.ellipse([48, 48, 80, 74], fill=body)           # head
    eye = (255, 240, 120, 255) if not attack else (255, 70, 60, 255)
    for ex in (56, 62, 66, 72):
        d.ellipse([ex - 2, 56, ex + 2, 60], fill=eye)
    if attack:
        d.polygon([(58, 74), (62, 86), (64, 74)], fill=(230, 230, 230, 255))
        d.polygon([(64, 74), (66, 86), (70, 74)], fill=(230, 230, 230, 255))
    return im


atlas = Image.new("RGBA", (128 * 3, 128 * 2), (0, 0, 0, 0))
for i, f in enumerate([spider(0), spider(1.2), spider(2.4), spider(3.6), spider(0.6, True), spider(2.0, True)]):
    atlas.paste(f, ((i % 3) * 128, (i // 3) * 128))
save(atlas, SPR, "pajak.png")


# --- boss: the Crypt Warden (crowned, armoured, red core)
def warden(phase, attack=False):
    s = 192
    im = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    gc = (255, 60, 40, 140) if attack else (200, 40, 80, 110)
    im = Image.alpha_composite(im, glow_layer((s, s), [20, 10, 172, 186], gc, 14))
    d = ImageDraw.Draw(im)
    sway = int(8 * math.sin(phase))
    armour = (70, 60, 80, 255) if not attack else (110, 50, 60, 255)
    d.polygon([(96, 30), (150, 80), (140 + sway, 170), (96, 150), (52 - sway, 170), (42, 80)], fill=armour, outline=(30, 25, 35, 255))
    d.polygon([(70, 34), (78, 12), (88, 28), (96, 6), (104, 28), (114, 12), (122, 34)], fill=(230, 190, 70, 255))
    d.ellipse([70, 40, 122, 88], fill=(25, 15, 25, 255))
    eye = (255, 220, 80, 255) if not attack else (255, 50, 40, 255)
    d.ellipse([80, 56, 92, 66], fill=eye)
    d.ellipse([100, 56, 112, 66], fill=eye)
    d.ellipse([84, 100, 108, 124], fill=(255, 60, 70, 255), outline=(255, 180, 180, 255), width=2)
    if attack:
        d.line([40, 90, 8, 50], fill=(255, 120, 80, 255), width=8)
        d.line([152, 90, 184, 50], fill=(255, 120, 80, 255), width=8)
    return im


atlas = Image.new("RGBA", (192 * 3, 192 * 2), (0, 0, 0, 0))
for i, f in enumerate([warden(0), warden(1.0), warden(2.2), warden(3.4), warden(0.5, True), warden(2.0, True)]):
    atlas.paste(f, ((i % 3) * 192, (i // 3) * 192))
save(atlas, SPR, "straznik_krypty.png")


print("assets written to", ROOT)
