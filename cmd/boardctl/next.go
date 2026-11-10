package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/coding-hermes/boardctl/internal/board"
)

var errNextNoWork = errors.New("no dispatchable work: all complete")

// cmdNext prints the single highest-priority row that a foreman can dispatch.
// Rows retain their tasks.jsonl order for ties, which is the board's
// chronological order.
func cmdNext(dir string, args []string) error {
	fs := newFlagSet("next")
	args = reorderArgs(args, valueFlags("C"))
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "boardctl next -C dir\n")
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("next takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}

	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	report, err := b.AnalyzeDispatchability()
	if err != nil {
		return err
	}

	type candidate struct {
		row   *board.Row
		order int
	}
	orderByID := make(map[string]int, len(report.Rows))
	for order, row := range report.Rows {
		orderByID[row.String("id")] = order
	}
	candidates := make([]candidate, 0, len(report.Ready))
	for _, row := range report.Ready {
		candidates = append(candidates, candidate{row: row, order: orderByID[row.String("id")]})
	}
	deferred, blocked := 0, 0
	for _, row := range report.Rows {
		status := board.NormalizeStatus(row.String("status"))
		if status == "complete" || status == "failed" {
			continue
		}
		if board.RowIsDeferred(row) && status == "pending" {
			deferred++
		} else if !report.IsReady(row) {
			blocked++
		}
	}
	if len(candidates) == 0 {
		switch {
		case deferred > 0 && blocked == 0:
			return fmt.Errorf("%w: all deferred", errNextNoWork)
		case blocked > 0:
			return fmt.Errorf("%w: all blocked", errNextNoWork)
		default:
			return errNextNoWork
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		pi := nextPriority(candidates[i].row.String("priority"))
		pj := nextPriority(candidates[j].row.String("priority"))
		if pi != pj {
			return pi < pj
		}
		return candidates[i].order < candidates[j].order
	})
	row := candidates[0].row
	fmt.Printf("id: %s\npriority: %s\ntitle: %s\nacceptance_criteria: %s\n",
		row.String("id"), row.String("priority"), row.String("title"), row.String("acceptance_criteria"))
	return nil
}

func nextPriority(priority string) int {
	priority = strings.ToUpper(strings.TrimSpace(priority))
	if len(priority) == 2 && priority[0] == 'P' && priority[1] >= '0' && priority[1] <= '5' {
		return int(priority[1] - '0')
	}
	return 6
}
