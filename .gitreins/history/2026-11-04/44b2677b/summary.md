# Verdict: BT-087

**Task:** Vendored golangci config hijacks go_lint — root config + vendor-config gate
**Evaluated:** 2026-11-04T18:29:35.985845
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ go_lint must exit 0 on a commit touching vendor Go files; a checker must fail if any vendor/**/.golangci* exists; full gate set green; staged kin-openapi bump landed: (a) go_lint on vendor files: `golangci-lint run --new-from-rev=HEAD~1 vendor/github.com/go-openapi/jsonpointer/...` -> EXIT_VENDOR=0, "0 issues."; also `vendor/...` -> 0, `./...` -> 0, `./internal/board` -> 0, `./internal/vendorcheck` -> 0 (golangci-lint 2.12.2). Root .golangci.yml (version "2", linters.exclusions.paths: vendor/) pins config discovery to the root. (b) Gate is RED-capable: planting vendor/github.com/go-openapi/jsonpointer/.golangci.yml made `go test -count=1 -run TestVendorcheck ./internal/vendorcheck` exit 1 with "found 1 golangci config file(s) under vendor/ ... vendor/github.com/go-openapi/jsonpointer/.golangci.yml"; after removal it is green (ok 0.002s). Checker at internal/vendorcheck/vendorcheck.go (FindVendoredConfigs walks vendor/ matching ".golangci" prefix, sorted, errors on missing vendor/), tests in vendorcheck_test.go (planted fixture, clean vendor, missing-vendor error), wired at Makefile:101-102 (vendor-check), .github/workflows/ci.yml:42 (Vendor-check step), and pinned by the workflowcheck gate-step census (TestWorkflowcheck exit 0). (c) Full gate set green: go build ./... EXIT 0; go vet ./... EXIT 0; go test -short -count=1 ./... EXIT 0 (16 ok); go test -count=1 ./... EXIT 0 (16 ok); TestGofmt EXIT 0; TestVersioncheck EXIT 0; speccheck EXIT 0; TestVulncheck EXIT 0; make cover-check EXIT 0 ("covercheck: PASS: 15 packages evaluated", total 83.1%); go mod verify "all modules verified"; go build -mod=vendor ./... EXIT 0. (d) kin-openapi bump landed: go.mod at HEAD requires github.com/getkin/kin-openapi v0.152.0 (HEAD~1: v0.149.0) plus jsonpointer v1.0.2, oasdiff/yaml v0.1.3, yaml3 v0.0.16, x/text v0.43.0; vendor/modules.txt agrees; vendor tree present and builds; working tree clean (git status empty).
Root .golangci.yml pins lint config discovery so go_lint exits 0 on vendor-touching commits, the new internal/vendorcheck gate demonstrably fails when a vendor/**/.golangci* is planted, all gates (build/vet/tests/coverage/vuln/workflowcheck) are green, and the kin-openapi v0.152.0 bump is committed.

## Summary

Judge Result: BT-087

Stage tier1: PASS
    ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ go_lint must exit 0 on a commit touching vendor Go files; a checker must fail if any vendor/**/.golangci* exists; full gate set green; staged kin-openapi bump landed: (a) go_lint on vendor files: `golangci-lint run --new-from-rev=HEAD~1 vendor/github.com/go-openapi/jsonpointer/...` -> EXIT_VENDOR=0, "0 issues."; also `vendor/...` -> 0, `./...` -> 0, `./internal/board` -> 0, `./internal/vendorcheck` -> 0 (golangci-lint 2.12.2). Root .golangci.yml (version "2", linters.exclusions.paths: vendor/) pins config discovery to the root. (b) Gate is RED-capable: planting vendor/github.com/go-openapi/jsonpointer/.golangci.yml made `go test -count=1 -run TestVendorcheck ./internal/vendorcheck` exit 1 with "found 1 golangci config file(s) under vendor/ ... vendor/github.com/go-openapi/jsonpointer/.golangci.yml"; after removal it is green (ok 0.002s). Checker at internal/vendorcheck/vendorcheck.go (FindVendoredConfigs walks vendor/ matching ".golangci" prefix, sorted, errors on missing vendor/), tests in vendorcheck_test.go (planted fixture, clean vendor, missing-vendor error), wired at Makefile:101-102 (vendor-check), .github/workflows/ci.yml:42 (Vendor-check step), and pinned by the workflowcheck gate-step census (TestWorkflowcheck exit 0). (c) Full gate set green: go build ./... EXIT 0; go vet ./... EXIT 0; go test -short -count=1 ./... EXIT 0 (16 ok); go test -count=1 ./... EXIT 0 (16 ok); TestGofmt EXIT 0; TestVersioncheck EXIT 0; speccheck EXIT 0; TestVulncheck EXIT 0; make cover-check EXIT 0 ("covercheck: PASS: 15 packages evaluated", total 83.1%); go mod verify "all modules verified"; go build -mod=vendor ./... EXIT 0. (d) kin-openapi bump landed: go.mod at HEAD requires github.com/getkin/kin-openapi v0.152.0 (HEAD~1: v0.149.0) plus jsonpointer v1.0.2, oasdiff/yaml v0.1.3, yaml3 v0.0.16, x/text v0.43.0; vendor/modules.txt agrees; vendor tree present and builds; working tree clean (git status empty).
Root .golangci.yml pins lint config discovery so go_lint exits 0 on vendor-touching commits, the new internal/vendorcheck gate demonstrably fails when a vendor/**/.golangci* is planted, all gates (build/vet/tests/coverage/vuln/workflowcheck) are green, and the kin-openapi v0.152.0 bump is committed.

Overall: PASS ✓
