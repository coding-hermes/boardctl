package board

// DEFER-EVIDENCE: a deferral must name its blocker. These tests pin the
// contract on top of BT-076:
//
//   - create --deferred WITHOUT --defer-reason is REFUSED (nothing written)
//   - create --deferred with a reason writes deferred + defer_reason
//   - update --deferred true without a reason (here or already on the row)
//     is REFUSED with nothing written
//   - update --deferred false removes the audit keys with the flag
//     (the DF-BOARDCTL-29 mirror rule)
//   - defer_reason / defer_review_by are sanctioned keys
//   - validate WARNS (never fails) on a hand-edited deferred row that
//     carries no reason — legacy debt stays visible, boards stay exit-0

import (
	"os"
	"strings"
	"testing"
)

func seedDeferEvidenceBoard(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id": "DE-1", "title": "plain pending", "status": "pending", "priority": "P2"}` + "\n" +
			`{"id": "DE-2", "title": "parked with reason", "status": "pending", "priority": "P1", "deferred": true, "defer_reason": "owner-gated: needs budget approval"}` + "\n" +
			`{"id": "DE-3", "title": "parked bare", "status": "pending", "priority": "P2", "deferred": true}` + "\n",
		"events.jsonl": `{"id": 1, "timestamp": "2026-11-07 00:00:00", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		"board.jsonl":  `{"project": "deferev", "namespace": "deferev", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": "abc1234"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func deferEvidenceIDs(t *testing.T, b *Board) []string {
	t.Helper()
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.String("id"))
	}
	return ids
}

func TestCreateDeferredWithoutReasonRefused(t *testing.T) {
	b := seedDeferEvidenceBoard(t)
	before := deferEvidenceIDs(t, b)
	truth := true
	_, err := b.Create(TaskRowSpec{ID: "DE-NEW", Title: "no reason", Deferred: &truth})
	if err == nil {
		t.Fatal("create --deferred without --defer-reason must be refused")
	}
	if !strings.Contains(err.Error(), "--defer-reason") {
		t.Fatalf("refusal must name the missing flag: %v", err)
	}
	if after := deferEvidenceIDs(t, b); strings.Join(after, ",") != strings.Join(before, ",") {
		t.Fatalf("refused create changed the board: before=%v after=%v", before, after)
	}
}

func TestCreateDeferredWithReasonWritesKeys(t *testing.T) {
	b := seedDeferEvidenceBoard(t)
	truth := true
	reason := "external dependency: upstream release cut"
	if _, err := b.Create(TaskRowSpec{ID: "DE-NEW", Title: "deferred with reason", Deferred: &truth, DeferReason: &reason}); err != nil {
		t.Fatal(err)
	}
	raw := deferEvidenceRow(t, b, "DE-NEW")
	if !strings.Contains(raw, `"defer_reason"`) || !strings.Contains(raw, "upstream release cut") {
		t.Fatalf("defer_reason not written: %s", raw)
	}
}

func TestUpdateDeferWithoutReasonRefused(t *testing.T) {
	b := seedDeferEvidenceBoard(t)
	truth := true
	// DE-1 carries no defer_reason and none is supplied -> refused.
	if _, err := b.UpdateTask("DE-1", UpdateSpec{Deferred: &truth}); err == nil {
		t.Fatal("update --deferred true without --defer-reason must be refused")
	} else if !strings.Contains(err.Error(), "--defer-reason") {
		t.Fatalf("refusal must name the missing flag: %v", err)
	}
	// The same update on DE-2 (reason already on the row) succeeds.
	if _, err := b.UpdateTask("DE-2", UpdateSpec{Deferred: &truth}); err != nil {
		t.Fatalf("update --deferred true on a row that already carries a reason must succeed: %v", err)
	}
}

func TestUpdateUnDeferRemovesAuditKeys(t *testing.T) {
	b := seedDeferEvidenceBoard(t)
	falsity := false
	if _, err := b.UpdateTask("DE-2", UpdateSpec{Deferred: &falsity}); err != nil {
		t.Fatal(err)
	}
	raw := deferEvidenceRow(t, b, "DE-2")
	if strings.Contains(raw, "deferred") || strings.Contains(raw, "defer_reason") {
		t.Fatalf("un-defer must remove the flag AND the audit keys: %s", raw)
	}
}

func TestDeferAuditKeysSanctioned(t *testing.T) {
	for _, k := range []string{"defer_reason", "defer_review_by"} {
		if !IsSanctionedTaskKey(k) {
			t.Fatalf("key %q should be sanctioned (DEFER-EVIDENCE)", k)
		}
	}
	// The drift spellings stay unsanctioned.
	for _, k := range []string{"deferred_reason", "defered"} {
		if IsSanctionedTaskKey(k) {
			t.Fatalf("key %q must stay unsanctioned drift", k)
		}
	}
}

func TestValidateDeferredWithoutReasonWarns(t *testing.T) {
	b := seedDeferEvidenceBoard(t)
	rep, verr := b.Validate()
	if verr != nil {
		t.Fatal(verr)
	}
	saw := false
	for _, f := range rep.Findings {
		if f.Level == "warn" && strings.Contains(f.Msg, "no defer_reason") {
			saw = true
		}
	}
	if !saw {
		t.Fatal("validate must WARN on the bare deferred row DE-3")
	}
	// And it stays a warning, not a failure: the board exit policy is warn-0.
	if rep.HasErrors() {
		t.Fatalf("missing defer_reason must be a warn, not an error: %v", rep.Findings)
	}
}

// deferEvidenceRow returns the raw tasks.jsonl line for one id.
func deferEvidenceRow(t *testing.T, b *Board, id string) string {
	t.Helper()
	raw, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.Contains(l, `"id": "`+id+`"`) {
			return l
		}
	}
	t.Fatalf("task %q not found", id)
	return ""
}
