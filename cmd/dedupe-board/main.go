// Command dedupe-board is the board-collapse repair tool for coding-hermes
// JSONL foreman boards. It has two modes:
//
// DEFAULT — the one-shot SG-126 finding-fingerprint backfill.
//
// QA and dogfood lanes re-filed the same finding 3-8x before boardctl learned
// to fingerprint them; those duplicate rows are still open on the affected
// boards. This mode collapses each collision group:
//
//   - every open row without a finding fingerprint gains one;
//   - each group of >= 2 open rows sharing a fingerprint keeps the EARLIEST row
//     and closes the rest (status=complete, worker_summary "merged into <kept>:
//     dedupe backfill <date>", completed_at now);
//   - the merged-away rows' evidence is carried onto the kept row;
//   - one `audit` event per group records kept / merged / fingerprint.
//
// --by-id — the DF-BOARDCTL-10 recycled-id repair.
//
// Live boards also carry many DISTINCT findings filed under ONE recycled task
// id (six QA-ASCE-1 rows, every title+reasoning different: header-frozen,
// spawn-pool exhausted, port-range exhausted x2, bunker-dropped). validate
// permanently errors on every such board, and the fingerprint mode correctly
// REFUSES to collapse them — their fingerprints all differ, because they are
// different findings. This mode repairs the hygiene:
//
//   - rows are grouped by id; the EARLIEST line per id is kept untouched;
//   - each later line is closed as superseded-by-earliest (status=complete,
//     superseded_by=<id>, worker_summary naming the kept line, completed_at
//     now) — the row's own title/reasoning bytes are NOT rewritten;
//   - one `audit` event per closed line quotes that line's original
//     title+reasoning verbatim, so the finding text survives in the event log;
//   - later lines that are already terminal (complete/failed) are left as-is.
//
// FRESH-ID RULE (fix direction (a) of DF-BOARDCTL-10): a lane must ALWAYS mint
// a fresh task id for a new finding. An id-recycled filing — a new finding
// under an old key — is never a re-observation of the same task; the --by-id
// mode exists to repair boards where lanes re-used a cycle number instead of
// advancing the id.
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
	"strconv"
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
	byID := fs.Bool("by-id", false, "recycled-id mode (DF-BOARDCTL-10): group rows by id, keep the earliest line per id, close later lines as superseded-by-earliest — for DISTINCT findings filed under ONE recycled id (a lane must ALWAYS mint a fresh id for a new finding; id-recycled = new finding under an old key)")
	lanes := fs.String("lanes", "", "comma-separated lane names allowed to be touched (guard; refuses any other board)")
	asJSON := fs.Bool("json", false, "emit the --by-id report as JSON (only supported with --by-id)")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `dedupe-board — board-collapse repair tool (default: dry-run)

Two modes:

  DEFAULT — finding-fingerprint collapse (SG-126): rows sharing a normalized
  title+reasoning are the SAME finding re-filed; the earliest row is kept and
  the rest are merged into it.

  --by-id — recycled-id repair (DF-BOARDCTL-10): rows sharing ONE task id are
  DISTINCT findings filed under a recycled key (a lane re-used a cycle number
  instead of minting a fresh id). The EARLIEST line per id is kept; each later
  line is closed as superseded-by-earliest (status=complete,
  superseded_by=<id>, worker_summary naming the kept line), and the closed
  line's original title+reasoning is preserved in an audit event. Later lines
  that are already terminal (complete/failed) are left untouched.

FRESH-ID RULE: a lane must ALWAYS mint a fresh task id for a new finding. An
id-recycled filing — a new finding under an old key — is never a
re-observation of the same task.

usage:
  dedupe-board --board-dir <path> [--apply] [--lanes a,b,c]
  dedupe-board --board-dir <path> --by-id [--apply] [--lanes a,b,c] [--json]

flags:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		// An explicit -h/-help is a successful help request (flag's own
		// ExitOnError exits 0 for ErrHelp); any other parse failure is
		// usage/refusal.
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	if *asJSON && !*byID {
		fmt.Fprintln(os.Stderr, "dedupe-board: --json is only supported with --by-id (the default fingerprint mode keeps its text report)")
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

	if *byID {
		return runByID(b, lane, *apply, *asJSON)
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

// runByID executes the --by-id mode: report (dry-run or applied), then the
// text or JSON rendering. Kept in its own function so the default fingerprint
// path above stays byte-for-byte what it always was.
func runByID(b *board.Board, lane string, apply, asJSON bool) int {
	rep, err := b.DedupeByID(apply)
	if err != nil {
		// A refusal (unknown status on a later duplicate) or a read failure.
		// The message names the file, line, id and offending value; in
		// dry-run nothing was written, in apply the message itself says
		// whether the board was rewritten before the failure.
		fmt.Fprintf(os.Stderr, "dedupe-board: %v\n", err)
		return 1
	}

	if asJSON {
		out := struct {
			Lane string `json:"lane"`
			*board.IDDupeReport
		}{Lane: lane, IDDupeReport: rep}
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
	fmt.Printf("task rows: %d (distinct ids: %d, recycled ids: %d)\n", rep.TaskRows, rep.DistinctIDs, rep.RecycledIDs)
	for _, g := range rep.Groups {
		note := ""
		if len(g.AlreadyTerminal) > 0 {
			terms := make([]string, len(g.AlreadyTerminal))
			for i, ln := range g.AlreadyTerminal {
				terms[i] = strconv.Itoa(ln)
			}
			note = "  [already terminal, left as-is: line " + strings.Join(terms, ", ") + "]"
		}
		fmt.Printf("  %s  keep line %d  supersede %s%s\n", g.ID, g.KeptLine, idDupeClosedLabel(g), note)
		for i, ln := range g.ClosedLines {
			fmt.Printf("    line %d [%s]: %s\n", ln, g.ClosedStatusWas[i], g.ClosedTitles[i])
		}
	}
	if !apply {
		if rep.Changed() {
			fmt.Println("DRY-RUN — no files written (pass --apply to close later lines as superseded-by-earliest)")
		} else {
			fmt.Println("nothing to do — no recycled task ids on this board (no files written)")
		}
		return 0
	}
	if !rep.Changed() {
		fmt.Println("nothing to do — no recycled task ids on this board (no files written)")
		return 0
	}
	fmt.Println("applied — later lines closed as superseded-by-earliest, one audit event per closed line (original title+reasoning preserved in events.jsonl)")
	return 0
}

// idDupeClosedLabel renders a group's closed lines as "<id> (line N), …".
func idDupeClosedLabel(g board.IDDupeGroup) string {
	parts := make([]string, 0, len(g.ClosedLines))
	for _, ln := range g.ClosedLines {
		parts = append(parts, fmt.Sprintf("%s (line %d)", g.ID, ln))
	}
	return strings.Join(parts, ", ")
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
