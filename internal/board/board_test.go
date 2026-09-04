package board

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestBoard builds a minimal topology-A board on disk.
func newTestBoard(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("tasks.jsonl", `{"id":"EXIST-1","title":"Existing","status":"complete","priority":"P1"}`+"\n")
	write("events.jsonl", `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}`+"\n")
	write("board.jsonl", `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":"abc1234"}`+"\n")
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Topology != "A" {
		t.Fatalf("topology = %q, want A", b.Topology)
	}
	return b
}

func TestCreateRoundtripStylePreserved(t *testing.T) {
	b := newTestBoard(t)

	cx := int64(3)
	if _, err := b.Create(TaskRowSpec{
		ID:         "NEW-1",
		Title:      "New task",
		Status:     "pending",
		Priority:   "P2",
		Complexity: &cx,
		Reasoning:  "because testing",
	}); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimRight(raw, "\n"), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("tasks.jsonl has %d lines, want 2", len(lines))
	}
	// line 1 untouched byte-for-byte
	want1 := `{"id":"EXIST-1","title":"Existing","status":"complete","priority":"P1"}`
	if string(lines[0]) != want1 {
		t.Fatalf("existing row mutated:\n got %s\nwant %s", lines[0], want1)
	}
	// appended row parses as valid JSON with the expected fields
	var row map[string]any
	if err := json.Unmarshal(lines[1], &row); err != nil {
		t.Fatal(err)
	}
	if row["id"] != "NEW-1" || row["status"] != "pending" || row["priority"] != "P2" {
		t.Fatalf("appended row wrong: %s", lines[1])
	}
	if row["complexity"] != float64(3) {
		t.Fatalf("complexity = %v, want 3", row["complexity"])
	}
}

func TestCreateDuplicateIDRejected(t *testing.T) {
	b := newTestBoard(t)
	_, err := b.Create(TaskRowSpec{ID: "EXIST-1", Title: "dup", Status: "pending", Priority: "P1"})
	if err == nil {
		t.Fatal("duplicate id accepted")
	}
	if _, ok := err.(*ErrDuplicateTaskID); !ok {
		t.Fatalf("error type = %T, want *ErrDuplicateTaskID", err)
	}
}

func TestAppendEventSequencing(t *testing.T) {
	b := newTestBoard(t)
	if _, err := b.AppendEvent(EventSpec{Type: "audit", Actor: "foreman", DetailText: strPtr(`"tick 2 summary"`), Tick: i64Ptr(2)}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.AppendEvent(EventSpec{Type: "task_completed", TaskID: "EXIST-1", Actor: "foreman", DetailText: strPtr(`{"commit":"abc"}`), Tick: i64Ptr(2)}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimRight(raw, "\n"), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("events.jsonl has %d lines, want 3", len(lines))
	}
	var e map[string]any
	if err := json.Unmarshal(lines[1], &e); err != nil {
		t.Fatal(err)
	}
	if e["id"] != float64(2) || e["tick_number"] != float64(2) {
		t.Fatalf("second event wrong: %s", lines[1])
	}
	if err := json.Unmarshal(lines[2], &e); err != nil {
		t.Fatal(err)
	}
	if e["id"] != float64(3) || e["event_type"] != "task_completed" || e["task_id"] != "EXIST-1" {
		t.Fatalf("third event wrong: %s", lines[2])
	}
}

func TestUpdateTaskRewritesOnlyTargetRow(t *testing.T) {
	b := newTestBoard(t)
	status := "complete"
	commit := "deadbee"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Status: &status, CommitHash: &commit}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, `"status":"complete"`) || !strings.Contains(body, `"commit_hash":"deadbee"`) {
		t.Fatalf("update not applied: %s", body)
	}
	// the appended NEW row must be gone — only EXIST-1 remains, single line
	lines := bytes.Split(bytes.TrimRight(raw, "\n"), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("tasks.jsonl has %d lines, want 1", len(lines))
	}
}

