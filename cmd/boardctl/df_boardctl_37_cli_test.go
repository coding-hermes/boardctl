package main

// DF-BOARDCTL-37 CLI surface: `umbrella --list --status S` filters the
// unified task table by status. Contract:
//   - a single canonical status lists only rows in that status;
//   - a comma-separated list ORs the statuses;
//   - read-side aliases (done/todo/...) normalize to their canonical value,
//     so a filter that names only aliases still lists the matching rows;
//   - an unknown status is a usage-class error (exit 2) naming the offending
//     value and the vocabulary — never a silent empty view;
//   - --status is a --list-only filter: without --list it is refused
//     (exit 2), and --json + --list --status stays a usage conflict.
//
// Fixtures reuse the BT-078 CLI helpers (bt078Fixture/bt078Create/bt078Run).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// df37AppendRawStatus writes one verbatim task row with the given status
// spelling into a repo's tasks.jsonl (create's write gate rightly refuses
// alias spellings — a stored alias row is the hand-written shape this
// filter must still match canonically).
func df37AppendRawStatus(t *testing.T, repo, id, rawStatus string) {
	t.Helper()
	bd := filepath.Join(repo, ".coding-hermes", "board")
	f, err := os.OpenFile(filepath.Join(bd, "tasks.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck
	if _, err := f.WriteString(`{"id":"` + id + `","title":"df37 ` + id + `","status":"` + rawStatus + `","priority":"P2"}` + "\n"); err != nil {
		t.Fatal(err)
	}
}

// df37RowPresent reports whether the unified table carries a row whose REPO
// and ID columns are repo and id (the list renderer prints them as separate
// aligned columns, not the qualified repo/id string --validate uses).
func df37RowPresent(out, repo, id string) bool {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == id && i > 0 && fields[i-1] == repo {
				return true
			}
		}
	}
	return false
}

func TestDFBoardCTL37UmbrellaListSingleStatus(t *testing.T) {
	umbrella, childA, childB := bt078Fixture(t)
	bt078Create(t, childA, "DF37-A1", "childA pending")
	bt078Create(t, childB, "DF37-B1", "childB pending")
	df37AppendRawStatus(t, childA, "DF37-A2", "complete")
	df37AppendRawStatus(t, childB, "DF37-B2", "complete")
	df37AppendRawStatus(t, childB, "DF37-B3", "failed")

	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--list", "--status", "complete"})
	if code != 0 {
		t.Fatalf("umbrella --list --status complete exit = %d, want 0 (out: %s)", code, out)
	}
	if !df37RowPresent(out, "childA", "DF37-A2") || !df37RowPresent(out, "childB", "DF37-B2") {
		t.Fatalf("filtered list lost the complete rows across both boards:\n%s", out)
	}
	for _, leak := range []string{"DF37-A1", "DF37-B1", "DF37-B3"} {
		if strings.Contains(out, leak) {
			t.Fatalf("filtered --status complete leaked non-matching row %s:\n%s", leak, out)
		}
	}
}

func TestDFBoardCTL37UmbrellaListCommaSeparatedStatuses(t *testing.T) {
	umbrella, childA, childB := bt078Fixture(t)
	bt078Create(t, childA, "DF37-A1", "childA pending")
	df37AppendRawStatus(t, childA, "DF37-A2", "complete")
	df37AppendRawStatus(t, childB, "DF37-B1", "in_progress")
	df37AppendRawStatus(t, childB, "DF37-B2", "failed")

	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--list", "--status", "complete,in_progress"})
	if code != 0 {
		t.Fatalf("umbrella --list --status complete,in_progress exit = %d, want 0 (out: %s)", code, out)
	}
	if !df37RowPresent(out, "childA", "DF37-A2") || !df37RowPresent(out, "childB", "DF37-B1") {
		t.Fatalf("comma-filtered list lost the OR-matched rows:\n%s", out)
	}
	for _, leak := range []string{"DF37-A1", "DF37-B2"} {
		if strings.Contains(out, leak) {
			t.Fatalf("comma-filtered list leaked non-matching row %s:\n%s", leak, out)
		}
	}
}

// Alias handling consistent with the validate/sweep-status vocabulary
// policy: aliases normalize to their canonical value, so a filter written
// entirely in aliases ("completed") still lists a stored "done" row, and
// two aliases of the SAME value ("done,completed") collapse to one arm —
// exactly one matching row either way. (todo->pending means a
// "completed,todo" filter legitimately matches pending rows too.)
func TestDFBoardCTL37UmbrellaListAliasesNormalize(t *testing.T) {
	umbrella, childA, _ := bt078Fixture(t)
	bt078Create(t, childA, "DF37-A1", "childA pending")
	df37AppendRawStatus(t, childA, "DF37-A2", "done") // stored alias spelling

	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--list", "--status", "completed"})
	if code != 0 {
		t.Fatalf("umbrella --list --status completed exit = %d, want 0 (out: %s)", code, out)
	}
	if !df37RowPresent(out, "childA", "DF37-A2") {
		t.Fatalf("alias filter (completed->complete) must list the stored \"done\" row:\n%s", out)
	}
	if strings.Contains(out, "DF37-A1") {
		t.Fatalf("filter [completed] must drop the pending row:\n%s", out)
	}

	// done,completed collapse to the same canonical arm — still the one row.
	out, code = bt078Run(t, []string{"-C", umbrella, "umbrella", "--list", "--status", "done,completed"})
	if code != 0 {
		t.Fatalf("umbrella --list --status done,completed exit = %d, want 0 (out: %s)", code, out)
	}
	if !df37RowPresent(out, "childA", "DF37-A2") || strings.Contains(out, "DF37-A1") {
		t.Fatalf("duplicate-alias filter must keep exactly the done row:\n%s", out)
	}
}

func TestDFBoardCTL37UmbrellaListUnknownStatusUsageError(t *testing.T) {
	umbrella, childA, _ := bt078Fixture(t)
	bt078Create(t, childA, "DF37-A1", "childA pending")

	out, code := bt078Run(t, []string{"-C", umbrella, "umbrella", "--list", "--status", "nonsense"})
	if code != 2 {
		t.Fatalf("unknown status exit = %d, want 2 (usage) (out: %s)", code, out)
	}
	if got := bt078LastStderr; !strings.Contains(got, "nonsense") || !strings.Contains(got, "vocabulary") {
		t.Fatalf("unknown-status error must name the value and the vocabulary:\n%s", got)
	}

	// One bad value in an otherwise valid list fails the whole flag.
	_, code = bt078Run(t, []string{"-C", umbrella, "umbrella", "--list", "--status", "pending,bogus"})
	if code != 2 {
		t.Fatalf("list with one unknown value exit = %d, want 2", code)
	}
}

// --status is a --list-only filter: the bare census (and every other mode)
// refuses it, exit 2 (usage class), and --json + --list + --status stays a
// usage conflict.
func TestDFBoardCTL37UmbrellaStatusIsListOnly(t *testing.T) {
	umbrella, childA, _ := bt078Fixture(t)
	bt078Create(t, childA, "DF37-A1", "childA pending")

	for _, args := range [][]string{
		{"-C", umbrella, "umbrella", "--status", "pending"},
		{"-C", umbrella, "umbrella", "--stats", "--status", "pending"},
		{"-C", umbrella, "umbrella", "--validate", "--status", "pending"},
		{"-C", umbrella, "umbrella", "--json", "--list", "--status", "pending"},
	} {
		if _, code := bt078Run(t, args); code != 2 {
			t.Fatalf("umbrella %v exit = %d, want 2 (usage)", args[2:], code)
		}
	}
}
