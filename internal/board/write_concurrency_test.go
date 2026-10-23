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

// ---- (c) rewrite verbs serialize against appends (DF-BOARDCTL-48/49) -------

// probeTaskID is the id the concurrent appender creates. It is a valid fleet id
// (PREFIX-SEGMENT-N) and deliberately absent from every fixture.
const probeTaskID = "PROBE-MID-1"

// probeEventMarker is the detail marker of the event the concurrent appender
// writes in the --repair-events case. The append path stores detail as an
// escaped JSON STRING value, so the census counts the bare marker (the quoting
// around it differs between a freshly appended row and a renumbered one).
const probeEventMarker = `PROBE-EV-1`

// rewriteAppendCase is one rewrite verb under test plus the concurrent append it
// must not swallow: the fixture (which arms genuine work for the verb, so it
// reaches its write point), the verb call, the append issued on its OWN Board
// value (the shape two boardctl processes take), and the census that names the
// damage when the append was overwritten.
type rewriteAppendCase struct {
	verb    string // tag the rewrite-window seam must report for this verb
	fixture func(t *testing.T) *Board
	rewrite func(b *Board) error
	append  func(b *Board) error
	inspect func(t *testing.T, b *Board) []string
}

// rewriteAppendInterleave runs ONE rewrite verb and ONE real append through a
// deterministic rendezvous at the verb's read/classify→write boundary:
//
//   - the verb runs on this goroutine; the rewrite-window seam fires immediately
//     after it has classified its snapshot and immediately before it writes;
//   - a second goroutine then probes the board guard NONBLOCKING. blocked=true
//     means the verb already holds the guard at that point — the whole
//     read/classify/check/write window is one critical section, so the append
//     queues behind it and lands after the rewrite;
//   - blocked=false means the window is unguarded: the append lands INSIDE it,
//     and the harness waits for it to complete before letting the rewrite write,
//     so the clobber is deterministic instead of a timing race.
//
// No sleeps: the only timers are hang guards on a rendezvous that must complete.
func rewriteAppendInterleave(t *testing.T, dir, verb string, rewrite func() error, appendFn func() error) (blocked bool, hookVerb string, appendErr, rewriteErr error) {
	t.Helper()
	probe := make(chan bool, 1)
	appendDone := make(chan error, 1)
	fired := false

	rewriteWindowHook = func(name string) {
		if fired {
			return // one rendezvous per run
		}
		fired = true
		hookVerb = name
		if name != verb {
			return // a different verb fired the seam; the caller reports it
		}
		go func() {
			// "Is somebody holding the guard right now?" — the verb is the only
			// candidate holder on this board dir, so a failed nonblocking
			// acquire is exactly "the verb classified its snapshot locked".
			held := false
			if lk, ok, err := tryAcquireFileLock(filepath.Join(dir, boardLockFileName)); err == nil && ok {
				lk.release()
			} else if err == nil {
				held = true
			}
			probe <- held
			appendDone <- appendFn()
		}()
		blocked = <-probe
		if blocked {
			return // queued behind the guard: the append lands after the write
		}
		// Unguarded window: wait for the append to land in it, so the rewrite's
		// write is guaranteed to happen AFTER the append.
		select {
		case appendErr = <-appendDone:
		case <-time.After(30 * time.Second):
			t.Errorf("%s: the concurrent append never finished in the unlocked window", verb)
		}
	}
	defer func() { rewriteWindowHook = nil }()

	rewriteErr = rewrite()
	if blocked {
		select {
		case appendErr = <-appendDone:
		case <-time.After(30 * time.Second):
			t.Errorf("%s: the append queued behind the guard never completed", verb)
		}
	}
	return blocked, hookVerb, appendErr, rewriteErr
}

