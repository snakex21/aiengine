package main

import (
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"aiengine/parser"

	webview "github.com/webview/webview_go"
)

//go:embed web/*
var webFS embed.FS

type FileEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Kind string `json:"kind"`
	Size int64  `json:"size"`
}

type ScanResult struct {
	Root    string      `json:"root"`
	Entries []FileEntry `json:"entries"`
}

type SaveResult struct {
	Path string `json:"path"`
}

type ProjectInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type ActiveProjectInfo struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	ManifestPath string `json:"manifestPath"`
	ConfigPath   string `json:"configPath"`
	MainMap      string `json:"mainMap"`
	MainScript   string `json:"mainScript"`
	HUDScript    string `json:"hudScript"`
	PlayerAssets string `json:"playerAssets"`
	PlayerAtlas  string `json:"playerAtlas"`
	EnemyAtlas   string `json:"enemyAtlas"`
	EngineExe    string `json:"engineExe"`
}

type MapFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type MapData struct {
	Name string   `json:"name"`
	Path string   `json:"path"`
	Rows []string `json:"rows"`
	Raw  string   `json:"raw"`
}

type Bridge struct {
	root string
}

func main() {
	runtime.LockOSThread()

	root := defaultProjectRoot()
	bridge := &Bridge{root: root}

	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle("AIEngine Editor")
	w.SetSize(1180, 760, webview.HintNone)

	mustBind(w, "getDefaultProject", bridge.GetDefaultProject)
	mustBind(w, "getProjectInfo", bridge.GetProjectInfo)
	mustBind(w, "listProjects", bridge.ListProjects)
	mustBind(w, "openProjectFolder", bridge.OpenProjectFolder)
	mustBind(w, "listMaps", bridge.ListMaps)
	mustBind(w, "loadMap", bridge.LoadMap)
	mustBind(w, "saveMap", bridge.SaveMap)
	mustBind(w, "scanProject", bridge.ScanProject)
	mustBind(w, "readTextFile", bridge.ReadTextFile)
	mustBind(w, "writeTextFile", bridge.WriteTextFile)
	mustBind(w, "readImageData", bridge.ReadImageData)
	mustBind(w, "exportMap", bridge.ExportMap)
	mustBind(w, "launchEngine", bridge.LaunchEngine)

	w.SetHtml(renderHTML())
	w.Run()
}

func mustBind(w webview.WebView, name string, fn interface{}) {
	if err := w.Bind(name, fn); err != nil {
		panic(fmt.Sprintf("bind %s: %v", name, err))
	}
}

func renderHTML() string {
	index := mustReadEmbedded("web/index.html")
	style := mustReadEmbedded("web/style.css")
	script := mustReadEmbedded("web/app.js")

	tpl := template.Must(template.New("index").Parse(index))
	var b strings.Builder
	if err := tpl.Execute(&b, map[string]interface{}{
		"Style":  template.CSS(style),
		"Script": template.JS(script),
	}); err != nil {
		panic(err)
	}
	return b.String()
}

func mustReadEmbedded(path string) string {
	b, err := webFS.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func defaultProjectRoot() string {
	if wd, err := os.Getwd(); err == nil {
		demo := filepath.Join(wd, "projects", "demo")
		if looksLikeProject(demo) {
			return demo
		}
		if looksLikeProject(wd) {
			return wd
		}
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		demo := filepath.Join(dir, "projects", "demo")
		if looksLikeProject(demo) {
			return demo
		}
		if looksLikeProject(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		demo = filepath.Join(parent, "projects", "demo")
		if looksLikeProject(demo) {
			return demo
		}
		if looksLikeProject(parent) {
			return parent
		}
	}
	wd, _ := os.Getwd()
	return wd
}

func looksLikeProject(dir string) bool {
	checks := []string{"config.aie", "manifest.aie", "assets"}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(dir, c)); err == nil {
			return true
		}
	}
	return false
}

func (b *Bridge) GetDefaultProject() (string, error) {
	return b.root, nil
}

