package main

import (
	"fmt"
	"os"
	"time"

	"github.com/coding-hermes/boardctl/internal/board"
	"github.com/coding-hermes/boardctl/internal/render"
)

// ---------- render ----------

// cmdRender builds the board-analytics payload and writes the
// self-contained HTML report (BT-020; design authority:
// docs/specs/board-analytics-report.md). Render is READ-ONLY toward board
// files: nothing under the board dir is written or touched.
//
// Flags: -o out.html (default board-report.html; "-" = stdout), --tz
// <IANA zone> pins the report timezone for deterministic output, --json
// out.json additionally writes the board-report/v1 payload (the BT-022
// export format). --skip-bad-lines (DF-BOARDCTL-9) degrades with evidence:
// the render's tolerant loader already skips unparseable rows with
// ParseWarnings; the flag additionally surfaces the exact skipped lines
// (path, 1-based line, snippet, parse error) on stderr and forces exit 1 so
// a degraded report cannot pass silently. Exit codes via the shared
// contract: 2 board-not-found, 1 write/operational failure, 0 success.
func cmdRender(dir string, args []string) error {
	fs := newFlagSet("render")
	args = reorderArgs(args, valueFlags("C", "o", "tz", "json"))
	out := fs.String("o", "board-report.html", "output HTML file ('-' for stdout)")
	tz := fs.String("tz", "", "report timezone (IANA name, e.g. America/Bogota); default: machine local")
	jsonOut := fs.String("json", "", "also write the board-report/v1 payload as JSON")
	skipBad := useSkipBad(fs)
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "boardctl render [-C dir] [-o out.html] [--tz Zone] [--json out.json] [--skip-bad-lines]\n")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("render takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}

	loc := time.Local
	tzName := "Local"
	if *tz != "" {
		l, err := time.LoadLocation(*tz)
		if err != nil {
			return fmt.Errorf("--tz %q: %w", *tz, err)
		}
		loc = l
		tzName = l.String()
	}

	b, err := board.Resolve(dir)
	if err != nil {
		if _, oerr := openBoard(dir); oerr != nil {
			return oerr // reuse the openBoard hint wrapping for exit 2
		}
		return err
	}
	armSkipBad(b, skipBad)
	payload, err := render.BuildFromBoard(b, render.Options{Now: time.Now(), Zone: loc})
	if err != nil {
		return err
	}
	payload.ReportTimezone = tzName
	// DF-BOARDCTL-9: without the flag a degraded read is still a degraded
	// read — render keeps its tolerant-loader warnings, but the
	// SKIPPED-LINES evidence and the exit-1 contract only apply when the
	// operator asked for the degrade mode.
	degraded := reportSkipped(b)

	if *jsonOut != "" {
		jb, err := render.EscapeJSONIsland(payload)
		if err != nil {
			return fmt.Errorf("--json: %w", err)
		}
		if werr := writeReportFile(*jsonOut, append(jb, '\n')); werr != nil {
			return werr
		}
		fmt.Fprintf(os.Stdout, "wrote %s (board-report/v1 payload)\n", *jsonOut)
	}

	html, err := render.RenderHTML(payload)
	if err != nil {
		return err
	}
	if *out == "-" {
		os.Stdout.WriteString(html)
		fmt.Fprintf(os.Stderr, "wrote report to stdout (%d bytes)\n", len(html))
		return exitError(degraded)
	}
	if werr := writeReportFile(*out, []byte(html)); werr != nil {
		return werr
	}
	fmt.Fprintf(os.Stdout, "wrote %s (%d bytes, %d board(s))\n", *out, len(html), len(payload.Boards))
	return exitError(degraded)
}

// writeReportFile writes a render artifact to path, creating parent dirs.
func writeReportFile(path string, data []byte) error {
	if dir := dirOf(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				return "/"
			}
			return path[:i]
		}
	}
	return ""
}
