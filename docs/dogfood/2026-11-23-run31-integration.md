# Run 31 integration — boardctl v0.6.0 (2026-11-23)

Run 31 of the boardctl dogfood lane. Unlike run 30 (typed rows, 2026-11-17), the
product moved 67 files and shipped v0.6.0 in between; this run walked what a user
touching v0.6.0 for the first time meets: the initiatives program view, the
TR-251 evidence gate, `--repoint-commit`, `depcheck`, the gzip serve path, the
import round-trip, and the hardened `make install`.

## The real-user path that worked end-to-end

```
make install            # refuses without staged assets; with dist/ assets:
                        # sha256-verifies and installs to ~/.local/bin
make install-check      # "deployed /home/kara/.local/bin/boardctl matches release v0.6.0"
boardctl -C demo init   # fresh board: tasks/events/board/fixtures + a printed next step
boardctl -C demo create --id DEMO-1 --title "P2 — wire the reporter" --type task ...
boardctl -C demo create --id DEMO-2 --title "P1 — fix the crash" --depends-on DEMO-1 \
    --initiative demo-prog --initiative-ref demo:src/report.go#L10
boardctl -C demo next                    # DEMO-1 (dependency-aware)
boardctl -C demo update DEMO-2 --status complete            # REFUSED (TR-251)
boardctl -C demo update DEMO-2 --status complete --evidence-run-id run-1   # WARN + done
boardctl -C demo update DEMO-2 --repoint-commit <40-hex>    # sanctioned re-judge re-point
boardctl -C demo initiatives             # INITIATIVE total actionable deferred | by status
boardctl -C demo render --json export.json ; boardctl -C demo import export.json --renumber
```

Every refusal we hit was the documented one, with the next command copy-pasteable:
dangling `--depends-on` (rc=1), duplicate id (rc=1), lowercase/segmentless id
format (rc=1, names `--force`), short `--commit-hash` (rc=1), `complete` without
`--evidence-run-id` (rc=1, TR-251), `--repoint-commit` on a non-complete row (rc=1),
cross-board `import` (rc=1, README-026 gate). `validate` on a torn board copy names
the exact line; `--skip-bad-lines` degrades loudly; `import` of an export is
plan-first and idempotent (3 identical rows → 0 written).

## What is new since run 30 and how it behaved

- **`--initiative` program scoping + `initiatives` rollup (BT-093).** The rollup
  and `list --initiative` / `ready` grouping all work. One defect filed:
  DF-BOARDCTL-061 — the singular `initiative` key the flag writes is not in the
  key canon, so validate permanently warns about the feature's own key and
  `--normalize` refuses to fix it (the real board warns on BT-091..095).
- **`update --repoint-commit` (DF-DIGEST-068).** Refused on a pending row,
  accepted on a complete one, rewrote `commit_hash` and appended a 205-char
  disclosure note, no event — exactly as `help` documents.
- **`make dep-check` (BT-89).** Report arm exits 0 and honestly says
  "1 of 2 build-set module(s) behind" (kin-openapi) instead of graph-wide noise.
- **gzip serve middleware (PERF-001).** All three routes (ui + 2 API) send
  `Content-Encoding: gzip` + `Vary: Accept-Encoding` and serve plain when the
  client does not advertise gzip; 69.4 KB → 18.8 KB on the ui page.
- **Evidence gate hardening (INFRA-LORE-045).** A run id that does name an
  existing `.gitreins/history` directory must carry a verdict grading THIS task
  and commit; the not-found acceptance we saw on a scratch board is the
  documented test-env carve-out, not a hole.
- **v0.6.0 install path.** `make install` with no assets refuses loudly and
  prints staging commands (RELEASE-2026-11-16-02); with assets it verifies the
  checksums and refuses to build from the checkout.

## Perf (Step 2b) — honest non-finding

Warm: stats 26.3 ms, list 28.5 ms, list --json 26.0 ms, validate 43.6 ms,
initiatives 23.1 ms, ready 24.9 ms (hyperfine, 15 runs, 295-row board, load
12.7/16 cores). Cold first-run: stats 0.02 s, validate 0.03 s. Nothing a user
would feel → no profile, no PERF row, per the perf doctrine.

## Ephemeral-bunker battery (install leg)

bunker-mvp hit 97% disk between two checks (92% → 97% in ~5 min, sibling agents
writing) and the launcher refused cleanly (`PREFLIGHT FAIL ... exceeds 94%`);
retargeted bunker-las-03 (80%, act=yes). Agent a9c51a53 ran the full battery:
launch/fresh-install/tests/ci-pass/upgrade/lint/collect all OK, chaos-disconnect/
errorpath/resource OK, docker-deploy + chaos-shutdown honestly SKIPPED (forced
off on the live Hermes host), 0 FAIL / 0 UNVERIFIED. Notably `ci-pass` is OK now
where run 30 graded it FAIL (coverage step) — the QA-BOARDCTL-014-R2 fix held.

## Files this run left in the repo

- `docs/dogfood/2026-11-23-run31-integration.md` (this file)
- `docs/dogfood/diagnostics.md` (run-31 section)
- `skills/boardctl-usage/SKILL.md` (v1.27.0: initiative canon caveat, repoint-commit,
  depcheck, import round-trip, evidence-gate carve-out)
- `.coding-hermes/evidence/boardctl-dogfood-2026-11-23-14-28-25/` (INDEX.md + proofs)
- `.coding-hermes/evidence/qa-20261123T150253Z/` (battery evidence adopted by collect)
- Board rows DF-BOARDCTL-061 (P2), DF-BOARDCTL-062 (P3) + paired events
