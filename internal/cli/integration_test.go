//go:build integration
// +build integration

package cli

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/turing/shack/internal/config"
)

// TestE2E exercises the real binary against the user's real brew + caddy.
// Run with: go test -tags=integration ./...
func TestE2E(t *testing.T) {
	app := newApp()
	if err := app.Preflight(); err != nil {
		t.Skipf("skipping integration: %v", err)
	}

	// Find a free port and bind a listener for the test duration so the
	// alive check sees something.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	portStr := strconv.Itoa(port)

	out := &bytes.Buffer{}
	if err := app.RunAdd(out, "shacktest", port); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Best-effort cleanup so a failure mid-test doesn't leave the user's
	// real Caddyfile dirty.
	t.Cleanup(func() { _ = app.RunRm(io.Discard, "shacktest") })

	out.Reset()
	if err := app.RunList(out); err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out.String(), "shacktest.localhost → localhost:"+portStr) {
		t.Errorf("list missing entry: %q", out.String())
	}

	out.Reset()
	if err := app.RunRm(out, "shacktest"); err != nil {
		t.Fatalf("rm: %v", err)
	}

	// Lifecycle smoke (matches spec's smoke test description). We don't
	// actually want to leave caddy stopped on the user's machine — start it
	// back up after.
	out.Reset()
	if err := app.RunDown(out, false); err != nil {
		t.Fatalf("stop: %v", err)
	}
	t.Cleanup(func() { _ = app.RunUp(io.Discard) })
}

func TestE2EDevWrap(t *testing.T) {
	app := newApp()
	if err := app.Preflight(); err != nil {
		t.Skipf("skipping integration: %v", err)
	}
	caddyfilePath := "/opt/homebrew/etc/Caddyfile"
	if _, err := os.Stat(caddyfilePath); err != nil {
		t.Skipf("skipping: caddyfile not found: %v", err)
	}

	dir := t.TempDir()
	// Tiny inline node program that opens a listener on a free port and
	// holds it for ~2s, printing the chosen port to stdout.
	script := `node -e "const s=require('net').createServer().listen(0,'127.0.0.1',()=>{process.stdout.write('listening:'+s.address().port+'\\n');setTimeout(()=>process.exit(0),2000);});"`
	if err := config.Write(dir, config.Project{
		Name: "csint",
		Commands: []config.Command{{
			Label:     "dev",
			Cmd:       script,
			Listeners: []config.Listener{{Match: config.Match{Comm: "node"}, As: "csint"}},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(t.TempDir(), "shack")
	if out, err := exec.Command("go", "build", "-o", bin, "./cmd/shack").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	t.Cleanup(func() { _ = app.RunRm(io.Discard, "csint") })

	// Run the wrapper. While it runs, sample the Caddyfile to confirm the
	// registration appears mid-flight.
	c := exec.CommandContext(context.Background(), bin, "dev")
	c.Dir = dir

	var midflightHit atomic.Bool
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if raw, _ := os.ReadFile(caddyfilePath); strings.Contains(string(raw), "csint.localhost {") {
					midflightHit.Store(true)
				}
			}
		}
	}()

	out, err := c.CombinedOutput()
	close(stop)
	if err != nil {
		t.Fatalf("dev wrap: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "listening:") {
		t.Errorf("child output missing 'listening:': %q", out)
	}
	if !midflightHit.Load() {
		t.Errorf("never observed csint.localhost mid-flight; registration didn't happen or was too fast")
	}

	// After exit, the registration must be gone.
	raw, _ := os.ReadFile(caddyfilePath)
	if strings.Contains(string(raw), "csint.localhost") {
		t.Errorf("csint.localhost should be unregistered after wrapper exit")
	}
}
