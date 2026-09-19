package board

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BT-037: header operations on a HEADERLESS board.
//
// MEASURED (2026-09-19) on the real coding-hermes-tools board: a board dir
// holding tasks.jsonl + events.jsonl and NO board.jsonl, whose line 1 is an
// ordinary task row, was classified as writable topology B. `header
// --set-ticks-total 90` then rewrote line 1 — task CHT-001 grew from 9 keys to
// 10 and lost its updated_at — and `validate` read that task row as the header
// and invented three "not an integer counter" errors.
//
// The fix classifies such boards as HEADERLESS (topology stays "B", the board
// just has no header): reads keep working, header reads/writes refuse with
// ErrNoHeader, validate/doctor skip the header checks with one note.

// headerlessTask1 / headerlessTask2 are ordinary task rows; line 1 is a TASK
// row (9 keys), which is what the topology-B detection used to mistake for a
// header.
const (
	headerlessTask1 = `{"id":"CHT-001","title":"Bootstrap","status":"complete","priority":"P1","created_at":"2026-09-17T01:20:00-05:00","reasoning":"First row.","updated_at":"2026-09-17T02:49:50-05:00","completed_at":"2026-09-17T02:49:50-05:00","attributes":{"tick":"t1"}}`
	headerlessTask2 = `{"id":"CHT-002","title":"Diff3","status":"pending","priority":"P2","updated_at":"2026-09-17T03:00:00-05:00"}`
)

// headerlessBoardFiles is the measured shape: tasks.jsonl (task rows only, no
// metadata row), events.jsonl, and no board.jsonl. The newest event tick (50)
// is deliberately far above any counter so a header-drift check WOULD fire if
// the board had a header to compare against.
func headerlessBoardFiles() map[string]string {
	return map[string]string{
		"tasks.jsonl": headerlessTask1 + "\n" + headerlessTask2 + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-17 01:20:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n" +
			`{"id":2,"timestamp":"2026-09-17 05:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":50}` + "\n",
	}
}

func newTestHeaderlessBoard(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, headerlessBoardFiles())
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Topology != "B" {
		t.Fatalf("topology = %q, want B (no board.jsonl)", b.Topology)
	}
	if b.HasHeader() {
		t.Fatal("board with no board.jsonl and a task row on line 1 was classified as having a header")
	}
	return b
}

func fileSHA(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func findingsText(rep *Report) string {
	var sb strings.Builder
	for _, f := range rep.Findings {
		sb.WriteString("[" + f.Level + "] " + f.Msg + "\n")
	}
	return sb.String()
}

// TestHeaderlessReadsKeepWorking: headerless is a HEADER property, not a read
// restriction — every task row, including line 1, is enumerated.
func TestHeaderlessReadsKeepWorking(t *testing.T) {
	b := newTestHeaderlessBoard(t)
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("TaskRows = %d rows, want 2 (line 1 is a task, not a header)", len(rows))
	}
	if rows[0].String("id") != "CHT-001" {
		t.Fatalf("first task row id = %q, want CHT-001", rows[0].String("id"))
	}
	row, file, err := b.ShowTask("CHT-001")
	if err != nil {
		t.Fatal(err)
	}
	if row == nil || file != b.TasksPath() {
		t.Fatalf("ShowTask(CHT-001) = %v, %q; want the task row from tasks.jsonl", row, file)
	}
	// LaneName (backfill) reads the header opportunistically — on a headerless
	// board it must fall through to the directory name, not error out.
	if got := b.LaneName(); got != filepath.Base(filepath.Dir(b.Dir)) {
		t.Fatalf("LaneName = %q, want the repo dir name %q", got, filepath.Base(filepath.Dir(b.Dir)))
	}
}

