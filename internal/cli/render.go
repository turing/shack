package cli

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

type svcPhase int

const (
	phaseBooting svcPhase = iota
	phaseServing
	phaseCrashed
	phaseWedged
)

// svcView is one service's state in a rendered frame.
type svcView struct {
	label string
	phase svcPhase
	url   string   // phaseServing: "<alias>.localhost"
	exit  int      // phaseCrashed
	tail  []string // booting/crashed/wedged: recent pane lines
}

var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

const frameTailLines = 6

// renderFrame builds the lines of one frame. Pure; no I/O. `spinner` indexes
// spinnerFrames modulo its length, so it never panics on a large counter.
func renderFrame(views []svcView, spinner int) []string {
	sp := spinnerFrames[spinner%len(spinnerFrames)]
	var lines []string
	for _, v := range views {
		switch v.phase {
		case phaseServing:
			lines = append(lines, fmt.Sprintf("  %-14s ✓  https://%s", v.label, v.url))
		case phaseCrashed:
			lines = append(lines, fmt.Sprintf("  %-14s ✗  crashed (exit %d)", v.label, v.exit))
			lines = appendTail(lines, v.tail)
		case phaseWedged:
			lines = append(lines, fmt.Sprintf("  %-14s ⚠  not responding", v.label))
			lines = appendTail(lines, v.tail)
		default: // phaseBooting
			lines = append(lines, fmt.Sprintf("  %-14s %c  booting", v.label, sp))
			lines = appendTail(lines, v.tail)
		}
	}
	return lines
}

// appendTail adds indented log lines aligned under the status column.
// It renders at most the last frameTailLines lines of tail.
func appendTail(lines []string, tail []string) []string {
	if len(tail) > frameTailLines {
		tail = tail[len(tail)-frameTailLines:]
	}
	for _, ln := range tail {
		lines = append(lines, fmt.Sprintf("  %-14s |  %s", "", ln))
	}
	return lines
}

// redraw repaints an in-place frame: it moves the cursor up prevLines and
// clears to end of screen (skipped on the first paint, prevLines == 0), then
// writes the frame. Returns the new line count for the next call to pass back.
func redraw(out io.Writer, prevLines int, frame []string) int {
	if prevLines > 0 {
		fmt.Fprintf(out, "\x1b[%dA\x1b[0J", prevLines)
	}
	for _, ln := range frame {
		fmt.Fprintln(out, ln)
	}
	return len(frame)
}

// terminalWidth returns the column count of out's terminal, or 80 if out is
// not a terminal or the width is unavailable.
func terminalWidth(out io.Writer) int {
	f, ok := out.(*os.File)
	if !ok {
		return 80
	}
	ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 {
		return 80
	}
	return int(ws.Col)
}

// truncateLine hard-cuts s to at most width-1 runes so a drawn frame line never
// wraps to a second physical row (which would desync the in-place redraw).
func truncateLine(s string, width int) string {
	if width <= 1 {
		return s
	}
	r := []rune(s)
	if len(r) <= width-1 {
		return s
	}
	return string(r[:width-1])
}

// isTerminal reports whether out is a character device (a real terminal),
// so the in-place collapse view is only used where cursor control works.
func isTerminal(out io.Writer) bool {
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
