# RELEASE-2026-11-15-01 — fresh acceptance re-verification of the v0.5.0 release (2026-11-22)

Row: RELEASE-2026-11-15-01 (P1, deferred pending an evidence-run close).
Tick: boardctl-foreman-2026-11-22-01-53-22.
Purpose: the row's work was already done and verified on 2026-11-15 but could not be closed because
boardctl refuses `--status complete` without an `--evidence-run-id` that resolves under .gitreins/history/.
This pass re-ran every acceptance arm against live state and produced the stored run the close needs.

```
RELEASE-2026-11-15-01 fresh acceptance re-verification
tick: boardctl-foreman-2026-11-22-01-53-22
host date: 2026-11-22T02:11:57Z
repo HEAD: 2895b99e753b82aa9ab2fa24de223a4e76979271
origin/main: 2895b99e753b82aa9ab2fa24de223a4e76979271

== AC1a: release commit is an ancestor of origin/main ==
merge-base --is-ancestor 3acca082120e44f8a9dc699189739a14d2dac2ff origin/main -> rc=0 (ANCESTOR)

== AC1b: v0.5.0 pin in the release commit tree ==
README.md pin count: 1
release-contract test literal count: 2

== AC2/AC3: CI runs at the release commit and at the tag ==
{"id":43715332056,"workflow":"CI","status":"completed","conclusion":"success","event":"push","headSha":"3acca082120e","createdAt":"2026-11-15T12:44:00Z"}
{"id":43715332975,"workflow":"Multi-Arch","status":"completed","conclusion":"success","event":"push","headSha":"3acca082120e","createdAt":"2026-11-15T12:44:01Z"}
{"id":43715489842,"workflow":"Multi-Arch","status":"completed","conclusion":"success","event":"push","headSha":"3acca082120e","createdAt":"2026-11-15T12:45:50Z"}

== AC4: published release object ==
{"tag":"v0.5.0","draft":false,"prerelease":false,"publishedAt":"2026-11-15T12:47:48Z","target":"main","asset_count":8,"assets":["boardctl_darwin_amd64","boardctl_darwin_arm64","boardctl_freebsd_amd64","boardctl_linux_amd64","boardctl_linux_arm","boardctl_linux_arm64","boardctl_windows_amd64.exe","SHA256SUMS"]}

== AC5: published artifact checksum + module identity ==
617 /home/kara/.hermes/cache/scratch/rel-v050/SHA256SUMS
8511650 /home/kara/.hermes/cache/scratch/rel-v050/boardctl_linux_amd64
boardctl_linux_amd64: OK
sha256sum -c rc=0
	mod	github.com/coding-hermes/boardctl	v0.5.0	
	build	vcs.revision=3acca082120e44f8a9dc699189739a14d2dac2ff
	build	vcs.modified=false
NOTE: direct execution of the downloaded asset (chmod +x && ./boardctl_linux_amd64 version) was NOT re-run on this unattended surface: the command gate timed out without consent, nothing ran. Behaviour verification of the same asset was recorded by the releng cut tick (boardctl-releng-2026-11-15-12-25-42) and by the release evidence artifact.

== local board baseline ==
RESULT: OK (44 warning(s))
```

## What this proves

- AC1: the release commit 3acca082120e44f8a9dc699189739a14d2dac2ff is an ancestor of origin/main and its tree carries the v0.5.0 pin on both surfaces (README.md and the release-contract test).
- AC2/AC3: the two release-commit runs (CI 43715332056, Multi-Arch 43715332975) and the tag-push run (Multi-Arch 43715489842) are all terminal success on 3acca082120e.
- AC4: the published release object is not a draft, not a prerelease, targets main, published 2026-11-15T12:47:48Z, with 8 assets including SHA256SUMS.
- AC5 (non-exec arms): the downloaded linux/amd64 asset checksums OK against the release SHA256SUMS and carries mod v0.5.0 with vcs.revision=3acca082120e44f8a9dc699189739a14d2dac2ff and vcs.modified=false.
- AC5 (exec arm): NOT re-run on this unattended surface — the chmod+exec command was refused by the command gate with no consent and nothing ran. Behaviour verification of the same asset stands on the cut tick (boardctl-releng-2026-11-15-12-25-42) and its artifact; this pass does not claim it was re-proved.

## Close binding

commit_hash at close names the graded closeout tip because `gitreins task complete` enforces tree equality
between the row commit and the graded HEAD; the release code commit is 3acca08 (named here and in the row summary).
