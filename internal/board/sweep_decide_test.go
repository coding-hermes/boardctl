package board

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// ---------- REVIEW-BOARDCTL-001 --decide: the decision-table unit tests ----------
//
// Every row of the DecideResultValue table is pinned here, including the
// refusal arms — a change to the table that is not reflected in this file
// (or in ResultDecideRuleDoc, cross-checked below) cannot land silently.

// TestDecideResultValueTable pins the decided=true arms: fleet prose in, one
// canonical vocabulary value out, per column.
func TestDecideResultValueTable(t *testing.T) {
	cases := []struct {
		column string
		raw    string
		want   string // canonical value; "" means refused (decided=false)
	}{
		// First-token rules (the census's dominant shapes).
		{"guard_result", "PASS 5/5", "PASS"},
		{"guard_result", "PASS (secrets)", "PASS"},
		{"guard_result", "pass 5/5", "PASS"}, // case tolerance on the rule input
		{"guard_result", "OK (smoke only)", "PASS"},
		{"guard_result", "FAIL 2/5", "FAIL"},
		{"guard_result", "ERROR: daemon did not boot", "FAIL"},
		{"ci_result", "GREEN first time", "GREEN"},
		{"ci_result", "RED — 2 jobs failed", "RED"},
		{"guard_result", "(PASS) after retry", "PASS"}, // punctuation-stripped lead
		// Anywhere rules.
		{"guard_result", "not run, SKIP recorded by hand", "SKIP"},
		{"guard_result", "N/A (bash script; local bash -n + check PASS)", "SKIP"},
		{"ci_result", "n/a — no CI for this lane", "SKIP"},
		{"ci_result", "pipeline 1166 SUCCESS all 6 jobs (lint GREEN first time)", "GREEN"},
		{"guard_result", "pipeline 1166 SUCCESS all 6 jobs", "PASS"}, // SUCCESS maps per column
		{"guard_result", "all ok per runbook", "PASS"},               // standalone OK maps too
		// Refusals (decided=false).
		{"guard_result", "PENDING", ""},                            // never rewritten
		{"ci_result", "pending", ""},                               // case-insensitive
		{"guard_result", "PASS but really FAIL", ""},               // ambiguous prose
		{"ci_result", "first FAIL then PASS on rerun", ""},         // ambiguous, either order
		{"guard_result", "watch the PASSED output later", ""},      // PASSED is not PASS
		{"guard_result", "pipeline finished, verdict unclear", ""}, // unmatched
		{"guard_result", "", ""},                                   // empty is not a result
	}
	for _, tc := range cases {
		got, decided, reason := DecideResultValue(tc.column, tc.raw)
		if tc.want == "" {
			if decided {
				t.Errorf("DecideResultValue(%q, %q) decided %q — want refusal", tc.column, tc.raw, got)
			}
			if reason == "" {
				t.Errorf("DecideResultValue(%q, %q) refused without a reason", tc.column, tc.raw)
			}
			continue
		}
		if !decided {
			t.Errorf("DecideResultValue(%q, %q) refused (%s) — want %q", tc.column, tc.raw, reason, tc.want)
			continue
		}
		vocab := GuardResultVocabulary
		if tc.column == "ci_result" {
			vocab = CIResultVocabulary
		}
		if !vocab[got] {
			t.Errorf("DecideResultValue(%q, %q) = %q — outside the %s vocabulary", tc.column, tc.raw, got, tc.column)
		}
		if got != tc.want {
			t.Errorf("DecideResultValue(%q, %q) = %q, want %q", tc.column, tc.raw, got, tc.want)
		}
	}
}

// TestDecideResultValueSUCCESSColumnSplit: SUCCESS alone maps per column —
// GREEN for ci_result, PASS for guard_result — and PENDING never maps.
func TestDecideResultValueSUCCESSColumnSplit(t *testing.T) {
	if got, ok, _ := DecideResultValue("ci_result", "SUCCESS"); !ok || got != "GREEN" {
		t.Errorf("ci SUCCESS = %q ok=%v, want GREEN", got, ok)
	}
	if got, ok, _ := DecideResultValue("guard_result", "SUCCESS"); !ok || got != "PASS" {
		t.Errorf("guard SUCCESS = %q ok=%v, want PASS", got, ok)
	}
	if _, ok, _ := DecideResultValue("ci_result", "still PENDING in the pipeline"); ok {
		t.Error("PENDING must never be decided (the check has not run)")
	}
}

