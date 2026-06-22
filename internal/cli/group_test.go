package cli

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/turing/shack/internal/config"
)

// ---- tmuxSessionName -------------------------------------------------------

func TestTmuxSessionName_Simple(t *testing.T) {
	got := tmuxSessionName("fmdplanner")
	want := "shack-fmdplanner"
	if got != want {
		t.Errorf("tmuxSessionName(%q) = %q, want %q", "fmdplanner", got, want)
	}
}

func TestTmuxSessionName_WithDot(t *testing.T) {
	got := tmuxSessionName("foo.bar")
	want := "shack-foo-bar"
	if got != want {
		t.Errorf("tmuxSessionName(%q) = %q, want %q", "foo.bar", got, want)
	}
}

func TestTmuxSessionName_WithColon(t *testing.T) {
	got := tmuxSessionName("foo:bar")
	want := "shack-foo-bar"
	if got != want {
		t.Errorf("tmuxSessionName(%q) = %q, want %q", "foo:bar", got, want)
	}
}

// ---- RunUp — outside project -------------------------------------------

func TestRunUpOutsideProject_AlreadyRunning(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	app, _ := newTestApp(t, r)

	dir := t.TempDir() // no .shack/config.json
	old := chdir(t, dir)
	defer chdir(t, old)

	out := &bytes.Buffer{}
	if err := app.RunUp(out); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	if !strings.Contains(out.String(), "already running") {
		t.Errorf("expected 'already running', got %q", out.String())
	}
}

func TestRunUpOutsideProject_StartsDaemon(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":            {stdout: "Homebrew\n"},
		"brew services list":        {stdout: "caddy none alice\n"},
		"brew services start caddy": {},
	}}
	app, _ := newTestApp(t, r)

	dir := t.TempDir()
	old := chdir(t, dir)
	defer chdir(t, old)

	out := &bytes.Buffer{}
	if err := app.RunUp(out); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	if !strings.Contains(out.String(), "started") {
		t.Errorf("expected 'started', got %q", out.String())
	}
}

// ---- RunUp — inside project, no defaults --------------------------------

func TestRunUpInProject_EmptyDefaults(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
	}}
	app, _ := newTestApp(t, r)

	dir := writeProjectConfig(t, config.Project{
		Name: "myapp",
		// Defaults intentionally empty.
		Commands: []config.Command{
			{Label: "dev", Cmd: "echo hi", Listeners: []config.Listener{{Match: config.Match{Comm: "sh"}, As: "myapp"}}},
		},
	})
	old := chdir(t, dir)
	defer chdir(t, old)

	out := &bytes.Buffer{}
	if err := app.RunUp(out); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	if !strings.Contains(out.String(), "No default commands configured") {
		t.Errorf("expected no-defaults note, got: %q", out.String())
	}
}

// ---- RunUp — tmux missing -----------------------------------------------

func TestRunUpInProject_TmuxMissing(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
		"tmux -V":            {err: errors.New("tmux: command not found")},
	}}
	app, _ := newTestApp(t, r)

	dir := writeProjectConfig(t, config.Project{
		Name:     "myapp",
		Defaults: []string{"dev"},
		Commands: []config.Command{
			{Label: "dev", Cmd: "echo hi", Listeners: []config.Listener{{Match: config.Match{Comm: "sh"}, As: "myapp"}}},
		},
	})
	old := chdir(t, dir)
	defer chdir(t, old)

	err := app.RunUp(&bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error when tmux missing")
	}
	if _, ok := err.(*userError); !ok {
		t.Errorf("expected *userError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "tmux not installed") {
		t.Errorf("expected 'tmux not installed', got %q", err.Error())
	}
}

// ---- RunUp — session already running ------------------------------------

