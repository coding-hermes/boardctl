package board

// BT-078: umbrella-repo board discovery — a read-only unified view over the
// symlinked child boards registered under an umbrella repo's
// .coding-hermes/links/ directory.
//
// Registry convention (documented in README and the boardctl-usage skill):
//
//	<umbrella>/.coding-hermes/links/<repo> -> <child>/.coding-hermes
//
// One symlink per child repo, NAMED after the child (the name is the repo
// qualifier used in every provenance-bearing result and in the repo-qualified
// task identity <repo>/<id>); the target is the child's .coding-hermes
// directory. A link aimed at the child repo root, or straight at its board
// dir, resolves to the same board — but the documented convention is the
// .coding-hermes target.
//
// SAFETY CONTRACT (BT-078): this file is READ-ONLY. It exposes no write
// method; every read reuses the same Resolve-class read paths the single-repo
// commands use (boardFromDir -> TaskRows/ListTasks/ComputeStats/Validate), so
// an aggregated view can never write through a child symlink. Writes stay
// single-repo: the ordinary verbs with -C aimed at ONE owning repo.
//
// Link validation is fail-loud, bounded, and never guesses:
//   - every link target is canonicalized (every path component resolved) and
//     must be a directory hosting a board (the tasks.jsonl+events.jsonl pair);
//   - symlink chains are resolved with a hard hop cap
//     (MaxUmbrellaSymlinkDepth) and a visited set — a loop or an over-deep
//     chain is a named finding, never a hang or an unbounded follow;
//   - a target outside any determinable git repo boundary (no .git ancestor,
//     the same rule findRepoRoot uses for doctor) is refused with a finding
//     rather than followed;
//   - two links to the same target deduplicate to ONE board, with a warning
//     naming the alias links;
//   - duplicate task ids across boards keep repo-qualified identity — rows
//     are never merged — and cross-board depends_on references resolve only
//     when unambiguous (DepResolutions): an ambiguous id is reported, never
//     bound to a guessed task.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MaxUmbrellaSymlinkDepth is the hard cap on symlink hops followed while
// canonicalizing one registry link. A chain that needs more hops — and any
// chain that revisits a path (a loop) — stops with a named finding instead of
// being followed. 8 comfortably covers the legitimate shapes (links dir ->
// indirection -> child checkout); arbitrary deep chains are exactly the
// "recursively follow symlinks" hazard BT-078 forbids.
const MaxUmbrellaSymlinkDepth = 8

// UmbrellaFinding is one named problem in the umbrella view. Level is
// "error" (a board/link the view could not cover — the umbrella command
// exits 1) or "warn" (reported, view still complete; exit 1 only under
// --strict). Link names the registry entry responsible, or "" when the
// finding is about the umbrella root itself.
type UmbrellaFinding struct {
	Level string `json:"level"`
	Link  string `json:"link,omitempty"`
	Msg   string `json:"msg"`
}

// IsError reports whether the finding fails the umbrella command (exit 1).
func (f UmbrellaFinding) IsError() bool { return f.Level == "error" }

// QualifiedBoard is one board in the unified view with its provenance.
// Repo is the repo qualifier (the registry link name; the umbrella's own
// board uses its repo-root basename). Board is the SAME read-path Board value
// single-repo commands use — the umbrella surface exposes it for reads only.
type QualifiedBoard struct {
	Repo     string `json:"repo"`
	RepoRoot string `json:"repo_root,omitempty"`
	BoardDir string `json:"board_dir"`
	Topology string `json:"topology"`
	Board    *Board `json:"-"`
}

// QualifiedTask is one task row with its source provenance. Identity across
// the view is QualifiedTaskID(Repo, Row id) — "<repo>/<id>" — so colliding
// raw ids across child boards stay distinguishable and are never merged.
type QualifiedTask struct {
	Repo  string
	Board QualifiedBoard
	Row   *Row
}

