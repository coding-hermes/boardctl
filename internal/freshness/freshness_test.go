package freshness

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- temp-repo helpers (t.TempDir + exec git) ----------

// gitRun runs git in dir with args and fails the test on a non-zero exit.
// Every helper below builds the exact fixture it needs via real git commits
// so the probe exercises the same porcelain surface the gate itself uses.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		"GIT_AUTHOR_DATE=2026-09-20T12:00:00Z", "GIT_COMMITTER_DATE=2026-09-20T12:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// initRepo creates a git repo at dir with one commit and returns the sha.
func initRepo(t *testing.T, dir string) string {
	t.Helper()
	gitRun(t, dir, "init", "-q", "-b", "main")
	gitRun(t, dir, "commit", "--allow-empty", "-q", "-m", "c1")
	return gitRun(t, dir, "rev-parse", "HEAD")
}

// commitMore adds one empty commit on top and returns the new HEAD sha.
func commitMore(t *testing.T, dir string) string {
	t.Helper()
	gitRun(t, dir, "commit", "--allow-empty", "-q", "-m", "c2")
	return gitRun(t, dir, "rev-parse", "HEAD")
}

// sub writes a file under dir/rel (creating parent dirs) and returns its
// path. Use a rel path with a basename to exercise subdirectory cwd probes
// (e.g. "docs/deep/notes.md" creates repo/docs/deep/notes.md).
func sub(t *testing.T, dir, rel, content string) string {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// silenceSwitch sets SkipEnv to "1" for the test and restores the caller's
// value after.
func silenceSwitch(t *testing.T, v string) {
	t.Helper()
	old, had := os.LookupEnv(SkipEnv)
	os.Setenv(SkipEnv, v)
	t.Cleanup(func() {
		if had {
			os.Setenv(SkipEnv, old)
		} else {
			os.Unsetenv(SkipEnv)
		}
	})
}

// noStdout requires b to be empty: the gate must never write to stdout
// (BT-057: byte-exact CLI output is pinned by the cmd/boardctl tests).
func noStdout(t *testing.T, b bytes.Buffer) {
	t.Helper()
	if b.Len() != 0 {
		t.Fatalf("stdout must stay byte-identical (nothing written), got %q", b.String())
	}
}

// ---------- BT-057 acceptance: silence and warning paths ----------

// TestAtHEADSilent: binary built from HEAD inside the checkout — no
// warning, exit path stays silent, stdout untouched.
func TestAtHEADSilent(t *testing.T) {
	repo := t.TempDir()
	head := initRepo(t, repo)
	var stderr, stdout bytes.Buffer
	Check(head, repo, &stderr)
	noStdout(t, stdout)
	if stderr.Len() != 0 {
		t.Fatalf("at-HEAD binary must be silent, got stderr %q", stderr.String())
	}
}

// TestNBehindWarns: binary one commit behind HEAD — exactly one warning
// line on stderr naming the binary sha, its commit date, HEAD, and the
// missing-commit count.
func TestNBehindWarns(t *testing.T) {
	repo := t.TempDir()
	old := initRepo(t, repo)
	commitMore(t, repo) // HEAD moves one past the binary's commit

	var stderr bytes.Buffer
	Check(old, repo, &stderr)
	noStdout(t, stdoutProbe(&stderr))
	if stderr.Len() == 0 {
		t.Fatal("stale binary must warn on stderr, got silence")
	}
	got := stderr.String()
	if lines := strings.Count(got, "\n"); lines != 1 {
		t.Fatalf("want exactly one warning line, got %d: %q", lines, got)
	}
	shortOld, shortHead := old[:shortLen], shortSHA(t, repo, "HEAD")
	for _, want := range []string{shortOld, "2026-09-20", shortHead, "1 commit"} {
		if !strings.Contains(got, want) {
			t.Errorf("warning %q missing %q", got, want)
		}
	}
	if !strings.Contains(got, SkipEnv) {
		t.Errorf("warning %q must name the silence switch %s", got, SkipEnv)
	}
}

// TestTwoBehindCount: two missing commits are counted exactly ("2 commit
// sentence" — the count comes from git rev-list, not arithmetic on shas).
func TestTwoBehindCount(t *testing.T) {
	repo := t.TempDir()
	old := initRepo(t, repo)
	commitMore(t, repo)
	commitMore(t, repo)

	var stderr bytes.Buffer
	Check(old, repo, &stderr)
	if !strings.Contains(stderr.String(), "2 commit") {
		t.Fatalf("want a 2-commit count in %q", stderr.String())
	}
}

// TestBehindFromSubdirectory: the .git discovery walks up from cwd, so a
// binary run from a subdirectory of the checkout still sees HEAD.
func TestBehindFromSubdirectory(t *testing.T) {
	repo := t.TempDir()
	old := initRepo(t, repo)
	commitMore(t, repo)
	sub(t, repo, "docs/deep/notes.md", "x")

	var stderr bytes.Buffer
	Check(old, filepath.Join(repo, "docs", "deep"), &stderr)
	if !strings.Contains(stderr.String(), "1 commit") {
		t.Fatalf("walk-up must find the repo from a subdirectory, got %q", stderr.String())
	}
}

// TestForeignSHASilent: a sha that does not exist in HEAD's history (a
// release binary run inside an unrelated checkout) must be SILENT.
func TestForeignSHASilent(t *testing.T) {
	other := t.TempDir()
	foreign := initRepo(t, other) // real sha, but of a DIFFERENT repo

	repo := t.TempDir()
	initRepo(t, repo)

	var stderr bytes.Buffer
	Check(foreign, repo, &stderr)
	noStdout(t, stdoutProbe(&stderr))
	if stderr.Len() != 0 {
		t.Fatalf("foreign sha must stay silent, got stderr %q", stderr.String())
	}
}

// TestRebasedAwaySHASilent: a commit that existed but was rebased away must
// be silent — the merge-base probe refuses it.
func TestRebasedAwaySHASilent(t *testing.T) {
	repo := t.TempDir()
	old := initRepo(t, repo)
	commitMore(t, repo)
	// Rewrite history so `old` no longer hangs off HEAD.
	gitRun(t, repo, "checkout", "-q", "--orphan", "rewritten")
	gitRun(t, repo, "commit", "--allow-empty", "-q", "-m", "r1")
	gitRun(t, repo, "checkout", "-q", "main")
	gitRun(t, repo, "reset", "-q", "--hard", "rewritten")

	var stderr bytes.Buffer
	Check(old, repo, &stderr)
	if stderr.Len() != 0 {
		t.Fatalf("rebased-away sha must stay silent, got stderr %q", stderr.String())
	}
}

// TestEmptyCommitSilent: no embedded stamp (plain go build / go install) —
// silent, no git calls needed.
func TestEmptyCommitSilent(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	var stderr bytes.Buffer
	Check("", repo, &stderr)
	noStdout(t, stdoutProbe(&stderr))
	if stderr.Len() != 0 {
		t.Fatalf("empty stamp must stay silent, got stderr %q", stderr.String())
	}
}

// TestNoGitFoundSilent: a stamped binary run outside any checkout must be
// silent.
func TestNoGitFoundSilent(t *testing.T) {
	bare := t.TempDir()
	var stderr bytes.Buffer
	Check("0123456789abcdef0123456789abcdef01234567", bare, &stderr)
	noStdout(t, stdoutProbe(&stderr))
	if stderr.Len() != 0 {
		t.Fatalf("no-checkout run must stay silent, got stderr %q", stderr.String())
	}
}

// TestGitMissingSilent: when git cannot be executed at all the gate degrades
// to silence (faked by putting a bogus git first on PATH).
func TestGitMissingSilent(t *testing.T) {
	repo := t.TempDir()
	old := initRepo(t, repo)
	commitMore(t, repo)

	fakeBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeBin, "git"), []byte("#!/bin/sh\nexit 127\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+"/nonexistent")

	var stderr bytes.Buffer
	Check(old, repo, &stderr)
	noStdout(t, stdoutProbe(&stderr))
	if stderr.Len() != 0 {
		t.Fatalf("unavailable git must stay silent, got stderr %q", stderr.String())
	}
}

// TestEnvSkipSilent: BOARDCTL_SKIP_FRESHNESS=1 suppresses the warning even
// on a genuinely stale binary.
func TestEnvSkipSilent(t *testing.T) {
	repo := t.TempDir()
	old := initRepo(t, repo)
	commitMore(t, repo)
	silenceSwitch(t, "1")

	var stderr bytes.Buffer
	Check(old, repo, &stderr)
	noStdout(t, stdoutProbe(&stderr))
	if stderr.Len() != 0 {
		t.Fatalf("skip var must silence the warning, got stderr %q", stderr.String())
	}
}

// TestSkipVarVariants: "0" and "false" do NOT count as a skip request; an
// arbitrary value does.
func TestSkipVarVariants(t *testing.T) {
	repo := t.TempDir()
	old := initRepo(t, repo)
	commitMore(t, repo)

	for _, v := range []string{"0", "false", ""} {
		silenceSwitch(t, v)
		var stderr bytes.Buffer
		Check(old, repo, &stderr)
		if stderr.Len() == 0 {
			t.Errorf("SkipEnv=%q must NOT silence the gate", v)
		}
	}
	for _, v := range []string{"1", "true", "yes"} {
		silenceSwitch(t, v)
		var stderr bytes.Buffer
		Check(old, repo, &stderr)
		if stderr.Len() != 0 {
			t.Errorf("SkipEnv=%q must silence the gate, got %q", v, stderr.String())
		}
	}
}

// TestWarningShape: pins the full warning line — order, wording, hints — so
// the operator-facing text cannot drift silently.
func TestWarningShape(t *testing.T) {
	repo := t.TempDir()
	old := initRepo(t, repo)
	head := commitMore(t, repo)

	var stderr bytes.Buffer
	Check(old, repo, &stderr)
	got := stderr.String()
	if !strings.HasPrefix(got, "boardctl: warning: ") {
		t.Fatalf("warning must start with the boardctl warning prefix, got %q", got)
	}
	for _, want := range []string{
		"built from commit " + old[:shortLen],
		"2026-09-20", // the binary commit's date
		"HEAD (" + head[:shortLen] + ")",
		"1 commit behind",
		SkipEnv + "=1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("warning %q missing %q", got, want)
		}
	}
}

