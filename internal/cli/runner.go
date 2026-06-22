package cli

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/turing/shack/internal/config"
	"github.com/turing/shack/internal/proctree"
)

// RunCommand is the in-process entry for `shack <label>`. It looks
// up label in projectRoot/.shack/config.json, runs Preflight (so
// missing brew/caddy fails before any child is spawned), spawns the
// configured command, watches for listeners, registers each match, and
// unregisters on exit. Output during the child's lifetime is
// byte-identical to running the configured command directly.
func (a *App) RunCommand(
	ctx context.Context,
	projectRoot string,
	label string,
	stdout, stderr io.Writer,
	procRunner proctree.Runner,
) (int, error) {
	if err := a.preflightTouchingCaddyfile(); err != nil {
		return 1, err
	}
	proj, err := config.Read(projectRoot)
	if err != nil {
		return 1, newUserError("shack: read config: " + err.Error())
	}
	cmdCfg, ok := findCommand(proj, label)
	if !ok {
		return 1, newUserError(fmt.Sprintf("shack: no command %q — run `shack init` to configure", label))
	}

	// Mutable state shared between the observer callback and the
	// post-exit phase. All access goes through obsMu.
	//
	// postExitMessages must be declared in the same var block (not after
	// cb) — Go's lexical scoping makes a forward-reference from cb a
	// compile error.
	var (
		obsMu            sync.Mutex
		registered       []string // hostnames we registered, in order
		postExitMessages []string // messages flushed to stderr after Wait
		matched          atomic.Bool
	)

	cb := func(l proctree.Listener, id proctree.Identifiers) {
		// Match + decision under obsMu, but release before I/O.
		// RegisterHostname acquires the Caddyfile write-lock; holding obsMu
		// across that call would create a lock-order hazard if any future
		// code path acquires Caddyfile lock then tries to acquire obsMu.
		var (
			name        string
			wasUnmatched bool
		)
		obsMu.Lock()
		if entry := MatchListener(id, cmdCfg.Listeners); entry != nil {
			name = entry.As
			matched.Store(true)
		} else {
			name = fmt.Sprintf("%s-%d", proj.Name, l.Port)
			wasUnmatched = true
		}
		obsMu.Unlock()

		// I/O outside obsMu.
		if err := a.RegisterHostname(name, l.Port); err != nil {
			// Registration failure: queue a post-exit message; do not
			// kill the child. For unmatched ports we use a different
			// message that does NOT claim "registered as" — that would
			// be a lie since registration failed.
			obsMu.Lock()
			if wasUnmatched {
				postExitMessages = append(postExitMessages, fmt.Sprintf("shack: listener on :%d didn't match config — could not register as %s: %v\n", l.Port, name, err))
			} else {
				postExitMessages = append(postExitMessages, fmt.Sprintf("shack: register %s.localhost failed: %v\n", name, err))
			}
			obsMu.Unlock()
			return
		}

		obsMu.Lock()
		registered = append(registered, name)
		if wasUnmatched {
			postExitMessages = append(postExitMessages, fmt.Sprintf("shack: listener on :%d didn't match config — registered as %s. Re-run `shack init` to capture it.\n", l.Port, name))
		}
		obsMu.Unlock()
	}

	if strings.TrimSpace(cmdCfg.Pre) != "" {
		fmt.Fprintf(stdout, "[%s] running pre-hook: %s\n", label, cmdCfg.Pre)
		pre := exec.Command("sh", "-c", cmdCfg.Pre)
		pre.Stdout = stdout
		pre.Stderr = stderr
		pre.Dir = projectRoot
		if err := pre.Run(); err != nil {
			return 0, fmt.Errorf("pre-hook failed: %w", err)
		}
	}

	c := exec.Command("sh", "-c", cmdCfg.Cmd)
	c.Dir = projectRoot

	h, err := startChild(c, stdout, stderr)
	if err != nil {
		return 2, fmt.Errorf("shack: failed to spawn %s: %w", cmdCfg.Cmd, err)
	}
	defer h.Stop()

	watchCtx, cancelWatch := context.WithCancel(ctx)
	obs := newObserver(procRunner, h.PID, cb)
	go obs.Watch(watchCtx)

	exitCode, waitErr := h.Wait()
	cancelWatch()
	obs.Wait()

	// Snapshot the shared state under the lock.
	obsMu.Lock()
	regSnap := append([]string(nil), registered...)
	postSnap := append([]string(nil), postExitMessages...)
	obsMu.Unlock()

	if len(regSnap) > 0 {
		if err := a.UnregisterHostnames(regSnap); err != nil {
			fmt.Fprintf(stderr, "shack: unregister: %v\n", err)
		}
	}

	for _, m := range postSnap {
		fmt.Fprint(stderr, m)
	}

	return exitCode, waitErr
}

func findCommand(p config.Project, label string) (*config.Command, bool) {
	for i := range p.Commands {
		if p.Commands[i].Label == label {
			return &p.Commands[i], true
		}
	}
	return nil, false
}

