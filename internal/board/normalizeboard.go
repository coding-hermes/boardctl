package board

// DF-BOARDCTL-36: the drift-normalization/repair engine for boards that
// drifted outside the declared canon (the measured warpfs shape, 2026-10-13:
// 377 rows / 83 distinct keys, 127 rows with off-canon keys, 38 null
// statuses, 27 string-valued depends_on, 158 rows with no created_at — the
// report parser excluded most of the board from time metrics).
//
// The engine follows every refusal discipline the package already owns:
//
//   - KEY ALIASES via the existing BT-056 table (keys.go CanonicalKeyFor /
//     Row.RenameKey): acceptance -> acceptance_criteria, why -> reasoning,
//     ... RenameKey refuses when the canonical key is already present (two
//     values, one slot) or the alias appears twice; those rows are reported
//     as REFUSED, never mutated.
//   - STRING depends_on (the BT-035/BT-055 shape drift): "[]" and ""
//     become [], a JSON array string '["A","B"]' becomes the real array,
//     and a single bare id "GAP-077" becomes ["GAP-077"]. A number, object,
//     or unparseable value is ambiguous — reported, never guessed.
//   - STATUS ALIASES via BT-025 ResolveStatus: open -> pending,
//     done -> complete, closed -> complete (DF-BOARDCTL-41). An
//     absent/null status or an unknown value (retired, duplicate, wip)
//     needs a human decision — never written.
//   - missing created_at is REPORTED ONLY (the task is explicit: tolerate,
//     do not fabricate a birth timestamp).
//
// DEFAULT is a DRY-RUN: NormalizeBoard classifies every row and writes
// NOTHING. Only --apply mutates, through ONE guarded atomic rewrite of
// tasks.jsonl (no per-row rewrite loop): every repair is classified first,
// any surprise aborts before the file is touched, untouched lines are
// asserted byte-identical, and the write guard serializes against sibling
// writers. No audit event is appended — this is a spelling/shape repair of
// values already on the board (the NormalizeTask discipline), and the
// printed change list is the audit trail.
//
// Untracked repair CACHE files (tasks.normalized.jsonl) are written only in
// dry-run, hold exactly the bytes --apply would write, and are validated
// against the doctor's untracked-fossil rule the way the existing
// tasks.rewritten.jsonl cache is.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// normalizeBoardCacheName is the dry-run review file written under the
// board dir (same family as DF-BOARDCTL-9's tasks.rewritten.jsonl).
const normalizeBoardCacheName = "tasks.normalized.jsonl"

