package board

import (
	"os"
	"strings"
	"testing"
)

// seedAliasRowBoard builds a topology-A board whose single task row carries
// the given raw status/guard/ci spellings, for NormalizeTask tests.
func seedAliasRowBoard(t *testing.T, status, guard, ci string) *Board {
	t.Helper()
	dir := t.TempDir()
	row := `{"id":"EXIST-1","title":"Existing","status":"` + status + `","priority":"P2","guard_result":` + guard + `,"ci_result":` + ci + `}` + "\n"
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  row,
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":"abc1234"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// BT-025: NormalizeTask rewrites exactly the dirty fields — status via the
// alias table, guard/ci via the upper-case/trim normalizer — leaving every
// other field byte-identical.
func TestNormalizeTaskRewritesAliasFields(t *testing.T) {
	for _, tc := range []struct {
		status, guard, ci string
		wantStatus        string
		wantGuard         string
		wantCI            string
	}{
		{"done", `"pass"`, `"red"`, "complete", "PASS", "RED"},
		{"todo", `null`, `null`, "pending", "", ""},
		{"in-progress", `"skip"`, `null`, "in_progress", "SKIP", ""},
	} {
		b := seedAliasRowBoard(t, tc.status, tc.guard, tc.ci)
		before, err := os.ReadFile(b.tasksPath)
		if err != nil {
			t.Fatal(err)
		}
		changes, err := b.NormalizeTask("EXIST-1", false)
		if err != nil {
			t.Fatalf("NormalizeTask(%s/%s/%s): %v", tc.status, tc.guard, tc.ci, err)
		}
		after, err := os.ReadFile(b.tasksPath)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := b.TaskRows()
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("row count = %d, want 1", len(rows))
		}
		row := rows[0]
		if got := row.String("status"); got != tc.wantStatus {
			t.Errorf("status = %q, want %q", got, tc.wantStatus)
		}
		if tc.wantGuard != "" {
			if got := row.String("guard_result"); got != tc.wantGuard {
				t.Errorf("guard_result = %q, want %q", got, tc.wantGuard)
			}
		}
		if tc.wantCI != "" {
			if got := row.String("ci_result"); got != tc.wantCI {
				t.Errorf("ci_result = %q, want %q", got, tc.wantCI)
			}
		}
		// id/title/priority bytes untouched
		if !strings.Contains(string(after), `"priority":"P2"`) || !strings.Contains(string(after), `"id":"EXIST-1"`) {
			t.Errorf("untouched fields disturbed:\nbefore %s\nafter  %s", before, after)
		}
		// every reported change must be a real old->new spelling pair
		if len(changes) == 0 {
			t.Error("NormalizeTask reported no changes for a dirty row")
		}
		for _, ch := range changes {
			if ch.Old == ch.New {
				t.Errorf("reported no-op change %+v", ch)
			}
			if !strings.Contains(string(after), `"`+ch.New+`"`) {
				t.Errorf("change %+v not visible on disk: %s", ch, after)
			}
		}
	}
}

