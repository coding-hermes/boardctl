package board

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Board locates one JSONL-canonical foreman board and its tracked files.
type Board struct {
	Dir      string // absolute board dir (.coding-hermes/board or equivalent)
	Topology string // "A" (board.jsonl header present) or "B"

	headerPath   string // board.jsonl (topology A only)
	tasksPath    string // tasks.jsonl
	eventsPath   string // events.jsonl
	fixturesPath string // fixtures.jsonl (optional)

	// headerless marks a board with NO board.jsonl whose tasks.jsonl line 1
	// is not header-shaped (BT-037) — a board holding just task rows. The
	// topology stays "B" (there is no board.jsonl), but header reads and
	// writes refuse instead of treating line 1 of tasks.jsonl as the header.
	headerless bool
}

// ErrBoardNotFound is wrapped with the directories probed. It is also the
// sentinel for a PARTIAL board (tasks.jsonl or events.jsonl present, its pair
// missing — BT-026), so the CLI keeps the documented exit-2 class for that
// case while the wrapped text names the detected file and the missing one.
var ErrBoardNotFound = errors.New("no JSONL foreman board found")

// ErrNoHeader is the sentinel for a HEADERLESS board: no board.jsonl AND
// tasks.jsonl line 1 is an ordinary task row rather than board metadata
// (BT-037). Such a board has no header to read and no header row to rewrite,
// so every header operation refuses loudly instead of stamping header keys
// (ticks_total/ticks_idle/last_commit/updated_at) into a TASK row.
var ErrNoHeader = errors.New("no header on this board")

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
	var partial string // a candidate holding exactly one of the required pair (BT-026)
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
				// BT-037: topology B means "the header is line 1 of
				// tasks.jsonl" — but only when that line is header-SHAPED.
				// A board with no board.jsonl whose line 1 is an ordinary
				// task row is HEADERLESS: classifying it as writable
				// topology B is exactly what let `header --set-*` rewrite a
				// task row. Reads keep working (task enumeration skips line
				// 1 only when it IS header-shaped), header operations
				// refuse.
				b.Topology = "B"
				b.headerless = !boardHasLineOneHeader(tasks)
			}
			return b, nil
		}
		// BT-026: a candidate with exactly one of the pair is a partial /
		// legacy board, not "nothing here". The FIRST partial candidate
		// (probing order) wins so the hint points at the shallowest board.
		if partial == "" {
			if fileExists(tasks) {
				partial = fmt.Sprintf("%s (a tasks.jsonl-only board: events.jsonl is missing)", tasks)
			} else if fileExists(events) {
				partial = fmt.Sprintf("%s (an events.jsonl-only board: tasks.jsonl is missing)", events)
			}
		}
	}
	if partial != "" {
		return nil, fmt.Errorf("%w: %s — found %s",
			ErrBoardNotFound, target, partial)
	}
	return nil, fmt.Errorf("%w: %s (looked for tasks.jsonl+events.jsonl in %s)",
		ErrBoardNotFound, target, strings.Join(cands, ", "))
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// firstNonBlankLine returns the first non-blank line of a file, trimmed of
// surrounding whitespace and without its terminating newline. ok is false for
// an unreadable, empty, or all-blank file. Only the first line is read, so the
// probe stays cheap on large boards.
func firstNonBlankLine(path string) ([]byte, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
			return trimmed, true
		}
		if err != nil {
			return nil, false
		}
	}
}

