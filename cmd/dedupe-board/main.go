// Command dedupe-board is the one-shot SG-126 backfill for coding-hermes
// JSONL foreman boards.
//
// QA and dogfood lanes re-filed the same finding 3-8x before boardctl learned
// to fingerprint them; those duplicate rows are still open on the affected
// boards. This tool collapses each collision group:
//
//   - every open row without a finding fingerprint gains one;
//   - each group of >= 2 open rows sharing a fingerprint keeps the EARLIEST row
//     and closes the rest (status=complete, worker_summary "merged into <kept>:
//     dedupe backfill <date>", completed_at now);
//   - the merged-away rows' evidence is carried onto the kept row;
//   - one `audit` event per group records kept / merged / fingerprint.
//
// Safety rails, in order of importance:
//
//  1. It runs on ONE board at a time: --board-dir is REQUIRED (a repo root, a
//     .coding-hermes dir, or a board dir). There is no fleet-wide sweep, so it
//     can never touch a board the operator did not name.
//  2. DRY-RUN by default. Nothing is written — no rows, no fingerprints, no
//     audit events — until --apply is passed.
//  3. --lanes <a,b,c> is a guard, not a target: when given, the resolved
//     board's lane (header project/namespace, else the repo dir name) must be
//     in the list or the run refuses with exit 2.
//
// Exit codes: 0 success (dry-run included), 1 runtime error, 2 usage / refusal.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/coding-hermes/boardctl/internal/board"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("dedupe-board", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	boardDir := fs.String("board-dir", "", "board dir, .coding-hermes dir, or repo root (REQUIRED — one board per run)")
	apply := fs.Bool("apply", false, "write the changes (default: dry-run, nothing is written)")
	lanes := fs.String("lanes", "", "comma-separated lane names allowed to be touched (guard; refuses any other board)")
	asJSON := fs.Bool("json", false, "emit the report as JSON")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `dedupe-board — SG-126 one-shot board-collapse (default: dry-run)

usage:
  dedupe-board --board-dir <path> [--apply] [--lanes a,b,c] [--json]

flags:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "dedupe-board: takes no positional args (got %q)\n", fs.Arg(0))
		return 2
	}
	if *boardDir == "" {
		fmt.Fprintln(os.Stderr, "dedupe-board: --board-dir is required (one board per run — this tool never sweeps the fleet)")
		return 2
	}
	b, err := board.Resolve(*boardDir)
	if err != nil {
		if errors.Is(err, board.ErrBoardNotFound) {
			fmt.Fprintf(os.Stderr, "dedupe-board: %v\n", err)
			return 2
		}
		fmt.Fprintf(os.Stderr, "dedupe-board: %v\n", err)
		return 2
	}

	lane := b.LaneName()
	if *lanes != "" {
		allowed := map[string]bool{}
		for _, l := range strings.Split(*lanes, ",") {
			if l = strings.TrimSpace(strings.ToLower(l)); l != "" {
				allowed[l] = true
			}
		}
		if !allowed[strings.ToLower(lane)] {
			fmt.Fprintf(os.Stderr, "dedupe-board: refusing to touch board %s (lane %q) — it is not in --lanes %s\n", b.TasksPath(), lane, *lanes)
			return 2
		}
	}

	rep, err := b.DedupeBackfill(*apply)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dedupe-board: %v\n", err)
		return 1
	}

	if *asJSON {
		out := struct {
			Lane string `json:"lane"`
			*board.DedupeReport
		}{Lane: lane, DedupeReport: rep}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "dedupe-board: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Printf("lane:  %s\n", lane)
	fmt.Printf("board: %s\n", rep.BoardPath)
	fmt.Printf("open rows: %d (already fingerprinted: %d, gained: %d)\n", rep.OpenRows, rep.AlreadySet, len(rep.Fingerprinted))
	if len(rep.Drifted) > 0 {
		fmt.Printf("drift: %d open row(s) whose stored fingerprint no longer matches a recompute: %s\n", len(rep.Drifted), strings.Join(rep.Drifted, ", "))
	}
	fmt.Printf("merge groups: %d (rows merged away: %d)\n", len(rep.Groups), len(rep.MergedAway))
	for _, g := range rep.Groups {
		note := ""
		if g.DuplicateIDs {
			note = "  [ids repeated in this group — line numbers identify the rows]"
		}
		fmt.Printf("  %s  keep %s (line %d)  merge %s%s\n",
			shortFingerprint(g.Fingerprint), g.Kept, g.KeptLine, mergedLabel(g), note)
	}
	if !*apply {
		fmt.Println("DRY-RUN — no files written (pass --apply to collapse)")
		return 0
	}
	if !rep.Changed() {
		fmt.Println("nothing to do — board already collapsed (no files written)")
		return 0
	}
	fmt.Println("applied — tasks.jsonl rewritten in place, one audit event per merge group")
	return 0
}

// shortFingerprint abbreviates a hex digest for human output (the JSON report
// carries the full value).
func shortFingerprint(fp string) string {
	if len(fp) <= 12 {
		return fp
	}
	return fp[:12] + "…"
}

// mergedLabel renders a group's merged-away rows as "<id> (line N), …" so a
// board that reuses an id on several lines stays readable.
func mergedLabel(g board.DedupeMerge) string {
	parts := make([]string, 0, len(g.Merged))
	for i, id := range g.Merged {
		if i < len(g.MergedLines) {
			parts = append(parts, fmt.Sprintf("%s (line %d)", id, g.MergedLines[i]))
			continue
		}
		parts = append(parts, id)
	}
	return strings.Join(parts, ", ")
}
