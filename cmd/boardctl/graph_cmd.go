package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/coding-hermes/boardctl/internal/board"
)

type readyRowJSON struct {
	ID        string   `json:"id"`
	Priority  string   `json:"priority"`
	Title     string   `json:"title"`
	DependsOn []string `json:"depends_on"`
}

type readyGroupJSON struct {
	Initiative string         `json:"initiative"`
	Count      int            `json:"count"`
	Rows       []readyRowJSON `json:"rows"`
}

type blockedJSON struct {
	ID           string                 `json:"id"`
	Priority     string                 `json:"priority"`
	Title        string                 `json:"title"`
	Initiative   string                 `json:"initiative"`
	BlockerClass string                 `json:"blocker_class"`
	Chain        []string               `json:"chain"`
	ChainText    string                 `json:"chain_text"`
	Unsatisfied  []board.UnsatisfiedDep `json:"unsatisfied"`
}

type graphStatsJSON struct {
	OpenRows     int `json:"open_rows"`
	ReadyRows    int `json:"ready_rows"`
	BlockedRows  int `json:"blocked_rows"`
	DeferredOpen int `json:"deferred_open"`
}

type readyJSON struct {
	Board            string              `json:"board"`
	Verb             string              `json:"verb"`
	InitiativeFilter string              `json:"initiative_filter"`
	TotalReady       int                 `json:"total_ready"`
	Groups           []readyGroupJSON    `json:"groups"`
	Blocked          []blockedJSON       `json:"blocked"`
	Cycles           [][]string          `json:"cycles"`
	Dangling         []board.DanglingRef `json:"dangling"`
	Stats            graphStatsJSON      `json:"stats"`
}

type directionInitiativeJSON struct {
	Initiative     string   `json:"initiative"`
	Total          int      `json:"total"`
	Complete       int      `json:"complete"`
	Failed         int      `json:"failed"`
	Deferred       int      `json:"deferred"`
	Remaining      int      `json:"remaining"`
	ReadyNow       int      `json:"ready_now"`
	Blocked        int      `json:"blocked"`
	ReadyIDs       []string `json:"ready_ids"`
	WaveProjection []int    `json:"wave_projection"`
}

type directionUnscopedJSON struct {
	Total          int   `json:"total"`
	Remaining      int   `json:"remaining"`
	ReadyNow       int   `json:"ready_now"`
	WaveProjection []int `json:"wave_projection"`
}

type directionJSON struct {
	Board       string                    `json:"board"`
	Verb        string                    `json:"verb"`
	Initiatives []directionInitiativeJSON `json:"initiatives"`
	Unscoped    directionUnscopedJSON     `json:"unscoped"`
	Cycles      [][]string                `json:"cycles"`
	Dangling    []board.DanglingRef       `json:"dangling"`
	Stats       directionStatsJSON        `json:"stats"`
}

type directionStatsJSON struct {
	OpenInitiatives int `json:"open_initiatives"`
	TotalReady      int `json:"total_ready"`
}

func cmdReady(dir string, args []string) error {
	fs := newFlagSet("ready")
	args = reorderArgs(args, valueFlags("C", "initiative"))
	asJSON := fs.Bool("json", false, "emit the graph report as JSON")
	blocked := fs.Bool("blocked", false, "name every blocked row and its blocker chain")
	initiative := fs.String("initiative", "", "filter to one initiative slug")
	skipBad := useSkipBad(fs)
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		_, _ = fmt.Fprint(fs.Output(), "boardctl ready -C dir [--initiative SLUG] [--json] [--blocked] [--skip-bad-lines]\n")
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("ready takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	armSkipBad(b, skipBad)
	report, err := b.AnalyzeDispatchability()
	if err != nil {
		return err
	}
	filter := strings.ToLower(strings.TrimSpace(*initiative))
	payload := buildReadyJSON(b, report, filter)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			return err
		}
	} else {
		printReady(b, payload, *blocked)
	}
	if b.HasSkipped() {
		return exitError(reportSkipped(b))
	}
	if len(report.Cycles) > 0 {
		return fmt.Errorf("dispatchability report contains %d dependency cycle(s)", len(report.Cycles))
	}
	return nil
}