// BT-025: a row whose normalize targets are already canonical (or absent) is
// a no-op — nothing is written, so the file is byte-identical and a second
// run is idempotent.
func TestNormalizeTaskNoopAndIdempotent(t *testing.T) {
	// canonical status, dirty guard: first run rewrites guard only
	b := seedAliasRowBoard(t, "complete", `"pass"`, "null")
	changes, err := b.NormalizeTask("EXIST-1", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Field != "guard_result" || changes[0].Old != "pass" || changes[0].New != "PASS" {
		t.Fatalf("changes = %+v, want exactly guard_result pass->PASS", changes)
	}
	mid, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	// second run: fully canonical -> no-op, byte-identical
	changes2, err := b.NormalizeTask("EXIST-1", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes2) != 0 {
		t.Fatalf("second normalize reported changes %+v, want none", changes2)
	}
	after, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(mid) != string(after) {
		t.Fatalf("no-op normalize rewrote the file:\nbefore %s\nafter  %s", mid, after)
	}
	// a canonical row with no guard/ci at all is also a no-op
	b2 := seedAliasRowBoard(t, "pending", "null", "null")
	if changes, err := b2.NormalizeTask("EXIST-1", false); err != nil || len(changes) != 0 {
		t.Fatalf("canonical row normalize = (%+v, %v), want no-op", changes, err)
	}
	raw, _ := os.ReadFile(b2.tasksPath)
	if !strings.Contains(string(raw), `"status":"pending"`) {
		t.Fatalf("canonical row disturbed: %s", raw)
	}
}

// BT-025: an unknown status refuses to normalize — the error names the field
// and value, and the file is byte-identical (nothing written).
func TestNormalizeTaskRefusesUnknownStatus(t *testing.T) {
	// "\\\"pending" is the JSON-escaped on-disk spelling of the malformed
	// junk status '"pending' (a leading quote) from the BT-025 spec.
	for _, st := range []string{"retired", "closed", "wip", "\\\"pending"} {
		b := seedAliasRowBoard(t, st, "null", "null")
		before, err := os.ReadFile(b.tasksPath)
		if err != nil {
			t.Fatal(err)
		}
		changes, err := b.NormalizeTask("EXIST-1", false)
		if err == nil {
			t.Fatalf("status %q normalized (%+v), want refusal", st, changes)
		}
		if !strings.Contains(err.Error(), "status") || !strings.Contains(err.Error(), `"`+st+`"`) {
			t.Errorf("refusal should name field+value, got: %v", err)
		}
		after, err := os.ReadFile(b.tasksPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Errorf("refused normalize rewrote the file:\nbefore %s\nafter  %s", before, after)
		}
	}
}

// BT-025: a guard/ci value outside its vocabulary refuses to normalize too.
func TestNormalizeTaskRefusesUnknownResults(t *testing.T) {
	b := seedAliasRowBoard(t, "done", `"MAYBE"`, `"BANANA"`)
	before, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.NormalizeTask("EXIST-1", false); err == nil {
		t.Fatal("guard_result MAYBE normalized, want refusal")
	} else if !strings.Contains(err.Error(), "guard_result") || !strings.Contains(err.Error(), "MAYBE") {
		t.Errorf("refusal should name guard_result MAYBE, got: %v", err)
	}
	after, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("refused normalize rewrote the file:\nbefore %s\nafter  %s", before, after)
	}
}

// BT-025: normalize works on topology B — the line-1 header is never a
// target and round-trips byte-identical while the alias row below is fixed.
func TestNormalizeTaskTopologyBHeaderUntouched(t *testing.T) {
	dir := t.TempDir()
	header := `{"project":"legacy","namespace":"legacy","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":"abc1234"}`
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": header + "\n" +
			`{"id":"EXIST-1","title":"Existing","status":"done","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Topology != "B" {
		t.Fatalf("topology = %q, want B", b.Topology)
	}
	changes, err := b.NormalizeTask("EXIST-1", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Field != "status" || changes[0].Old != "done" || changes[0].New != "complete" {
		t.Fatalf("changes = %+v, want exactly status done->complete", changes)
	}
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if lines[0] != header {
		t.Fatalf("header line disturbed:\nwant %s\ngot  %s", header, lines[0])
	}
	if !strings.Contains(lines[1], `"status":"complete"`) {
		t.Fatalf("alias status not normalized: %s", lines[1])
	}
}

// BT-025: normalize is a pure spelling repair — no audit event is appended
// and updated_at is not refreshed.
func TestNormalizeTaskWritesNoEvent(t *testing.T) {
	b := seedAliasRowBoard(t, "todo", "null", "null")
	eventsBefore, err := os.ReadFile(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.NormalizeTask("EXIST-1", false); err != nil {
		t.Fatal(err)
	}
	eventsAfter, err := os.ReadFile(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(eventsBefore) != string(eventsAfter) {
		t.Fatalf("normalize appended an event:\nbefore %s\nafter  %s", eventsBefore, eventsAfter)
	}
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Has("updated_at") {
		t.Fatalf("normalize refreshed updated_at: %s", RowJSONCompact(rows[0]))
	}
}
