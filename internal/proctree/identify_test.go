package proctree

import "testing"

func TestIdentifyReadsAllSources(t *testing.T) {
	r := &fakeRunner{table: map[string]response{
		"ps eww -p 200 -o command=": {stdout: "node /server.js npm_lifecycle_event=dev:server npm_package_name=fmdplanner\n"},
	}}
	got, err := Identify(r, 200, "node")
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if got.PID != 200 || got.Comm != "node" {
		t.Errorf("PID/Comm wrong: %+v", got)
	}
	if got.Script != "dev:server" {
		t.Errorf("Script = %q, want dev:server", got.Script)
	}
	if got.Package != "fmdplanner" {
		t.Errorf("Package = %q, want fmdplanner", got.Package)
	}
}

func TestIdentifyMissingEnvIsNotAnError(t *testing.T) {
	r := &fakeRunner{table: map[string]response{
		"ps eww -p 200 -o command=": {stdout: "node /server.js\n"},
	}}
	got, err := Identify(r, 200, "node")
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if got.Script != "" || got.Package != "" {
		t.Errorf("expected empty Script/Package, got %+v", got)
	}
}
