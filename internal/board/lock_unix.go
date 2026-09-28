//go:build !windows

package board

import (
	"fmt"
	"os"
	"syscall"
)

// fileLock is one acquired advisory flock. The open fd owns the lock; the
// lock lives on the sidecar file and dies with the process (flock semantics:
// the kernel releases it when the fd is closed by exit, including on crash
// or kill -9 — no stale-lock cleanup path exists or is needed).
type fileLock struct {
	f *os.File
}

// acquireFileLock takes a blocking exclusive flock on path, creating the
// file if needed. Blocking (LOCK_EX) means concurrent writers queue instead
// of failing; the queue drains in kernel order and every writer re-reads the
// board files after acquiring, so serialization is exact.
func acquireFileLock(path string) (*fileLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("board lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("board lock: %w", err)
	}
	return &fileLock{f: f}, nil
}

// tryAcquireFileLock attempts a NONBLOCKING exclusive flock; ok is false
// when another writer holds the lock right now. Test seam: the concurrency
// tests use it to prove the lock is a real exclusive resource (the no-lock
// RED control bypasses it entirely).
func tryAcquireFileLock(path string) (*fileLock, bool, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, fmt.Errorf("board lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("board lock: %w", err)
	}
	return &fileLock{f: f}, true, nil
}

// release drops the flock and closes the fd.
func (l *fileLock) release() {
	syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	l.f.Close()
}
