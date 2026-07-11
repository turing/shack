package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/turing/shack/internal/config"
	"github.com/turing/shack/internal/lifecycle"
	"github.com/turing/shack/internal/plugins"
	_ "github.com/turing/shack/internal/plugins/node" // register
	"github.com/turing/shack/internal/proctree"
	"github.com/turing/shack/internal/validate"
	"github.com/briandowns/spinner"
	"github.com/charmbracelet/huh"

	"github.com/spf13/cobra"
)

const (
	initSettleWindow = 3 * time.Second
	initRunTimeout   = 30 * time.Second
)

// indent prefixes every non-empty line of s with prefix.
func indent(s, prefix string) string {
	var sb strings.Builder
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		if line == "" {
			continue
		}
		sb.WriteString(prefix)
		sb.WriteString(line)
	}
	return sb.String()
}

func newInitCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Set up .shack/config.json for the current project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pwd, err := os.Getwd()
			if err != nil {
				return err
			}
			// P-5: lifecycle.ExecRunner already structurally satisfies
			// proctree.Runner; no wrapper needed.
			var procRunner proctree.Runner = lifecycle.ExecRunner{}
			rc := &execRunCmd{procRunner: procRunner}
			return app.RunInit(cmd.OutOrStdout(), cmd.ErrOrStderr(), pwd, rc)
		},
	}
}

// runCmd abstracts the test-run engine so init_test can substitute a fake.
// stdout/stderr are the writers the spawned command's output streams to —
// during the test phase init pipes these to a buffer so a spinner can
// run cleanly above; the buffer is dumped to the real stderr only if the
// test failed (no listener bound).
type runCmd interface {
	testRun(projectRoot, cmd string, stdout, stderr io.Writer) []observedListener
}

type execRunCmd struct {
	procRunner proctree.Runner
}

func (e *execRunCmd) testRun(projectRoot, cmd string, stdout, stderr io.Writer) []observedListener {
	return testRunCommand(context.Background(), projectRoot, cmd,
		stdout, stderr, e.procRunner, initSettleWindow, initRunTimeout)
}

// reservedLabels are the shack subcommand names that project
// labels cannot collide with.
var reservedLabels = map[string]struct{}{
	"add": {}, "drop": {}, "init": {}, "list": {}, "reload": {},
	"rm": {}, "up": {}, "status": {}, "down": {}, "attach": {},
}

// renameSuggestion returns a default replacement for a label that
// collides with a reserved name. Default rule: prefix with "app-".
func renameSuggestion(label string) string {
	return "app-" + label
}

// pkgManagers are package-manager names we'll skip when deriving a label
// from a custom command (so "pnpm dev serve" → "dev-serve" not "pnpm").
var pkgManagers = map[string]bool{
	"pnpm": true, "npm": true, "yarn": true, "bun": true,
}

// deriveCustomLabel picks a sensible shack-side label from a custom
// command string. Rules:
//
//  1. If the first token is a package manager (pnpm/npm/yarn/bun), drop
//     it. If the next token is the literal "run", drop that too.
//  2. From the remaining tokens, take everything up to (but not
//     including) the first flag-shaped token (starts with "-").
//  3. Sanitize each kept token and join with hyphens.
//  4. If nothing usable remains, fall back to "custom".
//
// Examples:
//
//	"pnpm dev serve"               → "dev-serve"
//	"pnpm run dev"                 → "dev"
//	"caddy run --config Caddyfile" → "caddy-run"
//	"python -m my_app.server"      → "python"
//	"node dist/server.js"          → "node-dist-server-js"
func deriveCustomLabel(cmd string) string {
	tokens := strings.Fields(cmd)
	if len(tokens) == 0 {
		return "custom"
	}
	if pkgManagers[tokens[0]] {
		tokens = tokens[1:]
		if len(tokens) > 0 && tokens[0] == "run" {
			tokens = tokens[1:]
		}
	}
	var kept []string
	for _, t := range tokens {
		if strings.HasPrefix(t, "-") {
			break
		}
		s := sanitize(t)
		if s != "" {
			kept = append(kept, s)
		}
	}
	if len(kept) == 0 {
		return "custom"
	}
	return strings.Join(kept, "-")
}

// uniquify appends -2, -3, … to base until the result isn't in used.
func uniquify(base string, used map[string]bool) string {
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !used[candidate] {
			return candidate
		}
	}
}

