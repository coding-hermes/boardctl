package board

// DF-BOARDCTL-35: doctor's git tracked-set check only inspects `git ls-files`
// output, so an UNTRACKED board.db (gitignored cache fossil) physically
// sitting in the board dir went unnoticed — doctor exited 0 in silence while
// README declares "the JSONL files are the board". These tests pin the new
// fossil warning on the in-process git fixture (newGitTestBoard's hand-built
// .git), whose tracked set comes from the seeded index — a file present on
// disk but absent from that index is untracked by construction, no git
// subprocesses needed. TestDoctorFossilWarnRealExecParity re-proves the same
// three verdicts against ONE real git toolchain, the same parity shape
// TestGitTrackedSetRealExecParity pins for the tracked-set check.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// df35FossilWarns returns the report's untracked-set (fossil) findings.
func df35FossilWarns(rep *Report) []string {
	var out []string
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "untracked-set:") {
			out = append(out, f.Msg)
		}
	}
	return out
}

// df35SeedFiles is the minimal healthy board (the shape every other doctor
// test seeds).
func df35SeedFiles() map[string]string {
	return map[string]string{
		"tasks.jsonl":  `{"id":"WORK-1","title":"Work","status":"pending","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"cooldown_s":21600}` + "\n",
	}
}

// df35WriteExtra writes one extra file into the board dir WITHOUT touching
// the tracked set (initGitRepo's index was already built from the files
// map): the on-disk-untracked shape.
func df35WriteExtra(t *testing.T, b *Board, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(b.Dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDoctorUntrackedDBFossilWarns: an untracked board.db in the board dir
// yields exactly one fossil warning naming the file as a cache fossil, and
// doctor stays green (warn, never an error — the board itself is healthy).
func TestDoctorUntrackedDBFossilWarns(t *testing.T) {
	b := newGitTestBoard(t, df35SeedFiles())
	df35WriteExtra(t, b, "board.db", "not a real duckdb — untracked cache fossil\n")
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() {
		t.Fatalf("untracked fossil must WARN, never error: %+v", rep.Findings)
	}
	warns := df35FossilWarns(rep)
	if len(warns) != 1 {
		t.Fatalf("untracked-set warnings = %d, want exactly 1: %+v", len(warns), rep.Findings)
	}
	for _, want := range []string{
		".coding-hermes/board/board.db",
		"not tracked by git",
		"cache fossil",
		"the JSONL files are the board",
	} {
		if !strings.Contains(warns[0], want) {
			t.Fatalf("fossil warning %q lacks %q", warns[0], want)
		}
	}
}

// TestDoctorUntrackedParquetFossilWarns: the other extension in the
// tracked-set rule warns too.
func TestDoctorUntrackedParquetFossilWarns(t *testing.T) {
	b := newGitTestBoard(t, df35SeedFiles())
	df35WriteExtra(t, b, "events.parquet", "stale parquet cache\n")
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	warns := df35FossilWarns(rep)
	if len(warns) != 1 {
		t.Fatalf("untracked-set warnings = %d, want exactly 1 (the parquet): %+v", len(warns), rep.Findings)
	}
	if !strings.Contains(warns[0], ".coding-hermes/board/events.parquet") {
		t.Fatalf("fossil warning does not name the parquet file: %q", warns[0])
	}
}

// TestDoctorTrackedDBNoFossilWarn: the mirrored regression half — a TRACKED
// board.db still produces exactly the git-tracked-set ERROR and MUST NOT
// gain the fossil warning (the file's finding is the tracked-set error, not
// an untracked fossil; the two checks must never double-report one file).
func TestDoctorTrackedDBNoFossilWarn(t *testing.T) {
	b := newGitTestBoard(t, map[string]string{
		"tasks.jsonl":  `{"id":"WORK-1","title":"Work","status":"pending","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"cooldown_s":21600}` + "\n",
		"board.db":     "tracked cache — the tracked-set error's subject\n",
	})
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	msgs := errorMsgs(rep)
	if len(msgs) != 1 {
		t.Fatalf("errors = %d, want exactly 1: %+v", len(msgs), rep.Findings)
	}
	if !strings.Contains(msgs[0], "git-tracked-set:") {
		t.Fatalf("error %q is not the git tracked-set finding", msgs[0])
	}
	if warns := df35FossilWarns(rep); len(warns) != 0 {
		t.Fatalf("tracked board.db must not also warn as an untracked fossil: %v", warns)
	}
}

// TestDoctorCleanDirNoFossilWarn: a clean board (no db/parquet on disk at
// all) gains no new warning — criterion 3's in-process pin.
func TestDoctorCleanDirNoFossilWarn(t *testing.T) {
	b := newGitTestBoard(t, df35SeedFiles())
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	if warns := df35FossilWarns(rep); len(warns) != 0 {
		t.Fatalf("clean board dir produced fossil warnings: %v", warns)
	}
}

// TestDoctorFossilWarnOutsideGitRepo: a board dir with no enclosing repo has
// no tracked/untracked distinction — the file must still surface, labeled
// unclassified, not silently pass as the old tracked-set-only path did.
func TestDoctorFossilWarnOutsideGitRepo(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"WORK-1","title":"Work","status":"pending","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"cooldown_s":21600}` + "\n",
		"board.db":     "untracked by definition — no repo here\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() {
		t.Fatalf("no-repo fossil must WARN, never error: %+v", rep.Findings)
	}
	warns := df35FossilWarns(rep)
	if len(warns) != 1 {
		t.Fatalf("untracked-set warnings = %d, want exactly 1: %+v", len(warns), rep.Findings)
	}
	if !strings.Contains(warns[0], "not inside a git repo") || !strings.Contains(warns[0], "board.db") {
		t.Fatalf("no-repo warning must name the file and the missing repo: %q", warns[0])
	}
	// init_test.go pins the same finding shape: the repo-less tracked-set
	// note and the fossil warn are both present, distinct findings.
	var hasTrackedSetNote bool
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "not inside a git repo — git tracked-set check skipped") {
			hasTrackedSetNote = true
		}
	}
	if !hasTrackedSetNote {
		t.Fatalf("expected the tracked-set skip note alongside the fossil warn: %+v", rep.Findings)
	}
}

