package cli

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestStartChildPidAndWait(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	c := exec.Command("sh", "-c", "echo hello-stdout; echo hello-stderr 1>&2; exit 7")
	h, err := startChild(c, stdout, stderr)
	if err != nil {
		t.Fatalf("startChild: %v", err)
	}
	if h.PID <= 0 {
		t.Errorf("PID = %d", h.PID)
	}
	exitCode, werr := h.Wait()
	h.Stop()
	if werr != nil {
		t.Fatalf("Wait: %v", werr)
	}
	if exitCode != 7 {
		t.Errorf("exit = %d, want 7", exitCode)
	}
	if !strings.Contains(stdout.String(), "hello-stdout") {
		t.Errorf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "hello-stderr") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestStartChildBinaryNotFound(t *testing.T) {
	c := exec.Command("/this/binary/does/not/exist")
	if _, err := startChild(c, nil, nil); err == nil {
		t.Error("expected error for missing binary")
	}
}

// TestStartChildNonTerminalStdin confirms that startChild succeeds and
// completes normally when fd 0 is not a terminal (the test-runner
// environment). The tcsetpgrp code path must be skipped silently.
func TestStartChildNonTerminalStdin(t *testing.T) {
	if isFDTerminal(0) {
		t.Skip("fd 0 is a terminal; test targets non-terminal environments")
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	c := exec.Command("sh", "-c", "exit 0")
	h, err := startChild(c, stdout, stderr)
	if err != nil {
		t.Fatalf("startChild on non-terminal fd 0: %v", err)
	}
	exitCode, werr := h.Wait()
	h.Stop()
	if werr != nil {
		t.Fatalf("Wait: %v", werr)
	}
	if exitCode != 0 {
		t.Errorf("exit = %d, want 0", exitCode)
	}
}
