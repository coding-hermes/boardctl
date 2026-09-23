// BT-054: `boardctl install` — install a git pre-commit hook that lints the
// coding-hermes JSONL board at commit time. The hook shells out to the
// boardctl binary itself (`boardctl validate -C <repo-root>
// --fail-on dangling-dep`) instead of linking internal packages: the hook is
// a plain shell script and stays decoupled from boardctl's Go internals.
// The --fail-on flag (BT-054-R) is what makes a DANGLING depends_on
// reference a commit blocker: plain `boardctl validate` reports it as a
// warning and exits 0 (legacy boards carry dangling refs that must not fail
// interactive reads), but the hook path needs the class rejected.
//
// Chaining contract (BT-054-R remediation): board-lint runs AFTER the
// existing hook body, never before it. Naively appending the block would be
// dead code on hooks that exit early — the repo's own GitReins guard exits 0
// when .gitreins/config.yaml is absent — so install REWRITES the existing
// hook: the original body is wrapped in a subshell `( ... )` (its bare
// `exit 0` / `exit N` statements now only leave the subshell) and the
// board-lint logic runs in an epilogue AFTER that subshell, combining both
// statuses:
//
//   - the existing body's nonzero status still propagates (its reject path
//     still blocks the commit),
//   - board-lint still runs even when the body exits early with 0,
//   - a board-lint rejection wins when both fail.
//
// A fresh hook gets the standalone block (lint exits directly). A hook that
// already carries the marker gets the block REPLACED in place — re-running
// install is an idempotent update, never a duplicate. The chained shape is
// detected by its epilogue variable, so re-install regenerates the correct
// block shape for the hook it is updating.
//
// Timeout: the validate call is wrapped in `timeout <S>` (GNU coreutils).
// On exit 124 the hook SKIPS (prints one stderr note, exits 0) so a slow or
// wedged boardctl can never wedge every commit in the repo. The same skip
// applies when the recorded binary is missing and no `boardctl` is on PATH.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	boardLintMarkerBegin = "# >>> boardctl board-lint managed block (boardctl install; replaced on re-install) >>>"
	boardLintMarkerEnd   = "# <<< boardctl board-lint managed block <<<"
	boardLintDefaultTTL  = 30
	// boardLintChainStatus / boardLintChainMain / boardLintChainExisting
	// name the chained hook's state. boardLintChainStatus doubles as the
	// shape detector on re-install: a hook carrying it was generated in the
	// chained shape, so the replacement block must be the chained shape too.
	boardLintChainStatus   = "boardctl_lint_status"
	boardLintChainMain     = "boardctl_lint_main"
	boardLintChainExisting = "boardctl_lint_existing_status"
)

// cmdInstall implements `boardctl install [-C repo] [--hook-path P]
// [--timeout S] [--dry-run]`.
func cmdInstall(dir string, args []string) error {
	fs := newFlagSet("install")
	args = reorderArgs(args, valueFlags("C"))
	var cdir string
	addCFlag(fs, &cdir)
	hookPath := fs.String("hook-path", "", "path of the hook file to write (default <repo-root>/.git/hooks/pre-commit)")
	timeout := fs.Int("timeout", boardLintDefaultTTL, "seconds granted to boardctl validate before the hook skips it")
	dryRun := fs.Bool("dry-run", false, "print the hook file content that would be written and write nothing")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "boardctl install [-C repo] [--hook-path P] [--timeout S] [--dry-run]\n")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if dir == "" {
		dir = cdir
	}
	if *timeout < 1 {
		return errUsage
	}

	root, err := repoRootFor(dir)
	if err != nil {
		return err
	}
	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("boardctl install: cannot resolve the boardctl binary path: %w", err)
	}
	binPath, err = filepath.Abs(binPath)
	if err != nil {
		return fmt.Errorf("boardctl install: cannot resolve the boardctl binary path: %w", err)
	}

	target := *hookPath
	if target == "" {
		target = filepath.Join(root, ".git", "hooks", "pre-commit")
	}

	// NOTE: os.ReadErr on a not-yet-existing hook file is the fresh-install
	// case, not a failure.
	existing, readErr := os.ReadFile(target)
	var existingContent []byte
	if readErr == nil {
		existingContent = existing
	} else if !os.IsNotExist(readErr) {
		return fmt.Errorf("boardctl install: cannot read existing hook %s: %w", target, readErr)
	}

	content := generateHookContent(string(existingContent), binPath, root, *timeout)

	if *dryRun {
		fmt.Print(content)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("boardctl install: cannot create hooks directory: %w", err)
	}
	if err := os.WriteFile(target, []byte(content), 0o755); err != nil {
		return fmt.Errorf("boardctl install: cannot write hook %s: %w", target, err)
	}
	if err := os.Chmod(target, 0o755); err != nil {
		return fmt.Errorf("boardctl install: cannot chmod hook %s: %w", target, err)
	}
	fmt.Fprintf(os.Stdout, "board-lint pre-commit hook installed at %s (timeout %ds)\n", target, *timeout)
	return nil
}

