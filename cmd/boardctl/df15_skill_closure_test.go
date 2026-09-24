package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
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

// skillHeaderStaleTolerance is how far behind the run date the skill's
// frontmatter `date:` field may sit before the freshness gate fails. The
// date is the operator-facing "last verified against the real CLI" stamp:
// the version bump alone (TestUsageSkillVersionTracksGateResult) cannot go
// stale on its own, so the date is the half of the header that actually
// decays and forces a re-verification pass over the skill. 45 days covers
// roughly one foreman cycle per calendar month plus slack for review
// queues — a rotted skill must fail the build within about six weeks of
// its last proven-fresh edit, not at the next dogfood audit.
const skillHeaderStaleTolerance = 45 * 24 * time.Hour

// skillFrontmatter returns the YAML frontmatter block of a SKILL.md body:
// the text between the opening "---" line and the closing "---" line. It
// returns "" when the block is absent or unterminated, so a restructured
// header fails the date gate loudly at the extraction site rather than
// reading as "no date found" ambiguity downstream.
func skillFrontmatter(skill string) string {
	if !strings.HasPrefix(skill, "---\n") {
		return ""
	}
	end := strings.Index(skill[len("---\n"):], "\n---")
	if end < 0 {
		return ""
	}
	return skill[len("---\n") : len("---\n")+end]
}

// usageSkillDate parses the frontmatter `date: YYYY-MM-DD` field of the
// usage skill. It reports ok=false with a reason for each failure class —
// missing field, malformed value, unparseable calendar date — so the gate
// can name the exact header defect instead of one generic "bad date".
// Only the strict `2006-01-02` layout is accepted: the skill header is a
// machine-checked contract, not free prose (a human-written "Sept 24,
// 2026" in the header is a gate failure by design).
func usageSkillDate(t *testing.T, skill string) (time.Time, bool) {
	t.Helper()
	fm := skillFrontmatter(skill)
	if fm == "" {
		t.Fatal("usage skill has no YAML frontmatter block — the header must carry `date: YYYY-MM-DD`")
	}
	m := regexp.MustCompile(`(?m)^date: (\d{4}-\d{2}-\d{2})$`).FindStringSubmatch(fm)
	if m == nil {
		t.Fatal("usage skill frontmatter has no `date: YYYY-MM-DD` line — the header must state when the skill was last verified against the real CLI")
	}
	d, err := time.Parse("2006-01-02", m[1])
	if err != nil {
		t.Fatalf("usage skill frontmatter date %q does not parse as a calendar date: %v", m[1], err)
	}
	return d, true
}

// usageSkillDateError is the predicate side of the date gate, factored out
// so the mutation test drives exactly the extraction and freshness logic
// the gate uses. It returns "" when the header date is present, parses,
// and sits within skillHeaderStaleTolerance of the run date; otherwise the
// returned message names the exact defect class — the same text the gate's
// t.Fatal/t.Errorf would surface.
func usageSkillDateError(skill string) string {
	fm := skillFrontmatter(skill)
	if fm == "" {
		return "usage skill has no YAML frontmatter block — the header must carry `date: YYYY-MM-DD`"
	}
	m := regexp.MustCompile(`(?m)^date: (\d{4}-\d{2}-\d{2})$`).FindStringSubmatch(fm)
	if m == nil {
		return "usage skill frontmatter has no `date: YYYY-MM-DD` line — the header must state when the skill was last verified against the real CLI"
	}
	d, err := time.Parse("2006-01-02", m[1])
	if err != nil {
		return fmt.Sprintf("usage skill frontmatter date %q does not parse as a calendar date: %v", m[1], err)
	}
	if age := time.Since(d); age > skillHeaderStaleTolerance {
		return fmt.Sprintf("usage skill frontmatter date %s is %d days behind the run date (%s) — beyond the %d-day tolerance; re-verify the skill against the current CLI and refresh the header date",
			d.Format("2006-01-02"),
			int(age.Hours()/24),
			time.Now().Format("2006-01-02"),
			int(skillHeaderStaleTolerance.Hours()/24))
	}
	return ""
}

// TestUsageSkillDateWithinTolerance — the other half of the header
// contract: the frontmatter `date:` field must agree with the gate's run
// date within skillHeaderStaleTolerance. The version floor above can never
// rot (1.7.0 stays >= 1.7.0 forever); the date is the freshness signal —
// when it goes stale, an operator reading the skill cannot tell whether
// the proven commands still match the shipped CLI, and the gate forces the
// re-verification edit that answers the question.
//
// Failure classes covered:
//   - missing `date:` field          → fail
//   - malformed / non-ISO date       → fail
//   - date older than the tolerance  → fail
//   - date current (within window)   → pass (the live skills/ tree)
//
// The mutation cases (missing, malformed, stale) are pinned by
// TestUsageSkillDateGateBitesOnHeaderMutations below, which runs the same
// predicate against mutated copies of the real skill body — so a future
// edit that neuters this test cannot pass silently.
func TestUsageSkillDateWithinTolerance(t *testing.T) {
	skill := readUsageSkill(t)
	d, _ := usageSkillDate(t, skill)

	if msg := usageSkillDateError(skill); msg != "" {
		t.Errorf("%s (header date read as %s)", msg, d.Format("2006-01-02"))
	}
}

