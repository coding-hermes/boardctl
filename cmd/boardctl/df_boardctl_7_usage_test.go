package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ---------- DF-BOARDCTL-7: usage text newline integrity ----------
//
// Two defects shipped together and must not come back:
//
//  1. a literal two-character backslash-n inside a usage/error format string
//     printed the characters "\" "n" instead of ending the line; and
//  2. the usage text therefore never ended with a newline, so the usage line
//     and the following "boardctl: ..." exit line landed on ONE line.
//
// There are two distinct shapes of the bad sequence and they need two distinct
// detectors — conflating them is how a scan goes vacuous or false-positives:
//
//   - IN OUTPUT the defect is two characters: backslash, 'n'. Emitted text has
//     no legitimate reason to contain that pair.
//   - IN SOURCE the defect is THREE characters: backslash, backslash, 'n'.
//     A CORRECT newline escape in Go source is only two characters
//     (backslash, 'n'), so a source scan that looks for the two-character
//     sequence flags every healthy line — which is exactly the trap this
//     package's first draft fell into.
//
// The checks are split between the two surfaces that can regress
// independently: RENDERED output (behavioural — drives run() through every
// usage-printing path) and SOURCE (static — so a new command is covered the
// moment it is added, not when someone remembers to extend a table).

// outputBackslashN is the defect as a user sees it: an actual backslash
// followed by 'n'. Built by concatenation so this file does not itself contain
// the sequence it forbids.
var outputBackslashN = `\` + "n"

// sourceBackslashN is the same defect as WRITTEN IN SOURCE: backslash,
// backslash, 'n' — the Go spelling that compiles to outputBackslashN. Every
// character is separate here for the same self-reference reason.
var sourceBackslashN = `\` + `\` + "n"

// realTrailingNewline matches a format-string tail that closes with a REAL
// newline escape: a non-backslash character, then backslash-n, quote,
// close-paren. The leading character class is what rejects the bad spelling
// `\\n")`, which would otherwise satisfy a naive suffix check because
// `...dir]\\n")` also ends with the four characters `\n")`.
var realTrailingNewline = regexp.MustCompile(`[^\\]\\n"\)`)

// usageCase is one invocation that must print usage text.
type usageCase struct {
	name string
	args []string
	// usageSig is a substring identifying the usage line expected in the
	// output.
	usageSig string
	// errLine, when true, means the case ends with a "boardctl: ..." line
	// that must start on a fresh line after the usage text.
	errLine bool
}

