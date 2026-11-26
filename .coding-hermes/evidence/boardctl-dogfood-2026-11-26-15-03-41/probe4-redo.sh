#!/usr/bin/env bash
# Run 32 probe 4: redo broken arms — bare comment rc, next-id width parity,
# validate dangling-parent + parent-cycle (jq-inserted, properly matched).
set -u
BIN=/tmp/df32-boardctl
S=/home/kara/.hermes/cache/scratch/df-run32
step() { echo "=== $* ==="; }

step "bare comment (no body flags) true rc, no pipe"
B=$S/board3
$BIN -C "$B" comment CH-1 > /tmp/df32-bare.out 2>&1
echo "rc=$?"; cat /tmp/df32-bare.out

step "comment --body-file happy path"
printf 'body from file\nsecond line' > /tmp/df32-body.txt
$BIN -C "$B" comment CH-1 --body-file /tmp/df32-body.txt --author fileprobe > /tmp/df32-bf.out 2>&1
echo "rc=$?"; cat /tmp/df32-bf.out
jq -r 'select(.event_type=="comment" and .task_id=="CH-1")|.detail' "$B/.coding-hermes/board/events.jsonl" | tail -1

step "next-id width parity on CH family (CH-1..CH-9, CH-010, CH-011 present)"
$BIN -C "$B" next-id --family CH 2>&1; echo "rc=$?"
$BIN -C "$B" next-id --family CH --width 1 2>&1; echo "rc=$?"

step "validate: dangling parent (jq-inserted)"
B7=$S/board7
rm -rf "$B7" && mkdir -p "$B7"
$BIN -C "$B7" init --project df32b7 >/dev/null 2>&1
$BIN -C "$B7" create --id VA-1 --title a --status pending >/dev/null 2>&1
$BIN -C "$B7" create --id VA-2 --title b --status pending >/dev/null 2>&1
$BIN -C "$B7" create --id VA-3 --title c --status pending >/dev/null 2>&1
VB=$B7/.coding-hermes/board/tasks.jsonl
jq -c 'if .id=="VA-2" then . + {parent:"VA-9"} else . end' "$VB" > "$VB.tmp" && mv "$VB.tmp" "$VB"
echo "VA-2 now:"; jq -c 'select(.id=="VA-2")|{id,parent}' "$VB"
$BIN -C "$B7" validate > /tmp/df32-v7.out 2>&1
echo "validate rc=$?"; grep -i 'parent\|RESULT\|error' /tmp/df32-v7.out | head -5
$BIN -C "$B7" validate --fail-on parent-cycle >/dev/null 2>&1; echo "fail-on parent-cycle rc=$?"

step "validate: 2-node cycle (VA-1<->VA-3 hand-edited)"
jq -c 'if .id=="VA-1" then . + {parent:"VA-3"} elif .id=="VA-3" then . + {parent:"VA-1"} else . end' "$VB" > "$VB.tmp" && mv "$VB.tmp" "$VB"
$BIN -C "$B7" validate > /tmp/df32-v7b.out 2>&1
echo "validate rc=$?"; grep -i 'cycle\|parent\|RESULT' /tmp/df32-v7b.out | head -6
$BIN -C "$B7" validate --fail-on parent-cycle >/dev/null 2>&1; echo "fail-on parent-cycle rc=$?"

step "sweep-status/doctor tolerate parent rows (no crash on new key)"
$BIN -C "$B7" doctor > /tmp/df32-doc.out 2>&1; echo "doctor rc=$?"; tail -3 /tmp/df32-doc.out
echo "PROBE4 DONE"
