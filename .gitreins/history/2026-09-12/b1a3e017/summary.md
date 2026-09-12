# Verdict: BT-020

**Task:** render: static HTML report from JSONL boards
**Evaluated:** 2026-09-12T12:14:27.466233
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.024s
ok  	github.com/coding-hermes/boardctl/in
  ✓ secrets: [90m7:09AM[0m [32mINF[0m [1mscanned ~598779 bytes (598.78 KB) in 273ms[0m
[90m7:09AM[0m [32
- ✓ **tier2**
  - COMPLETE
  ✓ Implement docs/specs/board-analytics-report.md sections 2, 3, 5, 6 and every BT-020 acceptance criterion in section 7.1: tolerant topology-A/B JSONL loading; exact Go-derived analytics and fixture exclusion; boardctl render CLI with timezone and optional JSON output; one self-contained file:// HTML with inline CSS/JS/SVG, collapse-first expandable task rows and filters; inert HTML/script payload escaping; deterministic output except rendered_at; tests; go build, vet, and full tests green; render must not modify board files.: All 10 BT-020 acceptance criteria verified with live output. Build/vet/test: `go build ./...` EXIT=0, `go vet ./...` EXIT=0, `go test ./... -count=1 -short` EXIT=0 (ok cmd/boardctl, ok internal/board, ok internal/render). AC7.1.1: `boardctl render -C . -o /tmp/r.html` exit 0, 222242 bytes (>50KB,<10MB). AC7.1.2: `google-chrome --headless --dump-dom file:///tmp/r.html` renders boardctl card + sparkline SVG, zero console errors. AC7.1.3: topology B (header moved to tasks.jsonl line 1, board.jsonl deleted) exit 0, ticks_total 71, badge B, derived==A byte-equal. AC7.1.4: empty board exit 0, tasks==[] events==[], HTML contains 'no tasks yet'/'no completions yet'/'no events'. AC7.1.5: torn line exit 0, parse_warnings.tasks==1, X row absent. AC7.1.6: removing fixture row -> derived.* identical (29 vs 28 tasks). AC7.1.7: hostile `</script><img src=x onerror=alert(1)>` title -> exactly 1 JSON island, 2 closing script tags, no raw `<img src=x`, island JSON.parse succeeds. AC7.1.8: two renders --tz America/Bogota differ only in rendered_at (diff empty after sed). AC7.1.9: TestStreakTodayBoundary PASS (current=3 live longest=5 on 09-10; ended since 2026-09-10 on 09-12). AC7.1.10: isolated git board sha256sum -c OK after render — board files unchanged. Spec sections: 2 (tolerant loader load.go: blank-line skip, torn-row warn, dup-id last-wins, 3-signal fixture union isFixture load.go:364, detail ladder decodeDetail load.go:400, dual timestamp layouts parseStamp), 3 (all analytics in Go derive.go: burndown/burnup/velocity/cycle/streaks/tick_health/work_clock/model_share/status+priority counts), 5 (overview card grid, project page, collapse-first rows all aria-expanded=false, 150ms debounced filters, status dropdown, fixtures toggle, hash routing + hashchange, compare view, empty/error/warning states), 6 (CLI -C before/after subcommand, -o/-o -, --tz, --json, exit 0/1/2; self-contained HTML: 1 <style>, 1 <script>, 0 external src/href, no innerHTML, no fetch/XHR, no canvas/d3; island escaping <,>,&,U+2028,U+2029 all escaped with no raw chars). Minor non-blocking deviations: collapsed row adds an extra updated_at date column beyond the spec's 'EXACTLY' field list; the 5MB size guardrail 'log a note' is not implemented (soft, not in 7.1); the 3.1 clock-skew parse-warning note is not explicit.


## Summary

Judge Result: BT-020

Stage tier1: PASS
    ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.024s
ok  	github.com/coding-hermes/boardctl/in
  ✓ secrets: [90m7:09AM[0m [32mINF[0m [1mscanned ~598779 bytes (598.78 KB) in 273ms[0m
[90m7:09AM[0m [32

Stage tier2: PASS
  COMPLETE
  ✓ Implement docs/specs/board-analytics-report.md sections 2, 3, 5, 6 and every BT-020 acceptance criterion in section 7.1: tolerant topology-A/B JSONL loading; exact Go-derived analytics and fixture exclusion; boardctl render CLI with timezone and optional JSON output; one self-contained file:// HTML with inline CSS/JS/SVG, collapse-first expandable task rows and filters; inert HTML/script payload escaping; deterministic output except rendered_at; tests; go build, vet, and full tests green; render must not modify board files.: All 10 BT-020 acceptance criteria verified with live output. Build/vet/test: `go build ./...` EXIT=0, `go vet ./...` EXIT=0, `go test ./... -count=1 -short` EXIT=0 (ok cmd/boardctl, ok internal/board, ok internal/render). AC7.1.1: `boardctl render -C . -o /tmp/r.html` exit 0, 222242 bytes (>50KB,<10MB). AC7.1.2: `google-chrome --headless --dump-dom file:///tmp/r.html` renders boardctl card + sparkline SVG, zero console errors. AC7.1.3: topology B (header moved to tasks.jsonl line 1, board.jsonl deleted) exit 0, ticks_total 71, badge B, derived==A byte-equal. AC7.1.4: empty board exit 0, tasks==[] events==[], HTML contains 'no tasks yet'/'no completions yet'/'no events'. AC7.1.5: torn line exit 0, parse_warnings.tasks==1, X row absent. AC7.1.6: removing fixture row -> derived.* identical (29 vs 28 tasks). AC7.1.7: hostile `</script><img src=x onerror=alert(1)>` title -> exactly 1 JSON island, 2 closing script tags, no raw `<img src=x`, island JSON.parse succeeds. AC7.1.8: two renders --tz America/Bogota differ only in rendered_at (diff empty after sed). AC7.1.9: TestStreakTodayBoundary PASS (current=3 live longest=5 on 09-10; ended since 2026-09-10 on 09-12). AC7.1.10: isolated git board sha256sum -c OK after render — board files unchanged. Spec sections: 2 (tolerant loader load.go: blank-line skip, torn-row warn, dup-id last-wins, 3-signal fixture union isFixture load.go:364, detail ladder decodeDetail load.go:400, dual timestamp layouts parseStamp), 3 (all analytics in Go derive.go: burndown/burnup/velocity/cycle/streaks/tick_health/work_clock/model_share/status+priority counts), 5 (overview card grid, project page, collapse-first rows all aria-expanded=false, 150ms debounced filters, status dropdown, fixtures toggle, hash routing + hashchange, compare view, empty/error/warning states), 6 (CLI -C before/after subcommand, -o/-o -, --tz, --json, exit 0/1/2; self-contained HTML: 1 <style>, 1 <script>, 0 external src/href, no innerHTML, no fetch/XHR, no canvas/d3; island escaping <,>,&,U+2028,U+2029 all escaped with no raw chars). Minor non-blocking deviations: collapsed row adds an extra updated_at date column beyond the spec's 'EXACTLY' field list; the 5MB size guardrail 'log a note' is not implemented (soft, not in 7.1); the 3.1 clock-skew parse-warning note is not explicit.


Overall: PASS ✓
