package cli

type exitErr struct{ code int }

func (e *exitErr) Error() string { return "exit" }
func (e *exitErr) ExitCode() int { return e.code }
