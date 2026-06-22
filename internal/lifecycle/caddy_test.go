package lifecycle

import (
	"errors"
	"strings"
	"testing"
)

func TestIsRunningTrue(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew services list": {stdout: "Name   Status   User\ncaddy  started  alice\n"},
	}}
	c := NewCaddy(r, NewResolver(r))
	got, err := c.IsRunning()
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected IsRunning=true")
	}
}

func TestIsRunningFalseWhenStopped(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew services list": {stdout: "Name   Status  User\ncaddy  none    alice\n"},
	}}
	c := NewCaddy(r, NewResolver(r))
	got, err := c.IsRunning()
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("expected IsRunning=false")
	}
}

func TestIsRunningFalseWhenAbsent(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew services list": {stdout: "Name  Status  User\n"},
	}}
	c := NewCaddy(r, NewResolver(r))
	got, err := c.IsRunning()
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("expected IsRunning=false when caddy not in list")
	}
}

func TestStartCallsBrewServicesStart(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew services list":      {stdout: "Name  Status  User\ncaddy none alice\n"},
		"brew services start caddy": {},
	}}
	c := NewCaddy(r, NewResolver(r))
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, call := range r.calls {
		if call == "brew services start caddy" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected brew services start caddy, got calls: %v", r.calls)
	}
}

func TestStopCallsBrewServicesStop(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew services stop caddy": {},
	}}
	c := NewCaddy(r, NewResolver(r))
	if err := c.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestStartPropagatesError(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew services list":        {stdout: "caddy none alice\n"},
		"brew services start caddy": {stderr: "permission denied", err: errors.New("exit 1")},
	}}
	c := NewCaddy(r, NewResolver(r))
	if err := c.Start(); err == nil {
		t.Error("expected error from Start")
	}
}

func TestReloadInvokesCaddyBinaryWithAdapter(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew --prefix": {stdout: "/opt/homebrew\n"},
		"/opt/homebrew/bin/caddy reload --config /opt/homebrew/etc/Caddyfile --adapter caddyfile": {},
	}}
	c := NewCaddy(r, NewResolver(r))
	if err := c.Reload(); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, call := range r.calls {
		if call == "/opt/homebrew/bin/caddy reload --config /opt/homebrew/etc/Caddyfile --adapter caddyfile" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected caddy reload invocation with --adapter caddyfile, got: %v", r.calls)
	}
}

func TestReloadForwardsStderrOnError(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew --prefix": {stdout: "/opt/homebrew\n"},
		"/opt/homebrew/bin/caddy reload --config /opt/homebrew/etc/Caddyfile --adapter caddyfile": {
			stderr: "adapt config error: line 12 unexpected directive",
			err:    errors.New("exit 1"),
		},
	}}
	c := NewCaddy(r, NewResolver(r))
	err := c.Reload()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "line 12") {
		t.Errorf("expected stderr in error, got: %v", err)
	}
}

// When stderr is empty but err is non-nil, we still need a meaningful
// message — fall back to wrapping the underlying error.
func TestReloadFallsBackToUnderlyingErrWhenStderrEmpty(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew --prefix": {stdout: "/opt/homebrew\n"},
		"/opt/homebrew/bin/caddy reload --config /opt/homebrew/etc/Caddyfile --adapter caddyfile": {
			stderr: "",
			err:    errors.New("exec: caddy: not found"),
		},
	}}
	c := NewCaddy(r, NewResolver(r))
	err := c.Reload()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected underlying err in message, got: %v", err)
	}
}

func TestEnsureRunningSkipsWhenRunning(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	c := NewCaddy(r, NewResolver(r))
	if err := c.EnsureRunning(); err != nil {
		t.Fatal(err)
	}
	for _, call := range r.calls {
		if call == "brew services start caddy" {
			t.Error("EnsureRunning should not call start when already running")
		}
	}
}

func TestEnsureRunningStartsWhenStopped(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew services list":        {stdout: "caddy none alice\n"},
		"brew services start caddy": {},
	}}
	c := NewCaddy(r, NewResolver(r))
	if err := c.EnsureRunning(); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, call := range r.calls {
		if call == "brew services start caddy" {
			found = true
		}
	}
	if !found {
		t.Error("EnsureRunning should have called start")
	}
}

func TestIsCATrustedTrueWhenSecurityReturnsCert(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"security find-certificate -a -c Caddy Local Authority /Library/Keychains/System.keychain": {
			stdout: "keychain: \"/Library/Keychains/System.keychain\"\nlabl<blob>=\"Caddy Local Authority\"\n",
		},
	}}
	c := NewCaddy(r, NewResolver(r))
	if !c.IsCATrusted() {
		t.Error("expected IsCATrusted=true when security finds the cert")
	}
}

func TestIsCATrustedFalseWhenStdoutEmpty(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"security find-certificate -a -c Caddy Local Authority /Library/Keychains/System.keychain": {
			stdout: "",
		},
	}}
	c := NewCaddy(r, NewResolver(r))
	if c.IsCATrusted() {
		t.Error("expected IsCATrusted=false when security returns empty stdout")
	}
}

func TestIsCATrustedFalseOnError(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"security find-certificate -a -c Caddy Local Authority /Library/Keychains/System.keychain": {
			err: errors.New("security: not found"),
		},
	}}
	c := NewCaddy(r, NewResolver(r))
	if c.IsCATrusted() {
		t.Error("expected IsCATrusted=false when security errors")
	}
}

func TestStartRecoversFromBootstrapAlreadyLoaded(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew services list":          {stdout: "caddy none alice\n"},
		"brew services start caddy":   {stderr: "Bootstrap failed: 5: Input/output error\n", err: errors.New("exit status 1")},
		"brew services restart caddy": {},
	}}
	c := NewCaddy(r, NewResolver(r))
	if err := c.Start(); err != nil {
		t.Fatalf("expected Start to recover via restart, got: %v", err)
	}
	saw := map[string]bool{}
	for _, call := range r.calls {
		saw[call] = true
	}
	if !saw["brew services start caddy"] {
		t.Errorf("expected start to be attempted")
	}
	if !saw["brew services restart caddy"] {
		t.Errorf("expected restart fallback to be invoked")
	}
}

func TestStartRecoveryFailureReportsBoth(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew services list":          {stdout: "caddy none alice\n"},
		"brew services start caddy":   {stderr: "Bootstrap failed: 5\n", err: errors.New("exit 1")},
		"brew services restart caddy": {stderr: "still broken\n", err: errors.New("exit 1")},
	}}
	c := NewCaddy(r, NewResolver(r))
	err := c.Start()
	if err == nil {
		t.Fatal("expected error when both start and restart fail")
	}
	if !strings.Contains(err.Error(), "Bootstrap failed") {
		t.Errorf("expected original bootstrap stderr in message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "recovery via `brew services restart caddy` also failed") {
		t.Errorf("expected recovery-failure annotation, got: %v", err)
	}
}

func TestStartUnrelatedErrorIsNotRecovered(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"brew services list":        {stdout: "caddy none alice\n"},
		"brew services start caddy": {stderr: "permission denied\n", err: errors.New("exit 1")},
	}}
	c := NewCaddy(r, NewResolver(r))
	if err := c.Start(); err == nil {
		t.Fatal("expected error to propagate")
	}
	for _, call := range r.calls {
		if call == "brew services restart caddy" {
			t.Errorf("restart should NOT be attempted for unrelated errors; got calls: %v", r.calls)
		}
	}
}
