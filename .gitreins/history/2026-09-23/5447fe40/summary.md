# Verdict: BT-051

**Task:** Cut v0.1.8 release (drift class recurrence)
**Evaluated:** 2026-09-23T07:07:29.689868
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.603s
- ✓ **tier2**
  - COMPLETE
  ✓ README pins moved to v0.1.8; tag v0.1.8 pushed from the origin/main tip; published v0.1.8 asset sha256-verified with vcs.revision == tag commit == origin/main and vcs.modified=false; asset 'version' prints v0.1.8; behavior probe proves a shipped fix vs the v0.1.7 asset; CI Multi-Arch release run green: README pins: README.md:17 'Current release: **v0.1.8**', README.md:20-21 download URLs point at /releases/download/v0.1.8/, README.md:69,73 version examples; `grep -n v0.1.7 README.md` returns nothing (no stale pins). Tag: `git rev-parse v0.1.8` = a3328d799947ba104a4481a3e75e370aaebc3ba1 == `git rev-parse origin/main` == HEAD (tag pushed from origin/main tip). Asset: `gh release download v0.1.8` boardctl_linux_amd64 sha256 = 5d422d6ab37d13032f49fa7e4dccb01cdb6b4480c6b56403f6fe529f87e49b6d, matching the published SHA256SUMS line for boardctl_linux_amd64. `go version -m boardctl_linux_amd64` shows vcs.revision=a3328d799947ba104a4481a3e75e370aaebc3ba1 (== tag commit == origin/main) and vcs.modified=false. `./boardctl_linux_amd64 version` prints 'boardctl version v0.1.8' and `version --json` prints {"version":"v0.1.8","build":"v0.1.8"}. Behavior probe vs the v0.1.7 asset (vcs.revision=f17253b9, version v0.1.7): on a board with an unparseable JSONL line, v0.1.7 `list` aborts (exit 1, 'not valid JSON'), while v0.1.8 `list --skip-bad-lines` degrades with evidence ('SKIPPED-LINES: 1 line(s) could not be parsed and were skipped', exit 1) — DF-BOARDCTL-9; and on a row with priority P9, v0.1.7 `validate` emits no priority warning while v0.1.8 warns 'priority "P9" is not in vocabulary {P0,P1,P2,P3}' — BT-048. CI: `gh run list --workflow multiarch.yml` shows run 35829487898 on tag v0.1.8 = completed/success; `gh run view 35829487898` shows all jobs green including 'multiarch / Release' (✓).
All v0.1.8 release sub-claims verified: README pins, tag==origin/main, sha256+vcs.revision+vcs.modified=false, version output, behavior probe vs v0.1.7, and green Multi-Arch CI run.

## Summary

Judge Result: BT-051

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.603s

Stage tier2: PASS
  COMPLETE
  ✓ README pins moved to v0.1.8; tag v0.1.8 pushed from the origin/main tip; published v0.1.8 asset sha256-verified with vcs.revision == tag commit == origin/main and vcs.modified=false; asset 'version' prints v0.1.8; behavior probe proves a shipped fix vs the v0.1.7 asset; CI Multi-Arch release run green: README pins: README.md:17 'Current release: **v0.1.8**', README.md:20-21 download URLs point at /releases/download/v0.1.8/, README.md:69,73 version examples; `grep -n v0.1.7 README.md` returns nothing (no stale pins). Tag: `git rev-parse v0.1.8` = a3328d799947ba104a4481a3e75e370aaebc3ba1 == `git rev-parse origin/main` == HEAD (tag pushed from origin/main tip). Asset: `gh release download v0.1.8` boardctl_linux_amd64 sha256 = 5d422d6ab37d13032f49fa7e4dccb01cdb6b4480c6b56403f6fe529f87e49b6d, matching the published SHA256SUMS line for boardctl_linux_amd64. `go version -m boardctl_linux_amd64` shows vcs.revision=a3328d799947ba104a4481a3e75e370aaebc3ba1 (== tag commit == origin/main) and vcs.modified=false. `./boardctl_linux_amd64 version` prints 'boardctl version v0.1.8' and `version --json` prints {"version":"v0.1.8","build":"v0.1.8"}. Behavior probe vs the v0.1.7 asset (vcs.revision=f17253b9, version v0.1.7): on a board with an unparseable JSONL line, v0.1.7 `list` aborts (exit 1, 'not valid JSON'), while v0.1.8 `list --skip-bad-lines` degrades with evidence ('SKIPPED-LINES: 1 line(s) could not be parsed and were skipped', exit 1) — DF-BOARDCTL-9; and on a row with priority P9, v0.1.7 `validate` emits no priority warning while v0.1.8 warns 'priority "P9" is not in vocabulary {P0,P1,P2,P3}' — BT-048. CI: `gh run list --workflow multiarch.yml` shows run 35829487898 on tag v0.1.8 = completed/success; `gh run view 35829487898` shows all jobs green including 'multiarch / Release' (✓).
All v0.1.8 release sub-claims verified: README pins, tag==origin/main, sha256+vcs.revision+vcs.modified=false, version output, behavior probe vs v0.1.7, and green Multi-Arch CI run.

Overall: PASS ✓
