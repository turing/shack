package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/turing/shack/internal/config"
)

// tmuxAttacher abstracts the final exec into tmux so tests can fake it.
type tmuxAttacher interface {
	Attach(sessionName string, windowIndex int) error
}

// execTmuxAttacher is the production attacher that execs into tmux.
type execTmuxAttacher struct{}

func (execTmuxAttacher) Attach(sessionName string, windowIndex int) error {
	tmuxBin, err := lookPath("tmux")
	if err != nil {
		return fmt.Errorf("shack: tmux not found on PATH: %w", err)
	}
	// Select the target window first (a separate process), since the exec
	// below replaces this process and can run only one command.
	if windowIndex >= 0 {
		target := sessionName + ":" + strconv.Itoa(windowIndex)
		if err := exec.Command(tmuxBin, "select-window", "-t", target).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "shack: could not select window %s: %v\n", target, err)
		}
	}
	subCmd := "attach-session"
	if os.Getenv("TMUX") != "" {
		subCmd = "switch-client"
	}
	return syscall.Exec(tmuxBin, []string{"tmux", subCmd, "-t", sessionName}, os.Environ())
}

// lookPath is a thin wrapper around exec.LookPath.
func lookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func newAttachCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "attach [service]",
		Short: "Attach to the project's tmux session (optionally a service's window)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			label := ""
			if len(args) == 1 {
				label = args[0]
			}
			return app.RunAttach(cmd.OutOrStdout(), execTmuxAttacher{}, label)
		},
	}
}

func (a *App) RunAttach(out io.Writer, attacher tmuxAttacher, label string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	projectRoot, err := config.FindRoot(cwd)
	if err != nil {
		return newUserError("shack: not in a shack project; run from a directory with `.shack/config.json`")
	}
	proj, err := config.Read(projectRoot)
	if err != nil {
		return fmt.Errorf("shack: read config: %w", err)
	}
	sessionName := tmuxSessionName(proj.Name)
	if !tmuxHasSession(a.Runner, sessionName) {
		return newUserError("shack: no running session for this project. Run `shack up` first.")
	}
	windowIndex := -1
	if label != "" {
		for i, d := range proj.Defaults {
			if d == label {
				windowIndex = i
				break
			}
		}
		if windowIndex < 0 {
			return newUserError(fmt.Sprintf(
				"shack: unknown service %q — defaults are: %s",
				label, strings.Join(proj.Defaults, ", "),
			))
		}
	}
	return attacher.Attach(sessionName, windowIndex)
}
