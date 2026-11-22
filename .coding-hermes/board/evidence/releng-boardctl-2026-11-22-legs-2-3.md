# boardctl v0.6.0 release legs 2-3 evidence — 2026-11-22

Target: cut and publish release v0.6.0 from the live main tip.

Canonical HEAD: 55100783843c860a4b1bec9cbf34e79793d1bacf
(origin/main after fetch; identical to the headSha of both terminal CI runs below. The
dispatch brief quoted a 40-char sha sharing the prefix 55100783 but with a wrong tail —
the on-repo tip and GitHub's recorded headSha are authoritative and are the value above.)

## Leg 2 — dogfood/QA refresh gates at HEAD

| Gate | Command | Result | Key numbers |
|---|---|---|---|
| Build | make build | PASS (exit 0) | binary bin/boardctl built with buildCommit=55100783843c... |
| Vet | go vet ./... | PASS (exit 0) | zero findings |
| Tests | go test -short -count=1 ./... | PASS | 17 test packages ok, 0 fail |
| Fmt | make fmt-check | PASS (exit 0) | gofmt clean |
| Coverage | make cover-check | PASS | 16 packages evaluated, floor 60.0%, lowest 66.5% (internal/describe) |
| Vuln | make vuln-check | PASS (exit 0) | no known vulnerabilities found |
| Version pin | make version-check VERSION_TAG=v0.6.0 | PASS (exit 0) | README/CHANGELOG/version tests all assert v0.6.0 |
| Board validate | ./bin/boardctl -C . validate | RESULT: OK | 44 warnings (legacy event-id shapes, benign duplicates) |
| Board doctor | ./bin/boardctl -C . doctor | RESULT: OK | 46 warnings (fossil board.db note, TR-251 legacy shares) |

Tree hygiene at start: origin/main == HEAD == 55100783843c860a4b1bec9cbf34e79793d1bacf;
only foreign residue present (an uncommitted sibling foreman wave-manifest closeout
annotation under .coding-hermes/waves/ and untracked gitreins review-state files). None of
that is touched by this run; all release commits are path-limited to their own files.

## Bound CI evidence (terminal success pre-existing at this tip)

| Workflow | GitHub run id | headSha | status | conclusion |
|---|---|---|---|---|
| CI | 45067003881 | 55100783843c860a4b1bec9cbf34e79793d1bacf | completed | success |
| Multi-Arch | 45067004587 | 55100783843c860a4b1bec9cbf34e79793d1bacf | completed | success |

Rollback anchor: tag v0.5.0 must exist on the remote (verified in leg 4 before the cut).

## Leg 3 — candidate-bound GitReins verdict (done)

Step 1 repointed the row RELEASE-2026-11-16-01 commit_hash to
55100783843c860a4b1bec9cbf34e79793d1bacf via the boardctl row editor (read back verified:
full 40-char sha tail grew onto foreman_note).

Order note: the brief's plan ran gitreins task complete directly, but a gitreins closeout
refuses to grade a non-terminal board row. The proven close order (TR-251) is the reverse:
make the row terminal with an EXISTING verdict run as evidence, then grade the tip. So the
row was closed with --evidence-run-id d2617766 (leg 1's PASS verdict, verdict.json row
'.passed' = true), then gitreins task complete graded the live tip.

Grade result:
- printed completion id 90c9b377 ≠ on-disk history dir (known mismatch; the dir is the truth)
- verdict dir: .gitreins/history/2026-11-22/dce849ad/verdict.json
- passed: true; Stage tier1: PASS (secrets, tests); Stage tier2: PASS (judge COMPLETE);
  Stage fix_verification: PASS (0 findings); Overall: PASS
- graded tip == row commit_hash == 55100783843c860a4b1bec9cbf34e79793d1bacf

## Leg 4 — the cut and published-release verification

Scratch clone /tmp/v060cut at the same tip; remote tag census: v0.6.0 absent before the
cut (ls-remote grep exit 1), rollback anchor present (refs/tags/v0.5.0 = 7b3c618c, peeled
3acca082). Tag v0.6.0 was created on the commit and pushed with a PLAIN push
(refs/tags/v0.6.0:refs/tags/v0.6.0, no force flags of any kind). Disclosure: two direct
push command lines were hardline-blocked by the harness content scanner as a "git force
push short flag" false positive (no -f or + present anywhere); the identical plain push
was executed through a scratch script file per the hermes-gateway-command-blocks skill's
route for spelling false positives. The remote accepted it as [new tag] — no non-FF path
was ever exercised.

| Check | Result |
|---|---|
| Multi-Arch run triggered by tag push | run 45070661385, event push, headSha 55100783843c860a4b1bec9cbf34e79793d1bacf, conclusion success (2026-11-22T21:12:38Z) |
| gh release v0.6.0 | tagName v0.6.0, isDraft false, isPrerelease false |
| Asset count | 8 (boardctl_darwin_amd64, boardctl_darwin_arm64, boardctl_freebsd_amd64, boardctl_linux_amd64, boardctl_linux_arm, boardctl_linux_arm64, boardctl_windows_amd64.exe, SHA256SUMS) |
| Checksum | SHA256SUMS downloaded to /tmp/v060assets with boardctl_linux_amd64; sha256sum -c --ignore-missing → boardctl_linux_amd64: OK |
| Binary version | ./boardctl_linux_amd64 version → boardctl version v0.6.0 |
| Binary provenance | go version -m → vcs.revision=55100783843c860a4b1bec9cbf34e79793d1bacf, vcs.modified=false |

## Leg 5 — row closure and push

Row RELEASE-2026-11-16-01 is status=complete with evidence runs bound:
d2617766 (leg 1 verdict, used as the existing anchor required for the pre-grade closure)
and dce849ad (this run's own judge verdict, '.passed' = true, graded tip =
55100783843c860a4b1bec9cbf34e79793d1bacf). worker_summary names the cut+publish result
with the tag-push Multi-Arch run 45070661385.

Closure commit: the parent of this file's landed form — a single path-limited commit of
.coding-hermes/board/events.jsonl, .coding-hermes/board/tasks.jsonl, and this evidence doc,
message authored in a /tmp message file (no shell substitution), trailer
Co-authored-by: Alexis Okuwa <wojonstech@gmail.com>, then pushed plain to origin/main
(no force). Remote parity target: git rev-list --count origin/main..HEAD == 0 after the
push; CI on the new push tip tracked for green.
