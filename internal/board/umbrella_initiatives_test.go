package board

// BT-093 library tests: the umbrella-scoped initiative rollup over
// umbFixture's real boards. Table-driven per the task contract:
//
//   - AC1/AC5: umbrella + two children carrying the same slug aggregate to
//     ONE bucket holding TWO repo entries (same slug on the umbrella's own
//     board adds a third), totals equal the sum of the per-board rollups;
//   - AC2: the counting rules come from the single-board rollup itself —
//     deferred excluded from actionable, complete/failed excluded, unscoped
//     rows never bucketed (verified through the aggregate's bucket data);
//   - AC3: a corrupt child becomes a named ERROR finding and its rows are
//     ABSENT from the totals while the healthy boards still aggregate.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// umbSeedInitiativeRow writes one task row carrying an initiative slug (and
// optional status/deferred) through the real write path.
func umbSeedInitiativeRow(t *testing.T, repo, id, slug, status string, deferred bool) {
	t.Helper()
	b, err := Resolve(repo)
	if err != nil {
		t.Fatal(err)
	}
	spec := TaskRowSpec{
		ID:         id,
		Title:      "bt093 " + id,
		Status:     status,
		Priority:   "P2",
		Initiative: func() *string { s := slug; return &s }(),
	}
	if deferred {
		spec.Deferred = func() *bool { v := true; return &v }()
		spec.DeferReason = func() *string { s := "blocked on the BT-093 fixture"; return &s }()
	}
	if _, err := b.Create(spec); err != nil {
		t.Fatalf("create %s in %s: %v", id, repo, err)
	}
}