// repoRootFor walks up from target looking for a .git directory (or a bare
// .git FILE, as in worktrees/submodules) and returns the repository root.
func repoRootFor(target string) (string, error) {
	if target == "" {
		target = "."
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("boardctl install: cannot resolve %s: %w", target, err)
	}
	cur := abs
	for {
		if st, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			if st.IsDir() || st.Mode().IsRegular() {
				return cur, nil
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("boardctl install: no git repository found above %s (no .git directory)", abs)
		}
		cur = parent
	}
}

// boardLintBlock renders the marked shell block a STANDALONE hook carries
// (fresh install): the lint logic runs at the marker position and only the
// reject arm exits — every skip arm falls through.
func boardLintBlock(binPath, root string, timeout int) string {
	var b strings.Builder
	b.WriteString(boardLintMarkerBegin + "\n")
	fmt.Fprintf(&b, "boardctl_bin=%q\n", binPath)
	fmt.Fprintf(&b, "board_root=%q\n", root)
	fmt.Fprintf(&b, "boardctl_timeout=%d\n", timeout)
	b.WriteString(boardLintHookBody)
	b.WriteString(boardLintMarkerEnd + "\n")
	return b.String()
}

// boardLintChainBlock renders the marked shell block a CHAINED hook carries:
// the same lint logic, defined as a function, plus the status init. The call
// happens in the epilogue (boardLintChainWrap) AFTER the existing hook body,
// so board-lint runs after it. Derived from boardLintHookBody with the reject
// arm's `exit 1` -> `return 1` (that arm is the only exit in the body) so the
// lint logic lives in exactly one place.
func boardLintChainBlock(binPath, root string, timeout int) string {
	const rejectExit = "\t\t\t\texit 1\n"
	if !strings.Contains(boardLintHookBody, rejectExit) {
		// Structural invariant of the shared lint body: the reject arm is
		// the single `exit 1`. Fail the install loudly rather than generate
		// a chained block whose reject arm still exits the whole hook.
		panic("boardctl install: boardLintHookBody lost its reject-arm `exit 1`")
	}
	var b strings.Builder
	b.WriteString(boardLintMarkerBegin + "\n")
	fmt.Fprintf(&b, "boardctl_bin=%q\n", binPath)
	fmt.Fprintf(&b, "board_root=%q\n", root)
	fmt.Fprintf(&b, "boardctl_timeout=%d\n", timeout)
	fmt.Fprintf(&b, "%s=0\n", boardLintChainStatus)
	fmt.Fprintf(&b, "%s() {\n", boardLintChainMain)
	b.WriteString(strings.Replace(boardLintHookBody, rejectExit, "\t\t\t\treturn 1\n", 1))
	b.WriteString("}\n")
	b.WriteString(boardLintMarkerEnd + "\n")
	return b.String()
}

// boardLintChainWrap wraps an existing hook body in a subshell (neutralizing
// its bare `exit` statements) and appends the epilogue that runs board-lint
// AFTER it and combines the two statuses. Contract: the existing body's
// nonzero still propagates; board-lint runs even when the body exits early
// with 0; a board-lint rejection wins when both fail.
func boardLintChainWrap(body string) string {
	if strings.TrimSpace(body) == "" {
		body = ":" // no-op: keep the subshell a valid command list
	}
	var b strings.Builder
	b.WriteString("# The original hook body runs inside a subshell: its bare `exit` statements\n")
	b.WriteString("# only leave the subshell, so board-lint below still runs. Its nonzero\n")
	b.WriteString("# status still propagates (the epilogue exits with it when board-lint\n")
	b.WriteString("# passes). Rewritten by boardctl install (BT-054-R chaining).\n")
	b.WriteString("(\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(")\n")
	fmt.Fprintf(&b, "%s=$?\n", boardLintChainExisting)
	b.WriteString("# board-lint epilogue (managed by boardctl install): board-lint runs\n")
	b.WriteString("# AFTER the existing hook body; both statuses are honored.\n")
	fmt.Fprintf(&b, "%s || %s=1\n", boardLintChainMain, boardLintChainStatus)
	fmt.Fprintf(&b, "if [ \"$%s\" -ne 0 ]; then\n\texit \"$%s\"\nfi\n", boardLintChainStatus, boardLintChainStatus)
	fmt.Fprintf(&b, "exit \"$%s\"\n", boardLintChainExisting)
	return b.String()
}

