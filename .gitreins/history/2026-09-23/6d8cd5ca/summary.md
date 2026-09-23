# Verdict: DF-BOARDCTL-11

**Task:** README fingerprint section: --evidence-run-id is create-only
**Evaluated:** 2026-09-23T10:34:19.470893
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.633s
- ✓ **tier2**
  - COMPLETE
  ✓ README.md fingerprint section (~lines 276-290) says to evidence an existing row with --evidence-run-id without saying the flag exists only on CREATE (update rejects it: 'flag provided but not defined'), and never names the exit-2-vs-exit-1 distinction it implies. Fix the README text: state the flag is create-only, that evidence lands via boardctl update on an existing row, and spell out exit 2 = duplicate-suppressed re-detection vs exit 1 = plain failure. Add a README-consistency test if the repo has one; gates green (gofmt irrelevant but build/test must pass).: README.md:296-300 now states 'Evidence the EXISTING row with `boardctl update <id>` — carry the run id in `--summary` or `--note`; `update` does not accept `--evidence-run-id`. That flag exists ONLY on `create`'. README.md:283-291 adds the exit-code table: 'exit 1 plain failure (validation, duplicate task id, runtime errors)' and 'exit 2 duplicate-suppressed re-detection: DUPLICATE-SUPPRESSED on stderr plus a task_evidence event on the existing row'. Verified against code: cmd/boardctl/main.go:526 defines --evidence-run-id only in cmdCreate; cmdUpdate (main.go:597-599) valueFlags list omits it; main.go:160-163 maps ErrDuplicateFinding->exit 2, main.go:152-155 ErrDuplicateTaskID->exit 1, main.go:166 fallback->exit 1. E2E with built binary: `update QA-X-1 --evidence-run-id qa-1` => 'flag provided but not defined: -evidence-run-id' exit 1; duplicate create => 'DUPLICATE-SUPPRESSED: QA-X-1 matches fingerprint ...' exit 2; `update QA-X-1 --summary ...` => exit 0. docs/board-fingerprint-rules.md:80-86 also updated to say create-only. Gates: `go build ./...` exit 0; `go test -count=1 ./...` exit 0 (ok cmd/boardctl, internal/board, internal/fmtcheck, internal/render, internal/versioncheck, internal/vulncheck). Repo has README-consistency tests (internal/versioncheck for release tags, internal/vulncheck for `make vuln-check`) but none covering the fingerprint section, so there was no existing fingerprint-section consistency test to extend; the conditional 'if the repo has one' is satisfied.
README fingerprint section now correctly documents --evidence-run-id as create-only, evidence via boardctl update on existing rows, and the exit-2-vs-exit-1 distinction, all verified against code and passing build/tests.

## Summary

Judge Result: DF-BOARDCTL-11

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.633s

Stage tier2: PASS
  COMPLETE
  ✓ README.md fingerprint section (~lines 276-290) says to evidence an existing row with --evidence-run-id without saying the flag exists only on CREATE (update rejects it: 'flag provided but not defined'), and never names the exit-2-vs-exit-1 distinction it implies. Fix the README text: state the flag is create-only, that evidence lands via boardctl update on an existing row, and spell out exit 2 = duplicate-suppressed re-detection vs exit 1 = plain failure. Add a README-consistency test if the repo has one; gates green (gofmt irrelevant but build/test must pass).: README.md:296-300 now states 'Evidence the EXISTING row with `boardctl update <id>` — carry the run id in `--summary` or `--note`; `update` does not accept `--evidence-run-id`. That flag exists ONLY on `create`'. README.md:283-291 adds the exit-code table: 'exit 1 plain failure (validation, duplicate task id, runtime errors)' and 'exit 2 duplicate-suppressed re-detection: DUPLICATE-SUPPRESSED on stderr plus a task_evidence event on the existing row'. Verified against code: cmd/boardctl/main.go:526 defines --evidence-run-id only in cmdCreate; cmdUpdate (main.go:597-599) valueFlags list omits it; main.go:160-163 maps ErrDuplicateFinding->exit 2, main.go:152-155 ErrDuplicateTaskID->exit 1, main.go:166 fallback->exit 1. E2E with built binary: `update QA-X-1 --evidence-run-id qa-1` => 'flag provided but not defined: -evidence-run-id' exit 1; duplicate create => 'DUPLICATE-SUPPRESSED: QA-X-1 matches fingerprint ...' exit 2; `update QA-X-1 --summary ...` => exit 0. docs/board-fingerprint-rules.md:80-86 also updated to say create-only. Gates: `go build ./...` exit 0; `go test -count=1 ./...` exit 0 (ok cmd/boardctl, internal/board, internal/fmtcheck, internal/render, internal/versioncheck, internal/vulncheck). Repo has README-consistency tests (internal/versioncheck for release tags, internal/vulncheck for `make vuln-check`) but none covering the fingerprint section, so there was no existing fingerprint-section consistency test to extend; the conditional 'if the repo has one' is satisfied.
README fingerprint section now correctly documents --evidence-run-id as create-only, evidence via boardctl update on existing rows, and the exit-2-vs-exit-1 distinction, all verified against code and passing build/tests.

Overall: PASS ✓
