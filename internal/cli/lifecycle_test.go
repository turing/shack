package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestStartCommandIdempotent(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	app, _ := newTestApp(t, r)
	out := &bytes.Buffer{}
	if err := app.RunUp(out); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	if !strings.Contains(out.String(), "already running") {
		t.Errorf("expected 'already running', got %q", out.String())
	}
}

func TestStartCommandStartsWhenStopped(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":            {stdout: "Homebrew\n"},
		"brew services list":        {stdout: "caddy none alice\n"},
		"brew services start caddy": {},
	}}
	app, _ := newTestApp(t, r)
	out := &bytes.Buffer{}
	if err := app.RunUp(out); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	if !strings.Contains(out.String(), "started") {
		t.Errorf("expected 'started', got %q", out.String())
	}
}

func TestStopCommand(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":           {stdout: "Homebrew\n"},
		"brew services stop caddy": {},
	}}
	app, _ := newTestApp(t, r)
	out := &bytes.Buffer{}
	if err := app.RunDown(out, false); err != nil {
		t.Fatalf("RunDown: %v", err)
	}
	if !strings.Contains(out.String(), "stopped") {
		t.Errorf("expected 'stopped', got %q", out.String())
	}
}

func TestReloadCommand(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version": {stdout: "Homebrew\n"},
	}}
	app, caddyfilePath := newTestApp(t, r)
	prefix, _ := app.Resolver.BrewPrefix()
	r.responses[prefix+"/bin/caddy reload --config "+caddyfilePath+" --adapter caddyfile"] = struct {
		stdout string
		stderr string
		err    error
	}{}
	out := &bytes.Buffer{}
	if err := app.RunReload(out); err != nil {
		t.Fatalf("RunReload: %v", err)
	}
	if !strings.Contains(out.String(), "reloaded") {
		t.Errorf("expected 'reloaded', got %q", out.String())
	}
}
