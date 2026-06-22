package lifecycle

import (
	"fmt"
	"strings"
)

type Caddy struct {
	runner   Runner
	resolver *Resolver
}

func NewCaddy(r Runner, res *Resolver) *Caddy {
	return &Caddy{runner: r, resolver: res}
}

func (c *Caddy) IsRunning() (bool, error) {
	stdout, stderr, err := c.runner.Run("brew", "services", "list")
	if err != nil {
		return false, fmt.Errorf("brew services list failed: %w (stderr: %s)", err, strings.TrimSpace(stderr))
	}
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "caddy" {
			return fields[1] == "started", nil
		}
	}
	return false, nil
}

func (c *Caddy) Start() error {
	running, err := c.IsRunning()
	if err != nil {
		return err
	}
	if running {
		return nil
	}
	_, stderr, err := c.runner.Run("brew", "services", "start", "caddy")
	if err == nil {
		return nil
	}
	// launchd half-loaded state: when caddy was previously bootstrapped but
	// the process subsequently died, `brew services start caddy` fails with
	// "Bootstrap failed: 5: Input/output error" because launchctl refuses
	// to bootstrap a job that's already loaded. The cure is bootout +
	// bootstrap, which is exactly what `brew services restart` does.
	if isBootstrapAlreadyLoaded(stderr) {
		_, rstderr, rerr := c.runner.Run("brew", "services", "restart", "caddy")
		if rerr == nil {
			return nil
		}
		return fmt.Errorf("brew services start caddy: %w (stderr: %s); recovery via `brew services restart caddy` also failed: %v (stderr: %s)",
			err, strings.TrimSpace(stderr), rerr, strings.TrimSpace(rstderr))
	}
	return fmt.Errorf("brew services start caddy: %w (stderr: %s)", err, strings.TrimSpace(stderr))
}

// isBootstrapAlreadyLoaded matches the stderr signature brew emits when
// launchctl bootstrap rejects the request because the service is already
// loaded into launchd. Both the literal "Bootstrap failed" line and the
// downstream "already loaded" wording have shown up across macOS releases,
// so check for both.
func isBootstrapAlreadyLoaded(stderr string) bool {
	return strings.Contains(stderr, "Bootstrap failed") ||
		strings.Contains(stderr, "already loaded") ||
		strings.Contains(stderr, "service already loaded")
}

func (c *Caddy) Stop() error {
	_, stderr, err := c.runner.Run("brew", "services", "stop", "caddy")
	if err != nil {
		return fmt.Errorf("brew services stop caddy: %w (stderr: %s)", err, strings.TrimSpace(stderr))
	}
	return nil
}

func (c *Caddy) Reload() error {
	bin, err := c.resolver.CaddyBinary()
	if err != nil {
		return err
	}
	cfg, err := c.resolver.CaddyfilePath()
	if err != nil {
		return err
	}
	_, stderr, err := c.runner.Run(bin, "reload", "--config", cfg, "--adapter", "caddyfile")
	if err != nil {
		// Forward caddy's stderr verbatim (per spec). When stderr is empty
		// fall back to wrapping the exec error so the user gets something
		// actionable instead of a bare prefix.
		if strings.TrimSpace(stderr) == "" {
			return fmt.Errorf("caddy reload failed: %w", err)
		}
		return fmt.Errorf("caddy reload failed: %s", stderr)
	}
	return nil
}

func (c *Caddy) EnsureRunning() error {
	running, err := c.IsRunning()
	if err != nil {
		return err
	}
	if running {
		return nil
	}
	return c.Start()
}

// IsCATrusted reports whether Caddy's local-authority root cert is
// installed in the macOS System keychain — i.e., whether `sudo caddy
// trust` has been run. Browsers only suppress the "your connection is not
// private" warning for *.localhost when the root is in the System
// keychain.
//
// Detection: `security find-certificate -a -c "Caddy Local Authority"
// /Library/Keychains/System.keychain` exits 0 either way; presence is
// signalled by non-empty stdout.
func (c *Caddy) IsCATrusted() bool {
	const subjectPrefix = "Caddy Local Authority"
	const systemKeychain = "/Library/Keychains/System.keychain"
	stdout, _, err := c.runner.Run("security", "find-certificate", "-a", "-c", subjectPrefix, systemKeychain)
	if err != nil {
		return false
	}
	return strings.TrimSpace(stdout) != ""
}
