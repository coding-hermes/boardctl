package main

// BT-078 CLI coverage: `boardctl umbrella` end-to-end on a hand-built temp
// umbrella repo. Fixtures bootstrap REAL boards through the real CLI paths
// (cmdInit + run create) per the house pattern, then symlink them under the
// umbrella's .coding-hermes/links registry. The suite holds the command to
// its contracts:
//
//   - AC2: census lists both child boards with per-repo task counts + provenance
//   - AC3: colliding ids appear twice, distinguishable by repo qualifier;
//     an ambiguous cross-board depends_on is REPORTED, never resolved
//   - AC4: broken + looping links are named findings with exit 1, no hang
//   - AC5: zero writes through child symlinks (sha256 snapshot before/after)
//   - AC6: direct-root behavior unchanged (list/validate regression)
//   - --json census, --strict, mode-conflict usage refusals, help exit 0

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// bt078InitChild bootstraps a child repo (real git repo + real board via the
// CLI init path) and seeds tasks through `run create`.
func bt078InitChild(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v (%s)", dir, err, out)
	}
	if err := cmdInit(dir, []string{"--project", filepath.Base(dir)}); err != nil {
		t.Fatalf("init %s: %v", dir, err)
	}
}

// bt078Create seeds one task through the real CLI create path.
func bt078Create(t *testing.T, dir, id, title string) {
	t.Helper()
	if code := run([]string{"-C", dir, "create", "--id", id, "--title", title}); code != 0 {
		t.Fatalf("create %s in %s exit = %d, want 0", id, dir, code)
	}
}

// bt078Fixture builds the umbrella workspace: umbrella + childA + childB,
// each a real repo with a real board, linked via .coding-hermes/links.
func bt078Fixture(t *testing.T) (umbrella, childA, childB string) {
	t.Helper()
	root := t.TempDir()
	umbrella = filepath.Join(root, "umbrella")
	childA = filepath.Join(root, "childA")
	childB = filepath.Join(root, "childB")
	bt078InitChild(t, umbrella)
	bt078InitChild(t, childA)
	bt078InitChild(t, childB)
	links := filepath.Join(umbrella, ".coding-hermes", "links")
	if err := os.MkdirAll(links, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, l := range []struct{ name, target string }{
		{"childA", filepath.Join(childA, ".coding-hermes")},
		{"childB", filepath.Join(childB, ".coding-hermes")},
	} {
		if err := os.Symlink(l.target, filepath.Join(links, l.name)); err != nil {
			t.Fatal(err)
		}
	}
	return umbrella, childA, childB
}

// bt078HashBoardFile sha256s one file ("" when absent).
func bt078HashBoardFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "<absent>"
		}
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// bt078AllBoardHashes hashes every tracked board file of the umbrella's own
// board plus every child board reachable through the registry (AC5's
// snapshot). The .lock sidecar is excluded — a READ must never need the
// write guard, but the lock file is not board content; its presence or not
// says nothing about the read-only guarantee.
func bt078AllBoardHashes(t *testing.T, umbrella string) map[string]string {
	t.Helper()
	boards := []string{filepath.Join(umbrella, ".coding-hermes", "board")}
	links := filepath.Join(umbrella, ".coding-hermes", "links")
	entries, err := os.ReadDir(links)
	if err == nil {
		for _, e := range entries {
			target, err := filepath.EvalSymlinks(filepath.Join(links, e.Name()))
			if err != nil {
				continue
			}
			boards = append(boards, filepath.Join(target, "board"))
		}
	}
	out := map[string]string{}
	for _, bd := range boards {
		for _, name := range []string{"tasks.jsonl", "events.jsonl", "board.jsonl", "fixtures.jsonl"} {
			out[bd+"/"+name] = bt078HashBoardFile(t, filepath.Join(bd, name))
		}
	}
	return out
}