// TestResultDecideRuleDocMatchesTable: the help-text table and the function
// agree — every decision keyword the doc names is one the function acts on,
// and the doc names PENDING as a non-decision.
func TestResultDecideRuleDocMatchesTable(t *testing.T) {
	for _, keyword := range []string{"PASS", "OK", "FAIL", "ERROR", "GREEN", "RED", "SKIP", "N/A", "SUCCESS", "PENDING"} {
		if !strings.Contains(ResultDecideRuleDoc, keyword) {
			t.Errorf("ResultDecideRuleDoc does not name %q:\n%s", keyword, ResultDecideRuleDoc)
		}
	}
	// Spot-check the function against the doc's headline claims.
	if _, ok, _ := DecideResultValue("guard_result", "PASS x"); !ok {
		t.Error("doc claims first-token PASS decides; function refuses")
	}
	if _, ok, _ := DecideResultValue("guard_result", "PENDING"); ok {
		t.Error("doc claims PENDING stays explicit; function decided it")
	}
}

// ---------- --decide sweep integration (internal/board level) ----------

// seedDecideBoard plants a board exercising every census example class and
// resolves it (same shape as seedBT066Board).
func seedDecideBoard(t *testing.T, taskLines []string) *Board {
	t.Helper()
	b := seedBT066Board(t, taskLines)
	return b
}

// TestStatusSweepDecideDryRunClassification: the census prose examples
// classify exactly as designed, dry-run — decided rows are Normalized
// (carrying column, value and rule), PENDING/ambiguous stay Explicit, a
// duplicate with a twin is planned for supersede, a duplicate without a
// twin is refused — and NOTHING is written.
func TestStatusSweepDecideDryRunClassification(t *testing.T) {
	b := seedDecideBoard(t, []string{
		`{"id":"DEC-PASS-PROSE","title":"prose pass","status":"complete","priority":"P2","guard_result":"PASS 5/5"}` + "\n",
		`{"id":"DEC-PIPELINE","title":"pipeline success","status":"review","priority":"P1","ci_result":"pipeline 1166 SUCCESS all 6 jobs (lint GREEN first time)"}` + "\n",
		`{"id":"DEC-NA","title":"bash only","status":"complete","priority":"P3","guard_result":"N/A (bash script; local bash -n + check PASS)"}` + "\n",
		`{"id":"DEC-PENDING","title":"not run yet","status":"pending","priority":"P2","ci_result":"PENDING"}` + "\n",
		`{"id":"DEC-AMBIG","title":"conflicting prose","status":"blocked","priority":"P2","guard_result":"PASS but really FAIL"}` + "\n",
		`{"id":"DEC-DUP","title":"[P2] header-frozen on asce main","reasoning":"cell detail: header never advances","status":"pending","priority":"P2"}` + "\n",
		`{"id":"DEC-DUP","title":"[P2] the recycled duplicate filing","reasoning":"cell detail: second finding, same id","status":"duplicate","priority":"P2"}` + "\n",
		`{"id":"DEC-LONEDUP","title":"a duplicate with no twin","status":"duplicate","priority":"P3"}` + "\n",
		`{"id":"DEC-CLEAN-ALIAS","title":"plain alias row","status":"todo","priority":"P2"}` + "\n",
	})
	before := bt066FileLines(t, b.tasksPath)

	rep, err := b.StatusSweepDecide(false, true)
	if err != nil {
		t.Fatalf("decide dry-run errored: %v", err)
	}

	// Decided: PASS prose, pipeline SUCCESS, N/A, the twin duplicate — plus
	// the plain alias row through the unchanged normalize path.
	decided := map[string]StatusSweepRow{}
	for _, e := range rep.Normalized {
		decided[e.ID] = e
	}
	for id, wantCol := range map[string]string{
		"DEC-PASS-PROSE": "guard_result",
		"DEC-PIPELINE":   "ci_result",
		"DEC-NA":         "guard_result",
	} {
		e, hit := decided[id]
		if !hit {
			t.Errorf("%s not classified fixable: %+v", id, rep.Normalized)
			continue
		}
		if e.Decided != wantCol {
			t.Errorf("%s Decided = %q, want %q", id, e.Decided, wantCol)
		}
		if e.DecidedValue == "" || e.Reason == "" {
			t.Errorf("%s missing decided value or rule reason: %+v", id, e)
		}
	}
	if e := decided["DEC-DUP"]; e.Decided != "status" || e.DupKeptLine != 6 || e.DupSupersede == nil || !*e.DupSupersede {
		t.Errorf("DEC-DUP not planned as superseded-by-earliest (kept line 6): %+v", e)
	}
	if e := decided["DEC-CLEAN-ALIAS"]; e.Canonical != "pending" || e.Decided != "" {
		t.Errorf("plain alias row must ride the normalize path unchanged: %+v", e)
	}
	if len(rep.Normalized) != 5 {
		t.Errorf("Normalized = %d rows, want 5: %+v", len(rep.Normalized), rep.Normalized)
	}

	// Explicit: PENDING, the ambiguous prose, the twin-less duplicate.
	explicit := map[string]StatusSweepRow{}
	for _, e := range rep.Explicit {
		explicit[e.ID] = e
	}
	for _, id := range []string{"DEC-PENDING", "DEC-AMBIG", "DEC-LONEDUP"} {
		if _, hit := explicit[id]; !hit {
			t.Errorf("%s must stay explicit: %+v", id, rep.Explicit)
		}
	}
	if e := explicit["DEC-PENDING"]; e.Blocked != "ci_result" || e.BlockedValue != "PENDING" {
		t.Errorf("PENDING blocked fields wrong: %+v", e)
	}
	if e := explicit["DEC-LONEDUP"]; !strings.Contains(e.Action, "NO earlier same-id twin") {
		t.Errorf("twin-less duplicate must name the refusal cause: %+v", e)
	}
	if len(rep.Explicit) != 3 {
		t.Errorf("Explicit = %d rows, want 3: %+v", len(rep.Explicit), rep.Explicit)
	}
	if rep.OffBefore != 8 {
		t.Errorf("OffBefore = %d, want 8", rep.OffBefore)
	}

	// Dry-run: byte-identical file.
	after := bt066FileLines(t, b.tasksPath)
	if len(before) != len(after) {
		t.Fatalf("dry-run changed the line count: %d -> %d", len(before), len(after))
	}
	for i := range before {
		if !bytesEqual(before[i], after[i]) {
			t.Errorf("decide dry-run rewrote line %d:\nbefore %s\nafter  %s", i+1, before[i], after[i])
		}
	}
}

