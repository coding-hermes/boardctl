README HEALTH SWEEP — boardctl
==============================
Lane:        boardctl-readme (skill coding-hermes-readme v1.0.5)
Tick:        boardctl-readme-2026-11-08-04-02-17
Sweep start: 2026-11-08T04:08:07Z   Sweep end (artifact frozen): 2026-11-08T04:26Z
Target repo: <repo>  (git rev-parse --show-toplevel = itself; remote git@github.com:coding-hermes/boardctl.git)
Published tip at sweep start: 1cda136f85e753ea1de006ca79e3966ba9db5034 (git ls-remote --symref origin HEAD -> refs/heads/main)
Published tip re-read at 04:23:28Z: 1cda136f85e753ea1de006ca79e3966ba9db5034 (unchanged; ge/README byte-identical -> both SHAs
are the same tree, so every probe below is a published-tip probe at 1cda136f).
Latest release: v0.2.0 (gh release list: v0.2.0 Latest 2026-10-30T20:57:48Z). README last moved 2026-11-06 (commit cbd817a).

BINARIES EXERCISED
  published asset  <probe>/boardctl_linux_amd64   -> boardctl version v0.2.0
                   sha256 489b93bac0a1a46e2438e28523f32662a80e8e7d9217511b7b433ac7f435c038  (verified against the
                   release's own SHA256SUMS: "boardctl_linux_amd64: OK"), vcs.revision=2d3f2e6c63d2...
  from-HEAD build  <probe>/boardctl_head_build    -> boardctl version dev (go build ./cmd/boardctl at 1cda136f)

SCOPE / METHOD
  * The Install block, the whole Usage reference, all fenced command blocks, the Development gate block and the
    Deployment-drift probe were EXECUTED, not read. 117 documented command lines were extracted mechanically
    (extract-cmds.py) and run against scratch boards in <probe>/matrixA..F.
  * Scratch boards only. No target board, README or doc file was edited by this lane.
  * 8787 was occupied by a foreign process (python3 pid 7454); serve was probed on the documented alternate
    --addr 127.0.0.1:9000 / :9100 and the socket owner was verified with ss -ltnp before any HTTP call.
  * BOOKKEEPING BOUNDARY: while this sweep ran, a sibling writer appended DF-BOARDCTL-52..55 to the canonical
    checkout's board at 2026-11-08 04:21:07 (git diff --stat: tasks.jsonl +4, events.jsonl +4, all four events
    event_type=task_created actor=foreman). The shared checkout is therefore DIRTY and this lane did not write to
    it. Rows were filed from a dedicated worktree <worktree> branched off origin/main at 1cda136f
    (branch readme-audit-2026-11-08) and are branch-only / pending merge until the default branch contains them.

FINDINGS INDEX (5 rows, doc severity P4/P5 per Bane ruling 2026-09-30)
  README-013  P4  stale "published v0.1.11 does not accept --acceptance-criteria" blocks (5 sites) — v0.2.0 accepts it
  README-014  P4  initiatives section documents current-surface commands the published v0.2.0 asset refuses
  README-015  P4  v0.1.11-pinned verification commands fail on the current tree (make version-check / install-check)
  README-016  P4  README says make install "never builds from the checkout"; Makefile falls back to a source build
  README-017  P5  vendor gates (make vendor-check / vendor-strip, CI "Vendor-check") are undocumented

WHAT PASSED (re-measured this sweep, filed nothing)
  Install: both v0.2.0 URLs HTTP 200; sha256sum -c OK; boardctl version v0.2.0; 7 platforms in SHA256SUMS match the
    README's platform list.
  version --json, list/stats fixture semantics (README:83-91: 2 vs 3), show/--events/--skip-bad-lines, validate,
    doctor, validate --repair/--strict-keys/--fail-on dangling-dep, create/update/header/event vocabulary refusals,
    sweep-status, normalize-board (+--json), render (html/json/-o -), import (--dry-run/--renumber), umbrella
    (census/list/stats/validate/json/broken-link finding), serve (/ui 200, /api/boards, non-loopback refusal),
    install --dry-run/--timeout (pre-commit hook armed), initiatives on a from-HEAD build.
  Gates: go build, go vet, make fmt-check, make version-check (unpinned form, as CI runs it), make spec-check,
    make cover-check (total 82.7% floor 80%, lowest package 66.5% floor 60%), make vuln-check — all exit 0.
  Links/anchors: docs/specs/board-analytics-report.md, docs/board-fingerprint-rules.md,
    .github/workflows/multiarch.yml, docs/dogfood/, skills/boardctl-usage/, .gitreins/, LICENSE, .gitleaks.toml
    all exist; anchors #cutting-a-release-bt-030, #development, #deployment-freshness-gate-bt-057, #board-topology
    resolve to real headings; no badges exist, so no badge drift.

UNVERIFIED THIS SWEEP
  * make release (cross-compile + stamp check): not run — it rewrites dist/ and needs a clean tagged clone; the
    v0.2.0 stamp was instead verified from the published asset (vcs.revision=2d3f2e6c…, vcs.modified=false).
  * The tag-push CI cut (multiarch.yml -> org reusable workflow) was not triggered; it needs a real tag push.
  * github.com/coding-hermes (org link in README:3) was not fetched (no browser fetch in this lane).



================================================================================
== TRACE MARKERS
================================================================================

================================================================================
== TRACE MARKERS (one per filed finding; every claim below carries row= and evidence=)
================================================================================
ch:trace row=README-013 spec=README.md#install wave=none:serial-tick test=none:doc-only doc=README.md prompt=tick:boardctl-readme-2026-11-08-04-02-17 evidence=.coding-hermes/audits/readme-2026-11-08/readme-sweep-2026-11-08.txt witness=none:awaiting-worker-fix verdict=none:open-finding commit=none:open memory=/readme/audit/2026-11-08
ch:trace row=README-014 spec=README.md#initiatives-scoping-work-into-scoopable-programmes wave=none:serial-tick test=none:doc-only doc=README.md prompt=tick:boardctl-readme-2026-11-08-04-02-17 evidence=.coding-hermes/audits/readme-2026-11-08/readme-sweep-2026-11-08.txt witness=none:awaiting-worker-fix verdict=none:open-finding commit=none:open memory=/readme/audit/2026-11-08
ch:trace row=README-015 spec=README.md#readme-commandreference-drift-check-manual-only wave=none:serial-tick test=none:doc-only doc=README.md prompt=tick:boardctl-readme-2026-11-08-04-02-17 evidence=.coding-hermes/audits/readme-2026-11-08/readme-sweep-2026-11-08.txt witness=none:awaiting-worker-fix verdict=none:open-finding commit=none:open memory=/readme/audit/2026-11-08
ch:trace row=README-016 spec=README.md#installing-the-deployed-copy-bt-042 wave=none:serial-tick test=none:doc-only doc=README.md prompt=tick:boardctl-readme-2026-11-08-04-02-17 evidence=.coding-hermes/audits/readme-2026-11-08/readme-sweep-2026-11-08.txt witness=none:awaiting-worker-fix verdict=none:open-finding commit=none:open memory=/readme/audit/2026-11-08
ch:trace row=README-017 spec=README.md#development wave=none:serial-tick test=none:doc-only doc=README.md prompt=tick:boardctl-readme-2026-11-08-04-02-17 evidence=.coding-hermes/audits/readme-2026-11-08/readme-sweep-2026-11-08.txt witness=none:awaiting-worker-fix verdict=none:open-finding commit=none:open memory=/readme/audit/2026-11-08

