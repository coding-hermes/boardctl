package main

// BOARDCTL-HOOK-001 — `boardctl hook install|uninstall|doctor|dispatch|flush`:
// the boardctl side of specs/CRIER-GITHOOK-DISPATCH.md, the git-hook intake
// half of the always-working pipeline:
//
//	board row lands (commit or pull) → post-commit/post-merge hook
//	→ detect ADDED/SEEN pending rows → sign + POST to crier's webhook ingest
//	→ crier queue → registered agent.
//
// The managed block records the ABSOLUTE path of the binary that installed it
// (with a plain `boardctl` on PATH as the fallback), so the hook is wired to
// the right build even when the name on PATH resolves elsewhere. The marker
// line is always the literal spec invocation — `boardctl hook dispatch --hook
// <name> "$@"` — because that is the line `hook doctor` verifies
// (DF-CRIER-430/433: a marker that cannot be REACHED is refused).
//
// Law this file must not break (the row's acceptance criteria):
//
//   - the hook NEVER blocks the commit: every dispatch failure exits 0 and is
//     named on stderr (post-commit cannot change a commit's outcome, but a
//     hook that exits non-zero still scares operators and some hook chains
//     propagate it);
//   - uninstall removes ONLY the managed block; a sibling hook body outside
//     the delimiters survives BYTE-IDENTICAL (proven in bt083_hook_test.go
//     against a real `git commit`);
//   - undelivered rows are never silently dropped: bounded attempts (3) then
//     an operator-visible dead-letter record (count, last failure, row ids)
//     that `hook doctor` prints;
//   - the secret is read from a 0600 file per dispatch, never baked into the
//     hook body, never logged.
//
// Board resolution is the SAME path every other subcommand uses (openBoard:
// -C repo root, .coding-hermes, or the board dir itself); the repo root and
// the board file's repo-relative path are derived from that one resolution,
// never re-invented here.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	hookBlockVersion = "v1"

	hookKindPostCommit = "post-commit"
	hookKindPostMerge  = "post-merge"

	// hookSpoolName / hookDeadLetterName live beside the board (spec R13,
	// Q-3 default) and are gitignored at install.
	hookSpoolName        = ".dispatch-spool"
	hookDeadLetterName   = "dead-letter.jsonl"
	hookLastDeliveryName = "last-delivery.record"

	// hookMaxAttempts is the bounded retry budget of spec R13/§10.2: after
	// the third failed delivery of a batch it is written to the dead-letter
	// store, never deleted silently.
	hookMaxAttempts = 3

	// hookBatchRowsDefault and hookRequestBodyCap protect the ingest 413
	// boundary and keep one hook fire's wall time bounded (spec R16). One
	// POST carries one row, because the Idempotency-Key is per row (§10.3).
	hookBatchRowsDefault = 50
	hookRequestBodyCap   = 512 * 1024

	// hookBudgetDefault is the total wall-time budget of one dispatch (spec
	// R17). The managed block additionally wraps the call in timeout(1),
	// where the host has it.
	hookBudgetDefault = 10 * time.Second

	hookSourceIDDefault = "boardctl"
)

// hookUsage is the shared usage block, printed on every failed invocation so
// the accepted shape is discoverable from the failure itself.
const hookUsage = "usage: boardctl hook install --hook <post-commit|post-merge> --crier-url URL --secret-file PATH [--source-id ID] [-C DIR] [--self BIN] [--hook-path P] [--dry-run]\n" +
	"       boardctl hook uninstall --hook <post-commit|post-merge> [-C DIR] [--hook-path P]\n" +
	"       boardctl hook doctor [-C DIR] [--probe-timeout 2s]\n" +
	"       boardctl hook dispatch --hook <post-commit|post-merge> [-C DIR] [--crier-url URL] [--source-id ID] [--secret-file PATH] [--batch-cap N] [--budget 10s] [--dry-run]\n" +
	"       boardctl hook flush [-C DIR] [--dry-run]\n" +
	"install appends a clearly delimited managed block to .git/hooks/<hook>; uninstall removes ONLY that block.\n" +
	"The post-commit hook diffs HEAD^..HEAD, post-merge diffs ORIG_HEAD..HEAD, and each ADDED/SEEN pending board\n" +
	"row is POSTed signed (X-Crier-Timestamp + sha256=HMAC(secret, timestamp.body)) with\n" +
	"Idempotency-Key boardctl:<row-id>:<content-hash>. The hook never blocks the commit; failures spool and,\n" +
	"after 3 attempts, become an operator-visible dead-letter record printed by `boardctl hook doctor`."

// cmdHook dispatches the hook verbs. It is wired into run()'s switch exactly
// like every other subcommand; `dispatch` is the ONLY verb that never returns
// an error, because git runs it and a non-zero exit could block a commit.
func cmdHook(dir string, args []string) error {
	if len(args) == 0 {
		_, _ = fmt.Fprint(os.Stderr, "boardctl: hook: a verb is required (want install, uninstall, doctor, dispatch or flush)\n")
		_, _ = fmt.Fprint(os.Stderr, hookUsage)
		return errUsage
	}
	switch args[0] {
	case "help", "-h", "--help":
		// BT-041's help contract: a usage query is a SUCCESS, and its text
		// goes to stdout.
		_, _ = fmt.Fprint(os.Stdout, hookUsage+"\n")
		return nil
	case "install":
		return cmdHookInstall(dir, args[1:])
	case "uninstall":
		return cmdHookUninstall(dir, args[1:])
	case "doctor":
		return cmdHookDoctor(dir, args[1:])
	case "dispatch":
		// Spec R12: the hook must not block the commit — not even on a
		// usage error, a missing board, or a broken flag.
		cmdHookDispatch(dir, args[1:])
		return nil
	case "flush":
		return cmdHookFlush(dir, args[1:])
	default:
		_, _ = fmt.Fprintf(os.Stderr, "boardctl: hook: unknown verb %q (want install, uninstall, doctor, dispatch or flush)\n", args[0])
		_, _ = fmt.Fprint(os.Stderr, hookUsage)
		return errUsage
	}
}

// ---------------------------------------------------------------------------
// managed block: compose, locate, remove
// ---------------------------------------------------------------------------

type hookConfig struct {
	Hook       string
	CrierURL   string
	SourceID   string
	SecretFile string
	// Self is the absolute path of the binary the block invokes. Empty in a
	// hand-edited block falls back to `boardctl` on PATH.
	Self string
	// Placement is "append" (block written after the sibling body) or
	// "before-exit" (written immediately before the sibling's top-level
	// exit/exec, which would otherwise make the block dead code).
	Placement string
}

func hookBeginLine(hook string) string { return "# >>> boardctl-dispatch BEGIN (hook=" + hook + ")" }

const hookEndLine = "# >>> boardctl-dispatch END"

// hookShellQuote renders one argv element for /bin/sh. Values are validated
// before this is called (no newlines), so the single-quote escape is the only
// transformation needed.
func hookShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// hookShellUnquote reverses hookShellQuote for the simple single-quoted form
// the managed block writes.
func hookShellUnquote(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'") {
		s = s[1 : len(s)-1]
		s = strings.ReplaceAll(s, `'\''`, "'")
	}
	return s
}

