package board

// DF-BOARDCTL-9 Part 2: validate --repair.
//
// ReReadTolerant re-reads the board's JSONL files with the tolerant reader
// (bad lines ride the skipped-line evidence, salvageable rows are kept) and
// Repair re-serializes the salvageable rows into <boarddir>/tasks.rewritten.jsonl
// for MANUAL REVIEW. Nothing under the board dir is overwritten: tasks.jsonl
// is never touched by the repair path — the operator diffs and applies.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RepairResult reports what one Repair run salvaged and dropped.
type RepairResult struct {
	// OutputPath is the review file the salvageable rows were written to.
	OutputPath string
	// Salvaged is the number of rows carried into the review file.
	Salvaged int
	// Dropped is the number of lines that could not be parsed.
	Dropped int
	// DroppedLines lists the 1-based line numbers dropped, in file order.
	DroppedLines []int
}

// reReadTolerant re-reads one file with the tolerant reader, returning the
// salvageable rows in file order.
func (b *Board) reReadTolerant(path string) (rows []*Row, err error) {
	lines, err := ReadJSONLLines(path)
	if err != nil {
		return nil, err
	}
	b.iterParsedTolerant(lines, path, func(row *Row, _ int, _ []byte) error {
		rows = append(rows, row)
		return nil
	})
	return rows, nil
}

// styleForFile detects the serialization style to re-serialize rows in: the
// style of the file's last non-empty line (python json.dumps conventions —
// see DetectStyle), falling back to the python-default style when the file is
// empty or carries no parseable sample.
func (b *Board) styleForFile(path string) Style {
	raw := rawLastLine(path)
	if raw == nil {
		return DefaultStyle()
	}
	st := DetectStyle(raw)
	return st
}

// Repair re-reads tasks.jsonl tolerantly, re-serializes each salvageable row
// in the board's own serialization style, and writes the result to
// <boarddir>/tasks.rewritten.jsonl — a REVIEW file. It never touches
// tasks.jsonl.
//
// A topology-B board's line-1 header row is salvageable data like any other,
// so it is carried into the review file too; the operator reviews before
// applying anything.
//
// GR-150 — the recovery path guards the data it cannot see:
//
//  1. REFUSAL. A physical line carrying more than one JSON value (a fused
//     record) or trailing data after the object holds bytes that belong to a
//     record no parser can recover from the line alone. Normalizing such a
//     line would keep the first object and silently DROP the rest — exactly
//     the data loss this repair exists to prevent — so ambiguous lines refuse
//     the repair outright instead of guessing. (Truncated fragments and
//     embedded-newline debris are a different class: they carry no hidden
//     records and stay salvageable, reported as dropped lines.)
//  2. COMPARISON. Before the rewrite the salvage stream is captured; after it,
//     the review file is re-read STRICTLY and its row-id stream must equal the
//     source's row-id stream position-for-position (duplicates included), and
//     events.jsonl must be byte-identical. A mismatch aborts the run as a
//     failed repair rather than reporting success on a file that lost or
//     invented a record.
func (b *Board) Repair() (*RepairResult, error) {
	// Repair always reads tolerantly — salvage is the point — and the
	// dropped-line report below reads the evidence this arms.
	b.SkipBad = true
	// Snapshot the untouched source first: the contract "repair does not
	// modify tasks.jsonl" is proven by tests asserting byte-identity, and
	// reading it up front means a mid-run write by a sibling process cannot
	// go unnoticed between the tolerant read and the style detection.
	before, err := os.ReadFile(b.tasksPath)
	if err != nil {
		return nil, fmt.Errorf("tasks.jsonl: %w", err)
	}
	eventsBefore, err := os.ReadFile(b.eventsPath)
	if err != nil {
		return nil, fmt.Errorf("events.jsonl: %w", err)
	}
	// GR-150 refusal: report every ambiguous line before writing anything.
	if ambiguous := fusedLineNumbers(bytes.Split(before, []byte("\n"))); len(ambiguous) > 0 {
		where := make([]string, 0, len(ambiguous))
		for _, n := range ambiguous {
			where = append(where, fmt.Sprintf("line %d", n))
		}
		return nil, fmt.Errorf("tasks.jsonl %s: %v / %v — refusing repair, because re-serializing such a line would keep the first object and silently drop the rest. Split the ambiguous line(s) into one JSON object per physical line first (e.g. `jq -c . <file >split`), then re-run",
			strings.Join(where, ", "), ErrMultipleJSONValues, ErrTrailingData)
	}
	rows, err := b.reReadTolerant(b.tasksPath)
	if err != nil {
		return nil, err
	}
	style := b.styleForFile(b.tasksPath)
	// The id stream the source's parseable rows carry, captured BEFORE the
	// rewrite for the after-the-fact comparison below.
	sourceIDs := rowIDStream(rows)

	var buf bytes.Buffer
	for _, row := range rows {
		buf.Write(row.Marshal(style))
		buf.WriteByte('\n')
	}
	out := filepath.Join(b.Dir, "tasks.rewritten.jsonl")
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("write %s: %w", out, err)
	}

	// The source must be byte-identical after the repair: this function
	// writes ONLY the review file.
	after, err := os.ReadFile(b.tasksPath)
	if err != nil {
		return nil, fmt.Errorf("tasks.jsonl (post-read): %w", err)
	}
	if !bytes.Equal(before, after) {
		return nil, fmt.Errorf("tasks.jsonl changed during repair (concurrent writer?) — review file written, refusing to report success")
	}
	eventsAfter, err := os.ReadFile(b.eventsPath)
	if err != nil {
		return nil, fmt.Errorf("events.jsonl (post-read): %w", err)
	}
	if !bytes.Equal(eventsBefore, eventsAfter) {
		return nil, fmt.Errorf("events.jsonl changed during repair (concurrent writer?) — the event records before and after the rewrite must match; refusing to report success")
	}
	// GR-150 comparison: the review file must carry exactly the row ids the
	// source carried, in order and with duplicates. It is read back and parsed
	// STRICTLY, so a review file that is itself fused or malformed fails here
	// rather than being handed to the operator.
	written, err := os.ReadFile(out)
	if err != nil {
		return nil, fmt.Errorf("re-read %s: %w", out, err)
	}
	gotIDs, err := strictIDStream(out, written)
	if err != nil {
		return nil, err
	}
	if !sameStringStream(sourceIDs, gotIDs) {
		return nil, fmt.Errorf("repair lost or invented a record: the source carries %d row id(s) %v but the review file carries %d %v — refusing to report success",
			len(sourceIDs), sourceIDs, len(gotIDs), gotIDs)
	}

	res := &RepairResult{OutputPath: out, Salvaged: len(rows)}
	for _, s := range b.SkippedLines() {
		if s.Path == b.tasksPath {
			res.Dropped++
			res.DroppedLines = append(res.DroppedLines, s.Line)
		}
	}
	return res, nil
}