// TestUsageOutputNeverContainsLiteralBackslashN drives every usage-printing
// path through run() and asserts, on the captured output:
//   - zero literal backslash-n sequences (acceptance 1);
//   - the output ends with exactly one newline, so the usage text and the exit
//     line can never share a line (acceptance 2);
//   - no single line carries BOTH a usage signature and the "boardctl: "
//     prefix, which is the precise shape of the original collision.
func TestUsageOutputNeverContainsLiteralBackslashN(t *testing.T) {
	// `-h` reaches each subcommand's fs.Usage closure via flag.ErrHelp, so
	// this table executes all 14 usage printers that exist today.
	subcommands := []string{
		"init", "list", "show", "create", "update", "event", "header",
		"validate", "doctor", "version", "stats", "render", "import", "serve",
	}
	cases := []usageCase{
		// Top-level paths.
		{name: "no args", args: nil, usageSig: "usage:"},
		{name: "unknown command", args: []string{"nosuchcmd"}, usageSig: "commands:"},
	}
	for _, cmd := range subcommands {
		cases = append(cases, usageCase{
			name:     cmd + " -h",
			args:     []string{cmd, "-h"},
			usageSig: "boardctl " + cmd,
			errLine:  true,
		})
	}
	// The exact invocation named in the acceptance criteria: bad flag.
	cases = append(cases,
		usageCase{name: "create --bogus-flag", args: []string{"create", "--bogus-flag"}, usageSig: "boardctl create", errLine: true},
		usageCase{name: "update --bogus-flag", args: []string{"update", "--bogus-flag"}, usageSig: "boardctl update", errLine: true},
	)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := captureStderr(func() { run(tc.args) })
			if err != nil {
				t.Fatal(err)
			}
			if got == "" {
				t.Fatalf("no usage output on stderr for %v", tc.args)
			}
			assertUsageShape(t, got, tc)
		})
	}

	// `help` prints the same text on stdout (exit 0), so the integrity
	// assertions apply there too.
	t.Run("help on stdout", func(t *testing.T) {
		got, err := captureStdout(func() {
			if code := run([]string{"help"}); code != 0 {
				t.Fatalf("help exit code = %d, want 0", code)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		assertUsageShape(t, got, usageCase{name: "help", usageSig: "usage:"})
	})
}

// assertUsageShape applies the shared output-shape contract.
func assertUsageShape(t *testing.T, out string, tc usageCase) {
	t.Helper()

	if n := strings.Count(out, outputBackslashN); n > 0 {
		t.Fatalf("output contains %d literal backslash-n sequence(s); usage text must use real newlines:\n%q", n, out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("output does not end with a newline — the next line would be glued onto it:\n%q", out)
	}
	if strings.HasSuffix(out, "\n\n") {
		t.Fatalf("output ends with more than one newline:\n%q", out)
	}
	if tc.usageSig != "" && !strings.Contains(out, tc.usageSig) {
		t.Fatalf("output = %q, want usage signature %q", out, tc.usageSig)
	}

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	for i, line := range lines {
		if strings.Contains(line, outputBackslashN) {
			t.Fatalf("line %d contains a literal backslash-n: %q", i+1, line)
		}
		// The collision signature: a usage line and the exit line on one row.
		if tc.usageSig != "" && strings.Contains(line, tc.usageSig) && strings.Contains(line, "boardctl: ") {
			t.Fatalf("line %d glues usage text to the exit line: %q", i+1, line)
		}
	}

	if tc.errLine {
		last := lines[len(lines)-1]
		if !strings.HasPrefix(last, "boardctl: ") {
			t.Fatalf("last line = %q, want a fresh \"boardctl: ...\" exit line", last)
		}
	}
}

// TestUsageSourceHasNoLiteralBackslashN scans every NON-TEST Go file in
// cmd/boardctl for the SOURCE spelling of the bad sequence (backslash,
// backslash, 'n'), so a sibling occurrence — a new command, serve.go,
// import.go, render.go — fails here without anyone extending a table.
//
// Test files are excluded on purpose: a fixture or a t.Fatalf message may
// legitimately spell the three-character sequence, and only a usage/error
// format string reaches a user's terminal.
func TestUsageSourceHasNoLiteralBackslashN(t *testing.T) {
	files := productionGoFiles(t)
	scanned := 0
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		for i, line := range strings.Split(string(body), "\n") {
			if strings.Contains(line, sourceBackslashN) {
				t.Errorf("%s:%d contains a literal backslash-n in a format string: %s", f, i+1, strings.TrimSpace(line))
			}
		}
	}
	if scanned != len(files) {
		t.Fatalf("scanned %d of %d files", scanned, len(files))
	}
}

// productionGoFiles lists the package's non-test .go files, failing loudly if
// the walk finds nothing (which would make every source invariant vacuous).
func productionGoFiles(t *testing.T) []string {
	t.Helper()
	all, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range all {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		t.Fatal("no non-test .go files found — the scan ran from the wrong directory")
	}
	return out
}

// TestEveryUsageClosureEndsWithRealNewline is the preventive half of the fix:
// acceptance criterion 2 is only structurally guaranteed if EVERY usage
// printer's format string ends with a real newline. Brace-depth walking finds
// each `fs.Usage = func() {...}` block, so a new subcommand is covered
// automatically rather than when someone extends a list.
func TestEveryUsageClosureEndsWithRealNewline(t *testing.T) {
	seen := 0
	for _, f := range productionGoFiles(t) {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, blk := range usageBlocks(strings.Split(string(body), "\n")) {
			seen++
			var prints []string
			for _, line := range blk.lines {
				if strings.Contains(line, "Fprint") {
					prints = append(prints, line)
				}
			}
			if len(prints) == 0 {
				t.Errorf("%s:%d usage closure prints nothing — unusable usage text", f, blk.start)
				continue
			}
			last := prints[len(prints)-1]
			if !realTrailingNewline.MatchString(last) {
				t.Errorf("%s:%d usage format string does not end with a real newline: %s", f, blk.start, strings.TrimSpace(last))
			}
		}
	}
	// 12 closures live in main.go, 1 each in import.go, render.go, serve.go.
	if want := 14; seen != want {
		t.Fatalf("found %d fs.Usage closures, want %d — the walker missed blocks or a command lost its usage text", seen, want)
	}
}

// usageBlock is one `fs.Usage = func() { ... }` block: the 1-based line the
// block starts on plus every line of the block.
type usageBlock struct {
	start int
	lines []string
}

// usageBlocks collects every fs.Usage closure by brace depth: the block opens
// on the line containing the marker and closes when the depth returns to zero,
// which handles both the multi-line form and the single-line
// `fs.Usage = func() { fmt.Fprintf(...) }` form.
func usageBlocks(lines []string) []usageBlock {
	var blocks []usageBlock
	for i := 0; i < len(lines); i++ {
		if !strings.Contains(lines[i], "fs.Usage = func()") {
			continue
		}
		depth := 0
		opened := false
		blk := usageBlock{start: i + 1}
		for j := i; j < len(lines); j++ {
			blk.lines = append(blk.lines, lines[j])
			depth += strings.Count(lines[j], "{")
			if strings.Contains(lines[j], "{") {
				opened = true
			}
			depth -= strings.Count(lines[j], "}")
			if opened && depth <= 0 {
				i = j
				break
			}
		}
		blocks = append(blocks, blk)
	}
	return blocks
}

// TestUsageDetectorsDiscriminate proves each detector separates the two
// spellings. Without it, an over-broad regex or a mis-shaped literal would
// make the invariants above vacuous and every test would pass on broken code.
func TestUsageDetectorsDiscriminate(t *testing.T) {
	// The bad spelling, assembled without embedding it literally here.
	badClosure := `fmt.Fprintf(os.Stderr, "boardctl create ...[-C dir]` + sourceBackslashN + `")`
	goodClosure := `fmt.Fprintf(os.Stderr, "boardctl create ...[-C dir]\n")`

	// SOURCE detector: sees the bad spelling, ignores the healthy one.
	if !strings.Contains(badClosure, sourceBackslashN) {
		t.Fatalf("source detector does not see the bad spelling: %q", badClosure)
	}
	if strings.Contains(goodClosure, sourceBackslashN) {
		t.Fatalf("source detector false-positives on the healthy spelling: %q", goodClosure)
	}
	// The honest limit of the source scan: the two-character sequence appears
	// in BOTH spellings, so it cannot be used against source text. Pinning
	// that here stops a future edit from "simplifying" the scan into a
	// detector that flags every correct line.
	if !strings.Contains(badClosure, outputBackslashN) || !strings.Contains(goodClosure, outputBackslashN) {
		t.Fatalf("source text should contain the two-character sequence in BOTH spellings")
	}

	// NEWLINE regex: accepts the healthy tail, rejects the bad one.
	if !realTrailingNewline.MatchString(goodClosure) {
		t.Fatalf("newline regex rejects the healthy spelling: %q", goodClosure)
	}
	if realTrailingNewline.MatchString(badClosure) {
		t.Fatalf("newline regex accepts the bad spelling: %q", badClosure)
	}

	// The brace walker must survive the single-line closure form.
	blocks := usageBlocks([]string{
		"fs.Usage = func() { fmt.Fprintf(os.Stderr, \"boardctl x\\n\") }",
	})
	if len(blocks) != 1 {
		t.Fatalf("usageBlocks found %d blocks in a single-line closure, want 1", len(blocks))
	}
	if len(blocks[0].lines) != 1 {
		t.Fatalf("usageBlocks = %v, want exactly the closure's own line", blocks[0].lines)
	}
}

// TestTopLevelUsageTextEndsWithNewline covers the one usage string that is
// printed directly rather than through a flagset: the bare `boardctl`
// invocation and `help` both write usageText verbatim, so it must itself end
// with a newline. It is a raw string, where the two-character sequence would
// also be printed literally.
func TestTopLevelUsageTextEndsWithNewline(t *testing.T) {
	if strings.Contains(usageText, outputBackslashN) {
		t.Errorf("usageText contains a literal backslash-n sequence")
	}
	if !strings.HasSuffix(usageText, "\n") {
		t.Fatal("usageText does not end with a newline")
	}
	if strings.HasSuffix(usageText, "\n\n") {
		t.Fatal("usageText ends with more than one newline")
	}
	for _, want := range []string{"boardctl", "usage:", "commands:"} {
		if !strings.Contains(usageText, want) {
			t.Errorf("usageText lost %q", want)
		}
	}
}
