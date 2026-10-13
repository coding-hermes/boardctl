package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- DF-BOARDCTL-31 CLI acceptance: validate after the id-dupe repair ----------
//
// Proven through a REAL boardctl binary so the exit codes are the operator's
// exit codes (skipped under -short like the other binary-building tests).
// The three arms:
//
//  1. a later duplicate carrying superseded_by (what dedupe-board --by-id
//     --apply writes) validates with EXIT 0 and a RESOLVED warning;
//  2. the same board WITHOUT superseded_by still exits 1 with the plain
//     duplicate-id error;
//  3. end-to-end: plain-dup board fails -> dedupe-board --by-id --apply ->
//     the SAME dir validates exit 0 (this arm is TestValidateIDDupeRepairGoesGreenCLI below).
const df31KeptLine = `{"id":"QA-ASCE-1","title":"[P2] header-frozen on asce main","status":"pending","priority":"P2","created_at":"2026-10-01 00:00:00"}`

// df31BuildBoard writes a two-line duplicate-id tasks.jsonl into a fresh
// boardctl-init'd board dir and returns the dir.
func df31BuildBoard(t *testing.T, boardctlBin, laterLine string) string {
	t.Helper()
	dir := t.TempDir()
	out, err := exec.Command(boardctlBin, "init", "-C", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	boardDir := ""
	for _, l := range strings.Split(string(out), "\n") {
		if i := strings.Index(l, "initialized board at "); i >= 0 {
			boardDir = strings.TrimSpace(l[i+len("initialized board at "):])
			break
		}
	}
	if boardDir == "" {
		t.Fatalf("init output missing board dir:\n%s", out)
	}
	tasks := df31KeptLine + "\n" + laterLine + "\n"
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	return boardDir
}

// df31RunValidate runs validate with the REAL binary and returns (output, exitCode).
func df31RunValidate(t *testing.T, boardctlBin, dir string) (string, int) {
	t.Helper()
	out, err := exec.Command(boardctlBin, "validate", "-C", dir).CombinedOutput()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("validate: %v\n%s", err, out)
		}
		code = ee.ExitCode()
	}
	return string(out), code
}

// Arm 1: the superseded duplicate validates EXIT 0 with a RESOLVED warning.
func TestDF31ValidateSupersededDuplicateExitZero(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the real binary; skipped under -short")
	}
	bin := filepath.Join(t.TempDir(), "boardctl")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/boardctl")
	cmd.Dir = repoRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	dir := df31BuildBoard(t, bin,
		`{"id":"QA-ASCE-1","title":"[P2] spawn pool exhausted","status":"complete","priority":"P2","superseded_by":"QA-ASCE-1","worker_summary":"superseded-by-earliest: recycled task id — kept line 1 (id-dupe backfill 2026-10-13)","completed_at":"2026-10-13 00:00:00","created_at":"2026-10-02 00:00:00"}`)
	out, code := df31RunValidate(t, bin, dir)
	if code != 0 {
		t.Fatalf("validate on a superseded-duplicate board exit = %d, want 0:\n%s", code, out)
	}
	if !strings.Contains(out, "[warn]") || !strings.Contains(out, "RESOLVED") || !strings.Contains(out, "duplicate task id") {
		t.Fatalf("expected a RESOLVED duplicate-task-id warning in the report:\n%s", out)
	}
	if strings.Contains(out, "[error]") || strings.Contains(out, "RESULT: FAIL") {
		t.Fatalf("resolved duplicate still reported as an error:\n%s", out)
	}
	if !strings.Contains(out, "RESULT: OK") {
		t.Fatalf("report missing RESULT: OK:\n%s", out)
	}
}

// Arm 2: the SAME board WITHOUT superseded_by still exits 1 with the
// historical duplicate-id error (behavior unchanged).
func TestDF31ValidatePlainDuplicateStillExitOne(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the real binary; skipped under -short")
	}
	bin := filepath.Join(t.TempDir(), "boardctl")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/boardctl")
	cmd.Dir = repoRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	dir := df31BuildBoard(t, bin,
		`{"id":"QA-ASCE-1","title":"[P2] spawn pool exhausted","status":"pending","priority":"P2","created_at":"2026-10-02 00:00:00"}`)
	out, code := df31RunValidate(t, bin, dir)
	if code != 1 {
		t.Fatalf("validate on a plain-duplicate board exit = %d, want 1:\n%s", code, out)
	}
	if !strings.Contains(out, `[error] tasks.jsonl line 2: duplicate task id "QA-ASCE-1" (also line 1)`) {
		t.Fatalf("plain duplicate-id error changed:\n%s", out)
	}
	if !strings.Contains(out, "RESULT: FAIL") {
		t.Fatalf("plain duplicate board must read RESULT: FAIL:\n%s", out)
	}
}

// Arm 3 (end-to-end): a plain-duplicate board fails validate, then
// dedupe-board --by-id --apply runs against the SAME dir, then validate
// exits 0 on it — the fleet loop the task acceptance describes.
func TestDF31DedupeBoardApplyThenValidateGreen(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the real binary; skipped under -short")
	}
	root := repoRoot(t)
	bin := filepath.Join(t.TempDir(), "boardctl")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/boardctl")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build boardctl: %v\n%s", err, out)
	}
	dbin := filepath.Join(t.TempDir(), "dedupe-board")
	cmd = exec.Command("go", "build", "-o", dbin, "./cmd/dedupe-board")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build dedupe-board: %v\n%s", err, out)
	}
	dir := df31BuildBoard(t, bin,
		`{"id":"QA-ASCE-1","title":"[P2] spawn pool exhausted","status":"in_progress","priority":"P2","created_at":"2026-10-02 00:00:00"}`)
	if _, code := df31RunValidate(t, bin, dir); code != 1 {
		t.Fatalf("precondition: pre-repair validate must exit 1, got %d", code)
	}
	out, err := exec.Command(dbin, "--board-dir", dir, "--by-id", "--apply").CombinedOutput()
	if err != nil {
		t.Fatalf("dedupe-board --by-id --apply: %v\n%s", err, out)
	}
	got, code := df31RunValidate(t, bin, dir)
	if code != 0 {
		t.Fatalf("post-repair validate exit = %d, want 0:\n%s", code, got)
	}
	if !strings.Contains(got, "RESULT: OK") || !strings.Contains(got, "RESOLVED") {
		t.Fatalf("post-repair report missing RESULT: OK / RESOLVED warning:\n%s", got)
	}
}
