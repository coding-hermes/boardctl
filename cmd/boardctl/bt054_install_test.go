package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BT-054: pin the install command surface — hook generation (fresh, chained,
// idempotent), the dispatch case, dry-run behavior, and exit codes.
//
// Helpers live here (not in main_test.go) so this file is self-contained:
//   - instCapture runs run() capturing stdout+stderr (stderr is needed:
//     usage errors go to stderr, and run() does not surface it as a value).
//   - instRunExpect asserts run()'s exit code.
//   - instRepoRoot seeds a throwaway git repo with a real `boardctl init`
//     board via run() itself (no subprocess).

// instCapture runs fn while capturing BOTH stdout and stderr (pipes, so the
// captured output never interleaves into the test's own output).
func instCapture(fn func()) (string, string) {
	outR, outW, err := os.Pipe()
	if err != nil {
		return "", ""
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		return "", ""
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	doneOut := make(chan string, 1)
	doneErr := make(chan string, 1)
	go func() { b, _ := io.ReadAll(outR); doneOut <- string(b) }()
	go func() { b, _ := io.ReadAll(errR); doneErr <- string(b) }()

	fn()
	outW.Close()
	errW.Close()
	return <-doneOut, <-doneErr
}

// instRunExpect asserts run()'s exit code and returns it (callers return
// early on mismatch so t.Fatal fires inside the test goroutine).
func instRunExpect(t *testing.T, args []string, want int) int {
	t.Helper()
	code := run(args)
	if code != want {
		_, errOut := instCapture(func() { _ = run(args) })
		t.Fatalf("run(%v) exit = %d, want %d\nstderr: %s", args, code, want, errOut)
	}
	return code
}

// instRepoRoot seeds a throwaway git repo (real .git dir) holding a real
// boardctl-init'd board, and returns the repo root. run() is invoked with an
// explicit -C ABSOLUTE path (never cwd-relative) so the seeded board can
// never land inside the real repo tree.
func instRepoRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, errOut := instCapture(func() {
		if run([]string{"init", "-C", dir, "--project", "t", "--namespace", "t"}) != 0 {
			t.Errorf("boardctl init inside seed failed")
		}
	}); errOut != "" {
		t.Fatalf("init emitted stderr: %s", errOut)
	}
	return dir
}

// TestBT054InstallFreshHook covers: install into a repo with no pre-existing
// hook writes an executable hook whose content carries the marker, the
// timeout invocation, and the recorded binary + repo root.
func TestBT054InstallFreshHook(t *testing.T) {
	root := instRepoRoot(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	self, _ = filepath.Abs(self)
	instRunExpect(t, []string{"install", "-C", root}, 0)
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	raw, err := os.ReadFile(hook)
	if err != nil {
		t.Fatalf("hook not written: %v", err)
	}
	if st, err := os.Stat(hook); err != nil || st.Mode()&0o111 == 0 {
		t.Fatalf("hook not executable: mode=%v err=%v", st.Mode(), err)
	}
	content := string(raw)
	if !strings.Contains(content, boardLintMarkerBegin) || !strings.Contains(content, boardLintMarkerEnd) {
		t.Fatalf("hook missing board-lint markers:\n%s", content)
	}
	if !strings.Contains(content, "timeout") {
		t.Fatalf("hook missing timeout invocation:\n%s", content)
	}
	if !strings.Contains(content, `validate -C`) {
		t.Fatalf("hook missing boardctl validate call:\n%s", content)
	}
	if !strings.Contains(content, self) {
		t.Fatalf("hook does not record the resolved binary path %s:\n%s", self, content)
	}
	if !strings.Contains(content, root) {
		t.Fatalf("hook does not record the repo root %s:\n%s", root, content)
	}
}

// TestBT054InstallChainedKeepsExistingHook covers the GitReins-shaped chain:
// an existing hook's logic must survive, and the board-lint block must be
// inserted BEFORE the existing logic's first early exit — the only placement
// guaranteed to run when existing logic exits early (generateHookContent
// inserts after the shebang for exactly that reason).
func TestBT054InstallChainedKeepsExistingHook(t *testing.T) {
	root := instRepoRoot(t)
	hooks := filepath.Join(root, ".git", "hooks")
	existing := "#!/bin/sh\n# gitreins guard stub\necho existing-ran >> \"" + root + "/chain.txt\"\nexit 0\n"
	if err := os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	instRunExpect(t, []string{"install", "-C", root}, 0)
	raw, err := os.ReadFile(filepath.Join(hooks, "pre-commit"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	if !strings.Contains(content, "existing-ran") {
		t.Fatalf("existing hook logic lost:\n%s", content)
	}
	// The block must precede the existing exit 0 so board-lint runs even on
	// the early-exit path.
	markerIdx := strings.Index(content, boardLintMarkerBegin)
	exitIdx := strings.Index(content, "exit 0")
	if markerIdx < 0 || exitIdx < 0 || markerIdx > exitIdx {
		t.Fatalf("board-lint block not inserted before existing early exit:\n%s", content)
	}
}

// TestBT054InstallIdempotent: two installs leave exactly ONE marked block.
func TestBT054InstallIdempotent(t *testing.T) {
	root := instRepoRoot(t)
	instRunExpect(t, []string{"install", "-C", root}, 0)
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	instRunExpect(t, []string{"install", "-C", root}, 0)
	raw, err := os.ReadFile(hook)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(raw), boardLintMarkerBegin); n != 1 {
		t.Fatalf("expected exactly one board-lint block after re-install, got %d:\n%s", n, raw)
	}
}

// TestBT054InstallDryRun: --dry-run prints the would-be content and writes
// nothing.
func TestBT054InstallDryRun(t *testing.T) {
	root := instRepoRoot(t)
	out, _ := instCapture(func() {
		if code := run([]string{"install", "-C", root, "--dry-run"}); code != 0 {
			t.Errorf("dry-run exit = %d", code)
		}
	})
	if !strings.Contains(out, boardLintMarkerBegin) {
		t.Fatalf("dry-run stdout lacks hook content:\n%s", out)
	}
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(hook); !os.IsNotExist(err) {
		t.Fatalf("dry-run must not write the hook (stat err=%v)", err)
	}
}

// TestBT054InstallOutsideGitRepo: no .git anywhere above => exit 1 via the
// plain error arm (not the usage arm).
func TestBT054InstallOutsideGitRepo(t *testing.T) {
	dir := t.TempDir()
	instRunExpect(t, []string{"install", "-C", dir}, 1)
}

// TestBT054InstallTimeoutFlag: a non-positive --timeout is a usage failure
// (exit 2) and the hook body carries the requested value when valid.
func TestBT054InstallTimeoutFlag(t *testing.T) {
	root := instRepoRoot(t)
	instRunExpect(t, []string{"install", "-C", root, "--timeout", "0"}, 2)
	instRunExpect(t, []string{"install", "-C", root, "--timeout", "5"}, 0)
	raw, err := os.ReadFile(filepath.Join(root, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "boardctl_timeout=5\n") {
		t.Fatalf("hook does not carry the overridden timeout:\n%s", raw)
	}
}

// TestBT054HookBodyExitContract pins the shell contract of the managed block
// textually: the timeout-124 skip note, the no-board skip, the missing-binary
// skip, the blocked-commit arm, and the validate call.
func TestBT054HookBodyExitContract(t *testing.T) {
	cases := map[string]string{
		"timeout skip note":    `echo "board-lint: skipped (timeout)"`,
		"no-board skip":        `skipped (no board found under`,
		"missing binary skip":  `skipped (boardctl binary not found)`,
		"timeout(1) skip":      `skipped (timeout(1) unavailable)`,
		"blocked commit arm":   `commit rejected (boardctl validate failed)`,
		"validate invocation":  `validate -C "$board_root"`,
		"no-board early skip":  `[ -d "$board_root/.coding-hermes/board" ]`,
		"timeout wrapper":      `timeout "$boardctl_timeout"`,
		"chained fall-through": `case "$lint_status" in`,
	}
	for name, needle := range cases {
		if !strings.Contains(boardLintHookBody, needle) {
			t.Errorf("hook body missing %s (%q)", name, needle)
		}
	}
}

// TestBT054GenerateHookContentNoShebang: an existing hook without a shebang
// still gets the block, and the content is preserved.
func TestBT054GenerateHookContentNoShebang(t *testing.T) {
	out := generateHookContent("echo legacy\n", "/bin/boardctl", "/repo", 30)
	if !strings.Contains(out, "echo legacy") {
		t.Fatalf("legacy content lost:\n%s", out)
	}
	if !strings.Contains(out, boardLintMarkerBegin) {
		t.Fatalf("block missing:\n%s", out)
	}
}
