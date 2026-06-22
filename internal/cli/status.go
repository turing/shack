package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newStatusCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print caddy state, caddyfile path, entry count",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.RunStatus(cmd.OutOrStdout())
		},
	}
}

func (a *App) RunStatus(out io.Writer) error {
	if err := a.Preflight(); err != nil {
		return err
	}
	running, err := a.Caddy.IsRunning()
	if err != nil {
		return err
	}
	state := "stopped"
	if running {
		state = "running"
	}
	path, err := a.caddyfilePath()
	if err != nil {
		return err
	}
	doc, err := a.loadDocument(path)
	if err != nil {
		return err
	}
	caState := "not trusted (run: sudo caddy trust)"
	if a.Caddy.IsCATrusted() {
		caState = "trusted"
	}
	fmt.Fprintf(out, "caddy: %s\ncaddyfile: %s\nentries: %d\nca: %s\n", state, path, len(doc.Entries), caState)
	return nil
}