func TestRunUpInProject_SessionAlreadyRunning(t *testing.T) {
	sessionName := "shack-myapp"
	r := &stubRunner{responses: stubResponses{
		"brew --version":                              {stdout: "Homebrew\n"},
		"brew services list":                          {stdout: "caddy started alice\n"},
		"tmux -V":                                     {stdout: "tmux 3.3\n"},
		"tmux has-session -t " + sessionName:          {},
	}}
	app, _ := newTestApp(t, r)

	dir := writeProjectConfig(t, config.Project{
		Name:     "myapp",
		Defaults: []string{"dev"},
		Commands: []config.Command{
			{Label: "dev", Cmd: "echo hi", Listeners: []config.Listener{{Match: config.Match{Comm: "sh"}, As: "myapp"}}},
		},
	})
	old := chdir(t, dir)
	defer chdir(t, old)

	out := &bytes.Buffer{}
	if err := app.RunUp(out); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	if !strings.Contains(out.String(), "session already running") {
		t.Errorf("expected 'session already running', got %q", out.String())
	}
}

// ---- RunUp — creates tmux session with correct commands -----------------

func TestRunUpInProject_CreatesTmuxSession(t *testing.T) {
	sessionName := "shack-fmdplanner"
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
		"tmux -V":            {stdout: "tmux 3.3\n"},
		// has-session fails → no existing session.
		"tmux has-session -t " + sessionName: {err: errors.New("no session")},
		// capture-pane for each window.
		"tmux capture-pane -p -t " + sessionName + ":0 -S -1000": {stdout: "dev:web up\n"},
		"tmux capture-pane -p -t " + sessionName + ":1 -S -1000": {stdout: "dev:server up\n"},
		// list-panes for each window: alive, not crashed.
		"tmux list-panes -t " + sessionName + ":0 -F #{pane_dead} #{pane_dead_status}": {stdout: "0 \n"},
		"tmux list-panes -t " + sessionName + ":1 -F #{pane_dead} #{pane_dead_status}": {stdout: "0 \n"},
		// new-session and subsequent commands succeed (zero-value response).
	}}
	app, _ := newTestApp(t, r)
	app.PollInterval = 5 * time.Millisecond
	app.Probe = func(host string) probeOutcome { return probeServing }

	dir := writeProjectConfig(t, config.Project{
		Name:     "fmdplanner",
		Defaults: []string{"dev:web", "dev:server"},
		Commands: []config.Command{
			{Label: "dev:web", Cmd: "npm run dev", Listeners: []config.Listener{{Match: config.Match{Comm: "node"}, As: "fmdplanner-web"}}},
			{Label: "dev:server", Cmd: "npm run server", Listeners: []config.Listener{{Match: config.Match{Comm: "node"}, As: "fmdplanner-server"}}},
		},
	})
	old := chdir(t, dir)
	defer chdir(t, old)

	// After chdir, Getwd() may return a symlink-resolved path (macOS /private/var…).
	resolvedDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}

	out := &bytes.Buffer{}
	if err := app.RunUp(out); err != nil {
		t.Fatalf("RunUp: %v", err)
	}

	wantNewSession := "tmux new-session -d -s " + sessionName + " -c " + resolvedDir + " -n dev:web shack dev:web"
	if !containsCall(r.calls, wantNewSession) {
		t.Errorf("missing new-session call.\nwant: %s\ngot:\n%s", wantNewSession, formatCalls(r.calls))
	}
	wantKeep0 := "tmux set-option -w -t " + sessionName + ":0 remain-on-exit failed"
	if !containsCall(r.calls, wantKeep0) {
		t.Errorf("missing remain-on-exit for window 0.\nwant: %s\ngot:\n%s", wantKeep0, formatCalls(r.calls))
	}
	wantNewWindow := "tmux new-window -t " + sessionName + " -c " + resolvedDir + " -n dev:server shack dev:server"
	if !containsCall(r.calls, wantNewWindow) {
		t.Errorf("missing new-window call.\nwant: %s\ngot:\n%s", wantNewWindow, formatCalls(r.calls))
	}
	wantKeep1 := "tmux set-option -w -t " + sessionName + ":1 remain-on-exit failed"
	if !containsCall(r.calls, wantKeep1) {
		t.Errorf("missing remain-on-exit for window 1.\nwant: %s\ngot:\n%s", wantKeep1, formatCalls(r.calls))
	}
	for _, c := range r.calls {
		if strings.HasPrefix(c, "tmux send-keys") {
			t.Errorf("send-keys should no longer be used: %s", c)
		}
	}

	// Verify streaming output.
	if !strings.Contains(out.String(), "Starting 2 service") {
		t.Errorf("expected 'Starting 2 service', got:\n%s", out.String())
	}
	// Both services serve immediately under the stub probe.
	if !strings.Contains(out.String(), "✓  https://") {
		t.Errorf("expected '✓  https://' in output, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "shack attach") {
		t.Errorf("expected 'shack attach' in output, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "shack down") {
		t.Errorf("expected 'shack down' in output, got:\n%s", out.String())
	}
}

// ---- RunAttach — outside project ------------------------------------------

func TestRunAttachOutsideProject(t *testing.T) {
	r := &stubRunner{responses: stubResponses{}}
	app, _ := newTestApp(t, r)

	dir := t.TempDir() // no .shack/config.json
	old := chdir(t, dir)
	defer chdir(t, old)

	err := app.RunAttach(&bytes.Buffer{}, &fakeAttacher{}, "")
	if err == nil {
		t.Fatal("expected userError")
	}
	if _, ok := err.(*userError); !ok {
		t.Errorf("expected *userError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "not in a shack project") {
		t.Errorf("expected 'not in a shack project', got %q", err.Error())
	}
}

// ---- RunAttach — in project, no session ------------------------------------

func TestRunAttachInProject_NoSession(t *testing.T) {
	sessionName := "shack-myapp"
	r := &stubRunner{responses: stubResponses{
		"tmux has-session -t " + sessionName: {err: errors.New("no session")},
	}}
	app, _ := newTestApp(t, r)

	dir := writeProjectConfig(t, config.Project{Name: "myapp"})
	old := chdir(t, dir)
	defer chdir(t, old)

	err := app.RunAttach(&bytes.Buffer{}, &fakeAttacher{}, "")
	if err == nil {
		t.Fatal("expected userError")
	}
	if _, ok := err.(*userError); !ok {
		t.Errorf("expected *userError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "no running session") {
		t.Errorf("expected 'no running session', got %q", err.Error())
	}
}

// ---- RunAttach — in project, session exists --------------------------------

func TestRunAttachInProject_Attaches(t *testing.T) {
	sessionName := "shack-myapp"
	r := &stubRunner{responses: stubResponses{
		"tmux has-session -t " + sessionName: {}, // session exists
	}}
	app, _ := newTestApp(t, r)

	dir := writeProjectConfig(t, config.Project{Name: "myapp"})
	old := chdir(t, dir)
	defer chdir(t, old)

	fa := &fakeAttacher{}
	if err := app.RunAttach(&bytes.Buffer{}, fa, ""); err != nil {
		t.Fatalf("RunAttach: %v", err)
	}
	if fa.lastSession != sessionName {
		t.Errorf("Attach called with %q, want %q", fa.lastSession, sessionName)
	}
}

// ---- RunAttach — select service window ------------------------------------

func TestRunAttachSelectsServiceWindow(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"tmux has-session -t shack-demo": {}, // exists
	}}
	app, _ := newTestApp(t, r)
	dir := writeProjectConfig(t, config.Project{
		Name: "demo", Defaults: []string{"dev:api", "dev:ui"},
		Commands: []config.Command{
			{Label: "dev:api", Listeners: []config.Listener{{Match: config.Match{Script: "dev:api"}, As: "demo-api"}}},
			{Label: "dev:ui", Listeners: []config.Listener{{Match: config.Match{Script: "dev:ui"}, As: "demo"}}},
		},
	})
	old := chdir(t, dir)
	defer chdir(t, old)

	f := &fakeAttacher{}
	if err := app.RunAttach(&bytes.Buffer{}, f, "dev:ui"); err != nil {
		t.Fatalf("RunAttach: %v", err)
	}
	if f.lastSession != "shack-demo" || f.gotWindow != 1 {
		t.Errorf("got session=%q window=%d, want shack-demo/1", f.lastSession, f.gotWindow)
	}
}

func TestRunAttachNoLabelWholeSession(t *testing.T) {
	r := &stubRunner{responses: stubResponses{"tmux has-session -t shack-demo": {}}}
	app, _ := newTestApp(t, r)
	dir := writeProjectConfig(t, config.Project{
		Name: "demo", Defaults: []string{"dev:api"},
		Commands: []config.Command{{Label: "dev:api", Listeners: []config.Listener{{Match: config.Match{Script: "dev:api"}, As: "demo-api"}}}},
	})
	old := chdir(t, dir)
	defer chdir(t, old)

	f := &fakeAttacher{}
	if err := app.RunAttach(&bytes.Buffer{}, f, ""); err != nil {
		t.Fatalf("RunAttach: %v", err)
	}
	if f.gotWindow != -1 {
		t.Errorf("no label should pass window=-1, got %d", f.gotWindow)
	}
}

func TestRunAttachUnknownLabel(t *testing.T) {
	r := &stubRunner{responses: stubResponses{"tmux has-session -t shack-demo": {}}}
	app, _ := newTestApp(t, r)
	dir := writeProjectConfig(t, config.Project{
		Name: "demo", Defaults: []string{"dev:api"},
		Commands: []config.Command{{Label: "dev:api", Listeners: []config.Listener{{Match: config.Match{Script: "dev:api"}, As: "demo-api"}}}},
	})
	old := chdir(t, dir)
	defer chdir(t, old)

	err := app.RunAttach(&bytes.Buffer{}, &fakeAttacher{}, "nope")
	if err == nil {
		t.Fatal("expected error for unknown service label")
	}
	if !strings.Contains(err.Error(), "dev:api") {
		t.Errorf("error should list valid defaults, got: %v", err)
	}
}

// ---- RunDown — outside project --------------------------------------------

func TestRunDownOutsideProject(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":           {stdout: "Homebrew\n"},
		"brew services stop caddy": {},
	}}
	app, _ := newTestApp(t, r)

	dir := t.TempDir()
	old := chdir(t, dir)
	defer chdir(t, old)

	out := &bytes.Buffer{}
	if err := app.RunDown(out, false); err != nil {
		t.Fatalf("RunDown: %v", err)
	}
	if !strings.Contains(out.String(), "stopped") {
		t.Errorf("expected 'stopped', got %q", out.String())
	}
}

