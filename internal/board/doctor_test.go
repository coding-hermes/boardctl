package board

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// newGitTestBoard creates a fresh git repo whose board lives at
// <repo>/.coding-hermes/board, seeds it with files, commits the seed, and
// resolves the board through the repo root (exercising the walk-up-to-.git
// path in Doctor).
func newGitTestBoard(t *testing.T, files map[string]string) *Board {
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
// event (topology A) yields the counter-drift error.
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
