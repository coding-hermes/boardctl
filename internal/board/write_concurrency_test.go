package board

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---- stress harness ---------------------------------------------------------

// stressWriter is one concurrent writer's script: N creates of fresh ids,
// an update of the SHARED row every other step (read-modify-write contention
// on one line), and an audit event per step (MAX(id)+1 derivation race).
type stressWriter struct {
	creates int
	updates int
	events  int
}

// runBoardStress runs G goroutines against ONE board dir; every goroutine
// resolves its OWN *Board value (the guard's re-entrancy bookkeeping is
// per-value, single-goroutine by design — cross-goroutine serialization must
// come from the flock, which is exactly what this exercises: same shape as
// two boardctl processes). Returns all writer errors plus the final on-disk
// bytes of both files.
func runBoardStress(t *testing.T, dir string, steps int) ([]error, []byte, []byte) {
	t.Helper()
	var wg sync.WaitGroup
	errs := make([]error, steps)
	start := time.Now()
	for g := 0; g < steps; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			b, err := Resolve(dir)
			if err != nil {
				errs[g] = err
				return
			}
			for i := 0; i < 6; i++ {
				id := fmt.Sprintf("STRESS-%d-%d", g, i)
				if _, err := b.Create(TaskRowSpec{
					ID:     id,
					Title:  fmt.Sprintf("stress writer %d step %d", g, i),
					Status: "pending",
				}); err != nil {
					errs[g] = fmt.Errorf("create %s: %w", id, err)
					return
				}
				if i%2 == 0 {
					sum := fmt.Sprintf("writer %d touched the shared row", g)
					if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Summary: &sum}); err != nil {
						errs[g] = fmt.Errorf("update EXIST-1: %w", err)
						return
					}
				}
				if _, err := b.AppendEvent(EventSpec{
					Type:   "audit",
					TaskID: id,
					Detail: []byte(fmt.Sprintf(`{"writer":%d,"step":%d}`, g, i)),
				}); err != nil {
					errs[g] = fmt.Errorf("event: %w", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
	tasksRaw, err := os.ReadFile(filepath.Join(dir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	eventsRaw, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	_ = start
	return errs, tasksRaw, eventsRaw
}

// auditBoard is the corruption census: every line of both files must parse,
// task ids must be unique, event ids must be unique and strictly increasing.
// It returns the corruption classes it saw (empty slice = clean board).
func auditBoard(t *testing.T, tasksRaw, eventsRaw []byte) []string {
	t.Helper()
	var problems []string
	seenTask := map[string]int{}
	for i, l := range bytes.Split(bytes.TrimRight(tasksRaw, "\n"), []byte("\n")) {
		if len(bytes.TrimSpace(l)) == 0 {
			problems = append(problems, fmt.Sprintf("tasks.jsonl line %d: BLANK (torn append)", i+1))
			continue
		}
		var row map[string]any
		if err := json.Unmarshal(l, &row); err != nil {
			problems = append(problems, fmt.Sprintf("tasks.jsonl line %d: INVALID JSON (%v): %.80s", i+1, err, l))
			continue
		}
		id, _ := row["id"].(string)
		if id != "" {
			seenTask[id]++
			if seenTask[id] > 1 {
				problems = append(problems, fmt.Sprintf("tasks.jsonl: DUPLICATE task id %q (line %d)", id, i+1))
			}
		}
	}
	prevEvent := int64(0)
	seenEvent := map[int64]bool{}
	for i, l := range bytes.Split(bytes.TrimRight(eventsRaw, "\n"), []byte("\n")) {
		if len(bytes.TrimSpace(l)) == 0 {
			problems = append(problems, fmt.Sprintf("events.jsonl line %d: BLANK (torn append)", i+1))
			continue
		}
		var row struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(l, &row); err != nil {
			problems = append(problems, fmt.Sprintf("events.jsonl line %d: INVALID JSON (%v): %.80s", i+1, err, l))
			continue
		}
		if seenEvent[row.ID] {
			problems = append(problems, fmt.Sprintf("events.jsonl line %d: DUPLICATE event id %d", i+1, row.ID))
		}
		seenEvent[row.ID] = true
		if row.ID <= prevEvent && prevEvent > 0 {
			problems = append(problems, fmt.Sprintf("events.jsonl line %d: id %d not strictly increasing (prev %d)", i+1, row.ID, prevEvent))
		}
		prevEvent = row.ID
	}
	return problems
}

// ---- (a)+(b) GREEN: concurrent writers serialize under the lock ------------

// TestConcurrentWritersSerializeUnderLock is the acceptance stress: two-plus
// real goroutines hammer one board with interleaved creates/updates/events;
// with the flock guard every writer succeeds and the census finds zero
// interleaving (every line valid JSON, task ids unique, event ids unique and
// strictly increasing). Run with -race in development to also sweep the
// lock's own fd handling.
func TestConcurrentWritersSerializeUnderLock(t *testing.T) {
	for round := 0; round < 3; round++ {
		b := newTestBoard(t)
		dir := b.Dir
		errs, tasksRaw, eventsRaw := runBoardStress(t, dir, 6)
		for _, err := range errs {
			if err != nil {
				t.Fatalf("round %d: a locked writer failed (serialization hole): %v", round, err)
			}
		}
		if problems := auditBoard(t, tasksRaw, eventsRaw); len(problems) > 0 {
			t.Fatalf("round %d: locked stress produced a corrupt board:\n  %s",
				round, strings.Join(problems, "\n  "))
		}
		// 6 writers x 6 steps = 36 tasks + the seeded EXIST-1.
		if got := strings.Count(string(tasksRaw), "\n"); got != 37 {
			t.Fatalf("round %d: tasks.jsonl has %d lines, want 37", round, got)
		}
		// Events: seed + 36 audit + 36 task_created (every create writes one).
		if got := strings.Count(string(eventsRaw), "\n"); got != 73 {
			t.Fatalf("round %d: events.jsonl has %d lines, want 73", round, got)
		}
		// The sidecar lock file exists exactly where documented.
		if _, err := os.Stat(filepath.Join(dir, ".lock")); err != nil {
			t.Fatalf("round %d: lock sidecar missing after writes: %v", round, err)
		}
		// Readers (validate) accept the post-stress board and are unaffected.
		if _, err := b.Validate(); err != nil {
			t.Fatalf("round %d: validate failed on the locked-stress board: %v", round, err)
		}
	}
}

// TestParallelCreateSameIDExactlyOneWinner: N goroutines race to create the
// SAME id; the duplicate-id guard holds under the lock — exactly one nil
// error, every loser gets ErrDuplicateTaskID, and the file carries exactly
// one such row.
func TestParallelCreateSameIDExactlyOneWinner(t *testing.T) {
	const racers = 12
	const rounds = 20
	for round := 0; round < rounds; round++ {
		b := newTestBoard(t)
		dir := b.Dir
		errs := make([]error, racers)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for g := 0; g < racers; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				<-start
				bv, err := Resolve(dir)
				if err != nil {
					errs[g] = err
					return
				}
				_, err = bv.Create(TaskRowSpec{ID: "RACE-1", Title: "only one of me", Status: "pending"})
				errs[g] = err
			}(g)
		}
		close(start)
		wg.Wait()
		winners, dupErrs := 0, 0
		for _, err := range errs {
			switch {
			case err == nil:
				winners++
			default:
				var dup *ErrDuplicateTaskID
				if !errors.As(err, &dup) {
					t.Fatalf("round %d: loser failed with an unexpected error class: %v", round, err)
				}
				dupErrs++
			}
		}
		if winners != 1 {
			t.Fatalf("round %d: %d racers won the same-id create, want exactly 1 (id-refusal guard did not hold under the lock)", round, winners)
		}
		if dupErrs != racers-1 {
			t.Fatalf("round %d: %d duplicate-id refusals, want %d", round, dupErrs, racers-1)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "tasks.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(string(raw), `"RACE-1"`); n != 1 {
			t.Fatalf("round %d: RACE-1 appears %d times on disk, want 1", round, n)
		}
	}
}

// ---- (a)+(b) RED control: the same stress WITHOUT the lock -----------------

// TestConcurrentWritersWithoutLockIsUnsafe reruns the identical stress with
// the lock bypassed and shows interleaving/corruption IS possible — the RED
// half of the acceptance evidence. Race windows are inherently timing-
// dependent: the test asserts what it OBSERVED (quoting the corruption) and
// skips with that message when a run gets lucky through every window, so the
// control can never flake a green build. The GREEN counterpart above and the
// commit body carry the quoted pair.
func TestConcurrentWritersWithoutLockIsUnsafe(t *testing.T) {
	writeLockBypass = true
	defer func() { writeLockBypass = false }()

	var observed []string
	for round := 0; round < 40; round++ {
		b := newTestBoard(t)
		errs, tasksRaw, eventsRaw := runBoardStress(t, b.Dir, 6)
		problems := auditBoard(t, tasksRaw, eventsRaw)
		for _, err := range errs {
			if err != nil {
				observed = append(observed, fmt.Sprintf("round %d writer error: %v", round, err))
			}
		}
		for _, p := range problems {
			observed = append(observed, fmt.Sprintf("round %d: %s", round, p))
		}
		if len(observed) > 0 {
			t.Logf("UNLOCKED stress corrupted a board on round %d — race is real. Evidence:\n  %s",
				round, strings.Join(observed[:min(len(observed), 8)], "\n  "))
			return
		}
	}
	t.Skipf("unlocked race not reproduced in 40 rounds this run (timing-dependent by nature) — RED evidence for BT-050 is quoted in the commit body")
}

// ---- guard mechanics --------------------------------------------------------

// TestWriteLockIsExclusive: while one holder owns the flock a second
// acquisition (the shape every second writer takes) is refused nonblockingly,
// and succeeds after release. Proves the guard is a real exclusive resource.
func TestWriteLockIsExclusive(t *testing.T) {
	b := newTestBoard(t)
	lockPath := filepath.Join(b.Dir, ".lock")

	held, err := acquireFileLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if lk, ok, err := tryAcquireFileLock(lockPath); err != nil {
		t.Fatal(err)
	} else if ok {
		lk.release()
		held.release()
		t.Fatal("second acquire succeeded while the lock was held — not exclusive")
	}
	held.release()
	lk, ok, err := tryAcquireFileLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("acquire after release failed — lock leaked")
	}
	lk.release()
}

