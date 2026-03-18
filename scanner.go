package main

import (
	"os"
	"path/filepath"
	"strings"
)

// Directories that never contain git repos and are expensive to walk.
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"vendor":       true,
	".cache":       true,
	".npm":         true,
	".cargo":       true,
	".rustup":      true,
	".local":       true,
	".Trash":       true,
	"Library":      true,
	"Applications": true,
	".docker":      true,
	".nvm":         true,
	".pyenv":       true,
	".rbenv":       true,
	".gradle":      true,
	".m2":          true,
	".vscode":      true,
	".idea":        true,
	".tox":         true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	".terraform":   true,
	"dist":         true,
	"build":        true,
	".next":        true,
	".nuxt":        true,
	".angular":     true,
	".svelte-kit":  true,
	"target":       true, // Rust / Java
	"Pods":         true, // CocoaPods
}

// scanForBeads walks each root up to maxDepth looking for .beads directories.
// Returns absolute paths to every .beads directory found.
func scanForBeads(roots []string, maxDepth int) []string {
	var dirs []string
	seen := make(map[string]bool)

	for _, root := range roots {
		root = filepath.Clean(root)
		rootDepth := depthOf(root)

		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return filepath.SkipDir
			}
			if !d.IsDir() {
				return nil
			}

			name := d.Name()

			// Respect depth limit.
			if depthOf(path)-rootDepth > maxDepth {
				return filepath.SkipDir
			}

			// Skip known heavy directories (but never skip .beads itself).
			if skipDirs[name] {
				return filepath.SkipDir
			}

			if name == ".beads" {
				abs, e := filepath.Abs(path)
				if e != nil {
					abs = path
				}
				if !seen[abs] {
					seen[abs] = true
					dirs = append(dirs, abs)
				}
				return filepath.SkipDir
			}
			return nil
		})
	}
	return dirs
}

func depthOf(p string) int {
	return strings.Count(filepath.Clean(p), string(filepath.Separator))
}