// boardHasLineOneHeader reports whether line 1 of tasks.jsonl is a board
// header row (metadata without a task id) — the topology-B header location.
//
// An empty/all-blank file has no header. An UNPARSEABLE line 1 is reported as
// "yes" on purpose: it is not classified as headerless, so the read and write
// paths surface the parse failure itself (the pretty-printed-JSON hint, a
// truncated line, ...) instead of masking it with a shape verdict, and
// SetHeader's task-row guard still refuses to touch any row carrying task keys.
func boardHasLineOneHeader(tasksPath string) bool {
	line, ok := firstNonBlankLine(tasksPath)
	if !ok {
		return false
	}
	row, err := ParseRow(line)
	if err != nil {
		return true
	}
	return rowIsHeaderShape(row)
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

// IsTopologyA reports whether the header lives in board.jsonl (the modern
// layout). Topology B — the header is line 1 of tasks.jsonl — is fully
// writable too; only `init` treats the topologies differently (it bootstraps
// fresh boards and refuses to layer a new board.jsonl over an existing
// topology-B header).
func (b *Board) IsTopologyA() bool { return b.Topology == "A" }

// HasHeader reports whether the board carries a header row: always true on
// topology A (board.jsonl), and on topology B only when line 1 of
// tasks.jsonl is header-shaped. A board with neither (no board.jsonl, line 1
// is a task row — BT-037) is HEADERLESS: reads still work, header operations
// refuse.
func (b *Board) HasHeader() bool { return !b.headerless }

// noHeaderError builds the "no header on this board" refusal for a headerless
// board, naming the file and the task row that a header operation would
// otherwise corrupt.
func (b *Board) noHeaderError() error {
	name := filepath.Base(b.tasksPath)
	if line, ok := firstNonBlankLine(b.tasksPath); ok {
		if row, err := ParseRow(line); err == nil {
			if id := row.String("id"); id != "" {
				return fmt.Errorf("%w: %s holds no board.jsonl and line 1 of %s is the TASK row %q — a header operation there would rewrite that task row, so it is refused (create a board.jsonl header, or run 'boardctl init' on a fresh board)",
					ErrNoHeader, b.Dir, name, id)
			}
		}
	}
	return fmt.Errorf("%w: %s holds no board.jsonl and line 1 of %s is not a board header row — nothing to read or rewrite (create a board.jsonl header, or run 'boardctl init' on a fresh board)",
		ErrNoHeader, b.Dir, name)
}

// headerPathFor returns the file that carries the board header: board.jsonl
// on topology A, tasks.jsonl (line 1) on topology B. It is only meaningful
// when HasHeader() is true — callers that write must check first (SetHeader
// does), because on a headerless board this path is a file full of TASK rows.
func (b *Board) headerPathFor() string {
	if b.IsTopologyA() {
		return b.headerPath
	}
	return b.tasksPath
}

// rowIsHeaderShape reports whether a parsed row looks like a board header
// (metadata without a task id): no "id" plus at least one of the header
// identity keys. Shared by init's topology-B detection and the task-row
// enumeration that must skip line 1 on topology B.
func rowIsHeaderShape(row *Row) bool {
	return row.String("id") == "" &&
		(row.Has("project") || row.Has("namespace") || row.Has("version") || row.Has("ticks_total"))
}

// rowIsTaskShape reports whether a parsed row carries task identity keys.
// BT-037: SetHeader's hard guard uses it — a header rewrite must never land
// in a row that also holds id/title/status, whatever the topology says.
func rowIsTaskShape(row *Row) bool {
	return row.Has("id") || row.Has("title") || row.Has("status")
}

// skipTaskLine reports whether the raw line at lines[idx] is a header row
// (topology-B line 1) rather than a task row. Task-line enumeration on
// topology B skips it; line indices remain file-absolute so callers can map
// back to the exact file line.
func (b *Board) skipTaskLine(lines [][]byte, idx int) bool {
	if b.IsTopologyA() || idx != 0 {
		return false
	}
	row, err := ParseRow(bytes.TrimSpace(lines[0]))
	return err == nil && rowIsHeaderShape(row)
}

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
//
// BT-026: when a line fails to parse with the decoder's EOF signature AND the
// joined lines form one valid JSON document, the source is pretty-printed
// JSON — one object spread over multiple physical lines — and the error says
// so with the migration hint. A truncated file (e.g. cut off mid-object) also
// fails with EOF but is NOT valid JSON as a whole, so it keeps the plain
// per-line error; arbitrary corrupt JSON is never mislabeled as valid.
func IterParsed(lines [][]byte, fn func(row *Row, lineIdx int, raw []byte) error) error {
	for i, l := range lines {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		row, err := ParseRow(l)
		if err != nil {
			if isEOFJSONErr(err) && wholeFileIsValidJSON(lines) {
				return fmt.Errorf("line %d: %w (the source is valid JSON, but it is not line-loadable JSONL — JSONL requires one complete JSON object per line; pretty-printed objects span lines. Compact the file to one line per object, e.g. with jq -c)", i+1, err)
			}
			return fmt.Errorf("line %d: %w", i+1, err)
		}
		if err := fn(row, i, l); err != nil {
			return err
		}
	}
	return nil
}

// isEOFJSONErr reports whether err is the decoder's EOF-class failure — the
// signature of a line that ends mid-JSON-value (pretty-printed multi-line
// JSON, or a truncated file). ParseRow wraps the token error ("not valid
// JSON: ..."), so match the underlying io.EOF through the chain.
func isEOFJSONErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, io.EOF) || strings.Contains(err.Error(), "unexpected end of JSON input")
}

// wholeFileIsValidJSON reports whether the joined non-empty lines parse as
// exactly ONE complete JSON document (the pretty-print signature: the file is
// valid JSON, only not line-oriented). Leading whitespace (the "{\n  " first
// line of pretty output) is fine for the decoder.
func wholeFileIsValidJSON(lines [][]byte) bool {
	var joined bytes.Buffer
	for _, l := range lines {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		joined.Write(l)
		joined.WriteByte('\n')
	}
	dec := json.NewDecoder(bytes.NewReader(joined.Bytes()))
	var v any
	if err := dec.Decode(&v); err != nil {
		return false
	}
	// exactly one value: anything after it must be whitespace only
	rest := joined.Bytes()[dec.InputOffset():]
	return len(bytes.TrimSpace(rest)) == 0
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

// HeaderRow returns the parsed board header row. Topology A reads line 1 of
// board.jsonl; topology B (legacy layout) reads line 1 of tasks.jsonl — the
// metadata row that shares the file with the task rows. BT-010: topology B
// is fully writable (SetHeader rewrites line 1 in place), so the header is
// read the same way in both topologies.
//
// BT-037: a HEADERLESS board (no board.jsonl, line 1 of tasks.jsonl is a task
// row) has no header to read — it fails with ErrNoHeader rather than handing
// a task row back as if it were the header.
func (b *Board) HeaderRow() (*Row, error) {
	if !b.HasHeader() {
		return nil, b.noHeaderError()
	}
	path := b.headerPathFor()
	lines, err := ReadJSONLLines(path)
	if err != nil {
		return nil, err
	}
	for _, l := range lines {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		row, err := ParseRow(l)
		if err != nil {
			return nil, fmt.Errorf("%s line 1: %w", filepath.Base(path), err)
		}
		return row, nil
	}
	return nil, fmt.Errorf("%s is empty (a board header row is required)", filepath.Base(path))
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
