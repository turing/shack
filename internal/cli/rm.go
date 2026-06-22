package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/turing/shack/internal/validate"
	"github.com/spf13/cobra"
)

func newRmCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Drop name.localhost",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.RunRm(cmd.OutOrStdout(), args[0])
		},
	}
}

func (a *App) RunRm(out io.Writer, name string) error {
	if err := validate.Name(name); err != nil {
		return newUserError("shack: " + err.Error())
	}
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
		removed := doc.Remove(name)
		if !removed && len(dropped) == 0 {
			fmt.Fprintf(out, "%s not registered\n", name)
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
		if removed {
			fmt.Fprintf(out, "removed %s.localhost\n", name)
		} else {
			fmt.Fprintf(out, "%s not registered\n", name)
		}
		return nil
	})
}
