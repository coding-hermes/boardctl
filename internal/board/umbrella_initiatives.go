package board

// BT-093: umbrella-scoped initiative rollup. `initiatives` on ONE repo
// answers "what programmes live on this board"; standing in an umbrella repo
// a programme spanning repos could not be answered from one command. This
// file wires the initiative rollup onto the BT-078 umbrella view: the
// aggregation runs over the umbrella's OWN board plus every child board
// reachable through the QualifiedBoard view, reusing the SAME counting
// rules (Board.Initiatives — the fixture filter, the deferred/actionable
// split, the unscoped-rows-never-bucketed rule) so the two surfaces cannot
// drift.
//
// Bucket identity across the view is the initiative SLUG itself (read-side
// comparison is case-insensitive and write-side values are normalized
// lowercase, so the same slug on two boards IS the same programme). Each
// bucket carries the per-repo breakdown keyed by the existing repo
// qualifiers (QualifiedBoard.Repo) — colliding SLUGS across boards, like
// colliding ids, keep their provenance but sum into one bucket with one
// entry per repo that carries rows for it.
//
// Fail-closed discipline (AC3, mirroring umbrella --stats): a child board
// that fails to read becomes a named ERROR-class finding and its rows are
// ABSENT from the totals — the healthy boards' totals still render, but the
// command's exit makes the degraded output impossible to read as a clean
// success. The method never writes: it consumes the read-only view only.

import (
	"fmt"
	"sort"
	"strings"
)

// RepoInitiativeTotals is one board's contribution to one initiative bucket.
// Repo is the existing umbrella repo qualifier (QualifiedBoard.Repo). The
// counts are that board's own InitiativeSummary for the slug. A repo entry
// appears only when the repo actually carries bucketed rows for the slug.
type RepoInitiativeTotals struct {
	Repo       string         `json:"repo"`
	BoardDir   string         `json:"board_dir,omitempty"`
	Total      int            `json:"total"`
	Actionable int            `json:"actionable"`
	Deferred   int            `json:"deferred"`
	ByStatus   map[string]int `json:"by_status,omitempty"`
}

// UmbrellaInitiative is one initiative rolled up across the whole view:
// per-initiative totals aggregated over the umbrella's own board plus every
// child board, plus the per-repo breakdown. Counting rules match the
// single-board rollup exactly because the numbers ARE those rollups.
type UmbrellaInitiative struct {
	Initiative string                 `json:"initiative"`
	Total      int                    `json:"total"`
	Actionable int                    `json:"actionable"`
	Deferred   int                    `json:"deferred"`
	ByStatus   map[string]int         `json:"by_status"`
	Repos      []RepoInitiativeTotals `json:"repos"`
}

// Initiatives aggregates the per-initiative rollup across every board in the
// view, in board order (umbrella's own board first, then children by
// qualifier). It returns the ordered buckets plus the AGGREGATE findings:
// one ERROR-class finding per board whose read failed, whose rows are absent
// from the totals (degraded totals must never read as a clean success). The
// caller renders the findings with the umbrella view's own discovery
// findings and gates the exit on them. A whole-view read failure (an error
// Umbrella.OpenUmbrella itself already refused, or a non-read defect) still
// returns an error; a per-board read failure is exactly the degraded case.
func (u *Umbrella) Initiatives() ([]UmbrellaInitiative, []UmbrellaFinding, error) {
	by := map[string]*UmbrellaInitiative{}
	var findings []UmbrellaFinding
	for _, qb := range u.Boards {
		sums, err := qb.Board.Initiatives()
		if err != nil {
			findings = append(findings, UmbrellaFinding{
				Level: "error",
				Link:  qb.Repo,
				Msg: fmt.Sprintf("board %s (%s) unreadable: %v — this board's initiative rows are ABSENT from the totals (degraded view)",
					qb.Repo, qb.BoardDir, err),
			})
			continue
		}
		for _, s := range sums {
			agg := by[s.Initiative]
			if agg == nil {
				agg = &UmbrellaInitiative{Initiative: s.Initiative, ByStatus: map[string]int{}}
				by[s.Initiative] = agg
			}
			agg.Total += s.Total
			agg.Actionable += s.Actionable
			agg.Deferred += s.Deferred
			for k, v := range s.ByStatus {
				agg.ByStatus[k] += v
			}
			agg.Repos = append(agg.Repos, RepoInitiativeTotals{
				Repo:       qb.Repo,
				BoardDir:   qb.BoardDir,
				Total:      s.Total,
				Actionable: s.Actionable,
				Deferred:   s.Deferred,
				ByStatus:   mergeCopyByStatus(s.ByStatus),
			})
		}
	}
	out := make([]UmbrellaInitiative, 0, len(by))
	for _, agg := range by {
		sort.Slice(agg.Repos, func(i, j int) bool { return agg.Repos[i].Repo < agg.Repos[j].Repo })
		out = append(out, *agg)
	}
	// Same order contract as the single-board rollup: total-descending, then
	// slug, so the biggest programmes come first and the output diffs cleanly.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Initiative < out[j].Initiative
	})
	return out, findings, nil
}

// mergeCopyByStatus returns a defensive copy of a by-status tally (an
// aggregate must not alias a board's live map).
func mergeCopyByStatus(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// RenderUmbrellaInitiatives renders the per-initiative aggregate table: one
// line per initiative with its totals and the per-repo breakdown (repo=total
// pairs, repo-sorted) — one line per initiative, repos listed, as the
// command's text contract.
func RenderUmbrellaInitiatives(aggs []UmbrellaInitiative) string {
	var sb strings.Builder
	for _, a := range aggs {
		var byStatus []string
		for _, st := range []string{"pending", "in_progress", "review", "blocked", "failed", "complete"} {
			if n := a.ByStatus[st]; n > 0 {
				byStatus = append(byStatus, fmt.Sprintf("%s=%d", st, n))
			}
		}
		var repos []string
		for _, r := range a.Repos {
			repos = append(repos, fmt.Sprintf("%s=%d", r.Repo, r.Total))
		}
		fmt.Fprintf(&sb, "%s\t%d\t%d\t%d\t%s\t%s\n",
			a.Initiative, a.Total, a.Actionable, a.Deferred,
			strings.Join(byStatus, " "), strings.Join(repos, " "))
	}
	return sb.String()
}
