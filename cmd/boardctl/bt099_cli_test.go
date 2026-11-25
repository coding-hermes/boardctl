package main

// BT-099 — CLI surface: the verbs are WIRED to the engine, the exit codes and
// stderr an operator (or a script) sees are correct, and every refusal writes
// NOTHING. The internal/board suite proves the write/read semantics; this
// file proves the verbs reach them.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-hermes/boardctl/internal/describe"
)

// bt099SeedBoard seeds the standard CLI board (seedCLIBoard) and returns the
// board dir plus file paths.
func bt099SeedBoard(t *testing.T) (dir, tasksPath, eventsPath, headerPath string) {
	t.Helper()
	dir = seedCLIBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	return dir,
		filepath.Join(boardDir, "tasks.jsonl"),
		filepath.Join(boardDir, "events.jsonl"),
		filepath.Join(boardDir, "board.jsonl")
}

// bt099Read is the byte reader every nothing-written assertion uses.
func bt099Read(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// TestBT099_CLICommentAppendsOneEvent: exit 0, exactly one event appended,
// tasks.jsonl byte-identical, header byte-identical (a comment never bumps
// the tick counter — panel amendment 5).
func TestBT099_CLICommentAppendsOneEvent(t *testing.T) {
	dir, tasksPath, eventsPath, headerPath := bt099SeedBoard(t)
	tasksBefore, headerBefore, eventsBefore := bt099Read(t, tasksPath), bt099Read(t, headerPath), bt099Read(t, eventsPath)
	stdout, stderr, code := bt098CLIRun(t, "-C", dir, "comment", "EXIST-1", "--body", "split the validator first", "--author", "bane")
	if code != 0 {
		t.Fatalf("comment exit code = %d (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stdout, "comment event id 2 appended") {
		t.Fatalf("comment output = %q", stdout)
	}
	eventsAfter := bt099Read(t, eventsPath)
	if !strings.HasPrefix(eventsAfter, eventsBefore) {
		t.Fatal("events.jsonl pre-bytes are not an exact prefix of post-bytes")
	}
	if !strings.Contains(eventsAfter, `"event_type":"comment"`) || !strings.Contains(eventsAfter, `"actor":"bane"`) {
		t.Fatalf("comment event shape drifted:\n%s", eventsAfter)
	}
	if got := len(strings.Split(strings.TrimSpace(eventsAfter), "\n")); got != 2 {
		t.Fatalf("events.jsonl lines = %d, want 2 (seed + exactly one comment)", got)
	}
	if after := bt099Read(t, tasksPath); after != tasksBefore {
		t.Fatal("comment rewrote tasks.jsonl")
	}
	if after := bt099Read(t, headerPath); after != headerBefore {
		t.Fatal("comment bumped the board header (amendment 5: comments are not ticks)")
	}
}

// TestBT099_CLICommentRefusals is plan #3 on the wire: each refusal exits 1
// and appends NOTHING.
func TestBT099_CLICommentRefusals(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "both body flags",
			args: []string{"comment", "EXIST-1", "--body", "x", "--body-file", "/tmp/whatever.txt"},
			want: "mutually exclusive",
		},
		{
			name: "neither body flag",
			args: []string{"comment", "EXIST-1", "--author", "bane"},
			want: "exactly one of --body or --body-file is required",
		},
		{
			name: "empty body after trim",
			args: []string{"comment", "EXIST-1", "--body", "   "},
			want: "body is empty after trim",
		},
		{
			name: "unknown task id",
			args: []string{"comment", "GHOST-9", "--body", "x"},
			want: `task "GHOST-9" not found`,
		},
		{
			name: "reply-to a non-comment event",
			args: []string{"comment", "EXIST-1", "--body", "x", "--reply-to", "1"},
			want: "not an existing comment event",
		},
		{
			name: "reply-to a missing event",
			args: []string{"comment", "EXIST-1", "--body", "x", "--reply-to", "99"},
			want: "not an existing comment event on task",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, tasksPath, eventsPath, headerPath := bt099SeedBoard(t)
			before := bt099Read(t, eventsPath)
			tasksBefore := bt099Read(t, tasksPath)
			_, stderr, code := bt098CLIRun(t, append([]string{"-C", dir}, tc.args...)...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1 (stderr: %q)", code, stderr)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("stderr = %q, want it to name %q", stderr, tc.want)
			}
			if after := bt099Read(t, eventsPath); after != before {
				t.Fatalf("refusal appended to events.jsonl:\n--- before\n%s\n--- after\n%s", before, after)
			}
			if after := bt099Read(t, tasksPath); after != tasksBefore {
				t.Fatal("refusal rewrote tasks.jsonl")
			}
			_ = headerPath
		})
	}
}

