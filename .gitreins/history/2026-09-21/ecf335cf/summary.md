# Verdict: BT-040

**Task:** Cut v0.1.7 release: move README pins off stale v0.1.6 asset and publish v0.1.7 with the 8 shipped fixes
**Evaluated:** 2026-09-21T01:53:30.489648
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	6.366s
- ✓ **tier2**
  - COMPLETE
  ✓ README.md Current-release line and all /releases/download/ URLs pin v0.1.7; git tag v0.1.7 pushed and CI release workflow green; downloaded asset boardctl_linux_amd64 passes sha256 -c, prints version v0.1.7, go version -m shows vcs.revision == v0.1.7 commit == origin/main and vcs.modified=false; behavior probe proves a shipped fix (usage-error newline formatting) differs from the v0.1.6 asset: README.md:17 'Current release: **v0.1.7**.'; README.md:20-21 both /releases/download/ URLs pin v0.1.7 (grep for v0.1.6 shows only historical mentions at 463/469). git tag v0.1.7 = f17253b9619d106ddad3eac727dfecb438e5ae00 == origin/main == HEAD; git ls-remote origin refs/tags/v0.1.7 = f17253b... (pushed). CI Multi-Arch run 35551898349 (tag v0.1.7) all jobs success incl. 'multiarch / Release'. Downloaded asset: sha256sum -c --ignore-missing SHA256SUMS => 'boardctl_linux_amd64: OK' exit=0; ./boardctl_linux_amd64 version => 'boardctl version v0.1.7'; go version -m => vcs.revision=f17253b9619d106ddad3eac727dfecb438e5ae00 (== v0.1.7 commit == origin/main), vcs.modified=false. Behavior probe: `create --bogus-flag` v0.1.6 stderr = '...[-C dir]\nboardctl: flag provided but not defined: -bogus-flag$' (literal backslash-n gluing exit line) vs v0.1.7 = '...[-C dir]$' newline 'boardctl: flag provided but not defined: -bogus-flag$'; identical divergence for `update --bogus-flag` (fix commit ac5442e DF-BOARDCTL-7).
v0.1.7 is fully cut: README pins, pushed tag, green CI release workflow, verified asset (checksum/version/vcs.revision==origin/main/modified=false), and the usage-error newline fix demonstrably differs from the v0.1.6 asset.

## Summary

Judge Result: BT-040

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	6.366s

Stage tier2: PASS
  COMPLETE
  ✓ README.md Current-release line and all /releases/download/ URLs pin v0.1.7; git tag v0.1.7 pushed and CI release workflow green; downloaded asset boardctl_linux_amd64 passes sha256 -c, prints version v0.1.7, go version -m shows vcs.revision == v0.1.7 commit == origin/main and vcs.modified=false; behavior probe proves a shipped fix (usage-error newline formatting) differs from the v0.1.6 asset: README.md:17 'Current release: **v0.1.7**.'; README.md:20-21 both /releases/download/ URLs pin v0.1.7 (grep for v0.1.6 shows only historical mentions at 463/469). git tag v0.1.7 = f17253b9619d106ddad3eac727dfecb438e5ae00 == origin/main == HEAD; git ls-remote origin refs/tags/v0.1.7 = f17253b... (pushed). CI Multi-Arch run 35551898349 (tag v0.1.7) all jobs success incl. 'multiarch / Release'. Downloaded asset: sha256sum -c --ignore-missing SHA256SUMS => 'boardctl_linux_amd64: OK' exit=0; ./boardctl_linux_amd64 version => 'boardctl version v0.1.7'; go version -m => vcs.revision=f17253b9619d106ddad3eac727dfecb438e5ae00 (== v0.1.7 commit == origin/main), vcs.modified=false. Behavior probe: `create --bogus-flag` v0.1.6 stderr = '...[-C dir]\nboardctl: flag provided but not defined: -bogus-flag$' (literal backslash-n gluing exit line) vs v0.1.7 = '...[-C dir]$' newline 'boardctl: flag provided but not defined: -bogus-flag$'; identical divergence for `update --bogus-flag` (fix commit ac5442e DF-BOARDCTL-7).
v0.1.7 is fully cut: README pins, pushed tag, green CI release workflow, verified asset (checksum/version/vcs.revision==origin/main/modified=false), and the usage-error newline fix demonstrably differs from the v0.1.6 asset.

Overall: PASS ✓
