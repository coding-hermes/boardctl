package board

// DF-BOARDCTL-50: acceptance_criteria had no sanctioned write path.
//
// The fleet's foremen refuse to dispatch a row without acceptance_criteria,
// yet neither `create` nor `update` could author it: the only way to write
// the field was a line-wise JSON edit of tasks.jsonl — exactly the bypass
// README:1262-1264 warns about, because it skips the id-refusal guard, the
// row-shape checks AND the BT-050 write lock, and is the known source of
// duplicate-id corruption on fleet boards. A required field with no
// sanctioned door guarantees every lane takes the unsanctioned one.
//
// The contract these tests pin:
//
//  1. create with criteria persists the value; update persists/replaces it
//     on an existing row (both through the locked byte-preserving machinery,
//     so every OTHER line of the file round-trips untouched);
//  2. multi-line criteria are ONE JSON string: the physical JSONL row stays
//     a single line (newlines JSON-escaped) and the value reads back
//     byte-exact, newlines included;
//  3. omission writes NO acceptance_criteria key at all — not an empty
//     string, not a null placeholder. This is the BT-076 `deferred`
//     discipline: the key is never neutral-mirrored from the preceding row,
//     so a board whose rows all carry criteria cannot stamp a null key onto
//     a row created without them.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// df50Criteria is deliberately multi-line AND non-ASCII: the value has to
// survive as ONE JSON string on ONE physical line.
const df50Criteria = "1) create a row through the locked write path\n2) update it and read the criteria back byte-exact\n3) — unicode, \"quotes\" and\ttabs survive too"

// df50RowLine returns the raw tasks.jsonl line carrying the given task id
// (fails when absent). The returned bytes are one physical line.
func df50RowLine(t *testing.T, b *Board, id string) []byte {
	t.Helper()
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		row, perr := ParseRow([]byte(line))
		if perr != nil {
			continue // topology-B header line: not a task row
		}
		if row.String("id") == id {
			return []byte(line)
		}
	}
	t.Fatalf("task %q not found in %s", id, b.tasksPath)
	return nil
}

// df50Row decodes the raw line of a task id back into a Row.
func df50Row(t *testing.T, b *Board, id string) *Row {
	t.Helper()
	row, err := ParseRow(df50RowLine(t, b, id))
	if err != nil {
		t.Fatalf("row %q does not parse: %v", id, err)
	}
	return row
}

