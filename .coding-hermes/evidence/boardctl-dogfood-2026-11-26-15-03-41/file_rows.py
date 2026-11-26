#!/usr/bin/env python3
"""Run 32 filing: three dogfood findings via the board's own writer (argv lists)."""
import json
import subprocess
import sys

BOARD_DIR = "/home/kara/coding-hermes-boardctl"
BCTL = ["/home/kara/.local/bin/boardctl", "-C", BOARD_DIR]
TASKS = BOARD_DIR + "/.coding-hermes/board/tasks.jsonl"
EVENTS = BOARD_DIR + "/.coding-hermes/board/events.jsonl"

ROWS = [
    dict(
        id="DF-BOARDCTL-063",
        title="decompose silently drops detail.metadata (expected_files/verify_steps) for UNTYPED parents — spec BT-099 §5 promises them unconditionally",
        priority="P2",
        complexity=2,
        type="task",
        reasoning=(
            "dogfood run 32 (tick boardctl-dogfood-2026-11-26-15-03-41) at HEAD c6924e7. "
            "BT-099 spec §5: children inherit 'priority, capability_tags, type + configured metadata, "
            "expected_files, verify_steps — the parent template' (unconditional list). Implementation "
            "internal/board/decompose.go:203-207 gates spec.Metadata = inheritedMetadata(parent) inside "
            "'if parentType != \"\"', so an untyped parent's detail.metadata never reaches the children. "
            "Contrast arms on one scratch board (HEAD build): typed parent REL-100 (type=release, "
            "detail.metadata={channel,expected_files,signoffs,verify_steps}) -> children REL-101/102 inherit "
            "all four keys byte-identical; untyped parent UNT-100 with expected_files=[a.go,b.go] + "
            "verify_steps=[go test ./...] -> children UNT-101/102 store detail={fingerprint} ONLY, arrays "
            "gone, rc=0, no warning. The owner quote in the spec's header ('the foreman should know ... "
            "what files it thinks are needed, the substeps, verification steps') is defeated for untyped "
            "rows, which are the majority on live fleet boards (this repo's own 300-row board: 0 typed rows)."
        ),
        ac=(
            "AC1 (BT-099 spec §5 inheritance, decompose.go:203-207): decomposing an UNTYPED parent whose "
            "detail.metadata carries expected_files+verify_steps yields children carrying the same "
            "detail.metadata arrays — OR, if the type gate is deliberate doctrine, decompose prints an "
            "explicit warning naming the dropped keys AND spec §5 + the --help decompose text are amended "
            "to state the gate. AC2: typed-parent inheritance (REL-arm shape) stays byte-identical "
            "(regression test). AC3: a test covers the untyped-parent decompose inheritance path "
            "(bt099_parent_test.go family). Repro: create untyped parent with --expected-files a.go,b.go "
            "--verify-steps 'go test ./...', decompose --count 2, jq the children's .detail."
        ),
    ),
    dict(
        id="DF-BOARDCTL-064",
        title="create --meta on an UNTYPED row silently drops the metadata (rc=0, no warning) while update --meta refuses the same row rc=1 — asymmetric fail-open vs fail-closed",
        priority="P2",
        complexity=2,
        type="task",
        reasoning=(
            "dogfood run 32 at HEAD c6924e7. Measured arms on one scratch board: (1) create --id UNT-2 "
            "--meta foo=bar -> rc=0 'created task UNT-2', stored row detail={fingerprint} only — foo is "
            "NOWHERE (tasks.jsonl and events.jsonl checked), no warning printed; (2) update UNT-2 --meta "
            "foo=baz -> rc=1 'update refused: row type \"\" is not configured'. Same field, same untyped "
            "row: create fails open and silently, update fails closed and loudly. DF-BOARDCTL-58 (fixed, "
            "f23a2aa) made create warn on UNDECLARED keys of a TYPED row; the untyped-row path was left "
            "silent. A foreman composing rows from the --help create line ('[--meta k=v]...', no type "
            "qualifier) believes the metadata was stored."
        ),
        ac=(
            "AC1: create --meta k=v on a row with no configured type either (a) refuses rc=1 with update's "
            "message shape ('row type \"\" is not configured') writing nothing, or (b) writes the key into "
            "detail.metadata and warns like the typed-undeclared path — pick ONE and make create/update "
            "agree. AC2: refusal/warning behaviour covered by a test in the df_boardctl_58 family. "
            "AC3: typed-row behaviour (declared + undeclared keys) unchanged (regression). Repro: on a "
            "scratch board, create --id X-1 --title t --meta foo=bar, then jq '.detail' — expect either "
            "rc=1 or detail.metadata.foo present, never a silent drop."
        ),
    ),
    dict(
        id="DF-BOARDCTL-065",
        title="BT-099 create/update flags (--parent, --expected-files, --verify-steps) and show --json are absent from the --help usage blocks and README (0 grep hits) — spec §7 count-pinned-surface rule honoured for verbs but not row flags",
        priority="P3",
        complexity=1,
        type="task",
        reasoning=(
            "dogfood run 32 discovery census at HEAD c6924e7. The flags are implemented and functional "
            "(cmd/boardctl/main.go:815/881-884/1068/1147-1151) but: top-level 'boardctl --help' create "
            "block ends at [--defer-review-by DATE] and never lists --parent/--expected-files/--verify-steps; "
            "the update block likewise omits them; the help show line reads 'show <id> [--events]' with no "
            "--json (the flag's own error-path usage at main.go:719 DOES list it). README.md: grep -c "
            "'expected-files'=0, 'verify-steps'=0, '--parent'=0 (README documents the comment/decompose "
            "VERBS at lines 265-290 but not the create/update flags). skills/boardctl-usage: "
            "expected_files 1 hit, verify_steps 1 hit, '--parent' 0 hits. BT-099 spec §7 required the "
            "count-pinned surfaces to update WITH the feature. Same class as DF-BOARDCTL-54/57."
        ),
        ac=(
            "AC1: 'boardctl --help' create and update blocks list --parent, --expected-files, --verify-steps; "
            "the show line lists --json. AC2: README gains a section (or extends the BT-099 block at ~265) "
            "documenting the three create/update flags with one copy-pasteable example each — grep -c for "
            "each flag token >= 1 in README.md. AC3: skills/boardctl-usage proven-commands gains a --parent "
            "example. AC4: the docs fence gate (or describe parity test) that would have caught this is "
            "named in the fix commit or a follow-up row. Repro: boardctl --help | grep -c -- '--parent' "
            "-> 0 at c6924e7."
        ),
    ),
]


