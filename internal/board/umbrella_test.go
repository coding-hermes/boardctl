package board

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- BT-078 library tests: umbrella discovery + unified view ----------
//
// Fixtures are REAL boards bootstrapped through the real Init path in
// t.TempDir (house pattern), linked from an umbrella's .coding-hermes/links/
// registry. Every test asserts read-only behavior: the umbrella surface has
// no write verb, and the dedicated zero-write proof hashes every child board
// file before/after a full read sweep.

// umbInitBoard bootstraps a real board through Init and returns the repo root.
func umbInitBoard(t *testing.T, dir string) string {
	t.Helper()
	if _, _, err := Init(dir, InitOptions{Project: filepath.Base(dir)}); err != nil {
		t.Fatalf("init %s: %v", dir, err)
	}
	return dir
}

// umbSeedTask appends one task row directly to a board's tasks.jsonl via the
// write path (b.Create), keeping fixtures intact.
func umbSeedTask(t *testing.T, repo, id, title string, deps ...string) {
	t.Helper()
	b, err := Resolve(repo)
	if err != nil {
		t.Fatal(err)
	}
	spec := TaskRowSpec{
		ID:           id,
		Title:        title,
		Status:       "pending",
		Priority:     "P2",
		HasDependsOn: len(deps) > 0,
		DependsOn:    deps,
	}
	if _, err := b.Create(spec); err != nil {
		t.Fatalf("create %s in %s: %v", id, repo, err)
	}
}

// umbSeedTaskRaw writes one task row verbatim into tasks.jsonl — the path
// for CROSS-REPO depends_on references, which create's dangling-dep gate
// correctly refuses (the id does not exist in the OWNING repo). A cross-repo
// relation is exactly the shape that lands on a real board via a
// hand-written or imported row, and the umbrella view is what reads it.
func umbSeedTaskRaw(t *testing.T, repo, id, title string, deps ...string) {
	t.Helper()
	b, err := Resolve(repo)
	if err != nil {
		t.Fatal(err)
	}
	row := map[string]any{
		"id": id, "title": title, "status": "pending", "priority": "P2",
		"depends_on": deps,
	}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(b.TasksPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck
	if _, err := f.Write(append(data, '\n')); err != nil {
		t.Fatal(err)
	}
}

// umbGitInit makes dir a real git repository (the repo-boundary gate only
// accepts link targets inside a determinable repo). No commit is needed —
// findRepoRoot keys on the .git entry.
func umbGitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v (%s)", dir, err, out)
	}
}

// umbFixture builds umbrella/ with childA and childB child repos (real git
// repos), real boards in each, and the links registry pointing at both
// children's .coding-hermes dirs. Returns the umbrella root and child roots.
func umbFixture(t *testing.T) (umbrella, childA, childB string) {
	t.Helper()
	root := t.TempDir()
	umbrella = filepath.Join(root, "umbrella")
	childA = filepath.Join(root, "childA")
	childB = filepath.Join(root, "childB")
	for _, d := range []string{umbrella, childA, childB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		umbGitInit(t, d)
	}
	umbInitBoard(t, umbrella)
	umbInitBoard(t, childA)
	umbInitBoard(t, childB)
	links := filepath.Join(umbrella, ".coding-hermes", "links")
	if err := os.MkdirAll(links, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(childA, ".coding-hermes"), filepath.Join(links, "childA")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(childB, ".coding-hermes"), filepath.Join(links, "childB")); err != nil {
		t.Fatal(err)
	}
	return umbrella, childA, childB
}

// umbrellaBoardFileHashes hashes every file of every board dir reachable
// through the registry plus the umbrella's own board (the AC5 snapshot).
func umbrellaBoardFileHashes(t *testing.T, umbrella string) map[string]string {
	t.Helper()
	out := map[string]string{}
	roots := []string{filepath.Join(umbrella, ".coding-hermes", "board")}
	links := filepath.Join(umbrella, ".coding-hermes", "links")
	entries, err := os.ReadDir(links)
	if err == nil {
		for _, e := range entries {
			target, err := filepath.EvalSymlinks(filepath.Join(links, e.Name()))
			if err != nil {
				continue
			}
			roots = append(roots, filepath.Join(target, "board"))
		}
	}
	for _, root := range roots {
		names, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, n := range names {
			if n.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, n.Name()))
			if err != nil {
				t.Fatal(err)
			}
			// hashDirName + "/" + name so per-board files cannot collide.
			sum := uint64(0)
			for _, c := range data {
				sum = sum*31 + uint64(c)
			}
			out[root+"/"+n.Name()] = strings.Repeat("x", 0) + itou(sum)
		}
	}
	return out
}