// QualifiedTaskID renders the repo-qualified identity of a task: the ONLY
// identity the umbrella view promises. Raw ids are unique per repo, not
// across the view.
func QualifiedTaskID(repo, id string) string { return repo + "/" + id }

// Umbrella is the discovered umbrella workspace: the boards in view plus
// every named finding from discovery. OpenUmbrella builds it; the read
// methods (ListTasks/Stats/DepResolutions) consume it read-only.
type Umbrella struct {
	Root     string            // absolute -C target (repo root or .coding-hermes dir)
	LinksDir string            // .coding-hermes/links ("" when the registry is absent)
	Boards   []QualifiedBoard  // the umbrella's own board first (when present), then children sorted by qualifier
	Findings []UmbrellaFinding // discovery findings, in deterministic order
}

// addFinding appends one finding (printf-style).
func (u *Umbrella) addFinding(level, link, format string, args ...any) {
	u.Findings = append(u.Findings, UmbrellaFinding{Level: level, Link: link, Msg: fmt.Sprintf(format, args...)})
}

// HasErrors reports whether any error-class finding exists (those boards are
// missing from the view — degraded output must not read as clean success).
func (u *Umbrella) HasErrors() bool {
	for _, f := range u.Findings {
		if f.IsError() {
			return true
		}
	}
	return false
}

// WarnCount returns the number of warning-level findings.
func (u *Umbrella) WarnCount() int {
	n := 0
	for _, f := range u.Findings {
		if !f.IsError() {
			n++
		}
	}
	return n
}

// RenderFindings renders the itemized findings block (deterministic order).
func (u *Umbrella) RenderFindings() string {
	var sb strings.Builder
	for _, f := range u.Findings {
		if f.Link != "" {
			fmt.Fprintf(&sb, "[%s] link %s: %s\n", f.Level, f.Link, f.Msg)
		} else {
			fmt.Fprintf(&sb, "[%s] %s\n", f.Level, f.Msg)
		}
	}
	return sb.String()
}

// appendQualified adds one resolved board to the view (read-path Board only).
func (u *Umbrella) appendQualified(repo string, b *Board) {
	u.Boards = append(u.Boards, QualifiedBoard{
		Repo:     repo,
		RepoRoot: findRepoRoot(b.Dir),
		BoardDir: b.Dir,
		Topology: b.Topology,
		Board:    b,
	})
}

// OpenUmbrella discovers the umbrella workspace at target (-C semantics: a
// repo root or its .coding-hermes dir), validates every registry link, and
// returns the boards in view plus named findings. It returns a wrapped
// ErrBoardNotFound when the target is no directory at all, or holds neither a
// board nor a links registry (the ordinary "not a board here" usage error).
//
// Discovery itself never fails on a bad LINK: broken, looping, over-deep,
// non-board, and out-of-repo links become findings and the rest of the view
// still loads (degrade with evidence, the DF-BOARDCTL-9 house rule).
func OpenUmbrella(target string) (*Umbrella, error) {
	if target == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		target = wd
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	if fi, statErr := os.Stat(abs); statErr != nil || !fi.IsDir() {
		return nil, fmt.Errorf("%w: %s (umbrella -C expects a directory: a repo root or its .coding-hermes dir)", ErrBoardNotFound, target)
	}
	ch := filepath.Join(abs, umbrellaCHDirName)
	if filepath.Base(abs) == umbrellaCHDirName {
		ch = abs
	}
	u := &Umbrella{Root: abs}
	linksDir := filepath.Join(ch, umbrellaLinksDirName)
	if st, statErr := os.Stat(linksDir); statErr == nil && st.IsDir() {
		u.LinksDir = linksDir
	}

	// The umbrella's own board is OPTIONAL (an umbrella may be nothing but
	// a links registry). A partial board (one half of the pair) surfaces as
	// a warning, never silently disappears.
	own, partial := resolveOptionalBoard(abs)
	if own != nil {
		u.appendQualified(umbrellaSelfQualifier(abs), own)
	} else if partial != "" {
		u.addFinding("warn", "", "umbrella root holds a partial board: %s", partial)
	}

	if u.LinksDir == "" {
		if len(u.Boards) == 0 {
			return nil, fmt.Errorf("%w: %s holds no board and no %s registry (an umbrella repo links child boards under .coding-hermes/links/)",
				ErrBoardNotFound, target, filepath.Join(umbrellaCHDirName, umbrellaLinksDirName))
		}
		u.addFinding("warn", "", "no .coding-hermes/links registry — the view covers only this repo's own board")
		return u, nil
	}
	u.loadLinks()
	if len(u.Boards) == 0 && !u.HasErrors() {
		u.addFinding("error", "", "links registry holds no resolvable board and the umbrella has no own board — the view is empty")
	}
	return u, nil
}

