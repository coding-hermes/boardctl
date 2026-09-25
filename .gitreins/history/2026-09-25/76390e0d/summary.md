# Verdict: BT-RELEASE-0904

**Task:** create --help documents the field flags
**Evaluated:** 2026-09-25T16:46:28.146280
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.608s
- ✓ **tier2**
  - COMPLETE
  ✓ cmd/boardctl/main.go create usage closure lists --status/--priority/--complexity/--depends-on/--reasoning/--capability-tags/--evidence-run-id; new df_boardctl_7_usage_test.go assertion fails on the old string; build/vet/test -short green; diff limited to main.go usage string + new test: main.go:613 create usage closure now prints all seven flags: "boardctl create --id ID --title T [--status S] [--priority P] [--complexity N] [--depends-on a,b] [--reasoning R] [--capability-tags tags] [--evidence-run-id id] [--worktree PATH]...". New TestCreateHelpListsFieldFlags (df_boardctl_7_usage_test.go:387-401) asserts all seven flags via run(["create","--help"]). RED verified: reverting the usage string to the old text makes the test FAIL with 'create --help usage output missing "--priority"' etc. GREEN verified: go test -short -count=1 -run TestCreateHelpListsFieldFlags => PASS. go build ./... exit 0; go vet ./cmd/boardctl exit 0; go test -short -count=1 ./cmd/boardctl => 'ok github.com/coding-hermes/boardctl/cmd/boardctl 0.807s' exit 0. Diff (git show HEAD --stat) limited to cmd/boardctl/main.go (2 lines, 1 changed) + cmd/boardctl/df_boardctl_7_usage_test.go (27 insertions).
create --help usage now documents all seven field flags, the new test is load-bearing (RED on old string, GREEN after fix), build/vet/test -short are green, and the diff is limited to the usage string plus the new test.

## Summary

Judge Result: BT-RELEASE-0904

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.608s

Stage tier2: PASS
  COMPLETE
  ✓ cmd/boardctl/main.go create usage closure lists --status/--priority/--complexity/--depends-on/--reasoning/--capability-tags/--evidence-run-id; new df_boardctl_7_usage_test.go assertion fails on the old string; build/vet/test -short green; diff limited to main.go usage string + new test: main.go:613 create usage closure now prints all seven flags: "boardctl create --id ID --title T [--status S] [--priority P] [--complexity N] [--depends-on a,b] [--reasoning R] [--capability-tags tags] [--evidence-run-id id] [--worktree PATH]...". New TestCreateHelpListsFieldFlags (df_boardctl_7_usage_test.go:387-401) asserts all seven flags via run(["create","--help"]). RED verified: reverting the usage string to the old text makes the test FAIL with 'create --help usage output missing "--priority"' etc. GREEN verified: go test -short -count=1 -run TestCreateHelpListsFieldFlags => PASS. go build ./... exit 0; go vet ./cmd/boardctl exit 0; go test -short -count=1 ./cmd/boardctl => 'ok github.com/coding-hermes/boardctl/cmd/boardctl 0.807s' exit 0. Diff (git show HEAD --stat) limited to cmd/boardctl/main.go (2 lines, 1 changed) + cmd/boardctl/df_boardctl_7_usage_test.go (27 insertions).
create --help usage now documents all seven field flags, the new test is load-bearing (RED on old string, GREEN after fix), build/vet/test -short are green, and the diff is limited to the usage string plus the new test.

Overall: PASS ✓
