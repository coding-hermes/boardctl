package main

// DF-BOARDCTL-54: mechanical README-fence gate.
//
// Every ```-fenced `boardctl` command in README.md is executed against a
// throwaway git+board environment built from the CURRENT working tree, and
// the gate FAILS on any non-zero exit that the allowlist below does not
// explicitly document. This is the mechanical half of the dogfood run-28
// finding: the README's headline completion example and its deferred example
// had drifted out of sync with the write-path guards (BOARD-GUARD-003
// full-hash provenance, DEFER-EVIDENCE --defer-reason), and nothing noticed
// until a human ran the fences. With this gate, README recipe drift breaks
// `go test ./...` instead of waiting for the next dogfood run.
//
// Environment contract (mirrors what a reader following the README has):
//   - each fence runs against a FRESH throwaway $HOME: every `-C ~/name`
//     root is a real git repository (one seed commit carrying a product
//     file, app.go) with an initialized topology-A board;
//   - TMPDIR is redirected to an EMPTY directory, so the boards are NOT
//     under os.TempDir() and the binary's test-environment carve-out
//     (internal/board/evidence_gate.go isTestEnvironment) does NOT apply:
//     the completion provenance gate runs at full strength (resolvable
//     commit hashes, existing .gitreins/history run dirs, verdict content);
//   - rows that a fence references but never creates (e.g. `show
//     DF-MYPROJECT-1` in the Usage fence) are seeded by the harness and
//     listed in the seed table below — the fence documents a board that
//     already carries those rows;
//   - fences whose commands need an evidence run (`update <id> --status
//     complete --evidence-run-id R`) get a verdict.json fixture graded for
//     exactly that task id and the repo's HEAD, which is what TR-251
//     requires a real caller to have.
//
// Skipped fences (each skip is logged with its reason, never silent):
//   - non-boardctl fences (curl/go/make install snippets, jq/sed pipelines,
//     dedupe-board invocations) — outside the boardctl recipe surface;
//   - MIXED fences that interleave boardctl with other setup binaries
//     (mktemp/printf demo rigs, mkdir/ln umbrella rigs, cp/diff comparisons):
//     the fence is a script, not a recipe, and executing half of it would
//     prove nothing;
//   - `boardctl serve` fences: a long-running loopback daemon, not a
//     batch command;
//   - fences carrying `<placeholder>` arguments (e.g. `-C <dir>` in the
//     headerless-board section): they are prose-shaped by construction.
//
// The allowlist is keyed by (fence's first command, normalized command) —
// content-derived, so it survives unrelated README edits above a fence — and
// every entry must MATCH at least once: an allowlist entry whose command
// disappeared from the README is reported as stale, so the negative space
// cannot rot.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// allowlistEntry is one permitted non-zero outcome: the exit code plus a
// substring that MUST appear in the combined output (empty marker: any
// output). An entry matches when rc and marker both agree.
type allowlistEntry struct {
	rc     int
	marker string
}

