package board

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TopologyBWriteError is returned when a mutating command targets a
// topology-B board (no board.jsonl: the header is line 1 of tasks.jsonl,
// a read-only legacy layout). Migrate by splitting line 1 of tasks.jsonl
// into its own board.jsonl; boardctl does not rewrite topology-B boards.
var TopologyBWriteError = errors.New("topology B: header is line 1 of tasks.jsonl (read-only legacy layout) — migrate by splitting line 1 of tasks.jsonl into board.jsonl")

// Board locates one JSONL-canonical foreman board and its tracked files.
type Board struct {
	Dir      string // absolute board dir (.coding-hermes/board or equivalent)
	Topology string // "A" (board.jsonl header present) or "B"

	headerPath   string // board.jsonl (topology A only)
	tasksPath    string // tasks.jsonl
	eventsPath   string // events.jsonl
	fixturesPath string // fixtures.jsonl (optional)
}

// ErrBoardNotFound is wrapped with the directories probed.
var ErrBoardNotFound = errors.New("no JSONL foreman board found")

// boardDirCandidates lists the directories Resolve probes for a
// tasks.jsonl+events.jsonl pair, in order. For an ordinary target (a repo
// root or a board dir) that is: the target itself, its .coding-hermes, then
// .coding-hermes/board. When the target itself is named .coding-hermes the
// nested .coding-hermes/.coding-hermes/board candidate is meaningless, so the
// board subdir <given>/board is probed instead — passing -C
// <repo>/.coding-hermes finds <repo>/.coding-hermes/board (BT-006). Init uses
// the same order (and falls back to the LAST candidate as the fresh-board
// location).
func boardDirCandidates(abs string) []string {
	if filepath.Base(abs) == ".coding-hermes" {
		return []string{
			abs,
			filepath.Join(abs, "board"),
		}
	}
	return []string{
		abs,
		filepath.Join(abs, ".coding-hermes"),
		filepath.Join(abs, ".coding-hermes", "board"),
	}
}

// Resolve locates a board from a user-supplied -C target:
//
//	-C <repo-root>                  -> <repo>/.coding-hermes/board
//	-C <repo>/.coding-hermes        -> <repo>/.coding-hermes/board
//	-C <repo>/.coding-hermes/board  -> the board dir itself
//
// The probing order is boardDirCandidates: the target itself, then the
// nested locations (for a .coding-hermes target the board/ subdir, otherwise
// .coding-hermes/board).
//
// An empty target means the current working directory (same probing rules).
func Resolve(target string) (*Board, error) {
	if target == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		target = wd
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	cands := boardDirCandidates(abs)
	for _, c := range cands {
		tasks := filepath.Join(c, "tasks.jsonl")
		events := filepath.Join(c, "events.jsonl")
		if fileExists(tasks) && fileExists(events) {
			b := &Board{
				Dir:          c,
				tasksPath:    tasks,
				eventsPath:   events,
				fixturesPath: filepath.Join(c, "fixtures.jsonl"),
			}
			header := filepath.Join(c, "board.jsonl")
			if fileExists(header) {
				b.Topology = "A"
				b.headerPath = header
			} else {
				b.Topology = "B"
			}
			return b, nil
		}
	}
	return nil, fmt.Errorf("%w: %s (looked for tasks.jsonl+events.jsonl in %s)",
		ErrBoardNotFound, target, strings.Join(cands, ", "))
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// TasksPath / EventsPath / HeaderPath / FixturesPath expose the tracked file
// paths (fixtures is "" when absent).
func (b *Board) TasksPath() string  { return b.tasksPath }
func (b *Board) EventsPath() string { return b.eventsPath }
func (b *Board) HeaderPath() string { return b.headerPath }
func (b *Board) FixturesPath() string {
	if fileExists(b.fixturesPath) {
		return b.fixturesPath
	}
	return ""
}

// IsTopologyA reports whether the header lives in board.jsonl (writable).
func (b *Board) IsTopologyA() bool { return b.Topology == "A" }

// ReadJSONLLines reads a file, splitting on "\n" exactly as stored. The
// returned slice round-trips byte-identically through JoinLines: the file is
// reproduced exactly (trailing blank lines included, single trailing newline
// preserved, no duplicates introduced).
func ReadJSONLLines(path string) ([][]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytes.Split(content, []byte("\n")), nil
}

// JoinLines is the exact inverse of ReadJSONLLines.
func JoinLines(lines [][]byte) []byte {
	return bytes.Join(lines, []byte("\n"))
}

// NonEmptyLines filters out empty/whitespace-only lines (blank lines are
// tolerated in board files) while retaining their index positions via
// IterRows below when needed.
func NonEmptyLines(lines [][]byte) [][]byte {
	var out [][]byte
	for _, l := range lines {
		if len(bytes.TrimSpace(l)) > 0 {
			out = append(out, l)
		}
	}
	return out
}

// IterParsed calls fn for every non-empty line with its parsed ordered Row.
// Blank lines are skipped. fn may return an error to abort.
func IterParsed(lines [][]byte, fn func(row *Row, lineIdx int, raw []byte) error) error {
	for i, l := range lines {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		row, err := ParseRow(l)
		if err != nil {
			return fmt.Errorf("line %d: %w", i+1, err)
		}
		if err := fn(row, i, l); err != nil {
			return err
		}
	}
	return nil
}

// ReadAllRows parses every non-empty line of a JSONL file into rows. The
// parallel raws slice holds the original line bytes (without newline).
func ReadAllRows(path string) (rows []*Row, raws [][]byte, err error) {
	lines, err := ReadJSONLLines(path)
	if err != nil {
		return nil, nil, err
	}
	err = IterParsed(lines, func(row *Row, _ int, raw []byte) error {
		rows = append(rows, row)
		raws = append(raws, raw)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	return rows, raws, nil
}

// HeaderRow returns the parsed line-1 header (topology A). Topology B (the
// header is line 1 of tasks.jsonl, a read-only legacy layout) yields a nil
// row — boardctl never reads a header out of tasks.jsonl.
func (b *Board) HeaderRow() (*Row, error) {
	if b.Topology != "A" {
		return nil, nil
	}
	lines, err := ReadJSONLLines(b.headerPath)
	if err != nil {
		return nil, err
	}
	for _, l := range lines {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		row, err := ParseRow(l)
		if err != nil {
			return nil, fmt.Errorf("board.jsonl line 1: %w", err)
		}
		return row, nil
	}
	return nil, fmt.Errorf("board.jsonl is empty (topology A requires a header row)")
}

// FixtureIDs returns the union of task ids that live in fixtures.jsonl
// (permanent fixture rows — never selectable as tasks). Missing fixtures file
// yields an empty set.
func (b *Board) FixtureIDs() (map[string]bool, error) {
	ids := map[string]bool{}
	if !fileExists(b.fixturesPath) {
		return ids, nil
	}
	_, raws, err := ReadAllRows(b.fixturesPath)
	if err != nil {
		return nil, fmt.Errorf("fixtures.jsonl: %w", err)
	}
	for _, raw := range raws {
		row, err := ParseRow(raw)
		if err != nil {
			continue
		}
		if id := row.String("id"); id != "" {
			ids[id] = true
		}
	}
	return ids, nil
}
