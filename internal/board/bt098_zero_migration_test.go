package board

// BT-098 §10 — the zero-migration control, in the two shapes panel amendment
// 1 requires.
//
// The amendment replaced the spec's original AC-9 (one SHA-256 digest of four
// board files, unchanged across a full command battery) because that control
// is unpassable by construction: a WORKING `create` must append a row and emit
// `task_created`, so the digest MUST move. What can be pinned instead:
//
//   (a) READ-ONLY battery — `types` + `validate` over a PINNED testdata board
//       snapshot, asserting byte-identity before/after. This is the
//       "reading a board never writes to it" control.
//   (b) DIFF battery — `create`/`update` change EXACTLY the expected lines
//       (one row + one event), with every other line byte-identical. This is
//       the "no serialization churn" control: the writer must not re-render
//       neighbours, reorder keys, or reformat the file.
//
// The snapshot is a CHECKED-IN fixture, never a copy of the live board: sibling
// lanes write the live board daily, so a runtime copy would move the baseline
// under the test.
//
// AC-1's golden-bytes half lives here too (TestBT098_ZeroMigration_Create).
// Its expected line was captured from the PRE-BT-098 writer (commit 0985a3f)
// running the identical command, then pinned with the two timestamps masked —
// so this test fails the moment built-in defaults start influencing the
// written bytes.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// bt098Pinned is the SHA-256 of every file in testdata/bt098. Pinned here (not
// computed from the live board) so a fixture edit is a loud, named failure
// instead of a silently relocating baseline.
var bt098Pinned = map[string]string{
	"tasks.jsonl":  "35251b0f659b729c462ccee6cd2c89604558cf62d70312898bf7cfccf5737925",
	"events.jsonl": "c42c835eec88b3ff5963d9fb2eb0cd0b85ed4580190f63de7648b38c77acf784",
	"board.jsonl":  "44749ebae4a36b7aa4733e104b22434c3fa1e7b18a311dfecb06126a66be1f4c",
	"types.jsonl":  "d18a84ad04158bd40b73a5d4eb2bef99dfb511b754b4aca6d6a5ab329761ee51",
}

// bt098CopyFixture copies the pinned snapshot into a temp dir and asserts the
// copy's digests still match the pins — a corrupted copy can never be graded
// as the frozen baseline.
func bt098CopyFixture(t *testing.T) string {
	t.Helper()
	src := filepath.Join("testdata", "bt098")
	dst := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read pinned fixture dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(src, entry.Name()))
		if err != nil {
			t.Fatalf("read fixture %s: %v", entry.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dst, entry.Name()), raw, 0o644); err != nil {
			t.Fatalf("write fixture copy %s: %v", entry.Name(), err)
		}
	}
	bt098AssertPinned(t, dst)
	return dst
}

// bt098AssertPinned fails, naming the file, when the snapshot has drifted from
// the pinned digests.
func bt098AssertPinned(t *testing.T, dir string) map[string]string {
	t.Helper()
	got := map[string]string{}
	for name, want := range bt098Pinned {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("pinned fixture missing %s: %v", name, err)
		}
		sum := sha256.Sum256(raw)
		got[name] = hex.EncodeToString(sum[:])
		if got[name] != want {
			t.Fatalf("pinned fixture %s drifted: sha256 %s, want %s — update the fixture AND every golden assertion together", name, got[name], want)
		}
	}
	return got
}

