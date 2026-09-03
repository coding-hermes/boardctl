// Command boardctl manages coding-hermes JSONL foreman boards (full CRUD).
//
// JSONL is the canonical git-tracked board store; board.db and *.parquet are
// untracked rebuildable caches and are NEVER written by boardctl.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coding-hermes/boardctl/internal/board"
)

const usageText = `boardctl — manage coding-hermes JSONL foreman boards

usage:
  boardctl [-C <repo-or-board-dir>] <command> [flags]

commands:
  list    [--status S] [--priority P] [--json] [--all]
  show    <id> [--events]
  create  --id ID --title T [--priority P2] [--complexity N] [--depends-on a,b]
          [--reasoning R] [--capability-tags a,b] [--status pending]
  update  <id> --status complete [--worker-status S] [--commit-hash SHA]
          [--guard PASS|FAIL|SKIP] [--ci GREEN|RED|SKIP] [--summary S]
          [--note S] [--blocked-reason R] [--completed-at TS]
  event   --type task_created|task_dispatched|task_completed|audit|...
          [--task-id ID] [--actor foreman] [--detail @file | --detail-text '...']
          [--tick N]
  header  [--json] [--set-ticks-total N] [--set-ticks-idle N] [--set-last-commit SHA]
  validate
  doctor
  stats   [--json] [--all]

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
	case "stats":
		err = cmdStats(boardDir, rest)
	case "help", "-h", "--help":
		fmt.Fprint(os.Stdout, usageText)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "boardctl: unknown command %q\n\n%s", cmd, usageText)
		return 2
	}
	if err != nil {
		var dup *board.ErrDuplicateTaskID
		if errors.As(err, &dup) {
			fmt.Fprintf(os.Stderr, "boardctl: %v\n", err)
			return 1
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
	args = reorderArgs(args, valueFlags("C", "id", "title", "status", "priority", "complexity", "depends-on", "reasoning", "capability-tags"))
	id := fs.String("id", "", "task id (required)")
	title := fs.String("title", "", "task title (required)")
	status := fs.String("status", "pending", "status (write vocabulary)")
	priority := fs.String("priority", "", "priority (default P2)")
	complexity := fs.String("complexity", "", "complexity (default 3)")
	dependsOn := fs.String("depends-on", "", "comma-separated dependency ids")
	reasoning := fs.String("reasoning", "", "reasoning note")
	capTags := fs.String("capability-tags", "", "comma-separated capability tags")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() { fmt.Fprintf(os.Stderr, "boardctl create --id ID --title T [flags] [-C dir]\n") }
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
		ID:           *id,
		Title:        *title,
		Status:       *status,
		Priority:     *priority,
		Reasoning:    *reasoning,
		HasDependsOn: *dependsOn != "",
		HasTags:      *capTags != "",
	}
	if spec.HasDependsOn {
		spec.DependsOn = splitCSV(*dependsOn)
	}
	if spec.HasTags {
		spec.CapabilityTags = splitCSV(*capTags)
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
	args = reorderArgs(args, valueFlags("C", "status", "worker-status", "commit-hash", "guard", "ci", "summary", "note", "blocked-reason", "completed-at"))
	status := fs.String("status", "", "status (write vocabulary)")
	workerStatus := fs.String("worker-status", "", "worker_status")
	commitHash := fs.String("commit-hash", "", "commit_hash")
	guard := fs.String("guard", "", "guard_result: PASS|FAIL|SKIP")
	ci := fs.String("ci", "", "ci_result: GREEN|RED|SKIP")
	summary := fs.String("summary", "", "worker_summary")
	note := fs.String("note", "", "foreman_note")
	blockedReason := fs.String("blocked-reason", "", "blocked_reason")
	completedAt := fs.String("completed-at", "", "completed_at timestamp")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() { fmt.Fprintf(os.Stderr, "boardctl update <id> --status complete [flags] [-C dir]\n") }
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
	if b.Topology != "A" {
		// topology B: header lives in board.db — read reports, writes fail.
		msg := "topology B: header lives in board.db (DuckDB cache) — use scripts/update header manually"
		if writing {
			return errors.New(msg)
		}
		if *asJSON {
			out, _ := json.Marshal(map[string]string{"topology": "B", "note": msg})
			fmt.Fprintln(os.Stdout, string(out))
		} else {
			fmt.Fprintf(os.Stderr, "boardctl: %s\n", msg)
		}
		return nil
	}
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
	fmt.Fprintf(os.Stdout, "header updated (line 1 of board.jsonl): %s\n", strings.Join(changed, ", "))
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
