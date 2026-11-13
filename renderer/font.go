// renderer/font.go — Minimal 5×7 bitmap font for the in‑game console overlay.
// Rendered directly into the CPU pixel buffer.

package renderer

import "unicode/utf8"

// fontChar holds a 5×7 glyph as 7 rows; each row uses bits 5..1 (bit 5 =
// leftmost column, bit 0 unused), row 0 = top.
type fontChar [7]byte

// consoleFont provides a minimal character set for cheat commands.
var consoleFont = [128]fontChar{}

func init() {
	// Digits 0-9.
	consoleFont['0'] = fontChar{0x1C, 0x22, 0x26, 0x2A, 0x32, 0x22, 0x1C}
	consoleFont['1'] = fontChar{0x08, 0x18, 0x08, 0x08, 0x08, 0x08, 0x1C}
	consoleFont['2'] = fontChar{0x1C, 0x22, 0x02, 0x04, 0x08, 0x10, 0x3E}
	consoleFont['3'] = fontChar{0x1C, 0x22, 0x02, 0x0C, 0x02, 0x22, 0x1C}
	consoleFont['4'] = fontChar{0x04, 0x0C, 0x14, 0x24, 0x3E, 0x04, 0x04}
	consoleFont['5'] = fontChar{0x3E, 0x20, 0x3C, 0x02, 0x02, 0x22, 0x1C}
	consoleFont['6'] = fontChar{0x0C, 0x10, 0x20, 0x3C, 0x22, 0x22, 0x1C}
	consoleFont['7'] = fontChar{0x3E, 0x02, 0x04, 0x08, 0x10, 0x10, 0x10}
	consoleFont['8'] = fontChar{0x1C, 0x22, 0x22, 0x1C, 0x22, 0x22, 0x1C}
	consoleFont['9'] = fontChar{0x1C, 0x22, 0x22, 0x1E, 0x02, 0x04, 0x18}

	// Uppercase A-Z.
	consoleFont['A'] = fontChar{0x08, 0x14, 0x22, 0x22, 0x3E, 0x22, 0x22}
	consoleFont['B'] = fontChar{0x3C, 0x22, 0x22, 0x3C, 0x22, 0x22, 0x3C}
	consoleFont['C'] = fontChar{0x1C, 0x22, 0x20, 0x20, 0x20, 0x22, 0x1C}
	consoleFont['D'] = fontChar{0x38, 0x24, 0x22, 0x22, 0x22, 0x24, 0x38}
	consoleFont['E'] = fontChar{0x3E, 0x20, 0x20, 0x3C, 0x20, 0x20, 0x3E}
	consoleFont['F'] = fontChar{0x3E, 0x20, 0x20, 0x3C, 0x20, 0x20, 0x20}
	consoleFont['G'] = fontChar{0x1C, 0x22, 0x20, 0x2E, 0x22, 0x22, 0x1C}
	consoleFont['H'] = fontChar{0x22, 0x22, 0x22, 0x3E, 0x22, 0x22, 0x22}
	consoleFont['I'] = fontChar{0x1C, 0x08, 0x08, 0x08, 0x08, 0x08, 0x1C}
	consoleFont['J'] = fontChar{0x0E, 0x04, 0x04, 0x04, 0x04, 0x24, 0x18}
	consoleFont['K'] = fontChar{0x22, 0x24, 0x28, 0x30, 0x28, 0x24, 0x22}
	consoleFont['L'] = fontChar{0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x3E}
	consoleFont['M'] = fontChar{0x22, 0x36, 0x2A, 0x2A, 0x22, 0x22, 0x22}
	consoleFont['N'] = fontChar{0x22, 0x32, 0x2A, 0x26, 0x22, 0x22, 0x22}
	consoleFont['O'] = fontChar{0x1C, 0x22, 0x22, 0x22, 0x22, 0x22, 0x1C}
	consoleFont['P'] = fontChar{0x3C, 0x22, 0x22, 0x3C, 0x20, 0x20, 0x20}
	consoleFont['Q'] = fontChar{0x1C, 0x22, 0x22, 0x22, 0x2A, 0x24, 0x1A}
	consoleFont['R'] = fontChar{0x3C, 0x22, 0x22, 0x3C, 0x28, 0x24, 0x22}
	consoleFont['S'] = fontChar{0x1C, 0x22, 0x20, 0x1C, 0x02, 0x22, 0x1C}
	consoleFont['T'] = fontChar{0x3E, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08}
	consoleFont['U'] = fontChar{0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x1C}
	consoleFont['V'] = fontChar{0x22, 0x22, 0x22, 0x22, 0x14, 0x14, 0x08}
	consoleFont['W'] = fontChar{0x22, 0x22, 0x22, 0x2A, 0x2A, 0x36, 0x22}
	consoleFont['X'] = fontChar{0x22, 0x22, 0x14, 0x08, 0x14, 0x22, 0x22}
	consoleFont['Y'] = fontChar{0x22, 0x22, 0x14, 0x08, 0x08, 0x08, 0x08}
	consoleFont['Z'] = fontChar{0x3E, 0x02, 0x04, 0x08, 0x10, 0x20, 0x3E}

	// Lowercase (same as uppercase for simplicity).
	for c := 'A'; c <= 'Z'; c++ {
		consoleFont[c+('a'-'A')] = consoleFont[c]
	}

	// Symbols.
	consoleFont[' '] = fontChar{}
	consoleFont['>'] = fontChar{0x00, 0x10, 0x08, 0x04, 0x08, 0x10, 0x00}
	consoleFont['<'] = fontChar{0x00, 0x04, 0x08, 0x10, 0x08, 0x04, 0x00}
	consoleFont[':'] = fontChar{0x00, 0x00, 0x08, 0x00, 0x08, 0x00, 0x00}
	consoleFont['.'] = fontChar{0x00, 0x00, 0x00, 0x00, 0x00, 0x0C, 0x0C}
	consoleFont['-'] = fontChar{0x00, 0x00, 0x00, 0x3E, 0x00, 0x00, 0x00}
	consoleFont['_'] = fontChar{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3E}
	consoleFont['/'] = fontChar{0x02, 0x02, 0x04, 0x08, 0x10, 0x20, 0x20}
	consoleFont['['] = fontChar{0x1C, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1C}
	consoleFont[']'] = fontChar{0x1C, 0x04, 0x04, 0x04, 0x04, 0x04, 0x1C}
	consoleFont['?'] = fontChar{0x1C, 0x22, 0x02, 0x04, 0x08, 0x00, 0x08}
	consoleFont['!'] = fontChar{0x08, 0x08, 0x08, 0x08, 0x08, 0x00, 0x08}
	consoleFont[','] = fontChar{0x00, 0x00, 0x00, 0x00, 0x0C, 0x04, 0x08}
	consoleFont['\''] = fontChar{0x08, 0x08, 0x10, 0x00, 0x00, 0x00, 0x00}
	consoleFont['"'] = fontChar{0x14, 0x14, 0x00, 0x00, 0x00, 0x00, 0x00}
	consoleFont['('] = fontChar{0x04, 0x08, 0x10, 0x10, 0x10, 0x08, 0x04}
	consoleFont[')'] = fontChar{0x10, 0x08, 0x04, 0x04, 0x04, 0x08, 0x10}
	consoleFont['+'] = fontChar{0x00, 0x08, 0x08, 0x3E, 0x08, 0x08, 0x00}
	consoleFont['='] = fontChar{0x00, 0x00, 0x3E, 0x00, 0x3E, 0x00, 0x00}
	consoleFont['*'] = fontChar{0x00, 0x2A, 0x1C, 0x3E, 0x1C, 0x2A, 0x00}
	consoleFont['%'] = fontChar{0x30, 0x32, 0x04, 0x08, 0x10, 0x26, 0x06}
	consoleFont['#'] = fontChar{0x14, 0x14, 0x3E, 0x14, 0x3E, 0x14, 0x14}
}

