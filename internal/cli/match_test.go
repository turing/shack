package cli

import (
	"testing"

	"github.com/turing/shack/internal/config"
	"github.com/turing/shack/internal/proctree"
)

func TestMatchByScript(t *testing.T) {
	listeners := []config.Listener{
		{Match: config.Match{Script: "dev:server"}, As: "api"},
		{Match: config.Match{Package: "web"},       As: "web"},
	}
	id := proctree.Identifiers{Script: "dev:server", Comm: "node"}
	got := MatchListener(id, listeners)
	if got == nil || got.As != "api" {
		t.Errorf("got %+v, want api", got)
	}
}

func TestMatchByPackageWhenScriptMisses(t *testing.T) {
	listeners := []config.Listener{
		{Match: config.Match{Script: "dev:server"}, As: "api"},
		{Match: config.Match{Package: "web"},       As: "web"},
	}
	id := proctree.Identifiers{Package: "web", Comm: "vite"}
	got := MatchListener(id, listeners)
	if got == nil || got.As != "web" {
		t.Errorf("got %+v, want web", got)
	}
}

func TestMatchByComm(t *testing.T) {
	listeners := []config.Listener{{Match: config.Match{Comm: "vite"}, As: "v"}}
	id := proctree.Identifiers{Comm: "vite"}
	got := MatchListener(id, listeners)
	if got == nil || got.As != "v" {
		t.Errorf("got %+v, want v", got)
	}
}

func TestMatchOrderFirstWins(t *testing.T) {
	listeners := []config.Listener{
		{Match: config.Match{Comm: "node"}, As: "first"},
		{Match: config.Match{Comm: "node"}, As: "second"},
	}
	id := proctree.Identifiers{Comm: "node"}
	got := MatchListener(id, listeners)
	if got == nil || got.As != "first" {
		t.Errorf("got %+v, want first", got)
	}
}

func TestMatchNone(t *testing.T) {
	listeners := []config.Listener{{Match: config.Match{Script: "build"}, As: "b"}}
	id := proctree.Identifiers{Script: "dev"}
	if got := MatchListener(id, listeners); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

// TestMatchNoneCommOnly verifies that an Identifiers with only a Comm
// value still returns nil when no listeners[] entry matches it. There is
// no defensive Comm-only fallback inside MatchListener; the synthesized
// `<project>-<port>` post-exit warning path in RunCommand handles the
// no-match case downstream.
func TestMatchNoneCommOnly(t *testing.T) {
	listeners := []config.Listener{{Match: config.Match{Script: "build"}, As: "b"}}
	id := proctree.Identifiers{Comm: "node"}
	if got := MatchListener(id, listeners); got != nil {
		t.Errorf("got %+v, want nil (Comm-only id, no matching listeners)", got)
	}
}
