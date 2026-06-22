package cli

import (
	"net/http"
	"testing"
)

func TestClassifyProbe(t *testing.T) {
	if got := classifyProbe(nil, http.ErrServerClosed); got != probeUnreachable {
		t.Errorf("error → %d, want probeUnreachable", got)
	}
	if got := classifyProbe(&http.Response{StatusCode: 200}, nil); got != probeServing {
		t.Errorf("200 → %d, want probeServing", got)
	}
	if got := classifyProbe(&http.Response{StatusCode: 404}, nil); got != probeServing {
		t.Errorf("404 → %d, want probeServing (backend answered)", got)
	}
	if got := classifyProbe(&http.Response{StatusCode: 502}, nil); got != probeGateway {
		t.Errorf("502 → %d, want probeGateway", got)
	}
	if got := classifyProbe(&http.Response{StatusCode: 503}, nil); got != probeGateway {
		t.Errorf("503 → %d, want probeGateway", got)
	}
	if got := classifyProbe(&http.Response{StatusCode: 504}, nil); got != probeGateway {
		t.Errorf("504 → %d, want probeGateway", got)
	}
}

func TestInspectPaneDead(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"tmux list-panes -t sess:0 -F #{pane_dead} #{pane_dead_status}": {stdout: "1 1\n"},
	}}
	ps := inspectPane(r, "sess", 0)
	if !ps.dead || ps.exitStatus != 1 {
		t.Errorf("got %+v, want {dead:true exitStatus:1}", ps)
	}
}

func TestInspectPaneAlive(t *testing.T) {
	r := &stubRunner{responses: stubResponses{
		"tmux list-panes -t sess:0 -F #{pane_dead} #{pane_dead_status}": {stdout: "0 \n"},
	}}
	ps := inspectPane(r, "sess", 0)
	if ps.dead {
		t.Errorf("got %+v, want dead:false", ps)
	}
}

func TestLoopbackAddr(t *testing.T) {
	got, err := loopbackAddr("qbosync.localhost:443")
	if err != nil {
		t.Fatalf("loopbackAddr: %v", err)
	}
	if got != "127.0.0.1:443" {
		t.Errorf("loopbackAddr(qbosync.localhost:443) = %q, want 127.0.0.1:443", got)
	}
	if _, err := loopbackAddr("no-port"); err == nil {
		t.Error("expected error for address without port")
	}
}
