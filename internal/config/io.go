package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/turing/shack/internal/store"
)

// ErrNotFound signals that no .shack/config.json exists.
var ErrNotFound = errors.New("shack config not found")

const (
	dirName  = ".shack"
	fileName = "config.json"
)

// Read parses <projectRoot>/.shack/config.json.
func Read(projectRoot string) (Project, error) {
	path := filepath.Join(projectRoot, dirName, fileName)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("read %s: %w", path, err)
	}
	var p Project
	if err := json.Unmarshal(raw, &p); err != nil {
		return Project{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return p, nil
}

// Write writes config.json atomically. Creates the directory if missing.
func Write(projectRoot string, p Project) error {
	dir := filepath.Join(projectRoot, dirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, fileName)
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	raw = append(raw, '\n')
	return store.WriteAtomic(path, raw)
}

// Exists reports whether the project already has a .shack/config.json.
func Exists(projectRoot string) bool {
	path := filepath.Join(projectRoot, dirName, fileName)
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// FindRoot walks up from start, returning the first ancestor with a
// readable .shack/config.json.
func FindRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if Exists(abs) {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", ErrNotFound
		}
		abs = parent
	}
}
