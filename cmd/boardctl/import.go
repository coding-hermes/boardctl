package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/coding-hermes/boardctl/internal/board"
)

// BT-022 — `boardctl import <export.json> [--dry-run] [--renumber]`.
//
// Import round-trips a board-report/v1 export (the `render --json` payload,
// design authority: docs/specs/board-analytics-report.md) back into a board.
// Semantics fixed by spec 7.3 and the payload contract (6.2):
//
//   - The export's schema field must be exactly "board-report/v1"; anything
//     else fails with "unsupported schema" BEFORE anything is read further.
//   - The export is accepted only for the board it was rendered from: the
//     payload board name must match the target board's display name (header
//     project, else the board dir base). Mismatch names the expected target.
//   - Import touches task rows, event rows, and fixture ids ONLY. The board
//     header is never imported, overwritten, or counter-bumped (spec Q1
//     decision: headers are foreman-owned counters).
//   - Task rows: byte-identical rows (after stripping import provenance) are
//     no-ops; same-id rows whose content differs are SKIPPED and counted
//     ("N skipped (differs)") — import never rewrites an existing row.
//     --renumber instead appends the export row under the next free
//     fleet-style id for its prefix (same prefix, same digit width).
//   - New task rows are appended verbatim through the target's serialization
//     style (DetectStyle on the last tasks.jsonl line) and carry the
//     boardctl-web-export provenance appended to their reasoning (existing
//     reasoning is preserved, never destroyed). Each appended row gets a
//     task_created event, mirroring `boardctl create`.
//   - Events absent from the target (fingerprint on event_type/task_id/
//     actor/detail — exported ids are NEVER trusted) are appended with fresh
//     MAX(id)+1 ids in the target's own timestamp dialect.
//   - Fixture ids missing from an existing fixtures.jsonl are appended;
//     when the target has no fixtures.jsonl they are skipped with a plan
//     warning — import NEVER creates fixtures.jsonl.
//   - --dry-run prints the identical plan and writes nothing. The whole plan
//     (including row serialization and vocabulary validation) is built
//     before any byte is written, so a validation failure never leaves a
//     partial import behind. A second import of the same export is a true
//     no-op (idempotent).

// importProvenance is stamped into the reasoning field of every task row
// import appends. The canonical comparison strips it from both sides, so a
// restored row compares equal to its export row on the next import
// (idempotent round-trip) while the on-board row still records where it
// came from.
const importProvenance = "source: boardctl-web-export"

// importSchemaName is the only accepted export schema (6.2).
const importSchemaName = "board-report/v1"

// importExport / importBoard mirror the payload fields import consumes.
// Tasks and events stay json.RawMessage so each row can be parsed with
// board.ParseRow (verbatim field bytes, key order preserved).
type importExport struct {
	Schema     string        `json:"schema"`
	RenderedAt string        `json:"rendered_at"`
	Boards     []importBoard `json:"boards"`
}

type importBoard struct {
	Name       string            `json:"name"`
	Slug       string            `json:"slug"`
	Topology   string            `json:"topology"`
	Tasks      []json.RawMessage `json:"tasks"`
	Events     []json.RawMessage `json:"events"`
	FixtureIDs []string          `json:"fixture_ids"`
}