// umbrellaCHDirName / umbrellaLinksDirName are the registry convention paths.
const (
	umbrellaCHDirName    = ".coding-hermes"
	umbrellaLinksDirName = "links"
)

// umbrellaSelfQualifier derives the repo qualifier for the umbrella's OWN
// board: its git repo root basename, or the target dir basename when no repo
// boundary exists (the same fallback findRepoRoot implies).
func umbrellaSelfQualifier(abs string) string {
	if root := findRepoRoot(abs); root != "" {
		return filepath.Base(root)
	}
	return filepath.Base(abs)
}

// resolveOptionalBoard probes dir with the SAME candidate order Resolve uses
// (itself, .coding-hermes, .coding-hermes/board — or <dir>/board for a
// .coding-hermes target) but treats "nothing here" as (nil, "") instead of an
// error. A partial board (exactly one of the pair) returns the BT-026-style
// description so the caller can surface it as a named finding. The Board
// construction is shared with Resolve (boardFromDir), so the two read paths
// cannot drift.
func resolveOptionalBoard(dir string) (*Board, string) {
	for _, c := range boardDirCandidates(dir) {
		tasks := filepath.Join(c, "tasks.jsonl")
		events := filepath.Join(c, "events.jsonl")
		if fileExists(tasks) && fileExists(events) {
			return boardFromDir(c), ""
		}
	}
	// Mirror Resolve's BT-026 partial detection (first partial candidate).
	for _, c := range boardDirCandidates(dir) {
		tasks := filepath.Join(c, "tasks.jsonl")
		events := filepath.Join(c, "events.jsonl")
		if fileExists(tasks) {
			return nil, fmt.Sprintf("%s (a tasks.jsonl-only board: events.jsonl is missing)", tasks)
		}
		if fileExists(events) {
			return nil, fmt.Sprintf("%s (an events.jsonl-only board: tasks.jsonl is missing)", events)
		}
	}
	return nil, ""
}

// resolveLinkBounded resolves the symlink chain starting at linkPath (whose
// first-hop raw target is raw) with a hard hop cap and a visited set.
// Returns the fully-resolved final path; loop is the revisited path when the
// chain is a cycle; err names a missing component (broken link) or the
// over-deep chain refusal. It NEVER follows unboundedly: the cap bounds every
// chain regardless of shape.
func resolveLinkBounded(linkPath, raw string) (final string, loop string, err error) {
	seen := map[string]bool{filepath.Clean(linkPath): true}
	cur := filepath.Clean(raw)
	if !filepath.IsAbs(raw) {
		cur = filepath.Clean(filepath.Join(filepath.Dir(linkPath), raw))
	}
	for hop := 0; hop <= MaxUmbrellaSymlinkDepth; hop++ {
		if seen[cur] {
			return "", cur, nil
		}
		seen[cur] = true
		st, statErr := os.Lstat(cur)
		if statErr != nil {
			return "", "", statErr
		}
		if st.Mode()&fs.ModeSymlink == 0 {
			return cur, "", nil
		}
		next, readErr := os.Readlink(cur)
		if readErr != nil {
			return "", "", readErr
		}
		if !filepath.IsAbs(next) {
			next = filepath.Join(filepath.Dir(cur), next)
		}
		cur = filepath.Clean(next)
	}
	return "", "", fmt.Errorf("symlink chain deeper than %d hops — refused (bound traversal)", MaxUmbrellaSymlinkDepth)
}

