package main

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- REVIEW-BOARDCTL-008: `event --tick` no longer silently rewrites board.jsonl ----------
//
// The BT-014 behavior — a tick-bearing event auto-bumps the header
// ticks_total (max(current, tick)) — REWRITES board.jsonl as a side effect
// of appending to events.jsonl, and until this file it did so silently: no
// notice, no opt-out. The review row calls it an undocumented, unrequested
// write. The fix contract pinned here:
//
//   - DEFAULT: the rewrite still happens (BT-014 stands — doctor's drift
//     check depends on a fresh counter), but one notice line goes to
//     STDERR naming the rewritten file and the old->new ticks_total value.
//     The notice must NOT leak to stdout, which stays the plain
//     "event id N appended to ..." confirmation.
//   - OPT-OUT: `--no-tick-write` leaves board.jsonl byte-identical (the
//     event row still appends, still carries tick_number) and emits no
//     notice. The opt-out is per-invocation, not sticky.
//   - A no-op bump (tick <= current ticks_total) never rewrites and stays
//     silent — the notice fires only on an actual rewrite.
//   - `event --help` documents both facts (the flag and the rewrite).
//
// Three focused tests, one fresh temp board each, all through the real CLI
// dispatch (run()) — the repo's established test shape.

// rb008BoardDir returns the board dir the CLI resolves under a repo root
// (same shape bt041rBoardDir resolves; a private copy so this file's
// helpers never couple to another test file's).
func rb008BoardDir(repo string) string {
	return filepath.Join(repo, ".coding-hermes", "board")
}

// rb008NewBoard bootstraps a fresh board through the real cmdInit in a
// t.TempDir. A fresh board's header carries ticks_total 0, so any positive
// --tick exercises an actual rewrite.
func rb008NewBoard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "rb008-tick-write", "--namespace", "test"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	return dir
}

// rb008BoardFileHashes hashes every regular file in the board dir (the
// bt041r shape): a rewrite the opted-out path was supposed to skip fails
// the per-file comparison.
func rb008BoardFileHashes(t *testing.T, boardDir string) map[string][sha256.Size]byte {
	t.Helper()
	entries, err := os.ReadDir(boardDir)
	if err != nil {
		t.Fatalf("read board dir %s: %v", boardDir, err)
	}
	out := make(map[string][sha256.Size]byte, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(boardDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		out[e.Name()] = sha256.Sum256(data)
	}
	return out
}

// rb008HeaderLine returns line 1 of a board file (the header on both
// topologies).
func rb008HeaderLine(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		t.Fatalf("%s has no header line", path)
	}
	return lines[0]
}

// rb008HeaderTicksTotal parses the header line as JSON and returns its
// ticks_total (parsed, not substring-matched).
func rb008HeaderTicksTotal(t *testing.T, line string) int64 {
	t.Helper()
	var row map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &row); err != nil {
		t.Fatalf("header line is not valid JSON: %v\nline: %s", err, line)
	}
	raw, ok := row["ticks_total"]
	if !ok {
		t.Fatalf("header line has no ticks_total field: %s", line)
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err != nil {
		t.Fatalf("ticks_total is not a JSON number: %v\nline: %s", err, line)
	}
	return n
}

