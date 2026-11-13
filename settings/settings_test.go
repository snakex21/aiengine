package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	s := Defaults(1280, 720)
	s.MouseSensitivity, s.InvertY, s.MasterVolume, s.MusicVolume = 1.7, true, 0.5, 0.2
	s.SfxVolume, s.Width, s.Height, s.Fullscreen, s.FOV = 0.9, 1600, 900, true, 80
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, Defaults(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	if got != s {
		t.Fatalf("round trip:\n got %+v\nwant %+v", got, s)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file left behind")
	}
}

func TestLoadMissingAndGarbage(t *testing.T) {
	dir := t.TempDir()
	def := Defaults(1280, 720)
	if got, err := Load(filepath.Join(dir, "none.aie"), def); err != nil || got != def {
		t.Fatalf("missing file: %+v %v", got, err)
	}
	path := filepath.Join(dir, FileName)
	os.WriteFile(path, []byte("# c\nFOV abc\nMOUSE_SENSITIVITY 99\nRESOLUTION big\nINVERT_Y tak\nNOPE 1\n"), 0o644)
	got, err := Load(path, def)
	if err != nil {
		t.Fatal(err)
	}
	if got.FOV != 66 || got.MouseSensitivity != 3 || got.Width != 1280 || !got.InvertY {
		t.Fatalf("garbage handling: %+v", got)
	}
}
