package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BT-037 CLI coverage: a HEADERLESS board (tasks.jsonl + events.jsonl, NO
// board.jsonl, line 1 an ordinary task row — the measured coding-hermes-tools
// shape) must REFUSE header --set-* with a non-zero exit and keep tasks.jsonl
// byte-identical, while validate reports an informational headerless line
// instead of inventing 'not an integer counter' errors from the task row.
// The package-level contract is pinned in internal/board (bt037_headerless_test.go).
//
// A headerless board and a REAL topology-B board (seedCLITopologyBBoard, whose
// line 1 IS a header) differ only by line 1's shape — the topology-B workflow
// tests in main_test.go stay green and are the counter-case to these.

func seedCLIHeaderlessBoard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"tasks.jsonl": `{"id":"CHT-001","title":"Bootstrap","status":"complete","priority":"P1","created_at":"2026-09-17T01:20:00-05:00","reasoning":"First row.","updated_at":"2026-09-17T02:49:50-05:00","completed_at":"2026-09-17T02:49:50-05:00","attributes":{"tick":"t1"}}` + "\n" +
			`{"id":"CHT-002","title":"Diff3","status":"pending","priority":"P2","updated_at":"2026-09-17T03:00:00-05:00"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-17 01:20:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(boardDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func cliFileSHA(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestCmdHeaderlessHeaderSetRefused is acceptance criterion 1 at the CLI:
// non-zero exit, explicit 'no header' error, tasks.jsonl sha256 unchanged.
func TestCmdHeaderlessHeaderSetRefused(t *testing.T) {
	dir := seedCLIHeaderlessBoard(t)
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	before := cliFileSHA(t, tasksPath)

	var code int
	stderr, err := captureStderr(func() {
		code = run([]string{"-C", dir, "header", "--set-ticks-total", "90"})
	})
	if err != nil {
		t.Fatalf("captureStderr: %v", err)
	}
	if code == 0 {
		t.Fatalf("header --set-ticks-total on a headerless board exited 0 (stderr: %s)", stderr)
	}
	if !strings.Contains(stderr, "no header on this board") {
		t.Fatalf("error does not say 'no header on this board': %q", stderr)
	}
	if !strings.Contains(stderr, "CHT-001") {
		t.Fatalf("error does not name the task row it protected: %q", stderr)
	}
	if after := cliFileSHA(t, tasksPath); after != before {
		t.Fatalf("tasks.jsonl changed under a refused header write:\nbefore %s\nafter  %s", before, after)
	}
	// the corrupted shape BT-037 was filed for: line 1 grew a header counter
	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	line1 := strings.SplitN(string(raw), "\n", 2)[0]
	if strings.Contains(line1, "ticks_total") {
		t.Fatalf("header counter written into the first task row: %s", line1)
	}
}

// TestCmdHeaderlessHeaderReadRefused: reading the header of a board that has
// none must not print a task row as if it were the header.
func TestCmdHeaderlessHeaderReadRefused(t *testing.T) {
	dir := seedCLIHeaderlessBoard(t)
	var code int
	var out string
	stderr, err := captureStderr(func() {
		out, _ = captureStdout(func() {
			code = run([]string{"-C", dir, "header", "--json"})
		})
	})
	if err != nil {
		t.Fatalf("captureStderr: %v", err)
	}
	if code == 0 {
		t.Fatalf("header --json on a headerless board exited 0 (stdout %q)", out)
	}
	if !strings.Contains(stderr, "no header on this board") {
		t.Fatalf("error %q does not say 'no header on this board'", stderr)
	}
	if strings.Contains(out, `"title"`) || strings.Contains(out, "CHT-001") {
		t.Fatalf("a task row was printed as the board header: %q", out)
	}
}

// TestCmdHeaderlessValidateInformational is acceptance criterion 2: exit 0,
// the informational headerless line, no false counter errors.
func TestCmdHeaderlessValidateInformational(t *testing.T) {
	dir := seedCLIHeaderlessBoard(t)
	var code int
	got, err := captureStdout(func() {
		code = run([]string{"-C", dir, "validate"})
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if code != 0 {
		t.Fatalf("validate on a clean headerless board exit code = %d, want 0\noutput:\n%s", code, got)
	}
	if !strings.Contains(got, "headerless board (no board.jsonl)") {
		t.Fatalf("validate did not report the headerless board:\n%s", got)
	}
	if strings.Contains(got, "not an integer counter") {
		t.Fatalf("validate validated a task row as the header:\n%s", got)
	}
	if strings.Contains(got, "header:") {
		t.Fatalf("validate emitted header findings:\n%s", got)
	}
	if !strings.Contains(got, "RESULT: OK") {
		t.Fatalf("validate should stay OK on a headerless board:\n%s", got)
	}
	if !strings.Contains(got, "2 tasks, 1 events") {
		t.Fatalf("validate did not count BOTH task rows (line 1 is a task):\n%s", got)
	}
}

// TestCmdHeaderlessDoctorGreen: doctor skips the header cross-checks with a
// note and stays green.
func TestCmdHeaderlessDoctorGreen(t *testing.T) {
	dir := seedCLIHeaderlessBoard(t)
	var code int
	got, err := captureStdout(func() {
		code = run([]string{"-C", dir, "doctor"})
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if code != 0 || strings.Contains(got, "RESULT: FAIL") {
		t.Fatalf("doctor on a headerless board exit code = %d\noutput:\n%s", code, got)
	}
	if !strings.Contains(got, "header-vs-events tick drift checks skipped") {
		t.Fatalf("doctor did not note the skipped header check:\n%s", got)
	}
	if strings.Contains(got, "[error]") {
		t.Fatalf("doctor emitted errors on a clean headerless board:\n%s", got)
	}
}

// TestCmdHeaderlessEventTickUnchanged: `event --tick` still appends on a
// board with no header, and never touches tasks.jsonl.
func TestCmdHeaderlessEventTickUnchanged(t *testing.T) {
	dir := seedCLIHeaderlessBoard(t)
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	before := cliFileSHA(t, tasksPath)
	if code := run([]string{"-C", dir, "event", "--type", "tick", "--tick", "52"}); code != 0 {
		t.Fatalf("event --tick on a headerless board exit code = %d, want 0", code)
	}
	if after := cliFileSHA(t, tasksPath); after != before {
		t.Fatalf("event --tick rewrote tasks.jsonl on a headerless board:\nbefore %s\nafter  %s", before, after)
	}
}

// TestCmdHeaderlessListAndShowRead: the headerless classification must not
// restrict reads — list and show still see every task row.
func TestCmdHeaderlessListAndShowRead(t *testing.T) {
	dir := seedCLIHeaderlessBoard(t)
	var code int
	got, err := captureStdout(func() {
		code = run([]string{"-C", dir, "list", "--json", "--all"})
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if code != 0 {
		t.Fatalf("list on a headerless board exit code = %d, want 0\noutput:\n%s", code, got)
	}
	for _, id := range []string{"CHT-001", "CHT-002"} {
		if !strings.Contains(got, id) {
			t.Fatalf("list output is missing %s:\n%s", id, got)
		}
	}
	if code := run([]string{"-C", dir, "show", "CHT-001"}); code != 0 {
		t.Fatalf("show CHT-001 on a headerless board exit code = %d, want 0", code)
	}
}

// TestCmdHeaderlessArchiveWritesStillWork pins the rest of the write path on
// the same board: create appends after the last task row and leaves line 1
// byte-identical.
func TestCmdHeaderlessArchiveWritesStillWork(t *testing.T) {
	dir := seedCLIHeaderlessBoard(t)
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	line1 := strings.SplitN(string(raw), "\n", 2)[0]
	if code := run([]string{"-C", dir, "create", "--id", "CHT-003", "--title", "Patch", "--priority", "P2"}); code != 0 {
		t.Fatalf("create on a headerless board exit code = %d, want 0", code)
	}
	raw, err = os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("tasks.jsonl has %d rows, want 3", len(lines))
	}
	if lines[0] != line1 {
		t.Fatalf("create mutated line 1:\n got %s\nwant %s", lines[0], line1)
	}
	if !strings.Contains(lines[2], `"CHT-003"`) {
		t.Fatalf("create appended the wrong row: %s", lines[2])
	}
}
