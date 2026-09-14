package board

import (
	"strings"
	"testing"
)

// seedAliasBoard writes a topology-A board carrying one row per status
// spelling the BT-025 alias tests need (each task id encodes its spelling).
func seedAliasBoard(t *testing.T, statuses ...string) *Board {
	t.Helper()
	dir := t.TempDir()
	var tasks string
	for i, st := range statuses {
		tasks += `{"id":"ALIAS-` + string(rune('A'+i)) + `","title":"t","status":"` + st + `","priority":"P2"}` + "\n"
	}
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  tasks,
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// BT-025: the alias table covers every spelling named in the policy, all
// case-insensitive and whitespace-trimmed, and maps them to exactly one
// canonical value.
func TestStatusAliasesTable(t *testing.T) {
	cases := []struct {
		raw       string
		canonical string
	}{
		{"completed", "complete"},
		{"done", "complete"},
		{"todo", "pending"},
		{"open", "pending"},
		{"in-progress", "in_progress"},
		{"inprogress", "in_progress"},
		// the canonical spelling itself: resolves via the vocabulary with
		// alias=false (not drift), canonicalizing to itself either way
		{"in_progress", "in_progress"},
		{"reopen", "pending"},
		{"reopened", "pending"},
		// case / whitespace variants
		{"  TODO ", "pending"},
		{"Done", "complete"},
		{"COMPLETED", "complete"},
		{" In-Progress ", "in_progress"},
		{"INPROGRESS", "in_progress"},
		{"Reopened", "pending"},
	}
	for _, c := range cases {
		got := NormalizeStatus(c.raw)
		if got != c.canonical {
			t.Errorf("NormalizeStatus(%q) = %q, want %q", c.raw, got, c.canonical)
		}
		canonical, ok, alias := ResolveStatus(c.raw)
		if !ok || canonical != c.canonical {
			t.Errorf("ResolveStatus(%q) = (%q,%v,%v), want ok with canonical %q", c.raw, canonical, ok, alias, c.canonical)
		}
	}
	// every spelling the policy names as an ALIAS (in_progress is canonical
	// and therefore never drift) must classify as alias=true
	for _, alias := range []string{"completed", "done", "todo", "open", "in-progress", "inprogress", "reopen", "reopened"} {
		if _, _, isAlias := ResolveStatus(alias); !isAlias {
			t.Errorf("ResolveStatus(%q).alias = false, want true (policy alias)", alias)
		}
	}
}

// BT-025: canonical values resolve with alias=false; unknown values —
// explicitly retired, closed, wip, and malformed junk — stay unresolved and
// pass through NormalizeStatus unchanged.
func TestStatusResolveCanonicalAndUnknown(t *testing.T) {
	for _, st := range []string{"pending", "in_progress", "review", "blocked", "complete", "failed"} {
		canonical, ok, alias := ResolveStatus(st)
		if !ok || canonical != st || alias {
			t.Errorf("canonical %q resolved to (%q,%v,%v), want itself with alias=false", st, canonical, ok, alias)
		}
	}
	for _, st := range []string{"retired", "closed", "wip", `"pending`, "shipped", "in progress", " " /* whitespace-only */, "pending-ish"} {
		if got := NormalizeStatus(st); got != st {
			t.Errorf("NormalizeStatus(%q) = %q, want pass-through", st, got)
		}
		if _, ok, _ := ResolveStatus(st); ok {
			t.Errorf("unknown status %q unexpectedly resolved", st)
		}
	}
	// the empty status is its own "missing" case: ok, not an alias
	if canonical, ok, alias := ResolveStatus(""); !ok || canonical != "" || alias {
		t.Errorf(`ResolveStatus("") = (%q,%v,%v), want ("",true,false)`, canonical, ok, alias)
	}
}

// BT-025: every known alias on a board is a WARNING naming the canonical
// value and the normalize fix path — validate must exit OK (zero errors) on
// a board that only carries alias statuses.
func TestValidateAliasStatusesWarnNotError(t *testing.T) {
	b := seedAliasBoard(t, "completed", "done", "todo", "open", "in-progress", "inprogress", "reopen", "reopened")
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() {
		t.Fatalf("alias statuses must not error; findings: %+v", rep.Findings)
	}
	for _, st := range []string{"completed", "done", "todo", "open", "in-progress", "inprogress", "reopen", "reopened"} {
		found := false
		for _, f := range rep.Findings {
			if f.Level == "warn" && strings.Contains(f.Msg, `status "`+st+`"`) && strings.Contains(f.Msg, "read alias") && strings.Contains(f.Msg, "--normalize") {
				found = true
			}
		}
		if !found {
			t.Errorf("no alias warning naming status %q (with canonical value + --normalize path); findings: %+v", st, rep.Findings)
		}
	}
	// spot-check that the warning names the canonical form
	if got := findWarn(rep, `"done"`); !strings.Contains(got, `"complete"`) {
		t.Errorf(`alias warning for "done" does not name canonical "complete": %s`, got)
	}
	if got := findWarn(rep, `"todo"`); !strings.Contains(got, `"pending"`) {
		t.Errorf(`alias warning for "todo" does not name canonical "pending": %s`, got)
	}
}

// BT-025: unknown statuses keep ERRORING, and the error names the accepted
// read aliases.
func TestValidateUnknownStatusesStillError(t *testing.T) {
	// "\\\"pending" is the JSON-escaped on-disk spelling of the malformed
	// junk status '"pending' (a leading quote) from the BT-025 spec.
	for _, st := range []string{"retired", "closed", "wip", "\\\"pending"} {
		b := seedAliasBoard(t, st)
		rep, err := b.Validate()
		if err != nil {
			t.Fatal(err)
		}
		if !rep.HasErrors() {
			t.Errorf("unknown status %q must be an error; findings: %+v", st, rep.Findings)
		}
		found := false
		for _, f := range errorMsgs(rep) {
			if strings.Contains(f, `status "`+st+`"`) && strings.Contains(f, "read aliases") {
				found = true
			}
		}
		if !found {
			t.Errorf("error for unknown status %q does not name the accepted read aliases; findings: %+v", st, rep.Findings)
		}
	}
}

// BT-025: a mixed board (canonical + alias + unknown) fails overall (exit
// non-zero via HasErrors) but still warns on every alias row individually.
func TestValidateMixedAliasBoardErrorsAndWarns(t *testing.T) {
	b := seedAliasBoard(t, "pending", "todo", "retired")
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if !rep.HasErrors() {
		t.Fatal("board carrying retired must fail validation")
	}
	if got := findWarn(rep, `"todo"`); got == "" {
		t.Errorf("alias row not warned on a failing board: %+v", rep.Findings)
	}
}
