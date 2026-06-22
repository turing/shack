package proctree

import (
	"fmt"
	"strconv"
	"strings"
)

// ReadEnv shells out to `ps eww -p <pid> -o command=` and extracts
// KEY=VALUE pairs from the output. macOS prints the command line and the
// environment together separated by spaces, with no clean delimiter — we
// scan all whitespace-separated tokens and treat any matching the
// `[A-Za-z_][A-Za-z0-9_]*=...` shape as an env var.
//
// Heuristic: parses ps eww output; literal KEY=value args in command lines may yield false positives.
func ReadEnv(r Runner, pid int) (map[string]string, error) {
	stdout, _, err := r.Run("ps", "eww", "-p", strconv.Itoa(pid), "-o", "command=")
	if err != nil {
		return nil, fmt.Errorf("ps eww -p %d: %w", pid, err)
	}
	env := map[string]string{}
	for _, tok := range strings.Fields(stdout) {
		k, v, ok := splitKV(tok)
		if !ok {
			continue
		}
		env[k] = v
	}
	return env, nil
}

func splitKV(tok string) (string, string, bool) {
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return "", "", false
	}
	key := tok[:eq]
	if !validKey(key) {
		return "", "", false
	}
	return key, tok[eq+1:], true
}

func validKey(k string) bool {
	if k == "" {
		return false
	}
	for i, c := range k {
		switch {
		case c == '_':
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case i > 0 && c >= '0' && c <= '9':
		default:
			return false
		}
	}
	return true
}
