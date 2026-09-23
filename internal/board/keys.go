package board

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Task-row key canon.
//
// WHY THIS EXISTS (BT-056): validate checked status/priority vocabularies and
// duplicate ids but never the ROW SHAPE, so the fleet drifted into ~230 distinct
// keys across 8,422 rows — including one-key dialects (`why` on 19 rows, all in
// one project), composite squatters (`lines_added_removed`) and outright typos
// (`review_otes`). Nothing reported it, because "the row parsed" was the only
// shape test. This file DECLARES the canon so validate reports drift against it
// and `update --normalize` can repair an alias.
//
// THE CANON IS DECLARED, NOT INFERRED, and it lives in Go source rather than a
// JSON/TOML manifest: the boards are consumed by this package, so the canon
// belongs with the code that reads them, and a data file would let a board widen
// its own canon.
//
// Tiers:
//   - RequiredTaskRowKeys: keys every task row must carry.
//   - SanctionedTaskRowKeys: every other key boardctl or the fleet may write.
//     This is DefaultTaskRowKeys (the create schema) UNION the measured extras,
//     computed so a change to the create schema can never silently become drift.
//   - the `_note` suffix rule: any `<anything>_note` key is sanctioned, because
//     the fleet annotates rows (`guard_note`, `ci_note`, `rollback_note`,
//     `priority_note`, `stale_note`, `status_note`, ...). Sanctioning the FAMILY
//     rather than listing each name is what keeps the canon from being widened
//     one annotation at a time.
//
// Evidence for the extras: the key census over 57 boards / 8,422 parsed rows
// (2026-09-23). A rare key is listed only when it names a real concept used by
// more than one project family; a rare key that merely respells a canonical one
// belongs in TaskKeyAliases instead — that is the difference between an allowed
// key and drift with a named repair.

// RequiredTaskRowKeys are the keys every task row must carry.
var RequiredTaskRowKeys = []string{"id", "title", "status", "priority"}

// sanctionedExtras is the measured, concept-by-concept addition to the create
// schema (DefaultTaskRowKeys). Every entry is justified by the census: it names
// something the create schema has no field for.
var sanctionedExtras = []string{
	// provenance / lifecycle the create schema omits
	"closed_at", "completed_ts", "completed_tick", "claimed_by_tick",
	"created_by", "filed_by", "found_by", "closed_by", "resolved_by", "dispatched_by",
	"base_commit", "merge_commit", "code_commit", "fix_commit_hash", "followup_commit",
	"work_commit", "worker_commit", "red_commit", "superseded_by", "due_date",
	// work description
	"detail", "description", "source", "type", "acceptance_criteria", "scope_files",
	"worktree", "branch", "sessions",
	// model / worker accounting distinct from the row's primary_* assignment:
	// model_used/provider_used record what ACTUALLY ran, which is why they are
	// sanctioned rather than aliased onto primary_model/primary_provider.
	"model_used", "provider_used", "worker_model", "worker_provider",
	"lines_added_removed", // composite "+523/-43 (13 files)" — no canonical equivalent
	// gates / evaluation beyond guard_result + ci_result
	"judge_verdict", "judge_result", "tier2_verdict", "judge_verdict_artifact",
	"verdict_artifact", "verdict_hash",
	// scheduler / bookkeeping keys the fleet writes around waves and repairs
	"ts", "perpetual", "perpetual_reason", "park_reason", "slot_reused", "slot_note",
	"dup_sim", "dup_note", "board_cleanup", "re_id_from", "repaired_escape",
	"reconciliation", "wave", "waves", "phase", "phases",
	// labels: array-valued free tags, carried by 8 project families. Sanctioned
	// on the census rule (a real concept used by more than one family), NOT
	// because it is convenient — `active` (2 families, true/null) is left OUT
	// for the opposite reason: it may be a dialect of `perpetual`, and resolving
	// that is the owning project's decision, not a rename this canon may guess.
	"labels",
}