// TestHeaderlessHeaderRowRefuses: there is no header to read, so HeaderRow
// must say so — never hand back a task row as if it were the header.
func TestHeaderlessHeaderRowRefuses(t *testing.T) {
	b := newTestHeaderlessBoard(t)
	row, err := b.HeaderRow()
	if err == nil {
		t.Fatalf("HeaderRow on a headerless board returned %s, want an error", RowJSONCompact(row))
	}
	if !errors.Is(err, ErrNoHeader) {
		t.Fatalf("HeaderRow error %v does not wrap ErrNoHeader", err)
	}
	if !strings.Contains(err.Error(), "no header on this board") {
		t.Fatalf("HeaderRow error %q does not say 'no header on this board'", err)
	}
	if !strings.Contains(err.Error(), "CHT-001") {
		t.Fatalf("HeaderRow error %q does not name the task row it refused to use", err)
	}
}

// TestHeaderlessSetHeaderRefusesByteIdentical is acceptance criterion 1 at the
// package level: every --set flag is refused and tasks.jsonl keeps its exact
// bytes (sha256) and its 9-key first row.
func TestHeaderlessSetHeaderRefusesByteIdentical(t *testing.T) {
	b := newTestHeaderlessBoard(t)
	before := fileSHA(t, b.TasksPath())
	eventsBefore := fileSHA(t, b.EventsPath())

	ninety := int64(90)
	seven := int64(7)
	sha := "abc1234"
	cases := map[string]HeaderUpdate{
		"--set-ticks-total":  {TicksTotal: &ninety},
		"--set-ticks-idle":   {TicksIdle: &seven},
		"--set-last-commit":  {LastCommit: &sha},
		"all three together": {TicksTotal: &ninety, TicksIdle: &seven, LastCommit: &sha},
	}
	for name, u := range cases {
		changed, err := b.SetHeader(u)
		if err == nil {
			t.Fatalf("%s on a headerless board succeeded (changed %v), want a refusal", name, changed)
		}
		if !errors.Is(err, ErrNoHeader) {
			t.Fatalf("%s error %v does not wrap ErrNoHeader", name, err)
		}
		if !strings.Contains(err.Error(), "no header on this board") {
			t.Fatalf("%s error %q does not say 'no header on this board'", name, err)
		}
	}
	if after := fileSHA(t, b.TasksPath()); after != before {
		t.Fatalf("tasks.jsonl changed under a refused header write:\nbefore %s\nafter  %s", before, after)
	}
	if after := fileSHA(t, b.EventsPath()); after != eventsBefore {
		t.Fatalf("events.jsonl changed under a refused header write: %s -> %s", eventsBefore, after)
	}
	// The corruption this row is about: the FIRST TASK ROW must not grow a
	// header key. (Pre-fix it went 9 keys -> 10 with ticks_total.)
	line, ok := firstNonBlankLine(b.TasksPath())
	if !ok {
		t.Fatal("tasks.jsonl has no line 1")
	}
	row, err := ParseRow(line)
	if err != nil {
		t.Fatal(err)
	}
	if row.Has("ticks_total") || row.Has("ticks_idle") {
		t.Fatalf("header counters landed in a task row: %s", line)
	}
	if got := len(row.Keys); got != 9 {
		t.Fatalf("first task row has %d keys, want 9 (unchanged): %s", got, line)
	}
	if row.String("id") != "CHT-001" || row.String("status") != "complete" {
		t.Fatalf("first task row identity changed: %s", line)
	}
}

