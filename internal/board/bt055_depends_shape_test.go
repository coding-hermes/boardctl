package board

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

// seedBT055Board writes a board carrying one row per depends_on SHAPE case
// (each task id encodes its case). BT-055: rowStringSlice (validate.go)
// returns nil for any non-array depends_on, so a malformed shape silently
// read as "no dependencies" and the dangling-ref cross-check saw nothing —
// validate stayed silent on the exact population BT-050 describes
// (hand-written / legacy rows; the write path types DependsOn as []string
// and can never emit one). The fix adds a WARN naming file, line, task id
// and the stored shape, plus a second warning when a non-empty string value
// carries a reference that was therefore never cross-checked.
func seedBT055Board(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id":"BT055-ARRAY","title":"canonical empty array","status":"pending","priority":"P2","depends_on":[]}` + "\n" +
			`{"id":"BT055-ABSENT","title":"key never written","status":"pending","priority":"P2"}` + "\n" +
			`{"id":"BT055-EMPTYSTR","title":"stringified empty array (the BT-035 shape)","status":"pending","priority":"P2","depends_on":"[]"}` + "\n" +
			`{"id":"BT055-REF","title":"bare id in a string (BT-019 does not exist on this board)","status":"pending","priority":"P2","depends_on":"BT-019"}` + "\n" +
			`{"id":"BT055-STRARR","title":"stringified real array","status":"pending","priority":"P2","depends_on":"[\"BT-019\"]"}` + "\n" +
			`{"id":"BT055-NUM","title":"number where an array belongs","status":"pending","priority":"P2","depends_on":7}` + "\n" +
			`{"id":"BT055-NULL","title":"explicit null","status":"pending","priority":"P2","depends_on":null}` + "\n" +
			`{"id":"BT055-OBJ","title":"object where an array belongs","status":"pending","priority":"P2","depends_on":{"0":"BT-019"}}}` + "\n" +
			`{"id":"BT055-BOOL","title":"boolean where an array belongs","status":"pending","priority":"P2","depends_on":true}` + "\n",
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
	bt055ShapeMsg = "depends_on is not an array"
	bt055NoXCheck = "was not cross-checked"
)

// bt055WarnsFor returns the BT-055-class warnings addressed to one task id
// (the messages embed "(task <id>)"), joined for substring checks.
func bt055WarnsFor(rep *Report, id string) string {
	var got []string
	for _, f := range rep.Findings {
		if f.Level == "warn" && strings.Contains(f.Msg, "(task "+id+")") {
			got = append(got, f.Msg)
		}
	}
	return strings.Join(got, "\n")
}

func bt055CountWarns(rep *Report, id string) int {
	n := 0
	for _, f := range rep.Findings {
		if f.Level == "warn" && strings.Contains(f.Msg, "(task "+id+")") {
			n++
		}
	}
	return n
}

// TestBT055DependsOnShapeWarnings: each malformed shape warns (never errors),
// the warning names file + 1-based line + task id and the JSON kind, a
// stringified empty array gets exactly one warning (nothing to cross-check),
// and a string carrying a real reference additionally warns it was not
// cross-checked. Canonical array and absent key stay silent.
func TestBT055DependsOnShapeWarnings(t *testing.T) {
	b := seedBT055Board(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	// warn severity — the class stays exit-0
	if rep.HasErrors() {
		t.Fatalf("depends_on shape drift must warn, not error: %+v", rep.Findings)
	}
	cases := []struct {
		id      string
		line    int // 1-based tasks.jsonl line the row sits on
		wantN   int // number of BT-055 warnings for this row
		wantSub []string
	}{
		{id: "BT055-ARRAY", line: 1, wantN: 0},
		{id: "BT055-ABSENT", line: 2, wantN: 0},
		{
			// "[]" as a STRING is non-empty string CONTENT — the shape
			// warning plus the not-cross-checked note both apply.
			id: "BT055-EMPTYSTR", line: 3, wantN: 2,
			wantSub: []string{"tasks.jsonl line", bt055ShapeMsg, "(string)", "treated as no dependencies", `"[]"`, bt055NoXCheck},
		},
		{
			id: "BT055-REF", line: 4, wantN: 2,
			wantSub: []string{"tasks.jsonl line", bt055ShapeMsg, "(string)", "treated as no dependencies", `"BT-019"`, bt055NoXCheck},
		},
		{
			id: "BT055-STRARR", line: 5, wantN: 2,
			wantSub: []string{"tasks.jsonl line", bt055ShapeMsg, "(string)", bt055NoXCheck},
		},
		{id: "BT055-NUM", line: 6, wantN: 1, wantSub: []string{"tasks.jsonl line", bt055ShapeMsg, "(number)"}},
		{
			// null: the write path's neutral spellings for "no deps" are an
			// ABSENT key or a real [] array — it never stores null, so an
			// explicit null is hand-written data too and warns (brief's
			// shape table lists null as a warn case).
			id: "BT055-NULL", line: 7, wantN: 1,
			wantSub: []string{"tasks.jsonl line", bt055ShapeMsg, "(null)"},
		},
		{id: "BT055-OBJ", line: 8, wantN: 1, wantSub: []string{"tasks.jsonl line", bt055ShapeMsg, "(object)"}},
		{id: "BT055-BOOL", line: 9, wantN: 1, wantSub: []string{"tasks.jsonl line", bt055ShapeMsg, "(boolean)"}},
	}
	for _, c := range cases {
		if got := bt055CountWarns(rep, c.id); got != c.wantN {
			t.Errorf("%s: want %d depends_on-shape warning(s), got %d; findings: %+v",
				c.id, c.wantN, got, rep.Findings)
			continue
		}
		got := bt055WarnsFor(rep, c.id)
		for _, sub := range c.wantSub {
			if !strings.Contains(got, sub) {
				t.Errorf("%s: warning(s) must mention %q, got: %q", c.id, sub, got)
			}
		}
		if c.wantN > 0 {
			if want := fmt.Sprintf("line %d (task %s)", c.line, c.id); !strings.Contains(got, want) {
				t.Errorf("%s: warning must name line %d and the task id, got: %q", c.id, c.line, got)
			}
		}
	}
}

// TestBT055MalformedRefSkipsCrossCheck: BT-019 does not exist on this board.
// A reference stored in the wrong shape must NOT feed the dangling-ref
// cross-check (rowStringSlice reads it as no dependencies — that silence is
// the bug), and validate must say so instead of leaving the row clean.
func TestBT055MalformedRefSkipsCrossCheck(t *testing.T) {
	b := seedBT055Board(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, DanglingDepMsg) {
			t.Errorf("malformed-shape reference must not enter the dangling cross-check, got: %s", f.Msg)
		}
	}
	got := bt055WarnsFor(rep, "BT055-REF")
	if !strings.Contains(got, bt055NoXCheck) || !strings.Contains(got, `"BT-019"`) {
		t.Errorf("reference-in-string row must warn the reference was not cross-checked, got: %q", got)
	}
}

// TestBT055ValidateWritesNothing: validate is a read-only check — the board
// bytes on disk must be byte-identical after a Validate run, whatever it
// warned about (untouched rows are never rewritten by the warning path).
func TestBT055ValidateWritesNothing(t *testing.T) {
	b := seedBT055Board(t)
	before, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Validate(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("validate rewrote tasks.jsonl (%d -> %d bytes)", len(before), len(after))
	}
}
