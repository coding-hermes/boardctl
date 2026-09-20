# boardctl dogfood — integration report (render / serve surface)

Date: 2026-09-20
Run: dogfood lane `boardctl-dogfood`, tick `boardctl-dogfood-2026-09-20-04-33-31`
Target: `boardctl` — `/home/kara/coding-hermes-boardctl` @ `ab0489d`
Verdict: **PROMISING-BUT-ROUGH** (CLI/board core remains SHIPPABLE; the report surface is rough)
Previous runs: 2026-09-04 (PROMISING-BUT-ROUGH, install + bootstrap), 2026-09-12
(SHIPPABLE, CLI/board surface). This run deliberately took the surface those two
never touched.

---

## 1. Why this angle

Runs 4 and 12 both passed the CLI/board surface green. The README's analytics
stack — `render` (self-contained HTML report), `serve` (loopback upload server),
`import` (report → board round trip), governed by
`docs/specs/board-analytics-report.md` — had no dogfood coverage at all. That is
where this run went, and it produced six findings, including a chart that has
never rendered on any board.

## 2. What was promised

> A user can turn any coding-hermes board into a self-contained multi-board HTML
> analytics report (`render`), open an uploader in a browser to get the same
> report from a board folder or a `.zip` (`serve`), and round-trip the report's
> data payload back into a board (`import`) — with zero network access, zero
> external assets, and zero writes to board files.

## 3. What actually happened

### Worked

- `render` on the real board: 0.016 s, 318 KB self-contained HTML, 1 board.
  `grep -c 'https?://'` for `src|href` → **0 external references** (spec
  non-goal "no CDN, no external fonts, no external charts" holds).
- Report opens in Chromium with **no JS errors** (`window.onerror` +
  `unhandledrejection` clean). Tabs (Overview / Compare), board cards, the row
  expander (42 → 43 rows on click), the compare table, the burned-up overlay,
  and the 26-week calendar all render and respond.
- `serve`: loopback-only enforcement is real and well-worded —
  `--addr 0.0.0.0:8790` and `--addr 10.1.2.3:8790` both refuse with exit 2.
- `serve` zip upload: `HTTP 200`, 3 boards rendered (`-C` board first, then the
  two uploaded). The `0 boards found in upload` notice path works (wrong field
  name → HTTP 200 + visible banner, exactly as documented).
- **Read-only claim verified**: sha256 of all four board files identical before
  and after a render plus a serve upload.
- `import` round trip: `render` a board to JSON, `import --dry-run` it back →
  `0 new tasks, 2 identical (no-op)`, `nothing written`, exit 0. A name mismatch
  is refused cleanly (`board mismatch: export is for board "Probe"; target board
  is "RT"`).
- Data honesty where it counts: the `completed_no_done_ids` footnote is real —
  crier's report carries 192 ids and renders the "Data quality" heading.

### Broke

