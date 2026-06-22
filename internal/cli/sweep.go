package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/turing/shack/internal/caddyfile"
	"github.com/turing/shack/internal/portcheck"
	"github.com/turing/shack/internal/store"
)

// LockTimeout is how long we wait for the shack file lock.
const LockTimeout = 2 * time.Second

// caddyfilePath returns the system Caddyfile path without reading it. Used
// when a command only needs the path (e.g. to take the lock) before
// re-reading inside the lock.
func (a *App) caddyfilePath() (string, error) {
	return a.Resolver.CaddyfilePath()
}

// loadDocument reads and parses the Caddyfile at path, treating "not found"
// as empty.
func (a *App) loadDocument(path string) (caddyfile.Document, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		content = nil
	} else if err != nil {
		return caddyfile.Document{}, fmt.Errorf("read caddyfile %s: %w", path, err)
	}
	return caddyfile.Parse(string(content))
}

// saveDocument writes the document to disk atomically. Permission-denied
// errors are rewrapped to match the spec's user-facing format.
func (a *App) saveDocument(doc caddyfile.Document, path string) error {
	if err := store.WriteAtomic(path, []byte(doc.Render())); err != nil {
		if errors.Is(err, fs.ErrPermission) || os.IsPermission(err) {
			return fmt.Errorf("shack: cannot write to %s: permission denied", path)
		}
		return err
	}
	return nil
}

// withLock acquires the shack file lock for the duration of fn.
func (a *App) withLock(caddyfilePath string, fn func() error) error {
	lockPath := caddyfilePath + ".shack.lock"
	err := store.WithLock(lockPath, LockTimeout, fn)
	if errors.Is(err, store.ErrLockTimeout) {
		return fmt.Errorf("shack: another shack process is holding the lock")
	}
	return err
}

// sweepDead drops entries whose ports are not currently in use. Returns the
// names of dropped entries. Warnings about lsof failures are written to warn
// (typically os.Stderr).
func (a *App) sweepDead(doc *caddyfile.Document, warn io.Writer) []string {
	var dropped []string
	kept := doc.Entries[:0]
	for _, e := range doc.Entries {
		if portcheck.IsAlive(a.Runner, e.Port, warn) {
			kept = append(kept, e)
		} else {
			dropped = append(dropped, e.Name)
		}
	}
	doc.Entries = kept
	return dropped
}

// printSwept writes one line per dropped name in the canonical gc format.
func printSwept(out io.Writer, dropped []string) {
	for _, name := range dropped {
		fmt.Fprintf(out, "gc: dropped %s.localhost\n", name)
	}
}
