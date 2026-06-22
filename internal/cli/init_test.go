// init_test.go shares helpers with the rest of the cli package:
//   - noListenerProctree → runner_test.go (Task 13)
//   - proctreeListener / proctreeIdentifiers → proctree_test_helpers_test.go (Task 13)
//   - exitErr → exiterr_test.go (Task 11)
//
// fakeRunCmd / noopRunCmd are declared below; they are init-only.
package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/turing/shack/internal/config"
	"github.com/turing/shack/internal/plugins"
)

// fakeRunCmd / noopRunCmd are init-test-only because the runCmd interface
// signature is local to this package's init flow.
type fakeRunCmd struct{ results map[string][]observedListener }

func (f *fakeRunCmd) testRun(_, cmd string, _, _ io.Writer) []observedListener {
	return f.results[cmd]
}

type noopRunCmd struct{}

func (*noopRunCmd) testRun(_, _ string, _, _ io.Writer) []observedListener { return nil }

// stubCaTrustResponses returns the runner stubs needed for Preflight to
// pass (brew, caddy binary, CA trusted).
func stubCaTrustResponses() stubResponses {
	return stubResponses{
		"brew --version": {stdout: "Homebrew\n"},
		"brew --prefix":  {stdout: "/opt/homebrew\n"},
		"security find-certificate -a -c Caddy Local Authority /Library/Keychains/System.keychain": {stdout: "keychain: cert\n"},
	}
}

// ---- Tests for pure helpers ----

// TestDefaultProjectNameFallback verifies that when sanitize produces an
// empty string (e.g., project directory is /tmp/!!!), defaultProjectName
// returns the hardcoded fallback "project".
func TestDefaultProjectNameFallback(t *testing.T) {
	// Create a temp directory with a name that sanitizes to empty.
	dir := t.TempDir()
	unsanitizable := filepath.Join(dir, "!!!")
	if err := os.Mkdir(unsanitizable, 0755); err != nil {
		t.Fatal(err)
	}
	got := defaultProjectName(unsanitizable)
	if got != "project" {
		t.Errorf("got=%q, want project (fallback)", got)
	}
}

// TestRenameSuggestion verifies the app- prefix rule.
func TestRenameSuggestion(t *testing.T) {
	cases := []struct {
		label string
		want  string
	}{
		{"start", "app-start"},
		{"stop", "app-stop"},
		{"init", "app-init"},
		{"dev", "app-dev"},
		{"myserver", "app-myserver"},
	}
	for _, c := range cases {
		got := renameSuggestion(c.label)
		if got != c.want {
			t.Errorf("renameSuggestion(%q) = %q, want %q", c.label, got, c.want)
		}
	}
}

