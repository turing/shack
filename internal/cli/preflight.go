package cli

import "fmt"

// Preflight checks that brew is on PATH and the caddy binary exists.
// Status (and any future read-only ops) call this directly.
func (a *App) Preflight() error {
	if _, _, err := a.Runner.Run("brew", "--version"); err != nil {
		return newUserError("shack: brew is not on PATH (shack requires Homebrew)")
	}
	bin, err := a.Resolver.CaddyBinary()
	if err != nil {
		return fmt.Errorf("resolve caddy binary: %w", err)
	}
	if !a.FileExists(bin) {
		return newUserError("shack: caddy is not installed — run `brew install caddy`")
	}
	return nil
}

// preflightTouchingCaddyfile is the stricter check used by every op that
// writes the Caddyfile or spawns caddy: add, rm, list, gc, start, stop,
// reload, init, and RunCommand. It runs Preflight() then verifies the
// caddy local CA is trusted by the system keychain, since otherwise every
// browser hit on https://<name>.localhost would warn.
func (a *App) preflightTouchingCaddyfile() error {
	if err := a.Preflight(); err != nil {
		return err
	}
	if !a.Caddy.IsCATrusted() {
		return newUserError("shack: caddy local CA is not trusted by your system — browsers will warn on https://<name>.localhost.\n  fix: sudo caddy trust")
	}
	return nil
}
