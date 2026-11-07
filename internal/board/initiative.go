package board

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// InitiativeSlug is the shape an initiative key must have. It is deliberately
// a SLUG, not free text: the whole point of the key is that a caller can type
// it into a flag and get exactly the rows it means. "Lane Hooks", "lane_hooks"
// and "lane-hooks " would otherwise be three different initiatives that no
// query can reconcile, and a filter that fails to match returns an empty set —
// which reads exactly like "no work left", the most expensive possible way to
// be wrong.
//
// Letters, digits and single hyphens, starting and ending alphanumeric, max 64.
var initiativeSlugRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateInitiativeSlug rejects anything that cannot be used as an exact query
// key. It is applied at the WRITE surfaces (create/update) so a bad value can
// never reach a row; the READ surfaces only ever compare, so historical rows are
// never broken by a change to this rule.
func ValidateInitiativeSlug(s string) error {
	if s == "" {
		return fmt.Errorf("initiative must not be empty (omit the flag to leave a row unscoped; pass --clear-initiative to remove an existing one)")
	}
	if len(s) > 64 {
		return fmt.Errorf("initiative %q is %d chars; the limit is 64", s, len(s))
	}
	if initiativeSlugRE.MatchString(s) {
		return nil
	}
	// One suggestion, computed the way a human would fix it: lowercase and
	// collapse every run of non-slug characters into a single hyphen. Suggesting
	// only strings.ToLower would offer "lane hooks" for "Lane Hooks" — a value
	// that is still rejected, which teaches the caller nothing.
	suggestion := strings.ToLower(strings.Trim(s, " -_"))
	for _, r := range []string{" ", "_", "--", "..", "/", "\\"} {
		for strings.Contains(suggestion, r) {
			suggestion = strings.ReplaceAll(suggestion, r, "-")
		}
	}
	suggestion = strings.Trim(suggestion, "-")
	if suggestion == "" || len(suggestion) > 64 || !initiativeSlugRE.MatchString(suggestion) {
		return fmt.Errorf("initiative %q is not a slug: use lowercase letters, digits and single hyphens, starting and ending with a letter or digit (e.g. \"lane-hooks\")", s)
	}
	return fmt.Errorf("initiative %q is not a slug: use lowercase letters, digits and single hyphens, starting and ending with a letter or digit — did you mean %q?", s, suggestion)
}

// InitiativeSummary is one initiative's rollup: how much work is grouped under
// it and in what state. Deferred rows are counted separately and excluded from
// Actionable for the same reason stats excludes them (BT-076) — an initiative
// whose whole remaining work is deferred has no work to scoop.
type InitiativeSummary struct {
	Initiative string         `json:"initiative"`
	Total      int            `json:"total"`
	Actionable int            `json:"actionable"`
	Deferred   int            `json:"deferred"`
	ByStatus   map[string]int `json:"by_status"`
}

// Initiatives groups every task row that carries an initiative key. Rows with
// no initiative are not reported under a synthetic "" bucket: "unscoped" is the
// absence of the key, and inventing a bucket for it would let a caller believe
// everything belongs to an initiative.
//
// Ordered by descending total, then slug, so the biggest programmes come first
// and the order is stable for diffing.
func (b *Board) Initiatives() ([]InitiativeSummary, error) {
	rows, err := b.TaskRows()
	if err != nil {
		return nil, err
	}
	fid, err := b.FixtureIDs()
	if err != nil {
		return nil, err
	}
	kept := make([]*Row, 0, len(rows))
	for _, r := range rows {
		if id := r.String("id"); id != "" && fid[id] {
			continue
		}
		kept = append(kept, r)
	}
	return summarizeRows(kept), nil
}

// summarizeRows is the pure half of Initiatives: given rows that already passed
// the fixture filter, produce the ordered rollup. Split out so the counting
// rules (deferred is not actionable, complete is not actionable, unscoped is not
// an initiative) are testable without a board on disk.
func summarizeRows(rows []*Row) []InitiativeSummary {
	by := map[string]*InitiativeSummary{}
	for _, r := range rows {
		slug := strings.ToLower(strings.TrimSpace(r.String("initiative")))
		if slug == "" {
			continue
		}
		s := by[slug]
		if s == nil {
			s = &InitiativeSummary{Initiative: slug, ByStatus: map[string]int{}}
			by[slug] = s
		}
		s.Total++
		st := NormalizeStatus(r.String("status"))
		if st == "" {
			st = "unknown"
		}
		s.ByStatus[st]++
		if rowDeferred(r) {
			s.Deferred++
			continue
		}
		if st != "complete" && st != "failed" {
			s.Actionable++
		}
	}
	out := make([]InitiativeSummary, 0, len(by))
	for _, s := range by {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Initiative < out[j].Initiative
	})
	return out
}

// rowDeferred reports the BT-076 deferred flag as the row stores it.
//
// The flag is a JSON BOOLEAN, so Row.String — which only unmarshals into a Go
// string — returns "" for it and would silently report every deferred row as
// actionable. Reading the raw value and accepting both spellings keeps a
// hand-edited "deferred":"true" from dropping out of the deferral count too.
func rowDeferred(r *Row) bool {
	raw, ok := r.Vals["deferred"]
	if !ok {
		return false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.EqualFold(strings.TrimSpace(s), "true")
	}
	return false
}