func TestSetHeaderBump(t *testing.T) {
	b := newTestBoard(t)
	tt := int64(9)
	lc := "fff000"
	if _, err := b.SetHeader(HeaderUpdate{TicksTotal: &tt, LastCommit: &lc}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(b.headerPath)
	if err != nil {
		t.Fatal(err)
	}
	var hdr map[string]any
	if err := json.Unmarshal(raw, &hdr); err != nil {
		t.Fatal(err)
	}
	if hdr["ticks_total"] != float64(9) || hdr["last_commit"] != "fff000" {
		t.Fatalf("header not updated: %s", raw)
	}
	if hdr["project"] != "test" {
		t.Fatalf("unrelated header key lost: %s", raw)
	}
}

func TestValidateTopologyA(t *testing.T) {
	b := newTestBoard(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.Tasks != 1 || rep.Events != 1 {
		t.Fatalf("counts = %d/%d, want 1/1", rep.Tasks, rep.Events)
	}
	errs := 0
	for _, f := range rep.Findings {
		if f.Level == "error" {
			errs++
		}
	}
	if errs != 0 {
		t.Fatalf("unexpected errors on clean board: %+v", rep.Findings)
	}
}

func strPtr(s string) *string { return &s }
func i64Ptr(i int64) *int64   { return &i }

// BT-006: Resolve(-C) contract — repo root, .coding-hermes dir, and the
// board dir itself all land on the same board; an empty dir finds nothing.
func TestResolveRepoRootFindsNestedBoard(t *testing.T) {
	repo := t.TempDir()
	boardDir := filepath.Join(repo, ".coding-hermes", "board")
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  `{"id":"E-1","title":"Existing","status":"pending","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
	})

	b, err := Resolve(repo)
	if err != nil {
		t.Fatal(err)
	}
	if b.Dir != boardDir {
		t.Fatalf("Resolve(repo) = %q, want %q", b.Dir, boardDir)
	}
}

func TestResolveCodingHermesDirFindsItsBoard(t *testing.T) {
	repo := t.TempDir()
	boardDir := filepath.Join(repo, ".coding-hermes", "board")
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  `{"id":"E-1","title":"Existing","status":"pending","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
	})

	// The BT-006 bug: -C <repo>/.coding-hermes probed
	// .coding-hermes/.coding-hermes/board and missed the real board.
	b, err := Resolve(filepath.Join(repo, ".coding-hermes"))
	if err != nil {
		t.Fatal(err)
	}
	if b.Dir != boardDir {
		t.Fatalf("Resolve(<repo>/.coding-hermes) = %q, want %q", b.Dir, boardDir)
	}
}

func TestResolveBoardDirItself(t *testing.T) {
	repo := t.TempDir()
	boardDir := filepath.Join(repo, ".coding-hermes", "board")
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  `{"id":"E-1","title":"Existing","status":"pending","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
	})

	b, err := Resolve(boardDir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Dir != boardDir {
		t.Fatalf("Resolve(<board dir>) = %q, want %q", b.Dir, boardDir)
	}
}

func TestResolveEmptyDirNotFound(t *testing.T) {
	empty := t.TempDir()
	_, err := Resolve(empty)
	if !errors.Is(err, ErrBoardNotFound) {
		t.Fatalf("Resolve(<empty dir>) err = %v, want ErrBoardNotFound", err)
	}
}

// BT-006: init on <repo>/.coding-hermes must land the board at
// .coding-hermes/board (NOT .coding-hermes/.coding-hermes/board) so a
// follow-up Resolve on the .coding-hermes dir finds it.
func TestInitOnCodingHermesDirUsesItsBoardSubdir(t *testing.T) {
	repo := t.TempDir()
	chDir := filepath.Join(repo, ".coding-hermes")
	boardDir, _, err := Init(chDir, InitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(chDir, "board")
	if boardDir != want {
		t.Fatalf("Init(<repo>/.coding-hermes) board dir = %q, want %q", boardDir, want)
	}
	b, err := Resolve(chDir)
	if err != nil {
		t.Fatalf("Resolve(<repo>/.coding-hermes) after init: %v", err)
	}
	if b.Dir != want {
		t.Fatalf("Resolve found %q, want %q", b.Dir, want)
	}
}

// lastEventLine returns the last non-empty line of events.jsonl.
func lastEventLine(t *testing.T, b *Board) []byte {
	t.Helper()
	raw, err := os.ReadFile(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimRight(raw, "\n"), []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		if len(bytes.TrimSpace(lines[i])) > 0 {
			return lines[i]
		}
	}
	t.Fatal("events.jsonl has no rows")
	return nil
}

func eventLineCount(t *testing.T, b *Board) int {
	t.Helper()
	raw, err := os.ReadFile(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, l := range bytes.Split(raw, []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			n++
		}
	}
	return n
}

// BT-011: create must append the README-promised task_created event.
func TestCreateWritesTaskCreatedEvent(t *testing.T) {
	b := newTestBoard(t)
	if _, err := b.Create(TaskRowSpec{ID: "NEW-1", Title: "New task", Status: "pending"}); err != nil {
		t.Fatal(err)
	}
	// fixture board ships 1 event row; the create adds exactly one more
	if n := eventLineCount(t, b); n != 2 {
		t.Fatalf("events.jsonl has %d rows, want 2", n)
	}
	var e map[string]any
	if err := json.Unmarshal(lastEventLine(t, b), &e); err != nil {
		t.Fatal(err)
	}
	if e["id"] != float64(2) {
		t.Fatalf("event id = %v, want 2 (MAX+1)", e["id"])
	}
	if e["event_type"] != "task_created" || e["task_id"] != "NEW-1" {
		t.Fatalf("event wrong: %s", lastEventLine(t, b))
	}
	// detail is a JSON-encoded string carrying the created id + status
	detail, ok := e["detail"].(string)
	if !ok {
		t.Fatalf("detail not a JSON string: %s", lastEventLine(t, b))
	}
	var d map[string]any
	if err := json.Unmarshal([]byte(detail), &d); err != nil {
		t.Fatalf("detail %q is not decodable JSON: %v", detail, err)
	}
	if d["id"] != "NEW-1" || d["status"] != "pending" {
		t.Fatalf("detail = %s, want {id,status}", detail)
	}
}

// BT-011: update to the write form "complete" must append a task_completed
// event (even when the row already read complete — the flag is the trigger).
func TestUpdateToCompleteWritesCompletionEvent(t *testing.T) {
	b := newTestBoard(t)
	status := "complete"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Status: &status}); err != nil {
		t.Fatal(err)
	}
	var e map[string]any
	if err := json.Unmarshal(lastEventLine(t, b), &e); err != nil {
		t.Fatal(err)
	}
	if e["id"] != float64(2) {
		t.Fatalf("event id = %v, want 2 (MAX+1)", e["id"])
	}
	if e["event_type"] != "task_completed" || e["task_id"] != "EXIST-1" {
		t.Fatalf("event wrong: %s", lastEventLine(t, b))
	}
}

// BT-011: update to any other valid vocabulary status must append a
// task_updated event.
func TestUpdateNonCompleteStatusWritesUpdatedEvent(t *testing.T) {
	b := newTestBoard(t)
	status := "in_progress"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Status: &status}); err != nil {
		t.Fatal(err)
	}
	var e map[string]any
	if err := json.Unmarshal(lastEventLine(t, b), &e); err != nil {
		t.Fatal(err)
	}
	if e["event_type"] != "task_updated" || e["task_id"] != "EXIST-1" {
		t.Fatalf("event wrong: %s", lastEventLine(t, b))
	}
}

// BT-011: update with --commit-hash must bump the board header's last_commit
// (the README-promised "header bump") without appending an extra event.
func TestUpdateCommitHashBumpsHeader(t *testing.T) {
	b := newTestBoard(t)
	commit := "deadbeef"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{CommitHash: &commit}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(b.headerPath)
	if err != nil {
		t.Fatal(err)
	}
	var hdr map[string]any
	if err := json.Unmarshal(raw, &hdr); err != nil {
		t.Fatal(err)
	}
	if hdr["last_commit"] != commit {
		t.Fatalf("last_commit = %v, want %s", hdr["last_commit"], commit)
	}
	// commit-hash-only update: row audit, not a trail event
	if n := eventLineCount(t, b); n != 1 {
		t.Fatalf("events.jsonl has %d rows, want 1 (no event for commit-hash-only update)", n)
	}
}

// ---------- BT-010: topology-B boards are writable ----------

// newTestBoardB builds a minimal topology-B board: the header is line 1 of
// tasks.jsonl and task rows follow it; board.jsonl does not exist.
func newTestBoardB(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"project":"legacy","namespace":"legacy","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":"abc1234"}` + "\n" +
			`{"id":"EXIST-1","title":"Existing","status":"complete","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Topology != "B" {
		t.Fatalf("topology = %q, want B", b.Topology)
	}
	return b
}

// TestBoardBHeaderRowReadsLine1: HeaderRow on topology B parses line 1 of
// tasks.jsonl (previously it returned nil for B).
func TestBoardBHeaderRowReadsLine1(t *testing.T) {
	b := newTestBoardB(t)
	hdr, err := b.HeaderRow()
	if err != nil {
		t.Fatal(err)
	}
	if hdr.String("project") != "legacy" || hdr.String("namespace") != "legacy" {
		t.Fatalf("header identity wrong: %s", RowJSONCompact(hdr))
	}
	if n, ok := hdr.Int("ticks_total"); !ok || n != 1 {
		t.Fatalf("ticks_total = %v, want 1", hdr.Get("ticks_total"))
	}
}

// TestCreateOnTopologyBAppendsAfterHeader: create on a topology-B board
// appends a task row mirroring the LAST TASK row's schema (NOT line 1's
// header keys), and line 1 stays byte-identical.
func TestCreateOnTopologyBAppendsAfterHeader(t *testing.T) {
	b := newTestBoardB(t)
	before, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	linesBefore := strings.SplitN(strings.TrimRight(string(before), "\n"), "\n", 2)
	if _, err := b.Create(TaskRowSpec{ID: "NEW-1", Title: "New task", Status: "pending", Priority: "P2"}); err != nil {
		t.Fatalf("create on topology B: %v", err)
	}
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitN(strings.TrimRight(string(raw), "\n"), "\n", 2)
	// line 1 (the header) untouched byte-for-byte
	if lines[0] != linesBefore[0] {
		t.Fatalf("topology-B header line mutated by create:\n got %s\nwant %s", lines[0], linesBefore[0])
	}
	if len(lines) != 2 || !strings.Contains(lines[1], `"NEW-1"`) {
		t.Fatalf("appended rows wrong: %q", lines)
	}
	// the appended row mirrors the LAST TASK row's key set (EXIST-1's), not
	// the header's {project,namespace,version,...} key set. (Create's
	// guarantee-core-schema step may ADD keys the mirrored row lacks —
	// complexity/created_at/updated_at — which is the topology-A contract
	// too; it must never DROP or swap the mirrored set.)
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("task rows = %d, want 2 (header excluded)", len(rows))
	}
	if len(rows[1].Keys) < len(rows[0].Keys) {
		t.Fatalf("new row schema smaller than the mirrored task row: %d keys vs %d\n%s",
			len(rows[1].Keys), len(rows[0].Keys), RowJSONCompact(rows[1]))
	}
	for _, k := range rows[0].Keys {
		if !rows[1].Has(k) {
			t.Fatalf("new row missing mirrored key %q: %s", k, RowJSONCompact(rows[1]))
		}
	}
	if rows[1].Has("project") || rows[1].Has("namespace") {
		t.Fatalf("new row inherited header keys: %s", RowJSONCompact(rows[1]))
	}
}

// TestCreateDuplicateOnTopologyBRejected: the dup scan must ignore the header
// row and still catch a duplicate task id.
func TestCreateDuplicateOnTopologyBRejected(t *testing.T) {
	b := newTestBoardB(t)
	_, err := b.Create(TaskRowSpec{ID: "EXIST-1", Title: "dup", Status: "pending", Priority: "P1"})
	if err == nil {
		t.Fatal("duplicate id accepted on topology B")
	}
	if _, ok := err.(*ErrDuplicateTaskID); !ok {
		t.Fatalf("error type = %T, want *ErrDuplicateTaskID", err)
	}
}

// TestUpdateTaskOnTopologyB: a status flip rewrites ONLY the target task
// row — line 1 (header) and the other task lines stay byte-identical.
func TestUpdateTaskOnTopologyB(t *testing.T) {
	b := newTestBoardB(t)
	before, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	linesBefore := strings.SplitN(strings.TrimRight(string(before), "\n"), "\n", 2)
	status := "in_progress"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Status: &status}); err != nil {
		t.Fatalf("update on topology B: %v", err)
	}
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitN(strings.TrimRight(string(raw), "\n"), "\n", 2)
	if lines[0] != linesBefore[0] {
		t.Fatalf("topology-B header line mutated by update:\n got %s\nwant %s", lines[0], linesBefore[0])
	}
	if !strings.Contains(lines[1], `"status":"in_progress"`) {
		t.Fatalf("status not flipped: %s", lines[1])
	}
}

// TestUpdateCommitHashBumpsLine1HeaderOnTopologyB: --commit-hash bumps
// last_commit IN LINE 1 of tasks.jsonl (via SetHeader) while the task rows
// keep their bytes (the target row's commit_hash aside).
func TestUpdateCommitHashBumpsLine1HeaderOnTopologyB(t *testing.T) {
	b := newTestBoardB(t)
	commit := "f00df00"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{CommitHash: &commit}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitN(strings.TrimRight(string(raw), "\n"), "\n", 2)
	var hdr map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &hdr); err != nil {
		t.Fatalf("line 1 no longer parses: %v (%s)", err, lines[0])
	}
	if hdr["last_commit"] != commit {
		t.Fatalf("line-1 last_commit = %v, want %s", hdr["last_commit"], commit)
	}
	if hdr["project"] != "legacy" {
		t.Fatalf("line-1 header identity lost: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"commit_hash":"f00df00"`) {
		t.Fatalf("task row missing commit_hash: %s", lines[1])
	}
}