// loadLinks reads the registry and resolves every symlink entry. Entries are
// processed in directory order (os.ReadDir sorts by name), so the view and
// its findings are deterministic.
func (u *Umbrella) loadLinks() {
	entries, err := os.ReadDir(u.LinksDir)
	if err != nil {
		u.addFinding("error", "", "links registry unreadable: %v", err)
		return
	}
	type linkEntry struct {
		name    string
		linkPth string
		raw     string
		canon   string
		aliases []string
	}
	var resolvedLinks []linkEntry
	byCanon := map[string]int{} // canonical target dir -> index in resolvedLinks
	for _, e := range entries {
		name := e.Name()
		linkPath := filepath.Join(u.LinksDir, name)
		if e.Type()&fs.ModeSymlink == 0 {
			u.addFinding("warn", name, "registry entry is not a symlink — skipped (the registry holds symlinks to each child repo's .coding-hermes dir)")
			continue
		}
		raw, readErr := os.Readlink(linkPath)
		if readErr != nil {
			u.addFinding("error", name, "readlink failed: %v", readErr)
			continue
		}
		final, loop, resErr := resolveLinkBounded(linkPath, raw)
		if loop != "" {
			u.addFinding("error", name, "symlink loop detected (chain revisits %s) — not followed", loop)
			continue
		}
		if resErr != nil {
			u.addFinding("error", name, "broken symlink: %v", resErr)
			continue
		}
		// Canonicalize (resolve any symlink left in parent components) —
		// the dedupe key and every reported path is this canonical form.
		canon, evalErr := filepath.EvalSymlinks(final)
		if evalErr != nil {
			u.addFinding("error", name, "unresolvable symlink target: %v", evalErr)
			continue
		}
		if fi, statErr := os.Stat(canon); statErr != nil || !fi.IsDir() {
			u.addFinding("error", name, "symlink target %s is not a directory", canon)
			continue
		}
		if idx, dup := byCanon[canon]; dup {
			resolvedLinks[idx].aliases = append(resolvedLinks[idx].aliases, name)
			continue
		}
		byCanon[canon] = len(resolvedLinks)
		resolvedLinks = append(resolvedLinks, linkEntry{name: name, linkPth: linkPath, raw: raw, canon: canon})
	}
	for _, rl := range resolvedLinks {
		if len(rl.aliases) > 0 {
			u.addFinding("warn", rl.name, "link(s) %s resolve to the same target %s — deduplicated to one board",
				strings.Join(rl.aliases, ", "), rl.canon)
		}
		// Repo-boundary gate: the target must sit inside a determinable git
		// repo. No .git ancestor means no boundary we can name — refuse
		// rather than follow (BT-078 safety rule).
		if findRepoRoot(rl.canon) == "" {
			u.addFinding("error", rl.name, "link target %s is not inside a git repository — refused (outside any determinable repo boundary)", rl.canon)
			continue
		}
		child, partial := resolveOptionalBoard(rl.canon)
		if child == nil {
			if partial != "" {
				u.addFinding("error", rl.name, "link target %s holds a partial board: %s", rl.canon, partial)
			} else {
				u.addFinding("error", rl.name, "link target %s holds no board (no tasks.jsonl+events.jsonl pair under it)", rl.canon)
			}
			continue
		}
		u.appendQualified(rl.name, child)
	}
}

