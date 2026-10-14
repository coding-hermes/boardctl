package main

// DF-BOARDCTL-32: the --by-id "nothing to do" census must not lie about
// pre-repair boards. A recycled id whose later lines are ALL already
// terminal needs no closure (no group, nothing to write) — but the id is
// still counted by recycled_ids, and the old text ("no recycled task ids on
// this board") contradicted that count. Covered here at CLI level:
//
//   (a) all-terminal recycled ids -> the new explanatory line, exit 0,
//       zero mutation (sha256 before/after, dry-run AND --apply);
//   (b) --json explains the census on its own (already_terminal_ids);
//   (c) recycled_ids == 0 keeps the old message verbatim;
//   (d) mixed board (one actionable group + one all-terminal id) keeps the
//       group behavior and gains the info — and the second --apply after a
//       real repair reports the new line (all later lines terminal by then).

import (
	"encoding/json"
	"strings"
	"testing"
)

// allTerminalFixtureTasks is a pre-repair board whose recycled ids have
// nothing left to close: QA-ASCE-1 keeps line 1 and its later lines (2, 4)
// are complete/failed; QA-TB-1 keeps line 5 and its later line (6) is
// complete. OTHER-1 is a single non-recycled row. Two recycled ids, zero
// actionable groups.
func allTerminalFixtureTasks() string {
	rows := []string{
		`{"id":"QA-ASCE-1","title":"[P2] header-frozen on asce main","reasoning":"cell detail: header never advances","status":"pending","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] spawn pool exhausted","reasoning":"cell detail: no free worker slots","status":"complete","priority":"P2"}`,
		`{"id":"OTHER-1","title":"unrelated clean row","reasoning":"nothing to see","status":"complete","priority":"P3"}`,
		`{"id":"QA-ASCE-1","title":"[P2] bunker dropped the agent","reasoning":"cell detail: ssh session died mid-run","status":"failed","priority":"P2"}`,
		`{"id":"QA-TB-1","title":"[P1] topology-B header drift","reasoning":"cell detail: header not a task row","status":"pending","priority":"P1"}`,
		`{"id":"QA-TB-1","title":"[P1] topology-B header drift (recurrence)","reasoning":"cell detail: second filing","status":"complete","priority":"P1"}`,
	}
	return strings.Join(rows, "\n") + "\n"
}

// noRecycledFixtureTasks has three distinct ids, one line each — genuinely no
// recycled task id on the board.
func noRecycledFixtureTasks() string {
	rows := []string{
		`{"id":"CLEAN-1","title":"first finding","reasoning":"cell detail: one","status":"pending","priority":"P2"}`,
		`{"id":"CLEAN-2","title":"second finding","reasoning":"cell detail: two","status":"in_progress","priority":"P2"}`,
		`{"id":"CLEAN-3","title":"third finding","reasoning":"cell detail: three","status":"complete","priority":"P3"}`,
	}
	return strings.Join(rows, "\n") + "\n"
}

// (a) recycled ids, all later lines terminal: the new message in BOTH the
// dry-run and the applied branch, exit 0, and zero mutation both times.
func TestCLI_ByID32_AllTerminalRecycledIDs(t *testing.T) {
	dir := t.TempDir()
	writeBoard(t, dir, allTerminalFixtureTasks(), testEvents, testHeader)

	for _, apply := range []bool{false, true} {
		before := boardSnapshot(t, dir)
		args := []string{"--by-id"}
		if apply {
			args = append(args, "--apply")
		}
		code, out := runCLI(t, dir, args...)
		if code != 0 {
			t.Fatalf("apply=%v: exit = %d, want 0\nstdout:\n%s", apply, code, out)
		}
		if !strings.Contains(out, "recycled ids: 2") {
			t.Fatalf("apply=%v: census does not count the 2 recycled ids:\n%s", apply, out)
		}
		want := "nothing to do — 2 recycled id(s), all later lines already terminal (no files written)"
		if !strings.Contains(out, want) {
			t.Fatalf("apply=%v: output missing explanatory line %q:\n%s", apply, want, out)
		}
		if strings.Contains(out, "no recycled task ids on this board") {
			t.Fatalf("apply=%v: output still claims no recycled task ids:\n%s", apply, out)
		}
		assertSnapshotsEqual(t, before, boardSnapshot(t, dir))
	}
}

