#!/usr/bin/env bash
# Run 32 probe 3: refusal battery (comment/parent/decompose/validate),
# BT-105 completion-event evidence id, probe1 tail (withhold release),
# DF-BOARDCTL-061 re-proof, release boundary arm, zero-migration arm.
set -u
BIN=/tmp/df32-boardctl
S=/home/kara/.hermes/cache/scratch/df-run32
B=$S/board3
rm -rf "$B" && mkdir -p "$B"
BD="$B/.coding-hermes/board"
rcp() { echo "rc=$?"; }
step() { echo "=== $* ==="; }
evcount() { wc -l < "$BD/events.jsonl"; }
evmax() { jq -r '.id // empty' "$BD/events.jsonl" 2>/dev/null | sort -n | tail -1; }

$BIN -C "$B" init --project df32p3 >/dev/null 2>&1
$BIN -C "$B" create --id PR-1 --title "parent one" --priority P2 --status pending >/dev/null 2>&1
$BIN -C "$B" create --id CH-1 --title "child one" --parent PR-1 --status pending >/dev/null 2>&1

step "comment refusals — each must rc=1 and append NOTHING"
echo "baseline: events=$(evcount) maxid=$(evmax)"
$BIN -C "$B" comment NOPE-1 --body x 2>&1; rcp
$BIN -C "$B" comment CH-1 --body "   " 2>&1; rcp
$BIN -C "$B" comment CH-1 --body a --body-file /etc/hostname 2>&1; rcp
$BIN -C "$B" comment CH-1 2>&1 | head -2; rcp
$BIN -C "$B" comment CH-1 --body x --reply-to 9999 2>&1; rcp
# reply-to a NON-comment event (task_created id 1)
$BIN -C "$B" comment CH-1 --body x --reply-to 1 2>&1; rcp
# reply-to a comment on a DIFFERENT task
$BIN -C "$B" comment PR-1 --body "on parent" --author p >/dev/null 2>&1
PID=$(jq -r 'select(.event_type=="comment" and .task_id=="PR-1")|.id' "$BD/events.jsonl" | head -1)
$BIN -C "$B" comment CH-1 --body x --reply-to "$PID" 2>&1; rcp
echo "after refusals: events=$(evcount) maxid=$(evmax)  (parent comment added 1: expect events baseline+1)"

step "parent validation refusals (create path)"
$BIN -C "$B" create --id SELF-1 --title s --parent SELF-1 --status pending 2>&1; rcp
$BIN -C "$B" create --id ORPH-1 --title o --parent GHOST-9 --status pending 2>&1; rcp
$BIN -C "$B" update PR-1 --status complete --evidence-run-id df32x --commit-hash 0123456789abcdef0123456789abcdef01234567 >/dev/null 2>&1
$BIN -C "$B" create --id LATE-1 --title l --parent PR-1 --status pending 2>&1; rcp
echo "cycle: CH-1 parent PR-1; re-point PR-1 -> CH-1 (2-node cycle)"
$BIN -C "$B" update PR-1 --parent CH-1 2>&1; rcp
grep -c '"id": *"SELF-1"\|"id":"SELF-1"' "$BD/tasks.jsonl"; echo "(0 = refusal wrote nothing)"

step "decompose refusals"
$BIN -C "$B" decompose PR-1 --count 2 2>&1; rcp
$BIN -C "$B" decompose CH-1 --count 0 2>&1; rcp
$BIN -C "$B" create --id CH-9 --title "pre-existing" --status pending >/dev/null 2>&1
$BIN -C "$B" decompose CH-1 --count 3 --first-id CH-8 2>&1; rcp
echo "(CH-8 should exist, CH-9 collision stops loop; children created remain)"
jq -r 'select(.id|startswith("CH-"))|.id' "$BD/tasks.jsonl" | tr '\n' ' '; echo

step "validate catches hand-edited dangling parent + cycle"
B2=$S/board3v
rm -rf "$B2" && mkdir -p "$B2"
$BIN -C "$B2" init --project df32v >/dev/null 2>&1
$BIN -C "$B2" create --id VA-1 --title a --status pending >/dev/null 2>&1
$BIN -C "$B2" create --id VA-2 --title b --status pending >/dev/null 2>&1
$BIN -C "$B2" create --id VA-3 --title c --parent VA-9 --status pending >/dev/null 2>&1; echo "create w/ dangling parent rc=$? (expect refusal)"
# hand-edit a dangling parent into VA-2
VB=$B2/.coding-hermes/board/tasks.jsonl
sed -i 's/{"id": *"VA-2"/{"id": "VA-2", "parent": "VA-9"/' "$VB" 2>/dev/null || true
jq -c 'select(.id=="VA-2")' "$VB" | head -1
$BIN -C "$B2" validate 2>&1 | grep -i 'parent' | head -4
$BIN -C "$B2" validate >/dev/null 2>&1; echo "validate rc=$?"
$BIN -C "$B2" validate --fail-on parent-cycle >/dev/null 2>&1; echo "fail-on parent-cycle rc=$?"

