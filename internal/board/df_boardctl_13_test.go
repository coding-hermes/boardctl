package board

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// DF-BOARDCTL-13: `update` accepts --priority and --evidence-run-id. BT-060
// documented the title/priority contradiction class and shipped update
// --title, but the priority FIELD stayed unrepairable through the CLI
// (dogfood run 15 measured `update <id> --priority P1` exiting "flag
// provided but not defined: -priority") — rows created before the BT-060 fix
// kept their stale priority forever. Same defect for --evidence-run-id:
// SG-126 gave create the flag but never update, so run evidence could not be
// recorded on an EXISTING row. These tests pin the whole write contract,
// following bt060_update_title_test.go / bt037_fields_test.go's shape:
//
//   - --priority rewrites the priority in place through the SAME write-time
//     gate as create (BT-007): bare digits and case variants normalize onto
//     the canonical P0-P3 vocabulary, junk is rejected with NOTHING written
//   - --evidence-run-id merges one {run_id, ts} entry into the row's detail
//     field (the SG-126 envelope), keeping unknown detail keys and the text
//     payload verbatim — withEvidence's exact-entry idempotency semantics
//   - untouched rows stay byte-identical; untouched fields stay verbatim
//   - an omitted flag never touches the key (nil-untouched discipline,
//     cf. TestUpdateWithoutWorktreeFieldsCreatesNoKeys)
//   - --priority counts as a change flag on its own
//   - a flag-only priority update appends NO event (the historical no-event
//     behavior of every flag-only update except title/status)
//
// seedDF13Board: spaced-style rows — one with updated_at, one without, one
// carrying a rich object detail — so value encoding runs in the file's own
// style and the evidence merge has real prior state to preserve.
func seedDF13Board(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id": "DF13-1", "title": "first", "status": "pending", "priority": "P2", "updated_at": "2026-09-25 01:02:03"}` + "\n" +
			`{"id": "DF13-2", "title": "second", "status": "pending", "priority": "P2"}` + "\n" +
			`{"id": "DF13-3", "title": "third", "status": "review", "priority": "P1", "detail": {"qa_owner": "kai", "fingerprint": "deadbeef", "evidence": [{"run_id": "RUN-A", "ts": "2026-09-01 00:00:00"}], "text": "seen again"}}` + "\n",
		"events.jsonl": `{"id": 1, "timestamp": "2026-09-25 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		"board.jsonl":  `{"project": "df13", "namespace": "df13", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": "abc1234"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// df13DetailRaw returns the verbatim detail value bytes of the named row.
func df13DetailRaw(t *testing.T, b *Board, id string) []byte {
	t.Helper()
	var row *Row
	if err := IterParsed(mustReadLines(t, b.TasksPath()), func(r *Row, _ int, _ []byte) error {
		if r.String("id") == id {
			row = r
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if row == nil {
		t.Fatalf("task %s not found", id)
	}
	raw := row.Get("detail")
	if raw == nil {
		t.Fatalf("task %s carries no detail key", id)
	}
	return raw
}

// mustReadLines reads a JSONL file into raw lines for IterParsed.
func mustReadLines(t *testing.T, path string) [][]byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out [][]byte
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		out = append(out, []byte(l))
	}
	return out
}

// TestUpdatePriorityRewritesRowBytePreserving: --priority replaces the
// priority through create's write-time vocabulary, keeps the key in its
// original position, leaves every other byte of the row and every other row
// byte-identical, refreshes updated_at, and appends NO event.
func TestUpdatePriorityRewritesRowBytePreserving(t *testing.T) {
	b := seedDF13Board(t)
	otherBefore := taskLines(t, b)[1]
	eventsBefore, err := os.ReadFile(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}

	p := "P1"
	changed, err := b.UpdateTask("DF13-1", UpdateSpec{Priority: &p})
	if err != nil {
		t.Fatalf("update --priority rejected: %v", err)
	}
	if !containsStr(changed, "priority") {
		t.Fatalf("changed = %v, missing %q", changed, "priority")
	}
	if containsStr(changed, "detail") {
		t.Fatalf("priority-only update touched detail: %v", changed)
	}

	lines := taskLines(t, b)
	if lines[1] != otherBefore {
		t.Fatalf("untouched row changed:\nbefore %s\nafter  %s", otherBefore, lines[1])
	}
	row, err := ParseRow([]byte(lines[0]))
	if err != nil {
		t.Fatal(err)
	}
	if got := row.String("priority"); got != "P1" {
		t.Fatalf("priority = %q, want P1", got)
	}
	// priority keeps its ORIGINAL key position (4th key), not appended at
	// the end — the field exists on the row, SetRaw replaces in place.
	if len(row.Keys) < 4 || row.Keys[3] != "priority" {
		t.Fatalf("priority key moved: key order %v", row.Keys)
	}
	// every other field keeps its verbatim spaced style, updated_at refreshed
	if !strings.HasPrefix(lines[0], `{"id": "DF13-1", "title": "first", "status": "pending", "priority": "P1", "updated_at":`) {
		t.Fatalf("existing field bytes/order changed: %s", lines[0])
	}
	// flag-only priority update: no audit event (mirrors worker_status and
	// friends; only --status and --title have documented event behavior).
	eventsAfter, err := os.ReadFile(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(eventsBefore, eventsAfter) {
		t.Fatalf("priority-only update appended an event:\nbefore %s\nafter  %s", eventsBefore, eventsAfter)
	}
}

// TestUpdatePriorityNormalizesLikeCreate: the write-time gate is create's
// BT-007 gate verbatim — bare digits and case/whitespace variants map onto
// the canonical P0-P5 set so stats never splits groups.
func TestUpdatePriorityNormalizesLikeCreate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1", "P1"},
		{"p3", "P3"},
		{" P0 ", "P0"},
		{"P2", "P2"},
		{"4", "P4"},
		{"5", "P5"},
		{"p4", "P4"},
		{" P5 ", "P5"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			b := seedDF13Board(t)
			v := c.in
			if _, err := b.UpdateTask("DF13-1", UpdateSpec{Priority: &v}); err != nil {
				t.Fatalf("update --priority %q rejected: %v", c.in, err)
			}
			if got := taskRowByID(t, b, "DF13-1")["priority"]; got != c.want {
				t.Fatalf("priority = %v, want %q", got, c.want)
			}
		})
	}
}

// TestUpdateRejectsJunkPriority: out-of-vocabulary spellings fail the whole
// update with create's error text style — and the file is untouched.
func TestUpdateRejectsJunkPriority(t *testing.T) {
	for _, junk := range []string{"PX", "P9", "p8", "high"} {
		t.Run(junk, func(t *testing.T) {
			b := seedDF13Board(t)
			before, err := os.ReadFile(b.TasksPath())
			if err != nil {
				t.Fatal(err)
			}
			v := junk
			_, err = b.UpdateTask("DF13-1", UpdateSpec{Priority: &v})
			if err == nil {
				t.Fatalf("junk priority %q accepted", junk)
			}
			if !strings.Contains(err.Error(), "not in vocabulary {P0,P1,P2,P3,P4,P5}") {
				t.Fatalf("error does not name the vocabulary gate: %v", err)
			}
			after, err := os.ReadFile(b.TasksPath())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatalf("rejected update mutated tasks.jsonl")
			}
		})
	}
}

