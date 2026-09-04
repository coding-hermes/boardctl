package board

import (
	"strings"
	"testing"
)

// findWarn returns the first warn-level finding whose message contains substr.
func findWarn(rep *Report, substr string) string {
	for _, f := range rep.Findings {
		if f.Level == "warn" && strings.Contains(f.Msg, substr) {
			return f.Msg
		}
	}
	return ""
}

// seedJunkBoard writes a board carrying the exact junk rows the BT-007
// dogfood findings describe: a free-form guard_result row and a dangling
// depends_on reference.
func seedJunkBoard(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id":"JUNK-1","title":"junk guard","status":"pending","priority":"P2","guard_result":"MAYBE","ci_result":"BANANA","depends_on":["GHOST-9"]}` + "\n" +
			`{"id":"OK-1","title":"clean","status":"pending","priority":"P2","guard_result":"PASS","ci_result":"GREEN","depends_on":["JUNK-1"]}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// BT-007: validate must FLAG (warn) out-of-vocab guard_result/ci_result
// values already sitting on boards — pre-fix, validate returned OK on these.
func TestValidateFlagsFreeFormGuardAndCI(t *testing.T) {
	b := seedJunkBoard(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if got := findWarn(rep, "guard_result"); got == "" || !strings.Contains(got, `"MAYBE"`) {
		t.Fatalf("validate did not flag free-form guard_result MAYBE; findings: %+v", rep.Findings)
	}
	if got := findWarn(rep, "ci_result"); got == "" || !strings.Contains(got, `"BANANA"`) {
		t.Fatalf("validate did not flag free-form ci_result BANANA; findings: %+v", rep.Findings)
	}
	// the in-vocab row must NOT be flagged
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "OK-1") && (strings.Contains(f.Msg, "guard_result") || strings.Contains(f.Msg, "ci_result")) {
			t.Fatalf("clean row OK-1 flagged: %s", f.Msg)
		}
	}
	// junk results are warnings, not errors — the board still validates OK
	if rep.HasErrors() {
		t.Fatalf("free-form legacy results must warn, not error: %+v", rep.Findings)
	}
}

// BT-007: validate must WARN on depends_on ids that reference no task row,
// itemizing the referencing row.
func TestValidateWarnsOnDanglingDependsOn(t *testing.T) {
	b := seedJunkBoard(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	got := findWarn(rep, "GHOST-9")
	if got == "" {
		t.Fatalf("validate did not flag dangling depends_on GHOST-9; findings: %+v", rep.Findings)
	}
	if !strings.Contains(got, "JUNK-1") {
		t.Fatalf("finding should name the referencing row JUNK-1, got: %s", got)
	}
	// the resolvable dependency (OK-1 -> JUNK-1) must not be flagged
	if got := findWarn(rep, "depends_on references nonexistent"); strings.Contains(got, "JUNK-1") && strings.Contains(got, `"JUNK-1"`) {
		t.Fatalf("existing dependency flagged as dangling: %s", got)
	}
}

// BT-007: validate must ERROR on negative header counters.
func TestValidateErrorsOnNegativeHeaderCounters(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"T-1","title":"t","status":"pending","priority":"P2"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":-5,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if !rep.HasErrors() {
		t.Fatalf("negative ticks_total must be an error; findings: %+v", rep.Findings)
	}
	found := false
	for _, f := range errorMsgs(rep) {
		if strings.Contains(f, "ticks_total") && strings.Contains(f, "negative") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no negative-counter error for ticks_total: %+v", rep.Findings)
	}
}

// BT-007: a healthy legacy board with lowercase (but in-vocab) results must
// not trip the guard/ci checks — case is tolerated on read.
func TestValidateToleratesLowercaseResults(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"T-1","title":"t","status":"pending","priority":"P2","guard_result":"pass","ci_result":"skip"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if got := findWarn(rep, "guard_result") + findWarn(rep, "ci_result"); got != "" {
		t.Fatalf("lowercase in-vocab results flagged: %s", got)
	}
}

// BT-007: doctor inherits the new checks via Validate() — a board with junk
// rows now fails doctor too (pre-fix doctor only caught ticks_idle > total).
func TestDoctorSurfacesVocabularyAndDepFindings(t *testing.T) {
	b := seedJunkBoard(t)
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	if findWarn(rep, "guard_result") == "" {
		t.Fatalf("doctor missing guard_result finding: %+v", rep.Findings)
	}
	if findWarn(rep, "GHOST-9") == "" {
		t.Fatalf("doctor missing dangling depends_on finding: %+v", rep.Findings)
	}
}
