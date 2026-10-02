package board

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Doctor validates a board (JSONL shape, via Validate) and then appends deep
// cross-checks that go beyond per-file shape to the SAME report, so rendering
// and exit-code handling are identical to Validate:
//
//   - git tracked-set: no tracked path under the board dir may carry a .db or
//     .parquet extension — JSONL is the canonical store, caches are untracked
//   - header counter sanity (both topologies — topology B reads the line-1
//     tasks.jsonl header): header ticks_total must not be behind the newest
//     numeric tick_number in events.jsonl, and ticks_idle must not exceed
//     ticks_total
//   - fixture orphan detection: every id listed in fixtures.jsonl must have a
//     definition row in tasks.jsonl
//   - BT-023 fleet id format: task ids not matching the fleet pattern
//     (^[A-Z][A-Z0-9]+(-[A-Z0-9]+)+$) are itemized WARNINGS with line
//     numbers — legacy junk ids surface but never fail doctor (validate
//     stays silent; the write paths enforce the format on new ids)
//   - DF-BOARDCTL-25: on topology B the board header is line 1 of
//     tasks.jsonl, so doctor states the legacy layout explicitly when that
//     line is a valid header (a warn — the board is healthy) and reports a
//     corrupt line 1 as salvageable header corruption (an error naming the
//     `boardctl validate --repair` salvage path) instead of letting a bare
//     parse error read exactly like the healthy legacy board
func (b *Board) Doctor() (*Report, error) {
	rep, err := b.Validate()
	if err != nil {
		return nil, err
	}
	b.doctorGitTrackedSet(rep)
	b.doctorTopologyBLineOne(rep)
	b.doctorHeaderVsEvents(rep)
	b.doctorFixtureOrphans(rep)
	b.doctorTaskIDFormat(rep)
	return rep, nil
}

// doctorGitTrackedSet walks up from the board dir to find the enclosing git
// repo, then asks git for every tracked path under the board dir and flags
// .db/.parquet files (rebuildable caches must never be tracked).
func (b *Board) doctorGitTrackedSet(rep *Report) {
	root := findRepoRoot(b.Dir)
	if root == "" {
		rep.Add("warn", "not inside a git repo — git tracked-set check skipped")
		return
	}
	rel, err := filepath.Rel(root, b.Dir)
	if err != nil {
		rep.Add("warn", "git tracked-set check skipped: cannot compute board dir relative to repo root: %v", err)
		return
	}
	cmd := exec.Command("git", "-C", root, "ls-files", "--", rel)
	out, err := cmd.CombinedOutput()
	if err != nil {
		switch {
		case errors.Is(err, exec.ErrNotFound):
			rep.Add("warn", "git not available — git tracked-set check skipped")
		default:
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			rep.Add("warn", "git ls-files failed — git tracked-set check skipped: %s", msg)
		}
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch filepath.Ext(line) {
		case ".db", ".parquet":
			rep.Add("error", "git-tracked-set: %s is tracked under %s — JSONL-canonical boards must not track db/parquet caches", line, rel)
		}
	}
}

