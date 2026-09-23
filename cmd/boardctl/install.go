// BT-054: `boardctl install` — install a git pre-commit hook that lints the
// coding-hermes JSONL board at commit time. The hook shells out to the
// boardctl binary itself (`boardctl validate -C <repo-root>`) instead of
// linking internal packages: the hook is a plain shell script and stays
// decoupled from boardctl's Go internals.
//
// Chaining contract: an existing pre-commit hook keeps its logic. When the
// hook does NOT already carry the board-lint marker, the marked block is
// inserted immediately AFTER the shebang line — i.e. board-lint runs FIRST,
// before the existing logic. Appending after the existing logic is not an
// option here: hooks like the GitReins guard exit 0 early on repos without
// .gitreins/config.yaml, and a trailing block would then never run. Inserting
// before any early exit is the design that GUARANTEES board-lint runs on
// every commit of a chained repo (existing logic still runs, right after).
// When the marker is already present the block is REPLACED in place —
// re-running install is an idempotent update, never a duplicate.
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

// boardLintBlock renders the marked shell block the hook carries.
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

// boardLintHookBody is the executable shell shared by fresh and chained
// hooks. Exit contract: validate OK -> 0; validate failure -> 1 with its
// output on stderr (commit BLOCKED); no board, missing binary, missing
// timeout(1), or timeout 124 -> 0 with a one-line stderr note (SKIP).
//
// Chaining rule: only the REJECT arm exits. Every non-blocking arm falls
// through (no exit) so a chained hook's remaining logic still runs — the
// block may be inserted ahead of an existing hook's body (see
// generateHookContent), and an `exit 0` on the happy path would short-circuit
// it. A fresh hook simply falls off the end of the file (exit 0).
const boardLintHookBody = `# board-lint: reject commits that break the JSONL board
# (duplicate ids, dangling depends_on). Installed by boardctl install.
# A slow/missing/wedged boardctl SKIPS (no block) — it must never wedge commits.
if [ -d "$board_root/.coding-hermes/board" ]; then
	if [ -x "$boardctl_bin" ]; then
		lint_bin="$boardctl_bin"
	elif command -v boardctl >/dev/null 2>&1; then
		lint_bin="$(command -v boardctl)"
	fi
	if [ -n "${lint_bin:-}" ] && command -v timeout >/dev/null 2>&1; then
		lint_out="$(timeout "$boardctl_timeout" "$lint_bin" validate -C "$board_root" 2>&1)"
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
// a fresh hook gets a shebang + the block; an existing hook keeps its content
// with the block inserted right after the shebang (before any early exits so
// board-lint always runs); a hook already carrying the block gets the block
// replaced in place — install is idempotent.
func generateHookContent(existing, binPath, root string, timeout int) string {
	block := boardLintBlock(binPath, root, timeout)
	trimmed := strings.TrimRight(existing, "\n")
	if strings.Contains(existing, boardLintMarkerBegin) {
		begin := strings.Index(existing, boardLintMarkerBegin)
		end := strings.Index(existing[begin:], boardLintMarkerEnd)
		if end >= 0 {
			end += begin + len(boardLintMarkerEnd)
			return existing[:begin] + block + existing[end:]
		}
		// Unterminated block: keep everything before the marker and rebuild.
		return existing[:begin] + block
	}
	if trimmed == "" {
		return "#!/bin/sh\n" + block
	}
	// Insert after the shebang (line 1) so the block precedes any early
	// exit in the existing hook — that is the only placement guaranteed to
	// run on a chained hook whose existing logic can exit early.
	if strings.HasPrefix(trimmed, "#!") {
		if idx := strings.Index(trimmed, "\n"); idx >= 0 {
			return trimmed[:idx+1] + block + trimmed[idx+1:] + "\n"
		}
		return trimmed + "\n" + block
	}
	return block + trimmed + "\n"
}
