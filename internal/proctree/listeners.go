package proctree

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Listener describes one TCP listening socket attributed to a single PID.
type Listener struct {
	PID  int
	Comm string
	Port int
}

// portRegex matches the address column of lsof -P -n output for TCP LISTEN
// rows: "*:3000", "127.0.0.1:5173", "[::1]:3001".
var portRegex = regexp.MustCompile(`(?:\*|\[?[0-9a-fA-F.:]+\]?):(\d+)$`)

// Listeners runs `lsof -nP -a -iTCP -sTCP:LISTEN -p <pids>` and returns one
// Listener per (pid, port) pair found. Empty pids → empty result. Exit 1
// with empty stdout (lsof's "no match" signal) → empty result.
//
// The -a flag (AND selection criteria) is required on macOS. Without it, lsof
// treats -p and -iTCP as OR conditions, causing -p to be ignored entirely and
// all system TCP listeners to be returned regardless of the pid list.
func Listeners(r Runner, pids []int) ([]Listener, error) {
	if len(pids) == 0 {
		return nil, nil
	}
	pidArg := joinPids(pids)
	stdout, _, err := r.Run("lsof", "-nP", "-a", "-iTCP", "-sTCP:LISTEN", "-p", pidArg)
	if err != nil {
		if isExitOneEmpty(err, strings.TrimSpace(stdout)) {
			return nil, nil
		}
		return nil, fmt.Errorf("lsof -p %s: %w", pidArg, err)
	}
	var out []Listener
	for i, line := range strings.Split(stdout, "\n") {
		if i == 0 || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		comm := fields[0]
		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		name := fields[8]
		m := portRegex.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		port, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		out = append(out, Listener{PID: pid, Comm: comm, Port: port})
	}
	return out, nil
}

func joinPids(pids []int) string {
	parts := make([]string, len(pids))
	for i, p := range pids {
		parts[i] = strconv.Itoa(p)
	}
	return strings.Join(parts, ",")
}
