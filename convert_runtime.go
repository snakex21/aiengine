// convert_runtime.go - the --grid2sectors command: GRID map file to a
// sector map file.

package main

import (
	"fmt"
	"os"

	"aiengine/level"
)

// convertGridMap writes the sector version of a GRID map file.
func convertGridMap(in, out string) error {
	src, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	res, err := level.GridToSectors(string(src))
	if err != nil {
		return fmt.Errorf("%s: %w", in, err)
	}
	return os.WriteFile(out, []byte(res), 0o644)
}