// ---- RunDown — in project, session exists ----------------------------------

func TestRunDownInProject_KillsSession(t *testing.T) {
	sessionName := "shack-myapp"
	r := &stubRunner{responses: stubResponses{
		"tmux has-session -t " + sessionName:  {},
		"tmux kill-session -t " + sessionName: {},
	}}
	app, _ := newTestApp(t, r)

	dir := writeProjectConfig(t, config.Project{Name: "myapp"})
	old := chdir(t, dir)
	defer chdir(t, old)

	out := &bytes.Buffer{}
	if err := app.RunDown(out, false); err != nil {
		t.Fatalf("RunDown: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Stopped session") {
		t.Errorf("expected 'Stopped session', got %q", got)
	}
	if !strings.Contains(got, sessionName) {
		t.Errorf("expected session name in output, got %q", got)
	}
	// No daemon-stop message when all=false.
	if strings.Contains(got, "caddy: stopped") {
		t.Errorf("unexpected daemon stop message with all=false: %q", got)
	}
}

// ---- RunDown — in project, all=true: kills session AND stops daemon ---------

func TestRunDownInProject_KillsSessionAndDaemon(t *testing.T) {
	sessionName := "shack-myapp"
	r := &stubRunner{responses: stubResponses{
		"brew --version":                                      {stdout: "Homebrew\n"},
		"brew services stop caddy":                            {},
		"tmux list-sessions -F #{session_name}":               {stdout: sessionName + "\n"},
		"tmux has-session -t " + sessionName:                  {},
		"tmux kill-session -t " + sessionName:                 {},
	}}
	app, _ := newTestApp(t, r)

	dir := writeProjectConfig(t, config.Project{Name: "myapp"})
	old := chdir(t, dir)
	defer chdir(t, old)

	out := &bytes.Buffer{}
	if err := app.RunDown(out, true); err != nil {
		t.Fatalf("RunDown(all=true): %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Stopped session "+sessionName) {
		t.Errorf("expected 'Stopped session %s', got %q", sessionName, got)
	}
	if !strings.Contains(got, "stopped") {
		t.Errorf("expected daemon stopped message, got %q", got)
	}
}

// ---- RunDown — in project, no session --------------------------------------

func TestRunDownInProject_NoSession(t *testing.T) {
	sessionName := "shack-myapp"
	r := &stubRunner{responses: stubResponses{
		"tmux has-session -t " + sessionName: {err: errors.New("no session")},
	}}
	app, _ := newTestApp(t, r)

	dir := writeProjectConfig(t, config.Project{Name: "myapp"})
	old := chdir(t, dir)
	defer chdir(t, old)

	out := &bytes.Buffer{}
	if err := app.RunDown(out, false); err != nil {
		t.Fatalf("RunDown: %v", err)
	}
	if !strings.Contains(out.String(), "no running session") {
		t.Errorf("expected 'no running session', got %q", out.String())
	}
}

// ---- RunUp all — behaves identically to no-arg --------------------------

func TestRunUpAll_OutsideProject(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":            {stdout: "Homebrew\n"},
		"brew services list":        {stdout: "caddy none alice\n"},
		"brew services start caddy": {},
	}}
	app, _ := newTestApp(t, r)

	dir := t.TempDir()
	old := chdir(t, dir)
	defer chdir(t, old)

	// Simulate cobra parsing: arg "all" is accepted and consumed; RunUp
	// behavior is identical to the no-arg path.
	out := &bytes.Buffer{}
	if err := app.RunUp(out); err != nil {
		t.Fatalf("RunUp (all): %v", err)
	}
	if !strings.Contains(out.String(), "started") {
		t.Errorf("expected 'started', got %q", out.String())
	}
}

