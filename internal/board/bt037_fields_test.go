package board

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// BT-037: worktree / branch / sessions are first-class row fields. These tests
// pin the whole write contract for them:
//
//   - create and update place the values on the row verbatim
//   - --session collects an ordered ARRAY (one element per occurrence)
//   - every untouched byte of the file survives (other rows byte-identical)
//   - a row with no worktree/branch/sessions never grows an empty key
//   - the new keys are APPENDED at the end of the row's key order
//   - validate/doctor tolerate the fields (no allowlist, no new findings)
//
// The seed rows use python SPACED separators on purpose: the array and string
// values must be encoded in the file's own style, not a hand-rolled compact
// json.Marshal.

// seedBT037Board writes a topology-A board whose task rows are spaced-style.
func seedBT037Board(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id": "BT-1", "title": "first", "status": "pending", "priority": "P2"}` + "\n" +
			`{"id": "BT-2", "title": "second", "status": "pending", "priority": "P2"}` + "\n",
		"events.jsonl": `{"id": 1, "timestamp": "2026-09-19 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		"board.jsonl":  `{"project": "bt037", "namespace": "bt037", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": "abc1234"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Topology != "A" {
		t.Fatalf("topology = %q, want A", b.Topology)
	}
	return b
}

// taskLines returns the non-empty tasks.jsonl lines of a board.
func taskLines(t *testing.T, b *Board) []string {
	t.Helper()
	raw, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// sessionsRaw extracts the raw bytes of a row's "sessions" value (nil when the
// key is absent), so a test can assert the EXACT serialization — array shape,
// element order, and the file's separator style.
func sessionsRaw(t *testing.T, line string) []byte {
	t.Helper()
	row, err := ParseRow([]byte(line))
	if err != nil {
		t.Fatalf("row does not parse: %v (%s)", err, line)
	}
	return row.Get("sessions")
}

// TestCreateWritesWorktreeBranchSessions: create with the three flags lands
// worktree/branch/sessions on the new row as APPENDED keys, in the board's own
// (spaced) style, and leaves every pre-existing byte alone.
func TestCreateWritesWorktreeBranchSessions(t *testing.T) {
	b := seedBT037Board(t)
	before, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}

	wt := "/home/kara/wt-bt-schema"
	br := "bt/session-worktree-fields"
	list := []string{"sess-a", "sess-b"}
	if _, err := b.Create(TaskRowSpec{
		ID:       "BT-3",
		Title:    "third",
		Worktree: &wt,
		Branch:   &br,
		Sessions: &list,
	}); err != nil {
		t.Fatalf("create with worktree/branch/session rejected: %v", err)
	}

	raw, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	// append-only: every byte that was there before is still there, unchanged.
	if !bytes.HasPrefix(raw, before) {
		t.Fatalf("create rewrote existing bytes:\nbefore %q\nafter  %q", before, raw)
	}
	lines := taskLines(t, b)
	if len(lines) != 3 {
		t.Fatalf("tasks.jsonl has %d rows, want 3", len(lines))
	}
	if lines[0] != `{"id": "BT-1", "title": "first", "status": "pending", "priority": "P2"}` ||
		lines[1] != `{"id": "BT-2", "title": "second", "status": "pending", "priority": "P2"}` {
		t.Fatalf("pre-existing rows changed:\n%s\n%s", lines[0], lines[1])
	}
	last := lines[2]
	// Exact serialization: the three keys are appended AFTER the mirrored
	// schema's keys and BEFORE the guarantee-core keys create adds to any
	// mirrored row, contiguous, spaced like the rest of the file.
	wantFragment := `"priority": "P2", "worktree": "/home/kara/wt-bt-schema", "branch": "bt/session-worktree-fields", "sessions": ["sess-a", "sess-b"], "complexity": 3`
	if !strings.Contains(last, wantFragment) {
		t.Fatalf("new row must carry the appended fields in order, in the board's style\n got: %s\nwant fragment: %s", last, wantFragment)
	}
	row, err := ParseRow([]byte(last))
	if err != nil {
		t.Fatal(err)
	}
	if got := row.String("worktree"); got != wt {
		t.Fatalf("worktree = %q, want %q", got, wt)
	}
	if got := row.String("branch"); got != br {
		t.Fatalf("branch = %q, want %q", got, br)
	}
	var gotSessions []string
	if err := json.Unmarshal(row.Get("sessions"), &gotSessions); err != nil {
		t.Fatalf("sessions is not a JSON array: %v (%s)", err, row.Get("sessions"))
	}
	if len(gotSessions) != 2 || gotSessions[0] != "sess-a" || gotSessions[1] != "sess-b" {
		t.Fatalf("sessions = %v, want [sess-a sess-b] in order", gotSessions)
	}
	keys := row.Keys
	if i := indexOfKey(keys, "worktree"); i < 0 || i+2 >= len(keys) || keys[i+1] != "branch" || keys[i+2] != "sessions" {
		t.Fatalf("new keys must be appended (contiguous, in flag order) after the mirrored schema, got key order %v", keys)
	}
	// The mirrored row's own values are still untouched by the new fields.
	if row.String("id") != "BT-3" || row.String("title") != "third" {
		t.Fatalf("core fields wrong: %s", last)
	}
}

