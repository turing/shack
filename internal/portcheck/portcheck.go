package portcheck

import (
	"fmt"
	"io"
	"strings"
)

// Runner abstracts os/exec for testability. The lifecycle package defines
// the canonical implementation; this package depends on the interface only.
type Runner interface {
	Run(cmd string, args ...string) (stdout, stderr string, err error)
}

// IsAlive reports whether a TCP listener is bound to the given local port.
//
// Errs on the side of "alive" when lsof itself fails (e.g. binary missing,
// signal) — never garbage-collect on uncertainty. Treats lsof exit-1 with
// empty output as "no listener" because that is the documented signal lsof
// uses when no matching processes are found.
//
// When a genuine error occurs (and we therefore err on the side of "alive"),
// a one-line warning is written to warn (if non-nil).
func IsAlive(r Runner, port int, warn io.Writer) bool {
	stdout, _, err := r.Run("lsof", "-i", fmt.Sprintf(":%d", port), "-sTCP:LISTEN", "-P", "-n")
	stdout = strings.TrimSpace(stdout)
	if err != nil {
		if isExitOneNoMatch(err, stdout) {
			return false
		}
		if warn != nil {
			fmt.Fprintf(warn, "shack: lsof check for port %d failed (%v); treating entry as alive\n", port, err)
		}
		return true
	}
	return stdout != ""
}

func isExitOneNoMatch(err error, stdout string) bool {
	type exitCoder interface{ ExitCode() int }
	if ec, ok := err.(exitCoder); ok && ec.ExitCode() == 1 && stdout == "" {
		return true
	}
	return false
}
