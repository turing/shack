package proctree

import "testing"

func TestReadEnvParsesNpmVars(t *testing.T) {
	const sample = `node /tmp/server.js --port 3000 npm_lifecycle_event=dev:server npm_package_name=fmdplanner PATH=/usr/bin USER=alice`
	r := &fakeRunner{table: map[string]response{
		"ps eww -p 200 -o command=": {stdout: sample + "\n"},
	}}
	got, err := ReadEnv(r, 200)
	if err != nil {
		t.Fatalf("ReadEnv: %v", err)
	}
	if got["npm_lifecycle_event"] != "dev:server" {
		t.Errorf("npm_lifecycle_event = %q, want dev:server", got["npm_lifecycle_event"])
	}
	if got["npm_package_name"] != "fmdplanner" {
		t.Errorf("npm_package_name = %q, want fmdplanner", got["npm_package_name"])
	}
}

func TestReadEnvNoMatches(t *testing.T) {
	r := &fakeRunner{table: map[string]response{
		"ps eww -p 200 -o command=": {stdout: "node /tmp/server.js\n"},
	}}
	got, err := ReadEnv(r, 200)
	if err != nil {
		t.Fatalf("ReadEnv: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty env, got %v", got)
	}
}