// readmeFenceAllowlist documents every README fence command that is EXPECTED
// to exit non-zero. Everything else must exit 0.
//
// Key: "<first command of the fence>|<normalized command>" where the command
// is normalized by stripping the leading "boardctl" and the root-agnostic
// "-C <root>" segment and collapsing whitespace.
var readmeFenceAllowlist = map[string][]allowlistEntry{
	// Usage quickstart: the fence re-files FEAT-1 twice to show alternative
	// create forms (with --acceptance-criteria, with --evidence-run-id). Run
	// sequentially against the board the fence itself just created FEAT-1 on,
	// the duplicate-ID guard refuses with exit 1 — the documented outcome of
	// re-creating an existing id (the exit-2 DUPLICATE-SUPPRESSED path in the
	// same section applies to re-detected findings under a DIFFERENT id).
	`stats|create --id FEAT-1 --title "Add retry" --acceptance-criteria "1) retry is idempotent; 2) full suite green"`: {
		{rc: 1, marker: "already exists in tasks.jsonl"},
	},
	`stats|create --id FEAT-1 --title "Add retry" --reasoning "Idempotent retry with backoff" --evidence-run-id qa-2026-09-18T04:11Z`: {
		{rc: 1, marker: "already exists in tasks.jsonl"},
	},
	// The --detail @file example reads an operator-supplied JSON file
	// (/tmp/detail.json is created by the caller, never by the README), so
	// the documented refusal is "no such file". rc=0 is also tolerated for
	// environments where the operator file happens to exist.
	`stats|event --type audit --tick 44 --task-id BT-042 --actor foreman --detail @/tmp/detail.json`: {
		{rc: 1, marker: "no such file"},
		{rc: 0, marker: ""},
	},
	`event --type audit --tick 44 --task-id BT-042 --actor foreman --detail @/tmp/detail.json|event --type audit --tick 44 --task-id BT-042 --actor foreman --detail @/tmp/detail.json`: {
		{rc: 1, marker: "no such file"},
		{rc: 0, marker: ""},
	},
	// Status-alias section: INTENTIONALLY negative — the fence comment says
	// "exit 1 — writes reject aliases". A drift to rc=0 here would mean the
	// write-vocabulary guard vanished, so no rc=0 arm is offered.
	`validate|update FEAT-1 --status done`: {
		{rc: 1, marker: "not in write vocabulary"},
	},
	// Task-id format section: INTENTIONALLY negative — the fence comment says
	// "exit 1"; --force is the documented escape hatch (next line, rc 0).
	`create --id "bad id!" --title x|create --id "bad id!" --title x`: {
		{rc: 1, marker: "does not match the fleet id format"},
	},
	// Fleet rollout (BT-053): the first example's --fleet pattern names the
	// OPERATOR's home (`/home/me/*`), which a throwaway environment cannot
	// reproduce — the refusal ("no repository under <home> matches") is the
	// rollout guard correctly matching nothing. The second example IS
	// reproduced (the harness seeds ~/src/axiom + ~/src/crier with boards, so
	// the pattern-match + hook-install path runs for real).
	`install --fleet '/home/me/*' --dry-run|install --fleet '/home/me/*' --dry-run`: {
		{rc: 1, marker: "no repository under"},
	},
}

// readmeFenceSeeds lists rows a fence references but never creates, keyed by
// the fence's first command. The harness creates each row (open, pending)
// before the fence runs. Documented preconditions, not behavior overrides:
// seeding only ever runs `create --id X --title "Seed X"`.
var readmeFenceSeeds = map[string][]string{
	// Usage quickstart: `next` and `show DF-MYPROJECT-1 --events` document
	// reading a board that already carries a row.
	"stats": {"DF-MYPROJECT-1"},
	// show FEAT-1 examples.
	"show FEAT-1": {"FEAT-1"},
	// update FEAT-2 --normalize + validate --strict-keys.
	"update FEAT-2 --normalize": {"FEAT-2"},
	// Status-alias section (fences starting with a bare `validate`) — FEAT-1
	// must exist for the intentionally refused `--status done` write.
	"validate": {"FEAT-1"},
	// Finding-fingerprints closeout examples operate on the EXISTING QA-X-3
	// row ("Evidence the EXISTING row", README).
	`update QA-X-3 --summary "re-detected on run qa-1"`: {"QA-X-3"},
	// BT-077 plural-association examples update and show FEAT-3.
	"update FEAT-3 --pull-request 42 --pull-request 43": {"FEAT-3"},
}

var (
	readmeDashCRe     = regexp.MustCompile(`\s-C\s+\S+`)
	readmeCommentRe   = regexp.MustCompile(`\s#.*$`)
	readmePlaceholder = regexp.MustCompile(`[<>]`)
)

type readmeCommand struct {
	line int
	text string
}

type readmeFence struct {
	startLine int
	commands  []readmeCommand
}

// parseReadmeFences extracts every fenced code block's commands, with shell
// backslash-continuations joined and full-line comments dropped (trailing
// comments stay — the shell ignores them).
func parseReadmeFences(t *testing.T, readme string) []readmeFence {
	t.Helper()
	raw, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("read %s: %v", readme, err)
	}
	lines := strings.Split(string(raw), "\n")
	var fences []readmeFence
	var cur *readmeFence
	var buf []string
	bufStart := 0
	for i, line := range lines {
		ln := i + 1
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if cur != nil {
				if len(buf) > 0 { // dangling backslash-continuation at fence end
					cur.commands = append(cur.commands, readmeCommand{line: bufStart, text: strings.Join(buf, " ")})
				}
				fences = append(fences, *cur)
				cur = nil
				buf = nil
			} else {
				cur = &readmeFence{startLine: ln}
			}
			continue
		}
		if cur == nil {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasSuffix(trimmed, "\\") {
			if len(buf) == 0 {
				bufStart = ln
			}
			buf = append(buf, strings.TrimSpace(strings.TrimSuffix(trimmed, "\\")))
			continue
		}
		if len(buf) == 0 {
			bufStart = ln
		}
		buf = append(buf, trimmed)
		cur.commands = append(cur.commands, readmeCommand{line: bufStart, text: strings.Join(buf, " ")})
		buf = nil
	}
	return fences
}