step "BT-105: completion event carries evidence run id"
B4=$S/board4
rm -rf "$B4" && mkdir -p "$B4"
cd /home/kara/coding-hermes-boardctl || exit 9
SHA=$(git rev-parse HEAD)
$BIN -C "$B4" init --project df32b4 >/dev/null 2>&1
$BIN -C "$B4" create --id EV-1 --title "evidence probe" --status pending >/dev/null 2>&1
$BIN -C "$B4" update EV-1 --status complete --evidence-run-id abc123def456 --commit-hash "$SHA" 2>&1; rcp
grep -o '"event_type": *"task_completed"[^}]*' "$B4/.coding-hermes/board/events.jsonl" | head -1
jq -c 'select(.event_type=="task_completed")|.detail' "$B4/.coding-hermes/board/events.jsonl" | head -2

step "probe1 tail: complete all children -> parent released in ready"
B1=$S/board1
$BIN -C "$B1" update DF-EX-101 --status complete --evidence-run-id df32r1 --commit-hash "$SHA" >/dev/null 2>&1; echo "c101 rc=$?"
jq -c 'select(.event_type=="task_completed" and .task_id=="DF-EX-101")|.detail' "$B1/.coding-hermes/board/events.jsonl" | head -1
$BIN -C "$B1" update DF-EX-102 --status complete --evidence-run-id df32r2 --commit-hash "$SHA" >/dev/null 2>&1
$BIN -C "$B1" update DF-EX-103 --status complete --evidence-run-id df32r3 --commit-hash "$SHA" >/dev/null 2>&1
$BIN -C "$B1" ready 2>&1 | head -14

step "DF-BOARDCTL-061 re-proof: --initiative singular key no longer warns"
B5=$S/board5
rm -rf "$B5" && mkdir -p "$B5"
$BIN -C "$B5" init --project df32b5 >/dev/null 2>&1
$BIN -C "$B5" create --id IN-1 --title "initiative probe" --initiative myprog --status pending >/dev/null 2>&1
jq -c 'select(.id=="IN-1")|{initiative, initiative_refs}' "$B5/.coding-hermes/board/tasks.jsonl"
$BIN -C "$B5" validate 2>&1 | tail -2; echo "validate rc=$?"

step "release boundary: deployed v0.6.0 refuses comment/decompose"
~/.local/bin/boardctl -C "$B5" comment IN-1 --body x 2>&1 | head -2; echo "v060 comment rc=${PIPESTATUS[0]}"
~/.local/bin/boardctl -C "$B5" decompose IN-1 --count 2 2>&1 | head -2

step "zero-migration: real repo board copy, HEAD vs v0.6.0 validate/stats"
B6=$S/board6
rm -rf "$B6" && mkdir -p "$B6/.coding-hermes"
cp -r /home/kara/coding-hermes-boardctl/.coding-hermes/board "$B6/.coding-hermes/board"
$BIN -C "$B6" validate > "$S/validate-head.out" 2>&1; echo "HEAD validate rc=$?"
~/.local/bin/boardctl -C "$B6" validate > "$S/validate-v060.out" 2>&1; echo "v060 validate rc=$?"
diff <(grep -c 'warning' "$S/validate-head.out") <(grep -c 'warning' "$S/validate-v060.out") >/dev/null && echo "warning counts same" || { echo "warning counts differ:"; grep -c warning "$S/validate-head.out"; grep -c warning "$S/validate-v060.out"; }
$BIN -C "$B6" stats > "$S/stats-head.out" 2>&1
~/.local/bin/boardctl -C "$B6" stats > "$S/stats-v060.out" 2>&1
diff "$S/stats-head.out" "$S/stats-v060.out" >/dev/null && echo "stats IDENTICAL" || diff "$S/stats-head.out" "$S/stats-v060.out" | head -6
echo "PROBE3 DONE"
