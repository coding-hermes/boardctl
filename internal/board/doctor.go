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
//   - header counter sanity (topology A): header ticks_total must not be
//     behind the newest numeric tick_number in events.jsonl, and ticks_idle
//     must not exceed ticks_total
//   - fixture orphan detection: every id listed in fixtures.jsonl must have a
//     definition row in tasks.jsonl
func (b *Board) Doctor() (*Report, error) {
	rep, err := b.Validate()
	if err != nil {
		return nil, err
	}
	b.doctorGitTrackedSet(rep)
	b.doctorHeaderVsEvents(rep)
	b.doctorFixtureOrphans(rep)
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

// doctorHeaderVsEvents compares header counters (topology A) against the
// events stream: ticks_total behind the newest numeric event tick_number is
// drift (an error); ticks_idle above ticks_total is an internal inconsistency
// (a warn). Events without a numeric tick_number (legacy rows) are ignored;
// header/event read failures are skipped because Validate already itemized
// them.
func (b *Board) doctorHeaderVsEvents(rep *Report) {
	if b.Topology != "A" {
		rep.Add("warn", "topology B: no board.jsonl — header counters not checked")
		return
	}
	rows, _, err := ReadAllRows(b.eventsPath)
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
		rep.Add("error", "header ticks_total %d < max events tick_number %d — header counter stale or an event tick never completed", total, maxTick)
	}
	if idle > total {
		rep.Add("warn", "header ticks_idle exceeds ticks_total — counters inconsistent")
	}
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
	taskRows, _, err := ReadAllRows(b.tasksPath)
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