// boardLintHookBody is the executable lint logic shared by the standalone and
// the chained block shape. In the standalone shape it runs at top level (the
// reject arm's `exit 1` blocks the commit; every skip arm falls through). In
// the chained shape the same text is wrapped in boardctl_lint_main() with the
// reject arm rewritten to `return 1` (see boardLintChainBlock) — the call
// happens in the epilogue after the existing hook body.
//
// Exit contract: validate OK -> 0; validate failure (including a dangling
// depends_on promoted by --fail-on dangling-dep) -> 1 with its output on
// stderr (commit BLOCKED); no board, missing binary, missing timeout(1), or
// timeout 124 -> 0 with a one-line stderr note (SKIP).
const boardLintHookBody = `# board-lint: reject commits that break the JSONL board
# (duplicate ids; dangling depends_on via --fail-on dangling-dep).
# Installed by boardctl install.
# A slow/missing/wedged boardctl SKIPS (no block) — it must never wedge commits.
if [ -d "$board_root/.coding-hermes/board" ]; then
	if [ -x "$boardctl_bin" ]; then
		lint_bin="$boardctl_bin"
	elif command -v boardctl >/dev/null 2>&1; then
		lint_bin="$(command -v boardctl)"
	fi
	if [ -n "${lint_bin:-}" ] && command -v timeout >/dev/null 2>&1; then
		lint_out="$(timeout "$boardctl_timeout" "$lint_bin" validate -C "$board_root" --fail-on dangling-dep 2>&1)"
		lint_status=$?
		case "$lint_status" in
			0)
				:
				;;
			124)
				echo "board-lint: skipped (timeout)" >&2
				;;
			2)
				echo "board-lint: skipped (no board found under $board_root)" >&2
				;;
			*)
				echo "board-lint: commit rejected (boardctl validate failed)" >&2
				echo "$lint_out" >&2
				exit 1
				;;
		esac
	elif [ -z "${lint_bin:-}" ]; then
		echo "board-lint: skipped (boardctl binary not found)" >&2
	else
		echo "board-lint: skipped (timeout(1) unavailable)" >&2
	fi
fi
`

// generateHookContent folds the board-lint block into a hook file's content:
//   - a fresh hook gets a shebang + the standalone block;
//   - an existing hook WITHOUT the marker is REWRITTEN into the chained
//     shape: shebang, the chained block (lint logic defined as a function),
//     then the original body wrapped in a subshell, then the epilogue that
//     runs board-lint after the body and combines the statuses — board-lint
//     runs AFTER the existing logic (BT-054-R) and the body's early exits
//     can no longer skip it;
//   - a hook already carrying the marker gets the block replaced in place —
//     install is idempotent. The replacement block matches the hook's shape
//     (chained hooks get the chained block, standalone hooks the standalone
//     block) so a re-install never downgrades the chaining semantics.
func generateHookContent(existing, binPath, root string, timeout int) string {
	trimmed := strings.TrimRight(existing, "\n")
	if strings.Contains(existing, boardLintMarkerBegin) {
		begin := strings.Index(existing, boardLintMarkerBegin)
		end := strings.Index(existing[begin:], boardLintMarkerEnd)
		block := boardLintBlock(binPath, root, timeout)
		if strings.Contains(existing, boardLintChainExisting) {
			block = boardLintChainBlock(binPath, root, timeout)
		}
		if end >= 0 {
			end += begin + len(boardLintMarkerEnd)
			return existing[:begin] + block + existing[end:]
		}
		// Unterminated block: keep everything before the marker and rebuild.
		return existing[:begin] + block
	}
	if trimmed == "" {
		return "#!/bin/sh\n" + boardLintBlock(binPath, root, timeout)
	}
	// Chained rewrite: split off the shebang (line 1) if present, supply one
	// when the hook lacks it, then emit chain block + wrapped body + epilogue.
	body := trimmed
	head := "#!/bin/sh\n"
	if strings.HasPrefix(body, "#!") {
		if idx := strings.Index(body, "\n"); idx >= 0 {
			head, body = body[:idx+1], body[idx+1:]
		} else {
			head, body = body+"\n", ""
		}
	}
	return head + boardLintChainBlock(binPath, root, timeout) + boardLintChainWrap(body)
}
