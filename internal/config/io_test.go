package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteThenRead(t *testing.T) {
	dir := t.TempDir()
	p := Project{Name: "x", Commands: []Command{{Label: "dev", Cmd: "pnpm dev",
		Listeners: []Listener{{Match: Match{Script: "dev"}, As: "x"}}}}}
	if err := Write(dir, p); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Name != "x" || len(got.Commands) != 1 || got.Commands[0].Label != "dev" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestReadMissingReturnsErrNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Read(dir)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestFindRootInCwd(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".shack"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".shack", "config.json"),
		[]byte(`{"name":"x","commands":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := FindRoot(dir)
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

func TestFindRootWalkUp(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".shack"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".shack", "config.json"),
		[]byte(`{"name":"x","commands":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(dir, "a", "b", "c")
	_ = os.MkdirAll(deep, 0755)
	got, err := FindRoot(deep)
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

func TestFindRootMissingReturnsErrNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := FindRoot(dir)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}