// findRepoRoot walks up from dir until it finds a .git entry (a directory, or
// a file for linked worktrees/submodules). It returns "" when no git repo
// encloses dir.
func findRepoRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// doctorHeaderVsEvents compares header counters against the events stream in
// BOTH topologies (BT-010 reads the topology-B line-1 header via HeaderRow):
// ticks_total behind the newest numeric event tick_number is drift (an
// error, with a BT-013 remediation hint naming the exact fix command);
// ticks_idle above ticks_total is an internal inconsistency (a warn).
// Events without a numeric tick_number (legacy rows) are ignored;
// header/event read failures are skipped because Validate already itemized
// them.
//
// BT-037: a HEADERLESS board has no counters to compare, so the cross-check
// is skipped with a note naming why — the drift ERROR (which would propose
// `boardctl header --set-ticks-total`, a command that now refuses) must never
// be emitted for a board that carries no header.
func (b *Board) doctorHeaderVsEvents(rep *Report) {
	if !b.HasHeader() {
		rep.Add("warn", "headerless board (no board.jsonl): no header counters to compare — header-vs-events tick drift checks skipped")
		return
	}
	var rows []*Row
	var err error
	if b.SkipBad {
		// DF-BOARDCTL-9: tolerant scan; validate already itemized any bad
		// lines, so doctor only consumes the salvageable events here.
		rows, _, err = b.ReadAllRowsTolerant(b.eventsPath)
	} else {
		rows, _, err = ReadAllRows(b.eventsPath)
	}
	if err != nil {
		return // validate already reported the events read failure
	}
	var maxTick int64
	haveTick := false
	for _, r := range rows {
		if t, ok := r.Int("tick_number"); ok {
			if !haveTick || t > maxTick {
				maxTick = t
				haveTick = true
			}
		}
	}
	hdr, err := b.HeaderRow()
	if err != nil {
		return // validate already reported the header failure
	}
	total, okTotal := hdr.Int("ticks_total")
	idle, okIdle := hdr.Int("ticks_idle")
	if !okTotal || !okIdle {
		return // non-integer/missing counters already flagged by validate
	}
	if haveTick && total < maxTick {
		// BT-013: the original drift text is asserted verbatim by tests,
		// so the remediation rides inside the same finding as a second
		// line — the fix command printed directly under the error, with
		// the counter value that resolves the drift (max event tick).
		rep.Add("error",
			"header ticks_total %d < max events tick_number %d — header counter stale or an event tick never completed\nfix: boardctl header --set-ticks-total=%d",
			total, maxTick, maxTick)
	}
	if idle > total {
		rep.Add("warn", "header ticks_idle exceeds ticks_total — counters inconsistent")
	}
}

// df25TopologyBHeaderCorruptionMsg is the error text doctor emits when line 1
// of tasks.jsonl on a board WITHOUT board.jsonl cannot be parsed: the
// topology-B header line is where the board header lives, so this is
// corruption of real board state — but the remaining task rows are salvageable
// (`boardctl validate --repair` re-reads tolerantly into tasks.rewritten.jsonl
// for manual review), which a bare parse error never told the operator.
const df25TopologyBHeaderCorruptionMsg = "corruption of the topology-B header line"

// doctorTopologyBLineOne makes the topology-B header line EXPLICIT in the
// report (DF-BOARDCTL-25): a topology-B board and a corrupt tasks.jsonl line
// 1 resolve to the same topology and used to produce indistinguishable doctor
// output — the healthy legacy board stayed silent about its header line while
// the corrupt one printed only bare parse errors, with nothing telling an
// operator which situation they were in or that the corrupt one is
// salvageable.
//
//   - topology B + line 1 IS a valid header row  -> a distinct warn stating
//     the legacy layout is expected and writable, zero errors (the board is
//     healthy; the header cross-checks already ran against it)
//   - topology B + line 1 unparseable            -> an ERROR naming the line
//     as topology-B header corruption with the `boardctl validate --repair`
//     salvage hint — corruption that is recoverable, never silently passed
//   - topology A / headerless boards             -> nothing: the header lives
//     in board.jsonl (or nowhere), so line 1 carries no header meaning
//
// The check is read-only: it parses the line in memory and never writes.
func (b *Board) doctorTopologyBLineOne(rep *Report) {
	if b.IsTopologyA() || !b.HasHeader() {
		return // topology A: header is board.jsonl; headerless: no header row at all
	}
	line, ok := firstNonBlankLine(b.tasksPath)
	if !ok {
		// Unreadable, empty, or all-blank file: validate already itemized
		// the read failure or the empty-board shape — nothing to classify.
		return
	}
	row, perr := ParseRow(line)
	if perr == nil {
		if !rowIsHeaderShape(row) {
			return // impossible under the current Resolve headerless rule; stay silent rather than mislabel a task row
		}
		rep.Add("warn",
			"legacy topology B: tasks.jsonl line 1 is the board header (metadata without a task id) — this is the expected legacy layout, not corruption; the board is fully writable")
		return
	}
	// Line 1 does not parse: the topology-B header row itself is destroyed.
	// Quote a short snippet of the offending line (%q escapes binary junk)
	// so the finding is self-contained; "line 1" means the header slot —
	// the first non-blank line, the same convention HeaderRow uses.
	snippet := string(line)
	if len(snippet) > 120 {
		snippet = snippet[:120] + "…"
	}
	rep.Add("error",
		"tasks.jsonl line 1: %v — %s (the board header lives on line 1 of tasks.jsonl when there is no board.jsonl, so the header is destroyed even though the remaining task rows may be fine; offending line: %q)\nfix: boardctl validate --repair re-reads the board tolerantly and writes the salvageable rows to tasks.rewritten.jsonl for manual review",
		perr, df25TopologyBHeaderCorruptionMsg, snippet)
}

