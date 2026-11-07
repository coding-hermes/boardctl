package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CLI coverage for the initiative key: create/update/list/initiatives, plus the
// refusals that keep the key queryable. The board-level rules (slug shape,
// filter semantics, rollup counting) are pinned in internal/board/initiative_test.go;
// these tests pin the surfaces an operator or foreman actually types.

func cliRowHasInitiativeKey(t *testing.T, dir, id string) (string, bool) {
	t.Helper()
	var row map[string]any
	if err := json.Unmarshal([]byte(cliTaskLine(t, dir, id)), &row); err != nil {
		t.Fatal(err)
	}
	v, ok := row["initiative"]
	if !ok {
		return "", false
	}
	s, _ := v.(string)
	return s, true
}

func TestCmdCreateAndListByInitiative(t *testing.T) {
	dir := seedCLIBoard(t)
	if code := run([]string{"-C", dir, "create", "--id", "INIT-1", "--title", "scoped",
		"--initiative", "lane-hooks"}); code != 0 {
		t.Fatalf("create --initiative exit = %d, want 0", code)
	}
	if code := run([]string{"-C", dir, "create", "--id", "INIT-2", "--title", "other scope",
		"--initiative", "remote-dispatch"}); code != 0 {
		t.Fatalf("create second initiative exit = %d, want 0", code)
	}
	if code := run([]string{"-C", dir, "create", "--id", "INIT-3", "--title", "unscoped"}); code != 0 {
		t.Fatalf("create unscoped exit = %d, want 0", code)
	}

	// The unscoped row must carry NO initiative key at all — an empty-string
	// key would be a second, invisible initiative.
	if v, ok := cliRowHasInitiativeKey(t, dir, "INIT-3"); ok {
		t.Fatalf("unscoped row wrote initiative=%q; the key must be absent", v)
	}

	out, _ := captureStdout(func() {
		if code := run([]string{"-C", dir, "list", "--initiative", "lane-hooks", "--json"}); code != 0 {
			t.Errorf("list --initiative exit = %d, want 0", code)
		}
	})
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("list --json output is not JSON: %v\n%s", err, out)
	}
	if len(rows) != 1 || rows[0]["id"] != "INIT-1" {
		t.Fatalf("list --initiative lane-hooks returned %v, want only INIT-1", ids(rows))
	}
}

func TestCmdCreateRejectsNonSlugInitiative(t *testing.T) {
	dir := seedCLIBoard(t)
	for _, bad := range []string{"Lane Hooks", "lane_hooks", "Lane-Hooks", "-lane", "lane-"} {
		if code := run([]string{"-C", dir, "create", "--id", "BAD-1", "--title", "x",
			"--initiative", bad}); code == 0 {
			t.Errorf("create --initiative %q exited 0, want a refusal", bad)
		}
	}
	// Nothing was written by the refusals: the id must be absent entirely.
	if line := cliLineOrEmpty(t, dir, "BAD-1"); line != "" {
		t.Fatalf("a refused create left a row on the board: %s", line)
	}
}

func TestCmdUpdateSetsAndClearsInitiative(t *testing.T) {
	dir := seedCLIBoard(t)
	if code := run([]string{"-C", dir, "create", "--id", "UPD-1", "--title", "t"}); code != 0 {
		t.Fatalf("create exit = %d", code)
	}
	if code := run([]string{"-C", dir, "update", "UPD-1", "--initiative", "xml-spec-graph"}); code != 0 {
		t.Fatalf("update --initiative exit = %d, want 0", code)
	}
	if v, ok := cliRowHasInitiativeKey(t, dir, "UPD-1"); !ok || v != "xml-spec-graph" {
		t.Fatalf("initiative = %q (present=%v), want xml-spec-graph", v, ok)
	}
	if code := run([]string{"-C", dir, "update", "UPD-1", "--clear-initiative"}); code != 0 {
		t.Fatalf("update --clear-initiative exit = %d, want 0", code)
	}
	if v, ok := cliRowHasInitiativeKey(t, dir, "UPD-1"); ok {
		t.Fatalf("initiative survived --clear-initiative: %q", v)
	}
	// Both flags together is a usage error, not a precedence rule.
	if code := run([]string{"-C", dir, "update", "UPD-1", "--initiative", "x",
		"--clear-initiative"}); code == 0 {
		t.Fatal("--initiative with --clear-initiative exited 0, want a refusal")
	}
}

func TestCmdInitiativesRollsUpAndFilters(t *testing.T) {
	dir := seedCLIBoard(t)
	run([]string{"-C", dir, "create", "--id", "ROLL-1", "--title", "a", "--initiative", "lane-hooks"})
	run([]string{"-C", dir, "create", "--id", "ROLL-2", "--title", "b", "--initiative", "lane-hooks", "--deferred"})
	run([]string{"-C", dir, "create", "--id", "ROLL-3", "--title", "c", "--initiative", "remote-dispatch"})

	out, _ := captureStdout(func() {
		if code := run([]string{"-C", dir, "initiatives", "--json"}); code != 0 {
			t.Errorf("initiatives --json exit = %d, want 0", code)
		}
	})
	var sums []struct {
		Initiative string         `json:"initiative"`
		Total      int            `json:"total"`
		Actionable int            `json:"actionable"`
		Deferred   int            `json:"deferred"`
		ByStatus   map[string]int `json:"by_status"`
	}
	if err := json.Unmarshal([]byte(out), &sums); err != nil {
		t.Fatalf("initiatives --json is not JSON: %v\n%s", err, out)
	}
	if len(sums) != 2 {
		t.Fatalf("got %d initiatives, want 2: %s", len(sums), out)
	}
	var lh *struct {
		Initiative string         `json:"initiative"`
		Total      int            `json:"total"`
		Actionable int            `json:"actionable"`
		Deferred   int            `json:"deferred"`
		ByStatus   map[string]int `json:"by_status"`
	}
	for i := range sums {
		if sums[i].Initiative == "lane-hooks" {
			lh = &sums[i]
		}
	}
	if lh == nil {
		t.Fatalf("lane-hooks missing from rollup: %s", out)
	}
	if lh.Total != 2 || lh.Deferred != 1 || lh.Actionable != 1 {
		t.Fatalf("lane-hooks total=%d deferred=%d actionable=%d, want 2/1/1", lh.Total, lh.Deferred, lh.Actionable)
	}
	// A foreman that is told an initiative slug it does not recognise must be
	// able to tell "wrong slug" from "no work left": the rollup is the check.
	text, _ := captureStdout(func() {
		if code := run([]string{"-C", dir, "initiatives"}); code != 0 {
			t.Errorf("initiatives text exit = %d", code)
		}
	})
	if !strings.Contains(text, "lane-hooks") || !strings.Contains(text, "remote-dispatch") {
		t.Fatalf("initiatives table missing a slug:\n%s", text)
	}
}

func ids(rows []map[string]any) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if s, ok := r["id"].(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// cliLineOrEmpty returns the row line for id, or "" when the id is absent —
// the absence is the assertion for refusals, so it must not Fatalf.
func cliLineOrEmpty(t *testing.T, dir, id string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(l), &row); err != nil {
			continue
		}
		if row["id"] == id {
			return l
		}
	}
	return ""
}