FINDING DETAIL AT A GLANCE (the row text carries the exact commands; this is the index)
  README-013  README.md:104-106, 142-144, 1460-1463, 1490-1498, 1547-1551 say the published asset is v0.1.11 and
              rejects --acceptance-criteria; README.md:17 says the current release is v0.2.0, and the v0.2.0 asset
              creates and updates the field (rc=0, key on the row) — the caveat now misdirects readers to a source
              build for a feature the release ships. Inverse recurrence of closed README-012.
  README-014  README.md:1381-1386 present `initiatives`, `list --initiative`, `--clear-initiative` as the current
              surface with no release caveat; the published v0.2.0 asset answers `unknown command "initiatives"`
              (rc=2) and `flag provided but not defined: -initiative` (rc=1). A from-HEAD build runs all four (rc=0).
  README-015  README.md:1641 tells the reader to run `make version-check VERSION_TAG=v0.1.11` "exactly as written";
              it fails rc=2 ("repo README is stale vs the expected release v0.1.11") on the clean published tip.
              README.md:1798-1813's drift table is likewise pinned to v0.1.11 with revision 3f3dd16…, and
              `make install-check RELEASE_TAG=v0.1.11` fails rc=2 against a correctly installed v0.2.0 copy
              (real revision 2d3f2e6c…). The unpinned forms (`make version-check`, `RELEASE_TAG=v0.2.0`) pass.
  README-016  README.md:1785-1788 promises `make install` "never builds from the checkout" and "fails loudly on a
              missing dist/"; Makefile:245-255 instead prints "no release assets found; building from source",
              runs go build, and installs a `dev` binary — rc=0 — which is exactly the drift BT-042 exists to catch.
  README-017  The Development block (README.md:1590-1600) and the ci.yml description (README.md:1571) omit the
              vendor gates: ci.yml runs a "Vendor-check" step (go test -run TestVendorcheck ./internal/vendorcheck)
              and the Makefile ships make vendor-check / make vendor-strip; `grep -i vendor README.md` finds nothing.


================================================================================
== A1. CURRENT PUBLISHED RELEASE: fetch, checksum verify, identity (README Install block)
================================================================================
### release identity of the CURRENT published release (v0.2.0), fetched 2026-11-08T04:19Z
$ curl -sSL -o boardctl_linux_amd64 https://github.com/coding-hermes/boardctl/releases/download/v0.2.0/boardctl_linux_amd64
HTTP 200, size_download=7757986
$ curl -sSL -o SHA256SUMS https://github.com/coding-hermes/boardctl/releases/download/v0.2.0/SHA256SUMS
HTTP 200, size_download=617
$ cat SHA256SUMS
e45a2bdd00919fa8b5a920f2cac6f90639181d4192a2583dd239edc649dbd4cc  boardctl_darwin_amd64
24d923c73fef609b75640b4081d0f1d9515dc0a451d31e3e551ba77a7c542249  boardctl_darwin_arm64
15476d353b75a8a65c55ba4fc5abb9c9f04720a449614c4b2fc566418e4f3ac7  boardctl_freebsd_amd64
489b93bac0a1a46e2438e28523f32662a80e8e7d9217511b7b433ac7f435c038  boardctl_linux_amd64
73f9b23965854d6002f57a6b020a9669cbb111fb02c428d3e2230923282898b8  boardctl_linux_arm
b7b4c9e1ddbf09afea2f7f56d1e577065c7eface19d8cca94dacd2e5acb98e79  boardctl_linux_arm64
70d351f52df2e6f1c6fc94c0db8bd4cf4dc4ceda46f94877eef538a1ab5308d8  boardctl_windows_amd64.exe
$ sha256sum -c --ignore-missing SHA256SUMS
boardctl_linux_amd64: OK
rc=0
$ ./boardctl_linux_amd64 version
boardctl version v0.2.0
rc=0
$ go version -m boardctl_linux_amd64
<probe>/boardctl_linux_amd64: go1.26.10
	path	github.com/coding-hermes/boardctl/cmd/boardctl
	mod	github.com/coding-hermes/boardctl	v0.2.0	
	build	-buildmode=exe
	build	-compiler=gc
	build	-trimpath=true
	build	CGO_ENABLED=0
	build	GOARCH=amd64
	build	GOOS=linux
	build	GOAMD64=v1
	build	vcs=git
	build	vcs.revision=2d3f2e6c63d220ab31656f93de3e642f5cde3a37
	build	vcs.time=2026-10-30T20:51:34Z
	build	vcs.modified=false

### gh release list
v0.2.0	Latest	v0.2.0	2026-10-30T20:57:48Z
v0.1.11		v0.1.11	2026-10-18T14:38:39Z
v0.1.10		v0.1.10	2026-09-29T11:30:17Z

### acceptance-criteria: does the PUBLISHED v0.2.0 asset accept the flag the README says it rejects?
(README.md:104-106, 142-144, 1460-1463, 1490-1498, 1547-1551)


================================================================================
== A2. --acceptance-criteria LIVE WRITE with the published v0.2.0 asset (README:1466-1469 examples, verbatim)
================================================================================
### live write probe with the PUBLISHED v0.2.0 asset (README.md:1466-1469 examples, verbatim)
$ boardctl -C <scratch>/proj init --project accprobe --namespace accprobe
initialized board at <probe>/frozen/acc/proj/.coding-hermes/board
  wrote tasks.jsonl
  wrote events.jsonl
  wrote board.jsonl
  wrote fixtures.jsonl
