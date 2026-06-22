package cli

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/turing/shack/internal/config"
)

func TestNewPaneLines(t *testing.T) {
	// Trailing blank lines (terminal padding) are trimmed; lines past `already`
	// are returned with the new total.
	raw := "booting\nlistening on :51372\n\n\n"
	lines, total := newPaneLines(raw, 0)
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(lines) != 2 || lines[0] != "booting" || lines[1] != "listening on :51372" {
		t.Fatalf("lines = %#v", lines)
	}
	// Second call after already=2 yields nothing new.
	lines2, total2 := newPaneLines(raw, 2)
	if total2 != 2 || len(lines2) != 0 {
		t.Errorf("second call: lines=%#v total=%d, want none/2", lines2, total2)
	}
	// already beyond current length is clamped (history scrolled).
	lines3, _ := newPaneLines(raw, 99)
	if len(lines3) != 0 {
		t.Errorf("clamp: got %#v, want none", lines3)
	}
}

func TestEmitNewLines(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"tmux capture-pane -p -t sess:0 -S -1000": {stdout: "one\ntwo\n\n"},
	}}
	out := &bytes.Buffer{}
	total := emitNewLines(out, "dev:api", r, "sess", 0, 0, streamHistory)
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	got := out.String()
	prefix := fmt.Sprintf("  %-14s | ", "dev:api")
	for _, want := range []string{prefix + "one", prefix + "two"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestNewPaneLinesNegativeAlready(t *testing.T) {
	lines, total := newPaneLines("a\nb\n", -5)
	if total != 2 || len(lines) != 2 || lines[0] != "a" || lines[1] != "b" {
		t.Errorf("negative already: lines=%#v total=%d, want [a b]/2", lines, total)
	}
}

func TestCaptureLastLines(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"tmux capture-pane -p -t sess:0 -S -60": {stdout: "a\nb\nERROR boom\n\n"},
	}}
	got := captureLastLines(r, "sess", 0, 15)
	if len(got) != 3 || got[2] != "ERROR boom" {
		t.Errorf("got %#v, want [a b ERROR boom]", got)
	}
}

func TestStreamServicesServes(t *testing.T) {
	sessionName := "shack-demo"
	r := &stubRunner{responses: stubResponses{
		"tmux capture-pane -p -t " + sessionName + ":0 -S -1000": {stdout: "vite ready\nLocal: http://localhost:51373\n"},
		"tmux list-panes -t " + sessionName + ":0 -F #{pane_dead} #{pane_dead_status}": {stdout: "0 \n"},
	}}
	app, _ := newTestApp(t, r)
	app.PollInterval = 5 * time.Millisecond
	app.Probe = func(host string) probeOutcome { return probeServing } // serves immediately

	proj := config.Project{
		Name:     "demo",
		Defaults: []string{"dev:ui"},
		Commands: []config.Command{{
			Label:     "dev:ui",
			Listeners: []config.Listener{{Match: config.Match{Script: "dev:ui"}, As: "demo"}},
		}},
	}
	out := &bytes.Buffer{}
	app.streamServices(context.Background(), out, sessionName, proj)

	got := out.String()
	if !strings.Contains(got, "  dev:ui         | vite ready") {
		t.Errorf("expected streamed log line, got:\n%s", got)
	}
	if !strings.Contains(got, "✓  https://demo.localhost") {
		t.Errorf("expected served line, got:\n%s", got)
	}
}

func TestStreamServicesCrash(t *testing.T) {
	sessionName := "shack-demo"
	r := &stubRunner{responses: stubResponses{
		"tmux capture-pane -p -t " + sessionName + ":0 -S -1000": {stdout: "boot\nError: listen EADDRINUSE :51372\n"},
		"tmux list-panes -t " + sessionName + ":0 -F #{pane_dead} #{pane_dead_status}": {stdout: "1 1\n"},
		"tmux capture-pane -p -t " + sessionName + ":0 -S -60":   {stdout: "boot\nError: listen EADDRINUSE :51372\n"},
	}}
	app, _ := newTestApp(t, r)
	app.PollInterval = 5 * time.Millisecond
	app.Probe = func(host string) probeOutcome { return probeUnreachable } // never serves

	proj := config.Project{
		Name:     "demo",
		Defaults: []string{"dev:api"},
		Commands: []config.Command{{
			Label:     "dev:api",
			Listeners: []config.Listener{{Match: config.Match{Script: "dev:api"}, As: "demo-api"}},
		}},
	}
	out := &bytes.Buffer{}
	app.streamServices(context.Background(), out, sessionName, proj)

	got := out.String()
	if !strings.Contains(got, "✗  crashed (exit 1)") || !strings.Contains(got, "EADDRINUSE") {
		t.Errorf("expected crash + tail, got:\n%s", got)
	}
}

