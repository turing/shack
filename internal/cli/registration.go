package cli

import (
	"github.com/turing/shack/internal/validate"
)

// RegisterHostname adds (or replaces) a single managed-region entry and
// reloads caddy. Same-port re-register is a silent no-op (no rewrite, no
// reload). Used by both `add` and the dev runner — caller is responsible
// for any preflight checks; this helper writes nothing to stdout/stderr
// to preserve the runtime byte-identical contract.
//
// intentionally lighter than RunAdd: no sweep, no warnings — gc handles cleanup.
func (a *App) RegisterHostname(name string, port int) error {
	if err := validate.Name(name); err != nil {
		return newUserError("shack: " + err.Error())
	}
	if err := validate.Port(port); err != nil {
		return newUserError("shack: " + err.Error())
	}
	path, err := a.caddyfilePath()
	if err != nil {
		return err
	}
	return a.withLock(path, func() error {
		doc, err := a.loadDocument(path)
		if err != nil {
			return err
		}
		existing, found := doc.Get(name)
		if found && existing.Port == port {
			return nil
		}
		doc.Add(name, port)
		if err := a.saveDocument(doc, path); err != nil {
			return err
		}
		if err := a.Caddy.EnsureRunning(); err != nil {
			return err
		}
		return a.Caddy.Reload()
	})
}

// UnregisterHostnames drops a batch of names in one Caddyfile rewrite +
// reload. Names that don't exist are silently ignored. Reloads only if
// caddy is running; never starts caddy here.
func (a *App) UnregisterHostnames(names []string) error {
	if len(names) == 0 {
		return nil
	}
	path, err := a.caddyfilePath()
	if err != nil {
		return err
	}
	return a.withLock(path, func() error {
		doc, err := a.loadDocument(path)
		if err != nil {
			return err
		}
		changed := false
		for _, n := range names {
			if doc.Remove(n) {
				changed = true
			}
		}
		if !changed {
			return nil
		}
		if err := a.saveDocument(doc, path); err != nil {
			return err
		}
		running, err := a.Caddy.IsRunning()
		if err != nil {
			return err
		}
		if !running {
			return nil
		}
		return a.Caddy.Reload()
	})
}
