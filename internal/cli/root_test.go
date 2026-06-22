package cli

import (
	"os"
	"testing"
)

func TestFallbackMissingConfigReturnsUserError(t *testing.T) {
	// Create a temporary directory without .shack/config.json
	tmpDir := t.TempDir()

	// Create a minimal app with stub runner
	r := &stubRunner{responses: stubResponses{}}
	app := &App{
		Runner:   r,
		Resolver: nil, // Not needed for this test
		Caddy:    nil, // Not needed for this test
		FileExists: func(string) bool {
			return false
		},
	}

	// Change to the temp directory
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldCwd); err != nil {
			t.Fatalf("os.Chdir: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir(%s): %v", tmpDir, err)
	}

	// Execute the root command with a label (triggers the fallback RunE)
	root := newRootCmd(app)
	root.SetArgs([]string{"dev"})
	err = root.Execute()

	// Verify we get a userError with the expected message
	if err == nil {
		t.Fatalf("root.Execute() with no config returned nil, want userError")
	}

	ue, ok := err.(*userError)
	if !ok {
		t.Fatalf("root.Execute() returned %T, want *userError; err=%v", err, err)
	}

	expectedMsg := "shack: no .shack/config.json found — run `shack init` to configure"
	if ue.Error() != expectedMsg {
		t.Errorf("userError message = %q, want %q", ue.Error(), expectedMsg)
	}
}

func TestFallbackNoArgsReturnsHelp(t *testing.T) {
	r := &stubRunner{responses: stubResponses{}}
	app := &App{
		Runner:     r,
		Resolver:   nil,
		Caddy:      nil,
		FileExists: func(string) bool { return false },
	}

	root := newRootCmd(app)
	root.SetArgs([]string{})

	err := root.Execute()

	// Help returns nil error
	if err != nil {
		t.Errorf("root.Execute() with no args returned error %v, want nil", err)
	}
}

func TestExitCodeMapping(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "ChildExitError",
			err:      &ChildExitError{Code: 42},
			wantCode: 42,
		},
		{
			name:     "userError",
			err:      newUserError("invalid config"),
			wantCode: 1,
		},
		{
			name:     "generic error",
			err:      newUserError("something broke"),
			wantCode: 1, // userError always maps to 1
		},
		{
			name:     "nil error",
			err:      nil,
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExitCode(tt.err)
			if got != tt.wantCode {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.wantCode)
			}
		})
	}
}

func TestRootCmdWithInitSubcommand(t *testing.T) {
	tmpDir := t.TempDir()

	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldCwd); err != nil {
			t.Fatalf("os.Chdir: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}

	// newRootCmd includes the init subcommand
	r := &stubRunner{responses: stubResponses{}}
	app := &App{
		Runner:     r,
		Resolver:   nil,
		Caddy:      nil,
		FileExists: func(string) bool { return false },
	}

	root := newRootCmd(app)

	// Verify init subcommand exists
	for _, cmd := range root.Commands() {
		if cmd.Name() == "init" {
			return // Found init subcommand
		}
	}
	t.Errorf("init subcommand not found under root command")
}
