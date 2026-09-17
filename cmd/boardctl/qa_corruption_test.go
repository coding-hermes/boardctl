package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ---------- QA-BOARDCTL-001: corruption coverage on disposable boards ----------

// The old QA chaos-corruption cell was VACUOUS for boardctl: the harness
// truncated dagger.db in the project workdir believing it was project state.
// It is not — it is the live dagger-engine run store (untracked, held open by
// the running engine) and boardctl never reads it. These tests replace that
// gap with real coverage: corrupt ONE named board JSONL file on a disposable
// t.TempDir board, exercise the REAL CLI (run(), not a fake validator), and
// hold it to its contract:
//
//   - nonzero exit with a diagnostic naming the corrupted file
//   - no panic, no mention of dagger.db (board state is JSONL only)
//   - ZERO writes after the mutation (validation/reads never "repair" the
//     corrupted file or touch its siblings or the decoy)
//   - restoring the original bytes returns the board to a clean state
//     (detection, not destruction)
//
// Every board lives in t.TempDir; the real repo board and the real dagger.db
// are never opened. The dagger.db decoy mirrors the incident shape (a
// dagger.db sitting in the project root) and must stay byte-identical and
// unmentioned throughout.
//
// GAP CLOSED (BT-032, this file's validate table now covers fixtures.jsonl):
// the KNOWN GAP recorded below was a real defect — `boardctl validate`
// SILENTLY PASSED a truncated fixtures.jsonl because validateFixtures
// (internal/board/validate.go) called IterParsed without capturing its
// error, so parse failures produced no finding and exit 0 (siblings
// validateTasks/validateEvents did capture it). Discovered by QA-BOARDCTL-001
// (commit f447943), which moved the fixtures case out of the validate table
// into TestQACorruptionFixturesDetectedViaReader rather than enshrine the
// silent pass. BT-032 captures the error in validateFixtures, restores the
// fixtures.jsonl case to the TestQACorruption table, and keeps the
// reader-path test as complementary coverage.

// qaBoardDir returns the board dir the CLI resolves under a repo root.
func qaBoardDir(repo string) string {
	return filepath.Join(repo, ".coding-hermes", "board")
}

// qaRegistryFixtureID is a fixtures.jsonl registry row seeded WITHOUT a task
// row: `show` on it must consult fixtures.jsonl (tasks.jsonl misses first),
// which makes the fixtures corruption case a real detection path.
const qaRegistryFixtureID = "QA-REG-FIXTURE"

// qaSeedValidBoard bootstraps a fresh board through the REAL CLI paths
// (cmdInit + run create/event) in a t.TempDir, seeds one task, one event and
// one fixtures.jsonl registry row, plants a dagger.db decoy in the repo root,
// and proves the board validates clean — the premise every corruption case
// depends on.
func qaSeedValidBoard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "qa-corruption"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if code := run([]string{"-C", dir, "create", "--id", "QA-CORRUPT-1",
		"--title", "valid seeded task", "--priority", "P2"}); code != 0 {
		t.Fatalf("create exit code = %d, want 0", code)
	}
	if code := run([]string{"-C", dir, "event", "--type", "audit",
		"--task-id", "QA-CORRUPT-1", "--detail-text", "qa corruption seed event"}); code != 0 {
		t.Fatalf("event exit code = %d, want 0", code)
	}
	fixtures := filepath.Join(qaBoardDir(dir), "fixtures.jsonl")
	f, err := os.OpenFile(fixtures, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	regRow := `{"id": "` + qaRegistryFixtureID + `", "title": "registry fixture (no task row)", "status": "pending", "active": true}` + "\n"
	if _, err := f.WriteString(regRow); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	decoy := filepath.Join(dir, "dagger.db")
	decoyContent := "QA decoy — NOT board state; boardctl reads JSONL only, never dagger.db\n"
	if err := os.WriteFile(decoy, []byte(decoyContent), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := qaRunCLI(t, dir, "validate")
	if code != 0 {
		t.Fatalf("premise: freshly seeded board failed validate (exit %d):\n%s", code, out)
	}
	return dir
}

// qaRunCLI runs `boardctl <args...> -C dir` through the real run(),
// capturing stdout+stderr and converting any panic into a test failure
// instead of a dead test binary.
func qaRunCLI(t *testing.T, dir string, args ...string) (code int, output string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("boardctl %v panicked: %v", args, r)
		}
	}()
	full := append([]string{"-C", dir}, args...)
	var errOut string
	out, err := captureStdout(func() {
		serr, cerr := captureStderr(func() {
			code = run(full)
		})
		if cerr != nil {
			t.Errorf("captureStderr: %v", cerr)
		}
		errOut = serr
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	return code, out + errOut
}

// qaReadBytes reads a file's full content (test helper with fatal errors).
func qaReadBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// qaBoardFileHashes hashes every board-state file in the board dir. Missing
// files hash as "<absent>" so a CLI run that creates or deletes a file is
// detectable too. The dagger.db decoy (and its sqlite sidecars) are included
// so any write to them by a boardctl run fails the same comparator.
func qaBoardFileHashes(t *testing.T, boardDir string) map[string]string {
	t.Helper()
	names := []string{"tasks.jsonl", "events.jsonl", "board.jsonl", "fixtures.jsonl",
		"dagger.db", "dagger.db-wal", "dagger.db-shm"}
	out := map[string]string{}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(boardDir, name))
		if err != nil {
			out[name] = "<absent>"
			continue
		}
		sum := sha256.Sum256(data)
		out[name] = fmt.Sprintf("%x", sum)
	}
	return out
}