func firstToken(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

// classifyReadmeFence decides whether a fence is mechanically executable and
// otherwise returns the reason it is skipped (logged, never silent).
// Classification runs on the comment-stripped command core: a trailing
// comment's `->` or `<dir>` says nothing about what the shell executes.
func classifyReadmeFence(f readmeFence) (skip string) {
	for _, c := range f.commands {
		core := readmeCommentRe.ReplaceAllString(" "+strings.TrimSpace(c.text), "")
		tok := firstToken(core)
		fields := strings.Fields(core)
		switch {
		case tok == "cd":
			if len(fields) < 2 || !strings.HasPrefix(fields[1], "~/") {
				return fmt.Sprintf("line %d: cd to a non-~ root (%q) cannot be mapped to the throwaway home", c.line, c.text)
			}
		case tok != "boardctl":
			return fmt.Sprintf("line %d: non-boardctl command %q (fence mixes binaries; it is a setup/demo script, not a boardctl recipe)", c.line, c.text)
		case len(fields) > 1 && fields[1] == "serve":
			return fmt.Sprintf("line %d: `boardctl serve` is a long-running loopback daemon, not a batch command", c.line)
		}
		if readmePlaceholder.MatchString(core) {
			return fmt.Sprintf("line %d: command carries a <placeholder> or shell redirect (%q) — illustrative, not runnable as written", c.line, c.text)
		}
	}
	return ""
}

// normalizeReadmeCommand strips trailing comments, the `-C <root>` segment
// and the leading `boardctl` so allowlist/seed keys are independent of the
// root spelling and of decorative trailing comments. Order matters: the -C
// regex needs a leading boundary, so it runs BEFORE "boardctl " is trimmed.
// A `#` only starts a comment at a word boundary (README:1585's
// `repo:path#ANCHOR` refs survive), and full-line comments never reach this
// function (dropped at parse time).
func normalizeReadmeCommand(text string) string {
	s := " " + strings.TrimSpace(text)
	s = readmeCommentRe.ReplaceAllString(s, "")
	s = readmeDashCRe.ReplaceAllString(s, " ")
	s = strings.TrimPrefix(strings.TrimSpace(s), "boardctl ")
	return strings.Join(strings.Fields(s), " ")
}

// fenceKey derives the content-stable key for a fence: its first command,
// normalized. Two fences that start with the same command share the key —
// their allowlist/seed expectations must then coincide (checked below).
func fenceKey(f readmeFence) string {
	if len(f.commands) == 0 {
		return "<none>"
	}
	return normalizeReadmeCommand(f.commands[0].text)
}

// readmeEnv is one fence's throwaway environment.
type readmeEnv struct {
	t        *testing.T
	home     string
	bin      string
	emptyTmp string
}

// rootFor resolves a `-C`/`cd` token to a directory under the fence's
// throwaway home. `~/x` is NOT rewritten: the shell's own ~ expansion (HOME
// is set to the throwaway home) resolves it to the same path the setup step
// materialized. Only absolute placeholder roots (/path/to/repo) are
// rewritten, by the caller.
func (e *readmeEnv) rootFor(token string) string {
	e.t.Helper()
	if strings.HasPrefix(token, "~") {
		return filepath.Join(e.home, strings.TrimPrefix(token, "~"))
	}
	if filepath.IsAbs(token) {
		return token
	}
	e.t.Fatalf("unmappable root token %q", token)
	return ""
}

// cRoot extracts the working root a command names via -C ("" when absent).
func (e *readmeEnv) cRoot(text string) string {
	m := readmeDashCRe.FindString(text)
	if m == "" {
		return ""
	}
	tok := strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(m), "-C")), "\"'")
	return e.rootFor(tok)
}