// (b) --json must explain the census on its own: recycled_ids=2 with zero
// groups is explained by already_terminal_ids naming both ids.
func TestCLI_ByID32_JSONExplainsAllTerminal(t *testing.T) {
	dir := t.TempDir()
	writeBoard(t, dir, allTerminalFixtureTasks(), testEvents, testHeader)
	before := boardSnapshot(t, dir)

	code, out := runCLI(t, dir, "--by-id", "--json")
	if code != 0 {
		t.Fatalf("--json exit = %d, want 0\nstdout:\n%s", code, out)
	}
	var report struct {
		Lane               string   `json:"lane"`
		TaskRows           int      `json:"task_rows"`
		DistinctIDs        int      `json:"distinct_ids"`
		RecycledIDs        int      `json:"recycled_ids"`
		Groups             []any    `json:"groups"`
		AlreadyTerminalIDs []string `json:"already_terminal_ids"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("--json output does not parse: %v\n%s", err, out)
	}
	if report.Lane != "asce" {
		t.Fatalf("lane = %q, want asce", report.Lane)
	}
	if report.RecycledIDs != 2 || len(report.Groups) != 0 {
		t.Fatalf("census wrong: recycled_ids=%d groups=%d, want 2/0", report.RecycledIDs, len(report.Groups))
	}
	// The explanation: recycled_ids == len(groups) + len(already_terminal_ids).
	if len(report.AlreadyTerminalIDs) != 2 ||
		report.AlreadyTerminalIDs[0] != "QA-ASCE-1" ||
		report.AlreadyTerminalIDs[1] != "QA-TB-1" {
		t.Fatalf("already_terminal_ids = %v, want [QA-ASCE-1 QA-TB-1] (first-seen order)", report.AlreadyTerminalIDs)
	}
	if report.TaskRows != 6 || report.DistinctIDs != 3 {
		t.Fatalf("rows/ids = %d/%d, want 6/3", report.TaskRows, report.DistinctIDs)
	}
	assertSnapshotsEqual(t, before, boardSnapshot(t, dir))
}

// (c) recycled_ids == 0 keeps the old message verbatim, dry-run and applied.
func TestCLI_ByID32_NoRecycledIDsKeepsOldMessage(t *testing.T) {
	dir := t.TempDir()
	writeBoard(t, dir, noRecycledFixtureTasks(), testEvents, testHeader)

	oldLine := "nothing to do — no recycled task ids on this board (no files written)"
	for _, apply := range []bool{false, true} {
		args := []string{"--by-id"}
		if apply {
			args = append(args, "--apply")
		}
		code, out := runCLI(t, dir, args...)
		if code != 0 {
			t.Fatalf("apply=%v: exit = %d, want 0\nstdout:\n%s", apply, code, out)
		}
		if !strings.Contains(out, oldLine) {
			t.Fatalf("apply=%v: output missing old line %q:\n%s", apply, oldLine, out)
		}
		if strings.Contains(out, "already terminal") {
			t.Fatalf("apply=%v: clean board must not mention terminal lines:\n%s", apply, out)
		}
	}
}

// (d) mixed board: the asce group (all five later lines actionable) plus
// QA-MIX-1 — one recycled id with BOTH a terminal later line (left as-is,
// per-group note) and an actionable one — plus one all-terminal id. After a
// real --apply, a second run reports the new explanatory line (every later
// line is terminal by then).
func TestCLI_ByID32_MixedGroupPlusTerminalID(t *testing.T) {
	dir := t.TempDir()
	mixed := asceFixtureTasks() + strings.Join([]string{
		`{"id":"QA-MIX-1","title":"[P2] mixed kept","reasoning":"cell detail: kept open","status":"pending","priority":"P2"}`,
		`{"id":"QA-MIX-1","title":"[P2] mixed closed by hand","reasoning":"cell detail: already terminal","status":"complete","priority":"P2"}`,
		`{"id":"QA-MIX-1","title":"[P2] mixed still open","reasoning":"cell detail: to close","status":"pending","priority":"P2"}`,
		`{"id":"QA-DONE-9","title":"[P3] already closed finding","reasoning":"cell detail: closed by hand","status":"complete","priority":"P3"}`,
		`{"id":"QA-DONE-9","title":"[P3] already closed finding (recurrence)","reasoning":"cell detail: re-filed closed","status":"failed","priority":"P3"}`,
	}, "\n") + "\n"
	writeBoard(t, dir, mixed, testEvents, testHeader)

	// --- dry-run: both actionable groups render as before; QA-MIX-1's
	// terminal later line carries the per-group note; the fully-terminal
	// QA-DONE-9 produces no group at all.
	code, out := runCLI(t, dir, "--by-id")
	if code != 0 {
		t.Fatalf("dry-run exit = %d, want 0\nstdout:\n%s", code, out)
	}
	if !strings.Contains(out, "recycled ids: 3") || !strings.Contains(out, "DRY-RUN") {
		t.Fatalf("dry-run census/markers wrong:\n%s", out)
	}
	if !strings.Contains(out, "QA-ASCE-1  keep line 1  supersede QA-ASCE-1 (line 2)") {
		t.Fatalf("asce group changed:\n%s", out)
	}
	if !strings.Contains(out, "QA-MIX-1  keep line 8  supersede QA-MIX-1 (line 10)  [already terminal, left as-is: line 9]") {
		t.Fatalf("mixed group lost its terminal-line note:\n%s", out)
	}
	if strings.Contains(out, "QA-DONE-9  keep") {
		t.Fatalf("all-terminal id must not render as a group:\n%s", out)
	}

	// --- --json: two groups + one all-terminal id explains recycled_ids=3.
	code, out = runCLI(t, dir, "--by-id", "--json")
	if code != 0 {
		t.Fatalf("--json exit = %d, want 0\nstdout:\n%s", code, out)
	}
	var report struct {
		RecycledIDs        int      `json:"recycled_ids"`
		Groups             []any    `json:"groups"`
		AlreadyTerminalIDs []string `json:"already_terminal_ids"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("--json does not parse: %v\n%s", err, out)
	}
	if report.RecycledIDs != 3 || len(report.Groups) != 2 || len(report.AlreadyTerminalIDs) != 1 || report.AlreadyTerminalIDs[0] != "QA-DONE-9" {
		t.Fatalf("json census wrong: recycled=%d groups=%d terminal=%v", report.RecycledIDs, len(report.Groups), report.AlreadyTerminalIDs)
	}

	// --- apply repairs the actionable lines; the second apply then has ALL
	// three ids fully terminal and must say so with the new line.
	code, out = runCLI(t, dir, "--by-id", "--apply")
	if code != 0 || !strings.Contains(out, "applied") {
		t.Fatalf("first apply exit = %d:\n%s", code, out)
	}
	before := boardSnapshot(t, dir)
	code, out = runCLI(t, dir, "--by-id", "--apply")
	if code != 0 {
		t.Fatalf("second apply exit = %d, want 0\nstdout:\n%s", code, out)
	}
	want := "nothing to do — 3 recycled id(s), all later lines already terminal (no files written)"
	if !strings.Contains(out, want) {
		t.Fatalf("second apply output missing %q:\n%s", want, out)
	}
	assertSnapshotsEqual(t, before, boardSnapshot(t, dir))
}
