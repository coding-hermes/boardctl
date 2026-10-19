package main

// DOC-37 regression pin: the --repair-events flag must stay visible on BOTH
// help surfaces (the top-level usageText normalize-board entry AND the
// normalize-board -h closure — the same flag-agreement defect class
// TestRootUsageTextCreateUpdateMatchClosures pins for create/update), and
// README.md must document the event repair with its dry-run/apply arms.
//
// The flag is registered in cmdNormalizeBoard as `repair-events` and extends
// NormalizeBoard with descending event-id repair in events.jsonl
// (DF-BOARDCTL-38, internal/board/repair_events.go). The behavior itself has
// unit coverage there; this file pins the DOC/HELP surface only.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-hermes/boardctl/internal/versioncheck"
)

// TestDoc37RootUsageNamesRepairEvents: the top-level usageText
// normalize-board entry lists --repair-events alongside the other flags.
func TestDoc37RootUsageNamesRepairEvents(t *testing.T) {
	for _, line := range strings.Split(usageText, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "normalize-board ") {
			if !strings.Contains(line, "--repair-events") {
				t.Errorf("usageText normalize-board entry lost --repair-events: %q", line)
			}
			return
		}
	}
	t.Fatal("usageText has no normalize-board entry")
}

// TestDoc37SubcommandHelpNamesRepairEvents: `normalize-board -h` (the
// closure output) names --repair-events on its usage line, and the
// bracketed flag-LIST of the root usageText entry agrees flag-for-flag
// with the closure's own usage line (both directions — no root-only or
// closure-only flag on the usage line itself; prose flag mentions outside
// the bracket list are deliberately not compared).
func TestDoc37SubcommandHelpNamesRepairEvents(t *testing.T) {
	got, err := captureStdout(func() { run([]string{"normalize-board", "-h"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "--repair-events") {
		t.Errorf("normalize-board -h missing --repair-events:\n%s", got)
	}

	// Flags inside the [...] list of one usage line (first bracket pair
	// group on the line). Prose elsewhere may legitimately name other
	// flags (`--apply`, `update --normalize`), so the comparison sticks
	// to the line that IS the flag list.
	listFlags := func(line string) map[string]bool {
		out := map[string]bool{}
		open := strings.Index(line, "[")
		if open < 0 {
			return out
		}
		if closeIdx := strings.LastIndex(line, "]"); closeIdx > open {
			for _, m := range rootFlagRe.FindAllStringSubmatch(line[open:closeIdx], -1) {
				out[m[1]] = true
			}
		}
		return out
	}
	var rootLine string
	for _, line := range strings.Split(usageText, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "normalize-board ") {
			rootLine = line
			break
		}
	}
	if rootLine == "" {
		t.Fatal("usageText has no normalize-board entry")
	}
	var closureLine string
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "normalize-board") && strings.Contains(line, "[") {
			closureLine = line
			break
		}
	}
	if closureLine == "" {
		t.Fatalf("normalize-board -h has no usage line:\n%s", got)
	}
	root, closure := listFlags(rootLine), listFlags(closureLine)
	if len(root) == 0 || len(closure) == 0 {
		t.Fatalf("normalize-board: parsed zero usage-line flags (root %v, closure %v) — the extractor is blind", root, closure)
	}
	for f := range closure {
		if !root[f] {
			t.Errorf("normalize-board: flag --%s on the closure usage line but MISSING from the root usageText entry", f)
		}
	}
	for f := range root {
		if !closure[f] {
			t.Errorf("normalize-board: flag --%s on the root usageText entry but MISSING from the closure usage line", f)
		}
	}
}

// TestDoc37ReadmeDocumentsRepairEvents: README's normalize-board section
// explains the event repair — descending event ids in events.jsonl, the
// dry-run default, and the --apply rewrite — kept distinct from the
// tasks.jsonl row-shape repairs.
func TestDoc37ReadmeDocumentsRepairEvents(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := versioncheck.RepoRoot(cwd)
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, versioncheck.READMEName))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	doc := string(data)
	for _, want := range []string{
		"--repair-events",
		"events.jsonl",
		"descending",
		"DRY-RUN",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("README.md missing %q", want)
		}
	}
	// The event repair must be documented near the dry-run/apply contract,
	// not just as a bare flag mention: every --repair-events-bearing bash
	// or prose region should sit in a document that names the apply arm.
	if !strings.Contains(doc, "--repair-events --apply") {
		t.Errorf("README.md does not document the --repair-events --apply arm")
	}
	// Distinctness: the README must attribute the event repair to
	// events.jsonl and the row repairs to tasks.jsonl — a doc that
	// conflates the two files fails this pin. The section spanning the
	// first --repair-events heading through the next `## ` heading must
	// mention BOTH files: events.jsonl as the file repaired, tasks.jsonl
	// as the file the row pass (not this flag) touches.
	start := strings.Index(doc, "--repair-events")
	if start < 0 {
		t.Fatalf("README.md has no --repair-events documentation")
	}
	end := strings.Index(doc[start:], "\n## ")
	if end < 0 {
		t.Fatalf("README.md --repair-events section never closes with a '## ' heading")
	}
	section := doc[start : start+end]
	if !strings.Contains(section, "events.jsonl") {
		t.Errorf("README.md --repair-events section does not name events.jsonl")
	}
	if !strings.Contains(section, "tasks.jsonl") {
		t.Errorf("README.md --repair-events section does not distinguish the tasks.jsonl row repairs")
	}
}