func (b *Bridge) ListProjects() ([]ProjectInfo, error) {
	repo := findRepoRoot(b.root)
	projectsRoot := filepath.Join(repo, "projects")
	entries, err := os.ReadDir(projectsRoot)
	if err != nil {
		return []ProjectInfo{{Name: filepath.Base(b.root), Path: b.root}}, nil
	}
	projects := make([]ProjectInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(projectsRoot, entry.Name())
		if looksLikeProject(path) {
			projects = append(projects, ProjectInfo{Name: entry.Name(), Path: path})
		}
	}
	if len(projects) == 0 {
		projects = append(projects, ProjectInfo{Name: filepath.Base(b.root), Path: b.root})
	}
	return projects, nil
}

func (b *Bridge) GetProjectInfo() (*ActiveProjectInfo, error) {
	manifest, err := parser.ParseManifestFile(filepath.Join(b.root, "manifest.aie"))
	if err != nil {
		manifest = parser.DefaultManifest()
	}
	exe, _ := b.findEngineExe()
	return &ActiveProjectInfo{
		Name:         manifest.Project,
		Path:         b.root,
		ManifestPath: "manifest.aie",
		ConfigPath:   manifest.Config,
		MainMap:      manifest.MainMap,
		MainScript:   manifest.MainScript,
		HUDScript:    manifest.HUDScript,
		PlayerAssets: manifest.PlayerAssets,
		PlayerAtlas:  manifest.PlayerAtlas,
		EnemyAtlas:   manifest.EnemyAtlas,
		EngineExe:    exe,
	}, nil
}

func (b *Bridge) OpenProjectFolder() (string, error) {
	cmd := exec.Command("explorer.exe", b.root)
	if err := cmd.Start(); err != nil {
		return "", err
	}
	return "Otworzono folder projektu", nil
}

func (b *Bridge) ListMaps() ([]MapFile, error) {
	mapsDir := filepath.Join(b.root, "maps")
	entries, err := os.ReadDir(mapsDir)
	if err != nil {
		return nil, err
	}
	maps := make([]MapFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".aie" {
			continue
		}
		maps = append(maps, MapFile{Name: strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), Path: filepath.ToSlash(filepath.Join("maps", entry.Name()))})
	}
	return maps, nil
}

func (b *Bridge) LoadMap(path string) (*MapData, error) {
	abs, err := b.resolve(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	content := string(data)
	rows := extractGridRows(content)
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && strings.EqualFold(fields[0], "MAP") {
			name = fields[1]
			break
		}
	}
	return &MapData{Name: name, Path: filepath.ToSlash(path), Rows: rows, Raw: content}, nil
}

func extractGridRows(content string) []string {
	rows := []string{}
	inGrid := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if inGrid && len(rows) > 0 {
				break
			}
			continue
		}
		if strings.EqualFold(trimmed, "GRID") {
			inGrid = true
			continue
		}
		if !inGrid {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		if isMapDirective(fields[0]) {
			break
		}
		rows = append(rows, strings.Join(fields, " "))
	}
	return rows
}

// isMapDirective reports a line that ends the GRID rows: grid cells are
// single characters, so any longer first token (PLACE, TRIGGER, FLOOR_GRID,
// CEILING_GRID, LIGHT_GRID, ...) starts something else. Blocks the editor
// does not edit (height and light grids) are kept verbatim on save.
func isMapDirective(token string) bool {
	return utf8.RuneCountInString(token) > 1
}