// TestStatusSweepDecideApplyWrites: --decide --apply writes every decided
// row through its sanctioned path — result values via the vocabulary-gated
// writer, the duplicate closed machine-readably with superseded_by + an
// audit event quoting the preserved title — while PENDING, the ambiguous
// prose and the twin-less duplicate stay byte-identical and still explicit.
// A second run reports 0 remaining for the decided rows (idempotent).
func TestStatusSweepDecideApplyWrites(t *testing.T) {
	b := seedDecideBoard(t, []string{
		`{"id":"DEC-PASS-PROSE","title":"prose pass","status":"complete","priority":"P2","guard_result":"PASS 5/5"}` + "\n",
		`{"id":"DEC-PIPELINE","title":"pipeline success","status":"review","priority":"P1","ci_result":"pipeline 1166 SUCCESS all 6 jobs (lint GREEN first time)"}` + "\n",
		`{"id":"DEC-PENDING","title":"not run yet","status":"pending","priority":"P2","ci_result":"PENDING"}` + "\n",
		`{"id":"DEC-DUP","title":"[P2] header-frozen on asce main","reasoning":"cell detail: header never advances","status":"pending","priority":"P2"}` + "\n",
		`{"id":"DEC-DUP","title":"[P2] the recycled duplicate filing","reasoning":"cell detail: second finding, same id","status":"duplicate","priority":"P2"}` + "\n",
	})
	dupBefore := string(bt066FileLines(t, b.tasksPath)[4]) // the duplicate line, byte-pinned

	rep, err := b.StatusSweepDecide(true, true)
	if err != nil {
		t.Fatalf("decide apply errored: %v", err)
	}

	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string][]*Row{}
	for _, r := range rows {
		id := r.String("id")
		byID[id] = append(byID[id], r)
	}
	if g := byID["DEC-PASS-PROSE"][0].String("guard_result"); g != "PASS" {
		t.Errorf("DEC-PASS-PROSE guard_result = %q, want PASS", g)
	}
	if c := byID["DEC-PIPELINE"][0].String("ci_result"); c != "GREEN" {
		t.Errorf("DEC-PIPELINE ci_result = %q, want GREEN", c)
	}
	if p := byID["DEC-PENDING"][0].String("ci_result"); p != "PENDING" {
		t.Errorf("DEC-PENDING ci_result was rewritten: %q (must never be)", p)
	}
	dup := byID["DEC-DUP"][1]
	if dup.String("status") != "complete" || dup.String("superseded_by") != "DEC-DUP" {
		t.Errorf("duplicate not closed machine-readably: status=%q superseded_by=%q", dup.String("status"), dup.String("superseded_by"))
	}
	if !strings.Contains(dup.String("worker_summary"), "superseded-by-earliest") || !strings.Contains(dup.String("worker_summary"), "kept line 4") {
		t.Errorf("duplicate close summary wrong: %q", dup.String("worker_summary"))
	}
	if dup.String("title") != "[P2] the recycled duplicate filing" || !strings.Contains(dup.String("reasoning"), "second finding, same id") {
		t.Errorf("closed duplicate lost its original text: %+v", dup)
	}
	// the kept twin is untouched and still open
	if kept := byID["DEC-DUP"][0]; kept.String("status") != "pending" || kept.String("superseded_by") != "" {
		t.Errorf("kept twin was modified: %+v", kept)
	}
	// the closed line is NOT byte-identical (it was repaired), and the audit
	// event quotes the preserved title verbatim.
	afterLines := bt066FileLines(t, b.tasksPath)
	if string(afterLines[4]) == dupBefore {
		t.Error("duplicate line still byte-identical after apply")
	}
	events, _, err := ReadAllRows(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, ev := range events[1:] {
		if ev.String("event_type") != "audit" || ev.String("task_id") != "DEC-DUP" {
			continue
		}
		var d struct {
			Action   string `json:"action"`
			Title    string `json:"title"`
			ClosedAs string `json:"closed_as"`
		}
		if err := json.Unmarshal([]byte(ev.String("detail")), &d); err != nil {
			t.Fatalf("audit detail does not parse: %v", err)
		}
		if d.Action == "id-dupe" && d.Title == "[P2] the recycled duplicate filing" && d.ClosedAs == "superseded-by-earliest" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no audit event quoting the preserved duplicate text:\n%s", mustRead(t, b.EventsPath()))
	}

	// The census is honest: exactly the PENDING row remains off-vocabulary.
	if rep.OffAfter != 1 {
		t.Errorf("OffAfter = %d, want 1 (PENDING stays explicit)", rep.OffAfter)
	}
	if len(rep.Explicit) != 1 || rep.Explicit[0].ID != "DEC-PENDING" {
		t.Errorf("apply report explicit rows wrong: %+v", rep.Explicit)
	}

	// Idempotent: a second decide-apply is a byte-identical no-op with the
	// decided rows no longer reported.
	snap1 := idDupeBoardSnapshot(t, b)
	rep2, err := b.StatusSweepDecide(true, true)
	if err != nil {
		t.Fatalf("second decide apply errored: %v", err)
	}
	if rep2.OffAfter != 1 || len(rep2.Normalized) != 0 {
		t.Errorf("second run = %d fixable / %d off after, want 0 fixable / 1 off: %+v", len(rep2.Normalized), rep2.OffAfter, rep2)
	}
	snap2 := idDupeBoardSnapshot(t, b)
	for p, h := range snap1 {
		if snap2[p] != h {
			t.Errorf("second decide apply rewrote %s", p)
		}
	}
}

