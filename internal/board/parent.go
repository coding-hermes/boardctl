package board

// BT-099 — the parent/child (subtask) concept: row links, read-time rollups,
// write-time validation, and the board-wide cycle detector.
//
// Doctrine (spec §1/§4):
//   - `parent` is a single-valued ROW key naming another task id on the same
//     board. A row has at most one parent; a parent has unbounded children.
//   - Rollups are COMPUTED at read time — nothing about children is ever
//     stored on the parent row.
//   - Write paths (create/update) validate the link before anything lands;
//     hand-edited rows are caught by validate's read-time checks.
//   - The cycle walk carries a visited set and a depth cap of the row count
//     (panel amendment 4), so a hand-edited pre-existing cycle can never hang
//     the create/update path.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// parentKey is the canon row key holding the subtask link (keys.go).
const parentKey = "parent"

// RowParentID returns the row's parent id ("" when absent, null, or not a
// string). Whitespace is trimmed; an empty value reads as no parent.
func RowParentID(row *Row) string {
	raw := row.Get(parentKey)
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

// isOpenStatus reports whether a normalized status counts as OPEN for rollups
// and ready suppression: the work is not finished (complete) and not dead
// (failed). pending/in_progress/review/blocked are open.
func isOpenStatus(status string) bool {
	return status == "pending" || status == "in_progress" || status == "review" || status == "blocked"
}

// ParentRollup is the read-time child rollup for one parent row (spec §4.1).
// Every field is computed from the board rows on each read — never stored.
type ParentRollup struct {
	ParentID string         `json:"parent"`
	Children int            `json:"children"`  // total children (rows whose parent == this id)
	Open     int            `json:"open"`      // children whose status is open (not complete/failed)
	ByStatus map[string]int `json:"by_status"` // normalized child status -> count
	OpenDeps []string       `json:"open_deps"` // union of children's depends_on refs that are NOT complete (sorted, deduped)
}

// HasChildren reports whether the rollup describes an actual parent (a row
// with at least one child). Rows without children print no rollup block.
func (p *ParentRollup) HasChildren() bool { return p != nil && p.Children > 0 }

// ChildrenLine renders the "CHILDREN: 2/5 open (complete 3, pending 1,
// in_progress 1)" line. Status counts list every status present among the
// children in canonical order (complete and failed first — the settled
// counts — then the open vocabulary), matching the spec example's order.
func (p *ParentRollup) ChildrenLine() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "CHILDREN: %d/%d open", p.Open, p.Children)
	order := []string{"complete", "failed", "pending", "in_progress", "review", "blocked"}
	var parts []string
	for _, status := range order {
		if n := p.ByStatus[status]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", status, n))
		}
	}
	// Any non-vocabulary status still has to show up (counted, never hidden).
	present := map[string]bool{}
	for _, status := range order {
		present[status] = true
	}
	extra := make([]string, 0)
	for status, n := range p.ByStatus {
		if !present[status] && n > 0 {
			extra = append(extra, fmt.Sprintf("%s %d", statusOrUnknown(status), n))
		}
	}
	sort.Strings(extra)
	parts = append(parts, extra...)
	if len(parts) > 0 {
		sb.WriteString(" (" + strings.Join(parts, ", ") + ")")
	}
	return sb.String()
}

// DepsLine renders the "CHILD DEPS (union): ..." line — the union of every
// child's depends_on refs that are not themselves complete (the "what's still
// in the way of this parent" read). Empty union renders "(none)".
func (p *ParentRollup) DepsLine() string {
	if len(p.OpenDeps) == 0 {
		return "CHILD DEPS (union): (none)"
	}
	return "CHILD DEPS (union): " + strings.Join(p.OpenDeps, ", ")
}

