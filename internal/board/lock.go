package board

import (
	"path/filepath"
	"runtime"
)

// BT-050: file-lock-guarded board writes.
//
// Every append and rewrite in this package goes through one of the verbs
// below (Create, UpdateTask, NormalizeTask, AppendEvent, SetHeader). Without
// a lock two concurrent writers race: both read the same file, both pass the
// duplicate-id scan, and both append — the interleave depends on where each
// writer's O_APPEND offset lands, so the loser's append can tear the winner's
// line in half (observed live as hermes-canopy tick 557's two DF-HERMES-CANOPY-52
// rows, and as the 'duplicate task id "X" (also line N)' validator refusal).
// The lock also protects read-modify-write rewrites (update/normalize/header):
// both writers rewriting from the same snapshot would silently drop the
// winner's change (lost update) even though each rewrite alone is atomic.
//
// The lock is an advisory flock (LOCK_EX) on a dedicated sidecar file,
// <board dir>/.lock — never on tasks.jsonl/events.jsonl themselves, so
// readers (list, validate, doctor, jq) never block and never need the lock.
// flock semantics release the lock when the holding process dies — crashed
// or killed writers leave no stale lock and there is deliberately no
// cleanup path. Advisory means it serializes only writers that acquire it:
// raw `>>`/jq rewrites from outside boardctl stay unprotected (that bypass
// is what this file documents against — see the write-path docs note).
//
// All internal writers (create/update/normalize/event/header) acquire the
// lock for the WHOLE check-then-act window — duplicate-id scan through
// final append/rewrite — so the check and the write are one critical
// section. Nesting is handled with an in-process depth counter: Create's
// internal AppendEvent (and AppendEvent's header bump, and UpdateTask's
// audit event) re-enter the guard on the same OS thread; the outermost
// caller holds the flock, inner acquisitions are counted no-ops. A depth
// counter is safe here because every lock user is a synchronous method on
// *Board — no writer path spawns a goroutine between acquire and release.
//
// Cross-process (two boardctl processes) and cross-goroutine (two Board
// values, one dir) writers are both serialized: cross-process by the flock,
// cross-goroutine by ensuring the exclusive flock is held only by the
// goroutine inside the critical section — a second goroutine's acquire
// attempts the same flock on its own fd and blocks until the first releases
// (see lock_unix.go / lock_windows.go: the blocking acquire is a plain
// syscall, and Go's syscall blocks its OS thread without blocking the
// scheduler, so concurrent writers queue correctly).

// boardLockFileName is the sidecar lock file inside the board dir. It is
// data, not board content: validate/doctor never enumerate the directory,
// so its presence cannot trip a read path.
const boardLockFileName = ".lock"

// boardLockDir returns the directory whose .lock serializes writes to path.
// All writers lock the directory that contains the file being written; for
// every tracked board file that is the board dir itself, so one lock file
// covers tasks.jsonl, events.jsonl, board.jsonl and fixtures.jsonl.
func boardLockDir(path string) string {
	return filepath.Dir(path)
}

// boardLock holds one acquired guard: the depth for re-entrant acquisitions
// on this value, plus the fd-level lock carrying the actual flock.
type boardLock struct {
	inner *fileLock
	depth int
}

// lockBoardDir acquires the exclusive write guard for dir. The bool result
// distinguishes a fresh acquisition from a re-entrant no-op count.
func (b *Board) lockBoardDir(dir string) (*boardLock, error) {
	if b.lock != nil {
		// Re-entrant (Create -> AppendEvent -> bumpHeaderTicksTotal ->
		// SetHeader): the OS-thread-held flock already covers us.
		b.lock.depth++
		return b.lock, nil
	}
	inner, err := acquireFileLock(filepath.Join(dir, boardLockFileName))
	if err != nil {
		return nil, err
	}
	l := &boardLock{inner: inner, depth: 1}
	b.lock = l
	return l, nil
}

// release drops one depth level; the flock itself is released only by the
// outermost caller.
func (l *boardLock) release(b *Board) {
	l.depth--
	if l.depth > 0 {
		return
	}
	l.inner.release()
	if b.lock == l {
		b.lock = nil
	}
}

// writeLockBypass disables the flock guard. It exists ONLY for the RED
// control in write_concurrency_test.go, which proves the concurrency stress
// can actually observe an unserialized board; production code never sets it.
var writeLockBypass = false

// withWriteLock runs fn while holding the board's write guard. Every
// check-then-act write verb in this package wraps its whole body (validation
// through final write) in this helper, so the guard's verdict and the write
// land in one critical section.
func (b *Board) withWriteLock(path string, fn func() error) error {
	// writeLockBypass is the RED-control switch used ONLY by
	// write_concurrency_test.go to demonstrate the unprotected race.
	if writeLockBypass {
		return fn()
	}
	lk, err := b.lockBoardDir(boardLockDir(path))
	if err != nil {
		return err
	}
	defer lk.release(b)
	return fn()
}

// lockDirForTests is the test seam: it reports where the lock file for a
// board's write guard lives (relative to Dir), so concurrency tests can
// assert the sidecar file exists and that bypassing the guard reintroduces
// the race.
func (b *Board) lockDirForTests() string { return b.Dir }

// lockAcquireForProbe is a test-only helper that proves two independent
// Board values on the same dir contend (used by the concurrency tests to
// show the lock is a real exclusive resource, not a formality). It acquires
// and immediately releases; the nonblocking probe in the caller decides
// pass/fail.
func lockAcquireForProbe(dir string) (release func(), err error) {
	lk, err := acquireFileLock(filepath.Join(dir, boardLockFileName))
	if err != nil {
		return nil, err
	}
	return lk.release, nil
}

// runtimeLockedOS reports the locking primitive in use — for the docs note
// and diagnostics ("flock(2)" on unix, "LockFileEx" on windows).
func runtimeLockedOS() string { return runtime.GOOS }