// ListTasks returns every task row in the view matching the filter, in board
// order (umbrella's own board first, then children by qualifier), each row
// carrying its repo provenance. Board-order means colliding raw ids stay
// distinguishable by QualifiedTaskID — the view never merges rows.
func (u *Umbrella) ListTasks(f TaskFilter) ([]QualifiedTask, error) {
	var out []QualifiedTask
	for _, qb := range u.Boards {
		rows, err := qb.Board.ListTasks(f)
		if err != nil {
			return nil, fmt.Errorf("board %s (%s): %w", qb.Repo, qb.BoardDir, err)
		}
		for _, r := range rows {
			out = append(out, QualifiedTask{Repo: qb.Repo, Board: qb, Row: r})
		}
	}
	return out, nil
}

// RepoStats is one board's stats with provenance.
type RepoStats struct {
	Repo     string `json:"repo"`
	BoardDir string `json:"board_dir"`
	Stats    *Stats `json:"stats"`
}

// UmbrellaStats aggregates per-repo stats plus a grand total.
type UmbrellaStats struct {
	Root  string      `json:"root"`
	Repos []RepoStats `json:"repos"`
	Total int         `json:"total"`
	// BT-076: the grand-total twin of Stats.Deferred — the sum of the
	// per-repo counts, excluded from the actionable reading the same way.
	Deferred   int            `json:"deferred"`
	Actionable int            `json:"actionable"`
	Status     map[string]int `json:"status"`
	Priority   map[string]int `json:"priority"`
}

// Stats computes per-repo status/priority tallies (the same ComputeStats the
// single-repo `stats` command uses) plus grand totals over the whole view.
func (u *Umbrella) Stats(f TaskFilter) (*UmbrellaStats, error) {
	us := &UmbrellaStats{Root: u.Root, Status: map[string]int{}, Priority: map[string]int{}}
	for _, qb := range u.Boards {
		st, err := qb.Board.ComputeStats(f)
		if err != nil {
			return nil, fmt.Errorf("board %s (%s): %w", qb.Repo, qb.BoardDir, err)
		}
		us.Repos = append(us.Repos, RepoStats{Repo: qb.Repo, BoardDir: qb.BoardDir, Stats: st})
		us.Total += st.Total
		us.Deferred += st.Deferred
		us.Actionable += st.Actionable
		for k, v := range st.Status {
			us.Status[k] += v
		}
		for k, v := range st.Priority {
			us.Priority[k] += v
		}
	}
	return us, nil
}

// ValidateAll runs the per-board Validate on every board in the view (read
// only — Validate never writes; --repair is a separate single-board command).
func (u *Umbrella) ValidateAll() ([]QualifiedReport, error) {
	var out []QualifiedReport
	for _, qb := range u.Boards {
		rep, err := qb.Board.Validate()
		if err != nil {
			return nil, fmt.Errorf("board %s (%s): %w", qb.Repo, qb.BoardDir, err)
		}
		out = append(out, QualifiedReport{Repo: qb.Repo, BoardDir: qb.BoardDir, Report: rep})
	}
	return out, nil
}

// QualifiedReport is one board's validate report with provenance.
type QualifiedReport struct {
	Repo     string  `json:"repo"`
	BoardDir string  `json:"board_dir"`
	Report   *Report `json:"report"`
}

// DepResolution is the outcome of resolving ONE depends_on reference found in
// the unified view. Status is one of:
//
//	local     the id exists in the referencing task's own repo — resolved
//	           there (dependencies are repo-local by convention)
//	cross     the id exists in exactly one OTHER repo — resolved, qualified
//	ambiguous the id exists in several other repos and NOT locally — NOT
//	           resolved; Candidates names every qualifier that holds it
//	dangling  the id exists nowhere in the view
type DepResolution struct {
	Repo         string   `json:"repo"`       // referencing task's repo qualifier
	TaskID       string   `json:"task_id"`    // referencing task's raw id
	Dep          string   `json:"depends_on"` // the referenced id, as written
	Status       string   `json:"status"`
	ResolvedRepo string   `json:"resolved_repo,omitempty"` // local/cross only
	Candidates   []string `json:"candidates,omitempty"`    // ambiguous only (sorted)
	AlsoIn       []string `json:"also_in,omitempty"`       // local only: other repos also holding the id (sorted)
}

