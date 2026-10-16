package board

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// TR-251: evidence-run gate (AC1) + verified_complete census (AC2). Every
// fixture is a tmp_path board — never a live one.

// tr251Board builds a two-row board: one pending row to complete, one
// already-complete row (for idempotent re-write coverage).
func tr251Board() string {
	return `{"id": "TR-901", "title": "gate fixture target", "status": "pending", "priority": "P1", "created_at": "2026-10-16 00:00:00", "updated_at": "2026-10-16 00:00:00"}
{"id": "TR-902", "title": "already complete", "status": "complete", "priority": "P1", "created_at": "2026-10-16 00:00:00", "updated_at": "2026-10-16 00:00:00"}
`
}

// TestUpdateCompleteRefusedWithoutEvidenceRun pins AC1: the refusal names the
// missing flag and nothing is written.
func TestUpdateCompleteRefusedWithoutEvidenceRun(t *testing.T) {
	dir := t.TempDir()
	boardDir := dir + "/.coding-hermes/board"
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  tr251Board(),
		"events.jsonl": `{"id": 1, "timestamp": "2026-09-25 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	before, rerr := os.ReadFile(boardDir + "/tasks.jsonl")
	if rerr != nil {
		t.Fatal(rerr)
	}
	_, err = b.UpdateTask("TR-901", UpdateSpec{Status: strPtr("complete")})
	if err == nil {
		t.Fatal("update --status complete without --evidence-run-id must be refused")
	}
	if !errors.Is(err, ErrEvidenceRunRequired) {
		t.Fatalf("error must wrap ErrEvidenceRunRequired, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--evidence-run-id") {
		t.Fatalf("refusal must NAME the missing flag, got: %v", err)
	}
	if !strings.Contains(err.Error(), "TR-901") {
		t.Fatalf("refusal must name the row, got: %v", err)
	}
	after, rerr := os.ReadFile(boardDir + "/tasks.jsonl")
	if rerr != nil {
		t.Fatal(rerr)
	}
	if string(before) != string(after) {
		t.Fatal("a refused update must write NOTHING")
	}
}

// TestUpdateCompleteAcceptedWithEvidenceRun: the flagged form passes, and the
// run id lands in detail.evidence[].run_id (the SG-126 envelope).
func TestUpdateCompleteAcceptedWithEvidenceRun(t *testing.T) {
	dir := t.TempDir()
	boardDir := dir + "/.coding-hermes/board"
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  tr251Board(),
		"events.jsonl": `{"id": 1, "timestamp": "2026-09-25 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := b.UpdateTask("TR-901", UpdateSpec{
		Status:        strPtr("complete"),
		EvidenceRunID: "run-tr901-a",
	})
	if err != nil {
		t.Fatalf("flagged completion must pass: %v", err)
	}
	lines, rerr := ReadJSONLLines(boardDir + "/tasks.jsonl")
	if rerr != nil {
		t.Fatal(rerr)
	}
	var target *Row
	_ = IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		if row.String("id") == "TR-901" {
			target = row
		}
		return nil
	})
	if target == nil || target.String("status") != "complete" {
		t.Fatal("row must now be complete")
	}
	runs := RowEvidenceRunIDs(target)
	if len(runs) != 1 || runs[0] != "run-tr901-a" {
		t.Fatalf("evidence run must be bound on the row, got %v (changed=%v)", runs, changed)
	}
}

