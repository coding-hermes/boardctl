package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// BT-069: --lint-mode on `boardctl install`. enforce (the default) is the
// exact BT-054 behavior: a validate failure rejects the commit. warn prints
// the same evidence behind a WARN-ONLY banner and allows the commit — the
// mode for boards with known, not-yet-fixed lint debt. The mode is recorded
// in the hook body (boardctl_lint_mode="warn") and the marker-based
// in-place replacement carries it, so re-installing with the other mode
// flips an installed hook in place, in both the standalone and the chained
// shape, without duplicating the managed block.
//
// The mode values are spelled as string literals in this file so the tests
// compile (and go RED with loud behavioral failures) against the pre-change
// tree too.
//
// Helpers reused from bt054_install_test.go (same package): instCapture,
// instRunExpect, instRepoRoot, captureStdout, assertUsageShape.

// bt069DupTasks is a tasks.jsonl whose task id appears twice — the cheapest
// validate failure: exit 1 under --fail-on dangling-dep via the
// ErrDuplicateTaskID arm.
const bt069DupTasks = `{"id":"DUP-1","title":"first twin","status":"pending","priority":"P2","depends_on":[]}` + "\n" +
	`{"id":"DUP-1","title":"second twin","status":"pending","priority":"P2","depends_on":[]}` + "\n"

// bt069BoardFixtures are the three board files that make validate fail:
// duplicate task id, plus the minimal events/board rows bt054's fixtures use.
func bt069BoardFixtures() map[string]string {
	return map[string]string{
		"tasks.jsonl":  bt069DupTasks,
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	}
}

// bt069RealBin builds a REAL boardctl binary once per test run and returns
// its path. The hook-executing tests need it because install runs in-process
// (run()) and os.Executable() then records the TEST binary as boardctl_bin —
// a path that exists and is executable, so the hook's missing-binary skip
// never fires, but the test binary is not boardctl and wedges under the
// hook's `timeout` (bt054 dodged this by never executing hooks; BT-069's
// acceptance asks for executed hooks). The helper repoints the generated
// hook's boardctl_bin line at the real binary — the only fixture surgery;
// the rest of the hook is exactly what install wrote.
var (
	bt069BinOnce sync.Once
	bt069BinPath string
	bt069BinErr  error
)

func bt069RealBin(t *testing.T) string {
	t.Helper()
	bt069BinOnce.Do(func() {
		// NOT t.TempDir(): the first caller's temp dir is deleted when its
		// test ends, and every later test's Once hit would repoint hooks at
		// a vanished binary (observed: "skipped (boardctl binary not
		// found)"). A pid-keyed /tmp path outlives every test in the
		// process; one small binary per test run is left behind.
		out := filepath.Join(os.TempDir(), fmt.Sprintf("bt069-boardctl-%d", os.Getpid()))
		cmd := exec.Command("go", "build", "-o", out, "github.com/coding-hermes/boardctl/cmd/boardctl")
		if combined, err := cmd.CombinedOutput(); err != nil {
			bt069BinErr = fmt.Errorf("go build boardctl for hook execution: %v\n%s", err, combined)
			return
		}
		bt069BinPath = out
	})
	if bt069BinErr != nil {
		t.Fatal(bt069BinErr)
	}
	return bt069BinPath
}

// bt069RepointHookBin rewrites the installed hook's boardctl_bin= line to
// point at a real boardctl binary (see bt069RealBin) so the hook can be
// EXECUTED the way git would.
func bt069RepointHookBin(t *testing.T, hookPath, realBin string) {
	t.Helper()
	raw, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	hit := false
	for i, line := range lines {
		if strings.HasPrefix(line, "boardctl_bin=") {
			lines[i] = "boardctl_bin=" + strconv.Quote(realBin)
			hit = true
		}
	}
	if !hit {
		t.Fatalf("installed hook has no boardctl_bin= line:\n%s", raw)
	}
	if err := os.WriteFile(hookPath, []byte(strings.Join(lines, "\n")), 0o755); err != nil {
		t.Fatal(err)
	}
}