// NormalizeRowChange is one mechanical repair NormalizeBoard classified (or
// applied) on one row. Kind names the repair family: "key" (alias rename),
// "depends_on" (string -> array), "status" (alias -> canonical).
type NormalizeRowChange struct {
	ID    string `json:"id"`
	Line  int    `json:"line"` // 1-based physical line in tasks.jsonl
	Kind  string `json:"kind"`
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// NormalizeRowRefusal is one row NormalizeBoard would not touch, with the
// reason: ambiguous values and alias-vs-canon conflicts need a human
// decision, and a refusal never blocks the mechanical repairs on other rows.
type NormalizeRowRefusal struct {
	ID     string `json:"id"`
	Line   int    `json:"line"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// NormalizeReportNote is one board-wide observation that is not a row
// repair — today, the missing-created_at census (reported, never repaired).
type NormalizeReportNote struct {
	Kind   string `json:"kind"` // "missing_created_at" today
	Rows   int    `json:"rows"`
	IDs    string `json:"ids"` // first few, for the human report
	Detail string `json:"detail"`
}

// NormalizeBoardReport is the full census of one NormalizeBoard run.
type NormalizeBoardReport struct {
	BoardPath string                `json:"board"`
	Rows      int                   `json:"rows"`
	Changes   []NormalizeRowChange  `json:"changes"`
	Refusals  []NormalizeRowRefusal `json:"refusals"`
	Notes     []NormalizeReportNote `json:"notes"`
	Applied   bool                  `json:"applied"`
	// AppliedPath is set on apply: the file that was rewritten (tasks.jsonl).
	// On a dry-run it names the review cache file written (or "" when the
	// board was already canonical and nothing needed writing).
	AppliedPath string `json:"applied_path,omitempty"`
	Clean       bool   `json:"clean"` // zero changes, zero refusals: already canon
}

// depParse is the outcome of classifying one stored depends_on value.
type depParse struct {
	// arr is the canonical array value when ok=true.
	arr []string
	// ok reports whether the value converts to an array mechanically.
	ok bool
	// report is the human clause for a refusal (why it is ambiguous).
	report string
}

// normalizeDependsOn classifies one stored depends_on value (raw JSON bytes)
// for repair. Array stays as-is (nothing to do — ok=true, arr=nil means
// "already an array"). String converts: "[]" and "" to the empty array, a
// JSON array string to the real array, a single bare id to a one-element
// array. Anything else (number, boolean, object, malformed JSON) is refused
// with a reason — guessing a dependency list from those shapes would invent
// the board's history.
func normalizeDependsOn(raw json.RawMessage) depParse {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return depParse{report: "stored value is not valid JSON — cannot classify"}
	}
	switch t := v.(type) {
	case []any:
		return depParse{ok: true} // already canonical shape
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return depParse{ok: true, arr: []string{}}
		}
		// A JSON array spelled as a string ("[]" or '["A","B"]').
		dec2 := json.NewDecoder(strings.NewReader(s))
		dec2.UseNumber()
		var inner any
		if err := dec2.Decode(&inner); err == nil {
			if elems, isArr := inner.([]any); isArr {
				out := make([]string, 0, len(elems))
				for _, e := range elems {
					es, isStr := e.(string)
					if !isStr || strings.TrimSpace(es) == "" {
						return depParse{report: fmt.Sprintf("stringified array holds a non-string element %s — cannot convert", mustJSON(e))}
					}
					out = append(out, es)
				}
				return depParse{ok: true, arr: out}
			}
			// A JSON value that is not an array (a number, an object…)
			// inside the string: ambiguous, refuse.
			return depParse{report: fmt.Sprintf("string holds a JSON %s, not an array — cannot convert", jsonKindName(mustJSONRaw(inner)))}
		}
		// Not JSON at all: a bare id reference ("GAP-077", the BT-019
		// shape) is the one non-array value with an unambiguous meaning —
		// but ONLY when it is shaped like a fleet task id. Anything else
		// (prose, "memory, duckdb", "[GAP-000") is ambiguous: converting it
		// would invent a dependency that was never there.
		if !MatchesFleetTaskID(strings.TrimSpace(t)) {
			return depParse{report: fmt.Sprintf("string value %q is neither a JSON array nor a fleet task id — cannot convert without guessing", t)}
		}
		return depParse{ok: true, arr: []string{t}}
	default:
		return depParse{report: fmt.Sprintf("stored value is a JSON %s — cannot convert to an array without guessing", jsonKindName(raw))}
	}
}

// mustJSON renders a decoded Go value back to compact JSON (test-helper
// grade: only called on values json just produced).
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "?"
	}
	return string(b)
}

// mustJSONRaw is mustJSON for the jsonKindName helper's byte interface.
func mustJSONRaw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// NormalizeBoardRepairHint is the inline remedy attached to normalize-board's
// fail-closed refusal on an unparseable tasks.jsonl line (DF-BOARDCTL-46).
// normalize-board is a ROW-SHAPE repair, not a salvage tool: it never rewrites
// a board it could not fully read, and the only safe path to a readable board
// is validate --repair (write a review file) followed by a deliberate manual
// swap. The hint names every step in order so the refusal is actionable where
// it is printed.
const NormalizeBoardRepairHint = "remedy: run 'boardctl validate --repair' (salvages every parseable row into tasks.rewritten.jsonl), review tasks.rewritten.jsonl, swap the reviewed file into tasks.jsonl, then re-run 'boardctl normalize-board'"

// NormalizeBoard classifies every task row against the canon and, when
// apply=true, rewrites tasks.jsonl with every mechanical repair in ONE
// guarded atomic rewrite. Dry-run (apply=false) writes the would-be bytes to
// tasks.normalized.jsonl under the board dir for review — unless the board
// is already clean, in which case nothing is written at all.
//
// FAIL-CLOSED on an unparseable line: the whole command refuses (naming the
// file and the 1-based line number) and nothing is written in either mode.
// There is no degraded writer here by design — a row-shape repair must never
// write a board it could not read in full — so the refusal carries
// NormalizeBoardRepairHint rather than a --skip-bad-lines escape hatch.
//
// Refusal rules (never mutate, always report):
//   - an alias whose canonical key is already present on the row, or an
//     alias key appearing twice (RenameKey would refuse — surfaced instead);
//   - a depends_on value that cannot be converted without guessing;
//   - a status that is absent, null, or unknown (BT-025: never coerced).
//
// A row carrying a refusal keeps its OTHER mechanical repairs unapplied: a
// refusal marks the WHOLE row untouched, so the operator resolves it by hand
// without half-repaired rows. Rows with only mechanical repairs convert
// together.
func (b *Board) NormalizeBoard(apply bool) (*NormalizeBoardReport, error) {
	lines, err := ReadJSONLLines(b.tasksPath)
	if err != nil {
		return nil, err
	}
	rep := &NormalizeBoardReport{BoardPath: b.tasksPath}

	// Pass 1 — classify (no mutation). newLines[i] != nil marks a repaired
	// line; a refusal keeps the index out of the repair set entirely.
	newLines := make([][]byte, len(lines))
	missingCreated := []string{}
	noteSeen := map[string]bool{}

	classifyRow := func(row *Row, idx int) {
		id := row.String("id")
		line := idx + 1
		var rowChanges []NormalizeRowChange
		var rowRefusals []NormalizeRowRefusal

		// (a) key aliases — the BT-056 table. Classify every rename;
		// RenameKey's destructive cases become row refusals: the canonical
		// key already present on the row, an alias key appearing twice, and
		// TWO DIFFERENT aliases of the SAME canonical key on one row (a
		// measured warpfs shape: `acceptance` + `_pass_criteria` together)
		// — two values, one slot, a human decides which survives.
		aliasOnce := map[string]bool{}
		targetClaimed := map[string]string{} // canonical target -> claiming alias
		for _, k := range row.Keys {
			target, isAlias := CanonicalKeyFor(k)
			if !isAlias {
				continue
			}
			switch {
			case row.Has(target):
				rowRefusals = append(rowRefusals, NormalizeRowRefusal{ID: id, Line: line, Kind: "key",
					Detail: fmt.Sprintf("alias %q maps to %q which the row already carries — decide which value survives by hand", k, target)})
			case aliasOnce[k]:
				rowRefusals = append(rowRefusals, NormalizeRowRefusal{ID: id, Line: line, Kind: "key",
					Detail: fmt.Sprintf("alias key %q appears more than once — refusing ambiguous rename", k)})
			case targetClaimed[target] != "":
				rowRefusals = append(rowRefusals, NormalizeRowRefusal{ID: id, Line: line, Kind: "key",
					Detail: fmt.Sprintf("aliases %q and %q both map to %q on this row — decide which value survives by hand", targetClaimed[target], k, target)})
			default:
				aliasOnce[k] = true
				targetClaimed[target] = k
				rowChanges = append(rowChanges, NormalizeRowChange{ID: id, Line: line, Kind: "key", Field: k, From: k, To: target})
			}
		}

		// (b) depends_on shape drift (BT-035/BT-055): every non-array,
		// non-null stored shape classifies. String converts when
		// unambiguous; number/boolean/object/malformed refuse. Absent and
		// null stay silent (the write path's own no-dependencies spelling).
		if raw := row.Get("depends_on"); raw != nil {
			if kind := jsonKindName(raw); kind != "" && kind != "array" && kind != "null" {
				p := normalizeDependsOn(raw)
				if p.ok {
					rowStyle := DetectStyle(lines[idx])
					rowChanges = append(rowChanges, NormalizeRowChange{ID: id, Line: line, Kind: "depends_on", Field: "depends_on",
						From: string(bytes.TrimSpace(raw)), To: renderStringSlice(p.arr, rowStyle)})
				} else {
					rowRefusals = append(rowRefusals, NormalizeRowRefusal{ID: id, Line: line, Kind: "depends_on", Detail: p.report})
				}
			}
		}

		// (c) status aliases (BT-025). Absent/null/unknown: refusal, never
		// guessed.
		st := row.String("status")
		if st == "" {
			rowRefusals = append(rowRefusals, NormalizeRowRefusal{ID: id, Line: line, Kind: "status",
				Detail: "status is absent or null — set the real status by hand (never guessed)"})
		} else if canonical, ok, alias := ResolveStatus(st); !ok {
			rowRefusals = append(rowRefusals, NormalizeRowRefusal{ID: id, Line: line, Kind: "status",
				Detail: fmt.Sprintf("status %q is not in the vocabulary or the alias table — needs an explicit decision", st)})
		} else if alias {
			rowChanges = append(rowChanges, NormalizeRowChange{ID: id, Line: line, Kind: "status", Field: "status", From: st, To: canonical})
		}

		// (d) missing created_at — REPORT ONLY (never fabricated).
		if raw := row.Get("created_at"); raw == nil || string(bytes.TrimSpace(raw)) == "null" {
			if !noteSeen[id] {
				noteSeen[id] = true
				missingCreated = append(missingCreated, id)
			}
		}

		if len(rowRefusals) > 0 {
			rep.Refusals = append(rep.Refusals, rowRefusals...)
			return // the whole row stays untouched
		}
		if len(rowChanges) > 0 {
			rep.Changes = append(rep.Changes, rowChanges...)
			newLines[idx] = []byte{} // marker: this line needs a rewrite
		}
	}

	// DF-BOARDCTL-46: fail closed on an unparseable line in BOTH modes — the
	// file cannot be classified as a whole, and this command has no degraded
	// writer. The error keeps the file + 1-based line number from IterParsed
	// (e.g. a duplicate-key line reports `line N: duplicate key "status" in
	// row`) and appends the step-by-step remedy, so the refusal is actionable
	// without opening the README.
	if err := IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		if b.skipTaskLine(lines, idx) {
			return nil
		}
		classifyRow(row, idx)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("%s: %w — %s", b.tasksPath, err, NormalizeBoardRepairHint)
	}

	if len(missingCreated) > 0 {
		shown := missingCreated
		if len(shown) > 5 {
			shown = shown[:5]
		}
		rep.Notes = append(rep.Notes, NormalizeReportNote{
			Kind:   "missing_created_at",
			Rows:   len(missingCreated),
			IDs:    strings.Join(shown, ", "),
			Detail: "rows without a parseable created_at are excluded from time-based metrics — add real timestamps by hand; the repair never fabricates one",
		})
	}

	rep.Rows = countTaskRows(b, lines)
	rep.Clean = len(rep.Changes) == 0 && len(rep.Refusals) == 0
	if len(rep.Changes) == 0 {
		// Nothing mechanical to do — refusals ride the report only, and no
		// file is written in either mode (a refusal-only dry-run must not
		// stage "would-be bytes" identical to the input, and an apply with
		// zero changes must not rewrite).
		return rep, nil
	}

	// Pass 2 — build the rewritten lines from the classification.
	for idx := range newLines {
		if newLines[idx] == nil {
			continue
		}
		row, err := ParseRow(lines[idx])
		if err != nil {
			return nil, fmt.Errorf("internal error: line %d classified but no longer parses: %w", idx+1, err)
		}
		rstyle := DetectStyle(lines[idx])
		id := row.String("id")
		for _, ch := range rep.Changes {
			if ch.Line != idx+1 || ch.ID != id {
				continue
			}
			switch ch.Kind {
			case "key":
				if err := row.RenameKey(ch.From, ch.To); err != nil {
					return nil, fmt.Errorf("internal error: line %d: %w", idx+1, err)
				}
			case "depends_on":
				var arr []string
				dec := json.NewDecoder(strings.NewReader(ch.To))
				if err := dec.Decode(&arr); err != nil {
					return nil, fmt.Errorf("internal error: line %d: decoded change %q is not a JSON array: %w", idx+1, ch.To, err)
				}
				// Drop a no-op rewrite: an already-canonical array string
				// (From == To) must round-trip byte-identical, so only a
				// value whose SHAPE actually changes is re-encoded.
				if ch.From == ch.To {
					continue
				}
				if err := row.SetGoValue("depends_on", arr, rstyle); err != nil {
					return nil, err
				}
			case "status":
				if err := row.SetGoValue("status", ch.To, rstyle); err != nil {
					return nil, err
				}
			}
		}
		newLines[idx] = row.Marshal(rstyle)
	}

	// Assemble: repaired lines replace their originals; everything else
	// round-trips byte-identical (asserted, then written).
	out := make([][]byte, len(lines))
	for i := range lines {
		if newLines[i] != nil {
			out[i] = newLines[i]
			continue
		}
		out[i] = lines[i]
	}
	for i := range lines {
		if newLines[i] != nil {
			continue
		}
		if !bytes.Equal(out[i], lines[i]) {
			return nil, fmt.Errorf("internal error: untouched line %d would change — %s aborted (nothing written)", i+1, opName(apply))
		}
	}
	rewritten := JoinLines(out)

	if !apply {
		// Dry-run: write the would-be bytes to the review cache file so the
		// operator can diff before committing to the repair (the same
		// manual-review contract as validate --repair). The cache is
		// UNTRACKED doctor-fossil material by design.
		cache := b.normalizeBoardCachePath()
		if err := os.WriteFile(cache, rewritten, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", cache, err)
		}
		rep.AppliedPath = cache
		return rep, nil
	}

	// Apply: one guarded atomic rewrite — the whole classified batch lands
	// or nothing does (same discipline as normalizeTaskLocked, board-wide).
	var applyErr error
	lockErr := b.withWriteLock(b.tasksPath, func() error {
		applyErr = atomicRewrite(b.tasksPath, rewritten)
		return nil // err rides applyErr; the guard must drop regardless
	})
	if lockErr != nil {
		return nil, lockErr
	}
	if applyErr != nil {
		return nil, applyErr
	}
	rep.Applied = true
	rep.AppliedPath = b.tasksPath
	return rep, nil
}

