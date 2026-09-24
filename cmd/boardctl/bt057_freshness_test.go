package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- BT-057: deployment freshness gate wiring ----------
// (repoRoot is declared in serve_test.go — reused here.)

// bt057TempRepo creates a tiny standalone git repo (t.TempDir + exec git) —
// used as the FOREIGN checkout a stamped binary knows nothing about.
func bt057TempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("commit", "--allow-empty", "-q", "-m", "c1")
	return dir
}

// TestBT057RunWiresFreshnessBeforeDispatch pins the call site: run() must
// invoke freshness.Check BEFORE the subcommand switch, so every dispatch
// path (including version/help/unknown) is covered. The check reads the
// package file from disk (never inspect.getsource-style runtime magic, and
// never a line-number pin that formatting would shift).
func TestBT057RunWiresFreshnessBeforeDispatch(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	checkIdx := strings.Index(s, "freshness.Check(")
	if checkIdx < 0 {
		t.Fatal("run() must call freshness.Check — no call site found in cmd/boardctl/main.go")
	}
	if strings.Count(s, "freshness.Check(") != 1 {
		t.Fatal("freshness.Check must be wired exactly once (single startup gate)")
	}
	switchIdx := strings.Index(s, "\tswitch cmd {")
	if switchIdx < 0 {
		t.Fatal("dispatch switch not found in run()")
	}
	if checkIdx > switchIdx {
		t.Fatalf("freshness.Check (offset %d) must run BEFORE the subcommand switch (offset %d)", checkIdx, switchIdx)
	}
}

// TestBT057StaleBinaryForeignRepoSilent builds a REAL stamped binary (a fake
// older sha — the stale path itself is proven by the internal/freshness unit
// tests, per the task's acceptance evidence) and runs it inside a DIFFERENT
// tiny git repo: a foreign sha must mean SILENT skip — empty stderr, empty
// stdout, exit 0. This exercises the whole live chain: ldflags stamp ->
// main.buildCommit -> run() -> freshness.Check -> git probe.
func TestBT057StaleBinaryForeignRepoSilent(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the real binary; skipped under -short")
	}
	fakeOld := "0123456789abcdef0123456789abcdef01234567"
	bin := filepath.Join(t.TempDir(), "boardctl")
	cmd := exec.Command("go", "build", "-o", bin,
		"-ldflags", "-X main.buildCommit="+fakeOld, "./cmd/boardctl")
	cmd.Dir = repoRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	foreign := bt057TempRepo(t)
	out, err := exec.Command(bin, "version").Output()
	if err != nil {
		t.Fatalf("version exit: %v", err)
	}
	if string(out) != "boardctl version dev\n" {
		t.Fatalf("stdout must stay byte-identical, got %q", string(out))
	}

	// stderr must be empty for the foreign-sha run inside the scratch repo.
	var errBuf bytes.Buffer
	probe := exec.Command(bin, "version")
	probe.Dir = foreign
	probe.Stderr = &errBuf
	if err := probe.Run(); err != nil {
		t.Fatalf("version exit inside foreign repo: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("foreign-sha run must print nothing to stderr, got %q", errBuf.String())
	}
}

// TestBT057SkipVarSilencesRealBinary: the same stamped binary, run inside
// ITS OWN checkout (where the fake sha is foreign too, but BOARDCTL_SKIP_
// FRESHNESS must hold even if it were stale) — the skip var keeps the gate
// silent end to end.
func TestBT057SkipVarSilencesRealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the real binary; skipped under -short")
	}
	fakeOld := "0123456789abcdef0123456789abcdef01234567"
	bin := filepath.Join(t.TempDir(), "boardctl")
	cmd := exec.Command("go", "build", "-o", bin,
		"-ldflags", "-X main.buildCommit="+fakeOld, "./cmd/boardctl")
	cmd.Dir = repoRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	var errBuf bytes.Buffer
	probe := exec.Command(bin, "version")
	probe.Dir = "." // this worktree — a real checkout the fake sha is foreign to
	probe.Env = append(os.Environ(), "BOARDCTL_SKIP_FRESHNESS=1")
	probe.Stderr = &errBuf
	out, err := probe.Output()
	if err != nil {
		t.Fatalf("version exit: %v", err)
	}
	if string(out) != "boardctl version dev\n" {
		t.Fatalf("stdout must stay byte-identical, got %q", string(out))
	}
	if errBuf.Len() != 0 {
		t.Fatalf("BOARDCTL_SKIP_FRESHNESS=1 must silence the gate, got stderr %q", errBuf.String())
	}
}
