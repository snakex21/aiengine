package level

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aiengine/parser"
)

// Every project that opts into the level runtime must have a loadable
// MAIN_MAP, so a broken map fails `go test ./...` instead of the game launch.
func TestLevelRuntimeProjectsLoad(t *testing.T) {
	dirs, _ := filepath.Glob(filepath.Join("..", "projects", "*", "manifest.aie"))
	checked := 0
	for _, mf := range dirs {
		m, err := parser.ParseManifestFile(mf)
		if err != nil {
			t.Errorf("%s: %v", mf, err)
			continue
		}
		if !strings.EqualFold(m.Raw["RUNTIME"], "level") {
			continue
		}
		dir := filepath.Dir(mf)
		lv, err := LoadFile(filepath.Join(dir, m.MainMap))
		if err != nil {
			t.Errorf("%s: %v", dir, err)
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, m.MainScript)); err != nil {
			t.Errorf("%s: MAIN_SCRIPT: %v", dir, err)
		}
		if lv.Map.Width == 0 {
			t.Errorf("%s: empty map", dir)
		}
		checked++
	}
	t.Logf("checked %d level-runtime project(s)", checked)
}
