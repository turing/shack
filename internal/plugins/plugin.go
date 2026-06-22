package plugins

// Plugin discovers candidate commands from a project directory.
type Plugin interface {
	// Name returns the plugin's stable identifier.
	Name() string
	// Markers returns the file basenames whose presence indicates this
	// plugin can handle the project (e.g. "package.json"). Used by init
	// for project-root detection.
	Markers() []string
	// Suggest enumerates candidate (label, cmd) pairs for projectDir.
	Suggest(projectDir string) ([]Suggestion, error)
}

type Suggestion struct {
	Label string
	Cmd   string  // what shack will execute (e.g. "pnpm dev:server")
	Body  string  // raw script body for display only (e.g. "tsx watch src/server.ts")
}

var registry []Plugin

// Register adds a plugin to the global list. Called from each plugin's
// init() function.
func Register(p Plugin) { registry = append(registry, p) }

// Collect runs every registered plugin against projectDir and returns the
// union of suggestions plus any errors (one per failing plugin).
func Collect(projectDir string) ([]Suggestion, []error) {
	var (
		all  []Suggestion
		errs []error
	)
	for _, p := range registry {
		s, err := p.Suggest(projectDir)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		all = append(all, s...)
	}
	return all, errs
}

// AllMarkers returns the union of every plugin's project-root markers.
func AllMarkers() []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range registry {
		for _, m := range p.Markers() {
			if !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	return out
}
