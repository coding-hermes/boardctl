package board

import (
	"strings"
	"testing"
)

// SCHED-GAP-1572: guard_result/ci_result accept a structured form
// {"status":"pass|fail|skip|pending","details":"<prose>"} alongside the
// plain vocabulary string. The schema check must:
//
//   - accept the valid structured form SILENTLY (it is schema, not drift);
//   - accept extra keys on the object (rows carry per-run sub-records);
//   - itemize every INVALID shape with a diagnosis naming the exact rule
//     (missing status / status not a string / status off-vocabulary /
//     missing details / details not a string / not an object at all);
//   - keep the legacy BT-007 arm: a free-form PLAIN string still warns
//     (this file pins only the new arm; bt049_result_vocab_test.go pins
//     the string arm);
//   - never ERROR — the class stays exit-0 like every cross-check;
//   - stay silent on absent/null/empty (never-run rows are not results).
//
// The migrate-to-structured pattern is what the SCHED-GAP-1572 board
// migration wrote: legacy prose became {"status":"pending","details":prose}.
func seed1572Board(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id":"S1572-OBJ-OK","title":"valid structured form","status":"pending","priority":"P2","ci_result":{"status":"pending","details":"pending CI poll"},"created_at":"2026-09-01 00:00:00"}` + "\n" +
			`{"id":"S1572-OBJ-EXTRA","title":"structured with extra keys","status":"pending","priority":"P2","guard_result":{"status":"pass","details":"guard PASS 4/4","runs":2},"created_at":"2026-09-01 00:00:00"}` + "\n" +
			`{"id":"S1572-OBJ-NOSTATUS","title":"object missing status","status":"pending","priority":"P2","ci_result":{"details":"orphan prose"},"created_at":"2026-09-01 00:00:00"}` + "\n" +
			`{"id":"S1572-OBJ-BADSTATUS","title":"status off-vocabulary","status":"pending","priority":"P2","ci_result":{"status":"GREENISH","details":"x"},"created_at":"2026-09-01 00:00:00"}` + "\n" +
			`{"id":"S1572-OBJ-NUMSTATUS","title":"status not a string","status":"pending","priority":"P2","ci_result":{"status":3,"details":"x"},"created_at":"2026-09-01 00:00:00"}` + "\n" +
			`{"id":"S1572-OBJ-NODETAILS","title":"object missing details","status":"pending","priority":"P2","guard_result":{"status":"skip"},"created_at":"2026-09-01 00:00:00"}` + "\n" +
			`{"id":"S1572-OBJ-NUMDETAILS","title":"details not a string","status":"pending","priority":"P2","guard_result":{"status":"fail","details":42},"created_at":"2026-09-01 00:00:00"}` + "\n" +
			`{"id":"S1572-NUMBER","title":"number where a result belongs","status":"pending","priority":"P2","ci_result":7,"created_at":"2026-09-01 00:00:00"}` + "\n" +
			`{"id":"S1572-ARRAY","title":"array where a result belongs","status":"pending","priority":"P2","ci_result":["GREEN"],"created_at":"2026-09-01 00:00:00"}` + "\n" +
			`{"id":"S1572-SILENT-ABSENT","title":"results never run","status":"pending","priority":"P2","created_at":"2026-09-01 00:00:00"}` + "\n" +
			`{"id":"S1572-SILENT-NULL","title":"explicit null result","status":"pending","priority":"P2","guard_result":null,"created_at":"2026-09-01 00:00:00"}` + "\n" +
			`{"id":"S1572-SILENT-EMPTY","title":"empty string result","status":"pending","priority":"P2","ci_result":"","created_at":"2026-09-01 00:00:00"}` + "\n" +
			`{"id":"S1572-SILENT-CANON","title":"canonical string still fine","status":"pending","priority":"P2","guard_result":"pass","ci_result":"GREEN","created_at":"2026-09-01 00:00:00"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Test1572StructuredResultAccepted: the valid structured form (with or
// without extra keys) is schema-conformant — silent, no new warning class.
func Test1572StructuredResultAccepted(t *testing.T) {
	b := seed1572Board(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"S1572-OBJ-OK", "S1572-OBJ-EXTRA", "S1572-SILENT-ABSENT", "S1572-SILENT-NULL", "S1572-SILENT-EMPTY", "S1572-SILENT-CANON"} {
		if got := findWarn(rep, id); got != "" {
			t.Errorf("%s must not warn, got: %s", id, got)
		}
	}
	if rep.HasErrors() {
		t.Fatalf("result-schema drift must warn, not error: %+v", rep.Findings)
	}
}

// Test1572InvalidShapesItemized: every invalid shape warns with a
// schema-specific diagnosis (never the generic free-form message — the
// value is not a string, so "free-form" would be a lie about the shape).
func Test1572InvalidShapesItemized(t *testing.T) {
	b := seed1572Board(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		id      string
		wantSub []string
		notSub  string // must NOT appear: the free-form message is string-only
	}{
		{id: "S1572-OBJ-NOSTATUS", wantSub: []string{"fails the result-value schema", `missing "status"`}, notSub: "is free-form"},
		{id: "S1572-OBJ-BADSTATUS", wantSub: []string{"fails the result-value schema", "not in vocabulary {pass,fail,skip,pending}"}, notSub: "is free-form"},
		{id: "S1572-OBJ-NUMSTATUS", wantSub: []string{"fails the result-value schema", `"status" is not a string`}, notSub: "is free-form"},
		{id: "S1572-OBJ-NODETAILS", wantSub: []string{"fails the result-value schema", `missing "details"`}, notSub: "is free-form"},
		{id: "S1572-OBJ-NUMDETAILS", wantSub: []string{"fails the result-value schema", `"details" is not a string`}, notSub: "is free-form"},
		{id: "S1572-NUMBER", wantSub: []string{"fails the result-value schema", "neither a vocabulary string nor a {status, details} object"}, notSub: "is free-form"},
		{id: "S1572-ARRAY", wantSub: []string{"fails the result-value schema", "neither a vocabulary string nor a {status, details} object"}, notSub: "is free-form"},
	}
	for _, c := range cases {
		got := findWarn(rep, c.id)
		if got == "" {
			t.Errorf("%s: validate did not warn; findings: %+v", c.id, rep.Findings)
			continue
		}
		for _, sub := range c.wantSub {
			if !strings.Contains(got, sub) {
				t.Errorf("%s: warning must mention %q, got: %s", c.id, sub, got)
			}
		}
		if strings.Contains(got, c.notSub) {
			t.Errorf("%s: non-string value must not be called \"free-form\", got: %s", c.id, got)
		}
		if !strings.Contains(got, "guard_result") && !strings.Contains(got, "ci_result") {
			t.Errorf("%s: warning must name the field, got: %s", c.id, got)
		}
	}
}
