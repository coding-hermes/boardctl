# RELEASE-2026-11-11-01 verification

Tick: boardctl-foreman-2026-11-14-23-06-53
Tested published commit: f243e0d90ac947e3ea484b16fe4b0f742a57a6a8
Origin parity: 0 ahead / 0 behind

## Direct gates

All exited 0 in a detached worktree at origin/main:

go test -count=1 -short ./...
make vuln-check
make cover-check: total 80.2%, floor 80.0%; lowest package 66.5%, floor 60.0%
make version-check VERSION_TAG=v0.4.0
make vendor-check
make spec-check
make fmt-check
go vet ./...

## External readback

Exact-head CI for f243e0d:
43651137157 — completed/success
43651136344 — completed/success

v0.4.0 release:
Non-draft, non-prerelease; published 2026-11-11T10:59:10Z; 8 assets: 7 platform binaries plus SHA256SUMS.

Existing implementation evidence:
9ebf7ad is the GO-2026-6859/toolchain fix commit.
CI 43009506833 completed/success on 3db2a70 with Vulnerability-check, Coverage, and Test successful.
Multi-Arch 43007841138 completed/success on 9ebf7ad, including the Release job.

Disposition:
The row's repository and release acceptance is satisfied by landed implementation plus independent gates. It remains pending because the only available GitReins artifact, .gitreins/history/2026-11-11/8907c118/verdict.json, is overall passed=false: Tier 2 passed, but Tier 1 failed on inherited audit-transcript secret findings. This verification does not promote that split to an overall PASS and does not create a replacement lifecycle task.

ch:trace row=RELEASE-2026-11-11-01 spec=.coding-hermes/board/tasks.jsonl#RELEASE-2026-11-11-01 wave=none:verification-only test=none:landed-drift-reverification doc=.coding-hermes/board/evidence/boardctl-foreman-2026-11-14-23-06-53.md evidence=.coding-hermes/board/evidence/boardctl-foreman-2026-11-14-23-06-53.md witness=CI:43651137157,43651136344 verdict=none:tier1-inherited-secret-baseline-split commit=f243e0d memory=none:not-written
STATUS: partial
TIER: T3
EVIDENCE: .coding-hermes/board/evidence/boardctl-foreman-2026-11-14-23-06-53.md
