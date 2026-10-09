# Dogfood Run 20 — 2026-10-09 — boardctl

## Promise under test

"Rows marked complete mean the fix is provable at HEAD; a fresh machine can
install from a clean clone or the release asset and the CLI/serve surface
works as documented."

## Angle (why this run is not a repeat)

Run 19 (2026-10-01) left DF-BOARDCTL-24/25. This run took the **closure
re-verification** angle: prove at current origin/main (5dbe70f) that
DF-BOARDCTL-22's claims ("four rows complete while fixes sit unmerged on wt/
branches") are now false — i.e. the three branches landed and the gates they
carry actually fire. Plus a fresh-box install leg (bunker-mvp; las-bunker-03
and las-bunker-04 unreachable).

## Closure re-verification (all at HEAD 5dbe70f)

| Probe | Run-18/19 result | This run at 5dbe70f |
|---|---|---|
| wt/DF-BOARDCTL-19, wt/REVIEW-BOARDCTL-009, wt/BT-064 vs origin/main | NOT merged | **MERGED, ahead=0 all three** |
| CI gate-step census (delete `Gofmt` step from ci.yml in a scratch clone) | undetected | **FAIL** — census names the missing step and the exact restore command |
| `serve` + POST /nonexistent | 400 | **404** (http.NotFound wired at serve.go:420/487) |
| vulncheck version pin no-Scanner arm | silent skip | **StrictScannerEnv** (`VULNCHECK_STRICT_SCANNER`) upgrades warn→refuse; CheckVersion parses `Scanner: govulncheck@` (vulncheck.go:145,169) |

DF-BOARDCTL-22's premise is DISPROVEN at HEAD — correctly closed. Probe done
in a throwaway clone at /tmp, repo tree untouched (checked out back,
`git status` clean apart from untracked sibling files).

## Fresh-box install leg (EXECUTED — bunker-mvp)

las-bunker-03 (100.69.3.13) ssh timeout; las-bunker-04 timeout; **bunker-mvp
answered, bunkerd active** → spawn eb23223a, TTL 2h.

- Clone of documented origin (github.com/coding-hermes/boardctl) over https
  from a bare Debian user: OK, HEAD 5dbe70f.
- `make build` from checkout: **50s** (fresh Go present on the box — the
  binary path stays the zero-dep release asset per README).
- Smoke: `version`, `init` (wrote tasks/events/board/fixtures JSONL),
  `create --id SMOKE-1`, `list`, `validate` → RESULT OK 0 warnings.
- serve loopback: `/` → 200, `/nope` → 404.
- Agent destroyed, local key removed. 0 spawn 500s.

## Performance (Step 2b)

- validate on the 144-row repo board: **12.0 ms ± 0.6** (v0.1.10 binary,
  hyperfine 20 runs) / 15.4 ms ± 1.6 (HEAD build). Cold ≈ warm (0.01s).
- render of the repo board: 0.03s / 626 KB.
- install-from-checkout on the fresh box: 50s (inherent build cost).

Nothing here is slow enough that a user would notice → **no PERF rows**.

## Verdict

**SHIPPABLE** (closure-verification pass; prior SHIPPABLE core stands).
Every run-18/19 blocking finding is now false at HEAD in the direction of
fixed. No new defects found; friction this run: 0 substantive.

## Left behind

This doc, the dogfood-log.md run-20 entry. No board rows filed (nothing to
file), no code changes, no repo visibility/permission changes.
