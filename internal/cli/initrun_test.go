package cli

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// noListenerProctree declared in runner_test.go (Task 13).

func TestTestRunNoListenerTimesOut(t *testing.T) {
	out := &bytes.Buffer{}
	res := testRunCommand(context.Background(), ".", "sh -c 'sleep 0.5; exit 0'",
		out, out, &noListenerProctree{},
		50*time.Millisecond, 200*time.Millisecond)
	if len(res) != 0 {
		t.Errorf("expected zero captured listeners, got %d", len(res))
	}
}

// TestTestRunChildExitsEarly verifies P-4: a child that crashes within
// the first second doesn't make us wait the full timeout.
func TestTestRunChildExitsEarly(t *testing.T) {
	out := &bytes.Buffer{}
	start := time.Now()
	_ = testRunCommand(context.Background(), ".", "sh -c 'exit 1'",
		out, out, &noListenerProctree{},
		200*time.Millisecond, 30*time.Second)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("testRunCommand waited %s for an already-dead child; should be near-instant", elapsed)
	}
}
