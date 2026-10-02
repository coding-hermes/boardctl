package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- DF-BOARDCTL-25: doctor distinguishes topology B from corrupt line 1 ----------

// The `validate --repair` QA pass against a damaged board found that doctor
// reports a topology-B board (no board.jsonl; line 1 of tasks.jsonl IS a valid
// board header) and a genuinely corrupt tasks.jsonl line 1 (mojibake /
// invalid JSON) identically: the healthy legacy board got NO line-1 note at
// all, while the corrupt one got only bare parse errors — with no
// salvageability distinction between "this is the legacy layout, it is fine"
// and "your header line is destroyed, here is how to recover the rows".
//
// These tests hold the real CLI (run(), not a fake validator) to the
// distinction on disposable boards, in the TestQACorruption shape: bootstrap
// through the real init/create paths, mutate exactly ONE named file, assert
// the premise, assert exit codes and doctor output lines, and prove zero
// writes with a sha256 snapshot of all four board files before/after.

// df25SeedTopologyB bootstraps a fresh board through the REAL CLI paths
// (cmdInit + run create) in a t.TempDir, then converts it to topology B the
// way live legacy boards look: board.jsonl's line 1 (the header row) is
// prepended to tasks.jsonl and board.jsonl is deleted. The premise is
// asserted: doctor resolves the result as topology B, green, header parsed.
func df25SeedTopologyB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "df25"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if code := run([]string{"-C", dir, "create", "--id", "DF25-1", "--title", "seed task", "--priority", "P2"}); code != 0 {
		t.Fatalf("create exit code = %d, want 0", code)
	}
	bd := qaBoardDir(dir)
	hdr, err := os.ReadFile(filepath.Join(bd, "board.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := os.ReadFile(filepath.Join(bd, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	hdrLine := bytes.TrimSpace(hdr)
	if !json.Valid(hdrLine) {
		t.Fatalf("premise: board.jsonl line 1 is not valid JSON: %q", string(hdrLine))
	}
	merged := append(append(append([]byte{}, hdrLine...), '\n'), tasks...)
	if err := os.WriteFile(filepath.Join(bd, "tasks.jsonl"), merged, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(bd, "board.jsonl")); err != nil {
		t.Fatal(err)
	}
	code, out := qaRunCLI(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("premise: doctor on the converted board exit = %d, want 0\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "(topology B)") {
		t.Fatalf("premise: doctor did not resolve the board as topology B\noutput:\n%s", out)
	}
	return dir
}

// df25CorruptLineOne replaces tasks.jsonl line 1 with a fixed mojibake row
// (asserted nonempty and NOT valid JSON), keeping every later line verbatim —
// exactly one named file changes.
func df25CorruptLineOne(t *testing.T, boardDir string) {
	t.Helper()
	path := filepath.Join(boardDir, "tasks.jsonl")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(string(raw), "\n", 2)
	rest := ""
	if len(parts) > 1 {
		rest = parts[1]
	}
	corrupt := "} mojibake \xff\xfe not json {\n"
	line := bytes.TrimSpace([]byte(corrupt))
	if len(line) == 0 {
		t.Fatal("premise: corruption string is empty")
	}
	if json.Valid(line) {
		t.Fatalf("premise: corruption string is VALID JSON: %q", string(line))
	}
	if err := os.WriteFile(path, append([]byte(corrupt), []byte(rest)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDF25DoctorDistinguishesTopologyBFromCorruptLineOne is the
// DF-BOARDCTL-25 acceptance pair, driven through the real CLI:
//
//   - a topology-B board whose line 1 IS a valid header must be reported as
//     the legacy layout — a distinct warn naming it valid and writable, zero
//     errors — and must never be called corruption
//   - a board whose tasks.jsonl line 1 is mojibake must be reported as
//     CORRUPTION of the topology-B header line — an error carrying the
//     salvage hint (`boardctl validate --repair`) — and must never be
//     reported as the healthy legacy layout
//
// RED on the pre-fix code: the healthy case has no topology note at all and
// the corrupt case carries only bare parse errors with no salvage hint, so
// the two outputs were indistinguishable without reading the JSON yourself.
func TestDF25DoctorDistinguishesTopologyBFromCorruptLineOne(t *testing.T) {
	t.Run("valid line-1 header: reported as legacy topology B, zero errors", func(t *testing.T) {
		dir := df25SeedTopologyB(t)
		bd := qaBoardDir(dir)
		before := qaBoardFileHashes(t, bd)
		code, out := qaRunCLI(t, dir, "doctor")
		if code != 0 || strings.Contains(out, "RESULT: FAIL") {
			t.Fatalf("doctor on a valid topology-B board exit = %d, want 0\noutput:\n%s", code, out)
		}
		if !strings.Contains(out, "[warn] legacy topology B") {
			t.Fatalf("doctor did not report the legacy topology-B header as a distinct, healthy layout\noutput:\n%s", out)
		}
		if !strings.Contains(out, "not corruption") {
			t.Fatalf("topology-B note does not say explicitly that the layout is not corruption\noutput:\n%s", out)
		}
		if strings.Contains(out, "[error]") {
			t.Fatalf("doctor emitted errors on a valid topology-B board\noutput:\n%s", out)
		}
		if strings.Contains(out, "corruption of the topology-B header line") {
			t.Fatalf("valid topology-B board misreported as corruption\noutput:\n%s", out)
		}
		if ch := qaChangedFiles(before, qaBoardFileHashes(t, bd)); len(ch) != 0 {
			t.Fatalf("doctor wrote to board files: %v", ch)
		}
	})

	t.Run("corrupt line 1: reported as corruption with a salvage hint, not the legacy layout", func(t *testing.T) {
		dir := df25SeedTopologyB(t)
		bd := qaBoardDir(dir)
		before := qaBoardFileHashes(t, bd)
		df25CorruptLineOne(t, bd)
		afterMut := qaBoardFileHashes(t, bd)
		if ch := qaChangedFiles(before, afterMut); len(ch) != 1 || ch[0] != "tasks.jsonl" {
			t.Fatalf("mutation changed %v, want exactly [tasks.jsonl]", ch)
		}
		code, out := qaRunCLI(t, dir, "doctor")
		if code != 1 || !strings.Contains(out, "RESULT: FAIL") {
			t.Fatalf("doctor on a corrupt line-1 board exit = %d, want 1\noutput:\n%s", code, out)
		}
		if !strings.Contains(out, "corruption of the topology-B header line") {
			t.Fatalf("doctor did not name corrupt line 1 as topology-B header corruption\noutput:\n%s", out)
		}
		if !strings.Contains(out, "fix: boardctl validate --repair") {
			t.Fatalf("corruption error missing the salvage hint\noutput:\n%s", out)
		}
		if strings.Contains(out, "[warn] legacy topology B") {
			t.Fatalf("corrupt line 1 reported as the healthy legacy layout\noutput:\n%s", out)
		}
		if ch := qaChangedFiles(afterMut, qaBoardFileHashes(t, bd)); len(ch) != 0 {
			t.Fatalf("doctor wrote to board files after the mutation: %v", ch)
		}
	})
}
