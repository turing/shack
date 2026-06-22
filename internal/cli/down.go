package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/turing/shack/internal/config"
	"github.com/spf13/cobra"
)

func newDownCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "down [all]",
		Short: "Bring the project session down — and caddy with 'down all'",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && args[0] != "all" {
				return newUserError(fmt.Sprintf("shack: unknown argument %q — did you mean `shack down all`?", args[0]))
			}
			all := len(args) == 1
			return app.RunDown(cmd.OutOrStdout(), all)
		},
	}
}

func (a *App) RunDown(out io.Writer, all bool) error {
	// `down all` means "all shack-managed sessions + the daemon",
	// regardless of cwd. It scans every tmux session whose name starts
	// with `shack-` and kills each, then stops the daemon.
	if all {
		killed := 0
		for _, sess := range tmuxListShackSessions(a.Runner) {
			if err := tmuxKillSession(a.Runner, sess); err != nil {
				fmt.Fprintf(out, "shack: kill %s: %v\n", sess, err)
				continue
			}
			fmt.Fprintf(out, "Stopped session %s.\n", sess)
			killed++
		}
		if killed == 0 {
			fmt.Fprintln(out, "no running shack sessions")
		}
		return a.runDownDaemon(out)
	}

	// `down` (no arg): scope to the current project.
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	projectRoot, projectErr := config.FindRoot(cwd)
	if projectErr != nil {
		// Outside a project — fall back to daemon-only behavior.
		return a.runDownDaemon(out)
	}
	proj, err := config.Read(projectRoot)
	if err != nil {
		return fmt.Errorf("shack: read config: %w", err)
	}
	sessionName := tmuxSessionName(proj.Name)
	if !tmuxHasSession(a.Runner, sessionName) {
		fmt.Fprintln(out, "no running session")
		return nil
	}
	if err := tmuxKillSession(a.Runner, sessionName); err != nil {
		return fmt.Errorf("shack: kill tmux session: %w", err)
	}
	fmt.Fprintf(out, "Stopped session %s.\n", sessionName)
	return nil
}

// runDownDaemon is the original outside-project stop behavior.
func (a *App) runDownDaemon(out io.Writer) error {
	if err := a.preflightTouchingCaddyfile(); err != nil {
		return err
	}
	if err := a.Caddy.Stop(); err != nil {
		return err
	}
	fmt.Fprintln(out, "caddy: stopped")
	return nil
}