// FontW / FontH are the character cell dimensions.
const FontW = 6
const FontH = 8

// FontAccentRows is how far accents (acute, dot) reach above a glyph's top row
// and ogonek/descenders below its 7th row. Leave this much room between lines
// of text that may contain Polish letters.
const FontAccentRows = 2

// glyph is a character with room for marks: rows 0-1 lie above the cap line
// (accents), rows 2-8 are the 7-row body, rows 9-10 are below (ogonek).
// Each row uses bits 5..1 like fontChar.
type glyph [11]byte

// Polish diacritics are composed from the base letter (the font is upper-case
// only, so lower-case letters map to the same shapes) plus a mark.
var (
	markAcute  = [2]byte{0x04, 0x08} // short stroke rising to the right
	markDot    = [2]byte{0x08, 0x00} // single dot, one row of air
	markOgonek = [2]byte{0x04, 0x02} // hook under the right side
)

var extraGlyphs = map[rune]glyph{}

func init() {
	above := func(base byte, mark [2]byte) glyph {
		var g glyph
		g[0], g[1] = mark[0], mark[1]
		copy(g[2:9], consoleFont[base][:])
		return g
	}
	below := func(base byte, mark [2]byte) glyph {
		var g glyph
		copy(g[2:9], consoleFont[base][:])
		g[9], g[10] = mark[0], mark[1]
		return g
	}
	var lStroke glyph // L with a diagonal stroke through the stem
	copy(lStroke[2:9], []byte{0x20, 0x20, 0x28, 0x30, 0x20, 0x20, 0x3E})

	pairs := map[[2]rune]glyph{
		{'Ą', 'ą'}: below('A', markOgonek),
		{'Ć', 'ć'}: above('C', markAcute),
		{'Ę', 'ę'}: below('E', markOgonek),
		{'Ł', 'ł'}: lStroke,
		{'Ń', 'ń'}: above('N', markAcute),
		{'Ó', 'ó'}: above('O', markAcute),
		{'Ś', 'ś'}: above('S', markAcute),
		{'Ź', 'ź'}: above('Z', markAcute),
		{'Ż', 'ż'}: above('Z', markDot),
	}
	for runes, g := range pairs {
		extraGlyphs[runes[0]] = g
		extraGlyphs[runes[1]] = g
	}
}

