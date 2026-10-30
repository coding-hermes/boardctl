package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// cmdNormalizeStatuses is deliberately a single-board command. The resolver
// follows -C and never discovers or writes sibling repositories.
func cmdNormalizeStatuses(dir string, args []string) error {
	fs := newFlagSet("normalize-statuses")
	args = reorderArgs(args, valueFlags("C"))
	apply := fs.Bool("apply", false, "atomically canonicalize known status aliases")
	fs.Bool("dry-run", false, "report only and write nothing (default)")
	asJSON := fs.Bool("json", false, "emit the per-board census as JSON")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(fs.Output(), "boardctl normalize-statuses [--apply] [--dry-run] [--json] [-C dir]")
		_, _ = fmt.Fprintln(fs.Output(), "\nOne resolved repository-local board only; dry-run is the default and writes nothing.")
		_, _ = fmt.Fprintln(fs.Output(), "Known read aliases are canonicalized on --apply. Unknown and missing statuses are refused, never guessed.")
		_, _ = fmt.Fprint(fs.Output(), "Fleet-wide apply and canonical-set policy changes remain owner-gated.\n\n")
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("normalize-statuses takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	rep, err := b.NormalizeStatuses(*apply)
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return err
		}
		return nil
	}
	_, _ = fmt.Fprintf(os.Stdout, "board: %s\n", rep.BoardPath)
	_, _ = fmt.Fprintf(os.Stdout, "rows: %d | aliases: %d | unknown: %d | missing: %d | changed: %d\n", rep.Rows, rep.AliasCount, rep.UnknownCount, rep.MissingCount, rep.Changed)
	for _, row := range rep.Normalized {
		_, _ = fmt.Fprintf(os.Stdout, "  line %d  %s: %s\n", row.Line, row.ID, row.Action)
	}
	for _, row := range rep.Refused {
		_, _ = fmt.Fprintf(os.Stdout, "  line %d  %s: %s\n", row.Line, row.ID, row.Action)
	}
	if !*apply {
		_, _ = fmt.Fprintln(os.Stdout, "DRY-RUN — no files written (pass --apply to normalize known aliases)")
	}
	return nil
}
