package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedSweepBoard writes a board with one ALIAS row (todo), one UNKNOWN row
// (duplicate) and one clean row — the REVIEW-BOARDCTL-001 sweep test board.
func seedSweepBoard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"tasks.jsonl": `{"id":"ALIAS-1","title":"alias row","status":"todo","priority":"P2"}` + "\n" +
			`{"id":"DUP-1","title":"duplicate row","status":"duplicate","priority":"P2"}` + "\n" +
			`{"id":"CLEAN-1","title":"clean row","status":"pending","priority":"P2"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"demo","namespace":"demo","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(boardDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// REVIEW-BOARDCTL-001: sweep-status is DRY-RUN BY DEFAULT — it reports the
// 2/3 off-vocabulary census (alias row fixable, unknown row needs a decision)
// and writes NOTHING (byte-compare).
func TestCmdSweepStatusDryRunWritesNothing(t *testing.T) {
	dir := seedSweepBoard(t)
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "sweep-status"}); code != 0 {
			t.Fatalf("sweep-status exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, `ALIAS-1: status "todo"`) || !strings.Contains(got, "--normalize") {
		t.Fatalf("dry-run missing the alias row's normalize action:\n%s", got)
	}
	if !strings.Contains(got, `DUP-1: status "duplicate"`) || !strings.Contains(got, "explicit") {
		t.Fatalf("dry-run missing the unknown row's explicit decision:\n%s", got)
	}
	if !strings.Contains(got, "2/3 rows off-vocabulary: 1 fixable by --normalize, 1 need explicit decision") {
		t.Fatalf("dry-run census line missing or wrong:\n%s", got)
	}
	if !strings.Contains(got, "DRY-RUN") {
		t.Fatalf("dry-run banner missing:\n%s", got)
	}
	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("dry-run sweep rewrote tasks.jsonl:\nbefore %s\nafter  %s", before, after)
	}
}

// REVIEW-BOARDCTL-001: with --apply the sweep canonicalizes ONLY the alias row
// (via the normalize machinery), leaves the duplicate row byte-identical, and
// reports before/after counts; a second run then reports 1/3 with the
// duplicate still demanding an explicit decision.
func TestCmdSweepStatusApplyNormalizesAliasOnly(t *testing.T) {
	dir := seedSweepBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	tasksPath := filepath.Join(boardDir, "tasks.jsonl")
	dupBefore, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}

	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "sweep-status", "--apply"}); code != 0 {
			t.Fatalf("sweep-status --apply exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, "before: 2 off-vocabulary rows; after: 1 off-vocabulary rows") {
		t.Fatalf("apply report missing before/after counts:\n%s", got)
	}
	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	rawStr := string(raw)
	if !strings.Contains(rawStr, `"status":"pending"`) || strings.Contains(rawStr, `"todo"`) {
		t.Fatalf("--apply did not canonicalize the alias row:\n%s", rawStr)
	}
	if !strings.Contains(rawStr, `"status":"duplicate"`) {
		t.Fatalf("--apply touched the unknown row (refuse-to-guess violated):\n%s", rawStr)
	}
	// the duplicate row's line is byte-identical to before
	dupLineBefore := `{"id":"DUP-1","title":"duplicate row","status":"duplicate","priority":"P2"}`
	if !strings.Contains(rawStr, dupLineBefore+"\n") {
		t.Fatalf("duplicate row was rewritten:\n%s", rawStr)
	}

	// second run: 1/3 off-vocab remains, the duplicate needs an explicit
	// decision, and the clean board portion stays untouched.
	got2, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "sweep-status"}); code != 0 {
			t.Fatalf("second sweep exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got2, "1/3 rows off-vocabulary: 0 fixable by --normalize, 1 need explicit decision") {
		t.Fatalf("second sweep census wrong:\n%s", got2)
	}
	if strings.Contains(got2, "ALIAS-1") {
		t.Fatalf("second sweep still reports the normalized row:\n%s", got2)
	}
	// ...and a second --apply is a no-op on the file (nothing to normalize).
	if _, err := os.ReadFile(tasksPath); err != nil {
		t.Fatal(err)
	}
	before2 := raw
	_, err = captureStdout(func() {
		if code := run([]string{"-C", dir, "sweep-status", "--apply"}); code != 0 {
			t.Fatalf("second apply exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	after2, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before2) != string(after2) {
		t.Fatalf("second --apply rewrote tasks.jsonl:\nbefore %s\nafter  %s", before2, after2)
	}
	_ = dupBefore
}

// REVIEW-BOARDCTL-001: --json mirrors dedupe-board's --json shape (the report
// struct encoded with the same fields the text report prints).
func TestCmdSweepStatusJSON(t *testing.T) {
	dir := seedSweepBoard(t)
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "sweep-status", "--json"}); code != 0 {
			t.Fatalf("sweep-status --json exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	for _, want := range []string{`"board_path"`, `"apply": false`, `"off_before": 2`, `"normalized"`, `"explicit"`, `"status": "duplicate"`, `"canonical": "pending"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("JSON report missing %s:\n%s", want, got)
		}
	}
}
