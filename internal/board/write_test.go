package board

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// boardLineCount counts non-empty lines of a board file.
func boardLineCount(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			n++
		}
	}
	return n
}

// lastTaskRaw returns the last non-empty tasks.jsonl line parsed as a map.
func lastTaskRaw(t *testing.T, b *Board) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	var last []byte
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			last = []byte(l)
		}
	}
	var row map[string]any
	if err := json.Unmarshal(last, &row); err != nil {
		t.Fatal(err)
	}
	return row
}

// BT-007: update --guard MAYBE must be REJECTED — only PASS|FAIL|SKIP are in
// the guard_result vocabulary — and nothing may be written.
func TestUpdateGuardRejectsOutOfVocab(t *testing.T) {
	b := newTestBoard(t)
	maybe := "MAYBE"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Guard: &maybe}); err == nil {
		t.Fatal("update --guard MAYBE accepted")
	} else if !strings.Contains(err.Error(), "guard") || !strings.Contains(err.Error(), "PASS,FAIL,SKIP") {
		t.Fatalf("error should name the guard vocabulary, got: %v", err)
	}
	// the row must be untouched (single line, original bytes)
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if n := boardLineCount(t, b.tasksPath); n != 1 {
		t.Fatalf("tasks.jsonl has %d lines after rejected update, want 1", n)
	}
	if strings.Contains(string(raw), "MAYBE") {
		t.Fatalf("rejected value leaked into tasks.jsonl: %s", raw)
	}
}

// BT-007: guard is normalized (trimmed + upper-cased) before the vocabulary
// check, so "pass" stores PASS — case tolerance on input, canonical on disk.
func TestUpdateGuardNormalizesCase(t *testing.T) {
	b := newTestBoard(t)
	pass := "pass"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Guard: &pass}); err != nil {
		t.Fatalf("update --guard pass rejected: %v", err)
	}
	row := lastTaskRaw(t, b)
	if row["guard_result"] != "PASS" {
		t.Fatalf("guard_result = %v, want PASS", row["guard_result"])
	}
}

// BT-007: update --ci BANANA must be rejected; GREEN|RED|SKIP are accepted.
func TestUpdateCIRejectsOutOfVocab(t *testing.T) {
	b := newTestBoard(t)
	banana := "BANANA"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{CI: &banana}); err == nil {
		t.Fatal("update --ci BANANA accepted")
	} else if !strings.Contains(err.Error(), "GREEN,RED,SKIP") {
		t.Fatalf("error should name the ci vocabulary, got: %v", err)
	}
	green := "green"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{CI: &green}); err != nil {
		t.Fatalf("update --ci green rejected: %v", err)
	}
	if row := lastTaskRaw(t, b); row["ci_result"] != "GREEN" {
		t.Fatalf("ci_result = %v, want GREEN", row["ci_result"])
	}
}

// BT-007: create --priority 1 is NORMALIZED to P1 (fleet boards use P0-P3;
// bare digits must not fork the stats grouping).
func TestCreatePriorityNormalizesBareDigits(t *testing.T) {
	b := newTestBoard(t)
	one := "1"
	if _, err := b.Create(TaskRowSpec{ID: "NEW-1", Title: "n1", Priority: one}); err != nil {
		t.Fatalf("create --priority 1 rejected: %v", err)
	}
	if row := lastTaskRaw(t, b); row["priority"] != "P1" {
		t.Fatalf("priority = %v, want P1 (normalized from %q)", row["priority"], one)
	}
	// case variant normalizes too
	if _, err := b.Create(TaskRowSpec{ID: "NEW-2", Title: "n2", Priority: "p3"}); err != nil {
		t.Fatalf("create --priority p3 rejected: %v", err)
	}
	if row := lastTaskRaw(t, b); row["priority"] != "P3" {
		t.Fatalf("priority = %v, want P3", row["priority"])
	}
}

// BT-007: a priority outside {P0,P1,P2,P3} (after normalization) is rejected.
func TestCreatePriorityRejectsGarbage(t *testing.T) {
	b := newTestBoard(t)
	before := boardLineCount(t, b.tasksPath)
	if _, err := b.Create(TaskRowSpec{ID: "NEW-1", Title: "n1", Priority: "banana"}); err == nil {
		t.Fatal("create --priority banana accepted")
	} else if !strings.Contains(err.Error(), "P0,P1,P2,P3") {
		t.Fatalf("error should name the priority vocabulary, got: %v", err)
	}
	if n := boardLineCount(t, b.tasksPath); n != before {
		t.Fatalf("tasks.jsonl grew from %d to %d lines on rejected create", before, n)
	}
}

