package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// testCardRenderer holds lipgloss styles for the test-phase "card" output.
// It is constructed once per RunInit call so the renderer is bound to the
// correct output writer (enabling automatic color-profile detection).
type testCardRenderer struct {
	bar      lipgloss.Style // the left vertical bar character
	label    lipgloss.Style // bold + colored command label
	dimmed   lipgloss.Style // dimmed text (counts, subtitles)
	bold     lipgloss.Style // plain bold
	checkOK  lipgloss.Style // green ✓
	checkErr lipgloss.Style // red ✗
	out      io.Writer
}

const (
	barChar     = "▎"
	colorBar    = lipgloss.Color("#C678DD") // soft purple — visible on both light + dark
	colorOK     = lipgloss.Color("#98C379") // green
	colorErr    = lipgloss.Color("#E06C75") // red
	colorLbl    = lipgloss.Color("#61AFEF") // blue — label accent
	colorWarn   = lipgloss.Color("#E5C07E") // amber — advisory warnings
	colorOrange = lipgloss.Color("#D19A66") // orange — warning badge
)

// printInitWarnings renders advisory warnings (e.g. ambiguous lockfile state)
// in the init flow's visual language: every line of a warning gets a ▎ bar
// prefix and amber text, matching the test-phase cards. When a warning's
// first line starts with "warning: ", that text prefix is replaced by a
// reverse-video orange " warning " badge. A trailing blank line follows so
// the warning does not visually fuse with the huh form printed immediately
// after. The color profile is auto-detected from w (lipgloss.NewRenderer
// strips color on non-TTY writers, the same convention the test cards
// follow).
func printInitWarnings(w io.Writer, warnings []string) {
	if len(warnings) == 0 {
		return
	}
	r := lipgloss.NewRenderer(w)
	bar := r.NewStyle().Foreground(colorWarn).Bold(true)
	text := r.NewStyle().Foreground(colorWarn)
	badge := r.NewStyle().Foreground(colorOrange).Reverse(true)
	for _, msg := range warnings {
		for i, line := range strings.Split(msg, "\n") {
			if i == 0 && strings.HasPrefix(line, "warning: ") {
				rest := strings.TrimPrefix(line, "warning: ")
				fmt.Fprintf(w, "%s %s %s\n",
					bar.Render(barChar), badge.Render(" warning "), text.Render(rest))
				continue
			}
			fmt.Fprintf(w, "%s %s\n", bar.Render(barChar), text.Render(line))
		}
	}
	fmt.Fprintln(w)
}

// newTestCardRenderer creates a renderer whose color profile is auto-detected
// from w (lipgloss.NewRenderer handles TTY vs non-TTY transparently).
func newTestCardRenderer(w io.Writer) *testCardRenderer {
	r := lipgloss.NewRenderer(w)
	return &testCardRenderer{
		bar:      r.NewStyle().Foreground(colorBar).Bold(true),
		label:    r.NewStyle().Foreground(colorLbl).Bold(true),
		dimmed:   r.NewStyle().Faint(true),
		bold:     r.NewStyle().Bold(true),
		checkOK:  r.NewStyle().Foreground(colorOK).Bold(true),
		checkErr: r.NewStyle().Foreground(colorErr).Bold(true),
		out:      w,
	}
}

// PrintTestOpen prints the two-line opening card before a test run starts.
//
//	▎ TESTING [1/2] dev:web
//	▎ pnpm dev:web — up to 30s
func (tc *testCardRenderer) PrintTestOpen(label, cmd string, index, total int) {
	b := tc.bar.Render(barChar)
	testing := tc.bold.Render("TESTING")
	count := tc.dimmed.Render(fmt.Sprintf("[%d/%d]", index, total))
	lbl := tc.label.Render(label)
	subtitle := tc.dimmed.Italic(true).Render(fmt.Sprintf("%s — up to 30s", cmd))
	fmt.Fprintf(tc.out, "\n%s %s %s %s\n%s %s\n",
		b, testing, count, lbl,
		b, subtitle,
	)
}

// SpinnerSuffix returns a styled suffix string to assign to spinner.Suffix.
//
//	shack: testing dev:web…
func (tc *testCardRenderer) SpinnerSuffix(label string) string {
	return " " + tc.dimmed.Render(fmt.Sprintf("shack: testing %s…", label))
}

// PrintTestClose prints the result line after a test run finishes, then a
// blank line as padding before the next test (or next wizard step).
//
// Success:
//
//	▎ ✓ dev:web — listener on port 5173
//
// Failure:
//
//	▎ ✗ dev:web — no port bound (timeout or early exit)
func (tc *testCardRenderer) PrintTestClose(label string, ports []int) {
	b := tc.bar.Render(barChar)
	lbl := tc.label.Render(label)
	if len(ports) > 0 {
		portList := ""
		for i, p := range ports {
			if i > 0 {
				portList += ", "
			}
			portList += fmt.Sprintf("port %d", p)
		}
		check := tc.checkOK.Render("✓")
		detail := tc.dimmed.Render(fmt.Sprintf("— listener on %s", portList))
		fmt.Fprintf(tc.out, "%s %s %s %s\n\n", b, check, lbl, detail)
	} else {
		check := tc.checkErr.Render("✗")
		detail := tc.dimmed.Render("— no port bound (timeout or early exit)")
		fmt.Fprintf(tc.out, "%s %s %s %s\n\n", b, check, lbl, detail)
	}
}
