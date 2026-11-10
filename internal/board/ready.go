package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// UnsatisfiedDep identifies a dependency that is not complete. Status is the
// normalized read-side status, or "missing" when the id is dangling.
type UnsatisfiedDep struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// DanglingRef identifies a depends_on id that has no task row on this board.
type DanglingRef struct {
	ID  string `json:"id"`
	Dep string `json:"dep"`
}

// BlockedRow is an open row that cannot be dispatched now. Class is one of
// deps, dangling_dep, cycle, status, or deferred. Chain starts at Row and
// follows the first deterministic blocker path to its root.
type BlockedRow struct {
	Row         *Row             `json:"-"`
	Class       string           `json:"blocker_class"`
	Chain       []string         `json:"chain"`
	ChainText   string           `json:"chain_text"`
	Unsatisfied []UnsatisfiedDep `json:"unsatisfied"`
}

// GraphReport is the shared dispatchability view consumed by next, ready, and
// direction. Ready is the one definition of a row a foreman may dispatch.
type GraphReport struct {
	Rows     []*Row
	Ready    []*Row
	Blocked  []BlockedRow
	Cycles   [][]string
	Dangling []DanglingRef
	// WaveLevels contains the topological completion wave for rows that can
	// eventually be dispatched (complete rows are level zero). It is shared by
	// direction so that projection does not repeatedly rescan the graph.
	WaveLevels   map[string]int
	readyByID    map[string]struct{}
	statusByID   map[string]string
	deferredByID map[string]bool
}

// Dispatchable is the exported per-row predicate. Callers that already have a
// report should use its Ready set; this convenience function keeps the named
// predicate available without creating a second readiness definition.
func Dispatchable(row *Row, b *Board) bool {
	if row == nil || b == nil {
		return false
	}
	report, err := b.AnalyzeDispatchability()
	if err != nil {
		return false
	}
	return report.IsReady(row)
}

// IsReady reports whether row belongs to this report's dispatchable set.
func (r *GraphReport) IsReady(row *Row) bool {
	if r == nil || row == nil {
		return false
	}
	id := row.String("id")
	if r.readyByID != nil {
		_, ok := r.readyByID[id]
		return ok
	}
	for _, candidate := range r.Ready {
		if candidate == row || candidate.String("id") == id {
			return true
		}
	}
	return false
}

// AnalyzeDispatchability builds the board's dependency graph in O(rows+edges),
// detects cycles iteratively, and classifies every non-fixture open row.
func (b *Board) AnalyzeDispatchability() (*GraphReport, error) {
	rows, err := b.TaskRows()
	if err != nil {
		return nil, err
	}
	fixtures, err := b.FixtureIDs()
	if err != nil {
		return nil, err
	}
	return analyzeDispatchabilityRows(rows, fixtures)
}

// analyzeDispatchabilityRows contains the graph engine separate from board I/O;
// tests and consumers that already have parsed rows can measure graph scaling
// without charging JSONL parsing and fixture discovery to the graph predicate.
func analyzeDispatchabilityRows(rows []*Row, fixtures map[string]bool) (*GraphReport, error) {
	kept := make([]*Row, 0, len(rows))
	for _, row := range rows {
		if id := row.String("id"); id != "" && fixtures != nil && fixtures[id] {
			continue
		}
		kept = append(kept, row)
	}

	return analyzeDispatchabilityRowsCore(kept)
}

func analyzeDispatchabilityRowsCore(kept []*Row) (*GraphReport, error) {
	byID := make(map[string]*Row, len(kept))
	order := make(map[string]int, len(kept))
	deps := make(map[string][]string, len(kept))
	statusByID := make(map[string]string, len(kept))
	deferredByID := make(map[string]bool, len(kept))
	for i, row := range kept {
		id := row.String("id")
		if id == "" {
			return nil, fmt.Errorf("task row at file position %d has no id", i+1)
		}
		if _, exists := byID[id]; exists {
			return nil, fmt.Errorf("duplicate task id %q", id)
		}
		byID[id] = row
		order[id] = i
		statusByID[id] = NormalizeStatus(row.String("status"))
		deferredByID[id] = RowIsDeferred(row)
		values, err := dependencies(row)
		if err != nil {
			return nil, fmt.Errorf("task %s: %w", id, err)
		}
		deps[id] = values
	}

	cycles, cycleRows := detectCycles(kept, deps, order)
	dangling := make([]DanglingRef, 0)
	danglingByRow := make(map[string]map[string]bool)
	for _, row := range kept {
		id := row.String("id")
		for _, dep := range deps[id] {
			if _, ok := byID[dep]; ok {
				continue
			}
			dangling = append(dangling, DanglingRef{ID: id, Dep: dep})
			if danglingByRow[id] == nil {
				danglingByRow[id] = map[string]bool{}
			}
			danglingByRow[id][dep] = true
		}
	}

	report := &GraphReport{Rows: kept, Cycles: cycles, Dangling: dangling, WaveLevels: dispatchWaveLevels(kept, byID, deps, cycleRows, danglingByRow, statusByID, deferredByID), readyByID: make(map[string]struct{}, len(kept)), statusByID: statusByID, deferredByID: deferredByID}
	readySet := make(map[string]bool, len(kept))
	for _, row := range kept {
		id := row.String("id")
		if cycleRows[id] || statusByID[id] != "pending" || deferredByID[id] || len(danglingByRow[id]) > 0 {
			continue
		}
		ready := true
		for _, dep := range deps[id] {
			_, exists := byID[dep]
			if !exists || statusByID[dep] != "complete" {
				ready = false
				break
			}
		}
		if ready {
			readySet[id] = true
			report.readyByID[id] = struct{}{}
			report.Ready = append(report.Ready, row)
		}
	}
	chainMemo := make(map[string][]string, len(kept))
	for _, row := range kept {
		id := row.String("id")
		status := statusByID[id]
		if status == "complete" || status == "failed" {
			continue
		}
		if readySet[id] {
			continue
		}
		report.Blocked = append(report.Blocked, classifyBlocked(row, byID, deps, danglingByRow, cycleRows, cycles, order, chainMemo, statusByID, deferredByID))
	}
	return report, nil
}