// defaultGroupSelection returns the labels that should be
// pre-checked in the default-group multiselect: any whose label is
// "dev" or starts with "dev:".
func defaultGroupSelection(commands []config.Command) []string {
	out := []string{}
	for _, c := range commands {
		if c.Label == "dev" || strings.HasPrefix(c.Label, "dev:") {
			out = append(out, c.Label)
		}
	}
	return out
}

// isTTY reports whether stdin is a terminal.
func isTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// buildSummary constructs the final summary string shown in the confirm step
// and printed after writing config.
func buildSummary(projName string, commands []config.Command, defaults []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Project: %s\n", projName)
	fmt.Fprintf(&sb, "Commands:\n")
	for _, cmd := range commands {
		if len(cmd.Listeners) == 1 {
			fmt.Fprintf(&sb, "  %-18s → %s.localhost\n", cmd.Label, cmd.Listeners[0].As)
		} else {
			for i, l := range cmd.Listeners {
				if i == 0 {
					fmt.Fprintf(&sb, "  %-18s → %s.localhost\n", cmd.Label, l.As)
				} else {
					fmt.Fprintf(&sb, "  %-18s   %s.localhost\n", "", l.As)
				}
			}
		}
		if cmd.Pre != "" {
			fmt.Fprintf(&sb, "    pre: %s\n", cmd.Pre)
		}
	}
	if len(defaults) > 0 {
		fmt.Fprintf(&sb, "\nDefault group (shack up): [%s]\n", strings.Join(defaults, ", "))
	} else {
		fmt.Fprintf(&sb, "\nDefault group (shack up): (none)\n")
	}
	return sb.String()
}

