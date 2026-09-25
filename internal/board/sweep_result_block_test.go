package board

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// BT-066: sweep-status classified fixable rows from the STATUS column only,
// but --apply rewrites each row through NormalizeTask, which validates ALL
// vocab columns. A clean alias status next to a legacy off-vocab ci_result
// ("pending") was advertised as fixable, then NormalizeTask refused it and
// sweep.go aborted MID-FILE — earlier rows already written, the rest never
// repaired, and the dry-run had shown no warning. These tests pin the two
// coordinated fixes: (1) all three vocab columns are classified up front and
// off-vocab guard/ci rows are Explicit (never fixable), and (2) --apply
// skips-and-reports a per-row refusal instead of aborting, with the
// measured after-count still counting blocked rows.

// seedBT066Board writes a topology-A board from one raw JSONL line per task
// row, in order, and resolves it.
func seedBT066Board(t *testing.T, taskLines []string) *Board {
	t.Helper()
	dir := t.TempDir()
	content := strings.Join(taskLines, "") // each line carries its own "\n"
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  content,
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// bt066FileLines reads tasks.jsonl and splits it into per-line bytes (no
// trailing empty element), so tests can assert byte-identical lines.
func bt066FileLines(t *testing.T, path string) [][]byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(raw, []byte("\n"))
	if n := len(lines); n > 0 && len(lines[n-1]) == 0 {
		lines = lines[:n-1]
	}
	return lines
}

// TestBT066DryRunNamesOffVocabResultAsExplicit: a board holding an alias
// status next to an off-vocab ci_result reports that row as
// blocked/explicit — naming the COLUMN and the VALUE — and never counts it
// fixable. An off-vocab guard_result is the same class, as is an unknown
// status.
func TestBT066DryRunNamesOffVocabResultAsExplicit(t *testing.T) {
	b := seedBT066Board(t, []string{
		`{"id":"BT066-CI-ALIAS","title":"alias status + legacy ci","status":"todo","priority":"P2","ci_result":"pending"}` + "\n",
		`{"id":"BT066-GUARD-STRAY","title":"ci value in guard column","status":"pending","priority":"P2","guard_result":"GREEN"}` + "\n",
		`{"id":"BT066-CLEAN-ALIAS","title":"clean alias","status":"done","priority":"P2"}` + "\n",
		`{"id":"BT066-UNKNOWN-STATUS","title":"unknown status","status":"parked","priority":"P2"}` + "\n",
	})
	rep, err := b.StatusSweep(false)
	if err != nil {
		t.Fatalf("dry-run sweep errored: %v", err)
	}

	// The alias+off-vocab-ci row must be Explicit, naming column and value.
	var ciRow *StatusSweepRow
	for i := range rep.Explicit {
		if rep.Explicit[i].ID == "BT066-CI-ALIAS" {
			ciRow = &rep.Explicit[i]
		}
	}
	if ciRow == nil {
		t.Fatalf("alias row with off-vocab ci_result not reported explicit: %+v", rep)
	}
	if ciRow.Blocked != "ci_result" || ciRow.BlockedValue != "pending" {
		t.Errorf("blocked = %q/%q, want ci_result/\"pending\"", ciRow.Blocked, ciRow.BlockedValue)
	}
	if !strings.Contains(ciRow.Action, `ci_result "pending" is not in vocabulary`) {
		t.Errorf("action must name the column and value, got: %s", ciRow.Action)
	}

	// It must NOT be advertised as fixable.
	for _, e := range rep.Normalized {
		if e.ID == "BT066-CI-ALIAS" {
			t.Errorf("row with off-vocab ci_result advertised as fixable: %+v", e)
		}
	}

	// The stray guard value is the same explicit class, naming its column.
	var gRow *StatusSweepRow
	for i := range rep.Explicit {
		if rep.Explicit[i].ID == "BT066-GUARD-STRAY" {
			gRow = &rep.Explicit[i]
		}
	}
	if gRow == nil {
		t.Fatalf("off-vocab guard_result row not reported explicit: %+v", rep)
	}
	if gRow.Blocked != "guard_result" || gRow.BlockedValue != "GREEN" {
		t.Errorf("blocked = %q/%q, want guard_result/\"GREEN\"", gRow.Blocked, gRow.BlockedValue)
	}

	// The clean alias row alone is fixable.
	if len(rep.Normalized) != 1 || rep.Normalized[0].ID != "BT066-CLEAN-ALIAS" {
		t.Errorf("normalized = %+v, want only BT066-CLEAN-ALIAS", rep.Normalized)
	}

	// Census: 4 off-vocab rows (2 blocked + 1 unknown status + 1 fixable),
	// 3 of them explicit.
	if rep.OffBefore != 4 {
		t.Errorf("OffBefore = %d, want 4 (blocked rows count)", rep.OffBefore)
	}
	if len(rep.Explicit) != 3 {
		t.Errorf("Explicit = %d, want 3", len(rep.Explicit))
	}

	// The rendered report names the blocked row's column and value.
	text := rep.RenderText()
	if !strings.Contains(text, `ci_result "pending"`) {
		t.Errorf("dry-run report must name the blocked column+value:\n%s", text)
	}
}

