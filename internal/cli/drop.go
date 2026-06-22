package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newDropCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "drop",
		Short: "Drop registrations whose ports are no longer in use",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.RunDrop(cmd.OutOrStdout())
		},
	}
}

func (a *App) RunDrop(out io.Writer) error {
	if err := a.preflightTouchingCaddyfile(); err != nil {
		return err
	}
	path, err := a.caddyfilePath()
	if err != nil {
		return err
	}
	return a.withLock(path, func() error {
		doc, err := a.loadDocument(path)
		if err != nil {
			return err
		}
		dropped := a.sweepDead(&doc, os.Stderr)
		if len(dropped) == 0 {
			fmt.Fprintln(out, "drop: nothing to do")
			return nil
		}
		if err := a.saveDocument(doc, path); err != nil {
			return err
		}
		running, err := a.Caddy.IsRunning()
		if err != nil {
			return err
		}
		if running {
			if err := a.Caddy.Reload(); err != nil {
				return err
			}
		}
		printSwept(out, dropped)
		return nil
	})
}
