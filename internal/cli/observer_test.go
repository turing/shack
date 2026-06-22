package cli

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/turing/shack/internal/proctree"
)

// response holds the return values for a single scripted Run call.
type response struct {
	stdout string
	stderr string
	err    error
}

// scriptedRunner answers a sequence of pgrep/lsof/ps queries.
type scriptedRunner struct {
	mu     sync.Mutex
	step   int
	frames []map[string]response
}

func (s *scriptedRunner) Run(cmd string, args ...string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.step >= len(s.frames) {
		s.step = len(s.frames) - 1
	}
	key := cmd
	for _, a := range args {
		key += " " + a
	}
	r := s.frames[s.step][key]
	return r.stdout, r.stderr, r.err
}

func (s *scriptedRunner) advance() {
	s.mu.Lock()
	s.step++
	s.mu.Unlock()
}

// scriptedRunner implements proctree.Runner directly — no wrapper needed.

func TestObserverDeliversNewListenersOnce(t *testing.T) {
	frames := []map[string]response{
		{ // step 0: no children, no listeners
			"pgrep -P 100": {err: &exitErr{1}},
		},
		{ // step 1: child 200 with listener on :3000
			"pgrep -P 100": {stdout: "200\n"},
			"pgrep -P 200": {err: &exitErr{1}},
			"lsof -nP -a -iTCP -sTCP:LISTEN -p 100,200": {stdout: "COMMAND PID USER FD TYPE DEVICE SIZE NODE NAME\nnode 200 alice 23u IPv4 0x0 0t0 TCP *:3000 (LISTEN)\n"},
			"ps eww -p 200 -o command=":               {stdout: "node /server.js npm_lifecycle_event=dev:server\n"},
		},
	}
	sr := &scriptedRunner{frames: frames}
	var r proctree.Runner = sr

	type event struct {
		l  proctree.Listener
		id proctree.Identifiers
	}
	var (
		mu     sync.Mutex
		events []event
	)
	cb := func(l proctree.Listener, id proctree.Identifiers) {
		mu.Lock()
		events = append(events, event{l, id})
		mu.Unlock()
	}

	ctx, cancel := context.WithCancel(context.Background())
	obs := newObserver(r, 100, cb)
	go obs.Watch(ctx)

	time.Sleep(50 * time.Millisecond)
	sr.advance()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := len(events)
		mu.Unlock()
		if got >= 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	obs.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(events), events)
	}
	if events[0].l.Port != 3000 {
		t.Errorf("port = %d", events[0].l.Port)
	}
	if events[0].id.Script != "dev:server" {
		t.Errorf("script = %q", events[0].id.Script)
	}
}
