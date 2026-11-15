package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/coding-hermes/boardctl/internal/board"
)

func cmdTypes(dir string, args []string) error {
	fs := newFlagSet("types")
	args = reorderArgs(args, valueFlags("C"))
	doctor := fs.Bool("doctor", false, "report types.jsonl configuration health")
	asJSON := fs.Bool("json", false, "emit configured types as JSON")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() { _, _ = fmt.Fprintf(fs.Output(), "boardctl types [-C dir] [--json] [--doctor]\n") }
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("types takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	reg, err := board.LoadTicketTypes(b.Dir)
	if err != nil {
		return err
	}
	for _, warning := range reg.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	if *doctor {
		_, _ = fmt.Fprint(os.Stdout, reg.DoctorText())
		return nil
	}
	if *asJSON {
		payload := struct {
			Types    []board.TicketType `json:"types"`
			Retired  []board.TicketType `json:"retired"`
			Warnings []string           `json:"warnings,omitempty"`
		}{Types: reg.Sorted(true), Retired: reg.Sorted(false), Warnings: reg.Warnings}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
	_, _ = fmt.Fprint(os.Stdout, reg.RenderText())
	return nil
}