// normalizeBoardCachePath is the dry-run review file's location.
func (b *Board) normalizeBoardCachePath() string {
	return b.Dir + string(os.PathSeparator) + normalizeBoardCacheName
}

// renderStringSlice renders []string as a JSON array in the given style
// (the encodeValue []string shape: ["a", "b"] spaced / ["a","b"] compact).
func renderStringSlice(xs []string, s Style) string {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, e := range xs {
		if i > 0 {
			buf.WriteByte(',')
			if s.Spaced {
				buf.WriteByte(' ')
			}
		}
		buf.Write(jsonString(e, s))
	}
	buf.WriteByte(']')
	return buf.String()
}

// opName names the mode for internal-error messages.
func opName(apply bool) string {
	if apply {
		return "normalize --apply"
	}
	return "normalize (dry-run)"
}

// countTaskRows counts the task rows the classification examined (topology-B
// header line excluded), mirroring validate's rep.Tasks census.
func countTaskRows(b *Board, lines [][]byte) int {
	n := 0
	for idx := range lines {
		if len(bytes.TrimSpace(lines[idx])) == 0 {
			continue
		}
		if b.skipTaskLine(lines, idx) {
			continue
		}
		n++
	}
	return n
}

// sortNormalizeRows orders change/refusal slices by line then kind, so the
// report reads in file order.
func sortNormalizeRows[T any](rows []T, line func(T) int, kind func(T) string) []T {
	if len(rows) == 0 {
		return nil
	}
	out := append([]T{}, rows...)
	sort.SliceStable(out, func(i, j int) bool {
		if line(out[i]) != line(out[j]) {
			return line(out[i]) < line(out[j])
		}
		return kind(out[i]) < kind(out[j])
	})
	return out
}

