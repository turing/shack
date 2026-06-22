package cli

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/turing/shack/internal/lifecycle"
)

// paneState is the observed tmux state of one window's pane.
type paneState struct {
	dead       bool
	exitStatus int // meaningful only when dead
}

// inspectPane reads pane_dead/pane_dead_status for session:index. Any tmux
// error is reported as a live pane (dead:false) — absence of evidence of death
// is treated as "still running", never as a crash.
func inspectPane(r lifecycle.Runner, session string, index int) paneState {
	out, _, err := r.Run("tmux", "list-panes", "-t",
		session+":"+strconv.Itoa(index), "-F",
		"#{pane_dead} #{pane_dead_status}")
	if err != nil {
		return paneState{}
	}
	fields := strings.Fields(strings.TrimSpace(out))
	ps := paneState{}
	if len(fields) >= 1 && fields[0] == "1" {
		ps.dead = true
	}
	if ps.dead && len(fields) >= 2 {
		ps.exitStatus, _ = strconv.Atoi(fields[1])
	}
	return ps
}

// probePath is the fixed sentinel hit through caddy. We don't care what the
// status means, only that the backend produced it.
const probePath = "/shack/smellin-the-tls-404-roses"

// probeOutcome is the result of the HTTPS readiness probe.
type probeOutcome int

const (
	probeUnreachable probeOutcome = iota // TLS/connection error — not serving (or CA untrusted)
	probeServing                         // a non-gateway HTTP status from the backend
	probeGateway                         // caddy 502/503/504 — registered but backend not responding
)

// classifyProbe maps an HTTP result to a probe outcome. Pure.
func classifyProbe(resp *http.Response, err error) probeOutcome {
	if err != nil || resp == nil {
		return probeUnreachable
	}
	switch resp.StatusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return probeGateway
	default:
		return probeServing
	}
}

// httpProbe does GET https://<host>.localhost<probePath> with the supplied
// client (system trust) and maps the result. host is a listener's As name
// (no scheme, no ".localhost").
func httpProbe(client *http.Client, host string) probeOutcome {
	resp, err := client.Get("https://" + host + ".localhost" + probePath)
	if resp != nil {
		defer resp.Body.Close()
	}
	return classifyProbe(resp, err)
}

// loopbackAddr rewrites a "<host>:<port>" dial target to loopback, preserving
// the port. Go's resolver does not resolve "*.localhost" to loopback (curl and
// browsers special-case it; Go does not), so the probe must route there itself
// — exactly what a browser does for a .localhost name.
func loopbackAddr(addr string) (string, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort("127.0.0.1", port), nil
}

// defaultProbeClient is the system-trust HTTPS client for readiness probes.
// No custom TLS config: Go validates against the OS trust store with the
// request's SNI (the .localhost host), so the probe sees exactly what a real
// client sees. Only the dial target is forced to loopback (see loopbackAddr),
// because Go cannot resolve *.localhost; SNI and the Host header are untouched.
func defaultProbeClient() *http.Client {
	dialer := &net.Dialer{}
	return &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				loopback, err := loopbackAddr(addr)
				if err != nil {
					return nil, err
				}
				return dialer.DialContext(ctx, network, loopback)
			},
		},
	}
}

