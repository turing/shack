package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/turing/shack/internal/caddyfile"
)

func TestRegisterHostnameNewEntry(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	app, caddyfilePath := newTestApp(t, r)
	prefix, _ := app.Resolver.BrewPrefix()
	r.responses[prefix+"/bin/caddy reload --config "+caddyfilePath+" --adapter caddyfile"] = struct {
		stdout string
		stderr string
		err    error
	}{}

	if err := app.RegisterHostname("foo", 3000); err != nil {
		t.Fatalf("RegisterHostname: %v", err)
	}
	got, _ := os.ReadFile(caddyfilePath)
	if !strings.Contains(string(got), "foo.localhost {") {
		t.Errorf("Caddyfile missing entry block:\n%s", got)
	}
	if !strings.Contains(string(got), "reverse_proxy localhost:3000") {
		t.Errorf("Caddyfile missing reverse_proxy line")
	}
}

func TestRegisterHostnameSamePortIsNoOp(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	app, caddyfilePath := newTestApp(t, r)

	doc := caddyfile.Document{HasManagedRegion: true,
		Entries: []caddyfile.Entry{{Name: "foo", Port: 3000}}}
	_ = os.MkdirAll(filepath.Dir(caddyfilePath), 0755)
	seeded := []byte(doc.Render())
	_ = os.WriteFile(caddyfilePath, seeded, 0644)

	if err := app.RegisterHostname("foo", 3000); err != nil {
		t.Fatalf("RegisterHostname: %v", err)
	}
	after, _ := os.ReadFile(caddyfilePath)
	if string(after) != string(seeded) {
		t.Errorf("Caddyfile rewritten on no-op:\nbefore: %q\nafter:  %q", seeded, after)
	}
}

func TestUnregisterHostnamesRemoves(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	app, caddyfilePath := newTestApp(t, r)
	prefix, _ := app.Resolver.BrewPrefix()
	r.responses[prefix+"/bin/caddy reload --config "+caddyfilePath+" --adapter caddyfile"] = struct {
		stdout string
		stderr string
		err    error
	}{}

	doc := caddyfile.Document{HasManagedRegion: true, Entries: []caddyfile.Entry{
		{Name: "alpha", Port: 3000},
		{Name: "beta", Port: 4000},
	}}
	_ = os.MkdirAll(filepath.Dir(caddyfilePath), 0755)
	_ = os.WriteFile(caddyfilePath, []byte(doc.Render()), 0644)

	if err := app.UnregisterHostnames([]string{"alpha", "gamma"}); err != nil {
		t.Fatalf("UnregisterHostnames: %v", err)
	}
	got, _ := os.ReadFile(caddyfilePath)
	if strings.Contains(string(got), "alpha.localhost") {
		t.Errorf("alpha should be removed")
	}
	if !strings.Contains(string(got), "beta.localhost") {
		t.Errorf("beta should remain")
	}
}