next: boardctl create --id TASK-1 --title "First task"
rc=0

$ boardctl -C <scratch>/proj create --id FEAT-1 --title "Add retry" --acceptance-criteria "1) retry is idempotent; 2) full suite green"
created task FEAT-1 (appended to <probe>/frozen/acc/proj/.coding-hermes/board/tasks.jsonl)
rc=0

$ boardctl -C <scratch>/proj update FEAT-1 --acceptance-criteria "1) retry is idempotent; 2) full suite green; 3) no new deps"
updated task FEAT-1: acceptance_criteria, updated_at
rc=0

$ grep FEAT-1 .coding-hermes/board/tasks.jsonl   (acceptance_criteria key present on the row)
{"attempts":0,"blocked_reason":null,"blocked_since":null,"blocks":[],"capability_tags":[],"ci_result":null,"commit_hash":null,"completed_at":null,"complexity":3,"created_at":"2026-11-08T04:23:47.35728+00:00","depends_on":[],"dispatched_at":null,"exit_code":null,"fallback_model":null,"fallback_provider":null,"files_changed":[],"foreman_note":null,"guard_result":null,"id":"FEAT-1","lines_added":0,"lines_removed":0,"perpetual":null,"primary_model":null,"primary_provider":null,"priority":"P2","reasoning":null,"review_notes":null,"status":"pending","title":"Add retry","updated_at":"2026-11-08T04:23:47.36157+00:00","worker_status":"pending","worker_summary":null,"acceptance_criteria":"1) retry is idempotent; 2) full suite green; 3) no new deps","detail":{"fingerprint":"b186b7bd2ec6cb6973eea32bcb86940a809ee7262d6974e4609d6fefd3b2e29d"}}

