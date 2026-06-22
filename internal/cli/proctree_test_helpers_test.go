// internal/cli/proctree_test_helpers_test.go
//
// Shared test helpers for runner_test.go, observer_test.go, init_test.go.
package cli

import "github.com/turing/shack/internal/proctree"

func proctreeListener(pid int, comm string, port int) proctree.Listener {
	return proctree.Listener{PID: pid, Comm: comm, Port: port}
}

func proctreeIdentifiers(pid int, comm, script, pkg string) proctree.Identifiers {
	return proctree.Identifiers{PID: pid, Comm: comm, Script: script, Package: pkg}
}