// cmdImport implements the import subcommand. Exit contract via run():
// 0 ok (plan printed; --dry-run wrote nothing), 1 error (unsupported schema,
// board mismatch, validation failure — nothing written), 2 usage.
func cmdImport(dir string, args []string) error {
	fs := newFlagSet("import")
	args = reorderArgs(args, valueFlags("C"))
	dryRun := fs.Bool("dry-run", false, "print the plan and write nothing")
	renumber := fs.Bool("renumber", false, "append conflicting rows (same id, different content) under the next free fleet-style id instead of skipping them")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "boardctl import <export.json> [--dry-run] [--renumber] [-C dir]\n")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if dir == "" {
		dir = cdir
	}
	if fs.NArg() != 1 {
		return usageErrorf("import requires exactly one export path (got %d)", fs.NArg())
	}
	exportPath := fs.Arg(0)

	raw, err := os.ReadFile(exportPath)
	if err != nil {
		return fmt.Errorf("read export: %w", err)
	}
	imp, bd, err := parseImportExport(raw)
	if err != nil {
		return err
	}

	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	targetName := boardDisplayName(b)
	if bd.Name != "" && bd.Name != targetName {
		return fmt.Errorf("board mismatch: export is for board %q; target board is %q (-C %s)", bd.Name, targetName, b.Dir)
	}

	plan, err := buildImportPlan(b, bd, *renumber)
	if err != nil {
		return err
	}
	printImportPlan(os.Stdout, exportPath, imp, targetName, b.Dir, plan, fixturesInfo{
		ids:      plan.fixtures,
		oldLines: fixturesOldLines(b),
		path:     b.FixturesPath(),
	})
	if *dryRun {
		fmt.Fprintf(os.Stdout, "dry-run: nothing written\n")
		return nil
	}
	return applyImportPlan(b, plan)
}

// parseImportExport decodes the payload and selects the board to import.
// Any decode failure, wrong schema, or empty boards array is rejected as
// "unsupported schema" / a payload error before the target board is touched.
func parseImportExport(raw []byte) (*importExport, *importBoard, error) {
	var imp importExport
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&imp); err != nil {
		return nil, nil, fmt.Errorf("unsupported schema: input is not a %s payload (%v)", importSchemaName, err)
	}
	if imp.Schema != importSchemaName {
		return nil, nil, fmt.Errorf("unsupported schema %q: import requires %s (the --json output of boardctl render)", imp.Schema, importSchemaName)
	}
	if len(imp.Boards) == 0 {
		return nil, nil, fmt.Errorf("unsupported schema: payload carries no boards array entries")
	}
	// Multi-board payloads (serve uploads) import their first board; board
	// identity is still verified against the target below.
	return &imp, &imp.Boards[0], nil
}

// boardDisplayName reproduces render's board naming rule: the header's
// project field when present, else the board dir's base name (internal/
// render/load.go loadBoard).
func boardDisplayName(b *board.Board) string {
	if hdr, err := b.HeaderRow(); err == nil && hdr != nil {
		if p := strings.TrimSpace(hdr.String("project")); p != "" {
			return p
		}
	}
	return filepath.Base(b.Dir)
}

// canonicalBoardFileNames are the tracked board file names used as diff
// headers in the plan output (relative to the board dir).
var canonicalBoardFileNames = map[string]string{
	"tasks":    "tasks.jsonl",
	"events":   "events.jsonl",
	"fixtures": "fixtures.jsonl",
}

// fixturesOldLines counts the non-empty lines of an existing fixtures.jsonl
// (0 when absent — import never creates the file).
func fixturesOldLines(b *board.Board) int {
	lines, err := board.ReadJSONLLines(b.FixturesPath())
	if err != nil {
		return 0
	}
	n := 0
	for _, l := range lines {
		if len(bytes.TrimSpace(l)) > 0 {
			n++
		}
	}
	return n
}

// plannedTask is one task row import will append. line is fully serialized
// (target style, provenance stamped, id already renumbered when applies).
// spec is the board.TaskRowSpec applyImportPlan hands to Board.Create —
// Create performs its full write contract (required fields, id format,
// status/priority vocabulary, duplicate-id and depends_on checks) and
// appends line verbatim via the spec's prevalidated Raw row.
type plannedTask struct {
	id       string
	exportID string // non-empty when the row was renumbered away from this id
	status   string
	line     []byte
	spec     board.TaskRowSpec
}