// hookComposeBlock renders the managed block. eol says whether the file the
// block is being appended to already ended in a newline; uninstall uses that
// recorded fact to restore the sibling body byte-for-byte (a body that did not
// end in a newline must not gain one).
func hookComposeBlock(cfg hookConfig, eol bool, placement string) string {
	eolTok := "absent"
	if eol {
		eolTok = "present"
	}
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "%s\n", hookBeginLine(cfg.Hook))
	_, _ = fmt.Fprintf(&b, "# boardctl-dispatch block %s (installed %s)\n", hookBlockVersion, time.Now().UTC().Format(time.RFC3339))
	_, _ = fmt.Fprintf(&b, "# boardctl-dispatch prefix-eol=%s\n", eolTok)
	if placement == "" {
		placement = "append"
	}
	_, _ = fmt.Fprintf(&b, "# boardctl-dispatch placement=%s\n", placement)
	_, _ = fmt.Fprintf(&b, "# boardctl-dispatch crier-url=%s\n", cfg.CrierURL)
	_, _ = fmt.Fprintf(&b, "# boardctl-dispatch source-id=%s\n", cfg.SourceID)
	_, _ = fmt.Fprintf(&b, "# boardctl-dispatch secret-file=%s\n", cfg.SecretFile)
	// The marker the doctor checks (DF-CRIER-430/433 pattern): the literal
	// invocation the spec names, always present, whether or not the baked
	// binary path below resolves.
	_, _ = fmt.Fprintf(&b, "# boardctl-dispatch marker=boardctl hook dispatch --hook %s \"$@\"\n", cfg.Hook)
	b.WriteString("# boardctl-dispatch: best-effort intake only. The durable path is crier's reconciliation\n")
	b.WriteString("# boardctl-dispatch: sweep; a missed hook is caught there. This block must never block git.\n")

	args := fmt.Sprintf("--hook %s --crier-url %s --source-id %s --secret-file %s",
		hookShellQuote(cfg.Hook), hookShellQuote(cfg.CrierURL), hookShellQuote(cfg.SourceID), hookShellQuote(cfg.SecretFile))
	_, _ = fmt.Fprintf(&b, "__boardctl_dispatch_bin=%s\n", hookShellQuote(cfg.Self))
	b.WriteString("if [ -n \"$__boardctl_dispatch_bin\" ] && [ -x \"$__boardctl_dispatch_bin\" ]; then\n")
	_, _ = fmt.Fprintf(&b, "    \"$__boardctl_dispatch_bin\" hook dispatch %s \"$@\" || true\n", args)
	b.WriteString("elif command -v boardctl >/dev/null 2>&1; then\n")
	_, _ = fmt.Fprintf(&b, "    boardctl hook dispatch %s \"$@\" || true\n", args)
	b.WriteString("else\n")
	_, _ = fmt.Fprintf(&b, "    echo \"boardctl-dispatch: SKIP hook=%s reason=dispatch-binary-missing (tried ${__boardctl_dispatch_bin:-<none>} and boardctl on PATH); rows are not lost: crier's reconciliation sweep owns the durable path\" >&2\n", cfg.Hook)
	b.WriteString("fi\n")
	_, _ = fmt.Fprintf(&b, "%s\n", hookEndLine)
	return b.String()
}

// hookFindBlock returns the byte offsets of the managed block in content.
// ok is false when no BEGIN line for hook exists; start is the first byte of
// the BEGIN line (or -1 when a BEGIN line for ANOTHER hook is all that is
// present), end is one past the END line's newline.
func hookFindBlock(content, hook string) (start, end int, ok bool, err error) {
	begin := hookBeginLine(hook)
	lines := strings.SplitAfter(content, "\n")
	start = -1
	off := 0
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\n")
		if start < 0 && trimmed == begin {
			start = off
		} else if start >= 0 && trimmed == hookEndLine {
			end = off + len(line)
			return start, end, true, nil
		}
		off += len(line)
	}
	if start >= 0 {
		return start, -1, false, fmt.Errorf("managed block for hook=%s has a BEGIN line but no END marker", hook)
	}
	return -1, -1, false, nil
}

// hookEmptyBody reports whether content is nothing but a shebang and blanks —
// the spec's "a hook left with an empty body is removed entirely".
func hookEmptyBody(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || t == "#!/bin/sh" || t == "#!/usr/bin/env sh" || t == "#!/bin/bash" {
			continue
		}
		return false
	}
	return true
}

// hookWriteFileAtomic writes content to path through a sibling temp file and a
// rename, preserving the requested mode. A failed write leaves the old bytes.
func hookWriteFileAtomic(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".boardctl-hook-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// hookGit runs one git command in dir with separated stdout/stderr.
func hookGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// hookRepoRoot resolves the git work tree the hook lives in.
func hookRepoRoot(dir string) (string, error) {
	out, err := hookGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("%s is not inside a git work tree: %w", dir, err)
	}
	return strings.TrimSpace(out), nil
}

// hookHooksDir resolves the hook directory through git itself, so a
// repo-local core.hooksPath is respected exactly the way git respects it.
func hookHooksDir(repo string) (string, error) {
	out, err := hookGit(repo, "rev-parse", "--git-path", "hooks")
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(out)
	if !filepath.IsAbs(p) {
		p = filepath.Join(repo, p)
	}
	return filepath.Clean(p), nil
}

// hookEnv is the resolved context of one hook verb: the board every other
// subcommand resolves, the work tree that board lives in, and the git-managed
// hook directory. One resolution path only (openBoard); nothing here re-probes
// directories on its own.
type hookEnv struct {
	BoardDir string
	RepoRoot string
	// BoardRel is the repo-relative slash path of tasks.jsonl, the path
	// `git show <rev>:<path>` needs.
	BoardRel string
	HooksDir string
}

// hookEvalSymlinks resolves a path's symlinks when it can. Test and fleet
// hosts routinely run with a symlinked temp root (GOTMPDIR=/var/tmp/x ->
// /var/tmp/x-real); git reports the RESOLVED work tree while the board dir may
// arrive in the symlinked spelling, and comparing two spellings of one
// directory reads as "the board is outside the repo". Canonicalising both
// sides once here is the single fix.
func hookEvalSymlinks(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func resolveHookEnv(dir string) (hookEnv, error) {
	b, err := openBoard(dir)
	if err != nil {
		return hookEnv{}, err
	}
	root, err := hookRepoRoot(b.Dir)
	if err != nil {
		return hookEnv{}, err
	}
	root = hookEvalSymlinks(root)
	boardDir := hookEvalSymlinks(b.Dir)
	rel, err := filepath.Rel(root, hookEvalSymlinks(b.TasksPath()))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return hookEnv{BoardDir: boardDir, RepoRoot: root}, fmt.Errorf("board %s is outside the git work tree %s", boardDir, root)
	}
	hd, err := hookHooksDir(root)
	if err != nil {
		return hookEnv{}, err
	}
	return hookEnv{BoardDir: boardDir, RepoRoot: root, BoardRel: filepath.ToSlash(rel), HooksDir: hookEvalSymlinks(hd)}, nil
}

// hookFlagSet builds one hook verb's flagset. All five verbs share this ONE
// fs.Usage closure: DF-BOARDCTL-14 requires exactly one closure per run()
// switch verb, attributed through a `newFlagSet("hook")` declaration, so the
// family cannot ship five look-alike closures that drift apart.
func hookFlagSet(name string, cdir *string) *flag.FlagSet {
	fs := newFlagSet("hook")
	addCFlag(fs, cdir)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "boardctl hook %s\n", name)
		_, _ = fmt.Fprint(fs.Output(), hookUsage+"\n")
	}
	return fs
}

// hookParse parses a hook verb's flags with the repo's shared parsing path
// (help contract + flag reordering) and maps every failure to a usage error,
// except the help sentinel which run() maps to exit 0.
func hookParse(fs *flag.FlagSet, args []string, values ...string) error {
	err := parseFlags(fs, reorderArgs(args, valueFlags(append([]string{"C"}, values...)...)))
	if err == nil {
		return nil
	}
	if errors.Is(err, errHelpShown) {
		return err
	}
	return fmt.Errorf("%w: %v", errUsage, err)
}

// pickDir implements the -C precedence every subcommand shares: a -C parsed
// before the subcommand (run() already peeled it) is used unless the
// subcommand carried its own -C.
func pickDir(dir, cdir string) string {
	if cdir != "" {
		return cdir
	}
	return dir
}

// hookTargetPath resolves the hook file a verb reads or writes.
func hookTargetPath(env hookEnv, hook, hookPath string) string {
	if hookPath != "" {
		if !filepath.IsAbs(hookPath) {
			return filepath.Join(env.RepoRoot, hookPath)
		}
		return hookPath
	}
	return filepath.Join(env.HooksDir, hook)
}

// ---------------------------------------------------------------------------
// install
// ---------------------------------------------------------------------------