// rewriteAppendCases builds the three rewrite verbs the DF-BOARDCTL-48/49 rows
// name, each with a fixture carrying real work for that verb.
func rewriteAppendCases() []rewriteAppendCase {
	return []rewriteAppendCase{
		{
			verb: "normalize-board",
			fixture: func(t *testing.T) *Board {
				return seedFindingBoard(t,
					`{"id":"DRIFT-1","title":"alias status + empty-string deps","status":"open","priority":"P2","depends_on":""}`,
					`{"id":"DRIFT-2","title":"done alias","status":"done","priority":"P1"}`,
					`{"id":"CLEAN-1","title":"already canonical","status":"pending","priority":"P2","depends_on":[],"created_at":"2026-10-01 00:00:00"}`,
				)
			},
			rewrite: func(b *Board) error {
				_, err := b.NormalizeBoard(true)
				return err
			},
			append: func(b *Board) error {
				_, err := b.Create(TaskRowSpec{ID: probeTaskID, Title: "created while normalize-board ran", Status: "pending"})
				return err
			},
			inspect: func(t *testing.T, b *Board) []string { return probeTaskProblems(t, b, probeTaskID) },
		},
		{
			verb:    "repair-events",
			fixture: seedRewriteEventsBoard,
			rewrite: func(b *Board) error {
				_, err := b.RepairEvents(false)
				return err
			},
			append: func(b *Board) error {
				_, err := b.AppendEvent(EventSpec{
					Type:   "audit",
					Actor:  "probe",
					Detail: []byte(`{"probe":"` + probeEventMarker + `"}`),
				})
				return err
			},
			inspect: probeEventProblems,
		},
		{
			verb: "dedupe-by-id",
			fixture: func(t *testing.T) *Board {
				return seedFindingBoard(t,
					`{"id":"QA-RECYCLE-1","title":"first finding","reasoning":"cell one","status":"complete","priority":"P2"}`,
					`{"id":"QA-RECYCLE-1","title":"second finding","reasoning":"cell two","status":"pending","priority":"P2"}`,
				)
			},
			rewrite: func(b *Board) error {
				_, err := b.DedupeByID(true)
				return err
			},
			append: func(b *Board) error {
				_, err := b.Create(TaskRowSpec{ID: probeTaskID, Title: "created while dedupe-board ran", Status: "pending"})
				return err
			},
			inspect: func(t *testing.T, b *Board) []string { return probeTaskProblems(t, b, probeTaskID) },
		},
	}
}

