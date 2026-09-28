# Run 17 — the quality-gate checker family, attacked with tamper probes (2026-09-28)

**Lane:** boardctl-dogfood · **Repo HEAD under test:** `9307bfa` (v0.1.9 cut)
**Previous runs:** 09-04, 09-12, 09-20, 09-22, 09-24 (run 15), 09-27 (run 16) — none
exercised `internal/{fmtcheck,versioncheck,speccheck,workflowcheck,vulncheck,freshness}`
or `cmd/gendocs` as a USER. That is this run's angle.

## Promise under test

> "The Makefile, CI, and `go test ./...` all enforce the exact same rule over
> the exact same file set" — every gate in this repo is a check-only Go
> package shared by three enforcement surfaces, so drift cannot ship.

A promise about *enforcement* can only be tested by trying to sneak drift past
each surface. So: scratch clone at `/tmp/dogfood-boardctl` (deleted after the
run), real tamper probes, every result recorded with exit codes.

## The tamper matrix (all on v0.1.9 at HEAD)

| Arm | Tamper | Expected | Result |
|---|---|---|---|
| fmtcheck | append unformatted decl to `internal/freshness/freshness.go` | make fmt-check FAIL + `go test ./...` FAIL, file named | ✅ `found 1 file(s) not gofmt-clean — run gofmt -w ... on: internal/freshness/freshness.go`, rc=2/1; CI-equivalent command = same checker |
| versioncheck (agreement) | README pin v0.1.9→v0.1.8 (now disagrees with its own URLs) | FAIL naming both sides | ✅ stale pin line + 2 stale download URLs quoted |
| versioncheck (currency) | README internally consistent, `VERSION_TAG=v0.1.10` | FAIL: README stale vs candidate tag | ✅ "stale release pin line #1 / stale download URL #1/#2" — the DF-BOARDCTL-17 fix works as documented |
| versioncheck (clean control) | `VERSION_TAG=v0.1.9` (current) | exit 0 | ✅ 0 |
| speccheck (out of contract) | edit an operation `summary` | no failure (parity scope = routes/ids/servers) | ✅ passes — scope matches its doc comment; **not** a finding |
| speccheck (operationId) | rename `getUploaderForm` | FAIL naming both sides + regen hint | ✅ exact message incl. `go run ./cmd/gendocs` |
| speccheck (route) | delete the whole `'/api/boards'` path block | FAIL: wired but missing from spec | ✅ "route GET /api/boards is wired in serve.go but missing from openapi.yaml — regenerate" |
| workflowcheck | swap `freebsd/amd64`→`linux/riscv64` in multiarch.yml platforms | FAIL with both lists | ✅ "platform drift: workflow [...] != Makefile [...] (order counts)" + fix line |
| vulncheck (broken scan) | `GOVULNCHECK=<stub exit 1>` | TOOL-ERROR, never a pass | ✅ "exited with code 1, which is neither 0 (clean) nor 3 (findings)" |
| vulncheck (findings) | `GOVULNCHECK=<stub exit 3>` | FAIL verdict | ✅ "vulncheck: FAIL — 0 reachable vulnerabilities found (exit 3)" |
| freshness (BT-057) | `make build` at HEAD, advance HEAD one empty commit, re-run | silent at HEAD; stderr warning when stale | ✅ silent first; then "binary built from 9307bfab (built 2026-09-28) is 1 commit behind HEAD (d93b0aab) — rebuild or BOARDCTL_SKIP_FRESHNESS=1" |
| gendocs | regenerate the published spec at HEAD | byte-identical (deterministic) | ✅ `git diff` empty |

## What did NOT hold

