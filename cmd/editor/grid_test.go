package main

import (
	"strings"
	"testing"
)

const heightMap = `MAP h
GRID
W W W
W P W
W W W
FLOOR_GRID
. . .
. 2 .
. . .
CEILING_GRID
. . .
. G .
. . .
PLACE item x 1.5 0 1.5
`

func TestEditorKeepsHeightGrids(t *testing.T) {
	rows := extractGridRows(heightMap)
	if len(rows) != 3 {
		t.Fatalf("GRID rows: %v", rows)
	}
	out := replaceMapGrid(heightMap, "h", []string{"W W W", "W P .", "W W W"})
	for _, want := range []string{"W P .", "FLOOR_GRID\n. . .\n. 2 .", "CEILING_GRID\n. . .\n. G .", "PLACE item x"} {
		if !strings.Contains(out, want) {
			t.Errorf("saved map lost %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "W P W") {
		t.Error("old GRID row kept")
	}
}

const animMap = `MAP a
DOOR_STYLE slide
TAGS w=winda
TEX_SYMBOLS l=lawa
GRID
W W W W
W P . W
W W W W
TAG_GRID
. . . .
. . w .
. . . .
FLOOR_TEX_GRID
. . . .
. l . .
. . . .
MOVER lift winda low=0 wait=2
MOVER crusher c start=on damage=12
TEXTURE_ANIM lawa frames=lawa_0,lawa_1 fps=3 scroll=0,0.1
PLACE switch dzwignia d1 1.5 0.3 1.2 tag=c action=stop
`

func TestEditorKeepsAnimationBlocks(t *testing.T) {
	if rows := extractGridRows(animMap); len(rows) != 3 {
		t.Fatalf("GRID rows: %v", rows)
	}
	out := replaceMapGrid(animMap, "a", []string{"W W W W", "W P W W", "W W W W"})
	for _, want := range []string{
		"DOOR_STYLE slide", "TAGS w=winda", "TEX_SYMBOLS l=lawa",
		"TAG_GRID\n. . . .\n. . w .\n. . . .", "FLOOR_TEX_GRID\n. . . .\n. l . .",
		"MOVER lift winda low=0 wait=2", "MOVER crusher c start=on damage=12",
		"TEXTURE_ANIM lawa frames=lawa_0,lawa_1 fps=3 scroll=0,0.1",
		"PLACE switch dzwignia d1 1.5 0.3 1.2 tag=c action=stop", "W P W W",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("saved map lost %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "GRID\n") != 3 { // GRID, TAG_GRID, FLOOR_TEX_GRID
		t.Errorf("grid blocks duplicated or lost:\n%s", out)
	}
}
