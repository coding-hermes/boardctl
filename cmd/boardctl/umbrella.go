package main

// BT-078: `boardctl umbrella` — a READ-ONLY unified view over an umbrella
// repo's own board plus every child board registered under
// .coding-hermes/links/. The command never writes: aggregation reuses the
// single-repo read paths (list/stats/validate), and the link-discovery
// findings ride the same degrade-with-evidence contract (stderr evidence +
// exit 1) as DF-BOARDCTL-9's skipped lines. Writes stay single-repo —
// create/update/event keep their -C semantics, and nothing in this file
// holds a Board write verb.
//
// Modes (mutually exclusive):
//
//	default (census)  one line per board: repo, task count, board dir
//	--list            aligned task rows with the REPO column (qualified ids)
//	--stats           per-repo stats + grand totals (same renderer as stats)
//	--validate        per-board validate reports + cross-board dep checks
//	--json            machine-readable census (implies nothing else)
//
// Cross-board depends_on resolution is reported, never guessed: a reference
// that resolves locally or uniquely cross-repo prints its repo-qualified
// target; an id that exists in several boards is named AMBIGUOUS with its
// candidates (an umbrella view must not silently bind relations to the wrong
// task).
//
// Exit codes (README contract): 0 ok; 1 any error-class finding (broken /
// looping / out-of-repo / non-board link), any failing per-board validate
// report, or (under --strict) any warning-class finding; 2 usage /
// board-not-found. Degraded but partially-successful discovery therefore
// reads as a failure, never as a clean success.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/coding-hermes/boardctl/internal/board"
)

func cmdUmbrella(dir string, args []string) error {
	fs := newFlagSet("umbrella")
	args = reorderArgs(args, valueFlags("C"))
	asJSON := fs.Bool("json", false, "emit the census as JSON (boards, findings, totals)")
	asList := fs.Bool("list", false, "list every task row across the boards, with the REPO column")
	asStats := fs.Bool("stats", false, "per-repo stats plus grand totals")
	asValidate := fs.Bool("validate", false, "run validate per board plus the cross-board depends_on checks")
	strict := fs.Bool("strict", false, "treat warning-level findings as failures (exit 1)")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "boardctl umbrella [--list|--stats|--validate] [--json] [--strict] [-C dir]\n")
		_, _ = fmt.Fprintf(fs.Output(), "\n  read-only unified view over one umbrella repo and every child board\n")
		_, _ = fmt.Fprintf(fs.Output(), "  symlinked under its .coding-hermes/links/ registry (one symlink per\n")
		_, _ = fmt.Fprintf(fs.Output(), "  child repo, named after the repo, aimed at <child>/.coding-hermes).\n")
		_, _ = fmt.Fprintf(fs.Output(), "  Tasks keep repo provenance; ids are repo-qualified (<repo>/<id>) and\n")
		_, _ = fmt.Fprintf(fs.Output(), "  cross-board depends_on references resolve only when unambiguous.\n")
		_, _ = fmt.Fprintf(fs.Output(), "  NEVER writes: broken/looping/out-of-repo links are reported (exit 1),\n")
		_, _ = fmt.Fprintf(fs.Output(), "  never followed or repaired. Writes stay single-repo via -C.\n")
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("umbrella takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}
	modes := 0
	for _, b := range []*bool{asList, asStats, asValidate} {
		if *b {
			modes++
		}
	}
	if modes > 1 {
		return fmt.Errorf("umbrella: --list, --stats and --validate are mutually exclusive: %w", errUsage)
	}
	if *asJSON && modes > 0 {
		return fmt.Errorf("umbrella: --json cannot be combined with --list/--stats/--validate: %w", errUsage)
	}
	um, err := board.OpenUmbrella(dir)
	if err != nil {
		return err
	}
	if *asJSON {
		return umbrellaJSON(um, *strict)
	}
	if *asList {
		return umbrellaList(um, *strict)
	}
	if *asStats {
		return umbrellaStats(um, *strict)
	}
	if *asValidate {
		return umbrellaValidate(um, *strict)
	}
	return umbrellaCensus(um, *strict)
}

