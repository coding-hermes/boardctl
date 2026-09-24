package board

import (
	"strings"
	"testing"
)

// REVIEW-BOARDCTL-001: the write vocabulary is enforced on EVERY write surface
// (create/update — write.go rejects with "not in write vocabulary"), but the
// notorious fleet offenders had no table-driven proof at the library level.
// duplicate/parked/resolved/retired are statuses real boards drifted into; the
// aliases (done/open/completed/todo) must ALSO be refused on the write side —
// reads accept them, writes never do.
func TestWriteRejectsOffVocabularyStatus(t *testing.T) {
	for _, st := range []string{"duplicate", "parked", "resolved", "retired", "done", "open", "completed", "todo"} {
		// create path: rejected, nothing appended, no event fired. The
		// "completed" alias has its own dedicated refusal message (write.go
		// names the singular 'complete'); everything else hits the generic
		// write-vocabulary rejection. Both are refusals — nothing is written.
		b := newTestBoard(t)
		tasksBefore := boardLineCount(t, b.tasksPath)
		eventsBefore := boardLineCount(t, b.eventsPath)
		if _, err := b.Create(TaskRowSpec{ID: "NEW-1", Title: "n", Status: st}); err == nil {
			t.Errorf("create --status %q accepted", st)
		} else if !strings.Contains(err.Error(), "not in write vocabulary") && !strings.Contains(err.Error(), "is not allowed") {
			t.Errorf("create --status %q error should refuse the spelling, got: %v", st, err)
		}
		if n := boardLineCount(t, b.tasksPath); n != tasksBefore {
			t.Errorf("tasks.jsonl grew from %d to %d lines on rejected create --status %q", tasksBefore, n, st)
		}
		if n := boardLineCount(t, b.eventsPath); n != eventsBefore {
			t.Errorf("events.jsonl grew on rejected create --status %q (no task_created may fire)", st)
		}

		// update path: rejected, the target row untouched
		b2 := newTestBoard(t)
		before, err := ReadJSONLLines(b2.tasksPath)
		if err != nil {
			t.Fatal(err)
		}
		s := st
		if _, err := b2.UpdateTask("EXIST-1", UpdateSpec{Status: &s}); err == nil {
			t.Errorf("update --status %q accepted", st)
		} else if !strings.Contains(err.Error(), "not in write vocabulary") && !strings.Contains(err.Error(), "is not allowed") {
			t.Errorf("update --status %q error should refuse the spelling, got: %v", st, err)
		}
		after, err := ReadJSONLLines(b2.tasksPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(before) != len(after) {
			t.Fatalf("tasks.jsonl line count changed on rejected update --status %q", st)
		}
		for i := range before {
			if string(before[i]) != string(after[i]) {
				t.Errorf("rejected update --status %q rewrote line %d:\n before %s\n after  %s", st, i+1, before[i], after[i])
			}
		}
	}
}
