package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/turing/shack/internal/validate"
)

func newAddCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <port>",
		Short: "Register name.localhost → localhost:port",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			port, err := strconv.Atoi(args[1])
			if err != nil {
				return newUserError(fmt.Sprintf("shack: port must be an integer, got %q", args[1]))
			}
			return app.RunAdd(cmd.OutOrStdout(), args[0], port)
		},
	}
}

// RunAdd is the testable implementation of the add subcommand.
func (a *App) RunAdd(out io.Writer, name string, port int) error {
	if err := validate.Name(name); err != nil {
		return newUserError("shack: " + err.Error())
	}
	if err := validate.Port(port); err != nil {
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

		existing, found := doc.Get(name)
		if found && existing.Port == port {
			// Even on the no-op success path, persist any sweep results so
			// dead entries don't pile up.
			if len(dropped) > 0 {
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
			}
			fmt.Fprintf(out, "%s.localhost → localhost:%d (already registered)\n", name, port)
			return nil
		}

		doc.Add(name, port)
		if err := a.saveDocument(doc, path); err != nil {
			return err
		}
		if err := a.Caddy.EnsureRunning(); err != nil {
			return err
		}
		if err := a.Caddy.Reload(); err != nil {
			return err
		}
		printSwept(out, dropped)
		fmt.Fprintf(out, "%s.localhost → localhost:%d\n", name, port)
		return nil
	})
}
