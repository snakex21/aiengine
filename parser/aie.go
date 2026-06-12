// Package parser implements parsers for AIEngine data formats: .aie, .css, and config.aie.
// parser/aie.go handles the .aie format — entities, maps, cutscenes, sounds, triggers.
// The format uses indentation-based nesting with three complexity levels.
// Entity types at indent 0 start new top-level nodes; properties at same indent belong
// to the preceding entity. Deeper indentation creates sub-blocks.

package parser

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// AIENode represents a single parsed element from a .aie file.
type AIENode struct {
	Type       string            // e.g. HUMANOID, MAP, GRID, TRIGGER, FRAME, CONDITION
	Name       string            // entity/block name (e.g. "grunt", "level1", "zone_clear")
	Properties map[string]string // key = value pairs at this node level
	Children   []*AIENode        // nested sub-blocks
	Values     []string          // raw value lines (e.g. GRID rows, TEXT lines)
	Line       int               // source line number (1-based)
}

// AIEDocument is the top-level result of parsing a .aie file.
type AIEDocument struct {
	Nodes []*AIENode
}

// entityKeywords is the set of top-level entity type identifiers.
// Note: TRIGGER, CUTSCENE, SOUND are handled as block keywords that attach to
// the current MAP (or stand alone when no entity is active).
var entityKeywords = map[string]bool{
	"HUMANOID": true, "ALIEN": true, "MUTANT": true, "ROBOT": true, "UNDEAD": true,
	"CREATURE": true, "QUADRUPED": true, "MULTI_LEG": true, "FLYING": true, "AQUATIC": true,
	"AMORPHOUS": true, "SERPENTINE": true, "GIANT": true,
	"VEHICLE": true, "PROP": true, "WEAPON": true, "MAP": true,
}

// blockKeywords is the set of keys that start a sub-block (containing children or values).
var blockKeywords = map[string]bool{
	"GRID": true, "GENERATE": true, "ZONE": true,
	"HP_PARTS": true, "HP_PHASES": true, "DAMAGE_PARTS": true,
	"BEHAVIOR": true, "ANIM": true, "MAP_PROPS": true,
	"PLACE":   true,
	"TRIGGER": true, "CUTSCENE": true, "SOUND": true,
}

// blockWithValueKeywords are keys that start a sub-block AND consume the rest of the line as Name.
// Note: HEAD/TORSO/etc. are handled contextually — children only inside HP_PARTS/HP_PHASES/DAMAGE_PARTS.
var blockWithValueKeywords = map[string]bool{
	"PHASE": true, "FRAME": true,
	"CONDITION": true, "ACTION": true,
	"TEXT": true, "IMAGE": true,
	"CAMERA_MOVE": true, "CAMERA_LOOK": true, "SPAWN": true, "ANIMATE": true,
	"SHAKE_CAMERA": true, "FREEZE_PLAYER": true, "UNFREEZE_PLAYER": true,
	"PLAY_SOUND": true, "WAIT": true, "FADE_IN": true, "FADE_OUT": true,
	"NEXT_ON_KEY": true, "LUA": true,
	"MUSIC":    true,
	"ENVELOPE": true, "WAVEFORM": true, "FILTER": true,
	"ON_DAMAGE": true, "ON_50HP": true, "ON_DEATH": true,
	"GRAVITY": true, "ROOMS": true, "CORRIDORS": true, "ENEMY_COUNT": true,
	"PROP_COUNT": true, "SEED": true,
	"SIZE": true, "RANDOMIZE": true, "CONTINUOUS": true, "VOLUME": true,
	"DECAY": true, "NOISE": true, "REVERB": true,
	"FREQUENCY": true, "TYPE": true,
	"LIGHT_POINT": true,
}

// bodyPartKeywords are keys that describe entity body parts and act as properties
// outside of HP_PARTS context, or as HP-value children within HP_PARTS.
var bodyPartKeywords = map[string]bool{
	"HEAD": true, "TORSO": true, "ARMS": true, "LEGS": true, "TAIL": true,
	"ARM_LEFT": true, "ARM_RIGHT": true, "LEG_LEFT": true, "LEG_RIGHT": true,
	"BODY": true, "WINGS": true, "FANGS": true, "EYES": true, "ANTENNA": true,
	"BODY_SEGMENTS": true, "TENTACLE": true, "EXTRA_EYES": true,
	"DEFORMED_HEAD": true, "EXTRA_ARM": true, "SENSOR": true, "JOINT_VISIBLE": true,
	"MISSING_ARM": true, "EXPOSED_BONE": true,
	"BARREL": true, "GRIP": true, "WHEELS": true,
}

