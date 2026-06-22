package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/turing/shack/internal/config"
)

// noListenerProctree is a proctree.Runner that always reports zero
// children/listeners — used in tests where no listener match is expected.
type noListenerProctree struct{}

func (*noListenerProctree) Run(cmd string, args ...string) (string, string, error) {
	return "", "", &exitErr{1}
}

// funcRunner is a proctree.Runner whose behaviour is determined by a
// user-supplied function. Useful when the test needs dynamic responses (e.g.
// when the child PID is not known ahead of time).
type funcRunner struct {
	fn func(cmd string, args ...string) (string, string, error)
}

func (f *funcRunner) Run(cmd string, args ...string) (string, string, error) {
	return f.fn(cmd, args...)
}

func TestRunCommandMissingLabel(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	app, _ := newTestApp(t, r)

	root := t.TempDir()
	_ = config.Write(root, config.Project{Name: "demo"})

	_, err := app.RunCommand(context.Background(), root, "missing",
		os.Stdout, os.Stderr, &noListenerProctree{})
	if err == nil {
		t.Fatal("expected error for missing label")
	}
	if _, ok := err.(*userError); !ok {
		t.Errorf("expected *userError, got %T: %v", err, err)
	}
}

// TestRunCommandUnmatchedRegisterFailureSingleMessage verifies that when an
// unmatched listener's RegisterHostname call fails, only ONE post-exit
// message is emitted — the honest "could not register as" form — and NOT
// the contradictory "registered as" message that previously leaked from the
// unmatched-snap loop.
func TestRunCommandUnmatchedRegisterFailureSingleMessage(t *testing.T) {
	// listenerDelivered counts how many times the lsof response includes the
	// fake listener. We return it exactly once so the observer fires cb once.
	var listenerDelivered atomic.Int32

	// reloadKey is built once we know the temp dir from newTestApp. We use a
	// pointer-to-string so the funcRunner closure can read it after it's set.
	var reloadKey string

	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	app, caddyfilePath := newTestApp(t, r)
	prefix, _ := app.Resolver.BrewPrefix()
	reloadKey = fmt.Sprintf("%s/bin/caddy reload --config %s --adapter caddyfile", prefix, caddyfilePath)

	// procRunner: responds to pgrep/lsof/ps generically without needing the
	// real child PID. Descendants() includes the root PID regardless of what
	// pgrep returns; Listeners() gets called with whatever pids are found.
	procRunner := &funcRunner{fn: func(cmd string, args ...string) (string, string, error) {
		switch cmd {
		case "pgrep":
			// No children — Descendants returns just the root PID.
			return "", "", &exitErr{1}
		case "lsof":
			// Deliver a fake listener on port 3001 exactly once.
			if listenerDelivered.Add(1) == 1 {
				// lsof header line + one LISTEN row. The PID in the row must
				// be in the pids arg so Listeners() includes it; we embed the
				// root PID by extracting it from args ("-p <pids>").
				// Simplest: use pid 0 — Listeners() parses whatever pid is in
				// the row, so we use a constant that won't collide.
				return "COMMAND PID USER FD TYPE DEVICE SIZE NODE NAME\nsh 1 alice 5u IPv4 0x0 0t0 TCP *:3001 (LISTEN)\n", "", nil
			}
			return "", "", &exitErr{1}
		case "ps":
			// No npm env vars → Identifiers.Comm = "sh", Script = "".
			return "sh", "", nil
		default:
			return "", "", errors.New("unexpected command: " + cmd)
		}
	}}

	// Make caddy reload fail so RegisterHostname returns an error.
	r.responses[reloadKey] = struct {
		stdout string
		stderr string
		err    error
	}{err: errors.New("caddy reload: synthetic failure")}

	root := t.TempDir()
	if err := config.Write(root, config.Project{
		Name: "demo",
		Commands: []config.Command{{
			Label: "svc",
			// Child exits quickly; we need it alive long enough for the
			// observer to fire at least once.
			Cmd: "sh -c 'sleep 0.4; exit 0'",
			Listeners: []config.Listener{
				// Match by script "dev" — our fake listener has Comm="sh" and
				// no script, so it will NOT match and will be treated as
				// unmatched.
				{Match: config.Match{Script: "dev"}, As: "demo-api"},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	stderr := &bytes.Buffer{}
	_, err := app.RunCommand(context.Background(), root, "svc",
		os.Stdout, stderr, procRunner)
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}

	got := stderr.String()

	// Exactly one message should mention port 3001.
	occurrences := strings.Count(got, ":3001")
	if occurrences != 1 {
		t.Errorf("expected exactly 1 message mentioning :3001, got %d; full stderr:\n%s", occurrences, got)
	}

	// The message must be the honest failure form, not the false "registered as" form.
	if strings.Contains(got, "registered as") {
		t.Errorf("stderr must not claim 'registered as' when registration failed; got:\n%s", got)
	}
	if !strings.Contains(got, "could not register as") {
		t.Errorf("expected 'could not register as' in stderr; got:\n%s", got)
	}
}

// TestRunCommandPreHookSuccess verifies that when a command has a pre-hook
// that exits 0, the pre-hook output is emitted and the main command runs.
func TestRunCommandPreHookSuccess(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	app, _ := newTestApp(t, r)

	root := t.TempDir()
	if err := config.Write(root, config.Project{
		Name: "demo",
		Commands: []config.Command{{
			Label: "build",
			Cmd:   "sh -c 'sleep 0.1; exit 0'",
			Pre:   "true", // exits 0 immediately
			Listeners: []config.Listener{
				{Match: config.Match{Script: "build"}, As: "demo-build"},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	stdout := &bytes.Buffer{}
	exit, err := app.RunCommand(context.Background(), root, "build",
		stdout, os.Stderr, &noListenerProctree{})
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout.String(), "running pre-hook: true") {
		t.Errorf("expected pre-hook message in stdout, got: %q", stdout.String())
	}
}

// TestRunCommandPreHookFailure verifies that when a pre-hook exits non-zero,
// RunCommand returns an error and the main command is NOT started.
func TestRunCommandPreHookFailure(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	app, _ := newTestApp(t, r)

	root := t.TempDir()
	// sentinel file: if the main command runs it touches this file
	sentinel := root + "/main-ran"
	if err := config.Write(root, config.Project{
		Name: "demo",
		Commands: []config.Command{{
			Label: "start",
			Cmd:   "sh -c 'touch " + sentinel + "; exit 0'",
			Pre:   "false", // exits 1 immediately
			Listeners: []config.Listener{
				{Match: config.Match{Script: "start"}, As: "demo"},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	_, err := app.RunCommand(context.Background(), root, "start",
		os.Stdout, os.Stderr, &noListenerProctree{})
	if err == nil {
		t.Fatal("expected error when pre-hook exits non-zero, got nil")
	}
	if !strings.Contains(err.Error(), "pre-hook failed") {
		t.Errorf("expected 'pre-hook failed' in error, got: %v", err)
	}
	// main command must NOT have run
	if _, statErr := os.Stat(sentinel); statErr == nil {
		t.Error("main command ran despite pre-hook failure")
	}
}

// TestRunCommandHappyPath verifies the sunny-day flow end-to-end:
// - RunCommand spawns the child, observes a matching listener, calls
//   RegisterHostname (Caddyfile gains the entry), and calls
//   UnregisterHostnames on exit (Caddyfile loses the entry).
// - Exit code 0 is returned with no error.
//
// The test uses a funcRunner that synthesises a single listener event on
// port 3000 with comm "sh" so MatchListener hits the comm-based rule,
// avoiding the need for npm env vars.
func TestRunCommandHappyPath(t *testing.T) {
	var listenerDelivered atomic.Int32

	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	app, caddyfilePath := newTestApp(t, r)
	prefix, _ := app.Resolver.BrewPrefix()
	reloadKey := fmt.Sprintf("%s/bin/caddy reload --config %s --adapter caddyfile", prefix, caddyfilePath)
	r.responses[reloadKey] = struct {
		stdout string
		stderr string
		err    error
	}{}

	procRunner := &funcRunner{fn: func(cmd string, args ...string) (string, string, error) {
		switch cmd {
		case "pgrep":
			return "", "", &exitErr{1}
		case "lsof":
			if listenerDelivered.Add(1) == 1 {
				return "COMMAND PID USER FD TYPE DEVICE SIZE NODE NAME\nsh 1 alice 5u IPv4 0x0 0t0 TCP *:3000 (LISTEN)\n", "", nil
			}
			return "", "", &exitErr{1}
		case "ps":
			// No npm lifecycle env — comm match only.
			return "sh", "", nil
		default:
			return "", "", errors.New("unexpected: " + cmd)
		}
	}}

	root := t.TempDir()
	if err := config.Write(root, config.Project{
		Name: "myapp",
		Commands: []config.Command{{
			Label: "dev",
			Cmd:   "sh -c 'sleep 0.4; exit 0'",
			Listeners: []config.Listener{
				{Match: config.Match{Comm: "sh"}, As: "myapp-dev"},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	stderr := &bytes.Buffer{}
	exit, err := app.RunCommand(context.Background(), root, "dev",
		os.Stdout, stderr, procRunner)
	if err != nil {
		t.Fatalf("RunCommand returned error: %v", err)
	}
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
	if stderr.Len() > 0 {
		t.Errorf("unexpected stderr output: %q", stderr.String())
	}

	// RegisterHostname should have written the entry to the Caddyfile.
	// UnregisterHostnames should have removed it again on exit.
	// Net result: Caddyfile must NOT contain myapp-dev after the child exits.
	caddyfileBytes, err := os.ReadFile(caddyfilePath)
	if err != nil {
		t.Fatalf("read Caddyfile: %v", err)
	}
	if bytes.Contains(caddyfileBytes, []byte("myapp-dev")) {
		t.Errorf("Caddyfile still contains myapp-dev after RunCommand returned (unregister did not fire):\n%s", caddyfileBytes)
	}
}


func TestRunCommandNeverRewritesConfig(t *testing.T) {
	root := t.TempDir()
	proj := config.Project{
		Name:     "demo",
		Defaults: []string{"ghost"},
		Commands: []config.Command{{
			Label:     "ghost",
			Cmd:       "true", // exits 0 immediately, never opens a listener
			Listeners: []config.Listener{{Match: config.Match{Script: "ghost"}, As: "ghost"}},
		}},
	}
	if err := config.Write(root, proj); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(root + "/.shack/config.json")
	if err != nil {
		t.Fatal(err)
	}

	r := &stubRunner{responses: stubResponses{
		"brew --version": {stdout: "Homebrew\n"},
		"brew --prefix":  {stdout: "/opt/homebrew\n"},
	}}
	app, _ := newTestApp(t, r)

	if _, err := app.RunCommand(context.Background(), root, "ghost",
		os.Stdout, &bytes.Buffer{}, &noListenerProctree{}); err != nil {
		t.Fatalf("RunCommand: %v", err)
	}

	after, err := os.ReadFile(root + "/.shack/config.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("config.json was rewritten:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

