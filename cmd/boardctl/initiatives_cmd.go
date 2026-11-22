package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/coding-hermes/boardctl/internal/board"
)

// cmdInitiatives answers the question the initiative key exists for: WHICH
// programmes of work are on this board, and how much of each is left?
//
// It is the discovery half of the scoop query. A foreman that is handed a task
// gets its initiative from the row; a foreman that is told "work the
// xml-spec-graph initiative" needs this list to confirm the slug exists before
// it scoops (a misspelled --initiative on `list` returns an empty set, which
// reads exactly like "no work left").
//
//	boardctl initiatives            # table: slug, total, actionable, deferred, by status
//	boardctl initiatives --json     # same data, machine readable
//	boardctl initiatives --status pending   # only initiatives with rows in that status
//
// Unscoped rows are NOT reported: absence of the key is not an initiative.
//
// BT-093: --umbrella extends the question across the BT-078 umbrella view —
// per-initiative totals aggregated over this repo's own board PLUS every
// child board reachable through .coding-hermes/links/, with the per-repo
// breakdown in each bucket. Read-only, reusing the umbrella read paths.
func cmdInitiatives(dir string, args []string) error {
	fs := newFlagSet("initiatives")
	args = reorderArgs(args, valueFlags("C", "status"))
	asJSON := fs.Bool("json", false, "emit the summary as JSON")
	onlyStatus := fs.String("status", "", "only list initiatives that have rows in this status")
	// BT-093: aggregate over the umbrella view instead of one board.
	umbrellaMode := fs.Bool("umbrella", false, "aggregate over the umbrella view: this repo's own board plus every child board under .coding-hermes/links (per-repo breakdown in each bucket; a child that fails to read is reported and excluded, exit 1)")
	skipBad := useSkipBad(fs)
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "boardctl initiatives [--umbrella] [--json] [--status S] [--skip-bad-lines] [-C dir]\n")
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("initiatives takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}
	if *umbrellaMode {
		return cmdInitiativesUmbrella(dir, *asJSON, onlyStatus, skipBad)
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	armSkipBad(b, skipBad)

	sums, err := b.Initiatives()
	if err != nil {
		return err
	}
	if filter, err := validateStatus(onlyStatus); err != nil {
		return err
	} else if filter != "" {
		sums = filterInitiativesByStatus(sums, filter)
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(sums)
	}
	if len(sums) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "no initiative-scoped rows on this board (scope one with: boardctl update <id> --initiative <slug>)")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "INITIATIVE\tTOTAL\tACTIONABLE\tDEFERRED\tBY STATUS")
	for _, s := range sums {
		var parts []string
		for _, st := range []string{"pending", "in_progress", "review", "blocked", "failed", "complete"} {
			if n := s.ByStatus[st]; n > 0 {
				parts = append(parts, fmt.Sprintf("%s=%d", st, n))
			}
		}
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%s\n", s.Initiative, s.Total, s.Actionable, s.Deferred, strings.Join(parts, " "))
	}
	return tw.Flush()
}

// validateStatus validates the --status value against the canonical status
// vocabulary (shared by the single-board and the BT-093 umbrella path);
// an empty value means no filter. The invalid spelling is a usage error
// naming the vocabulary — never a silent empty set.
func validateStatus(onlyStatus *string) (string, error) {
	if onlyStatus == nil || *onlyStatus == "" {
		return "", nil
	}
	want := board.NormalizeStatus(*onlyStatus)
	if !board.StatusVocabulary[want] {
		return "", fmt.Errorf("--status %q not in vocabulary {%s}", *onlyStatus, vocabSet())
	}
	return want, nil
}

// filterInitiativesByStatus keeps buckets with rows in the wanted status (the
// pre-BT-093 filter, generalized to both summary shapes' field access).
func filterInitiativesByStatus(sums []board.InitiativeSummary, want string) []board.InitiativeSummary {
	kept := sums[:0]
	for _, s := range sums {
		if s.ByStatus[want] > 0 {
			kept = append(kept, s)
		}
	}
	return kept
}

