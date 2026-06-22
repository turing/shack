package proctree

// Runner is the same shell-out interface used elsewhere in shack.
// `lifecycle.ExecRunner` satisfies this structurally.
type Runner interface {
	Run(cmd string, args ...string) (stdout, stderr string, err error)
}

// isExitOneEmpty matches "process exited 1 with empty stdout" — the signal
// pgrep, lsof, and similar tools use to mean "no match".
func isExitOneEmpty(err error, stdout string) bool {
	type exitCoder interface{ ExitCode() int }
	if ec, ok := err.(exitCoder); ok && ec.ExitCode() == 1 && stdout == "" {
		return true
	}
	return false
}
