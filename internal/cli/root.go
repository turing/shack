package cli

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/turing/shack/internal/config"
	"github.com/turing/shack/internal/lifecycle"
	"github.com/turing/shack/internal/proctree"
)

// Version is overridable at build time via `-ldflags "-X
// github.com/turing/shack/internal/cli.Version=<v>"`.
var Version = "0.1.0"

type App struct {
	Runner     lifecycle.Runner
	Resolver   *lifecycle.Resolver
	Caddy      *lifecycle.Caddy
	FileExists func(string) bool

	// PollInterval is the stream tick interval for `up`'s streaming loop.
	// Zero means use the default (200ms). Tests set small values.
	PollInterval time.Duration

	// Probe is the HTTPS readiness check for a registered host. nil means use
	// httpProbe with defaultProbeClient (system trust). Tests inject a fake.
	Probe func(host string) probeOutcome
}

func newApp() *App {
	r := lifecycle.ExecRunner{}
	res := lifecycle.NewResolver(r)
	return &App{
		Runner:   r,
		Resolver: res,
		Caddy:    lifecycle.NewCaddy(r, res),
		FileExists: func(p string) bool {
			_, err := os.Stat(p)
			return err == nil
		},
	}
}

func newRootCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:           "shack",
		Short:         "Manage *.localhost reverse proxies in your Caddyfile",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			label := args[0]
			pwd, err := os.Getwd()
			if err != nil {
				return err
			}
			projectRoot, err := config.FindRoot(pwd)
			if err != nil {
				return newUserError("shack: no .shack/config.json found — run `shack init` to configure")
			}
			var procRunner proctree.Runner = lifecycle.ExecRunner{}
			exit, runErr := app.RunCommand(context.Background(), projectRoot, label, os.Stdout, os.Stderr, procRunner)
			if runErr != nil {
				return runErr
			}
			if exit != 0 {
				return &ChildExitError{Code: exit}
			}
			return nil
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(newAddCmd(app))
	root.AddCommand(newRmCmd(app))
	root.AddCommand(newListCmd(app))
	root.AddCommand(newDropCmd(app))
	root.AddCommand(newUpCmd(app))
	root.AddCommand(newDownCmd(app))
	root.AddCommand(newAttachCmd(app))
	root.AddCommand(newReloadCmd(app))
	root.AddCommand(newStatusCmd(app))
	root.AddCommand(newInitCmd(app))
	return root
}

func Execute() error {
	app := newApp()
	return newRootCmd(app).Execute()
}
