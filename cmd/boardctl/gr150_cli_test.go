package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- GR-150: CLI surface for fused / multi-value physical lines ----------
//
// The corruption shape is the one measured on the committed GitReins board:
// several complete JSON objects concatenated on ONE physical line. Before the
// fix `validate` reported RESULT OK and `list` reported a smaller board; now
// every read surface rejects the line with a file:line diagnostic, and the
// tolerant read carries it as skipped evidence (exit stays 1).

// gr150SeedFusedBoard bootstraps a topology-A board via cmdInit and plants a
// tasks.jsonl whose line 2 fuses two complete records.
func gr150SeedFusedBoard(t *testing.T) (dir, boardDir string) {
	t.Helper()
	dir = t.TempDir()
	if err := cmdInit(dir, []string{"--project", "gr150"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	boardDir = filepath.Join(dir, ".coding-hermes", "board")
	tasks := `{"id": "GR150-A", "title": "alpha", "status": "pending", "priority": "P1"}` + "\n" +
		`{"id": "GR150-B", "title": "beta", "status": "pending", "priority": "P2"}` +
		`{"id": "GR150-C", "title": "gamma", "status": "complete", "priority": "P3"}` + "\n"
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, boardDir
}

// AC: validate REJECTS the fused line — exit 1, an error finding naming the
// physical line and the problem, and never "RESULT: OK".
func TestGR150CLIValidateRejectsFusedLine(t *testing.T) {
	dir, _ := gr150SeedFusedBoard(t)
	var out, errOut string
	var code int
	var err error
	out, err = captureStdout(func() {
		serr, cerr := captureStderr(func() { code = run([]string{"-C", dir, "validate"}) })
		if cerr != nil {
			t.Fatal(cerr)
		}
		errOut = serr
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("validate on a fused board exit = %d, want 1\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	if strings.Contains(out, "RESULT: OK") {
		t.Fatalf("validate reported RESULT: OK on a fused board:\n%s", out)
	}
	combined := out + errOut
	for _, want := range []string{"line 2", "multiple JSON values", "one JSON object per line"} {
		if !strings.Contains(combined, want) {
			t.Errorf("validate output missing %q:\nstdout:\n%s\nstderr:\n%s", want, out, errOut)
		}
	}
}

// AC: list REJECTS the fused board (exit 1, no rows) instead of silently
// reporting only the records that happened to stand alone.
func TestGR150CLIListRejectsFusedLine(t *testing.T) {
	dir, _ := gr150SeedFusedBoard(t)
	var out, errOut string
	var code int
	var err error
	out, err = captureStdout(func() {
		serr, cerr := captureStderr(func() { code = run([]string{"-C", dir, "list"}) })
		if cerr != nil {
			t.Fatal(cerr)
		}
		errOut = serr
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("list on a fused board exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "line 2") || !strings.Contains(errOut, "multiple JSON values") {
		t.Fatalf("list stderr = %q, want the file:line multi-value diagnostic", errOut)
	}
	if strings.Contains(out, "GR150-A") || strings.Contains(out, "GR150-B") || strings.Contains(out, "GR150-C") {
		t.Fatalf("list emitted rows for a fused board:\n%s", out)
	}
}

// AC: the tolerant read survives, lists the salvageable row, itemizes the
// fused line as skipped evidence, and still exits 1 (degraded != success).
func TestGR150CLIListSkipBadLinesReportsFusedEvidence(t *testing.T) {
	dir, _ := gr150SeedFusedBoard(t)
	var out, errOut string
	var code int
	var err error
	out, err = captureStdout(func() {
		serr, cerr := captureStderr(func() { code = run([]string{"-C", dir, "list", "--skip-bad-lines"}) })
		if cerr != nil {
			t.Fatal(cerr)
		}
		errOut = serr
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("list --skip-bad-lines on a fused board exit = %d, want 1", code)
	}
	if !strings.Contains(out, "GR150-A") {
		t.Fatalf("salvageable row missing from output:\n%s", out)
	}
	for _, want := range []string{"SKIPPED-LINES: 1", "tasks.jsonl line 2", "multiple JSON values"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("skip evidence missing %q:\n%s", want, errOut)
		}
	}
}

// AC: show refuses too — it must not hand back the first object as if the line
// were a clean record. The fused records (B and C) are unreachable by id, and
// the error says why.
func TestGR150CLIShowRejectsFusedLine(t *testing.T) {
	dir, _ := gr150SeedFusedBoard(t)
	var errOut string
	var code int
	var err error
	_, err = captureStdout(func() {
		serr, cerr := captureStderr(func() { code = run([]string{"-C", dir, "show", "GR150-B"}) })
		if cerr != nil {
			t.Fatal(cerr)
		}
		errOut = serr
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("show GR150-B on a fused board exit = %d, want 1 (not a silent not-found)", code)
	}
	if !strings.Contains(errOut, "line 2") || !strings.Contains(errOut, "multiple JSON values") {
		t.Fatalf("show stderr = %q, want the file:line multi-value diagnostic", errOut)
	}
}

// AC: validate --repair refuses the ambiguous stream, writes no review file,
// and leaves tasks.jsonl byte-identical.
func TestGR150CLIValidateRepairRefusesFusedLine(t *testing.T) {
	dir, boardDir := gr150SeedFusedBoard(t)
	tasksPath := filepath.Join(boardDir, "tasks.jsonl")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	var errOut string
	var code int
	_, err = captureStdout(func() {
		serr, cerr := captureStderr(func() { code = run([]string{"-C", dir, "validate", "--repair"}) })
		if cerr != nil {
			t.Fatal(cerr)
		}
		errOut = serr
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("validate --repair on a fused board exit = %d, want 1 (refusal)", code)
	}
	if !strings.Contains(errOut, "line 2") || !strings.Contains(errOut, "refus") {
		t.Fatalf("repair refusal stderr = %q, want the line number and an explicit refusal", errOut)
	}
	if _, statErr := os.Stat(filepath.Join(boardDir, "tasks.rewritten.jsonl")); statErr == nil {
		t.Error("refused repair wrote tasks.rewritten.jsonl")
	}
	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("refused repair modified tasks.jsonl")
	}
}

// AC: a clean board still validates OK and lists every row — the rejection is
// specific to the fused shape.
func TestGR150CLICleanBoardStillPasses(t *testing.T) {
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "gr150"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	tasks := `{"id": "GR150-A", "title": "alpha", "status": "pending", "priority": "P1"}` + "\n" +
		`{"id": "GR150-B", "title": "beta", "status": "pending", "priority": "P2"}` + "\n" +
		`{"id": "GR150-C", "title": "gamma", "status": "complete", "priority": "P3"}` + "\n"
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "validate"}); code != 0 {
			t.Errorf("validate on a clean board exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "rows: 3 tasks") {
		t.Fatalf("validate on the clean control did not count 3 tasks:\n%s", out)
	}
	if !strings.Contains(out, "RESULT: OK") {
		t.Fatalf("validate on a clean board did not report RESULT: OK:\n%s", out)
	}
}
