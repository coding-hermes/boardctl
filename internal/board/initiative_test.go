package board

import (
	"strings"
	"testing"
)

// The initiative key exists so "scoop this programme of work" is a query, not a
// title grep. These tests pin the three things that makes true: the slug rule
// (a typo must fail loudly at write time, never silently create a second
// initiative), the filter (exact, case-insensitive, and never matching an
// unscoped row), and the rollup (deferred work is not actionable, unscoped rows
// are not invented into a bucket).

func TestValidateInitiativeSlug(t *testing.T) {
	good := []string{"lane-hooks", "xml-spec-graph", "a", "a1", "sched-gap-16964", strings.Repeat("a", 64)}
	for _, s := range good {
		if err := ValidateInitiativeSlug(s); err != nil {
			t.Errorf("ValidateInitiativeSlug(%q) = %v, want nil", s, err)
		}
	}
	bad := map[string]string{
		"":                      "empty",
		"Lane-Hooks":            "not lowercase",
		"lane hooks":            "space",
		"lane_hooks":            "underscore",
		"-lane":                 "leading hyphen",
		"lane-":                 "trailing hyphen",
		"lane--hooks":           "double hyphen",
		strings.Repeat("a", 65): "too long",
		"lane-hooks!":           "punctuation",
	}
	for s, why := range bad {
		if err := ValidateInitiativeSlug(s); err == nil {
			t.Errorf("ValidateInitiativeSlug(%q) = nil, want an error (%s)", s, why)
		}
	}
}

func TestInitiativeFilterIsExactAndIgnoresUnscoped(t *testing.T) {
	rows := []*Row{
		mustRow(t, `{"id":"A-1","status":"pending","initiative":"lane-hooks"}`),
		mustRow(t, `{"id":"A-2","status":"pending","initiative":"Lane-Hooks"}`),
		mustRow(t, `{"id":"A-3","status":"pending","initiative":"lane-hooks-v2"}`),
		mustRow(t, `{"id":"A-4","status":"pending"}`),
	}
	got := []string{}
	for _, r := range rows {
		if (TaskFilter{Initiative: "lane-hooks"}).Match(r, map[string]bool{}) {
			got = append(got, r.String("id"))
		}
	}
	// A-1 matches exactly; A-2 matches case-insensitively (same initiative
	// typed with caps); A-3 is a DIFFERENT initiative and must not appear (a
	// prefix match would silently scoop another programme's work); A-4 has no
	// key and must never match.
	want := []string{"A-1", "A-2"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("initiative filter matched %v, want %v", got, want)
	}
}

func TestInitiativesRollupCountsDeferredSeparately(t *testing.T) {
	rows := []*Row{
		mustRow(t, `{"id":"B-1","status":"pending","initiative":"remote-dispatch"}`),
		mustRow(t, `{"id":"B-2","status":"in_progress","initiative":"remote-dispatch"}`),
		mustRow(t, `{"id":"B-3","status":"pending","initiative":"remote-dispatch","deferred":true}`),
		mustRow(t, `{"id":"B-4","status":"complete","initiative":"remote-dispatch"}`),
		mustRow(t, `{"id":"B-5","status":"pending","initiative":"lane-hooks"}`),
		mustRow(t, `{"id":"B-6","status":"pending"}`), // unscoped
	}
	got := summarizeRows(rows)
	if len(got) != 2 {
		t.Fatalf("got %d initiatives, want 2 (unscoped rows must not form a bucket): %+v", len(got), got)
	}
	// ordered by total desc, then slug
	if got[0].Initiative != "remote-dispatch" {
		t.Fatalf("first initiative = %q, want remote-dispatch", got[0].Initiative)
	}
	rd := got[0]
	if rd.Total != 4 {
		t.Errorf("remote-dispatch total = %d, want 4", rd.Total)
	}
	if rd.Deferred != 1 {
		t.Errorf("remote-dispatch deferred = %d, want 1", rd.Deferred)
	}
	// actionable = pending + in_progress, minus the deferred one, and never
	// counting complete rows: B-1 pending + B-2 in_progress = 2.
	if rd.Actionable != 2 {
		t.Errorf("remote-dispatch actionable = %d, want 2 (B-1, B-2; B-3 deferred, B-4 complete)", rd.Actionable)
	}
	if got[1].Actionable != 1 {
		t.Errorf("lane-hooks actionable = %d, want 1", got[1].Actionable)
	}
}

// mustRow parses a JSONL line into a Row for the tests above.
func mustRow(t *testing.T, line string) *Row {
	t.Helper()
	r, err := ParseRow([]byte(line))
	if err != nil {
		t.Fatalf("ParseRow(%s): %v", line, err)
	}
	return r
}