// TestBT066ApplySkipsRefusedRowAndRepairsTheRest: --apply exits without
// error, repairs the decidable alias rows — INCLUDING rows AFTER the refused
// one (the old code aborted mid-file) — and leaves every blocked or refused
// row byte-identical.
func TestBT066ApplySkipsRefusedRowAndRepairsTheRest(t *testing.T) {
	// Line 3 carries the genuine NormalizeTask refusal: the key-alias `why`
	// cannot be renamed onto the already-present canonical `reasoning`, on a
	// row whose status/guard/ci are all clean — unreachable by vocab
	// classification, so the sweep must survive its refusal.
	b := seedBT066Board(t, []string{
		`{"id":"BT066-A-HEAD","title":"fixable before the refusal","status":"todo","priority":"P2"}` + "\n",
		`{"id":"BT066-B-REFUSE","title":"key clash refuses normalize","status":"todo","priority":"P2","reasoning":"canonical","why":"alias spelling"}` + "\n",
		`{"id":"BT066-C-TAIL","title":"fixable AFTER the refusal","status":"open","priority":"P2"}` + "\n",
		`{"id":"BT066-D-BLOCKED","title":"alias status + legacy ci","status":"done","priority":"P2","ci_result":"pending"}` + "\n",
	})
	before := bt066FileLines(t, b.tasksPath)

	rep, err := b.StatusSweep(true)
	if err != nil {
		t.Fatalf("apply must not abort on a refused row, got: %v", err)
	}

	after := bt066FileLines(t, b.tasksPath)
	if len(after) != len(before) {
		t.Fatalf("line count changed: %d -> %d", len(before), len(after))
	}

	// Rows before AND after the refused row were repaired.
	wantFixed := map[string]bool{"BT066-A-HEAD": true, "BT066-C-TAIL": true}
	for i := range rep.Normalized {
		e := rep.Normalized[i]
		if e.Skipped {
			continue
		}
		if !wantFixed[e.ID] {
			t.Errorf("unexpected fixed row %q", e.ID)
			continue
		}
		delete(wantFixed, e.ID)
		if !e.Fixed {
			t.Errorf("row %q repaired on disk but not marked Fixed", e.ID)
		}
		if bytes.Equal(before[e.Line-1], after[e.Line-1]) {
			t.Errorf("row %q reported fixed but its line is byte-identical", e.ID)
		}
	}
	if len(wantFixed) > 0 {
		t.Errorf("apply did not repair every decidable alias row; missing: %v", wantFixed)
	}

	// The refused row: marked skipped, action explains, line byte-identical.
	var refused *StatusSweepRow
	for i := range rep.Normalized {
		if rep.Normalized[i].ID == "BT066-B-REFUSE" {
			refused = &rep.Normalized[i]
		}
	}
	if refused == nil {
		t.Fatalf("refused row missing from the report: %+v", rep.Normalized)
	}
	if !refused.Skipped {
		t.Errorf("refused row not marked Skipped: %+v", refused)
	}
	if !strings.Contains(refused.Action, "normalize refused") {
		t.Errorf("skipped action must say the normalize refused, got: %s", refused.Action)
	}
	if !bytes.Equal(before[refused.Line-1], after[refused.Line-1]) {
		t.Errorf("refused row was modified:\nbefore %s\nafter  %s", before[refused.Line-1], after[refused.Line-1])
	}

	// The vocab-blocked row (off-vocab ci_result) is explicit and untouched.
	var blocked *StatusSweepRow
	for i := range rep.Explicit {
		if rep.Explicit[i].ID == "BT066-D-BLOCKED" {
			blocked = &rep.Explicit[i]
		}
	}
	if blocked == nil {
		t.Fatalf("off-vocab ci row not reported explicit on apply: %+v", rep)
	}
	if !bytes.Equal(before[blocked.Line-1], after[blocked.Line-1]) {
		t.Errorf("explicit row was modified:\nbefore %s\nafter  %s", before[blocked.Line-1], after[blocked.Line-1])
	}

	// On-disk truth: the two decidable rows hold canonical statuses, the
	// refused and blocked rows still hold their original spellings.
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range rows {
		got[r.String("id")] = r.String("status")
	}
	if got["BT066-A-HEAD"] != "pending" || got["BT066-C-TAIL"] != "pending" {
		t.Errorf("decidable rows not canonicalized: %+v", got)
	}
	if got["BT066-B-REFUSE"] != "todo" {
		t.Errorf("refused row status changed on disk: %q", got["BT066-B-REFUSE"])
	}
	if got["BT066-D-BLOCKED"] != "done" {
		t.Errorf("blocked row status changed on disk: %q", got["BT066-D-BLOCKED"])
	}
}

