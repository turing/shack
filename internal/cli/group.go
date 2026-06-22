package cli

import (
	"errors"
	"strings"

	"github.com/turing/shack/internal/lifecycle"
)

// tmuxSessionName returns the tmux session name for a project.
// It sanitizes the project name by replacing characters not legal in tmux
// session names (dots, colons, spaces, etc.) with hyphens.
func tmuxSessionName(proj string) string {
	replacer := strings.NewReplacer(
		".", "-",
		":", "-",
		" ", "-",
		"/", "-",
	)
	return "shack-" + replacer.Replace(proj)
}

// tmuxAvailable returns true if tmux is on PATH.
func tmuxAvailable(runner lifecycle.Runner) bool {
	_, _, err := runner.Run("tmux", "-V")
	return err == nil
}

// tmuxHasSession returns true if a tmux session with the given name exists.
func tmuxHasSession(runner lifecycle.Runner, name string) bool {
	_, _, err := runner.Run("tmux", "has-session", "-t", name)
	return err == nil
}

// tmuxNewDetachedSession creates a detached tmux session with one window per
// label, each running `shack <label>` as the window's own process so tmux
// records its exit status. remain-on-exit=failed freezes a pane only when its
// command exits non-zero. labels must be non-empty.
func tmuxNewDetachedSession(runner lifecycle.Runner, name, projectRoot string, labels []string) error {
	if len(labels) == 0 {
		return errors.New("tmuxNewDetachedSession: labels must not be empty")
	}

	first := labels[0]
	if _, _, err := runner.Run("tmux", "new-session", "-d", "-s", name,
		"-c", projectRoot, "-n", first, "shack", first); err != nil {
		return err
	}
	if err := tmuxKeepDeadPane(runner, name, 0); err != nil {
		return err
	}

	for i, label := range labels[1:] {
		windowIndex := i + 1
		if _, _, err := runner.Run("tmux", "new-window", "-t", name,
			"-c", projectRoot, "-n", label, "shack", label); err != nil {
			return err
		}
		if err := tmuxKeepDeadPane(runner, name, windowIndex); err != nil {
			return err
		}
	}
	return nil
}

// tmuxKeepDeadPane sets remain-on-exit=failed on one window so a non-zero
// command exit freezes the pane instead of closing the window.
func tmuxKeepDeadPane(runner lifecycle.Runner, name string, index int) error {
	_, _, err := runner.Run("tmux", "set-option", "-w", "-t",
		name+":"+itoa(index), "remain-on-exit", "failed")
	return err
}

// tmuxKillSession kills the named tmux session.
func tmuxKillSession(runner lifecycle.Runner, name string) error {
	_, _, err := runner.Run("tmux", "kill-session", "-t", name)
	return err
}

// tmuxListShackSessions returns the names of all tmux sessions whose
// names start with "shack-". An empty list (and no error) is the
// expected result when tmux is installed but no sessions exist.
func tmuxListShackSessions(runner lifecycle.Runner) []string {
	stdout, _, err := runner.Run("tmux", "list-sessions", "-F", "#{session_name}")
	if err != nil {
		// `tmux list-sessions` exits 1 with "no server running" when the
		// tmux server isn't up. Treat as zero sessions.
		return nil
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "shack-") {
			out = append(out, line)
		}
	}
	return out
}

// itoa is a minimal int-to-string helper to avoid importing strconv
// in this helper file (strconv is available but keeping imports minimal).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