// ParseAIE reads a .aie file from reader and returns the parsed document.
func ParseAIE(r io.Reader) (*AIEDocument, error) {
	lines, err := readLines(r)
	if err != nil {
		return nil, fmt.Errorf("reading .aie: %w", err)
	}

	doc := &AIEDocument{}
	stack := &nodeStack{}
	var currentEntity *AIENode // last top-level entity (may be at indent >0 for nested TRIGGER etc.)

	for i, raw := range lines {
		lineNo := i + 1
		trimmed := strings.TrimSpace(raw)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := countIndent(raw)
		parts := splitLine(trimmed)
		if len(parts) == 0 {
			continue
		}

		firstWord := parts[0]
		firstUpper := strings.ToUpper(firstWord)

		// --- Handle END (closes ANIM blocks) ---
		if firstUpper == "END" && indent >= 0 {
			// Pop until we find the matching ANIM block.
			for stack.depth() > 0 {
				entry := stack.pop()
				if strings.EqualFold(entry.node.Type, "ANIM") {
					break
				}
			}
			continue
		}

		// --- Pop stack for indentation changes (unless inside a GRID block) ---
		if stack.depth() > 0 && !strings.EqualFold(stack.peek().node.Type, "GRID") {
			for stack.depth() > 0 && stack.peek().indent >= indent {
				stack.pop()
			}
		}

		// --- If inside a GRID block, check if this line is grid data ---
		if stack.depth() > 0 && strings.EqualFold(stack.peek().node.Type, "GRID") {
			if isGridRow(trimmed) {
				stack.peek().node.Values = append(stack.peek().node.Values, trimmed)
				continue
			}
			// Not a grid row — end GRID mode.
			stack.pop()
		}

		// --- Indent 0: entity, block, or property of current entity ---
		if indent == 0 {
			if entityKeywords[firstUpper] {
				// New primary entity at top level.
				entityName := ""
				if len(parts) >= 2 {
					entityName = parts[1]
				}
				node := &AIENode{
					Type:       firstUpper,
					Name:       entityName,
					Properties: make(map[string]string),
					Line:       lineNo,
				}
				doc.Nodes = append(doc.Nodes, node)
				currentEntity = node
				stack.reset()
				stack.push(&stackEntry{node: node, indent: indent})
			} else if currentEntity != nil {
				// Property or block of the current entity.
				addPropertyOrChild(currentEntity, parts, firstUpper, trimmed, indent, stack, lineNo)
			} else if blockKeywords[firstUpper] || blockWithValueKeywords[firstUpper] {
				// Standalone block (TRIGGER, CUTSCENE, SOUND) — treat as entity.
				entityName := ""
				if len(parts) >= 2 {
					entityName = parts[1]
				}
				node := &AIENode{
					Type:       firstUpper,
					Name:       entityName,
					Properties: make(map[string]string),
					Line:       lineNo,
				}
				doc.Nodes = append(doc.Nodes, node)
				currentEntity = node
				stack.reset()
				stack.push(&stackEntry{node: node, indent: indent})
			}
			continue
		}

		// --- Indent > 0: child of whatever is on the stack ---
		if stack.depth() == 0 {
			// If no parent on stack but we have a current entity, attach to it.
			if currentEntity != nil {
				stack.push(&stackEntry{node: currentEntity, indent: 0})
			} else {
				return nil, fmt.Errorf("line %d: indented content without parent", lineNo)
			}
		}

		parent := stack.peek().node
		addPropertyOrChild(parent, parts, firstUpper, trimmed, indent, stack, lineNo)
	}

	return doc, nil
}

// addPropertyOrChild determines whether a line is a simple property or starts a new sub-block,
// then adds it to the parent node accordingly.
func addPropertyOrChild(parent *AIENode, parts []string, firstUpper string, trimmed string, indent int, stack *nodeStack, lineNo int) {
	// Single word line.
	if len(parts) == 1 {
		if blockKeywords[firstUpper] {
			child := &AIENode{
				Type:       firstUpper,
				Properties: make(map[string]string),
				Line:       lineNo,
			}
			parent.Children = append(parent.Children, child)
			stack.push(&stackEntry{node: child, indent: indent})
			return
		}
		if bodyPartKeywords[firstUpper] {
			parent.Properties[firstUpper] = ""
			return
		}
		parent.Properties[parts[0]] = ""
		return
	}

	// Multi-word line.
	key := parts[0]
	keyUpper := strings.ToUpper(key)
	value := strings.Join(parts[1:], " ")

	// Body part keywords inside HP_PARTS / HP_PHASES / DAMAGE_PARTS become children.
	// Outside those blocks, they are simple properties.
	if bodyPartKeywords[keyUpper] {
		parentType := strings.ToUpper(parent.Type)
		if parentType == "HP_PARTS" || parentType == "HP_PHASES" || parentType == "DAMAGE_PARTS" {
			child := &AIENode{
				Type:       keyUpper,
				Name:       value,
				Properties: make(map[string]string),
				Line:       lineNo,
			}
			parent.Children = append(parent.Children, child)
			return
		}
		// Outside HP context — simple property.
		parent.Properties[key] = value
		return
	}

	// Check if this is a block-with-value starter.
	if blockWithValueKeywords[keyUpper] {
		child := &AIENode{
			Type:       keyUpper,
			Name:       value,
			Properties: make(map[string]string),
			Line:       lineNo,
		}
		parent.Children = append(parent.Children, child)
		stack.push(&stackEntry{node: child, indent: indent})
		return
	}

	// Check if this is a block-without-value starter.
	if blockKeywords[keyUpper] {
		child := &AIENode{
			Type:       keyUpper,
			Name:       value,
			Properties: make(map[string]string),
			Line:       lineNo,
		}
		parent.Children = append(parent.Children, child)
		stack.push(&stackEntry{node: child, indent: indent})
		return
	}

	// GENERATE block content → properties.
	if strings.EqualFold(parent.Type, "GENERATE") {
		parent.Properties[key] = value
		return
	}

	// Default: simple key = value property.
	parent.Properties[key] = value
}