// TestBT098_ZeroMigration_ReadOnlyBattery is AC-9(a): the read-only commands
// (`types`, `validate`) leave all four board files byte-identical. The rendered
// reports are pinned too, so "byte-identical because nothing was measured" is
// also impossible.
func TestBT098_ZeroMigration_ReadOnlyBattery(t *testing.T) {
	dir := bt098CopyFixture(t)
	b, err := Resolve(dir)
	if err != nil {
		t.Fatalf("resolve pinned board: %v", err)
	}
	before := bt098AssertPinned(t, dir)

	reg, err := LoadTicketTypes(b.Dir)
	if err != nil {
		t.Fatalf("types: %v", err)
	}
	const wantCensus = `bug (Bug)
release (Release) template: prefix="[RELEASE] " priority=P1 skeleton metadata: channel(enum, required: stable|beta|nightly), signoffs(list), source_commit(string, required), approval_ticket(ref)
`
	if got := reg.RenderText(); got != wantCensus {
		t.Fatalf("types census drifted:\n--- got\n%s\n--- want\n%s", got, wantCensus)
	}

	rep, err := b.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	const wantReport = "board: %s (topology A)\n" +
		"rows: 3 tasks, 2 events, 0 fixtures, header parsed\n" +
		"key uniformity: 3/3 rows carry only canon keys (0 drift)\n" +
		"RESULT: OK (0 warning(s))\n"
	if got, want := rep.RenderText(), strings.Replace(wantReport, "%s", b.Dir, 1); got != want {
		t.Fatalf("validate report drifted:\n--- got\n%s\n--- want\n%s", got, want)
	}
	if rep.HasErrors() {
		t.Fatalf("pinned fixture must validate clean:\n%s", rep.RenderText())
	}

	// (a) the control itself
	if after := bt098AssertPinned(t, dir); !bt098EqualDigests(before, after) {
		t.Fatalf("read-only battery changed the board: before=%v after=%v", before, after)
	}
}

// TestBT098_ZeroMigration_DiffIsMinimal is AC-9(b): a create appends exactly
// one row and one event; a status update rewrites exactly one row and appends
// exactly one event. Every other line is byte-identical — no serialization
// churn anywhere on the board.
func TestBT098_ZeroMigration_DiffIsMinimal(t *testing.T) {
	dir := bt098CopyFixture(t)
	b, err := Resolve(dir)
	if err != nil {
		t.Fatalf("resolve pinned board: %v", err)
	}
	tasksBefore := bt098Lines(bt098File(t, b.TasksPath()))
	eventsBefore := bt098Lines(bt098File(t, b.EventsPath()))

	if _, err := b.Create(TaskRowSpec{
		ID:       "REL-101",
		Title:    "next release",
		Type:     "release",
		HasType:  true,
		Metadata: map[string]string{"channel": "beta", "source_commit": "deadbee"},
		Status:   "pending",
	}); err != nil {
		t.Fatalf("create on pinned board: %v", err)
	}

	tasksAfter := bt098Lines(bt098File(t, b.TasksPath()))
	eventsAfter := bt098Lines(bt098File(t, b.EventsPath()))
	if len(tasksAfter) != len(tasksBefore)+1 {
		t.Fatalf("create changed tasks.jsonl line count %d -> %d, want +1 (one appended row)", len(tasksBefore), len(tasksAfter))
	}
	for i, line := range tasksBefore {
		if tasksAfter[i] != line {
			t.Fatalf("create rewrote existing tasks.jsonl line %d:\n--- before\n%s\n--- after\n%s", i+1, line, tasksAfter[i])
		}
	}
	if len(eventsAfter) != len(eventsBefore)+1 {
		t.Fatalf("create changed events.jsonl line count %d -> %d, want +1 (one task_created)", len(eventsBefore), len(eventsAfter))
	}
	for i, line := range eventsBefore {
		if eventsAfter[i] != line {
			t.Fatalf("create rewrote existing events.jsonl line %d:\n--- before\n%s\n--- after\n%s", i+1, line, eventsAfter[i])
		}
	}
	if !strings.Contains(eventsAfter[len(eventsAfter)-1], `"task_created"`) {
		t.Fatalf("the appended event is not task_created: %s", eventsAfter[len(eventsAfter)-1])
	}

	// Update: exactly ONE row moves (the target), everything else is frozen.
	tasksPreUpdate := bt098Lines(bt098File(t, b.TasksPath()))
	eventsPreUpdate := bt098Lines(bt098File(t, b.EventsPath()))
	status := "in_progress"
	if _, err := b.UpdateTask("REL-100", UpdateSpec{Status: &status}); err != nil {
		t.Fatalf("status update on pinned board: %v", err)
	}
	tasksPostUpdate := bt098Lines(bt098File(t, b.TasksPath()))
	eventsPostUpdate := bt098Lines(bt098File(t, b.EventsPath()))
	if len(tasksPostUpdate) != len(tasksPreUpdate) {
		t.Fatalf("update changed tasks.jsonl line count %d -> %d, want identical", len(tasksPreUpdate), len(tasksPostUpdate))
	}
	moved := 0
	for i, line := range tasksPreUpdate {
		if tasksPostUpdate[i] != line {
			moved++
			if !strings.Contains(tasksPostUpdate[i], `"id":"REL-100"`) {
				t.Fatalf("update rewrote a non-target line %d: %s", i+1, tasksPostUpdate[i])
			}
		}
	}
	if moved != 1 {
		t.Fatalf("update rewrote %d lines, want exactly 1 (the target row)", moved)
	}
	if len(eventsPostUpdate) != len(eventsPreUpdate)+1 {
		t.Fatalf("update appended %d events, want exactly 1", len(eventsPostUpdate)-len(eventsPreUpdate))
	}
	for i, line := range eventsPreUpdate {
		if eventsPostUpdate[i] != line {
			t.Fatalf("update rewrote existing events.jsonl line %d", i+1)
		}
	}
	if !strings.Contains(eventsPostUpdate[len(eventsPostUpdate)-1], `"task_updated"`) {
		t.Fatalf("the appended event is not task_updated: %s", eventsPostUpdate[len(eventsPostUpdate)-1])
	}
}

