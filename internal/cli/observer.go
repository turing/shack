package cli

import (
	"context"
	"sync"
	"time"

	"github.com/turing/shack/internal/proctree"
)

// pollInterval is how often the observer queries the process tree.
const pollInterval = 250 * time.Millisecond

// listenerKey uniquely identifies a (pid, port) pair so the observer
// dispatches each one exactly once.
type listenerKey struct {
	PID  int
	Port int
}

// Callback is invoked once per newly observed listener with the listener
// itself and its identifier bundle.
type Callback func(l proctree.Listener, id proctree.Identifiers)

// Observer polls a process tree and delivers each new listener to a
// callback exactly once.
type Observer struct {
	r       proctree.Runner
	rootPID int
	cb      Callback

	mu   sync.Mutex
	seen map[listenerKey]bool

	doneCh chan struct{}
}

func newObserver(r proctree.Runner, rootPID int, cb Callback) *Observer {
	return &Observer{
		r:       r,
		rootPID: rootPID,
		cb:      cb,
		seen:    map[listenerKey]bool{},
		doneCh:  make(chan struct{}),
	}
}

// Watch runs the polling loop until ctx is cancelled. Closes doneCh on
// return so callers can synchronize on Wait().
func (o *Observer) Watch(ctx context.Context) {
	defer close(o.doneCh)
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	o.tick() // run once immediately for fast pickup
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			o.tick()
		}
	}
}

// Wait blocks until Watch has fully returned. Safe to call after the
// context has been cancelled.
func (o *Observer) Wait() { <-o.doneCh }

func (o *Observer) tick() {
	pids, err := proctree.Descendants(o.r, o.rootPID)
	if err != nil || len(pids) == 0 {
		return
	}
	listeners, err := proctree.Listeners(o.r, pids)
	if err != nil {
		// Transient lsof errors (slow filesystem, brief permission flaps)
		// are common; swallow them and try again next tick. Keeping this
		// silent preserves the zero-stderr-bytes invariant during a
		// wrapped run.
		return
	}
	for _, l := range listeners {
		key := listenerKey{PID: l.PID, Port: l.Port}
		o.mu.Lock()
		if o.seen[key] {
			o.mu.Unlock()
			continue
		}
		o.seen[key] = true
		o.mu.Unlock()

		// Identify already pre-populates PID and Comm even on env-read
		// error, so we don't need to rebuild a fallback Identifiers here.
		id, _ := proctree.Identify(o.r, l.PID, l.Comm)
		o.cb(l, id)
	}
}
