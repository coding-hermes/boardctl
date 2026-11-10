boardctl documentation gap audit
Tick: boardctl-docs-2026-11-10-13-38-53
Target: /home/kara/coding-hermes-boardctl
Published revision: 13fb3708036972a3eabdf828beb152f8f8c3c823 (origin/main equal)
Audit mode: source/docs/board read-only; target README/docs were not edited.

Repository contract
Command: test -e docs/traceability-doctrine.md
Observed: exit 1; file absent.

Source surface
Command: nl -ba cmd/boardctl/main.go | sed -n '48,57p;97,123p;236,289p'
Observed:
48 commands:
54   ready   [-C dir] [--initiative SLUG] [--json] [--blocked] [--skip-bad-lines]
55   direction [-C dir] [--json] [--skip-bad-lines]
97   normalize-statuses [--apply] [--dry-run] [--json] [-C dir]
120  initiatives [--json] [--status S] [--skip-bad-lines]   list initiative rollups on this board
236 switch cmd {
251 case "ready":
253 case "direction":
263 case "normalize-statuses":
273 case "initiatives":

Command: nl -ba cmd/boardctl/graph_cmd.go | sed -n '91,102p;225,233p'
Observed:
91 func cmdReady(dir string, args []string) error {
94 asJSON := fs.Bool("json", false, "emit the graph report as JSON")
95 blocked := fs.Bool("blocked", false, "name every blocked row and its blocker chain")
96 initiative := fs.String("initiative", "", "filter to one initiative slug")
101 boardctl ready -C dir [--initiative SLUG] [--json] [--blocked] [--skip-bad-lines]
225 func cmdDirection(dir string, args []string) error {
228 asJSON := fs.Bool("json", false, "emit the direction report as JSON")
232 boardctl direction -C dir [--json] [--skip-bad-lines]

Command: nl -ba cmd/boardctl/statusnormalize.go | sed -n '9,22p'
Observed:
9 cmdNormalizeStatuses is deliberately a single-board command.
11 func cmdNormalizeStatuses(dir string, args []string) error {
14 apply := fs.Bool("apply", false, "atomically canonicalize known status aliases")
15 fs.Bool("dry-run", false, "report only and write nothing (default)")
16 asJSON := fs.Bool("json", false, "emit the per-board census as JSON")
20 boardctl normalize-statuses [--apply] [--dry-run] [--json] [-C dir]
21 One resolved repository-local board only; dry-run is the default and writes nothing.
22 Known read aliases are canonicalized on --apply. Unknown and missing statuses are refused, never guessed.

Fresh executable probes
Command: go build -o /home/kara/.hermes/stand-in/docs/boardctl/bin/boardctl ./cmd/boardctl
Observed: exit 0.
Command: /home/kara/.hermes/stand-in/docs/boardctl/bin/boardctl version
Observed: exit 0; boardctl version dev.
Command: /home/kara/.hermes/stand-in/docs/boardctl/bin/boardctl ready -C /home/kara/coding-hermes-boardctl
Observed: exit 0; ready now: 25 row(s) across 3 initiative group(s); blocked: 7 row(s).
Command: /home/kara/.hermes/stand-in/docs/boardctl/bin/boardctl direction -C /home/kara/coding-hermes-boardctl
Observed: exit 0; 2 open initiative(s); unscoped total 249, remaining 27, ready now 21.
Command: /home/kara/.hermes/stand-in/docs/boardctl/bin/boardctl normalize-statuses -C /home/kara/coding-hermes-boardctl --dry-run --json
Observed: exit 0; rows 255, alias_count 3, unknown_count 0, missing_count 0, changed 3, applied false.

Reader-facing documentation diff
Command: git ls-files '*.md' | grep -v '^\.gitreins/history/' | grep -v '^vendor/' | wc -l
Observed: 39 tracked markdown files in scope.
Command: grep -RIn --include='*.md' --exclude-dir=.gitreins --exclude-dir=.coding-hermes -F 'boardctl ready' README.md docs skills specs
Observed: matches only in specs/BT-097-work-directions.md and skills/boardctl-usage/SKILL.md; no README.md or docs/ reference hit.
Command: grep -RIn --include='*.md' --exclude-dir=.gitreins --exclude-dir=.coding-hermes -F 'boardctl direction' README.md docs skills specs
Observed: matches only in specs/BT-097-work-directions.md and skills/boardctl-usage/SKILL.md; no README.md or docs/ reference hit.
Command: grep -RIn --include='*.md' --exclude-dir=.gitreins --exclude-dir=.coding-hermes -F 'boardctl normalize-statuses' README.md docs skills specs
Observed: matches in docs/operations.md and skills/boardctl-usage/SKILL.md; no README.md hit.

Existing-row and board controls
Command: boardctl -C /home/kara/coding-hermes-boardctl validate
Observed: exit 0; RESULT: OK (44 warning(s)); 255 tasks, 661 events; baseline warnings are inherited legacy/schema drift.
Command: git -C /home/kara/coding-hermes-boardctl status --short
Observed before filing: empty; target board files were clean.
Command: git -C /home/kara/coding-hermes-boardctl rev-parse HEAD; git -C /home/kara/coding-hermes-boardctl rev-parse origin/main
Observed: both 13fb3708036972a3eabdf828beb152f8f8c3c823.
Command: jq -r 'select((.status // "") != "complete") | select(((.title // "")|tostring)|test("ready|direction|normalize-statuses|traceability-doctrine";"i") or ((.reasoning // "")|tostring)|test("ready|direction|normalize-statuses|traceability-doctrine";"i")) | [.id,.status,.priority,.title] | @tsv' .coding-hermes/board/tasks.jsonl
Observed: no pre-existing open row covering these exact surfaces.

Audience coverage
Developer: README Development section, skills/boardctl-usage/SKILL.md, Makefile and source tests provide build/test/extension homes.
Operator: README and docs/operations.md provide board recovery, install, hook, environment and runtime-operation homes; graph-dispatch front-door coverage is missing from README.
Integrator: README API table/OpenAPI parity and docs/muster/openapi.yaml provide the HTTP contract home; no live service was started in this audit.
End user: not applicable as a user-facing CLI/library; boardctl's audience is operators, foremen and integrators.

Findings to file after this artifact is frozen
1. Missing README front-door reference for ready and direction (P4).
2. Missing README front-door reference for normalize-statuses (P4; operations guide already covers it).
3. Missing repository-local docs/traceability-doctrine.md (P4 structure/contract documentation gap).

Unverified
No live long-running serve endpoint or external store was exercised. Destructive-command documentation scan was not run because the command gate would treat literal destructive tokens as execution-risk; it remains unverified. No release-asset or fresh-clone documentation reproduction was run. No worker fix or documentation change was performed.
