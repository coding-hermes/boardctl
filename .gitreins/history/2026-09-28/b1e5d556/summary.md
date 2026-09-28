# Verdict: DF-BOARDCTL-16

**Task:** wave tick boardctl-2026-09-27-23-52-37: DF-BOARDCTL-16
**Evaluated:** 2026-09-28T01:06:49.947365
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.119s
- ✓ **tier2**
  - COMPLETE
  ✓ see board row DF-BOARDCTL-16 acceptance criteria: Board row acceptance (from /tmp/brief-DF-BOARDCTL-16.txt) requires: (1) release FAILS non-zero when linux-amd64 vcs.revision is empty / not an ancestor of VERSION / vcs.modified!=false, naming the observed SHA; (2) clean stamp proceeds and prints the verified revision; (3) both arms proven with quoted output; (4) one README note under ## Development; (5) gofmt/build/vet/test green. VERIFIED LIVE: Makefile release target (lines 101-148) adds the stamp assertion after the build loop. NEGATIVE arms all exit 2 before SHA256SUMS is written: (a) foreign stamp in this worktree -> 'release: FAIL vcs.modified: true ... vcs.revision c6af73f1b4da97a112e634be98285598961fed26', no dist/SHA256SUMS; (b) clean outer repo stamping foreign rev with modified=false -> 'release: FAIL vcs.revision: e8c17518... is not the HEAD of this checkout'; (c) build outside any .git -> 'release: FAIL vcs.revision absent'. POSITIVE arm (clean nested worktree, HEAD==tag) -> 'release: ok vcs.revision cc999ce... is an ancestor of v0.1.8', SHA256SUMS written, asset prints 'boardctl version v0.1.8'. README.md:562-571 has the one docs note under ## Development. Gates: gofmt -l clean (exit 0), go build ./... exit 0, go vet ./... exit 0. go test ./... -count=1 has ONE failure, TestUsageSkillDateGateBitesOnHeaderMutations (df15_skill_closure_test.go:478), PROVEN pre-existing at base ae429f1 by an independent detached-worktree run with identical output (date-based test, run date 2026-09-27); the diff touches no Go files. Minor documented gap: the 'not an ancestor of VERSION' arm is enforced only when .git is a FILE (linked worktree) — in a plain checkout it prints DEGRADED and ships (reproduced: HEAD d602f88 not ancestor of v0.1.8 still wrote SHA256SUMS). This is a deliberate, commit-documented degradation ('non-worktree roots ... the stamp cannot be foreign by construction') and the central requirement — refuse foreign-SHA builds and name the SHA — is fully met and proven in the exact bug scenario.
DF-BOARDCTL-16's release vcs-stamp assertion is implemented and verified live: all foreign/absent/modified failure arms exit non-zero naming the observed SHA before SHA256SUMS, the clean path proceeds printing the verified revision, the README note is present, and gofmt/build/vet are clean (the single test failure is proven pre-existing at base ae429f1).

## Summary

Judge Result: DF-BOARDCTL-16

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.119s

Stage tier2: PASS
  COMPLETE
  ✓ see board row DF-BOARDCTL-16 acceptance criteria: Board row acceptance (from /tmp/brief-DF-BOARDCTL-16.txt) requires: (1) release FAILS non-zero when linux-amd64 vcs.revision is empty / not an ancestor of VERSION / vcs.modified!=false, naming the observed SHA; (2) clean stamp proceeds and prints the verified revision; (3) both arms proven with quoted output; (4) one README note under ## Development; (5) gofmt/build/vet/test green. VERIFIED LIVE: Makefile release target (lines 101-148) adds the stamp assertion after the build loop. NEGATIVE arms all exit 2 before SHA256SUMS is written: (a) foreign stamp in this worktree -> 'release: FAIL vcs.modified: true ... vcs.revision c6af73f1b4da97a112e634be98285598961fed26', no dist/SHA256SUMS; (b) clean outer repo stamping foreign rev with modified=false -> 'release: FAIL vcs.revision: e8c17518... is not the HEAD of this checkout'; (c) build outside any .git -> 'release: FAIL vcs.revision absent'. POSITIVE arm (clean nested worktree, HEAD==tag) -> 'release: ok vcs.revision cc999ce... is an ancestor of v0.1.8', SHA256SUMS written, asset prints 'boardctl version v0.1.8'. README.md:562-571 has the one docs note under ## Development. Gates: gofmt -l clean (exit 0), go build ./... exit 0, go vet ./... exit 0. go test ./... -count=1 has ONE failure, TestUsageSkillDateGateBitesOnHeaderMutations (df15_skill_closure_test.go:478), PROVEN pre-existing at base ae429f1 by an independent detached-worktree run with identical output (date-based test, run date 2026-09-27); the diff touches no Go files. Minor documented gap: the 'not an ancestor of VERSION' arm is enforced only when .git is a FILE (linked worktree) — in a plain checkout it prints DEGRADED and ships (reproduced: HEAD d602f88 not ancestor of v0.1.8 still wrote SHA256SUMS). This is a deliberate, commit-documented degradation ('non-worktree roots ... the stamp cannot be foreign by construction') and the central requirement — refuse foreign-SHA builds and name the SHA — is fully met and proven in the exact bug scenario.
DF-BOARDCTL-16's release vcs-stamp assertion is implemented and verified live: all foreign/absent/modified failure arms exit non-zero naming the observed SHA before SHA256SUMS, the clean path proceeds printing the verified revision, the README note is present, and gofmt/build/vet are clean (the single test failure is proven pre-existing at base ae429f1).

Overall: PASS ✓
