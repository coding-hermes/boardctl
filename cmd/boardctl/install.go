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
//
// Lint mode (BT-069): --lint-mode enforce (the default) keeps the exact
// BT-054 behavior — a validate failure BLOCKS the commit. --lint-mode warn
// records boardctl_lint_mode="warn" in the hook body; the reject arm prints
// the same validate evidence behind a WARN-ONLY banner and ALLOWS the
// commit — the survivable mode for boards carrying known, not-yet-fixed
// lint debt. The mode is carried through every shape the installer writes
// (standalone and chained) and the marker-based in-place replacement flips
// it in place (enforce->warn, warn->enforce) without duplicating the block.
// In --fleet mode the mode is carried into every hook the rollout writes,
// so a warn re-roll lands fleet-wide with one invocation — the remediating
// step for boards whose commits the freshly armed enforce hook rejects.
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
	// BT-069 lint modes: enforce (the default) rejects the commit on a
	// validate failure, warn prints the same evidence behind a WARN-ONLY
	// banner and allows it. boardLintModeVar names the shell variable that
	// records the mode inside the hook body; boardLintWarnBanner is the warn
	// arm's stderr prefix (the enforce arm keeps its historical wording).
	boardLintModeEnforce = "enforce"
	boardLintModeWarn    = "warn"
	boardLintModeVar     = "boardctl_lint_mode"
	boardLintWarnBanner  = "board-lint: WARN-ONLY — commit allowed (boardctl validate failed)"
)

