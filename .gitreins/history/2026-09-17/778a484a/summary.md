# Verdict: BT-032

**Task:** validate must fail loudly on a corrupt fixtures.jsonl
**Evaluated:** 2026-09-17T08:10:16.822372
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m3:09AM[0m [32mINF[0m [1mscanned ~914392 bytes (914.39 KB) in 351ms[0m
[90m3:09AM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.539s
ok  	github.com/coding-hermes/boardctl/in
- ✓ **tier2**
  - COMPLETE
  ✓ boardctl validate exits 1 and prints an itemized error naming fixtures.jsonl when a fixtures.jsonl row is truncated mid-JSON (the IterParsed error is captured in validateFixtures exactly like validateTasks/validateEvents); a regression test in cmd/boardctl/qa_corruption_test.go corrupts ONLY fixtures.jsonl on a disposable t.TempDir board through the real run() and asserts exit 1 + 'fixtures.jsonl' + RESULT: FAIL, then proves zero-write and restore-to-clean; the fix is proven load-bearing by reverting only the production line and observing the new test FAIL; go build, go vet, go test ./... -count=1 -short, make fmt-check all pass.: Production fix: internal/board/validate.go:315 `ierr := IterParsed(lines, ...)` and :328-330 `if ierr != nil { rep.Add("error", "fixtures.jsonl: %v", ierr) }` — identical pattern to validateTasks (validate.go:96) and validateEvents (validate.go:216). Regression test: cmd/boardctl/qa_corruption_test.go TestQACorruption table adds {"fixtures_jsonl_truncated_row","fixtures.jsonl"}; qaSeedValidBoard uses t.TempDir (line 66) and drives the real run() via qaRunCLI (line 117); qaTruncateFirstLine corrupts only the target and qaChangedFiles asserts exactly [fixtures.jsonl] changed; asserts code==1, strings.Contains(out, tc.file), strings.Contains(out,"RESULT: FAIL"); zero-write proven by qaBoardFileHashes(afterMut) vs afterValidate with len(changed)==0; restore-to-clean by os.WriteFile(target, orig) then validate exit 0 and hashes equal the clean baseline. Load-bearing proof: in a copy at /tmp/bt032check I reverted ONLY the production line (ierr := IterParsed -> IterParsed, ierr -> err) and the test FAILED: 'qa_corruption_test.go:251: validate with corrupted fixtures.jsonl exit code = 0, want 1 ... RESULT: OK (0 warning(s))'; copy deleted. Commands run fresh: `go build ./...` exit 0; `go vet ./...` exit 0; `go test ./... -count=1 -short` -> ok cmd/boardctl 0.541s, ok internal/board 0.151s, ok internal/fmtcheck 0.041s, ok internal/render 0.015s, ok internal/versioncheck 0.004s (all ok, no FAIL); `make fmt-check` -> ok internal/fmtcheck 0.033s, exit 0. Verbose run confirms `--- PASS: TestQACorruption/fixtures_jsonl_truncated_row`. Docs updated: docs/dogfood/qa-corruption.md:67 'Known gap DISCOVERED by QA-BOARDCTL-001 — FIXED by BT-032'.
validateFixtures now captures the IterParsed error and reports fixtures.jsonl; the new TestQACorruption fixtures case passes on the fixed code and fails (exit 0, want 1) when only the production line is reverted, with build/vet/test/fmt-check all green.

## Summary

Judge Result: BT-032

Stage tier1: PASS
    ✓ secrets: [90m3:09AM[0m [32mINF[0m [1mscanned ~914392 bytes (914.39 KB) in 351ms[0m
[90m3:09AM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.539s
ok  	github.com/coding-hermes/boardctl/in

Stage tier2: PASS
  COMPLETE
  ✓ boardctl validate exits 1 and prints an itemized error naming fixtures.jsonl when a fixtures.jsonl row is truncated mid-JSON (the IterParsed error is captured in validateFixtures exactly like validateTasks/validateEvents); a regression test in cmd/boardctl/qa_corruption_test.go corrupts ONLY fixtures.jsonl on a disposable t.TempDir board through the real run() and asserts exit 1 + 'fixtures.jsonl' + RESULT: FAIL, then proves zero-write and restore-to-clean; the fix is proven load-bearing by reverting only the production line and observing the new test FAIL; go build, go vet, go test ./... -count=1 -short, make fmt-check all pass.: Production fix: internal/board/validate.go:315 `ierr := IterParsed(lines, ...)` and :328-330 `if ierr != nil { rep.Add("error", "fixtures.jsonl: %v", ierr) }` — identical pattern to validateTasks (validate.go:96) and validateEvents (validate.go:216). Regression test: cmd/boardctl/qa_corruption_test.go TestQACorruption table adds {"fixtures_jsonl_truncated_row","fixtures.jsonl"}; qaSeedValidBoard uses t.TempDir (line 66) and drives the real run() via qaRunCLI (line 117); qaTruncateFirstLine corrupts only the target and qaChangedFiles asserts exactly [fixtures.jsonl] changed; asserts code==1, strings.Contains(out, tc.file), strings.Contains(out,"RESULT: FAIL"); zero-write proven by qaBoardFileHashes(afterMut) vs afterValidate with len(changed)==0; restore-to-clean by os.WriteFile(target, orig) then validate exit 0 and hashes equal the clean baseline. Load-bearing proof: in a copy at /tmp/bt032check I reverted ONLY the production line (ierr := IterParsed -> IterParsed, ierr -> err) and the test FAILED: 'qa_corruption_test.go:251: validate with corrupted fixtures.jsonl exit code = 0, want 1 ... RESULT: OK (0 warning(s))'; copy deleted. Commands run fresh: `go build ./...` exit 0; `go vet ./...` exit 0; `go test ./... -count=1 -short` -> ok cmd/boardctl 0.541s, ok internal/board 0.151s, ok internal/fmtcheck 0.041s, ok internal/render 0.015s, ok internal/versioncheck 0.004s (all ok, no FAIL); `make fmt-check` -> ok internal/fmtcheck 0.033s, exit 0. Verbose run confirms `--- PASS: TestQACorruption/fixtures_jsonl_truncated_row`. Docs updated: docs/dogfood/qa-corruption.md:67 'Known gap DISCOVERED by QA-BOARDCTL-001 — FIXED by BT-032'.
validateFixtures now captures the IterParsed error and reports fixtures.jsonl; the new TestQACorruption fixtures case passes on the fixed code and fails (exit 0, want 1) when only the production line is reverted, with build/vet/test/fmt-check all green.

Overall: PASS ✓
