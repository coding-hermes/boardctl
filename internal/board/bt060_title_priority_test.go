package board

import (
	"strings"
	"testing"
)

// BT-060: a title carrying a Pn token that disagrees with the row's priority
// field is a drift tell — the fleet titles rows "[P1] real title" by hand and
// then re-priorities the row, leaving the token stale. The priority FIELD
// wins for what the row means (BT-048 made it the machine-read value); the
// warning only points at the token and names the `boardctl update <id>
// --title/--priority` fix. Rules pinned here, following the
// bt049_result_vocab_test.go shape:
//
//   - warn severity (exit 0 semantics — a cross-check, never a gate)
//   - the warning names the row id, the conflicting token, the field's
//     priority, and both repair paths
//   - NO Pn token in the title → silent (no new warning class)
//   - token AGREEING with the field → silent
//   - multiple tokens each warn independently
//   - out-of-vocabulary tokens (P4/P9) are NOT this class (the BT-048
//     priority check owns the row's own priority drift)
//
// seedBT060Board writes one row per case; each task id encodes its case so
// findWarn can address it.
func seedBT060Board(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id":"BT060-MISMATCH","title":"[P1] real title","status":"pending","priority":"P2"}` + "\n" +
			`{"id":"BT060-AGREE","title":"[P2] aligned","status":"pending","priority":"P2"}` + "\n" +
			`{"id":"BT060-NOTOKEN","title":"plain title, no token","status":"pending","priority":"P1"}` + "\n" +
			`{"id":"BT060-BARE","title":"P3 spike","status":"pending","priority":"P0"}` + "\n" +
			`{"id":"BT060-LOWER","title":"fix [p1] thing","status":"pending","priority":"P2"}` + "\n" +
			`{"id":"BT060-P4","title":"[P4] out of vocab","status":"pending","priority":"P2"}` + "\n" +
			`{"id":"BT060-EMBED","title":"unstick API2 and P10 backlog","status":"pending","priority":"P1"}` + "\n" +
			`{"id":"BT060-TWO","title":"[P0] a (P3) b","status":"pending","priority":"P2"}` + "\n" +
			`{"id":"BT060-NOPRIO","title":"[P1] orphan token","status":"pending"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-25 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestBT060TitlePriorityMismatchWarns: the core contract — a disagreeing
// token produces exactly one warning naming the row, both values, the
// "priority field wins" rule, and both repair paths.
func TestBT060TitlePriorityMismatchWarns(t *testing.T) {
	b := seedBT060Board(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	// warn severity — the class stays exit-0
	if rep.HasErrors() {
		t.Fatalf("title-priority drift must warn, not error: %+v", rep.Findings)
	}
	for _, id := range []string{"BT060-MISMATCH", "BT060-BARE", "BT060-LOWER", "BT060-TWO", "BT060-NOPRIO"} {
		got := findWarn(rep, id)
		if got == "" {
			t.Errorf("%s: validate did not warn; findings: %+v", id, rep.Findings)
			continue
		}
	}
	got := findWarn(rep, "BT060-MISMATCH")
	for _, want := range []string{
		`"P1"`, // the conflicting token, quoted
		`"P2"`, // the row's priority field, quoted
		"priority field wins",
		"boardctl update BT060-MISMATCH --title",
		"boardctl update BT060-MISMATCH --priority",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("warning missing %q, got: %s", want, got)
		}
	}
	// bracket-adjacent and bare-word forms both match; case is tolerated
	if !strings.Contains(findWarn(rep, "BT060-BARE"), `"P3"`) {
		t.Errorf("bare-word P3 token not detected: %s", findWarn(rep, "BT060-BARE"))
	}
	if !strings.Contains(findWarn(rep, "BT060-LOWER"), `"P1"`) {
		t.Errorf("lowercase [p1] token not detected: %s", findWarn(rep, "BT060-LOWER"))
	}
}

// TestBT060TitlePrioritySilentCases: agreeing tokens, token-free titles,
// embedded lookalikes (API2, P10), and out-of-vocabulary tokens (P4) stay
// silent — the class must not grow false positives that train operators to
// ignore it.
func TestBT060TitlePrioritySilentCases(t *testing.T) {
	b := seedBT060Board(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"BT060-AGREE", "BT060-NOTOKEN", "BT060-EMBED", "BT060-P4"} {
		if got := findWarn(rep, id); got != "" {
			t.Errorf("%s must not warn, got: %s", id, got)
		}
	}
}

// TestBT060MultipleTokensWarnIndependently: "[P0] a (P3) b" against priority
// P2 carries TWO tells — each token gets its own warning.
func TestBT060MultipleTokensWarnIndependently(t *testing.T) {
	b := seedBT060Board(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range rep.Findings {
		if f.Level == "warn" && strings.Contains(f.Msg, "BT060-TWO") {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("BT060-TWO carries two disagreeing tokens, got %d warnings (%+v)", n, rep.Findings)
	}
}

// TestBT060RowWithoutPriorityStillWarns: the priority FIELD wins, but an
// absent field does not mean "P2 by default" — a disagreeing token on a
// priority-less row is still reported (the token claims a priority the row
// does not carry; the fix is setting the field or rewriting the title).
func TestBT060RowWithoutPriorityStillWarns(t *testing.T) {
	b := seedBT060Board(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	got := findWarn(rep, "BT060-NOPRIO")
	if got == "" {
		t.Fatalf("priority-less row with a [P1] title must still warn; findings: %+v", rep.Findings)
	}
	if !strings.Contains(got, `"P1"`) {
		t.Errorf("warning must name the token: %s", got)
	}
}

// TestTitlePriorityTokensTable pins the tokenizer itself — the regex is the
// contract every future caller of TitlePriorityTokens inherits.
func TestTitlePriorityTokensTable(t *testing.T) {
	cases := []struct {
		title string
		want  []string
	}{
		{"[P1] real title", []string{"P1"}},
		{"p2 lowercase", []string{"P2"}},
		{"P0", []string{"P0"}},
		{"end token P3:", []string{"P3"}},
		{"[P1] a (P2) b", []string{"P1", "P2"}},
		{"P10 is bigger than P3", []string{"P3"}}, // P10 must not yield a bare P1
		{"API2 and P4", nil},                      // embedded letter, out-of-vocab digit
		{"no tokens here", nil},
		{"", nil},
		{"xP1y", nil}, // letters hugging the token make it part of a word
	}
	for _, c := range cases {
		got := TitlePriorityTokens(c.title)
		if len(got) != len(c.want) {
			t.Errorf("TitlePriorityTokens(%q) = %v, want %v", c.title, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("TitlePriorityTokens(%q) = %v, want %v", c.title, got, c.want)
				break
			}
		}
	}
}
