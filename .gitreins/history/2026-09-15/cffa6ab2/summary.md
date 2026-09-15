# Verdict: BT-027

**Task:** serve upload UX: 0-board upload must warn visibly; document -C board prepend rule
**Evaluated:** 2026-09-15T02:28:41.722661
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m9:27PM[0m [32mINF[0m [1mscanned ~834666 bytes (834.67 KB) in 280ms[0m
[90m9:27PM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.446s
ok  	github.com/coding-hermes/boardctl/in
- ✓ **tier2**
  - COMPLETE
  ✓ A serve upload that registers 0 boards while the request carried file parts must render a visible notice (not just a report of the -C board); README/spec state that uploaded boards are appended after the -C board and that a board is a dir containing BOTH tasks.jsonl and events.jsonl; a test asserts the combined board count with and without -C; go build/vet/test and a live curl E2E prove it.: (1) Visible notice: cmd/boardctl/serve.go:354-357 sets uploadNotice=zeroBoardsNotice when len(boards)==0 && len(s.cBoards)>0, :377 payload.UploadNotice=uploadNotice; internal/render/derive.go:178 adds `UploadNotice string json:"upload_notice,omitempty"`; internal/render/template.go:288-322 renderBanner reads `payload.upload_notice`, and when set does b.hidden=false and appends the notice as a LEADING <strong>. LIVE curl E2E: `curl -F file=@stray.jsonl http://127.0.0.1:8791/` -> HTTP=200, body contains '0 boards found in upload: a board is a directory containing BOTH tasks.jsonl and events.jsonl', island upload_notice set, boards=1 (cboard only). Headless node DOM sim of the served HTML: banner.hidden=false with first child strong:"0 boards found in upload: ..." -> genuinely visible, not just a -C report. (2) Docs: README.md:96-108 states '-C board ... is PREPENDED to every report ... -C board first, then uploaded boards, so a two-board zip alongside -C renders three boards' and 'Every directory that contains BOTH tasks.jsonl and events.jsonl ... is registered as a board'; docs/specs/board-analytics-report.md:741-750 adds the clarifying note 'serve PREPENDS the -C board ... combined count is len(-C boards)+len(uploaded boards), -C first, uploads appended after' plus the upload_notice notice. (3) Combined-count test: cmd/boardctl/bt027_serve_notice_test.go TestBT027CombinedCountCPlusZip asserts len(p.Boards)==3 with -C (cboard first, no notice) and TestBT027NoNoticeOnSuccessfulUploadWithC asserts len==2 (1 -C + 1 uploaded); TestBT027ZeroBoardUploadWithCWarnsVisibly asserts len==1 + notice; TestBT027ZeroBoardUploadNoCStill400 asserts 400. LIVE curl: with -C -> 3 boards ['cboard','zipboarda','projB']; without -C -> 2 boards ['zipboarda','projB']. (4) Build/test/E2E: `go build ./...` exit 0, `go vet ./...` exit 0, `go test -count=1 ./...` exit 0 (ok cmd/boardctl 0.466s, ok internal/board, ok internal/render); `go test -count=1 -run 'BT027|Notice|ZeroBoard|Upload' ./cmd/boardctl/... -v` -> all 7 PASS. RED proof: reverting serve.go/template.go/derive.go to HEAD~1 makes the test fail to build (p.UploadNotice undefined), confirming the test exercises the fix. Live curl E2E on 127.0.0.1:8791/8792/8793 confirmed all behaviors; working tree restored clean.
BT-027 fully verified: 0-board serve uploads render a visible leading banner notice (proven by live curl + headless DOM sim), README/spec document the -C prepend and board-pair rules, tests assert combined counts with/without -C, and go build/vet/test all pass with a confirmed RED proof.

## Summary

Judge Result: BT-027

Stage tier1: PASS
    ✓ secrets: [90m9:27PM[0m [32mINF[0m [1mscanned ~834666 bytes (834.67 KB) in 280ms[0m
[90m9:27PM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.446s
ok  	github.com/coding-hermes/boardctl/in

Stage tier2: PASS
  COMPLETE
  ✓ A serve upload that registers 0 boards while the request carried file parts must render a visible notice (not just a report of the -C board); README/spec state that uploaded boards are appended after the -C board and that a board is a dir containing BOTH tasks.jsonl and events.jsonl; a test asserts the combined board count with and without -C; go build/vet/test and a live curl E2E prove it.: (1) Visible notice: cmd/boardctl/serve.go:354-357 sets uploadNotice=zeroBoardsNotice when len(boards)==0 && len(s.cBoards)>0, :377 payload.UploadNotice=uploadNotice; internal/render/derive.go:178 adds `UploadNotice string json:"upload_notice,omitempty"`; internal/render/template.go:288-322 renderBanner reads `payload.upload_notice`, and when set does b.hidden=false and appends the notice as a LEADING <strong>. LIVE curl E2E: `curl -F file=@stray.jsonl http://127.0.0.1:8791/` -> HTTP=200, body contains '0 boards found in upload: a board is a directory containing BOTH tasks.jsonl and events.jsonl', island upload_notice set, boards=1 (cboard only). Headless node DOM sim of the served HTML: banner.hidden=false with first child strong:"0 boards found in upload: ..." -> genuinely visible, not just a -C report. (2) Docs: README.md:96-108 states '-C board ... is PREPENDED to every report ... -C board first, then uploaded boards, so a two-board zip alongside -C renders three boards' and 'Every directory that contains BOTH tasks.jsonl and events.jsonl ... is registered as a board'; docs/specs/board-analytics-report.md:741-750 adds the clarifying note 'serve PREPENDS the -C board ... combined count is len(-C boards)+len(uploaded boards), -C first, uploads appended after' plus the upload_notice notice. (3) Combined-count test: cmd/boardctl/bt027_serve_notice_test.go TestBT027CombinedCountCPlusZip asserts len(p.Boards)==3 with -C (cboard first, no notice) and TestBT027NoNoticeOnSuccessfulUploadWithC asserts len==2 (1 -C + 1 uploaded); TestBT027ZeroBoardUploadWithCWarnsVisibly asserts len==1 + notice; TestBT027ZeroBoardUploadNoCStill400 asserts 400. LIVE curl: with -C -> 3 boards ['cboard','zipboarda','projB']; without -C -> 2 boards ['zipboarda','projB']. (4) Build/test/E2E: `go build ./...` exit 0, `go vet ./...` exit 0, `go test -count=1 ./...` exit 0 (ok cmd/boardctl 0.466s, ok internal/board, ok internal/render); `go test -count=1 -run 'BT027|Notice|ZeroBoard|Upload' ./cmd/boardctl/... -v` -> all 7 PASS. RED proof: reverting serve.go/template.go/derive.go to HEAD~1 makes the test fail to build (p.UploadNotice undefined), confirming the test exercises the fix. Live curl E2E on 127.0.0.1:8791/8792/8793 confirmed all behaviors; working tree restored clean.
BT-027 fully verified: 0-board serve uploads render a visible leading banner notice (proven by live curl + headless DOM sim), README/spec document the -C prepend and board-pair rules, tests assert combined counts with/without -C, and go build/vet/test all pass with a confirmed RED proof.

Overall: PASS ✓