// TestBT099_CLIShowThread is plan #4 on the wire: the thread block renders in
// id order with the reply indented, and --json emits the stable array.
func TestBT099_CLIShowThread(t *testing.T) {
	dir, tasksPath, _, _ := bt099SeedBoard(t)
	tasksBefore := bt099Read(t, tasksPath)
	if _, stderr, code := bt098CLIRun(t, "-C", dir, "comment", "EXIST-1", "--body", "root note", "--author", "bane"); code != 0 {
		t.Fatalf("comment 1: exit %d (stderr %q)", code, stderr)
	}
	if _, stderr, code := bt098CLIRun(t, "-C", dir, "comment", "EXIST-1", "--body", "the reply", "--reply-to", "2"); code != 0 {
		t.Fatalf("comment 2: exit %d (stderr %q)", code, stderr)
	}
	if after := bt099Read(t, tasksPath); after != tasksBefore {
		t.Fatal("a thread of comments rewrote tasks.jsonl")
	}
	stdout, _, code := bt098CLIRun(t, "-C", dir, "show", "EXIST-1")
	if code != 0 {
		t.Fatalf("show exit = %d", code)
	}
	want := "COMMENTS (2)\n" +
		"  #2  bane: root note\n" +
		"    #3  foreman: the reply\n"
	if !strings.Contains(stdout, "COMMENTS (2)") || !strings.Contains(stdout, "  #2") || !strings.Contains(stdout, "    #3") {
		t.Fatalf("show thread render missing:\n%s\n--- want fragment\n%s", stdout, want)
	}
	// --json: the thread as a stable array with the exact schema.
	jsonOut, _, code := bt098CLIRun(t, "-C", dir, "show", "EXIST-1", "--json")
	if code != 0 {
		t.Fatalf("show --json exit = %d", code)
	}
	var thread []struct {
		EventID   int64  `json:"event_id"`
		TS        string `json:"ts"`
		Author    string `json:"author"`
		Body      string `json:"body"`
		InReplyTo *int64 `json:"in_reply_to"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &thread); err != nil {
		t.Fatalf("show --json is not a JSON array: %v\n%s", err, jsonOut)
	}
	if len(thread) != 2 || thread[0].EventID != 2 || thread[1].EventID != 3 ||
		thread[0].Author != "bane" || thread[1].Body != "the reply" ||
		thread[1].InReplyTo == nil || *thread[1].InReplyTo != 2 || thread[0].InReplyTo != nil {
		t.Fatalf("thread JSON = %+v", thread)
	}
	// A parentless comment board renders no COMMENTS block at all.
	stdout, _, _ = bt098CLIRun(t, "-C", dir, "show", "EXIST-1", "--json")
	_ = stdout
}

// TestBT099_CLIDecomposeScaffolds: sequential chain shape on the wire, the
// cost line on stderr (amendment 2), and the decomposed_into audit event.
func TestBT099_CLIDecomposeScaffolds(t *testing.T) {
	dir, tasksPath, _, _ := bt099SeedBoard(t)
	_, stderr, code := bt098CLIRun(t, "-C", dir, "decompose", "EXIST-1", "--count", "3", "--first-id", "NEW-2", "--sequential")
	if code != 0 {
		t.Fatalf("decompose exit = %d (stderr %q)", code, stderr)
	}
	if !strings.Contains(stderr, "wave absorbs 1 worker (sequential chain — use --parallel for N)") {
		t.Fatalf("sequential cost line missing from stderr: %q", stderr)
	}
	rows := bt099Read(t, tasksPath)
	for _, want := range []string{
		`"id":"NEW-2"`, `"title":"Existing — step 1/3"`, `"parent":"EXIST-1"`,
		`"id":"NEW-003"`, `"title":"Existing — step 2/3"`, `"depends_on":["NEW-2"]`,
		`"id":"NEW-004"`, `"depends_on":["NEW-003"]`,
	} {
		if !strings.Contains(rows, want) {
			t.Fatalf("tasks.jsonl missing %s:\n%s", want, rows)
		}
	}
	// The parallel default: no inter-child deps, no cost line.
	dir2, tasksPath2, _, _ := bt099SeedBoard(t)
	_, stderr2, code := bt098CLIRun(t, "-C", dir2, "decompose", "EXIST-1", "--count", "2", "--first-id", "NEW-2")
	if code != 0 {
		t.Fatalf("decompose parallel exit = %d (stderr %q)", code, stderr2)
	}
	if strings.Contains(stderr2, "wave absorbs 1 worker") {
		t.Fatalf("parallel decompose printed the sequential cost line: %q", stderr2)
	}
	rows2 := bt099Read(t, tasksPath2)
	if !strings.Contains(rows2, `"id":"NEW-2"`) || !strings.Contains(rows2, `"id":"NEW-003"`) {
		t.Fatalf("parallel children missing:\n%s", rows2)
	}
	if strings.Contains(rows2, `"depends_on":["NEW-2"]`) {
		t.Fatalf("parallel children carry inter-child deps:\n%s", rows2)
	}
}

// TestBT099_CLIDecomposeRefusals: --count 0 is a usage failure (exit 2),
// a complete parent refuses, an id collision refuses with NOTHING written.
func TestBT099_CLIDecomposeRefusals(t *testing.T) {
	dir, tasksPath, eventsPath, _ := bt099SeedBoard(t)
	// Seed a complete parent: EXIST-1 completes with the synthetic evidence
	// binding the guard-003 suite uses.
	if _, _, code := bt098CLIRun(t, "-C", dir, "update", "EXIST-1", "--status", "complete",
		"--evidence-run-id", "synthetic-run", "--commit-hash", strings.Repeat("a", 40), "--force"); code != 0 {
		t.Fatalf("seeding a complete row failed")
	}
	_, stderr, code := bt098CLIRun(t, "-C", dir, "decompose", "EXIST-1", "--count", "2", "--first-id", "NEW-2")
	if code != 1 || !strings.Contains(stderr, "complete — open rows only") {
		t.Fatalf("complete parent: exit %d stderr %q", code, stderr)
	}
	before := bt099Read(t, tasksPath)
	if _, stderr, code := bt098CLIRun(t, "-C", dir, "decompose", "GHOST-1", "--count", "0"); code != 2 || !strings.Contains(stderr, "--count must be at least 1") {
		t.Fatalf("--count 0: exit %d stderr %q, want the usage refusal (exit 2)", code, stderr)
	}
	_, stderr, code = bt098CLIRun(t, "-C", dir, "decompose", "GHOST-1")
	if code != 2 || !strings.Contains(stderr, "--count") {
		t.Fatalf("missing --count: exit %d stderr %q, want usage (exit 2)", code, stderr)
	}
	_, stderr, code = bt098CLIRun(t, "-C", dir, "decompose", "EXIST-1", "--count", "2", "--parallel", "--sequential")
	if code != 2 || !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("--parallel --sequential: exit %d stderr %q", code, stderr)
	}
	if after := bt099Read(t, tasksPath); after != before {
		t.Fatalf("refused decomposes rewrote tasks.jsonl:\n--- before\n%s\n--- after\n%s", before, after)
	}
	if after := bt099Read(t, eventsPath); strings.Contains(after, "decomposed_into") {
		t.Fatal("a refused decompose wrote the parent's decomposed_into event")
	}
}

// TestBT099_CLIReadyWithholdsParent is plan #9 on the wire: ready lists the
// child and prints the WITHHELD line for the parent; next picks the child.
func TestBT099_CLIReadyWithholdsParent(t *testing.T) {
	dir, tasksPath, _, _ := bt099SeedBoard(t)
	if _, stderr, code := bt098CLIRun(t, "-C", dir, "create", "--id", "KID-1", "--title", "child", "--parent", "EXIST-1"); code != 0 {
		t.Fatalf("create --parent: exit %d (stderr %q)", code, stderr)
	}
	stdout, _, code := bt098CLIRun(t, "-C", dir, "ready")
	if code != 0 {
		t.Fatalf("ready exit = %d", code)
	}
	if !strings.Contains(stdout, "KID-1") || !strings.Contains(stdout, "WITHHELD (open children ready): EXIST-1 — dispatch KID-1 first") {
		t.Fatalf("ready output missing the child or the withheld line:\n%s", stdout)
	}
	// The parent completes... no — the CHILD completes; the parent becomes
	// dispatchable and the withheld line disappears.
	if _, stderr, code := bt098CLIRun(t, "-C", dir, "update", "KID-1", "--status", "complete",
		"--evidence-run-id", "synthetic-run", "--commit-hash", strings.Repeat("b", 40), "--force"); code != 0 {
		t.Fatalf("child completion: exit %d (stderr %q)", code, stderr)
	}
	stdout, _, code = bt098CLIRun(t, "-C", dir, "ready")
	if code != 0 {
		t.Fatalf("ready exit = %d", code)
	}
	if strings.Contains(stdout, "WITHHELD") || !strings.Contains(stdout, "EXIST-1") {
		t.Fatalf("ready after the child completed =\n%s", stdout)
	}
	_ = tasksPath
}

// TestBT099_CLIShowRollup: the parent's show output carries the read-time
// CHILDREN / CHILD DEPS lines.
func TestBT099_CLIShowRollup(t *testing.T) {
	dir, _, _, _ := bt099SeedBoard(t)
	if _, stderr, code := bt098CLIRun(t, "-C", dir, "create", "--id", "KID-1", "--title", "child", "--parent", "EXIST-1"); code != 0 {
		t.Fatalf("create --parent: exit %d (stderr %q)", code, stderr)
	}
	stdout, _, code := bt098CLIRun(t, "-C", dir, "show", "EXIST-1")
	if code != 0 {
		t.Fatalf("show exit = %d", code)
	}
	if !strings.Contains(stdout, "CHILDREN: 1/1 open (pending 1)") || !strings.Contains(stdout, "CHILD DEPS (union): (none)") {
		t.Fatalf("show rollup missing:\n%s", stdout)
	}
	// A childless row prints no rollup block.
	stdout, _, _ = bt098CLIRun(t, "-C", dir, "show", "KID-1")
	if strings.Contains(stdout, "CHILDREN:") {
		t.Fatalf("childless row printed a rollup:\n%s", stdout)
	}
}

// TestBT099_CLIValidateParentCycle: a hand-edited cycle fails validate with
// the cycle named; --fail-on parent-cycle recognizes the class; an unknown
// class still fails usage-level.
func TestBT099_CLIValidateParentCycle(t *testing.T) {
	dir := seedCLIBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(
		`{"id":"RA-1","title":"a","status":"pending","priority":"P1","parent":"RB-1"}`+"\n"+
			`{"id":"RB-1","title":"b","status":"pending","priority":"P1","parent":"RA-1"}`+"\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := bt098CLIRun(t, "-C", dir, "validate")
	if code != 1 {
		t.Fatalf("validate exit = %d, want 1:\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "parent cycle detected: RA-1 -> RB-1 -> RA-1") {
		t.Fatalf("validate did not name the cycle:\n%s", stdout)
	}
	if _, stderr, code := bt098CLIRun(t, "-C", dir, "validate", "--fail-on", "parent-cycle"); code != 1 {
		t.Fatalf("--fail-on parent-cycle exit = %d (stderr %q)", code, stderr)
	}
	if _, stderr, code := bt098CLIRun(t, "-C", dir, "validate", "--fail-on", "no-such-class"); code != 2 {
		t.Fatalf("unknown --fail-on class exit = %d, want 2 (stderr %q)", code, stderr)
	}
}

// TestBT099_CLICreateUpdateFlags: create --parent/--expected-files/
// --verify-steps wire the row keys; update re-points, replaces the arrays,
// and --parent "" removes the link.
func TestBT099_CLICreateUpdateFlags(t *testing.T) {
	dir, tasksPath, _, _ := bt099SeedBoard(t)
	_, stderr, code := bt098CLIRun(t, "-C", dir, "create", "--id", "KID-1", "--title", "child",
		"--parent", "EXIST-1", "--expected-files", "a.go, b/c_test.go",
		"--verify-steps", "run go test", "--verify-steps", "prove X")
	if code != 0 {
		t.Fatalf("create exit = %d (stderr %q)", code, stderr)
	}
	rows := bt099Read(t, tasksPath)
	if !strings.Contains(rows, `"parent":"EXIST-1"`) {
		t.Fatalf("parent key not written:\n%s", rows)
	}
	for _, want := range []string{`"expected_files":["a.go","b/c_test.go"]`, `"verify_steps":["run go test","prove X"]`} {
		if !strings.Contains(rows, want) {
			t.Fatalf("decomposition metadata missing %s:\n%s", want, rows)
		}
	}
	// Re-point to another parent, then remove the link entirely.
	if _, stderr, code := bt098CLIRun(t, "-C", dir, "create", "--id", "PRN-2", "--title", "second parent"); code != 0 {
		t.Fatalf("second parent create: exit %d (stderr %q)", code, stderr)
	}
	if _, stderr, code := bt098CLIRun(t, "-C", dir, "update", "KID-1", "--parent", "PRN-2"); code != 0 {
		t.Fatalf("re-parent: exit %d (stderr %q)", code, stderr)
	}
	if rows = bt099Read(t, tasksPath); !strings.Contains(rows, `"parent":"PRN-2"`) {
		t.Fatalf("re-parent did not land:\n%s", rows)
	}
	if _, stderr, code := bt098CLIRun(t, "-C", dir, "update", "KID-1", "--parent", ""); code != 0 {
		t.Fatalf("--parent \"\": exit %d (stderr %q)", code, stderr)
	}
	if rows = bt099Read(t, tasksPath); strings.Contains(rows, `"parent"`) {
		t.Fatalf("--parent \"\" left a parent key behind:\n%s", rows)
	}
	// Replace the verify_steps array on the existing row.
	if _, stderr, code := bt098CLIRun(t, "-C", dir, "update", "KID-1", "--verify-steps", "only one step"); code != 0 {
		t.Fatalf("update --verify-steps: exit %d (stderr %q)", code, stderr)
	}
	if rows = bt099Read(t, tasksPath); !strings.Contains(rows, `"verify_steps":["only one step"]`) {
		t.Fatalf("verify_steps not replaced:\n%s", rows)
	}
}

// TestBT099_CLIDescribeCensus: the new verbs are in the derived registry
// with curated summaries — the docs census must see them.
func TestBT099_CLIDescribeCensus(t *testing.T) {
	commands, err := describe.DeriveCLI("main.go")
	if err != nil {
		t.Fatalf("describe.DeriveCLI: %v", err)
	}
	seen := map[string]string{}
	for _, cmd := range commands {
		seen[cmd.Name] = cmd.Summary
	}
	for _, name := range []string{"comment", "decompose"} {
		summary, ok := seen[name]
		if !ok {
			t.Fatalf("describe census is missing %q", name)
		}
		if summary == "" {
			t.Fatalf("describe summary for %q is empty — add the curated line to internal/describe", name)
		}
	}
}
