# Verdict: QA-BOARDCTL-RELENG-2

**Task:** Verification rerun: product-scoped QA battery
**Evaluated:** 2026-10-25T15:30:31.861321
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ Verify the releng battery evidence is product-scoped to boardctl at the recorded HEAD, contains per-cell verdicts without no-agent or unverified product collection, and preserves the row as pending when collection cannot be independently completed. Evidence: /tmp/bunker-qa-evidence-boardctl-releng-2-20261025-151451.jsonl: Evidence file exists (23 lines, 5363 bytes). (1) PRODUCT-SCOPED: meta file /tmp/bunker-qa-evidence-boardctl-releng-2-20261025-151451.jsonl.meta has project=coding-hermes-boardctl; battery ran in clone /home/bunker-c80c0028/qa-coding-hermes-boardctl, avoiding the parent repo (git -C /home/kara rev-parse --show-toplevel => /home/kara, the original failure mode). (2) RECORDED HEAD: evidence repo-head cell records HEAD=2f97c81acfa3354d6ecc8d62f1bb80a34bf0d626; actual `git rev-parse HEAD` in /home/kara/coding-hermes-boardctl = 2f97c81acfa3354d6ecc8d62f1bb80a34bf0d626 — exact match. (3) PER-CELL VERDICTS: 22 cells with statuses OK/FAIL/N/A/INFO/PASS — repo-head, launch, toolchain-bootstrap(x9), fresh-install(FAIL), ci-pass(OK), upgrade(OK), docker-deploy(N/A), ui-probe(INFO), chaos-disconnect(OK), chaos-shutdown(N/A), chaos-corruption(N/A), chaos-resource(PASS), chaos-errorpath(OK). (4) NO no-agent/unverified: `grep -o 'no-agent|unverified'` over the evidence returned zero matches. (5) ROW PENDING: final evidence line is cell=collect status=FAIL ('cannot reach agent=c80c0028 on bunker-mvp after 3 probe attempts ... may be TRANSIENT ssh/auth'), i.e. collection not independently completed; the git-tracked authoritative board .coding-hermes/board/tasks.jsonl row QA-BOARDCTL-RELENG-2 has status="pending" with review_notes="COLLECT-FAIL no-agent" — row not closed. Test evidence: `go test -count=1 ./...` in /home/kara/coding-hermes-boardctl => EXIT_CODE=0, all 12 packages 'ok' (cmd/boardctl 4.773s, internal/board 0.765s, internal/vulncheck 2.969s). Caveat noted: .gitreins/tasks.yaml (gitignored harness state per .gitignore:5) shows status: complete, but that is evaluator bookkeeping, not the product board row.
Evidence is product-scoped to boardctl at the exact recorded HEAD 2f97c81, carries 22 per-cell verdicts with no no-agent/unverified markers, and the authoritative board row remains pending after the collect cell failed — all conjunctive parts satisfied, with go test -count=1 ./... passing (exit 0).

## Summary

Judge Result: QA-BOARDCTL-RELENG-2

Stage tier1: FAIL
    ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ Verify the releng battery evidence is product-scoped to boardctl at the recorded HEAD, contains per-cell verdicts without no-agent or unverified product collection, and preserves the row as pending when collection cannot be independently completed. Evidence: /tmp/bunker-qa-evidence-boardctl-releng-2-20261025-151451.jsonl: Evidence file exists (23 lines, 5363 bytes). (1) PRODUCT-SCOPED: meta file /tmp/bunker-qa-evidence-boardctl-releng-2-20261025-151451.jsonl.meta has project=coding-hermes-boardctl; battery ran in clone /home/bunker-c80c0028/qa-coding-hermes-boardctl, avoiding the parent repo (git -C /home/kara rev-parse --show-toplevel => /home/kara, the original failure mode). (2) RECORDED HEAD: evidence repo-head cell records HEAD=2f97c81acfa3354d6ecc8d62f1bb80a34bf0d626; actual `git rev-parse HEAD` in /home/kara/coding-hermes-boardctl = 2f97c81acfa3354d6ecc8d62f1bb80a34bf0d626 — exact match. (3) PER-CELL VERDICTS: 22 cells with statuses OK/FAIL/N/A/INFO/PASS — repo-head, launch, toolchain-bootstrap(x9), fresh-install(FAIL), ci-pass(OK), upgrade(OK), docker-deploy(N/A), ui-probe(INFO), chaos-disconnect(OK), chaos-shutdown(N/A), chaos-corruption(N/A), chaos-resource(PASS), chaos-errorpath(OK). (4) NO no-agent/unverified: `grep -o 'no-agent|unverified'` over the evidence returned zero matches. (5) ROW PENDING: final evidence line is cell=collect status=FAIL ('cannot reach agent=c80c0028 on bunker-mvp after 3 probe attempts ... may be TRANSIENT ssh/auth'), i.e. collection not independently completed; the git-tracked authoritative board .coding-hermes/board/tasks.jsonl row QA-BOARDCTL-RELENG-2 has status="pending" with review_notes="COLLECT-FAIL no-agent" — row not closed. Test evidence: `go test -count=1 ./...` in /home/kara/coding-hermes-boardctl => EXIT_CODE=0, all 12 packages 'ok' (cmd/boardctl 4.773s, internal/board 0.765s, internal/vulncheck 2.969s). Caveat noted: .gitreins/tasks.yaml (gitignored harness state per .gitignore:5) shows status: complete, but that is evaluator bookkeeping, not the product board row.
Evidence is product-scoped to boardctl at the exact recorded HEAD 2f97c81, carries 22 per-cell verdicts with no no-agent/unverified markers, and the authoritative board row remains pending after the collect cell failed — all conjunctive parts satisfied, with go test -count=1 ./... passing (exit 0).

Overall: FAIL ✗
