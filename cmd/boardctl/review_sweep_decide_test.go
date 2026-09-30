package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- REVIEW-BOARDCTL-001 --decide: CLI-level sweep-status tests ----------
//
// Acceptance A–C and E, driven through run() on fixture boards: the decide
// report classifies the fleet prose examples, --decide --apply writes them,
// PENDING is never rewritten, the duplicate close is machine-readable with
// an audit event, the dry-run writes nothing, and the legacy (no --decide)
// behavior is byte-for-byte unchanged.

// seedDecideCLIBoard plants the acceptance-A fixture board (same prose
// examples the fleet census quoted) plus one duplicate-with-twin pair and
// one twin-less duplicate.
func seedDecideCLIBoard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"tasks.jsonl": `{"id":"AC-PASS","title":"prose pass","status":"complete","priority":"P2","guard_result":"PASS 5/5"}` + "\n" +
			`{"id":"AC-SECRETS","title":"secrets pass","status":"review","priority":"P1","guard_result":"PASS (secrets)"}` + "\n" +
			`{"id":"AC-PIPE","title":"pipeline","status":"review","priority":"P1","ci_result":"pipeline 1166 SUCCESS all 6 jobs (lint GREEN first time)"}` + "\n" +
			`{"id":"AC-NA","title":"bash only","status":"complete","priority":"P3","guard_result":"N/A (bash script; local bash -n + check PASS)"}` + "\n" +
			`{"id":"AC-PENDING","title":"not run yet","status":"pending","priority":"P2","ci_result":"PENDING"}` + "\n" +
			`{"id":"AC-DUP","title":"[P2] the kept finding","reasoning":"cell: kept finding text","status":"pending","priority":"P2"}` + "\n" +
			`{"id":"AC-DUP","title":"[P2] the recycled duplicate","reasoning":"cell: duplicate finding text","status":"duplicate","priority":"P2"}` + "\n" +
			`{"id":"AC-LONE","title":"a duplicate with no twin","status":"duplicate","priority":"P3"}` + "\n" +
			`{"id":"AC-ALIAS","title":"plain alias row","status":"todo","priority":"P2"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"demo","namespace":"demo","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(boardDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// tasksAndEvents re-reads the fixture board's two written files.
func tasksAndEvents(t *testing.T, dir string) (string, string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	ev, err := os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), string(ev)
}

// TestCmdSweepStatusDecideDryRun: `sweep-status --decide` reports the
// intended decisions for every census example, refuses the ambiguous and
// twin-less rows, and writes NOTHING (byte-compare of both files).
func TestCmdSweepStatusDecideDryRun(t *testing.T) {
	dir := seedDecideCLIBoard(t)
	tasksBefore, eventsBefore := tasksAndEvents(t, dir)
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "sweep-status", "--decide"}); code != 0 {
			t.Fatalf("sweep-status --decide exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	// decided rows, with the rule that fired
	for _, want := range []string{
		`AC-PASS: status "complete" — --decide: guard_result "PASS 5/5" -> "PASS"`,
		`AC-SECRETS: status "review" — --decide: guard_result "PASS (secrets)" -> "PASS"`,
		`AC-PIPE: status "review" — --decide: ci_result "pipeline 1166 SUCCESS all 6 jobs (lint GREEN first time)" -> "GREEN"`,
		`AC-NA: status "complete" — --decide: guard_result "N/A (bash script; local bash -n + check PASS)" -> "SKIP"`,
		`AC-DUP: status "duplicate" — --decide: duplicate status closes as superseded-by-earliest (kept line 6`,
		`AC-ALIAS: status "todo" — --normalize`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("decide dry-run missing %q:\n%s", want, got)
		}
	}
	// refusals stay explicit
	for _, want := range []string{
		`AC-PENDING: status "pending" — needs explicit decision: ci_result "PENDING"`,
		`AC-LONE: status "duplicate" — --decide: status "duplicate" has NO earlier same-id twin`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("decide dry-run missing the explicit refusal %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "DRY-RUN") {
		t.Fatalf("dry-run banner missing:\n%s", got)
	}
	tasksAfter, eventsAfter := tasksAndEvents(t, dir)
	if tasksBefore != tasksAfter || eventsBefore != eventsAfter {
		t.Fatalf("decide dry-run wrote files:\ntasks %q -> %q\nevents %q -> %q", tasksBefore, tasksAfter, eventsBefore, eventsAfter)
	}
}

// TestCmdSweepStatusDecideHelpDocumentsTable: --help carries the exact
// decision table (criterion A's "documented in the help text").
func TestCmdSweepStatusDecideHelpDocumentsTable(t *testing.T) {
	got, err := captureStdout(func() {
		if code := run([]string{"-C", t.TempDir(), "sweep-status", "--help"}); code != 0 {
			t.Fatalf("sweep-status --help exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--decide", "PASS|OK", "FAIL|ERROR", "GREEN", "RED", "N/A", "SUCCESS", "PENDING", "superseded-by-earliest"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help text missing %q:\n%s", want, got)
		}
	}
}

// TestCmdSweepStatusDecideApply: --decide --apply writes the decided values
// and the duplicate close (criterion A), NEVER touches PENDING (criterion
// B), closes the twin duplicate machine-readably with an audit event
// quoting the preserved text (criterion C), and a re-run reports 0
// remaining for the decided rows.
func TestCmdSweepStatusDecideApply(t *testing.T) {
	dir := seedDecideCLIBoard(t)
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "sweep-status", "--decide", "--apply"}); code != 0 {
			t.Fatalf("sweep-status --decide --apply exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "before: 8 off-vocabulary rows") || !strings.Contains(got, "after: 2 off-vocabulary rows") {
		t.Fatalf("apply census wrong (want before 8 / after 2 — PENDING + twin-less duplicate):\n%s", got)
	}
	tasks, events := tasksAndEvents(t, dir)
	// decided values landed, canonical only
	for _, want := range []string{`"guard_result":"PASS"`, `"ci_result":"GREEN"`, `"guard_result":"SKIP"`} {
		if !strings.Contains(tasks, want) {
			t.Fatalf("tasks.jsonl missing %q:\n%s", want, tasks)
		}
	}
	// criterion B: PENDING never rewritten, still on disk
	if !strings.Contains(tasks, `"ci_result":"PENDING"`) {
		t.Fatalf("criterion B violated — PENDING was rewritten:\n%s", tasks)
	}
	// criterion C: the duplicate closed machine-readably, original text kept
	if !strings.Contains(tasks, `"status":"complete","priority":"P2","superseded_by":"AC-DUP"`) &&
		!strings.Contains(tasks, `"superseded_by":"AC-DUP"`) {
		t.Fatalf("criterion C violated — no superseded_by marker:\n%s", tasks)
	}
	if !strings.Contains(tasks, `"title":"[P2] the recycled duplicate"`) {
		t.Fatalf("criterion C violated — the duplicate's original title was not preserved:\n%s", tasks)
	}
	if !strings.Contains(tasks, `superseded-by-earliest: recycled task id — kept line 6`) {
		t.Fatalf("criterion C violated — close summary missing:\n%s", tasks)
	}
	// criterion C: the audit event quotes the preserved text. The detail is
	// stored as a JSON-encoded string, so quote-literal assertions must be
	// escaping-agnostic — match the verbatim fragments, not the raw quotes.
	if !strings.Contains(events, "the recycled duplicate") || !strings.Contains(events, "status_was") || !strings.Contains(events, "superseded-by-earliest") {
		t.Fatalf("criterion C violated — audit event does not quote the preserved text:\n%s", events)
	}
	if !strings.Contains(events, `"task_id":"AC-DUP"`) || !strings.Contains(events, "cell: duplicate finding text") {
		t.Fatalf("criterion C violated — audit event not bound to AC-DUP with its reasoning:\n%s", events)
	}
	// the twin-less duplicate and the kept twin are untouched
	if !strings.Contains(tasks, `"id":"AC-LONE","title":"a duplicate with no twin","status":"duplicate"`) {
		t.Fatalf("twin-less duplicate was modified:\n%s", tasks)
	}

	// Re-run: the decided rows are gone from the report (criterion A's
	// "re-run shows 0 remaining explicit for those rows"), the two refusals
	// remain, and a second --decide --apply writes nothing.
	got2, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "sweep-status", "--decide"}); code != 0 {
			t.Fatalf("re-run exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got2, "AC-PASS") || strings.Contains(got2, "AC-PIPE") || strings.Contains(got2, "AC-DUP") {
		t.Fatalf("re-run still reports decided rows:\n%s", got2)
	}
	if !strings.Contains(got2, "2/9 rows off-vocabulary: 0 fixable by --normalize, 2 need explicit decision") {
		t.Fatalf("re-run census wrong:\n%s", got2)
	}
	tasks2Before, events2Before := tasksAndEvents(t, dir)
	if _, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "sweep-status", "--decide", "--apply"}); code != 0 {
			t.Fatalf("second apply exit = %d, want 0", code)
		}
	}); err != nil {
		t.Fatal(err)
	}
	tasks2After, events2After := tasksAndEvents(t, dir)
	if tasks2Before != tasks2After || events2Before != events2After {
		t.Fatal("second --decide --apply rewrote files (idempotence violated)")
	}
}

// TestCmdSweepStatusNoDecideUnchanged: WITHOUT --decide the CLI behaves
// exactly as before — prose rows stay explicit even under --apply, which
// writes only the alias row (criterion E's unchanged legacy path).
func TestCmdSweepStatusNoDecideUnchanged(t *testing.T) {
	dir := seedDecideCLIBoard(t)
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "sweep-status", "--apply"}); code != 0 {
			t.Fatalf("legacy apply exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	// legacy census: 1 fixable (the alias) + 7 explicit; the duplicate rows
	// are plain unknown statuses here, never touched.
	if !strings.Contains(got, "1 fixable by --normalize, 7 need explicit decision") {
		t.Fatalf("legacy census wrong:\n%s", got)
	}
	tasks, _ := tasksAndEvents(t, dir)
	for _, forbidden := range []string{`"guard_result":"PASS"`, `"ci_result":"GREEN"`, `superseded_by`} {
		if strings.Contains(tasks, forbidden) {
			t.Fatalf("legacy apply wrote a decided value (%q) without --decide:\n%s", forbidden, tasks)
		}
	}
	if !strings.Contains(tasks, `"guard_result":"PASS 5/5"`) {
		t.Fatalf("legacy apply rewrote the prose row:\n%s", tasks)
	}
}