// umbSeedCompleteRow writes one status=complete task row VERBATIM into
// tasks.jsonl: the hand-edited shape the TR-251 completion gate forbids on
// the CLI path (that gate is a write-surface contract; the umbrella rollup
// is a read surface over boards that legitimately contain such rows).
func umbSeedCompleteRow(t *testing.T, repo, id, slug string) {
	t.Helper()
	b, err := Resolve(repo)
	if err != nil {
		t.Fatal(err)
	}
	row := map[string]any{
		"id": id, "title": "bt093 " + id, "status": "complete", "priority": "P2",
		"initiative": slug,
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

// findBucket returns the bucket for slug (nil when absent).
func findBucket(aggs []UmbrellaInitiative, slug string) *UmbrellaInitiative {
	for i := range aggs {
		if aggs[i].Initiative == slug {
			return &aggs[i]
		}
	}
	return nil
}

// TestBT093UmbrellaInitiativesAggregate covers AC1+AC5: same slug across the
// umbrella's own board and two children = ONE bucket; each bucket carries one
// repo entry per repo holding rows (QualfiedBoard qualifiers); the bucket
// totals equal the sum of the per-board initiatives.
func TestBT093UmbrellaInitiativesAggregate(t *testing.T) {
	umbrella, childA, childB := umbFixture(t)
	// The SAME slug on THREE boards (own + two children), different shapes.
	umbSeedInitiativeRow(t, umbrella, "BT93-U1", "lane-hooks", "pending", false)
	umbSeedInitiativeRow(t, umbrella, "BT93-U2", "lane-hooks", "blocked", false)
	umbSeedInitiativeRow(t, childA, "BT93-A1", "lane-hooks", "pending", false)
	umbSeedInitiativeRow(t, childA, "BT93-A2", "solo", "pending", false)
	umbSeedInitiativeRow(t, childB, "BT93-B1", "lane-hooks", "in_progress", true)
	umbSeedCompleteRow(t, childB, "BT93-B2", "lane-hooks")
	// Unscoped rows are never bucketed, on any board.
	umbSeedTask(t, childB, "BT93-BUNSCOPED", "no initiative here")

	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	aggs, findings, err := um.Initiatives()
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("healthy umbrella carries aggregate findings: %+v", findings)
	}

	cases := []struct {
		name  string
		slug  string
		want  *UmbrellaInitiative
		total int
	}{
		{"lane-hooks aggregated", "lane-hooks", findBucket(aggs, "lane-hooks"), 5},
		{"solo", "solo", findBucket(aggs, "solo"), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.want == nil {
				t.Fatalf("bucket %q missing from %+v", tc.slug, aggs)
			}
			// ONE bucket, not three: the slug is the cross-board identity.
			count := 0
			for _, a := range aggs {
				if a.Initiative == tc.slug {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("slug %q produced %d buckets, want ONE", tc.slug, count)
			}
			if got := tc.want.Total; got != tc.total {
				t.Fatalf("bucket total = %d, want %d (%+v)", got, tc.total, *tc.want)
			}
		})
	}

	t.Run("lane-hooks one bucket three repo entries, summed exactly", func(t *testing.T) {
		b := findBucket(aggs, "lane-hooks")
		if len(b.Repos) != 3 {
			t.Fatalf("lane-hooks has %d repo entries, want 3 (umbrella+two children): %+v", len(b.Repos), b.Repos)
		}
		if b.Repos[0].Repo != "childA" || b.Repos[1].Repo != "childB" || b.Repos[2].Repo != "umbrella" {
			t.Fatalf("repo entries not sorted by qualifier: %+v", b.Repos)
		}
		// Per-repo breakdown values: childA 1 actionable; childB 2 rows,
		// 1 deferred-excluded, 1 complete-excluded => 0 actionable,
		// 1 deferred; umbrella 2 rows, 0 actionable (pending+blocked:
		// blocked IS actionable under the single-board rules — verify below).
		if b.Repos[0].Total != 1 || b.Repos[0].Actionable != 1 || b.Repos[0].Deferred != 0 {
			t.Fatalf("childA breakdown wrong: %+v", b.Repos[0])
		}
		if b.Repos[1].Total != 2 || b.Repos[1].Actionable != 0 || b.Repos[1].Deferred != 1 {
			t.Fatalf("childB breakdown wrong: %+v", b.Repos[1])
		}
		if b.Repos[2].Total != 2 || b.Repos[2].Actionable != 2 {
			t.Fatalf("umbrella breakdown wrong: %+v", b.Repos[2])
		}
		// ByStatus aggregated across repos.
		if b.ByStatus["pending"] != 2 || b.ByStatus["blocked"] != 1 ||
			b.ByStatus["in_progress"] != 1 || b.ByStatus["complete"] != 1 {
			t.Fatalf("lane-hooks by-status wrong: %+v", b.ByStatus)
		}
	})

	t.Run("totals equal the per-board initiatives sums (AC5)", func(t *testing.T) {
		standalone := map[string]InitiativeSummary{}
		for _, root := range []string{umbrella, childA, childB} {
			b2, err := Resolve(root)
			if err != nil {
				t.Fatal(err)
			}
			sums, err := b2.Initiatives()
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range sums {
				standalone[s.Initiative] = InitiativeSummary{
					Initiative: s.Initiative,
					Total:      standalone[s.Initiative].Total + s.Total,
					Actionable: standalone[s.Initiative].Actionable + s.Actionable,
					Deferred:   standalone[s.Initiative].Deferred + s.Deferred,
				}
			}
		}
		for slug, want := range standalone {
			b := findBucket(aggs, slug)
			if b == nil {
				t.Fatalf("aggregate lacks bucket %q present on the per-board rollup", slug)
			}
			if b.Total != want.Total || b.Actionable != want.Actionable || b.Deferred != want.Deferred {
				t.Fatalf("bucket %q = %d/%#v/%d, want %d/%d/%d",
					slug, b.Total, b.Actionable, b.Deferred, want.Total, want.Actionable, want.Deferred)
			}
		}
	})

	t.Run("unscoped rows never bucketed", func(t *testing.T) {
		if findBucket(aggs, "") != nil {
			t.Fatal("an unscoped row was bucketed under a synthetic key")
		}
	})

	t.Run("AC2 counting rules on own bucket shape", func(t *testing.T) {
		b := findBucket(aggs, "solo")
		if b == nil {
			t.Fatal("solo bucket missing")
		}
		if b.Actionable != 1 || b.Deferred != 0 {
			t.Fatalf("solo = actionable %d deferred %d, want 1/0 (pending, not deferred)", b.Actionable, b.Deferred)
		}
	})
}

// TestBT093UmbrellaInitiativesCorruptChild (AC3): a child board whose
// tasks.jsonl cannot be read becomes a named ERROR-class finding, its rows
// are ABSENT from the totals, and the healthy boards still aggregate.
func TestBT093UmbrellaInitiativesCorruptChild(t *testing.T) {
	umbrella, childA, childB := umbFixture(t)
	umbSeedInitiativeRow(t, childA, "BT93-A1", "lane-hooks", "pending", false)
	umbSeedInitiativeRow(t, childB, "BT93-B1", "lane-hooks", "pending", false)
	// Corrupt childA: garbage line where tasks.jsonl parses nothing.
	bad := filepath.Join(childA, ".coding-hermes", "board", "tasks.jsonl")
	if err := os.WriteFile(bad, []byte("not json\n"), 0600); err != nil {
		t.Fatal(err)
	}

	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	aggs, findings, err := um.Initiatives()
	if err != nil {
		t.Fatalf("aggregate returned a fatal error instead of the degraded finding: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("want exactly 1 finding for the corrupt board, got %+v", findings)
	}
	if !findings[0].IsError() {
		t.Fatalf("corrupt board must be an ERROR-class finding: %+v", findings[0])
	}
	if findings[0].Link != "childA" {
		t.Fatalf("finding must name the corrupt board via its qualifier: %+v", findings[0])
	}
	b := findBucket(aggs, "lane-hooks")
	if b == nil {
		t.Fatal("healthy child's rows lost from the totals")
	}
	if b.Total != 1 {
		t.Fatalf("corrupt child's rows leaked into the totals: %+v", *b)
	}
	for _, r := range b.Repos {
		if r.Repo == "childA" {
			t.Fatalf("excluded board still has a repo entry: %+v", b.Repos)
		}
	}
	if len(b.Repos) != 1 || b.Repos[0].Repo != "childB" {
		t.Fatalf("repo breakdown wrong: %+v", b.Repos)
	}
}

// TestBT093RenderUmbrellaInitiatives pins the text contract: one line per
// initiative with repos listed (the AC4 wording), column order TOTAL,
// ACTIONABLE, DEFERRED, BY STATUS, REPOS.
func TestBT093RenderUmbrellaInitiatives(t *testing.T) {
	aggs := []UmbrellaInitiative{{
		Initiative: "lane-hooks",
		Total:      3, Actionable: 2, Deferred: 1,
		ByStatus: map[string]int{"pending": 2, "in_progress": 1},
		Repos: []RepoInitiativeTotals{
			{Repo: "auth", Total: 2},
			{Repo: "billing", Total: 1},
		},
	}}
	out := RenderUmbrellaInitiatives(aggs)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("render produced %d lines, want one per initiative: %q", len(lines), out)
	}
	for _, want := range []string{"lane-hooks", "3", "2", "1", "pending=2 in_progress=1", "auth=2 billing=1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("render missing %q: %q", want, out)
		}
	}
}