| # | Row | What a user sees |
|---|-----|------------------|
| 1 | `DF-BOARDCTL-1` | **The burndown chart is blank on every report.** Shows "no tasks yet" on a 42-task board. |
| 2 | `DF-BOARDCTL-5` | Multi-board reports list *another board's* ids in the data-quality footnote. |
| 3 | `DF-BOARDCTL-6` | The folder-upload arm (README's first serve path) could not be exercised — now a falsifiable row, not a silent pass. |
| 4 | `DF-BOARDCTL-2` | Re-uploading the same boards duplicates them in the report and compare table forever. |
| 5 | `DF-BOARDCTL-3` | The parser rejects the `-0500` dialect boardctl's own detector accepts and preserves. |
| 6 | `DF-BOARDCTL-4` | Velocity reports 199 completions on a board with 391 — under half. |

## 4. Finding 1 in detail — the burndown that never rendered

This is the run's headline, because it is invisible and it is total.

The payload's two time series are shaped differently:

```json
"burndown": {"window": ["2026-09-03","2026-09-19"], "open": [2,1,0,0,0,0,0,0,0,5,4,2,1,1,0,0,2]}
"burnup":   {"window": ["2026-09-03","2026-09-19"], "days": ["2026-09-03", ...], "cum": [3,11,15,...]}
```

`template.go:714` (and `:725` for the overlay) hands the chart
`days: d.burndown.days || []`. `chartLines` (`template.go:388`) opens with:

```js
var days = series[0] ? series[0].days : [];
if (!days.length) return emptyChart(opts.empty, W, H);
```

`burndown` has no `days` key, so `days.length === 0` and the function returns the
`"no tasks yet"` placeholder — **discarding all 17 data points.**

Measured in the live DOM of a rendered report:

```json
{"burndown_has_days_key": false, "burndown_data_points": 17,
 "chartLines_BURNDOWN_returns_empty": true,
 "burnup_has_days_key": true,  "chartLines_BURNUP_returns_empty": false}
```

Reproduced on a fresh bunker box by grep as well: `"burndown":{...}` contains 0
occurrences of `"days"`, `"burnup":{...}` contains 1.

**Why "all tests are green" missed it.** `derive_test.go` asserts on
`der.Burndown.Open` (lengths, values) and `load_test.go:151` asserts a non-nil
empty struct. Nothing asserts the **JSON shape the template consumes**. The
derive layer is correct; the template contract is broken; no test spans the
boundary. This is the classic premature-completion shape: every layer's own
tests pass, the composition is wrong.

### The right way

Two valid fixes, and the second is the more honest one:

1. Add `Days []string \`json:"days"\`` to `BurndownSeries` (`derive.go:38`),
   populated from the same window, mirroring `BurnupSeries` (`derive.go:45`).
   This aligns burndown with its sibling and needs no template change.
2. Change `template.go:714/725` to read `window`.

Either way, add a test that **renders the HTML** and asserts the burndown panel
is not the empty placeholder for a board with open tasks. A payload-shape test
would also have caught it.

## 5. Finding 4 in detail — the velocity shortfall

crier (`~/crier`, 421 rows, 391 complete) reports:

- `derived.complete_count`: **391**
- `derived.velocity` total across the window: **199**
- chart caption: *"Are we completing more or fewer tasks per week than last month?"*

The gap is structural, not a bug in the fallback. Measured breakdown:

- 192 complete rows have **neither** a parseable `completed_at` **nor**
  `updated_at`.
- 182 of those 192 **do** carry a `task_completed` event in `events.jsonl` —
  the completion *is* recorded, just not on the row.
- velocity (and cycle time) read row timestamps only, so those completions
  vanish from the time series.

Note the fallback that *does* exist and works: `derive.go:335-338` uses
`updated_at` when a complete row has no `completed_at`. On boardctl's own board
that is what reconciles 12 row-timestamps + 4 rows without `completed_at` into
the payload's 16 for W36. So the mechanism is there — it just never consults the
event stream. A board whose rows carry no timestamps at all therefore reports
roughly half its delivered work, and the chart looks like a quiet week rather
than a data gap.

## 6. Finding 5 in detail — the footnote names the wrong board

`report.go` assembles the multi-board payload by unioning every board's
no-done ids into a single **payload-level** field:

```go
noDone := map[string]bool{}
for _, b := range boards { ...; for _, id := range noDoneCompletedIDs(d) { noDone[id] = true } }
rp.NoDoneCompleted = sortedKeys(noDone)
```

The template renders it once in the footer
(`template.go:1321-1324`), unscoped. So the 3-board upload of
`[boardctl, coding-hermes-tools(b2)]` shows
`CHT-007, CHT-008, CHT-023, CHT-026, CHT-027, CHT-046, CHT-050` — **b2's ids** —
in the report a user opened for the boardctl board. Single-board `render` is
unaffected only because there is exactly one board to attribute.

This is the honesty footnote misattributing rows, which is worth a P1 on
principle: the field exists to make the report trustworthy.

## 7. Time to first success

| Step | Time | Friction |
|---|---|---|
| install (release binary, README verbatim, fresh box) | **3 s** | 0 |
| `boardctl version` on the downloaded asset | instant | 0 |
| first real command (`doctor` on a cloned board) | instant | 0 |
| **render a usable report** | **0.016 s** | 0 |
| understanding a *wrong* chart (burndown blank) | — | **blocking** — no error, no warning, no way to know it is broken |

Time-to-first-success is essentially zero. Time-to-**trust** is where it fails:
the report renders confidently and incorrectly, and the user has no signal.

## 8. Findings NOT filed (checked and clean)

- Self-containment, no-network, no-CDN: **holds** (0 external refs).
- Read-only toward board files: **holds** (sha256 identical).
- Loopback-only enforcement: **holds** (exit 2, good message).
- `doctor` on a real board: `RESULT: OK (0 warning(s))`.
- Board-not-found exit contract: exit **2**, as documented.
- `import` name-mismatch refusal and no-op detection: **correct**.
- Payload HTML escaping (`<`/`>`/`&`/U+2028) so `</script>` cannot terminate the
  island: implemented (`report.go:149`), no break observed.
- Fixture exclusion in counts: **correct** — the 43-row board reports 42
  nonfixture tasks because `NEVER-DONE` is excluded (that is why the report's
  numbers differ from `wc -l`, and it is right).

### Minor, not filed

- `boardctl <cmd> --help` exits **1** for every subcommand, while the bare
  `help` / `--help` exit 0. `docs/dogfood/diagnostics.md` documents this shape
  deliberately ("a usage error"), so it is recorded here rather than re-filed.
- Report `report_timezone` prints `Local`. Correct, but a report shared to
  another machine is ambiguous about which zone `Local` was.

## 9. Reproduce this run

```bash
cd /path/to/boardctl
go build -o /tmp/bc ./cmd/boardctl

# finding 1 — the burndown that never renders
/tmp/bc -C . render -o /tmp/r.html --json /tmp/r.json
jq -c '.boards[0].derived.burndown, .boards[0].derived.burnup' /tmp/r.json
# burndown has window+open; burnup has window+days+cum. Open the HTML:
# the Burndown panel shows "no tasks yet" while Burn-up draws normally.

# finding 5 — fleet-global footnote
/tmp/bc -C <repoA> serve --addr 127.0.0.1:8899 &
curl -s -F "zip=@two-boards.zip" http://127.0.0.1:8899/ -o /tmp/multi.html
grep -o '"completed_no_done_ids":\[[^]]*\]' /tmp/multi.html

# finding 2 — serve accumulation
for i in 1 2 3; do curl -s -o /dev/null -F "zip=@two-boards.zip" http://127.0.0.1:8899/; done
curl -s http://127.0.0.1:8899/api/boards

# finding 3 — the rejected dialect
printf '%s\n' '{"id":"X-1","title":"t","status":"pending","priority":"P1","created_at":"2026-09-18T03:47:31-0500"}' > /tmp/tb/tasks.jsonl
/tmp/bc -C /tmp/tb render -o /dev/null --json /tmp/tb.json && grep timestamp_excluded /tmp/tb.json
```

## 10. Left behind

- This report: `docs/dogfood/2026-09-20-render-serve-integration.md`
- Diagnostics trail: `docs/dogfood/diagnostics.md` (Run 8 section)
- Row entries: `.coding-hermes/dogfood-log.md`
- Board rows: `DF-BOARDCTL-1..6` in `.coding-hermes/board/tasks.jsonl`

Nothing was fixed — findings are rows, the foreman works them.
