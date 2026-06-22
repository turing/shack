package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

// childHandle wraps a spawned child process plus its signal-forwarding
// goroutine. Callers MUST call Wait() to reap the process before calling
// Stop(); Stop() only shuts down the signal forwarder, it does not kill
// or reap the child. The runtime callers (RunCommand, testRunCommand)
// follow this pattern: spawn, observe, Wait, then Stop.
type childHandle struct {
	PID  int
	Wait func() (int, error)
	Stop func()
	// SendSignal sends sig to the child's process group. Used by init's
	// test-run engine to issue SIGINT after settling.
	SendSignal func(sig syscall.Signal) error
}

// isFDTerminal reports whether fd is an open terminal device.
func isFDTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TIOCGETA)
	return err == nil
}

// startChild runs cmd with stdio piped to the supplied writers and the
// real os.Stdin. The child runs in its own process group so signals can
// be delivered to the whole group.
//
// When stdin (fd 0) is a terminal, startChild promotes the child's process
// group to the foreground via TIOCSPGRP so that the child receives terminal
// input correctly (e.g. raw-mode / tcsetattr calls from tools like vite).
// The previous foreground process group is restored after the child exits.
//
// startChild returns immediately after cmd.Start() succeeds. The caller
// invokes h.Wait() to block until the child exits, and h.Stop() to tear
// down the signal-forwarding goroutine.
func startChild(cmd *exec.Cmd, stdout, stderr io.Writer) (*childHandle, error) {
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("spawn %s: %w", cmd.Path, err)
	}
	pgid := cmd.Process.Pid

	// --- terminal foreground-PG management ---
	// oldPG == 0 means we are not running in a terminal (CI, piped) and the
	// restoration no-op below is safe.
	var oldPG int
	if isFDTerminal(0) {
		// Suppress SIGTTOU while we call TIOCSPGRP from a background PG.
		// We never need to act on SIGTTOU so leaving it ignored is fine.
		signal.Ignore(syscall.SIGTTOU)

		if pg, err := unix.IoctlGetInt(0, unix.TIOCGPGRP); err == nil {
			oldPG = pg
			// Make the child's PG (which equals child PID since Setpgid:true
			// creates a new group with the child as leader) the foreground.
			_ = unix.IoctlSetPointerInt(0, unix.TIOCSPGRP, cmd.Process.Pid)
		}
	}

	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	stopCh := make(chan struct{})

	go func() {
		for {
			select {
			case s := <-sigCh:
				_ = syscall.Kill(-pgid, s.(syscall.Signal))
			case <-stopCh:
				return
			}
		}
	}()

	stop := func() {
		signal.Stop(sigCh)
		select {
		case <-stopCh:
		default:
			close(stopCh)
		}
	}

	// restoreFG gives the terminal back to our own process group once the
	// child has exited.
	restoreFG := func() {
		if oldPG != 0 {
			_ = unix.IoctlSetPointerInt(0, unix.TIOCSPGRP, oldPG)
		}
	}

	wait := func() (int, error) {
		err := cmd.Wait()
		restoreFG()
		if err == nil {
			return 0, nil
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), nil
		}
		return -1, err
	}

	send := func(sig syscall.Signal) error {
		return syscall.Kill(-pgid, sig)
	}

	return &childHandle{PID: pgid, Wait: wait, Stop: stop, SendSignal: send}, nil
}