func dependencies(row *Row) ([]string, error) {
	raw := row.Get("depends_on")
	if raw == nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err == nil {
		out := make([]string, 0, len(values))
		for _, value := range values {
			if value != "" {
				out = append(out, value)
			}
		}
		return out, nil
	}
	var legacy string
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, fmt.Errorf("depends_on is not an array or legacy string")
	}
	legacy = strings.TrimSpace(legacy)
	if legacy == "" {
		return []string{}, nil
	}
	var array []string
	if json.Unmarshal([]byte(legacy), &array) == nil {
		return array, nil
	}
	return []string{legacy}, nil
}

func detectCycles(rows []*Row, deps map[string][]string, order map[string]int) ([][]string, map[string]bool) {
	state := make(map[string]uint8, len(rows))
	cycles := make([][]string, 0)
	cycleRows := make(map[string]bool)
	seen := make(map[string]bool)
	for _, root := range rows {
		rootID := root.String("id")
		if state[rootID] != 0 {
			continue
		}
		stack := []string{rootID}
		positions := map[string]int{rootID: 0}
		state[rootID] = 1
		indices := []int{0}
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			idx := indices[len(indices)-1]
			if idx >= len(deps[current]) {
				state[current] = 2
				delete(positions, current)
				stack = stack[:len(stack)-1]
				indices = indices[:len(indices)-1]
				continue
			}
			next := deps[current][idx]
			indices[len(indices)-1]++
			if _, exists := order[next]; !exists {
				continue
			}
			switch state[next] {
			case 0:
				state[next] = 1
				positions[next] = len(stack)
				stack = append(stack, next)
				indices = append(indices, 0)
			case 1:
				start := positions[next]
				cycle := append([]string{}, stack[start:]...)
				cycle = append(cycle, next)
				key := strings.Join(cycle, "\x00")
				if !seen[key] {
					seen[key] = true
					cycles = append(cycles, cycle)
					for _, id := range cycle[:len(cycle)-1] {
						cycleRows[id] = true
					}
				}
			}
		}
	}
	return cycles, cycleRows
}

func classifyBlocked(row *Row, byID map[string]*Row, deps map[string][]string, dangling map[string]map[string]bool, cycleRows map[string]bool, cycles [][]string, order map[string]int, memo map[string][]string, statusByID map[string]string, deferredByID map[string]bool) BlockedRow {
	id := row.String("id")
	status := statusByID[id]
	out := BlockedRow{Row: row, Unsatisfied: []UnsatisfiedDep{}}
	if cycleRows[id] {
		out.Class = "cycle"
		for _, cycle := range cycles {
			for _, member := range cycle[:len(cycle)-1] {
				if member == id {
					out.Chain = append([]string{}, cycle...)
					out.ChainText = chainTextWithStatusMap(cycle, byID, statusByID, deferredByID)
					return out
				}
			}
		}
	}
	if deferredByID[id] && status == "pending" {
		out.Class = "deferred"
		out.Chain = []string{id}
		out.ChainText = id + " (pending, deferred)"
		return out
	}
	if status != "pending" {
		out.Class = "status"
		out.Chain = []string{id}
		out.ChainText = id + " (" + statusOrUnknown(status) + ")"
		return out
	}
	for _, dep := range deps[id] {
		if _, ok := byID[dep]; !ok {
			out.Unsatisfied = append(out.Unsatisfied, UnsatisfiedDep{ID: dep, Status: "missing"})
			continue
		}
		parentStatus := statusByID[dep]
		if parentStatus != "complete" {
			out.Unsatisfied = append(out.Unsatisfied, UnsatisfiedDep{ID: dep, Status: statusOrUnknown(parentStatus)})
		}
	}
	if len(dangling[id]) > 0 {
		out.Class = "dangling_dep"
	} else {
		out.Class = "deps"
	}
	// Memoize the deterministic first blocker path. Without this, a chain of
	// N blocked rows is walked from every row (and rendered as quadratic work).
	out.Chain = blockerChainMemo(id, byID, deps, order, memo, map[string]bool{}, statusByID)
	out.ChainText = chainTextWithStatusMap(out.Chain, byID, statusByID, deferredByID)
	return out
}