// TestBT078UmbrellaCensusE2E (AC2): the hand-built temp umbrella lists both
// child boards with per-repo task counts and provenance; exit 0.
func TestBT078UmbrellaCensusE2E(t *testing.T) {
	umbrella, childA, childB := bt078Fixture(t)
	bt078Create(t, childA, "CHILDA-1", "childA one")
	bt078Create(t, childA, "CHILDA-2", "childA two")
	bt078Create(t, childB, "CHILDB-1", "childB one")

	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella"})
	if code != 0 {
		t.Fatalf("umbrella exit = %d, want 0 (stderr: %s)", code, out)
	}
	if !strings.Contains(out, "UMBRELLA "+umbrella) {
		t.Fatalf("census missing umbrella header: %s", out)
	}
	if !strings.Contains(out, "childA") || !strings.Contains(out, "childB") {
		t.Fatalf("census missing child boards: %s", out)
	}
	// Per-repo task counts: childA seeded 2 (+1 NEVER-DONE fixture row on
	// disk), childB 1 (+1), umbrella 0 (+1). Census counts RAW task rows.
	if !strings.Contains(out, fmt.Sprintf("%-20s", "childA")) {
		t.Fatalf("census lacks childA row: %s", out)
	}
	if n := strings.Count(out, "task(s)"); n != 3 {
		t.Fatalf("census shows %d board rows, want 3: %s", n, out)
	}
	// Provenance: the child board dirs appear.
	if !strings.Contains(out, filepath.Join(childA, ".coding-hermes", "board")) {
		t.Fatalf("census lacks childA board dir provenance: %s", out)
	}
}

// TestBT078UmbrellaListCollidingIDs (AC3): --list shows the same id twice
// under different repo qualifiers.
func TestBT078UmbrellaListCollidingIDs(t *testing.T) {
	umbrella, childA, childB := bt078Fixture(t)
	bt078Create(t, childA, "SHARED-1", "the childA copy")
	bt078Create(t, childB, "SHARED-1", "the childB copy")

	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--list"})
	if code != 0 {
		t.Fatalf("umbrella --list exit = %d, want 0 (stderr: %s)", code, out)
	}
	if strings.Count(out, "SHARED-1") < 2 {
		t.Fatalf("--list shows %d SHARED-1 rows, want 2 distinct: %s", strings.Count(out, "SHARED-1"), out)
	}
	// Both copies carry the REPO column values.
	lines := strings.Split(out, "\n")
	childACopy, childBCopy := false, false
	for _, l := range lines {
		if strings.Contains(l, "SHARED-1") && strings.Contains(l, "childA") {
			childACopy = true
		}
		if strings.Contains(l, "SHARED-1") && strings.Contains(l, "childB") {
			childBCopy = true
		}
	}
	if !childACopy || !childBCopy {
		t.Fatalf("colliding rows lack repo provenance (childA=%v childB=%v): %s", childACopy, childBCopy, out)
	}
}

// TestBT078UmbrellaAmbiguousDepReported (AC3): --validate reports the
// ambiguous cross-board reference with its candidates and exit 1, never a
// silent binding.
func TestBT078UmbrellaAmbiguousDepReported(t *testing.T) {
	umbrella, childA, childB := bt078Fixture(t)
	bt078Create(t, childA, "SHARED-1", "childA copy")
	bt078Create(t, childB, "SHARED-1", "childB copy")
	// The referencing row lives in the umbrella board (no local SHARED-1):
	// a raw row, because create's dangling-dep gate rightly refuses.
	bt078AppendRawTask(t, umbrella, "UMB-DEP", "needs shared", "SHARED-1")

	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--validate"})
	if code != 1 {
		t.Fatalf("umbrella --validate exit = %d, want 1 on ambiguity (out: %s)", code, out)
	}
	if !strings.Contains(out, "AMBIGUOUS") {
		t.Fatalf("--validate output lacks the AMBIGUOUS verdict: %s", out)
	}
	if !strings.Contains(out, "childA/SHARED-1") || !strings.Contains(out, "childB/SHARED-1") {
		t.Fatalf("ambiguity must name candidate repos: %s", out)
	}
	if !strings.Contains(out, "umbrella/UMB-DEP") {
		t.Fatalf("ambiguity must name the referencing task: %s", out)
	}
}

