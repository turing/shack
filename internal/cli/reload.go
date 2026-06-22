package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newReloadCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "reload",
		Short: "Trigger caddy hot reload",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.RunReload(cmd.OutOrStdout())
		},
	}
}

func (a *App) RunReload(out io.Writer) error {
	if err := a.preflightTouchingCaddyfile(); err != nil {
		return err
	}
	if err := a.Caddy.Reload(); err != nil {
		return err
	}
	fmt.Fprintln(out, "caddy: reloaded")
	return nil
}
