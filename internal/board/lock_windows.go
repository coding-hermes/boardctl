//go:build windows

package board

import (
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

// fileLock is one acquired exclusive byte-range lock (LockFileEx) on the
// sidecar lock file. Windows has no flock(2); LockFileEx with
// LOCKFILE_EXCLUSIVE_LOCK blocks the same way and the handle is released by
// process exit even on crash, so the no-stale-lock property holds. The
// locked range is the first byte of the file, which all acquirers contend
// on.
//
// LockFileEx/UnlockFileEx are reached through kernel32 via syscall.NewLazyDLL
// — the stdlib syscall package does not wrap them, and this module carries no
// external dependencies (no golang.org/x/sys) to get them.
type fileLock struct {
	f    *os.File
	ol   syscall.Overlapped
	held bool
}

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
	errLockViolation        = syscall.Errno(158) // ERROR_LOCK_VIOLATION
)

func lockFileEx(h syscall.Handle, flags uint32, ol *syscall.Overlapped) error {
	r1, _, err := procLockFileEx.Call(
		uintptr(h),
		uintptr(flags),
		0, // dwReserved
		1, // lock 1 byte (low)
		0, // lock 0 bytes (high)
		uintptr(unsafe.Pointer(ol)),
	)
	if r1 == 0 {
		return err
	}
	return nil
}

func unlockFileEx(h syscall.Handle, ol *syscall.Overlapped) error {
	r1, _, err := procUnlockFileEx.Call(
		uintptr(h),
		0, // dwReserved
		1, // unlock 1 byte (low)
		0, // unlock 0 bytes (high)
		uintptr(unsafe.Pointer(ol)),
	)
	if r1 == 0 {
		return err
	}
	return nil
}

// acquireFileLock takes a blocking exclusive lock on path, creating the file
// if needed: probe first (FAIL_IMMEDIATELY), and on contention retry until
// the holder releases. Holders are single file writes, so the poll interval
// is short.
func acquireFileLock(path string) (*fileLock, error) {
	for {
		l, ok, err := tryAcquireFileLock(path)
		if err != nil {
			return nil, err
		}
		if ok {
			return l, nil
		}
		time.Sleep(500 * time.Microsecond)
	}
}

// tryAcquireFileLock is the nonblocking probe used by acquireFileLock and by
// the concurrency tests to prove the lock is a real exclusive resource.
func tryAcquireFileLock(path string) (*fileLock, bool, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, fmt.Errorf("board lock: %w", err)
	}
	h := syscall.Handle(f.Fd())
	var ol syscall.Overlapped
	if err := lockFileEx(h, lockfileExclusiveLock|lockfileFailImmediately, &ol); err != nil {
		f.Close()
		if err == errLockViolation {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("board lock: %w", err)
	}
	return &fileLock{f: f, ol: ol, held: true}, true, nil
}

// release drops the exclusive lock and closes the fd.
func (l *fileLock) release() {
	if !l.held {
		return
	}
	h := syscall.Handle(l.f.Fd())
	unlockFileEx(h, &l.ol)
	l.held = false
	l.f.Close()
}
