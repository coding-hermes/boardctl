package main

// BT-099 — `decompose`: scaffold N child rows from a parent as template.
//
// Default is PARALLEL (independent children — a wave can absorb N workers);
// --sequential is the opt-in dep-chained form and MUST print the cost line
// "wave absorbs 1 worker (sequential chain — use --parallel for N)" so the
// cost is visible at scaffold time (panel amendment 2). The engine is the
// library loop over the ordinary guarded Create path; a refusal mid-loop
// leaves the already-created children in place (append-only doctrine —
// re-running with --first-id resumes) and the parent's decomposed_into audit
// event is written only after every child lands.

import (
	"fmt"
	"os"
	"strings"

	"github.com/coding-hermes/boardctl/internal/board"
)

// sequentialCostLine is amendment 2's required cost line.
const sequentialCostLine = "wave absorbs 1 worker (sequential chain — use --parallel for N)"

func cmdDecompose(dir string, args []string) error {
	fs := newFlagSet("decompose")
	args = reorderArgs(args, valueFlags("C", "count", "first-id", "title-prefix"))
	count := fs.String("count", "", "number of children to scaffold (required, >= 1)")
	parallel := fs.Bool("parallel", false, "independent children, no inter-child deps (DEFAULT)")
	sequential := fs.Bool("sequential", false, "chain child_i on child_{i-1} (opt-in; absorbs 1 worker per wave)")
	firstID := fs.String("first-id", "", "pin the first child's id (subsequent ids mint next-id-style: max+1, observed width preserved)")
	titlePrefix := fs.String("title-prefix", "", "child titles \"<prefix> i/N\" instead of \"<parent title> — step i/N\"")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "boardctl decompose <PARENT-ID> --count N [--parallel] [--sequential] [--first-id ID] [--title-prefix \"string\"] [-C dir]\n")
		_, _ = fmt.Fprintf(fs.Output(), "\n          scaffolds N children from the parent as template: each inherits priority,\n")
		_, _ = fmt.Fprintf(fs.Output(), "          capability_tags, type + configured detail.metadata (expected_files /\n")
		_, _ = fmt.Fprintf(fs.Output(), "          verify_steps ride it), carries parent: <PARENT-ID>, and does NOT depend\n")
		_, _ = fmt.Fprintf(fs.Output(), "          on the parent. Default --parallel (independent children); --sequential\n")
		_, _ = fmt.Fprintf(fs.Output(), "          chains child_i on child_{i-1} and absorbs 1 worker per wave. The loop\n")
		_, _ = fmt.Fprintf(fs.Output(), "          runs the ordinary guarded create per child: a refusal mid-loop leaves\n")
		_, _ = fmt.Fprintf(fs.Output(), "          the created children in place (append-only; resume with --first-id).\n")
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if dir == "" {
		dir = cdir
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("decompose requires exactly one parent task id")
	}
	if *count == "" {
		return fmt.Errorf("decompose requires --count N (the number of children to scaffold): %w", errUsage)
	}
	n, err := parseIntFlag("--count", *count)
	if err != nil {
		return err
	}
	// Spec §2: --count is required and must be >= 1 — 0 or negative is a
	// usage failure (exit 2), not a silent no-op.
	if n < 1 {
		return fmt.Errorf("decompose: --count must be at least 1 (got %d): %w", n, errUsage)
	}
	if *parallel && *sequential {
		return fmt.Errorf("decompose: --parallel and --sequential are mutually exclusive (default is parallel): %w", errUsage)
	}
	opts := board.DecomposeOptions{Count: int(n), FirstID: *firstID, TitlePrefix: *titlePrefix}
	if *sequential {
		opts.Sequential = true
	} else {
		opts.Parallel = true
	}
	if *sequential && opts.Count > 1 {
		fmt.Fprintln(os.Stderr, sequentialCostLine)
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	ids, err := b.Decompose(fs.Arg(0), opts)
	if err != nil {
		// The created children (when a mid-loop refusal left them) are real
		// rows — print what landed before the refusal so the operator sees
		// the partial scaffold, then surface the refusal itself.
		for _, id := range ids {
			if row, _, ferr := b.ShowTask(id); ferr == nil && row != nil {
				_, _ = fmt.Fprintf(os.Stdout, "%s created (earlier child of the refused decompose; kept)\n", id)
			}
		}
		return err
	}
	for i, id := range ids {
		deps := "—"
		if *sequential && i > 0 {
			deps = ids[i-1]
		}
		row, _, ferr := b.ShowTask(id)
		if ferr != nil || row == nil {
			_, _ = fmt.Fprintf(os.Stdout, "%s created (parent %s, deps: %s)\n", id, fs.Arg(0), deps)
			continue
		}
		_, _ = fmt.Fprintf(os.Stdout, "%s created: %q (parent %s, deps: %s)\n", id, row.String("title"), fs.Arg(0), deps)
	}
	_, _ = fmt.Fprintf(os.Stdout, "parent %s updated: decomposed into %s\n", fs.Arg(0), strings.Join(ids, ", "))
	return nil
}
