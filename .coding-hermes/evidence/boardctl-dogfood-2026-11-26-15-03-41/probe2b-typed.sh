#!/usr/bin/env bash
# Run 32 probe 2b: typed-parent decompose inheritance vs untyped, show --json
# full shape, --meta survival across updates (DF-56 class on BT-099 arrays),
# untyped --meta drop asymmetry vs update.
set -u
BIN=/tmp/df32-boardctl
B=/home/kara/.hermes/cache/scratch/df-run32/board2b
rm -rf "$B" && mkdir -p "$B"
cd "$B" || exit 9
BD="$B/.coding-hermes/board"
rcp() { echo "rc=$?"; }
step() { echo "=== $* ==="; }

$BIN -C "$B" init --project df32p2b >/dev/null 2>&1

step "author types.jsonl (schema from testdata) with expected_files+verify_steps declared"
cat > "$BD/types.jsonl" <<'EOF'
{"slug":"release","display":"Release","rev":1,"template":{"title_prefix":"[REL] ","default_priority":"P1","ac_skeleton":"- [ ] artifact built"},"metadata":[{"name":"channel","type":"enum","required":true,"values":["stable","beta","nightly"],"description":"ship channel"},{"name":"expected_files","type":"list","required":false,"description":"files touched"},{"name":"verify_steps","type":"list","required":false,"description":"verification"},{"name":"signoffs","type":"list","required":false,"description":"signoffs"}]}
EOF
$BIN -C "$B" types > /tmp/df32-types.out 2>&1
echo "types rc=$?"; head -6 /tmp/df32-types.out

step "typed parent REL-100: channel enum + expected-files + verify-steps + signoffs list"
$BIN -C "$B" create --id REL-100 --title "Cut release" --type release --meta channel=beta \
  --meta 'signoffs=ana@x,bo@x' \
  --expected-files "goreleaser.yml,README.md" --verify-steps "make release-check" \
  --status pending > /tmp/df32-c100.out 2>&1
echo "create rc=$?"; cat /tmp/df32-c100.out
jq -c 'select(.id=="REL-100")|{type,priority,title,detail:.detail.metadata}' "$BD/tasks.jsonl"

step "decompose typed parent x2 (default parallel)"
$BIN -C "$B" decompose REL-100 --count 2 > /tmp/df32-dec.out 2>&1
echo "decompose rc=$?"; cat /tmp/df32-dec.out
jq -c 'select(.id|startswith("REL-1"))|{id,type,detail:.detail.metadata}' "$BD/tasks.jsonl"

step "CONTRAST: untyped parent UNT-100 with same arrays -> decompose"
$BIN -C "$B" create --id UNT-100 --title "Untyped shell" --priority P2 \
  --expected-files "a.go,b.go" --verify-steps "go test ./..." --status pending >/dev/null 2>&1
$BIN -C "$B" decompose UNT-100 --count 2 --first-id UNT-101 > /tmp/df32-dec2.out 2>&1
echo "decompose rc=$?"; cat /tmp/df32-dec2.out
jq -c 'select(.id|startswith("UNT-1"))|{id,detail}' "$BD/tasks.jsonl"

step "show --json FULL shape on REL-101 (has comment)"
$BIN -C "$B" comment REL-101 --body "hello json" --author probe >/dev/null 2>&1
$BIN -C "$B" show REL-101 --json > /tmp/df32-showjson.out 2>&1
echo "rc=$?"
echo "--- doc kinds (jq -s):"; jq -s '[.[] | type]' /tmp/df32-showjson.out
echo "--- first doc keys:"; jq -s '.[0] | if type=="object" then (keys|join(",")) else . end' /tmp/df32-showjson.out 2>&1 | head -3
echo "--- second doc:"; jq -s '.[1]' /tmp/df32-showjson.out 2>&1 | head -12
echo "--- v0.6.0 show --json doc kinds on same row:"
~/.local/bin/boardctl -C "$B" show REL-101 --json > /tmp/df32-showjson-v060.out 2>&1
echo "v060 rc=$?"; jq -s '[.[] | type]' /tmp/df32-showjson-v060.out 2>&1

step "DF-56-class: update --meta channel=stable on REL-100; signoffs list + expected_files survive?"
$BIN -C "$B" update REL-100 --meta channel=stable > /tmp/df32-u1.out 2>&1
echo "update rc=$?"; cat /tmp/df32-u1.out
jq -c 'select(.id=="REL-100")|.detail.metadata' "$BD/tasks.jsonl"

step "update --note on REL-100; arrays survive?"
$BIN -C "$B" update REL-100 --note "probe note" >/dev/null 2>&1; echo "rc=$?"
jq -c 'select(.id=="REL-100")|.detail.metadata' "$BD/tasks.jsonl"

step "update --expected-files REPLACE on REL-101"
$BIN -C "$B" update REL-101 --expected-files "new.go" >/dev/null 2>&1; echo "rc=$?"
jq -c 'select(.id=="REL-101")|.detail.metadata' "$BD/tasks.jsonl"

step "untyped create --meta foo=bar: warning? silent drop?"
$BIN -C "$B" create --id UNT-2 --title "untyped meta probe" --meta foo=bar --status pending > /tmp/df32-u2.out 2>&1
echo "create rc=$?"; cat /tmp/df32-u2.out
jq -c 'select(.id=="UNT-2")|{detail}' "$BD/tasks.jsonl"

step "untyped update --meta foo=baz on UNT-2: warning? silent?"
$BIN -C "$B" update UNT-2 --meta foo=baz > /tmp/df32-u3.out 2>&1
echo "update rc=$?"; cat /tmp/df32-u3.out
jq -c 'select(.id=="UNT-2")|{detail}' "$BD/tasks.jsonl"

step "typed create --meta UNDECLARED key extra=1 on release type"
$BIN -C "$B" create --id REL-200 --title "extra key probe" --type release --meta channel=beta --meta extra=1 --status pending > /tmp/df32-u4.out 2>&1
echo "create rc=$?"; cat /tmp/df32-u4.out
jq -c 'select(.id=="REL-200")|{detail:.detail.metadata}' "$BD/tasks.jsonl" 2>/dev/null

step "update --parent re-point + removal on REL-101"
$BIN -C "$B" create --id REL-300 --title "other parent" --type release --meta channel=beta --status pending >/dev/null 2>&1
$BIN -C "$B" update REL-101 --parent REL-300 >/dev/null 2>&1; echo "repoint rc=$?"
jq -c 'select(.id=="REL-101")|{parent}' "$BD/tasks.jsonl"
$BIN -C "$B" update REL-101 --parent "" >/dev/null 2>&1; echo "remove rc=$?"
jq -c 'select(.id=="REL-101")' "$BD/tasks.jsonl" | jq -c '{parent: (if has("parent") then .parent else "KEY-ABSENT" end)}'
echo "PROBE2B DONE"
