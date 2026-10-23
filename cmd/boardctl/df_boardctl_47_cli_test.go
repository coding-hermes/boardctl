package main

// DF-BOARDCTL-47 CLI regressions: the umbrella view's line-level read modes.
//
// The measured failure (dogfood run 25) was an umbrella over boardctl+warpfs
// where ONE duplicate-key line in the second child board aborted the whole
// view: the census printed the first board, then died on
// `board warpfs (<dir>): <tasks.jsonl>: line 412: duplicate key "worker_status"
// in row`, exit 1 — no per-repo stats, no grand total, no list. These tests
// build the same shape on a temp umbrella (second child = childB) and pin both
// modes:
//
//   - default: fail-closed, unchanged message, no partial output
//   - --skip-bad-lines: every parseable row of every healthy board is still
//     counted/listed, each skipped line is reported on stderr as a
//     SKIPPED-LINES block naming the board and file:line, exit 1
//
// A duplicate-key line is the real shape: it IS valid JSON, so only the
// JSONL row parser rejects it (the premise is asserted, not assumed).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-hermes/boardctl/internal/board"
)

// df47BadLine is the duplicate-key row the parser refuses (ParseRow:
// `duplicate key "worker_status" in row`). json.Valid ACCEPTS it — that
// premise is asserted by the fixture so a future decoder change cannot make
// these tests silently vacuous.
const df47BadLine = `{"id": "DF47-BAD", "title": "duplicate worker_status", "status": "pending", "priority": "P2", "worker_status": "complete", "worker_status": "review"}`

// df47AppendBadLine appends df47BadLine to repo's tasks.jsonl and returns its
// 1-based physical line number. It asserts the line is valid JSON (so the
// parser, not the JSON validator, is the rejecting side) and that ParseRow
// really refuses it with the duplicate-key diagnosis.
func df47AppendBadLine(t *testing.T, repo string) int {
	t.Helper()
	if !json.Valid([]byte(df47BadLine)) {
		t.Fatalf("premise broken: %q is not valid JSON, so this fixture would not exercise the duplicate-key class", df47BadLine)
	}
	if _, err := board.ParseRow([]byte(df47BadLine)); err == nil {
		t.Fatal("premise broken: ParseRow accepted a duplicate-key line")
	} else if !strings.Contains(err.Error(), `duplicate key "worker_status"`) {
		t.Fatalf("premise broken: ParseRow refused the line for another reason: %v", err)
	}
	path := filepath.Join(repo, ".coding-hermes", "board", "tasks.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(df47BadLine + "\n")); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "\n")
}

// df47Fixture builds the dogfood shape: umbrella + childA (healthy, 2 tasks) +
// childB (1 healthy task, then one duplicate-key line). The bad line is in the
// SECOND child, matching the live run.
func df47Fixture(t *testing.T) (umbrella, childA, childB string, badLine int) {
	t.Helper()
	umbrella, childA, childB = bt078Fixture(t)
	bt078Create(t, childA, "CHILDA-1", "childA one")
	bt078Create(t, childA, "CHILDA-2", "childA two")
	bt078Create(t, childB, "CHILDB-1", "childB one")
	badLine = df47AppendBadLine(t, childB)
	return umbrella, childA, childB, badLine
}

// df47AssertSkipBlock pins the SKIPPED-LINES evidence block on STDERR: the
// count, the repo qualifier, the board dir, the file + 1-based line and the
// parse error.
func df47AssertSkipBlock(t *testing.T, stderr, childB, badLine string) {
	t.Helper()
	tasksPath := filepath.Join(childB, ".coding-hermes", "board", "tasks.jsonl")
	for _, want := range []string{
		"SKIPPED-LINES: 1",
		"childB",
		filepath.Join(childB, ".coding-hermes", "board"),
		tasksPath,
		"line " + badLine,
		`duplicate key "worker_status"`,
		"DF47-BAD",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("SKIPPED-LINES block on stderr missing %q:\n%s", want, stderr)
		}
	}
}

// df47RunSplit drives run() with stdout and stderr captured SEPARATELY. The
// shared bt078Run concatenates both streams, which cannot distinguish a listed
// row from skip evidence quoting the same bytes.
func df47RunSplit(t *testing.T, args []string) (stdout, stderr string, code int) {
	t.Helper()
	var cerr error
	stdout, err := captureStdout(func() {
		stderr, cerr = captureStderrString(func() { code = run(args) })
	})
	if err != nil {
		t.Fatal(err)
	}
	if cerr != nil {
		t.Fatal(cerr)
	}
	return stdout, stderr, code
}

