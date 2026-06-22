package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultMode is used when WriteAtomic creates a brand-new file.
const DefaultMode os.FileMode = 0644

// WriteAtomic writes content to path by writing to a sibling temp file and
// renaming. A crash mid-write cannot leave path half-written.
//
// If path already exists, the temp file's mode is set to match before the
// rename — so brew-installed Caddyfiles keep their 0644 permissions across
// shack writes. If path doesn't exist, DefaultMode (0644) is used.
func WriteAtomic(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".shack-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()

	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp: %w", err)
	}

	mode := DefaultMode
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("rename temp to %s: %w", path, err)
	}
	return nil
}