// TestSetHeaderOnTopologyB: a direct SetHeader rewrites ONLY line 1 of
// tasks.jsonl; the task rows round-trip byte-identical (the same guarantee
// topology A has for board.jsonl).
func TestSetHeaderOnTopologyB(t *testing.T) {
	b := newTestBoardB(t)
	before, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	linesBefore := strings.SplitN(strings.TrimRight(string(before), "\n"), "\n", 2)
	tt := int64(42)
	if _, err := b.SetHeader(HeaderUpdate{TicksTotal: &tt}); err != nil {
		t.Fatalf("SetHeader on topology B: %v", err)
	}
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitN(strings.TrimRight(string(raw), "\n"), "\n", 2)
	var hdr map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &hdr); err != nil {
		t.Fatalf("line 1 no longer parses: %v (%s)", err, lines[0])
	}
	if hdr["ticks_total"] != float64(42) || hdr["project"] != "legacy" {
		t.Fatalf("line-1 header wrong: %s", lines[0])
	}
	if lines[1] != linesBefore[1] {
		t.Fatalf("task row mutated by SetHeader:\n got %s\nwant %s", lines[1], linesBefore[1])
	}
}

// TestSetHeaderRejectsNegativeCountersOnTopologyB: the negative-counter guard
// holds on topology B too — and nothing is written.
func TestSetHeaderRejectsNegativeCountersOnTopologyB(t *testing.T) {
	b := newTestBoardB(t)
	before, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	neg := int64(-5)
	if _, err := b.SetHeader(HeaderUpdate{TicksTotal: &neg}); err == nil {
		t.Fatal("negative ticks_total accepted on topology B")
	}
	after, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("rejected header update mutated tasks.jsonl:\nbefore %s\nafter  %s", before, after)
	}
}