// ComputeParentRollup computes the child rollup for parentID from the given
// rows. Returns nil when no row carries parent == parentID (callers print
// nothing for childless rows).
func ComputeParentRollup(parentID string, rows []*Row) *ParentRollup {
	if parentID == "" {
		return nil
	}
	byID := make(map[string]*Row, len(rows))
	for _, row := range rows {
		if id := row.String("id"); id != "" {
			byID[id] = row
		}
	}
	rollup := &ParentRollup{ParentID: parentID, ByStatus: map[string]int{}, OpenDeps: []string{}}
	seenDep := map[string]bool{}
	for _, row := range rows {
		if RowParentID(row) != parentID {
			continue
		}
		rollup.Children++
		status := NormalizeStatus(row.String("status"))
		rollup.ByStatus[status]++
		if isOpenStatus(status) {
			rollup.Open++
		}
		for _, dep := range rowStringSlice(row, "depends_on") {
			if seenDep[dep] {
				continue
			}
			seenDep[dep] = true
			if parent, ok := byID[dep]; ok && NormalizeStatus(parent.String("status")) == "complete" {
				continue // satisfied: omitted from the union
			}
			rollup.OpenDeps = append(rollup.OpenDeps, dep)
		}
	}
	if rollup.Children == 0 {
		return nil
	}
	sort.Strings(rollup.OpenDeps)
	return rollup
}

// ValidateParentLink checks the would-be link "rowID's parent is parentID"
// against the CURRENT board state. It is the write-time gate every create and
// update that writes `parent` runs under the board's write lock:
//
//   - self-parent (rowID == parentID)
//   - parent id does not exist on the board
//   - parent row is terminal (complete or failed) — "open rows only"
//   - the link closes a parent cycle: following parent links upward from the
//     proposed parent reaches rowID. The refusal prints the cycle
//     "A -> B -> C -> A".
//
// The walk carries a visited set and a depth cap of the row count (panel
// amendment 4), so a hand-edited pre-existing cycle terminates instead of
// hanging the write.
func (b *Board) ValidateParentLink(rowID, parentID string) error {
	if parentID == "" {
		return nil
	}
	if parentID == rowID {
		return fmt.Errorf("parent refused for %q: self-parent — a row cannot be its own parent", rowID)
	}
	rows, err := b.TaskRows()
	if err != nil {
		return err
	}
	parentOf := make(map[string]string, len(rows))
	statusOf := make(map[string]string, len(rows))
	known := make(map[string]bool, len(rows))
	for _, row := range rows {
		if id := row.String("id"); id != "" {
			parentOf[id] = RowParentID(row)
			statusOf[id] = NormalizeStatus(row.String("status"))
			known[id] = true
		}
	}
	if !known[parentID] {
		return fmt.Errorf("parent refused for %q: parent task %q does not exist in tasks.jsonl (create the parent first)", rowID, parentID)
	}
	if st := statusOf[parentID]; st == "complete" || st == "failed" {
		return fmt.Errorf("parent refused for %q: parent %q is complete — open rows only", rowID, parentID)
	}
	// Cycle walk: follow parent links from the proposed parent upward. The
	// row is reachable means the new link closes a cycle. chain[0] is the row
	// itself so the printed cycle reads A -> B -> C -> A.
	chain := []string{rowID, parentID}
	pos := map[string]int{rowID: 0, parentID: 1}
	cur := parentOf[parentID]
	for cur != "" {
		if i, ok := pos[cur]; ok {
			cycle := append(append([]string{}, chain[i:]...), cur)
			return fmt.Errorf("parent refused for %q: link closes a parent cycle %s", rowID, strings.Join(cycle, " -> "))
		}
		if len(chain) > len(rows) {
			// Depth cap <= row count (amendment 4): a parent chain longer
			// than the board has rows can only exist if a row appears twice
			// — the walk hit a cycle whose entry the visited set above
			// already names. Refusing here is the belt-and-braces arm.
			return fmt.Errorf("parent refused for %q: parent chain exceeds the board's row count — a hand-edited cycle is present", rowID)
		}
		pos[cur] = len(chain)
		chain = append(chain, cur)
		cur = parentOf[cur]
	}
	return nil
}

