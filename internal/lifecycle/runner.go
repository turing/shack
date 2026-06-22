package lifecycle

import (
	"bytes"
	"os/exec"
)

// Runner abstracts os/exec so commands can be faked in tests.
type Runner interface {
	Run(cmd string, args ...string) (stdout, stderr string, err error)
}

// ExecRunner is the production Runner — invokes commands via os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(cmd string, args ...string) (string, string, error) {
	c := exec.Command(cmd, args...)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	return stdout.String(), stderr.String(), err
}
