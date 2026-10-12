package board

// DF-BOARDCTL-34: umbrella --stats gate on the canonical priority vocabulary.
// ComputeStats tallies the RAW stored priority label verbatim, so a
// hand-edited row (or an embedded-JSON accident like
// "P1\",\"source\":\"qa-dagger") used to ride into the aggregated per-repo
// and grand-total priority maps as a bucket key. The umbrella result must
// never emit non-vocabulary keys; invalid values become findings with
// repo/board provenance instead of silently counting as canonical priorities.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// corruptPriority is the exact shape observed on a real dogfood run: an
// embedded-JSON priority value whose bucket key carried escaped quotes.
const corruptPriority = `P1\",\"source\":\"qa-dagger`

// umbAppendPriority writes one task row with a verbatim priority VALUE into a
// repo's tasks.jsonl — the path for priorities the write path refuses
// (hand-edited or legacy drift). The row is otherwise the create default
// shape (pending status).
func umbAppendPriority(t *testing.T, repo, id, title, rawPriority string) {
	t.Helper()
	b, err := Resolve(repo)
	if err != nil {
		t.Fatal(err)
	}
	row := map[string]any{
		"id": id, "title": title, "status": "pending", "priority": rawPriority,
	}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(b.TasksPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck
	if _, err := f.Write(append(data, '\n')); err != nil {
		t.Fatal(err)
	}
}

func TestDFBoardCTL34UmbrellaStatsRejectsInvalidPriorities(t *testing.T) {
	umbrella, childA, childB := umbFixture(t)

	// Canonical values via the real write path (default P2) and raw rows for
	// every other P0..P5 spelling — valid buckets must keep counting.
	umbSeedTask(t, childA, "DF34-A1", "childA p2")
	umbSeedTask(t, childB, "DF34-B1", "childB p2")
	umbAppendPriority(t, childA, "DF34-A2", "childA p0", "P0")
	umbAppendPriority(t, childA, "DF34-A3", "childA p1", "P1")
	umbAppendPriority(t, childA, "DF34-A4", "childA p3", "P3")
	umbAppendPriority(t, childB, "DF34-B2", "childB p4", "P4")
	umbAppendPriority(t, childB, "DF34-B3", "childB p5", "P5")

	// Invalid stored values: the embedded-JSON accident and a canonical-shaped
	// out-of-vocabulary value.
	umbAppendPriority(t, childA, "DF34-A5", "childA corrupt", corruptPriority)
	umbAppendPriority(t, childB, "DF34-B4", "childB p9", "P9")

	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	us, err := um.Stats(TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}

	// AC1: no non-vocabulary bucket key in the grand-total map, and none in
	// any per-repo map either (the per-repo stats in the umbrella result are
	// the sanitized view, not the raw ComputeStats).
	for _, bad := range []string{corruptPriority, "P9"} {
		if _, ok := us.Priority[bad]; ok {
			t.Fatalf("grand-total priority map carries invalid bucket %q: %+v", bad, us.Priority)
		}
	}
	for _, rs := range us.Repos {
		for k := range rs.Stats.Priority {
			if !PriorityVocabulary[k] {
				t.Fatalf("repo %s stats carry non-vocabulary priority bucket %q: %+v", rs.Repo, k, rs.Stats.Priority)
			}
		}
	}

	// AC3: valid P0..P5 buckets count correctly, aggregated across repos
	// (P2 is created in both children).
	wantPriority := map[string]int{
		"P0": 1, "P1": 1, "P2": 2, "P3": 1, "P4": 1, "P5": 1,
	}
	if len(us.Priority) != len(wantPriority) {
		t.Fatalf("grand-total priority buckets = %v, want exactly %v", us.Priority, wantPriority)
	}
	for k, want := range wantPriority {
		if got := us.Priority[k]; got != want {
			t.Fatalf("grand-total priority %q = %d, want %d (map: %v)", k, got, want, us.Priority)
		}
	}

	// AC2: findings carry repo + board provenance, the invalid value, and the
	// row count — in deterministic order (board order, then sorted values).
	if len(us.Findings) != 2 {
		t.Fatalf("got %d priority findings, want 2: %+v", len(us.Findings), us.Findings)
	}
	fA, fB := us.Findings[0], us.Findings[1]
	if fA.Repo != "childA" || fA.Value != corruptPriority || fA.Count != 1 {
		t.Fatalf("childA finding wrong: %+v", fA)
	}
	if !strings.Contains(fA.BoardDir, "childA") || fA.BoardDir == "" {
		t.Fatalf("childA finding lacks board provenance: %+v", fA)
	}
	if !strings.Contains(fA.Msg, "childA") || !strings.Contains(fA.Msg, "P0,P1,P2,P3,P4,P5") {
		t.Fatalf("childA finding message lacks provenance/vocabulary: %s", fA.Msg)
	}
	if fB.Repo != "childB" || fB.Value != "P9" || fB.Count != 1 {
		t.Fatalf("childB finding wrong: %+v", fB)
	}
	if !strings.Contains(fB.BoardDir, "childB") {
		t.Fatalf("childB finding lacks board provenance: %+v", fB)
	}

	// Unrelated totals untouched: task totals, status and deferred/actionable
	// still count the invalid-priority rows as rows.
	if us.Total != 9 {
		t.Fatalf("grand total = %d, want 9", us.Total)
	}
	if us.Status["pending"] != 9 || us.Deferred != 0 || us.Actionable != 9 {
		t.Fatalf("status/deferred/actionable wrong: pending=%d deferred=%d actionable=%d",
			us.Status["pending"], us.Deferred, us.Actionable)
	}
	for _, rs := range us.Repos {
		if rs.Stats.Deferred != 0 {
			t.Fatalf("repo %s deferred = %d, want 0", rs.Repo, rs.Stats.Deferred)
		}
	}
	wantRepoTotal := map[string]int{"childA": 5, "childB": 4}
	for _, rs := range us.Repos {
		if rs.Stats.Total != wantRepoTotal[rs.Repo] {
			t.Fatalf("repo %s total = %d, want %d", rs.Repo, rs.Stats.Total, wantRepoTotal[rs.Repo])
		}
	}
	// Per-repo sanitized priority views.
	wantRepoPriority := map[string]map[string]int{
		"childA": {"P0": 1, "P1": 1, "P2": 1, "P3": 1},
		"childB": {"P2": 1, "P4": 1, "P5": 1},
	}
	for _, rs := range us.Repos {
		want := wantRepoPriority[rs.Repo]
		if len(rs.Stats.Priority) != len(want) {
			t.Fatalf("repo %s priority buckets = %v, want %v", rs.Repo, rs.Stats.Priority, want)
		}
		for k, wantN := range want {
			if rs.Stats.Priority[k] != wantN {
				t.Fatalf("repo %s priority %q = %d, want %d", rs.Repo, k, rs.Stats.Priority[k], wantN)
			}
		}
	}

	// Discovery findings are a separate surface — the stats gate adds none.
	if len(um.Findings) != 0 {
		t.Fatalf("discovery findings polluted by the stats gate: %+v", um.Findings)
	}
}