// ParentCycles reports every parent cycle on the board as closed id chains
// ("A -> B -> A" rendered by the caller), in first-found order, each reported
// exactly once. Single-valued parent links make the parent graph functional
// (one outgoing edge per row), so each component holds at most one cycle —
// but walks from rows that LEAD INTO a cycle share the cycle's nodes, which
// is exactly where a single global-DFS detector under-reports (panel
// amendment 4's shared-node fixture): every row gets its own walk with a
// LOCAL visited set and the row-count depth cap, and a node already
// classified from an earlier walk only terminates its own walk.
func ParentCycles(rows []*Row) [][]string {
	parentOf := make(map[string]string, len(rows))
	for _, row := range rows {
		if id := row.String("id"); id != "" {
			parentOf[id] = RowParentID(row)
		}
	}
	seen := map[string]bool{} // member-key -> cycle already reported
	var cycles [][]string
	for _, row := range rows {
		start := row.String("id")
		if start == "" {
			continue
		}
		chain := []string{start}
		pos := map[string]int{start: 0}
		cur := parentOf[start]
		for cur != "" {
			if i, ok := pos[cur]; ok {
				cycle := append(append([]string{}, chain[i:]...), cur)
				key := cycleKey(cycle)
				if !seen[key] {
					seen[key] = true
					cycles = append(cycles, cycle)
				}
				break
			}
			if _, known := parentOf[cur]; !known {
				break // dangling parent: no cycle reachable from here
			}
			if len(chain) > len(rows) {
				break // depth cap: malformed board, never hang (amendment 4)
			}
			pos[cur] = len(chain)
			chain = append(chain, cur)
			cur = parentOf[cur]
		}
	}
	return cycles
}

// cycleKey dedupes a cycle across the rotations the per-row walks produce
// (A->B->A and B->A->B are the same cycle): the sorted member set.
func cycleKey(cycle []string) string {
	members := append([]string{}, cycle[:len(cycle)-1]...)
	sort.Strings(members)
	return strings.Join(members, "\x00")
}

// decompositionMetadataKeys are the BT-099 metadata keys living inside
// detail.metadata (panel amendment 1). They are boardctl-authored canonical
// metadata: exempt from the BT-098 "undeclared metadata key" warning (the
// row-canon _note precedent) and writable on untyped rows too — the container
// is sanctioned detail state, not a typed field.
var decompositionMetadataKeys = map[string]bool{
	"expected_files": true,
	"verify_steps":   true,
}

// mergeDecompositionMetadata sets expected_files / verify_steps (whichever
// are non-nil) inside the detail.metadata object of the given detail bytes,
// leaving every other metadata key and every other Extra key byte-verbatim.
// Returns the re-encoded detail in the row's own style.
func mergeDecompositionMetadata(detail json.RawMessage, expectedFiles, verifySteps []string, style Style) (json.RawMessage, error) {
	pd := ParseFindingDetail(detail)
	pd.Object = true
	if pd.Extra == nil {
		pd.Extra = map[string]json.RawMessage{}
	}
	meta := map[string]json.RawMessage{}
	if len(pd.Extra["metadata"]) > 0 {
		if err := json.Unmarshal(pd.Extra["metadata"], &meta); err != nil || meta == nil {
			meta = map[string]json.RawMessage{}
		}
	}
	setArr := func(key string, values []string) error {
		if values == nil {
			values = []string{}
		}
		enc, err := encodeValue(values, style)
		if err != nil {
			return fmt.Errorf("detail.metadata.%s: %w", key, err)
		}
		meta[key] = enc
		return nil
	}
	if expectedFiles != nil {
		if err := setArr("expected_files", expectedFiles); err != nil {
			return nil, err
		}
	}
	if verifySteps != nil {
		if err := setArr("verify_steps", verifySteps); err != nil {
			return nil, err
		}
	}
	enc, err := encodeValue(meta, style)
	if err != nil {
		return nil, fmt.Errorf("detail.metadata: %w", err)
	}
	pd.Extra["metadata"] = enc
	out, err := EncodeFindingDetail(pd, style)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(out), nil
}
