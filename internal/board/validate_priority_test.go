package board

import (
	"strings"
	"testing"
)

// seedPriorityBoard writes a topology-A board carrying one task row per
// priority spelling (each task id encodes its spelling), like seedAliasBoard.
func seedPriorityBoard(t *testing.T, priorities ...string) *Board {
	t.Helper()
	dir := t.TempDir()
	var tasks string
	for i, p := range priorities {
		tasks += `{"id":"PRIO-` + string(rune('A'+i)) + `","title":"t","status":"pending","priority":"` + p + `"}` + "\n"
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

// countPriorityWarns counts warn-level findings flagging a priority value.
func countPriorityWarns(rep *Report) int {
	n := 0
	for _, f := range rep.Findings {
		if f.Level == "warn" && strings.Contains(f.Msg, "priority ") {
			n++
		}
	}
	return n
}

// BT-048: priority values already on boards must be spelled in the canonical
// {P0,P1,P2,P3} vocabulary. The check is on the RAW stored value — bare
// digits ("3") and case variants ("p2") warn even though the write path
// normalizes them (BT-007) — because the flag exists to stop the
// off-vocabulary class regrowing on disk (28 live rows measured 2026-09-22).
func TestValidateWarnsOnOffVocabularyPriority(t *testing.T) {
	cases := []struct {
		raw       string
		canonical string // "" = no canonical form exists; warning names the vocabulary
	}{
		{"3", "P3"},
		{"P4", ""},
		{"p2", "P2"},
		{"urgent", ""},
	}
	for _, c := range cases {
		b := seedPriorityBoard(t, c.raw)
		rep, err := b.Validate()
		if err != nil {
			t.Fatal(err)
		}
		if rep.HasErrors() {
			t.Fatalf("off-vocabulary priority %q must warn, not error; findings: %+v", c.raw, rep.Findings)
		}
		if got := countPriorityWarns(rep); got != 1 {
			t.Fatalf("priority %q: want exactly 1 priority warning, got %d; findings: %+v", c.raw, got, rep.Findings)
		}
		msg := findWarn(rep, `priority "`+c.raw+`"`)
		if msg == "" {
			t.Fatalf("priority %q: warning does not quote the stored value; findings: %+v", c.raw, rep.Findings)
		}
		if !strings.Contains(msg, "PRIO-A") {
			t.Fatalf("priority %q: warning does not name the row id; msg: %s", c.raw, msg)
		}
		if !strings.Contains(msg, "{P0,P1,P2,P3}") {
			t.Fatalf("priority %q: warning does not name the canonical vocabulary; msg: %s", c.raw, msg)
		}
		if c.canonical != "" && !strings.Contains(msg, `"`+c.canonical+`"`) {
			t.Fatalf("priority %q: warning does not name the canonical form %q; msg: %s", c.raw, c.canonical, msg)
		}
	}
}

// BT-048: canonical priorities stay silent — the flag exists only for
// out-of-vocabulary values.
func TestValidateCanonicalPrioritiesSilent(t *testing.T) {
	b := seedPriorityBoard(t, "P0", "P1", "P2", "P3")
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() {
		t.Fatalf("canonical priorities must not error; findings: %+v", rep.Findings)
	}
	if got := countPriorityWarns(rep); got != 0 {
		t.Fatalf("canonical priorities flagged: %+v", rep.Findings)
	}
}

// BT-048: a board carrying ONLY priority warnings validates OK — warnings do
// not block (the CLI exits non-zero on HasErrors, which stays false here).
func TestValidatePriorityWarningsKeepBoardOK(t *testing.T) {
	b := seedPriorityBoard(t, "3", "P4", "p2", "urgent")
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() {
		t.Fatalf("priority warnings must not error; findings: %+v", rep.Findings)
	}
	if got := countPriorityWarns(rep); got != 4 {
		t.Fatalf("want 4 priority warnings (one per row), got %d; findings: %+v", got, rep.Findings)
	}
	if !strings.Contains(rep.RenderText(), "RESULT: OK") {
		t.Fatalf("board with only priority warnings must render RESULT: OK: %s", rep.RenderText())
	}
}

// BT-048: absent/missing priority is NOT a new error class — an empty
// priority passes validate untouched, exactly as before this change.
func TestValidateMissingPriorityUntouched(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"PRIO-EMPTY","title":"t","status":"pending"}` + "\n",
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
	if rep.HasErrors() {
		t.Fatalf("missing priority must not be a new error class; findings: %+v", rep.Findings)
	}
	if got := countPriorityWarns(rep); got != 0 {
		t.Fatalf("missing priority must not produce a priority warning: %+v", rep.Findings)
	}
}
