package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/turing/shack/internal/config"
	"github.com/spf13/cobra"
)

func newUpCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "up [all]",
		Short: "Bring caddy up — and the project's default services if in-project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && args[0] != "all" {
				return newUserError(fmt.Sprintf("shack: unknown argument %q — did you mean `shack up all`?", args[0]))
			}
			return app.RunUp(cmd.OutOrStdout())
		},
	}
}

func (a *App) RunUp(out io.Writer) error {
	// Detect whether we are inside a project.
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	projectRoot, projectErr := config.FindRoot(cwd)
	if projectErr != nil {
		// Outside a project — existing daemon-only behavior.
		return a.runUpDaemon(out)
	}

	// Inside a project.
	// Step 1: ensure caddy daemon is running.
	if err := a.preflightTouchingCaddyfile(); err != nil {
		return err
	}
	if err := a.Caddy.EnsureRunning(); err != nil {
		return err
	}

	// Step 2: read project config.
	proj, err := config.Read(projectRoot)
	if err != nil {
		return fmt.Errorf("shack: read config: %w", err)
	}

	// Step 3: if no defaults, print note and exit.
	if len(proj.Defaults) == 0 {
		fmt.Fprintln(out, "Caddy is up. No default commands configured; run `shack <label>` directly or re-run `shack init` to set defaults.")
		return nil
	}

	// Step 4: check for tmux.
	if !tmuxAvailable(a.Runner) {
		return newUserError("shack: tmux not installed — run `brew install tmux`")
	}

	// Step 5: build session name and check if already running.
	sessionName := tmuxSessionName(proj.Name)
	if tmuxHasSession(a.Runner, sessionName) {
		fmt.Fprintf(out, "session already running. Run `shack attach` to view, or `shack down` to tear down.\n")
		return nil
	}

	// Step 6: create the tmux session.
	if err := tmuxNewDetachedSession(a.Runner, sessionName, projectRoot, proj.Defaults); err != nil {
		return fmt.Errorf("shack: start tmux session: %w", err)
	}

	// Step 7: stream each service's output and probe its known URL until every
	// default serves once or its pane dies — or the user interrupts.
	fmt.Fprintf(out, "Starting %d service%s in tmux session %q...\n\n",
		len(proj.Defaults), pluralS(len(proj.Defaults)), sessionName)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if isTerminal(out) {
		a.streamFrames(ctx, out, sessionName, proj)
	} else {
		a.streamServices(ctx, out, sessionName, proj)
	}

	if ctx.Err() != nil {
		fmt.Fprintf(out, "\nServices still running in tmux session %q.\n", sessionName)
	}
	a.printUpFooter(out, proj)
	return nil
}

// runUpDaemon is the original outside-project start behavior.
func (a *App) runUpDaemon(out io.Writer) error {
	if err := a.preflightTouchingCaddyfile(); err != nil {
		return err
	}
	running, err := a.Caddy.IsRunning()
	if err != nil {
		return err
	}
	if running {
		fmt.Fprintln(out, "caddy: already running")
		return nil
	}
	if err := a.Caddy.Start(); err != nil {
		return err
	}
	fmt.Fprintln(out, "caddy: started")
	return nil
}

// pluralS returns "s" if n != 1, else "".
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// printUpFooter prints per-service attach instructions and the tmux detach +
// down reminders. Printed on every up, terminal or not.
func (a *App) printUpFooter(out io.Writer, proj config.Project) {
	fmt.Fprintln(out)
	for _, label := range proj.Defaults {
		fmt.Fprintf(out, "  %-14s attach: shack attach %s\n", label, label)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  Detach:  Ctrl-b then d")
	fmt.Fprintln(out, "  Down:    shack down")
}

// probe runs the readiness check for one host (a listener's As name),
// using the injected Probe if set, else the system-trust HTTPS probe.
func (a *App) probe(host string) probeOutcome {
	if a.Probe != nil {
		return a.Probe(host)
	}
	return httpProbe(defaultProbeClient(), host)
}