// bt078AppendRawTask writes one verbatim task row into a repo's tasks.jsonl
// (the create gate rightly refuses cross-repo dangling deps).
func bt078AppendRawTask(t *testing.T, repo, id, title, dep string) {
	t.Helper()
	bd := filepath.Join(repo, ".coding-hermes", "board")
	row, err := json.Marshal(map[string]any{
		"id": id, "title": title, "status": "pending", "priority": "P2",
		"depends_on": []string{dep},
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(bd, "tasks.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck
	if _, err := f.Write(append(row, '\n')); err != nil {
		t.Fatal(err)
	}
}

// bt078Run drives run() and returns combined stdout+stderr plus the exit
// code. stderr is ALSO kept verbatim in bt078LastStderr for assertions that
// must pin the findings to the stderr channel specifically.
func bt078Run(t *testing.T, args []string) (string, int) {
	t.Helper()
	stdout, err := captureStdout(func() {
		stderr, e := captureStderrString(func() {
			bt078LastCode = run(args)
		})
		if e != nil {
			t.Fatal(e)
		}
		bt078LastStderr = stderr
	})
	if err != nil {
		t.Fatal(err)
	}
	return stdout + bt078LastStderr, bt078LastCode
}

// bt078LastCode / bt078LastStderr carry the most recent bt078Run results.
var (
	bt078LastCode   int
	bt078LastStderr string
)

// captureStderrString captures stderr during fn (see captureStdout in
// main_test.go for the stdout twin).
func captureStderrString(fn func()) (string, error) {
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 0, 8192)
		tmp := make([]byte, 1024)
		for {
			n, rerr := r.Read(tmp)
			buf = append(buf, tmp[:n]...)
			if rerr != nil {
				break
			}
		}
		done <- string(buf)
	}()
	fn()
	_ = w.Close()
	return <-done, nil
}

// TestBT078UmbrellaCrossRepoDepResolves: a UNIQUE cross-repo reference
// resolves qualified (no ambiguity), exit 0.
func TestBT078UmbrellaCrossRepoDepResolves(t *testing.T) {
	umbrella, childA, childB := bt078Fixture(t)
	bt078Create(t, childB, "ONLY-B-1", "unique in childB")
	bt078AppendRawTask(t, childA, "CHILDA-X", "cross-repo dependent", "ONLY-B-1")

	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--validate"})
	if code != 0 {
		t.Fatalf("umbrella --validate exit = %d, want 0 (out: %s)", code, out)
	}
	if !strings.Contains(out, "childA/CHILDA-X -> childB/ONLY-B-1 (cross-repo)") {
		t.Fatalf("cross-repo resolution not rendered qualified: %s", out)
	}
}

// TestBT078UmbrellaBrokenAndLoopLinksExit1 (AC4): each is a named finding,
// exit 1, and the command terminates (the test itself bounds the hang risk).
func TestBT078UmbrellaBrokenAndLoopLinksExit1(t *testing.T) {
	umbrella, childA, _ := bt078Fixture(t)
	bt078Create(t, childA, "CHILDA-1", "childA one")
	links := filepath.Join(umbrella, ".coding-hermes", "links")

	// Broken link.
	if err := os.Symlink(filepath.Join(umbrella, "ghost", ".coding-hermes"), filepath.Join(links, "broken")); err != nil {
		t.Fatal(err)
	}
	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella"})
	if code != 1 {
		t.Fatalf("broken-link census exit = %d, want 1", code)
	}
	if !strings.Contains(out, "link broken") || !strings.Contains(out, "broken symlink") {
		t.Fatalf("broken link finding not named: %s", out)
	}
	// stderr carries the finding (stdout stays the census).
	if !strings.Contains(bt078LastStderr, "[error] link broken") {
		t.Fatalf("findings must render on stderr: %q", bt078LastStderr)
	}

	// Loop link.
	if err := os.Symlink("loopB", filepath.Join(links, "loopA")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("loopA", filepath.Join(links, "loopB")); err != nil {
		t.Fatal(err)
	}
	out, code = bt078Run(t, []string{"-C", umbrella, "umbrella"})
	if code != 1 {
		t.Fatalf("loop-link census exit = %d, want 1", code)
	}
	if !strings.Contains(out, "symlink loop") {
		t.Fatalf("loop finding not named: %s", out)
	}
	if !strings.Contains(out, "link loopA") || !strings.Contains(out, "link loopB") {
		t.Fatalf("loop findings must name both loop members: %s", out)
	}
}

// TestBT078UmbrellaZeroWritesThroughLinks (AC5): a full read sweep leaves
// every child board file byte-identical (sha256 before/after).
func TestBT078UmbrellaZeroWritesThroughLinks(t *testing.T) {
	umbrella, childA, childB := bt078Fixture(t)
	bt078Create(t, childA, "CHILDA-1", "childA one")
	bt078Create(t, childB, "CHILDB-1", "childB one")

	before := bt078AllBoardHashes(t, umbrella)
	if len(before) != 12 {
		t.Fatalf("snapshot covered %d files, want 12 (3 boards x 4 files)", len(before))
	}
	for _, mode := range [][]string{
		{"umbrella"},
		{"umbrella", "--list"},
		{"umbrella", "--stats"},
		{"umbrella", "--validate"},
		{"umbrella", "--json"},
	} {
		args := append([]string{"-C", umbrella}, mode...)
		if _, code := bt078Run(t, args); code != 0 {
			t.Fatalf("umbrella %v exit = %d, want 0", mode, code)
		}
	}
	after := bt078AllBoardHashes(t, umbrella)
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("file %s changed through an umbrella read: before %s after %s", k, v, after[k])
		}
	}
}

