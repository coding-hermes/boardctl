package board

// DF-BOARDCTL-46: normalize-board's refusal on an unparseable line must be
// actionable. Before this change the error named the file and the line (and
// the duplicate key, for that defect class) but stopped there — while the CLI
// help advertised --skip-bad-lines, a degraded read the WRITING command could
// never honor without rewriting rows it had not classified. The contract
// pinned here, at the library seam the CLI prints verbatim:
//
//   - an unparseable line refuses NormalizeBoard in BOTH modes (dry-run and
//     apply), with the file path and 1-based line number;
//   - the refusal carries NormalizeBoardRepairHint, in the order an operator
//     must follow it: validate --repair -> review tasks.rewritten.jsonl ->
//     swap the reviewed file -> re-run normalize-board;
//   - nothing is written: tasks.jsonl is byte-identical and no
//     tasks.normalized.jsonl staging file appears;
//   - the SkipBad (tolerant-read) flag can no longer turn the refusal into a
//     degraded run — setting it changes nothing, which is what makes the
//     removed CLI flag honest rather than a silent behavior switch.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const df46DupKeyRow = `{"id":"DF46-DUPKEY","title":"duplicate key row","status":"pending","status":"review","priority":"P3"}`

// df46SeedBrokenBoard writes a topology-A board whose tasks.jsonl line 2 is
// the duplicate-key row.
func df46SeedBrokenBoard(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id":"DF46-OK","title":"clean row","status":"pending","priority":"P2"}` + "\n" +
			df46DupKeyRow + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"df46","namespace":"df46","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestDF46NormalizeBoardRefusalCarriesRemedy: the refusal names the defect
// (file + line + duplicate key) AND the ordered remedy.
func TestDF46NormalizeBoardRefusalCarriesRemedy(t *testing.T) {
	b := df46SeedBrokenBoard(t)
	before, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, apply := range []bool{false, true} {
		_, err := b.NormalizeBoard(apply)
		if err == nil {
			t.Fatalf("NormalizeBoard(apply=%v) on a duplicate-key board = nil error, want refusal", apply)
		}
		msg := err.Error()
		for _, want := range []string{
			b.tasksPath,
			"line 2",
			`duplicate key "status"`,
			NormalizeBoardRepairHint,
		} {
			if !strings.Contains(msg, want) {
				t.Errorf("apply=%v refusal missing %q:\n%s", apply, want, msg)
			}
		}
		// Order: salvage, review, swap, re-run. Out of order = wrong recipe.
		prev := -1
		for _, step := range []string{"validate --repair", "tasks.rewritten.jsonl", "swap", "re-run"} {
			i := strings.Index(msg, step)
			if i <= prev {
				t.Errorf("apply=%v refusal step %q out of order (index %d <= %d):\n%s", apply, step, i, prev, msg)
			}
			prev = i
		}
		after, rerr := os.ReadFile(b.tasksPath)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if string(before) != string(after) {
			t.Fatalf("apply=%v refusal rewrote tasks.jsonl:\nbefore %s\nafter  %s", apply, before, after)
		}
		if _, serr := os.Stat(filepath.Join(b.Dir, normalizeBoardCacheName)); serr == nil {
			t.Fatalf("apply=%v refusal staged %s (a failed read must write nothing)", apply, normalizeBoardCacheName)
		}
	}
}

// TestDF46NormalizeBoardIgnoresTolerantReadFlag: SkipBad is a read-side
// degrade for list/show/stats/render/validate. On a WRITING repair it must
// not silently succeed — otherwise the removed CLI flag would remain
// reachable from any library caller and the refusal would be advisory.
func TestDF46NormalizeBoardIgnoresTolerantReadFlag(t *testing.T) {
	b := df46SeedBrokenBoard(t)
	b.SkipBad = true
	rep, err := b.NormalizeBoard(true)
	if err == nil {
		t.Fatalf("NormalizeBoard(apply) with SkipBad=true = report %+v, nil error; want the same refusal", rep)
	}
	if !strings.Contains(err.Error(), "validate --repair") {
		t.Errorf("tolerant-mode refusal lost the remedy:\n%s", err)
	}
	if _, serr := os.Stat(filepath.Join(b.Dir, normalizeBoardCacheName)); serr == nil {
		t.Errorf("tolerant-mode refusal staged %s", normalizeBoardCacheName)
	}
}

// TestDF46RepairHintNamesEveryStep is the doc/behavior agreement pin: the
// hint text is the single source both the refusal and the CLI help print, so
// it must keep naming each step of the documented procedure.
func TestDF46RepairHintNamesEveryStep(t *testing.T) {
	for _, want := range []string{"validate --repair", "tasks.rewritten.jsonl", "swap", "tasks.jsonl", "re-run", "normalize-board"} {
		if !strings.Contains(NormalizeBoardRepairHint, want) {
			t.Errorf("NormalizeBoardRepairHint lost %q: %s", want, NormalizeBoardRepairHint)
		}
	}
	if strings.Contains(NormalizeBoardRepairHint, "skip-bad-lines") {
		t.Errorf("NormalizeBoardRepairHint still offers --skip-bad-lines: %s", NormalizeBoardRepairHint)
	}
}
