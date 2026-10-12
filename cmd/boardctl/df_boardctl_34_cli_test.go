package main

// DF-BOARDCTL-34 CLI surface: `umbrella --stats` must never print a
// non-vocabulary priority bucket key (the real dogfood failure printed
// `P1\",\"source\":\"qa-dagger` under "by priority"), must report the invalid
// values as stderr warnings with repo provenance, and must fail under
// --strict. Valid P0..P5 buckets keep counting.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// df34AppendRawPriority writes one task row with a verbatim priority VALUE
// into a repo's tasks.jsonl (the create gate rightly refuses the value).
func df34AppendRawPriority(t *testing.T, repo, id, title, rawPriority string) {
	t.Helper()
	bd := filepath.Join(repo, ".coding-hermes", "board")
	f, err := os.OpenFile(filepath.Join(bd, "tasks.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck
	if _, err := f.WriteString(`{"id":"` + id + `","title":"` + title + `","status":"pending","priority":"` + rawPriority + `"}` + "\n"); err != nil {
		t.Fatal(err)
	}
}

func TestDFBoardCTL34UmbrellaStatsCLIGatesPriorityBuckets(t *testing.T) {
	umbrella, childA, childB := bt078Fixture(t)

	// Valid vocabulary across both children (create defaults to P2; the rest
	// raw-written so every canonical bucket is exercised).
	bt078Create(t, childA, "DF34-A1", "childA p2")
	bt078Create(t, childB, "DF34-B1", "childB p2")
	df34AppendRawPriority(t, childA, "DF34-A2", "childA p3", `P3`)
	// The malformed value from the real dogfood run (embedded JSON).
	df34AppendRawPriority(t, childA, "DF34-A3", "childA corrupt", `P1\",\"source\":\"qa-dagger`)

	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--stats"})
	if code != 0 {
		t.Fatalf("umbrella --stats exit = %d, want 0 (out: %s)", code, out)
	}
	if got := bt078LastStderr; !containsAll(got, "[warn]", "childA", "not in vocabulary", "qa-dagger") {
		t.Fatalf("stderr lacks the priority-value warnings (want repo provenance + vocabulary naming):\n%s", got)
	}
	// bt078Run returns stdout+stderr; the leak check applies to stdout only
	// (the stderr warning legitimately quotes the invalid value).
	stdoutOnly := strings.TrimSuffix(out, bt078LastStderr)
	if containsAny(stdoutOnly, "qa-dagger", "\\\"") {
		t.Fatalf("corrupt priority bucket leaked into the stats output:\n%s", stdoutOnly)
	}
	// Grand total renders only canonical buckets; valid counts intact.
	if !containsAll(out, "by priority:", "P2", "P3") {
		t.Fatalf("valid priority buckets missing from the render:\n%s", out)
	}
	if !containsAll(out, "total tasks: 4") {
		t.Fatalf("task total must still count the invalid-priority rows:\n%s", out)
	}
}

func TestDFBoardCTL34UmbrellaStatsStrictFailsOnPriorityWarnings(t *testing.T) {
	umbrella, childA, _ := bt078Fixture(t)
	bt078Create(t, childA, "DF34-A1", "childA p2")
	df34AppendRawPriority(t, childA, "DF34-A2", "childA corrupt", `P1\",\"source\":\"qa-dagger`)

	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--stats", "--strict"})
	if code != 1 {
		t.Fatalf("umbrella --stats --strict exit = %d, want 1 on a priority-value warning (out: %s)", code, out)
	}
	if got := bt078LastStderr; !containsAll(got, "qa-dagger", "childA") {
		t.Fatalf("strict stderr lacks the priority-value warning:\n%s", got)
	}
	// The strict failure summary names the finding count (umbrellaExit's
	// counting path): 1 priority-value warning + 1 strict escalation.
	if !containsAny(out, "finding(s)") {
		t.Fatalf("strict failure summary must name the finding count:\n%s", out)
	}
}

// containsAll / containsAny are small readable guards over strings.Contains.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
