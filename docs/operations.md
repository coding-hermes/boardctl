# Operating boardctl — operator quick-reference

A 3am index for failure classes. Every entry: symptom → command → the README
section with the full story. This is a pointer page, not a tutorial.
Line numbers refer to README.md on this branch and drift as sections are
added — the anchors are the stable reference.

## First five minutes (diagnose before you fix)

```bash
boardctl -C ~/myproject validate   # parse + row-level checks; exit 1 = findings
boardctl -C ~/myproject doctor     # validate + deep checks: git tracked-set, header-vs-events tick drift, fixture orphans
boardctl -C ~/myproject stats      # quick census: is the data even there?
```

## Failure classes

### 1. Corrupt / truncated board JSONL

Symptom: `validate` exits 1 with unparseable-line errors; whole board unreadable (one bad line poisons the file).

```bash
boardctl -C ~/myproject validate --repair   # writes tasks.rewritten.jsonl
```

`--repair` NEVER touches `tasks.jsonl` — the rewritten file is for manual
review: diff it, validate it in a scratch board, then swap by hand.

README: "### render" → "Damaged-board salvage (DF-BOARDCTL-9)", lines ~301-329 (anchor `#render`).

Related: class 5 (fused lines) — a line carrying SEVERAL records is a
different failure with a different repair path.

### 1b. Drifted row shapes (keys/depends_on/status outside canon)

Symptom: `validate` census shows many rows off-canon ("drift N row(s)"),
`render` report excludes most rows from time metrics, or rows carry
`depends_on: "[]"` string shapes the cross-check cannot see through.

```bash
boardctl -C ~/myproject normalize-board               # classify + stage tasks.normalized.jsonl
boardctl -C ~/myproject normalize-board --apply       # rewrite tasks.jsonl, one atomic pass
```

Mechanical repairs only (key aliases, string `depends_on` -> array, alias
statuses); ambiguous rows are REFUSED and listed — decide those by hand.
Rows without `created_at` are reported, never fabricated.

`normalize-board` is FAIL-CLOSED on an unparseable `tasks.jsonl` line: it
takes no `--skip-bad-lines`, names the file + line, writes nothing, and
prints the remedy inline — `validate --repair` -> review
`tasks.rewritten.jsonl` -> swap it in -> re-run `normalize-board` (see 1a).
A writing repair must never rewrite a board it could not read in full.

README: "## normalize-board — drifted row SHAPES, board-wide (DF-BOARDCTL-36)".

### 2. Header drift — ticks_total vs max event tick

Symptom: `doctor` FAILS with `header ticks_total N < max events tick_number M —
header counter stale or an event tick never completed`. Happens after
`event --tick N --no-tick-write` or a tool that manages the header itself.

```bash
boardctl -C ~/myproject header --set-ticks-total <max-event-tick>
boardctl -C ~/myproject doctor              # re-check: drift gone
```

Never hand-edit the header line — `header --set-ticks-total` is the only write path (refuses, exit 1, on a headerless board).

README: "## Usage" command block (`header --set-ticks-total`, `--no-tick-write` notes), lines ~116-134; drift semantics lines ~516-531 (anchor `#usage`).

### 3. vuln-check TOOL-ERROR

Symptom: `make vuln-check` reports `TOOL-ERROR` — the scan itself did not
complete (exit code neither 0 nor 3). Usually govulncheck missing, stale, or
corrupted. Findings (exit 3) are a different case: the output names the vuln
ids and the fixing version — upgrade and re-run.

```bash
go install golang.org/x/vuln/cmd/govulncheck@v1.7.0   # pinned scanner
make vuln-check                                        # re-run
GOVULNCHECK=/path/to/binary make vuln-check            # override PATH lookup
```

Stub/unparsable installs only warn by default; `VULNCHECK_STRICT_SCANNER=1` turns that into a hard refusal.

README: "## Development" → vuln-check paragraphs, lines ~1075-1100 (anchor `#development`).

### 4. Deploy freshness warning at startup

Symptom: one stderr line at startup naming the binary's commit, current HEAD, and how many commits behind — a warning, never a failure; the fixes it hides live at HEAD.

```bash
make build                     # restamp from HEAD, redeploy
BOARDCTL_SKIP_FRESHNESS=1 ...  # suppress the warning when staying stale on purpose
```

README: "### Deployment freshness gate (BT-057)", lines ~1102-1117 (anchor `#deployment-freshness-gate-bt-057`).

### 5. Fused JSONL lines — several records on one physical line

Symptom: `validate` exits 1 with `multiple JSON values on one physical line
(a fused record — JSONL requires exactly one JSON object per line)` naming the
physical line; `list`/`show`/`stats` exit 1 with the same file:line diagnostic
instead of quietly reporting fewer tasks.

Cause: two (or many) JSON objects landed on ONE physical line — a lost final
newline plus an append, a hand-edit, a bad merge, or a generator that wrote
records without terminators. A line-based reader sees one line, so everything
after the first object is invisible; before GR-150 `validate` reported
`RESULT OK` and the missing rows simply were not there.

```bash
boardctl -C ~/myproject validate            # fails, names the fused line(s)
boardctl -C ~/myproject list --skip-bad-lines   # salvage the clean rows, exit 1, SKIPPED-LINES evidence
```

Fix by hand: split the named line(s) so each object occupies its own physical
line (`jq -c . <file >split` writes exactly that), then re-run `validate`.
`validate --repair` REFUSES a board with fused lines — re-serializing one would
keep the first object and silently drop the rest — so it must be split first.

## Not covered here

- Pre-commit board-lint hook install/retarget → README "Board-lint pre-commit
  hook (BT-054)".
- Row key/status vocabulary and alias canonicalization (`update --normalize`)
  → README "Status vocabulary and read aliases (BT-025)".