// SanctionedTaskRowKeys is the full allowed set: the create schema plus the
// measured extras, deduped and sorted. Computed at init so it cannot drift from
// DefaultTaskRowKeys.
var SanctionedTaskRowKeys = func() []string {
	set := map[string]bool{}
	for _, k := range DefaultTaskRowKeys {
		set[k] = true
	}
	for _, k := range sanctionedExtras {
		set[k] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}()

// TaskKeyAliases maps a non-canonical spelling to its canonical key. These are
// drift the canon does NOT sanction: each respells a key that already exists.
// `boardctl update <id> --normalize` applies them, and REFUSES (never guesses)
// when the canonical key is already present on the row — that would mean
// silently dropping one of two different values.
//
// THE TABLE IS HELD TO EVIDENCE, NOT TO PLAUSIBILITY. Every entry was checked
// against real rows across 57 boards (2026-09-23), and four candidates were
// REMOVED for merging different concepts rather than fixing a spelling:
//   - `commit`: 6 rows carry it, TWO of them also carry `commit_hash` with a
//     DIFFERENT value (commit="afffb64" vs commit_hash="9b4536e") — two
//     different commits, not one misspelled. Renaming it would have rewritten
//     "which commit" on a row that says two things.
//   - `deps`: 7 rows, every value the EMPTY STRING, no `depends_on` anywhere —
//     renaming would put a string where the canon expects a list of ids.
//   - `review_otes`: 2 rows, and it is NOT a typo of `review_notes` — it holds
//     captured cell output as prose while `review_notes` on the same rows holds
//     provenance notes. Different kinds of content behind a similar name.
//   - `review_notes_dupe`: a candidate that turned out to match NO row at all.
//
// They remain reported as drift. A misspelled key that names something the canon
// has no field for needs a human decision, not a rename.
var TaskKeyAliases = map[string]string{
	"why":            "reasoning",
	"acceptance":     "acceptance_criteria",
	"pass_criteria":  "acceptance_criteria",
	"_pass_criteria": "acceptance_criteria", // 222 rows, 0 conflicts, values are pass conditions
	"ac":             "acceptance_criteria",
	"pri":            "priority",
	"cpx":            "complexity",
}

var requiredTaskKeys = func() map[string]bool {
	m := make(map[string]bool, len(RequiredTaskRowKeys))
	for _, k := range RequiredTaskRowKeys {
		m[k] = true
	}
	return m
}()

var sanctionedTaskKeys = func() map[string]bool {
	m := make(map[string]bool, len(SanctionedTaskRowKeys))
	for _, k := range SanctionedTaskRowKeys {
		m[k] = true
	}
	return m
}()

// IsRequiredTaskKey reports whether k is a required task-row key.
func IsRequiredTaskKey(k string) bool { return requiredTaskKeys[k] }

// IsSanctionedTaskKey reports whether k may appear on a task row: a required
// key, a declared optional key, or any `_note` annotation.
func IsSanctionedTaskKey(k string) bool {
	if requiredTaskKeys[k] || sanctionedTaskKeys[k] {
		return true
	}
	return strings.HasSuffix(k, "_note") && len(k) > len("_note")
}

// CanonicalKeyFor returns the canonical spelling for a non-canonical key, or
// ("", false) when the key is not a known alias.
func CanonicalKeyFor(k string) (string, bool) {
	c, ok := TaskKeyAliases[k]
	return c, ok
}

// UnknownTaskKeys returns the row's keys outside the declared canon, sorted.
// A known alias is deliberately still "unknown": it is drift, and it has a
// named repair rather than an exemption.
func UnknownTaskKeys(row *Row) []string {
	var out []string
	seen := map[string]bool{}
	for _, k := range row.Keys {
		if seen[k] || IsSanctionedTaskKey(k) {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// KeyDriftSummary aggregates key drift for a board, for the validate summary.
type KeyDriftSummary struct {
	Rows    int            // task rows examined
	Clean   int            // rows whose keys are all canon
	Keys    map[string]int // offending key -> row count
	Fixable int            // rows whose only drift is known aliases
}

// DriftRows returns the number of rows carrying at least one unknown key.
func (s KeyDriftSummary) DriftRows() int { return s.Rows - s.Clean }

// AddRow folds one row into the summary.
func (s *KeyDriftSummary) AddRow(row *Row) {
	s.Rows++
	unknown := UnknownTaskKeys(row)
	if len(unknown) == 0 {
		s.Clean++
		return
	}
	if s.Keys == nil {
		s.Keys = map[string]int{}
	}
	fixable := true
	for _, k := range unknown {
		s.Keys[k]++
		target, ok := CanonicalKeyFor(k)
		// Not an alias at all, OR the canonical key is already on the row —
		// present even as `null` — in which case RenameKey REFUSES (two values,
		// one slot) and the row needs an explicit update, not a rename. Both
		// cases are excluded here so the summary never advertises a repair that
		// will not happen.
		if !ok || row.Has(target) {
			fixable = false
		}
	}
	if fixable {
		s.Fixable++
	}
}

// fixableByAlias reports whether every offending key has a declared canonical
// spelling that --normalize can actually apply: the key must be a known alias
// AND its target must be ABSENT from the row. A target that is present — even as
// `null` — means RenameKey would refuse (two values, one slot), so the row needs
// an explicit update instead and must not be advertised as a mechanical repair.
func fixableByAlias(row *Row, unknown []string) bool {
	for _, k := range unknown {
		target, ok := CanonicalKeyFor(k)
		if !ok || row.Has(target) {
			return false
		}
	}
	return len(unknown) > 0
}

// RenameKey renames one key IN PLACE, preserving its position in the row so the
// rewritten line differs only in the key spelling (minimal diff, and the value's
// raw bytes are untouched). It refuses on the two cases where a rename would
// destroy information rather than repair drift:
//   - the target key is already present on the row (two values, one slot)
//   - the source key appears more than once (which occurrence moves?)
func (r *Row) RenameKey(oldKey, newKey string) error {
	if oldKey == newKey {
		return nil
	}
	if _, clash := r.Vals[newKey]; clash {
		return fmt.Errorf("key %q already present — refusing to rename %q over it (decide which value survives by hand)", newKey, oldKey)
	}
	idx, count := -1, 0
	for i, k := range r.Keys {
		if k == oldKey {
			count++
			if idx == -1 {
				idx = i
			}
		}
	}
	switch {
	case count == 0:
		return fmt.Errorf("key %q not present on the row", oldKey)
	case count > 1:
		return fmt.Errorf("key %q appears %d times on the row — refusing ambiguous rename", oldKey, count)
	}
	val, ok := r.Vals[oldKey]
	if !ok {
		return fmt.Errorf("key %q is in the row's key order but has no value — refusing to rename a malformed row", oldKey)
	}
	r.Keys[idx] = newKey
	r.Vals[newKey] = val
	delete(r.Vals, oldKey)
	return nil
}

// Summary renders the one-line key-uniformity summary validate prints: the
// number the fleet quotes as proof of zero drift.
func (s KeyDriftSummary) Summary() string {
	head := "key uniformity: " + strconv.Itoa(s.Clean) + "/" + strconv.Itoa(s.Rows) + " rows carry only canon keys"
	if len(s.Keys) == 0 {
		return head + " (0 drift)"
	}
	keys := make([]string, 0, len(s.Keys))
	for k := range s.Keys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"("+strconv.Itoa(s.Keys[k])+")")
	}
	return head + "; drift " + strconv.Itoa(s.DriftRows()) + " row(s): " + strings.Join(parts, ", ") +
		" — " + strconv.Itoa(s.Fixable) + " fixable by 'boardctl update <id> --normalize'"
}