func (b *Bridge) ScanProject(dir string) (*ScanResult, error) {
	root := strings.TrimSpace(dir)
	if root == "" {
		root = b.root
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	b.root = absRoot

	entries := make([]FileEntry, 0, 128)
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == absRoot {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			switch name {
			case ".git", "node_modules", "target", ".playwright-mcp":
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(name))
		if !isInteresting(ext) {
			return nil
		}
		info, _ := d.Info()
		rel, _ := filepath.Rel(absRoot, path)
		entries = append(entries, FileEntry{
			Name: name,
			Path: filepath.ToSlash(rel),
			Kind: kindFromExt(ext),
			Size: safeSize(info),
		})
		if len(entries) >= 1500 {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &ScanResult{Root: absRoot, Entries: entries}, nil
}

func isInteresting(ext string) bool {
	switch ext {
	case ".aie", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ogg", ".wav", ".mp3", ".lua", ".js", ".css", ".go", ".md", ".json":
		return true
	default:
		return false
	}
}

func kindFromExt(ext string) string {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return "image"
	case ".ogg", ".wav", ".mp3":
		return "audio"
	case ".aie":
		return "aie"
	case ".go", ".lua", ".js", ".css", ".md", ".json":
		return "text"
	default:
		return "file"
	}
}

func safeSize(info fs.FileInfo) int64 {
	if info == nil {
		return 0
	}
	return info.Size()
}

func (b *Bridge) ReadTextFile(path string) (string, error) {
	abs, err := b.resolve(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (b *Bridge) WriteTextFile(path, content string) (*SaveResult, error) {
	abs, err := b.resolve(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		return nil, err
	}
	rel, _ := filepath.Rel(b.root, abs)
	return &SaveResult{Path: filepath.ToSlash(rel)}, nil
}

func (b *Bridge) ReadImageData(path string) (string, error) {
	abs, err := b.resolve(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	mime := "image/png"
	switch strings.ToLower(filepath.Ext(abs)) {
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".gif":
		mime = "image/gif"
	case ".webp":
		mime = "image/webp"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func (b *Bridge) ExportMap(name string, rows []string) (*SaveResult, error) {
	name = sanitizeName(name)
	if name == "" {
		name = "untitled"
	}
	var out strings.Builder
	out.WriteString("MAP ")
	out.WriteString(name)
	out.WriteString("\nGRID\n")
	for _, row := range rows {
		out.WriteString(row)
		out.WriteByte('\n')
	}
	return b.WriteTextFile(filepath.Join("maps", name+".aie"), out.String())
}

func (b *Bridge) SaveMap(path, name string, rows []string, raw string) (*SaveResult, error) {
	name = sanitizeName(name)
	if name == "" {
		name = "untitled"
	}
	if strings.TrimSpace(path) == "" {
		path = filepath.Join("maps", name+".aie")
	}
	content := replaceMapGrid(raw, name, rows)
	return b.WriteTextFile(path, content)
}

func replaceMapGrid(raw, name string, rows []string) string {
	gridBlock := buildGridBlock(rows)
	if strings.TrimSpace(raw) == "" {
		return "MAP " + name + "\n" + gridBlock + "\n"
	}
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines)+len(rows))
	inGrid := false
	replaced := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		fields := strings.Fields(trimmed)
		if len(fields) >= 1 && strings.EqualFold(fields[0], "MAP") {
			out = append(out, "MAP "+name)
			continue
		}
		if strings.EqualFold(trimmed, "GRID") {
			out = append(out, strings.TrimSuffix(gridBlock, "\n"))
			inGrid = true
			replaced = true
			continue
		}
		if inGrid {
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				inGrid = false
				out = append(out, line)
				continue
			}
			if len(fields) > 0 && isMapDirective(fields[0]) {
				inGrid = false
				out = append(out, line)
			}
			continue
		}
		out = append(out, line)
	}
	if !replaced {
		out = append(out, strings.TrimSuffix(gridBlock, "\n"))
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}

func buildGridBlock(rows []string) string {
	var out strings.Builder
	out.WriteString("GRID\n")
	for _, row := range rows {
		out.WriteString(row)
		out.WriteByte('\n')
	}
	return out.String()
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, s)
	return strings.Trim(s, "_")
}

func (b *Bridge) LaunchEngine() (string, error) {
	exe, workDir := b.findEngineExe()
	if exe == "" {
		exe = filepath.Join(findRepoRoot(b.root), "bin", "aiengine.exe")
	}
	if _, err := os.Stat(exe); err != nil {
		return "", fmt.Errorf("nie znaleziono bin/aiengine.exe; zbuduj najpierw: .\\scripts\\build.ps1")
	}
	if workDir == "" {
		workDir = filepath.Dir(exe)
	}
	cmd := exec.Command(exe)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "AIENGINE_PROJECT="+b.root)
	if err := cmd.Start(); err != nil {
		return "", err
	}
	return fmt.Sprintf("Uruchomiono AIEngine (PID %d)", cmd.Process.Pid), nil
}

func (b *Bridge) findEngineExe() (exe string, workDir string) {
	dir := b.root
	for i := 0; i < 5; i++ {
		candidates := []string{
			filepath.Join(dir, "bin", "aiengine.exe"),
			filepath.Join(dir, "aiengine.exe"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, filepath.Dir(candidate)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", b.root
}

func findRepoRoot(start string) string {
	dir := start
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return start
}

func (b *Bridge) resolve(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("pusta ścieżka")
	}
	path = filepath.FromSlash(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(b.root, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return abs, nil
}
