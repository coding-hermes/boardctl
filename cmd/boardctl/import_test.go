package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedImportBoard writes a small topology-A board (same shape render_test.go
// seeds) and returns the repo root to pass via -C. All ids use the QA-T
// prefix so --renumber has a prefix-numeric sequence to continue.
func seedImportBoard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"board.jsonl": `{"project": "importtest", "namespace": "ns", "version": 1, "ticks_total": 5, "ticks_idle": 1, "last_commit": null}` + "\n",
		"tasks.jsonl": `{"id": "QA-T-1", "title": "first", "status": "complete", "priority": "P1", "created_at": "2026-09-01 00:00:00", "completed_at": "2026-09-02 00:00:00", "updated_at": "2026-09-02 00:00:00"}` + "\n" +
			`{"id": "QA-T-2", "title": "second", "status": "pending", "priority": "P2", "created_at": "2026-09-03 00:00:00", "updated_at": "2026-09-03 00:00:00"}` + "\n" +
			`{"id": "QA-X-9", "title": "extra done", "status": "complete", "priority": "P3", "created_at": "2026-09-02 00:00:00", "completed_at": "2026-09-03 00:00:00", "updated_at": "2026-09-03 00:00:00"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-02 00:00:00","event_type":"task_completed","task_id":"QA-T-1","actor":"foreman","detail":"{\"status\":\"complete\"}","tick_number":1}` + "\n" +
			`{"id":2,"timestamp":"2026-09-03 00:30:00","event_type":"idle","task_id":null,"actor":"foreman","detail":null,"tick_number":2}` + "\n",
		"fixtures.jsonl": `{"id": "QA-FIX", "status": "pending", "active": true}` + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(boardDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// importBoardDir returns the board dir of a seeded/copied repo root.
func importBoardDir(dir string) string {
	return filepath.Join(dir, ".coding-hermes", "board")
}

// mustRenderExport runs `boardctl render --json` against dir and returns the
// export path. This is the same contract the spec round-trip names.
func mustRenderExport(t *testing.T, dir string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "export.json")
	if code := run([]string{"-C", dir, "render", "--json", out, "-o", filepath.Join(t.TempDir(), "r.html")}); code != 0 {
		t.Fatalf("render --json exit code = %d, want 0", code)
	}
	return out
}

// boardFileHashes hashes the four board files (missing files are reported
// as absent so absence itself is assertable).
func boardFileHashes(t *testing.T, dir string) map[string]string {
	t.Helper()
	bd := importBoardDir(dir)
	out := map[string]string{}
	for _, name := range []string{"tasks.jsonl", "events.jsonl", "board.jsonl", "fixtures.jsonl"} {
		data, err := os.ReadFile(filepath.Join(bd, name))
		if err != nil {
			out[name] = "<absent>"
			continue
		}
		sum := sha256.Sum256(data)
		out[name] = fmt.Sprintf("%x", sum)
	}
	return out
}

// runImport runs the import subcommand through run() and returns the exit code.
func runImport(t *testing.T, dir, export string, extra ...string) int {
	t.Helper()
	args := append([]string{"-C", dir, "import", export}, extra...)
	return run(args)
}

// TestImportUnsupportedSchema: spec 7.3.7 — `{}` exits 1 with "unsupported
// schema" and writes nothing.
func TestImportUnsupportedSchema(t *testing.T) {
	dir := seedImportBoard(t)
	before := boardFileHashes(t, dir)
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := captureStderr(func() {
		if code := runImport(t, dir, bad); code != 1 {
			t.Fatalf("import({}) exit code = %d, want 1", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "unsupported schema") {
		t.Fatalf("stderr %q missing \"unsupported schema\"", got)
	}
	if after := boardFileHashes(t, dir); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("board files changed on rejected import:\nbefore %v\nafter  %v", before, after)
	}
}

// captureStderr mirrors captureStdout for os.Stderr.
func captureStderr(fn func()) (string, error) {
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	w.Close()
	return <-done, nil
}

// TestImportWrongBoardNames expected target: BT-022 board-row criterion.
func TestImportWrongBoardNames(t *testing.T) {
	dir := seedImportBoard(t)
	// Export rendered from ANOTHER board.
	other := seedImportBoard(t)
	obd := importBoardDir(other)
	if err := os.WriteFile(filepath.Join(obd, "board.jsonl"), []byte(`{"project": "otherproject", "namespace": "ns", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": null}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	otherExport := mustRenderExport(t, other)

	before := boardFileHashes(t, dir)
	got, err := captureStderr(func() {
		if code := runImport(t, dir, otherExport); code != 1 {
			t.Fatalf("import(wrong board) exit code = %d, want 1", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `board mismatch`) || !strings.Contains(got, "importtest") {
		t.Fatalf("stderr %q must name the mismatch and the expected board \"importtest\"", got)
	}
	if after := boardFileHashes(t, dir); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("board files changed on board-mismatch import:\nbefore %v\nafter  %v", before, after)
	}
}

// TestImportSameBoardNoop: spec 7.3.1 + 7.3.2 + 7.3.6 — render --json then
// import --dry-run prints the zero plan and writes nothing; the real run is
// byte-identical (clean).
func TestImportSameBoardNoop(t *testing.T) {
	dir := seedImportBoard(t)
	export := mustRenderExport(t, dir)
	before := boardFileHashes(t, dir)

	// dry-run: exit 0, zero plan, zero writes.
	out, err := captureStdout(func() {
		if code := runImport(t, dir, export, "--dry-run"); code != 0 {
			t.Fatalf("dry-run exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "0 new tasks, 0 updates, 0 events") {
		t.Fatalf("dry-run output missing zero plan line:\n%s", out)
	}
	if !strings.Contains(out, "dry-run: nothing written") {
		t.Fatalf("dry-run output missing nothing-written marker:\n%s", out)
	}
	if after := boardFileHashes(t, dir); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("dry-run changed board files:\nbefore %v\nafter  %v", before, after)
	}

	// real run: exit 0, board stays byte-identical.
	out2, err := captureStdout(func() {
		if code := runImport(t, dir, export); code != 0 {
			t.Fatalf("real import exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out2, "0 new tasks, 0 updates, 0 events") {
		t.Fatalf("real run output missing zero plan line:\n%s", out2)
	}
	if after := boardFileHashes(t, dir); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("no-op import changed board files:\nbefore %v\nafter  %v", before, after)
	}
}

// TestImportRestoresMissingTasks: spec 7.3.3 — a copy missing two completed
// tasks reports "2 new tasks", appends both in target style, passes
// validate, and a second import is a no-op.
func TestImportRestoresMissingTasks(t *testing.T) {
	src := seedImportBoard(t)
	export := mustRenderExport(t, src)

	// Copy the board and delete the two completed tasks.
	dst := t.TempDir()
	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}
	dbd := importBoardDir(dst)
	origTasks, err := os.ReadFile(filepath.Join(importBoardDir(src), "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, l := range strings.Split(strings.TrimRight(string(origTasks), "\n"), "\n") {
		var row map[string]any
		if json.Unmarshal([]byte(l), &row) == nil && row["status"] == "complete" {
			continue
		}
		kept = append(kept, l)
	}
	if len(kept) != 1 {
		t.Fatalf("expected 1 surviving row after deleting completed tasks, got %d", len(kept))
	}
	if err := os.WriteFile(filepath.Join(dbd, "tasks.jsonl"), []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// dry-run says 2 new tasks and writes nothing.
	before := boardFileHashes(t, dst)
	out, err := captureStdout(func() {
		if code := runImport(t, dst, export, "--dry-run"); code != 0 {
			t.Fatalf("dry-run exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2 new tasks") {
		t.Fatalf("dry-run output missing \"2 new tasks\":\n%s", out)
	}
	if after := boardFileHashes(t, dst); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("dry-run wrote to the board:\nbefore %v\nafter  %v", before, after)
	}

	// Real run appends both rows; validate passes.
	if _, err := captureStdout(func() {
		if code := runImport(t, dst, export); code != 0 {
			t.Fatalf("real import exit = %d, want 0", code)
		}
	}); err != nil {
		t.Fatal(err)
	}
	rows := readJSONLLines(t, filepath.Join(dbd, "tasks.jsonl"))
	if len(rows) != 3 {
		t.Fatalf("tasks.jsonl has %d rows, want 3", len(rows))
	}
	for _, id := range []string{"QA-T-1", "QA-T-2"} {
		if !importBoardHasID(t, filepath.Join(dbd, "tasks.jsonl"), id) {
			t.Fatalf("task %s not restored", id)
		}
	}
	if code := run([]string{"-C", dst, "validate"}); code != 0 {
		t.Fatalf("validate after import exit = %d, want 0", code)
	}

	// Restored rows carry the provenance marker.
	restored := importRowByID(t, filepath.Join(dbd, "tasks.jsonl"), "QA-T-1")
	if reason, _ := restored["reasoning"].(string); !strings.Contains(reason, "boardctl-web-export") {
		t.Fatalf("restored row reasoning %q missing boardctl-web-export provenance", reason)
	}

	// Second import: true no-op, byte-identical.
	mid := boardFileHashes(t, dst)
	out2, err := captureStdout(func() {
		if code := runImport(t, dst, export); code != 0 {
			t.Fatalf("second import exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out2, "0 new tasks, 0 updates, 0 events") {
		t.Fatalf("second import not a no-op:\n%s", out2)
	}
	if after := boardFileHashes(t, dst); fmt.Sprint(after) != fmt.Sprint(mid) {
		t.Fatalf("second import changed board files:\nbefore %v\nafter  %v", mid, after)
	}
}

// TestImportEventsFreshSequencing: spec 7.3.4 — absent export events append
// with fresh MAX(id)+1 ids, dry-run names N, delta is exactly N.
func TestImportEventsFreshSequencing(t *testing.T) {
	// Build a source board with extra events absent from the target copy:
	// append them FIRST, then render the export (so the export carries them).
	src := seedImportBoard(t)
	sbd := importBoardDir(src)
	f, _ := os.OpenFile(filepath.Join(sbd, "events.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	if _, err := f.WriteString(`{"id":3,"timestamp":"2026-09-04 10:00:00","event_type":"task_started","task_id":"QA-T-2","actor":"foreman","detail":null,"tick_number":null}` + "\n" +
		`{"id":4,"timestamp":"2026-09-04 11:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":"external note","tick_number":null}` + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	export := mustRenderExport(t, src)

	dst := t.TempDir()
	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}
	// The copy must NOT have the two extra events: rewrite events.jsonl to
	// the first two lines.
	dbd := importBoardDir(dst)
	all := readJSONLLines(t, filepath.Join(dbd, "events.jsonl"))
	if len(all) != 4 {
		t.Fatalf("expected 4 events on source-side copy, got %d", len(all))
	}
	if err := os.WriteFile(filepath.Join(dbd, "events.jsonl"), []byte(strings.Join(all[:2], "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before := countLines(t, filepath.Join(dbd, "events.jsonl"))
	// Dry-run names exactly 2 events.
	out, err := captureStdout(func() {
		if code := runImport(t, dst, export, "--dry-run"); code != 0 {
			t.Fatalf("dry-run exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2 events appended (2 from export") {
		t.Fatalf("dry-run output missing \"2 events appended\":\n%s", out)
	}
	// Real run: delta exactly 2, fresh ids 3 and 4.
	if _, err := captureStdout(func() {
		if code := runImport(t, dst, export); code != 0 {
			t.Fatalf("real import exit = %d, want 0", code)
		}
	}); err != nil {
		t.Fatal(err)
	}
	ev := readJSONLLines(t, filepath.Join(dbd, "events.jsonl"))
	if got := len(ev) - before; got != 2 {
		t.Fatalf("events delta = %d, want 2", got)
	}
	var last struct {
		ID    int    `json:"id"`
		Type  string `json:"event_type"`
		Stamp string `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(ev[len(ev)-1]), &last); err != nil {
		t.Fatal(err)
	}
	if last.ID != 4 || last.Type != "audit" {
		t.Fatalf("last event = id %d (%s), want id 4 audit (fresh MAX(id)+1 sequencing)", last.ID, last.Type)
	}
	if last.Stamp == "2026-09-04 11:00:00" {
		t.Fatal("imported event kept the exported timestamp; want a fresh target-dialect stamp")
	}
	// Idempotent: second run appends nothing.
	mid := countLines(t, filepath.Join(dbd, "events.jsonl"))
	if _, err := captureStdout(func() {
		if code := runImport(t, dst, export); code != 0 {
			t.Fatalf("second import exit = %d, want 0", code)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if after := countLines(t, filepath.Join(dbd, "events.jsonl")); after != mid {
		t.Fatalf("second import changed events.jsonl %d -> %d", mid, after)
	}
}

// TestImportConflictSkipPreservesRow: spec 7.3.5 — same id/different title
// skips by default ("1 skipped (differs)"), row bytes unchanged.
func TestImportConflictSkipPreservesRow(t *testing.T) {
	src := seedImportBoard(t)
	export := mustRenderExport(t, src)

	dst := t.TempDir()
	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}
	dbd := importBoardDir(dst)
	// Mutate QA-T-1's title on the copy (same id, different content).
	conflict := `{"id": "QA-T-1", "title": "renamed on target", "status": "complete", "priority": "P1", "created_at": "2026-09-01 00:00:00", "completed_at": "2026-09-02 00:00:00", "updated_at": "2026-09-02 00:00:00"}` + "\n"
	if err := replaceTaskLine(t, filepath.Join(dbd, "tasks.jsonl"), "QA-T-1", conflict); err != nil {
		t.Fatal(err)
	}
	before := boardFileHashes(t, dst)
	out, err := captureStdout(func() {
		if code := runImport(t, dst, export, "--dry-run"); code != 0 {
			t.Fatalf("dry-run exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "1 skipped (differs)") {
		t.Fatalf("dry-run output missing \"1 skipped (differs)\":\n%s", out)
	}
	if after := boardFileHashes(t, dst); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatal("conflict dry-run wrote to the board")
	}
	// Real run: still skips, target row bytes unchanged.
	if _, err := captureStdout(func() {
		if code := runImport(t, dst, export); code != 0 {
			t.Fatalf("real import exit = %d, want 0", code)
		}
	}); err != nil {
		t.Fatal(err)
	}
	after := boardFileHashes(t, dst)
	if after["tasks.jsonl"] != before["tasks.jsonl"] {
		t.Fatalf("conflicting row bytes changed:\nbefore %s\nafter  %s", before["tasks.jsonl"], after["tasks.jsonl"])
	}
}

// TestImportConflictRenumber: BT-022 board-row criterion — --renumber
// appends the conflicting row under the next free fleet-style id.
func TestImportConflictRenumber(t *testing.T) {
	src := seedImportBoard(t)
	export := mustRenderExport(t, src)

	dst := t.TempDir()
	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}
	dbd := importBoardDir(dst)
	conflict := `{"id": "QA-T-1", "title": "renamed on target", "status": "complete", "priority": "P1", "created_at": "2026-09-01 00:00:00", "completed_at": "2026-09-02 00:00:00", "updated_at": "2026-09-02 00:00:00"}` + "\n"
	if err := replaceTaskLine(t, filepath.Join(dbd, "tasks.jsonl"), "QA-T-1", conflict); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(func() {
		if code := runImport(t, dst, export, "--renumber"); code != 0 {
			t.Fatalf("--renumber import exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "QA-T-3 new task (renumbered from QA-T-1)") {
		t.Fatalf("plan must renumber QA-T-1 to the next free id QA-T-3:\n%s", out)
	}
	if !importBoardHasID(t, filepath.Join(dbd, "tasks.jsonl"), "QA-T-3") {
		t.Fatal("renumbered row QA-T-3 not appended")
	}
	// The conflicting original row must be untouched.
	rows := readJSONLLines(t, filepath.Join(dbd, "tasks.jsonl"))
	for _, l := range rows {
		var m map[string]any
		if json.Unmarshal([]byte(l), &m) == nil && m["id"] == "QA-T-1" && m["title"] != "renamed on target" {
			t.Fatal("original QA-T-1 row was modified")
		}
	}
	// The renumbered row is registered under its NEW id (no duplicate ids).
	seen := map[string]int{}
	for _, l := range rows {
		var m map[string]any
		if json.Unmarshal([]byte(l), &m) != nil {
			continue
		}
		id, _ := m["id"].(string)
		seen[id]++
	}
	for id, n := range seen {
		if n > 1 {
			t.Fatalf("duplicate id %s after renumber import", id)
		}
	}
}

// TestImportFixturesPresent appends missing fixture ids to an existing
// fixtures.jsonl (spec 7.3.8, present branch).
func TestImportFixturesPresent(t *testing.T) {
	src := seedImportBoard(t)
	export := mustRenderExport(t, src)
	dst := t.TempDir()
	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}
	dbd := importBoardDir(dst)
	// Drop QA-FIX from the copy's fixtures.jsonl (and its task row would not
	// exist here; fixture ids alone are what the registry needs) — actually
	// keep tasks untouched, only empty the registry, since import only
	// manages fixtures.jsonl ids.
	if err := os.WriteFile(filepath.Join(dbd, "fixtures.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	before := boardFileHashes(t, dst)
	if _, err := captureStdout(func() {
		if code := runImport(t, dst, export); code != 0 {
			t.Fatalf("import exit = %d, want 0", code)
		}
	}); err != nil {
		t.Fatal(err)
	}
	after := boardFileHashes(t, dst)
	if after["fixtures.jsonl"] == before["fixtures.jsonl"] {
		t.Fatal("fixtures.jsonl unchanged; expected the QA-FIX id appended")
	}
	if !strings.Contains(readFileString(t, filepath.Join(dbd, "fixtures.jsonl")), `"QA-FIX"`) {
		t.Fatal("fixtures.jsonl missing QA-FIX after import")
	}
}

// TestImportFixturesAbsentSkipped: spec 7.3.8 — no fixtures.jsonl on target
// prints the exact warning and never creates the file.
func TestImportFixturesAbsentSkipped(t *testing.T) {
	src := seedImportBoard(t)
	export := mustRenderExport(t, src)
	dst := t.TempDir()
	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}
	dbd := importBoardDir(dst)
	if err := os.Remove(filepath.Join(dbd, "fixtures.jsonl")); err != nil {
		t.Fatal(err)
	}
	before := boardFileHashes(t, dst)
	out, err := captureStdout(func() {
		if code := runImport(t, dst, export); code != 0 {
			t.Fatalf("import exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "no fixtures.jsonl on target; 1 fixture row skipped") {
		t.Fatalf("output missing exact fixture warning:\n%s", out)
	}
	after := boardFileHashes(t, dst)
	if after["fixtures.jsonl"] != "<absent>" {
		t.Fatal("import created fixtures.jsonl; it must never create it")
	}
	if after["tasks.jsonl"] != before["tasks.jsonl"] || after["events.jsonl"] != before["events.jsonl"] {
		t.Fatal("fixture-absent import unexpectedly changed tasks/events")
	}
}

// TestImportHelpAndUnknownArgs: flags/usage coverage.
func TestImportHelpAndUnknownArgs(t *testing.T) {
	// No positional arg -> usage error (exit 2 via run()).
	if code := run([]string{"import"}); code != 2 {
		t.Fatalf("run(import) with no args = %d, want 2", code)
	}
	// Unknown flag -> flag package parse error. House behavior for every
	// subcommand is exit 1 (run() maps only ErrBoardNotFound/errUsage to 2),
	// so import matches the existing commands.
	if code := run([]string{"import", "--bogus", "x"}); code != 1 {
		t.Fatalf("run(import --bogus) = %d, want 1 (house flag-error contract)", code)
	}
	// Unknown subcommand stays unknown.
	if code := run([]string{"importx"}); code != 2 {
		t.Fatalf("run(importx) = %d, want 2", code)
	}
}

// TestImportDryRunByteIdentityAllFiles: spec 7.3.6 — SHA256 identity for
// every board file across a dry-run (already covered inside the no-op test,
// but asserted standalone against a busy plan: filtered target with new
// tasks, events, and fixtures pending).
func TestImportDryRunByteIdentityAllFiles(t *testing.T) {
	src := seedImportBoard(t)
	export := mustRenderExport(t, src)
	dst := t.TempDir()
	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}
	dbd := importBoardDir(dst)
	// Make the plan busy: delete a task row and the fixtures registry.
	all := readJSONLLines(t, filepath.Join(dbd, "tasks.jsonl"))
	if err := os.WriteFile(filepath.Join(dbd, "tasks.jsonl"), []byte(strings.Join(all[:1], "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dbd, "fixtures.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	before := boardFileHashes(t, dst)
	out, err := captureStdout(func() {
		if code := runImport(t, dst, export, "--dry-run"); code != 0 {
			t.Fatalf("dry-run exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2 new tasks") {
		t.Fatalf("dry-run should plan 2 new tasks:\n%s", out)
	}
	after := boardFileHashes(t, dst)
	for _, f := range []string{"tasks.jsonl", "events.jsonl", "board.jsonl", "fixtures.jsonl"} {
		if before[f] != after[f] {
			t.Fatalf("dry-run changed %s", f)
		}
	}
}

// TestImportPlanSummaryLineFormat pins the exact plan-line substrings the
// spec observables name.
func TestImportPlanSummaryLineFormat(t *testing.T) {
	dir := seedImportBoard(t)
	export := mustRenderExport(t, dir)
	out, err := captureStdout(func() {
		if code := runImport(t, dir, export, "--dry-run"); code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"0 new tasks, 0 updates, 0 events",
		"dry-run: nothing written",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("plan output missing %q:\n%s", want, out)
		}
	}
}

// ---------- small fs/test helpers ----------

func readFileString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	return len(readJSONLLines(t, path))
}

func readJSONLLines(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
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

func importBoardHasID(t *testing.T, path, id string) bool {
	t.Helper()
	for _, l := range readJSONLLines(t, path) {
		var m map[string]any
		if json.Unmarshal([]byte(l), &m) == nil && m["id"] == id {
			return true
		}
	}
	return false
}

func importRowByID(t *testing.T, path, id string) map[string]any {
	t.Helper()
	for _, l := range readJSONLLines(t, path) {
		var m map[string]any
		if json.Unmarshal([]byte(l), &m) == nil && m["id"] == id {
			return m
		}
	}
	t.Fatalf("row %s not found in %s", id, path)
	return nil
}

// replaceTaskLine swaps the line carrying id for replacement.
func replaceTaskLine(t *testing.T, path, id, replacement string) error {
	t.Helper()
	lines := readJSONLLines(t, path)
	var out []string
	replaced := false
	for _, l := range lines {
		var m map[string]any
		if json.Unmarshal([]byte(l), &m) == nil && m["id"] == id {
			out = append(out, replacement)
			replaced = true
			continue
		}
		out = append(out, l)
	}
	if !replaced {
		return fmt.Errorf("id %s not found in %s", id, path)
	}
	return os.WriteFile(path, []byte(strings.Join(out, "\n")+"\n"), 0o644)
}

// copyDir recursive copy for the small seeded fixture trees.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