// TestUpdateCompleteIdempotentRewritePasses: a row already complete may be
// re-written complete without an evidence run (history is not re-judged).
func TestUpdateCompleteIdempotentRewritePasses(t *testing.T) {
	dir := t.TempDir()
	boardDir := dir + "/.coding-hermes/board"
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  tr251Board(),
		"events.jsonl": `{"id": 1, "timestamp": "2026-09-25 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.UpdateTask("TR-902", UpdateSpec{Status: strPtr("complete")}); err != nil {
		t.Fatalf("idempotent re-write of an already-complete row must pass: %v", err)
	}
	// …and a complete->failed->complete cycle is a fresh transition: gated.
	if _, err := b.UpdateTask("TR-902", UpdateSpec{Status: strPtr("failed")}); err != nil {
		t.Fatalf("complete->failed must pass: %v", err)
	}
	_, err = b.UpdateTask("TR-902", UpdateSpec{Status: strPtr("complete")})
	if err == nil || !errors.Is(err, ErrEvidenceRunRequired) {
		t.Fatalf("failed->complete is a fresh transition and must be gated, got: %v", err)
	}
}

// TestCreateCompleteRequiresEvidenceRun: the create path carries the gate too.
func TestCreateCompleteRequiresEvidenceRun(t *testing.T) {
	dir := t.TempDir()
	boardDir := dir + "/.coding-hermes/board"
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  tr251Board(),
		"events.jsonl": `{"id": 1, "timestamp": "2026-09-25 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.Create(TaskRowSpec{ID: "TR-903", Title: "born complete", Status: "complete"})
	if err == nil || !errors.Is(err, ErrEvidenceRunRequired) {
		t.Fatalf("create-into-complete without a run must be refused, got: %v", err)
	}
	if _, err := b.Create(TaskRowSpec{
		ID: "TR-903", Title: "born complete", Status: "complete",
		EvidenceRunID: "run-tr903",
	}); err != nil {
		t.Fatalf("create-into-complete with a run must pass: %v", err)
	}
}

// TestVerifiedCompleteCensus pins AC2's numbers: complete rows without a run
// count unverified (both spellings), detail-evidence and top-level
// evidence_run_id both count as verified, non-complete rows are ignored.
func TestVerifiedCompleteCensus(t *testing.T) {
	dir := t.TempDir()
	tasks := "" +
		`{"id": "TR-911", "title": "bare complete", "status": "complete", "priority": "P1"}
` +
		`{"id": "TR-912", "title": "alias complete", "status": "done", "priority": "P1"}
` +
		`{"id": "TR-913", "title": "verified detail", "status": "complete", "priority": "P1", "detail": {"evidence": [{"run_id": "run-913"}]}}
` +
		`{"id": "TR-914", "title": "verified field", "status": "complete", "priority": "P1", "evidence_run_id": "run-914"}
` +
		`{"id": "TR-915", "title": "failed row", "status": "failed", "priority": "P1"}
` +
		`{"id": "TR-916", "title": "pending row", "status": "pending", "priority": "P1"}
`
	boardDir := dir + "/.coding-hermes/board"
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  tasks,
		"events.jsonl": `{"id": 1, "timestamp": "2026-09-25 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := b.VerifiedCompleteCensus()
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 4 {
		t.Fatalf("total complete = %d, want 4 (two bare + two verified)", stats.Total)
	}
	if stats.Verified != 2 {
		t.Fatalf("verified = %d, want 2", stats.Verified)
	}
	want := map[string]bool{"TR-911": true, "TR-912": true}
	if len(stats.UnverifiedIDs) != 2 {
		t.Fatalf("unverified ids = %v, want 2 entries", stats.UnverifiedIDs)
	}
	for _, id := range stats.UnverifiedIDs {
		if !want[id] {
			t.Fatalf("unexpected unverified id %q (want only TR-911/TR-912)", id)
		}
	}
	if stats.Share() != 0.5 {
		t.Fatalf("share = %v, want 0.5", stats.Share())
	}
}

// TestDoctorVerifiedCompleteWarning pins the doctor surface: the census
// becomes a warn finding naming the share and the unverified ids; a fully
// verified board emits the no-debt spelling; a board with zero complete rows
// stays silent.
func TestDoctorVerifiedCompleteWarning(t *testing.T) {
	t.Run("reports debt", func(t *testing.T) {
		dir := t.TempDir()
		boardDir := dir + "/.coding-hermes/board"
		writeBoardFiles(t, boardDir, map[string]string{
			"tasks.jsonl":  tr251Board(),
			"events.jsonl": `{"id": 1, "timestamp": "2026-09-25 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		})
		b, err := Resolve(dir)
		if err != nil {
			t.Fatal(err)
		}
		rep, err := b.Doctor()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, f := range rep.Findings {
			if strings.Contains(f.Msg, "verified_complete_share") {
				found = true
				if !strings.Contains(f.Msg, "TR-902") {
					t.Fatalf("debt warn must name the unverified row, got: %s", f.Msg)
				}
				if f.Level != "warn" {
					t.Fatalf("debt census is a warn, got %q (the write gate is the error)", f.Level)
				}
			}
		}
		if !found {
			t.Fatal("doctor must report verified_complete_share")
		}
	})
	t.Run("silent when no complete rows", func(t *testing.T) {
		dir := t.TempDir()
		boardDir := dir + "/.coding-hermes/board"
		writeBoardFiles(t, boardDir, map[string]string{
			"tasks.jsonl":  `{"id": "TR-920", "title": "open", "status": "pending", "priority": "P1"}`,
			"events.jsonl": `{"id": 1, "timestamp": "2026-09-25 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		})
		b, err := Resolve(dir)
		if err != nil {
			t.Fatal(err)
		}
		rep, err := b.Doctor()
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range rep.Findings {
			if strings.Contains(f.Msg, "verified_complete_share") {
				t.Fatalf("no complete rows -> no census finding, got: %s", f.Msg)
			}
		}
	})
}
