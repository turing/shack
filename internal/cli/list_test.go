package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/turing/shack/internal/caddyfile"
)

func TestListPrintsAliveDead(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":                   {stdout: "Homebrew\n"},
		"lsof -i :3000 -sTCP:LISTEN -P -n": {stdout: "node listening\n"},
		"lsof -i :8080 -sTCP:LISTEN -P -n": {stdout: ""},
	}}
	app, caddyfilePath := newTestApp(t, r)
	doc := caddyfile.Document{HasManagedRegion: true, Entries: []caddyfile.Entry{
		{Name: "foo", Port: 3000},
		{Name: "bar", Port: 8080},
	}}
	_ = os.MkdirAll(filepath.Dir(caddyfilePath), 0755)
	_ = os.WriteFile(caddyfilePath, []byte(doc.Render()), 0644)

	out := &bytes.Buffer{}
	if err := app.RunList(out); err != nil {
		t.Fatalf("RunList: %v", err)
	}
	if !strings.Contains(out.String(), "foo.localhost → localhost:3000") {
		t.Errorf("missing foo entry: %s", out.String())
	}
	if !strings.Contains(out.String(), "(alive)") {
		t.Errorf("missing alive marker")
	}
	if !strings.Contains(out.String(), "(dead)") {
		t.Errorf("missing dead marker")
	}
}

func TestListEmpty(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version": {stdout: "Homebrew\n"},
	}}
	app, _ := newTestApp(t, r)
	out := &bytes.Buffer{}
	if err := app.RunList(out); err != nil {
		t.Fatalf("RunList: %v", err)
	}
	if out.String() != "" {
		t.Errorf("expected empty output, got %q", out.String())
	}
}