// importPlan is the complete preflight result: everything applyImportPlan
// writes, decided before the first byte lands.
type importPlan struct {
	tasks     []plannedTask
	identical []string // export ids already on the target byte-identically
	skipped   []string // export ids skipped (same id, different content)

	// tasksOldLines is the target tasks.jsonl non-empty line count at plan
	// time — the "-" side of the planned append-only unified diff.
	tasksOldLines int

	events      []board.ImportEvent
	fromExport  int // replayed export events
	taskCreated int // task_created events for the new rows

	// eventsOldLines is the target events.jsonl non-empty line count at
	// plan time — the "-" side of the planned append-only diff for
	// events.jsonl. exportTaskCreated counts the export's own task_created
	// rows (none of which replay: the target-side events are emitted by
	// Board.Create with fresh ids/dialect), printed so the plan total stays
	// verifiable against Create's emits.
	eventsOldLines    int
	exportTaskCreated int

	fixtures     []string // ids to append to an existing fixtures.jsonl
	fixturesSkip int      // fixture ids dropped because the target has no fixtures.jsonl
}

// importIDRe splits a fleet-style id into its prefix and numeric suffix for
// --renumber ("BT-022" -> "BT", "022").
var importIDRe = regexp.MustCompile(`^([A-Z][A-Z0-9]+(?:-[A-Z0-9]+)*)-([0-9]+)$`)

// nextFreeID returns the next unused id for id's prefix, preserving the
// export id's digit width (BT-005 -> BT-024 when BT-006..N are taken).
// Ids without a numeric suffix fall back to <id>-1, <id>-2, ...
func nextFreeID(used map[string]bool, id string) string {
	if m := importIDRe.FindStringSubmatch(id); m != nil {
		prefix, digits := m[1], m[2]
		if n, err := strconv.ParseUint(digits, 10, 63); err == nil {
			width := len(digits)
			for {
				n++
				cand := fmt.Sprintf("%s-%0*d", prefix, width, n)
				if !used[cand] {
					return cand
				}
			}
		}
	}
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s-%d", id, i)
		if !used[cand] {
			return cand
		}
	}
}

