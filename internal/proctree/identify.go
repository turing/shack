package proctree

// Identifiers is the bundle the matcher consumes for each listener.
type Identifiers struct {
	PID     int
	Comm    string
	Script  string // npm_lifecycle_event
	Package string // npm_package_name
}

// Identify combines comm (from lsof) with env-derived identifiers.
func Identify(r Runner, pid int, comm string) (Identifiers, error) {
	id := Identifiers{PID: pid, Comm: comm}
	env, err := ReadEnv(r, pid)
	if err != nil {
		return id, err
	}
	id.Script = env["npm_lifecycle_event"]
	id.Package = env["npm_package_name"]
	return id, nil
}