// rowIDStream returns the id of every row in order (an id-less row keeps its
// position as "" so the streams stay comparable position-for-position).
func rowIDStream(rows []*Row) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.String("id"))
	}
	return out
}

// strictIDStream parses each non-empty line of a written JSONL file with the
// strict single-value reader and returns the row-id stream. Any line that is
// fused, malformed, or carries trailing data aborts the read.
func strictIDStream(path string, content []byte) ([]string, error) {
	var out []string
	for i, l := range bytes.Split(content, []byte("\n")) {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		row, err := ParseRow(l)
		if err != nil {
			return nil, fmt.Errorf("%s line %d does not hold exactly one JSON object: %w", filepath.Base(path), i+1, err)
		}
		out = append(out, row.String("id"))
	}
	return out, nil
}

// sameStringStream compares two id streams position-for-position.
func sameStringStream(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// RenderRepairText renders the operator-facing repair summary. It names the
// review file, the counts, the dropped line numbers, and says explicitly that
// the output is for manual review — the canonical tasks.jsonl is untouched.
func (r *RepairResult) RenderRepairText() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "repair: wrote %d salvaged row(s) to %s\n", r.Salvaged, r.OutputPath)
	fmt.Fprintf(&sb, "repair: dropped %d unparseable line(s)", r.Dropped)
	if len(r.DroppedLines) > 0 {
		fmt.Fprintf(&sb, " (tasks.jsonl line(s): %s)", joinInts(r.DroppedLines))
	}
	sb.WriteString("\n")
	sb.WriteString("review: tasks.rewritten.jsonl is FOR MANUAL REVIEW — tasks.jsonl was NOT modified. Diff, verify, then replace by hand if it looks right.\n")
	return sb.String()
}

// joinInts renders ints comma-separated.
func joinInts(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = fmt.Sprintf("%d", x)
	}
	return strings.Join(parts, ", ")
}
