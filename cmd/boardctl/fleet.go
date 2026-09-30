// BT-053: `boardctl install --fleet` — the fleet rollout mode of the
// per-repo installer landed in BT-054. One invocation enumerates every git
// repository under --fleet-root (default ~), classifies it, installs the
// SAME hook the single-repo command writes (chaining an existing hook,
// never clobbering one), and prints exactly ONE outcome line per repo:
//
//	installed:         the hook was written — fresh install, chained
//	                   rewrite of an existing hook, or a managed-block
//	                   update; the detail names which
//	already present:   the existing hook already equals what this
//	                   invocation would write — a true no-op, nothing
//	                   written
//	skipped-no-board:  the repo carries no .coding-hermes/board dir (the
//	                   hook would skip every commit anyway); nothing written
//	skipped-no-binary: the running boardctl binary is unreachable the way a
//	                   hook reaches it (recorded path gone AND no boardctl
//	                   on PATH) — installing would write a hook that skips
//	                   everything; nothing written
//	skipped:           a git WORKTREE checkout (.git is a file): the hook
//	                   lives in the main checkout's .git dir; nothing
//	                   written
//	FAILED:            an I/O error; the reason is on the line and the run
//	                   exits 1 after reporting every repo
//
// Outcome lines read `<class>: <repo>[ — <detail>][ (dry-run)]` so a
// rollout log is greppable by class token. The run closes with a summary
// line counting every class.
//
// Per-repo work is independent and the hook lands via temp-file + rename,
// so an interrupted run leaves every completed repo consistent and a re-run
// finishes the rest, reporting `already present` for the done ones. The
// rollout never writes .coding-hermes/board, .gitreins, or anything outside
// .git/hooks — safe to run while foreman ticks are live.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// boardDirRel is the board dir the hook lints, relative to a repo root. It
// must stay in sync with boardLintHookBody's early-skip test (that test is
// pinned textually by TestBT054HookBodyExitContract's "no-board early skip"
// needle).
const boardDirRel = ".coding-hermes/board"

// The fleet outcome classes — the complete vocabulary of an outcome line.
const (
	fleetOutcomeInstalled = "installed"
	fleetOutcomePresent   = "already present"
	fleetOutcomeNoBoard   = "skipped-no-board"
	fleetOutcomeNoBinary  = "skipped-no-binary"
	fleetOutcomeSkipped   = "skipped"
	fleetOutcomeFailed    = "FAILED"
)

// fleetClasses is the fixed summary order. Zeros are printed too, so a
// summary line always has the same shape to parse.
var fleetClasses = []string{
	fleetOutcomeInstalled,
	fleetOutcomePresent,
	fleetOutcomeNoBoard,
	fleetOutcomeNoBinary,
	fleetOutcomeSkipped,
	fleetOutcomeFailed,
}

// fleetMaxDepth caps the repository walk as a runaway guard. The deepest
// real fleet layout (/home/kara/<repo>/namespaces/<ns>) sits three levels
// below the root.
const fleetMaxDepth = 8

// fleetSkipDirNames are VISIBLE directory names the walk never descends
// into: dependency trees and test fixtures, never rollout targets. Hidden
// dirs (dot-prefixed) are skipped by rule.
var fleetSkipDirNames = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	"testdata":     true,
}

// fleetInstallRequest carries one --fleet invocation's resolved inputs.
type fleetInstallRequest struct {
	pattern  string // comma-separated glob-or-path list (never empty)
	root     string // enumeration root ("" = home directory)
	binPath  string // the running boardctl binary, recorded into hooks
	timeout  int    // hook timeout seconds (already validated >= 1)
	dryRun   bool   // report outcomes, write nothing
	lintMode string // BT-069 lint mode carried into every hook (validated by the caller)
}