// umbrellaExit maps the discovery findings + per-board validate results to
// the command's exit error: any error-class finding (or, under --strict, any
// warning) fails the command. stderr renders the findings block so a partial
// view can never read as clean success.
func umbrellaExit(um *board.Umbrella, strict bool, depFindings []board.UmbrellaFinding, validateErrs int) error {
	nErr := 0
	for _, f := range um.Findings {
		if f.IsError() {
			nErr++
		}
	}
	// Error-class dep findings (ambiguous cross-board refs) fail the view
	// unconditionally — like discovery errors, warnings only under --strict.
	for _, f := range depFindings {
		if f.IsError() {
			nErr++
		}
	}
	_, _ = fmt.Fprint(os.Stderr, um.RenderFindings())
	for _, f := range depFindings {
		_, _ = fmt.Fprintf(os.Stderr, "[%s] %s\n", f.Level, f.Msg)
	}
	if strict {
		nErr += um.WarnCount()
		for _, f := range depFindings {
			if !f.IsError() {
				nErr++
			}
		}
	}
	if nErr > 0 || validateErrs > 0 {
		return fmt.Errorf("umbrella view incomplete or invalid: %d finding(s), %d failing board(s)", nErr, validateErrs)
	}
	return nil
}

// umbrellaCensus renders the default one-line-per-board census.
func umbrellaCensus(um *board.Umbrella, strict bool) error {
	_, _ = fmt.Fprintf(os.Stdout, "UMBRELLA %s (%d board(s) in view)\n", um.Root, len(um.Boards))
	for _, qb := range um.Boards {
		rows, err := qb.Board.TaskRows()
		if err != nil {
			return fmt.Errorf("board %s (%s): %w", qb.Repo, qb.BoardDir, err)
		}
		_, _ = fmt.Fprintf(os.Stdout, "  %-20s %4d task(s)  topology %s  %s\n", qb.Repo, len(rows), qb.Topology, qb.BoardDir)
	}
	if len(um.Boards) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "  (no boards in view)")
	}
	return umbrellaExit(um, strict, nil, 0)
}

// umbrellaList renders the unified task table with the repo provenance
// column and repo-qualified ids.
func umbrellaList(um *board.Umbrella, strict bool) error {
	qts, err := um.ListTasks(board.TaskFilter{})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "%-20s %-16s %-4s %-11s %s\n", "REPO", "ID", "PRI", "STATUS", "TITLE")
	for _, qt := range qts {
		title := truncateRunes(qt.Row.String("title"), 100)
		prio := qt.Row.String("priority")
		status := board.NormalizeStatus(qt.Row.String("status"))
		_, _ = fmt.Fprintf(os.Stdout, "%-20s %-16s %-4s %-11s %s\n", qt.Repo, qt.Row.String("id"), prio, status, title)
	}
	if len(qts) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "no tasks in view")
	}
	return umbrellaExit(um, strict, nil, 0)
}

// umbrellaStats renders per-repo stats plus the grand totals.
func umbrellaStats(um *board.Umbrella, strict bool) error {
	us, err := um.Stats(board.TaskFilter{})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "umbrella: %s\n", us.Root)
	for _, rs := range us.Repos {
		_, _ = fmt.Fprintf(os.Stdout, "repo %s (%d tasks):\n", rs.Repo, rs.Stats.Total)
		_, _ = fmt.Fprint(os.Stdout, indentBlock(rs.Stats.RenderText(), "  "))
	}
	_, _ = fmt.Fprintln(os.Stdout, "umbrella total:")
	_, _ = fmt.Fprintf(os.Stdout, "  total tasks: %d\n", us.Total)
	_, _ = fmt.Fprintf(os.Stdout, "  actionable: %d\n", us.Actionable)
	_, _ = fmt.Fprintf(os.Stdout, "  deferred: %d\n", us.Deferred)
	_, _ = fmt.Fprintln(os.Stdout, "  by status:")
	for _, k := range board.SortedKeys(us.Status) {
		_, _ = fmt.Fprintf(os.Stdout, "    %-12s %d\n", k, us.Status[k])
	}
	_, _ = fmt.Fprintln(os.Stdout, "  by priority:")
	for _, k := range board.SortedKeys(us.Priority) {
		_, _ = fmt.Fprintf(os.Stdout, "    %-4s %d\n", k, us.Priority[k])
	}
	return umbrellaExit(um, strict, nil, 0)
}

// indentBlock re-indents every line of s by pad (for nesting one board's
// stats render under its repo header line).
func indentBlock(s, pad string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n") + "\n"
}

