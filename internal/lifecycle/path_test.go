package lifecycle

import "testing"

type fakeRunner struct {
	responses map[string]response
	calls     []string
}

type response struct {
	stdout string
	stderr string
	err    error
}

func (f *fakeRunner) Run(cmd string, args ...string) (string, string, error) {
	key := cmd
	for _, a := range args {
		key += " " + a
	}
	f.calls = append(f.calls, key)
	r, ok := f.responses[key]
	if !ok {
		return "", "", nil
	}
	return r.stdout, r.stderr, r.err
}

func TestResolverBrewPrefix(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew --prefix": {stdout: "/opt/homebrew\n"},
	}}
	res := NewResolver(r)
	got, err := res.BrewPrefix()
	if err != nil {
		t.Fatalf("BrewPrefix: %v", err)
	}
	if got != "/opt/homebrew" {
		t.Errorf("got %q, want /opt/homebrew", got)
	}
}

func TestResolverCachesBrewPrefix(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew --prefix": {stdout: "/opt/homebrew\n"},
	}}
	res := NewResolver(r)
	_, _ = res.BrewPrefix()
	_, _ = res.BrewPrefix()
	count := 0
	for _, c := range r.calls {
		if c == "brew --prefix" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("brew --prefix invoked %d times, want 1 (cached)", count)
	}
}

func TestResolverCaddyfilePath(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew --prefix": {stdout: "/opt/homebrew\n"},
	}}
	res := NewResolver(r)
	got, err := res.CaddyfilePath()
	if err != nil {
		t.Fatalf("CaddyfilePath: %v", err)
	}
	if got != "/opt/homebrew/etc/Caddyfile" {
		t.Errorf("got %q", got)
	}
}

func TestResolverCaddyBinary(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew --prefix": {stdout: "/opt/homebrew\n"},
	}}
	res := NewResolver(r)
	got, err := res.CaddyBinary()
	if err != nil {
		t.Fatalf("CaddyBinary: %v", err)
	}
	if got != "/opt/homebrew/bin/caddy" {
		t.Errorf("got %q", got)
	}
}

func TestResolverRejectsEmptyPrefix(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew --prefix": {stdout: "\n"},
	}}
	res := NewResolver(r)
	_, err := res.BrewPrefix()
	if err == nil {
		t.Fatal("expected error on empty brew --prefix output")
	}
}