// TestHeaderlessSetHeaderGuardFiresOnTaskRow is the HARD GUARD (acceptance
// criterion 5's RED case): even with the headerless classification defeated,
// SetHeader must refuse to stamp header keys into a row carrying
// id/title/status. Pre-fix this call succeeded and rewrote the task row.
func TestHeaderlessSetHeaderGuardFiresOnTaskRow(t *testing.T) {
	b := newTestHeaderlessBoard(t)
	before := fileSHA(t, b.TasksPath())

	// Defeat layer 1 (the classification) to exercise layer 2 (the shape
	// guard) on its own: the board is now "topology B with a header".
	b.headerless = false
	if !b.HasHeader() {
		t.Fatal("test premise broken: board should report a header once headerless is cleared")
	}
	ninety := int64(90)
	_, err := b.SetHeader(HeaderUpdate{TicksTotal: &ninety})
	if err == nil {
		t.Fatal("SetHeader stamped a header counter into a task row — the task-shape guard did not fire")
	}
	if !errors.Is(err, ErrNoHeader) {
		t.Fatalf("guard error %v does not wrap ErrNoHeader", err)
	}
	if !strings.Contains(err.Error(), "id/title/status") || !strings.Contains(err.Error(), "CHT-001") {
		t.Fatalf("guard error %q does not name the task row / the keys it refuses to write", err)
	}
	if after := fileSHA(t, b.TasksPath()); after != before {
		t.Fatalf("tasks.jsonl rewritten despite the guard:\nbefore %s\nafter  %s", before, after)
	}
}

// TestHeaderlessSetHeaderGuardFiresOnTopologyATaskRow: the same guard covers
// the other file — a board.jsonl whose line 1 is accidentally a task row must
// not be rewritten as a header either.
func TestHeaderlessSetHeaderGuardFiresOnTopologyATaskRow(t *testing.T) {
	dir := t.TempDir()
	files := headerlessBoardFiles()
	files["board.jsonl"] = `{"id":"BOGUS-1","title":"not a header","status":"pending","ticks_total":1}` + "\n"
	writeBoardFiles(t, dir, files)
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Topology != "A" || !b.HasHeader() {
		t.Fatalf("topology = %q HasHeader = %v, want A/true", b.Topology, b.HasHeader())
	}
	before := fileSHA(t, b.HeaderPath())
	seven := int64(7)
	if _, err := b.SetHeader(HeaderUpdate{TicksTotal: &seven}); err == nil {
		t.Fatal("SetHeader rewrote a task row in board.jsonl as the header")
	} else if !errors.Is(err, ErrNoHeader) {
		t.Fatalf("guard error %v does not wrap ErrNoHeader", err)
	}
	if after := fileSHA(t, b.HeaderPath()); after != before {
		t.Fatalf("board.jsonl rewritten despite the guard: %s -> %s", before, after)
	}
}

// TestHeaderlessValidateInformational is acceptance criterion 2: one
// informational headerless line, NONE of the three false
// 'not an integer counter' errors, zero errors overall.
func TestHeaderlessValidateInformational(t *testing.T) {
	b := newTestHeaderlessBoard(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	text := findingsText(rep)
	if !strings.Contains(text, "headerless board (no board.jsonl)") {
		t.Fatalf("validate did not report the headerless board:\n%s", text)
	}
	if strings.Contains(text, "not an integer counter") {
		t.Fatalf("validate validated a task row as a header:\n%s", text)
	}
	if strings.Contains(text, "header:") {
		t.Fatalf("validate emitted header findings for a board with no header:\n%s", text)
	}
	if rep.HasErrors() {
		t.Fatalf("headerless board has %d error(s), want 0:\n%s", rep.Errors(), text)
	}
	if rep.Header {
		t.Fatal("report claims a header was parsed on a headerless board")
	}
	if rep.Tasks != 2 || rep.Events != 2 {
		t.Fatalf("report rows = %d tasks / %d events, want 2/2", rep.Tasks, rep.Events)
	}
	headerless := 0
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "headerless board") {
			headerless++
		}
	}
	if headerless != 1 {
		t.Fatalf("validate emitted %d headerless lines, want exactly 1:\n%s", headerless, text)
	}
}

