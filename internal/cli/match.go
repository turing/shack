package cli

import (
	"github.com/turing/shack/internal/config"
	"github.com/turing/shack/internal/proctree"
)

// MatchListener walks listeners in order and returns the first entry
// whose match key equals the corresponding identifier on id. Returns nil
// on no match.
func MatchListener(id proctree.Identifiers, listeners []config.Listener) *config.Listener {
	for i := range listeners {
		l := &listeners[i]
		switch l.Match.Kind() {
		case "script":
			if id.Script != "" && id.Script == l.Match.Script {
				return l
			}
		case "package":
			if id.Package != "" && id.Package == l.Match.Package {
				return l
			}
		case "comm":
			if id.Comm != "" && id.Comm == l.Match.Comm {
				return l
			}
		}
	}
	return nil
}
