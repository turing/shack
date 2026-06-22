package store

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWithLockRunsFn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".lock")
	called := false
	err := WithLock(path, 1*time.Second, func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithLock: %v", err)
	}
	if !called {
		t.Error("fn was not called")
	}
}

func TestWithLockPropagatesFnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".lock")
	want := errors.New("boom")
	err := WithLock(path, 1*time.Second, func() error { return want })
	if !errors.Is(err, want) {
		t.Errorf("got %v, want %v", err, want)
	}
}

func TestWithLockSerializesConcurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".lock")

	var mu sync.Mutex
	inside := 0
	maxInside := 0

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = WithLock(path, 5*time.Second, func() error {
				mu.Lock()
				inside++
				if inside > maxInside {
					maxInside = inside
				}
				mu.Unlock()
				time.Sleep(20 * time.Millisecond)
				mu.Lock()
				inside--
				mu.Unlock()
				return nil
			})
		}()
	}
	wg.Wait()
	if maxInside != 1 {
		t.Errorf("maxInside = %d, want 1 (serialization failed)", maxInside)
	}
}