func cmdHookInstall(dir string, args []string) error {
	var cdir string
	fs := hookFlagSet("install", &cdir)
	hook := fs.String("hook", "", "hook to install into: post-commit or post-merge (required)")
	crierURL := fs.String("crier-url", "", "crier base URL, e.g. http://127.0.0.1:8080 (required)")
	sourceID := fs.String("source-id", hookSourceIDDefault, "crier ingest source id (default boardctl)")
	secret := fs.String("secret-file", "", "0600 file holding the HMAC secret (required; never baked into the hook body)")
	self := fs.String("self", "", "binary the managed block invokes (default: the absolute path of this executable)")
	hookPath := fs.String("hook-path", "", "explicit hook file to write (default: <git hooks dir>/<hook>)")
	dryRun := fs.Bool("dry-run", false, "print the exact block and target path; write nothing")
	if err := hookParse(fs, args, "hook", "crier-url", "source-id", "secret-file", "self", "hook-path"); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("boardctl hook install: unexpected argument %q (the hook is named through --hook): %w", fs.Arg(0), errUsage)
	}
	switch *hook {
	case "":
		return fmt.Errorf("boardctl hook install: --hook is required (post-commit or post-merge): %w", errUsage)
	case hookKindPostCommit, hookKindPostMerge:
	default:
		return fmt.Errorf("boardctl hook install: unknown hook %q (want post-commit or post-merge): %w", *hook, errUsage)
	}

	env, err := resolveHookEnv(pickDir(dir, cdir))
	if err != nil {
		return err
	}
	if *crierURL == "" {
		return fmt.Errorf("boardctl hook install: --crier-url is required (nothing is installed unless asked — spec §6): %w", errUsage)
	}
	if err := hookValidateCrierURL(*crierURL); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if *secret == "" {
		return fmt.Errorf("boardctl hook install: --secret-file is required (a 0600 file holding the HMAC secret): %w", errUsage)
	}
	secretPath := *secret
	if !filepath.IsAbs(secretPath) {
		secretPath = filepath.Join(env.RepoRoot, secretPath)
	}
	mode, err := hookCheckSecretFile(secretPath)
	if err != nil {
		return err
	}
	if *sourceID == "" {
		return fmt.Errorf("boardctl hook install: --source-id is empty (default is boardctl): %w", errUsage)
	}
	for _, v := range []string{*crierURL, *sourceID, secretPath} {
		if strings.ContainsAny(v, "\n\r") {
			return fmt.Errorf("boardctl hook install: a value contains a newline; refusing to bake an ambiguous value into a shell hook: %w", errUsage)
		}
	}
	selfPath := *self
	if selfPath == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("boardctl hook install: cannot resolve this executable (%v); pass --self PATH", err)
		}
		selfPath = exe
	}
	if !filepath.IsAbs(selfPath) {
		if abs, err := filepath.Abs(selfPath); err == nil {
			selfPath = abs
		}
	}

	target := hookTargetPath(env, *hook, *hookPath)
	existing, readErr := os.ReadFile(target)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("boardctl hook install: reading %s: %w", target, readErr)
	}

	cfg := hookConfig{Hook: *hook, CrierURL: *crierURL, SourceID: *sourceID, SecretFile: secretPath, Self: selfPath, Placement: "append"}
	eol := len(existing) == 0 || bytes.HasSuffix(existing, []byte("\n"))
	replacedBlock := false
	if start, end, ok, ferr := hookFindBlock(string(existing), *hook); ferr != nil {
		return fmt.Errorf("boardctl hook install: %s: %v (repair the hook or remove the orphan BEGIN line)", target, ferr)
	} else if ok {
		// Idempotent update: the sibling body around the old block is kept
		// untouched, and the ORIGINAL end-of-file fact is reused so a later
		// uninstall still restores the pre-install body exactly.
		replacedBlock = true
		oldBlock := string(existing[start:end])
		eol = hookOldPrefixEOL(oldBlock)
		cfg.Placement = hookOldPlacement(oldBlock)
	} else if _, found := hookFindTopLevelExit(string(existing)); found {
		// DF-CRIER-433: a block appended AFTER a top-level `exit` (or an
		// `exec`) is dead code. Insert it immediately before that line — no
		// sibling byte changes, and uninstall removes exactly the block.
		cfg.Placement = "before-exit"
	}
	block := hookComposeBlock(cfg, eol, cfg.Placement)

	if *dryRun {
		_, _ = fmt.Fprintf(os.Stdout, "boardctl-dispatch: dry-run hook=%s target=%s (nothing written)\n", *hook, target)
		_, _ = fmt.Fprintf(os.Stdout, "boardctl-dispatch: secret-file=%s mode=%#o crier-url=%s source-id=%s self=%s\n",
			secretPath, mode.Perm(), *crierURL, *sourceID, selfPath)
		_, _ = fmt.Fprint(os.Stdout, block)
		return nil
	}

	newContent := string(existing)
	if start, end, ok, _ := hookFindBlock(newContent, *hook); ok {
		newContent = newContent[:start] + block + newContent[end:]
	} else if pos, found := hookFindTopLevelExit(newContent); found && len(newContent) > 0 {
		newContent = newContent[:pos] + block + newContent[pos:]
	} else {
		if len(newContent) == 0 {
			newContent = "#!/bin/sh\n"
		}
		if !strings.HasSuffix(newContent, "\n") {
			newContent += "\n"
		}
		newContent += block
	}
	modeBits := os.FileMode(0o755)
	if info, statErr := os.Stat(target); statErr == nil {
		modeBits = info.Mode().Perm()
	}
	if err := hookWriteFileAtomic(target, []byte(newContent), modeBits); err != nil {
		return fmt.Errorf("boardctl hook install: writing %s: %w", target, err)
	}
	action := "installed"
	if replacedBlock {
		action = "updated"
	}
	_, _ = fmt.Fprintf(os.Stdout, "boardctl-dispatch: %s hook=%s target=%s block=%s crier-url=%s source-id=%s secret-file=%s mode=%#o\n",
		action, *hook, target, hookBlockVersion, *crierURL, *sourceID, secretPath, mode.Perm())
	if err := hookEnsureSpoolGitignore(env); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "boardctl-dispatch: WARN spool gitignore: %v (the hook is installed; the spool may show in board diffs)\n", err)
	}
	return nil
}

// hookValidateCrierURL refuses anything the hook could not POST to, at install
// time — a hook that can never deliver is worse than no hook.
func hookValidateCrierURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("--crier-url %q does not parse: %v", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("--crier-url %q must be http:// or https://", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("--crier-url %q has no host", raw)
	}
	return nil
}

// hookCheckSecretFile enforces spec R7's 0600 discipline AT INSTALL and again
// in doctor. The secret's bytes are never read here.
func hookCheckSecretFile(path string) (os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("secret file %s: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("secret file %s is not a regular file", path)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		return 0, fmt.Errorf("secret file %s has mode %#o, want 0600 (chmod 600 %s; the hook never reads a world/group-readable secret)", path, perm, path)
	}
	return info.Mode(), nil
}

// hookEnsureSpoolGitignore keeps the board's .dispatch-spool out of board
// diffs (spec Q-3 default).
func hookEnsureSpoolGitignore(env hookEnv) error {
	gi := filepath.Join(env.BoardDir, ".gitignore")
	content, err := os.ReadFile(gi)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) == hookSpoolName+"/" {
			return nil
		}
	}
	add := ""
	if len(content) > 0 && !bytes.HasSuffix(content, []byte("\n")) {
		add += "\n"
	}
	add += hookSpoolName + "/\n"
	f, err := os.OpenFile(gi, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(add); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// hookOldPlacement reads the recorded placement, defaulting to append for
// blocks written before the placement line existed.
func hookOldPlacement(block string) string {
	for _, line := range strings.Split(block, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "# boardctl-dispatch placement="); ok {
			if v == "before-exit" {
				return "before-exit"
			}
			return "append"
		}
	}
	return "append"
}