// buildImportPlan computes the entire import without writing anything:
// every appended row is serialized and vocabulary-checked here so that any
// validation failure aborts before the first byte is written.
func buildImportPlan(b *board.Board, bd *importBoard, renumber bool) (*importPlan, error) {
	plan := &importPlan{}

	// Target state (read-only).
	targetRows, err := b.TaskRows()
	if err != nil {
		return nil, err
	}
	plan.tasksOldLines = len(targetRows)
	target := map[string]*board.Row{}
	used := map[string]bool{}
	for _, r := range targetRows {
		if id := r.String("id"); id != "" {
			target[id] = r
			used[id] = true
		}
	}
	fixtureIDs, err := b.FixtureIDs()
	if err != nil {
		return nil, err
	}
	for id := range fixtureIDs {
		used[id] = true
	}
	targetStyle, err := tasksStyle(b)
	if err != nil {
		return nil, err
	}
	// eventsOldLines is the target events.jsonl non-empty line count at plan
	// time — the "-" side of the planned append-only diff for events.jsonl.
	// Import never rewrites or deletes event rows, so today's line count is
	// exactly the old-line count apply's appends start from.
	eventsLines, err := board.ReadJSONLLines(b.EventsPath())
	if err != nil {
		return nil, err
	}
	for _, l := range eventsLines {
		if len(bytes.TrimSpace(l)) > 0 {
			plan.eventsOldLines++
		}
	}
	// exportEventTypes counts the export's event rows by type; the plan
	// prints how many task_created events the export itself carries so the
	// total plan count is verifiable against Create's own emits.
	exportEventTypes := map[string]int{}
	for _, raw := range bd.Events {
		row, err := board.ParseRow(raw)
		if err != nil {
			// buildImportPlan's event loop reports this with the exact row
			// index; here it is only counted for the summary, so tolerate
			// and let that loop produce the error.
			continue
		}
		etype := row.String("event_type")
		if etype == "" {
			etype = "audit"
		}
		exportEventTypes[etype]++
	}
	plan.exportTaskCreated = exportEventTypes["task_created"]

	// ---- task rows ----
	for _, raw := range bd.Tasks {
		row, err := board.ParseRow(raw)
		if err != nil {
			return nil, fmt.Errorf("export task row: %w", err)
		}
		id := row.String("id")
		if id == "" {
			return nil, fmt.Errorf("export task row has no id — import aborted")
		}
		if row.String("title") == "" {
			return nil, fmt.Errorf("export task %q has no title — import aborted", id)
		}
		line, err := buildImportTaskLine(row, targetStyle, "")
		if err != nil {
			return nil, err
		}
		if exist, ok := target[id]; ok {
			if canonicalTaskJSON(row) == canonicalTaskJSON(exist) {
				plan.identical = append(plan.identical, id)
				continue
			}
			if !renumber {
				plan.skipped = append(plan.skipped, id)
				continue
			}
			newID := nextFreeID(used, id)
			if err := board.ValidateFleetTaskID(newID); err != nil {
				return nil, fmt.Errorf("--renumber %q: %w", id, err)
			}
			line, err = buildImportTaskLine(row, targetStyle, newID)
			if err != nil {
				return nil, err
			}
			used[newID] = true
			plan.tasks = append(plan.tasks, plannedTask{id: newID, exportID: id, status: planStatus(row), line: line, spec: taskRowSpecFor(newID, row, line)})
			continue
		}
		if !board.MatchesFleetTaskID(id) {
			return nil, fmt.Errorf("export task id %q does not match the fleet id format %s — import aborted", id, board.FleetTaskIDPattern)
		}
		used[id] = true
		plan.tasks = append(plan.tasks, plannedTask{id: id, status: planStatus(row), line: line, spec: taskRowSpecFor(id, row, line)})
	}

	// ---- events ----
	seen, err := targetEventFingerprints(b)
	if err != nil {
		return nil, err
	}
	// task_created first (mirrors create: row lands, then its event). These
	// plan rows are PREVIEW-ONLY (apply routes the appends through
	// Board.Create, which emits the real events), so they are shaped exactly
	// like AppendEvent writes them: actor defaults to "foreman" and the
	// detail payload is embedded as a JSON string, not a raw object.
	for _, t := range plan.tasks {
		detail := json.RawMessage(fmt.Sprintf(`{"id":%q,"status":%q}`, t.id, t.status))
		plan.events = append(plan.events, board.ImportEvent{
			Type:   "task_created",
			TaskID: t.id,
			Actor:  "foreman",
			Detail: json.RawMessage(jsonPreviewString(string(detail))),
		})
		plan.taskCreated++
	}
	for i, raw := range bd.Events {
		row, err := board.ParseRow(raw)
		if err != nil {
			return nil, fmt.Errorf("export event row %d: %w", i+1, err)
		}
		etype := row.String("event_type")
		if etype == "" {
			etype = "audit"
		}
		// task_created rows in the export are NEVER replayed: on any import
		// that appends the task, Board.Create emits the fresh task_created
		// itself (exactly one per imported row); when the task already
		// exists the event is either already on the target (fingerprint
		// dedupe below would drop it) or belongs to history the row-level
		// no-op preserves. Planning them would double-count against apply.
		if etype == "task_created" {
			continue
		}
		fp := eventFingerprint(row)
		if seen[fp] {
			continue
		}
		seen[fp] = true
		if !board.EventTypeVocabulary[etype] {
			return nil, fmt.Errorf("export event row %d: event type %q not in vocabulary — import aborted (nothing written)", i+1, etype)
		}
		var detail json.RawMessage
		if d := row.Get("detail"); d != nil {
			detail = json.RawMessage(bytes.TrimSpace(d))
		}
		plan.events = append(plan.events, board.ImportEvent{
			Type:   etype,
			TaskID: row.String("task_id"),
			Actor:  row.String("actor"),
			Detail: detail,
		})
		plan.fromExport++
	}

	// ---- fixtures ----
	fixturesExist := b.FixturesPath() != ""
	seenFix := map[string]bool{}
	for _, fid := range bd.FixtureIDs {
		if fid == "" || seenFix[fid] {
			continue
		}
		seenFix[fid] = true
		if fixtureIDs[fid] {
			continue
		}
		if !fixturesExist {
			plan.fixturesSkip++
			continue
		}
		plan.fixtures = append(plan.fixtures, fid)
	}
	return plan, nil
}