// cmdInitiativesUmbrella is the BT-093 aggregate path of `initiatives
// --umbrella`: per-initiative totals over the umbrella's OWN board plus every
// child board reachable through the BT-078 view, each bucket with its
// per-repo breakdown. Fail-closed discipline mirrored from `umbrella`:
//
//   - any error-class finding (a broken link, a child board that fails to
//     read) renders on stderr and the command exits 1 while the healthy
//     boards' totals still print — degraded output never reads as clean
//     success, and the excluded board's rows are absent from the totals;
//   - --skip-bad-lines arms the tolerant read on EVERY board in the view and
//     renders the SKIPPED-LINES evidence block, exit 1 when anything was
//     skipped (the DF-BOARDCTL-47 contract this file reuses verbatim).
func cmdInitiativesUmbrella(dir string, asJSON bool, onlyStatus *string, skipBad *bool) error {
	um, err := board.OpenUmbrella(dir)
	if err != nil {
		return err
	}
	if skipBad != nil && *skipBad {
		um.SetSkipBad()
	}
	aggs, readFindings, err := um.Initiatives()
	if err != nil {
		return err
	}
	filter, err := validateStatus(onlyStatus)
	if err != nil {
		return err
	}
	if filter != "" {
		kept := aggs[:0]
		for _, a := range aggs {
			if a.ByStatus[filter] > 0 {
				kept = append(kept, a)
			}
		}
		aggs = kept
	}

	// Findings = discovery findings (broken links, etc.) + aggregate read
	// findings (unreadable child boards), in view order.
	findings := append([]board.UmbrellaFinding{}, um.Findings...)
	findings = append(findings, readFindings...)

	if asJSON {
		out := struct {
			Root        string                     `json:"root"`
			Initiatives []board.UmbrellaInitiative `json:"initiatives"`
			Total       int                        `json:"total"`
			Actionable  int                        `json:"actionable"`
			Deferred    int                        `json:"deferred"`
			Findings    []board.UmbrellaFinding    `json:"findings"`
		}{Root: um.Root}
		for _, a := range aggs {
			out.Total += a.Total
			out.Actionable += a.Actionable
			out.Deferred += a.Deferred
		}
		out.Initiatives = aggs
		if out.Initiatives == nil {
			out.Initiatives = []board.UmbrellaInitiative{}
		}
		out.Findings = findings
		if out.Findings == nil {
			out.Findings = []board.UmbrellaFinding{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return err
		}
		return umbInitiativesExit(um, findings)
	}

	var total, actionable, deferred int
	for _, a := range aggs {
		total += a.Total
		actionable += a.Actionable
		deferred += a.Deferred
	}
	_, _ = fmt.Fprintf(os.Stdout, "UMBRELLA %s — %d initiative bucket(s), %d task(s): %d actionable, %d deferred\n",
		um.Root, len(aggs), total, actionable, deferred)
	if len(aggs) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "no initiative-scoped rows in the view (scope one with: boardctl update <id> --initiative <slug>)")
		return umbInitiativesExit(um, findings)
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "INITIATIVE\tTOTAL\tACTIONABLE\tDEFERRED\tBY STATUS\tREPOS")
	_, _ = fmt.Fprint(tw, board.RenderUmbrellaInitiatives(aggs))
	if err := tw.Flush(); err != nil {
		return err
	}
	return umbInitiativesExit(um, findings)
}

// umbInitiativesExit is the fail-closed exit gate shared by both emitters:
// findings and SKIPPED-LINES evidence ride stderr (stdout stays the totals /
// parseable JSON), any error-class finding fails the command, and a degraded
// read (--skip-bad-lines) exits 1 on its own. Mirrors umbrellaExit's shape.
func umbInitiativesExit(um *board.Umbrella, findings []board.UmbrellaFinding) error {
	nErr := 0
	for _, f := range findings {
		if f.Link != "" {
			_, _ = fmt.Fprintf(os.Stderr, "[%s] link %s: %s\n", f.Level, f.Link, f.Msg)
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "[%s] %s\n", f.Level, f.Msg)
		}
		if f.IsError() {
			nErr++
		}
	}
	skipped := len(um.SkippedLines())
	if skipped > 0 {
		_, _ = fmt.Fprint(os.Stderr, um.RenderSkippedLines())
	}
	if skipped > 0 {
		return fmt.Errorf("initiatives umbrella view incomplete or invalid: %d finding(s), %d skipped line(s) (degraded read: --skip-bad-lines kept every parseable row)", nErr, skipped)
	}
	if nErr > 0 {
		return fmt.Errorf("initiatives umbrella view incomplete or invalid: %d finding(s) — degraded totals must not read as a clean success", nErr)
	}
	return nil
}