// qaChangedFiles diffs two hash snapshots and returns the sorted names whose
// content changed (or appeared/vanished).
func qaChangedFiles(before, after map[string]string) []string {
	var changed []string
	for name, h := range before {
		if after[name] != h {
			changed = append(changed, name)
		}
	}
	for name := range after {
		if _, ok := before[name]; !ok {
			changed = append(changed, name)
		}
	}
	sort.Strings(changed)
	return changed
}

// qaDecoyUnchanged asserts the dagger.db decoy is byte-identical to the
// seeded content.
func qaDecoyUnchanged(t *testing.T, decoy string, want []byte) {
	t.Helper()
	got := qaReadBytes(t, decoy)
	if !bytes.Equal(got, want) {
		t.Fatalf("decoy dagger.db was modified — board tooling must never touch it:\nwant %q\ngot  %q", want, got)
	}
}

// qaTruncateFirstLine truncates the file's first line to a nonempty prefix
// (a row cut off mid-object: the corruption signature from the vacuous-cell
// incident, minus the wrong target). It asserts the premise — the mutated
// row is nonempty and NOT valid JSON — before writing it back.
func qaTruncateFirstLine(t *testing.T, path string) []byte {
	t.Helper()
	orig := qaReadBytes(t, path)
	idx := bytes.IndexByte(orig, '\n')
	if idx < 8 {
		t.Fatalf("%s first line too short to truncate meaningfully (%d bytes)", path, idx)
	}
	mutated := append([]byte{}, orig[:idx/2]...)
	mutated = append(mutated, '\n')
	line := bytes.TrimSpace(mutated)
	if len(line) == 0 {
		t.Fatalf("truncation of %s produced an empty row", path)
	}
	if json.Valid(line) {
		t.Fatalf("premise: truncation of %s produced VALID JSON %q — must be a truncated nonempty row", path, line)
	}
	if err := os.WriteFile(path, mutated, 0o644); err != nil {
		t.Fatalf("write corrupted %s: %v", path, err)
	}
	return mutated
}

