package main

// BT-098 §10 — CLI surface. The internal/board suite proves the writer's
// semantics; this file proves the VERBS are wired to them and that the exit
// codes and stderr contracts an operator (and a script) actually sees are
// correct.
//
// Three things only a CLI test can catch:
//   - a refusal that exits 0 (silently swallowed by `set -e` scripts),
//   - the unknown-metadata WARNING never reaching stderr (GAP 2: it was
//     accepted, written, and reported nowhere),
//   - a verb that exists in the switch but is missing from `describe`, so the
//     generated docs/OpenAPI census does not know it exists.
//
// Every refusal here also asserts tasks.jsonl is byte-identical: "exit 1" plus
// a half-written board is worse than no gate at all.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-hermes/boardctl/internal/describe"
)

// bt098CLIReleaseType is the §2.4 worked example, read from the ONE fixture the
// internal suite also uses — schema and tests cannot drift apart.
func bt098CLIReleaseType(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "board", "testdata", "bt098-release-type.jsonl"))
	if err != nil {
		t.Fatalf("read §2.4 fixture: %v", err)
	}
	return strings.TrimSpace(string(raw))
}

// bt098CLIBoard seeds a board with the §2.4 registry and one conformant
// release row, returning the -C target and the tasks.jsonl path.
func bt098CLIBoard(t *testing.T) (dir, tasksPath string) {
	t.Helper()
	dir = seedCLIBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(
		`{"id":"REL-TEST-1","title":"[RELEASE] Existing release","type":"release","status":"pending","priority":"P1","complexity":3,"created_at":"2026-11-01T00:00:00+00:00","updated_at":"2026-11-01T00:00:00+00:00","detail":{"metadata":{"channel":"beta","source_commit":"abc1234"}}}`+"\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "types.jsonl"), []byte(bt098CLIReleaseType(t)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, filepath.Join(boardDir, "tasks.jsonl")
}

func bt098CLIRead(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

// bt098CLIRun captures stdout, stderr and the exit code of one invocation.
func bt098CLIRun(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	stdout, err := captureStdout(func() {
		stderr, _ = captureStderrString(func() {
			code = run(args)
		})
	})
	if err != nil {
		t.Fatalf("capture stdout: %v", err)
	}
	return stdout, stderr, code
}

// TestBT098_CLITypesCensus: `types` renders the configured registry, including
// the template and every metadata field with its type contract. Exit 0.
func TestBT098_CLITypesCensus(t *testing.T) {
	dir, _ := bt098CLIBoard(t)
	stdout, stderr, code := bt098CLIRun(t, "-C", dir, "types")
	if code != 0 {
		t.Fatalf("types exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("types wrote to stderr on a healthy registry: %q", stderr)
	}
	for _, want := range []string{
		"release (Release)",
		`template: prefix="[RELEASE] " priority=P1 skeleton`,
		"channel(enum, required: stable|beta|nightly)",
		"source_commit(string, required)",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("types output missing %q:\n%s", want, stdout)
		}
	}
}

// TestBT098_CLITypesJSON: `--json` is machine-readable and carries the
// metadata contract, so tooling never has to parse the human census.
func TestBT098_CLITypesJSON(t *testing.T) {
	dir, _ := bt098CLIBoard(t)
	stdout, stderr, code := bt098CLIRun(t, "-C", dir, "types", "--json")
	if code != 0 {
		t.Fatalf("types --json exit code = %d (stderr: %s)", code, stderr)
	}
	var payload struct {
		Types []struct {
			Slug     string `json:"slug"`
			Metadata []struct {
				Name     string `json:"name"`
				Required bool   `json:"required"`
			} `json:"metadata"`
		} `json:"types"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("types --json is not JSON: %v\n%s", err, stdout)
	}
	if len(payload.Types) != 1 || payload.Types[0].Slug != "release" {
		t.Fatalf("types --json = %+v", payload.Types)
	}
	required := map[string]bool{}
	for _, f := range payload.Types[0].Metadata {
		required[f.Name] = f.Required
	}
	if !required["channel"] || !required["source_commit"] {
		t.Fatalf("required flags lost in JSON: %v", required)
	}
	if required["signoffs"] {
		t.Fatalf("optional field reported required: %v", required)
	}
}

// TestBT098_CLICreateWiresType: the flagship path — `create --type release`
// with both required fields exits 0, writes the typed row, and prints the
// created-task line.
func TestBT098_CLICreateWiresType(t *testing.T) {
	dir, tasksPath := bt098CLIBoard(t)
	before := bt098CLIRead(t, tasksPath)
	stdout, stderr, code := bt098CLIRun(t, "-C", dir, "create",
		"--id", "REL-TEST-2", "--title", "R2", "--type", "release",
		"--meta", "channel=beta", "--meta", "source_commit=abc1234")
	if code != 0 {
		t.Fatalf("create exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "created task REL-TEST-2") {
		t.Fatalf("create output = %q", stdout)
	}
	after := bt098CLIRead(t, tasksPath)
	if !strings.HasPrefix(after, before) {
		t.Fatalf("create did not append:\n--- before\n%s\n--- after\n%s", before, after)
	}
	appended := strings.TrimSuffix(strings.TrimPrefix(after, before), "\n")
	for _, want := range []string{`"type":"release"`, `"title":"[RELEASE] R2"`, `"priority":"P1"`, `"channel":"beta"`} {
		if !strings.Contains(appended, want) {
			t.Fatalf("appended row missing %s:\n%s", want, appended)
		}
	}
}

// TestBT098_CLIRefusalsExitOneAndWriteNothing is the §10 falsifier triple on
// the CLI surface: exit 1, the exact stderr line, and a byte-identical board.
func TestBT098_CLIRefusalsExitOneAndWriteNothing(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "create missing required metadata",
			args: []string{"create", "--id", "REL-TEST-2", "--title", "R2", "--type", "release"},
			want: "boardctl: create refused: type \"release\" requires metadata fields that are missing:\n" +
				"  - channel (enum: stable|beta|nightly) — required: release channel this artifact ships to\n" +
				"  - source_commit (string) — required: commit the release was cut from\n" +
				"supply each with --meta field=value (repeat the flag per field); nothing was written\n",
		},
		{
			name: "create out-of-vocabulary enum",
			args: []string{"create", "--id", "REL-TEST-2", "--title", "R2", "--type", "release",
				"--meta", "channel=rc", "--meta", "source_commit=abc1234"},
			want: "boardctl: create refused: type \"release\" metadata values are invalid:\n" +
				"  - channel=\"rc\" (allowed: stable|beta|nightly)\n" +
				"nothing was written\n",
		},
		{
			name: "create unknown type",
			args: []string{"create", "--id", "X-1", "--title", "shipit", "--type", "shipit"},
			want: "boardctl: create refused: unknown ticket type \"shipit\" — configured types: release (run 'boardctl types' to list them)\n",
		},
		{
			name: "update drops a required field",
			args: []string{"update", "REL-TEST-1", "--meta", "channel="},
			want: "boardctl: update refused: \"channel\" is REQUIRED by type \"release\" (enum: stable|beta|nightly) — supply a value or change the row's type first\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, tasksPath := bt098CLIBoard(t)
			eventsPath := filepath.Join(filepath.Dir(tasksPath), "events.jsonl")
			tasksBefore, eventsBefore := bt098CLIRead(t, tasksPath), bt098CLIRead(t, eventsPath)

			_, stderr, code := bt098CLIRun(t, append([]string{"-C", dir}, tc.args...)...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1 (stderr: %q)", code, stderr)
			}
			if stderr != tc.want {
				t.Fatalf("stderr = %q\nwant %q", stderr, tc.want)
			}
			if got := bt098CLIRead(t, tasksPath); got != tasksBefore {
				t.Fatalf("refusal wrote to tasks.jsonl:\n--- before\n%s\n--- after\n%s", tasksBefore, got)
			}
			if got := bt098CLIRead(t, eventsPath); got != eventsBefore {
				t.Fatalf("refusal wrote to events.jsonl:\n--- before\n%s\n--- after\n%s", eventsBefore, got)
			}
		})
	}
}

// TestBT098_CLIUpdateUnknownKeyWarns is GAP 2's headline contract: an
// undeclared `--meta` key is WRITTEN (exit 0), and the operator is told on
// stderr with the spec's exact wording. Before the fix this printed only
// "updated task ..." and the key vanished from every report.
func TestBT098_CLIUpdateUnknownKeyWarns(t *testing.T) {
	dir, tasksPath := bt098CLIBoard(t)
	stdout, stderr, code := bt098CLIRun(t, "-C", dir, "update", "REL-TEST-1", "--meta", "weirdkey=v")
	if code != 0 {
		t.Fatalf("unknown metadata key must warn-and-write, exit code = %d (stderr: %q)", code, stderr)
	}
	wantStderr := "warning: metadata key \"weirdkey\" is not declared by type \"release\" — it was written, but 'boardctl validate' will report it\n"
	if stderr != wantStderr {
		t.Fatalf("stderr = %q\nwant %q", stderr, wantStderr)
	}
	if !strings.Contains(stdout, "updated task REL-TEST-1") {
		t.Fatalf("update output = %q", stdout)
	}
	if !strings.Contains(bt098CLIRead(t, tasksPath), `"weirdkey":"v"`) {
		t.Fatal("the undeclared key was not written")
	}

	// A conformant update must stay silent — a warning on every write is noise
	// that operators learn to ignore.
	_, stderr, code = bt098CLIRun(t, "-C", dir, "update", "REL-TEST-1", "--meta", "channel=stable")
	if code != 0 || stderr != "" {
		t.Fatalf("conformant update was not silent: code=%d stderr=%q", code, stderr)
	}
}

// TestBT098_CLIValidateUndeclaredKeyWarns: the warning the CLI promises must
// actually appear in `validate`, at WARNING level, with exit 0 — never an
// error (spec line 193).
func TestBT098_CLIValidateUndeclaredKeyWarns(t *testing.T) {
	dir, _ := bt098CLIBoard(t)
	if _, stderr, code := bt098CLIRun(t, "-C", dir, "update", "REL-TEST-1", "--meta", "weirdkey=v"); code != 0 {
		t.Fatalf("seed unknown key: code=%d stderr=%q", code, stderr)
	}

	stdout, stderr, code := bt098CLIRun(t, "-C", dir, "validate")
	if code != 0 {
		t.Fatalf("undeclared metadata key must not fail validate: exit %d (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stdout, `[warn] BT-098: row REL-TEST-1 (type "release") has undeclared metadata key "weirdkey"`) {
		t.Fatalf("validate did not report the undeclared key as a warning:\n%s", stdout)
	}
	if strings.Contains(stdout, "RESULT: FAIL") {
		t.Fatalf("validate failed on a warning-only board:\n%s", stdout)
	}
}

// TestBT098_CLIDescribeCensus: the verb must be visible to `describe`, or the
// generated CLI docs and the OpenAPI/README census silently omit typed rows —
// the exact failure mode that let §10 go missing in the first place.
func TestBT098_CLIDescribeCensus(t *testing.T) {
	commands, err := describe.DeriveCLI("main.go")
	if err != nil {
		t.Fatalf("describe.DeriveCLI: %v", err)
	}
	seen := map[string]string{}
	for _, cmd := range commands {
		seen[cmd.Name] = cmd.Summary
	}
	for _, name := range []string{"types", "create", "update", "validate"} {
		if _, ok := seen[name]; !ok {
			t.Fatalf("describe census is missing %q (derived verbs tell the docs what exists)", name)
		}
	}
	const wantSummary = "list configured ticket types + their required metadata fields"
	if seen["types"] != wantSummary {
		t.Fatalf("describe summary for types = %q, want %q", seen["types"], wantSummary)
	}
}