// AC3: without the flag the fail-closed behaviour AND message are unchanged —
// the first unparseable line aborts the whole view, exit 1, no partial stats.
func TestDF47DefaultFailClosedUnchanged(t *testing.T) {
	umbrella, childA, childB, badLine := df47Fixture(t)
	_ = childA

	stdout, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--stats"})
	if code != 1 {
		t.Fatalf("default umbrella --stats exit = %d, want 1 (fail-closed): %s", code, stdout)
	}
	// No partial output: the view aborted before the stats body.
	if strings.Contains(stdout, "umbrella total:") || strings.Contains(stdout, "repo childA") {
		t.Fatalf("default mode emitted partial stats before aborting:\n%s", stdout)
	}
	tasksPath := filepath.Join(childB, ".coding-hermes", "board", "tasks.jsonl")
	for _, want := range []string{
		"board childB",
		tasksPath,
		"line " + fmt.Sprintf("%d", badLine),
		`duplicate key "worker_status"`,
	} {
		if !strings.Contains(bt078LastStderr, want) {
			t.Fatalf("default abort message missing %q (unchanged-message contract):\n%s", want, bt078LastStderr)
		}
	}
	// The degraded clause must NOT leak into the default path.
	if strings.Contains(bt078LastStderr, "SKIPPED-LINES") || strings.Contains(bt078LastStderr, "skipped line(s)") {
		t.Fatalf("default mode reported degrade-mode evidence: %s", bt078LastStderr)
	}
}

// AC1 + AC2: --stats --skip-bad-lines counts the parseable rows of EVERY
// healthy board (both boards' per-repo stats and the grand total), reports the
// skipped board/line on stderr, and exits 1.
func TestDF47StatsSkipBadDegrades(t *testing.T) {
	umbrella, childA, childB, badLine := df47Fixture(t)
	_ = childA

	stdout, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--stats", "--skip-bad-lines"})
	if code != 1 {
		t.Fatalf("umbrella --stats --skip-bad-lines exit = %d, want 1 (degraded data must not read as success): %s", code, stdout)
	}
	for _, want := range []string{
		"repo childA (2 tasks):",
		"repo childB (1 tasks):",
		"umbrella total:",
		"total tasks: 3", // childA 2 + childB 1 — the healthy rows of the DAMAGED board still count
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("degraded stats missing %q:\n%s", want, stdout)
		}
	}
	df47AssertSkipBlock(t, bt078LastStderr, childB, fmt.Sprintf("%d", badLine))
	if !strings.Contains(bt078LastStderr, "1 skipped line(s)") {
		t.Fatalf("degraded exit message does not name the skipped-line count:\n%s", bt078LastStderr)
	}
}

// AC2: --list --skip-bad-lines prints the healthy rows of both boards plus the
// SKIPPED block instead of aborting.
func TestDF47ListSkipBadDegrades(t *testing.T) {
	umbrella, _, childB, badLine := df47Fixture(t)

	stdout, stderr, code := df47RunSplit(t, []string{"-C", umbrella, "umbrella", "--list", "--skip-bad-lines"})
	if code != 1 {
		t.Fatalf("umbrella --list --skip-bad-lines exit = %d, want 1: %s", code, stdout+stderr)
	}
	if !strings.Contains(stdout, "REPO") {
		t.Fatalf("degraded list lost its header: %s", stdout)
	}
	for _, want := range []string{"CHILDA-1", "CHILDA-2", "CHILDB-1"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("degraded list missing healthy row %q:\n%s", want, stdout)
		}
	}
	// The unparseable line is NOT a row: it may appear in stderr evidence
	// only.
	if strings.Contains(stdout, "DF47-BAD") {
		t.Fatalf("the unparseable line surfaced as a listed row: %s", stdout)
	}
	df47AssertSkipBlock(t, stderr, childB, fmt.Sprintf("%d", badLine))
}

// The census (default mode) degrades the same way: every board still gets its
// line, the exit code still says degraded.
func TestDF47CensusSkipBadDegrades(t *testing.T) {
	umbrella, _, childB, badLine := df47Fixture(t)

	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--skip-bad-lines"})
	if code != 1 {
		t.Fatalf("umbrella --skip-bad-lines exit = %d, want 1: %s", code, out)
	}
	if got := strings.Count(out, "task(s)"); got != 3 {
		t.Fatalf("degraded census shows %d board rows, want 3: %s", got, out)
	}
	df47AssertSkipBlock(t, bt078LastStderr, childB, fmt.Sprintf("%d", badLine))
}

