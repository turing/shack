package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/turing/shack/internal/caddyfile"
)

func TestRmCommandRemovesEntry(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	app, caddyfilePath := newTestApp(t, r)
	prefix, _ := app.Resolver.BrewPrefix()
	r.responses[prefix+"/bin/caddy reload --config "+caddyfilePath+" --adapter caddyfile"] = struct {
		stdout string
		stderr string
		err    error
	}{}
	r.responses["lsof -i :3000 -sTCP:LISTEN -P -n"] = struct {
		stdout string
		stderr string
		err    error
	}{stdout: "node listening\n"}

	doc := caddyfile.Document{HasManagedRegion: true, Entries: []caddyfile.Entry{{Name: "foo", Port: 3000}}}
	_ = os.MkdirAll(filepath.Dir(caddyfilePath), 0755)
	_ = os.WriteFile(caddyfilePath, []byte(doc.Render()), 0644)

	out := &bytes.Buffer{}
	if err := app.RunRm(out, "foo"); err != nil {
		t.Fatalf("RunRm: %v", err)
	}
	got, _ := os.ReadFile(caddyfilePath)
	if strings.Contains(string(got), "foo.localhost") {
		t.Error("foo entry should be gone")
	}
	if !strings.Contains(out.String(), "removed foo.localhost") {
		t.Errorf("output missing success line: %q", out.String())
	}
}

func TestRmCommandUnknownNameNoOp(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	app, _ := newTestApp(t, r)

	out := &bytes.Buffer{}
	if err := app.RunRm(out, "nope"); err != nil {
		t.Fatalf("RunRm: %v", err)
	}
	if !strings.Contains(out.String(), "nope not registered") {
		t.Errorf("output: %q", out.String())
	}
}