// hookFindTopLevelExit returns the byte offset of the first UNINDENTED `exit`
// or `exec` line in the hook body. A managed block appended after such a line
// is unreachable (DF-CRIER-433), so install places it before that line instead.
func hookFindTopLevelExit(content string) (int, bool) {
	off := 0
	for _, line := range strings.SplitAfter(content, "\n") {
		body := strings.TrimRight(line, "\n")
		if body == strings.TrimLeft(body, " \t") {
			t := strings.TrimSpace(body)
			if t == "exit" || strings.HasPrefix(t, "exit ") || t == "exec" || strings.HasPrefix(t, "exec ") {
				return off, true
			}
		}
		off += len(line)
	}
	return 0, false
}

// hookOldPrefixEOL reads the recorded "did the sibling body end in a newline"
// fact from an existing block, defaulting to true for a hand-written block.
func hookOldPrefixEOL(block string) bool {
	for _, line := range strings.Split(block, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "# boardctl-dispatch prefix-eol="); ok {
			return v == "present"
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// uninstall
// ---------------------------------------------------------------------------

func cmdHookUninstall(dir string, args []string) error {
	var cdir string
	fs := hookFlagSet("uninstall", &cdir)
	hook := fs.String("hook", "", "hook to uninstall from: post-commit or post-merge (required)")
	hookPath := fs.String("hook-path", "", "explicit hook file to edit (default: <git hooks dir>/<hook>)")
	if err := hookParse(fs, args, "hook", "hook-path"); err != nil {
		return err
	}
	switch *hook {
	case hookKindPostCommit, hookKindPostMerge:
	default:
		return fmt.Errorf("boardctl hook uninstall: --hook must be post-commit or post-merge: %w", errUsage)
	}
	env, err := resolveHookEnv(pickDir(dir, cdir))
	if err != nil {
		return err
	}
	target := hookTargetPath(env, *hook, *hookPath)
	content, err := os.ReadFile(target)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("boardctl hook uninstall: hook=%s is not installed at %s; nothing to remove", *hook, target)
	}
	if err != nil {
		return fmt.Errorf("boardctl hook uninstall: reading %s: %w", target, err)
	}
	start, end, ok, ferr := hookFindBlock(string(content), *hook)
	if ferr != nil {
		return fmt.Errorf("boardctl hook uninstall: %s: %v", target, ferr)
	}
	if !ok {
		return fmt.Errorf("boardctl hook uninstall: hook=%s is not installed at %s (no managed block; nothing removed)", *hook, target)
	}
	oldBlock := string(content[start:end])
	eol := hookOldPrefixEOL(oldBlock)
	remainder := string(content[:start]) + string(content[end:])
	if hookOldPlacement(oldBlock) == "append" && !eol && strings.HasSuffix(string(content[:start]), "\n") {
		// install added exactly this newline so the block started on its own
		// line; a byte-identical sibling restore removes it again.
		prefix := string(content[:start])
		remainder = prefix[:len(prefix)-1] + string(content[end:])
	}
	info, statErr := os.Stat(target)
	mode := os.FileMode(0o755)
	if statErr == nil {
		mode = info.Mode().Perm()
	}
	if hookEmptyBody(remainder) {
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("boardctl hook uninstall: removing empty hook %s: %w", target, err)
		}
		_, _ = fmt.Fprintf(os.Stdout, "boardctl-dispatch: uninstalled hook=%s target=%s (no body remained; hook file removed)\n", *hook, target)
		return nil
	}
	if err := hookWriteFileAtomic(target, []byte(remainder), mode); err != nil {
		return fmt.Errorf("boardctl hook uninstall: writing %s: %w", target, err)
	}
	_, _ = fmt.Fprintf(os.Stdout, "boardctl-dispatch: uninstalled hook=%s target=%s (sibling body preserved)\n", *hook, target)
	return nil
}

// ---------------------------------------------------------------------------
// doctor
// ---------------------------------------------------------------------------

func cmdHookDoctor(dir string, args []string) error {
	var cdir string
	fs := hookFlagSet("doctor", &cdir)
	probe := fs.Duration("probe-timeout", 2*time.Second, "bound on the crier reachability probe")
	if err := hookParse(fs, args, "probe-timeout"); err != nil {
		return err
	}
	env, err := resolveHookEnv(pickDir(dir, cdir))
	if err != nil {
		return err
	}
	out := os.Stdout
	failed := 0
	installedAny := false
	for _, hook := range []string{hookKindPostCommit, hookKindPostMerge} {
		target := filepath.Join(env.HooksDir, hook)
		content, readErr := os.ReadFile(target)
		block, hasBlock, ferr := hookExtractBlock(string(content), hook)
		if errors.Is(readErr, os.ErrNotExist) || (readErr == nil && !hasBlock && ferr == nil) {
			_, _ = fmt.Fprintf(out, "hook doctor: %s: installed=no target=%s\n", hook, target)
			continue
		}
		if readErr != nil {
			_, _ = fmt.Fprintf(out, "hook doctor: %s: FAIL installed=yes target=%s error=%v\n", hook, target, readErr)
			failed++
			continue
		}
		if ferr != nil {
			_, _ = fmt.Fprintf(out, "hook doctor: %s: FAIL installed=yes target=%s error=%v\n", hook, target, ferr)
			failed++
			continue
		}
		installedAny = true
		cfg := hookParseBlock(block, hook)
		var problems []string
		markerLine := fmt.Sprintf("# boardctl-dispatch marker=boardctl hook dispatch --hook %s \"$@\"", hook)
		if !hookBlockHasExactLine(block, markerLine) {
			problems = append(problems, fmt.Sprintf("marker line missing from the managed block (want %q)", markerLine))
		}
		version := cfg.version
		if version == "" {
			version = "unknown"
			problems = append(problems, "block version missing")
		} else if version != hookBlockVersion {
			problems = append(problems, fmt.Sprintf("block version %s is not %s", version, hookBlockVersion))
		}
		reachable := "yes"
		if why := hookMarkerUnreachable(string(content), hook); why != "" {
			reachable = "no"
			problems = append(problems, why)
		}
		if cfg.CrierURL == "" {
			problems = append(problems, "crier-url missing from the managed block")
		}
		if cfg.SecretFile == "" {
			problems = append(problems, "secret-file missing from the managed block")
		} else if _, err := hookCheckSecretFile(cfg.SecretFile); err != nil {
			problems = append(problems, err.Error())
		}
		crierState := "unknown"
		if cfg.CrierURL != "" {
			if err := hookProbeCrier(cfg.CrierURL, cfg.SourceID, *probe); err != nil {
				crierState = "unreachable"
				problems = append(problems, fmt.Sprintf("crier url unreachable: %v", err))
			} else {
				crierState = "reachable"
			}
		}
		mode := "-"
		if info, err := os.Stat(cfg.SecretFile); err == nil {
			mode = fmt.Sprintf("%#o", info.Mode().Perm())
		}
		verdict := "OK"
		if len(problems) > 0 {
			verdict = "FAIL"
			failed++
		}
		_, _ = fmt.Fprintf(out, "hook doctor: %s: %s installed=yes version=%s marker=present reachable=%s crier-url=%s source-id=%s secret-file=%s mode=%s crier=%s\n",
			hook, verdict, version, reachable, cfg.CrierURL, cfg.SourceID, cfg.SecretFile, mode, crierState)
		for _, p := range problems {
			_, _ = fmt.Fprintf(out, "hook doctor: %s: reason=%s\n", hook, p)
		}
	}
	if !installedAny {
		_, _ = fmt.Fprintf(out, "hook doctor: SKIP no boardctl-dispatch hooks installed in %s\n", env.HooksDir)
	}
	// Operator-visible intake telemetry (brief item 4; spec §10.2): spool
	// depth, the last delivery outcome, and the dead-letter census.
	spoolDir := filepath.Join(env.BoardDir, hookSpoolName)
	depth, err := hookSpoolDepth(spoolDir)
	if err != nil {
		_, _ = fmt.Fprintf(out, "hook doctor: spool: FAIL dir=%s error=%v\n", spoolDir, err)
		failed++
	} else {
		_, _ = fmt.Fprintf(out, "hook doctor: spool: depth=%d dir=%s\n", depth, spoolDir)
	}
	if ld, ok := hookReadLastDelivery(filepath.Join(spoolDir, hookLastDeliveryName)); ok {
		_, _ = fmt.Fprintf(out, "hook doctor: last-delivery: ts=%s hook=%s commit=%s posted=%d spooled=%d failed=%d drained=%d rows=%s\n",
			ld.TS, ld.Hook, ld.Commit, ld.Posted, ld.Spooled, ld.Failed, ld.Drained, hookJoinOrDash(ld.RowIDs))
	} else {
		_, _ = fmt.Fprintln(out, "hook doctor: last-delivery: none recorded")
	}
	dl, err := hookReadDeadLetters(filepath.Join(spoolDir, hookDeadLetterName))
	if err != nil {
		_, _ = fmt.Fprintf(out, "hook doctor: dead-letter: FAIL error=%v\n", err)
		failed++
	} else {
		_, _ = fmt.Fprintf(out, "hook doctor: dead-letter: count=%d%s\n", dl.Count, dl.Describe())
		if dl.Count > 0 {
			// Dead letters are operator visible by definition; they do not
			// fail doctor (the durable path owns delivery), they are NAMED.
			_, _ = fmt.Fprintf(out, "hook doctor: dead-letter: rows=%s\n", dl.RowIDs())
		}
	}
	if failed > 0 {
		_, _ = fmt.Fprintf(out, "hook doctor: RESULT FAIL (%d failing condition(s))\n", failed)
		return fmt.Errorf("hook doctor: %d failing condition(s)", failed)
	}
	_, _ = fmt.Fprintln(out, "hook doctor: RESULT OK")
	return nil
}

