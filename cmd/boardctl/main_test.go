package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStdout runs fn while capturing everything it writes to os.Stdout.
func captureStdout(fn func()) (string, error) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()
	w.Close()
	return <-done, nil
}

// TestVersionDefault verifies the unstamped default: a plain `go build`
// reports "dev" because the Makefile release target is what stamps
// main.version via -ldflags.
func TestVersionDefault(t *testing.T) {
	if version == "" {
		t.Fatal("version var must not be empty")
	}
	if version != "dev" {
		t.Fatalf("unstamped build should report %q, got %q", "dev", version)
	}
}

// TestCmdVersionOutput verifies cmdVersion prints the stamped version and
// exits cleanly (run() returns 0), matching the release stamping contract:
// -ldflags "-X main.version=$(VERSION)" must be observable on stdout.
func TestCmdVersionOutput(t *testing.T) {
	// Stamp through the same mechanism the Makefile uses, so the test
	// exercises the -X injection target itself.
	orig := version
	version = "20260903"
	t.Cleanup(func() { version = orig })

	got, err := captureStdout(func() {
		if code := run([]string{"version"}); code != 0 {
			t.Fatalf("run(version) exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	want := "boardctl version 20260903\n"
	if got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}

// TestCmdVersionRejectsArgs verifies the subcommand rejects positional args
// like every other boardctl subcommand.
func TestCmdVersionRejectsArgs(t *testing.T) {
	if err := cmdVersion([]string{"extra"}); err == nil {
		t.Fatal("cmdVersion with positional arg should error")
	} else if !strings.Contains(err.Error(), "takes no positional args") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// TestCmdInitRejectsArgs verifies init follows the same no-positional-args
// convention as the other subcommands.
func TestCmdInitRejectsArgs(t *testing.T) {
	if err := cmdInit(t.TempDir(), []string{"extra"}); err == nil {
		t.Fatal("cmdInit with positional arg should error")
	} else if !strings.Contains(err.Error(), "takes no positional args") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// TestCmdInitThenCreateSmoke: the fresh-user path end to end at the CLI
// layer — init on an empty dir, then `boardctl create` resolving the freshly
// seeded board — both exit 0.
func TestCmdInitThenCreateSmoke(t *testing.T) {
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "demo"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	// Re-init on the initialized board exits 0 with "already initialized".
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "init"}); code != 0 {
			t.Fatalf("re-init exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, "already initialized") {
		t.Fatalf("re-init output = %q, want an already-initialized note", got)
	}
	// create on the empty-but-initialized tasks.jsonl must succeed.
	got, err = captureStdout(func() {
		if code := run([]string{"-C", dir, "create", "--id", "T-1", "--title", "First"}); code != 0 {
			t.Fatalf("create exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, "created task T-1") {
		t.Fatalf("create output = %q, want a created-task note", got)
	}
}

// BT-006 exit-code contract: a normal command on a board exits 0; a command
// against a dir with NO board exits 2 (README: "2 usage/board-not-found"),
// not 1.
func TestCmdExitCodes(t *testing.T) {
	// 0: normal command against a seeded board.
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "demo"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "stats"}); code != 0 {
			t.Fatalf("stats exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, "total tasks: 0") {
		t.Fatalf("stats output = %q, want a zero-count summary", got)
	}

	// 2: board-not-found via stats.
	if code := run([]string{"-C", t.TempDir(), "stats"}); code != 2 {
		t.Fatalf("stats on boardless dir exit code = %d, want 2", code)
	}

	// 2: board-not-found via create (the wrapped openBoard error path).
	if code := run([]string{"-C", t.TempDir(), "create", "--id", "X-1", "--title", "X"}); code != 2 {
		t.Fatalf("create on boardless dir exit code = %d, want 2", code)
	}

	// 2: unknown command (unchanged).
	if code := run([]string{"-C", dir, "nope"}); code != 2 {
		t.Fatalf("unknown command exit code = %d, want 2", code)
	}
}

// BT-006 end-to-end user path: init a fresh repo, then run stats through the
// .coding-hermes dir — the exact -C form that used to fail.
func TestCmdInitThenStatsViaCodingHermesDir(t *testing.T) {
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "demo"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	got, err := captureStdout(func() {
		if code := run([]string{"-C", filepath.Join(dir, ".coding-hermes"), "stats"}); code != 0 {
			t.Fatalf("stats -C <repo>/.coding-hermes exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, "total tasks: 0") {
		t.Fatalf("stats output = %q, want a zero-count summary", got)
	}
}

// ---------- BT-010: topology-B boards are writable (CLI level) ----------

// seedCLITopologyBBoard writes a minimal topology-B board under dir (header
// on line 1 of tasks.jsonl, no board.jsonl) and returns the -C target.
func seedCLITopologyBBoard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"tasks.jsonl": `{"project":"legacy","namespace":"legacy","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n" +
			`{"id":"EXIST-1","title":"Existing","status":"pending","priority":"P2"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(boardDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestCmdTopologyBFullWorkflow walks the four write subcommands (create,
// update --commit-hash, event, header read/set) plus validate and doctor
// against the SAME hand-made topology-B board, asserting exit codes and the
// on-disk line-1 invariants at each step.
func TestCmdTopologyBFullWorkflow(t *testing.T) {
	dir := seedCLITopologyBBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	tasksPath := filepath.Join(boardDir, "tasks.jsonl")
	hdrBefore := `{"project":"legacy","namespace":"legacy","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":null}`

	// AC1: create appends a task and leaves line 1 (header) intact.
	if code := run([]string{"-C", dir, "create", "--id", "NEW-1", "--title", "Fresh"}); code != 0 {
		t.Fatalf("create on topology B exit code = %d, want 0", code)
	}
	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitN(strings.TrimRight(string(raw), "\n"), "\n", 3)
	if lines[0] != hdrBefore {
		t.Fatalf("create mutated line 1:\n got %s\nwant %s", lines[0], hdrBefore)
	}
	if len(lines) != 3 || !strings.Contains(lines[2], `"NEW-1"`) || strings.Contains(lines[2], `"project"`) {
		t.Fatalf("create appended wrong rows: %q", lines)
	}

	// AC3: event appends to events.jsonl. (create already appended its
	// README-promised task_created event, so the count goes 1 -> 2 -> 3.)
	if code := run([]string{"-C", dir, "event", "--type", "audit", "--task-id", "NEW-1"}); code != 0 {
		t.Fatalf("event on topology B exit code = %d, want 0", code)
	}
	eventsRaw, err := os.ReadFile(filepath.Join(boardDir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	eventRows := strings.Split(strings.TrimRight(string(eventsRaw), "\n"), "\n")
	if len(eventRows) != 3 {
		t.Fatalf("events.jsonl has %d rows, want 3 (seed + task_created + audit)", len(eventRows))
	}
	if !strings.Contains(eventRows[1], `"task_created"`) || !strings.Contains(eventRows[2], `"audit"`) {
		t.Fatalf("event rows wrong: %q", eventRows)
	}

	// AC2: update flips the row and bumps last_commit IN LINE 1.
	if code := run([]string{"-C", dir, "update", "NEW-1", "--status", "complete", "--commit-hash", "abc1234"}); code != 0 {
		t.Fatalf("update on topology B exit code = %d, want 0", code)
	}
	raw, err = os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	lines = strings.SplitN(strings.TrimRight(string(raw), "\n"), "\n", 3)
	if !strings.Contains(lines[0], `"last_commit":"abc1234"`) {
		t.Fatalf("update did not bump last_commit in line 1: %s", lines[0])
	}
	if lines[0] == hdrBefore {
		t.Fatal("line 1 unchanged — last_commit bump missing")
	}
	if !strings.Contains(lines[2], `"status":"complete"`) {
		t.Fatalf("update did not flip the task row: %s", lines[2])
	}

	// AC4a: header --json prints the line-1 header (ticks_total 1, project
	// legacy) — not a topology note.
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "header", "--json"}); code != 0 {
			t.Fatalf("header --json on topology B exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, `"project":"legacy"`) || !strings.Contains(got, `"ticks_total":1`) {
		t.Fatalf("header --json output = %q, want the line-1 header object", got)
	}

	// AC4b: header --set-ticks-total rewrites ONLY line 1.
	if code := run([]string{"-C", dir, "header", "--set-ticks-total", "7"}); code != 0 {
		t.Fatalf("header --set-ticks-total on topology B exit code = %d, want 0", code)
	}
	raw, err = os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	lines = strings.SplitN(strings.TrimRight(string(raw), "\n"), "\n", 3)
	if !strings.Contains(lines[0], `"ticks_total":7`) {
		t.Fatalf("line 1 ticks_total not bumped: %s", lines[0])
	}
	if !strings.Contains(lines[0], `"last_commit":"abc1234"`) {
		t.Fatalf("line 1 lost last_commit: %s", lines[0])
	}
	if !strings.Contains(lines[2], `"status":"complete"`) {
		t.Fatalf("task row mutated by header --set: %s", lines[2])
	}

	// AC5a: validate runs the header checks on the line-1 header — clean
	// board, no "not checked" warn.
	got, err = captureStdout(func() {
		if code := run([]string{"-C", dir, "validate"}); code != 0 {
			t.Fatalf("validate on topology B exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if strings.Contains(got, "not checked") || strings.Contains(got, "RESULT: FAIL") {
		t.Fatalf("validate output unexpected:\n%s", got)
	}
}

// AC5b: a negative ticks_total in the topology-B line-1 header FAILS
// validate (the counter checks must run, not be downgraded).
func TestCmdValidateTopologyBFlagsNegativeCounter(t *testing.T) {
	dir := seedCLITopologyBBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	tasksPath := filepath.Join(boardDir, "tasks.jsonl")
	bad := strings.Replace(
		`{"project":"legacy","namespace":"legacy","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":null}`,
		`"ticks_total":1`, `"ticks_total":-5`, 1)
	if err := os.WriteFile(tasksPath, []byte(bad+"\n"+
		`{"id":"EXIST-1","title":"Existing","status":"pending","priority":"P2"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "validate"}); code != 1 {
			t.Fatalf("validate with negative ticks_total exit code = %d, want 1", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, "negative") || !strings.Contains(got, "ticks_total") {
		t.Fatalf("validate report missing the negative-counter error:\n%s", got)
	}
}

// doctor on topology B compares the line-1 header counters vs the events
// stream (counter drift is an error; the "not checked" warn is gone).
func TestCmdDoctorTopologyBChecksHeaderVsEvents(t *testing.T) {
	dir := seedCLITopologyBBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	tasksPath := filepath.Join(boardDir, "tasks.jsonl")
	// ticks_total 0 with a tick_number 1 event -> drift.
	stale := strings.Replace(
		`{"project":"legacy","namespace":"legacy","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":null}`,
		`"ticks_total":1`, `"ticks_total":0`, 1)
	if err := os.WriteFile(tasksPath, []byte(stale+"\n"+
		`{"id":"EXIST-1","title":"Existing","status":"pending","priority":"P2"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "doctor"}); code != 1 {
			t.Fatalf("doctor with stale ticks_total exit code = %d, want 1", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, "header ticks_total 0 < max events tick_number 1") {
		t.Fatalf("doctor report missing the drift error:\n%s", got)
	}
	// BT-013: the remediation hint must print right under the drift error.
	if !strings.Contains(got, "fix: boardctl header --set-ticks-total=1") {
		t.Fatalf("doctor report missing the remediation hint:\n%s", got)
	}
	if strings.Contains(got, "not checked") {
		t.Fatalf("doctor still emits the topology-B 'not checked' warn:\n%s", got)
	}
}

// init on a topology-B board still refuses (fresh boards only), with the
// writable-era wording, and writes nothing.
func TestCmdInitOnTopologyBStillRefuses(t *testing.T) {
	dir := seedCLITopologyBBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := cmdInit(dir, nil); err == nil {
		t.Fatal("init on topology-B board accepted")
	} else if !strings.Contains(err.Error(), "topology B") || strings.Contains(err.Error(), "read-only") {
		t.Fatalf("init refusal wording wrong: %v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "board.jsonl")); !os.IsNotExist(err) {
		t.Fatal("board.jsonl written despite init refusal")
	}
}
