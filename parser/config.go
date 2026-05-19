// parser/config.go — Config file parser for AIEngine.
// Handles config.aie format: keybindings (BIND), server settings (SERVER), and connection settings (CONNECT).
// All values are loaded at startup and accessible through the Config struct.

package parser

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Config holds all parsed settings from a config.aie file.
type Config struct {
	Bindings  []Binding // keybinding entries
	Server    ServerConfig
	Client    ClientConfig

	// Window settings.
	WindowWidth  int // default 640
	WindowHeight int // default 480

	Raw map[string]string // catch-all for unknown directives
}

// Binding represents a single BIND directive: BIND <action> <key>.
type Binding struct {
	Action string
	Key    string
}

// ServerConfig holds server-related settings.
type ServerConfig struct {
	Port      int
	MaxPlayers int
	Mode      string
	Map       string
}

// ClientConfig holds client connection settings.
type ClientConfig struct {
	Address string
	Port    int
}

// ParseConfig reads a config.aie file from a reader and returns the parsed Config.
func ParseConfig(r io.Reader) (*Config, error) {
	cfg := &Config{
		Raw: make(map[string]string),
	}

	scanner := bufio.NewScanner(r)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		directive := strings.ToUpper(fields[0])

		switch directive {
		case "BIND":
			if len(fields) >= 3 {
				cfg.Bindings = append(cfg.Bindings, Binding{
					Action: fields[1],
					Key:    fields[2],
				})
			}

		case "SERVER":
			if len(fields) >= 3 {
				param := strings.ToLower(fields[1])
				value := fields[2]
				switch param {
				case "port":
					cfg.Server.Port, _ = strconv.Atoi(value)
				case "max_players":
					cfg.Server.MaxPlayers, _ = strconv.Atoi(value)
				case "mode":
					cfg.Server.Mode = value
				case "map":
					cfg.Server.Map = value
				default:
					cfg.Raw[fmt.Sprintf("SERVER %s", param)] = value
				}
			}

		case "CONNECT":
			if len(fields) >= 3 {
				cfg.Client.Address = fields[1]
				cfg.Client.Port, _ = strconv.Atoi(fields[2])
			}

		case "WINDOW_WIDTH":
			if len(fields) >= 2 {
				cfg.WindowWidth, _ = strconv.Atoi(fields[1])
			}

		case "WINDOW_HEIGHT":
			if len(fields) >= 2 {
				cfg.WindowHeight, _ = strconv.Atoi(fields[1])
			}

		default:
			// Store unknown directives in Raw for extensibility.
			cfg.Raw[directive] = strings.Join(fields[1:], " ")
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	return cfg, nil
}

// ParseConfigFile opens and parses a config.aie file by path.
func ParseConfigFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening config file %s: %w", path, err)
	}
	defer f.Close()
	return ParseConfig(f)
}

// GetBinding returns the key bound to an action, or empty string if not found.
func (c *Config) GetBinding(action string) string {
	for _, b := range c.Bindings {
		if strings.EqualFold(b.Action, action) {
			return b.Key
		}
	}
	return ""
}

// GetAllBindings returns all bindings for a given action name (rarely multiple).
func (c *Config) GetAllBindings(action string) []string {
	var result []string
	for _, b := range c.Bindings {
		if strings.EqualFold(b.Action, action) {
			result = append(result, b.Key)
		}
	}
	return result
}

// HasBinding returns true if an action has any binding defined.
func (c *Config) HasBinding(action string) bool {
	return c.GetBinding(action) != ""
}

// GetWindowWidth returns window width with fallback default.
func (c *Config) GetWindowWidth() int {
	if c.WindowWidth < 320 {
		return 640
	}
	return c.WindowWidth
}

// GetWindowHeight returns window height with fallback default.
func (c *Config) GetWindowHeight() int {
	if c.WindowHeight < 240 {
		return 480
	}
	return c.WindowHeight
}
