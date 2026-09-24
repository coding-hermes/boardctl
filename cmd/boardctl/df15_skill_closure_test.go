package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ---------- DF-BOARDCTL-15: usage-skill freshness gate ----------
//
// The agent-facing usage skill (skills/boardctl-usage/SKILL.md) is what
// coding agents read to operate boardctl. It has rotted before: the audit
// that filed DF-BOARDCTL-15 found `sweep-status` and `install` shipping in
// the CLI without ever appearing in the skill's agent-facing entry-point
// surfaces. Nothing asserted the skill still covered the real CLI — until
// now.
//
// The gate extracts the top-level command list from the REAL help text —
// the same `usageText` constant main.go prints for --help (no drift copy)
// — and requires every command name to appear in the skill inside one of
// the agent-facing surfaces: the "Entry points" bullet list or the
// "Proven commands" block. Coverage of the Proven commands block is a
// REQUIRED half of the contract, not an optional extra: both half-open
// findings named above rotted exactly there.

// usageSkillCommands is the single source of truth for the command list:
// the same usageText constant the CLI prints for --help. Parsing it (not
// re-typing it) means a new subcommand is covered the moment it is added
// to the help text — the gate cannot be bypassed by editing help and
// forgetting the test's private list.
//
// Command entry lines in usageText sit at EXACTLY two leading spaces:
//
//	"  init    [-C dir] ..."
//	"  sweep-status [--apply] ..."
//
// Flag-continuation and description lines hang deeper (10-11 spaces:
// "          [--reasoning R] ..." under create, the prose under
// sweep-status and install), so the indent width is the discriminator
// between an entry and its wrapped explanation. A reflow that changes
// that convention breaks TestSkillCoverageParserProvesGatesBite (16
// commands, exact census) and must be fixed consciously, not silently.
func usageSkillCommands() []string {
	// Extract the `commands:` section of the help text: from the
	// "\ncommands:\n" marker to the next paragraph break.
	body := skillSectionBetween(usageText, "\ncommands:\n", "\n\n")
	var cmds []string
	for _, line := range strings.Split(body, "\n") {
		if leadingSpaces(line) != 2 {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		cmd := strings.Fields(trimmed)[0]
		cmd = strings.TrimRight(cmd, ".,;:")
		if !isCommandNameToken(cmd) {
			continue
		}
		cmds = append(cmds, cmd)
	}
	return cmds
}

// leadingSpaces counts the leading space characters of a line.
func leadingSpaces(line string) int {
	n := 0
	for n < len(line) && line[n] == ' ' {
		n++
	}
	return n
}

// isCommandNameToken accepts bare lowercase words and hyphenated ones
// (init, show, sweep-status) — exactly the shapes a Go identifier-ish
// subcommand name takes. It rejects meta/help tokens and anything that
// smuggles flags, placeholders or punctuation.
func isCommandNameToken(tok string) bool {
	if tok == "" || tok == "help" {
		return false
	}
	for _, r := range tok {
		switch {
		case r >= 'a' && r <= 'z':
		case r == '-':
		default:
			return false
		}
	}
	// A bare hyphen is not a command.
	return tok != "-"
}

// skillEntrySections are the two agent-facing surfaces the gate requires
// coverage in, with the extraction rules for each.
var skillEntrySections = []struct {
	name   string
	marker string
}{
	{name: "Entry points", marker: "## Entry points"},
	{name: "Proven commands", marker: "## Proven commands"},
}

// skillSectionBetween returns the text of the first heading matching
// marker up to the next markdown heading at the marker's level. When the
// marker heading does not exist, it returns "" — a missing required
// surface fails the gate later, with the command names in hand, not here.
func skillSectionBetween(skill, marker, nextPrefix string) string {
	start := strings.Index(skill, marker)
	if start < 0 {
		return ""
	}
	body := skill[start+len(marker):]
	if end := strings.Index(body, "\n"+nextPrefix); end >= 0 {
		body = body[:end]
	}
	return body
}

// readUsageSkill loads skills/boardctl-usage/SKILL.md relative to the
// package directory (cmd/boardctl) without writing anything anywhere. It
// fails loudly when the skill is missing — a moved or renamed skill must
// read as a gate failure, not as an empty pass.
func readUsageSkill(t *testing.T) string {
	t.Helper()
	skillPath := filepath.Join("..", "..", "skills", "boardctl-usage", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("usage skill is unreadable — the freshness gate cannot run: %v", err)
	}
	return string(data)
}

// TestUsageSkillCoversEveryTopLevelCommand is the gate: every top-level
// command the real help text exposes must be present in the agent-facing
// usage skill (skills/boardctl-usage/SKILL.md), inside the "Entry points"
// bullet list OR the "Proven commands" block. A command rotting out of
// both surfaces — the DF-BOARDCTL-15 finding — fails the build here, at
// commit time, not months later in a dogfood audit.
func TestUsageSkillCoversEveryTopLevelCommand(t *testing.T) {
	skill := readUsageSkill(t)

	// Both required surfaces must exist as headings; otherwise the skill
	// was restructured and the gate's anchor is gone.
	var surfaces []string // surface bodies in skillEntrySections order
	for _, sec := range skillEntrySections {
		body := skillSectionBetween(skill, sec.marker, "## ")
		if body == "" {
			t.Fatalf("usage skill lost its %q section — re-anchor the gate at skills/boardctl-usage/SKILL.md", sec.name)
		}
		surfaces = append(surfaces, body)
	}

	commands := usageSkillCommands()
	if len(commands) == 0 {
		t.Fatal("no top-level commands parsed from usageText — the gate's parser must be fixed before this check can assert anything")
	}

	for _, cmd := range commands {
		covered := false
		for i := range skillEntrySections {
			if commandMentioned(surfaces[i], cmd) {
				covered = true
				break
			}
		}
		if covered {
			continue
		}
		t.Errorf("usage skill drift: top-level command %q appears in the real --help output but in NEITHER the %q list NOR the %q block of skills/boardctl-usage/SKILL.md (DF-BOARDCTL-15). Add it where the file's existing style keeps it.",
			cmd, skillEntrySections[0].name, skillEntrySections[1].name)
	}
}

// commandMentioned reports whether cmd appears as a standalone command
// name in a skill surface: the word-boundary form used by the help text,
// so `sweep-status` inside `sweep-statuses` does not pass, and a mention
// inside an unrelated identifier does not either. Word boundaries are
// ASCII letter/digit/underscore edges — hyphens are INTERNAL to command
// names (sweep-status), so the boundary class must exclude hyphens from
// both sides of a match.
func commandMentioned(surface, cmd string) bool {
	re := regexp.MustCompile(`[a-z0-9_-]+`)
	for _, tok := range re.FindAllString(surface, -1) {
		if tok == cmd {
			return true
		}
	}
	return false
}

// TestSkillCoverageParserProvesGatesBite proves the parser classes real
// command lines and rejects flag-continuation lines, so the coverage gate
// above cannot pass vacuously: if the command extractor went blind, the
// gate would degrade to "zero commands to check" and silently stop
// guarding. (The zero-commands case is a hard t.Fatal in the gate; this
// test pins the extractor's behaviour on usageText directly.)
func TestSkillCoverageParserProvesGatesBite(t *testing.T) {
	cmds := usageSkillCommands()
	// Commands the dispatcher actually routes (run() in main.go), all
	// present in usageText. If any goes missing from the parse, the gate
	// would stop checking it.
	for _, want := range []string{"init", "list", "show", "create", "update", "event", "header", "validate", "sweep-status", "doctor", "version", "stats", "render", "import", "serve", "install"} {
		found := false
		for _, c := range cmds {
			if c == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("usageSkillCommands() missed dispatched command %q; got %v", want, cmds)
		}
	}
	// 16 dispatched commands exactly — no phantom extras.
	if len(cmds) != 16 {
		t.Errorf("usageSkillCommands() = %v (%d), want the 16 dispatched commands", cmds, len(cmds))
	}
	// Continuation lines must not leak in as fake commands.
	for _, bad := range []string{"reasoning", "capability-tags", "set-ticks-total"} {
		for _, c := range cmds {
			if c == bad {
				t.Errorf("usageSkillCommands() picked up flag-continuation token %q", bad)
			}
		}
	}
}

// TestUsageSkillVersionTracksGateResult — the skill's frontmatter carries
// a `version:` field that must agree with the gate result: the header
// must exist, parse as semver, and be at least the version that last
// proved the skill covers the full CLI (1.7.0 bumped coverage in the
// DF-BOARDCTL-15 gate work).
func TestUsageSkillVersionTracksGateResult(t *testing.T) {
	skill := readUsageSkill(t)
	m := regexp.MustCompile(`(?m)^version: (\d+\.\d+\.\d+)$`).FindStringSubmatch(skill)
	if m == nil {
		t.Fatal("usage skill frontmatter has no `version: X.Y.Z` line — the header must agree with the gate result")
	}
	var major, minor, patch int
	if _, err := fmt.Sscanf(m[1], "%d.%d.%d", &major, &minor, &patch); err != nil {
		t.Fatalf("version %q does not parse as semver: %v", m[1], err)
	}
	if v := versionKey(major, minor, patch); v < versionKey(1, 7, 0) {
		t.Errorf("usage skill version %s predates the DF-BOARDCTL-15 coverage bump — bump the frontmatter version (1.7.0 minimum) before the gate can pass", m[1])
	}
}

// versionKey folds a semver triple into a single comparable integer.
func versionKey(major, minor, patch int) int {
	return major*1000000 + minor*1000 + patch
}
