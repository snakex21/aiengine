// systems/pack.go — Build system: pack project into .aiepack (ZIP) and unpack.
// aiengine.exe --build   → creates <project>.aiepack
// aiengine.exe --import  → extracts .aiepack to current directory

package systems

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PackProject creates a .aiepack archive from the given project directory.
func PackProject(projectDir, projectName, outputPath string) error {
	if outputPath == "" {
		outputPath = projectName + ".aiepack"
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", outputPath, err)
	}
	defer out.Close()

	w := zip.NewWriter(out)
	defer w.Close()

	// Write manifest.aie as the first entry.
	manifest := buildManifest(projectName, projectDir)
	mf, err := w.Create("manifest.aie")
	if err != nil {
		return fmt.Errorf("creating manifest entry: %w", err)
	}
	if _, err := mf.Write([]byte(manifest)); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	// Walk project directory and add all relevant files.
	skipPrefixes := []string{".git", ".aiepack", ".exe", ".exe~"}
	skipExts := []string{".exe", ".aiepack"}

	fileCount := 0

	err = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := filepath.Base(path)
			for _, skip := range skipPrefixes {
				if strings.HasPrefix(base, skip) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		for _, s := range skipExts {
			if ext == s {
				return nil
			}
		}
		for _, s := range skipPrefixes {
			if strings.Contains(path, string(filepath.Separator)+s) {
				return nil
			}
		}

		rel, err := filepath.Rel(projectDir, path)
		if err != nil {
			return fmt.Errorf("rel path %s: %w", path, err)
		}
		rel = filepath.ToSlash(rel)

		fw, err := w.Create(rel)
		if err != nil {
			return fmt.Errorf("creating zip entry %s: %w", rel, err)
		}

		src, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("opening %s: %w", path, err)
		}
		defer src.Close()

		if _, err := io.Copy(fw, src); err != nil {
			return fmt.Errorf("copying %s: %w", path, err)
		}
		fileCount++
		return nil
	})

	if err != nil {
		return fmt.Errorf("walking project: %w", err)
	}

	fmt.Printf("Packed %d files → %s\n", fileCount+1, outputPath) // +1 for manifest
	return w.Close()
}

// ImportProject extracts a .aiepack archive into the given directory.
func ImportProject(packPath, destDir string) error {
	r, err := zip.OpenReader(packPath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", packPath, err)
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", destDir, err)
	}

	count := 0
	for _, f := range r.File {
		dest := filepath.Join(destDir, filepath.FromSlash(f.Name))

		// Security: prevent zip-slip.
		clean := filepath.Clean(dest)
		if !strings.HasPrefix(clean, filepath.Clean(destDir)+string(filepath.Separator)) {
			return fmt.Errorf("illegal file path in archive: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(dest, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("opening zip entry %s: %w", f.Name, err)
		}

		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return fmt.Errorf("creating %s: %w", dest, err)
		}

		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return fmt.Errorf("extracting %s: %w", f.Name, err)
		}
		count++
	}

	fmt.Printf("Imported %d files from %s → %s\n", count, packPath, destDir)
	return nil
}

// buildManifest generates the manifest.aie content.
func buildManifest(projectName, dir string) string {
	now := time.Now().Format("2006-01-02 15:04:05")
	return fmt.Sprintf(`# manifest.aie — AIEngine project manifest
MANIFEST
  PROJECT %s
  ENGINE  aiengine 1.0
  CREATED %s
  FORMAT  project v1
  CONFIG config.aie
  MAIN_MAP maps/level1.aie
  MAIN_SCRIPT scripts/level1.lua
  HUD_SCRIPT scripts/hud.js
  PLAYER_ASSETS assets/sprites/player
  PLAYER_ATLAS gracz.png
  ENEMY_ATLAS przeciwnik.png
`, projectName, now)
}

// ReadManifest extracts the project name from a .aiepack manifest.
func ReadManifest(packPath string) (string, error) {
	r, err := zip.OpenReader(packPath)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", packPath, err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "manifest.aie" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "PROJECT ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "PROJECT ")), nil
				}
			}
			return filepath.Base(packPath), nil
		}
	}
	return filepath.Base(packPath), nil
}

func countZipEntries(w *zip.Writer) int {
	// The writer doesn't expose count directly; we track externally.
	return -1 // caller tracks externally
}