// df50SeedBoardWithCriteria seeds a topology-A board whose ONLY row already
// carries acceptance_criteria — the mirror trap for criterion 3: create
// mirrors the last row's sanctioned key schema, so without the explicit
// never-mirror rule a plain `create` would inherit a null placeholder.
func df50SeedBoardWithCriteria(t *testing.T, criteria string) *Board {
	t.Helper()
	dir := t.TempDir()
	seed := `{"id":"SEED-1","title":"row that already has criteria","status":"pending","priority":"P2","acceptance_criteria":` +
		df50JSONString(t, criteria) + `}` + "\n"
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  seed,
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"df50","namespace":"df50","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":"abc1234"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// df50JSONString renders s as a JSON string literal (test-side, for fixtures).
func df50JSONString(t *testing.T, s string) string {
	t.Helper()
	enc, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(enc)
}

// TestWriteAcceptanceCriteriaOnCreateByteExact is criterion 1's create half:
// the value lands under acceptance_criteria and reads back byte-exact, and
// criterion 2: a multi-line value occupies exactly one physical JSONL line
// (the newlines are JSON escapes inside the string, not row separators).
func TestWriteAcceptanceCriteriaOnCreateByteExact(t *testing.T) {
	b := newTestBoard(t)
	// newTestBoard's only row carries no acceptance_criteria, so this also
	// exercises the APPEND path (key absent from the mirrored schema).
	if df50Row(t, b, "EXIST-1").Has("acceptance_criteria") {
		t.Fatal("premise: seed row already carries acceptance_criteria")
	}

	crit := df50Criteria
	if _, err := b.Create(TaskRowSpec{
		ID:                 "DF50-1",
		Title:              "criteria-bearing row",
		Status:             "pending",
		Priority:           "P2",
		AcceptanceCriteria: &crit,
	}); err != nil {
		t.Fatalf("create with acceptance criteria: %v", err)
	}

	line := df50RowLine(t, b, "DF50-1")
	if strings.ContainsAny(string(line), "\n\r") {
		t.Fatalf("row spans more than one physical line: %q", line)
	}
	// One object, one line: the multi-line value must be escaped, not raw.
	if !strings.Contains(string(line), `\n`) {
		t.Fatalf("multi-line criteria were not JSON-escaped onto the row line: %s", line)
	}

	row := df50Row(t, b, "DF50-1")
	if !row.Has("acceptance_criteria") {
		t.Fatalf("acceptance_criteria key missing from created row: %s", line)
	}
	if got := row.String("acceptance_criteria"); got != crit {
		t.Fatalf("criteria round-trip mismatch:\n got %q\nwant %q", got, crit)
	}
	// The key is appended at the END of the row's own key order (the mirrored
	// schema lacks it) — and never before the mirror's own keys change.
	idxOf := func(key string) int {
		for i, k := range row.Keys {
			if k == key {
				return i
			}
		}
		return -1
	}
	if ac, pri := idxOf("acceptance_criteria"), idxOf("priority"); ac < 0 || ac < pri {
		t.Fatalf("acceptance_criteria not appended after the mirrored keys (order %v)", row.Keys)
	}

	// The board is still line-per-row: one seed row + the new row.
	if n := boardLineCount(t, b.tasksPath); n != 2 {
		t.Fatalf("tasks.jsonl has %d lines, want 2 (the multi-line value must not add lines)", n)
	}
}

// TestWriteAcceptanceCriteriaOnCreateOmittedNoKey is criterion 3 (and the
// mirror trap in particular): with the flag absent NO acceptance_criteria key
// is written, even on a board whose rows already carry criteria.
func TestWriteAcceptanceCriteriaOnCreateOmittedNoKey(t *testing.T) {
	b := df50SeedBoardWithCriteria(t, "seed criteria that must not be inherited")
	// Premise: the mirror row DOES carry the key, so the skip is load-bearing.
	if !df50Row(t, b, "SEED-1").Has("acceptance_criteria") {
		t.Fatal("premise: seed row was supposed to carry acceptance_criteria")
	}

	if _, err := b.Create(TaskRowSpec{
		ID:       "DF50-OMIT",
		Title:    "no criteria named",
		Status:   "pending",
		Priority: "P2",
	}); err != nil {
		t.Fatalf("create without acceptance criteria: %v", err)
	}

	line := string(df50RowLine(t, b, "DF50-OMIT"))
	if strings.Contains(line, "acceptance_criteria") {
		t.Fatalf("omitted flag still wrote an acceptance_criteria key: %s", line)
	}
	if df50Row(t, b, "DF50-OMIT").Has("acceptance_criteria") {
		t.Fatalf("omitted flag still produced the key: %s", line)
	}
}

// TestWriteAcceptanceCriteriaOnUpdateByteExact is criterion 1's update half: an
// existing row that never had the key gains it, and every other line of the
// file round-trips byte-identical (the locked byte-preserving rewrite).
func TestWriteAcceptanceCriteriaOnUpdateByteExact(t *testing.T) {
	dir := t.TempDir()
	first := `{"id":"DF50-A","title":"first row, untouched","status":"pending","priority":"P2"}` + "\n"
	second := `{"id":"DF50-B","title":"target row","status":"pending","priority":"P2"}` + "\n"
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  first + second,
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"df50","namespace":"df50","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":"abc1234"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}

	crit := df50Criteria
	changed, err := b.UpdateTask("DF50-B", UpdateSpec{AcceptanceCriteria: &crit})
	if err != nil {
		t.Fatalf("update with acceptance criteria: %v", err)
	}
	if len(changed) == 0 || changed[0] != "acceptance_criteria" {
		t.Fatalf("changed = %v, want acceptance_criteria reported first", changed)
	}

	row := df50Row(t, b, "DF50-B")
	if got := row.String("acceptance_criteria"); got != crit {
		t.Fatalf("criteria round-trip mismatch:\n got %q\nwant %q", got, crit)
	}
	// The pre-existing fields survive untouched.
	if row.String("title") != "target row" || row.String("status") != "pending" {
		t.Fatalf("update clobbered existing fields: %s", df50RowLine(t, b, "DF50-B"))
	}
	// Untouched line is byte-identical; the rewritten line is still ONE line.
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("tasks.jsonl has %d lines after update, want 2 (multi-line value must not add lines)", len(lines))
	}
	if lines[0] != strings.TrimRight(first, "\n") {
		t.Fatalf("untouched line mutated:\n got %q\nwant %q", lines[0], strings.TrimRight(first, "\n"))
	}

	// A second update REPLACES the value (the field is one prose blob).
	next := "rewritten criteria"
	if _, err := b.UpdateTask("DF50-B", UpdateSpec{AcceptanceCriteria: &next}); err != nil {
		t.Fatalf("second update: %v", err)
	}
	if got := df50Row(t, b, "DF50-B").String("acceptance_criteria"); got != next {
		t.Fatalf("criteria not replaced: got %q want %q", got, next)
	}
}

// TestWriteAcceptanceCriteriaOnUpdateOmittedNoKey: an update that does not
// name the flag leaves a row without the key exactly as it found it.
func TestWriteAcceptanceCriteriaOnUpdateOmittedNoKey(t *testing.T) {
	b := newTestBoard(t) // EXIST-1 carries no acceptance_criteria
	title := "renamed only"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Title: &title}); err != nil {
		t.Fatalf("title-only update: %v", err)
	}
	line := string(df50RowLine(t, b, "EXIST-1"))
	if strings.Contains(line, "acceptance_criteria") {
		t.Fatalf("title-only update wrote an acceptance_criteria key: %s", line)
	}
}