// setupReadmeRoot turns a mapped root into the README's precondition: a git
// repo with one seed commit that touches a PRODUCT file (app.go — the
// completion provenance gate refuses commits that touch zero product files),
// plus an initialized topology-A board.
func (e *readmeEnv) setupReadmeRoot(root string) {
	e.t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		e.t.Fatalf("mkdir %s: %v", root, err)
	}
	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = root
		cmd.Env = e.childEnv()
		out, err := cmd.CombinedOutput()
		if err != nil {
			e.t.Fatalf("setup %s %v in %s: %v\n%s", name, args, root, err, out)
		}
	}
	run("git", "init")
	if err := os.WriteFile(filepath.Join(root, "app.go"), []byte("package main\n"), 0o644); err != nil {
		e.t.Fatalf("write app.go: %v", err)
	}
	run("git", "add", "app.go")
	run("git", "-c", "user.name=boardctl-fence-gate", "-c", "user.email=fence-gate@invalid", "commit", "-m", "seed")
	run(e.bin, "-C", root, "init")
}

// childEnv builds the environment for fence commands: HOME points at the
// throwaway home (~ expansion, hook secret paths, git isolation) and TMPDIR
// at an EMPTY directory so the boardctl binary's isTestEnvironment carve-out
// never applies (the completion provenance gate must run at full strength).
func (e *readmeEnv) childEnv() []string {
	return append(os.Environ(),
		"HOME="+e.home,
		"TMPDIR="+e.emptyTmp,
		"PATH="+filepath.Dir(e.bin)+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
}

func (e *readmeEnv) output(cmd *exec.Cmd) (string, error) {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// workDir resolves the directory a command runs in: the -C root when named,
// else the fence's cd target, else the throwaway home.
func (e *readmeEnv) workDir(text, cdRoot, home string) string {
	if root := e.cRoot(text); root != "" {
		return root
	}
	if firstToken(text) == "cd" {
		return e.rootFor(strings.Fields(text)[1])
	}
	if cdRoot != "" {
		return cdRoot
	}
	return home
}

// seedRoot is the root a fence's seeds and fixtures belong to: the working
// root of the fence's first -C command (every seeded fence names its rows on
// one board consistently).
func (e *readmeEnv) seedRoot(cmds []readmeCommand) string {
	for _, c := range cmds {
		if root := e.cRoot(c.text); root != "" {
			return root
		}
	}
	return e.home
}

func TestREADMEFences(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	readme := filepath.Join(repoRoot, "README.md")

	// Build the current working tree's binary once; every fence command runs
	// through it (its dir goes first on PATH so bare `boardctl` resolves to
	// THIS build).
	binDir := t.TempDir()
	bin := filepath.Join(binDir, "boardctl")
	build := exec.Command("go", "build", "-o", bin, "./cmd/boardctl")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/boardctl: %v\n%s", err, out)
	}

	fences := parseReadmeFences(t, readme)
	if len(fences) == 0 {
		t.Fatal("no fenced blocks found in README.md — parser or file path broken")
	}

	// matchedAllowlist tracks which allowlist keys fired; after the sweep,
	// every entry must have matched at least once (stale-entry detection).
	matchedAllowlist := map[string]bool{}
	skipped := 0
	for _, f := range fences {
		f := f
		if reason := classifyReadmeFence(f); reason != "" {
			skipped++
			t.Logf("README:%d fence SKIPPED: %s", f.startLine, reason)
			continue
		}
		t.Run(fmt.Sprintf("fence_%d", f.startLine), func(t *testing.T) {
			runReadmeFence(t, repoRoot, bin, f, matchedAllowlist)
		})
	}
	t.Logf("README fence sweep: %d fences, %d executed, %d skipped (reasons in -v log)", len(fences), len(fences)-skipped, skipped)

	for key, entries := range readmeFenceAllowlist {
		if !matchedAllowlist[key] {
			t.Errorf("stale allowlist entry (key %q, entries %+v): no README fence carries that command any more — delete the entry or restore the documented refusal", key, entries)
		}
	}
}

func runReadmeFence(t *testing.T, repoRoot, bin string, f readmeFence, matchedAllowlist map[string]bool) {
	home, err := os.MkdirTemp("", "boardctl-readme-home-")
	if err != nil {
		t.Fatalf("temp home: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(home); err != nil {
			t.Logf("cleanup %s: %v", home, err)
		}
	})
	// EMPTY TMPDIR: boards are never under it, so the binary's
	// isTestEnvironment carve-out never applies (full gate enforcement).
	emptyTmp := filepath.Join(home, "empty-tmpdir")
	if err := os.MkdirAll(emptyTmp, 0o755); err != nil {
		t.Fatalf("empty tmpdir: %v", err)
	}
	env := &readmeEnv{t: t, home: home, bin: bin, emptyTmp: emptyTmp}

	// The crier hook examples sign with ~/.config/crier/boardctl.secret
	// (README operator notes: mode 0600). Materialize that documented
	// precondition inside the throwaway home so the real $HOME is never read.
	secret := filepath.Join(home, ".config", "crier", "boardctl.secret")
	if err := os.MkdirAll(filepath.Dir(secret), 0o700); err != nil {
		t.Fatalf("secret dir: %v", err)
	}
	if err := os.WriteFile(secret, []byte("fence-gate-secret"), 0o600); err != nil {
		t.Fatalf("secret file: %v", err)
	}

	// Map every root the fence names (via -C or cd) and set it up. The
	// /path/to/repo placeholder root in the types section is rewritten to a
	// directory under the throwaway home.
	key := fenceKey(f)
	cdRoot := ""
	var cmds []readmeCommand
	roots := map[string]bool{}
	for _, c := range f.commands {
		text := c.text
		if strings.Contains(text, "/path/to/repo") {
			text = strings.ReplaceAll(text, "/path/to/repo", env.rootFor("~/path/to/repo"))
		}
		if firstToken(text) == "cd" {
			cdRoot = env.rootFor(strings.Fields(text)[1])
			continue // directive, not a command to execute
		}
		root := env.cRoot(text)
		if root == "" {
			root = cdRoot
			if root == "" {
				root = home // bare commands (boardctl version) need no board
			}
		}
		if !roots[root] {
			roots[root] = true
			env.setupReadmeRoot(root)
		}
		cmds = append(cmds, readmeCommand{line: c.line, text: text})
	}

	// Seed rows this fence references but never creates (documented
	// preconditions; see readmeFenceSeeds).
	if ids, ok := readmeFenceSeeds[key]; ok {
		root := env.seedRoot(cmds)
		for _, id := range ids {
			seed := exec.Command(bin, "-C", root, "create", "--id", id, "--title", "Seed "+id)
			seed.Env = env.childEnv()
			if out, err := env.output(seed); err != nil {
				t.Fatalf("seed row %s in %s: %v\n%s", id, root, err, out)
			}
		}
	}

	// Fleet-rollout fixture: the BT-053 example rolls `--fleet 'axiom,crier'`
	// out under ~/src — materialize two boards under that root so the pattern
	// match and the per-repo hook install run for real (the first fleet
	// example's operator-home glob stays allowlisted; see readmeFenceAllowlist).
	for _, c := range cmds {
		if strings.Contains(c.text, "--fleet-root") {
			for _, repo := range []string{"axiom", "crier"} {
				env.setupReadmeRoot(env.rootFor("~/src/" + repo))
			}
			break
		}
	}

	// types.jsonl fixture: the types section's prose says to save the example
	// line as <boarddir>/types.jsonl before running the fence.
	for _, c := range cmds {
		if strings.Contains(c.text, " types --doctor") {
			for root := range roots {
				board := filepath.Join(root, ".coding-hermes", "board")
				if err := os.MkdirAll(board, 0o755); err != nil {
					t.Fatalf("board dir: %v", err)
				}
				line := `{"slug":"release","display":"Release","rev":1,"template":{"title_prefix":"[RELEASE] ","default_priority":"P1","ac_skeleton":"- [ ] Verify artifact"},"metadata":[{"name":"channel","type":"enum","required":true,"values":["stable","beta"],"description":"Release channel"},{"name":"owner","type":"string","required":true}]}`
				if err := os.WriteFile(filepath.Join(board, "types.jsonl"), []byte(line+"\n"), 0o644); err != nil {
					t.Fatalf("write types.jsonl: %v", err)
				}
			}
			break
		}
	}

	// Completion fixtures: any `update <id> --status complete …
	// --evidence-run-id R` needs a stored run whose verdict.json grades
	// exactly that task at the repo's HEAD (TR-251 + INFRA-LORE-045). This is
	// the fixture a real caller must have; the gate materializes it so the
	// README recipe itself is what is under test.
	for _, c := range cmds {
		if !strings.Contains(c.text, "--status complete") || !strings.Contains(c.text, "--evidence-run-id") {
			continue
		}
		id, runID := parseCompletionArgs(c.text)
		if id == "" || runID == "" {
			continue
		}
		root := env.seedRoot(cmds)
		history := filepath.Join(root, ".gitreins", "history", time.Now().Format("2006-01-02"), runID)
		if err := os.MkdirAll(history, 0o755); err != nil {
			t.Fatalf("run dir: %v", err)
		}
		verdict := fmt.Sprintf(`{"task_id":%q,"commit":%q,"passed":true}`+"\n", id, gitHead(t, env, root))
		if err := os.WriteFile(filepath.Join(history, "verdict.json"), []byte(verdict), 0o644); err != nil {
			t.Fatalf("verdict.json: %v", err)
		}
	}

	// export.json fixture: the import examples round-trip a board-report/v1
	// export (render --json). Materialize it so the documented import recipe
	// is what runs, not a missing-file refusal.
	for _, c := range cmds {
		if strings.Contains(c.text, "import export.json") {
			root := env.seedRoot(cmds)
			render := exec.Command(bin, "-C", root, "render", "-o", filepath.Join(home, "seed-report.html"), "--json", filepath.Join(root, "export.json"))
			render.Env = env.childEnv()
			if out, err := env.output(render); err != nil {
				t.Fatalf("seed export.json in %s: %v\n%s", root, err, out)
			}
			break
		}
	}

	// Execute the fence's commands in document order.
	for _, c := range cmds {
		norm := normalizeReadmeCommand(c.text)
		fullKey := key + "|" + norm
		cmd := exec.Command("sh", "-c", c.text)
		cmd.Dir = env.workDir(c.text, cdRoot, home)
		cmd.Env = env.childEnv()
		out, err := env.output(cmd)
		rc := 0
		if err != nil {
			var ee *exec.ExitError
			if !errors.As(err, &ee) {
				t.Fatalf("README:%d: command failed to start: %v", c.line, err)
			}
			rc = ee.ExitCode()
		}
		entries, allowlisted := readmeFenceAllowlist[fullKey]
		if rc == 0 {
			matchedAllowlist[fullKey] = true
			if allowlisted && !allowlistHasRc(entries, 0) {
				t.Errorf("README:%d: command now exits 0 but is allowlisted as a refusal — the guard it demonstrated may be gone:\n  cmd: %s", c.line, c.text)
			}
			continue
		}
		if allowlisted && allowlistMatches(entries, rc, out) {
			matchedAllowlist[fullKey] = true
			continue
		}
		t.Errorf("README:%d fence command FAILED (rc=%d) — README recipe drift or missing allowlist entry:\n  fence key: %q\n  cmd: %s\n  output:\n%s\n  if this refusal is intended, document it in readmeFenceAllowlist with the expected rc + error marker",
			c.line, rc, fullKey, c.text, indentLines(out))
	}
}

