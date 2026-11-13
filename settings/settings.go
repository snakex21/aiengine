// Package settings holds player options (mouse, audio, video) and persists
// them as a small text file next to the game (settings.aie in the project
// directory), never in a per-user profile folder.
//
// File format, one option per line (unknown lines are ignored):
//
//	MOUSE_SENSITIVITY 1.0
//	INVERT_Y no
//	MASTER_VOLUME 1.0
//	MUSIC_VOLUME 0.7
//	SFX_VOLUME 1.0
//	RESOLUTION 1280x720
//	FULLSCREEN no
//	FOV 66
package settings

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FileName is the settings file created in the project directory.
const FileName = "settings.aie"

// Resolutions offered by the settings menu (internal render size).
var Resolutions = [][2]int{{960, 540}, {1280, 720}, {1600, 900}, {1920, 1080}}

// Settings are the player-adjustable options.
type Settings struct {
	MouseSensitivity float64 // multiplier, 0.2..3
	InvertY          bool
	MasterVolume     float64 // 0..1
	MusicVolume      float64 // 0..1
	SfxVolume        float64 // 0..1
	Width, Height    int
	Fullscreen       bool
	FOV              float64 // degrees, 50..110
}

// Defaults returns the defaults for a window of w×h.
func Defaults(w, h int) Settings {
	return Settings{
		MouseSensitivity: 1, MasterVolume: 1, MusicVolume: 0.7, SfxVolume: 1,
		Width: w, Height: h, FOV: 66,
	}
}

// Clamp keeps all values in their valid ranges.
func (s *Settings) Clamp() {
	s.MouseSensitivity = clamp(s.MouseSensitivity, 0.2, 3)
	s.MasterVolume = clamp(s.MasterVolume, 0, 1)
	s.MusicVolume = clamp(s.MusicVolume, 0, 1)
	s.SfxVolume = clamp(s.SfxVolume, 0, 1)
	s.FOV = clamp(s.FOV, 50, 110)
	if s.Width < 320 || s.Height < 200 {
		s.Width, s.Height = 1280, 720
	}
}

func clamp(v, lo, hi float64) float64 {
	if v != v || v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Load reads path on top of def. A missing file is not an error.
func Load(path string, def Settings) (Settings, error) {
	s := def
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		key, val := strings.ToUpper(fields[0]), fields[1]
		num, numErr := strconv.ParseFloat(val, 64)
		switch key {
		case "MOUSE_SENSITIVITY":
			if numErr == nil {
				s.MouseSensitivity = num
			}
		case "INVERT_Y":
			s.InvertY = parseBool(val)
		case "MASTER_VOLUME":
			if numErr == nil {
				s.MasterVolume = num
			}
		case "MUSIC_VOLUME":
			if numErr == nil {
				s.MusicVolume = num
			}
		case "SFX_VOLUME":
			if numErr == nil {
				s.SfxVolume = num
			}
		case "RESOLUTION":
			var w, h int
			if _, err := fmt.Sscanf(strings.ToLower(val), "%dx%d", &w, &h); err == nil {
				s.Width, s.Height = w, h
			}
		case "FULLSCREEN":
			s.Fullscreen = parseBool(val)
		case "FOV":
			if numErr == nil {
				s.FOV = num
			}
		}
	}
	s.Clamp()
	return s, sc.Err()
}

func parseBool(v string) bool {
	switch strings.ToLower(v) {
	case "yes", "true", "1", "on", "tak":
		return true
	}
	return false
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// Save writes the settings atomically (temp file + rename).
func (s Settings) Save(path string) error {
	s.Clamp()
	var b strings.Builder
	b.WriteString("# AIEngine settings - written by the game, safe to edit\n")
	fmt.Fprintf(&b, "MOUSE_SENSITIVITY %.2f\n", s.MouseSensitivity)
	fmt.Fprintf(&b, "INVERT_Y %s\n", yesNo(s.InvertY))
	fmt.Fprintf(&b, "MASTER_VOLUME %.2f\n", s.MasterVolume)
	fmt.Fprintf(&b, "MUSIC_VOLUME %.2f\n", s.MusicVolume)
	fmt.Fprintf(&b, "SFX_VOLUME %.2f\n", s.SfxVolume)
	fmt.Fprintf(&b, "RESOLUTION %dx%d\n", s.Width, s.Height)
	fmt.Fprintf(&b, "FULLSCREEN %s\n", yesNo(s.Fullscreen))
	fmt.Fprintf(&b, "FOV %.0f\n", s.FOV)
	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