// lookupGlyph returns the glyph for r; unknown runes report false.
func lookupGlyph(r rune) (glyph, bool) {
	if r >= 0 && r < rune(len(consoleFont)) {
		var g glyph
		copy(g[2:9], consoleFont[r][:])
		return g, true
	}
	g, ok := extraGlyphs[r]
	return g, ok
}

// HasGlyph reports whether the font can draw r (space counts as drawable).
func HasGlyph(r rune) bool {
	g, ok := lookupGlyph(r)
	if !ok {
		return false
	}
	if r == ' ' {
		return true
	}
	for _, row := range g {
		if row != 0 {
			return true
		}
	}
	return false
}

// DrawConsoleString renders a string onto the pixel buffer at (px,py).
// Color must be a 4‑element array [R,G,B,A]. Text is UTF-8.
func DrawConsoleString(pixels []uint8, screenW int, px, py int, text string, color [4]uint8) {
	sh := 0
	if screenW > 0 {
		sh = len(pixels) / 4 / screenW
	}
	drawTextScaledRaw(pixels, screenW, sh, px, py, text, color, 1)
}

// TextWidth returns the pixel width of text drawn at the given scale.
func TextWidth(text string, scale int) int {
	if scale < 1 {
		scale = 1
	}
	return utf8.RuneCountInString(text) * FontW * scale
}

// DrawTextScaled draws text with each font pixel enlarged to scale×scale and a
// one-step dark drop shadow, clipped to the screen. py is the top of the
// glyph body; accents extend FontAccentRows*scale above and below it.
func DrawTextScaled(pixels []uint8, sw, sh, px, py int, text string, color [4]uint8, scale int) {
	if scale < 1 {
		scale = 1
	}
	shadow := [4]uint8{0, 0, 0, 255}
	drawTextScaledRaw(pixels, sw, sh, px+scale, py+scale, text, shadow, scale)
	drawTextScaledRaw(pixels, sw, sh, px, py, text, color, scale)
}

func drawTextScaledRaw(pixels []uint8, sw, sh, px, py int, text string, color [4]uint8, scale int) {
	i := 0
	for _, r := range text {
		g, ok := lookupGlyph(r)
		ox := px + i*FontW*scale
		i++
		if !ok {
			continue
		}
		for row := 0; row < len(g); row++ {
			bits := g[row]
			if bits == 0 {
				continue
			}
			for col := 0; col < FontW-1; col++ {
				if bits&(1<<(5-col)) == 0 {
					continue
				}
				for dy := 0; dy < scale; dy++ {
					y := py + (row-FontAccentRows)*scale + dy
					if y < 0 || y >= sh {
						continue
					}
					for dx := 0; dx < scale; dx++ {
						x := ox + col*scale + dx
						if x < 0 || x >= sw {
							continue
						}
						idx := (y*sw + x) * 4
						pixels[idx], pixels[idx+1], pixels[idx+2], pixels[idx+3] = color[0], color[1], color[2], color[3]
					}
				}
			}
		}
	}
}
