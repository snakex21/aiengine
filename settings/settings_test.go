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

func TestNativeResolutionAndRenderScale(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	s := Defaults(1280, 720)
	s.Width, s.Height, s.RenderScale = 0, 0, 0.5
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	got, _ := Load(path, Defaults(1280, 720))
	if !got.Native() || got.RenderScale != 0.5 {
		t.Fatalf("native/scale lost: %+v", got)
	}
	if w, h := got.Output(3840, 2160); w != 3840 || h != 2160 {
		t.Fatalf("native output %dx%d", w, h)
	}
	if w, h := got.RenderSize(3840, 2160); w != 1920 || h != 1080 {
		t.Fatalf("render size %dx%d", w, h)
	}
	s.Width, s.Height, s.RenderScale = 3440, 1440, 1
	if w, h := s.RenderSize(s.Output(0, 0)); w != 3440 || h != 1440 {
		t.Fatalf("ultrawide render %dx%d", w, h)
	}
	s.RenderScale = 9
	s.Clamp()
	if s.RenderScale != 2 {
		t.Fatalf("render scale clamp %v", s.RenderScale)
	}
}

func TestFullscreenModeAndBackgroundVolume(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	s := Defaults(1280, 720)
	if s.Exclusive || s.BackgroundVolume != 0 {
		t.Fatal("defaults must be borderless fullscreen and mute in background")
	}
	s.Exclusive, s.BackgroundVolume = true, 0.3
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	got, _ := Load(path, Defaults(1, 1))
	if !got.Exclusive || got.BackgroundVolume != 0.3 {
		t.Fatalf("got %+v", got)
	}
}