// TestQACorruption: corrupt exactly ONE named board JSONL per case (tasks,
// events, header, fixtures) on a disposable board and hold the real CLI
// validate to its contract. Not parallel (in-process stdout capture); runs
// under -short.
func TestQACorruption(t *testing.T) {
	cases := []struct {
		name string
		file string
	}{
		{"tasks_jsonl_truncated_row", "tasks.jsonl"},
		{"events_jsonl_truncated_row", "events.jsonl"},
		{"header_board_jsonl_truncated_row", "board.jsonl"},
		{"fixtures_jsonl_truncated_row", "fixtures.jsonl"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := qaSeedValidBoard(t)
			bd := qaBoardDir(dir)
			target := filepath.Join(bd, tc.file)
			decoy := filepath.Join(dir, "dagger.db")
			decoyBefore := qaReadBytes(t, decoy)
			clean := qaBoardFileHashes(t, bd)

			// Corrupt exactly one named file; every other file must be
			// byte-identical after the mutation.
			orig := qaReadBytes(t, target)
			qaTruncateFirstLine(t, target)
			afterMut := qaBoardFileHashes(t, bd)
			if ch := qaChangedFiles(clean, afterMut); len(ch) != 1 || ch[0] != tc.file {
				t.Fatalf("mutation changed %v, want exactly [%s]", ch, tc.file)
			}

			// The real CLI must fail with the runtime-error exit code and
			// name the corrupted file in its diagnostic.
			code, out := qaRunCLI(t, dir, "validate")
			if code != 1 {
				t.Fatalf("validate with corrupted %s exit code = %d, want 1\noutput:\n%s", tc.file, code, out)
			}
			if !strings.Contains(out, tc.file) {
				t.Fatalf("diagnostic does not name the corrupted file %s\noutput:\n%s", tc.file, out)
			}
			if !strings.Contains(out, "RESULT: FAIL") {
				t.Fatalf("validate output missing RESULT: FAIL\noutput:\n%s", out)
			}
			if strings.Contains(out, "dagger.db") {
				t.Fatalf("validate mentioned the dagger.db decoy — board state is JSONL only\noutput:\n%s", out)
			}

			// Byte-preservation: validation performed ZERO writes after the
			// mutation — it never repairs the corrupted file and never
			// touches its siblings or the decoy.
			afterValidate := qaBoardFileHashes(t, bd)
			if ch := qaChangedFiles(afterMut, afterValidate); len(ch) != 0 {
				t.Fatalf("validate wrote to board files after mutation: %v", ch)
			}
			qaDecoyUnchanged(t, decoy, decoyBefore)

			// Restoration returns the board to a clean validating state:
			// the corruption is detected, not destroyed.
			if err := os.WriteFile(target, orig, 0o644); err != nil {
				t.Fatalf("restore %s: %v", target, err)
			}
			code, out = qaRunCLI(t, dir, "validate")
			if code != 0 {
				t.Fatalf("validate after restoring %s exit code = %d, want 0\noutput:\n%s", tc.file, code, out)
			}
			if ch := qaChangedFiles(clean, qaBoardFileHashes(t, bd)); len(ch) != 0 {
				t.Fatalf("board files differ from the clean baseline after restore: %v", ch)
			}
			qaDecoyUnchanged(t, decoy, decoyBefore)
		})
	}
}