**DF-BOARDCTL-19 (P1): the "three surfaces" doctrine is asserted, not
enforced — CI steps can be deleted and every gate stays green.** The gates'
doc comments all claim "shared by three enforcement surfaces (kept in
agreement by construction)". Only *construction* holds: Makefile targets and
`go test ./...` both invoke the checker code, so they cannot drift. The CI
workflow is just YAML text next to the code — nothing proves it still wires
the steps. Proven by deleting the `Gofmt`, `Version-check`, and `Spec-check`
steps from `.github/workflows/ci.yml`: `go test -short ./...` exit 0, all
three `make` targets exit 0. A CI config edit (or a workflow-refactor merge)
silently downgrades those gates to local-only, and no test, gate, or doctor
check notices. Contrast that proves the gap is real and not philosophical:
`internal/vulncheck` DOES self-assert its CI wiring (its test fails when
ci.yml no longer installs govulncheck@v1.7.0), and workflowcheck parses the
workflow — but for fmtcheck/versioncheck/speccheck the CI step is unprotected.
Fix direction: a small step-presence test per gate package (same pattern as
vulncheck's ci.yml assertion), or extend workflowcheck to assert each gate's
step string exists in ci.yml.

**DF-BOARDCTL-20 (P3): `vuln-check` runs whatever `govulncheck` is on PATH —
only CI checks the version.** The gate pins `govulncheck@v1.7.0` and
`InstallHint` names it, and the pin is enforced against ci.yml's install step —
but a local `make vuln-check` never verifies the resolved binary's version
(govulncheck has a `-version` flag; the scanner self-reported `govulncheck@v1.7.0`
here, so this host passes). A developer with an older binary gets a green gate
against a different rule set. Fix direction: one `bin -version` probe in the
gate (warn or fail), consistent with the "gate agrees with itself" doctrine.

**DF-BOARDCTL-13 closed and verified:** run 15's half-open `update --title` gap
is shipped at HEAD — `update DF-BOARDCTL-13 --title` exists, and validate now
warns on title-Pn/priority-field disagreement (4 live warnings on this repo's
own board, e.g. DF-BOARDCTL-13 line 81 — the warnings work; the rows just
haven't been canonicalized yet).

## Fresh-machine install leg (bunker-las-03, agent affa0eaf, destroyed + key removed)

README quickstart verbatim (release-asset path, no Go toolchain): download
v0.1.9 asset + SHA256SUMS, `sha256sum -c --ignore-missing` → `OK`,
**INSTALL_SECONDS=4**. Smoke on the repo's own board transferred to the fresh
box: stats 109 tasks, validate `OK (4 warning(s))` — same 4 warnings as the
source box, doctor `OK (5 warning(s))`, then a real write probe:
`create RUN17-PROBE-1` → `update --status complete --guard PASS --ci GREEN` →
`show` returns the row, validate still OK. The write path works first-try on a
bare Debian box with zero dependencies.

Transfer trap worth recording (not a boardctl defect): the per-file ssh `cat`
relay needs `< /dev/null` on the sibling `mkdir` ssh when driven from a
`while read` loop — without it the mkdir ssh eats the loop's stdin (loop died
after 1 file), and with `ssh -n` on the relay instead, every file lands 0 bytes
while the counter says ok=447. Verify transfers with a sha256 census, never
the loop counter: run 1 produced 447 "successful" 0-byte files.

## Performance (measured, this run, v0.1.9 build, 109-task board)

stats 10.0ms · show 9.3ms · validate 14.4ms · doctor 24.6ms · list --json
12.7ms (warm medians, n=20/10; first-runs within 3ms of warm). Gate suite:
fmtcheck 0.07s, versioncheck 0.003s, speccheck 0.013s, workflowcheck 0.43s,
vulncheck 2.77s (network scan), `go test -short ./...` 6.75s total. **No PERF
rows — nothing here is slow enough for a user to feel.**

## Verdict

**SHIPPABLE** for the gate family: every tamper that the gates' contracts
claim to catch was caught, with error messages that name the drifted value,
both sides, and the fix command. The P1 gap is one level up — the enforcement
*of the enforcement* (CI wiring) is real for vulncheck and platform parity,
and absent for the other three gates. Time-to-first-success on a fresh
machine: 4 seconds.
