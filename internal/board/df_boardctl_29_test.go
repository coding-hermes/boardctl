package board

import (
	"strings"
	"testing"
)

// DF-BOARDCTL-29: update --deferred false must REMOVE the deferred key from
// the row entirely (nil-out semantics) instead of writing deferred:false.
// The old behavior stored a key that forever differed byte-wise from a
// never-deferred row, so external consumers diffing rows saw a phantom
// field. After the fix an un-deferred row is byte-identical to a
// never-deferred one apart from updated_at.

// TestUpdateDeferredFalseRemovesKey: create --deferred, then update
// --deferred false — the written line must not mention "deferred" at all.
func TestUpdateDeferredFalseRemovesKey(t *testing.T) {
	b := seedBT076Board(t)
	if _, err := b.Create(TaskRowSpec{ID: "DF-29", Title: "parked then released", Deferred: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}
	line := bt076Raw(t, b, "DF-29")
	if !strings.Contains(line, `"deferred": true`) {
		t.Fatalf("create --deferred did not write the key: %s", line)
	}
	dv := false
	if _, err := b.UpdateTask("DF-29", UpdateSpec{Deferred: &dv}); err != nil {
		t.Fatal(err)
	}
	after := bt076Raw(t, b, "DF-29")
	if strings.Contains(after, "deferred") {
		t.Fatalf("update --deferred false left a deferred key behind: %s", after)
	}
	r := bt076Row(t, b, "DF-29")
	if r.Has("deferred") {
		t.Fatalf("parsed row still carries the key: %s", after)
	}
	if RowIsDeferred(r) {
		t.Fatal("row reads as deferred without the key")
	}
}

// TestUpdateDeferredFalseMatchesNeverDeferred: the un-deferred row must be
// byte-identical to a row that was never deferred once the id and the
// timestamp fields are stripped (DF-BOARDCTL-29's core promise).
func TestUpdateDeferredFalseMatchesNeverDeferred(t *testing.T) {
	b := seedBT076Board(t)
	if _, err := b.Create(TaskRowSpec{ID: "DF-29D", Title: "same title", Deferred: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Create(TaskRowSpec{ID: "DF-29N", Title: "same title", Force: true}); err != nil {
		t.Fatal(err)
	}
	dv := false
	if _, err := b.UpdateTask("DF-29D", UpdateSpec{Deferred: &dv}); err != nil {
		t.Fatal(err)
	}
	// strip the fields that are ALLOWED to differ: the id and the
	// *_at timestamps (updated_at churns on the un-defer write).
	var comparableForm = func(line string) string {
		out := []string{}
		for _, part := range strings.Split(line, ",") {
			if strings.Contains(part, `"id"`) || strings.Contains(part, "_at") {
				continue
			}
			out = append(out, part)
		}
		return strings.Join(out, ",")
	}
	d := comparableForm(bt076Raw(t, b, "DF-29D"))
	n := comparableForm(bt076Raw(t, b, "DF-29N"))
	if d != n {
		t.Fatalf("un-deferred row differs from never-deferred row (id/_at stripped):\nun-deferred: %s\nnever:       %s", d, n)
	}
}

// TestUpdateDeferredFalseNoOpOnPlainRow: --deferred false on a row that
// never carried the key is an idempotent no-op — the change gate must not
// refuse it, nothing is reported as changed, and the line bytes stay
// identical (no updated_at churn).
func TestUpdateDeferredFalseNoOpOnPlainRow(t *testing.T) {
	b := seedBT076Board(t)
	before := bt076Raw(t, b, "DF-1")
	dv := false
	changed, err := b.UpdateTask("DF-1", UpdateSpec{Deferred: &dv})
	if err != nil {
		t.Fatalf("--deferred false on a key-less row must not be refused: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("no-op --deferred false reported changes %v", changed)
	}
	if after := bt076Raw(t, b, "DF-1"); after != before {
		t.Fatalf("no-op --deferred false rewrote the line:\nbefore %s\nafter  %s", before, after)
	}
	if strings.Contains(bt076Raw(t, b, "DF-1"), "deferred") {
		t.Fatal("no-op --deferred false created the key")
	}
}

// TestUpdateDeferredFalseKeyOrderPreserved: removing the key must not
// reorder the surviving keys — DeleteKey splices the key out of Keys, so a
// middle-of-row deferred key leaves the rest in its exact positions.
func TestUpdateDeferredFalseKeyOrderPreserved(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id": "DF-29O", "title": "mid-key", "deferred": true, "status": "pending", "priority": "P2"}` + "\n",
		"events.jsonl": `{"id": 1, "timestamp": "2026-10-07 00:00:00", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		"board.jsonl":  `{"project": "bt076", "namespace": "bt076", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": "abc1234"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	dv := false
	if _, err := b.UpdateTask("DF-29O", UpdateSpec{Deferred: &dv}); err != nil {
		t.Fatal(err)
	}
	line := bt076Raw(t, b, "DF-29O")
	// the seeded row carries no updated_at, so the update refreshes nothing:
	// the expected line is exactly the old row minus the removed key.
	want := `{"id": "DF-29O", "title": "mid-key", "status": "pending", "priority": "P2"}`
	if line != want {
		t.Fatalf("key order changed after key removal:\n got %s\nwant        %s", line, want)
	}
}

// TestUpdateDeferredFalseThenStatsNotDeferred: stats must count the
// un-deferred row as NOT deferred after the key removal (actionable picks
// it back up, deferred drops to zero).
func TestUpdateDeferredFalseThenStatsNotDeferred(t *testing.T) {
	b := seedBT076Board(t) // DF-1 plain pending, DF-2 deferred pending
	dv := false
	if _, err := b.UpdateTask("DF-2", UpdateSpec{Deferred: &dv}); err != nil {
		t.Fatal(err)
	}
	st, err := b.ComputeStats(TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if st.Deferred != 0 {
		t.Fatalf("deferred = %d, want 0 after un-defer", st.Deferred)
	}
	if st.Actionable != 2 {
		t.Fatalf("actionable = %d, want 2 (both pending rows are actionable again)", st.Actionable)
	}
	if st.Total != 2 {
		t.Fatalf("total = %d, want 2", st.Total)
	}
}

// TestUpdateNeverDeferredStaysByteIdentical: an unrelated update on a row
// that never carried the deferred key must not create it — the new bytes
// are exactly the old row + the appended fields.
func TestUpdateNeverDeferredStaysByteIdentical(t *testing.T) {
	b := seedBT076Board(t)
	before := bt076Raw(t, b, "DF-1")
	if _, err := b.UpdateTask("DF-1", UpdateSpec{Summary: strPtr("unrelated work")}); err != nil {
		t.Fatal(err)
	}
	after := bt076Raw(t, b, "DF-1")
	if strings.Contains(after, "deferred") {
		t.Fatalf("unrelated update created the deferred key: %s", after)
	}
	// before: {"id": ...,"priority": "P2"} — the update may only append
	// worker_summary + updated_at before the closing brace.
	if !strings.HasPrefix(after, before[:len(before)-1]) {
		t.Fatalf("unrelated update changed more than appended fields:\nbefore %s\nafter  %s", before, after)
	}
}