// RunInit drives the init flow. projectRoot is the current project directory.
// The runCmd interface is parameterized so tests can supply canned listener
// results without spawning real processes.
//
// out    — progress output; user terminal stdout.
// stderr — (unused directly; kept for interface consistency with other commands).
func (a *App) RunInit(out, stderr io.Writer, projectRoot string, rc runCmd) error {
	// ---- Step 1: preflight ----
	if !isTTY() {
		return newUserError("shack: init must be run from a terminal (no TTY detected)")
	}
	if err := a.preflightTouchingCaddyfile(); err != nil {
		return err
	}
	if !tmuxAvailable(a.Runner) {
		return newUserError("shack: tmux is not installed — run `brew install tmux`")
	}

	// Clear screen + scrollback so the wizard starts at the top.
	// \x1b[3J = clear scrollback, \x1b[2J = clear visible, \x1b[H = home cursor.
	fmt.Fprint(out, "\x1b[3J\x1b[2J\x1b[H")

	// ---- Step 2: overwrite check ----
	if root, err := config.FindRoot(projectRoot); err == nil && root != "" {
		var overwrite bool
		if err := huh.NewConfirm().
			Title(fmt.Sprintf("Overwrite existing %s/.shack/config.json?", root)).
			Affirmative("Yes").
			Negative("No").
			Value(&overwrite).
			Run(); err != nil {
			return err
		}
		if !overwrite {
			fmt.Fprintln(out, "Nothing configured.")
			return nil
		}
		projectRoot = root
	}

	// ---- Step 3: project name ----
	projName := defaultProjectName(projectRoot)
	if err := huh.NewInput().
		Title("Project name").
		Value(&projName).
		Validate(func(s string) error {
			if s == "" {
				return errors.New("required")
			}
			return validate.Name(s)
		}).
		Run(); err != nil {
		return err
	}

	// ---- Step 4: plugin discovery (silent) ----
	suggestions, warnings, _ := plugins.Collect(projectRoot)
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Label < suggestions[j].Label
	})

	// ---- Step 5: select tests + add custom ----
	// Default = none selected. shack will only run scripts the
	// user explicitly opts into.
	var testLabels []string
	options := make([]huh.Option[string], 0, len(suggestions))
	for _, s := range suggestions {
		body := s.Body
		if body == "" {
			body = s.Cmd
		}
		options = append(options, huh.NewOption(s.Label+"  —  "+body, s.Label))
	}
	var customRaw string
	var form5 *huh.Form
	if len(suggestions) == 0 {
		// skip multi-select if no suggestions; just ask for customs
		form5 = huh.NewForm(huh.NewGroup(
			huh.NewText().
				Title("No server scripts detected. Add custom commands (one per line, blank to abort)").
				Value(&customRaw),
		))
	} else {
		form5 = huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Which commands should shack test for port binds? (space to select)").
					Options(options...).
					Value(&testLabels),
				huh.NewText().
					Title("Add custom commands (one per line, optional)").
					Placeholder("e.g. caddy run --config Caddyfile").
					Value(&customRaw),
			),
		)
	}
	// Surface plugin advisories (e.g. ambiguous lockfile state) styled to match
	// the init flow, directly above the selection form.
	printInitWarnings(out, warnings)
	if err := form5.Run(); err != nil {
		return err
	}

	selected := []plugins.Suggestion{}
	for _, s := range suggestions {
		for _, l := range testLabels {
			if l == s.Label {
				selected = append(selected, s)
				break
			}
		}
	}
	usedLabels := map[string]bool{}
	for _, s := range selected {
		usedLabels[s.Label] = true
	}
	for _, line := range strings.Split(customRaw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		label := uniquify(deriveCustomLabel(line), usedLabels)
		usedLabels[label] = true
		selected = append(selected, plugins.Suggestion{
			Label: label,
			Cmd:   line,
			Body:  line,
		})
	}

	if len(selected) == 0 {
		fmt.Fprintln(out, "Nothing configured.")
		return nil
	}

	// ---- Step 6: test runs (sequential, line output) ----
	// binder holds test-run results for one selected script.
	// synthetic is true for scripts that didn't bind a port but the user
	// chose to configure anyway; their match is derived from the script
	// label rather than from observed Identifiers.
	type binder struct {
		sug       plugins.Suggestion
		listeners []observedListener
		synthetic bool
	}
	var binders []binder
	card := newTestCardRenderer(out)
	for i, s := range selected {
		// Opening card — shows label, count, command, timeout.
		card.PrintTestOpen(s.Label, s.Cmd, i+1, len(selected))

		// Capture child output to a buffer so the spinner can run cleanly.
		// We only dump the buffer afterward if the test failed (so the
		// user can see the error) — successful tests stay quiet.
		buf := &strings.Builder{}
		sp := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		sp.Suffix = card.SpinnerSuffix(s.Label)
		sp.Start()
		listeners := rc.testRun(projectRoot, s.Cmd, buf, buf)
		sp.Stop()

		if len(listeners) == 0 && buf.Len() > 0 {
			// Surface the captured output so user can debug what happened.
			fmt.Fprintln(out, "  --- script output ---")
			fmt.Fprintln(out, indent(buf.String(), "  "))
			fmt.Fprintln(out, "  ---")
		}

		// Closing card — ✓/✗ with port or failure message.
		ports := make([]int, 0, len(listeners))
		for _, l := range listeners {
			ports = append(ports, l.Listener.Port)
		}
		card.PrintTestClose(s.Label, ports)

		if len(listeners) > 0 {
			binders = append(binders, binder{sug: s, listeners: listeners})
		}
	}

	// ---- Step 6b: unbound-script prompt ----
	// Collect scripts that were selected but produced no observed listeners.
	var unbound []plugins.Suggestion
	binderLabels := map[string]bool{}
	for _, b := range binders {
		binderLabels[b.sug.Label] = true
	}
	for _, s := range selected {
		if !binderLabels[s.Label] {
			unbound = append(unbound, s)
		}
	}
	for _, s := range unbound {
		action, newCmd, err := promptUnboundScript(s)
		if err != nil {
			return err
		}
		switch action {
		case unboundActionConfigure:
			binders = append(binders, binder{sug: s, synthetic: true})
		case unboundActionRewrite:
			rewritten := applyUnboundRewrite(s, newCmd)
			binders = append(binders, binder{sug: rewritten, synthetic: true})
		case unboundActionSkip:
			// do nothing
		}
	}

	if len(binders) == 0 {
		fmt.Fprintln(out, "Nothing configured.")
		return nil
	}

	// ---- Step 7: hostname assignment ----
	var commands []config.Command
	for _, b := range binders {
		cmdCfg := config.Command{
			Label: b.sug.Label,
			Cmd:   b.sug.Cmd,
		}
		if b.synthetic {
			// No observed listener: synthesize a single entry matched by script label.
			match := config.Match{Script: b.sug.Label}
			proposed := proposeHostname(b.sug.Label, observedListener{}, projName, 0, 1)
			host := proposed
			if err := huh.NewInput().
				Title(fmt.Sprintf("[%s] hostname (script=%s)", b.sug.Label, b.sug.Label)).
				Value(&host).
				Validate(func(s string) error {
					return validate.Name(s)
				}).
				Run(); err != nil {
				return err
			}
			cmdCfg.Listeners = append(cmdCfg.Listeners, config.Listener{
				Match: match,
				As:    host,
			})
		} else {
			for li, lst := range b.listeners {
				proposed := proposeHostname(b.sug.Label, lst, projName, li, len(b.listeners))
				host := proposed
				match := proposeMatch(lst.ID, projName)
				if err := huh.NewInput().
					Title(fmt.Sprintf("[%s] hostname for port %d (%s=%s)",
						b.sug.Label, lst.Listener.Port, match.Kind(), matchValue(match))).
					Value(&host).
					Validate(func(s string) error {
						return validate.Name(s)
					}).
					Run(); err != nil {
					return err
				}
				cmdCfg.Listeners = append(cmdCfg.Listeners, config.Listener{
					Match: match,
					As:    host,
				})
			}
		}
		commands = append(commands, cmdCfg)
	}

	// ---- Step 7b: optional pre-run hooks ----
	if len(commands) > 0 {
		for i := range commands {
			var pre string
			if err := huh.NewInput().
				Title(fmt.Sprintf("Pre-run for %q (optional)", commands[i].Label)).
				Description("Runs synchronously before the command. Must exit 0. Leave blank to skip.").
				Value(&pre).
				Run(); err != nil {
				return err
			}
			if strings.TrimSpace(pre) != "" {
				commands[i].Pre = pre
			}
		}
	}

	// ---- Step 8: collision rename ----
	used := map[string]bool{}
	for i := range commands {
		if _, reserved := reservedLabels[commands[i].Label]; !reserved && !used[commands[i].Label] {
			used[commands[i].Label] = true
			continue
		}
		original := commands[i].Label
		suggested := renameSuggestion(original)
		newLabel := suggested
		prompt := fmt.Sprintf("Label %q conflicts with shack built-in.\nPick a new label:", original)
		if used[original] {
			prompt = fmt.Sprintf("Label %q conflicts with another configured label.\nPick a new label:", original)
		}
		if err := huh.NewInput().
			Title(prompt).
			Value(&newLabel).
			Validate(func(s string) error {
				if s == "" {
					return errors.New("required")
				}
				if _, r := reservedLabels[s]; r {
					return fmt.Errorf("%q is reserved", s)
				}
				if used[s] {
					return fmt.Errorf("%q already used by another command", s)
				}
				return validate.Name(s)
			}).
			Run(); err != nil {
			return err
		}
		commands[i].Label = newLabel
		used[newLabel] = true
	}

	// ---- Step 9: default group selection ----
	// Default = none selected. An explicit "none" sentinel option lets
	// the user clearly say "shack up should not run anything."
	// If "none" is checked, all other selections are ignored.
	const noneSentinel = "__none__"
	var defaults []string
	if len(commands) > 0 {
		groupOpts := make([]huh.Option[string], 0, len(commands)+1)
		groupOpts = append(groupOpts, huh.NewOption("(none — shack up only ensures caddy is running)", noneSentinel))
		for _, c := range commands {
			groupOpts = append(groupOpts, huh.NewOption(c.Label, c.Label))
		}
		if err := huh.NewMultiSelect[string]().
			Title("Which commands should run when you 'shack up'?").
			Description("These start together; the rest run on demand via 'shack <label>'.").
			Options(groupOpts...).
			Value(&defaults).
			Run(); err != nil {
			return err
		}
		// If the user picked "none", drop everything else.
		for _, v := range defaults {
			if v == noneSentinel {
				defaults = nil
				break
			}
		}
	}

	// ---- Step 10: confirm + write ----
	var confirm bool = true
	summary := buildSummary(projName, commands, defaults)
	if err := huh.NewConfirm().
		Title("Write config?").
		Description(summary).
		Affirmative("Write").
		Negative("Cancel").
		Value(&confirm).
		Run(); err != nil {
		return err
	}
	if !confirm {
		fmt.Fprintln(out, "Nothing configured.")
		return nil
	}

	proj := config.Project{
		Name:     projName,
		Defaults: defaults,
		Commands: commands,
	}
	if err := config.Write(projectRoot, proj); err != nil {
		return err
	}
	fmt.Fprintf(out, "wrote %s\n", filepath.Join(projectRoot, ".shack", "config.json"))
	fmt.Fprintln(out, "")
	fmt.Fprint(out, summary)
	return nil
}

