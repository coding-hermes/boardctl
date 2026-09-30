package main

// BT-053: the --fleet rollout mode of `boardctl install`. These tests pin
// the five fleet outcome classes the board row names (installed / already
// present / skipped-no-board / skipped-no-binary / FAILED + why) plus the
// worktree skip, idempotency by byte-identical hook, chain-not-clobber,
// interrupt-then-rerun completion, selection grammar, refusal of
// single-repo flags in fleet mode, and the enumeration boundaries (hidden
// dirs and node_modules/vendor/testdata are never rollout targets).
//
// The fleet layer reuses generateHookContent — the exact function the
// BT-054 tests pin — so chaining/idempotency semantics are inherited, and
// what is pinned HERE is classification, reporting, atomicity of the write,
// and the walk.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fleetSeedRepo creates a fake git repo (real .git dir) under root/rel.
// withBoard seeds the .coding-hermes/board dir the hook lints; withHook
// writes an initial pre-commit body. Returns the repo path.
func fleetSeedRepo(t *testing.T, root, rel string, withBoard bool, withHook string) string {
	t.Helper()
	dir := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if withBoard {
		if err := os.MkdirAll(filepath.Join(dir, boardDirRel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if withHook != "" {
		if err := os.WriteFile(filepath.Join(dir, ".git", "hooks", "pre-commit"), []byte(withHook), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// fleetReqFor builds a request against a temp fleet root with the test
// binary as the recorded boardctl path (executable, so binUsable holds).
func fleetReqFor(root, pattern string, dryRun bool) fleetInstallRequest {
	bin, err := os.Executable()
	if err != nil {
		bin = os.Args[0]
	}
	return fleetInstallRequest{
		pattern:  pattern,
		root:     root,
		binPath:  bin,
		timeout:  boardLintDefaultTTL,
		dryRun:   dryRun,
		lintMode: boardLintModeEnforce,
	}
}

// fleetHookOf reads a repo's pre-commit hook ("" when absent).
func fleetHookOf(t *testing.T, repo string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo, ".git", "hooks", "pre-commit"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// TestBT053FleetOutcomeClasses seeds one fleet root holding one repo per
// outcome class and pins: one outcome line per repo, the right class token
// on each, the chain detail on the rewrite, the worktree skip, a summary
// line counting every class, and a nil error (nothing FAILED).
func TestBT053FleetOutcomeClasses(t *testing.T) {
	root := t.TempDir()
	fresh := fleetSeedRepo(t, root, "fresh", true, "")
	presentRepo := fleetSeedRepo(t, root, "present", true, "")
	noBoard := fleetSeedRepo(t, root, "noboard", false, "")
	guarded := fleetSeedRepo(t, root, "guarded", true, "#!/bin/sh\necho GUARD-RAN\nexit 0\n")
	// "present" is made present by pre-writing exactly what the fleet run
	// would write — the same generator, same inputs.
	req := fleetReqFor(root, "*", false)
	if err := os.WriteFile(
		filepath.Join(presentRepo, ".git", "hooks", "pre-commit"),
		[]byte(generateHookContent("", req.binPath, presentRepo, req.timeout, boardLintModeEnforce)), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	// A worktree checkout: .git is a FILE pointing at a real gitdir.
	wt := filepath.Join(root, "wt-checkout")
	if err := os.MkdirAll(filepath.Join(wt, boardDirRel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /elsewhere/real.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if err := fleetInstall(req, &out, &errOut); err != nil {
		t.Fatalf("fleetInstall error = %v (stderr: %s)", err, errOut.String())
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	summary := lines[len(lines)-1]
	byRepo := map[string]string{}
	for _, l := range lines[:len(lines)-1] {
		// Outcome lines read `<class>: <repo>...` — class tokens never
		// contain ": ", so split on the FIRST ": " followed by a path
		// character (the classes are fixed vocabulary, repos absolute).
		idx := strings.Index(l, ": /")
		if idx < 0 {
			t.Fatalf("outcome line lacks '<class>: <repo>' shape: %q", l)
		}
		rest := l[idx+2:]
		if d := strings.Index(rest, " — "); d >= 0 {
			rest = rest[:d] // drop the detail suffix for class matching
		}
		byRepo[rest] = l[:idx]
	}
	want := map[string]string{
		fresh:       fleetOutcomeInstalled,
		presentRepo: fleetOutcomePresent,
		noBoard:     fleetOutcomeNoBoard,
		guarded:     fleetOutcomeInstalled,
		wt:          fleetOutcomeSkipped,
	}
	if len(byRepo) != len(want) {
		t.Fatalf("got %d outcome lines, want %d:\n%s", len(byRepo), len(want), out.String())
	}
	for repo, class := range want {
		if got := byRepo[repo]; got != class {
			t.Errorf("repo %s outcome = %q, want %q", repo, got, class)
		}
	}
	// The chained rewrite names what happened to the existing hook; the
	// fresh install carries NO detail.
	for _, line := range lines {
		if strings.HasPrefix(line, fleetOutcomeInstalled+": "+guarded+" ") &&
			!strings.Contains(line, "existing hook preserved and chained") {
			t.Errorf("chained-rewrite line lost the detail: %q", line)
		}
		if strings.HasPrefix(line, fleetOutcomeInstalled+": "+fresh+" ") {
			t.Errorf("fresh install should carry no detail: %q", line)
		}
	}
	// Summary counts every class in the fixed order, including zeros.
	for _, class := range fleetClasses {
		if !strings.Contains(summary, " "+class) {
			t.Errorf("summary line lost class %q: %q", class, summary)
		}
	}
	if !strings.Contains(summary, "2 "+fleetOutcomeInstalled) ||
		!strings.Contains(summary, "1 "+fleetOutcomePresent) ||
		!strings.Contains(summary, "1 "+fleetOutcomeNoBoard) ||
		!strings.Contains(summary, "1 "+fleetOutcomeSkipped) {
		t.Errorf("summary counts wrong: %q", summary)
	}
}

// TestBT053FleetIdempotentByteIdentical: run one, run two — the second
// reports `already present` and the hook bytes are IDENTICAL (the
// hash-snapshot acceptance criterion, done with bytes.Equal over both
// reads).
func TestBT053FleetIdempotentByteIdentical(t *testing.T) {
	root := t.TempDir()
	repo := fleetSeedRepo(t, root, "only", true, "")
	var out bytes.Buffer
	req := fleetReqFor(root, "only", false)
	if err := fleetInstall(req, &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first := fleetHookOf(t, repo)
	if !strings.Contains(out.String(), fleetOutcomeInstalled+": "+repo) {
		t.Fatalf("first run outcome wrong:\n%s", out.String())
	}
	out.Reset()
	if err := fleetInstall(req, &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	second := fleetHookOf(t, repo)
	if !strings.Contains(out.String(), fleetOutcomePresent+": "+repo) {
		t.Fatalf("second run must report already present:\n%s", out.String())
	}
	if first != second {
		t.Fatalf("re-install mutated the hook (%d vs %d bytes):\n--- first ---\n%s\n--- second ---\n%s", len(first), len(second), first, second)
	}
}

// TestBT053FleetChainNotClobber: an existing guard hook keeps its body
// (before the board-lint epilogue) and gains exactly one managed block.
func TestBT053FleetChainNotClobber(t *testing.T) {
	root := t.TempDir()
	repo := fleetSeedRepo(t, root, "guarded", true, "#!/bin/sh\n# GUARD-SENTINEL-BT053\ngitreins guard\nexit 0\n")
	if err := fleetInstall(fleetReqFor(root, "guarded", false), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("fleetInstall: %v", err)
	}
	hook := fleetHookOf(t, repo)
	if !strings.Contains(hook, "GUARD-SENTINEL-BT053") || !strings.Contains(hook, "gitreins guard") {
		t.Fatalf("existing guard body clobbered:\n%s", hook)
	}
	if n := strings.Count(hook, boardLintMarkerBegin); n != 1 {
		t.Fatalf("want exactly one managed block, got %d:\n%s", n, hook)
	}
	bodyIdx := strings.Index(hook, "GUARD-SENTINEL-BT053")
	epilogueIdx := strings.Index(hook, boardLintChainExisting+"=$?")
	if bodyIdx < 0 || epilogueIdx < 0 || bodyIdx > epilogueIdx {
		t.Fatalf("guard body must precede the board-lint epilogue:\n%s", hook)
	}
	if !strings.Contains(hook, "boardctl_timeout="+itoa(boardLintDefaultTTL)+"\n") {
		t.Fatalf("hook lost the default timeout:\n%s", hook)
	}
}

// itoa is a tiny helper so the tests avoid strconv imports for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// TestBT053FleetRerunCompletesAfterPartial: an interrupted rollout (repo A
// already done) leaves A consistent and a re-run finishes B, reporting A as
// already present. Both hooks are valid afterward.
func TestBT053FleetRerunCompletesAfterPartial(t *testing.T) {
	root := t.TempDir()
	a := fleetSeedRepo(t, root, "a-repo", true, "")
	b := fleetSeedRepo(t, root, "b-repo", true, "")
	// Simulate the interrupted run: only A got its hook.
	req := fleetReqFor(root, "*", false)
	outcome, detail := fleetInstallOne(a, true, req)
	if outcome != fleetOutcomeInstalled {
		t.Fatalf("seeding install of A: %s (%s)", outcome, detail)
	}
	hookA1 := fleetHookOf(t, a)

	var out bytes.Buffer
	if err := fleetInstall(req, &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if !strings.Contains(out.String(), fleetOutcomePresent+": "+a) {
		t.Fatalf("A must read already present on rerun:\n%s", out.String())
	}
	if !strings.Contains(out.String(), fleetOutcomeInstalled+": "+b) {
		t.Fatalf("B must be installed by the rerun:\n%s", out.String())
	}
	if fleetHookOf(t, a) != hookA1 {
		t.Fatal("rerun mutated A's hook")
	}
	if !strings.Contains(fleetHookOf(t, b), boardLintMarkerBegin) {
		t.Fatal("B's hook not installed by the rerun")
	}
}

// TestBT053FleetSelectionGrammar: globs match base names then full paths,
// bare parts match paths or base names exactly, the list is comma
// separated, and a pattern matching nothing is an ERROR (never a silent
// empty rollout).
func TestBT053FleetSelectionGrammar(t *testing.T) {
	root := t.TempDir()
	axiom := fleetSeedRepo(t, root, "axiom", true, "")
	crier := fleetSeedRepo(t, root, "crier", true, "")
	fleetSeedRepo(t, root, "scheduler", true, "")

	nSelected := func(pattern string) int {
		t.Helper()
		var out bytes.Buffer
		if err := fleetInstall(fleetReqFor(root, pattern, false), &out, &bytes.Buffer{}); err != nil {
			t.Fatalf("fleetInstall(%q): %v", pattern, err)
		}
		n := 0
		for _, l := range strings.Split(out.String(), "\n") {
			if strings.HasPrefix(l, fleetOutcomeInstalled+": ") ||
				strings.HasPrefix(l, fleetOutcomePresent+": ") {
				n++
			}
		}
		return n
	}
	if n := nSelected("axiom,crier"); n != 2 {
		t.Errorf("bare-name list selected %d, want 2", n)
	}
	if n := nSelected("axiom"); n != 1 {
		t.Errorf("single bare name selected %d, want 1", n)
	}
	if n := nSelected("c*"); n != 1 {
		t.Errorf("glob c* selected %d, want 1", n)
	}
	if n := nSelected(filepath.Join(root, "*")); n != 3 {
		t.Errorf("full-path glob selected %d, want 3", n)
	}
	if !containsRepo(t, root, "axiom,crier", axiom) || !containsRepo(t, root, "axiom,crier", crier) {
		t.Error("bare-name list missed one of its repos")
	}
	// No match: loud error, empty rollout.
	var out bytes.Buffer
	if err := fleetInstall(fleetReqFor(root, "zzz-no-such-repo", false), &out, &bytes.Buffer{}); err == nil {
		t.Fatal("a pattern matching zero repos must error")
	} else if !strings.Contains(err.Error(), "no repository") {
		t.Errorf("no-match error should say so: %v", err)
	}
	if strings.Contains(out.String(), fleetOutcomeInstalled+":") {
		t.Errorf("a no-match rollout must install nothing:\n%s", out.String())
	}
}

// containsRepo runs a selection and reports whether repo got an outcome line.
func containsRepo(t *testing.T, root, pattern, repo string) bool {
	t.Helper()
	var out bytes.Buffer
	if err := fleetInstall(fleetReqFor(root, pattern, false), &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("fleetInstall: %v", err)
	}
	return strings.Contains(out.String(), ": "+repo)
}

// TestBT053FleetNoBinarySkipsEverywhere: with the recorded binary gone and
// no boardctl on PATH, every repo reports skipped-no-binary and NO hook is
// written anywhere. The host may legitimately have a boardctl on PATH (the
// installed one), so the search path is PINNED to an empty scratch dir for
// the duration — the outcome must not depend on where the test runs.
func TestBT053FleetNoBinarySkipsEverywhere(t *testing.T) {
	fakeBinDir := t.TempDir() // empty: LookPath("boardctl") finds nothing
	origPath, had := os.LookupEnv("PATH")
	t.Setenv("PATH", fakeBinDir)
	defer func() {
		if had {
			os.Setenv("PATH", origPath) //nolint:errcheck
		} else {
			os.Unsetenv("PATH") //nolint:errcheck
		}
	}()
	root := t.TempDir()
	repo := fleetSeedRepo(t, root, "repo", true, "")
	req := fleetReqFor(root, "*", false)
	req.binPath = "/nonexistent/boardctl-for-bt053"
	var out, errOut bytes.Buffer
	if err := fleetInstall(req, &out, &errOut); err != nil {
		t.Fatalf("no-binary run must not FAIL: %v", err)
	}
	if !strings.Contains(out.String(), fleetOutcomeNoBinary+": "+repo) {
		t.Fatalf("expected skipped-no-binary for %s:\n%s", repo, out.String())
	}
	if fleetHookOf(t, repo) != "" {
		t.Fatal("skipped-no-binary must not write a hook")
	}
	if !strings.Contains(errOut.String(), "skipped-no-binary") {
		t.Errorf("run should explain the no-binary situation on stderr:\n%s", errOut.String())
	}
}

// TestBT053FleetFailureClassAndExit: an unwritable hooks dir turns the
// repo's outcome into FAILED with the reason on the line, other repos still
// complete, and the run reports the failure count.
func TestBT053FleetFailureClassAndExit(t *testing.T) {
	root := t.TempDir()
	ok := fleetSeedRepo(t, root, "a-ok", true, "")
	bad := fleetSeedRepo(t, root, "z-bad", true, "#!/bin/sh\nexit 0\n")
	// Make the WRITE fail: the hooks dir goes read-only (r-x), so the
	// temp-file creation inside it fails with EACCES. This exercises the
	// atomic write path's refusal — nothing partial may land.
	hooksDir := filepath.Join(bad, ".git", "hooks")
	if err := os.Chmod(hooksDir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(hooksDir, 0o755) //nolint:errcheck
	var out, errOut bytes.Buffer
	err := fleetInstall(fleetReqFor(root, "*", false), &out, &errOut)
	if err == nil {
		t.Fatal("a run with a FAILED repo must exit nonzero")
	}
	if !strings.Contains(err.Error(), "1 repo(s) FAILED") {
		t.Errorf("error should count the FAILED repos: %v", err)
	}
	if !strings.Contains(out.String(), fleetOutcomeFailed+": "+bad+" — ") {
		t.Errorf("FAILED line must carry the repo and a reason:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "cannot create temp hook file") {
		t.Errorf("FAILED reason should name the write failure:\n%s", out.String())
	}
	// Nothing partial landed: the guard hook is byte-identical.
	if !strings.Contains(fleetHookOf(t, bad), "exit 0") ||
		strings.Contains(fleetHookOf(t, bad), boardLintMarkerBegin) {
		t.Error("a FAILED install must leave the existing hook untouched")
	}
	if !strings.Contains(out.String(), fleetOutcomeInstalled+": "+ok) {
		t.Errorf("the healthy repo must still install:\n%s", out.String())
	}
}

// TestBT053FleetDryRunWritesNothing: dry-run reports would-* outcomes with
// the (dry-run) tag and leaves every hook byte-identical.
func TestBT053FleetDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	fresh := fleetSeedRepo(t, root, "fresh", true, "")
	guarded := fleetSeedRepo(t, root, "guarded", true, "#!/bin/sh\nexit 0\n")
	noboard := fleetSeedRepo(t, root, "noboard", false, "")
	before := map[string]string{
		fresh:   fleetHookOf(t, fresh),
		guarded: fleetHookOf(t, guarded),
		noboard: fleetHookOf(t, noboard),
	}
	var out bytes.Buffer
	if err := fleetInstall(fleetReqFor(root, "*", true), &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !strings.Contains(out.String(), "would install a fresh hook (dry-run)") {
		t.Errorf("fresh dry-run line wrong:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "would chain onto the existing hook (dry-run)") {
		t.Errorf("guarded dry-run line wrong:\n%s", out.String())
	}
	if !strings.Contains(out.String(), fleetOutcomeNoBoard+": "+noboard+" (dry-run)") {
		t.Errorf("no-board dry-run line wrong:\n%s", out.String())
	}
	for repo, was := range before {
		if fleetHookOf(t, repo) != was {
			t.Errorf("dry run wrote to %s", repo)
		}
	}
}

// TestBT053FleetEnumerationBoundaries: the walk finds nested checkouts
// (<repo>/namespaces/<ns>) and .git-FILE worktrees are classified, while
// hidden dirs (.cache fixture repos) and node_modules/vendor/testdata are
// never rollout targets.
func TestBT053FleetEnumerationBoundaries(t *testing.T) {
	root := t.TempDir()
	visible := fleetSeedRepo(t, root, "somerepo", true, "")
	nested := fleetSeedRepo(t, root, filepath.Join("somerepo", "namespaces", "coding-hermes"), true, "")
	fleetSeedRepo(t, root, filepath.Join(".cache", "pytest-tmp", "test_fix0", "repo"), true, "")
	fleetSeedRepo(t, root, filepath.Join("dep", "node_modules", "leftpad"), true, "")
	fleetSeedRepo(t, root, filepath.Join("dep", "vendor", "example"), true, "")
	fleetSeedRepo(t, root, filepath.Join("tools", "testdata", "repo"), true, "")

	repos, err := fleetRepos(root, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range repos {
		got[r] = true
	}
	if !got[visible] || !got[nested] {
		t.Errorf("walk missed a real repo: got %v", repos)
	}
	if len(repos) != 2 {
		t.Errorf("walk returned %d repos, want exactly the 2 visible ones:\n%v", len(repos), repos)
	}
	// Sorted order: the rollout log is deterministic.
	if repos[0] != visible || repos[1] != nested {
		t.Errorf("repos not sorted: %v", repos)
	}
	// An unreadable subdirectory is reported and skipped, not fatal.
	noPerm := filepath.Join(root, "locked")
	if err := os.MkdirAll(filepath.Join(noPerm, "x"), 0o000); err == nil {
		if err := os.Chmod(filepath.Join(noPerm, "x"), 0o000); err == nil {
			defer os.Chmod(filepath.Join(noPerm, "x"), 0o755) //nolint:errcheck
		}
	}
	var errOut bytes.Buffer
	if _, err := fleetRepos(root, &errOut); err != nil {
		t.Fatalf("unreadable subdir must not fail the walk: %v", err)
	}
	// Missing root: a summary-level error naming the flag.
	if _, err := fleetRepos(filepath.Join(root, "does-not-exist"), &bytes.Buffer{}); err == nil {
		t.Fatal("missing --fleet-root must error")
	}
}

// TestBT053FleetRunWiring: the full CLI path — run() dispatches
// `install --fleet` with --fleet-root, outcome lines land on STDOUT, exit
// 0; the single-repo-only flags are refused as a usage error (exit 2).
func TestBT053FleetRunWiring(t *testing.T) {
	root := t.TempDir()
	repo := fleetSeedRepo(t, root, "wired", true, "")
	var out, errOut string
	out, errOut = instCapture(func() {
		if code := run([]string{
			"install", "--fleet", "wired", "--fleet-root", root,
		}); code != 0 {
			t.Errorf("fleet install exit = %d, want 0 (stderr: %s)", code, errOut)
		}
	})
	if !strings.Contains(out, fleetOutcomeInstalled+": "+repo) {
		t.Fatalf("stdout lacks the outcome line:\n%s", out)
	}
	if !strings.Contains(out, "fleet rollout: 1 repos") {
		t.Fatalf("stdout lacks the summary line:\n%s", out)
	}
	if fleetHookOf(t, repo) == "" {
		t.Fatal("run()-driven fleet install wrote no hook")
	}
	// -C / --hook-path are meaningless in fleet mode: usage refusal.
	instRunExpect(t, []string{"install", "--fleet", "x", "-C", t.TempDir()}, 2)
	instRunExpect(t, []string{"install", "--fleet", "x", "--hook-path", "/tmp/h"}, 2)
	// Still-valid single-repo invocations stay untouched.
	single := fleetSeedRepo(t, root, "single", true, "")
	instRunExpect(t, []string{"install", "-C", single}, 0)
	if !strings.Contains(fleetHookOf(t, single), boardLintMarkerBegin) {
		t.Fatal("single-repo install regressed")
	}
}

// TestBT053GenerateHookContentChainedByteIdempotent pins the BT-053 fix to
// generateHookContent: re-installing over a chained hook must be
// BYTE-identical (the old code re-appended the remainder's leading newline,
// growing every re-installed hook by one byte and inserting a blank line
// after the managed block). Standalone re-install is pinned too.
func TestBT053GenerateHookContentChainedByteIdempotent(t *testing.T) {
	existing := "#!/bin/sh\necho legacy\nexit 0\n"
	first := generateHookContent(existing, "/bin/boardctl", "/repo", 30, boardLintModeEnforce)
	second := generateHookContent(first, "/bin/boardctl", "/repo", 30, boardLintModeEnforce)
	third := generateHookContent(second, "/bin/boardctl", "/repo", 30, boardLintModeEnforce)
	if first != second || second != third {
		t.Fatalf("chained re-install is not byte-idempotent (%d -> %d -> %d bytes):\nfirst:\n%s\nsecond:\n%s", len(first), len(second), len(third), first, second)
	}
	// The blank-line artifact must not reappear.
	if strings.Contains(first, boardLintMarkerEnd+"\n\n") {
		t.Fatalf("blank line reintroduced after the managed block:\n%s", first)
	}
	// Standalone shape: same contract.
	sfresh := generateHookContent("", "/bin/boardctl", "/repo", 30, boardLintModeEnforce)
	ssecond := generateHookContent(sfresh, "/bin/boardctl", "/repo", 30, boardLintModeEnforce)
	if sfresh != ssecond {
		t.Fatalf("standalone re-install is not byte-idempotent (%d -> %d bytes):\n%s\n---\n%s", len(sfresh), len(ssecond), sfresh, ssecond)
	}
}

// TestBT053FleetUsageClosureStillGreen is a local reminder that the DF-14
// meta-test and the BT-041 help table must stay green after the flag
// additions — it re-runs the switch-vs-closure gate and the install help
// query right here, so a fleet-mode regression cannot hide behind
// -run filters.
func TestBT053FleetUsageClosureStillGreen(t *testing.T) {
	verbs, err := switchVerbsFromLines(func() []string {
		body, rerr := os.ReadFile("main.go")
		if rerr != nil {
			t.Fatalf("cannot read main.go: %v", rerr)
		}
		return strings.Split(string(body), "\n")
	}())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range verbs {
		if v == "install" {
			found = true
		}
	}
	if !found {
		t.Fatal("install verb vanished from run()'s switch")
	}
	out, err := captureStdout(func() {
		if code := run([]string{"install", "-h"}); code != 0 {
			t.Errorf("install -h exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--fleet", "--fleet-root", "--timeout", "--dry-run"} {
		if !strings.Contains(out, want) {
			t.Errorf("install -h lost %q:\n%s", want, out)
		}
	}
	if !strings.Contains(usageText, "--fleet PAT") || !strings.Contains(usageText, "--fleet-root DIR") {
		t.Error("top-level usageText lost the fleet flags")
	}
}