// planStatus is the status a planned row will carry on disk (write
// vocabulary, same alias rule as create).
func planStatus(row *board.Row) string {
	st := board.NormalizeStatus(row.String("status"))
	if st == "" {
		st = "pending"
	}
	return st
}

// taskRowSpecFor builds the board.TaskRowSpec for one planned import row.
// The spec is populated from the PARSED export row so Board.Create enforces
// its full write contract — required fields, fleet id format, status and
// priority vocabulary, duplicate-id, and depends_on existence — against the
// planned values, while Raw carries the pre-serialized line (target style,
// provenance stamped, renumbered id applied) that Create appends verbatim
// once every check passes.
func taskRowSpecFor(id string, row *board.Row, line []byte) board.TaskRowSpec {
	spec := board.TaskRowSpec{
		ID:     id,
		Title:  row.String("title"),
		Status: planStatus(row),
		Raw:    line,
	}
	if row.Get("priority") != nil {
		spec.Priority = board.NormalizePriority(row.String("priority"))
	}
	if cx, ok := row.Int("complexity"); ok {
		spec.Complexity = &cx
	}
	if deps := rowGoStringSlice(row, "depends_on"); deps != nil {
		spec.DependsOn = deps
		spec.HasDependsOn = true
	}
	if tags := rowGoStringSlice(row, "capability_tags"); tags != nil {
		spec.CapabilityTags = tags
		spec.HasTags = true
	}
	if r := row.String("reasoning"); r != "" {
		spec.Reasoning = r
	}
	return spec
}

// rowGoStringSlice decodes a row key holding a JSON array of strings into a
// []string (nil when the key is absent, null, or not an array). Empty arrays
// return a non-nil empty slice so HasDependsOn/HasTags still route the key
// through Create's checks.
func rowGoStringSlice(row *board.Row, key string) []string {
	raw := row.Get(key)
	if raw == nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, el := range arr {
		if s, isStr := el.(string); isStr {
			out = append(out, s)
		}
	}
	return out
}

// buildImportTaskLine serializes one export task row for the target:
// verbatim field values and key order in the target's detected style, with
// three write-vocabulary adjustments mirroring `boardctl create` — status
// normalized ("completed" -> "complete"), priority normalized (P0-P3), and
// the boardctl-web-export provenance appended to reasoning (original
// reasoning preserved verbatim).
func buildImportTaskLine(row *board.Row, style board.Style, newID string) ([]byte, error) {
	c := cloneRow(row)
	if newID != "" {
		if err := c.SetGoValue("id", newID, style); err != nil {
			return nil, err
		}
	}
	if c.Get("status") != nil {
		st := board.NormalizeStatus(c.String("status"))
		if !board.StatusVocabulary[st] {
			return nil, fmt.Errorf("export task %q: status %q not in write vocabulary {%s} — import aborted", c.String("id"), c.String("status"), vocabSet())
		}
		if err := c.SetGoValue("status", st, style); err != nil {
			return nil, err
		}
	}
	if c.Get("priority") != nil {
		p := board.NormalizePriority(c.String("priority"))
		if !board.PriorityVocabulary[p] {
			return nil, fmt.Errorf("export task %q: priority %q not in vocabulary {P0,P1,P2,P3} — import aborted", c.String("id"), c.String("priority"))
		}
		if err := c.SetGoValue("priority", p, style); err != nil {
			return nil, err
		}
	}
	if err := c.SetGoValue("reasoning", withProvenance(c.String("reasoning")), style); err != nil {
		return nil, err
	}
	return c.Marshal(style), nil
}

// cloneRow deep-copies a Row (Keys slice + Vals map; value bytes are
// immutable RawMessages shared safely).
func cloneRow(r *board.Row) *board.Row {
	c := &board.Row{Vals: map[string]json.RawMessage{}}
	c.Keys = append(c.Keys, r.Keys...)
	for k, v := range r.Vals {
		c.Vals[k] = v
	}
	return c
}

