package proctree

import (
	"sort"
	"testing"
)

func TestListenersParse(t *testing.T) {
	const out = `COMMAND   PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
node      200 alice   23u  IPv4 0x00              0t0  TCP *:3000 (LISTEN)
node      200 alice   24u  IPv6 0x00              0t0  TCP [::1]:3001 (LISTEN)
vite      300 alice   18u  IPv4 0x00              0t0  TCP 127.0.0.1:5173 (LISTEN)
`
	r := &fakeRunner{table: map[string]response{
		"lsof -nP -a -iTCP -sTCP:LISTEN -p 200,300": {stdout: out},
	}}
	got, err := Listeners(r, []int{200, 300})
	if err != nil {
		t.Fatalf("Listeners: %v", err)
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Port < got[j].Port })
	want := []Listener{
		{PID: 200, Comm: "node", Port: 3000},
		{PID: 200, Comm: "node", Port: 3001},
		{PID: 300, Comm: "vite", Port: 5173},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d listeners, want %d: %+v", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("listener %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestListenersUsesAndFlag verifies that Listeners passes -a to lsof so that
// -p and -iTCP are ANDed, not ORed. On macOS, without -a, lsof returns ALL
// system TCP listeners regardless of the -p value — even for dead or
// nonexistent PIDs. The -a flag (AND selection criteria) is required to
// restrict output to only the requested PIDs.
//
// This test uses a stub Runner that simulates the macOS OR-semantics bug: it
// has a response for the old (no -a) command key that returns rapportd-like
// system-daemon output, and a response for the correct (with -a) command key
// that returns empty (dead PID → no sockets). If Listeners sends the wrong
// command, the bug manifests and the test catches it.
func TestListenersUsesAndFlag(t *testing.T) {
	// Simulated lsof output that macOS would return without -a for a dead PID:
	// all system listeners leak through.
	const macosLeakOut = `COMMAND    PID   USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
rapportd   974 alice   11u  IPv4 0x00              0t0  TCP *:57986 (LISTEN)
ControlCe 1074 alice    8u  IPv4 0x00              0t0  TCP *:7000 (LISTEN)
`
	// With the correct -a flag and a dead PID, lsof returns exit 1 (no match).
	r := &fakeRunner{table: map[string]response{
		// Old (broken) command without -a: simulates macOS leak.
		"lsof -nP -iTCP -sTCP:LISTEN -p 83774": {stdout: macosLeakOut},
		// Correct command with -a: dead PID returns no output (exit 1).
		"lsof -nP -a -iTCP -sTCP:LISTEN -p 83774": {err: &exitErr{1}},
	}}
	got, err := Listeners(r, []int{83774}) // 83774 is a "dead" PID
	if err != nil {
		t.Fatalf("Listeners: %v", err)
	}
	// With the correct -a flag, a dead PID yields no listeners.
	// Without -a, rapportd and ControlCe would appear — that is the bug.
	if len(got) != 0 {
		t.Errorf("Listeners with dead PID: expected 0 listeners, got %d: %+v", len(got), got)
	}
}

func TestListenersEmptyPids(t *testing.T) {
	r := &fakeRunner{table: map[string]response{}}
	got, err := Listeners(r, nil)
	if err != nil {
		t.Fatalf("Listeners: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestListenersNoMatchExitOne(t *testing.T) {
	r := &fakeRunner{table: map[string]response{
		"lsof -nP -a -iTCP -sTCP:LISTEN -p 200": {err: &exitErr{1}},
	}}
	got, err := Listeners(r, []int{200})
	if err != nil {
		t.Fatalf("Listeners: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}