// ---- RunUp — reports crash --------------------------------------------------

func TestRunUpReportsCrash(t *testing.T) {
	sessionName := "shack-demo"
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
		"tmux -V":            {stdout: "tmux 3.3\n"},
		// has-session fails → no existing session, so RunUp creates one.
		"tmux has-session -t " + sessionName: {err: errors.New("no session")},
		// pane 0 crashed, exit 1.
		"tmux list-panes -t " + sessionName + ":0 -F #{pane_dead} #{pane_dead_status}": {stdout: "1 1\n"},
		// stream output (history capture) and crash tail capture.
		"tmux capture-pane -p -t " + sessionName + ":0 -S -1000": {stdout: "Error: listen EADDRINUSE :51372\n"},
		"tmux capture-pane -p -t " + sessionName + ":0 -S -60":   {stdout: "Error: listen EADDRINUSE :51372\n"},
	}}
	app, _ := newTestApp(t, r)
	app.PollInterval = 5 * time.Millisecond
	app.Probe = func(host string) probeOutcome { return probeUnreachable }

	dir := writeProjectConfig(t, config.Project{
		Name:     "demo",
		Defaults: []string{"dev:api"},
		Commands: []config.Command{{
			Label:     "dev:api",
			Cmd:       "pnpm dev:api",
			Listeners: []config.Listener{{Match: config.Match{Script: "dev:api"}, As: "demo-api"}},
		}},
	})
	old := chdir(t, dir)
	defer chdir(t, old)

	out := &bytes.Buffer{}
	if err := app.RunUp(out); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	if !strings.Contains(out.String(), "crashed (exit 1)") ||
		!strings.Contains(out.String(), "EADDRINUSE") {
		t.Errorf("expected crash report, got:\n%s", out.String())
	}
}