// BT-007: create --depends-on GHOST-9 is rejected — the dependency must exist
// in tasks.jsonl — and NOTHING is written (no task row, no task_created
// event) because the append-only store cannot repair a dangling ref later.
func TestCreateDependsOnGhostRejected(t *testing.T) {
	b := newTestBoard(t)
	tasksBefore := boardLineCount(t, b.tasksPath)
	eventsBefore := boardLineCount(t, b.eventsPath)
	_, err := b.Create(TaskRowSpec{
		ID: "NEW-1", Title: "n1", Priority: "P2",
		HasDependsOn: true, DependsOn: []string{"GHOST-9"},
	})
	if err == nil {
		t.Fatal("create --depends-on GHOST-9 accepted")
	}
	if !strings.Contains(err.Error(), "GHOST-9") {
		t.Fatalf("error should name the missing id, got: %v", err)
	}
	if n := boardLineCount(t, b.tasksPath); n != tasksBefore {
		t.Fatalf("tasks.jsonl grew from %d to %d lines on rejected create", tasksBefore, n)
	}
	if n := boardLineCount(t, b.eventsPath); n != eventsBefore {
		t.Fatalf("events.jsonl grew from %d to %d lines on rejected create (no task_created event may fire)", eventsBefore, n)
	}
}

// BT-007: a depends_on id that DOES exist is accepted and stored.
func TestCreateDependsOnExistingAccepted(t *testing.T) {
	b := newTestBoard(t)
	if _, err := b.Create(TaskRowSpec{
		ID: "NEW-1", Title: "n1", Priority: "P2",
		HasDependsOn: true, DependsOn: []string{"EXIST-1"},
	}); err != nil {
		t.Fatalf("create with existing dependency rejected: %v", err)
	}
	if row := lastTaskRaw(t, b); row["depends_on"] == nil {
		t.Fatalf("depends_on not stored: %v", row)
	}
}

// BT-007: event --type outside the vocabulary is rejected at append time.
func TestAppendEventRejectsUnknownType(t *testing.T) {
	b := newTestBoard(t)
	if _, err := b.AppendEvent(EventSpec{Type: "banana"}); err == nil {
		t.Fatal("event --type banana accepted")
	} else if !strings.Contains(err.Error(), "event type") {
		t.Fatalf("error should name the event type vocabulary, got: %v", err)
	}
	// empty type still defaults to audit; every enumerated type is accepted
	for _, et := range []string{"", "audit", "task_created", "task_dispatched",
		"task_completed", "task_updated", "tick", "idle", "board_init"} {
		if _, err := b.AppendEvent(EventSpec{Type: et}); err != nil {
			t.Fatalf("event type %q rejected: %v", et, err)
		}
	}
}

// BT-007: header --set-ticks-total/-idle reject negative counters at write
// time, leaving the header untouched.
func TestSetHeaderRejectsNegativeCounters(t *testing.T) {
	b := newTestBoard(t)
	before, err := os.ReadFile(b.headerPath)
	if err != nil {
		t.Fatal(err)
	}
	neg := int64(-5)
	if _, err := b.SetHeader(HeaderUpdate{TicksTotal: &neg}); err == nil {
		t.Fatal("header --set-ticks-total -5 accepted")
	} else if !strings.Contains(err.Error(), "negative") {
		t.Fatalf("error should say negative, got: %v", err)
	}
	if _, err := b.SetHeader(HeaderUpdate{TicksIdle: &neg}); err == nil {
		t.Fatal("header --set-ticks-idle -5 accepted")
	}
	after, err := os.ReadFile(b.headerPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("rejected header update mutated board.jsonl:\nbefore %s\nafter  %s", before, after)
	}
	// zero stays legal (a fresh board's counters are 0)
	zero := int64(0)
	if _, err := b.SetHeader(HeaderUpdate{TicksTotal: &zero, TicksIdle: &zero}); err != nil {
		t.Fatalf("zero counters rejected: %v", err)
	}
}