// unboundScriptAction is the choice the user makes for each unbound script.
type unboundScriptAction int

const (
	unboundActionConfigure unboundScriptAction = iota // use the auto-detected cmd as-is
	unboundActionRewrite                              // replace the cmd with a user-supplied value
	unboundActionSkip                                 // drop this script
)

// promptUnboundScript shows the per-script three-way select for scripts that
// didn't bind a port during the test-run phase. It returns the chosen action
// and (when action == unboundActionRewrite) the new command string.
func promptUnboundScript(s plugins.Suggestion) (unboundScriptAction, string, error) {
	const (
		optConfigure = "configure"
		optRewrite   = "rewrite"
		optSkip      = "skip"
	)

	var choice string
	selectForm := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title(fmt.Sprintf("%q didn't bind a port during test", s.Label)).
			Description(fmt.Sprintf("Auto-detected command: %s", s.Cmd)).
			Options(
				huh.NewOption("Configure as-is (match by script name at runtime)", optConfigure),
				huh.NewOption("Rewrite the command", optRewrite),
				huh.NewOption("Skip (don't configure)", optSkip),
			).
			Value(&choice),
	))
	if err := selectForm.Run(); err != nil {
		return unboundActionSkip, "", err
	}

	switch choice {
	case optConfigure:
		return unboundActionConfigure, "", nil
	case optSkip:
		return unboundActionSkip, "", nil
	}

	// Rewrite path: ask for the new command.
	newCmd := s.Cmd
	inputForm := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title(fmt.Sprintf("New command for %q", s.Label)).
			Description("Replace the auto-detected command. The script label stays the same.").
			Value(&newCmd).
			Validate(func(v string) error {
				if strings.TrimSpace(v) == "" {
					return errors.New("command cannot be empty")
				}
				return nil
			}),
	))
	if err := inputForm.Run(); err != nil {
		return unboundActionSkip, "", err
	}
	return unboundActionRewrite, strings.TrimSpace(newCmd), nil
}

