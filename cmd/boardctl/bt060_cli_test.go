package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// BT-060 CLI coverage: --title on update, the update usage closure listing
// every registered value flag, and the root usageText agreeing with the
// subcommand closures flag-for-flag. The board-level write contract is
// pinned in internal/board (bt060_update_title_test.go, bt060_title_priority_test.go);
// these tests pin the CLI surface following bt037_cli_test.go's shape.

// TestCmdUpdateTitleRewritesAndAudits: `update <id> --title T` rewrites the
// row's title and appends the task_updated event (acceptance: events.jsonl
// gains the event; untouched rows byte-identical).
func TestCmdUpdateTitleRewritesAndAudits(t *testing.T) {
	dir := seedCLIBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	tasksPath := filepath.Join(boardDir, "tasks.jsonl")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--title", "New title"}); code != 0 {
		t.Fatalf("update --title exit = %d, want 0", code)
	}
	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeLines := strings.Split(strings.TrimRight(string(before), "\n"), "\n")
	afterLines := strings.Split(strings.TrimRight(string(after), "\n"), "\n")
	if len(beforeLines) != len(afterLines) {
		t.Fatalf("line count changed: %d -> %d", len(beforeLines), len(afterLines))
	}
	for i := range beforeLines {
		if strings.Contains(beforeLines[i], `"id":"EXIST-1"`) {
			continue // the target row is the one allowed to differ
		}
		if beforeLines[i] != afterLines[i] {
			t.Fatalf("line %d changed:\nbefore %s\nafter  %s", i+1, beforeLines[i], afterLines[i])
		}
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(afterLines[0]), &row); err != nil {
		t.Fatal(err)
	}
	if row["title"] != "New title" {
		t.Fatalf("title = %v, want %q", row["title"], "New title")
	}
	// events.jsonl gains the task_updated event
	eventsRaw, err := os.ReadFile(filepath.Join(boardDir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var lastEv map[string]any
	lines := strings.Split(strings.TrimRight(string(eventsRaw), "\n"), "\n")
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &lastEv); err != nil {
		t.Fatal(err)
	}
	if lastEv["event_type"] != "task_updated" || lastEv["task_id"] != "EXIST-1" {
		t.Fatalf("last event = %v, want task_updated on EXIST-1", lastEv)
	}
}

// TestCmdUpdateHelpListsEveryValueFlag: the acceptance criterion — update -h
// documents every registered value flag. RED against the pre-fix closure,
// which printed only status/worktree/branch/session/normalize/force.
func TestCmdUpdateHelpListsEveryValueFlag(t *testing.T) {
	got, err := captureStdout(func() {
		if code := run([]string{"update", "-h"}); code != 0 {
			t.Fatalf("update -h exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{
		"--status", "--title", "--worker-status", "--commit-hash", "--guard",
		"--ci", "--summary", "--note", "--blocked-reason", "--completed-at",
		"--worktree", "--branch", "--session", "--normalize", "--force",
	} {
		if !strings.Contains(got, flag) {
			t.Errorf("update -h usage output missing %q; got:\n%q", flag, got)
		}
	}
}

// TestCmdNormalizeRefusesTitle: --title is a change flag, so --normalize
// refuses the combination (same contract as --worktree/--branch/--session,
// TestCmdNormalizeRefusesWorktreeFlags).
func TestCmdNormalizeRefusesTitle(t *testing.T) {
	dir := seedCLIBoard(t)
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--normalize", "--title", "x"}); code != 1 {
		t.Fatalf("update --normalize --title exit = %d, want 1", code)
	}
	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("refused --normalize --title mutated tasks.jsonl")
	}
}

// TestRootUsageTextCreateUpdateMatchClosures: the root usageText create and
// update entries must list the SAME flags their own usage closures document
// (flag-for-flag agreement — the DF-BOARDCTL-14/BT-060 closure defect class).
// Extracts each entry's flag set from usageText and from the closure text the
// `-h` output actually prints, then diffs both directions.
func TestRootUsageTextCreateUpdateMatchClosures(t *testing.T) {
	// rootUsageEntryFlags gathers the flags on a command's usageText entry
	// plus its continuation lines (entries sit at 2 spaces; continuations
	// hang deeper, the convention TestSkillCoverageParserProvesGatesBite
	// pins). The entry starts at the "\n  <cmd>  " line and runs until the
	// next 2-space entry or the end of the commands block.
	rootUsageEntryFlags := func(cmd string) map[string]bool {
		out := map[string]bool{}
		lines := strings.Split(usageText, "\n")
		start := -1
		for i, line := range lines {
			if leadingSpaces(line) == 2 && strings.HasPrefix(strings.TrimSpace(line), cmd+" ") {
				start = i
				break
			}
		}
		if start < 0 {
			return out
		}
		for i := start; i < len(lines); i++ {
			line := lines[i]
			if i > start && leadingSpaces(line) <= 2 {
				break
			}
			for _, m := range rootFlagRe.FindAllStringSubmatch(line, -1) {
				out[m[1]] = true
			}
		}
		return out
	}
	closureFlags := func(cmd string) map[string]bool {
		got, err := captureStdout(func() { run([]string{cmd, "-h"}) })
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, m := range rootFlagRe.FindAllStringSubmatch(got, -1) {
			out[m[1]] = true
		}
		return out
	}
	for _, cmd := range []string{"create", "update"} {
		root, closure := rootUsageEntryFlags(cmd), closureFlags(cmd)
		if len(root) == 0 || len(closure) == 0 {
			t.Fatalf("%s: parsed zero flags (root %v, closure %v) — the extractor is blind", cmd, root, closure)
		}
		for f := range closure {
			if !root[f] {
				t.Errorf("%s: flag %s in the subcommand closure but MISSING from the root usageText entry", cmd, f)
			}
		}
		for f := range root {
			if !closure[f] {
				t.Errorf("%s: flag %s in the root usageText entry but MISSING from the subcommand closure", cmd, f)
			}
		}
	}
}

// rootFlagRe extracts `--flag-name` tokens (letters/digits/hyphens) from
// usage text. Boolean flags and value flags share the spelling, and -C is
// excluded by the leading double dash.
var rootFlagRe = regexp.MustCompile(`--([a-z0-9-]+)`)