// RenderText renders the human report: one line per change and refusal in
// file order, the board-wide notes, then the mode summary naming what was
// (or would be) written.
func (r *NormalizeBoardReport) RenderText() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "board: %s\n", r.BoardPath)
	fmt.Fprintf(&sb, "rows: %d\n", r.Rows)

	r.Changes = sortNormalizeRows(r.Changes,
		func(c NormalizeRowChange) int { return c.Line },
		func(c NormalizeRowChange) string { return c.Kind })
	r.Refusals = sortNormalizeRows(r.Refusals,
		func(x NormalizeRowRefusal) int { return x.Line },
		func(x NormalizeRowRefusal) string { return x.Kind })

	for _, ch := range r.Changes {
		fmt.Fprintf(&sb, "  line %d  %s: %s %s\n", ch.Line, ch.ID, ch.Kind, renderChangeArrow(ch))
	}
	for _, rf := range r.Refusals {
		fmt.Fprintf(&sb, "  line %d  %s: REFUSED %s — %s\n", rf.Line, rf.ID, rf.Kind, rf.Detail)
	}
	for _, n := range r.Notes {
		fmt.Fprintf(&sb, "  note: %s on %d row(s) (%s…) — %s\n", n.Kind, n.Rows, n.IDs, n.Detail)
	}
	if r.Clean {
		sb.WriteString("board already canonical — nothing to change (nothing written)\n")
		return sb.String()
	}
	if len(r.Changes) == 0 {
		// Refusal-only run: nothing mechanical exists, nothing was written
		// in either mode — the refusals are the whole deliverable.
		fmt.Fprintf(&sb, "%d row(s) refused (left untouched) — nothing mechanical to repair, no file written; decide the refused rows by hand\n",
			countRefusedRows(r.Refusals))
		return sb.String()
	}
	fmt.Fprintf(&sb, "%d mechanical repair(s) across %d row(s); %d row(s) refused (left untouched)\n",
		len(r.Changes), countDistinctIDs(r.Changes), countRefusedRows(r.Refusals))
	if r.Applied {
		fmt.Fprintf(&sb, "applied: %s rewritten in one atomic pass; validate the board next\n", r.AppliedPath)
	} else {
		fmt.Fprintf(&sb, "DRY-RUN — no board file written; would-be bytes staged at %s for review (pass --apply to write them to tasks.jsonl)\n", r.AppliedPath)
	}
	return sb.String()
}

// renderChangeArrow formats one change's from -> to clause.
func renderChangeArrow(ch NormalizeRowChange) string {
	if ch.Kind == "key" {
		return fmt.Sprintf("key %q -> %q", ch.From, ch.To)
	}
	return fmt.Sprintf("%s %s -> %s", ch.Field, ch.From, ch.To)
}

// countDistinctIDs counts the distinct task ids in a change list.
func countDistinctIDs(changes []NormalizeRowChange) int {
	seen := map[string]bool{}
	for _, c := range changes {
		seen[c.ID] = true
	}
	return len(seen)
}

// countRefusedRows counts the distinct (line, id) refusal rows.
func countRefusedRows(refusals []NormalizeRowRefusal) int {
	seen := map[string]bool{}
	for _, rf := range refusals {
		seen[fmt.Sprintf("%d/%s", rf.Line, rf.ID)] = true
	}
	return len(seen)
}