// ---- RunUp — reports serving ------------------------------------------------

func TestRunUpReportsServing(t *testing.T) {
	sessionName := "shack-demo"
	r := &stubRunner{responses: stubResponses{
		"brew --version":     {stdout: "Homebrew\n"},
		"brew services list": {stdout: "caddy started alice\n"},
		"tmux -V":            {stdout: "tmux 3.3\n"},
		"tmux has-session -t " + sessionName: {err: errors.New("no session")},
		"tmux capture-pane -p -t " + sessionName + ":0 -S -1000": {stdout: "dev:api up\n"},
		"tmux list-panes -t " + sessionName + ":0 -F #{pane_dead} #{pane_dead_status}": {stdout: "0 \n"},
	}}
	app, _ := newTestApp(t, r)
	app.PollInterval = 5 * time.Millisecond
	app.Probe = func(host string) probeOutcome { return probeServing }

	dir := writeProjectConfig(t, config.Project{
		Name:     "demo",
		Defaults: []string{"dev:api"},
		Commands: []config.Command{{
			Label:     "dev:api",
			Cmd:       "pnpm dev:api",
			Listeners: []config.Listener{{Match: config.Match{Script: "dev:api"}, As: "demo-api"}},
		}},
	})
	old := chdir(t, dir)
	defer chdir(t, old)

	out := &bytes.Buffer{}
	if err := app.RunUp(out); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	if !strings.Contains(out.String(), "✓  https://demo-api.localhost") {
		t.Errorf("expected served line, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "attach: shack attach dev:api") {
		t.Errorf("expected per-service attach line, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Detach:  Ctrl-b then d") {
		t.Errorf("expected detach reminder, got:\n%s", out.String())
	}
}

// ---- helpers ---------------------------------------------------------------

// fakeAttacher records the last Attach call.
type fakeAttacher struct {
	lastSession string
	gotWindow   int
	err         error
}

func (f *fakeAttacher) Attach(sessionName string, windowIndex int) error {
	f.lastSession = sessionName
	f.gotWindow = windowIndex
	return f.err
}

// writeProjectConfig writes a .shack/config.json in a fresh temp dir
// and returns that dir.
func writeProjectConfig(t *testing.T, proj config.Project) string {
	t.Helper()
	dir := t.TempDir()
	if err := config.Write(dir, proj); err != nil {
		t.Fatalf("config.Write: %v", err)
	}
	return dir
}

// chdir changes the working directory and returns the old one.
func chdir(t *testing.T, dir string) string {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%s): %v", dir, err)
	}
	return old
}

