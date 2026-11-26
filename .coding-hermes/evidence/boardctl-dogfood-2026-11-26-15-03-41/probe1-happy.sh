#!/usr/bin/env bash
# Run 32 probe 1: BT-099 happy path — init, create parent with decomposition
# metadata, decompose parallel + sequential, comment threads, rollups,
# ready/next parent suppression. Every step prints rc. Board: $B/board1
set -u
BIN=/tmp/df32-boardctl
B=/home/kara/.hermes/cache/scratch/df-run32/board1
rm -rf "$B" && mkdir -p "$B"
cd "$B" || exit 9

step() { echo "=== $* ==="; }
rcp() { echo "rc=$?"; }

step "init"
time -p $BIN -C "$B" init --project df32probe --namespace scratch 2>&1; rcp

step "create parent DF-EX-100 with expected-files + verify-steps + tags"
$BIN -C "$B" create --id DF-EX-100 --title "Ship the widget refactor" \
  --priority P2 --complexity 3 --capability-tags go,board \
  --expected-files "cmd/widget.go, internal/widget/render.go" \
  --verify-steps "go test ./..." --verify-steps "prove render goldens" \
  --reasoning "parent shell" --acceptance-criteria "AC1 widget renders" \
  --status pending 2>&1; rcp

step "stored parent row (jq keys + detail.metadata)"
jq -c 'select(.id=="DF-EX-100") | {keys: (keys), detail: .detail}' "$B/.coding-hermes/board/tasks.jsonl" 2>&1 | head -5

step "decompose DF-EX-100 --count 3 --first-id DF-EX-101 (default parallel)"
time -p $BIN -C "$B" decompose DF-EX-100 --count 3 --first-id DF-EX-101 2>&1; rcp

step "children shape: parent key, depends_on, titles, inherited fields"
jq -c 'select(.id|startswith("DF-EX-10")) | {id, parent, depends_on, title, priority, complexity, capability_tags, type, detail}' "$B/.coding-hermes/board/tasks.jsonl" 2>&1

step "parent audit event decomposed_into"
grep -o '"decomposed_into[^}]*}' "$B/.coding-hermes/board/events.jsonl" | head -2
step "task_created events per child"
for id in DF-EX-101 DF-EX-102 DF-EX-103; do
  echo "$id created-events: $(grep -c "\"task_id\": *\"$id\"" "$B/.coding-hermes/board/events.jsonl" 2>/dev/null || grep -c "\"task_id\":\"$id\"" "$B/.coding-hermes/board/events.jsonl")"
done

step "second parent DF-EX-200 + sequential decompose (cost line?)"
$BIN -C "$B" create --id DF-EX-200 --title "Sequential shell" --priority P3 --status pending 2>&1; rcp
$BIN -C "$B" decompose DF-EX-200 --count 3 --sequential --first-id DF-EX-201 2>&1; rcp
jq -c 'select(.id|startswith("DF-EX-20")) | {id, parent, depends_on}' "$B/.coding-hermes/board/tasks.jsonl"

step "comment thread on DF-EX-101: two top-level + one reply"
BEFORE_TASKS=$(md5sum "$B/.coding-hermes/board/tasks.jsonl" | cut -d' ' -f1)
BEFORE_HDR=$($BIN -C "$B" header --json 2>/dev/null)
time -p $BIN -C "$B" comment DF-EX-101 --body "taking this step first" --author bane 2>&1; rcp
$BIN -C "$B" comment DF-EX-101 --body "second thought" --author dogfood 2>&1; rcp
EID=$(jq -r 'select(.event_type=="comment" and .task_id=="DF-EX-101") | .id' "$B/.coding-hermes/board/events.jsonl" | head -1)
echo "first comment event id: $EID"
$BIN -C "$B" comment DF-EX-101 --body "agreed, replying" --author foreman --reply-to "$EID" 2>&1; rcp
AFTER_TASKS=$(md5sum "$B/.coding-hermes/board/tasks.jsonl" | cut -d' ' -f1)
AFTER_HDR=$($BIN -C "$B" header --json 2>/dev/null)
echo "tasks.jsonl md5 before=$BEFORE_TASKS after=$AFTER_TASKS same=$([ "$BEFORE_TASKS" = "$AFTER_TASKS" ] && echo YES || echo NO)"
echo "header before=$BEFORE_HDR"
echo "header after =$AFTER_HDR"

step "show DF-EX-101 renders COMMENTS thread"
$BIN -C "$B" show DF-EX-101 2>&1; rcp

step "show --json thread array"
$BIN -C "$B" show DF-EX-101 --json 2>/dev/null | jq -c '{comments: (.comments // .thread // "ABSENT-KEY")}' 2>&1 | head -3

step "show parent DF-EX-100 rollup (CHILDREN / CHILD DEPS)"
$BIN -C "$B" show DF-EX-100 2>&1; rcp

step "ready — parent withheld while children open+ready"
$BIN -C "$B" ready 2>&1; rcp
step "next — must NOT pick a withheld parent"
$BIN -C "$B" next 2>&1; rcp

step "complete child DF-EX-101 with evidence (TR-251), then ready again"
$BIN -C "$B" update DF-EX-101 --status complete --evidence-run-id df32run001 --commit-hash $(cd /home/kara/coding-hermes-boardctl && git rev-parse HEAD) 2>&1; rcp
$BIN -C "$B" ready 2>&1 | head -20; rcp

step "validate clean"
$BIN -C "$B" validate 2>&1 | tail -3; rcp
echo "PROBE1 DONE"