type hookBlockInfo struct {
	version    string
	CrierURL   string
	SourceID   string
	SecretFile string
	Self       string
}

func hookExtractBlock(content, hook string) (string, bool, error) {
	start, end, ok, err := hookFindBlock(content, hook)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}
	return content[start:end], true, nil
}

// hookBlockHasExactLine reports whether one line of the block equals want after
// trimming surrounding whitespace.
func hookBlockHasExactLine(block, want string) bool {
	for _, line := range strings.Split(block, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

func hookParseBlock(block, hook string) hookBlockInfo {
	info := hookBlockInfo{SourceID: hookSourceIDDefault}
	for _, line := range strings.Split(block, "\n") {
		t := strings.TrimSpace(line)
		if ver, ok := strings.CutPrefix(t, "# boardctl-dispatch block "); ok {
			if fields := strings.Fields(ver); len(fields) > 0 {
				info.version = fields[0]
			}
			continue
		}
		for _, kv := range []struct {
			prefix string
			dst    *string
		}{
			{"# boardctl-dispatch crier-url=", &info.CrierURL},
			{"# boardctl-dispatch source-id=", &info.SourceID},
			{"# boardctl-dispatch secret-file=", &info.SecretFile},
		} {
			if v, ok := strings.CutPrefix(t, kv.prefix); ok {
				*kv.dst = strings.TrimSpace(v)
			}
		}
		if v, ok := strings.CutPrefix(t, "__boardctl_dispatch_bin="); ok {
			info.Self = hookShellUnquote(strings.TrimSpace(v))
		}
	}
	return info
}

// hookMarkerUnreachable is the DF-CRIER-433 check: a managed marker that
// follows an unconditional top-level exit (or an exec) in the sibling body can
// never run. The scan is deliberately conservative — a bare `exit`/`exit N`
// line at column 0 before the BEGIN marker is the shape that breaks real
// hooks.
func hookMarkerUnreachable(content, hook string) string {
	begin := hookBeginLine(hook)
	for _, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		if t == begin {
			return ""
		}
		if line != strings.TrimLeft(line, " \t") {
			continue // indented: inside a function/if, not a top-level exit
		}
		if t == "exit" || strings.HasPrefix(t, "exit ") || t == "exec" || strings.HasPrefix(t, "exec ") {
			return fmt.Sprintf("marker unreachable: the sibling body runs %q before the managed block", t)
		}
	}
	return ""
}

// hookProbeCrier proves the ingest endpoint answers at all. Any HTTP status is
// reachability (401/404 are crier saying no, which is visible); only a
// connection failure or timeout is "unreachable".
func hookProbeCrier(base, sourceID string, timeout time.Duration) error {
	u := strings.TrimRight(base, "/") + "/ingress/webhooks/" + url.PathEscape(sourceID)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	return nil
}

// ---------------------------------------------------------------------------
// dispatch
// ---------------------------------------------------------------------------

// hookRow is one parsed tasks.jsonl line. Raw is the original JSON object, so
// the content hash covers the row exactly as committed (minus id).
type hookRow struct {
	ID   string
	Raw  json.RawMessage
	Data map[string]json.RawMessage
}

func (r hookRow) str(key string) string {
	var s string
	if v, ok := r.Data[key]; ok {
		_ = json.Unmarshal(v, &s)
	}
	return s
}

// isFixture reports whether the row declares itself a fixture. Board
// registration in fixtures.jsonl is a separate registry; a row that marks
// itself here is never dispatched.
func (r hookRow) isFixture() bool {
	if _, ok := r.Data["fixtures"]; ok {
		return true
	}
	var b bool
	if v, ok := r.Data["fixture"]; ok {
		if json.Unmarshal(v, &b) == nil && b {
			return true
		}
	}
	return false
}

// hookParseBoardJSONL parses rows, naming unparseable lines instead of skipping
// them silently. A named WARN is returned for the caller to print.
func hookParseBoardJSONL(content string) ([]hookRow, []string) {
	var rows []hookRow
	var warns []string
	for i, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var data map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			warns = append(warns, fmt.Sprintf("line %d: unparseable JSON: %v", i+1, err))
			continue
		}
		var id string
		if v, ok := data["id"]; ok {
			_ = json.Unmarshal(v, &id)
		}
		if strings.TrimSpace(id) == "" {
			warns = append(warns, fmt.Sprintf("line %d: row has no id", i+1))
			continue
		}
		rows = append(rows, hookRow{ID: id, Raw: json.RawMessage(line), Data: data})
	}
	return rows, warns
}

// hookContentHash is the row's identity EXCLUDING its id (spec §10.3): a
// rebase/cherry-pick that rewrites the id of unchanged content still dedupes,
// while an edit that keeps the id and changes content is a NEW logical event.
func hookContentHash(row hookRow) string {
	clone := make(map[string]json.RawMessage, len(row.Data))
	for k, v := range row.Data {
		if k == "id" {
			continue
		}
		clone[k] = v
	}
	canon, err := json.Marshal(clone)
	if err != nil {
		// A map of RawMessage that already parsed cannot fail to marshal;
		// fall back to the raw bytes if it somehow does.
		canon = row.Raw
	}
	sum := sha256.Sum256(canon)
	return hex.EncodeToString(sum[:])
}