// cloneRowWithout deep-copies a Row, dropping one key entirely.
func cloneRowWithout(r *board.Row, drop string) *board.Row {
	c := &board.Row{Vals: map[string]json.RawMessage{}}
	for _, k := range r.Keys {
		if k == drop {
			continue
		}
		c.Keys = append(c.Keys, k)
		c.Vals[k] = r.Vals[k]
	}
	return c
}

// stripProvenance removes the import provenance marker from a reasoning
// string, reporting whether a marker was present.
func stripProvenance(s string) (string, bool) {
	if s == importProvenance {
		return "", true
	}
	if suffix := "; " + importProvenance; strings.HasSuffix(s, suffix) {
		return strings.TrimSuffix(s, suffix), true
	}
	return s, false
}

// withProvenance appends the import provenance marker to a reasoning string
// without destroying the imported reasoning (idempotent: already-marked
// strings pass through unchanged).
func withProvenance(s string) string {
	if _, marked := stripProvenance(s); marked {
		return s
	}
	if s == "" {
		return importProvenance
	}
	return s + "; " + importProvenance
}

// canonicalTaskJSON renders a task row for comparison: a canonical decode +
// re-encode of the row's full content (map keys sorted by encoding/json,
// strings/numbers canonically re-encoded), so serialization differences
// between the export payload (encoding/json-compact, HTML-escaped) and the
// target file (fleet spaced style, raw UTF-8) never read as content
// differences — spec 7.3.2's same-board round-trip must be a true no-op.
// Import provenance is stripped from reasoning first (the key is dropped
// entirely when nothing but the marker remains), so an import-restored row
// compares equal to its export row on the next run.
func canonicalTaskJSON(row *board.Row) string {
	dec := json.NewDecoder(bytes.NewReader(board.RowJSONCompact(row)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		// Unparseable row content: fall back to raw byte comparison.
		return string(board.RowJSONCompact(row))
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return string(board.RowJSONCompact(row))
	}
	if s, isStr := obj["reasoning"].(string); isStr {
		if stripped, marked := stripProvenance(s); marked {
			if stripped == "" {
				delete(obj, "reasoning")
			} else {
				obj["reasoning"] = stripped
			}
		}
	}
	return canonicalJSONValue(obj)
}

// canonicalJSONValue decodes (UseNumber) and re-encodes a JSON value
// deterministically: numbers keep their literal form, object keys are
// sorted, strings are re-encoded without HTML escaping. Values that differ
// only in whitespace or \uXXXX escape spelling compare equal.
func canonicalJSONValue(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return ""
	}
	return strings.TrimRight(buf.String(), "\n")
}

// decodeCanonical decodes raw JSON bytes into the canonical comparison form.
func decodeCanonical(raw []byte) string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return string(raw) // unparseable: compare raw bytes
	}
	return canonicalJSONValue(v)
}

// tasksStyle detects the serialization style of the target tasks.jsonl from
// its last non-empty line (the style appended rows must speak).
func tasksStyle(b *board.Board) (board.Style, error) {
	lines, err := board.ReadJSONLLines(b.TasksPath())
	if err != nil {
		return board.DefaultStyle(), err
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if len(bytes.TrimSpace(lines[i])) > 0 {
			return board.DetectStyle(lines[i]), nil
		}
	}
	return board.DefaultStyle(), nil
}

// eventFingerprint identifies an event by content, never by id or
// timestamp: type + task + actor + canonical detail. Exported event ids are
// never trusted (7.3.4) and replayed events get fresh timestamps in the
// target's dialect, so both are excluded from the identity. The detail is
// compared in canonical decoded form so HTML-escape spelling differences
// between the export payload and the target file never split an event.
func eventFingerprint(r *board.Row) string {
	etype := r.String("event_type")
	if etype == "" {
		etype = "audit"
	}
	actor := r.String("actor")
	if actor == "" {
		actor = "foreman"
	}
	var detail string
	if d := r.Get("detail"); d != nil {
		detail = decodeCanonical(bytes.TrimSpace(d))
	}
	return etype + "\x1f" + r.String("task_id") + "\x1f" + actor + "\x1f" + detail
}

