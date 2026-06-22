package proctree

import (
	"errors"
	"sort"
	"testing"
)

type fakeRunner struct{ table map[string]response }
type response struct {
	stdout string
	stderr string
	err    error
}

func (f *fakeRunner) Run(cmd string, args ...string) (string, string, error) {
	key := cmd
	for _, a := range args {
		key += " " + a
	}
	r, ok := f.table[key]
	if !ok {
		return "", "", errors.New("no fake response for: " + key)
	}
	return r.stdout, r.stderr, r.err
}

type exitErr struct{ code int }

func (e *exitErr) Error() string { return "exit" }
func (e *exitErr) ExitCode() int { return e.code }

// TestDescendantsRejectZeroRoot verifies that root <= 0 is refused immediately.
// Without this guard, Descendants(r, 0) would return [0] and a subsequent
// Listeners([0]) call could — on macOS — return all system TCP listeners because
// lsof treats -p and -i as OR conditions without the -a flag.
func TestDescendantsRejectZeroRoot(t *testing.T) {
	r := &fakeRunner{table: map[string]response{}}
	for _, root := range []int{0, -1, -100} {
		got, err := Descendants(r, root)
		if err == nil {
			t.Errorf("Descendants(r, %d): expected error, got %v", root, got)
		}
		if len(got) != 0 {
			t.Errorf("Descendants(r, %d): expected empty slice on error, got %v", root, got)
		}
	}
}

func TestDescendantsLeafProcess(t *testing.T) {
	r := &fakeRunner{table: map[string]response{
		"pgrep -P 100": {err: &exitErr{1}},
	}}
	got, err := Descendants(r, 100)
	if err != nil {
		t.Fatalf("Descendants: %v", err)
	}
	if len(got) != 1 || got[0] != 100 {
		t.Errorf("got %v, want [100]", got)
	}
}

func TestDescendantsTree(t *testing.T) {
	r := &fakeRunner{table: map[string]response{
		"pgrep -P 100": {stdout: "200\n300\n"},
		"pgrep -P 200": {stdout: "400\n"},
		"pgrep -P 300": {err: &exitErr{1}},
		"pgrep -P 400": {err: &exitErr{1}},
	}}
	got, err := Descendants(r, 100)
	if err != nil {
		t.Fatalf("Descendants: %v", err)
	}
	sort.Ints(got)
	want := []int{100, 200, 300, 400}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}
