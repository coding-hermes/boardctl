// Package freshness is BT-057's deployment freshness gate: at startup,
// boardctl compares the build commit stamped into the binary
// (main.buildCommit, set by the Makefile build and release targets) against
// the HEAD of the git checkout it is running inside, and prints ONE warning
// line to stderr when the binary was built from a commit that HEAD has
// moved past.
//
// The failure mode it names: a deployed ~/.local/bin/boardctl that is days
// stale silently hides fixes that exist only at HEAD (BT-057 recorded one
// that masked 96 validate findings). Nothing else tells the operator.
//
// Silence contract — Check NEVER fails, NEVER writes to stdout, NEVER
// changes the process exit code, and prints NOTHING unless every one of
// these holds:
//
//   - the embedded build commit is non-empty (plain `go build` / `go
//     install` builds carry no stamp and stay silent),
//   - BOARDCTL_SKIP_FRESHNESS is not set,
//   - a .git directory (or worktree gitfile) is found walking up from cwd,
//   - git is available and answers,
//   - the stamp resolves in HEAD's history (git merge-base --is-ancestor),
//   - the stamp is not HEAD itself.
//
// A release binary run outside any checkout, a commit rebased away, or a
// missing git binary therefore degrade to silence — the gate can never turn
// into a startup failure. This repo pins byte-exact CLI output in tests
// (cmd/boardctl/bt041_help_exit_test.go and friends), so stdout stays
// byte-identical on every path.
package freshness

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// SkipEnv is the operator's silence switch: set it to any non-empty value
// other than "0"/"false" to suppress the warning (documented in README's
// Development section and in the warning line itself).
const SkipEnv = "BOARDCTL_SKIP_FRESHNESS"

// shortLen is how much of a commit sha the warning names — long enough to
// be unambiguous in a fleet repo, short enough to read.
const shortLen = 12

// Check runs the freshness gate. binaryCommit is the sha stamped into the
// binary at link time (main.buildCommit; empty for unstamped builds), cwd is
// where the binary was launched, stderr receives the single warning line
// when the binary is stale. It never returns an error and never panics:
// every probe failure degrades to silence by contract.
func Check(binaryCommit string, cwd string, stderr io.Writer) {
	// No embedded stamp — the plain go build / go install case. Silent.
	if strings.TrimSpace(binaryCommit) == "" {
		return
	}
	// Operator silence switch, honored before any probe runs. "0" and
	// "false" are treated as NOT set so a shell wrapper can pass the var
	// through unconditionally without disabling the gate.
	if v := os.Getenv(SkipEnv); v != "" && v != "0" && !strings.EqualFold(v, "false") {
		return
	}
	// Find the enclosing checkout the same way -C walks its targets.
	root, err := resolveGitDir(cwd)
	if err != nil {
		return
	}
	git := gitBinary()
	if git == "" {
		return
	}
	// Is the stamped commit part of this checkout's HEAD history at all?
	// A foreign sha (release binary in an unrelated repo) or a commit
	// rebase-wiped from the checkout both answer no — silent skip.
	if !isAncestor(git, cwd, binaryCommit) {
		return
	}
	// HEAD itself is fresh by definition.
	if headOf(git, cwd) == strings.TrimSpace(binaryCommit) {
		return
	}
	// Everything below is best-effort evidence for the one warning line;
	// any failure here degrades to silence.
	head := headOf(git, cwd)
	if head == "" {
		return
	}
	n := missingCommits(git, cwd, binaryCommit)
	if n <= 0 {
		return
	}
	date := commitDate(git, cwd, binaryCommit)
	unit := "commits"
	if n == 1 {
		unit = "commit"
	}
	fmt.Fprintf(stderr,
		"boardctl: warning: this binary was built from commit %s (built %s), which is %d %s behind HEAD (%s) of the checkout at %s — rebuild (make build) or reinstall; silence with %s=1\n",
		short(binaryCommit), date, n, unit, short(head), root, SkipEnv)
}

// gitBinary returns the absolute path of the git binary on PATH, or "" when
// git is unavailable (LookPath error) — the gate's "git missing" silence
// path. A var so tests can verify the contract without mocking exec.
var gitBinary = func() string {
	p, err := exec.LookPath("git")
	if err != nil {
		return ""
	}
	return p
}

// resolveGitDir walks up from cwd (falling back to the process working
// directory when empty) to the nearest .git entry, returning the checkout
// root that contains it. Accepts both a .git DIRECTORY (normal checkout)
// and a .git FILE (linked worktree gitfile) — walking up until filesystem
// root or an unreadable parent stops the loop.
func resolveGitDir(cwd string) (root string, err error) {
	start := cwd
	if start == "" {
		if start, err = os.Getwd(); err != nil {
			return "", err
		}
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if isGitDir(filepath.Join(dir, ".git")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf(".git not found above %s", start)
		}
		dir = parent
	}
}

// isGitDir reports whether p is a present .git entry, as either a directory
// (standard checkout) or a file (a linked worktree's gitfile pointer).
func isGitDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && (st.IsDir() || st.Mode().IsRegular())
}

// isAncestor asks git whether binaryCommit is reachable from HEAD:
// `git merge-base --is-ancestor <sha> HEAD` exits 0 exactly then. Any
// other outcome (unknown sha, exit 1, exec failure) reads as no.
func isAncestor(git, cwd, sha string) bool {
	return runGit(git, cwd, "merge-base", "--is-ancestor", sha, "HEAD") == nil
}

// headOf resolves the current HEAD sha, "" on any failure.
func headOf(git, cwd string) string {
	return strings.TrimSpace(runGitOut(git, cwd, "rev-parse", "HEAD"))
}

// missingCommits counts the commits HEAD carries that the binary's commit
// does not: `git rev-list --count <sha>..HEAD`. 0 on any failure (which
// keeps the gate silent rather than lying about the count).
func missingCommits(git, cwd, sha string) int {
	out := runGitOut(git, cwd, "rev-list", "--count", sha+"..HEAD")
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// commitDate renders the binary commit's author date in UTC as YYYY-MM-DD,
// "" on any failure.
func commitDate(git, cwd, sha string) string {
	out := runGitOut(git, cwd, "show", "-s", "--format=%ad", "--date=format-local:%Y-%m-%d", sha)
	return strings.TrimSpace(out)
}

// short renders a sha at warning length (12 hex chars), tolerating short
// input.
func short(sha string) string {
	if len(sha) > shortLen {
		return sha[:shortLen]
	}
	return sha
}

// runGit runs git with args in dir and reports whether it exited 0.
func runGit(git, dir string, args ...string) error {
	cmd := exec.Command(git, args...)
	cmd.Dir = dir
	return cmd.Run()
}

// runGitOut runs git with args in dir, returning combined stdout+stderr
// trimmed of surrounding whitespace; "" on exec failure.
func runGitOut(git, dir string, args ...string) string {
	cmd := exec.Command(git, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