// rb008EventLines reads events.jsonl and returns its content lines.
func rb008EventLines(t *testing.T, eventsPath string) []string {
	t.Helper()
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read %s: %v", eventsPath, err)
	}
	s := strings.TrimRight(string(data), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// rb008NoticeMarker is the exact prefix the fix puts on every rewrite
// notice; asserting on it keeps the no-notice arms from passing vacuously
// (an empty stderr would satisfy a bare "does not contain X" check).
const rb008NoticeMarker = "boardctl: notice: rewriting"

// TestEventTickWriteNoticesAndRewrites: the DEFAULT path — `event --tick N`
// rewrites board.jsonl ticks_total AND announces the rewrite on stderr
// (file named, old->new value), with stdout staying the plain event
// confirmation. Control: a no-op bump (tick <= current) neither rewrites
// the header nor emits a notice.
func TestEventTickWriteNoticesAndRewrites(t *testing.T) {
	dir := rb008NewBoard(t)
	boardDir := rb008BoardDir(dir)
	boardPath := filepath.Join(boardDir, "board.jsonl")
	eventsPath := filepath.Join(boardDir, "events.jsonl")

	if before := rb008HeaderTicksTotal(t, rb008HeaderLine(t, boardPath)); before != 0 {
		t.Fatalf("fresh board ticks_total = %d, want 0 (test premise: the first --tick must be an actual rewrite)", before)
	}

	// ---- DEFAULT: --tick 52 on ticks_total 0 — rewrite + stderr notice.
	var stdout, stderr string
	var soErr, seErr error
	stderr, seErr = captureStderr(func() {
		stdout, soErr = captureStdout(func() {
			if code := run([]string{"-C", dir, "event", "--type", "tick", "--tick", "52"}); code != 0 {
				t.Errorf("event --tick exit code = %d, want 0 (default behavior still succeeds)", code)
			}
		})
	})
	if seErr != nil {
		t.Fatal(seErr)
	}
	if soErr != nil {
		t.Fatal(soErr)
	}

	// the rewrite happened
	header := rb008HeaderLine(t, boardPath)
	if got := rb008HeaderTicksTotal(t, header); got != 52 {
		t.Errorf("default --tick: board.jsonl ticks_total = %d, want 52 (BT-014 bump must keep working)", got)
	}

	// the notice went to stderr, naming the file and the old->new value
	if !strings.Contains(stderr, rb008NoticeMarker) {
		t.Errorf("default --tick stderr = %q, want a %q rewrite notice", stderr, rb008NoticeMarker)
	}
	if !strings.Contains(stderr, boardPath) {
		t.Errorf("default --tick stderr notice %q does not name the rewritten file %q", stderr, boardPath)
	}
	if !strings.Contains(stderr, "ticks_total") || !strings.Contains(stderr, "0 -> 52") {
		t.Errorf("default --tick stderr notice %q does not name ticks_total old->new (%q)", stderr, "0 -> 52")
	}
	// ...and did not leak to stdout
	if strings.Contains(stdout, rb008NoticeMarker) || strings.Contains(stdout, "0 -> 52") {
		t.Errorf("the ticks_total rewrite notice leaked to stdout (%q); it must go to stderr", stdout)
	}
	// stdout stays the plain confirmation
	if !strings.Contains(stdout, "event id 1 appended to") {
		t.Errorf("event stdout = %q, want the plain \"event id N appended to\" confirmation", stdout)
	}

	// the event row itself appended, carrying tick_number 52
	lines := rb008EventLines(t, eventsPath)
	if len(lines) != 1 {
		t.Fatalf("events.jsonl has %d line(s) after one tick event, want exactly 1", len(lines))
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
		t.Fatal(err)
	}
	if row["tick_number"] != float64(52) {
		t.Errorf("event tick_number = %v, want 52", row["tick_number"])
	}

	// ---- CONTROL: a no-op bump (5 <= 52) — no notice, header untouched.
	stderr2, seErr2 := captureStderr(func() {
		if code := run([]string{"-C", dir, "event", "--type", "tick", "--tick", "5"}); code != 0 {
			t.Errorf("event --tick 5 exit code = %d, want 0", code)
		}
	})
	if seErr2 != nil {
		t.Fatal(seErr2)
	}
	if strings.Contains(stderr2, rb008NoticeMarker) {
		t.Errorf("no-op tick (5 <= 52) emitted a rewrite notice: %q (the notice must fire only on an actual rewrite)", stderr2)
	}
	if after := rb008HeaderLine(t, boardPath); after != header {
		t.Errorf("no-op tick rewrote the header:\nbefore %s\nafter  %s", header, after)
	}
}

// TestEventNoTickWriteLeavesBoardUntouched: the OPT-OUT path —
// `event --tick N --no-tick-write` appends the event row (tick_number
// carried) but leaves board.jsonl byte-identical and emits no rewrite
// notice. The opt-out is per-invocation: a later default --tick still
// rewrites and still announces.
func TestEventNoTickWriteLeavesBoardUntouched(t *testing.T) {
	dir := rb008NewBoard(t)
	boardDir := rb008BoardDir(dir)
	boardPath := filepath.Join(boardDir, "board.jsonl")
	eventsPath := filepath.Join(boardDir, "events.jsonl")

	beforeHashes := rb008BoardFileHashes(t, boardDir)
	beforeHeader := rb008HeaderLine(t, boardPath)

	stderr, seErr := captureStderr(func() {
		if code := run([]string{"-C", dir, "event", "--type", "tick", "--tick", "7", "--no-tick-write"}); code != 0 {
			t.Errorf("event --tick 7 --no-tick-write exit code = %d, want 0 (the event row itself must still append)", code)
		}
	})
	if seErr != nil {
		t.Fatal(seErr)
	}

	// board.jsonl byte-identical: the opt-out must not rewrite the header
	// even though tick 7 > ticks_total 0.
	if after := rb008HeaderLine(t, boardPath); after != beforeHeader {
		t.Errorf("--no-tick-write rewrote board.jsonl line 1:\nbefore %s\nafter  %s", beforeHeader, after)
	}
	afterHashes := rb008BoardFileHashes(t, boardDir)
	if afterHashes[filepath.Base(boardPath)] != beforeHashes[filepath.Base(boardPath)] {
		t.Errorf("--no-tick-write modified board.jsonl (whole-file hash changed)")
	}
	if strings.Contains(stderr, rb008NoticeMarker) {
		t.Errorf("--no-tick-write emitted a rewrite notice: %q", stderr)
	}

	// the event row itself still appends, carrying tick_number 7
	lines := rb008EventLines(t, eventsPath)
	if len(lines) != 1 {
		t.Fatalf("events.jsonl has %d line(s) after one --no-tick-write event, want exactly 1", len(lines))
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
		t.Fatal(err)
	}
	if row["tick_number"] != float64(7) {
		t.Errorf("event tick_number = %v, want 7 (the opt-out must not drop the tick from the event row)", row["tick_number"])
	}

	// ---- per-invocation, not sticky: a later default --tick rewrites and
	// announces again (0 -> 9).
	stderr2, seErr2 := captureStderr(func() {
		if code := run([]string{"-C", dir, "event", "--type", "tick", "--tick", "9"}); code != 0 {
			t.Errorf("event --tick 9 (after an opted-out call) exit code = %d, want 0", code)
		}
	})
	if seErr2 != nil {
		t.Fatal(seErr2)
	}
	if got := rb008HeaderTicksTotal(t, rb008HeaderLine(t, boardPath)); got != 9 {
		t.Errorf("post-opt-out default --tick: ticks_total = %d, want 9 (the opt-out must not be sticky)", got)
	}
	if !strings.Contains(stderr2, rb008NoticeMarker) || !strings.Contains(stderr2, boardPath) || !strings.Contains(stderr2, "0 -> 9") {
		t.Errorf("post-opt-out default --tick stderr = %q, want the rewrite notice naming %s and 0 -> 9", stderr2, boardPath)
	}
}

// TestEventNoTickWriteHelpDocumentsOptOut: `event --help` documents both
// the --no-tick-write flag and the ticks_total rewrite it opts out of
// (the review row's "document in event --help usage text" requirement).
func TestEventNoTickWriteHelpDocumentsOptOut(t *testing.T) {
	out, err := captureStdout(func() {
		if code := run([]string{"event", "-h"}); code != 0 {
			t.Fatalf("event -h exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "--no-tick-write") {
		t.Errorf("event -h output missing --no-tick-write:\n%s", out)
	}
	if !strings.Contains(out, "ticks_total") {
		t.Errorf("event -h output does not mention the ticks_total rewrite the flag opts out of:\n%s", out)
	}
}