// TestBT078DirectRootBehaviorUnchanged (AC6): plain single-repo commands on
// a child repo behave exactly as before the umbrella work — list renders the
// standard table, validate exits 0 with the standard report, and `create`
// still appends.
func TestBT078DirectRootBehaviorUnchanged(t *testing.T) {
	_, childA, _ := bt078Fixture(t)
	bt078Create(t, childA, "CHILDA-1", "childA one")

	out, code := bt078Run(t, []string{"-C", childA, "list"})
	if code != 0 {
		t.Fatalf("list exit = %d, want 0", code)
	}
	// The standard list header (BT-037-era byte shape) — no REPO column.
	if !strings.Contains(out, "ID") || !strings.Contains(out, "PRI") || !strings.Contains(out, "STATUS") || !strings.Contains(out, "TITLE") {
		t.Fatalf("list header changed: %q", out)
	}
	if strings.Contains(out, "REPO") {
		t.Fatalf("direct-root list must NOT carry the umbrella REPO column: %q", out)
	}
	if !strings.Contains(out, "CHILDA-1") || !strings.Contains(out, "childA one") {
		t.Fatalf("list lost the task row: %q", out)
	}

	out, code = bt078Run(t, []string{"-C", childA, "validate"})
	if code != 0 {
		t.Fatalf("validate exit = %d, want 0 (out: %s)", code, out)
	}
	if !strings.Contains(out, "RESULT: OK") || !strings.Contains(out, "topology A") {
		t.Fatalf("validate report shape changed: %s", out)
	}
	if !strings.Contains(out, "key uniformity:") {
		t.Fatalf("validate census line missing: %s", out)
	}

	// create still appends through the ordinary path.
	if code := run([]string{"-C", childA, "create", "--id", "CHILDA-2", "--title", "second"}); code != 0 {
		t.Fatalf("direct create exit = %d, want 0", code)
	}
	data, err := os.ReadFile(filepath.Join(childA, ".coding-hermes", "board", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "CHILDA-2") {
		t.Fatal("direct create did not append")
	}
}

// TestBT078UmbrellaJSONCensus: --json emits the machine census with
// provenance and zero findings on a healthy umbrella.
func TestBT078UmbrellaJSONCensus(t *testing.T) {
	umbrella, childA, _ := bt078Fixture(t)
	bt078Create(t, childA, "CHILDA-1", "childA one")

	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--json"})
	if code != 0 {
		t.Fatalf("umbrella --json exit = %d, want 0 (out: %s)", code, out)
	}
	var payload struct {
		Root     string `json:"root"`
		LinksDir string `json:"links_dir"`
		Boards   []struct {
			Repo     string `json:"repo"`
			BoardDir string `json:"board_dir"`
			Topology string `json:"topology"`
			Tasks    int    `json:"tasks"`
		} `json:"boards"`
		Findings []map[string]any `json:"findings"`
		Total    int              `json:"total"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &payload); err != nil {
		t.Fatalf("--json output does not parse: %v (%s)", err, out)
	}
	if payload.Root != umbrella || payload.LinksDir == "" {
		t.Fatalf("json root/links_dir wrong: %+v", payload)
	}
	if len(payload.Boards) != 3 {
		t.Fatalf("json boards = %d, want 3: %+v", len(payload.Boards), payload.Boards)
	}
	// Raw task-row census includes the NEVER-DONE fixture row: umbrella 1,
	// childA 2, childB 1 -> total 4.
	if payload.Total != 4 {
		t.Fatalf("json total = %d, want 4", payload.Total)
	}
	if len(payload.Findings) != 0 {
		t.Fatalf("healthy umbrella carries findings: %+v", payload.Findings)
	}
	// Provenance in the machine surface too.
	foundA := false
	for _, b := range payload.Boards {
		if b.Repo == "childA" && b.BoardDir == filepath.Join(childA, ".coding-hermes", "board") && b.Tasks == 2 {
			foundA = true
		}
	}
	if !foundA {
		t.Fatalf("json board childA provenance wrong: %+v", payload.Boards)
	}
}

// TestBT078UmbrellaStrict: --strict fails on warning-class findings (the
// missing-registry warning is always present when the umbrella has no own
// board context — here we force the no-registry warning by pointing at a
// plain board repo).
func TestBT078UmbrellaStrict(t *testing.T) {
	dir := t.TempDir()
	bt078InitChild(t, dir) // board but NO links registry

	if _, code := bt078Run(t, []string{"-C", dir, "umbrella"}); code != 0 {
		t.Fatalf("no-registry census exit = %d, want 0 (warn-only)", code)
	}
	if _, code := bt078Run(t, []string{"-C", dir, "umbrella", "--strict"}); code != 1 {
		t.Fatalf("--strict exit = %d, want 1 (the no-registry warning fails)", code)
	}
}

// TestBT078UmbrellaModeConflicts: mode flags are mutually exclusive and
// --json refuses combination (exit 2 usage class).
func TestBT078UmbrellaModeConflicts(t *testing.T) {
	umbrella, _, _ := bt078Fixture(t)
	for _, args := range [][]string{
		{"-C", umbrella, "umbrella", "--list", "--stats"},
		{"-C", umbrella, "umbrella", "--stats", "--validate"},
		{"-C", umbrella, "umbrella", "--list", "--validate"},
		{"-C", umbrella, "umbrella", "--json", "--list"},
		{"-C", umbrella, "umbrella", "--json", "--stats"},
		{"-C", umbrella, "umbrella", "--json", "--validate"},
	} {
		if _, code := bt078Run(t, args); code != 2 {
			t.Fatalf("umbrella %v exit = %d, want 2 (usage)", args[2:], code)
		}
	}
	if _, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--help"}); code != 0 {
		t.Fatal("umbrella --help must exit 0 (BT-041 contract)")
	}
	if _, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "surplus"}); code != 1 {
		t.Fatalf("umbrella positional arg exit = %d, want 1", code)
	}
}

// TestBT078UmbrellaStatsE2E: --stats renders per-repo stats plus grand
// totals.
func TestBT078UmbrellaStatsE2E(t *testing.T) {
	umbrella, childA, childB := bt078Fixture(t)
	bt078Create(t, childA, "CHILDA-1", "one")
	bt078Create(t, childB, "CHILDB-1", "two")
	bt078Create(t, childB, "CHILDB-2", "three")

	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--stats"})
	if code != 0 {
		t.Fatalf("umbrella --stats exit = %d, want 0 (out: %s)", code, out)
	}
	if !strings.Contains(out, "repo childA") || !strings.Contains(out, "repo childB") {
		t.Fatalf("stats lack per-repo sections: %s", out)
	}
	if !strings.Contains(out, "umbrella total:") {
		t.Fatalf("stats lack the grand total: %s", out)
	}
	if !strings.Contains(out, "total tasks: 3") {
		t.Fatalf("grand total wrong: %s", out)
	}
	if !strings.Contains(out, "pending") {
		t.Fatalf("status tally missing: %s", out)
	}
}

// TestBT078UmbrellaNotABoard (exit 2 class): a target with neither board nor
// registry is the ordinary board-not-found usage failure.
func TestBT078UmbrellaNotABoard(t *testing.T) {
	dir := t.TempDir()
	_, code := bt078Run(t, []string{"-C", dir, "umbrella"})
	if code != 2 {
		t.Fatalf("umbrella on a bare dir exit = %d, want 2", code)
	}
	if !strings.Contains(bt078LastStderr, "no JSONL foreman board found") {
		t.Fatalf("board-not-found wording missing: %q", bt078LastStderr)
	}
}

// TestBT078UmbrellaAliasDedupeE2E: two links to one child = one board + a
// dedupe warning (stderr), exit 0.
func TestBT078UmbrellaAliasDedupeE2E(t *testing.T) {
	umbrella, childA, _ := bt078Fixture(t)
	links := filepath.Join(umbrella, ".coding-hermes", "links")
	if err := os.Symlink(filepath.Join(childA, ".coding-hermes"), filepath.Join(links, "childA-alias")); err != nil {
		t.Fatal(err)
	}
	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella"})
	if code != 0 {
		t.Fatalf("alias census exit = %d, want 0", code)
	}
	if n := strings.Count(out, "task(s)"); n != 3 {
		t.Fatalf("alias produced %d board rows, want 3 (dedupe): %s", n, out)
	}
	if !strings.Contains(bt078LastStderr, "deduplicated") || !strings.Contains(bt078LastStderr, "childA-alias") {
		t.Fatalf("dedupe warning missing: %q", bt078LastStderr)
	}
}