// doctorFixtureOrphans flags fixture ids (fixtures.jsonl) that have no task
// row in tasks.jsonl: a fixture definition must exist as a real task row, with
// fixtures.jsonl marking it as a permanent fixture.
func (b *Board) doctorFixtureOrphans(rep *Report) {
	if b.FixturesPath() == "" {
		return // no fixtures file — nothing to cross-check
	}
	fixtureIDs, err := b.FixtureIDs()
	if err != nil {
		return // validate already reported the fixtures read failure
	}
	if len(fixtureIDs) == 0 {
		return
	}
	taskRows, err := b.TaskRows()
	if err != nil {
		return // validate already reported the tasks read failure
	}
	taskIDs := make(map[string]bool, len(taskRows))
	for _, r := range taskRows {
		if id := r.String("id"); id != "" {
			taskIDs[id] = true
		}
	}
	for _, id := range SortedKeys(fixtureIDs) {
		if !taskIDs[id] {
			rep.Add("error", "fixture %s has no task row in tasks.jsonl — orphaned fixture definition (fixture rows must exist in tasks.jsonl)", id)
		}
	}
}

// doctorTaskIDFormat itemizes task ids that do not match the fleet id format
// (BT-023) as WARNINGS, one per offending row with its tasks.jsonl line
// number — in the style of validate's itemized legacy findings. Junk ids
// already on a board must surface without failing doctor: the board stays
// green (HasErrors() is untouched by warnings) while flagging rows that can
// wedge downstream id-parsing tooling. Write-time enforcement lives in
// Create/UpdateTask (--force escape); validate stays silent so legacy boards
// do not newly fail validation.
func (b *Board) doctorTaskIDFormat(rep *Report) {
	lines, err := ReadJSONLLines(b.tasksPath)
	if err != nil {
		return // validate already reported the tasks read failure
	}
	if b.SkipBad {
		// DF-BOARDCTL-9: tolerant scan over the salvageable rows; the
		// bad lines themselves are itemized by validateTasks' own
		// tolerant pass.
		b.iterParsedTolerant(lines, b.tasksPath, func(row *Row, idx int, _ []byte) error {
			if b.skipTaskLine(lines, idx) {
				return nil
			}
			doctorTaskIDFormatRow(rep, row, idx)
			return nil
		})
		return
	}
	_ = IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		if b.skipTaskLine(lines, idx) {
			return nil // topology B: line 1 is the header, not a task row
		}
		doctorTaskIDFormatRow(rep, row, idx)
		return nil
	})
}

// doctorTaskIDFormatRow flags one row whose id does not match the fleet id
// format (BT-023) as a warning, shared by the default and tolerant scans.
func doctorTaskIDFormatRow(rep *Report, row *Row, idx int) {
	if id := row.String("id"); id != "" && !MatchesFleetTaskID(id) {
		rep.Add("warn", "tasks.jsonl line %d: task id %q does not match the fleet id format %s — downstream id parsing (task-router, board-scan) may wedge; writes now enforce this (rewrite via boardctl create --force or hand-edit the row)",
			idx+1, id, FleetTaskIDPattern)
	}
}