// TestBT093UmbrellaInitiativesJSONShape: the aggregate's JSON carries the
// by-status/repo shapes (the machine surface the CLI --json reuses).
func TestBT093UmbrellaInitiativesJSONShape(t *testing.T) {
	umbrella, childA, childB := umbFixture(t)
	umbSeedInitiativeRow(t, childA, "BT93-A1", "shared-slug", "pending", false)
	umbSeedInitiativeRow(t, childB, "BT93-B1", "shared-slug", "blocked", true)

	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	aggs, _, err := um.Initiatives()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(aggs)
	if err != nil {
		t.Fatal(err)
	}
	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 1 {
		t.Fatalf("want 1 aggregate bucket in JSON, got %d (%s)", len(raw), data)
	}
	if raw[0]["initiative"] != "shared-slug" {
		t.Fatalf("json initiative wrong: %v", raw[0]["initiative"])
	}
	repos, ok := raw[0]["repos"].([]any)
	if !ok || len(repos) != 2 {
		t.Fatalf("json repos wrong: %v", raw[0]["repos"])
	}
	for _, r := range repos {
		m, ok := r.(map[string]any)
		if !ok {
			t.Fatalf("json repo entry shape wrong: %v", r)
		}
		if m["repo"] == nil || m["total"] == nil || m["actionable"] == nil || m["deferred"] == nil {
			t.Fatalf("json repo entry missing repo/total/actionable/deferred: %v", m)
		}
	}
}
