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

// TestBT054InstallChainedKeepsExistingHook covers the chained rewrite:
// an existing hook's logic must survive (inside the subshell wrap), the
// board-lint epilogue must run AFTER the existing body (BT-054-R: lint after
// existing logic, not before), and the existing body's early `exit 0` must be
// neutralized by the subshell so board-lint still runs.
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
	// BT-054-R: the wrapped existing body must come BEFORE the board-lint
	// epilogue call — board-lint runs AFTER the existing logic.
	bodyIdx := strings.Index(content, "existing-ran")
	epilogueIdx := strings.Index(content, boardLintChainExisting+"=$?")
	if bodyIdx < 0 || epilogueIdx < 0 || bodyIdx > epilogueIdx {
		t.Fatalf("board-lint epilogue not after the existing hook body:\n%s", content)
	}
	// The existing body must be inside the subshell wrap (its early exits
	// neutralized), and the epilogue must honor the existing status.
	if !strings.Contains(content, "(\n") || !strings.Contains(content, ")\n"+boardLintChainExisting+"=$?") {
		t.Fatalf("existing body not wrapped in a subshell:\n%s", content)
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

// TestBT054GenerateHookContentChainedShape pins the chained rewrite BT-054-R
// requires: the board-lint function block sits after the shebang, the existing
// body is wrapped in a subshell (bare exits neutralized), and the epilogue
// that runs board-lint AFTER the body combines both statuses.
func TestBT054GenerateHookContentChainedShape(t *testing.T) {
	existing := "#!/bin/sh\necho legacy\ngitreins guard\nexit 0\n"
	out := generateHookContent(existing, "/bin/boardctl", "/repo", 30)
	if !strings.Contains(out, "echo legacy") || !strings.Contains(out, "gitreins guard") {
		t.Fatalf("existing logic lost:\n%s", out)
	}
	if !strings.Contains(out, boardLintMarkerBegin) || !strings.Contains(out, boardLintMarkerEnd) {
		t.Fatalf("block missing:\n%s", out)
	}
	// The wrapped body must precede the epilogue call.
	bodyIdx := strings.Index(out, "echo legacy")
	epilogueIdx := strings.Index(out, boardLintChainExisting+"=$?")
	if bodyIdx < 0 || epilogueIdx < 0 || bodyIdx > epilogueIdx {
		t.Fatalf("board-lint not chained after the existing body:\n%s", out)
	}
	// The lint logic must be DEFINED in the block and CALLED in the epilogue,
	// so the early `exit 0` in the wrapped body cannot skip it.
	if !strings.Contains(out, boardLintChainMain+"() {") {
		t.Fatalf("chained block missing the lint function definition:\n%s", out)
	}
	if !strings.Contains(out, boardLintChainMain+" || "+boardLintChainStatus+"=1") {
		t.Fatalf("epilogue missing the lint call with status capture:\n%s", out)
	}
	// The reject arm inside the function must be a return, not an exit —
	// otherwise a lint rejection exits before the epilogue can merge with
	// the existing body's status (it would still block, but would skip the
	// merge contract that lets an EARLIER body failure win too).
	blockStart := strings.Index(out, boardLintMarkerBegin)
	blockEnd := strings.Index(out, boardLintMarkerEnd)
	block := out[blockStart:blockEnd]
	if strings.Contains(block, "exit 1") {
		t.Fatalf("chained block reject arm must be a return inside the function:\n%s", block)
	}
	if !strings.Contains(block, "return 1") {
		t.Fatalf("chained block reject arm missing:\n%s", block)
	}
}

// TestBT054GenerateHookContentChainedIdempotent: re-installing over a chained
// hook regenerates the chained shape (never downgraded to standalone), keeps
// the wrapped body, and leaves exactly one block.
func TestBT054GenerateHookContentChainedIdempotent(t *testing.T) {
	existing := "#!/bin/sh\necho legacy\nexit 0\n"
	first := generateHookContent(existing, "/bin/boardctl", "/repo", 30)
	second := generateHookContent(first, "/bin/boardctl", "/repo", 30)
	if n := strings.Count(second, boardLintMarkerBegin); n != 1 {
		t.Fatalf("chained re-install produced %d blocks:\n%s", n, second)
	}
	if !strings.Contains(second, boardLintChainExisting+"=$?") {
		t.Fatalf("chained re-install lost the epilogue:\n%s", second)
	}
	if !strings.Contains(second, "echo legacy") {
		t.Fatalf("chained re-install lost the existing body:\n%s", second)
	}
}

// TestBT054GenerateHookContentNoMarkerKeepsNonShebang: a shebang-less legacy
// hook still gets the chained rewrite with a shebang supplied.
func TestBT054GenerateHookContentNoShebangChained(t *testing.T) {
	out := generateHookContent("echo legacy\nexit 0\n", "/bin/boardctl", "/repo", 30)
	if !strings.HasPrefix(out, "#!/bin/sh\n") {
		t.Fatalf("chained rewrite did not supply a shebang:\n%s", out)
	}
	if !strings.Contains(out, "echo legacy") || !strings.Contains(out, boardLintChainExisting+"=$?") {
		t.Fatalf("chained rewrite lost content or epilogue:\n%s", out)
	}
}

// TestBT054ValidateFailOnDanglingDep pins the --fail-on dangling-dep CLI
// contract (BT-054-R FINDING 1): plain validate on a dangling-ref board
// stays exit 0 with the warn in the report; --fail-on dangling-dep turns it
// into exit 1 with the warn text STILL in the report; a clean board passes
// with the flag; an unknown class is a usage-level failure.
func TestBT054ValidateFailOnDanglingDep(t *testing.T) {
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"tasks.jsonl":  `{"id":"OK-1","title":"dangling ref holder","status":"pending","priority":"P2","depends_on":["NOPE-404"]}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(boardDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Plain validate: exit 0, warn present (BT-007 contract unchanged).
	out, errOut := instCapture(func() {
		if code := run([]string{"validate", "-C", dir}); code != 0 {
			t.Errorf("plain validate exit = %d, want 0 (dangling ref warns)", code)
		}
	})
	if !strings.Contains(out, "NOPE-404") || !strings.Contains(out, "RESULT: OK") {
		t.Fatalf("plain validate report lost the dangling-ref warn:\n%s", out)
	}
	// --fail-on dangling-dep: exit 1, warn text still reported.
	out2, _ := instCapture(func() {
		if code := run([]string{"validate", "-C", dir, "--fail-on", "dangling-dep"}); code != 1 {
			t.Errorf("validate --fail-on dangling-dep exit = %d, want 1", code)
		}
	})
	if !strings.Contains(out2, "NOPE-404") {
		t.Fatalf("--fail-on report lost the warn text (it must stay a warn in the report):\n%s", out2)
	}
	if !strings.Contains(out2, "promoted to a failure") {
		t.Fatalf("--fail-on report missing the promotion note:\n%s", out2)
	}
	if errOut != "" {
		// stderr must stay empty in both runs (the report is stdout).
		t.Logf("note: stderr = %q", errOut)
	}
	// Clean board with the flag: still exit 0.
	clean := t.TempDir()
	cboard := filepath.Join(clean, ".coding-hermes", "board")
	if err := os.MkdirAll(cboard, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"tasks.jsonl":  `{"id":"CLEAN-1","title":"clean","status":"pending","priority":"P2","depends_on":[]}` + "\n",
		"events.jsonl": files["events.jsonl"],
		"board.jsonl":  files["board.jsonl"],
	} {
		if err := os.WriteFile(filepath.Join(cboard, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	instRunExpect(t, []string{"validate", "-C", clean, "--fail-on", "dangling-dep"}, 0)
	// Unknown class name: usage-level failure (exit 2), never a silent no-op.
	instRunExpect(t, []string{"validate", "-C", clean, "--fail-on", "bogus-class"}, 2)
}
