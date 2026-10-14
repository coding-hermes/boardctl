package board

// DF-BOARDCTL-37: the multi-status OR filter behind
// `umbrella --list --status s1,s2`. Statuses matches on the NORMALIZED row
// status (a stored alias row counts as its canonical value), composes with
// the single Status filter, and leaves the empty list as "any status" (the
// pre-existing behavior — TaskFilter{} literals everywhere must be
// unaffected). The CLI-level vocabulary/alias handling lives in
// cmd/boardctl (parseStatuses); this file proves the filter mechanics.
//
// The newTestBoard fixture ships its own row (EXIST-1, complete), so the
// assertions compare the SEEDED id set ("DF37-*") rather than raw counts.

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

// df37SeedStatus writes one task row with a verbatim status into the
// board's tasks.jsonl (the create gate rightly refuses alias spellings —
// a stored alias row is exactly the hand-written/imported shape this
// filter must still match canonically).
func df37SeedStatus(t *testing.T, b *Board, id, rawStatus string) {
	t.Helper()
	row, err := json.Marshal(map[string]any{
		"id": id, "title": "df37 " + id, "status": rawStatus, "priority": "P2",
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(b.TasksPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck
	if _, err := f.Write(append(row, '\n')); err != nil {
		t.Fatal(err)
	}
}

// df37FilteredIDs runs the Statuses filter and returns the seeded DF37-*
// ids that survived, sorted.
func df37FilteredIDs(t *testing.T, b *Board, statuses []string) []string {
	t.Helper()
	rows, err := b.ListTasks(TaskFilter{Statuses: statuses})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range rows {
		if id := r.String("id"); len(id) > 5 && id[:5] == "DF37-" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func TestTaskFilterStatusesSingleStatus(t *testing.T) {
	b := newTestBoard(t)
	df37SeedStatus(t, b, "DF37-P1", "pending")
	df37SeedStatus(t, b, "DF37-P2", "pending")
	df37SeedStatus(t, b, "DF37-C1", "complete")

	if got := df37FilteredIDs(t, b, []string{"pending"}); len(got) != 2 || got[0] != "DF37-P1" || got[1] != "DF37-P2" {
		t.Fatalf("Statuses=[pending] = %v, want [DF37-P1 DF37-P2]", got)
	}
	if got := df37FilteredIDs(t, b, []string{"complete"}); len(got) != 1 || got[0] != "DF37-C1" {
		t.Fatalf("Statuses=[complete] = %v, want [DF37-C1]", got)
	}
}

func TestTaskFilterStatusesCommaListIsOR(t *testing.T) {
	b := newTestBoard(t)
	df37SeedStatus(t, b, "DF37-P1", "pending")
	df37SeedStatus(t, b, "DF37-C1", "complete")
	df37SeedStatus(t, b, "DF37-F1", "failed")

	if got := df37FilteredIDs(t, b, []string{"complete", "failed"}); len(got) != 2 || got[0] != "DF37-C1" || got[1] != "DF37-F1" {
		t.Fatalf("Statuses=[complete,failed] = %v, want [DF37-C1 DF37-F1] (OR)", got)
	}
	if got := df37FilteredIDs(t, b, []string{"pending", "complete"}); len(got) != 2 || got[0] != "DF37-C1" || got[1] != "DF37-P1" {
		t.Fatalf("Statuses=[pending,complete] = %v, want [DF37-C1 DF37-P1]", got)
	}
}

// The alias contract: the CLI normalizes filter values, but the filter
// itself must hold the same reading the single Status filter has — a STORED
// alias row ("done") counts as its canonical value.
func TestTaskFilterStatusesMatchesStoredAliasRowsCanonically(t *testing.T) {
	b := newTestBoard(t)
	df37SeedStatus(t, b, "DF37-DONE", "done") // stored alias spelling
	df37SeedStatus(t, b, "DF37-IMPL", "in_progress")

	if got := df37FilteredIDs(t, b, []string{"complete"}); len(got) != 1 || got[0] != "DF37-DONE" {
		t.Fatalf("Statuses=[complete] must match a stored \"done\" row, got %v", got)
	}
	if got := df37FilteredIDs(t, b, []string{"pending", "in_progress"}); len(got) != 1 || got[0] != "DF37-IMPL" {
		t.Fatalf("Statuses=[pending,in_progress] = %v, want [DF37-IMPL]", got)
	}
}

func TestTaskFilterStatusesUnknownMatchesNothing(t *testing.T) {
	b := newTestBoard(t)
	df37SeedStatus(t, b, "DF37-P1", "pending")

	if got := df37FilteredIDs(t, b, []string{"nonexistent"}); len(got) != 0 {
		t.Fatalf("unknown status must match nothing, got %v", got)
	}
}

// Composition: when both Status and Statuses are set, BOTH must hold
// (defensive — the CLI populates only one; the filter must not turn the
// pair into a union).
func TestTaskFilterStatusesComposesWithStatus(t *testing.T) {
	b := newTestBoard(t)
	df37SeedStatus(t, b, "DF37-P1", "pending")
	df37SeedStatus(t, b, "DF37-C1", "complete")

	rows, err := b.ListTasks(TaskFilter{Status: "pending", Statuses: []string{"complete"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("Status=pending AND Statuses=[complete] must hold both, got %d rows", len(rows))
	}
	rows, err = b.ListTasks(TaskFilter{Status: "pending", Statuses: []string{"pending", "complete"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].String("id") != "DF37-P1" {
		t.Fatalf("Status=pending AND Statuses=[pending,complete] want only DF37-P1, got %d rows", len(rows))
	}
}

// The empty Statuses list must stay "any status": every pre-existing
// TaskFilter{} literal relies on it. Asserted on the whole row set (fixture
// row included) so an accidental default-status exclusion cannot hide.
func TestTaskFilterStatusesEmptyIsAny(t *testing.T) {
	b := newTestBoard(t)
	df37SeedStatus(t, b, "DF37-P1", "pending")
	df37SeedStatus(t, b, "DF37-C1", "complete")

	rows, err := b.ListTasks(TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("empty Statuses matched %d rows, want 3 (any status incl. the fixture row)", len(rows))
	}
}
