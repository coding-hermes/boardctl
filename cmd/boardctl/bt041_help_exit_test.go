package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-hermes/boardctl/internal/board"
)

// ---------- BT-041: CLI contract — help exits 0; event requires --type ----------
//
// Two defects, one task:
//
//  1. `boardctl <sub> --help` flowed the flag package's ErrHelp out of the
//     subcommand's fs.Parse as a plain error, so run()'s generic error path
//     classified a HELP REQUEST as a validation failure (exit 1). README
//     promises "0 ok, 1 validation failure, 2 usage/board-not-found" — a
//     wrapper keying on exit codes could not ask for usage without a red
//     exit. Decision pinned here: a help request is a SUCCESSFUL usage
//     query — usage text on stdout, exit 0.
//
//  2. `boardctl event` with NO --type fell through to the write path's
//     "audit" default and silently appended an event nobody asked for into
//     the append-only trail, exit 0. The subcommand now refuses before the
//     board is even opened: exit 1, the requirement and the enforced
//     event_type vocabulary on stderr, zero writes.

// bt041Subcommands is the full dispatch table from run(): every subcommand
// a user can name, so the help-exit decision is pinned fleet-wide rather
// than only on the commands that happened to be measured when the defect
// was filed.
var bt041Subcommands = []string{
	"init", "list", "show", "create", "update", "event", "header",
	"validate", "sweep-status", "doctor", "version", "stats", "render",
	"import", "serve", "install",
}

// TestBT041HelpExitCodeZeroPerSubcommand: `<sub> --help` (and `-h`) exits 0
// and prints the subcommand's usage line on stdout. -C points at an EMPTY
// temp dir on purpose: the help decision must happen at flag-parse time,
// before any board resolution, so a helpless directory is fine.
func TestBT041HelpExitCodeZeroPerSubcommand(t *testing.T) {
	for _, sub := range bt041Subcommands {
		for _, helpFlag := range []string{"--help", "-h"} {
			args := append([]string{"-C", t.TempDir(), sub}, helpFlag)
			var out string
			var err error
			out, err = captureStdout(func() {
				if code := run(args); code != 0 {
					t.Errorf("%s %s exit code = %d, want 0 (BT-041: a help request is a successful usage query, not a validation failure)", sub, helpFlag, code)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			if sig := "boardctl " + sub; !strings.Contains(out, sig) {
				t.Errorf("%s %s stdout = %q, want the usage signature %q on stdout (not stderr)", sub, helpFlag, out, sig)
			}
		}
	}
}

// TestBT041BareHelpPathsUnchanged: the top-level help entry points keep
// their exit 0, and the no-args invocation keeps its usage error (exit 2).
func TestBT041BareHelpPathsUnchanged(t *testing.T) {
	for _, args := range [][]string{
		{"help"},
		{"--help"},
		{"-h"},
	} {
		var out string
		var err error
		out, err = captureStdout(func() {
			if code := run(args); code != 0 {
				t.Errorf("run(%v) exit code = %d, want 0", args, code)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "usage:") {
			t.Errorf("run(%v) stdout = %q, want the usage text", args, out)
		}
	}
	if code := run(nil); code != 2 {
		t.Errorf("run(nil) exit code = %d, want 2 (usage error, unchanged)", code)
	}
}

// TestBT041EventWithoutTypeRefusesAndWritesNothing: `event` with no --type
// must be refused (exit 1) BEFORE the board is touched — events.jsonl stays
// byte-identical, so no default "audit" event can ever be appended.
func TestBT041EventWithoutTypeRefusesAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	if code := run([]string{"-C", dir, "init", "--project", "bt041", "--namespace", "test"}); code != 0 {
		t.Fatalf("init exit code = %d, want 0", code)
	}
	eventsPath := filepath.Join(dir, ".coding-hermes", "board", "events.jsonl")
	before, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-C", dir, "event"},
		{"-C", dir, "event", "--detail-text", `"looked like an audit"`},
		{"-C", dir, "event", "--task-id", "X-1", "--tick", "1"},
	} {
		got, cerr := captureStderr(func() {
			if code := run(args); code != 1 {
				t.Errorf("event without --type (%v) exit code = %d, want 1", args, code)
			}
		})
		if cerr != nil {
			t.Fatal(cerr)
		}
		if !strings.Contains(got, "--type is required") {
			t.Errorf("refusal stderr = %q, want it to name the missing --type flag", got)
		}
		// The refusal lists the enforced vocabulary, member by member —
		// derived from the write path's own set, so the two can never drift.
		for etype := range board.EventTypeVocabulary {
			if !strings.Contains(got, etype) {
				t.Errorf("refusal stderr = %q, want it to list allowed event_type %q", got, etype)
			}
		}
		after, rerr := os.ReadFile(eventsPath)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if string(before) != string(after) {
			t.Fatalf("refused event mutated events.jsonl:\nbefore %s\nafter  %s", before, after)
		}
	}
}

// TestBT041EventGateDoesNotBiteExplicitTypes: the gate only fires on an
// OMITTED --type. An explicit in-vocabulary type still appends (exit 0), and
// an explicit off-vocabulary type keeps the write path's own refusal.
func TestBT041EventGateDoesNotBiteExplicitTypes(t *testing.T) {
	dir := t.TempDir()
	if code := run([]string{"-C", dir, "init", "--project", "bt041", "--namespace", "test"}); code != 0 {
		t.Fatalf("init exit code = %d, want 0", code)
	}
	eventsPath := filepath.Join(dir, ".coding-hermes", "board", "events.jsonl")
	before, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-C", dir, "event", "--type", "audit", "--detail-text", `"explicit"`}); code != 0 {
		t.Errorf("event --type audit exit code = %d, want 0 (explicit type must still work)", code)
	}
	after, rerr := os.ReadFile(eventsPath)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if string(before) == string(after) {
		t.Fatal("event --type audit did not append an event row")
	}
	afterRows := strings.Split(strings.TrimRight(string(after), "\n"), "\n")
	last := afterRows[len(afterRows)-1]
	// The append path is single-line but not guaranteed compact — accept
	// both key spellings (repo-wide convention for JSONL substring checks).
	if !strings.Contains(last, `"event_type":"audit"`) && !strings.Contains(last, `"event_type": "audit"`) {
		t.Fatalf("appended row wrong: %s", last)
	}

	got, cerr := captureStderr(func() {
		if code := run([]string{"-C", dir, "event", "--type", "bogus"}); code != 1 {
			t.Errorf("event --type bogus exit code = %d, want 1 (write-path vocabulary check intact)", code)
		}
	})
	if cerr != nil {
		t.Fatal(cerr)
	}
	if !strings.Contains(got, "not in vocabulary") {
		t.Errorf("bogus-type stderr = %q, want the write path's vocabulary refusal", got)
	}
}
