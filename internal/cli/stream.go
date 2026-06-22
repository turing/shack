package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/turing/shack/internal/config"
	"github.com/turing/shack/internal/lifecycle"
)

const (
	streamHistory  = 1000 // capture-pane scrollback depth per tick
	crashTailLines = 15   // lines of a dead pane to show under a crash
)

// trimTrailingBlank drops trailing all-whitespace lines (terminal row padding
// that capture-pane includes below the last real output).
func trimTrailingBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// newPaneLines returns the lines of a captured pane beyond `already` (with
// trailing blank padding trimmed) and the new total line count. `already`
// greater than the current length (history scrolled) is clamped to no output.
func newPaneLines(raw string, already int) ([]string, int) {
	lines := trimTrailingBlank(strings.Split(strings.TrimRight(raw, "\n"), "\n"))
	total := len(lines)
	if already >= total {
		return nil, total
	}
	if already < 0 {
		already = 0
	}
	return lines[already:], total
}

// emitNewLines captures the pane, prints lines beyond `already` each prefixed
// "<label> | ", and returns the new total line count for the caller to track.
func emitNewLines(out io.Writer, label string, r lifecycle.Runner, session string, index, already, history int) int {
	raw, _, err := r.Run("tmux", "capture-pane", "-p", "-t",
		session+":"+strconv.Itoa(index), "-S", "-"+strconv.Itoa(history))
	if err != nil {
		return already
	}
	lines, total := newPaneLines(raw, already)
	for _, ln := range lines {
		fmt.Fprintf(out, "  %-14s | %s\n", label, ln)
	}
	return total
}

// captureLastLines returns the last n non-trailing-blank lines of the pane.
func captureLastLines(r lifecycle.Runner, session string, index, n int) []string {
	raw, _, err := r.Run("tmux", "capture-pane", "-p", "-t",
		session+":"+strconv.Itoa(index), "-S", "-"+strconv.Itoa(n*4))
	if err != nil {
		return nil
	}
	lines := trimTrailingBlank(strings.Split(strings.TrimRight(raw, "\n"), "\n"))
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// allListenersServe reports whether every listener URL of cmd answers the probe.
func (a *App) allListenersServe(cmd *config.Command) bool {
	if cmd == nil || len(cmd.Listeners) == 0 {
		return false
	}
	for _, l := range cmd.Listeners {
		if a.probe(l.As) != probeServing {
			return false
		}
	}
	return true
}

func (a *App) streamInterval() time.Duration {
	if a.PollInterval > 0 {
		return a.PollInterval
	}
	return 200 * time.Millisecond
}

// streamServices tails each default's pane and probes its known URL(s) each
// tick. A default flips to ✓ on its first successful fetch, or ✗ crashed when
// its pane dies (with the log tail). Returns when every default is terminal or
// ctx is cancelled (SIGINT). It never imposes a deadline.
func (a *App) streamServices(ctx context.Context, out io.Writer, sessionName string, proj config.Project) {
	n := len(proj.Defaults)
	printed := make([]int, n)
	terminal := make([]bool, n)
	interval := a.streamInterval()

	for {
		allDone := true
		for i, label := range proj.Defaults {
			if terminal[i] {
				continue
			}
			// 1. show new output
			printed[i] = emitNewLines(out, label, a.Runner, sessionName, i, printed[i], streamHistory)

			// 2. first successful fetch wins
			cmd, _ := findCommand(proj, label)
			if a.allListenersServe(cmd) {
				terminal[i] = true
				printServed(out, label, cmd)
				continue
			}

			// 3. dead pane → crashed
			if ps := inspectPane(a.Runner, sessionName, i); ps.dead {
				terminal[i] = true
				printCrashed(out, label, captureLastLines(a.Runner, sessionName, i, crashTailLines), ps.exitStatus)
				continue
			}

			allDone = false
		}
		if allDone {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// streamFrames is the terminal collapse-view: each tick it rebuilds a per-service
// view (tail + probe + pane state) and repaints the frame in place. A service
// collapses to a single ✓ line on its first successful fetch, or a ✗ crashed
// block on pane death. Returns when every default is terminal or ctx is
// cancelled (SIGINT), marking any still-booting service "not responding" in the
// final frame. No deadline.
func (a *App) streamFrames(ctx context.Context, out io.Writer, sessionName string, proj config.Project) {
	n := len(proj.Defaults)
	terminal := make([]bool, n)
	views := make([]svcView, n)
	for i, label := range proj.Defaults {
		views[i] = svcView{label: label, phase: phaseBooting}
	}
	interval := a.streamInterval()
	prevLines, spinner := 0, 0
	width := terminalWidth(out)

	for {
		allDone := true
		for i, label := range proj.Defaults {
			if terminal[i] {
				continue
			}
			cmd, _ := findCommand(proj, label)
			views[i].tail = captureLastLines(a.Runner, sessionName, i, frameTailLines)
			if a.allListenersServe(cmd) {
				terminal[i] = true
				views[i].phase = phaseServing
				views[i].tail = nil
				if cmd != nil && len(cmd.Listeners) > 0 {
					views[i].url = cmd.Listeners[0].As + ".localhost"
				}
				continue
			}
			if ps := inspectPane(a.Runner, sessionName, i); ps.dead {
				terminal[i] = true
				views[i].phase = phaseCrashed
				views[i].exit = ps.exitStatus
				continue
			}
			allDone = false
		}
		frame := renderFrame(views, spinner)
		for i := range frame {
			frame[i] = truncateLine(frame[i], width)
		}
		prevLines = redraw(out, prevLines, frame)
		spinner++
		if allDone {
			return
		}
		select {
		case <-ctx.Done():
			for i := range views {
				if !terminal[i] && views[i].phase == phaseBooting {
					views[i].phase = phaseWedged
				}
			}
			frame := renderFrame(views, spinner)
			for i := range frame {
				frame[i] = truncateLine(frame[i], width)
			}
			redraw(out, prevLines, frame)
			return
		case <-time.After(interval):
		}
	}
}

func printServed(out io.Writer, label string, cmd *config.Command) {
	if len(cmd.Listeners) == 0 {
		return
	}
	fmt.Fprintf(out, "  %-14s ✓  https://%s\n", label, cmd.Listeners[0].As+".localhost")
	for _, l := range cmd.Listeners[1:] {
		fmt.Fprintf(out, "  %-14s    https://%s\n", "", l.As+".localhost")
	}
}

func printCrashed(out io.Writer, label string, tail []string, exit int) {
	fmt.Fprintf(out, "  %-14s ✗  crashed (exit %d)\n", label, exit)
	for _, ln := range tail {
		fmt.Fprintf(out, "  %-14s | %s\n", label, ln)
	}
}