// TestUpdateEvidenceRunIDRecordsDetail: --evidence-run-id writes the SG-126
// envelope into the row's detail field, appended at the end of the row's key
// order when the row carries no detail, in the file's own style.
func TestUpdateEvidenceRunIDRecordsDetail(t *testing.T) {
	b := seedDF13Board(t)
	otherBefore := taskLines(t, b)[0]

	run := "DF-RUN-15"
	changed, err := b.UpdateTask("DF13-2", UpdateSpec{EvidenceRunID: run})
	if err != nil {
		t.Fatalf("update --evidence-run-id rejected: %v", err)
	}
	if !containsStr(changed, "detail") {
		t.Fatalf("changed = %v, missing %q", changed, "detail")
	}

	lines := taskLines(t, b)
	if lines[0] != otherBefore {
		t.Fatalf("untouched row changed:\nbefore %s\nafter  %s", otherBefore, lines[0])
	}
	row, err := ParseRow([]byte(lines[1]))
	if err != nil {
		t.Fatal(err)
	}
	var detail map[string]json.RawMessage
	if err := json.Unmarshal(row.Get("detail"), &detail); err != nil {
		t.Fatalf("detail is not a JSON object: %v (%s)", err, row.Get("detail"))
	}
	var evidence []map[string]string
	if err := json.Unmarshal(detail["evidence"], &evidence); err != nil {
		t.Fatalf("detail.evidence is not an array: %v", err)
	}
	if len(evidence) != 1 || evidence[0]["run_id"] != run || evidence[0]["ts"] == "" {
		t.Fatalf("evidence = %v, want one entry with run_id %q and a ts", evidence, run)
	}
	// detail is a NEW key on the row — appended at the end of key order.
	if row.Keys[len(row.Keys)-1] != "detail" {
		t.Fatalf("detail key not appended at the end: %v", row.Keys)
	}
}

