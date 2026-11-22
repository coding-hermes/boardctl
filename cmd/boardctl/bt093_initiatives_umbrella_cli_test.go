package main

// BT-093 CLI coverage: `boardctl initiatives --umbrella` end-to-end on the
// hand-built temp umbrella workspace (the same fixture builder BT-078 uses,
// seeded through the real CLI create path with --initiative). The suite
// holds the surface to its contracts:
//
//   - AC1: one bucket per initiative slug with the per-repo breakdown
//   - AC4: table shows one line per initiative with repos listed; --json
//     carries the repo breakdown; the single-board default path is untouched
//   - AC3: a child board that fails to read is reported as an ERROR finding
//     and its rows are ABSENT from the totals, exit 1 (fail-closed, the
//     umbrella --stats discipline)

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bt093CreateInitiative seeds one scoped task through the real CLI create path.
func bt093CreateInitiative(t *testing.T, dir, id, status, slug string) {
	t.Helper()
	args := []string{"-C", dir, "create", "--id", id, "--title", "bt093 " + id, "--initiative", slug}
	if status != "" {
		args = append(args, "--status", status)
	}
	if code := run(args); code != 0 {
		t.Fatalf("create %s in %s exit = %d", id, dir, code)
	}
}

// bt093WriteBadLine replaces a child board's tasks.jsonl with one garbage
// line: the read-failure shape behind AC3's degraded totals.
func bt093WriteBadLine(t *testing.T, dir string) {
	t.Helper()
	bad := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	if err := os.WriteFile(bad, []byte("not json\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

// TestBT093UmbrellaInitiativesCLI (AC1/AC4): text table — one line per
// initiative with repos listed — plus the healthy-path exit contract.
func TestBT093UmbrellaInitiativesCLI(t *testing.T) {
	umbrella, childA, childB := bt078Fixture(t)
	// The SAME slug on both children: ONE bucket, TWO repo entries (AC5's
	// aggregation premise), plus an umbrella-only slug and an unscoped row.
	bt093CreateInitiative(t, childA, "BT93-A1", "", "lane-hooks")
	bt093CreateInitiative(t, childA, "BT93-A2", "", "solo")
	bt093CreateInitiative(t, childB, "BT93-B1", "", "lane-hooks")
	bt093CreateInitiative(t, umbrella, "BT93-U1", "", "solo")

	out, code := bt078Run(t, []string{"-C", umbrella, "initiatives", "--umbrella"})
	if code != 0 {
		t.Fatalf("initiatives --umbrella exit = %d, want 0 (stdout: %s | stderr: %s)", code, out, bt078LastStderr)
	}
	if !strings.Contains(out, "REPOS") {
		t.Fatalf("table lacks the REPOS column: %q", out)
	}
	if !strings.Contains(out, "lane-hooks") || !strings.Contains(out, "childA=1 childB=1") {
		t.Fatalf("lane-hooks line lacks the per-repo breakdown: %q", out)
	}
	if !strings.Contains(out, "solo") || !strings.Contains(out, "childA=1 umbrella=1") {
		t.Fatalf("solo line lacks the umbrella-own-board contribution: %q", out)
	}
	// One line per initiative (plus the summary + header lines).
	initiativeLines := 0
	for _, l := range strings.Split(out, "\n") {
		if f := strings.Fields(l); len(f) > 0 && (f[0] == "lane-hooks" || f[0] == "solo") {
			initiativeLines++
		}
	}
	if initiativeLines != 2 {
		t.Fatalf("want exactly 2 initiative lines, got %d: %q", initiativeLines, out)
	}
}

// TestBT093UmbrellaInitiativesJSON (AC4): --json emits the repo breakdown;
// the aggregate totals equal the sum of per-board initiatives.
func TestBT093UmbrellaInitiativesJSON(t *testing.T) {
	umbrella, childA, childB := bt078Fixture(t)
	bt093CreateInitiative(t, childA, "BT93-A1", "", "lane-hooks")
	bt093CreateInitiative(t, childA, "BT93-A2", "", "lane-hooks")
	bt093CreateInitiative(t, childB, "BT93-B1", "", "lane-hooks")

	// Per-board rollups, for the sum assertion.
	var sums [2]int
	for i, root := range []string{childA, childB} {
		out, code := bt078Run(t, []string{"-C", root, "initiatives", "--json"})
		if code != 0 {
			t.Fatalf("single-board initiatives --json exit = %d on %s", code, root)
		}
		var per []struct {
			Initiative string         `json:"initiative"`
			Total      int            `json:"total"`
			Deferred   int            `json:"deferred"`
			ByStatus   map[string]int `json:"by_status"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &per); err != nil {
			t.Fatalf("single-board json does not parse: %v (%q)", err, out)
		}
		for _, s := range per {
			if s.Initiative == "lane-hooks" {
				sums[i] = s.Total
			}
		}
	}

	out, code := bt078Run(t, []string{"-C", umbrella, "initiatives", "--umbrella", "--json"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stdout: %s | stderr: %s)", code, out, bt078LastStderr)
	}
	var payload struct {
		Root        string `json:"root"`
		Initiatives []struct {
			Initiative string `json:"initiative"`
			Total      int    `json:"total"`
			Actionable int    `json:"actionable"`
			Deferred   int    `json:"deferred"`
			Repos      []struct {
				Repo       string `json:"repo"`
				Total      int    `json:"total"`
				Actionable int    `json:"actionable"`
				Deferred   int    `json:"deferred"`
			} `json:"repos"`
		} `json:"initiatives"`
		Total    int              `json:"total"`
		Findings []map[string]any `json:"findings"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &payload); err != nil {
		t.Fatalf("umbrella --json does not parse: %v (%q)", err, out)
	}
	if len(payload.Initiatives) != 1 {
		t.Fatalf("want ONE bucket for the shared slug, got %d: %+v", len(payload.Initiatives), payload.Initiatives)
	}
	b := payload.Initiatives[0]
	if b.Initiative != "lane-hooks" || b.Total != sums[0]+sums[1] {
		t.Fatalf("bucket wrong: %+v (per-board sums %d/%d)", b, sums[0], sums[1])
	}
	if len(b.Repos) != 2 || b.Repos[0].Repo != "childA" || b.Repos[1].Repo != "childB" {
		t.Fatalf("repo breakdown wrong: %+v", b.Repos)
	}
	if payload.Total != 3 || len(payload.Findings) != 0 {
		t.Fatalf("json grand totals/findings wrong: total=%d findings=%+v", payload.Total, payload.Findings)
	}
}

// TestBT093UmbrellaInitiativesCorruptChildExit1 (AC3): the corrupt child is
// reported as an ERROR finding on stderr, its rows are ABSENT from the
// totals, the healthy child's row still shows, and the command exits 1 —
// degraded output must not read as clean success.
func TestBT093UmbrellaInitiativesCorruptChildExit1(t *testing.T) {
	umbrella, childA, childB := bt078Fixture(t)
	bt093CreateInitiative(t, childA, "BT93-A1", "", "lane-hooks")
	bt093CreateInitiative(t, childB, "BT93-B1", "", "lane-hooks")
	bt093WriteBadLine(t, childA)

	out, code := bt078Run(t, []string{"-C", umbrella, "initiatives", "--umbrella"})
	if code != 1 {
		t.Fatalf("exit = %d, want 1 with a corrupt child (stdout: %s)", code, out)
	}
	if !strings.Contains(bt078LastStderr, "[error] link childA") {
		t.Fatalf("stderr lacks the childA error finding: %q", bt078LastStderr)
	}
	if strings.Contains(out, "childA=1") {
		t.Fatalf("corrupt child's rows leaked into the totals: %q", out)
	}
	if !strings.Contains(out, "childB=1") {
		t.Fatalf("healthy child's rows lost from the totals: %q", out)
	}
}

// TestBT093UmbrellaInitiativesBrokenLinkFinding (AC1/AC3): a broken link
// fails the same discipline — finding names the link, exit 1; the rest of
// the view still aggregates.
func TestBT093UmbrellaInitiativesBrokenLinkFinding(t *testing.T) {
	umbrella, childA, _ := bt078Fixture(t)
	bt093CreateInitiative(t, childA, "BT93-A1", "", "lane-hooks")
	links := filepath.Join(umbrella, ".coding-hermes", "links")
	if err := os.Symlink(filepath.Join(umbrella, "ghost", ".coding-hermes"), filepath.Join(links, "ghost")); err != nil {
		t.Fatal(err)
	}

	out, code := bt078Run(t, []string{"-C", umbrella, "initiatives", "--umbrella"})
	if code != 1 {
		t.Fatalf("exit = %d, want 1 with a broken link", code)
	}
	if !strings.Contains(bt078LastStderr, "link ghost") || !strings.Contains(bt078LastStderr, "broken symlink") {
		t.Fatalf("broken-link finding missing: %q", bt078LastStderr)
	}
	if !strings.Contains(out, "childA=1") {
		t.Fatalf("healthy boards' totals lost: %q", out)
	}
}

// TestBT093UmbrellaInitiativesSkipBadLines: --skip-bad-lines arms the
// tolerant read across the view — parseable rows still counted, the
// SKIPPED-LINES evidence block on stderr, exit 1 (DF-BOARDCTL-47 contract).
func TestBT093UmbrellaInitiativesSkipBadLines(t *testing.T) {
	umbrella, childA, childB := bt078Fixture(t)
	bt093CreateInitiative(t, childA, "BT93-A1", "", "lane-hooks")
	bt093CreateInitiative(t, childB, "BT93-B1", "", "lane-hooks")
	// Corrupt childA's last line (the seeded task row) — the fixture rows
	// above must render unparseable for this board.
	bad := filepath.Join(childA, ".coding-hermes", "board", "tasks.jsonl")
	if err := os.WriteFile(bad, []byte("neverdone-1\tnow json\n"), 0600); err != nil {
		t.Fatal(err)
	}

	out, code := bt078Run(t, []string{"-C", umbrella, "initiatives", "--umbrella", "--skip-bad-lines"})
	if code != 1 {
		t.Fatalf("skip-bad-lines exit = %d, want 1 (degraded)", code)
	}
	if !strings.Contains(bt078LastStderr, "SKIPPED-LINES") {
		t.Fatalf("stderr lacks the SKIPPED-LINES block: %q", bt078LastStderr)
	}
	if !strings.Contains(out, "childB=1") || strings.Contains(out, "childA=1") {
		t.Fatalf("counting wrong under skip-bad-lines: %q", out)
	}
}

// TestBT093UmbrellaInitiativesUsage: usage-class refusals (positional args,
// invalid status) and the help-exit-0 contract match the sibling verbs.
func TestBT093UmbrellaInitiativesUsage(t *testing.T) {
	umbrella, _, _ := bt078Fixture(t)
	for _, tc := range []struct {
		name string
		args []string
		code int
		err  string
	}{
		{"positional", []string{"-C", umbrella, "initiatives", "--umbrella", "surplus"}, 1, "takes no positional"},
		{"bad status", []string{"-C", umbrella, "initiatives", "--umbrella", "--status", "nonsense"}, 1, "not in vocabulary"},
		{"help", []string{"-C", umbrella, "initiatives", "--umbrella", "--help"}, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, code := bt078Run(t, tc.args)
			if code != tc.code {
				t.Fatalf("exit = %d, want %d", code, tc.code)
			}
			if tc.err != "" && !strings.Contains(bt078LastStderr, tc.err) {
				t.Fatalf("error wording missing %q: %q", tc.err, bt078LastStderr)
			}
		})
	}
}

// TestBT093SeededSingleBoardUnchanged: the plain `initiatives` table on a
// CHILD repo renders exactly the single-board rollup — no repo column, no
// umbrella header (AC4: the new surface is opt-in via --umbrella).
func TestBT093SeededSingleBoardUnchanged(t *testing.T) {
	_, childA, _ := bt078Fixture(t)
	bt093CreateInitiative(t, childA, "BT93-A1", "", "lane-hooks")
	out, code := bt078Run(t, []string{"-C", childA, "initiatives"})
	if code != 0 {
		t.Fatalf("single-board initiatives exit = %d", code)
	}
	if strings.Contains(out, "REPOS") || strings.Contains(out, "UMBRELLA") {
		t.Fatalf("plain initiatives leaked the umbrella surface: %q", out)
	}
	if !strings.Contains(out, "lane-hooks") {
		t.Fatalf("single-board rollup lost: %q", out)
	}
	// --umbrella on a plain repo with NO registry still works over its own
	// board (the warn finding rides stderr, exit 0).
	if out, code := bt078Run(t, []string{"-C", childA, "initiatives", "--umbrella"}); code != 0 || !strings.Contains(out, "childA=1") {
		t.Fatalf("umbrella rollup over a registry-less board failed: out=%q stderr=%q code=%d", out, bt078LastStderr, code)
	}
}
