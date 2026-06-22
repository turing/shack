package plugins

import "testing"

type stubPlugin struct {
	name        string
	markers     []string
	suggestions []Suggestion
	err         error
}

func (s *stubPlugin) Name() string         { return s.name }
func (s *stubPlugin) Markers() []string    { return s.markers }
func (s *stubPlugin) Suggest(string) ([]Suggestion, error) {
	return s.suggestions, s.err
}

func TestRegisterAndCollect(t *testing.T) {
	old := registry
	defer func() { registry = old }()
	registry = nil
	a := &stubPlugin{name: "a", suggestions: []Suggestion{{Label: "dev", Cmd: "pnpm dev"}}}
	b := &stubPlugin{name: "b", suggestions: []Suggestion{{Label: "serve", Cmd: "pnpm serve"}}}
	Register(a)
	Register(b)
	got, errs := Collect("/tmp/proj")
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 suggestions, got %d: %+v", len(got), got)
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