// TestBT066AfterCountCountsBlockedRows: the honest after-count — a blocked
// row the sweep never claimed to fix is still off-vocabulary after the
// apply, so OffAfter must include it (measured from the file).
func TestBT066AfterCountCountsBlockedRows(t *testing.T) {
	b := seedBT066Board(t, []string{
		`{"id":"BT066-FIXED-OK","title":"clean alias","status":"todo","priority":"P2"}` + "\n",
		`{"id":"BT066-STILL-BAD","title":"legacy ci","status":"pending","priority":"P2","ci_result":"pending"}` + "\n",
	})
	rep, err := b.StatusSweep(true)
	if err != nil {
		t.Fatalf("apply errored: %v", err)
	}
	if rep.OffBefore != 2 {
		t.Errorf("OffBefore = %d, want 2 (1 fixable + 1 blocked)", rep.OffBefore)
	}
	if rep.OffAfter != 1 {
		t.Errorf("OffAfter = %d, want 1 — the blocked row must still be counted as remaining", rep.OffAfter)
	}
	// The rendered census line shows the honest pair.
	text := rep.RenderText()
	if !strings.Contains(text, "before: 2 off-vocabulary rows; after: 1 off-vocabulary rows") {
		t.Errorf("census line must show before 2 / after 1:\n%s", text)
	}
	// And the applied report still lists the blocked row as explicit.
	if len(rep.Explicit) != 1 || rep.Explicit[0].ID != "BT066-STILL-BAD" {
		t.Errorf("apply report must keep the blocked row explicit: %+v", rep.Explicit)
	}
}

// TestBT066CleanVocabApplyIsByteIdenticalNoop: boards whose rows are all
// clean-vocabulary behave exactly as before BT-066 — a no-op apply writes
// nothing, and the report is empty at zero.
func TestBT066CleanVocabApplyIsByteIdenticalNoop(t *testing.T) {
	b := seedBT066Board(t, []string{
		`{"id":"BT066-CLEAN-1","title":"canonical","status":"pending","priority":"P2","guard_result":"PASS","ci_result":"GREEN"}` + "\n",
		`{"id":"BT066-CLEAN-2","title":"case-tolerant results","status":"complete","priority":"P1","guard_result":"skip","ci_result":"red"}` + "\n",
	})
	before := bt066FileLines(t, b.tasksPath)
	rep, err := b.StatusSweep(true)
	if err != nil {
		t.Fatalf("apply on a clean board errored: %v", err)
	}
	if rep.OffBefore != 0 || rep.OffAfter != 0 {
		t.Errorf("clean board census = before %d / after %d, want 0/0", rep.OffBefore, rep.OffAfter)
	}
	if len(rep.Normalized) != 0 || len(rep.Explicit) != 0 {
		t.Errorf("clean board reported rows: %+v / %+v", rep.Normalized, rep.Explicit)
	}
	after := bt066FileLines(t, b.tasksPath)
	if len(after) != len(before) {
		t.Fatalf("line count changed on a no-op apply: %d -> %d", len(before), len(after))
	}
	for i := range before {
		if !bytes.Equal(before[i], after[i]) {
			t.Errorf("no-op apply modified line %d:\nbefore %s\nafter  %s", i+1, before[i], after[i])
		}
	}
}
