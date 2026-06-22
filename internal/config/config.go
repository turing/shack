package config

import "fmt"

// Project is the top-level shape of .shack/config.json.
type Project struct {
	Name     string    `json:"name"`
	Defaults []string  `json:"defaults,omitempty"`
	Commands []Command `json:"commands"`
}

// Command names a runnable label and the listeners shack should look
// for in the resulting process tree.
type Command struct {
	Label     string     `json:"label"`
	Cmd       string     `json:"cmd"`
	Pre       string     `json:"pre,omitempty"`
	Listeners []Listener `json:"listeners"`
}

// Listener pairs a Match (how to identify a listener) with the hostname
// it should be registered as.
type Listener struct {
	Match Match  `json:"match"`
	As    string `json:"as"`
}

// Match has exactly one of Script, Package, or Comm set. Ports are never
// recorded — the runtime reads them from lsof.
type Match struct {
	Script  string `json:"script,omitempty"`
	Package string `json:"package,omitempty"`
	Comm    string `json:"comm,omitempty"`
}

// Kind returns "script", "package", "comm", or "" if no key is set.
func (m Match) Kind() string {
	switch {
	case m.Script != "":
		return "script"
	case m.Package != "":
		return "package"
	case m.Comm != "":
		return "comm"
	}
	return ""
}

// Validate enforces "exactly one matcher key set".
func (m Match) Validate() error {
	n := 0
	if m.Script != "" {
		n++
	}
	if m.Package != "" {
		n++
	}
	if m.Comm != "" {
		n++
	}
	if n == 0 {
		return fmt.Errorf("match: no key set (need one of script, package, comm)")
	}
	if n > 1 {
		return fmt.Errorf("match: %d keys set, want exactly 1", n)
	}
	return nil
}