// instBreakBoardWithDupID rewrites the seeded board's tasks.jsonl so
// validate --fail-on dangling-dep fails, and proves the fixture actually
// fails (a fixture that validates clean would make every assertion below
// vacuous).
func instBreakBoardWithDupID(t *testing.T, root string) {
	t.Helper()
	boardDir := filepath.Join(root, ".coding-hermes", "board")
	for name, content := range bt069BoardFixtures() {
		if err := os.WriteFile(filepath.Join(boardDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if code := run([]string{"validate", "-C", root, "--fail-on", "dangling-dep"}); code != 1 {
		t.Fatalf("fixture board does not fail validate (exit %d) — the lint-mode tests would be vacuous", code)
	}
}

// bt069RunHook executes an installed hook file the way git would — directly,
// under a minimal environment (no boardctl on PATH, so the hook must reach
// the binary via its recorded absolute path) — and returns the combined
// output.
func bt069RunHook(t *testing.T, path string) (string, error) {
	t.Helper()
	cmd := exec.Command(path)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	combined := &bytes.Buffer{}
	cmd.Stdout = combined
	cmd.Stderr = combined
	err := cmd.Run()
	return combined.String(), err
}

// TestBT069WarnModeStandaloneAllowsCommit: a warn-mode standalone hook on a
// validate-failing board exits 0 and still prints the validate evidence with
// the WARN-ONLY marker.
func TestBT069WarnModeStandaloneAllowsCommit(t *testing.T) {
	root := instRepoRoot(t)
	instBreakBoardWithDupID(t, root)
	instRunExpect(t, []string{"install", "-C", root, "--lint-mode", "warn"}, 0)
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	raw, err := os.ReadFile(hook)
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	if !strings.Contains(content, `boardctl_lint_mode="warn"`) {
		t.Fatalf("hook missing the warn-mode variable:\n%s", content)
	}
	if !strings.Contains(content, "WARN-ONLY — commit allowed (boardctl validate failed)") {
		t.Fatalf("hook missing the WARN-ONLY banner:\n%s", content)
	}
	bt069RepointHookBin(t, hook, bt069RealBin(t))
	out, runErr := bt069RunHook(t, hook)
	if runErr != nil {
		t.Fatalf("warn-mode hook exited nonzero on an invalid board: %v\noutput:\n%s", runErr, out)
	}
	if !strings.Contains(out, "WARN-ONLY") || !strings.Contains(out, "DUP-1") {
		t.Fatalf("warn-mode hook output lost the validate evidence:\n%s", out)
	}
}

// TestBT069EnforceModeStandaloneStillRejects: the default install is
// unchanged — the hook records the enforce mode and a validate failure still
// exits nonzero with the reject evidence.
func TestBT069EnforceModeStandaloneStillRejects(t *testing.T) {
	root := instRepoRoot(t)
	instBreakBoardWithDupID(t, root)
	instRunExpect(t, []string{"install", "-C", root}, 0)
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	raw, err := os.ReadFile(hook)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `boardctl_lint_mode="enforce"`) {
		t.Fatalf("default install did not record the enforce mode:\n%s", raw)
	}
	bt069RepointHookBin(t, hook, bt069RealBin(t))
	out, runErr := bt069RunHook(t, hook)
	if runErr == nil {
		t.Fatalf("enforce-mode hook allowed an invalid board\noutput:\n%s", out)
	}
	if !strings.Contains(out, "commit rejected") || !strings.Contains(out, "DUP-1") {
		t.Fatalf("enforce-mode hook output lost the reject evidence:\n%s", out)
	}
}

// TestBT069LintModeSwitchInPlace: re-installing with the other mode flips
// the installed hook in place — exactly one managed block, no duplicated
// marker, no stale mode line — in both directions.
func TestBT069LintModeSwitchInPlace(t *testing.T) {
	root := instRepoRoot(t)
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")

	instRunExpect(t, []string{"install", "-C", root}, 0)
	c1, err := os.ReadFile(hook)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(c1), `boardctl_lint_mode="enforce"`) {
		t.Fatalf("fresh install is not enforce:\n%s", c1)
	}

	// enforce -> warn on the same repo.
	instRunExpect(t, []string{"install", "-C", root, "--lint-mode", "warn"}, 0)
	c2raw, err := os.ReadFile(hook)
	if err != nil {
		t.Fatal(err)
	}
	c2 := string(c2raw)
	if n := strings.Count(c2, boardLintMarkerBegin); n != 1 {
		t.Fatalf("enforce->warn re-install produced %d managed blocks:\n%s", n, c2)
	}
	if !strings.Contains(c2, `boardctl_lint_mode="warn"`) {
		t.Fatalf("enforce->warn re-install did not flip the mode:\n%s", c2)
	}
	if strings.Contains(c2, `boardctl_lint_mode="enforce"`) {
		t.Fatalf("enforce->warn re-install left a stale enforce mode line:\n%s", c2)
	}

	// warn -> enforce flips back.
	instRunExpect(t, []string{"install", "-C", root}, 0)
	c3raw, err := os.ReadFile(hook)
	if err != nil {
		t.Fatal(err)
	}
	c3 := string(c3raw)
	if n := strings.Count(c3, boardLintMarkerBegin); n != 1 {
		t.Fatalf("warn->enforce re-install produced %d managed blocks:\n%s", n, c3)
	}
	if !strings.Contains(c3, `boardctl_lint_mode="enforce"`) {
		t.Fatalf("warn->enforce re-install did not flip the mode:\n%s", c3)
	}
	if strings.Contains(c3, `boardctl_lint_mode="warn"`) {
		t.Fatalf("warn->enforce re-install left a stale warn mode line:\n%s", c3)
	}
}

// TestBT069WarnChainedBodyRunsAndAllows: the chained shape with --lint-mode
// warn — the existing body still runs (its side effect lands), the lint
// function's warn arm returns 0, and the epilogue exits with the existing
// body's status, so the commit is allowed while the evidence is printed.
func TestBT069WarnChainedBodyRunsAndAllows(t *testing.T) {
	root := instRepoRoot(t)
	instBreakBoardWithDupID(t, root)
	hooks := filepath.Join(root, ".git", "hooks")
	chainFile := filepath.Join(root, "chain.txt")
	existing := "#!/bin/sh\n# legacy guard stub\necho existing-ran >> \"" + chainFile + "\"\nexit 0\n"
	if err := os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	instRunExpect(t, []string{"install", "-C", root, "--lint-mode", "warn"}, 0)
	hook := filepath.Join(hooks, "pre-commit")
	raw, err := os.ReadFile(hook)
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	if !strings.Contains(content, "existing-ran") {
		t.Fatalf("chained warn install lost the existing hook logic:\n%s", content)
	}
	if !strings.Contains(content, `boardctl_lint_mode="warn"`) {
		t.Fatalf("chained warn install missing the warn-mode variable:\n%s", content)
	}
	if !strings.Contains(content, "WARN-ONLY — commit allowed (boardctl validate failed)") {
		t.Fatalf("chained warn install missing the WARN-ONLY banner:\n%s", content)
	}
	if !strings.Contains(content, "return 0") {
		t.Fatalf("chained warn block lost its success-arm return:\n%s", content)
	}
	_ = os.Remove(chainFile)
	bt069RepointHookBin(t, hook, bt069RealBin(t))
	out, runErr := bt069RunHook(t, hook)
	if runErr != nil {
		t.Fatalf("warn chained hook exited nonzero on an invalid board: %v\noutput:\n%s", runErr, out)
	}
	if !strings.Contains(out, "WARN-ONLY") || !strings.Contains(out, "DUP-1") {
		t.Fatalf("warn chained hook output lost the validate evidence:\n%s", out)
	}
	if _, err := os.Stat(chainFile); err != nil {
		t.Fatalf("existing hook body did not run under the chained warn install: %v", err)
	}
}

// TestBT069ChainedModeFlipsInPlace: warn installed over a legacy hook, then
// enforce re-installed over THAT — the chained shape survives both flips, the
// mode line follows the request, and the enforce reject arm is restored as a
// `return 1` inside the function (never an `exit`).
func TestBT069ChainedModeFlipsInPlace(t *testing.T) {
	existing := "#!/bin/sh\necho legacy\nexit 0\n"
	warn := generateHookContent(existing, "/bin/boardctl", "/repo", 30, boardLintModeWarn)
	enforce := generateHookContent(warn, "/bin/boardctl", "/repo", 30, boardLintModeEnforce)
	if n := strings.Count(enforce, boardLintMarkerBegin); n != 1 {
		t.Fatalf("chained warn->enforce produced %d managed blocks:\n%s", n, enforce)
	}
	if strings.Contains(enforce, `boardctl_lint_mode="warn"`) {
		t.Fatalf("chained warn->enforce left a stale warn mode line:\n%s", enforce)
	}
	if !strings.Contains(enforce, boardLintChainExisting+"=$?") {
		t.Fatalf("chained warn->enforce lost the epilogue:\n%s", enforce)
	}
	if !strings.Contains(enforce, "echo legacy") {
		t.Fatalf("chained warn->enforce lost the existing body:\n%s", enforce)
	}
	blockStart := strings.Index(enforce, boardLintMarkerBegin)
	blockEnd := strings.Index(enforce, boardLintMarkerEnd)
	block := enforce[blockStart:blockEnd]
	if strings.Contains(block, "exit 1") {
		t.Fatalf("chained enforce reject arm must be a return inside the function:\n%s", block)
	}
	if !strings.Contains(block, "return 1") {
		t.Fatalf("chained warn->enforce did not restore the reject-arm return:\n%s", block)
	}
}

// TestBT069LintModeBadValueIsUsageError: an unknown --lint-mode value is a
// usage-level failure (exit 2, the errUsage class) and writes nothing.
func TestBT069LintModeBadValueIsUsageError(t *testing.T) {
	root := instRepoRoot(t)
	instRunExpect(t, []string{"install", "-C", root, "--lint-mode", "bogus"}, 2)
	instRunExpect(t, []string{"install", "-C", root, "--lint-mode=bogus"}, 2)
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(hook); !os.IsNotExist(err) {
		t.Fatalf("a refused --lint-mode must not write the hook (stat err=%v)", err)
	}
}

// TestBT069LintModeDocumentedInUsage: the install usage closure and the root
// usageText document --lint-mode, and install --help keeps the DF-BOARDCTL-7
// output shape.
func TestBT069LintModeDocumentedInUsage(t *testing.T) {
	got, err := captureStdout(func() {
		if code := run([]string{"install", "--help"}); code != 0 {
			t.Fatalf("install --help exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	assertUsageShape(t, got, usageCase{name: "install --help", usageSig: "boardctl install"})
	if !strings.Contains(got, "--lint-mode") {
		t.Errorf("install --help missing --lint-mode:\n%s", got)
	}
	if !strings.Contains(usageText, "--lint-mode enforce|warn") {
		t.Errorf("usageText install entry lost --lint-mode enforce|warn")
	}
}
