package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/turing/shack/internal/lifecycle"
)

type stubResponses map[string]struct {
	stdout string
	stderr string
	err    error
}

type stubRunner struct {
	responses stubResponses
	calls     []string
}

func (s *stubRunner) Run(cmd string, args ...string) (string, string, error) {
	key := cmd
	for _, a := range args {
		key += " " + a
	}
	s.calls = append(s.calls, key)
	r := s.responses[key]
	return r.stdout, r.stderr, r.err
}

func TestPreflightOK(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version": {stdout: "Homebrew 4.0\n"},
		"brew --prefix":  {stdout: "/opt/homebrew\n"},
	}}
	app := &App{
		Runner:   r,
		Resolver: lifecycle.NewResolver(r),
		FileExists: func(path string) bool {
			return path == "/opt/homebrew/bin/caddy"
		},
	}
	if err := app.Preflight(); err != nil {
		t.Fatalf("Preflight: %v", err)
	}
}

func TestPreflightBrewMissing(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version": {err: errors.New("not found")},
	}}
	app := &App{
		Runner:     r,
		Resolver:   lifecycle.NewResolver(r),
		FileExists: func(string) bool { return true },
	}
	err := app.Preflight()
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*userError); !ok {
		t.Errorf("expected userError, got %T", err)
	}
}

func TestPreflightCaddyMissing(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version": {stdout: "Homebrew\n"},
		"brew --prefix":  {stdout: "/opt/homebrew\n"},
	}}
	app := &App{
		Runner:     r,
		Resolver:   lifecycle.NewResolver(r),
		FileExists: func(string) bool { return false },
	}
	err := app.Preflight()
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*userError); !ok {
		t.Errorf("expected userError, got %T", err)
	}
}

func TestPreflightTouchingCaddyfileCAUntrusted(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version": {stdout: "Homebrew\n"},
		"brew --prefix":  {stdout: "/opt/homebrew\n"},
		// security find-certificate returns empty stdout → IsCATrusted=false.
		"security find-certificate -a -c Caddy Local Authority /Library/Keychains/System.keychain": {stdout: ""},
	}}
	app := &App{
		Runner:     r,
		Resolver:   lifecycle.NewResolver(r),
		Caddy:      lifecycle.NewCaddy(r, lifecycle.NewResolver(r)),
		FileExists: func(string) bool { return true },
	}
	err := app.preflightTouchingCaddyfile()
	if err == nil {
		t.Fatal("expected CA-untrusted userError, got nil")
	}
	ue, ok := err.(*userError)
	if !ok {
		t.Fatalf("expected *userError, got %T: %v", err, err)
	}
	if !strings.Contains(ue.Error(), "caddy local CA is not trusted") {
		t.Errorf("error message missing CA hint: %q", ue.Error())
	}
	if !strings.Contains(ue.Error(), "sudo caddy trust") {
		t.Errorf("error message missing fix hint: %q", ue.Error())
	}
}