// TestCreateWithoutWorktreeFieldsWritesNoKeys: omitting the flags must not
// create the keys at all (no empty worktree string, no empty sessions array) —
// "built in the main checkout" is expressed by ABSENCE.
func TestCreateWithoutWorktreeFieldsWritesNoKeys(t *testing.T) {
	b := seedBT037Board(t)
	if _, err := b.Create(TaskRowSpec{ID: "BT-3", Title: "third"}); err != nil {
		t.Fatal(err)
	}
	last := taskLines(t, b)[2]
	for _, k := range []string{"worktree", "branch", "sessions"} {
		if strings.Contains(last, `"`+k+`"`) {
			t.Fatalf("create without flags wrote %q: %s", k, last)
		}
	}
}

// TestCreateSingleSessionIsOneElementArray: one --session is a one-element
// ARRAY, never a bare string.
func TestCreateSingleSessionIsOneElementArray(t *testing.T) {
	b := seedBT037Board(t)
	list := []string{"only-session"}
	if _, err := b.Create(TaskRowSpec{ID: "BT-3", Title: "third", Sessions: &list}); err != nil {
		t.Fatal(err)
	}
	raw := sessionsRaw(t, taskLines(t, b)[2])
	if string(raw) != `["only-session"]` {
		t.Fatalf("sessions raw = %s, want [\"only-session\"]", raw)
	}
}

// TestUpdateWritesWorktreeBranchSessionsBytePreserving: update lands the three
// fields on the target row (appended, spaced, array in order) while the other
// row of the file stays byte-identical.
func TestUpdateWritesWorktreeBranchSessionsBytePreserving(t *testing.T) {
	b := seedBT037Board(t)
	otherBefore := taskLines(t, b)[1]

	complete := "complete"
	wt := "/home/kara/wt-cht-031"
	br := "wt/cht-031"
	list := []string{"A", "B", "C"}
	changed, err := b.UpdateTask("BT-1", UpdateSpec{
		Status:   &complete,
		Worktree: &wt,
		Branch:   &br,
		Sessions: &list,
	})
	if err != nil {
		t.Fatalf("update with worktree/branch/session rejected: %v", err)
	}
	for _, k := range []string{"status", "worktree", "branch", "sessions"} {
		if !containsStr(changed, k) {
			t.Fatalf("changed = %v, missing %q", changed, k)
		}
	}

	lines := taskLines(t, b)
	if lines[1] != otherBefore {
		t.Fatalf("untouched row changed:\nbefore %s\nafter  %s", otherBefore, lines[1])
	}
	row, err := ParseRow([]byte(lines[0]))
	if err != nil {
		t.Fatal(err)
	}
	if got := row.String("worktree"); got != wt {
		t.Fatalf("worktree = %q, want %q", got, wt)
	}
	if got := row.String("branch"); got != br {
		t.Fatalf("branch = %q, want %q", got, br)
	}
	if raw := sessionsRaw(t, lines[0]); string(raw) != `["A", "B", "C"]` {
		t.Fatalf("sessions raw = %s, want [\"A\", \"B\", \"C\"] (spaced, in order)", raw)
	}
	// The pre-existing fields keep their verbatim bytes (spaced style intact).
	if !strings.HasPrefix(lines[0], `{"id": "BT-1", "title": "first", "status": "complete", "priority": "P2"`) {
		t.Fatalf("existing field bytes/order changed: %s", lines[0])
	}
	keys := row.Keys
	if n := len(keys); n < 3 || keys[n-3] != "worktree" || keys[n-2] != "branch" || keys[n-1] != "sessions" {
		t.Fatalf("new keys must be appended at the end of the row, got key order %v", keys)
	}
}