// TestStatusSweepDecideOffLeavesExplicitRowsUntouchable: WITHOUT --decide the
// sweep behaves exactly as before BT-066+decide — the same prose rows stay
// explicit even under --apply, and nothing but alias rows is written.
func TestStatusSweepDecideOffLeavesExplicitRowsUntouchable(t *testing.T) {
	b := seedDecideBoard(t, []string{
		`{"id":"DEC-OFF-PROSE","title":"prose pass","status":"pending","priority":"P2","guard_result":"PASS 5/5"}` + "\n",
		`{"id":"DEC-OFF-ALIAS","title":"alias row","status":"todo","priority":"P2"}` + "\n",
	})
	rep, err := b.StatusSweep(true) // the legacy two-state entry point
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Normalized) != 1 || rep.Normalized[0].ID != "DEC-OFF-ALIAS" {
		t.Errorf("legacy sweep normalized rows wrong: %+v", rep.Normalized)
	}
	if len(rep.Explicit) != 1 || rep.Explicit[0].ID != "DEC-OFF-PROSE" {
		t.Errorf("legacy sweep must keep prose rows explicit: %+v", rep.Explicit)
	}
	// and --apply still refused the prose row byte-identically
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].String("guard_result") != "PASS 5/5" {
		t.Errorf("legacy apply rewrote the prose row: %q", rows[0].String("guard_result"))
	}
	_ = os.Getenv // keep os imported for symmetry with sibling tests
}