// fleetInstall runs the rollout: enumerate, select, classify+install one
// pass per repo, print one outcome line per repo and a closing summary.
// Outcome lines go to out, diagnostics to errOut. The returned error is
// nil, or a summary-level failure (bad root, no matching repo, bad
// pattern, at least one FAILED repo) — per-repo failures never abort the
// loop, they only mark that repo's line.
func fleetInstall(req fleetInstallRequest, out, errOut io.Writer) error {
	root, err := fleetRootDir(req.root)
	if err != nil {
		return err
	}
	repos, err := fleetRepos(root, errOut)
	if err != nil {
		return err
	}
	var selected []string
	for _, repo := range repos {
		ok, err := fleetRepoSelected(repo, req.pattern)
		if err != nil {
			return err
		}
		if ok {
			selected = append(selected, repo)
		}
	}
	if len(selected) == 0 {
		return fmt.Errorf("boardctl install: no repository under %s matches --fleet %q", root, req.pattern)
	}
	binUsable := fleetBinUsable(req.binPath)
	if !binUsable {
		fmt.Fprintln(errOut, "boardctl install: this boardctl binary is unreachable the way a hook reaches it (recorded path missing or non-executable, no boardctl on PATH); every repo will report skipped-no-binary")
	}
	summary := map[string]int{}
	failed := 0
	for _, repo := range selected {
		outcome, detail := fleetInstallOne(repo, binUsable, req)
		summary[outcome]++
		if outcome == fleetOutcomeFailed {
			failed++
		}
		line := fmt.Sprintf("%s: %s", outcome, repo)
		if detail != "" {
			line += " — " + detail
		}
		if req.dryRun {
			line += " (dry-run)"
		}
		fmt.Fprintln(out, line)
	}
	var parts []string
	for _, class := range fleetClasses {
		parts = append(parts, fmt.Sprintf("%d %s", summary[class], class))
	}
	fmt.Fprintf(out, "fleet rollout: %d repos — %s\n", len(selected), strings.Join(parts, ", "))
	if failed > 0 {
		return fmt.Errorf("fleet rollout: %d repo(s) FAILED (see the FAILED lines above)", failed)
	}
	return nil
}

// fleetRootDir resolves the enumeration root: the flag value, cleaned, or
// the home directory when unset.
func fleetRootDir(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("boardctl install: --fleet-root could not default to the home directory: %w", err)
		}
		return home, nil
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("boardctl install: cannot resolve --fleet-root %s: %w", root, err)
	}
	return abs, nil
}

// fleetRepos enumerates every git repository under root: a directory
// holding a .git entry (directory or worktree FILE). The walk descends
// through visible, non-symlink directories only — hidden dirs (.cache,
// .hermes, ...) and fleetSkipDirNames are fixture/dependency territory no
// rollout should touch — and does NOT stop at a discovered repo, because
// nested checkouts (e.g. <repo>/namespaces/*) are fleet repos too. Depth
// is capped at fleetMaxDepth. Directories that cannot be read are reported
// to errOut and skipped; only an unreadable ROOT is an error. The result
// is sorted for a stable rollout order.
func fleetRepos(root string, errOut io.Writer) ([]string, error) {
	var repos []string
	seen := map[string]bool{}
	var walk func(dir string, depth int) error
	walk = func(dir string, depth int) error {
		if depth > fleetMaxDepth {
			return nil
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if depth <= 1 {
				return fmt.Errorf("boardctl install: cannot read --fleet-root %s: %w", dir, err)
			}
			if errOut != nil {
				fmt.Fprintf(errOut, "boardctl install: cannot read directory %s: %v\n", dir, err)
			}
			return nil
		}
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") || fleetSkipDirNames[name] {
				continue
			}
			// IsDir() is false for symlinks: the walk never follows them.
			if !e.IsDir() {
				continue
			}
			full := filepath.Join(dir, name)
			if seen[full] {
				continue
			}
			seen[full] = true
			if gitEntry, err := os.Stat(filepath.Join(full, ".git")); err == nil &&
				(gitEntry.IsDir() || gitEntry.Mode().IsRegular()) {
				repos = append(repos, full)
			}
			if err := walk(full, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root, 1); err != nil {
		return nil, err
	}
	sort.Strings(repos)
	return repos, nil
}

// fleetRepoSelected reports whether repo matches the --fleet pattern: a
// comma-separated list whose parts are either bare paths (a whole-part
// match of the repo's full path or base name) or globs (path.Match syntax,
// POSIX separators) matched against the base name and then the full path.
func fleetRepoSelected(repo, pattern string) (bool, error) {
	for _, pat := range strings.Split(pattern, ",") {
		pat = strings.TrimSpace(pat)
		if pat == "" {
			continue
		}
		if !strings.ContainsAny(pat, "*?[") {
			if pat == repo || pat == filepath.Base(repo) {
				return true, nil
			}
			continue
		}
		for _, candidate := range []string{filepath.Base(repo), repo} {
			ok, err := path.Match(pat, candidate)
			if err != nil {
				return false, fmt.Errorf("boardctl install: bad --fleet pattern %q: %w", pat, err)
			}
			if ok {
				return true, nil
			}
		}
	}
	return false, nil
}