// applyUnboundRewrite returns a copy of s with Cmd replaced by newCmd.
// The label is preserved so the synthetic Match{Script: label} remains
// correct — npm_lifecycle_event is still the label at runtime regardless
// of subcommand arguments.
func applyUnboundRewrite(s plugins.Suggestion, newCmd string) plugins.Suggestion {
	s.Cmd = newCmd
	return s
}

// defaultProjectName prefers package.json#name, else the directory basename.
// If sanitize produces an empty string, falls back to "project".
func defaultProjectName(projectRoot string) string {
	type pkg struct{ Name string `json:"name"` }
	var raw string
	if data, err := os.ReadFile(filepath.Join(projectRoot, "package.json")); err == nil {
		var p pkg
		if err := json.Unmarshal(data, &p); err == nil && p.Name != "" {
			raw = p.Name
		}
	}
	if raw == "" {
		raw = filepath.Base(projectRoot)
	}
	// sanitize covers spaces/uppercase in dir basenames.
	sanitized := sanitize(raw)
	if sanitized == "" {
		return "project"
	}
	return sanitized
}

// sanitize lowercases and replaces invalid characters with hyphens, then
// trims leading/trailing hyphens, so the result conforms to shack's
// hostname regex.
func sanitize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// proposeMatch builds a config.Match from an Identifiers bundle following
// the spec's preference: script → package (when distinct from root) →
// comm. lsof always supplies a COMMAND, so id.Comm is the last-ditch
// signal. There is no defensive Match{Comm:""} branch — by the lsof
// contract, Comm is always populated when this is called.
func proposeMatch(id proctree.Identifiers, projectName string) config.Match {
	if id.Script != "" {
		return config.Match{Script: id.Script}
	}
	if id.Package != "" && id.Package != projectName {
		return config.Match{Package: id.Package}
	}
	return config.Match{Comm: id.Comm}
}

// proposeHostname picks a default hostname per the spec heuristics:
// script-suffix → workspace name → fallback (<project> / <project>-<n>).
func proposeHostname(label string, ol observedListener, projectName string, index, total int) string {
	if i := strings.IndexByte(label, ':'); i > 0 && i < len(label)-1 {
		return sanitize(projectName + "-" + label[i+1:])
	}
	if ol.ID.Package != "" && ol.ID.Package != projectName {
		return sanitize(projectName + "-" + ol.ID.Package)
	}
	if total <= 1 || index == 0 {
		return sanitize(projectName)
	}
	return sanitize(fmt.Sprintf("%s-%d", projectName, index+1))
}

// matchValue extracts the populated field of m as a string for prompt
// display.
func matchValue(m config.Match) string {
	switch m.Kind() {
	case "script":
		return m.Script
	case "package":
		return m.Package
	case "comm":
		return m.Comm
	}
	return ""
}
