package main

import (
	"strings"
	"testing"
)

// REVIEW-BOARDCTL-010: `boardctl event --help` printed a one-line usage
// ("boardctl event --type audit [flags] [-C dir]") with no flag enumeration,
// unlike every sibling subcommand (create/update/validate/...), whose
// hand-written Usage closures list their flags; the full event flag set was
// only discoverable via the top-level `boardctl help`. This pins the
// repaired Usage closure: every event flag must appear in the help output,
// in the sibling shape (signature line + continuation flags + the
// "[flags] [-C dir]" suffix + the top-level help's --type vocabulary
// rendering). The --detail assertion uses a trailing-space token so it
// cannot pass vacuously via "--detail-text".
func TestReviewBoardCTL010EventHelpEnumeratesFlags(t *testing.T) {
	for _, helpFlag := range []string{"--help", "-h"} {
		var out string
		var err error
		out, err = captureStdout(func() {
			if code := run([]string{"-C", t.TempDir(), "event", helpFlag}); code != 0 {
				t.Errorf("event %s exit code = %d, want 0 (BT-041: help is a successful usage query)", helpFlag, code)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, flag := range []string{
			"--type ", "--task-id", "--actor", "--detail ", "--detail-text", "--tick", "-C",
		} {
			if !strings.Contains(out, flag) {
				t.Errorf("event %s output missing flag %q\ngot:\n%s", helpFlag, flag, out)
			}
		}
		if sig := "boardctl event"; !strings.Contains(out, sig) {
			t.Errorf("event %s output = %q, want usage signature %q on stdout", helpFlag, out, sig)
		}
		if !strings.Contains(out, "[flags] [-C dir]") {
			t.Errorf("event %s output = %q, want the sibling usage suffix %q", helpFlag, out, "[flags] [-C dir]")
		}
	}
}
