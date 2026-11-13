package parser

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Manifest describes project-level entry points and asset locations.
// It is intentionally simple: one directive per line, e.g. `MAIN_SCRIPT scripts/level1.lua`.
type Manifest struct {
	Project      string
	Engine       string
	Created      string
	Format       string
	Config       string
	MainMap      string
	MainScript   string
	HUDScript    string
	PlayerAssets string
	PlayerAtlas  string
	EnemyAtlas   string
	Menu         string       // MENU menu.aie: UI/menu definition (optional)
	Levels       []LevelEntry // LEVEL lines, in order (optional)
	Raw          map[string]string
}

// LevelEntry is a "LEVEL <id> <map path> [title...]" manifest line.
type LevelEntry struct {
	ID    string
	Path  string
	Title string // defaults to ID
}

func DefaultManifest() *Manifest {
	return &Manifest{
		Project:      "Demo",
		Engine:       "aiengine 1.0",
		Format:       "project v1",
		Config:       "config.aie",
		MainMap:      "maps/level1.aie",
		MainScript:   "scripts/level1.lua",
		HUDScript:    "scripts/hud.js",
		PlayerAssets: "assets/sprites/player",
		PlayerAtlas:  "gracz.png",
		EnemyAtlas:   "przeciwnik.png",
		Raw:          make(map[string]string),
	}
}

func ParseManifest(r io.Reader) (*Manifest, error) {
	m := DefaultManifest()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.EqualFold(line, "MANIFEST") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		directive := strings.ToUpper(fields[0])
		value := strings.Join(fields[1:], " ")
		switch directive {
		case "PROJECT":
			m.Project = value
		case "ENGINE":
			m.Engine = value
		case "CREATED":
			m.Created = value
		case "FORMAT":
			m.Format = value
		case "CONFIG":
			m.Config = value
		case "MAIN_MAP":
			m.MainMap = value
		case "MAIN_SCRIPT":
			m.MainScript = value
		case "HUD_SCRIPT":
			m.HUDScript = value
		case "PLAYER_ASSETS":
			m.PlayerAssets = value
		case "PLAYER_ATLAS":
			m.PlayerAtlas = value
		case "ENEMY_ATLAS":
			m.EnemyAtlas = value
		case "MENU":
			m.Menu = value
		case "LEVEL":
			if len(fields) < 3 {
				return nil, fmt.Errorf("LEVEL needs <id> <map path> [title]: %q", line)
			}
			e := LevelEntry{ID: fields[1], Path: fields[2], Title: strings.Join(fields[3:], " ")}
			if e.Title == "" {
				e.Title = e.ID
			}
			m.Levels = append(m.Levels, e)
		default:
			m.Raw[directive] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	return m, nil
}

// LevelList returns the declared levels, or MAIN_MAP as a single level.
func (m *Manifest) LevelList() []LevelEntry {
	if len(m.Levels) > 0 {
		return m.Levels
	}
	return []LevelEntry{{ID: "main", Path: m.MainMap, Title: m.Project}}
}

// FindLevel returns the level with the given id.
func (m *Manifest) FindLevel(id string) (LevelEntry, bool) {
	for _, l := range m.LevelList() {
		if strings.EqualFold(l.ID, id) {
			return l, true
		}
	}
	return LevelEntry{}, false
}

func ParseManifestFile(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening manifest file %s: %w", path, err)
	}
	defer f.Close()
	return ParseManifest(f)
}
