package proctree

import (
	"fmt"
	"strconv"
	"strings"
)

// Descendants returns {root} ∪ all transitive children of root, as PIDs.
// `pgrep -P <pid>` returns exit 1 with empty stdout when a pid has no
// children — that's a successful "leaf", not an error.
//
// root <= 0 is rejected immediately: PID 0 is not a valid user process, and
// pgrep -P 0 on some systems (or lsof -p 0 on macOS) can return unexpected
// system-wide results.
func Descendants(r Runner, root int) ([]int, error) {
	if root <= 0 {
		return nil, fmt.Errorf("Descendants: invalid root PID %d", root)
	}
	visited := map[int]bool{}
	stack := []int{root}
	for len(stack) > 0 {
		pid := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if visited[pid] {
			continue
		}
		visited[pid] = true
		stdout, _, err := r.Run("pgrep", "-P", strconv.Itoa(pid))
		stdout = strings.TrimSpace(stdout)
		if err != nil {
			if isExitOneEmpty(err, stdout) {
				continue
			}
			return nil, fmt.Errorf("pgrep -P %d: %w", pid, err)
		}
		for _, line := range strings.Split(stdout, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			n, perr := strconv.Atoi(line)
			if perr != nil {
				continue
			}
			if !visited[n] {
				stack = append(stack, n)
			}
		}
	}
	out := make([]int, 0, len(visited))
	for pid := range visited {
		out = append(out, pid)
	}
	return out, nil
}
