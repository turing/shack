package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/turing/shack/internal/cli"
)

func main() {
	err := cli.Execute()
	if err == nil {
		return
	}
	var ce *cli.ChildExitError
	if !errors.As(err, &ce) {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(cli.ExitCode(err))
}