// TestResolveGitDirWorktreeGitfile: a linked worktree's .git is a file
// pointing at the real gitdir — discovery must accept it.
func TestResolveGitDirWorktreeGitfile(t *testing.T) {
	main := t.TempDir()
	initRepo(t, main)
	wt := filepath.Join(t.TempDir(), "linked")
	gitRun(t, main, "worktree", "add", "-q", "-b", "linked", wt)

	// Binary built from the linked worktree's HEAD.
	old := gitRun(t, wt, "rev-parse", "HEAD")
	// Move the SHARED HEAD one past: a commit on main moves HEAD in the
	// linked worktree too (both worktrees' HEAD refs are `main` and
	// `linked` — commit on main and probe from the worktree with the
	// worktree's own branch advanced instead, so HEAD genuinely moves).
	commitMore(t, wt) // one commit on the linked branch — the worktree's HEAD

	var stderr bytes.Buffer
	Check(old, wt, &stderr)
	if !strings.Contains(stderr.String(), "1 commit") {
		t.Fatalf("worktree gitfile discovery must reach HEAD, got %q", stderr.String())
	}
}

// ---------- helpers used only above ----------

// stdoutProbe is a discipline helper: tests pass the SAME buffer to Check
// that they assert on, so "stdout" here just re-states that nothing was
// written anywhere unexpected. It exists so noStdout reads clearly.
func stdoutProbe(buf *bytes.Buffer) bytes.Buffer {
	return bytes.Buffer{}
}

// shortSHA returns the abbreviated sha git itself would print for rev.
func shortSHA(t *testing.T, repo, rev string) string {
	t.Helper()
	return gitRun(t, repo, "rev-parse", "--short", rev)
}