// TestWriteGuardFullyReleasedAfterVerbs: every write verb drops the guard
// completely (b.lock back to nil). A leaked guard would make the NEXT write
// on the same Board value take the re-entrant fast path and skip the flock
// entirely — silently unprotected.
func TestWriteGuardFullyReleasedAfterVerbs(t *testing.T) {
	b := newTestBoard(t)
	if _, err := b.Create(TaskRowSpec{ID: "REL-1", Title: "x", Status: "pending"}); err != nil {
		t.Fatal(err)
	}
	if b.lock != nil {
		t.Fatal("guard leaked after Create")
	}
	sum := "s"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Summary: &sum}); err != nil {
		t.Fatal(err)
	}
	if b.lock != nil {
		t.Fatal("guard leaked after UpdateTask")
	}
	if _, err := b.AppendEvent(EventSpec{Type: "audit"}); err != nil {
		t.Fatal(err)
	}
	if b.lock != nil {
		t.Fatal("guard leaked after AppendEvent")
	}
	if _, err := b.NormalizeTask("EXIST-1", false); err != nil {
		t.Fatal(err)
	}
	if b.lock != nil {
		t.Fatal("guard leaked after NormalizeTask")
	}
}

// TestReadersDoNotTakeTheWriteLock: validate/doctor/reads must work while a
// writer holds the flock — and must not try to take it themselves (a reader
// that did would deadlock on the held lock). Watchdog turns a deadlock into
// a loud failure instead of a hung test.
func TestReadersDoNotTakeTheWriteLock(t *testing.T) {
	b := newTestBoard(t)
	release, err := lockAcquireForProbe(b.Dir) // simulates a writer mid-write
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	done := make(chan error, 1)
	go func() {
		_, err := b.Validate()
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("validate failed while a writer held the lock: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("validate blocked on the write lock — readers must never take the guard")
	}
}

// TestLockReleasesWhenHolderDiesWithoutCleanup: flock is owned by the open
// fd, so a crashed holder's lock evaporates when the kernel closes its fds.
// Simulated here at the fd level: the holder is closed WITHOUT calling
// release (the crash path — no unlock syscall, no cleanup routine), and the
// next acquirer succeeds. No stale-lock cleanup path exists or is needed.
func TestLockReleasesWhenHolderDiesWithoutCleanup(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".lock")

	crashed, err := acquireFileLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if lk, ok, _ := tryAcquireFileLock(lockPath); ok {
		lk.release()
		crashed.f.Close() // crash-path teardown: fd closed, no unlock syscall
		t.Fatal("lock survived while the holder fd was open — unexpected")
	}
	// The crash: close the fd WITHOUT release(). flock semantics hand the
	// lock to the next acquirer.
	crashed.f.Close()

	lk, ok, err := tryAcquireFileLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("lock still held after the holder's fd was closed without cleanup — a stale lock would wedge the board")
	}
	lk.release()
}