// targetEventFingerprints fingerprints every event already on the target.
func targetEventFingerprints(b *board.Board) (map[string]bool, error) {
	rows, _, err := board.ReadAllRows(b.EventsPath())
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(rows))
	for _, r := range rows {
		seen[eventFingerprint(r)] = true
	}
	return seen, nil
}

// fixturesInfo collects what the planned diff needs to know about the
// fixtures registry: the ids that will be appended and the non-empty line
// count of the existing fixtures.jsonl (0 when the target has none).
type fixturesInfo struct {
	ids      []string
	oldLines int
	path     string
}

// jsonPreviewString encodes a string as a JSON string value exactly the way
// the board package's serialization does (SetEscapeHTML(false), ASCII
// passthrough) so the planned diff's preview rows speak the same dialect as
// the rows apply will write.
func jsonPreviewString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return `""`
	}
	return strings.TrimRight(buf.String(), "\n")
}

// importFileDiff renders the append-only unified diff one planned file
// produces. Because import only ever appends, the hunk covers the tail of
// the file: the old side ends at the current last line (oldLines) with zero
// removed lines, and the new side adds one line per planned row:
//
//	--- <path>
//	+++ <path> (after import)
//	@@ -<oldLines>,0 +<oldLines+1>,<added> @@
//	+<planned serialized row>
func importFileDiff(path string, oldLines int, planned [][]byte) string {
	if len(planned) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n", path)
	fmt.Fprintf(&sb, "+++ %s (after import)\n", path)
	fmt.Fprintf(&sb, "@@ -%d,0 +%d,%d @@\n", oldLines, oldLines+1, len(planned))
	for _, l := range planned {
		sb.WriteString("+")
		sb.Write(l)
		sb.WriteString("\n")
	}
	return sb.String()
}

// printImportPlan renders the deterministic preflight plan. The summary line
// matches the spec 7.3 observables exactly ("0 new tasks, 0 updates, 0
// events", "N new tasks", "N skipped (differs)"). When the plan appends
// anything, a unified diff of every planned append is rendered after the
// summary blocks: because import is append-only, each changed file gets
// conventional ---/+++ headers and one hunk whose "+" lines are exactly the
// rows that will land (tasks.jsonl, events.jsonl, and fixtures.jsonl when
// the plan touches them). A no-op plan states that no file diff applies.
func printImportPlan(w io.Writer, exportPath string, imp *importExport, targetName, boardDir string, p *importPlan, fx fixturesInfo) {
	fmt.Fprintf(w, "boardctl import plan for %q (%s)\n", targetName, boardDir)
	fmt.Fprintf(w, "  export: %s (%s, rendered %s)\n", exportPath, imp.Schema, imp.RenderedAt)
	fmt.Fprintf(w, "  tasks: %d new, %d identical (no-op), %d skipped (differs)\n", len(p.tasks), len(p.identical), len(p.skipped))
	for _, id := range p.identical {
		fmt.Fprintf(w, "  = %s identical (no-op)\n", id)
	}
	for _, id := range p.skipped {
		fmt.Fprintf(w, "  ! %s skipped (differs)\n", id)
	}
	for _, t := range p.tasks {
		if t.exportID != "" {
			fmt.Fprintf(w, "  + %s new task (renumbered from %s)\n", t.id, t.exportID)
		} else {
			fmt.Fprintf(w, "  + %s new task\n", t.id)
		}
	}
	fmt.Fprintf(w, "  events: %d events appended (%d from export, %d task_created for new rows; %d task_created rows in the export itself are never replayed — Board.Create emits fresh ones at apply time)\n",
		len(p.events), p.fromExport, p.taskCreated, p.exportTaskCreated)
	if p.fixturesSkip > 0 {
		plural := "s"
		if p.fixturesSkip == 1 {
			plural = ""
		}
		fmt.Fprintf(w, "  no fixtures.jsonl on target; %d fixture row%s skipped\n", p.fixturesSkip, plural)
	}
	if len(p.fixtures) > 0 {
		fmt.Fprintf(w, "  fixtures: %d appended (%s)\n", len(p.fixtures), strings.Join(p.fixtures, ", "))
	}

	// Planned file diff (append-only unified diff of everything apply will
	// write). Deterministic: task rows in plan order, then events in plan
	// order, then fixture ids in plan order.
	diff := ""
	taskLines := make([][]byte, 0, len(p.tasks))
	for _, t := range p.tasks {
		taskLines = append(taskLines, t.line)
	}
	diff += importFileDiff(canonicalBoardFileNames["tasks"], p.tasksOldLines, taskLines)
	if len(p.events) > 0 {
		eventLines := make([][]byte, 0, len(p.events))
		var sb strings.Builder
		for _, ev := range p.events {
			sb.Reset()
			sb.WriteString("{")
			fmt.Fprintf(&sb, "\"event_type\": %s,", jsonPreviewString(ev.Type))
			if ev.TaskID != "" {
				fmt.Fprintf(&sb, " \"task_id\": %s,", jsonPreviewString(ev.TaskID))
			} else {
				sb.WriteString(" \"task_id\": null,")
			}
			if ev.Actor != "" {
				fmt.Fprintf(&sb, " \"actor\": %s,", jsonPreviewString(ev.Actor))
			} else {
				sb.WriteString(" \"actor\": null,")
			}
			if len(ev.Detail) > 0 {
				fmt.Fprintf(&sb, " \"detail\": %s", string(bytes.TrimSpace(ev.Detail)))
			} else {
				sb.WriteString(" \"detail\": null")
			}
			sb.WriteString(" }")
			eventLines = append(eventLines, []byte(sb.String()))
		}
		diff += importFileDiff(canonicalBoardFileNames["events"], p.eventsOldLines, eventLines)
	}
	if len(fx.ids) > 0 {
		fixLines := make([][]byte, 0, len(fx.ids))
		for _, id := range fx.ids {
			fixLines = append(fixLines, []byte(fmt.Sprintf("{ \"id\": %s }", jsonPreviewString(id))))
		}
		diff += importFileDiff(canonicalBoardFileNames["fixtures"], fx.oldLines, fixLines)
	}
	if diff == "" {
		fmt.Fprintf(w, "planned diff: no file changes (no-op import)\n")
	} else {
		fmt.Fprintf(w, "planned diff:\n%s", diff)
	}
	fmt.Fprintf(w, "plan: %d new tasks, %d updates, %d events\n", len(p.tasks), 0, len(p.events))
}

