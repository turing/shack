package cli

import "fmt"

// userError signals a user mistake (bad input, missing dependency).
// userErrors map to exit code 1; everything else maps to 2.
type userError struct{ msg string }

func (e *userError) Error() string { return e.msg }

func newUserError(msg string) error { return &userError{msg: msg} }

// ChildExitError signals that a wrapped command exited with a non-zero
// code. main.go translates this into the matching os.Exit code without
// printing the inner error message.
type ChildExitError struct{ Code int }

func (e *ChildExitError) Error() string { return fmt.Sprintf("child exited with code %d", e.Code) }

// ExitCode returns the shell exit code for an error from Execute.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if ce, ok := err.(*ChildExitError); ok {
		return ce.Code
	}
	if _, ok := err.(*userError); ok {
		return 1
	}
	return 2
}