// containsCall reports whether the exact call string appears in calls.
func containsCall(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
}

// formatCalls returns calls as a newline-separated list for diagnostics.
func formatCalls(calls []string) string {
	return strings.Join(calls, "\n")
}

// ---- RunDown all — kills ALL shack-* sessions, regardless of cwd -------

func TestRunDownAll_KillsAllShackSessions(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":                              {stdout: "Homebrew\n"},
		"brew services stop caddy":                    {},
		"tmux list-sessions -F #{session_name}":       {stdout: "shack-fmdplanner\nshack-mailcave\nother-session\n"},
		"tmux kill-session -t shack-fmdplanner": {},
		"tmux kill-session -t shack-mailcave":   {},
	}}
	app, _ := newTestApp(t, r)

	// Run from a directory with no project config — should still kill both.
	tmp := t.TempDir()
	old := chdir(t, tmp)
	defer chdir(t, old)

	out := &bytes.Buffer{}
	if err := app.RunDown(out, true); err != nil {
		t.Fatalf("RunDown(all=true): %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Stopped session shack-fmdplanner") {
		t.Errorf("expected fmdplanner stopped, got %q", got)
	}
	if !strings.Contains(got, "Stopped session shack-mailcave") {
		t.Errorf("expected mailcave stopped, got %q", got)
	}
	if strings.Contains(got, "Stopped session other-session") {
		t.Errorf("should NOT have killed non-shack session, got %q", got)
	}
	if !strings.Contains(got, "caddy: stopped") {
		t.Errorf("expected daemon stopped, got %q", got)
	}
}

func TestRunDownAll_NoSessionsRunning(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"brew --version":                        {stdout: "Homebrew\n"},
		"brew services stop caddy":              {},
		"tmux list-sessions -F #{session_name}": {err: &exitErr{code: 1}}, // tmux exits 1 when no server running
	}}
	app, _ := newTestApp(t, r)

	tmp := t.TempDir()
	old := chdir(t, tmp)
	defer chdir(t, old)

	out := &bytes.Buffer{}
	if err := app.RunDown(out, true); err != nil {
		t.Fatalf("RunDown(all=true): %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "no running shack sessions") {
		t.Errorf("expected no-sessions message, got %q", got)
	}
	if !strings.Contains(got, "caddy: stopped") {
		t.Errorf("expected daemon stopped, got %q", got)
	}
}
