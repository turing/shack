package portcheck

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	stdout string
	stderr string
	err    error

	gotCmd  string
	gotArgs []string
}

func (f *fakeRunner) Run(cmd string, args ...string) (string, string, error) {
	f.gotCmd = cmd
	f.gotArgs = args
	return f.stdout, f.stderr, f.err
}

func TestIsAliveTrueWhenLsofPrints(t *testing.T) {
	r := &fakeRunner{stdout: "node    1234 user   23u  IPv4 0x00 0t0  TCP *:3000 (LISTEN)\n"}
	warn := &bytes.Buffer{}
	if !IsAlive(r, 3000, warn) {
		t.Error("expected IsAlive=true when lsof has output")
	}
	if r.gotCmd != "lsof" {
		t.Errorf("expected lsof, got %q", r.gotCmd)
	}
	wantArgs := []string{"-i", ":3000", "-sTCP:LISTEN", "-P", "-n"}
	if !equalSlice(r.gotArgs, wantArgs) {
		t.Errorf("args = %v, want %v", r.gotArgs, wantArgs)
	}
	if warn.Len() != 0 {
		t.Errorf("expected no warning, got %q", warn.String())
	}
}

func TestIsAliveFalseWhenLsofEmpty(t *testing.T) {
	r := &fakeRunner{stdout: "", err: nil}
	if IsAlive(r, 3000, &bytes.Buffer{}) {
		t.Error("expected IsAlive=false when lsof is empty")
	}
}

// lsof returns exit code 1 with empty stdout when nothing matches.
// We treat (empty stdout, exit-code-1 error) as "dead".
func TestIsAliveFalseWhenLsofExitsOne(t *testing.T) {
	r := &fakeRunner{stdout: "", err: &exitErr{code: 1}}
	if IsAlive(r, 3000, &bytes.Buffer{}) {
		t.Error("expected IsAlive=false on lsof exit-1 with empty stdout")
	}
}

// Genuine errors (lsof not installed, signal) should be treated as alive
// AND a warning is emitted to the supplied writer.
func TestIsAliveTrueOnGenuineErrorAndWarns(t *testing.T) {
	r := &fakeRunner{stdout: "", err: errors.New("exec: lsof not found")}
	warn := &bytes.Buffer{}
	if !IsAlive(r, 3000, warn) {
		t.Error("expected IsAlive=true on genuine error (don't GC on uncertainty)")
	}
	got := warn.String()
	if !strings.Contains(got, "lsof") || !strings.Contains(got, "3000") {
		t.Errorf("expected warning mentioning lsof and port, got %q", got)
	}
}

// IsAlive must accept a nil writer without panicking — callers in pure
// query paths (e.g. list) may not want warnings.
func TestIsAliveAcceptsNilWriter(t *testing.T) {
	r := &fakeRunner{stdout: "", err: errors.New("boom")}
	if !IsAlive(r, 3000, nil) {
		t.Error("expected IsAlive=true even with nil writer")
	}
}

type exitErr struct{ code int }

func (e *exitErr) Error() string { return "exit" }
func (e *exitErr) ExitCode() int { return e.code }

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
