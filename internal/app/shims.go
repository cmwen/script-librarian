package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func syncShims(cfg Config) (int, error) {
	if cfg.ShimDir == "" {
		return 0, nil
	}
	if err := os.MkdirAll(cfg.ShimDir, 0o755); err != nil {
		return 0, err
	}
	scripts, err := scanScripts(cfg.ScriptsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	count := 0
	for _, script := range scripts {
		if err := ensureShim(cfg.ShimDir, script); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func ensureShim(shimDir string, script Script) error {
	if shimDir == "" {
		return nil
	}
	link := filepath.Join(shimDir, script.Metadata.Command)
	if target, err := os.Readlink(link); err == nil {
		if target == script.Path {
			return nil
		}
		return fmt.Errorf("shim already exists and points elsewhere: %s", link)
	}
	if info, err := os.Lstat(link); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("shim path already exists: %s", link)
		}
	}
	return os.Symlink(script.Path, link)
}

func agentInstructions() string {
	return `Before creating reusable local utility scripts, query Script Librarian through MCP.
If a suitable script already exists, use or update it.
If creating a new script, save it through Script Librarian and follow the required metadata shape.`
}