const maxBlockerChain = 32

func blockerChainMemo(id string, byID map[string]*Row, deps map[string][]string, order map[string]int, memo map[string][]string, visiting map[string]bool, statusByID map[string]string) []string {
	if path, ok := memo[id]; ok {
		return path
	}
	path := make([]string, 0, maxBlockerChain)
	current := id
	for len(path) < maxBlockerChain {
		if visiting[current] {
			path = append(path, current)
			break
		}
		if suffix, ok := memo[current]; ok && current != id {
			path = append(path, suffix...)
			break
		}
		path = append(path, current)
		visiting[current] = true
		next := ""
		for _, dep := range deps[current] {
			_, ok := byID[dep]
			if ok && statusByID[dep] == "complete" {
				continue
			}
			next = dep
			break
		}
		if next == "" {
			break
		}
		current = next
	}
	for key := range visiting {
		delete(visiting, key)
	}
	for i := range path {
		end := i + maxBlockerChain
		if end > len(path) {
			end = len(path)
		}
		if _, ok := memo[path[i]]; !ok {
			memo[path[i]] = append([]string(nil), path[i:end]...)
		}
	}
	return path
}

// dispatchWaveLevels computes topological completion levels once for the whole
// graph. Level zero is a completed row; pending rows start at level one.
func dispatchWaveLevels(rows []*Row, byID map[string]*Row, deps map[string][]string, cycleRows map[string]bool, dangling map[string]map[string]bool, statusByID map[string]string, deferredByID map[string]bool) map[string]int {
	levels := make(map[string]int, len(rows))
	remaining := make(map[string]int, len(rows))
	maxDep := make(map[string]int, len(rows))
	children := make(map[string][]string)
	queue := make([]string, 0, len(rows))
	for _, row := range rows {
		id := row.String("id")
		if statusByID[row.String("id")] == "complete" {
			levels[id] = 0
			queue = append(queue, id)
			continue
		}
		if statusByID[id] != "pending" || deferredByID[id] || cycleRows[id] || len(dangling[id]) > 0 {
			continue
		}
		for _, dep := range deps[id] {
			_, ok := byID[dep]
			if !ok || statusByID[dep] != "complete" {
				remaining[id]++
				children[dep] = append(children[dep], id)
				continue
			}
			if levels[dep] > maxDep[id] {
				maxDep[id] = levels[dep]
			}
		}
		if remaining[id] == 0 {
			levels[id] = maxDep[id] + 1
			queue = append(queue, id)
		}
	}
	for head := 0; head < len(queue); head++ {
		parent := queue[head]
		for _, child := range children[parent] {
			if levels[parent] > maxDep[child] {
				maxDep[child] = levels[parent]
			}
			remaining[child]--
			if remaining[child] == 0 {
				levels[child] = maxDep[child] + 1
				queue = append(queue, child)
			}
		}
	}
	return levels
}

func chainTextWithStatusMap(chain []string, byID map[string]*Row, statusByID map[string]string, deferredByID map[string]bool) string {
	parts := make([]string, 0, len(chain))
	for _, id := range chain {
		if _, ok := byID[id]; ok {
			status := statusByID[id]
			if deferredByID[id] {
				status += ", deferred"
			}
			parts = append(parts, id+" ("+statusOrUnknown(status)+")")
		} else {
			parts = append(parts, id+" (missing)")
		}
	}
	return strings.Join(parts, " <- ")
}

func statusOrUnknown(status string) string {
	if status == "" {
		return "unknown"
	}
	return status
}

// ReadyRows returns the rows selected by the shared graph predicate.
func (r *GraphReport) ReadyRows() []*Row { return r.Ready }

// SortRowsByPriority gives CLI consumers the stable P0..P5 ordering required by
// the wave surface. Ties retain file order because sort.SliceStable is used.
func SortRowsByPriority(rows []*Row) {
	sort.SliceStable(rows, func(i, j int) bool {
		pi, pj := priorityRank(rows[i].String("priority")), priorityRank(rows[j].String("priority"))
		return pi < pj
	})
}

func priorityRank(priority string) int {
	priority = strings.ToUpper(strings.TrimSpace(priority))
	if len(priority) == 2 && priority[0] == 'P' && priority[1] >= '0' && priority[1] <= '5' {
		return int(priority[1] - '0')
	}
	return 6
}