// TestUpdateEvidenceRunIDMergesObjectDetailLosslessly: an object-form detail
// keeps every unknown key verbatim, its fingerprint and text payload intact,
// and APPENDS the new evidence entry after the existing ones (withEvidence
// semantics — never a replacement).
func TestUpdateEvidenceRunIDMergesObjectDetailLosslessly(t *testing.T) {
	b := seedDF13Board(t)
	detailBefore := df13DetailRaw(t, b, "DF13-3")

	run := "RUN-B"
	if _, err := b.UpdateTask("DF13-3", UpdateSpec{EvidenceRunID: run}); err != nil {
		t.Fatal(err)
	}

	raw := df13DetailRaw(t, b, "DF13-3")
	if bytes.Equal(raw, detailBefore) {
		t.Fatalf("detail untouched by --evidence-run-id: %s", raw)
	}
	var detail map[string]json.RawMessage
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatalf("detail does not parse: %v (%s)", err, raw)
	}
	// unknown key preserved verbatim
	if got := strings.Trim(string(detail["qa_owner"]), `"`); got != "kai" {
		t.Fatalf("unknown detail key lost: %s", raw)
	}
	// SG-126 metadata preserved
	if got := strings.Trim(string(detail["fingerprint"]), `"`); got != "deadbeef" {
		t.Fatalf("fingerprint lost: %s", raw)
	}
	if got := strings.Trim(string(detail["text"]), `"`); got != "seen again" {
		t.Fatalf("text payload lost: %s", raw)
	}
	// evidence APPENDED, prior entry verbatim
	var evidence []struct {
		RunID string `json:"run_id"`
		TS    string `json:"ts"`
	}
	if err := json.Unmarshal(detail["evidence"], &evidence); err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 2 {
		t.Fatalf("evidence = %v, want 2 entries (prior + new)", evidence)
	}
	if evidence[0].RunID != "RUN-A" || evidence[0].TS != "2026-09-01 00:00:00" {
		t.Fatalf("prior evidence entry changed: %+v", evidence[0])
	}
	if evidence[1].RunID != "RUN-B" || evidence[1].TS == "" {
		t.Fatalf("new evidence entry wrong: %+v", evidence[1])
	}
}

// TestUpdateWithoutPriorityEvidenceLeavesKeysUntouched: omitted flags are
// UNTOUCHED — a status-only update must neither rewrite the priority, nor
// create a detail key on a row that has none, nor disturb the detail bytes
// of a row that has one.
func TestUpdateWithoutPriorityEvidenceLeavesKeysUntouched(t *testing.T) {
	b := seedDF13Board(t)
	lineBefore := taskLines(t, b)[0]
	detail3Before := df13DetailRaw(t, b, "DF13-3")

	s := "complete"
	if _, err := b.UpdateTask("DF13-1", UpdateSpec{Status: &s}); err != nil {
		t.Fatal(err)
	}
	line := taskLines(t, b)[0]
	if !strings.HasPrefix(line, `{"id": "DF13-1", "title": "first", "status": "complete", "priority": "P2"`) {
		t.Fatalf("priority rewritten by a status-only update: %s", line)
	}
	if strings.Contains(line, `"detail"`) {
		t.Fatalf("detail key created by a status-only update: %s", line)
	}
	if strings.Count(line, `"priority"`) != strings.Count(lineBefore, `"priority"`) {
		t.Fatalf("priority key count changed: %s", line)
	}

	// a priority-only update on the detail-bearing row leaves detail verbatim
	p := "P0"
	if _, err := b.UpdateTask("DF13-3", UpdateSpec{Priority: &p}); err != nil {
		t.Fatal(err)
	}
	if got := df13DetailRaw(t, b, "DF13-3"); !bytes.Equal(got, detail3Before) {
		t.Fatalf("detail changed by a priority-only update:\nbefore %s\nafter  %s", detail3Before, got)
	}
}