func buildReadyJSON(b *board.Board, report *board.GraphReport, filter string) readyJSON {
	groups := map[string][]*board.Row{}
	for _, row := range report.Ready {
		slug := initiativeOf(row)
		if filter != "" && slug != filter {
			continue
		}
		groups[slug] = append(groups[slug], row)
	}
	groupList := make([]readyGroupJSON, 0, len(groups))
	for slug, rows := range groups {
		board.SortRowsByPriority(rows)
		out := readyGroupJSON{Initiative: slug, Count: len(rows), Rows: make([]readyRowJSON, 0, len(rows))}
		for _, row := range rows {
			out.Rows = append(out.Rows, readyRow(row))
		}
		groupList = append(groupList, out)
	}
	sort.Slice(groupList, func(i, j int) bool {
		if groupList[i].Count != groupList[j].Count {
			return groupList[i].Count > groupList[j].Count
		}
		return groupList[i].Initiative < groupList[j].Initiative
	})
	blocked := make([]blockedJSON, 0, len(report.Blocked))
	for _, item := range report.Blocked {
		if filter != "" && initiativeOf(item.Row) != filter {
			continue
		}
		blocked = append(blocked, blockedJSON{ID: item.Row.String("id"), Priority: item.Row.String("priority"), Title: item.Row.String("title"), Initiative: initiativeOf(item.Row), BlockerClass: item.Class, Chain: item.Chain, ChainText: item.ChainText, Unsatisfied: item.Unsatisfied})
	}
	open, deferred := 0, 0
	for _, row := range report.Rows {
		st := board.NormalizeStatus(row.String("status"))
		if st != "complete" && st != "failed" {
			open++
		}
		if board.RowIsDeferred(row) && st != "complete" && st != "failed" {
			deferred++
		}
	}
	return readyJSON{Board: b.Dir + "/tasks.jsonl", Verb: "ready", InitiativeFilter: filter, TotalReady: countGroupRows(groupList), Groups: groupList, Blocked: blocked, Cycles: nonNilCycles(report.Cycles), Dangling: nonNilDangling(report.Dangling), Stats: graphStatsJSON{OpenRows: open, ReadyRows: countGroupRows(groupList), BlockedRows: len(blocked), DeferredOpen: deferred}}
}

func printReady(b *board.Board, payload readyJSON, showBlocked bool) {
	fmt.Fprintf(os.Stdout, "board: %s\nready now: %d row(s) across %d initiative group(s)\n\n", payload.Board, payload.TotalReady, len(payload.Groups))
	for _, group := range payload.Groups {
		name := group.Initiative
		if name == "" {
			name = "(no initiative)"
		}
		fmt.Fprintf(os.Stdout, "  %s (%d ready — could absorb %d workers)\n", name, group.Count, group.Count)
		for _, row := range group.Rows {
			fmt.Fprintf(os.Stdout, "    %s  %s  %s\n", row.ID, row.Priority, row.Title)
		}
	}
	fmt.Fprintf(os.Stdout, "blocked: %d row(s)", len(payload.Blocked))
	if !showBlocked {
		fmt.Fprintln(os.Stdout, " — re-run with --blocked to name each blocker chain")
		return
	}
	fmt.Fprintln(os.Stdout, "")
	for _, item := range payload.Blocked {
		fmt.Fprintf(os.Stdout, "  %s  %s: %s\n", item.ID, item.BlockerClass, item.ChainText)
	}
}

func printDirection(payload directionJSON) {
	fmt.Fprintf(os.Stdout, "board: %s\n%d open initiative(s)\n\n", payload.Board, len(payload.Initiatives))
	for _, item := range payload.Initiatives {
		fmt.Fprintf(os.Stdout, "  %-16s total %d | complete %d | failed %d | deferred %d | remaining %d\n", item.Initiative, item.Total, item.Complete, item.Failed, item.Deferred, item.Remaining)
		fmt.Fprintf(os.Stdout, "                 ready now: %d (%s)\n", item.ReadyNow, strings.Join(item.ReadyIDs, ", "))
		fmt.Fprintf(os.Stdout, "                 blocked:   %d\n", item.Blocked)
		fmt.Fprintf(os.Stdout, "                 wave projection: %v\n\n", item.WaveProjection)
	}
	if payload.Unscoped.Total > 0 {
		fmt.Fprintf(os.Stdout, "  (no initiative)  total %d | remaining %d | ready now %d | wave projection: %v\n", payload.Unscoped.Total, payload.Unscoped.Remaining, payload.Unscoped.ReadyNow, payload.Unscoped.WaveProjection)
	}
}

func cmdDirection(dir string, args []string) error {
	fs := newFlagSet("direction")
	args = reorderArgs(args, valueFlags("C"))
	asJSON := fs.Bool("json", false, "emit the direction report as JSON")
	skipBad := useSkipBad(fs)
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() { _, _ = fmt.Fprint(fs.Output(), "boardctl direction -C dir [--json] [--skip-bad-lines]\n") }
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("direction takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	armSkipBad(b, skipBad)
	report, err := b.AnalyzeDispatchability()
	if err != nil {
		return err
	}
	payload := buildDirectionJSON(b, report)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			return err
		}
	} else {
		printDirection(payload)
	}
	if b.HasSkipped() {
		return exitError(reportSkipped(b))
	}
	if len(report.Cycles) > 0 {
		return fmt.Errorf("dispatchability report contains %d dependency cycle(s)", len(report.Cycles))
	}
	return nil
}

