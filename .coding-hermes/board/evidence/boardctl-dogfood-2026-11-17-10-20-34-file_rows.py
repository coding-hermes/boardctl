#!/usr/bin/env python3
"""File this dogfood tick's findings on the boardctl board.

One row per finding, argv-list invocation (no shell), read-back verification.
Run: python3 file_rows.py
"""
import subprocess
import sys

BOARD = "/home/kara/coding-hermes-boardctl"
TASKS = BOARD + "/.coding-hermes/board/tasks.jsonl"
BIN = "/home/kara/.hermes/cache/scratch/df-2026-11-17/boardctl"

ROWS = [
    {
        "id": "DF-BOARDCTL-56",
        "type": "bug",
        "priority": "P1",
        "complexity": 3,
        "title": "P1 — update --meta nulls stored LIST metadata (silent, unrecoverable data loss in the BT-098 write path)",
        "reasoning": (
            "Measured 2026-11-17 on a HEAD build (0179f5b) on a scratch board with a types.jsonl declaring "
            "release{channel enum required, source_commit string required, signoffs list, approval_ticket ref}. "
            "REPRO: create REL-7 --type release --meta channel=stable --meta source_commit=abcdefg --meta signoffs=ana@x,bo@x "
            "-> tasks.jsonl detail.metadata = {\"channel\":\"stable\",\"signoffs\":[\"ana@x\",\"bo@x\"],\"source_commit\":\"abcdefg\"} (correct). "
            "Then `update REL-7 --meta channel=beta --status pending` -> rc=0, output 'updated task REL-7: status, detail, updated_at', "
            "and detail.metadata becomes {\"channel\":\"beta\",\"signoffs\":null,\"source_commit\":\"abcdefg\"}. "
            "The list is GONE: grep for 'ana@x' in tasks.jsonl = 0 and in events.jsonl = 0, because the task_updated events carry only "
            "{\"status\":\"pending\"} and no metadata snapshot — the value is not recoverable from the board or its event log. "
            "`boardctl validate` after the loss: RESULT: OK (the optional field is now null, so nothing is flagged) — the loss is invisible "
            "to every read verb at once: validate OK, show renders signoffs: null, export/render JSON carries the null, stats unaffected. "
            "Controls on the same row: update --status pending alone and update --note alone PRESERVE the array byte-for-byte; only a --meta "
            "edit destroys it. ROOT CAUSE: internal/board/write.go:1225-1231 decodes the stored metadata into map[string]string and DISCARDS "
            "the error (`_ = json.Unmarshal(pd.Extra[\"metadata\"], &values)`); a JSON array cannot decode into a string, so the key never "
            "enters `values`, and encodeMetadata at :1249 then emits every declared field, writing the absent list as null. Same defect class "
            "as DF-BOARDCTL-44 (non-string values mangled on re-serialization) but with total loss instead of a byte change, on the create/"
            "update path rather than a bulk repair. Also affects the update --type retag path (same block)."
        ),
        "ac": (
            "- A regression test in internal/board creates a row with a list metadata value (signoffs=[\"a@x\",\"b@x\"]), runs the update path "
            "with a different --meta key, and asserts the list survives byte-identical in tasks.jsonl.\n"
            "- internal/board/write.go:1225-1231 stops decoding stored metadata through map[string]string: decode per-field (map[string]json.RawMessage "
            "or the type's declared field types) and never discard a JSON error.\n"
            "- encodeMetadata no longer writes a declared-but-unset optional field as JSON null; absent stays absent.\n"
            "- When a stored value cannot be re-encoded, the update REFUSES and names the field instead of reporting success.\n"
            "- Verified on a scratch board: create -> update --meta (other key) -> show --json keeps signoffs, and the same holds for a ref/bool/number value."
        ),
    },
    {
        "id": "DF-BOARDCTL-57",
        "type": "chore",
        "priority": "P2",
        "complexity": 3,
        "title": "P2 — BT-098's typed-row surface is undiscoverable: `types` is missing from boardctl --help and the types.jsonl schema is undocumented",
        "reasoning": (
            "Measured 2026-11-17 at HEAD 0179f5b. `boardctl --help` lists init/list/show/next-id/next/ready/direction/create/update/event/header/"
            "validate/sweep-status/normalize-statuses/normalize-board/doctor/evidence/version/stats/initiatives/render/import/serve/install/hook/"
            "umbrella — `types` is NOT there (`boardctl --help | grep -c -i types` = 0), although the subcommand works and is the only way to see "
            "what metadata a type requires. README.md mentions types.jsonl 0 times and `--meta` 0 times (grep -c), and create's help line "
            "advertises `[--meta k=v]...` with no explanation of the key vocabulary it accepts. skills/boardctl-usage names `types` and shows one "
            "`--meta channel=stable --meta source_commit=abc` example, but never the types.jsonl RECORD SCHEMA — so a user who wants to declare "
            "their own type (template title_prefix/default_priority/ac_skeleton, metadata fields with type enum|string|list|ref, required, values, "
            "ref) has to read internal/board/types.go or internal/board/testdata/bt098-release-type.jsonl. In this run the author of the scratch "
            "board found the shape only by reading testdata; the live repo board has no types.jsonl at all, so the feature is invisible there too. "
            "`types --doctor` on a board without the file prints 'types.jsonl: absent — built-in defaults active', which is the single pointer that "
            "exists and is itself unreachable from --help. This is the DF-BOARDCTL-54 class (a shipped feature with no docs) one level up: there the "
            "flags were undocumented, here the whole configuration format is."
        ),
        "ac": (
            "- `boardctl --help` lists `types  [-C dir] [--json] [--doctor]` in the commands block with a one-line description of what it reports.\n"
            "- README (or docs/) documents the types.jsonl record schema: slug/display/rev, template{title_prefix,default_priority,ac_skeleton}, and "
            "metadata[] {name, type in enum|string|list|ref, required, values, ref, description}, plus how templates yield to explicit flags.\n"
            "- The documented example is copy-pasteable and a test (or doc gate) feeds it through LoadTicketTypes so the documented shape cannot drift from the loader.\n"
            "- skills/boardctl-usage carries the authoring section, not only the `--meta` filing example.\n"
            "- A user with no source access can declare a type and file a conformant row using only README/help output."
        ),
    },
    {
        "id": "DF-BOARDCTL-58",
        "type": "bug",
        "priority": "P3",
        "complexity": 1,
        "title": "P3 — undeclared --meta key warns on types that declare fields but writes silently on types that declare none",
        "reasoning": (
            "Measured 2026-11-17 at HEAD 0179f5b on the same scratch board. `update REL-6 --meta bogusfield=x` on a type whose registration declares "
            "a metadata list prints 'warning: metadata key \"bogusfield\" is not declared by type \"release\" — it was written, but boardctl validate "
            "will report it'. `create BUG-1 --title 'a bug' --type bug --meta severity=high` prints only 'created task BUG-1' and rc=0 — no warning — "
            "yet `boardctl validate` afterwards reports '[warn] BT-098: row BUG-1 (type \"bug\") has undeclared metadata key \"severity\"'. The write "
            "path therefore knows the key is undeclared in both cases but surfaces it only when the type happens to list fields (the warning is "
            "emitted inside the per-declared-field loop at internal/board/write.go:1240-1242, so a type with an empty metadata list never reaches it). "
            "A user filing against a bare builtin type gets a silently non-conformant row and learns about it only on the next validate run."
        ),
        "ac": (
            "- create and update emit the 'metadata key %q is not declared by type %q' warning for ANY undeclared key, including on types whose metadata list is empty.\n"
            "- A test covers create --type bug --meta <undeclared>=x and asserts the warning text appears on stderr/stdout.\n"
            "- validate's BT-098 warning for the same row still fires (the write-time warning is advisory, not a refusal)."
        ),
    },
]


def run(args):
    p = subprocess.run(args, text=True, capture_output=True)
    return p.returncode, (p.stdout or "").strip(), (p.stderr or "").strip()


def main():
    failed = []
    for row in ROWS:
        args = [
            BIN, "create", "-C", BOARD,
            "--id", row["id"], "--title", row["title"],
            "--type", row["type"], "--priority", row["priority"],
            "--complexity", str(row["complexity"]),
            "--reasoning", row["reasoning"],
            "--acceptance-criteria", row["ac"],
            "--status", "pending",
        ]
        rc, out, err = run(args)
        print(f"{row['id']}: rc={rc} out={out[:160]} err={err[:300]}")
        if rc != 0:
            failed.append(row["id"])
    print("FAILED:", failed if failed else "none")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