// TestAppendEventOnTopologyB: events.jsonl has no header in any topology, so
// appends work with plain MAX(id)+1 sequencing.
func TestAppendEventOnTopologyB(t *testing.T) {
	b := newTestBoardB(t)
	id, err := b.AppendEvent(EventSpec{Type: "audit", Actor: "foreman", DetailText: strPtr(`"tick 2 summary"`), Tick: i64Ptr(2)})
	if err != nil {
		t.Fatalf("AppendEvent on topology B: %v", err)
	}
	if id != 2 {
		t.Fatalf("event id = %d, want 2 (MAX+1)", id)
	}
	var e map[string]any
	if err := json.Unmarshal(lastEventLine(t, b), &e); err != nil {
		t.Fatal(err)
	}
	if e["id"] != float64(2) || e["tick_number"] != float64(2) {
		t.Fatalf("event wrong: %s", lastEventLine(t, b))
	}
}

// TestStatsAndListSkipHeaderOnTopologyB: the read side must not count the
// line-1 header as a task.
func TestStatsAndListSkipHeaderOnTopologyB(t *testing.T) {
	b := newTestBoardB(t)
	st, err := b.ComputeStats(TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 1 {
		t.Fatalf("stats total = %d, want 1 (header row must not be counted)", st.Total)
	}
	tasks, err := b.ListTasks(TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].String("id") != "EXIST-1" {
		t.Fatalf("list = %v, want only EXIST-1", len(tasks))
	}
	row, file, err := b.ShowTask("EXIST-1")
	if err != nil || row == nil || file != b.tasksPath {
		t.Fatalf("ShowTask on topology B failed: %v %v %v", row != nil, file, err)
	}
}
