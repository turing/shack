package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestPrintInitWarnings(t *testing.T) {
	var buf bytes.Buffer
	msg := "warning: multiple lockfiles found (pnpm-lock.yaml, package-lock.json)\n" +
		"using pnpm — if commands fail, remove the stale lockfile"
	printInitWarnings(&buf, []string{msg})
	out := buf.String()
	// On a non-TTY writer lipgloss strips styling, so the badge renders as
	// plain " warning " text.
	if !strings.Contains(out, "warning") {
		t.Errorf("expected warning badge text in output, got %q", out)
	}
	// Every line of the message carries the bar prefix.
	for _, want := range []string{
		barChar + "  warning  multiple lockfiles found (pnpm-lock.yaml, package-lock.json)\n",
		barChar + " using pnpm — if commands fail, remove the stale lockfile\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected line %q in output, got %q", want, out)
		}
	}
	// Trailing blank line separates the warning from the following form.
	if !strings.HasSuffix(out, "\n\n") {
		t.Errorf("expected trailing blank line, got %q", out)
	}
}

func TestPrintInitWarningsEmpty(t *testing.T) {
	var buf bytes.Buffer
	printInitWarnings(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty warnings, got %q", buf.String())
	}
}

func TestRenderFrameServing(t *testing.T) {
	got := renderFrame([]svcView{{label: "dev:ui", phase: phaseServing, url: "demo.localhost"}}, 0)
	if len(got) != 1 {
		t.Fatalf("serving should be one line, got %d: %#v", len(got), got)
	}
	if !strings.Contains(got[0], "dev:ui") || !strings.Contains(got[0], "✓  https://demo.localhost") {
		t.Errorf("got %q", got[0])
	}
}

func TestRenderFrameBooting(t *testing.T) {
	got := renderFrame([]svcView{{label: "dev:api", phase: phaseBooting, tail: []string{"line one", "line two"}}}, 0)
	// header + 2 tail lines
	if len(got) != 3 {
		t.Fatalf("booting block should be header+2 tail, got %d: %#v", len(got), got)
	}
	if !strings.Contains(got[0], "dev:api") || !strings.Contains(got[0], "booting") {
		t.Errorf("header = %q", got[0])
	}
	if !strings.Contains(got[1], "line one") || !strings.Contains(got[2], "line two") {
		t.Errorf("tail = %#v", got[1:])
	}
	// header carries the spinner rune for index 0
	if !strings.ContainsRune(got[0], spinnerFrames[0]) {
		t.Errorf("header missing spinner: %q", got[0])
	}
}

func TestRenderFrameCrashed(t *testing.T) {
	got := renderFrame([]svcView{{label: "dev:api", phase: phaseCrashed, exit: 1, tail: []string{"EADDRINUSE :51372"}}}, 0)
	if len(got) != 2 {
		t.Fatalf("crashed should be header+1 tail, got %#v", got)
	}
	if !strings.Contains(got[0], "✗  crashed (exit 1)") {
		t.Errorf("header = %q", got[0])
	}
	if !strings.Contains(got[1], "EADDRINUSE :51372") {
		t.Errorf("tail = %q", got[1])
	}
}

func TestRenderFrameSpinnerWraps(t *testing.T) {
	// spinner index beyond the frame count wraps, never panics.
	got := renderFrame([]svcView{{label: "x", phase: phaseBooting}}, len(spinnerFrames)+3)
	if !strings.ContainsRune(got[0], spinnerFrames[3]) {
		t.Errorf("expected wrapped spinner frame 3, got %q", got[0])
	}
}

func TestRenderFrameWedged(t *testing.T) {
	got := renderFrame([]svcView{{label: "dev:api", phase: phaseWedged, tail: []string{"still booting"}}}, 0)
	if len(got) != 2 {
		t.Fatalf("wedged should be header+1 tail, got %#v", got)
	}
	if !strings.Contains(got[0], "⚠  not responding") {
		t.Errorf("header = %q", got[0])
	}
	if !strings.Contains(got[1], "still booting") {
		t.Errorf("tail = %q", got[1])
	}
}

func TestRenderFrameTailCapped(t *testing.T) {
	var tail []string
	for i := 0; i < frameTailLines+4; i++ {
		tail = append(tail, fmt.Sprintf("line%d", i))
	}
	got := renderFrame([]svcView{{label: "x", phase: phaseBooting, tail: tail}}, 0)
	// header + at most frameTailLines tail lines
	if len(got) != 1+frameTailLines {
		t.Fatalf("want header+%d, got %d lines", frameTailLines, len(got))
	}
	// the oldest lines are dropped; the last tail line survives
	last := got[len(got)-1]
	if !strings.Contains(last, fmt.Sprintf("line%d", frameTailLines+3)) {
		t.Errorf("expected last tail line retained, got %q", last)
	}
	// the very first dropped line must not appear
	for _, ln := range got {
		if strings.Contains(ln, "line0 ") || strings.HasSuffix(ln, "line0") {
			t.Errorf("oldest line should have been capped out: %q", ln)
		}
	}
}

func TestRedrawFirstCall(t *testing.T) {
	out := &bytes.Buffer{}
	n := redraw(out, 0, []string{"a", "b"})
	if n != 2 {
		t.Fatalf("returned %d, want 2", n)
	}
	got := out.String()
	if strings.Contains(got, "\x1b[") {
		t.Errorf("first call must not emit cursor-move codes: %q", got)
	}
	if got != "a\nb\n" {
		t.Errorf("got %q, want \"a\\nb\\n\"", got)
	}
}

func TestRedrawClearsPrevious(t *testing.T) {
	out := &bytes.Buffer{}
	n := redraw(out, 3, []string{"x"})
	if n != 1 {
		t.Fatalf("returned %d, want 1", n)
	}
	got := out.String()
	if !strings.HasPrefix(got, "\x1b[3A\x1b[0J") {
		t.Errorf("expected cursor-up-3 + clear-below prefix, got %q", got)
	}
	if !strings.HasSuffix(got, "x\n") {
		t.Errorf("expected frame text after the clear, got %q", got)
	}
}

func TestIsTerminalBufferFalse(t *testing.T) {
	if isTerminal(&bytes.Buffer{}) {
		t.Error("a bytes.Buffer is not a terminal")
	}
}

func TestTruncateLineShort(t *testing.T) {
	if got := truncateLine("hi", 80); got != "hi" {
		t.Errorf("short line changed: %q", got)
	}
}

func TestTruncateLineLong(t *testing.T) {
	got := truncateLine("abcdefghij", 5) // width 5 -> cut to 4 runes
	if got != "abcd" {
		t.Errorf("got %q, want abcd", got)
	}
}

func TestTerminalWidthBufferDefault(t *testing.T) {
	if w := terminalWidth(&bytes.Buffer{}); w != 80 {
		t.Errorf("non-terminal width = %d, want 80 default", w)
	}
}