func buildDirectionJSON(b *board.Board, report *board.GraphReport) directionJSON {
	groups := map[string][]*board.Row{}
	for _, row := range report.Rows {
		groups[initiativeOf(row)] = append(groups[initiativeOf(row)], row)
	}
	readyBy := map[string][]string{}
	for _, row := range report.Ready {
		readyBy[initiativeOf(row)] = append(readyBy[initiativeOf(row)], row.String("id"))
	}
	blockedBy := map[string]int{}
	for _, item := range report.Blocked {
		blockedBy[initiativeOf(item.Row)]++
	}
	out := directionJSON{Board: b.Dir + "/tasks.jsonl", Verb: "direction", Initiatives: []directionInitiativeJSON{}, Cycles: nonNilCycles(report.Cycles), Dangling: nonNilDangling(report.Dangling), Unscoped: directionUnscopedJSON{WaveProjection: []int{}}, Stats: directionStatsJSON{TotalReady: len(report.Ready)}}
	for slug, rows := range groups {
		if slug == "" {
			out.Unscoped = directionUnscoped(rows, readyBy[slug])
			continue
		}
		item := directionInitiativeJSON{Initiative: slug, ReadyIDs: sortedStrings(readyBy[slug]), WaveProjection: waveProjection(rows)}
		for _, row := range rows {
			st := board.NormalizeStatus(row.String("status"))
			item.Total++
			if st == "complete" {
				item.Complete++
			} else if st == "failed" {
				item.Failed++
			} else {
				item.Remaining++
			}
			if board.RowIsDeferred(row) {
				item.Deferred++
			}
		}
		item.ReadyNow = len(readyBy[slug])
		item.Blocked = blockedBy[slug]
		if item.Total-item.Complete-item.Failed > 0 {
			out.Initiatives = append(out.Initiatives, item)
		}
	}
	sort.Slice(out.Initiatives, func(i, j int) bool { return out.Initiatives[i].Initiative < out.Initiatives[j].Initiative })
	out.Stats.OpenInitiatives = len(out.Initiatives)
	return out
}

func directionUnscoped(rows []*board.Row, ready []string) directionUnscopedJSON {
	out := directionUnscopedJSON{Total: len(rows), ReadyNow: len(ready), WaveProjection: waveProjection(rows)}
	for _, row := range rows {
		st := board.NormalizeStatus(row.String("status"))
		if st != "complete" && st != "failed" {
			out.Remaining++
		}
	}
	return out
}
func waveProjection(rows []*board.Row) []int {
	levels := map[string]int{}
	for _, row := range rows {
		if board.NormalizeStatus(row.String("status")) == "complete" {
			levels[row.String("id")] = 0
		}
	}
	changed := true
	for changed {
		changed = false
		for _, row := range rows {
			id := row.String("id")
			if levels[id] > 0 || board.NormalizeStatus(row.String("status")) != "pending" || board.RowIsDeferred(row) {
				continue
			}
			deps := boardDeps(row)
			max, ok := 0, true
			for _, dep := range deps {
				level, exists := levels[dep]
				if !exists {
					ok = false
					break
				}
				if level > max {
					max = level
				}
			}
			if ok {
				levels[id] = max + 1
				changed = true
			}
		}
	}
	counts := map[int]int{}
	max := 0
	for _, level := range levels {
		if level > max {
			max = level
		}
	}
	out := make([]int, max)
	for level, count := range counts {
		if level > 0 {
			out[level-1] = count
		}
	}
	for _, row := range rows {
		if level := levels[row.String("id")]; level > 0 {
			counts[level]++
		}
	}
	for level, count := range counts {
		if level > 0 {
			out[level-1] = count
		}
	}
	return out
}
func boardDeps(row *board.Row) []string {
	raw := row.Get("depends_on")
	var deps []string
	_ = json.Unmarshal(raw, &deps)
	return deps
}
func initiativeOf(row *board.Row) string {
	return strings.ToLower(strings.TrimSpace(row.String("initiative")))
}
func readyRow(row *board.Row) readyRowJSON {
	deps := boardDeps(row)
	if deps == nil {
		deps = []string{}
	}
	return readyRowJSON{ID: row.String("id"), Priority: row.String("priority"), Title: row.String("title"), DependsOn: deps}
}
func countGroupRows(groups []readyGroupJSON) int {
	n := 0
	for _, group := range groups {
		n += group.Count
	}
	return n
}
func sortedStrings(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	if out == nil {
		return []string{}
	}
	return out
}
func nonNilCycles(values [][]string) [][]string {
	if values == nil {
		return [][]string{}
	}
	return values
}
func nonNilDangling(values []board.DanglingRef) []board.DanglingRef {
	if values == nil {
		return []board.DanglingRef{}
	}
	return values
}