// TestUpdateWithoutWorktreeFieldsCreatesNoKeys: an update that does not pass
// the flags must leave the row without those keys — nil means UNTOUCHED, never
// "write an empty value".
func TestUpdateWithoutWorktreeFieldsCreatesNoKeys(t *testing.T) {
	b := seedBT037Board(t)
	complete := "complete"
	if _, err := b.UpdateTask("BT-1", UpdateSpec{Status: &complete}); err != nil {
		t.Fatal(err)
	}
	row := taskRowByID(t, b, "BT-1")
	for _, k := range []string{"worktree", "branch", "sessions"} {
		if _, ok := row[k]; ok {
			t.Fatalf("update without flags created key %q: %v", k, row)
		}
	}
	if row["status"] != "complete" {
		t.Fatalf("status = %v, want complete", row["status"])
	}
}

// TestUpdateSessionsReplacesList: --session REPLACES the array (never merges),
// so a second update records exactly the new ids in the given order.
func TestUpdateSessionsReplacesList(t *testing.T) {
	b := seedBT037Board(t)
	first := []string{"old-1", "old-2"}
	if _, err := b.UpdateTask("BT-1", UpdateSpec{Sessions: &first}); err != nil {
		t.Fatal(err)
	}
	if raw := sessionsRaw(t, taskLines(t, b)[0]); string(raw) != `["old-1", "old-2"]` {
		t.Fatalf("first write sessions = %s", raw)
	}
	second := []string{"new-1", "new-2", "new-3"}
	if _, err := b.UpdateTask("BT-1", UpdateSpec{Sessions: &second}); err != nil {
		t.Fatal(err)
	}
	line := taskLines(t, b)[0]
	if raw := sessionsRaw(t, line); string(raw) != `["new-1", "new-2", "new-3"]` {
		t.Fatalf("replaced sessions = %s, want [\"new-1\", \"new-2\", \"new-3\"]", raw)
	}
	if strings.Contains(line, "old-") {
		t.Fatalf("previous session ids survived the replace: %s", line)
	}
	// Three --session occurrences parse back as a three-element array, in order.
	var got []string
	row, err := ParseRow([]byte(line))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(row.Get("sessions"), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "new-1" || got[1] != "new-2" || got[2] != "new-3" {
		t.Fatalf("sessions = %v, want the three ids in order", got)
	}
}

// TestValidateAndDoctorTolerateWorktreeFields: a board carrying the new fields
// validates and doctors clean — the field walkers tolerate unknown keys, so no
// allowlist change was needed and no new finding appears.
func TestValidateAndDoctorTolerateWorktreeFields(t *testing.T) {
	b := seedBT037Board(t)
	wt := "/home/kara/wt-bt-schema"
	br := "bt/session-worktree-fields"
	list := []string{"s1", "s2"}
	if _, err := b.UpdateTask("BT-1", UpdateSpec{Worktree: &wt, Branch: &br, Sessions: &list}); err != nil {
		t.Fatal(err)
	}
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() {
		t.Fatalf("validate failed on a board with the new fields:\n%s", rep.RenderText())
	}
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "worktree") || strings.Contains(f.Msg, "branch") || strings.Contains(f.Msg, "sessions") {
			t.Fatalf("validate itemized the new fields: [%s] %s", f.Level, f.Msg)
		}
	}
	drep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	if drep.HasErrors() {
		t.Fatalf("doctor failed on a board with the new fields:\n%s", drep.RenderText())
	}
}

// containsStr is a tiny ordered-membership helper for change lists.
func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// indexOfKey returns the position of key in a row's key order (-1 when absent).
func indexOfKey(keys []string, key string) int {
	for i, k := range keys {
		if k == key {
			return i
		}
	}
	return -1
}
