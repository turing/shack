package cli

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/turing/shack/internal/caddyfile"
	"github.com/turing/shack/internal/portcheck"
	"github.com/spf13/cobra"
)

func newListCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Print registrations with alive/dead state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.RunList(cmd.OutOrStdout())
		},
	}
}

func (a *App) RunList(out io.Writer) error {
	if err := a.preflightTouchingCaddyfile(); err != nil {
		return err
	}
	path, err := a.caddyfilePath()
	if err != nil {
		return err
	}
	doc, err := a.loadDocument(path)
	if err != nil {
		return err
	}
	entries := append([]caddyfile.Entry(nil), doc.Entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	for _, e := range entries {
		state := "dead"
		if portcheck.IsAlive(a.Runner, e.Port, os.Stderr) {
			state = "alive"
		}
		fmt.Fprintf(out, "%s.localhost → localhost:%d   (%s)\n", e.Name, e.Port, state)
	}
	return nil
}
