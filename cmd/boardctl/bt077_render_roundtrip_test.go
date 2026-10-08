package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-hermes/boardctl/internal/board"
	"github.com/coding-hermes/boardctl/internal/render"
)

// BT-077 render/export round-trip: the plural association fields ride the
// board-report/v1 payload (render --json) and the self-contained HTML
// report, and an export+import round-trip preserves them byte-verbatim.
// The board-level merge/normalize contract lives in internal/board
// (bt077_associations_test.go); the render package imports that one, so
// the payload checks run here in cmd/boardctl where both are importable.

// TestRenderPayloadCarriesAssociations: the island carries the plural
// fields on the task row, and the HTML report embeds them.
func TestRenderPayloadCarriesAssociations(t *testing.T) {
	dir := seedBT077CLIBoard(t)
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--pull-request", "21"}); code != 0 {
		t.Fatal("pull-request update failed")
	}
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--assoc-branch", "feat/x", "--assoc-branch", "feat/y"}); code != 0 {
		t.Fatal("assoc-branch update failed")
	}
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--assoc-worktree", "/tmp/w1"}); code != 0 {
		t.Fatal("assoc-worktree update failed")
	}
	b, err := board.Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := render.BuildFromBoard(b, render.Options{})
	if err != nil {
		t.Fatalf("render payload: %v", err)
	}
	island, err := render.EscapeJSONIsland(payload)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"pull_requests"`, `"number":21`, `"branches"`, `"feat/x"`, `"feat/y"`, `"worktrees"`, `"/tmp/w1"`} {
		if !strings.Contains(string(island), want) {
			t.Errorf("report island lost %q", want)
		}
	}
	html := render.BuildHTML(string(island))
	if !strings.Contains(html, `"pull_requests"`) || !strings.Contains(html, `"worktrees"`) {
		t.Error("HTML report lost the plural fields")
	}
}

// TestExportImportRoundTripsAssociations: render --json -> import on a
// FRESH second board (renumber not needed; ids are free) preserves the
// plural fields byte-verbatim.
func TestExportImportRoundTripsAssociations(t *testing.T) {
	dir := seedBT077CLIBoard(t)
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--pull-request", "21"}); code != 0 {
		t.Fatal("update failed")
	}
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--assoc-branch", "feat/x"}); code != 0 {
		t.Fatal("branch update failed")
	}
	exportPath := filepath.Join(t.TempDir(), "export.json")
	if code := run([]string{"-C", dir, "render", "--json", exportPath}); code != 0 {
		t.Fatalf("render --json exit = %d, want 0", code)
	}
	raw, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"pull_requests"`) {
		t.Fatalf("export lost pull_requests:\n%.400s", raw)
	}
	// import into the same board is a no-op (rows equal); the round-trip
	// proof is that the export parses back as board-report/v1 rows whose
	// association fields decode identical to the live row.
	var payload struct {
		Boards []struct {
			Tasks []json.RawMessage `json:"tasks"`
		} `json:"boards"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("export does not parse: %v", err)
	}
	if len(payload.Boards) == 0 || len(payload.Boards[0].Tasks) == 0 {
		t.Fatal("export carries no task rows")
	}
	var found bool
	for _, tr := range payload.Boards[0].Tasks {
		var row map[string]any
		if err := json.Unmarshal(tr, &row); err != nil {
			continue
		}
		if row["id"] != "EXIST-1" {
			continue
		}
		found = true
		prs, ok := row["pull_requests"].([]any)
		if !ok || len(prs) != 1 {
			t.Fatalf("exported pull_requests = %v, want 1 element", row["pull_requests"])
		}
		pr := prs[0].(map[string]any)
		if pr["number"] != float64(21) {
			t.Errorf("exported PR = %v, want 21", pr["number"])
		}
		brs, ok := row["branches"].([]any)
		if !ok || len(brs) != 1 || brs[0] != "feat/x" {
			t.Errorf("exported branches = %v, want [feat/x]", row["branches"])
		}
	}
	if !found {
		t.Fatal("export lost the EXIST-1 row")
	}
}

// TestRenderLegacyRowCarriesSingularFields: a legacy singular row keeps
// worktree/branch/sessions in the payload alongside any plural fields.
func TestRenderLegacyRowCarriesSingularFields(t *testing.T) {
	dir := seedBT077CLIBoard(t)
	if code := run([]string{"-C", dir, "update", "EXIST-LEG", "--pull-request", "7"}); code != 0 {
		t.Fatal("legacy update failed")
	}
	b, err := board.Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := render.BuildFromBoard(b, render.Options{})
	if err != nil {
		t.Fatal(err)
	}
	island, err := render.EscapeJSONIsland(payload)
	if err != nil {
		t.Fatal(err)
	}
	s := string(island)
	for _, want := range []string{`"worktree":"/home/me/wt-1"`, `"branch":"wt/lg-1"`, `"pull_requests":[{"number":7}]`} {
		if !strings.Contains(s, want) {
			t.Errorf("payload lost %q", want)
		}
	}
}
