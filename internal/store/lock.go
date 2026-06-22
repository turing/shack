package store

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// ErrLockTimeout is returned when WithLock cannot acquire the lock in time.
var ErrLockTimeout = errors.New("lock acquisition timed out")

// WithLock acquires an exclusive flock(2) on path, runs fn, and releases the
// lock. If the lock is held by another process, blocks up to timeout.
func WithLock(path string, timeout time.Duration, fn func() error) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open lockfile %s: %w", path, err)
	}
	defer f.Close()

	deadline := time.Now().Add(timeout)
	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, unix.EWOULDBLOCK) {
			return fmt.Errorf("flock %s: %w", path, err)
		}
		if time.Now().After(deadline) {
			return ErrLockTimeout
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN)

	return fn()
}
