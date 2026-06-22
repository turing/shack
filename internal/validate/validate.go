package validate

import (
	"fmt"
	"regexp"
)

var nameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Name validates a shack registration name.
func Name(s string) error {
	if !nameRegex.MatchString(s) {
		return fmt.Errorf("invalid name %q — must match [a-z0-9][a-z0-9-]*", s)
	}
	return nil
}

// Port validates a TCP port number.
func Port(p int) error {
	if p < 1 || p > 65535 {
		return fmt.Errorf("port %d out of range (1–65535)", p)
	}
	return nil
}
