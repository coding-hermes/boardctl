package board

import (
	"encoding/json"
	"strings"
	"testing"
)

// DF-BOARDCTL-32: the id-dupe census must explain recycled ids whose later
// lines are all already terminal. Such an id produces no group (nothing to
// close), so before this fix a dry-run report showed recycled_ids:N with an
// empty group list and nothing else. The report now carries
// AlreadyTerminalIDs (first-seen order), keeping the invariant
// recycled_ids == len(Groups) + len(AlreadyTerminalIDs).

// All-terminal recycled ids: no groups, both ids named, counts intact, and
// the apply branch still writes nothing.
func TestIDDupe32_AllTerminalRecycledIDsExplained(t *testing.T) {
	b := seedIDDupeBoard(t,
		`{"id":"QA-ASCE-1","title":"[P2] header-frozen","reasoning":"cell: stuck","status":"pending","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] spawn pool exhausted","reasoning":"cell: slots","status":"complete","priority":"P2"}`,
		`{"id":"QA-TB-1","title":"[P1] header drift","reasoning":"cell: drift","status":"pending","priority":"P1"}`,
		`{"id":"QA-TB-1","title":"[P1] header drift again","reasoning":"cell: drift 2","status":"failed","priority":"P1"}`,
	)

	rep := mustIDDupe(t, b, false)
	if rep.RecycledIDs != 2 || rep.DistinctIDs != 2 || rep.TaskRows != 4 {
		t.Fatalf("census = %d/%d/%d, want 2/2/4", rep.RecycledIDs, rep.DistinctIDs, rep.TaskRows)
	}
	if len(rep.Groups) != 0 {
		t.Fatalf("groups = %d, want 0 (nothing is closable)", len(rep.Groups))
	}
	if len(rep.AlreadyTerminalIDs) != 2 || rep.AlreadyTerminalIDs[0] != "QA-ASCE-1" || rep.AlreadyTerminalIDs[1] != "QA-TB-1" {
		t.Fatalf("already_terminal_ids = %v, want [QA-ASCE-1 QA-TB-1] (first-seen order)", rep.AlreadyTerminalIDs)
	}
	if rep.Changed() {
		t.Fatal("report claims changes although every later line is terminal")
	}
	// The invariant that keeps the JSON census self-explanatory.
	if rep.RecycledIDs != len(rep.Groups)+len(rep.AlreadyTerminalIDs) {
		t.Fatalf("recycled_ids %d != groups %d + already_terminal_ids %d",
			rep.RecycledIDs, len(rep.Groups), len(rep.AlreadyTerminalIDs))
	}

	// apply=true on the same board: nothing written, same explanation.
	before := idDupeBoardSnapshot(t, b)
	repA := mustIDDupe(t, b, true)
	if repA.Changed() || len(repA.AlreadyTerminalIDs) != 2 {
		t.Fatalf("apply report wrong: changed=%v terminal=%v", repA.Changed(), repA.AlreadyTerminalIDs)
	}
	after := idDupeBoardSnapshot(t, b)
	for p, h := range before {
		if after[p] != h {
			t.Fatalf("apply rewrote %s although nothing was closable (sha %s -> %s)", p, h, after[p])
		}
	}
}

// A board with NO recycled ids leaves AlreadyTerminalIDs empty (omitted in
// JSON), and a fully actionable board does too — the field only appears when
// a recycled id was skipped as terminal.
func TestIDDupe32_AlreadyTerminalIDsOmittedUnlessTerminal(t *testing.T) {
	// Clean board: no recycled ids at all.
	clean := mustIDDupe(t, seedIDDupeBoard(t,
		`{"id":"CLEAN-1","title":"one","reasoning":"c","status":"pending","priority":"P2"}`,
		`{"id":"CLEAN-2","title":"two","reasoning":"c","status":"complete","priority":"P2"}`,
	), false)
	if clean.RecycledIDs != 0 || len(clean.AlreadyTerminalIDs) != 0 {
		t.Fatalf("clean board: recycled=%d terminal=%v, want 0/empty", clean.RecycledIDs, clean.AlreadyTerminalIDs)
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "already_terminal_ids") {
		t.Fatalf("omitted field leaked into JSON: %s", raw)
	}

	// Fully actionable board: every later line open — again omitted.
	actionable := mustIDDupe(t, seedIDDupeBoard(t, asceFixtureLines()...), false)
	if actionable.RecycledIDs != 1 || len(actionable.Groups) != 1 || len(actionable.AlreadyTerminalIDs) != 0 {
		t.Fatalf("actionable board: recycled=%d groups=%d terminal=%v, want 1/1/empty",
			actionable.RecycledIDs, len(actionable.Groups), actionable.AlreadyTerminalIDs)
	}
	rawA, err := json.Marshal(actionable)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rawA), "already_terminal_ids") {
		t.Fatalf("omitted field leaked into JSON: %s", rawA)
	}
}

// Mixed: one actionable group + one all-terminal id — the group behavior is
// unchanged and the terminal id is explained; the census invariant holds.
func TestIDDupe32_MixedGroupPlusTerminalID(t *testing.T) {
	b := seedIDDupeBoard(t,
		`{"id":"QA-ASCE-1","title":"[P2] header-frozen","reasoning":"cell: stuck","status":"pending","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] bunker dropped","reasoning":"cell: ssh died","status":"pending","priority":"P2"}`,
		`{"id":"QA-DONE-9","title":"[P3] closed finding","reasoning":"cell: closed","status":"pending","priority":"P3"}`,
		`{"id":"QA-DONE-9","title":"[P3] closed finding (recurrence)","reasoning":"cell: re-filed","status":"complete","priority":"P3"}`,
	)

	rep := mustIDDupe(t, b, false)
	if rep.RecycledIDs != 2 || len(rep.Groups) != 1 || len(rep.AlreadyTerminalIDs) != 1 {
		t.Fatalf("census = %d/%d/%v, want 2/1/[QA-DONE-9]", rep.RecycledIDs, len(rep.Groups), rep.AlreadyTerminalIDs)
	}
	if rep.AlreadyTerminalIDs[0] != "QA-DONE-9" {
		t.Fatalf("already_terminal_ids = %v, want [QA-DONE-9]", rep.AlreadyTerminalIDs)
	}
	g := rep.Groups[0]
	if g.ID != "QA-ASCE-1" || len(g.ClosedLines) != 1 || g.ClosedLines[0] != 2 || len(g.AlreadyTerminal) != 0 {
		t.Fatalf("actionable group changed: %+v", g)
	}
	if rep.RecycledIDs != len(rep.Groups)+len(rep.AlreadyTerminalIDs) {
		t.Fatalf("invariant broken: %d != %d + %d", rep.RecycledIDs, len(rep.Groups), len(rep.AlreadyTerminalIDs))
	}
}
