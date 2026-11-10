package main

import (
	"fmt"
	"os"

	"github.com/coding-hermes/boardctl/internal/board"
)

func cmdEvidence(dir string, args []string) error {
	// ch:trace row=BT-090 spec=.coding-hermes/board/tasks.jsonl#BT-090 wave=none:serial-fallback test=internal/board/bt090_completion_evidence_test.go::TestBT090CompletionEvidenceGateTable doc=README.md evidence=none:unit-and-cli-tests-are-the-evidence witness=none:pending-foreman-live-verification verdict=none:pending-foreman-judge commit=none:worker-commit-not-final memory=none:tick-local STATUS: partial TIER: T2 EVIDENCE: go test ./internal/board ./cmd/boardctl -count=1
	fs := newFlagSet("evidence")
	args = reorderArgs(args, valueFlags("C"))
	record := fs.Bool("record", false, "record current completion debt")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "boardctl evidence baseline --record [-C dir]\n")
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 || fs.Arg(0) != "baseline" || !*record {
		return fmt.Errorf("evidence requires: baseline --record")
	}
	if dir == "" {
		dir = cdir
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	count, err := b.RecordCompletionBaseline()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "completion evidence baseline recorded: %d debt entr%s\n", count, map[bool]string{true: "y", false: "ies"}[count == 1])
	return nil
}

func renderCompletionWarnings(b *board.Board) {
	for _, warning := range b.CompletionWarnings() {
		fmt.Fprintf(os.Stderr, "boardctl: WARN completion debt for %q (%s): %s; use a real commit, bind an evidence run, or deliberately record this id with --record-debt\n", warning.TaskID, warning.Hash, warning.Reason)
	}
}