// parseCompletionArgs extracts (task id, evidence run id) from an
// `update <id> --status complete … --evidence-run-id R` command.
func parseCompletionArgs(text string) (id, runID string) {
	fields := strings.Fields(strings.ReplaceAll(text, "=", " = "))
	for i, f := range fields {
		switch {
		case f == "update" && i+1 < len(fields):
			id = fields[i+1]
		case f == "--evidence-run-id" && i+1 < len(fields):
			runID = fields[i+1]
		}
	}
	return id, runID
}

func gitHead(t *testing.T, env *readmeEnv, root string) string {
	t.Helper()
	out, err := env.output(exec.Command("git", "-C", root, "rev-parse", "HEAD"))
	if err != nil {
		t.Fatalf("git rev-parse HEAD in %s: %v\n%s", root, err, out)
	}
	return strings.TrimSpace(out)
}

func allowlistHasRc(entries []allowlistEntry, rc int) bool {
	for _, e := range entries {
		if e.rc == rc {
			return true
		}
	}
	return false
}

func allowlistMatches(entries []allowlistEntry, rc int, out string) bool {
	for _, e := range entries {
		if e.rc != rc {
			continue
		}
		if e.marker == "" || strings.Contains(out, e.marker) {
			return true
		}
	}
	return false
}

func indentLines(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = "    " + l
	}
	return strings.Join(lines, "\n")
}
