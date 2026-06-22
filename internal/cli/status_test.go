package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/turing/shack/internal/caddyfile"
)

func TestStatusRunningWithEntries(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
		// Override the default trusted stub → not trusted, so the ca: not trusted path is exercised.
		"security find-certificate -a -c Caddy Local Authority /Library/Keychains/System.keychain": {stdout: ""},
	}}
	app, caddyfilePath := newTestApp(t, r)
	doc := caddyfile.Document{HasManagedRegion: true, Entries: []caddyfile.Entry{
		{Name: "foo", Port: 3000},
		{Name: "bar", Port: 8080},
	}}
	_ = os.MkdirAll(filepath.Dir(caddyfilePath), 0755)
	_ = os.WriteFile(caddyfilePath, []byte(doc.Render()), 0644)

	out := &bytes.Buffer{}
	if err := app.RunStatus(out); err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "caddy: running") {
		t.Errorf("expected 'caddy: running' in output: %q", s)
	}
	if !strings.Contains(s, "caddyfile: "+caddyfilePath) {
		t.Errorf("expected caddyfile path in output: %q", s)
	}
	if !strings.Contains(s, "entries: 2") {
		t.Errorf("expected entry count in output: %q", s)
	}
	// CA trust check is wired in. With no `security find-certificate`
	// response set, the stubRunner returns empty stdout — interpreted as
	// "not trusted".
	if !strings.Contains(s, "ca: not trusted") {
		t.Errorf("expected ca line in output: %q", s)
	}
}

func TestStatusReportsCATrustedWhenSecurityFindsCert(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
		"security find-certificate -a -c Caddy Local Authority /Library/Keychains/System.keychain": {
			stdout: "keychain: \"/Library/Keychains/System.keychain\"\nattributes:\n    \"labl\"<blob>=\"Caddy Local Authority - 2024 ECC Root\"\n",
		},
	}}
	app, _ := newTestApp(t, r)
	out := &bytes.Buffer{}
	if err := app.RunStatus(out); err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if !strings.Contains(out.String(), "ca: trusted") {
		t.Errorf("expected 'ca: trusted' in output: %q", out.String())
	}
}

func TestStatusStoppedNoFile(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy none alice\n"},
	}}
	app, _ := newTestApp(t, r)
	out := &bytes.Buffer{}
	if err := app.RunStatus(out); err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "caddy: stopped") {
		t.Errorf("expected 'caddy: stopped': %q", s)
	}
	if !strings.Contains(s, "entries: 0") {
		t.Errorf("expected 'entries: 0': %q", s)
	}
}