// The SKIPPED block rides stderr, so a degraded --json read still emits
// parseable JSON on stdout (machine consumers keep working) and exits 1.
func TestDF47JSONSkipBadKeepsStdoutParseable(t *testing.T) {
	umbrella, _, childB, badLine := df47Fixture(t)

	stdout, stderr, code := df47RunSplit(t, []string{"-C", umbrella, "umbrella", "--json", "--skip-bad-lines"})
	if code != 1 {
		t.Fatalf("umbrella --json --skip-bad-lines exit = %d, want 1", code)
	}
	var payload struct {
		Boards []struct {
			Repo  string `json:"repo"`
			Tasks int    `json:"tasks"`
		} `json:"boards"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		t.Fatalf("degraded --json stdout is not parseable JSON: %v\n%s", err, stdout)
	}
	if len(payload.Boards) != 3 {
		t.Fatalf("degraded --json boards = %d, want 3: %+v", len(payload.Boards), payload.Boards)
	}
	// Raw row census: umbrella 1 (NEVER-DONE), childA 3, childB 2 — the bad
	// line is not a row, so the total is the same as a healthy read of the
	// salvageable content.
	if payload.Total != 6 {
		t.Fatalf("degraded --json total = %d, want 6", payload.Total)
	}
	df47AssertSkipBlock(t, stderr, childB, fmt.Sprintf("%d", badLine))
}

// AC4: --help documents BOTH modes (fail-closed default and the degrade flag).
func TestDF47HelpDocumentsBothModes(t *testing.T) {
	umbrella, _, _, _ := df47Fixture(t)

	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--help"})
	if code != 0 {
		t.Fatalf("umbrella --help exit = %d, want 0: %s", code, out)
	}
	for _, want := range []string{"--skip-bad-lines", "fail-closed", "SKIPPED-LINES", "aborts the whole view"} {
		if !strings.Contains(out, want) {
			t.Fatalf("umbrella --help does not document %q:\n%s", want, out)
		}
	}
}

// The flag on a HEALTHY umbrella is not a false degradation: exit 0, no
// SKIPPED-LINES block. (A degrade mode that always degrades would be useless.)
func TestDF47SkipBadOnHealthyUmbrellaStaysClean(t *testing.T) {
	umbrella, childA, childB := bt078Fixture(t)
	bt078Create(t, childA, "CHILDA-1", "childA one")
	bt078Create(t, childB, "CHILDB-1", "childB one")

	for _, mode := range [][]string{{"umbrella", "--skip-bad-lines"}, {"umbrella", "--stats", "--skip-bad-lines"}, {"umbrella", "--list", "--skip-bad-lines"}} {
		args := append([]string{"-C", umbrella}, mode...)
		out, code := bt078Run(t, args)
		if code != 0 {
			t.Fatalf("%v on a healthy umbrella exit = %d, want 0: %s", mode, code, out)
		}
		if strings.Contains(bt078LastStderr, "SKIPPED-LINES") {
			t.Fatalf("%v invented skip evidence on a healthy umbrella: %s", mode, bt078LastStderr)
		}
	}
}

// A degraded read stays read-only: the flag must not repair, rewrite or touch
// any board file (same AC5 posture as the rest of the umbrella surface).
func TestDF47SkipBadIsReadOnly(t *testing.T) {
	umbrella, _, _, _ := df47Fixture(t)
	before := bt078AllBoardHashes(t, umbrella)
	for _, mode := range [][]string{
		{"umbrella", "--skip-bad-lines"},
		{"umbrella", "--list", "--skip-bad-lines"},
		{"umbrella", "--stats", "--skip-bad-lines"},
		{"umbrella", "--validate", "--skip-bad-lines"},
		{"umbrella", "--json", "--skip-bad-lines"},
	} {
		args := append([]string{"-C", umbrella}, mode...)
		if _, code := bt078Run(t, args); code != 1 {
			t.Fatalf("%v exit = %d, want 1 (degraded)", mode, code)
		}
	}
	after := bt078AllBoardHashes(t, umbrella)
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("file %s changed through a degraded umbrella read: before %s after %s", k, v, after[k])
		}
	}
}