func TestStreamFramesServes(t *testing.T) {
	sessionName := "shack-demo"
	r := &stubRunner{responses: stubResponses{
		"tmux capture-pane -p -t " + sessionName + ":0 -S -60": {stdout: "vite ready\n"},
		"tmux list-panes -t " + sessionName + ":0 -F #{pane_dead} #{pane_dead_status}": {stdout: "0 \n"},
	}}
	app, _ := newTestApp(t, r)
	app.PollInterval = 5 * time.Millisecond
	app.Probe = func(string) probeOutcome { return probeServing }

	proj := config.Project{
		Name: "demo", Defaults: []string{"dev:ui"},
		Commands: []config.Command{{Label: "dev:ui", Listeners: []config.Listener{{Match: config.Match{Script: "dev:ui"}, As: "demo"}}}},
	}
	out := &bytes.Buffer{}
	app.streamFrames(context.Background(), out, sessionName, proj)
	if !strings.Contains(out.String(), "✓  https://demo.localhost") {
		t.Errorf("expected collapsed serving line, got:\n%s", out.String())
	}
}

func TestStreamFramesCrash(t *testing.T) {
	sessionName := "shack-demo"
	r := &stubRunner{responses: stubResponses{
		"tmux capture-pane -p -t " + sessionName + ":0 -S -24": {stdout: "boot\nEADDRINUSE :51372\n"},
		"tmux list-panes -t " + sessionName + ":0 -F #{pane_dead} #{pane_dead_status}": {stdout: "1 1\n"},
	}}
	app, _ := newTestApp(t, r)
	app.PollInterval = 5 * time.Millisecond
	app.Probe = func(string) probeOutcome { return probeUnreachable }

	proj := config.Project{
		Name: "demo", Defaults: []string{"dev:api"},
		Commands: []config.Command{{Label: "dev:api", Listeners: []config.Listener{{Match: config.Match{Script: "dev:api"}, As: "demo-api"}}}},
	}
	out := &bytes.Buffer{}
	app.streamFrames(context.Background(), out, sessionName, proj)
	got := out.String()
	if !strings.Contains(got, "✗  crashed (exit 1)") || !strings.Contains(got, "EADDRINUSE") {
		t.Errorf("expected crash block + tail, got:\n%s", got)
	}
}

func TestStreamFramesInterruptWedged(t *testing.T) {
	sessionName := "shack-demo"
	r := &stubRunner{responses: stubResponses{
		"tmux capture-pane -p -t " + sessionName + ":0 -S -60": {stdout: "still booting\n"},
		"tmux list-panes -t " + sessionName + ":0 -F #{pane_dead} #{pane_dead_status}": {stdout: "0 \n"},
	}}
	app, _ := newTestApp(t, r)
	app.PollInterval = 5 * time.Millisecond
	app.Probe = func(string) probeOutcome { return probeUnreachable }

	proj := config.Project{
		Name: "demo", Defaults: []string{"dev:api"},
		Commands: []config.Command{{Label: "dev:api", Listeners: []config.Listener{{Match: config.Match{Script: "dev:api"}, As: "demo-api"}}}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	out := &bytes.Buffer{}
	go func() { app.streamFrames(ctx, out, sessionName, proj); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("streamFrames did not return after ctx cancel")
	}
	if !strings.Contains(out.String(), "not responding") {
		t.Errorf("expected wedged marker on interrupt, got:\n%s", out.String())
	}
}

func TestStreamServicesInterrupt(t *testing.T) {
	// A service that never serves and never dies: the loop must return when ctx
	// is cancelled (SIGINT), not hang.
	sessionName := "shack-demo"
	r := &stubRunner{responses: stubResponses{
		"tmux capture-pane -p -t " + sessionName + ":0 -S -1000": {stdout: "still booting\n"},
		"tmux list-panes -t " + sessionName + ":0 -F #{pane_dead} #{pane_dead_status}": {stdout: "0 \n"},
	}}
	app, _ := newTestApp(t, r)
	app.PollInterval = 5 * time.Millisecond
	app.Probe = func(host string) probeOutcome { return probeUnreachable }

	proj := config.Project{
		Name:     "demo",
		Defaults: []string{"dev:api"},
		Commands: []config.Command{{
			Label:     "dev:api",
			Listeners: []config.Listener{{Match: config.Match{Script: "dev:api"}, As: "demo-api"}},
		}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	out := &bytes.Buffer{}
	go func() { app.streamServices(ctx, out, sessionName, proj); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("streamServices did not return after ctx cancel")
	}
}