// bt098ZeroMigrationGolden is the row the PRE-BT-098 writer (0985a3f) emitted
// for `create --id PLAIN-1 --title "plain row" --status pending --priority P2`
// on a board with NO types.jsonl, with created_at/updated_at masked. It is the
// AC-1 golden: built-in defaults must not influence one byte of it.
const bt098ZeroMigrationGolden = `{"id":"PLAIN-1","title":"plain row","status":"pending","priority":"P2","complexity":3,"created_at":"<TS>","updated_at":"<TS>","detail":{"fingerprint":"7b3c755ac36f3bd45b3a0f57424803dd03c6c518b6516755c039f382ada37a8b"}}`

var bt098TimestampMask = regexp.MustCompile(`"(created|updated)_at":"[^"]*"`)

// TestBT098_ZeroMigration_Create is AC-1: on a board with no types.jsonl, a
// create WITHOUT --type writes the pre-BT-098 row bytes exactly — no `type`
// key, no `detail.metadata`, unchanged key order.
func TestBT098_ZeroMigration_Create(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"LEG-1","title":"legacy row","status":"pending","priority":"P2"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-11-01 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"bt098","namespace":"bt098","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatalf("resolve board: %v", err)
	}

	if _, err := b.Create(TaskRowSpec{ID: "PLAIN-1", Title: "plain row", Status: "pending", Priority: "P2"}); err != nil {
		t.Fatalf("untyped create: %v", err)
	}
	row := bt098LastRow(t, b)
	masked := bt098TimestampMask.ReplaceAllString(row, `"${1}_at":"<TS>"`)
	if masked != bt098ZeroMigrationGolden {
		t.Fatalf("untyped create row drifted from the pre-BT-098 writer:\n--- got\n%s\n--- want\n%s", masked, bt098ZeroMigrationGolden)
	}
	if strings.Contains(row, `"type":`) {
		t.Fatalf("an untyped create stamped a type key:\n%s", row)
	}
	if strings.Contains(row, `"metadata"`) {
		t.Fatalf("an untyped create stamped detail.metadata:\n%s", row)
	}

	// The paired event keeps its historical shape too.
	events := bt098Lines(bt098File(t, b.EventsPath()))
	if len(events) != 2 {
		t.Fatalf("events.jsonl rows = %d, want 2 (seed + task_created)", len(events))
	}
	created := events[1]
	if !strings.HasPrefix(created, `{"id":2,`) || !strings.Contains(created, `"event_type":"task_created"`) {
		t.Fatalf("task_created event shape drifted:\n%s", created)
	}
}

// bt098Lines splits file content into lines, dropping only the trailing empty
// element produced by the final newline (blank INTERIOR lines are preserved so
// a churn check cannot hide behind them).
func bt098Lines(content string) []string {
	trimmed := strings.TrimSuffix(content, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func bt098EqualDigests(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