$ boardctl create --help | grep acceptance
1:boardctl create --id ID --title T [--status S] [--priority P] [--complexity N] [--depends-on a,b] [--reasoning R] [--capability-tags tags] [--acceptance-criteria TEXT] [--evidence-run-id id] [--evidence-fingerprint FP] [--evidence-artifact PATH]... [--worktree PATH] [--branch NAME] [--session ID]... [--pull-request PR]... [--assoc-branch B]... [--assoc-worktree PATH]... [--deferred] [--force] [flags] [-C dir]
3:          --acceptance-criteria is free text and may span lines (embedded
6:          without it. Omit the flag and NO acceptance_criteria key is
$ boardctl update --help | grep acceptance
3:          [--replace] [--blocked-reason R] [--completed-at TS] [--deferred true|false] [--acceptance-criteria TEXT]
8:          --acceptance-criteria is free text and may span lines (embedded


================================================================================
== B. INITIATIVES: published v0.2.0 asset vs from-HEAD build, plus render / import / umbrella setup / hook install
================================================================================
== binaries: published=boardctl version v0.2.0 head=boardctl version dev
== README:1381-1386 initiatives on PUBLISHED v0.2.0
----- [init] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/proj init --project myproject --namespace myproject
initialized board at <probe>/matrixB/proj/.coding-hermes/board
  wrote tasks.jsonl
  wrote events.jsonl
  wrote board.jsonl
  wrote fixtures.jsonl
next: boardctl create --id TASK-1 --title "First task"
----- [pub_create_init] rc=1 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/proj create --id FEAT-9 --title arm the leg --initiative lane-hooks
flag provided but not defined: -initiative
boardctl create --id ID --title T [--status S] [--priority P] [--complexity N] [--depends-on a,b] [--reasoning R] [--capability-tags tags] [--acceptance-criteria TEXT] [--evidence-run-id id] [--evidence-fingerprint FP] [--evidence-artifact PATH]... [--worktree PATH] [--branch NAME] [--session ID]... [--pull-request PR]... [--assoc-branch B]... [--assoc-worktree PATH]... [--deferred] [--force] [flags] [-C dir]

          --acceptance-criteria is free text and may span lines (embedded
          newlines are JSON-escaped into the one physical row line). It is
          the dispatchability field: a foreman will not dispatch a row
          without it. Omit the flag and NO acceptance_criteria key is
          written (never an empty placeholder).

          --priority defaults to P2, EXCEPT on paperwork lanes:
          -review → P4, -docs/-readme → P5. An explicit --priority
          always wins (BT-071).
boardctl: flag provided but not defined: -initiative
----- [pub_initiatives] rc=2 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/proj initiatives
boardctl: unknown command "initiatives"

boardctl — manage coding-hermes JSONL foreman boards

usage:
  boardctl [-C <repo-or-board-dir>] <command> [flags]

commands:
  init    [-C dir] [--project P] [--namespace NS]   bootstrap a fresh board
  list    [--status S] [--priority P] [--json] [--all] [--skip-bad-lines]
  show    <id> [--events]
  create  --id ID --title T [--priority P] [--complexity N] [--depends-on a,b]
          [--reasoning R] [--capability-tags a,b] [--status pending] [--force]
          [--acceptance-criteria TEXT] [--evidence-run-id RUN] [--evidence-fingerprint FP] [--evidence-artifact PATH]... [--worktree PATH] [--branch NAME]
          [--session ID]... [--pull-request PR]... [--assoc-branch B]... [--assoc-worktree PATH]... [--deferred]   (a re-detected finding is REFUSED with exit 2)
  update  <id> --status complete [--title T] [--priority P1] [--worker-status S]
          [--commit-hash SHA] [--guard PASS|FAIL|SKIP] [--ci GREEN|RED|SKIP]
          [--summary S] [--note S (appends)] [--blocked-reason R] [--completed-at TS]
          [--replace] [--worktree PATH] [--branch NAME] [--session ID]...
          [--acceptance-criteria TEXT]
          [--pull-request PR]... [--assoc-branch B]... [--assoc-worktree PATH]...
          (repeatable; each merges one element into its pull_requests/
          branches/worktrees dimension — existing elements, the other
          dimensions, and the singular worktree/branch fields survive)
          [--evidence-run-id RUN] [--evidence-fingerprint FP] [--evidence-artifact PATH]... [--deferred true|false] [--normalize] [--force]
== README:1381-1386 initiatives on HEAD build
----- [head_create_init] rc=0 :: <probe>/boardctl_head_build -C <probe>/matrixB/proj create --id FEAT-9 --title arm the leg --initiative lane-hooks
created task FEAT-9 (appended to <probe>/matrixB/proj/.coding-hermes/board/tasks.jsonl)
----- [head_list_init] rc=0 :: <probe>/boardctl_head_build -C <probe>/matrixB/proj list --initiative lane-hooks --json
[
  {
    "attempts": 0,
    "blocked_reason": null,
    "blocked_since": null,
    "blocks": [],
    "capability_tags": [],
    "ci_result": null,
    "commit_hash": null,
    "completed_at": null,
    "complexity": 3,
    "created_at": "2026-11-08T04:20:53.278334+00:00",
    "depends_on": [],
    "dispatched_at": null,
    "exit_code": null,
    "fallback_model": null,
    "fallback_provider": null,
    "files_changed": [],
    "foreman_note": null,
    "guard_result": null,
    "id": "FEAT-9",
    "lines_added": 0,
    "lines_removed": 0,
    "perpetual": null,
    "primary_model": null,
----- [head_initiatives] rc=0 :: <probe>/boardctl_head_build -C <probe>/matrixB/proj initiatives
INITIATIVE  TOTAL  ACTIONABLE  DEFERRED  BY STATUS
lane-hooks  1      1           0         pending=1
----- [head_initiatives_json] rc=0 :: <probe>/boardctl_head_build -C <probe>/matrixB/proj initiatives --json --status pending
[
  {
    "initiative": "lane-hooks",
    "total": 1,
    "actionable": 1,
    "deferred": 0,
    "by_status": {
      "pending": 1
    }
  }
]
----- [head_clear] rc=0 :: <probe>/boardctl_head_build -C <probe>/matrixB/proj update FEAT-9 --clear-initiative
updated task FEAT-9: initiative, updated_at
== README:513-515 render
----- [render_html] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/proj render -o report.html
wrote report.html (68277 bytes, 1 board(s))
-rw-r--r-- 1 kara kara 68277 Nov  7 23:20 <probe>/matrixB/report.html
----- [render_json] rc=0 :: <probe>/boardctl_linux_amd64 render -C <probe>/matrixB/proj -o report2.html --tz America/Bogota --json report.json
wrote report.json (board-report/v1 payload)
wrote report2.html (68286 bytes, 1 board(s))
-rw-r--r-- 1 kara kara  3655 Nov  7 23:20 <probe>/matrixB/report.json
-rw-r--r-- 1 kara kara 68286 Nov  7 23:20 <probe>/matrixB/report2.html
----- [render_stdout] rc=0 :: <probe>/boardctl_linux_amd64 render -C <probe>/matrixB/proj -o -
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Board Report</title>
<style>
:root{
  --bg:#0d1117; --bg2:#161b22; --bg3:#1c2330; --fg:#e6edf3; --muted:#8b949e;
  --line:#30363d; --accent:#58a6ff; --green:#3fb950; --red:#f85149;
  --orange:#d29922; --blue:#58a6ff; --purple:#bc8cff; --gray:#6e7681;
}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.45 -apple-system,"Segoe UI",Roboto,Helvetica,Arial,sans-serif}
a{color:var(--accent);text-decoration:none;cursor:pointer}
a:hover{text-decoration:underline}
header#topbar{display:flex;align-items:center;gap:16px;padding:10px 18px;background:var(--bg2);border-bottom:1px solid var(--line);position:sticky;top:0;z-index:5}
header h1{font-size:16px;margin:0;font-weight:600}
.tab{background:none;border:1px solid transparent;color:var(--muted);padding:4px 12px;border-radius:6px;cursor:pointer;font-size:13px}
.tab.active{color:var(--fg);border-color:var(--line);background:var(--bg3)}
.tab:hover{color:var(--fg)}
#report-meta{margin-left:auto;color:var(--muted);font-size:12px}
main{padding:18px;max-width:1280px;margin:0 auto}
#banner{margin:12px 18px 0;padding:10px 14px;border:1px solid var(--orange);border-radius:8px;background:rgba(210,153,34,.08);display:flex;gap:10px;align-items:center}
#banner .bn-notes{margin-top:8px;display:none;white-space:pre-wrap;color:var(--muted);font-size:12px;max-height:200px;overflow:auto}
== README:251-252 import (needs a board-report/v1 json = render --json)
----- [import_dry] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/proj import report.json --dry-run --renumber
boardctl import plan for "myproject" (<probe>/matrixB/proj/.coding-hermes/board)
  export: report.json (board-report/v1, rendered 2026-11-07T23:20:53-05:00)
  tasks: 0 new, 2 identical (no-op), 0 skipped (differs)
  = NEVER-DONE identical (no-op)
  = FEAT-9 identical (no-op)
  events: 0 events appended (0 from export, 0 task_created for new rows; 1 task_created rows in the export itself are never replayed — Board.Create emits fresh ones at apply time)
planned diff: no file changes (no-op import)
plan: 0 new tasks, 0 updates, 0 events
dry-run: nothing written
----- [import_apply] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/proj import report.json --renumber
boardctl import plan for "myproject" (<probe>/matrixB/proj/.coding-hermes/board)
  export: report.json (board-report/v1, rendered 2026-11-07T23:20:53-05:00)
  tasks: 0 new, 2 identical (no-op), 0 skipped (differs)
  = NEVER-DONE identical (no-op)
  = FEAT-9 identical (no-op)
  events: 0 events appended (0 from export, 0 task_created for new rows; 1 task_created rows in the export itself are never replayed — Board.Create emits fresh ones at apply time)
planned diff: no file changes (no-op import)
plan: 0 new tasks, 0 updates, 0 events
imported: 0 task row(s), 0 event(s), 0 fixture row(s) written
== README:271-289 umbrella
----- [umbrella] rc=2 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/myproduct umbrella
boardctl: no JSONL foreman board found: <probe>/matrixB/myproduct holds no board and no .coding-hermes/links registry (an umbrella repo links child boards under .coding-hermes/links/)
----- [umbrella_list] rc=2 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/myproduct umbrella --list
boardctl: no JSONL foreman board found: <probe>/matrixB/myproduct holds no board and no .coding-hermes/links registry (an umbrella repo links child boards under .coding-hermes/links/)
----- [umbrella_list_status] rc=2 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/myproduct umbrella --list --status pending
boardctl: no JSONL foreman board found: <probe>/matrixB/myproduct holds no board and no .coding-hermes/links registry (an umbrella repo links child boards under .coding-hermes/links/)
----- [umbrella_list_status2] rc=2 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/myproduct umbrella --list --status in_progress,review
boardctl: no JSONL foreman board found: <probe>/matrixB/myproduct holds no board and no .coding-hermes/links registry (an umbrella repo links child boards under .coding-hermes/links/)
----- [umbrella_stats] rc=2 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/myproduct umbrella --stats
boardctl: no JSONL foreman board found: <probe>/matrixB/myproduct holds no board and no .coding-hermes/links registry (an umbrella repo links child boards under .coding-hermes/links/)
----- [umbrella_validate] rc=2 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/myproduct umbrella --validate
boardctl: no JSONL foreman board found: <probe>/matrixB/myproduct holds no board and no .coding-hermes/links registry (an umbrella repo links child boards under .coding-hermes/links/)
----- [umbrella_json] rc=2 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/myproduct umbrella --json
boardctl: no JSONL foreman board found: <probe>/matrixB/myproduct holds no board and no .coding-hermes/links registry (an umbrella repo links child boards under .coding-hermes/links/)
== README:643-650 install (pre-commit hook) dry-run then arm on scratch repo
----- [install_dry] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/hookrepo install --dry-run
#!/bin/sh
# >>> boardctl board-lint managed block (boardctl install; replaced on re-install) >>>
boardctl_bin="<probe>/boardctl_linux_amd64"
board_root="<probe>/matrixB/hookrepo"
boardctl_timeout=30
boardctl_lint_mode="enforce"
# board-lint: guard commits that break the JSONL board
# (duplicate ids; dangling depends_on via --fail-on dangling-dep).
# Installed by boardctl install.
# A slow/missing/wedged boardctl SKIPS (no block) — it must never wedge commits.
if [ -d "$board_root/.coding-hermes/board" ]; then
	if [ -x "$boardctl_bin" ]; then
		lint_bin="$boardctl_bin"
	elif command -v boardctl >/dev/null 2>&1; then
		lint_bin="$(command -v boardctl)"
	fi
	if [ -n "${lint_bin:-}" ] && command -v timeout >/dev/null 2>&1; then
		lint_out="$(timeout "$boardctl_timeout" "$lint_bin" validate -C "$board_root" --fail-on dangling-dep 2>&1)"
		lint_status=$?
		case "$lint_status" in
			0)
				:
				;;
			124)
				echo "board-lint: skipped (timeout)" >&2
----- [install_arm] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/hookrepo install
board-lint pre-commit hook installed at <probe>/matrixB/hookrepo/.git/hooks/pre-commit (timeout 30s)
-rwxr-xr-x 1 kara kara 1664 Nov  7 23:20 <probe>/matrixB/hookrepo/.git/hooks/pre-commit
----- [install_timeout] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixB/hookrepo install --timeout 10
board-lint pre-commit hook installed at <probe>/matrixB/hookrepo/.git/hooks/pre-commit (timeout 10s)
== END MATRIX B


================================================================================
== C. CORE COMMAND MATRIX (init/list/stats/show/validate/doctor/create/update/header/event/sweep/normalize)
================================================================================
== boardctl: boardctl version v0.2.0
== init
----- [init] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject init --project myproject --namespace myproject
initialized board at <probe>/matrixA/myproject/.coding-hermes/board
  wrote tasks.jsonl
  wrote events.jsonl
  wrote board.jsonl
  wrote fixtures.jsonl
next: boardctl create --id TASK-1 --title "First task"
== seed two real rows
----- [create1] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject create --id FEAT-1 --title Add retry --priority P2
created task FEAT-1 (appended to <probe>/matrixA/myproject/.coding-hermes/board/tasks.jsonl)
----- [create2] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject create --id FEAT-2 --title Add caching --priority P3
created task FEAT-2 (appended to <probe>/matrixA/myproject/.coding-hermes/board/tasks.jsonl)
== README:85 list shows 2, list --all shows 3; stats total 2, stats --all 3
----- [list] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject list
ID               PRI  STATUS      TITLE
FEAT-1           P2   pending     Add retry
FEAT-2           P3   pending     Add caching
----- [list_all] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject list --all
ID               PRI  STATUS      TITLE
NEVER-DONE       P3   pending     Perpetual audit fixture — find the next worthwhile improvement (never complete)
FEAT-1           P2   pending     Add retry
FEAT-2           P3   pending     Add caching
----- [stats] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject stats
total tasks: 2
actionable: 2
deferred: 0
by status:
  pending      2
by priority:
  P2   1
  P3   1
----- [stats_all] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject stats --all
total tasks: 3
actionable: 3
deferred: 0
by status:
  pending      3
by priority:
  P2   1
  P3   2
== README:96 version --json promises {"version":"...","build":"..."}
----- [version_json] rc=0 :: <probe>/boardctl_linux_amd64 version --json
{"version":"v0.2.0","build":"v0.2.0"}
== README:70-77 list/show/validate/doctor
----- [list_pending] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject list --status pending
ID               PRI  STATUS      TITLE
FEAT-1           P2   pending     Add retry
FEAT-2           P3   pending     Add caching
----- [show_events] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject show FEAT-1 --events
{
  "attempts": 0,
  "blocked_reason": null,
  "blocked_since": null,
  "blocks": [],
  "capability_tags": [],
  "ci_result": null,
  "commit_hash": null,
  "completed_at": null,
  "complexity": 3,
  "created_at": "2026-11-08T04:20:29.099077+00:00",
  "depends_on": [],
  "dispatched_at": null,
  "exit_code": null,
  "fallback_model": null,
  "fallback_provider": null,
  "files_changed": [],
  "foreman_note": null,
  "guard_result": null,
  "id": "FEAT-1",
  "lines_added": 0,
  "lines_removed": 0,
  "perpetual": null,
  "primary_model": null,
  "primary_provider": null,
  "priority": "P2",
  "reasoning": null,
  "review_notes": null,
  "status": "pending",
  "title": "Add retry",
  "updated_at": "2026-11-08T04:20:29.099077+00:00",
  "worker_status": "pending",
  "worker_summary": null,
  "detail": {
    "fingerprint": "b186b7bd2ec6cb6973eea32bcb86940a809ee7262d6974e4609d6fefd3b2e29d"
  }
}
events for FEAT-1 (1):
{"id":1,"timestamp":"2026-11-08T04:20:29.099249+00:00","event_type":"task_created","task_id":"FEAT-1","actor":"foreman","detail":"{\"id\":\"FEAT-1\",\"status\":\"pending\"}","tick_number":null}
----- [show_skipbad] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject show FEAT-1 --skip-bad-lines
{
  "attempts": 0,
  "blocked_reason": null,
  "blocked_since": null,
  "blocks": [],
  "capability_tags": [],
  "ci_result": null,
  "commit_hash": null,
  "completed_at": null,
  "complexity": 3,
  "created_at": "2026-11-08T04:20:29.099077+00:00",
  "depends_on": [],
  "dispatched_at": null,
  "exit_code": null,
  "fallback_model": null,
  "fallback_provider": null,
  "files_changed": [],
  "foreman_note": null,
  "guard_result": null,
  "id": "FEAT-1",
  "lines_added": 0,
  "lines_removed": 0,
  "perpetual": null,
  "primary_model": null,
  "primary_provider": null,
  "priority": "P2",
  "reasoning": null,
  "review_notes": null,
  "status": "pending",
  "title": "Add retry",
  "updated_at": "2026-11-08T04:20:29.099077+00:00",
  "worker_status": "pending",
  "worker_summary": null,
  "detail": {
    "fingerprint": "b186b7bd2ec6cb6973eea32bcb86940a809ee7262d6974e4609d6fefd3b2e29d"
  }
}
----- [validate] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject validate
board: <probe>/matrixA/myproject/.coding-hermes/board (topology A)
rows: 3 tasks, 2 events, 1 fixtures, header parsed
key uniformity: 3/3 rows carry only canon keys (0 drift)
RESULT: OK (0 warning(s))
----- [doctor] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject doctor
board: <probe>/matrixA/myproject/.coding-hermes/board (topology A)
rows: 3 tasks, 2 events, 1 fixtures, header parsed
key uniformity: 3/3 rows carry only canon keys (0 drift)
[warn] not inside a git repo — git tracked-set check skipped
RESULT: OK (1 warning(s))
== README:119-124 depends-on unknown id must be REFUSED exit 1
----- [depends_unknown] rc=1 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject create --id FEAT-3 --title Add jitter --depends-on FEAT-999
boardctl: depends_on references nonexistent task id(s): FEAT-999 — create aborted (create the dependency task first)
== README:890 update --status done must exit 1
----- [update_done] rc=1 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject update FEAT-1 --status done
boardctl: status "done" not in write vocabulary {failed,pending,in_progress,review,blocked,complete}
== README:1154-1155 bad id refused, --force writes
----- [badid] rc=1 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject create --id bad id! --title x
boardctl: task id "bad id!" does not match the fleet id format ^[A-Z][A-Z0-9]+(-[A-Z0-9]+)+$ (uppercase PREFIX-SEGMENT ids like QA-BOARDCTL-001 or BT-023; pass --force to write it anyway)
----- [badid_force] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject create --id bad id! --title x --force
created task bad id! (appended to <probe>/matrixA/myproject/.coding-hermes/board/tasks.jsonl)
== README:1216 completion REQUIRES --evidence-run-id
----- [complete_noev] rc=1 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject update FEAT-2 --status complete
boardctl: evidence run required: completion REFUSED for "FEAT-2": --evidence-run-id is REQUIRED when setting status "complete" (TR-251); re-run with --evidence-run-id <run-id> — the run id is recorded on the row's detail.evidence[] and must resolve to a stored run (e.g. .gitreins/history/<date>/<run-id>/verdict.json)
== README:1367 stats deferred line
----- [create_deferred] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject create --id FEAT-3 --title Maybe later --deferred
created task FEAT-3 (appended to <probe>/matrixA/myproject/.coding-hermes/board/tasks.jsonl)
----- [stats_deferred] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject stats
total tasks: 4
actionable: 3
deferred: 1
by status:
  pending      4
by priority:
  P2   3
  P3   1
== README:197-203 header
----- [header_json] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject header --json
{"project":"myproject","namespace":"myproject","version":1,"ticks_total":0,"ticks_idle":0,"cooldown_s":21600,"last_commit":null}
----- [header_pretty] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject header
{
  "project": "myproject",
  "namespace": "myproject",
  "version": 1,
  "ticks_total": 0,
  "ticks_idle": 0,
  "cooldown_s": 21600,
  "last_commit": null
}
----- [header_set] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject header --set-ticks-total 42 --set-last-commit abc1234
header updated (line 1 of board.jsonl): ticks_total, last_commit, updated_at
----- [header_neg] rc=1 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject header --set-ticks-total -1
boardctl: --set-ticks-total: -1 is negative — header counters must be >= 0
== README:180/182 event
----- [event_tick] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject event --type audit --tick 42 --detail-text tick 42 summary
event id 5 appended to <probe>/matrixA/myproject/.coding-hermes/board/events.jsonl
----- [event_badtick] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject event --type audit --tick 41 --detail-text lower tick
event id 6 appended to <probe>/matrixA/myproject/.coding-hermes/board/events.jsonl
----- [header_after] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject header --json
{"project":"myproject","namespace":"myproject","version":1,"ticks_total":42,"ticks_idle":0,"cooldown_s":21600,"last_commit":"abc1234","updated_at":"2026-11-08T04:20:29.522256+00:00"}
== README:837 event bogus vocabulary
----- [event_bogus] rc=1 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject event --type bogus --detail-text x
boardctl: event type "bogus" not in vocabulary {audit,board_bootstrap,board_init,board_migration,dogfood,e2e_verified,idle,spec_created,task_added,task_completed,task_created,task_dispatched,task_evidence,task_started,task_updated,task_verified,tick,worker_dispatched} — use one of the enumerated event_type values
== README:1384-1386 initiatives
----- [list_initiative] rc=1 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject list --initiative lane-hooks --json
flag provided but not defined: -initiative
boardctl list [--status S] [--priority P] [--json] [--all] [--skip-bad-lines] [-C dir]
boardctl: flag provided but not defined: -initiative
----- [initiatives] rc=2 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject initiatives
boardctl: unknown command "initiatives"

boardctl — manage coding-hermes JSONL foreman boards

usage:
  boardctl [-C <repo-or-board-dir>] <command> [flags]

commands:
  init    [-C dir] [--project P] [--namespace NS]   bootstrap a fresh board
  list    [--status S] [--priority P] [--json] [--all] [--skip-bad-lines]
  show    <id> [--events]
  create  --id ID --title T [--priority P] [--complexity N] [--depends-on a,b]
          [--reasoning R] [--capability-tags a,b] [--status pending] [--force]
          [--acceptance-criteria TEXT] [--evidence-run-id RUN] [--evidence-fingerprint FP] [--evidence-artifact PATH]... [--worktree PATH] [--branch NAME]
          [--session ID]... [--pull-request PR]... [--assoc-branch B]... [--assoc-worktree PATH]... [--deferred]   (a re-detected finding is REFUSED with exit 2)
  update  <id> --status complete [--title T] [--priority P1] [--worker-status S]
          [--commit-hash SHA] [--guard PASS|FAIL|SKIP] [--ci GREEN|RED|SKIP]
          [--summary S] [--note S (appends)] [--blocked-reason R] [--completed-at TS]
          [--replace] [--worktree PATH] [--branch NAME] [--session ID]...
          [--acceptance-criteria TEXT]
          [--pull-request PR]... [--assoc-branch B]... [--assoc-worktree PATH]...
          (repeatable; each merges one element into its pull_requests/
          branches/worktrees dimension — existing elements, the other
          dimensions, and the singular worktree/branch fields survive)
          [--evidence-run-id RUN] [--evidence-fingerprint FP] [--evidence-artifact PATH]... [--deferred true|false] [--normalize] [--force]
          (--status complete REQUIRES --evidence-run-id: a completion without a
          bound run is REFUSED — TR-251; doctor reports the historic debt as
          verified_complete_share)
  event   --type task_created|task_dispatched|task_completed|audit|...
          [--task-id ID] [--actor foreman] [--detail @file | --detail-text '...']
          [--tick N] [--no-tick-write]
  header  [--json] [--set-ticks-total N] [--set-ticks-idle N] [--set-last-commit SHA] [--set-last-tick TIMESTAMP]
  validate [--skip-bad-lines] [--repair] [--strict-keys] [--fail-on dangling-dep]
  sweep-status [--apply] [--decide] [--json] [--skip-bad-lines]
           report (and with --apply fix) off-vocabulary task statuses on ONE
           board: known read aliases are normalized to their canonical form;
           unknown statuses (duplicate, parked, ...) are NEVER guessed — they
           are reported with the explicit decision needed. --decide adds the
           machine-checkable decisions: off-vocab guard/ci prose maps by rule
           (first token PASS|OK->PASS, FAIL|ERROR->FAIL, GREEN->GREEN,
----- [initiatives_json] rc=2 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject initiatives --json --status pending
boardctl: unknown command "initiatives"

boardctl — manage coding-hermes JSONL foreman boards

usage:
  boardctl [-C <repo-or-board-dir>] <command> [flags]

commands:
  init    [-C dir] [--project P] [--namespace NS]   bootstrap a fresh board
  list    [--status S] [--priority P] [--json] [--all] [--skip-bad-lines]
  show    <id> [--events]
  create  --id ID --title T [--priority P] [--complexity N] [--depends-on a,b]
          [--reasoning R] [--capability-tags a,b] [--status pending] [--force]
          [--acceptance-criteria TEXT] [--evidence-run-id RUN] [--evidence-fingerprint FP] [--evidence-artifact PATH]... [--worktree PATH] [--branch NAME]
          [--session ID]... [--pull-request PR]... [--assoc-branch B]... [--assoc-worktree PATH]... [--deferred]   (a re-detected finding is REFUSED with exit 2)
  update  <id> --status complete [--title T] [--priority P1] [--worker-status S]
          [--commit-hash SHA] [--guard PASS|FAIL|SKIP] [--ci GREEN|RED|SKIP]
          [--summary S] [--note S (appends)] [--blocked-reason R] [--completed-at TS]
          [--replace] [--worktree PATH] [--branch NAME] [--session ID]...
          [--acceptance-criteria TEXT]
          [--pull-request PR]... [--assoc-branch B]... [--assoc-worktree PATH]...
          (repeatable; each merges one element into its pull_requests/
          branches/worktrees dimension — existing elements, the other
          dimensions, and the singular worktree/branch fields survive)
          [--evidence-run-id RUN] [--evidence-fingerprint FP] [--evidence-artifact PATH]... [--deferred true|false] [--normalize] [--force]
          (--status complete REQUIRES --evidence-run-id: a completion without a
          bound run is REFUSED — TR-251; doctor reports the historic debt as
          verified_complete_share)
  event   --type task_created|task_dispatched|task_completed|audit|...
          [--task-id ID] [--actor foreman] [--detail @file | --detail-text '...']
          [--tick N] [--no-tick-write]
  header  [--json] [--set-ticks-total N] [--set-ticks-idle N] [--set-last-commit SHA] [--set-last-tick TIMESTAMP]
  validate [--skip-bad-lines] [--repair] [--strict-keys] [--fail-on dangling-dep]
  sweep-status [--apply] [--decide] [--json] [--skip-bad-lines]
           report (and with --apply fix) off-vocabulary task statuses on ONE
           board: known read aliases are normalized to their canonical form;
           unknown statuses (duplicate, parked, ...) are NEVER guessed — they
           are reported with the explicit decision needed. --decide adds the
           machine-checkable decisions: off-vocab guard/ci prose maps by rule
           (first token PASS|OK->PASS, FAIL|ERROR->FAIL, GREEN->GREEN,
== README:174 normalized / 903-904 sweep-status
----- [sweep_status] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject sweep-status
board: <probe>/matrixA/myproject/.coding-hermes/board/tasks.jsonl
0/5 rows off-vocabulary: 0 fixable by --normalize, 0 need explicit decision
DRY-RUN — no files written (pass --apply to normalize the alias rows)
----- [sweep_apply] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject sweep-status --apply
board: <probe>/matrixA/myproject/.coding-hermes/board/tasks.jsonl
0/5 rows off-vocabulary: 0 fixable by --normalize, 0 need explicit decision
before: 0 off-vocabulary rows; after: 0 off-vocabulary rows
== README:996-1030 normalize-board
----- [normalize_board] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject normalize-board
board: <probe>/matrixA/myproject/.coding-hermes/board/tasks.jsonl
rows: 5
board already canonical — nothing to change (nothing written)
----- [normalize_board_json] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject normalize-board --json
{
  "board": "<probe>/matrixA/myproject/.coding-hermes/board/tasks.jsonl",
  "rows": 5,
  "changes": null,
  "refusals": null,
  "notes": null,
  "applied": false,
  "clean": true
}
== README:544 validate --repair / 603 strict-keys
----- [validate_repair] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject validate --repair
repair: wrote 5 salvaged row(s) to <probe>/matrixA/myproject/.coding-hermes/board/tasks.rewritten.jsonl
repair: dropped 0 unparseable line(s)
review: tasks.rewritten.jsonl is FOR MANUAL REVIEW — tasks.jsonl was NOT modified. Diff, verify, then replace by hand if it looks right.
----- [validate_strict] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject validate --strict-keys
board: <probe>/matrixA/myproject/.coding-hermes/board (topology A)
rows: 5 tasks, 6 events, 1 fixtures, header parsed
key uniformity: 5/5 rows carry only canon keys (0 drift)
RESULT: OK (0 warning(s))
----- [validate_dangling] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixA/myproject validate --fail-on dangling-dep
board: <probe>/matrixA/myproject/.coding-hermes/board (topology A)
rows: 5 tasks, 6 events, 1 fixtures, header parsed
key uniformity: 5/5 rows carry only canon keys (0 drift)
RESULT: OK (0 warning(s))
== END MATRIX A


================================================================================
== D. UMBRELLA per README:262-290 + broken-link finding + serve refusal of a non-loopback --addr
================================================================================
== build umbrella per README:262-268 (children are git repos with boards)
total 8
lrwxrwxrwx 1 kara kara 94 Nov  7 23:21 auth -> <probe>/matrixC/services/auth/.coding-hermes
lrwxrwxrwx 1 kara kara 97 Nov  7 23:21 billing -> <probe>/matrixC/services/billing/.coding-hermes
----- [umbrella] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixC/myproduct umbrella
UMBRELLA <probe>/matrixC/myproduct (2 board(s) in view)
  auth                    2 task(s)  topology A  <probe>/matrixC/services/auth/.coding-hermes/board
  billing                 2 task(s)  topology A  <probe>/matrixC/services/billing/.coding-hermes/board
----- [umbrella_list] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixC/myproduct umbrella --list
REPO                 ID               PRI  STATUS      TITLE
auth                 AUTH-1           P2   pending     auth row
billing              BILL-1           P2   pending     bill row
----- [umbrella_list_pending] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixC/myproduct umbrella --list --status pending
REPO                 ID               PRI  STATUS      TITLE
auth                 AUTH-1           P2   pending     auth row
billing              BILL-1           P2   pending     bill row
----- [umbrella_list_two] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixC/myproduct umbrella --list --status in_progress,review
REPO                 ID               PRI  STATUS      TITLE
no tasks in view
----- [umbrella_list_bogus] rc=2 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixC/myproduct umbrella --list --status bogus
boardctl: umbrella: --status "bogus" not in vocabulary {pending,in_progress,review,blocked,complete,failed} ('completed' accepted as alias): usage
----- [umbrella_stats] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixC/myproduct umbrella --stats
umbrella: <probe>/matrixC/myproduct
repo auth (1 tasks):
  total tasks: 1
  actionable: 1
  deferred: 0
  by status:
    pending      1
  by priority:
    P2   1
repo billing (1 tasks):
  total tasks: 1
  actionable: 1
  deferred: 0
  by status:
    pending      1
  by priority:
    P2   1
umbrella total:
  total tasks: 2
  actionable: 2
  deferred: 0
  by status:
----- [umbrella_validate] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixC/myproduct umbrella --validate
repo auth:
  board: <probe>/matrixC/services/auth/.coding-hermes/board (topology A)
  rows: 2 tasks, 1 events, 1 fixtures, header parsed
  key uniformity: 2/2 rows carry only canon keys (0 drift)
  RESULT: OK (0 warning(s))
repo billing:
  board: <probe>/matrixC/services/billing/.coding-hermes/board (topology A)
  rows: 2 tasks, 1 events, 1 fixtures, header parsed
  key uniformity: 2/2 rows carry only canon keys (0 drift)
  RESULT: OK (0 warning(s))
----- [umbrella_json] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixC/myproduct umbrella --json
{
  "root": "<probe>/matrixC/myproduct",
  "links_dir": "<probe>/matrixC/myproduct/.coding-hermes/links",
  "boards": [
    {
      "repo": "auth",
      "repo_root": "<probe>/matrixC/services/auth",
      "board_dir": "<probe>/matrixC/services/auth/.coding-hermes/board",
      "topology": "A",
      "tasks": 2
    },
    {
      "repo": "billing",
      "repo_root": "<probe>/matrixC/services/billing",
      "board_dir": "<probe>/matrixC/services/billing/.coding-hermes/board",
      "topology": "A",
      "tasks": 2
    }
  ],
  "findings": [],
  "total": 4
}
----- [umbrella_skipbad] rc=0 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixC/myproduct umbrella --stats --skip-bad-lines
umbrella: <probe>/matrixC/myproduct
repo auth (1 tasks):
  total tasks: 1
  actionable: 1
  deferred: 0
  by status:
    pending      1
  by priority:
    P2   1
repo billing (1 tasks):
  total tasks: 1
  actionable: 1
  deferred: 0
  by status:
    pending      1
  by priority:
    P2   1
umbrella total:
  total tasks: 2
  actionable: 2
  deferred: 0
  by status:
== broken link finding (README:305-307 claims exit 1 finding)
----- [umbrella_broken] rc=1 :: <probe>/boardctl_linux_amd64 -C <probe>/matrixC/brokenumbrella umbrella
UMBRELLA <probe>/matrixC/brokenumbrella (0 board(s) in view)
  (no boards in view)
[error] link ghost: broken symlink: lstat <probe>/matrixC/does-not-exist: no such file or directory
boardctl: umbrella view incomplete or invalid: 1 finding(s), 0 failing board(s)
== README:448-454 serve (bounded)
port 8787 before:
LISTEN 0      5                                   127.0.0.1:8787       0.0.0.0:*    users:(("python3",pid=7454,fd=3))            
SKIP serve: port 8787 already in use by another process
== README:449-451 non-loopback --addr must be refused exit 2
----- [serve_nonloop] rc=2 :: <probe>/boardctl_linux_amd64 serve --addr 0.0.0.0:8787
boardctl: usage: --addr "0.0.0.0:8787" refuses non-loopback host "0.0.0.0": serve binds 127.0.0.1, localhost or ::1 only (spec 6.1: loopback convenience uploader, no auth, no TLS)
== END MATRIX C