// bytesEqual is a local alias so the test file does not import bytes twice
// alongside os for one comparison.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestCloseIDDupeLineQuotesDuplicateStatusWas: the per-line close's audit
// event quotes the PRE-close status verbatim — for a --decide duplicate
// close that is exactly the string "duplicate", the machine-readable proof
// the closed line was the recycled duplicate. Also pins the keep-side
// refusal (the earliest line of an id can never close itself).
func TestCloseIDDupeLineQuotesDuplicateStatusWas(t *testing.T) {
	b := seedIDDupeBoard(t,
		`{"id":"QA-DEC-1","title":"[P2] the kept finding","reasoning":"cell: kept","status":"pending","priority":"P2"}`,
		`{"id":"QA-DEC-1","title":"[P2] the recycled duplicate","reasoning":"cell: dup","status":"duplicate","priority":"P2"}`,
	)
	if err := b.CloseIDDupeLine(2); err != nil {
		t.Fatalf("close refused: %v", err)
	}
	events, _, err := ReadAllRows(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2 (seed + one audit)", len(events))
	}
	var d struct {
		Action    string `json:"action"`
		StatusWas string `json:"status_was"`
		Title     string `json:"title"`
		KeptLine  int    `json:"kept_line"`
	}
	if err := json.Unmarshal([]byte(events[1].String("detail")), &d); err != nil {
		t.Fatal(err)
	}
	if d.Action != "id-dupe" || d.StatusWas != "duplicate" || d.Title != "[P2] the recycled duplicate" || d.KeptLine != 1 {
		t.Errorf("audit event wrong: %+v", d)
	}
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	if rows[1].String("status") != "complete" || rows[1].String("superseded_by") != "QA-DEC-1" {
		t.Errorf("closed row wrong: %+v", rows[1])
	}
	// The keep-side must refuse to close itself — nothing written.
	snap := idDupeBoardSnapshot(t, b)
	if err := b.CloseIDDupeLine(1); err == nil {
		t.Fatal("closing the kept line must be refused")
	}
	for p, h := range snap {
		if h != idDupeBoardSnapshot(t, b)[p] {
			t.Errorf("refused close still wrote %s", p)
		}
	}
}

// TestSetResultValueVocabGate: the --decide writer keeps the BT-007 gate —
// only canonical vocabulary members are writable, unknown ids refuse,
// unknown columns refuse — and a successful write leaves every other line
// byte-identical.
func TestSetResultValueVocabGate(t *testing.T) {
	b := seedBT066Board(t, []string{
		`{"id":"SET-1","title":"target","status":"pending","priority":"P2","guard_result":"PASS 5/5"}` + "\n",
		`{"id":"SET-2","title":"bystander","status":"pending","priority":"P2"}` + "\n",
	})
	before := bt066FileLines(t, b.tasksPath)
	for _, tc := range []struct{ column, value string }{
		{"guard_result", "PENDING"}, {"guard_result", "MAYBE"}, {"ci_result", "PASS"}, {"guard_result", ""},
	} {
		if err := b.SetResultValue("SET-1", tc.column, tc.value); err == nil {
			t.Errorf("SetResultValue(%q, %q) must refuse (vocab gate)", tc.column, tc.value)
		}
	}
	if err := b.SetResultValue("SET-NOPE", "guard_result", "PASS"); err == nil {
		t.Error("SetResultValue on a missing id must refuse")
	}
	if err := b.SetResultValue("SET-1", "status", "pending"); err == nil {
		t.Error("SetResultValue must refuse non-result columns")
	}
	if err := b.SetResultValue("SET-1", "guard_result", "PASS"); err != nil {
		t.Fatalf("canonical write refused: %v", err)
	}
	after := bt066FileLines(t, b.tasksPath)
	if len(after) != len(before) {
		t.Fatalf("line count changed: %d -> %d", len(before), len(after))
	}
	if !bytesEqual(before[1], after[1]) {
		t.Errorf("bystander line was touched:\nbefore %s\nafter  %s", before[1], after[1])
	}
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].String("guard_result") != "PASS" {
		t.Errorf("guard_result = %q, want PASS", rows[0].String("guard_result"))
	}
}