// DepResolutions classifies every depends_on reference in the view against
// the repo-qualified id set. It returns the per-reference outcomes plus the
// derived findings: dangling refs warn (the single-board validate wording),
// ambiguous refs warn with their candidate list and are NEVER bound to a
// guessed task, and a repo-local resolution whose id ALSO exists in other
// boards warns so the binding is never silent. Reads only — no write path is
// touched.
func (u *Umbrella) DepResolutions() ([]DepResolution, []UmbrellaFinding, error) {
	// id -> sorted repo qualifiers holding it (task rows only).
	holders := map[string][]string{}
	type depRefRow struct {
		repo, taskID string
		deps         []string
	}
	var refs []depRefRow
	for _, qb := range u.Boards {
		rows, err := qb.Board.TaskRows()
		if err != nil {
			return nil, nil, fmt.Errorf("board %s (%s): %w", qb.Repo, qb.BoardDir, err)
		}
		for _, r := range rows {
			id := r.String("id")
			if id == "" {
				continue
			}
			holders[id] = append(holders[id], qb.Repo)
			if deps := rowStringSlice(r, "depends_on"); len(deps) > 0 {
				refs = append(refs, depRefRow{repo: qb.Repo, taskID: id, deps: deps})
			}
		}
	}
	for id := range holders {
		sort.Strings(holders[id])
	}
	var out []DepResolution
	var findings []UmbrellaFinding
	for _, rr := range refs {
		for _, dep := range rr.deps {
			in := holders[dep]
			d := DepResolution{Repo: rr.repo, TaskID: rr.taskID, Dep: dep}
			switch {
			case len(in) == 0:
				d.Status = "dangling"
				findings = append(findings, UmbrellaFinding{
					Level: "warn",
					Msg:   fmt.Sprintf("task %s: depends_on %q not found in any board in the view", QualifiedTaskID(rr.repo, rr.taskID), dep),
				})
			case sliceContains(in, rr.repo):
				d.Status = "local"
				d.ResolvedRepo = rr.repo
				for _, q := range in {
					if q != rr.repo {
						d.AlsoIn = append(d.AlsoIn, q)
					}
				}
				if len(d.AlsoIn) > 0 {
					findings = append(findings, UmbrellaFinding{
						Level: "warn",
						Msg: fmt.Sprintf("task %s: depends_on %q resolves repo-locally (%s) but the id also exists in %s — qualify if another board was meant",
							QualifiedTaskID(rr.repo, rr.taskID), dep, QualifiedTaskID(rr.repo, dep), strings.Join(d.AlsoIn, ", ")),
					})
				}
			case len(in) == 1:
				d.Status = "cross"
				d.ResolvedRepo = in[0]
			default:
				d.Status = "ambiguous"
				d.Candidates = append([]string(nil), in...)
				// Ambiguity is an ERROR, not a warning: the view cannot say
				// which task the relation binds to, so the aggregated view
				// is degraded until a human qualifies the reference (exit 1
				// makes every scripted consumer notice — a silent guess is
				// exactly what BT-078 forbids). The message names every
				// candidate as its repo-qualified id.
				quals := make([]string, 0, len(in))
				for _, q := range in {
					quals = append(quals, QualifiedTaskID(q, dep))
				}
				findings = append(findings, UmbrellaFinding{
					Level: "error",
					Msg: fmt.Sprintf("task %s: depends_on %q is ambiguous — candidates: %s; NOT resolved (qualify the reference or make the ids unique)",
						QualifiedTaskID(rr.repo, rr.taskID), dep, strings.Join(quals, ", ")),
				})
			}
			out = append(out, d)
		}
	}
	return out, findings, nil
}

// sliceContains reports whether sorted-or-unsorted strs contains v.
func sliceContains(strs []string, v string) bool {
	for _, s := range strs {
		if s == v {
			return true
		}
	}
	return false
}