// applyImportPlan writes the plan: task rows through Board.Create (which
// enforces its full write contract, appends each pre-serialized raw row, and
// emits its own task_created event — so apply must NOT append task_created
// events itself), then the remaining planned events (fresh MAX(id)+1 ids,
// target timestamp dialect), then fixture rows. Every write was validated
// during planning; the header is never touched.
func applyImportPlan(b *board.Board, p *importPlan) error {
	for _, t := range p.tasks {
		if _, err := b.Create(t.spec); err != nil {
			return fmt.Errorf("create task %s: %w", t.id, err)
		}
	}
	// Board.Create already emitted one task_created event per imported row;
	// the replayed export events are appended here. The writer is built
	// lazily ONCE, after the task rows/events Create wrote, so it sees the
	// post-create MAX(id) for fresh sequencing. The type filter is an
	// invariant guard: planning never schedules a task_created append (the
	// plan-side skip mirrors it), so no imported task can ever end up with
	// two task_created events.
	var ew *board.ImportEventWriter
	for _, ev := range p.events {
		if ev.Type == "task_created" {
			continue
		}
		if ew == nil {
			var err error
			ew, err = b.NewImportEventWriter()
			if err != nil {
				return err
			}
		}
		if err := ew.Append(ev); err != nil {
			return fmt.Errorf("append event: %w", err)
		}
	}
	for _, fid := range p.fixtures {
		if err := b.AppendImportFixture(fid); err != nil {
			return fmt.Errorf("append fixture %s: %w", fid, err)
		}
	}
	fmt.Fprintf(os.Stdout, "imported: %d task row(s), %d event(s), %d fixture row(s) written\n", len(p.tasks), len(p.events), len(p.fixtures))
	return nil
}