// TestDoctorFossilWarnRealExecParity: the same real-exec parity shape
// TestGitTrackedSetRealExecParity pins for the tracked-set check, re-proven
// for the fossil warn on REAL git repos (init/add/commit + the doctor path's
// own ls-files probes). One repo per verdict; the hand-built index fixture
// and real git must agree on all three:
//   - untracked board.db on disk  -> exactly one untracked-set WARN, no error
//   - tracked board.db            -> exactly one tracked-set ERROR, no warn
//   - clean board                 -> no untracked-set finding at all
func TestDoctorFossilWarnRealExecParity(t *testing.T) {
	// build seeds a real repo in one of three shapes:
	//   untracked: commit the clean board, then drop board.db on disk AFTER
	//              the commit (the gitignored-fossil shape from dogfood)
	//   tracked:   board.db committed with the seed (the tracked-set shape)
	//   clean:     commit the clean board, touch nothing else
	build := func(t *testing.T, mode string) *Board {
		t.Helper()
		repo := t.TempDir()
		bd := filepath.Join(repo, ".coding-hermes", "board")
		seed := df35SeedFiles()
		if mode == "tracked" {
			seed["board.db"] = "tracked cache\n"
		}
		writeBoardFiles(t, bd, seed)
		gitRun(t, repo, "init", "-q")
		gitRun(t, repo, "add", "-A")
		gitRun(t, repo, "-c", "user.name=boardctl test", "-c", "user.email=test@example.com", "commit", "-q", "-m", "seed board")
		if mode == "untracked" {
			if err := os.WriteFile(filepath.Join(bd, "board.db"), []byte("fossil\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		b, err := Resolve(repo)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	t.Run("untracked board.db: exactly one fossil warn, no error", func(t *testing.T) {
		b := build(t, "untracked")
		rep, err := b.Doctor()
		if err != nil {
			t.Fatal(err)
		}
		if rep.HasErrors() {
			t.Fatalf("real repo: untracked fossil must not error: %+v", rep.Findings)
		}
		warns := df35FossilWarns(rep)
		if len(warns) != 1 {
			t.Fatalf("real repo: fossil warns = %d, want 1: %+v", len(warns), rep.Findings)
		}
		if !strings.Contains(warns[0], "board.db") || !strings.Contains(warns[0], "cache fossil") {
			t.Fatalf("real repo: warn %q does not name board.db as a fossil", warns[0])
		}
	})

	t.Run("tracked board.db: tracked-set error, no fossil warn", func(t *testing.T) {
		b := build(t, "tracked")
		rep, err := b.Doctor()
		if err != nil {
			t.Fatal(err)
		}
		msgs := errorMsgs(rep)
		if len(msgs) != 1 || !strings.Contains(msgs[0], "git-tracked-set:") {
			t.Fatalf("real repo: want exactly the tracked-set error, got %+v", rep.Findings)
		}
		if warns := df35FossilWarns(rep); len(warns) != 0 {
			t.Fatalf("real repo: tracked board.db must not fossil-warn: %v", warns)
		}
	})

	t.Run("clean board: no fossil finding", func(t *testing.T) {
		b := build(t, "clean")
		if _, err := os.Stat(filepath.Join(b.Dir, "board.db")); err == nil {
			t.Fatal("clean fixture unexpectedly carries board.db")
		}
		rep, err := b.Doctor()
		if err != nil {
			t.Fatal(err)
		}
		if warns := df35FossilWarns(rep); len(warns) != 0 {
			t.Fatalf("real repo clean board: fossil warns = %d, want 0: %v", len(warns), warns)
		}
	})
}