// cmdInstall implements `boardctl install [-C repo] [--hook-path P]
// [--timeout S] [--dry-run] [--fleet PAT] [--fleet-root DIR]
// [--lint-mode enforce|warn]`.
//
// BT-053: --fleet switches into the rollout mode (fleet.go): enumerate the
// git repos under --fleet-root, classify each, install the same hook this
// command writes for a single repo, print one outcome line per repo. The
// single-repo contract (chaining, no clobber, idempotent update) is the
// SAME code path — generateHookContent — so a fleet install can never
// drift from the installer the tests pin. In fleet mode -C and --hook-path
// are meaningless: they are refused as a usage error rather than silently
// ignored.
func cmdInstall(dir string, args []string) error {
	fs := newFlagSet("install")
	// BT-053: every VALUE flag of this command must be in the reorder set,
	// else `--fleet PAT --fleet-root DIR` misparses (the first flag swallows
	// the next flag name as its value when both precede their positional
	// usage). --hook-path/--timeout had the same latent flaw before the
	// fleet flags joined them; --lint-mode (BT-069) joins the same set.
	args = reorderArgs(args, valueFlags("C", "hook-path", "timeout", "fleet", "fleet-root", "lint-mode"))
	var cdir string
	addCFlag(fs, &cdir)
	hookPath := fs.String("hook-path", "", "path of the hook file to write (default <repo-root>/.git/hooks/pre-commit)")
	timeout := fs.Int("timeout", boardLintDefaultTTL, "seconds granted to boardctl validate before the hook skips it")
	dryRun := fs.Bool("dry-run", false, "print the hook file content that would be written and write nothing")
	fleet := fs.String("fleet", "", "roll out across the fleet: comma-separated glob-or-path list selecting repos (e.g. '/home/me/*' or 'axiom,crier'); prints one outcome line per repo")
	fleetRoot := fs.String("fleet-root", "", "directory the --fleet walk enumerates under (default the home directory)")
	// BT-069: enforce (default) is the exact BT-054 behavior; warn lets a
	// validate-failing board commit with the evidence on stderr.
	lintMode := fs.String("lint-mode", boardLintModeEnforce, "reject arm behavior on a boardctl validate failure: enforce (default) blocks the commit, warn prints the evidence behind a WARN-ONLY banner and allows it")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "boardctl install [-C repo] [--hook-path P] [--timeout S] [--dry-run] [--fleet PAT] [--fleet-root DIR] [--lint-mode enforce|warn]\n")
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *timeout < 1 {
		return errUsage
	}
	switch *lintMode {
	case boardLintModeEnforce, boardLintModeWarn:
	default:
		return fmt.Errorf("boardctl install: unknown --lint-mode mode %q (recognized: enforce, warn): %w", *lintMode, errUsage)
	}
	if *fleet != "" {
		if dir != "" || cdir != "" || *hookPath != "" {
			return errUsage
		}
		binPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("boardctl install: cannot resolve the boardctl binary path: %w", err)
		}
		binPath, err = filepath.Abs(binPath)
		if err != nil {
			return fmt.Errorf("boardctl install: cannot resolve the boardctl binary path: %w", err)
		}
		return fleetInstall(fleetInstallRequest{
			pattern:  *fleet,
			root:     *fleetRoot,
			binPath:  binPath,
			timeout:  *timeout,
			dryRun:   *dryRun,
			lintMode: *lintMode,
		}, os.Stdout, os.Stderr)
	}
	if dir == "" {
		dir = cdir
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

	content := generateHookContent(string(existingContent), binPath, root, *timeout, *lintMode)

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
// reject arm exits (enforce) or falls through behind the WARN-ONLY banner
// (warn) — every skip arm falls through.
func boardLintBlock(binPath, root string, timeout int, lintMode string) string {
	var b strings.Builder
	b.WriteString(boardLintMarkerBegin + "\n")
	fmt.Fprintf(&b, "boardctl_bin=%q\n", binPath)
	fmt.Fprintf(&b, "board_root=%q\n", root)
	fmt.Fprintf(&b, "boardctl_timeout=%d\n", timeout)
	fmt.Fprintf(&b, "%s=%q\n", boardLintModeVar, lintMode)
	b.WriteString(boardLintHookBody)
	b.WriteString(boardLintMarkerEnd + "\n")
	return b.String()
}

// boardLintChainBlock renders the marked shell block a CHAINED hook carries:
// the same lint logic, defined as a function, plus the status init. The call
// happens in the epilogue (boardLintChainWrap) AFTER the existing hook body,
// so board-lint runs after it. Derived from boardLintHookBody with the reject
// arm's `exit 1` -> `return 1` (that arm is the only exit in the body) so the
// lint logic lives in exactly one place. In warn mode the reject arm holds no
// exit at all — the function falls through to 0 on its own.
func boardLintChainBlock(binPath, root string, timeout int, lintMode string) string {
	const rejectExit = "				exit 1\n"
	if !strings.Contains(boardLintHookBody, rejectExit) {
		// Structural invariant of the shared lint body: the enforce-mode
		// reject arm is the single `exit 1`. Fail the install loudly rather
		// than generate a chained block whose reject arm still exits the
		// whole hook.
		panic("boardctl install: boardLintHookBody lost its reject-arm `exit 1`")
	}
	var b strings.Builder
	b.WriteString(boardLintMarkerBegin + "\n")
	fmt.Fprintf(&b, "boardctl_bin=%q\n", binPath)
	fmt.Fprintf(&b, "board_root=%q\n", root)
	fmt.Fprintf(&b, "boardctl_timeout=%d\n", timeout)
	fmt.Fprintf(&b, "%s=%q\n", boardLintModeVar, lintMode)
	fmt.Fprintf(&b, "%s=0\n", boardLintChainStatus)
	fmt.Fprintf(&b, "%s() {\n", boardLintChainMain)
	if lintMode == boardLintModeWarn {
		// Warn mode: the shared body's enforce-only `exit 1` is REMOVED in
		// the chained shape (the warn arm falls through to 0 on its own),
		// and the remaining `exit 0` (validate OK) becomes `return 0` so the
		// success path cannot exit the whole hook either. In enforce mode
		// the `exit 0` at top level is the hook's success exit and must
		// stay — only the warn derivation rewrites it.
		warnBody := strings.Replace(boardLintHookBody, rejectExit, "", 1)
		b.WriteString(strings.Replace(warnBody, "				:\n				;;\n			124)", "				return 0\n				;;\n			124)", 1))
	} else {
		b.WriteString(strings.Replace(boardLintHookBody, rejectExit, "				return 1\n", 1))
	}
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
// enforce-mode reject arm's `exit 1` blocks the commit; every skip arm falls
// through). In the chained shape the same text is wrapped in
// boardctl_lint_main() with the reject arm rewritten to `return 1` (see
// boardLintChainBlock) — the call happens in the epilogue after the existing
// hook body.
//
// Lint mode (BT-069): the body branches on $boardctl_lint_mode, written by
// both block shapes. enforce keeps the historical reject arm (`exit 1` /
// chained `return 1`, commit BLOCKED). warn prints the SAME validate
// evidence behind the WARN-ONLY banner and falls through (chained: `return
// 0`) — the commit is ALLOWED; an operator flips back with a plain
// `boardctl install` (enforce) re-install.
//
// Exit contract: validate OK -> 0; validate failure (including a dangling
// depends_on promoted by --fail-on dangling-dep) -> 1 with its output on
// stderr when enforce (commit BLOCKED), 0 with the same evidence behind the
// WARN-ONLY banner when warn; no board, missing binary, missing timeout(1),
// or timeout 124 -> 0 with a one-line stderr note (SKIP).
const boardLintHookBody = `# board-lint: guard commits that break the JSONL board
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
				if [ "$boardctl_lint_mode" = "warn" ]; then
					echo "board-lint: WARN-ONLY — commit allowed (boardctl validate failed)" >&2
					echo "$lint_out" >&2
				else
					echo "board-lint: commit rejected (boardctl validate failed)" >&2
					echo "$lint_out" >&2
					exit 1
				fi
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
//
// lintMode (BT-069) is carried into every generated block, so a re-install
// with the other mode flips the installed hook in place without changing its
// shape: enforce (the default) keeps the BT-054 reject arm, warn swaps it for
// the WARN-ONLY allow arm.
func generateHookContent(existing, binPath, root string, timeout int, lintMode string) string {
	trimmed := strings.TrimRight(existing, "\n")
	if strings.Contains(existing, boardLintMarkerBegin) {
		begin := strings.Index(existing, boardLintMarkerBegin)
		end := strings.Index(existing[begin:], boardLintMarkerEnd)
		block := boardLintBlock(binPath, root, timeout, lintMode)
		if strings.Contains(existing, boardLintChainExisting) {
			block = boardLintChainBlock(binPath, root, timeout, lintMode)
		}
		if end >= 0 {
			end += begin + len(boardLintMarkerEnd)
			// BT-053 fix (byte-idempotency): the block ends with its own
			// newline and every composition places the remainder directly
			// after it (a chained hook's wrapped body follows with no
			// separator). The old code re-appended the remainder's leading
			// newline run — growing every re-installed hook by one byte
			// per run and inserting a blank line after the block. Strip
			// the whole leading-newline run and append the rest directly:
			// a re-install then reads byte-identical to the first install
			// for both shapes, and a pure-newline tail (the artifact
			// itself) is dropped.
			trimmed := strings.TrimLeft(existing[end:], "\n")
			if trimmed == "" {
				return existing[:begin] + block
			}
			return existing[:begin] + block + trimmed
		}
		// Unterminated block: keep everything before the marker and rebuild.
		return existing[:begin] + block
	}
	if trimmed == "" {
		return "#!/bin/sh\n" + boardLintBlock(binPath, root, timeout, lintMode)
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
	return head + boardLintChainBlock(binPath, root, timeout, lintMode) + boardLintChainWrap(body)
}
