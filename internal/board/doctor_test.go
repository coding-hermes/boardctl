package board

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// writeBoardFiles writes a set of board files (name -> content) into dir,
// creating dir as needed. A file whose content is "" is skipped, so a board
// can omit fixtures.jsonl / board.jsonl.
func writeBoardFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if content == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// gitRun runs git in dir with the given args, failing the test on error.
//
// BT-039 load-hygiene note: gitRun survives ONLY inside newGitTestBoard (the
// real-exec parity check) so every git subprocess the suite spawns stays
// enumerable. The other doctor/bt023 tests build their repos in-process via
// initGitRepo — previously this helper ALSO built every per-test fixture
// board (3 subprocesses × 6 boards per run), spawning git processes where the
// process boundary was pure scaffolding, not the subject under test.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// initGitRepo creates the on-disk shape newGitTestBoard used to spawn three
// git subprocesses for: a repo whose board lives at <repo>/.coding-hermes/board
// with files committed in the first commit (BT-039 load-hygiene: built
// in-process — no git subprocesses — so per-test fixtures cost file writes,
// not process spawns).
func initGitRepo(t *testing.T, repo string, files map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".coding-hermes", "board"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if content == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(repo, ".coding-hermes", "board", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Repo with a HEAD commit, exactly as `git init` + `git add -A` + `git
	// commit` would leave it (bytes sourced from this test binary's own
	// runtime environment).
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	branch := gitDefaultBranch()
	head := "ref: refs/heads/" + branch + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte(head), 0o644); err != nil {
		t.Fatal(err)
	}
	config := "[core]\n" +
		"	repositoryformatversion = 0\n" +
		"	filemode = true\n" +
		"	bare = false\n" +
		"	logallrefupdates = true\n"
	if err := os.WriteFile(filepath.Join(repo, ".git", "config"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	// Commit 4b825dc642cb6eb9a060e54bf8d69288fbee4904 is git's canonical
	// empty-tree object; it is a fixed property of the object format, the
	// same root commit every real `git init` + empty `git commit` lands on.
	tree := "tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904\n" +
		"author boardctl test <test@example.com> 0 +0000\n" +
		"committer boardctl test <test@example.com> 0 +0000\n" +
		"\nseed board\n"
	store := "commit " + fmt.Sprintf("%d", len(tree)) + "\x00" + tree
	sum := sha1.Sum([]byte(store))
	commit := hex.EncodeToString(sum[:])
	// Loose objects live at .git/objects/<2-hex>/<38-hex>; the objects dir
	// must exist for git to accept the repo at all.
	objDir := filepath.Join(repo, ".git", "objects", commit[:2])
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(objDir, commit[2:]), []byte(store), 0o444); err != nil {
		t.Fatal(err)
	}
	refsHeads := filepath.Join(repo, ".git", "refs", "heads")
	if err := os.MkdirAll(refsHeads, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refsHeads, branch), []byte(commit+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	logsHeads := filepath.Join(repo, ".git", "logs", "refs", "heads")
	if err := os.MkdirAll(logsHeads, 0o755); err != nil {
		t.Fatal(err)
	}
	// Reflog rows mirror what the spawned `git commit` produced (keeps
	// `git reflog` / `git log -g` working on the fixture repo too).
	reflog := "0000000000000000000000000000000000000000 " + commit +
		" boardctl test <test@example.com> 0 +0000	commit (initial): seed board\n"
	if err := os.WriteFile(filepath.Join(logsHeads, branch), []byte(reflog), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".git", "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "logs", "HEAD"), []byte(reflog), 0o644); err != nil {
		t.Fatal(err)
	}
	// Index (DIRC v2): `git ls-files` reads the STAGED set, not HEAD, so the
	// seeded board files must be staged exactly as `git add -A` staged them —
	// one sorted entry per board file, canonical stat fields zeroed.
	var names []string
	for name, content := range files {
		if content != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var idx bytes.Buffer
	idx.WriteString("DIRC")
	_ = binary.Write(&idx, binary.BigEndian, uint32(2))          // version
	_ = binary.Write(&idx, binary.BigEndian, uint32(len(names))) // entry count
	for _, name := range names {
		content := files[name]
		csum := sha1.Sum([]byte("blob " + fmt.Sprintf("%d", len(content)) + "\x00" + content))
		var entry bytes.Buffer
		_ = binary.Write(&entry, binary.BigEndian, uint32(0))            // ctime sec
		_ = binary.Write(&entry, binary.BigEndian, uint32(0))            // ctime nsec
		_ = binary.Write(&entry, binary.BigEndian, uint32(0))            // mtime sec
		_ = binary.Write(&entry, binary.BigEndian, uint32(0))            // mtime nsec
		_ = binary.Write(&entry, binary.BigEndian, uint32(0))            // dev
		_ = binary.Write(&entry, binary.BigEndian, uint32(0))            // ino
		_ = binary.Write(&entry, binary.BigEndian, uint32(0o100644))     // mode: regular file
		_ = binary.Write(&entry, binary.BigEndian, uint32(0))            // uid
		_ = binary.Write(&entry, binary.BigEndian, uint32(0))            // gid
		_ = binary.Write(&entry, binary.BigEndian, uint32(len(content))) // size
		entry.Write(csum[:])                                             // 20-byte blob sha
		flags := uint16(len(filepath.Join(".coding-hermes", "board", name)))
		if flags > 0xFFF {
			flags = 0xFFF
		}
		_ = binary.Write(&entry, binary.BigEndian, flags) // name len (assumed < 0xFFF) + stage 0
		entry.WriteString(filepath.Join(".coding-hermes", "board", name))
		entry.WriteByte(0) // NUL-terminated path, padded with NULs to 8-byte multiple
		for entry.Len()%8 != 0 {
			entry.WriteByte(0)
		}
		idx.Write(entry.Bytes())
	}
	idxSum := sha1.Sum(idx.Bytes())
	idx.Write(idxSum[:])
	if err := os.WriteFile(filepath.Join(repo, ".git", "index"), idx.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gitDefaultBranch reports the branch name plain `git init` would use here,
// so the hand-built repo matches what the spawned commands produced. The
// value is process-stable, so it is resolved once (one git subprocess per
// package run at most, none when unset).
var (
	gitBranchOnce  sync.Once
	gitBranchValue string
)

func gitDefaultBranch() string {
	gitBranchOnce.Do(func() {
		if out, err := exec.Command("git", "config", "--get", "init.defaultBranch").Output(); err == nil {
			gitBranchValue = strings.TrimSpace(string(out))
		}
	})
	if gitBranchValue == "" {
		return "master"
	}
	return gitBranchValue
}

// newGitTestBoard creates a fresh git repo whose board lives at
// <repo>/.coding-hermes/board, seeds it with committed files, and resolves
// the board through the repo root (exercising the walk-up-to-.git path in
// Doctor).
//
// BT-039 load-hygiene: the repo is built IN-PROCESS by initGitRepo (file
// writes only — no git init/add/commit subprocesses per test). The git
// boundary here was pure scaffolding: Doctor exercises the real git path
// itself through its `git ls-files` subprocess. Real-exec behavior of that
// path is pinned once per package run by TestGitTrackedSetRealExecParity,
// which still spawns real git commands; the two must agree, and the parity
// test fails if the fake stops matching.
func newGitTestBoard(t *testing.T, files map[string]string) *Board {
	t.Helper()
	repo := t.TempDir()
	initGitRepo(t, repo, files)
	b, err := Resolve(repo)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// errorMsg returns the concatenation of all error-level finding messages.
func errorMsgs(rep *Report) []string {
	var out []string
	for _, f := range rep.Findings {
		if f.IsError() {
			out = append(out, f.Msg)
		}
	}
	return out
}

// TestDoctorCleanBoard: a healthy board (fixture rows present in tasks.jsonl,
// header counters ahead of the newest event tick, only .jsonl tracked) inside
// a git repo yields zero errors and zero warnings.
func TestDoctorCleanBoard(t *testing.T) {
	b := newGitTestBoard(t, map[string]string{
		"tasks.jsonl": `{"id":"WORK-1","title":"Work","status":"pending","priority":"P1"}` + "\n" +
			`{"id":"NEVER-DONE","title":"Perpetual fixture","status":"pending","priority":"P3","active":true}` + "\n",
		"events.jsonl":   `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":    `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"cooldown_s":21600}` + "\n",
		"fixtures.jsonl": `{"id":"NEVER-DONE","title":"Perpetual fixture","status":"pending","priority":"P3","active":true}` + "\n",
	})
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() {
		t.Fatalf("unexpected errors on clean board: %+v", rep.Findings)
	}
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "git-tracked-set") {
			t.Fatalf("clean board reported a git tracked-set finding: %s", f.Msg)
		}
	}
}

// TestDoctorRepoBoardNoTrackedCaches: the repo's own live board
// (../../.coding-hermes/board) copied into a fresh git repo must not yield a
// git-tracked-set error — nothing under the board dir carries a .db/.parquet
// extension. Other findings (e.g. the NEVER-DONE fixture orphan on a board
// that predates doctor) are expected and tolerated here; this test pins the
// tracked-set property only.
func TestDoctorRepoBoardNoTrackedCaches(t *testing.T) {
	src := filepath.Join("..", "..", ".coding-hermes", "board")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Skipf("repo board not present at %s: %v", src, err)
	}
	files := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[e.Name()] = string(data)
	}
	b := newGitTestBoard(t, files)
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Findings {
		if f.IsError() && strings.Contains(f.Msg, "git-tracked-set") {
			t.Fatalf("live board reported git-tracked-set error: %s", f.Msg)
		}
	}
}

// TestDoctorGitTrackedDBError: a board whose dir tracks a board.db cache
// yields exactly one error, the git-tracked-set finding.
func TestDoctorGitTrackedDBError(t *testing.T) {
	b := newGitTestBoard(t, map[string]string{
		"tasks.jsonl":  `{"id":"WORK-1","title":"Work","status":"pending","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"cooldown_s":21600}` + "\n",
		"board.db":     "not a real duckdb — cache files must stay untracked\n",
	})
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	msgs := errorMsgs(rep)
	if len(msgs) != 1 {
		t.Fatalf("errors = %d, want exactly 1: %+v", len(msgs), rep.Findings)
	}
	if !strings.Contains(msgs[0], "git-tracked-set:") || !strings.Contains(msgs[0], "board.db is tracked under .coding-hermes/board") {
		t.Fatalf("error %q is not the git tracked-set finding", msgs[0])
	}
}

// TestDoctorHeaderCounterDrift: header ticks_total 0 with a tick_number 3
// event (topology A) yields the counter-drift error plus the BT-013
// remediation hint naming the exact fix command with the right counter.
func TestDoctorHeaderCounterDrift(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"WORK-1","title":"Work","status":"pending","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":3}` + "\n",
		"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":0,"ticks_idle":0,"cooldown_s":21600}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	msgs := errorMsgs(rep)
	if len(msgs) != 1 {
		t.Fatalf("errors = %d, want exactly 1: %+v", len(msgs), rep.Findings)
	}
	if !strings.Contains(msgs[0], "header ticks_total 0 < max events tick_number 3") {
		t.Fatalf("error %q is not the counter-drift finding", msgs[0])
	}
	// BT-013: the remediation hint rides in the same finding, immediately
	// after the drift text, with the counter value that resolves it.
	if !strings.Contains(msgs[0], "fix: boardctl header --set-ticks-total=3") {
		t.Fatalf("drift error missing the remediation hint: %q", msgs[0])
	}
}

// TestDoctorOrphanFixture: a fixture id in fixtures.jsonl with no matching
// task row in tasks.jsonl yields the orphan error.
func TestDoctorOrphanFixture(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":    `{"id":"WORK-1","title":"Work","status":"pending","priority":"P1"}` + "\n",
		"events.jsonl":   `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":    `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"cooldown_s":21600}` + "\n",
		"fixtures.jsonl": `{"id":"ORPHAN-1","title":"Orphan fixture","status":"pending","priority":"P3","active":true}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	msgs := errorMsgs(rep)
	if len(msgs) != 1 {
		t.Fatalf("errors = %d, want exactly 1: %+v", len(msgs), rep.Findings)
	}
	if !strings.Contains(msgs[0], "fixture ORPHAN-1 has no task row in tasks.jsonl") {
		t.Fatalf("error %q is not the orphan-fixture finding", msgs[0])
	}
}

// TestDoctorHeaderCounterDriftOnTopologyB: doctor compares the topology-B
// line-1 header counters against the events stream — the drift error fires
// exactly as it does on topology A, with the BT-013 remediation hint, and the
// old "header counters not checked" warn is gone.
func TestDoctorHeaderCounterDriftOnTopologyB(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"project":"legacy","namespace":"legacy","version":3,"ticks_total":0,"ticks_idle":0}` + "\n" +
			`{"id":"WORK-1","title":"Work","status":"pending","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":3}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	msgs := errorMsgs(rep)
	if len(msgs) != 1 {
		t.Fatalf("errors = %d, want exactly 1: %+v", len(msgs), rep.Findings)
	}
	if !strings.Contains(msgs[0], "header ticks_total 0 < max events tick_number 3") {
		t.Fatalf("error %q is not the counter-drift finding", msgs[0])
	}
	if !strings.Contains(msgs[0], "fix: boardctl header --set-ticks-total=3") {
		t.Fatalf("drift error missing the remediation hint: %q", msgs[0])
	}
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "not checked") {
			t.Fatalf("topology-B 'not checked' warn is still emitted: %s", f.Msg)
		}
	}
}

// TestGitTrackedSetRealExecParity is the ONE real-exec git test in this
// package (BT-039 load-hygiene): it runs the git subprocess chain that the
// per-test board builder used to spawn for every fixture (git init + add -A +
// commit) and asserts that Doctor's git tracked-set verdict on the REAL repo
// matches the verdict on the in-process fixture shape (a hand-written .git
// with git's canonical empty-tree root commit). This is the parity check
// newGitTestBoard's doc comment promises: if git changes what makes a repo
// "a repo" (or the hand-built shape stops being one), this test fails while
// the rest of the package keeps running in-process.
//
// The tracked-set property is asserted in BOTH directions so a mismatch
// cannot hide:
//   - clean repo (no cache files)  -> no git-tracked-set ERROR
//   - repo tracking board.db       -> exactly the git-tracked-set ERROR
func TestGitTrackedSetRealExecParity(t *testing.T) {
	files := func(withDB bool) map[string]string {
		m := map[string]string{
			"tasks.jsonl":  `{"id":"WORK-1","title":"Work","status":"pending","priority":"P1"}` + "\n",
			"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
			"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"cooldown_s":21600}` + "\n",
		}
		if withDB {
			m["board.db"] = "not a real duckdb — cache files must stay untracked\n"
		}
		return m
	}

	// tracked-set verdict on a REAL git repo (spawns init/add/commit/ls-files).
	verdictReal := func(t *testing.T, files map[string]string) (hasTrackedSetErr bool, findings string) {
		t.Helper()
		repo := t.TempDir()
		writeBoardFiles(t, filepath.Join(repo, ".coding-hermes", "board"), files)
		gitRun(t, repo, "init", "-q")
		gitRun(t, repo, "add", "-A")
		gitRun(t, repo, "-c", "user.name=boardctl test", "-c", "user.email=test@example.com", "commit", "-q", "-m", "seed board")
		b, err := Resolve(repo)
		if err != nil {
			t.Fatal(err)
		}
		rep, err := b.Doctor()
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range rep.Findings {
			if strings.Contains(f.Msg, "git-tracked-set:") {
				hasTrackedSetErr = true
			}
		}
		return hasTrackedSetErr, fmt.Sprint(rep.Findings)
	}

	// Same verdict on the IN-PROCESS fixture shape newGitTestBoard builds.
	verdictFake := func(t *testing.T, files map[string]string) (hasTrackedSetErr bool, findings string) {
		t.Helper()
		repo := t.TempDir()
		initGitRepo(t, repo, files)
		b, err := Resolve(repo)
		if err != nil {
			t.Fatal(err)
		}
		rep, err := b.Doctor()
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range rep.Findings {
			if strings.Contains(f.Msg, "git-tracked-set:") {
				hasTrackedSetErr = true
			}
		}
		return hasTrackedSetErr, fmt.Sprint(rep.Findings)
	}

	t.Run("clean board: real exec and in-process fixture agree (no tracked-set error)", func(t *testing.T) {
		realErr, realFindings := verdictReal(t, files(false))
		fakeErr, fakeFindings := verdictFake(t, files(false))
		if realErr {
			t.Fatalf("real-exec clean repo unexpectedly produced a git-tracked-set error: %s", realFindings)
		}
		if fakeErr != realErr {
			t.Fatalf("in-process fixture disagrees with real exec on a clean repo: fake=%v real=%v (fake findings: %s)", fakeErr, realErr, fakeFindings)
		}
	})

	t.Run("tracked board.db: both shapes yield exactly the git-tracked-set error", func(t *testing.T) {
		realErr, realFindings := verdictReal(t, files(true))
		fakeErr, fakeFindings := verdictFake(t, files(true))
		if !realErr {
			t.Fatalf("real-exec repo with tracked board.db did NOT yield the git-tracked-set error: %s", realFindings)
		}
		if fakeErr != realErr {
			t.Fatalf("in-process fixture disagrees with real exec on a tracked-cache repo: fake=%v real=%v (fake findings: %s)", fakeErr, realErr, fakeFindings)
		}
	})
}
