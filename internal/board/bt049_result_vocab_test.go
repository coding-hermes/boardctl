package board

import (
	"strconv"
	"strings"
	"testing"
)

// seedBT049Board writes a board carrying one row per BT-049 result-vocab
// case (each task id encodes its case). BT-049: the guard_result/ci_result
// warning previously printed the BT-007 UNION string
// "{PASS,FAIL,SKIP} / {GREEN,RED,SKIP}" for any out-of-vocab value, so a
// guard vocabulary value ("PASS") sitting in ci_result read as if "PASS"
// were rejected everywhere. The message must name the FIELD and that
// field's OWN vocabulary instead.
func seedBT049Board(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id":"BT049-CIPASS","title":"guard value in ci field","status":"pending","priority":"P2","ci_result":"PASS"}` + "\n" +
			`{"id":"BT049-GREEN","title":"ci value in guard field","status":"pending","priority":"P2","guard_result":"GREEN"}` + "\n" +
			`{"id":"BT049-PROSE","title":"free-form prose","status":"pending","priority":"P2","ci_result":"prose nonsense"}` + "\n" +
			`{"id":"BT049-LOWER","title":"case-tolerant guard","status":"pending","priority":"P2","guard_result":"pass"}` + "\n" +
			`{"id":"BT049-CIGREEN","title":"canonical ci","status":"pending","priority":"P2","ci_result":"GREEN"}` + "\n" +
			`{"id":"BT049-ABSENT","title":"results never run","status":"pending","priority":"P2"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

const (
	bt049Union       = "{PASS,FAIL,SKIP} / {GREEN,RED,SKIP}"
	bt049GuardVocab  = "{PASS,FAIL,SKIP}"
	bt049CIVocab     = "{GREEN,RED,SKIP}"
	bt049ProseSuffix = "hand-edit the row or rewrite via boardctl update"
)

// TestBT049ResultVocabWarningPerField: a warning for one of the two result
// columns must name that column and print ONLY that column's own
// vocabulary — never the union set.
func TestBT049ResultVocabWarningPerField(t *testing.T) {
	b := seedBT049Board(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	// warn severity — the class stays exit-0
	if rep.HasErrors() {
		t.Fatalf("result-vocab drift must warn, not error: %+v", rep.Findings)
	}

	cases := []struct {
		id       string // task id encoding the case
		field    string // the offending column
		value    string // the stored value
		wantVoc  string // that field's OWN vocabulary, as printed
		otherVoc string // the OTHER field's vocabulary — must not appear
	}{
		{ // the BT-049 bug: guard value in the ci field
			id: "BT049-CIPASS", field: "ci_result", value: "PASS",
			wantVoc: bt049CIVocab, otherVoc: bt049GuardVocab,
		},
		{ // mirror case: ci value in the guard field
			id: "BT049-GREEN", field: "guard_result", value: "GREEN",
			wantVoc: bt049GuardVocab, otherVoc: bt049CIVocab,
		},
		{ // genuinely free-form: still warns, per-field message
			id: "BT049-PROSE", field: "ci_result", value: "prose nonsense",
			wantVoc: bt049CIVocab, otherVoc: bt049GuardVocab,
		},
	}
	for _, c := range cases {
		got := findWarn(rep, c.id)
		if got == "" {
			t.Errorf("%s: validate did not warn; findings: %+v", c.id, rep.Findings)
			continue
		}
		if !strings.Contains(got, c.field+" "+strconv.Quote(c.value)) {
			t.Errorf("%s: warning must name the field and value, got: %s", c.id, got)
		}
		if !strings.Contains(got, c.wantVoc) {
			t.Errorf("%s: warning must print the field's own vocabulary %s, got: %s", c.id, c.wantVoc, got)
		}
		if strings.Contains(got, c.otherVoc) {
			t.Errorf("%s: warning must not print the other field's vocabulary %s, got: %s", c.id, c.otherVoc, got)
		}
		if strings.Contains(got, bt049Union) {
			t.Errorf("%s: warning must not use the union format, got: %s", c.id, got)
		}
		if !strings.Contains(got, bt049ProseSuffix) {
			t.Errorf("%s: warning lost the repair hint, got: %s", c.id, got)
		}
	}
}

// TestBT049ResultVocabSilentCases: case-tolerant in-vocab spellings and
// absent fields stay silent (no new warning classes).
func TestBT049ResultVocabSilentCases(t *testing.T) {
	b := seedBT049Board(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"BT049-LOWER", "BT049-CIGREEN", "BT049-ABSENT"} {
		if got := findWarn(rep, id); got != "" {
			t.Errorf("%s must not warn, got: %s", id, got)
		}
	}
}
