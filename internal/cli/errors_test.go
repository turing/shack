package cli

import (
	"errors"
	"testing"
)

func TestExitCodeChildExitError(t *testing.T) {
	err := &ChildExitError{Code: 42}
	got := ExitCode(err)
	if got != 42 {
		t.Errorf("ExitCode(&ChildExitError{Code: 42}) = %d, want 42", got)
	}
}

func TestExitCodeUserError(t *testing.T) {
	err := newUserError("oops")
	got := ExitCode(err)
	if got != 1 {
		t.Errorf("ExitCode(newUserError(\"oops\")) = %d, want 1", got)
	}
}

func TestExitCodeOther(t *testing.T) {
	err := errors.New("oops")
	got := ExitCode(err)
	if got != 2 {
		t.Errorf("ExitCode(errors.New(\"oops\")) = %d, want 2", got)
	}
}

func TestExitCodeNil(t *testing.T) {
	got := ExitCode(nil)
	if got != 0 {
		t.Errorf("ExitCode(nil) = %d, want 0", got)
	}
}
