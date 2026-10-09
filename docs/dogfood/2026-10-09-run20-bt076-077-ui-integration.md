# Dogfood Run 20 — 2026-10-09/10 — BT-076/077 features + BT-075 web UI + bunker install

Verdict: SHIPPABLE core (deferred, plural associations, gendocs, serve/UI all work
as documented), PROMISING-BUT-ROUGH at the release boundary — the README teaches
flags the Current-release binary does not have, and the repo's own board carries
created_at corruption that validate passes silently but the report parser flags.

## Angle (runs 1-19 never touched)

- BT-076 `--deferred` (merged 10-09, unreleased)
- BT-077 plural association fields `--pull-request/--assoc-branch/--assoc-worktree`
  (merged 10-09, unreleased, README never documents them)
- BT-079 `cmd/gendocs` OpenAPI regeneration
- BT-075 `serve /ui` web UI in a real browser (run 19 only curl'd serve)
- zip upload path in the browser (DF-BOARDCTL-6's "unverifiable" surface, 09-20)

## How the run used it (real consumer)

1. Built HEAD from checkout (`go build ./cmd/boardctl`) — the path a HEAD user takes.
2. Fresh board in /tmp scratch: `init` → create deferred/non-deferred → stats →
   un-defer → stats. All BT-076 semantics held: `actionable` excludes deferred,
   `deferred` is its own line + JSON key, non-boolean flag value is a usage error
   with nothing written, hand-edited `"deferred":"parked"` produces the documented
   validate WARN (treated as not deferred), omitted flag never creates the key.
3. BT-077: merged PR numbers 42+43, branches, worktrees across separate invocations;
   dup re-add silently keeps one; singular worktree/branch and other dimensions
   survive each merge. Friction: one-dimension-per-invocation is undocumented.
4. BT-079: `go run ./cmd/gendocs` regenerates `docs/muster/openapi.yaml` (6 routes)
   with an empty git diff at HEAD — parity gate honored.
5. Browser drove `serve /ui` at HEAD: board list → card drill-in (charts render real
   series: burndown 3 pts, burn-up, velocity W41) → status filters, fixtures toggle,
   expand-all, validation WARN panel, streak-honesty notes. One 4xx on the whole
   surface: favicon.ico.
6. Zip upload through the real file input (CDP setFileInputFiles) with the repo's
   own 144-task board: both boards render side by side, counts cross-check against
   ground truth (141 complete + 2 blocked open of 143 non-fixture tasks; fixture
   excluded; 99%). 7 parse warnings read from the notes panel.

## Errors hit and fixes

- `serve --port` is not a flag (`--addr` is); first "success" on :8777 was actually
  a FOREIGN process on that port — verified via serve.log + ss before trusting.
- `init --ns` is not a flag on the released binary (`--namespace`).
- create id `S-1` refused by the id format guard (working as designed; used SMOKE-1).
- My own findings initially used status "todo" — boardctl validate flagged it as a
  read alias; normalized to "pending". The self-hosting validator caught the dogfooder.

## Install leg (bunker-las-02; las-03 offline)

Fresh Debian 13 agent 6416dcda (destroyed, key removed):
- README quickstart verbatim: v0.1.10 download + SHA256SUMS verify OK, `version`
  prints v0.1.10, clone of HEAD (05a41e3) succeeded over https, validate/stats/doctor
  on the repo board OK (4 warnings). INSTALL_SMOKE_SECONDS=2.
- BUT: `--deferred` / `--pull-request` both "flag provided but not defined" on the
  v0.1.10 binary → DF-BOARDCTL-26.
- Shipped a locally built HEAD binary to the same agent: deferred + plural-assoc
  smoke green (create --deferred → stats deferred:1; update --pull-request 7 +
  --assoc-branch wt/x → show shows both; un-defer → deferred:0).

## Performance (Step 2b) — nothing a user feels; NO PERF rows filed

- validate 12.9ms ± 0.8 (hyperfine 20 runs, warm, 148-task board); stats 7.5ms;
  list --json 11.2ms; cold first-runs 0.01s.
- render 0.03s / 626KB (144-task board); small board 0.00s.
- serve upload render + page load: loadEventEnd 86ms, 0 longtasks.
- Install leg: 2s to verified working binary.

## What a new user needs that isn't documented

- The one-dimension-per-invocation rule for association flags (error-only).
- That README feature sections can describe unreleased CLI behavior (release pin
  is v0.1.10 while HEAD docs teach BT-076/077 flags).

## Verdict basis

Value: real (fleet boards managed byte-stably, database-free; the report surface
now honest about parse warnings and streak gaming). Usability: friction 3
(docs-ahead-of-release, undocumented assoc semantics, favicon nit). Trust: report
numbers cross-checked against independent ground truth; read-only claim held
(board files byte-identical after render/serve/upload — checked via git status).
