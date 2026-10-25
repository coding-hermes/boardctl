# Verdict: QA-BOARDCTL-RELENG-2

**Task:** Collect product-scoped boardctl releng battery
**Evaluated:** 2026-10-25T16:36:03.276790
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ A product-scoped releng battery runs against /home/kara/coding-hermes-boardctl, collected evidence contains expected per-cell verdicts, and no-agent or unverified collection is accepted as green.: Product-scoped run confirmed: /tmp/bunker-qa-evidence-boardctl-releng-2-20261025-151451.jsonl.meta has project=coding-hermes-boardctl, agent=c80c0028, server=bunker-mvp (battery ran in clone /home/bunker-c80c0028/qa-coding-hermes-boardctl, not the parent /home repo). Evidence JSONL (23 lines) carries 22 product cells with per-cell verdicts: repo-head OK (HEAD=2f97c81), launch OK, toolchain-bootstrap x9 OK, fresh-install FAIL, ci-pass OK, upgrade OK, docker-deploy N/A, ui-probe INFO, chaos-disconnect OK, chaos-shutdown N/A, chaos-corruption N/A, chaos-resource PASS, chaos-errorpath OK, collect FAIL. `grep -o 'no-agent|unverified|UNVERIFIED'` over the evidence returned 0 matches, so no no-agent/unverified collection was accepted as green; the collect cell is recorded FAIL and the board row .coding-hermes/board/tasks.jsonl:205 is not closed green (status in_progress, review_notes COLLECT-FAIL no-agent). Independent verification recorded at .gitreins/history/2026-10-25/7183b418/verdict.json (tier2 PASS with per-part detail; tier1 FAIL only on the unrelated secrets guard for .gitreins/** scope). Test evidence: `go test -count=1 ./...` exit_code=0, all 12 packages 'ok' (cmd/boardctl 6.163s, internal/board 0.804s, internal/vulncheck 3.288s).
The product-scoped releng battery ran against the boardctl git root at HEAD 2f97c81, produced 22 per-cell verdicts with zero no-agent/unverified markers, and the row was not accepted as green on unverified collection; go test -count=1 ./... passes (exit 0).

## Summary

Judge Result: QA-BOARDCTL-RELENG-2

Stage tier1: FAIL
    ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ A product-scoped releng battery runs against /home/kara/coding-hermes-boardctl, collected evidence contains expected per-cell verdicts, and no-agent or unverified collection is accepted as green.: Product-scoped run confirmed: /tmp/bunker-qa-evidence-boardctl-releng-2-20261025-151451.jsonl.meta has project=coding-hermes-boardctl, agent=c80c0028, server=bunker-mvp (battery ran in clone /home/bunker-c80c0028/qa-coding-hermes-boardctl, not the parent /home repo). Evidence JSONL (23 lines) carries 22 product cells with per-cell verdicts: repo-head OK (HEAD=2f97c81), launch OK, toolchain-bootstrap x9 OK, fresh-install FAIL, ci-pass OK, upgrade OK, docker-deploy N/A, ui-probe INFO, chaos-disconnect OK, chaos-shutdown N/A, chaos-corruption N/A, chaos-resource PASS, chaos-errorpath OK, collect FAIL. `grep -o 'no-agent|unverified|UNVERIFIED'` over the evidence returned 0 matches, so no no-agent/unverified collection was accepted as green; the collect cell is recorded FAIL and the board row .coding-hermes/board/tasks.jsonl:205 is not closed green (status in_progress, review_notes COLLECT-FAIL no-agent). Independent verification recorded at .gitreins/history/2026-10-25/7183b418/verdict.json (tier2 PASS with per-part detail; tier1 FAIL only on the unrelated secrets guard for .gitreins/** scope). Test evidence: `go test -count=1 ./...` exit_code=0, all 12 packages 'ok' (cmd/boardctl 6.163s, internal/board 0.804s, internal/vulncheck 3.288s).
The product-scoped releng battery ran against the boardctl git root at HEAD 2f97c81, produced 22 per-cell verdicts with zero no-agent/unverified markers, and the row was not accepted as green on unverified collection; go test -count=1 ./... passes (exit 0).

Overall: FAIL ✗
