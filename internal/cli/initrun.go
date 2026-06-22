package cli

import (
	"context"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/turing/shack/internal/proctree"
)

// observedListener is one (listener, identifiers) pair captured during
// an init test-run.
type observedListener struct {
	Listener proctree.Listener
	ID       proctree.Identifiers
}

// testRunCommand spawns cmdLine, watches listeners for the lifetime of
// the child, settles `settle` after the most recent new listener (or
// hard-caps at `timeout` if none arrives), exits immediately if the
// child exits on its own, then sends SIGINT and waits. Returns the
// captured listeners in arrival order.
func testRunCommand(
	ctx context.Context,
	projectRoot string,
	cmdLine string,
	stdout, stderr io.Writer,
	procRunner proctree.Runner,
	settle, timeout time.Duration,
) []observedListener {
	c := exec.Command("sh", "-c", cmdLine)
	c.Dir = projectRoot

	h, err := startChild(c, stdout, stderr)
	if err != nil {
		// startChild already wraps the error in "spawn ...: %w" — but
		// callers (init) handle the missing-listener case the same way:
		// skip the script. Returning empty captures matches that flow.
		return nil
	}
	defer h.Stop()

	var (
		mu             sync.Mutex
		captured       []observedListener
		lastListenerAt time.Time
	)

	cb := func(l proctree.Listener, id proctree.Identifiers) {
		mu.Lock()
		captured = append(captured, observedListener{Listener: l, ID: id})
		lastListenerAt = time.Now()
		mu.Unlock()
	}

	watchCtx, cancelWatch := context.WithCancel(ctx)
	obs := newObserver(procRunner, h.PID, cb)
	go obs.Watch(watchCtx)

	// childExited closes when h.Wait() returns. The select below uses it
	// to bail out the moment a script crashes — without this branch, a
	// broken child made the loop wait the full 30s deadline before
	// SIGINT-ing a corpse.
	childExited := make(chan struct{})
	go func() {
		_, _ = h.Wait()
		close(childExited)
	}()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	settleCheck := time.NewTicker(100 * time.Millisecond)
	defer settleCheck.Stop()

loop:
	for {
		select {
		case <-deadline.C:
			break loop
		case <-settleCheck.C:
			mu.Lock()
			settled := !lastListenerAt.IsZero() && time.Since(lastListenerAt) >= settle
			mu.Unlock()
			if settled {
				break loop
			}
		case <-childExited:
			// Child died on its own; collect what we have and return.
			break loop
		case <-ctx.Done():
			break loop
		}
	}

	// Decide whether the child is still alive via a non-blocking select
	// on childExited. A bool flipped from the wait goroutine would be a
	// data race; reading the channel state is the goroutine-safe form.
	exited := false
	select {
	case <-childExited:
		exited = true
	default:
	}
	if !exited {
		// Child still alive: signal it, then wait up to 2s before SIGKILL.
		_ = h.SendSignal(syscall.SIGINT)
		select {
		case <-childExited:
		case <-time.After(2 * time.Second):
			_ = h.SendSignal(syscall.SIGKILL)
			<-childExited
		}
	}
	cancelWatch()
	obs.Wait()

	mu.Lock()
	defer mu.Unlock()
	out := make([]observedListener, len(captured))
	copy(out, captured)
	return out
}
