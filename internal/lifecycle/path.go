package lifecycle

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Resolver resolves brew-managed paths and memoizes the result.
type Resolver struct {
	runner Runner
	prefix string
	cached bool
}

func NewResolver(r Runner) *Resolver {
	return &Resolver{runner: r}
}

func (r *Resolver) BrewPrefix() (string, error) {
	if r.cached {
		return r.prefix, nil
	}
	stdout, stderr, err := r.runner.Run("brew", "--prefix")
	if err != nil {
		return "", fmt.Errorf("brew --prefix failed: %w (stderr: %s)", err, strings.TrimSpace(stderr))
	}
	prefix := strings.TrimSpace(stdout)
	if prefix == "" {
		return "", fmt.Errorf("brew --prefix returned empty output (stderr: %s)", strings.TrimSpace(stderr))
	}
	r.prefix = prefix
	r.cached = true
	return r.prefix, nil
}

func (r *Resolver) CaddyfilePath() (string, error) {
	p, err := r.BrewPrefix()
	if err != nil {
		return "", err
	}
	return filepath.Join(p, "etc", "Caddyfile"), nil
}

func (r *Resolver) CaddyBinary() (string, error) {
	p, err := r.BrewPrefix()
	if err != nil {
		return "", err
	}
	return filepath.Join(p, "bin", "caddy"), nil
}