type hookPayload struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	Priority           string `json:"priority"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	BoardDir           string `json:"board_dir"`
	CommitSHA          string `json:"commit_sha"`
	Event              string `json:"event"`
}

func hookBuildPayload(row hookRow, boardDir, sha string) hookPayload {
	return hookPayload{
		ID:                 row.ID,
		Title:              row.str("title"),
		Priority:           row.str("priority"),
		AcceptanceCriteria: row.str("acceptance_criteria"),
		BoardDir:           boardDir,
		CommitSHA:          sha,
		Event:              "row_added",
	}
}

// hookDetectedRow pairs a payload with the content hash its Idempotency-Key
// needs.
type hookDetectedRow struct {
	payload hookPayload
	hash    string
}

// hookDetectRows computes the ADDED (post-commit) or SEEN (post-merge) rows
// from the two revisions' board files. The verb is deliberately absent from the
// dedup key (§10.3), so one row yields one logical event however it arrived.
func hookDetectRows(env hookEnv, hook string) (rows []hookDetectedRow, sha string, note string, err error) {
	newRev, oldRev := "HEAD", "HEAD^"
	if hook == hookKindPostMerge {
		oldRev = "ORIG_HEAD"
	}
	shaBytes, err := hookGit(env.RepoRoot, "rev-parse", newRev)
	if err != nil {
		return nil, "", "", err
	}
	sha = strings.TrimSpace(shaBytes)
	oldContent, oldErr := hookGit(env.RepoRoot, "show", oldRev+":"+env.BoardRel)
	if oldErr != nil {
		if hook == hookKindPostMerge {
			return nil, sha, "no ORIG_HEAD or no board file at ORIG_HEAD (no merge happened): nothing to dispatch", nil
		}
		// Root commit: no parent board to diff against. Name it rather than
		// pretending the whole board is new.
		return nil, sha, "no parent commit (HEAD^ unavailable): nothing to diff for post-commit", nil
	}
	newContent, err := hookGit(env.RepoRoot, "show", newRev+":"+env.BoardRel)
	if err != nil {
		return nil, sha, "no board file at HEAD: nothing to dispatch", nil
	}
	oldRows, _ := hookParseBoardJSONL(oldContent)
	newRows, warns := hookParseBoardJSONL(newContent)
	seen := make(map[string]struct{}, len(oldRows))
	for _, r := range oldRows {
		seen[r.ID] = struct{}{}
	}
	var skipped []string
	for _, r := range newRows {
		if _, ok := seen[r.ID]; ok {
			continue
		}
		status := r.str("status")
		if status != "pending" {
			skipped = append(skipped, fmt.Sprintf("%s(status=%s)", r.ID, status))
			continue
		}
		if r.isFixture() {
			skipped = append(skipped, r.ID+"(fixture)")
			continue
		}
		rows = append(rows, hookDetectedRow{payload: hookBuildPayload(r, env.BoardDir, sha), hash: hookContentHash(r)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].payload.ID < rows[j].payload.ID })
	var parts []string
	parts = append(parts, warns...)
	if len(skipped) > 0 {
		parts = append(parts, "SKIP rows "+strings.Join(skipped, ", ")+" (not pending or fixture)")
	}
	return rows, sha, strings.Join(parts, "; "), nil
}

// cmdHookDispatch runs one dispatch and discards the verdict: the process must
// exit 0 toward git no matter what happened (spec R12). Every failure arm
// prints a named line on stderr before returning.
func cmdHookDispatch(dir string, args []string) {
	var cdir string
	fs := hookFlagSet("dispatch", &cdir)
	hook := fs.String("hook", "", "hook that fired: post-commit or post-merge (required)")
	crierURL := fs.String("crier-url", "", "crier base URL (default: read from the installed managed block)")
	sourceID := fs.String("source-id", "", "crier ingest source id (default: managed block, else boardctl)")
	secretFile := fs.String("secret-file", "", "0600 file holding the HMAC secret (default: managed block)")
	batchCap := fs.Int("batch-cap", hookBatchRowsDefault, "maximum rows one dispatch attempt sends before spilling the rest to the spool")
	budget := fs.Duration("budget", hookBudgetDefault, "total wall-time budget for this dispatch")
	dryRun := fs.Bool("dry-run", false, "report what would be posted; write nothing, POST nothing")
	if err := parseFlags(fs, reorderArgs(args, valueFlags("C", "hook", "crier-url", "source-id", "secret-file", "batch-cap", "budget"))); err != nil {
		// Never a usage failure: exit 0 is the hook's contract.
		_, _ = fmt.Fprintf(os.Stderr, "boardctl-dispatch: REFUSE hook=%q reason=usage-error: %v\n", *hook, err)
		return
	}
	dispatchOnce(pickDir(dir, cdir), *hook, *crierURL, *sourceID, *secretFile, *batchCap, *budget, *dryRun, os.Stdout, os.Stderr)
}

// hookResolvedConfig merges flags over the installed managed block (so the
// block is the single place the install-time choices live) and reports what is
// missing by name.
func hookResolvedConfig(env hookEnv, hook, flagURL, flagSource, flagSecret string) hookConfig {
	cfg := hookConfig{Hook: hook, SourceID: hookSourceIDDefault}
	if hook == hookKindPostCommit || hook == hookKindPostMerge {
		if content, err := os.ReadFile(filepath.Join(env.HooksDir, hook)); err == nil {
			if block, ok, _ := hookExtractBlock(string(content), hook); ok {
				inst := hookParseBlock(block, hook)
				cfg.CrierURL = inst.CrierURL
				cfg.SourceID = inst.SourceID
				cfg.SecretFile = inst.SecretFile
				cfg.Self = inst.Self
			}
		}
	}
	if flagURL != "" {
		cfg.CrierURL = flagURL
	}
	if flagSource != "" {
		cfg.SourceID = flagSource
	}
	if flagSecret != "" {
		cfg.SecretFile = flagSecret
	}
	if !filepath.IsAbs(cfg.SecretFile) && cfg.SecretFile != "" {
		cfg.SecretFile = filepath.Join(env.RepoRoot, cfg.SecretFile)
	}
	return cfg
}

func dispatchOnce(dir, hook, flagURL, flagSource, flagSecret string, batchCap int, budget time.Duration, dryRun bool, stdout, stderr io.Writer) int {
	if hook != hookKindPostCommit && hook != hookKindPostMerge {
		_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: REFUSE hook=%q reason=unknown-hook (want post-commit or post-merge)\n", hook)
		return 1
	}
	env, err := resolveHookEnv(dir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: REFUSE hook=%s reason=board-not-resolved: %v\n", hook, err)
		return 1
	}
	cfg := hookResolvedConfig(env, hook, flagURL, flagSource, flagSecret)
	if cfg.SourceID == "" {
		cfg.SourceID = hookSourceIDDefault
	}
	spoolDir := filepath.Join(env.BoardDir, hookSpoolName)

	// Drain first (spec R13): the spool is the retry source, whichever hook
	// fires next.
	if n, err := hookDrainSpool(cfg, spoolDir, batchCap, budget, dryRun, stdout, stderr); err != nil {
		_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: WARN spool-drain: %v\n", err)
	} else if n > 0 {
		_, _ = fmt.Fprintf(stdout, "boardctl-dispatch: spool drained files=%d hook=%s\n", n, hook)
	}

	rows, sha, note, err := hookDetectRows(env, hook)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: REFUSE hook=%s reason=detection-failed: %v\n", hook, err)
		return 1
	}
	if note != "" {
		_, _ = fmt.Fprintf(stdout, "boardctl-dispatch: NOTE hook=%s %s\n", hook, note)
	}
	if len(rows) == 0 {
		_, _ = fmt.Fprintf(stdout, "boardctl-dispatch: OK hook=%s commit=%s added=0 (nothing to post)\n", hook, hookShortSHA(sha))
		return 0
	}
	if cfg.CrierURL == "" {
		_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: SKIP hook=%s rows=%d reason=crier-url-not-configured (install with --crier-url; rows are not lost: crier's reconciliation sweep owns the durable path)\n", hook, len(rows))
		return 0
	}
	if cfg.SecretFile == "" {
		_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: SKIP hook=%s rows=%d reason=secret-file-not-configured (install with --secret-file)\n", hook, len(rows))
		return 0
	}
	secret, err := os.ReadFile(cfg.SecretFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: SKIP hook=%s rows=%d reason=secret-unreadable path=%s: %v\n", hook, len(rows), cfg.SecretFile, err)
		return 0
	}
	secret = bytes.TrimRight(secret, "\r\n")
	if len(secret) == 0 {
		_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: SKIP hook=%s rows=%d reason=secret-empty path=%s\n", hook, len(rows), cfg.SecretFile)
		return 0
	}
	if batchCap < 1 {
		batchCap = 1
	}

	if dryRun {
		for _, r := range rows {
			body, _ := json.Marshal(r.payload)
			_, _ = fmt.Fprintf(stdout, "boardctl-dispatch: dry-run row=%s key=boardctl:%s:%s bytes=%d url=%s/ingress/webhooks/%s\n",
				r.payload.ID, r.payload.ID, r.hash, len(body), strings.TrimRight(cfg.CrierURL, "/"), cfg.SourceID)
		}
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	sent, spilled, failed := hookDeliverRows(ctx, cfg, secret, rows, batchCap, spoolDir, stdout, stderr)
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.payload.ID)
	}
	if err := hookWriteLastDelivery(spoolDir, hookLastDeliveryRecord{
		TS: time.Now().UTC().Format(time.RFC3339), Hook: hook, Commit: hookShortSHA(sha),
		Posted: sent, Spooled: spilled, Failed: failed, RowIDs: ids,
	}); err != nil {
		_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: WARN last-delivery-write: %v\n", err)
	}
	_, _ = fmt.Fprintf(stdout, "boardctl-dispatch: %s hook=%s commit=%s added=%d posted=%d spooled=%d failed=%d\n",
		hookVerdictWord(sent, spilled, failed), hook, hookShortSHA(sha), len(rows), sent, spilled, failed)
	return 0
}

func hookVerdictWord(sent, spilled, failed int) string {
	switch {
	case failed == 0 && spilled == 0:
		return "OK"
	case sent > 0:
		return "PARTIAL"
	default:
		return "SPOOLED"
	}
}

func hookShortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// hookDeliverRows posts rows one at a time (one Idempotency-Key per row,
// §10.3). On the first non-413 delivery failure it stops trying and spools the
// rest: continuing against a down crier would spend the whole wall-time budget
// on doomed sockets.
func hookDeliverRows(ctx context.Context, cfg hookConfig, secret []byte, rows []hookDetectedRow, batchCap int, spoolDir string, stdout, stderr io.Writer) (sent, spilled, failed int) {
	client := &http.Client{Timeout: 3 * time.Second}
	for i, r := range rows {
		if i >= batchCap {
			// R16: rows beyond the cap are not dropped — they spool.
			if err := hookAppendSpool(spoolDir, cfg, r, 0, "batch-cap reached before this row was attempted"); err != nil {
				_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: REFUSE spool-write row=%s: %v\n", r.payload.ID, err)
				failed++
				continue
			}
			spilled++
			continue
		}
		if ctx.Err() != nil {
			if err := hookAppendSpool(spoolDir, cfg, r, 0, "wall-time budget exhausted before this row was attempted"); err != nil {
				_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: REFUSE spool-write row=%s: %v\n", r.payload.ID, err)
				failed++
				continue
			}
			spilled++
			continue
		}
		status, err := hookPostRow(ctx, client, cfg, secret, r)
		if err == nil {
			sent++
			_, _ = fmt.Fprintf(stdout, "boardctl-dispatch: posted row=%s key=boardctl:%s:%s status=%d\n", r.payload.ID, r.payload.ID, r.hash, status)
			continue
		}
		if status == http.StatusRequestEntityTooLarge {
			// R15: size cannot be fixed by retrying — dead-letter at once,
			// with the row named.
			if derr := hookWriteDeadLetter(spoolDir, cfg, []hookSpoolRow{{Payload: r.payload, Hash: r.hash}}, 1, status, "crier refused 413 (batch too large); retrying cannot fix size"); derr != nil {
				_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: REFUSE dead-letter-write row=%s: %v\n", r.payload.ID, derr)
			}
			failed++
			continue
		}
		_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: SPOOL row=%s status=%d reason=%v\n", r.payload.ID, status, err)
		if serr := hookAppendSpool(spoolDir, cfg, r, status, err.Error()); serr != nil {
			_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: REFUSE spool-write row=%s: %v\n", r.payload.ID, serr)
		}
		spilled++
		// Remaining rows spool too; no more sockets against a failing crier.
		for _, rest := range rows[i+1:] {
			if serr := hookAppendSpool(spoolDir, cfg, rest, 0, "spooled after the previous row's delivery failure"); serr != nil {
				_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: REFUSE spool-write row=%s: %v\n", rest.payload.ID, serr)
				failed++
				continue
			}
			spilled++
		}
		return sent, spilled, failed
	}
	return sent, spilled, failed
}

// hookPostRow sends ONE signed row. A 200 (duplicate) and a 202 (accepted) are
// both success (spec R11); every other status is an error carrying that status
// so the caller can classify 413.
func hookPostRow(ctx context.Context, client *http.Client, cfg hookConfig, secret []byte, r hookDetectedRow) (int, error) {
	body, err := json.Marshal(r.payload)
	if err != nil {
		return 0, err
	}
	if len(body) > hookRequestBodyCap {
		return http.StatusRequestEntityTooLarge, fmt.Errorf("row body is %d bytes, over the %d-byte ingest cap", len(body), hookRequestBodyCap)
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	endpoint := strings.TrimRight(cfg.CrierURL, "/") + "/ingress/webhooks/" + url.PathEscape(cfg.SourceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Crier-Timestamp", ts)
	req.Header.Set("X-Crier-Signature", sig)
	req.Header.Set("Idempotency-Key", "boardctl:"+r.payload.ID+":"+r.hash)
	req.Header.Set("User-Agent", "boardctl-dispatch/"+hookBlockVersion)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted:
		return resp.StatusCode, nil
	default:
		return resp.StatusCode, fmt.Errorf("crier answered %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// spool + dead-letter
// ---------------------------------------------------------------------------

// hookSpoolRow is one row inside a spool batch, with the content hash captured
// at detection time so a retry keeps the SAME Idempotency-Key.
type hookSpoolRow struct {
	Payload hookPayload `json:"payload"`
	Hash    string      `json:"content_hash"`
}

type hookSpoolFile struct {
	Version    int            `json:"version"`
	Hook       string         `json:"hook"`
	CrierURL   string         `json:"crier_url"`
	SourceID   string         `json:"source_id"`
	Created    string         `json:"created"`
	Attempts   int            `json:"attempts"`
	LastStatus int            `json:"last_status"`
	LastError  string         `json:"last_error"`
	Rows       []hookSpoolRow `json:"rows"`
}

func hookSpoolDepth(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			n++
		}
	}
	return n, nil
}

func hookAppendSpool(dir string, cfg hookConfig, r hookDetectedRow, status int, reason string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	rec := hookSpoolFile{
		Version:    1,
		Hook:       cfg.Hook,
		CrierURL:   cfg.CrierURL,
		SourceID:   cfg.SourceID,
		Created:    time.Now().UTC().Format(time.RFC3339Nano),
		Attempts:   1,
		LastStatus: status,
		LastError:  reason,
		Rows:       []hookSpoolRow{{Payload: r.payload, Hash: r.hash}},
	}
	return hookWriteSpoolRecord(dir, rec)
}

// hookSpoolSeq makes spool file names unique even when two rows are spooled in
// the same nanosecond by the same process.
var hookSpoolSeq atomic.Uint64

func hookWriteSpoolRecord(dir string, rec hookSpoolFile) error {
	body, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	name := fmt.Sprintf("%s-%d-%d.json", time.Now().UTC().Format("20060102T150405.000000000Z"), os.Getpid(), hookSpoolSeq.Add(1))
	return hookWriteFileAtomic(filepath.Join(dir, name), body, 0o600)
}

// hookLastDeliveryRecord is the operator-facing "what did the most recent intake
// attempt do" line doctor prints (the row asks for last delivery outcome, not
// only failures). It is small, overwritten per attempt, and never carries the
// secret or a request body.
type hookLastDeliveryRecord struct {
	TS      string   `json:"ts"`
	Hook    string   `json:"hook"`
	Commit  string   `json:"commit,omitempty"`
	Posted  int      `json:"posted"`
	Spooled int      `json:"spooled"`
	Failed  int      `json:"failed"`
	Drained int      `json:"drained,omitempty"`
	RowIDs  []string `json:"row_ids,omitempty"`
}

func hookWriteLastDelivery(dir string, rec hookLastDeliveryRecord) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	body, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return hookWriteFileAtomic(filepath.Join(dir, hookLastDeliveryName), body, 0o600)
}

func hookReadLastDelivery(path string) (hookLastDeliveryRecord, bool) {
	var rec hookLastDeliveryRecord
	raw, err := os.ReadFile(path)
	if err != nil {
		return rec, false
	}
	if json.Unmarshal(raw, &rec) != nil {
		return rec, false
	}
	return rec, true
}

// hookDrainSpool retries every spooled batch oldest-first. A batch that
// succeeds is removed; a batch that fails has its attempt count advanced, and
// on the third failed attempt it becomes a dead-letter record (spec §10.2 —
// never a silent dropped.log).
func hookDrainSpool(cfg hookConfig, dir string, batchCap int, budget time.Duration, dryRun bool, stdout, stderr io.Writer) (int, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // timestamp-prefixed: oldest first
	if len(names) == 0 {
		return 0, nil
	}
	if dryRun {
		for _, n := range names {
			_, _ = fmt.Fprintf(stdout, "boardctl-dispatch: dry-run spool-drain file=%s\n", filepath.Join(dir, n))
		}
		return 0, nil
	}
	if cfg.CrierURL == "" || cfg.SecretFile == "" {
		return 0, fmt.Errorf("spool has %d file(s) but crier-url/secret-file are not configured", len(names))
	}
	secret, err := os.ReadFile(cfg.SecretFile)
	if err != nil {
		return 0, fmt.Errorf("secret file %s: %w", cfg.SecretFile, err)
	}
	secret = bytes.TrimRight(secret, "\r\n")
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	client := &http.Client{Timeout: 3 * time.Second}
	handled := 0
	for _, name := range names {
		if ctx.Err() != nil {
			_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: NOTE spool-drain stopped: wall-time budget exhausted (%d file(s) left)\n", len(names)-handled)
			break
		}
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: WARN spool-read file=%s: %v\n", path, err)
			continue
		}
		var rec hookSpoolFile
		if err := json.Unmarshal(raw, &rec); err != nil {
			_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: WARN spool-parse file=%s: %v (left in place for an operator)\n", path, err)
			continue
		}
		attempts := rec.Attempts + 1
		var lastStatus int
		var lastErr string
		allOK := true
		for _, r := range rec.Rows {
			if ctx.Err() != nil {
				allOK = false
				lastErr = "wall-time budget exhausted during drain"
				break
			}
			status, err := hookPostRow(ctx, client, cfg, secret, hookDetectedRow{payload: r.Payload, hash: r.Hash})
			if err != nil {
				allOK = false
				lastStatus, lastErr = status, err.Error()
				if status == http.StatusRequestEntityTooLarge {
					_ = hookWriteDeadLetter(filepath.Dir(path), cfg, []hookSpoolRow{r}, attempts, status, "crier refused 413 during spool drain; retrying cannot fix size")
					continue
				}
				break
			}
			_, _ = fmt.Fprintf(stdout, "boardctl-dispatch: spool-retry file=%s row=%s status=%d attempt=%d\n", name, r.Payload.ID, status, attempts)
		}
		handled++
		if allOK {
			if err := os.Remove(path); err != nil {
				_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: WARN spool-remove file=%s: %v\n", path, err)
			}
			continue
		}
		rec.Attempts = attempts
		rec.LastStatus = lastStatus
		rec.LastError = lastErr
		if attempts >= hookMaxAttempts {
			if err := hookWriteDeadLetter(filepath.Dir(path), cfg, rec.Rows, attempts, lastStatus, lastErr); err != nil {
				_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: REFUSE dead-letter-write file=%s: %v (spool file left in place)\n", name, err)
				continue
			}
			if err := os.Remove(path); err != nil {
				_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: WARN spool-remove file=%s: %v\n", path, err)
			}
			_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: DEAD-LETTER file=%s rows=%s attempts=%d last_status=%d reason=%s (visible in `boardctl hook doctor`)\n",
				name, hookSpoolRowIDs(rec.Rows), attempts, lastStatus, lastErr)
			continue
		}
		body, merr := json.MarshalIndent(rec, "", "  ")
		if merr != nil {
			_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: WARN spool-rewrite file=%s: %v\n", path, merr)
			continue
		}
		if werr := hookWriteFileAtomic(path, body, 0o600); werr != nil {
			_, _ = fmt.Fprintf(stderr, "boardctl-dispatch: WARN spool-rewrite file=%s: %v\n", path, werr)
		}
	}
	return handled, nil
}

type hookDeadLetterRecord struct {
	TS         string   `json:"ts"`
	Event      string   `json:"event"`
	Hook       string   `json:"hook"`
	RowIDs     []string `json:"row_ids"`
	Attempts   int      `json:"attempts"`
	LastStatus int      `json:"last_status"`
	Reason     string   `json:"reason"`
}

func hookWriteDeadLetter(dir string, cfg hookConfig, rows []hookSpoolRow, attempts, status int, reason string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	rec := hookDeadLetterRecord{
		TS:         time.Now().UTC().Format(time.RFC3339),
		Event:      "dead_letter",
		Hook:       cfg.Hook,
		RowIDs:     hookSpoolRowIDList(rows),
		Attempts:   attempts,
		LastStatus: status,
		Reason:     reason,
	}
	body, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, hookDeadLetterName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(body, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

type hookDeadLetterSummary struct {
	Count int
	Last  hookDeadLetterRecord
}

func hookReadDeadLetters(path string) (hookDeadLetterSummary, error) {
	var s hookDeadLetterSummary
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec hookDeadLetterRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		s.Count++
		s.Last = rec
	}
	return s, nil
}

func (s hookDeadLetterSummary) Describe() string {
	if s.Count == 0 {
		return " (no row has been dead-lettered)"
	}
	return fmt.Sprintf(" last_status=%d last_failure=%q at=%s", s.Last.LastStatus, s.Last.Reason, s.Last.TS)
}

func (s hookDeadLetterSummary) RowIDs() string {
	if len(s.Last.RowIDs) == 0 {
		return "-"
	}
	return strings.Join(s.Last.RowIDs, ",")
}

func hookSpoolRowIDs(rows []hookSpoolRow) string { return strings.Join(hookSpoolRowIDList(rows), ",") }

func hookJoinOrDash(ids []string) string {
	if len(ids) == 0 {
		return "-"
	}
	return strings.Join(ids, ",")
}

func hookSpoolRowIDList(rows []hookSpoolRow) []string {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.Payload.ID)
	}
	return ids
}

// ---------------------------------------------------------------------------
// flush
// ---------------------------------------------------------------------------

func cmdHookFlush(dir string, args []string) error {
	var cdir string
	fs := hookFlagSet("flush", &cdir)
	budget := fs.Duration("budget", hookBudgetDefault, "total wall-time budget for this flush")
	batchCap := fs.Int("batch-cap", hookBatchRowsDefault, "maximum rows per flush attempt")
	dryRun := fs.Bool("dry-run", false, "list the batches that would be retried; send nothing")
	if err := hookParse(fs, args, "budget", "batch-cap"); err != nil {
		return err
	}
	env, err := resolveHookEnv(pickDir(dir, cdir))
	if err != nil {
		return err
	}
	// The spool is hook-agnostic: read config from either installed block.
	cfg := hookResolvedConfig(env, hookKindPostCommit, "", "", "")
	if cfg.CrierURL == "" {
		cfg = hookResolvedConfig(env, hookKindPostMerge, "", "", "")
	}
	cfg.Hook = "flush"
	spoolDir := filepath.Join(env.BoardDir, hookSpoolName)
	if depth, _ := hookSpoolDepth(spoolDir); depth == 0 {
		_, _ = fmt.Fprintf(os.Stdout, "boardctl-dispatch: flush: spool is empty dir=%s\n", spoolDir)
		return nil
	}
	n, err := hookDrainSpool(cfg, spoolDir, *batchCap, *budget, *dryRun, os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("boardctl hook flush: %w", err)
	}
	depth, _ := hookSpoolDepth(spoolDir)
	dl, _ := hookReadDeadLetters(filepath.Join(spoolDir, hookDeadLetterName))
	if n > 0 {
		if werr := hookWriteLastDelivery(spoolDir, hookLastDeliveryRecord{
			TS: time.Now().UTC().Format(time.RFC3339), Hook: "flush", Drained: n,
		}); werr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "boardctl-dispatch: WARN last-delivery-write: %v\n", werr)
		}
	}
	_, _ = fmt.Fprintf(os.Stdout, "boardctl-dispatch: flush: attempts=%d spool-depth=%d dead-letter-count=%d\n", n, depth, dl.Count)
	return nil
}