// seedRewriteEventsBoard seeds events.jsonl with descending ids (ids 1,2,5,3,4 —
// four renumberings of real work) over a topology-A board.
func seedRewriteEventsBoard(t *testing.T) *Board {
	t.Helper()
	b := seedFindingBoard(t, `{"id":"CLEAN-1","title":"canonical","status":"pending","priority":"P2","depends_on":[]}`)
	events := `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n" +
		`{"id":2,"timestamp":"2026-09-03 00:00:01.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n" +
		`{"id":5,"timestamp":"2026-09-03 00:00:02.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n" +
		`{"id":3,"timestamp":"2026-09-03 00:00:03.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n" +
		`{"id":4,"timestamp":"2026-09-03 00:00:04.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n"
	if err := os.WriteFile(b.eventsPath, []byte(events), 0o644); err != nil {
		t.Fatal(err)
	}
	return b
}

// probeTaskProblems is the damage census for a concurrent CREATE: the row must
// still be on the board (AC1) and the event log must not carry an orphan
// task_created for a row the rewrite dropped (AC3).
func probeTaskProblems(t *testing.T, b *Board, id string) []string {
	t.Helper()
	tasksRaw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	eventsRaw, err := os.ReadFile(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	rowPresent := false
	for _, l := range bytes.Split(tasksRaw, []byte("\n")) {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal(l, &row); err != nil {
			return []string{fmt.Sprintf("tasks.jsonl holds an unparseable line (torn rewrite?): %v: %.90s", err, l)}
		}
		if row["id"] == id {
			rowPresent = true
		}
	}
	createdEvents := 0
	for _, l := range bytes.Split(eventsRaw, []byte("\n")) {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		var ev struct {
			Type   string `json:"event_type"`
			TaskID string `json:"task_id"`
		}
		if err := json.Unmarshal(l, &ev); err != nil {
			return []string{fmt.Sprintf("events.jsonl holds an unparseable line (torn rewrite?): %v: %.90s", err, l)}
		}
		if ev.TaskID == id && ev.Type == "task_created" {
			createdEvents++
		}
	}
	var problems []string
	if !rowPresent {
		problems = append(problems, fmt.Sprintf("the task created while the rewrite ran (%s) is GONE from tasks.jsonl — the rewrite overwrote a completed append", id))
	}
	if createdEvents != 1 {
		problems = append(problems, fmt.Sprintf("task_created events for %s: %d, want exactly 1", id, createdEvents))
	} else if !rowPresent {
		problems = append(problems, fmt.Sprintf("ORPHAN task_created event: events.jsonl carries an event for %s while tasks.jsonl carries no such row", id))
	}
	return problems
}

// probeEventProblems is the damage census for a concurrent EVENT append: the
// appended record must survive the repair, and the repaired file must still be
// a valid event log (every line parses, ids ascending — the repair's own job).
func probeEventProblems(t *testing.T, b *Board) []string {
	t.Helper()
	raw, err := os.ReadFile(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	var problems []string
	if n := strings.Count(string(raw), probeEventMarker); n != 1 {
		problems = append(problems, fmt.Sprintf("the event appended while the repair ran is gone: %d record(s) carrying %s in events.jsonl, want 1 — the repair overwrote a completed append", n, probeEventMarker))
	}
	prev := int64(0)
	for i, l := range bytes.Split(bytes.TrimRight(raw, "\n"), []byte("\n")) {
		if len(bytes.TrimSpace(l)) == 0 {
			problems = append(problems, fmt.Sprintf("events.jsonl line %d is blank (torn rewrite)", i+1))
			continue
		}
		var ev struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(l, &ev); err != nil {
			problems = append(problems, fmt.Sprintf("events.jsonl line %d does not parse (torn rewrite): %v", i+1, err))
			continue
		}
		if i > 0 && ev.ID <= prev {
			problems = append(problems, fmt.Sprintf("events.jsonl line %d: id %d not ascending after the repair (prev %d)", i+1, ev.ID, prev))
		}
		prev = ev.ID
	}
	return problems
}

// TestRewriteVerbsSerializeAgainstConcurrentAppend is the DF-BOARDCTL-48/49
// acceptance: for each rewrite verb, an append issued at the verb's
// read/classify→write boundary survives the rewrite (AC1) and leaves no orphan
// task_created event (AC3). The append must find the guard ALREADY HELD at that
// boundary — that is the whole-window lock the README documents, and the bug was
// that the boundary sat outside it.
func TestRewriteVerbsSerializeAgainstConcurrentAppend(t *testing.T) {
	for _, c := range rewriteAppendCases() {
		c := c
		t.Run(c.verb, func(t *testing.T) {
			b := c.fixture(t)
			appender, err := Resolve(b.Dir)
			if err != nil {
				t.Fatal(err)
			}
			blocked, hookVerb, appendErr, rewriteErr := rewriteAppendInterleave(t, b.Dir, c.verb,
				func() error { return c.rewrite(b) },
				func() error { return c.append(appender) })
			if hookVerb != c.verb {
				t.Fatalf("the rewrite-window seam never fired for %s (reported %q): the verb never reached its write point, so this test measured nothing", c.verb, hookVerb)
			}
			if rewriteErr != nil {
				t.Fatalf("%s: rewrite failed: %v", c.verb, rewriteErr)
			}
			if appendErr != nil {
				t.Fatalf("%s: the concurrent append failed: %v", c.verb, appendErr)
			}
			if problems := c.inspect(t, b); len(problems) > 0 {
				t.Fatalf("%s: the rewrite damaged the concurrent append:\n  %s", c.verb, strings.Join(problems, "\n  "))
			}
			if !blocked {
				t.Fatalf("%s: the concurrent append found the board guard FREE at the verb's read/classify→write boundary — that window is outside the lock, so an append lands inside it and the rewrite's stale snapshot overwrites it", c.verb)
			}
			// The guard must still be fully released after the verb.
			if b.lock != nil {
				t.Fatalf("%s: write guard leaked after the rewrite", c.verb)
			}
		})
	}
}

// TestRewriteVerbsDropConcurrentAppendWithoutLock is the RED half of AC2: the
// identical rendezvous with the guard bypassed (the BT-050 RED seam). The append
// now completes inside the unguarded window, and every rewrite overwrites it —
// the exact dogfood-run-25 failure, reproduced deterministically.
func TestRewriteVerbsDropConcurrentAppendWithoutLock(t *testing.T) {
	writeLockBypass = true
	defer func() { writeLockBypass = false }()

	for _, c := range rewriteAppendCases() {
		c := c
		t.Run(c.verb, func(t *testing.T) {
			b := c.fixture(t)
			appender, err := Resolve(b.Dir)
			if err != nil {
				t.Fatal(err)
			}
			blocked, hookVerb, appendErr, rewriteErr := rewriteAppendInterleave(t, b.Dir, c.verb,
				func() error { return c.rewrite(b) },
				func() error { return c.append(appender) })
			if hookVerb != c.verb {
				t.Fatalf("the rewrite-window seam never fired for %s (reported %q)", c.verb, hookVerb)
			}
			if rewriteErr != nil {
				t.Fatalf("%s: rewrite failed: %v", c.verb, rewriteErr)
			}
			if appendErr != nil {
				t.Fatalf("%s: the concurrent append failed: %v", c.verb, appendErr)
			}
			if blocked {
				t.Fatalf("%s: the guard was observed HELD with writeLockBypass on — the RED control is broken", c.verb)
			}
			problems := c.inspect(t, b)
			if len(problems) == 0 {
				t.Fatalf("%s: bypassing the guard did NOT lose the concurrent append — this control no longer demonstrates the rewrite race", c.verb)
			}
			t.Logf("RED %s (guard bypassed): %s", c.verb, strings.Join(problems, "; "))
		})
	}
}

// TestRepairEventsReplacesEventsFileAtomically proves the --repair-events write
// is a same-directory temp file + rename rather than an in-place os.WriteFile
// (DF-BOARDCTL-49 secondary finding):
//
//  1. the repaired file is a DIFFERENT file than the one before the run (a
//     rename installs a new inode; an in-place write keeps the old one), and no
//     .boardctl-rewrite-* temp file is left behind;
//  2. when the directory refuses a new temp file the run FAILS and events.jsonl
//     keeps its previous bytes — an in-place write would have truncated the
//     audit trail first and only then failed (or silently succeeded).
func TestRepairEventsReplacesEventsFileAtomically(t *testing.T) {
	b := seedRewriteEventsBoard(t)
	before, err := os.Stat(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.RepairEvents(false); err != nil {
		t.Fatalf("RepairEvents(apply): %v", err)
	}
	after, err := os.Stat(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Fatal("events.jsonl was rewritten IN PLACE (same file) — the repair must replace it with a temp file + rename so a crash cannot truncate the audit trail")
	}
	entries, err := os.ReadDir(b.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".boardctl-rewrite-") {
			t.Fatalf("atomic rewrite left temp file %q behind", e.Name())
		}
	}

	// Arm 2: the directory cannot accept the temp file.
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions cannot refuse the temp file")
	}
	b2 := seedRewriteEventsBoard(t)
	// The guard's sidecar must be openable inside the read-only dir, or the
	// failure would be "cannot take the lock" instead of "cannot replace the
	// file atomically" — pre-create it while the dir is still writable.
	if err := os.WriteFile(filepath.Join(b2.Dir, boardLockFileName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	pre, err := os.ReadFile(b2.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(b2.Dir, 0o555); err != nil {
		t.Fatal(err)
	}
	_, repairErr := b2.RepairEvents(false)
	if err := os.Chmod(b2.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if repairErr == nil {
		t.Fatal("the repair reported success even though the directory refused a temp file — the write is not atomic (an in-place os.WriteFile truncates the audit trail instead of leaving it intact)")
	}
	post, err := os.ReadFile(b2.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pre, post) {
		t.Fatalf("events.jsonl changed even though the atomic replacement failed (%v) — the audit trail is not protected", repairErr)
	}
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