// TestDefaultGroupSelection verifies the dev/dev: pre-check heuristic.
func TestDefaultGroupSelection(t *testing.T) {
	commands := []config.Command{
		{Label: "dev"},
		{Label: "dev:server"},
		{Label: "dev:web"},
		{Label: "start"},
		{Label: "custom-1"},
		{Label: "app-start"},
	}
	got := defaultGroupSelection(commands)
	want := []string{"dev", "dev:server", "dev:web"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestDefaultGroupSelectionEmpty verifies that no dev commands produces empty slice.
func TestDefaultGroupSelectionEmpty(t *testing.T) {
	commands := []config.Command{
		{Label: "start"},
		{Label: "custom-1"},
	}
	got := defaultGroupSelection(commands)
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

// TestDefaultGroupSelectionOnlyDevPrefix verifies dev: prefix matching.
func TestDefaultGroupSelectionOnlyDevPrefix(t *testing.T) {
	commands := []config.Command{
		{Label: "dev:api"},
		{Label: "develop"},  // does NOT match — not "dev" exact or "dev:" prefix
		{Label: "dev:ui"},
	}
	got := defaultGroupSelection(commands)
	want := []string{"dev:api", "dev:ui"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestReservedLabelsContainsExpected verifies the reserved set matches the spec.
func TestReservedLabelsContainsExpected(t *testing.T) {
	expected := []string{"add", "drop", "init", "list", "reload", "rm", "up", "status", "down", "attach"}
	for _, label := range expected {
		if _, ok := reservedLabels[label]; !ok {
			t.Errorf("reservedLabels missing %q", label)
		}
	}
}

// TestBuildSummaryBasic verifies buildSummary produces expected output.
func TestBuildSummaryBasic(t *testing.T) {
	commands := []config.Command{
		{Label: "dev:server", Listeners: []config.Listener{{As: "myapp-server"}}},
		{Label: "dev:web", Listeners: []config.Listener{{As: "myapp-web"}}},
	}
	defaults := []string{"dev:server", "dev:web"}
	summary := buildSummary("myapp", commands, defaults)
	if !strings.Contains(summary, "Project: myapp") {
		t.Errorf("missing project name: %q", summary)
	}
	if !strings.Contains(summary, "dev:server") {
		t.Errorf("missing dev:server: %q", summary)
	}
	if !strings.Contains(summary, "myapp-server.localhost") {
		t.Errorf("missing hostname: %q", summary)
	}
	if !strings.Contains(summary, "dev:server, dev:web") {
		t.Errorf("missing defaults: %q", summary)
	}
}

// TestBuildSummaryWithPre verifies that pre-run hooks appear under their command.
func TestBuildSummaryWithPre(t *testing.T) {
	commands := []config.Command{
		{Label: "start", Pre: "pnpm build", Listeners: []config.Listener{{As: "myapp"}}},
		{Label: "dev", Listeners: []config.Listener{{As: "myapp-dev"}}},
	}
	summary := buildSummary("myapp", commands, nil)
	if !strings.Contains(summary, "pre: pnpm build") {
		t.Errorf("expected pre line in summary, got: %q", summary)
	}
	// dev has no pre — make sure the string "pre:" doesn't appear twice
	if strings.Count(summary, "pre:") != 1 {
		t.Errorf("expected exactly 1 pre: line, got:\n%s", summary)
	}
}

// TestBuildSummaryWithoutPre verifies that commands without a pre field don't
// produce a pre line.
func TestBuildSummaryWithoutPre(t *testing.T) {
	commands := []config.Command{
		{Label: "dev", Listeners: []config.Listener{{As: "myapp-dev"}}},
	}
	summary := buildSummary("myapp", commands, nil)
	if strings.Contains(summary, "pre:") {
		t.Errorf("expected no pre: line for command without Pre, got:\n%s", summary)
	}
}

// TestBuildSummaryNoDefaults verifies the "(none)" path.
func TestBuildSummaryNoDefaults(t *testing.T) {
	commands := []config.Command{
		{Label: "custom-1", Listeners: []config.Listener{{As: "myapp"}}},
	}
	summary := buildSummary("myapp", commands, nil)
	if !strings.Contains(summary, "(none)") {
		t.Errorf("expected (none) for empty defaults: %q", summary)
	}
}

// TestUnboundScriptDetection verifies the logic that identifies which
// selected scripts did not produce observed listeners during test-runs.
// This exercises the core of the Step-6b collection logic without touching
// any TUI prompt (huh is not invokable in tests).
func TestUnboundScriptDetection(t *testing.T) {
	selected := []plugins.Suggestion{
		{Label: "dev", Cmd: "pnpm dev"},
		{Label: "start", Cmd: "pnpm start"},
		{Label: "worker", Cmd: "pnpm worker"},
	}
	// "dev" and "worker" bound; "start" did not.
	binderLabels := map[string]bool{
		"dev":    true,
		"worker": true,
	}

	var unbound []plugins.Suggestion
	for _, s := range selected {
		if !binderLabels[s.Label] {
			unbound = append(unbound, s)
		}
	}

	if len(unbound) != 1 {
		t.Fatalf("expected 1 unbound, got %d: %v", len(unbound), unbound)
	}
	if unbound[0].Label != "start" {
		t.Errorf("expected unbound label=start, got %q", unbound[0].Label)
	}
}

// TestUnboundScriptDetectionAllBound verifies zero unbound when every
// selected script produced listeners.
func TestUnboundScriptDetectionAllBound(t *testing.T) {
	selected := []plugins.Suggestion{
		{Label: "dev", Cmd: "pnpm dev"},
		{Label: "start", Cmd: "pnpm start"},
	}
	binderLabels := map[string]bool{"dev": true, "start": true}

	var unbound []plugins.Suggestion
	for _, s := range selected {
		if !binderLabels[s.Label] {
			unbound = append(unbound, s)
		}
	}

	if len(unbound) != 0 {
		t.Errorf("expected 0 unbound, got %d: %v", len(unbound), unbound)
	}
}

// TestUnboundScriptDetectionNoneBound verifies all selected scripts are
// unbound when none produced listeners.
func TestUnboundScriptDetectionNoneBound(t *testing.T) {
	selected := []plugins.Suggestion{
		{Label: "start", Cmd: "pnpm start"},
		{Label: "serve", Cmd: "pnpm serve"},
	}
	binderLabels := map[string]bool{} // empty — none bound

	var unbound []plugins.Suggestion
	for _, s := range selected {
		if !binderLabels[s.Label] {
			unbound = append(unbound, s)
		}
	}

	if len(unbound) != len(selected) {
		t.Errorf("expected %d unbound, got %d", len(selected), len(unbound))
	}
}

// TestSyntheticMatchUsesScriptLabel verifies that when we build a config.Match
// for a synthetic (unbound) binder, the match key is Script and the value
// equals the suggestion label.
func TestSyntheticMatchUsesScriptLabel(t *testing.T) {
	label := "start"
	match := config.Match{Script: label}

	if match.Kind() != "script" {
		t.Errorf("expected kind=script, got %q", match.Kind())
	}
	if match.Script != label {
		t.Errorf("expected script=%q, got %q", label, match.Script)
	}
	if err := match.Validate(); err != nil {
		t.Errorf("match.Validate() = %v, want nil", err)
	}
}

// TestProposeHostnameForSyntheticBinder verifies that a zero observedListener
// (used for synthetic binders) with index=0, total=1 yields sanitize(projName).
func TestProposeHostnameForSyntheticBinder(t *testing.T) {
	cases := []struct {
		label    string
		projName string
		want     string
	}{
		// plain label — no colon, no package — falls through to project name
		{"start", "fmdplanner", "fmdplanner"},
		// label with colon → project-suffix heuristic
		{"dev:server", "myapp", "myapp-server"},
	}
	for _, c := range cases {
		got := proposeHostname(c.label, observedListener{}, c.projName, 0, 1)
		if got != c.want {
			t.Errorf("proposeHostname(%q, zero, %q, 0, 1) = %q, want %q",
				c.label, c.projName, got, c.want)
		}
	}
}

// TestInitNoTTYReturnsUserError: when stdin is not a TTY (non-interactive),
// RunInit should return a userError about TTY requirement. This test is
// only meaningful when stdin is NOT a terminal; it is skipped otherwise.
func TestInitNoTTYReturnsUserError(t *testing.T) {
	if isTTY() {
		t.Skip("stdin is a TTY; skipping no-TTY path test")
	}
	r := &stubRunner{responses: stubCaTrustResponses()}
	app, _ := newTestApp(t, r)

	dir := t.TempDir()
	err := app.RunInit(nil, nil, dir, &noopRunCmd{})
	if err == nil {
		t.Fatal("expected userError, got nil")
	}
	ue, ok := err.(*userError)
	if !ok {
		t.Fatalf("expected *userError, got %T: %v", err, err)
	}
	if !strings.Contains(ue.Error(), "no TTY detected") {
		t.Errorf("unexpected error message: %q", ue.Error())
	}
}

func TestDeriveCustomLabel(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"pnpm dev serve", "dev-serve"},
		{"pnpm run dev", "dev"},
		{"pnpm dev", "dev"},
		{"npm run start", "start"},
		{"yarn dev", "dev"},
		{"bun run server", "server"},
		{"caddy run --config Caddyfile", "caddy-run"},
		{"python -m my_app.server", "python"},
		{"node dist/server.js", "node-dist-server-js"},
		{"  ", "custom"},
		{"", "custom"},
		{"--just-flags --here", "custom"},
	}
	for _, c := range cases {
		got := deriveCustomLabel(c.in)
		if got != c.want {
			t.Errorf("deriveCustomLabel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUniquify(t *testing.T) {
	used := map[string]bool{"foo": true, "foo-2": true}
	if got := uniquify("foo", used); got != "foo-3" {
		t.Errorf("uniquify(foo) = %q, want foo-3", got)
	}
	if got := uniquify("bar", used); got != "bar" {
		t.Errorf("uniquify(bar) = %q, want bar", got)
	}
}

// TestApplyUnboundRewrite verifies that applyUnboundRewrite replaces Cmd while
// preserving the Label (and all other fields).
func TestApplyUnboundRewrite(t *testing.T) {
	original := plugins.Suggestion{
		Label: "dev",
		Cmd:   "pnpm dev",
		Body:  "tsx src/index.ts shows help and exits",
	}
	got := applyUnboundRewrite(original, "pnpm dev serve")

	if got.Cmd != "pnpm dev serve" {
		t.Errorf("Cmd = %q, want %q", got.Cmd, "pnpm dev serve")
	}
	if got.Label != original.Label {
		t.Errorf("Label changed: got %q, want %q", got.Label, original.Label)
	}
	if got.Body != original.Body {
		t.Errorf("Body changed: got %q, want %q", got.Body, original.Body)
	}
	// Original must be unchanged (value copy, not pointer).
	if original.Cmd != "pnpm dev" {
		t.Errorf("original Cmd mutated to %q", original.Cmd)
	}
}

// TestApplyUnboundRewritePreservesLabel verifies that the rewritten suggestion
// label is exactly what would be used for Match{Script: label} at runtime,
// regardless of the new command's content.
func TestApplyUnboundRewritePreservesLabel(t *testing.T) {
	cases := []struct {
		label      string
		originalCmd string
		newCmd     string
	}{
		{"dev", "pnpm dev", "pnpm dev serve"},
		{"start", "npm start", "node dist/server.js --port 3000"},
		{"serve", "yarn serve", "python -m http.server 8080"},
	}
	for _, c := range cases {
		s := plugins.Suggestion{Label: c.label, Cmd: c.originalCmd}
		got := applyUnboundRewrite(s, c.newCmd)
		if got.Label != c.label {
			t.Errorf("applyUnboundRewrite(%q → %q): Label = %q, want %q",
				c.originalCmd, c.newCmd, got.Label, c.label)
		}
		if got.Cmd != c.newCmd {
			t.Errorf("applyUnboundRewrite(%q → %q): Cmd = %q, want %q",
				c.originalCmd, c.newCmd, got.Cmd, c.newCmd)
		}
	}
}

// TestUnboundActionConstants verifies the action constants have distinct values
// and match the expected ordering (configure=0, rewrite=1, skip=2).
func TestUnboundActionConstants(t *testing.T) {
	if unboundActionConfigure == unboundActionRewrite {
		t.Error("configure and rewrite must be distinct")
	}
	if unboundActionConfigure == unboundActionSkip {
		t.Error("configure and skip must be distinct")
	}
	if unboundActionRewrite == unboundActionSkip {
		t.Error("rewrite and skip must be distinct")
	}
	if unboundActionConfigure != 0 {
		t.Errorf("configure should be 0 (default), got %d", unboundActionConfigure)
	}
}
