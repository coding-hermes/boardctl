# Verdict: DF-BOARDCTL-8

**Task:** Burndown renders 2 of 20 points — per-day labels missing
**Evaluated:** 2026-09-23T05:39:37.745155
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.069s
- ✓ **tier2**
  - COMPLETE
  ✓ BurndownSeries carries a per-day Days array matching Open's length; template passes it as x-axis at both chart sites; go build/vet/test green with a payload-shape test asserting len(days)==len(open)>2: internal/render/derive.go:43 adds `Days []string `json:"days,omitempty"`` to BurndownSeries; derive.go:310 `burndown()` sets `Days: window` and appends one Open value per window day, so len(Days)==len(Open). internal/render/template.go:714 (separate box) and :725 (overlay) both feed `days: d.burndown.days || []`; grep confirms no remaining `d.burndown.window` feed in template.go. Payload-shape tests: internal/render/derive_test.go:107 TestBurndownDaysMatchOpenPoints asserts len(Days)!=len(Open) fails and len(Days)<=2 fails (i.e. ==open and >2); cmd/boardctl/render_test.go TestRenderBurndownUsesDays asserts payload days==open, days>2, days[0]==window[0], `strings.Count(s, "days: d.burndown.days || []") == 2`, and rejects `days: d.burndown.window`. Gate run fresh: `go build ./...` exit 0, `go vet ./...` exit 0, `go test -count=1 ./...` exit 0 with `ok github.com/coding-hermes/boardctl/cmd/boardctl`, `ok .../internal/render`, `ok .../internal/board`, etc.; targeted `go test -count=1 -run 'TestBurndownDaysMatchOpenPoints|TestPayloadRoundTripAndShapes|TestRenderBurndownUsesDays' -v` → all PASS.
BurndownSeries.Days is populated per-day and fed as the x-axis at both template chart sites, with payload-shape tests asserting len(days)==len(open)>2 and a green build/vet/test gate.

## Summary

Judge Result: DF-BOARDCTL-8

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.069s

Stage tier2: PASS
  COMPLETE
  ✓ BurndownSeries carries a per-day Days array matching Open's length; template passes it as x-axis at both chart sites; go build/vet/test green with a payload-shape test asserting len(days)==len(open)>2: internal/render/derive.go:43 adds `Days []string `json:"days,omitempty"`` to BurndownSeries; derive.go:310 `burndown()` sets `Days: window` and appends one Open value per window day, so len(Days)==len(Open). internal/render/template.go:714 (separate box) and :725 (overlay) both feed `days: d.burndown.days || []`; grep confirms no remaining `d.burndown.window` feed in template.go. Payload-shape tests: internal/render/derive_test.go:107 TestBurndownDaysMatchOpenPoints asserts len(Days)!=len(Open) fails and len(Days)<=2 fails (i.e. ==open and >2); cmd/boardctl/render_test.go TestRenderBurndownUsesDays asserts payload days==open, days>2, days[0]==window[0], `strings.Count(s, "days: d.burndown.days || []") == 2`, and rejects `days: d.burndown.window`. Gate run fresh: `go build ./...` exit 0, `go vet ./...` exit 0, `go test -count=1 ./...` exit 0 with `ok github.com/coding-hermes/boardctl/cmd/boardctl`, `ok .../internal/render`, `ok .../internal/board`, etc.; targeted `go test -count=1 -run 'TestBurndownDaysMatchOpenPoints|TestPayloadRoundTripAndShapes|TestRenderBurndownUsesDays' -v` → all PASS.
BurndownSeries.Days is populated per-day and fed as the x-axis at both template chart sites, with payload-shape tests asserting len(days)==len(open)>2 and a green build/vet/test gate.

Overall: PASS ✓
