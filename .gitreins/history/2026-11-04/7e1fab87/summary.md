# Verdict: BT-088

**Task:** guard hook budget: set guards.hook_timeout above guards.test_timeout
**Evaluated:** 2026-11-04T18:46:48.165239
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ ACCEPTANCE: (1) .gitreins/config.yaml carries guards.hook_timeout strictly greater than guards.test_timeout with a comment naming BT-088; (2) a fresh commit/guard run produces NO 'hook_timeout <= test_timeout' warning and the go_tests arm executes in full mode; (3) negative proof: hook_timeout below test_timeout makes the warning reappear and the budget clamp, both quoted; (4) doctor/validate OK and the change committed with repo trailer + ch:trace row=BT-088.: (1) .gitreins/config.yaml:10 `test_timeout: 900`, :15 `hook_timeout: 1020` (1020 > 900), with comment at :11-14 'BT-088: the pre-commit hook budget must exceed test_timeout...'. (2) Fresh run `gitreins guard --full` (exit 0) printed 'Tier 1 Guards: PASS  (test mode: full, whole tree)' with '✓ go_tests — passed' and NO hook_timeout WARN line; log .gitreins/logs/guard-20261104T184557.924889Z.log:32-43 shows the full go test package list (cmd/boardctl, cmd/covercheck, internal/board, ...) i.e. full mode, and `grep -i hook_timeout` on that log returned nothing. (3) Negative proof reproduced live: setting hook_timeout: 300 (test_timeout 900) and re-running `gitreins guard --full` printed verbatim 'WARN: hook_timeout=300s <= test_timeout=900s in /home/kara/coding-hermes-boardctl: the guard would be cut off mid-run and report a fail-open PASS. Raising this run's budget to 1020s — set guards.hook_timeout above guards.test_timeout to fix it at the source.' — matching the clamp in engine/guard_manager.py:1585-1598 (`if self._hook_timeout <= self._test_timeout: derived = self._test_timeout + 120 ... self._hook_timeout = derived`). Config restored to 1020 afterwards; `git status --porcelain` clean. (4) `gitreins doctor` exit 0, 'OK: all 5 allowlist path entries compile as Go regexps' (no `validate` subcommand exists in gitreins 0.17.1 — `gitreins validate` errors 'invalid choice'; doctor is the available validator). Commit 6cefd6c touches only .gitreins/config.yaml (+5) and its message contains 'ch:trace row=BT-088 spec=none test=guards.hook_timeout evidence=guard-full-red-green-negative verdict=pending commit=this' (line 25) plus the repo trailer 'Co-authored-by: Alexis Okuwa <wojonstech@gmail.com>' (line 27).
All four acceptance parts verified with live evidence: config sets hook_timeout 1020 > test_timeout 900 with a BT-088 comment, a fresh full guard run is warning-free with go_tests in full mode, the negative arm reproduces the warning and 1020s clamp verbatim, and doctor passes with the commit carrying the ch:trace row=BT-088 and co-author trailer.

## Summary

Judge Result: BT-088

Stage tier1: PASS
    ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ ACCEPTANCE: (1) .gitreins/config.yaml carries guards.hook_timeout strictly greater than guards.test_timeout with a comment naming BT-088; (2) a fresh commit/guard run produces NO 'hook_timeout <= test_timeout' warning and the go_tests arm executes in full mode; (3) negative proof: hook_timeout below test_timeout makes the warning reappear and the budget clamp, both quoted; (4) doctor/validate OK and the change committed with repo trailer + ch:trace row=BT-088.: (1) .gitreins/config.yaml:10 `test_timeout: 900`, :15 `hook_timeout: 1020` (1020 > 900), with comment at :11-14 'BT-088: the pre-commit hook budget must exceed test_timeout...'. (2) Fresh run `gitreins guard --full` (exit 0) printed 'Tier 1 Guards: PASS  (test mode: full, whole tree)' with '✓ go_tests — passed' and NO hook_timeout WARN line; log .gitreins/logs/guard-20261104T184557.924889Z.log:32-43 shows the full go test package list (cmd/boardctl, cmd/covercheck, internal/board, ...) i.e. full mode, and `grep -i hook_timeout` on that log returned nothing. (3) Negative proof reproduced live: setting hook_timeout: 300 (test_timeout 900) and re-running `gitreins guard --full` printed verbatim 'WARN: hook_timeout=300s <= test_timeout=900s in /home/kara/coding-hermes-boardctl: the guard would be cut off mid-run and report a fail-open PASS. Raising this run's budget to 1020s — set guards.hook_timeout above guards.test_timeout to fix it at the source.' — matching the clamp in engine/guard_manager.py:1585-1598 (`if self._hook_timeout <= self._test_timeout: derived = self._test_timeout + 120 ... self._hook_timeout = derived`). Config restored to 1020 afterwards; `git status --porcelain` clean. (4) `gitreins doctor` exit 0, 'OK: all 5 allowlist path entries compile as Go regexps' (no `validate` subcommand exists in gitreins 0.17.1 — `gitreins validate` errors 'invalid choice'; doctor is the available validator). Commit 6cefd6c touches only .gitreins/config.yaml (+5) and its message contains 'ch:trace row=BT-088 spec=none test=guards.hook_timeout evidence=guard-full-red-green-negative verdict=pending commit=this' (line 25) plus the repo trailer 'Co-authored-by: Alexis Okuwa <wojonstech@gmail.com>' (line 27).
All four acceptance parts verified with live evidence: config sets hook_timeout 1020 > test_timeout 900 with a BT-088 comment, a fresh full guard run is warning-free with go_tests in full mode, the negative arm reproduces the warning and 1020s clamp verbatim, and doctor passes with the commit carrying the ch:trace row=BT-088 and co-author trailer.

Overall: PASS ✓
