// Command boardctl manages coding-hermes JSONL foreman boards (full CRUD).
//
// JSONL is the canonical git-tracked board store; any .db or *.parquet files
// are untracked rebuildable caches and are NEVER written by boardctl.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/coding-hermes/boardctl/internal/board"
)

// version is stamped at release time via the Makefile release target:
//
//	go build -ldflags "-X main.version=$(VERSION)" ./cmd/boardctl
//
// Unstamped builds (go build / go install) report "dev".
var version = "dev"

const usageText = `boardctl — manage coding-hermes JSONL foreman boards

usage:
  boardctl [-C <repo-or-board-dir>] <command> [flags]

commands:
  init    [-C dir] [--project P] [--namespace NS]   bootstrap a fresh board
  list    [--status S] [--priority P] [--json] [--all]
  show    <id> [--events]
  create  --id ID --title T [--priority P2] [--complexity N] [--depends-on a,b]
          [--reasoning R] [--capability-tags a,b] [--status pending] [--force]
          [--evidence-run-id RUN]   (a re-detected finding is REFUSED with exit 2)
  update  <id> --status complete [--worker-status S] [--commit-hash SHA]
          [--guard PASS|FAIL|SKIP] [--ci GREEN|RED|SKIP] [--summary S]
          [--note S] [--blocked-reason R] [--completed-at TS] [--normalize]
          [--force]
  event   --type task_created|task_dispatched|task_completed|audit|...
          [--task-id ID] [--actor foreman] [--detail @file | --detail-text '...']
          [--tick N]
  header  [--json] [--set-ticks-total N] [--set-ticks-idle N] [--set-last-commit SHA]
  validate
  doctor
  version [--json]
  stats   [--json] [--all]
  render  [-C dir] [-o out.html] [--tz Zone] [--json out.json]
  import  <export.json> [--dry-run] [--renumber]
  serve   [-C dir] [--addr 127.0.0.1:8787]

-C resolves the board dir: a repo root (looks for .coding-hermes/board),
.coding-hermes, or the board dir itself. Defaults to the current directory.
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	boardDir := ""
	// Global -C may appear before the subcommand: `boardctl -C dir list ...`
	// and `boardctl -C=dir list ...`. Stop the pre-scan at the first other arg.
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-C" && i+1 < len(args):
			boardDir = args[i+1]
			i += 2
			continue
		case strings.HasPrefix(a, "-C="):
			boardDir = strings.TrimPrefix(a, "-C=")
			i++
			continue
		}
		break
	}
	args = args[i:]
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usageText)
		return 2
	}
	cmd, rest := args[0], args[1:]

	var err error
	switch cmd {
	case "init":
		err = cmdInit(boardDir, rest)
	case "list":
		err = cmdList(boardDir, rest)
	case "show":
		err = cmdShow(boardDir, rest)
	case "create":
		err = cmdCreate(boardDir, rest)
	case "update":
		err = cmdUpdate(boardDir, rest)
	case "event":
		err = cmdEvent(boardDir, rest)
	case "header":
		err = cmdHeader(boardDir, rest)
	case "validate":
		err = cmdValidate(boardDir, rest)
	case "doctor":
		err = cmdDoctor(boardDir, rest)
	case "version":
		err = cmdVersion(rest)
	case "stats":
		err = cmdStats(boardDir, rest)
	case "render":
		err = cmdRender(boardDir, rest)
	case "import":
		err = cmdImport(boardDir, rest)
	case "serve":
		err = cmdServe(boardDir, rest)
	case "help", "-h", "--help":
		fmt.Fprint(os.Stdout, usageText)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "boardctl: unknown command %q\n\n%s", cmd, usageText)
		return 2
	}
	if err != nil {
		// Board-not-found and flag-level usage failures are usage-level
		// failures (README exit-code contract: "2 usage/board-not-found").
		// openBoard wraps ErrBoardNotFound with a hint, so match the chain;
		// serve's non-loopback --addr refusal arrives as errUsage.
		if errors.Is(err, board.ErrBoardNotFound) || errors.Is(err, errUsage) {
			fmt.Fprintf(os.Stderr, "boardctl: %v\n", err)
			return 2
		}
		var dup *board.ErrDuplicateTaskID
		if errors.As(err, &dup) {
			fmt.Fprintf(os.Stderr, "boardctl: %v\n", err)
			return 1
		}
		// SG-126: a suppressed duplicate finding is a DISPATCHER-detectable
		// outcome, not a plain failure — print the machine-readable line bare
		// (no "boardctl:" prefix) and use exit 2 so a lane can branch on it.
		var dupFinding *board.ErrDuplicateFinding
		if errors.As(err, &dupFinding) {
			fmt.Fprintf(os.Stderr, "%v\n", dupFinding)
			return 2
		}
		fmt.Fprintf(os.Stderr, "boardctl: %v\n", err)
		return 1
	}
	return 0
}

// openBoard resolves -C and prints a friendly error when resolution fails.
func openBoard(target string) (*board.Board, error) {
	b, err := board.Resolve(target)
	if err != nil {
		if errors.Is(err, board.ErrBoardNotFound) {
			// BT-026: when the wrap identifies a partial board (a detected
			// tasks.jsonl/events.jsonl whose pair is missing) the generic
			// location hint is noise — the specific diagnosis is the hint.
			if strings.Contains(err.Error(), " is missing)") {
				return nil, err
			}
			return nil, fmt.Errorf("%w\nhint: pass a repo root (-C <repo>), a .coding-hermes dir, or a board dir containing tasks.jsonl+events.jsonl", err)
		}
		return nil, err
	}
	return b, nil
}

// addCFlag registers -C on a subcommand flagset so `boardctl list -C dir`
// also works.
func addCFlag(fs *flag.FlagSet, target *string) {
	fs.StringVar(target, "C", *target, "repo root or board dir")
}

func newFlagSet(cmd string) *flag.FlagSet {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

// reorderArgs moves all flags ahead of positional arguments so commands like
// `boardctl show <id> --events` and `boardctl update <id> --status complete`
// parse correctly (Go's flag package stops at the first non-flag token).
// takesValue reports whether a flag name (without dashes) consumes an
// argument; boolean flags never do.
func reorderArgs(args []string, takesValue func(string) bool) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				continue
			}
			if takesValue(name) && i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		pos = append(pos, a)
	}
	return append(flags, pos...)
}

// valueFlags builds a takesValue closure from a set of flag names.
func valueFlags(names ...string) func(string) bool {
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	return func(n string) bool { return set[n] }
}

// stringListFlag is a REPEATABLE string flag: each occurrence appends one
// value, so `--session A --session B` collects [A B] instead of overwriting.
// String() renders the collected values comma-joined, which keeps an unset
// flag reading as "" — the normalize-combination gate in cmdUpdate tests
// flags with Value.String() != "".
type stringListFlag []string

func (s *stringListFlag) String() string { return strings.Join(*s, ",") }

func (s *stringListFlag) Set(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return fmt.Errorf("value must not be empty")
	}
	*s = append(*s, v)
	return nil
}

// parseIntFlag validates a CLI int.
func parseIntFlag(name, v string) (int64, error) {
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not an integer", name, v)
	}
	return n, nil
}

func splitCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---------- init ----------

// cmdInit bootstraps a fresh topology-A board (tasks.jsonl + events.jsonl +
// board.jsonl header) so `create` works on a brand-new project. Idempotent
// and no-clobber: existing board files are never overwritten; a fully
// initialized board reports "already initialized" and exits 0. Init writes
// board files only — no git init, no commits.
func cmdInit(dir string, args []string) error {
	fs := newFlagSet("init")
	args = reorderArgs(args, valueFlags("C", "project", "namespace"))
	project := fs.String("project", "", "project name for the board.jsonl header (default: target dir basename)")
	namespace := fs.String("namespace", "", "namespace for the board.jsonl header (default: project)")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "boardctl init [-C dir] [--project P] [--namespace NS]\n")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("init takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}
	boardDir, wrote, err := board.Init(dir, board.InitOptions{Project: *project, Namespace: *namespace})
	if err != nil {
		if errors.Is(err, board.ErrAlreadyInitialized) {
			fmt.Fprintf(os.Stdout, "already initialized: %s\n", boardDir)
			return nil
		}
		return err
	}
	fmt.Fprintf(os.Stdout, "initialized board at %s\n", boardDir)
	for _, p := range wrote {
		fmt.Fprintf(os.Stdout, "  wrote %s\n", filepath.Base(p))
	}
	fmt.Fprintln(os.Stdout, "next: boardctl create --id TASK-1 --title \"First task\"")
	return nil
}

// ---------- list ----------

func cmdList(dir string, args []string) error {
	fs := newFlagSet("list")
	args = reorderArgs(args, valueFlags("C", "status", "priority"))
	status := fs.String("status", "", "filter by status")
	priority := fs.String("priority", "", "filter by priority (e.g. P1)")
	asJSON := fs.Bool("json", false, "emit JSON array of full rows")
	all := fs.Bool("all", false, "include fixture rows (ids in fixtures.jsonl)")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "boardctl list [--status S] [--priority P] [--json] [--all] [-C dir]\n")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("list takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	f := board.TaskFilter{All: *all}
	if *status != "" {
		norm := board.NormalizeStatus(*status)
		if !board.StatusVocabulary[norm] {
			return fmt.Errorf("--status %q not in vocabulary {%s} ('completed' accepted as alias)", *status, vocabSet())
		}
		f.Status = norm
	}
	if *priority != "" {
		f.Priority = *priority
	}
	rows, err := b.ListTasks(f)
	if err != nil {
		return err
	}
	if *asJSON {
		var buf bytes.Buffer
		buf.WriteString("[")
		for i, r := range rows {
			if i > 0 {
				buf.WriteString(",")
			}
			buf.Write(board.RowJSONCompact(r))
		}
		buf.WriteString("]\n")
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, buf.Bytes(), "", "  "); err == nil {
			os.Stdout.Write(pretty.Bytes())
			os.Stdout.Write([]byte("\n"))
		} else {
			os.Stdout.Write(buf.Bytes())
		}
		return nil
	}
	if len(rows) == 0 {
		fmt.Fprintln(os.Stderr, "no tasks match")
		return nil
	}
	// Aligned text columns: id, priority, status, truncated title.
	fmt.Fprintln(os.Stdout, fmt.Sprintf("%-16s %-4s %-11s %s", "ID", "PRI", "STATUS", "TITLE"))
	for _, r := range rows {
		title := r.String("title")
		title = truncateRunes(title, 100)
		prio := r.String("priority")
		status := board.NormalizeStatus(r.String("status"))
		fmt.Fprintln(os.Stdout, fmt.Sprintf("%-16s %-4s %-11s %s", r.String("id"), prio, status, title))
	}
	return nil
}

func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

func vocabSet() string {
	return "pending,in_progress,review,blocked,complete,failed"
}

// ---------- show ----------

func cmdShow(dir string, args []string) error {
	fs := newFlagSet("show")
	args = reorderArgs(args, valueFlags("C"))
	withEvents := fs.Bool("events", false, "also print events for the task")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() { fmt.Fprintf(os.Stderr, "boardctl show <id> [--events] [-C dir]\n") }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if dir == "" {
		dir = cdir
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("show requires exactly one task id")
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	id := fs.Arg(0)
	row, file, err := b.ShowTask(id)
	if err != nil {
		return err
	}
	if row == nil {
		return fmt.Errorf("task %q not found (searched tasks.jsonl and fixtures.jsonl)", id)
	}
	pretty, err := board.MarshalRowJSON(row)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s\n", pretty)
	if file != b.TasksPath() {
		fmt.Fprintf(os.Stderr, "note: row found in %s (fixture row)\n", filepath.Base(file))
	}
	if *withEvents {
		evs, err := b.EventsForTask(id)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "events for %s (%d):\n", id, len(evs))
		for _, e := range evs {
			raw := board.RowJSONCompact(e)
			os.Stdout.Write(raw)
			os.Stdout.Write([]byte("\n"))
		}
	}
	return nil
}

// ---------- create ----------

func cmdCreate(dir string, args []string) error {
	fs := newFlagSet("create")
	args = reorderArgs(args, valueFlags("C", "id", "title", "status", "priority", "complexity", "depends-on", "reasoning", "capability-tags", "evidence-run-id", "worktree", "branch", "session"))
	id := fs.String("id", "", "task id (required)")
	title := fs.String("title", "", "task title (required)")
	status := fs.String("status", "pending", "status (write vocabulary)")
	priority := fs.String("priority", "", "priority (default P2)")
	complexity := fs.String("complexity", "", "complexity (default 3)")
	dependsOn := fs.String("depends-on", "", "comma-separated dependency ids")
	reasoning := fs.String("reasoning", "", "reasoning note")
	capTags := fs.String("capability-tags", "", "comma-separated capability tags")
	evidenceRunID := fs.String("evidence-run-id", "", "run identifier recorded as evidence (SG-126)")
	// BT-037: build-location + session audit fields. Omitted flags write
	// NOTHING (no empty worktree/branch, no empty sessions array) — a row
	// built in the main checkout simply carries no worktree/branch keys.
	worktree := fs.String("worktree", "", "absolute path of the git worktree this task is built in (omit = main checkout)")
	branch := fs.String("branch", "", "git branch of --worktree, e.g. wt/cht-031")
	var sessions stringListFlag
	fs.Var(&sessions, "session", "Hermes session id that worked this task (repeatable: --session A --session B records the ordered array)")
	force := fs.Bool("force", false, "write the id even if it violates the fleet id format (also files a duplicate finding as a variant)")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "boardctl create --id ID --title T [--worktree PATH] [--branch NAME] [--session ID]... [--force] [flags] [-C dir]\\n")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("create takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	spec := board.TaskRowSpec{
		ID:            *id,
		Title:         *title,
		Status:        *status,
		Priority:      *priority,
		Reasoning:     *reasoning,
		HasDependsOn:  *dependsOn != "",
		HasTags:       *capTags != "",
		Force:         *force,
		EvidenceRunID: *evidenceRunID,
	}
	if spec.HasDependsOn {
		spec.DependsOn = splitCSV(*dependsOn)
	}
	if spec.HasTags {
		spec.CapabilityTags = splitCSV(*capTags)
	}
	if *worktree != "" {
		spec.Worktree = worktree
	}
	if *branch != "" {
		spec.Branch = branch
	}
	if len(sessions) > 0 {
		list := []string(sessions)
		spec.Sessions = &list
	}
	if *complexity != "" {
		n, err := parseIntFlag("--complexity", *complexity)
		if err != nil {
			return err
		}
		spec.Complexity = &n
	}
	created, err := b.Create(spec)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "created task %s (appended to %s)\n", created, b.TasksPath())
	return nil
}

// ---------- update ----------

func cmdUpdate(dir string, args []string) error {
	fs := newFlagSet("update")
	args = reorderArgs(args, valueFlags("C", "status", "worker-status", "commit-hash", "guard", "ci", "summary", "note", "blocked-reason", "completed-at", "worktree", "branch", "session"))
	status := fs.String("status", "", "status (write vocabulary)")
	workerStatus := fs.String("worker-status", "", "worker_status")
	commitHash := fs.String("commit-hash", "", "commit_hash")
	guard := fs.String("guard", "", "guard_result: PASS|FAIL|SKIP")
	ci := fs.String("ci", "", "ci_result: GREEN|RED|SKIP")
	summary := fs.String("summary", "", "worker_summary")
	note := fs.String("note", "", "foreman_note")
	blockedReason := fs.String("blocked-reason", "", "blocked_reason")
	completedAt := fs.String("completed-at", "", "completed_at timestamp")
	// BT-037: build-location + session audit fields. An omitted flag leaves
	// the row's key untouched (and never creates it); --session REPLACES the
	// whole sessions array with the ids given here, in order.
	worktree := fs.String("worktree", "", "absolute path of the git worktree this task is built in (omit = leave untouched)")
	branch := fs.String("branch", "", "git branch of --worktree, e.g. wt/cht-031")
	var sessions stringListFlag
	fs.Var(&sessions, "session", "Hermes session id that worked this task (repeatable; REPLACES the row's sessions array with the ids given)")
	// BT-025: bool flag — rewrite status/guard_result/ci_result on the row
	// to their canonical forms (read-alias fix path). Takes no value, so it
	// must NOT join the reorderArgs valueFlags list.
	normalize := fs.Bool("normalize", false, "rewrite status/guard_result/ci_result to canonical forms (read-alias fix path)")
	force := fs.Bool("force", false, "allow updating rows whose id violates the fleet id format")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "boardctl update <id> [--status complete] [--worktree PATH] [--branch NAME] [--session ID]... [--normalize] [--force] [flags] [-C dir]\\n")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if dir == "" {
		dir = cdir
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("update requires exactly one task id")
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	// BT-025: --normalize is the sanctioned read-alias fix path — it owns
	// the whole command when present, because it already rewrites exactly
	// the fields (status, guard_result, ci_result) an explicit combination
	// would target, and mixing it with e.g. --status would make the outcome
	// order-dependent. Any other change flag alongside it is a usage error;
	// --normalize alone satisfies the change-flag gate and needs no --force
	// beyond the fleet-id escape hatch.
	if *normalize {
		for _, f := range []string{"status", "worker-status", "commit-hash", "guard", "ci", "summary", "note", "blocked-reason", "completed-at", "worktree", "branch", "session"} {
			if fs.Lookup(f).Value.String() != "" {
				return fmt.Errorf("--normalize cannot be combined with --%s (it already canonicalizes status/guard_result/ci_result)", f)
			}
		}
		changes, err := b.NormalizeTask(fs.Arg(0), *force)
		if err != nil {
			return err
		}
		if len(changes) == 0 {
			fmt.Fprintf(os.Stdout, "task %s already canonical — nothing to rewrite (file untouched)\n", fs.Arg(0))
			return nil
		}
		for _, ch := range changes {
			fmt.Fprintf(os.Stdout, "normalized task %s: %s %q -> %q\n", fs.Arg(0), ch.Field, ch.Old, ch.New)
		}
		return nil
	}
	ptr := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	spec := board.UpdateSpec{
		Status:        ptr(*status),
		WorkerStatus:  ptr(*workerStatus),
		CommitHash:    ptr(*commitHash),
		Guard:         ptr(*guard),
		CI:            ptr(*ci),
		Summary:       ptr(*summary),
		Note:          ptr(*note),
		BlockedReason: ptr(*blockedReason),
		CompletedAt:   ptr(*completedAt),
		Force:         *force,
	}
	if *worktree != "" {
		spec.Worktree = worktree
	}
	if *branch != "" {
		spec.Branch = branch
	}
	if len(sessions) > 0 {
		list := []string(sessions)
		spec.Sessions = &list
	}
	changed, err := b.UpdateTask(fs.Arg(0), spec)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "updated task %s: %s\n", fs.Arg(0), strings.Join(changed, ", "))
	return nil
}

// ---------- event ----------

func cmdEvent(dir string, args []string) error {
	fs := newFlagSet("event")
	args = reorderArgs(args, valueFlags("C", "type", "task-id", "actor", "detail", "detail-text", "tick"))
	etype := fs.String("type", "", "event_type (default audit)")
	taskID := fs.String("task-id", "", "task_id")
	actor := fs.String("actor", "", "actor (default foreman)")
	detailFile := fs.String("detail", "", "detail JSON payload: @/path/file.json")
	detailText := fs.String("detail-text", "", "detail plain text")
	tick := fs.String("tick", "", "tick_number")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() { fmt.Fprintf(os.Stderr, "boardctl event --type audit [flags] [-C dir]\n") }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("event takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}
	if *detailFile != "" && *detailText != "" {
		return fmt.Errorf("--detail and --detail-text are mutually exclusive")
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	spec := board.EventSpec{Type: *etype, TaskID: *taskID, Actor: *actor}
	if *detailFile != "" {
		p := *detailFile
		if strings.HasPrefix(p, "@") {
			p = strings.TrimPrefix(p, "@")
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("--detail: %w", err)
		}
		if !json.Valid(bytes.TrimSpace(content)) {
			return fmt.Errorf("--detail: %s does not contain valid JSON", p)
		}
		spec.Detail = content
	}
	if *detailText != "" {
		spec.DetailText = detailText
	}
	if *tick != "" {
		n, err := parseIntFlag("--tick", *tick)
		if err != nil {
			return err
		}
		spec.Tick = &n
	}
	id, err := b.AppendEvent(spec)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "event id %d appended to %s\n", id, b.EventsPath())
	return nil
}

// ---------- header ----------

func cmdHeader(dir string, args []string) error {
	fs := newFlagSet("header")
	args = reorderArgs(args, valueFlags("C", "set-ticks-total", "set-ticks-idle", "set-last-commit"))
	asJSON := fs.Bool("json", false, "emit header as JSON")
	setTotal := fs.String("set-ticks-total", "", "set ticks_total counter")
	setIdle := fs.String("set-ticks-idle", "", "set ticks_idle counter")
	setCommit := fs.String("set-last-commit", "", "set last_commit sha")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "boardctl header [--json] [--set-ticks-total N] [--set-ticks-idle N] [--set-last-commit SHA] [-C dir]\n")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("header takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	writing := *setTotal != "" || *setIdle != "" || *setCommit != ""
	// BT-010: no topology special case — HeaderRow reads board.jsonl (A) or
	// line 1 of tasks.jsonl (B), and SetHeader rewrites whichever carries
	// the header. Both read and write work identically on either topology.
	row, err := b.HeaderRow()
	if err != nil {
		return err
	}
	if !writing {
		if *asJSON {
			os.Stdout.Write(board.RowJSONCompact(row))
			os.Stdout.Write([]byte("\n"))
			return nil
		}
		pretty, err := board.MarshalRowJSON(row)
		if err != nil {
			return err
		}
		os.Stdout.Write(pretty)
		os.Stdout.Write([]byte("\n"))
		return nil
	}
	u := board.HeaderUpdate{}
	if *setTotal != "" {
		n, err := parseIntFlag("--set-ticks-total", *setTotal)
		if err != nil {
			return err
		}
		u.TicksTotal = &n
	}
	if *setIdle != "" {
		n, err := parseIntFlag("--set-ticks-idle", *setIdle)
		if err != nil {
			return err
		}
		u.TicksIdle = &n
	}
	if *setCommit != "" {
		u.LastCommit = setCommit
	}
	changed, err := b.SetHeader(u)
	if err != nil {
		return err
	}
	where := "line 1 of board.jsonl"
	if b.Topology != "A" {
		where = "line 1 of tasks.jsonl"
	}
	fmt.Fprintf(os.Stdout, "header updated (%s): %s\n", where, strings.Join(changed, ", "))
	return nil
}

// ---------- validate ----------

func cmdValidate(dir string, args []string) error {
	fs := newFlagSet("validate")
	args = reorderArgs(args, valueFlags("C"))
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() { fmt.Fprintf(os.Stderr, "boardctl validate [-C dir]\n") }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if dir == "" {
		dir = cdir
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	rep, err := b.Validate()
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stdout, rep.RenderText())
	if rep.HasErrors() {
		return errors.New("validation failed")
	}
	return nil
}

// ---------- doctor ----------

func cmdDoctor(dir string, args []string) error {
	fs := newFlagSet("doctor")
	args = reorderArgs(args, valueFlags("C"))
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() { fmt.Fprintf(os.Stderr, "boardctl doctor [-C dir]\n") }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("doctor takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	rep, err := b.Doctor()
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stdout, rep.RenderText())
	if rep.HasErrors() {
		return errors.New("doctor found errors")
	}
	return nil
}

// ---------- version ----------

// moduleVersionRe matches exactly a released semver tag (v0.1.3). It
// deliberately rejects the OTHER shapes buildinfo carries: pseudo-versions
// (`v0.1.4-0.<date>-<sha>+dirty`, what a plain `go build` embeds — they are
// semver-prefixed but are not release tags), "(devel)", date stamps, and ""
// (`go test` binaries and file-path builds carry no main module).
var moduleVersionRe = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

// moduleVersion reads the main module version the Go toolchain embeds at
// link time (the same one `go version -m` reports), verbatim: "" when the
// build carries no main module at all. It is a package-level var so tests
// can stub it; CLASSIFICATION happens in releaseVersion via moduleVersionRe,
// so a stub can never smuggle a non-tag shape past the gate.
var moduleVersion = func() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return bi.Main.Version
}

// releaseVersion returns the release identity to report, by precedence:
// an explicit tag-shaped stamp (make release VERSION=vX.Y.Z on a tagged
// checkout — the acceptance override) wins; else the toolchain-embedded
// module tag (a tagged-checkout build or `go install pkg@vX.Y.Z`); else the
// explicit stamp as-is (the date from an untagged checkout); else "dev".
func releaseVersion() string {
	if moduleVersionRe.MatchString(version) {
		return version
	}
	if mv := moduleVersion(); moduleVersionRe.MatchString(mv) {
		return mv
	}
	if version != "" && version != "dev" {
		return version
	}
	return "dev"
}

// cmdVersion prints the release identity. The Makefile release target
// injects it via -ldflags "-X main.version=..." (a vX.Y.Z tag on a tagged
// checkout, a UTC date on an untagged one); unstamped builds fall back to
// the toolchain-embedded module version when it is a real vX.Y.Z (so
// `go install ...@v0.1.3` reports v0.1.3), and to "dev" otherwise.
//
// --json emits {"version": <identity>, "build": <main.version stamp>} for
// scripts; the human line adds "(build <stamp>)" only when the build stamp
// differs from the reported identity, so the unstamped path keeps its
// single-value output.
func cmdVersion(args []string) error {
	fs := newFlagSet("version")
	asJSON := fs.Bool("json", false, "emit version info as JSON")
	fs.Usage = func() { fmt.Fprintf(os.Stderr, "boardctl version [--json]\n") }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("version takes no positional args (got %q)", fs.Arg(0))
	}
	v := releaseVersion()
	if *asJSON {
		b, err := json.Marshal(struct {
			Version string `json:"version"`
			Build   string `json:"build"`
		}{Version: v, Build: version})
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "%s\n", b)
		return nil
	}
	// The parenthetical only appears when the raw stamp adds information
	// beyond the identity (e.g. "v0.1.3 (build 20260915)"); when the stamp
	// IS the identity — or absent — the line stays single-value.
	if version != "" && version != "dev" && version != v {
		fmt.Fprintf(os.Stdout, "boardctl version %s (build %s)\n", v, version)
		return nil
	}
	fmt.Fprintf(os.Stdout, "boardctl version %s\n", v)
	return nil
}

// ---------- stats ----------

func cmdStats(dir string, args []string) error {
	fs := newFlagSet("stats")
	args = reorderArgs(args, valueFlags("C"))
	asJSON := fs.Bool("json", false, "emit stats as JSON")
	all := fs.Bool("all", false, "include fixture rows")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() { fmt.Fprintf(os.Stderr, "boardctl stats [--json] [--all] [-C dir]\n") }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("stats takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	st, err := b.ComputeStats(board.TaskFilter{All: *all})
	if err != nil {
		return err
	}
	if *asJSON {
		out, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(out))
		return nil
	}
	fmt.Fprint(os.Stdout, st.RenderText())
	return nil
}