// TestHeaderlessDoctorSkipsHeaderChecks: doctor's header-vs-events drift check
// cannot run without a header — it must skip with a note rather than invent an
// error (the remediation it would print, `header --set-ticks-total`, now
// refuses on this board).
func TestHeaderlessDoctorSkipsHeaderChecks(t *testing.T) {
	b := newTestHeaderlessBoard(t)
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	text := findingsText(rep)
	if rep.HasErrors() {
		t.Fatalf("doctor on a headerless board has %d error(s), want 0:\n%s", rep.Errors(), text)
	}
	if !strings.Contains(text, "header-vs-events tick drift checks skipped") {
		t.Fatalf("doctor did not note the skipped drift check:\n%s", text)
	}
	if strings.Contains(text, "ticks_total") {
		t.Fatalf("doctor compared header counters on a board that has none:\n%s", text)
	}
	if strings.Contains(text, "not an integer counter") {
		t.Fatalf("doctor inherited validate's false header errors:\n%s", text)
	}
}

// TestHeaderlessEventTickAppendsWithoutHeaderBump: `event --tick N` bumps the
// header counter on header-bearing boards; on a headerless board there is no
// counter, so the bump is a no-op and the event still lands — tasks.jsonl is
// never touched (pre-fix the bump wrote ticks_total into the first task row).
func TestHeaderlessEventTickAppendsWithoutHeaderBump(t *testing.T) {
	b := newTestHeaderlessBoard(t)
	tasksBefore := fileSHA(t, b.TasksPath())
	tick := int64(51)
	id, err := b.AppendEvent(EventSpec{Type: "tick", Actor: "foreman", Tick: &tick})
	if err != nil {
		t.Fatalf("AppendEvent with a tick on a headerless board: %v", err)
	}
	if id <= 2 {
		t.Fatalf("appended event id = %d, want > 2", id)
	}
	if after := fileSHA(t, b.TasksPath()); after != tasksBefore {
		t.Fatalf("tick event bumped a header into tasks.jsonl:\nbefore %s\nafter  %s", tasksBefore, after)
	}
	rows, _, err := ReadAllRows(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("events.jsonl has %d rows, want 3", len(rows))
	}
}

// TestHeaderlessEmptyTasksFileRefuses: an empty tasks.jsonl has no line 1 to
// be a header, so it is headerless too — a header write must refuse instead of
// creating a header row out of nothing.
func TestHeaderlessEmptyTasksFileRefuses(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"events.jsonl": `{"id":1,"timestamp":"2026-09-17 01:20:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
	})
	// writeBoardFiles skips empty content, so create the 0-byte file directly.
	if err := os.WriteFile(filepath.Join(dir, "tasks.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.HasHeader() {
		t.Fatal("empty tasks.jsonl classified as carrying a header")
	}
	ninety := int64(90)
	if _, err := b.SetHeader(HeaderUpdate{TicksTotal: &ninety}); !errors.Is(err, ErrNoHeader) {
		t.Fatalf("SetHeader on an empty tasks.jsonl error = %v, want ErrNoHeader", err)
	}
}

// TestUnparseableLineOneIsNotHeaderless: a break in line 1 is a PARSE failure
// and must surface as one (the pretty-printed/truncated-JSON diagnosis), not be
// masked by a shape verdict. The write path still refuses via the task-shape
// guard, but the reported error is the parser's.
func TestUnparseableLineOneIsNotHeaderless(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  headerlessTask1[:20] + "\n" + headerlessTask2 + "\n", // truncated line 1
		"events.jsonl": `{"id":1,"timestamp":"2026-09-17 01:20:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !b.HasHeader() {
		t.Fatal("unparseable line 1 was classified as headerless (masking the parse error)")
	}
	ninety := int64(90)
	err = func() error {
		_, e := b.SetHeader(HeaderUpdate{TicksTotal: &ninety})
		return e
	}()
	if err == nil {
		t.Fatal("SetHeader succeeded on a corrupt tasks.jsonl")
	}
	if errors.Is(err, ErrNoHeader) {
		t.Fatalf("prioritized the shape verdict over the parse error: %v", err)
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("parse error %q does not name the offending line", err)
	}
}