def main() -> int:
    # prove ids free first
    with open(TASKS, encoding="utf-8") as fh:
        existing = [ln for ln in fh if ln.strip()]
    for row in ROWS:
        hits = sum(1 for ln in existing if json.loads(ln).get("id") == row["id"])
        if hits:
            print(f"ABORT: id {row['id']} already on board ({hits} hits)")
            return 1
    print(f"ids free; board has {len(existing)} rows before filing")

    for row in ROWS:
        cmd = BCTL + [
            "create",
            "--id", row["id"],
            "--title", row["title"],
            "--priority", row["priority"],
            "--complexity", str(row["complexity"]),
            "--type", row["type"],
            "--reasoning", row["reasoning"],
            "--acceptance-criteria", row["ac"],
            "--status", "pending",
        ]
        proc = subprocess.run(cmd, capture_output=True, text=True)
        print(f"create {row['id']}: rc={proc.returncode}")
        if proc.stdout.strip():
            print("  out:", proc.stdout.strip()[:200])
        if proc.stderr.strip():
            print("  err:", proc.stderr.strip()[:200])
        if proc.returncode != 0:
            return proc.returncode

    # verify rows + paired events
    with open(TASKS, encoding="utf-8") as fh:
        after = [json.loads(ln) for ln in fh if ln.strip()]
    with open(EVENTS, encoding="utf-8") as fh:
        evs = []
        for ln in fh:
            try:
                obj = json.loads(ln)
                if isinstance(obj, dict):
                    evs.append(obj)
            except json.JSONDecodeError:
                pass
    for row in ROWS:
        rid = row["id"]
        matches = [r for r in after if r.get("id") == rid]
        created = [
            e for e in evs
            if e.get("task_id") == rid
            and (e.get("event_type") == "task_created" or e.get("event") == "task_created")
        ]
        ac_len = len(matches[0].get("acceptance_criteria") or "") if matches else 0
        print(f"verify {rid}: rows={len(matches)} created_events={len(created)} ac_chars={ac_len} status={matches[0].get('status') if matches else '?'}")
        if len(matches) != 1 or len(created) != 1:
            return 1
    print(f"board rows after: {len(after)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
