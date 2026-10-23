package board

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRepairEvents_AppendCannotBeLost verifies the full repair critical section.
// The hook pauses after planning and before the atomic replacement; the append
// is started during that pause. With writeLockBypass=true the append completes
// before the stale replacement and this same assertion is RED (the marker is
// lost), proving the regression is testing the production guard rather than
// merely observing sequential behavior.
func TestConcurrentRepairEventsAppendCannotBeLost(t *testing.T) {
	b := newTestBoard(t)
	if err := os.WriteFile(b.EventsPath(), []byte("{\"id\":1,\"event_type\":\"audit\"}\n{\"id\":3,\"event_type\":\"audit\"}\n{\"id\":2,\"event_type\":\"audit\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repairBoard, err := Resolve(b.Dir)
	if err != nil {
		t.Fatal(err)
	}
	appendBoard, err := Resolve(b.Dir)
	if err != nil {
		t.Fatal(err)
	}

	repairReady := make(chan struct{})
	appendStarted := make(chan struct{})
	appendDone := make(chan error, 1)
	oldHook := repairEventsBeforeRewriteHook
	repairEventsBeforeRewriteHook = func() {
		close(repairReady)
		<-appendStarted
	}
	defer func() { repairEventsBeforeRewriteHook = oldHook }()

	go func() {
		<-repairReady
		close(appendStarted)
		_, err := appendBoard.AppendEvent(EventSpec{
			Type:        "audit",
			DetailText:  strPtr("PROBE-EV-1"),
			NoTickWrite: true,
		})
		appendDone <- err
	}()

	if _, err := repairBoard.RepairEvents(false); err != nil {
		t.Fatalf("RepairEvents(apply): %v", err)
	}
	select {
	case err := <-appendDone:
		if err != nil {
			t.Fatalf("concurrent append: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent append did not complete after repair released the guard")
	}
	got, err := os.ReadFile(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(got), "PROBE-EV-1"); count != 1 {
		t.Fatalf("concurrent append marker count = %d, want 1; events=%s", count, got)
	}
}

// TestRepairEvents_descendingIds verifies that --repair-events renumbers
// descending event ids to ascending order (DF-BOARDCTL-38 AC1-AC2).
func TestRepairEvents_descendingIds(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")
	input := `{ "id": 1, "action": "create" }
{ "id": 2, "action": "update" }
{ "id": 5, "action": "assign" }
{ "id": 3, "action": "resolve" }
{ "id": 4, "action": "comment" }
{ "id": 10, "action": "close" }
{ "id": 7, "action": "reopen" }
{ "id": 8, "action": "reassign" }
{ "id": 9, "action": "relabel" }`
	if err := os.WriteFile(eventsPath, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	b := &Board{
		Dir:        dir,
		Topology:   "B",
		eventsPath: eventsPath,
	}

	rep, err := b.RepairEvents(true)
	if err != nil {
		t.Fatalf("RepairEvents(dry=true): %v", err)
	}
	if rep.Clean {
		t.Fatal("expected descending ids to be found, but board was clean")
	}
	if rep.Descending != 5 {
		t.Errorf("expected 5 descending ids, got %d", rep.Descending)
	}
	if rep.FixedCount != 5 {
		t.Errorf("expected 5 fixed events, got %d", rep.FixedCount)
	}
	if rep.TotalEvents != 9 {
		t.Errorf("expected 9 total events, got %d", rep.TotalEvents)
	}
	if rep.Applied {
		t.Error("dry-run should not mark applied")
	}

	rep2, err := b.RepairEvents(false)
	if err != nil {
		t.Fatalf("RepairEvents(apply): %v", err)
	}
	if !rep2.Applied {
		t.Error("apply should mark applied")
	}
	if rep2.FixedCount != 5 {
		t.Errorf("expected 5 fixed events on apply, got %d", rep2.FixedCount)
	}

	expectedIDs := []int64{1, 2, 5, 6, 7, 10, 11, 12, 13}
	lines, err := ReadJSONLLines(eventsPath)
	if err != nil {
		t.Fatalf("read events after apply: %v", err)
	}
	if len(lines) != 9 {
		t.Fatalf("expected 9 lines after apply, got %d", len(lines))
	}

	for i, expID := range expectedIDs {
		var m map[string]any
		if err := json.Unmarshal(lines[i], &m); err != nil {
			t.Fatalf("line %d parse: %v", i+1, err)
		}
		id, ok := m["id"].(float64)
		if !ok {
			t.Errorf("line %d: expected numeric id, got %T", i+1, m["id"])
			continue
		}
		if int64(id) != expID {
			t.Errorf("line %d: expected id %d, got %d", i+1, expID, int64(id))
		}
	}

	rep3, err := b.RepairEvents(true)
	if err != nil {
		t.Fatalf("RepairEvents after repair: %v", err)
	}
	if !rep3.Clean {
		t.Errorf("expected clean after repair, got %d descending", rep3.Descending)
	}
}

// TestRepairEvents_alreadyCanonical verifies no-op when ids are ascending.
func TestRepairEvents_alreadyCanonical(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")
	input := `{ "id": 1, "action": "create" }
{ "id": 2, "action": "update" }
{ "id": 3, "action": "assign" }
{ "id": 4, "action": "resolve" }
{ "id": 5, "action": "close" }`
	if err := os.WriteFile(eventsPath, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	b := &Board{
		Dir:        dir,
		Topology:   "B",
		eventsPath: eventsPath,
	}

	rep, err := b.RepairEvents(false)
	if err != nil {
		t.Fatalf("RepairEvents: %v", err)
	}
	if !rep.Clean {
		t.Errorf("expected clean board, got %d descending", rep.Descending)
	}
	if rep.TotalEvents != 5 {
		t.Errorf("expected 5 events, got %d", rep.TotalEvents)
	}
}

// TestRepairEvents_legacyShape verifies events without numeric ids are skipped.
func TestRepairEvents_legacyShape(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")
	input := `{ "id": 1, "action": "create" }
{ "id": 5, "action": "assign" }
{ "id": "legacy-ref", "action": "legacy_event" }
{ "id": 3, "action": "resolve" }
{ "action": "no_id_event" }
{ "id": 10, "action": "close" }
{ "id": 8, "action": "reopen" }`
	if err := os.WriteFile(eventsPath, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	b := &Board{
		Dir:        dir,
		Topology:   "B",
		eventsPath: eventsPath,
	}

	rep, err := b.RepairEvents(true)
	if err != nil {
		t.Fatalf("RepairEvents: %v", err)
	}
	if rep.Clean {
		t.Fatal("expected descending ids")
	}
	if rep.NonNumeric != 2 {
		t.Errorf("expected 2 non-numeric events, got %d", rep.NonNumeric)
	}
	if rep.Descending != 2 {
		t.Errorf("expected 2 descending ids, got %d", rep.Descending)
	}

	rep2, err := b.RepairEvents(false)
	if err != nil {
		t.Fatalf("RepairEvents apply: %v", err)
	}
	if !rep2.Applied {
		t.Error("expected applied")
	}

	lines, err := ReadJSONLLines(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	testLines := []struct {
		line      int
		expID     any
		expectKey string
	}{
		{1, int64(1), "id"},
		{2, int64(5), "id"},
		{3, "legacy-ref", "id"},
		{4, int64(6), "id"},
		{5, nil, ""},
		{6, int64(10), "id"},
		{7, int64(11), "id"},
	}
	for _, tc := range testLines {
		var m map[string]any
		if err := json.Unmarshal(lines[tc.line-1], &m); err != nil {
			t.Fatalf("line %d parse: %v", tc.line, err)
		}
		if tc.expectKey == "" {
			if _, has := m["id"]; has {
				t.Errorf("line %d: expected no id, got: %v", tc.line, m)
			}
			continue
		}
		switch v := tc.expID.(type) {
		case int64:
			if m["id"] != float64(v) {
				t.Errorf("line %d: expected id=%d, got %v", tc.line, v, m["id"])
			}
		case string:
			if m["id"] != v {
				t.Errorf("line %d: expected id=%q, got %v", tc.line, v, m["id"])
			}
		}
	}
}

// TestRepairEvents_topologyA verifies event repair on a topology-A board
// (board.jsonl header present).
func TestRepairEvents_topologyA(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")
	// IDs: 1, 5, 3, 10, 7 — descending at line 3 (3<5) and line 5 (7<10)
	// Running max: 1→1, 5→5, 3→6, 10→10, 7→11
	input := `{ "id": 1, "action": "start" }
{ "id": 5, "action": "update" }
{ "id": 3, "action": "descend" }
{ "id": 10, "action": "max" }
{ "id": 7, "action": "descend2" }`
	if err := os.WriteFile(eventsPath, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	b := &Board{
		Dir:        dir,
		Topology:   "A",
		eventsPath: eventsPath,
	}

	if b.Topology != "A" {
		t.Fatalf("expected topology A, got %s", b.Topology)
	}

	rep, err := b.RepairEvents(true)
	if err != nil {
		t.Fatalf("RepairEvents: %v", err)
	}
	if rep.Descending != 2 {
		t.Errorf("expected 2 descending ids, got %d", rep.Descending)
	}

	rep2, err := b.RepairEvents(false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !rep2.Applied {
		t.Error("expected applied")
	}

	lines, err := ReadJSONLLines(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	expected := []int64{1, 5, 6, 10, 11}
	for i, exp := range expected {
		var m map[string]any
		if err := json.Unmarshal(lines[i], &m); err != nil {
			t.Fatalf("line %d: %v", i+1, err)
		}
		got, ok := m["id"].(float64)
		if !ok {
			t.Errorf("line %d: expected numeric id, got %T", i+1, m["id"])
			continue
		}
		if int64(got) != exp {
			t.Errorf("line %d: expected id=%d, got %d", i+1, exp, int64(got))
		}
	}

	rep3, err := b.RepairEvents(true)
	if err != nil {
		t.Fatalf("post-repair: %v", err)
	}
	if !rep3.Clean {
		t.Errorf("expected clean after topology-A repair, got %d descending", rep3.Descending)
	}
}

// TestRepairEvents_dryRunCache verifies dry-run cache matches applied output.
func TestRepairEvents_dryRunCache(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")
	input := `{ "id": 1, "action": "a" }
{ "id": 5, "action": "b" }
{ "id": 3, "action": "c" }`
	if err := os.WriteFile(eventsPath, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	b := &Board{
		Dir:        dir,
		Topology:   "B",
		eventsPath: eventsPath,
	}

	rep, err := b.RepairEvents(true)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if rep.AppliedPath == "" {
		t.Fatal("dry-run should write cache file")
	}

	cacheData, err := os.ReadFile(rep.AppliedPath)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}

	rep2, err := b.RepairEvents(false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !rep2.Applied {
		t.Error("expected applied")
	}

	afterData, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	if !bytes.Equal(cacheData, afterData) {
		t.Errorf("dry-run cache != applied output:\ncache: %s\napplied: %s", cacheData, afterData)
	}
}

// TestRepairEvents_preservesNonIDBytes pins DF-BOARDCTL-44 AC1/AC2: renumbering
// replaces ONLY the raw byte range of the id token on the renumbered line.
// Every other byte survives verbatim — key order (deliberately
// non-alphabetical), separator spacing, double spaces inside string values,
// escaped quotes, and an integer above 2^53 (which the old map[string]any
// round-trip rounded through float64).
func TestRepairEvents_preservesNonIDBytes(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")

	const firstLine = `{ "id": 5, "action": "create", "note": "first  line" }`
	const richLine = `{ "title": "fix  double  spaces", "detail": "cmd: rsync -a  /x  /y", "tick_number": 9007199254740993, "zeta": "last  alphabetically", "id": 2, "nested": { "id": 42, "note": "nested  spaces" }, "quote": "say \"id\": 9  now" }`
	// The only difference: the renumbered line's id token 2 -> 6.
	before := firstLine + "\n" + richLine + "\n"
	after := firstLine + "\n" + strings.Replace(richLine, `"id": 2,`, `"id": 6,`, 1) + "\n"

	if err := os.WriteFile(eventsPath, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	b := &Board{Dir: dir, Topology: "B", eventsPath: eventsPath}

	rep, err := b.RepairEvents(false)
	if err != nil {
		t.Fatalf("RepairEvents(apply): %v", err)
	}
	if !rep.Applied {
		t.Fatal("expected applied")
	}
	if rep.FixedCount != 1 {
		t.Fatalf("expected exactly 1 renumbered event, got %d", rep.FixedCount)
	}

	got, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	if string(got) != after {
		t.Errorf("only the id token may change\nwant: %q\ngot:  %q", after, got)
	}

	// State the AC's bytes explicitly, so a future serializer change that
	// happens to keep the golden above but mangles content still fails.
	for _, want := range []string{
		"fix  double  spaces",
		"cmd: rsync -a  /x  /y",
		"9007199254740993",
		"nested  spaces",
		`say \"id\": 9  now`,
		`"nested": { "id": 42`,
	} {
		if !bytes.Contains(got, []byte(want)) {
			t.Errorf("preserved bytes missing %q in:\n%s", want, got)
		}
	}
	if bytes.Contains(got, []byte("9007199254740992")) {
		t.Errorf("integer above 2^53 was rounded:\n%s", got)
	}

	// Key order is part of the contract: compare the ordered key lists of the
	// renumbered line before and after.
	beforeRow, err := ParseRow([]byte(richLine))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	gotLines, err := ReadJSONLLines(eventsPath)
	if err != nil {
		t.Fatalf("read lines: %v", err)
	}
	afterRow, err := ParseRow(gotLines[1])
	if err != nil {
		t.Fatalf("parse repaired line: %v", err)
	}
	if strings.Join(beforeRow.Keys, ",") != strings.Join(afterRow.Keys, ",") {
		t.Errorf("key order changed: before %v, after %v", beforeRow.Keys, afterRow.Keys)
	}
	if id, ok := afterRow.Int("id"); !ok || id != 6 {
		t.Errorf("expected renumbered id 6, got %d (ok=%v)", id, ok)
	}
	if id, ok := afterRow.Int("tick_number"); !ok || id != 9007199254740993 {
		t.Errorf("expected tick_number 9007199254740993 intact, got %d (ok=%v)", id, ok)
	}
}

// TestRepairEvents_largeIntBeyondFloatPrecision pins the >2^53 half of
// DF-BOARDCTL-44: a renumbered line's integer fields (and the running-max id
// itself) must survive float64-free, and running max + 1 must stay exact.
func TestRepairEvents_largeIntBeyondFloatPrecision(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")

	before := `{"id": 9007199254740993, "tick_number": 9007199254740993, "detail": "a  b"}
{"id": 1, "tick_number": 9007199254740993, "detail": "c  d"}
`
	after := `{"id": 9007199254740993, "tick_number": 9007199254740993, "detail": "a  b"}
{"id": 9007199254740994, "tick_number": 9007199254740993, "detail": "c  d"}
`
	if err := os.WriteFile(eventsPath, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	b := &Board{Dir: dir, Topology: "B", eventsPath: eventsPath}
	if _, err := b.RepairEvents(false); err != nil {
		t.Fatalf("RepairEvents(apply): %v", err)
	}

	got, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if string(got) != after {
		t.Errorf("large-integer event bytes changed\nwant: %q\ngot:  %q", after, got)
	}
	if bytes.Contains(got, []byte("9007199254740992")) {
		t.Errorf("value above 2^53 was rounded through float64:\n%s", got)
	}
}

// TestRepairEvents_renumberEventIDSplicesOnlyID is the unit-level guard on the
// raw-byte splice: compact and spaced lines, escaping, nested objects that
// carry their own "id", and the refusal paths for missing/non-numeric ids.
func TestRepairEvents_renumberEventIDSplicesOnlyID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		id   int64
		want string
	}{
		{
			name: "compact",
			in:   `{"id":1,"x":"y  z"}`,
			id:   2,
			want: `{"id":2,"x":"y  z"}`,
		},
		{
			name: "spaced",
			in:   `{ "zeta": 1, "id": 7, "alpha": 2 }`,
			id:   8,
			want: `{ "zeta": 1, "id": 8, "alpha": 2 }`,
		},
		{
			name: "escaped quotes in preceding and following values",
			in:   `{"a":"say \"id\": 4","id":4,"b":"x  y"}`,
			id:   5,
			want: `{"a":"say \"id\": 4","id":5,"b":"x  y"}`,
		},
		{
			name: "nested id untouched",
			in:   `{"nested":{"id":1,"k":[1,2]},"id":3}`,
			id:   4,
			want: `{"nested":{"id":1,"k":[1,2]},"id":4}`,
		},
		{
			name: "brackets and braces inside strings",
			in:   `{"note":"}{] id","id":9}`,
			id:   10,
			want: `{"note":"}{] id","id":10}`,
		},
		{
			name: "id last field, no trailing newline",
			in:   `{"tick_number":9007199254740993,"id":1}`,
			id:   2,
			want: `{"tick_number":9007199254740993,"id":2}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := renumberEventID([]byte(tc.in), tc.id)
			if err != nil {
				t.Fatalf("renumberEventID: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("want %q, got %q", tc.want, got)
			}
		})
	}

	// Legacy shapes are refused by the helper (RepairEvents itself skips them
	// before reaching it, which the legacy-shape test above pins).
	for _, in := range []string{`{"id":"legacy-ref"}`, `{"action":"no_id"}`, `{"id":null}`} {
		if _, err := renumberEventID([]byte(in), 5); err == nil {
			t.Errorf("expected an error for %s, got none", in)
		}
	}
}

// validRepairedBoard writes a topology-B board whose events.jsonl carries the
// DF-BOARDCTL-45 fixture: numeric ids 10, 9, 11 in FILE ORDER. Line 2 descends
// below 10, so the pre-fix repair renumbered it to runningMax+1 = 11 — the id
// line 3 still carries — and the census reported "duplicate ids: 0" because it
// counted only the INPUT.
func validRepairedBoard(t *testing.T, events string) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"project":"t","namespace":"t","version":3,"ticks_total":11,"ticks_idle":0,"last_commit":"abc1234"}` + "\n" +
			`{"id":"DUP-1","title":"fixture task","status":"pending","priority":"P2"}` + "\n",
		"events.jsonl": events,
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// eventIDsOf reads the repaired events.jsonl and returns its numeric ids in
// file order, failing on any line whose id is not a number.
func eventIDsOf(t *testing.T, b *Board) []int64 {
	t.Helper()
	lines, err := ReadJSONLLines(b.EventsPath())
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	ids := make([]int64, 0, len(lines))
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		row, err := ParseRow(line)
		if err != nil {
			t.Fatalf("line %d parse: %v", i+1, err)
		}
		id, ok := row.Int("id")
		if !ok {
			t.Fatalf("line %d: id is not a JSON number: %s", i+1, line)
		}
		ids = append(ids, id)
	}
	return ids
}

// TestRepairEvents_neverCreatesDuplicateID is the DF-BOARDCTL-45 regression:
// the repair may not mint an id another event already carries, and its
// duplicate count must describe the REPAIRED output. Pre-fix this fixture
// repaired to 10, 11, 11 with a "duplicate ids: 0" census; post-fix it repairs
// to 10, 11, 12 with a 0 census that is actually true, and validate carries no
// duplicate event id line.
func TestRepairEvents_neverCreatesDuplicateID(t *testing.T) {
	b := validRepairedBoard(t, `{"id": 10, "event_type": "audit", "detail": "first"}`+"\n"+
		`{"id": 9, "event_type": "audit", "detail": "second  spaced"}`+"\n"+
		`{"id": 11, "event_type": "audit", "detail": "third"}`+"\n")

	rep, err := b.RepairEvents(false)
	if err != nil {
		t.Fatalf("RepairEvents(apply): %v", err)
	}
	if !rep.Applied {
		t.Fatal("expected the repair to be applied")
	}
	// Two events move: the descending 9, and the 11 it would have collided with.
	if rep.Descending != 2 || rep.FixedCount != 2 {
		t.Errorf("descending/fixed = %d/%d, want 2/2", rep.Descending, rep.FixedCount)
	}
	// The census is taken on the repaired output, which holds no duplicate.
	if rep.Duplicates != 0 {
		t.Errorf("reported duplicate count = %d, want 0 (census of the repaired output)", rep.Duplicates)
	}

	ids := eventIDsOf(t, b)
	want := []int64{10, 11, 12}
	if len(ids) != len(want) {
		t.Fatalf("repaired ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("repaired ids = %v, want %v", ids, want)
		}
	}
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("repair left two events sharing id %d: %v", id, ids)
		}
		seen[id] = true
	}

	// DF-BOARDCTL-44's byte contract still holds for the renumbered lines.
	raw, err := os.ReadFile(b.EventsPath())
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if !bytes.Contains(raw, []byte("second  spaced")) {
		t.Errorf("non-id bytes of the renumbered event were not preserved:\n%s", raw)
	}

	// AC2: validate must not report a duplicate event id on the repaired file.
	vrep, err := b.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	rendered := vrep.RenderText()
	if strings.Contains(rendered, "duplicate event id") {
		t.Fatalf("validate still reports a duplicate event id:\n%s", rendered)
	}
	if vrep.HasErrors() {
		t.Fatalf("validate reports errors after repair:\n%s", rendered)
	}

	// The repair converges: a second pass has nothing left to do.
	again, err := b.RepairEvents(true)
	if err != nil {
		t.Fatalf("RepairEvents after repair: %v", err)
	}
	if !again.Clean || again.Duplicates != 0 {
		t.Errorf("second pass clean=%v duplicates=%d, want clean with 0 duplicates", again.Clean, again.Duplicates)
	}
}

// TestRepairEvents_duplicateCensusCountsOutput pins the other half of
// DF-BOARDCTL-45: the census is of the repaired file, not of the input. An
// untouched pre-existing duplicate is still reported; a duplicate the repair
// renumbers away is reported as 0 even though the input carried it.
func TestRepairEvents_duplicateCensusCountsOutput(t *testing.T) {
	cases := []struct {
		name      string
		events    string
		wantDupes int
		wantFixed int
		wantIDs   []int64
	}{
		{
			name:      "clean board still counts its pre-existing duplicates",
			events:    `{"id": 3}` + "\n" + `{"id": 3}` + "\n" + `{"id": 5}` + "\n",
			wantDupes: 1,
			wantFixed: 0,
			wantIDs:   []int64{3, 3, 5},
		},
		{
			name:      "duplicate the repair renumbers away is not reported",
			events:    `{"id": 3}` + "\n" + `{"id": 1}` + "\n" + `{"id": 3}` + "\n",
			wantDupes: 0,
			wantFixed: 2,
			wantIDs:   []int64{3, 4, 5},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := validRepairedBoard(t, tc.events)
			rep, err := b.RepairEvents(false)
			if err != nil {
				t.Fatalf("RepairEvents: %v", err)
			}
			if rep.Duplicates != tc.wantDupes {
				t.Errorf("duplicates = %d, want %d", rep.Duplicates, tc.wantDupes)
			}
			if rep.FixedCount != tc.wantFixed {
				t.Errorf("fixed = %d, want %d", rep.FixedCount, tc.wantFixed)
			}
			ids := eventIDsOf(t, b)
			if len(ids) != len(tc.wantIDs) {
				t.Fatalf("ids = %v, want %v", ids, tc.wantIDs)
			}
			for i := range tc.wantIDs {
				if ids[i] != tc.wantIDs[i] {
					t.Fatalf("ids = %v, want %v", ids, tc.wantIDs)
				}
			}
		})
	}
}