// TestUsageSkillDateGateBitesOnHeaderMutations proves the date gate is not
// vacuous: it runs the real skill body through mutated frontmatter copies
// and requires each broken class to fail the same predicate the gate uses
// (usageSkillDateError), while the untouched body and freshly-dated bodies
// pass. Without this, a refactor that drops the extraction or freshness
// checks would leave TestUsageSkillDateWithinTolerance passing against a
// skill with no date at all.
func TestUsageSkillDateGateBitesOnHeaderMutations(t *testing.T) {
	skill := readUsageSkill(t)
	fm := skillFrontmatter(skill)
	if fm == "" {
		t.Fatal("usage skill has no frontmatter to mutate — the freshness gate anchor is gone")
	}

	// The live skill must pass its own gate (guards against the header
	// already being stale/malformed, which would make every mutation
	// comparison meaningless).
	if msg := usageSkillDateError(skill); msg != "" {
		t.Fatalf("the live usage skill fails its own freshness gate — fix skills/boardctl-usage/SKILL.md before this mutation battery can assert anything: %s", msg)
	}

	today := time.Now().Format("2006-01-02")
	rotten := time.Now().Add(-skillHeaderStaleTolerance - 24*time.Hour).Format("2006-01-02")
	edgeInside := time.Now().Add(-skillHeaderStaleTolerance + 24*time.Hour).Format("2006-01-02")
	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")

	// The battery mutates the LIVE header date, so the search string is
	// derived from the file each run — no hardcoded date to drift out of
	// sync with the skill.
	dre := regexp.MustCompile(`(?m)^date: (\d{4}-\d{2}-\d{2})$`)
	live := dre.FindStringSubmatch(fm)
	if live == nil {
		t.Fatal("live frontmatter has no parsable date line — the battery's anchor is gone")
	}
	search := "date: " + live[1]
	setDate := func(v string) func(string) string {
		return func(s string) string { return strings.Replace(s, search, "date: "+v, 1) }
	}

	cases := []struct {
		name       string
		mutate     func(string) string
		wantOK     bool
		wantHit    string // substring expected in the failure message; "" for passing cases
		allowNoOp  bool   // skip (not fail) when the mutation is a no-op today
		noOpReason string
	}{
		{
			// A one-day-old date must pass — freshness is a window, not
			// an equality check on today.
			name:   "date one day old passes",
			mutate: setDate(yesterday),
			wantOK: true,
		},
		{
			name:   "date just inside tolerance passes",
			mutate: setDate(edgeInside),
			wantOK: true,
		},
		{
			// When the run date equals the header date this mutation is a
			// no-op; the live body is then already the current-date
			// specimen, proven by the self-check above.
			name:       "current date passes",
			mutate:     setDate(today),
			wantOK:     true,
			allowNoOp:  true,
			noOpReason: "run date already equals the header date — the live body above is the current-date specimen",
		},
		{
			name:    "missing date field fails",
			mutate:  func(s string) string { return strings.Replace(s, search+"\n", "", 1) },
			wantOK:  false,
			wantHit: "no `date: YYYY-MM-DD` line",
		},
		{
			name:    "malformed date fails",
			mutate:  setDate("September 24, 2026"),
			wantOK:  false,
			wantHit: "no `date: YYYY-MM-DD` line",
		},
		{
			name:    "non-calendar date fails",
			mutate:  setDate("2026-13-40"),
			wantOK:  false,
			wantHit: "does not parse as a calendar date",
		},
		{
			name:    "stale date beyond tolerance fails",
			mutate:  setDate(rotten),
			wantOK:  false,
			wantHit: rotten,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mutated := tc.mutate(skill)
			if mutated == skill {
				if tc.allowNoOp {
					t.Skip(tc.noOpReason)
				}
				t.Fatalf("mutation %q did not change the skill body — the battery's date anchor has drifted", tc.name)
			}
			msg := usageSkillDateError(mutated)
			gotOK := msg == ""
			if gotOK != tc.wantOK {
				t.Fatalf("mutated skill freshness = %v, want %v (gate message: %q)", gotOK, tc.wantOK, msg)
			}
			if !tc.wantOK && !strings.Contains(msg, tc.wantHit) {
				t.Fatalf("gate failure for %q does not name the defect: got %q, want substring %q", tc.name, msg, tc.wantHit)
			}
		})
	}
}