// fleetBinUsable reports whether the running binary is reachable the way a
// hook will reach it: the recorded absolute path executable (the hook's
// first choice) OR `boardctl` on PATH (the hook's fallback). Usable is the
// precondition for installing anything — the alternative is a hook that
// skips every commit.
func fleetBinUsable(binPath string) bool {
	if st, err := os.Stat(binPath); err == nil && st.Mode()&0o111 != 0 {
		return true
	}
	if _, err := exec.LookPath("boardctl"); err == nil {
		return true
	}
	return false
}

// fleetInstallOne classifies one repo and, when it installs, writes the
// hook the single-repo installer would: generateHookContent over the
// existing content (fresh standalone block, chained rewrite, or in-place
// managed-block replacement), executable, moved into place atomically. The
// outcome class and a short detail are returned; nothing is printed here.
func fleetInstallOne(repo string, binUsable bool, req fleetInstallRequest) (string, string) {
	if !binUsable {
		return fleetOutcomeNoBinary, "boardctl binary unreachable by the hook (recorded path missing, none on PATH)"
	}
	if st, err := os.Stat(filepath.Join(repo, boardDirRel)); err != nil || !st.IsDir() {
		return fleetOutcomeNoBoard, ""
	}
	gitDir := filepath.Join(repo, ".git")
	if st, err := os.Stat(gitDir); err != nil {
		return fleetOutcomeFailed, fmt.Sprintf("cannot stat .git: %v", err)
	} else if !st.IsDir() {
		return fleetOutcomeSkipped, "worktree checkout: .git is a file, the hook lives in the main checkout"
	}

	hookPath := filepath.Join(gitDir, "hooks", "pre-commit")
	existing, readErr := os.ReadFile(hookPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fleetOutcomeFailed, fmt.Sprintf("cannot read existing hook: %v", readErr)
	}

	// BT-069: the rollout carries the requested --lint-mode into every hook
	// it writes — the fleet contract is to install the SAME hook the
	// single-repo command writes, and the mode composes cleanly (unlike
	// -C/--hook-path, which name one repo and stay refused in fleet mode).
	content := generateHookContent(string(existing), req.binPath, repo, req.timeout, req.lintMode)
	if string(existing) == content {
		return fleetOutcomePresent, ""
	}
	if req.dryRun {
		if len(existing) == 0 {
			return fleetOutcomeInstalled, "would install a fresh hook"
		}
		if strings.Contains(string(existing), boardLintMarkerBegin) {
			return fleetOutcomeInstalled, "would update the managed block"
		}
		return fleetOutcomeInstalled, "would chain onto the existing hook"
	}

	if err := os.MkdirAll(filepath.Join(gitDir, "hooks"), 0o755); err != nil {
		return fleetOutcomeFailed, fmt.Sprintf("cannot create hooks directory: %v", err)
	}
	hooksDir := filepath.Join(gitDir, "hooks")
	tmp, err := os.CreateTemp(hooksDir, ".boardctl-pre-commit-*")
	if err != nil {
		return fleetOutcomeFailed, fmt.Sprintf("cannot create temp hook file: %v", err)
	}
	tmpName := tmp.Name()
	// No-op once the rename below succeeded; cleans up on every failure path.
	defer os.Remove(tmpName)
	if _, err := tmp.Write([]byte(content)); err != nil {
		tmp.Close()
		return fleetOutcomeFailed, fmt.Sprintf("cannot write temp hook file: %v", err)
	}
	if err := tmp.Close(); err != nil {
		return fleetOutcomeFailed, fmt.Sprintf("cannot close temp hook file: %v", err)
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return fleetOutcomeFailed, fmt.Sprintf("cannot chmod temp hook file: %v", err)
	}
	if err := os.Rename(tmpName, hookPath); err != nil {
		return fleetOutcomeFailed, fmt.Sprintf("cannot move the new hook into place: %v", err)
	}
	switch {
	case len(existing) == 0:
		return fleetOutcomeInstalled, ""
	case strings.Contains(string(existing), boardLintMarkerBegin):
		return fleetOutcomeInstalled, "managed block updated"
	default:
		return fleetOutcomeInstalled, "existing hook preserved and chained"
	}
}
