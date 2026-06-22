package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/turing/shack/internal/caddyfile"
	"github.com/turing/shack/internal/lifecycle"
)

// newTestApp builds a CLI App backed by the given stubRunner and a
// per-test temp directory. The fake brew --prefix returns the temp dir, so
// the resolver computes the Caddyfile path as <tmp>/etc/Caddyfile and the
// caddy binary path as <tmp>/bin/caddy. Both subdirectories are created so
// real file operations (lockfile, atomic write) work.
func newTestApp(t *testing.T, runner *stubRunner) (*App, string) {
	t.Helper()
	dir := t.TempDir()

	if runner.responses == nil {
		runner.responses = stubResponses{}
	}
	if _, ok := runner.responses["brew --prefix"]; !ok {
		runner.responses["brew --prefix"] = struct {
			stdout string
			stderr string
			err    error
		}{stdout: dir + "\n"}
	}
	if _, ok := runner.responses["security find-certificate -a -c Caddy Local Authority /Library/Keychains/System.keychain"]; !ok {
		runner.responses["security find-certificate -a -c Caddy Local Authority /Library/Keychains/System.keychain"] = struct {
			stdout string
			stderr string
			err    error
		}{stdout: "keychain: \"/Library/Keychains/System.keychain\"\nattributes:\n    \"labl\"<blob>=\"Caddy Local Authority - 2024 ECC Root\"\n"}
	}

	if err := os.MkdirAll(filepath.Join(dir, "etc"), 0755); err != nil {
		t.Fatalf("mkdir etc: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	res := lifecycle.NewResolver(runner)
	app := &App{
		Runner:     runner,
		Resolver:   res,
		Caddy:      lifecycle.NewCaddy(runner, res),
		FileExists: func(string) bool { return true },
	}
	caddyfilePath := filepath.Join(dir, "etc", "Caddyfile")
	return app, caddyfilePath
}

func TestAddCommandWritesEntry(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
		// reload happens inside add; resolver fills in the binary path
	}}
	app, caddyfilePath := newTestApp(t, r)

	// Pre-register the caddy binary path with reload responding empty
	prefix, _ := app.Resolver.BrewPrefix()
	r.responses[prefix+"/bin/caddy reload --config "+caddyfilePath+" --adapter caddyfile"] = struct {
		stdout string
		stderr string
		err    error
	}{}
	// Simulate port 3000 being alive.
	r.responses["lsof -i :3000 -sTCP:LISTEN -P -n"] = struct {
		stdout string
		stderr string
		err    error
	}{stdout: "node 1 alice 23u IPv4 0x0 0t0 TCP *:3000 (LISTEN)\n"}

	out := &bytes.Buffer{}
	err := app.RunAdd(out, "foo", 3000)
	if err != nil {
		t.Fatalf("RunAdd: %v", err)
	}

	got, err := os.ReadFile(caddyfilePath)
	if err != nil {
		t.Fatalf("read caddyfile: %v", err)
	}
	if !strings.Contains(string(got), "foo.localhost {") {
		t.Errorf("caddyfile missing entry block:\n%s", got)
	}
	if !strings.Contains(string(got), "reverse_proxy localhost:3000") {
		t.Errorf("caddyfile missing reverse_proxy:\n%s", got)
	}
	if !strings.Contains(out.String(), "foo.localhost → localhost:3000") {
		t.Errorf("stdout missing success line: %q", out.String())
	}
}

func TestAddCommandRejectsInvalidName(t *testing.T) {
	r := &stubRunner{responses: stubResponses{}}
	app, _ := newTestApp(t, r)
	err := app.RunAdd(&bytes.Buffer{}, "Foo Bar", 3000)
	if _, ok := err.(*userError); !ok {
		t.Errorf("expected userError, got %T: %v", err, err)
	}
}

func TestAddCommandRejectsInvalidPort(t *testing.T) {
	r := &stubRunner{responses: stubResponses{}}
	app, _ := newTestApp(t, r)
	err := app.RunAdd(&bytes.Buffer{}, "foo", 99999)
	if _, ok := err.(*userError); !ok {
		t.Errorf("expected userError, got %T: %v", err, err)
	}
}

func TestAddCommandSamePortNoOp(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew services list":               {stdout: "caddy started alice\n"},
		"lsof -i :3000 -sTCP:LISTEN -P -n": {stdout: "node listening\n"},
	}}
	app, caddyfilePath := newTestApp(t, r)

	// Seed the caddyfile with foo:3000 already registered.
	doc := caddyfile.Document{HasManagedRegion: true, Entries: []caddyfile.Entry{{Name: "foo", Port: 3000}}}
	_ = os.MkdirAll(filepath.Dir(caddyfilePath), 0755)
	seeded := []byte(doc.Render())
	_ = os.WriteFile(caddyfilePath, seeded, 0644)

	out := &bytes.Buffer{}
	if err := app.RunAdd(out, "foo", 3000); err != nil {
		t.Fatalf("RunAdd: %v", err)
	}
	got, err := os.ReadFile(caddyfilePath)
	if err != nil {
		t.Fatalf("read caddyfile: %v", err)
	}
	if !bytes.Equal(got, seeded) {
		t.Errorf("expected no rewrite when already registered.\nbefore: %q\nafter:  %q", seeded, got)
	}
	if !strings.Contains(out.String(), "already registered") {
		t.Errorf("expected 'already registered' in output: %q", out.String())
	}
}

func TestAddCommandReportsSweptEntries(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
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

	// Seed: foo:3000 alive, bar:8080 dead.
	doc := caddyfile.Document{HasManagedRegion: true, Entries: []caddyfile.Entry{
		{Name: "foo", Port: 3000},
		{Name: "bar", Port: 8080},
	}}
	_ = os.WriteFile(caddyfilePath, []byte(doc.Render()), 0644)

	out := &bytes.Buffer{}
	// add only sweeps existing entries — it doesn't probe the new port — so
	// 9000's lsof response is irrelevant here.
	if err := app.RunAdd(out, "baz", 9000); err != nil {
		t.Fatalf("RunAdd: %v", err)
	}
	// The sweep dropped bar; add added baz.
	if !strings.Contains(out.String(), "gc: dropped bar.localhost") {
		t.Errorf("expected sweep report for bar, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "baz.localhost → localhost:9000") {
		t.Errorf("expected baz registration line, got: %q", out.String())
	}
}