// umbrellaValidate runs per-board validate plus the cross-board depends_on
// classification, and reports ambiguity instead of guessing.
func umbrellaValidate(um *board.Umbrella, strict bool) error {
	reps, err := um.ValidateAll()
	if err != nil {
		return err
	}
	nErrs := 0
	for _, qr := range reps {
		_, _ = fmt.Fprintf(os.Stdout, "repo %s:\n", qr.Repo)
		_, _ = fmt.Fprint(os.Stdout, indentBlock(qr.Report.RenderText(), "  "))
		if qr.Report.HasErrors() {
			nErrs++
		}
	}
	// Cross-board relation resolution: printed on stdout (the resolved
	// bindings), findings on stderr via umbrellaExit.
	deps, depFindings, err := um.DepResolutions()
	if err != nil {
		return err
	}
	if len(deps) > 0 {
		_, _ = fmt.Fprintln(os.Stdout, "cross-board depends_on:")
		for _, d := range deps {
			switch d.Status {
			case "local":
				if len(d.AlsoIn) > 0 {
					_, _ = fmt.Fprintf(os.Stdout, "  %s -> %s (local; id also exists in %s)\n",
						qualifiedRef(d), board.QualifiedTaskID(d.ResolvedRepo, d.Dep), strings.Join(d.AlsoIn, ", "))
				} else {
					_, _ = fmt.Fprintf(os.Stdout, "  %s -> %s (local)\n", qualifiedRef(d), board.QualifiedTaskID(d.ResolvedRepo, d.Dep))
				}
			case "cross":
				_, _ = fmt.Fprintf(os.Stdout, "  %s -> %s (cross-repo)\n", qualifiedRef(d), board.QualifiedTaskID(d.ResolvedRepo, d.Dep))
			case "ambiguous":
				_, _ = fmt.Fprintf(os.Stdout, "  %s -> %q AMBIGUOUS (exists in %s) — not resolved\n",
					qualifiedRef(d), d.Dep, strings.Join(d.Candidates, ", "))
			case "dangling":
				_, _ = fmt.Fprintf(os.Stdout, "  %s -> %q DANGLING (not found in any board in the view)\n", qualifiedRef(d), d.Dep)
			}
		}
	}
	return umbrellaExit(um, strict, depFindings, nErrs)
}

// qualifiedRef renders the referencing side of a depends_on row.
func qualifiedRef(d board.DepResolution) string {
	return board.QualifiedTaskID(d.Repo, d.TaskID)
}

// umbrellaJSON emits the census as machine-readable JSON: boards, discovery
// findings, per-repo task counts and grand totals. Structured provenance for
// future tooling (the row's stable result surface).
func umbrellaJSON(um *board.Umbrella, strict bool) error {
	type jsonBoard struct {
		Repo     string `json:"repo"`
		RepoRoot string `json:"repo_root,omitempty"`
		BoardDir string `json:"board_dir"`
		Topology string `json:"topology"`
		Tasks    int    `json:"tasks"`
	}
	out := struct {
		Root     string                  `json:"root"`
		LinksDir string                  `json:"links_dir,omitempty"`
		Boards   []jsonBoard             `json:"boards"`
		Findings []board.UmbrellaFinding `json:"findings"`
		Total    int                     `json:"total"`
	}{Root: um.Root, LinksDir: um.LinksDir}
	for _, qb := range um.Boards {
		rows, err := qb.Board.TaskRows()
		if err != nil {
			return fmt.Errorf("board %s (%s): %w", qb.Repo, qb.BoardDir, err)
		}
		out.Boards = append(out.Boards, jsonBoard{
			Repo: qb.Repo, RepoRoot: qb.RepoRoot, BoardDir: qb.BoardDir, Topology: qb.Topology, Tasks: len(rows),
		})
		out.Total += len(rows)
	}
	if out.Boards == nil {
		out.Boards = []jsonBoard{}
	}
	out.Findings = um.Findings
	if out.Findings == nil {
		out.Findings = []board.UmbrellaFinding{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, buf.Bytes(), "", "  "); err == nil {
		_, _ = os.Stdout.Write(pretty.Bytes())
		_, _ = os.Stdout.Write([]byte("\n"))
	} else {
		_, _ = os.Stdout.Write(buf.Bytes())
	}
	return umbrellaExit(um, strict, nil, 0)
}
