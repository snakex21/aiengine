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