func itou(v uint64) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}

// TestUmbrellaDiscoveryTwoChildren (AC2): the view holds the umbrella's own
// board first, then both children by qualifier, each carrying provenance.
func TestUmbrellaDiscoveryTwoChildren(t *testing.T) {
	umbrella, childA, childB := umbFixture(t)
	umbSeedTask(t, childA, "CHILDA-1", "childA task")
	umbSeedTask(t, childA, "CHILDA-2", "childA task 2")
	umbSeedTask(t, childB, "CHILDB-1", "childB task")

	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	if len(um.Boards) != 3 {
		t.Fatalf("boards in view = %d, want 3 (umbrella + childA + childB): %+v", len(um.Boards), um.Boards)
	}
	if um.Boards[0].Repo != "umbrella" {
		t.Errorf("first board repo = %q, want the umbrella's own %q", um.Boards[0].Repo, "umbrella")
	}
	if um.Boards[1].Repo != "childA" || um.Boards[2].Repo != "childB" {
		t.Errorf("child order = [%s %s], want [childA childB] (sorted qualifiers)", um.Boards[1].Repo, um.Boards[2].Repo)
	}
	if um.Boards[1].RepoRoot != childA {
		t.Errorf("childA repo_root = %q, want %q", um.Boards[1].RepoRoot, childA)
	}
	if um.Boards[1].BoardDir != filepath.Join(childA, ".coding-hermes", "board") {
		t.Errorf("childA board dir = %q", um.Boards[1].BoardDir)
	}
	if um.Boards[1].Topology != "A" {
		t.Errorf("childA topology = %q, want A", um.Boards[1].Topology)
	}
	if um.HasErrors() {
		t.Fatalf("healthy fixture produced error findings: %+v", um.Findings)
	}

	// Task counts per repo. NEVER-DONE (the init seed row) is a fixture id,
	// excluded from the default filter on every board — same as `list`.
	counts := map[string]int{}
	qts, err := um.ListTasks(TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, qt := range qts {
		counts[qt.Repo]++
	}
	if counts["umbrella"] != 0 || counts["childA"] != 2 || counts["childB"] != 1 {
		t.Fatalf("task counts = %v, want umbrella:0 childA:2 childB:1 (NEVER-DONE is fixture-excluded)", counts)
	}
	// Provenance rides every row.
	for _, qt := range qts {
		if qt.Board.BoardDir == "" {
			t.Fatalf("row %s carries no board provenance", qt.Row.String("id"))
		}
	}
}

// TestUmbrellaCollidingIDsStayDistinct (AC3): the same raw id in two child
// boards appears twice, distinguishable by repo qualifier, and never merges.
func TestUmbrellaCollidingIDsStayDistinct(t *testing.T) {
	umbrella, childA, childB := umbFixture(t)
	umbSeedTask(t, childA, "SHARED-1", "the childA copy")
	umbSeedTask(t, childB, "SHARED-1", "the childB copy")

	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	qts, err := um.ListTasks(TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	var hits []QualifiedTask
	for _, qt := range qts {
		if qt.Row.String("id") == "SHARED-1" {
			hits = append(hits, qt)
		}
	}
	if len(hits) != 2 {
		t.Fatalf("SHARED-1 rows in view = %d, want 2 (distinct boards, never merged)", len(hits))
	}
	if hits[0].Repo == hits[1].Repo {
		t.Fatalf("both SHARED-1 rows from repo %q — provenance lost", hits[0].Repo)
	}
	if QualifiedTaskID(hits[0].Repo, "SHARED-1") == QualifiedTaskID(hits[1].Repo, "SHARED-1") {
		t.Fatal("repo-qualified identities collided")
	}
	if QualifiedTaskID("childA", "SHARED-1") != "childA/SHARED-1" {
		t.Fatalf("QualifiedTaskID = %q, want childA/SHARED-1", QualifiedTaskID("childA", "SHARED-1"))
	}
}

// TestUmbrellaDepAmbiguityReported (AC3): a depends_on id that exists in
// several boards and NOT in the referencing repo is classified ambiguous
// with candidates — never bound to one repo.
func TestUmbrellaDepAmbiguityReported(t *testing.T) {
	umbrella, childA, childB := umbFixture(t)
	umbSeedTask(t, childA, "SHARED-1", "childA copy")
	umbSeedTask(t, childB, "SHARED-1", "childB copy")
	// The referencing task sits in the umbrella's own board, which holds NO
	// SHARED-1 — so the reference cannot resolve locally.
	umbSeedTaskRaw(t, umbrella, "UMB-DEP", "needs the ambiguous one", "SHARED-1")

	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	deps, findings, err := um.DepResolutions()
	if err != nil {
		t.Fatal(err)
	}
	var amb *DepResolution
	for i := range deps {
		if deps[i].Status == "ambiguous" {
			amb = &deps[i]
		}
	}
	if amb == nil {
		t.Fatalf("no ambiguous resolution found: %+v", deps)
	}
	if amb.Repo != "umbrella" || amb.TaskID != "UMB-DEP" || amb.Dep != "SHARED-1" {
		t.Fatalf("ambiguous row misattributed: %+v", amb)
	}
	if len(amb.Candidates) != 2 || amb.Candidates[0] != "childA" || amb.Candidates[1] != "childB" {
		t.Fatalf("candidates = %v, want [childA childB] sorted", amb.Candidates)
	}
	if amb.ResolvedRepo != "" {
		t.Fatalf("ambiguous reference was silently bound to %q", amb.ResolvedRepo)
	}
	if len(findings) == 0 {
		t.Fatal("ambiguity produced no finding")
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.Msg, "ambiguous") && strings.Contains(f.Msg, "umbrella/UMB-DEP") {
			found = true
		}
	}
	if !found {
		t.Fatalf("ambiguity finding does not name the referencing task: %+v", findings)
	}
}

// TestUmbrellaDepLocalAndCrossResolution: repo-local resolution wins when the
// id exists locally (with an "also in" warning); a unique cross-repo id
// resolves qualified.
func TestUmbrellaDepLocalAndCrossResolution(t *testing.T) {
	umbrella, childA, childB := umbFixture(t)
	umbSeedTask(t, childA, "LOCAL-1", "local anchor")
	umbSeedTask(t, childB, "LOCAL-1", "same id elsewhere")
	umbSeedTask(t, childA, "CHILDA-A", "refs local", "LOCAL-1")
	umbSeedTaskRaw(t, childA, "CHILDA-B", "refs cross-repo", "ONLY-B-1")
	umbSeedTask(t, childB, "ONLY-B-1", "unique in childB")

	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	deps, findings, err := um.DepResolutions()
	if err != nil {
		t.Fatal(err)
	}
	byDep := map[string]DepResolution{}
	for _, d := range deps {
		byDep[d.TaskID+"/"+d.Dep] = d
	}
	local, ok := byDep["CHILDA-A/LOCAL-1"]
	if !ok || local.Status != "local" || local.ResolvedRepo != "childA" {
		t.Fatalf("local resolution = %+v", local)
	}
	if len(local.AlsoIn) != 1 || local.AlsoIn[0] != "childB" {
		t.Fatalf("AlsoIn = %v, want [childB]", local.AlsoIn)
	}
	cross, ok := byDep["CHILDA-B/ONLY-B-1"]
	if !ok || cross.Status != "cross" || cross.ResolvedRepo != "childB" {
		t.Fatalf("cross resolution = %+v", cross)
	}
	// The also-in case must warn (a binding that LOOKS silent).
	alsoWarned := false
	for _, f := range findings {
		if strings.Contains(f.Msg, "also exists in childB") {
			alsoWarned = true
		}
	}
	if !alsoWarned {
		t.Fatalf("repo-local binding with cross-board twin produced no warning: %+v", findings)
	}
}

// TestUmbrellaBrokenAndLoopingLinks (AC4): each produces a named ERROR
// finding and the healthy board still loads. No hang, no unbounded follow.
func TestUmbrellaBrokenAndLoopingLinks(t *testing.T) {
	umbrella, _, _ := umbFixture(t)
	links := filepath.Join(umbrella, ".coding-hermes", "links")

	// Broken: target path does not exist.
	if err := os.Symlink(filepath.Join(umbrella, "no-such-child", ".coding-hermes"), filepath.Join(links, "broken")); err != nil {
		t.Fatal(err)
	}
	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	var brokenFound bool
	for _, f := range um.Findings {
		if f.Link == "broken" && f.IsError() && strings.Contains(f.Msg, "broken symlink") {
			brokenFound = true
		}
	}
	if !brokenFound {
		t.Fatalf("broken link produced no named error finding: %+v", um.Findings)
	}
	if !um.HasErrors() {
		t.Fatal("broken link must be an error-class finding")
	}
	repos := map[string]bool{}
	for _, qb := range um.Boards {
		repos[qb.Repo] = true
	}
	if !repos["childA"] {
		t.Fatal("healthy childA vanished from the view because a sibling link was broken")
	}

	// Loop: linkA -> linkB -> linkA.
	if err := os.Symlink("linkB", filepath.Join(links, "linkA")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("linkA", filepath.Join(links, "linkB")); err != nil {
		t.Fatal(err)
	}
	um2, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	loopA, loopB := false, false
	for _, f := range um2.Findings {
		if !f.IsError() || !strings.Contains(f.Msg, "symlink loop") {
			continue
		}
		if f.Link == "linkA" {
			loopA = true
		}
		if f.Link == "linkB" {
			loopB = true
		}
	}
	if !loopA || !loopB {
		t.Fatalf("symlink loop findings missing (linkA=%v linkB=%v): %+v", loopA, loopB, um2.Findings)
	}
	// Still no hang, and the healthy boards are intact.
	if len(um2.Boards) < 3 {
		t.Fatalf("loop killed the healthy view: %d boards", len(um2.Boards))
	}
}

// TestUmbrellaLinkOutsideRepoRefused: a target with no .git ancestor is
// refused, not followed.
func TestUmbrellaLinkOutsideRepoRefused(t *testing.T) {
	umbrella, _, _ := umbFixture(t)
	links := filepath.Join(umbrella, ".coding-hermes", "links")
	// A real directory with a real board, but outside any git repo.
	bare := filepath.Join(t.TempDir(), "notarepo")
	umbInitBoard(t, bare)
	if err := os.Symlink(bare, filepath.Join(links, "notarepo")); err != nil {
		t.Fatal(err)
	}
	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range um.Findings {
		if f.Link == "notarepo" && f.IsError() && strings.Contains(f.Msg, "not inside a git repository") {
			found = true
		}
	}
	if !found {
		t.Fatalf("out-of-repo link not refused with a named finding: %+v", um.Findings)
	}
	for _, qb := range um.Boards {
		if qb.Repo == "notarepo" {
			t.Fatal("out-of-repo link was followed into the view")
		}
	}
}

// TestUmbrellaOverDeepChainRefused: a chain longer than the hop cap is a
// finding, never followed to the end.
func TestUmbrellaOverDeepChainRefused(t *testing.T) {
	umbrella, childA, _ := umbFixture(t)
	links := filepath.Join(umbrella, ".coding-hermes", "links")
	// childA/.coding-hermes via a chain of MaxUmbrellaSymlinkDepth+1 hops.
	prev := filepath.Join(childA, ".coding-hermes")
	for i := 0; i <= MaxUmbrellaSymlinkDepth; i++ {
		hop := filepath.Join(t.TempDir(), "hop"+itou(uint64(i)))
		if err := os.Symlink(prev, hop); err != nil {
			t.Fatal(err)
		}
		prev = hop
	}
	if err := os.Symlink(prev, filepath.Join(links, "deep")); err != nil {
		t.Fatal(err)
	}
	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range um.Findings {
		if f.Link == "deep" && f.IsError() && strings.Contains(f.Msg, "refused") {
			found = true
		}
	}
	if !found {
		t.Fatalf("over-deep chain not refused: %+v", um.Findings)
	}
}

// TestUmbrellaAliasDeduplicated: two links to the same target = one board,
// reported as a warning.
func TestUmbrellaAliasDeduplicated(t *testing.T) {
	umbrella, childA, _ := umbFixture(t)
	links := filepath.Join(umbrella, ".coding-hermes", "links")
	if err := os.Symlink(filepath.Join(childA, ".coding-hermes"), filepath.Join(links, "childA-alias")); err != nil {
		t.Fatal(err)
	}
	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, qb := range um.Boards {
		if qb.Repo == "childA" || qb.Repo == "childA-alias" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("alias links produced %d boards, want 1 deduplicated board", n)
	}
	warned := false
	for _, f := range um.Findings {
		if !f.IsError() && strings.Contains(f.Msg, "deduplicated") && strings.Contains(f.Msg, "childA-alias") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("alias dedupe produced no warning: %+v", um.Findings)
	}
}

// TestUmbrellaPartialChildBoardIsNamedError: a child whose board is missing
// events.jsonl is a named error, not silence.
func TestUmbrellaPartialChildBoardIsNamedError(t *testing.T) {
	umbrella, _, childB := umbFixture(t)
	// childB loses its events.jsonl -> partial.
	if err := os.Remove(filepath.Join(childB, ".coding-hermes", "board", "events.jsonl")); err != nil {
		t.Fatal(err)
	}
	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range um.Findings {
		if f.Link == "childB" && f.IsError() && strings.Contains(f.Msg, "partial board") && strings.Contains(f.Msg, "events.jsonl is missing") {
			found = true
		}
	}
	if !found {
		t.Fatalf("partial child board not named: %+v", um.Findings)
	}
}

// TestUmbrellaNonDirectoryTargetRefused: a link aimed at a FILE is refused.
func TestUmbrellaNonDirectoryTargetRefused(t *testing.T) {
	umbrella, _, _ := umbFixture(t)
	links := filepath.Join(umbrella, ".coding-hermes", "links")
	fileTarget := filepath.Join(umbrella, "plain.txt")
	if err := os.WriteFile(fileTarget, []byte("not a board\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(fileTarget, filepath.Join(links, "plain")); err != nil {
		t.Fatal(err)
	}
	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range um.Findings {
		if f.Link == "plain" && f.IsError() && strings.Contains(f.Msg, "not a directory") {
			found = true
		}
	}
	if !found {
		t.Fatalf("file-target link not refused: %+v", um.Findings)
	}
}

// TestUmbrellaReadSweepWritesNothing (AC5): a full read sweep over the view
// (list, stats, validate, dep resolution) leaves every child board file
// byte-identical.
func TestUmbrellaReadSweepWritesNothing(t *testing.T) {
	umbrella, childA, childB := umbFixture(t)
	umbSeedTask(t, childA, "CHILDA-1", "childA task")
	umbSeedTaskRaw(t, childB, "CHILDB-1", "childB task", "CHILDA-1") // cross-repo ref

	before := umbrellaBoardFileHashes(t, umbrella)

	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := um.ListTasks(TaskFilter{}); err != nil {
		t.Fatal(err)
	}
	if _, err := um.Stats(TaskFilter{}); err != nil {
		t.Fatal(err)
	}
	if _, err := um.ValidateAll(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := um.DepResolutions(); err != nil {
		t.Fatal(err)
	}

	after := umbrellaBoardFileHashes(t, umbrella)
	if len(before) == 0 {
		t.Fatal("snapshot covered no files — the premise is broken")
	}
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("file %s changed during a unified read (before %s, after %s)", k, v, after[k])
		}
	}
	if len(after) != len(before) {
		t.Fatalf("file set changed during a unified read: %d -> %d", len(before), len(after))
	}
}

// TestUmbrellaNoRegistryWarnsSingleBoard: a repo with no links registry
// degrades to its own board with a warning — never an error.
func TestUmbrellaNoRegistryWarnsSingleBoard(t *testing.T) {
	dir := t.TempDir()
	umbInitBoard(t, dir)
	um, err := OpenUmbrella(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(um.Boards) != 1 || um.Boards[0].Repo != filepath.Base(dir) {
		t.Fatalf("boards = %+v", um.Boards)
	}
	if um.HasErrors() {
		t.Fatalf("no-registry case must warn, not error: %+v", um.Findings)
	}
	if um.WarnCount() == 0 {
		t.Fatal("missing registry must produce a warning so the partial view is visible")
	}
}

// TestUmbrellaNotABoardAtAll: neither board nor registry = the ordinary
// board-not-found usage error (exit 2 class).
func TestUmbrellaNotABoardAtAll(t *testing.T) {
	dir := t.TempDir()
	_, err := OpenUmbrella(dir)
	if err == nil || !strings.Contains(err.Error(), ErrBoardNotFound.Error()) {
		t.Fatalf("err = %v, want a wrapped ErrBoardNotFound", err)
	}
}

// TestUmbrellaTargetNotDirErrors: a missing target errors (not a panic).
func TestUmbrellaTargetNotDirErrors(t *testing.T) {
	_, err := OpenUmbrella(filepath.Join(t.TempDir(), "absent"))
	if err == nil {
		t.Fatal("absent target must error")
	}
}

// TestUmbrellaResolveBoardDirTargetDirectly: -C pointed at the umbrella's
// .coding-hermes dir works too.
func TestUmbrellaTargetCHDirDirectly(t *testing.T) {
	umbrella, _, _ := umbFixture(t)
	um, err := OpenUmbrella(filepath.Join(umbrella, ".coding-hermes"))
	if err != nil {
		t.Fatal(err)
	}
	if len(um.Boards) != 3 {
		t.Fatalf("boards = %d, want 3", len(um.Boards))
	}
}
