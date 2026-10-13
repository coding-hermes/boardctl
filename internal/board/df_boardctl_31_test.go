package board

import (
	"strings"
	"testing"
)

// ---------- DF-BOARDCTL-31: a superseded duplicate validates as resolved ----------
//
// dedupe-board --by-id --apply (and sweep-status --decide --apply) close a
// LATER duplicate line as superseded-by-earliest: status=complete,
// superseded_by=<kept id>, completed_at set. Pre-fix, validate kept erroring
// "duplicate task id ... (also line N)" on that closed line forever — the
// repair could never turn a board green. Post-fix: a later duplicate carrying
// a non-empty superseded_by downgrades to a WARNING naming the kept line; a
// plain duplicate (no superseded_by, including the empty-string spelling)
// still errors with the byte-identical message.

// seedIDDupe31 writes kept + later task lines and resolves the board.
// Both lines carry created_at so no timestamp warnings pollute the findings.
func seedIDDupe31(t *testing.T, laterLine string) *Board {
	t.Helper()
	return seedFindingBoard(t,
		`{"id":"QA-ASCE-1","title":"[P2] header-frozen on asce main","reasoning":"cell detail: header never advances","status":"pending","priority":"P2","created_at":"2026-10-01 00:00:00"}`,
		laterLine,
	)
}

// idDupe31Finding returns the duplicate-id finding for the board ("" if none).
func idDupe31Finding(t *testing.T, b *Board) Finding {
	t.Helper()
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "duplicate task id") {
			return f
		}
	}
	return Finding{}
}

// AC (resolved arm): a later duplicate closed as superseded-by-earliest is a
// WARNING, not an error — the board validates clean (exit 0 semantics).
func TestValidateIDDupeSupersededResolves(t *testing.T) {
	b := seedIDDupe31(t,
		`{"id":"QA-ASCE-1","title":"[P2] spawn pool exhausted","reasoning":"cell detail: no free worker slots","status":"complete","priority":"P2","superseded_by":"QA-ASCE-1","worker_summary":"superseded-by-earliest: recycled task id — kept line 1 (id-dupe backfill 2026-10-13)","completed_at":"2026-10-13 00:00:00","created_at":"2026-10-02 00:00:00"}`)
	f := idDupe31Finding(t, b)
	if f.Level == "" {
		t.Fatalf("resolved duplicate produced no finding at all; want a warning: %+v", mustValidate31(t, b).Findings)
	}
	if f.IsError() {
		t.Fatalf("superseded duplicate still an error: [%s] %s", f.Level, f.Msg)
	}
	if !strings.Contains(f.Msg, "RESOLVED") || !strings.Contains(f.Msg, "superseded_by") {
		t.Fatalf("resolved-duplicate warning lost its resolution markers: %s", f.Msg)
	}
	if !strings.Contains(f.Msg, "line 1") {
		t.Fatalf("resolved-duplicate warning does not name the kept line: %s", f.Msg)
	}
	if rep := mustValidate31(t, b); rep.HasErrors() {
		t.Fatalf("repaired board must validate clean (exit 0): %+v", rep.Findings)
	}
}

// AC (plain arm, unchanged): a later duplicate WITHOUT superseded_by still
// errors with the exact historical message.
func TestValidateIDDupePlainStillErrors(t *testing.T) {
	b := seedIDDupe31(t,
		`{"id":"QA-ASCE-1","title":"[P2] spawn pool exhausted","reasoning":"cell detail: no free worker slots","status":"pending","priority":"P2","created_at":"2026-10-02 00:00:00"}`)
	f := idDupe31Finding(t, b)
	if !f.IsError() {
		t.Fatalf("plain duplicate downgraded to [%s], want error: %s", f.Level, f.Msg)
	}
	want := `duplicate task id "QA-ASCE-1" (also line 1)`
	if !strings.Contains(f.Msg, want) {
		t.Fatalf("plain-duplicate message changed; want substring %q, got: %s", want, f.Msg)
	}
	if rep := mustValidate31(t, b); !rep.HasErrors() {
		t.Fatal("plain duplicate must keep validate at exit 1")
	}
}

// The empty-string spelling of the marker is NOT a resolution: only a
// non-empty superseded_by downgrades.
func TestValidateIDDupeEmptySupersededByStillErrors(t *testing.T) {
	b := seedIDDupe31(t,
		`{"id":"QA-ASCE-1","title":"[P2] spawn pool exhausted","reasoning":"cell detail: no free worker slots","status":"pending","priority":"P2","superseded_by":"","created_at":"2026-10-02 00:00:00"}`)
	f := idDupe31Finding(t, b)
	if !f.IsError() {
		t.Fatalf("empty superseded_by downgraded to [%s], want error: %s", f.Level, f.Msg)
	}
}

// AC (end-to-end): the exact fleet repair loop — a plain-duplicate board
// fails validate, dedupe-board --by-id --apply closes the later line, and the
// SAME board then validates clean. The closed line carries the repair's
// fields and the audit event lands.
func TestValidateIDDupeRepairMakesBoardGreen(t *testing.T) {
	b := seedIDDupe31(t,
		`{"id":"QA-ASCE-1","title":"[P2] spawn pool exhausted","reasoning":"cell detail: no free worker slots","status":"in_progress","priority":"P2","created_at":"2026-10-02 00:00:00"}`)
	if rep := mustValidate31(t, b); !rep.HasErrors() {
		t.Fatal("precondition failed: the plain-duplicate board must fail validate pre-repair")
	}
	if _, err := b.DedupeByID(true); err != nil {
		t.Fatal(err)
	}
	rep := mustValidate31(t, b)
	if rep.HasErrors() {
		t.Fatalf("board still fails validate after the id-dupe repair: %+v", rep.Findings)
	}
	f := idDupe31Finding(t, b)
	if f.Level != "warn" || !strings.Contains(f.Msg, "RESOLVED") {
		t.Fatalf("post-repair finding = [%s] %s, want the RESOLVED warning", f.Level, f.Msg)
	}
	// The closed line carries exactly the repair contract's fields.
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	var closed *Row
	for _, r := range rows {
		if r.String("status") == "complete" && r.String("superseded_by") == "QA-ASCE-1" {
			closed = r
		}
	}
	if closed == nil {
		t.Fatalf("no superseded closed line after repair: %d rows", len(rows))
	}
	if closed.String("worker_summary") == "" || strings.Contains(closed.String("worker_summary"), "kept line 1") == false {
		t.Fatalf("closed line worker_summary wrong: %q", closed.String("worker_summary"))
	}
	if closed.String("completed_at") == "" {
		t.Fatalf("closed line missing completed_at: %s", RowJSONCompact(closed))
	}
}

// mustValidate31 runs Validate and fails the test on the hard error return.
func mustValidate31(t *testing.T, b *Board) *Report {
	t.Helper()
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	return rep
}
