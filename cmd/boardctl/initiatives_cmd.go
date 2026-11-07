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
func cmdInitiatives(dir string, args []string) error {
	fs := newFlagSet("initiatives")
	args = reorderArgs(args, valueFlags("C", "status"))
	asJSON := fs.Bool("json", false, "emit the summary as JSON")
	onlyStatus := fs.String("status", "", "only list initiatives that have rows in this status")
	skipBad := useSkipBad(fs)
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "boardctl initiatives [--json] [--status S] [--skip-bad-lines] [-C dir]\n")
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
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	armSkipBad(b, skipBad)

	sums, err := b.Initiatives()
	if err != nil {
		return err
	}
	if *onlyStatus != "" {
		want := board.NormalizeStatus(*onlyStatus)
		if !board.StatusVocabulary[want] {
			return fmt.Errorf("--status %q not in vocabulary {%s}", *onlyStatus, vocabSet())
		}
		kept := sums[:0]
		for _, s := range sums {
			if s.ByStatus[want] > 0 {
				kept = append(kept, s)
			}
		}
		sums = kept
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
