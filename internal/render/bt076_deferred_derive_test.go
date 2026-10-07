package render

import (
	"testing"

	"github.com/coding-hermes/boardctl/internal/board"
)

// BT-076: the derived block counts deferred rows separately (deferred_count)
// and excludes them from open_count — the actionable tally the completion %
// and burndown headline read. Status counts keep the deferred rows.

// TestDerivedDeferredExcludedFromOpen: a pending deferred row rides
// deferred_count, not open_count; a plain pending row still counts open.
func TestDerivedDeferredExcludedFromOpen(t *testing.T) {
	tasks := []*board.Row{
		mustRow(t, taskLine("DF-A", "2026-09-01 00:00:00", "", "pending")),
		// spaced seed style with the boolean deferred key
		mustRow(t, `{"id":"DF-B","title":"t DF-B","status":"pending","priority":"P1","created_at":"2026-09-01 00:00:00","deferred":true}`),
	}
	d := mkBoard(tasks, nil)
	der := derive(d)
	if der.DeferredCount != 1 {
		t.Fatalf("deferred_count = %d, want 1", der.DeferredCount)
	}
	if der.OpenCount != 1 {
		t.Fatalf("open_count = %d, want 1 (the deferred pending row is excluded)", der.OpenCount)
	}
	if der.StatusCounts["pending"] != 2 {
		t.Fatalf("status_counts[pending] = %d, want 2 (deferred rows keep their status tally)", der.StatusCounts["pending"])
	}
	if der.TotalNonFix != 2 {
		t.Fatalf("total_nonfixture = %d, want 2", der.TotalNonFix)
	}
}

// TestDerivedDeferredCompleteRow: a deferred COMPLETE row counts complete
// (deferral neutralizes open work, not done work) and rides deferred_count.
func TestDerivedDeferredCompleteRow(t *testing.T) {
	tasks := []*board.Row{
		mustRow(t, taskLine("DF-C", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete")),
		mustRow(t, `{"id":"DF-D","title":"t DF-D","status":"complete","priority":"P2","created_at":"2026-09-01 00:00:00","completed_at":"2026-09-02 00:00:00","deferred":true}`),
	}
	d := mkBoard(tasks, nil)
	der := derive(d)
	if der.DeferredCount != 1 {
		t.Fatalf("deferred_count = %d, want 1", der.DeferredCount)
	}
	if der.CompleteCount != 2 {
		t.Fatalf("complete_count = %d, want 2", der.CompleteCount)
	}
	if der.OpenCount != 0 {
		t.Fatalf("open_count = %d, want 0", der.OpenCount)
	}
}

// TestDerivedDeferredNullAndAbsent: null/absent deferred never counts.
func TestDerivedDeferredNullAndAbsent(t *testing.T) {
	tasks := []*board.Row{
		mustRow(t, `{"id":"DF-E","title":"t DF-E","status":"pending","priority":"P2","created_at":"2026-09-01 00:00:00","deferred":null}`),
		mustRow(t, taskLine("DF-F", "2026-09-01 00:00:00", "", "pending")),
	}
	d := mkBoard(tasks, nil)
	der := derive(d)
	if der.DeferredCount != 0 {
		t.Fatalf("deferred_count = %d, want 0 (null is not deferred)", der.DeferredCount)
	}
	if der.OpenCount != 2 {
		t.Fatalf("open_count = %d, want 2", der.OpenCount)
	}
}
