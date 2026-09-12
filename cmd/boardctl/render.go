package main

import (
	"fmt"
	"os"
	"time"

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
// export format). Exit codes via the shared contract: 2 board-not-found,
// 1 write/operational failure, 0 success.
func cmdRender(dir string, args []string) error {
	fs := newFlagSet("render")
	args = reorderArgs(args, valueFlags("C", "o", "tz", "json"))
	out := fs.String("o", "board-report.html", "output HTML file ('-' for stdout)")
	tz := fs.String("tz", "", "report timezone (IANA name, e.g. America/Bogota); default: machine local")
	jsonOut := fs.String("json", "", "also write the board-report/v1 payload as JSON")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "boardctl render [-C dir] [-o out.html] [--tz Zone] [--json out.json]\n")
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

	payload, err := render.Build(dir, render.Options{Now: time.Now(), Zone: loc})
	if err != nil {
		return err
	}
	payload.ReportTimezone = tzName

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
		return nil
	}
	if werr := writeReportFile(*out, []byte(html)); werr != nil {
		return werr
	}
	fmt.Fprintf(os.Stdout, "wrote %s (%d bytes, %d board(s))\n", *out, len(html), len(payload.Boards))
	return nil
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