// isGridRow returns true if a line is a grid data row: a whitespace-separated
// sequence of single-character tile symbols (e.g. "W P . . S . W"). This is
// symbol-agnostic so AI-generated maps may use any single-rune tile markers,
// while real keywords (PLACE, TRIGGER, MUSIC, ...) are multi-character tokens
// and are correctly rejected.
func isGridRow(line string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	for _, f := range fields {
		if utf8.RuneCountInString(f) != 1 {
			return false
		}
	}
	return true
}

// --- Helper types ---

type stackEntry struct {
	node   *AIENode
	indent int
}

type nodeStack struct {
	entries []stackEntry
}

func (s *nodeStack) push(e *stackEntry) {
	s.entries = append(s.entries, *e)
}

func (s *nodeStack) pop() *stackEntry {
	if len(s.entries) == 0 {
		return nil
	}
	idx := len(s.entries) - 1
	e := s.entries[idx]
	s.entries = s.entries[:idx]
	return &e
}

func (s *nodeStack) peek() *stackEntry {
	if len(s.entries) == 0 {
		return nil
	}
	return &s.entries[len(s.entries)-1]
}

func (s *nodeStack) depth() int {
	return len(s.entries)
}

func (s *nodeStack) reset() {
	s.entries = s.entries[:0]
}

// --- Utility functions ---

// readLines reads all lines from a reader.
func readLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// countIndent returns the number of leading spaces in a line (tabs = 2 spaces).
func countIndent(line string) int {
	n := 0
	for _, ch := range line {
		if ch == ' ' {
			n++
		} else if ch == '\t' {
			n += 2
		} else {
			break
		}
	}
	return n
}

// splitLine splits a line on whitespace while respecting quoted strings.
func splitLine(line string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(line); i++ {
		ch := line[i]

		if inQuote {
			if ch == quoteChar {
				inQuote = false
			} else {
				current.WriteByte(ch)
			}
			continue
		}

		if ch == '"' || ch == '\'' {
			inQuote = true
			quoteChar = ch
			continue
		}

		if ch == ' ' || ch == '\t' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(ch)
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// --- Typed accessors ---

// GetProperty retrieves a property by key (case-insensitive).
func (n *AIENode) GetProperty(key string) string {
	for k, v := range n.Properties {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// GetPropertyFloat retrieves a float64 property.
func (n *AIENode) GetPropertyFloat(key string) float64 {
	v := n.GetProperty(key)
	if v == "" {
		return 0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return f
}

// GetPropertyInt retrieves an int property.
func (n *AIENode) GetPropertyInt(key string) int {
	v := n.GetProperty(key)
	if v == "" {
		return 0
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return i
}

// GetPropertyBool retrieves a boolean property (yes/true/1 = true).
func (n *AIENode) GetPropertyBool(key string) bool {
	v := strings.ToLower(n.GetProperty(key))
	return v == "yes" || v == "true" || v == "1"
}

// FindChild finds the first child node with the given type (case-insensitive).
func (n *AIENode) FindChild(nodeType string) *AIENode {
	for _, c := range n.Children {
		if strings.EqualFold(c.Type, nodeType) {
			return c
		}
	}
	return nil
}

// FindChildren returns all child nodes matching the given type.
func (n *AIENode) FindChildren(nodeType string) []*AIENode {
	var result []*AIENode
	for _, c := range n.Children {
		if strings.EqualFold(c.Type, nodeType) {
			result = append(result, c)
		}
	}
	return result
}

// --- Entity type helpers ---

// IsEntity returns true for entity-defining node types.
func (n *AIENode) IsEntity() bool {
	return entityKeywords[n.Type]
}

// IsMap returns true for MAP nodes.
func (n *AIENode) IsMap() bool {
	return strings.EqualFold(n.Type, "MAP")
}

// IsWeapon returns true for WEAPON nodes.
func (n *AIENode) IsWeapon() bool {
	return strings.EqualFold(n.Type, "WEAPON")
}

// IsTrigger returns true for TRIGGER nodes.
func (n *AIENode) IsTrigger() bool {
	return strings.EqualFold(n.Type, "TRIGGER")
}

// IsCutscene returns true for CUTSCENE nodes.
func (n *AIENode) IsCutscene() bool {
	return strings.EqualFold(n.Type, "CUTSCENE")
}

// IsSound returns true for SOUND nodes.
func (n *AIENode) IsSound() bool {
	return strings.EqualFold(n.Type, "SOUND")
}
