package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/turing/shack/internal/caddyfile"
)

func TestGcDropsDeadEntries(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":                   {stdout: "Homebrew\n"},
		"brew services list":               {stdout: "caddy started alice\n"},
		"lsof -i :3000 -sTCP:LISTEN -P -n": {stdout: "node listening\n"},
		"lsof -i :8080 -sTCP:LISTEN -P -n": {stdout: ""},
	}}
	app, caddyfilePath := newTestApp(t, r)
	prefix, _ := app.Resolver.BrewPrefix()
	r.responses[prefix+"/bin/caddy reload --config "+caddyfilePath+" --adapter caddyfile"] = struct {
		stdout string
		stderr string
		err    error
	}{}

	doc := caddyfile.Document{HasManagedRegion: true, Entries: []caddyfile.Entry{
		{Name: "foo", Port: 3000},
		{Name: "bar", Port: 8080},
	}}
	_ = os.MkdirAll(filepath.Dir(caddyfilePath), 0755)
	_ = os.WriteFile(caddyfilePath, []byte(doc.Render()), 0644)

	out := &bytes.Buffer{}
	if err := app.RunGc(out); err != nil {
		t.Fatalf("RunGc: %v", err)
	}
	if !strings.Contains(out.String(), "dropped bar.localhost") {
		t.Errorf("expected drop log, got: %q", out.String())
	}
	got, _ := os.ReadFile(caddyfilePath)
	if strings.Contains(string(got), "bar.localhost") {
		t.Error("bar should be removed from caddyfile")
	}
}

func TestGcNothingToDo(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	app, _ := newTestApp(t, r)
	out := &bytes.Buffer{}
	if err := app.RunGc(out); err != nil {
		t.Fatalf("RunGc: %v", err)
	}
	if !strings.Contains(out.String(), "nothing to do") {
		t.Errorf("expected 'nothing to do', got: %q", out.String())
	}
}