// TestQACorruptionFixturesDetectedViaReader covers the fourth canonical file
// through the reader path: a truncated fixtures.jsonl must be detectable with
// a nonzero exit and a diagnostic naming the target file when a real read
// command hits it (`show` on a registry fixture id — tasks.jsonl misses, so
// the fixtures.jsonl read is exercised). The validate-side coverage for this
// file lives in the TestQACorruption table (BT-032 closed the silent-pass
// gap — see the file-header GAP CLOSED note); this test stays as
// complementary coverage of the reader path itself, which fails
// independently of validate.
func TestQACorruptionFixturesDetectedViaReader(t *testing.T) {
	dir := qaSeedValidBoard(t)
	bd := qaBoardDir(dir)
	target := filepath.Join(bd, "fixtures.jsonl")
	decoy := filepath.Join(dir, "dagger.db")
	decoyBefore := qaReadBytes(t, decoy)
	clean := qaBoardFileHashes(t, bd)

	// Premise: on the clean board the registry fixture resolves through
	// fixtures.jsonl (exit 0).
	code, out := qaRunCLI(t, dir, "show", qaRegistryFixtureID)
	if code != 0 {
		t.Fatalf("premise: show %s on clean board exit = %d, want 0\noutput:\n%s", qaRegistryFixtureID, code, out)
	}

	orig := qaReadBytes(t, target)
	qaTruncateFirstLine(t, target)
	afterMut := qaBoardFileHashes(t, bd)
	if ch := qaChangedFiles(clean, afterMut); len(ch) != 1 || ch[0] != "fixtures.jsonl" {
		t.Fatalf("mutation changed %v, want exactly [fixtures.jsonl]", ch)
	}

	// Detection through the reader: nonzero exit, target file named, no
	// decoy mention, no panic.
	code, out = qaRunCLI(t, dir, "show", qaRegistryFixtureID)
	if code != 1 {
		t.Fatalf("show with corrupted fixtures.jsonl exit code = %d, want 1\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "fixtures.jsonl") {
		t.Fatalf("diagnostic does not name the corrupted file fixtures.jsonl\noutput:\n%s", out)
	}
	if strings.Contains(out, "dagger.db") {
		t.Fatalf("show mentioned the dagger.db decoy\noutput:\n%s", out)
	}

	// Zero writes: the failed read must not modify anything on disk.
	if ch := qaChangedFiles(afterMut, qaBoardFileHashes(t, bd)); len(ch) != 0 {
		t.Fatalf("show wrote to board files after mutation: %v", ch)
	}
	qaDecoyUnchanged(t, decoy, decoyBefore)

	// Restoration: the registry fixture resolves again and the board is
	// byte-identical to the clean baseline.
	if err := os.WriteFile(target, orig, 0o644); err != nil {
		t.Fatalf("restore fixtures.jsonl: %v", err)
	}
	if code, out = qaRunCLI(t, dir, "show", qaRegistryFixtureID); code != 0 {
		t.Fatalf("show after restoring fixtures.jsonl exit code = %d, want 0\noutput:\n%s", code, out)
	}
	if ch := qaChangedFiles(clean, qaBoardFileHashes(t, bd)); len(ch) != 0 {
		t.Fatalf("board files differ from the clean baseline after restore: %v", ch)
	}
	qaDecoyUnchanged(t, decoy, decoyBefore)
}

// TestQACorruptionCleanBaseline: an uncorrupted board with a dagger.db decoy
// in the repo root validates green (exit 0, RESULT: OK), performs zero
// writes, and the decoy is ignored completely — neither read into the report
// nor modified.
func TestQACorruptionCleanBaseline(t *testing.T) {
	dir := qaSeedValidBoard(t)
	bd := qaBoardDir(dir)
	decoy := filepath.Join(dir, "dagger.db")
	decoyBefore := qaReadBytes(t, decoy)
	before := qaBoardFileHashes(t, bd)

	code, out := qaRunCLI(t, dir, "validate")
	if code != 0 {
		t.Fatalf("validate on clean board exit code = %d, want 0\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "RESULT: OK") {
		t.Fatalf("clean board validate output missing RESULT: OK\noutput:\n%s", out)
	}
	if strings.Contains(out, "dagger.db") {
		t.Fatalf("validate mentioned the dagger.db decoy on a clean board\noutput:\n%s", out)
	}
	if ch := qaChangedFiles(before, qaBoardFileHashes(t, bd)); len(ch) != 0 {
		t.Fatalf("validate wrote to a clean board: %v", ch)
	}
	qaDecoyUnchanged(t, decoy, decoyBefore)
}

// TestQACorruptionNoWriteAssertionDetectsWrites proves the byte-preservation
// assertion is load-bearing (not vacuous): a single appended byte to one
// board file — the shape a "repairing" validator would produce — is detected
// by the hash comparator, naming exactly the changed file. Runs on a
// disposable board in t.TempDir; the real repo is never touched.
func TestQACorruptionNoWriteAssertionDetectsWrites(t *testing.T) {
	dir := qaSeedValidBoard(t)
	bd := qaBoardDir(dir)
	before := qaBoardFileHashes(t, bd)

	tasksPath := filepath.Join(bd, "tasks.jsonl")
	f, err := os.OpenFile(tasksPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	after := qaBoardFileHashes(t, bd)
	changed := qaChangedFiles(before, after)
	if len(changed) != 1 || changed[0] != "tasks.jsonl" {
		t.Fatalf("hash comparator failed to detect a real write — changed %v, want [tasks.jsonl]; the no-write assertion would be vacuous", changed)
	}
}
