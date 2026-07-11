package plugins

import "testing"

type stubPlugin struct {
	name        string
	markers     []string
	suggestions []Suggestion
	warnings    []string
	err         error
}

func (s *stubPlugin) Name() string      { return s.name }
func (s *stubPlugin) Markers() []string { return s.markers }
func (s *stubPlugin) Suggest(string) ([]Suggestion, []string, error) {
	return s.suggestions, s.warnings, s.err
}

func TestRegisterAndCollect(t *testing.T) {
	old := registry
	defer func() { registry = old }()
	registry = nil
	a := &stubPlugin{name: "a", suggestions: []Suggestion{{Label: "dev", Cmd: "pnpm dev"}}, warnings: []string{"warn-a"}}
	b := &stubPlugin{name: "b", suggestions: []Suggestion{{Label: "serve", Cmd: "pnpm serve"}}}
	Register(a)
	Register(b)
	got, warnings, errs := Collect("/tmp/proj")
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 suggestions, got %d: %+v", len(got), got)
	}
	if len(warnings) != 1 || warnings[0] != "warn-a" {
		t.Errorf("expected warnings [warn-a], got %+v", warnings)
	}
}

func TestAllMarkers(t *testing.T) {
	old := registry
	defer func() { registry = old }()
	registry = nil
	Register(&stubPlugin{name: "node", markers: []string{"package.json"}})
	Register(&stubPlugin{name: "rust", markers: []string{"Cargo.toml"}})
	got := AllMarkers()
	if len(got) != 2 {
		t.Errorf("got %v, want 2", got)
	}
}
