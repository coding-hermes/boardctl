#!/usr/bin/env bash
# Run 32 perf: headline ops warm+cold. Mutating verbs measured with a restore
# loop (restore excluded from timing). Synthetic 5k-row parent/child board for
# the ready/validate graph pass. All numbers are UPPER bounds (shared host).
set -u
BIN=/tmp/df32-boardctl
S=/home/kara/.hermes/cache/scratch/df-run32
P=$S/perf
rm -rf "$P" && mkdir -p "$P"
step() { echo "=== $* ==="; }

HAVE_HYPERFINE=no
command -v hyperfine >/dev/null 2>&1 && HAVE_HYPERFINE=yes
echo "hyperfine: $HAVE_HYPERFINE"
uptime

# --- fixture: real board copy (300 rows)
mkdir -p "$P/real/.coding-hermes"
cp -r /home/kara/coding-hermes-boardctl/.coding-hermes/board "$P/real/.coding-hermes/board"
RB="$P/real"

# --- fixture: synthetic 5k-row board: 500 parents x 9 parallel children + metadata
mkdir -p "$P/big/.coding-hermes"
$BIN -C "$P/big" init --project perf5k >/dev/null 2>&1
python3 - "$P/big/.coding-hermes/board/tasks.jsonl" <<'PYEOF'
import json, sys
path = sys.argv[1]
lines = []
n = 0
for p in range(1, 501):
    pid = f"PERF-{p:04d}"
    lines.append({"id": pid, "title": f"parent {p}", "status": "pending", "priority": "P2",
                  "depends_on": [], "created_at": "2026-11-26T00:00:00Z", "updated_at": "2026-11-26T00:00:00Z",
                  "detail": {"metadata": {"expected_files": ["a.go", "b.go"], "verify_steps": ["go test ./..."]}}})
    for c in range(1, 10):
        n += 1
        cid = f"PERF-{5000 + p * 10 + c:05d}"
        lines.append({"id": cid, "title": f"child {p}.{c}", "status": "pending", "priority": "P3",
                      "parent": pid, "depends_on": [], "created_at": "2026-11-26T00:00:00Z", "updated_at": "2026-11-26T00:00:00Z"})
with open(path, "a") as f:
    for row in lines:
        f.write(json.dumps(row, separators=(",", ":")) + "\n")
print(f"wrote {len(lines)} synthetic rows")
PYEOF
wc -l "$P/big/.coding-hermes/board/tasks.jsonl"

# --- fixture: deep sequential chain board (1k rows, depth 999) for cycle-walk worst case
mkdir -p "$P/deep/.coding-hermes"
$BIN -C "$P/deep" init --project perfdeep >/dev/null 2>&1
python3 - "$P/deep/.coding-hermes/board/tasks.jsonl" <<'PYEOF'
import json, sys
path = sys.argv[1]
with open(path, "a") as f:
    prev = None
    for i in range(1, 1001):
        row = {"id": f"DEEP-{i:04d}", "title": f"chain {i}", "status": "pending", "priority": "P2",
               "depends_on": ([prev] if prev else []), "parent": ("DEEP-0001" if i > 1 else None),
               "created_at": "2026-11-26T00:00:00Z", "updated_at": "2026-11-26T00:00:00Z"}
        if row["parent"] is None:
            del row["parent"]
        f.write(json.dumps(row, separators=(",", ":")) + "\n")
        prev = row["id"]
print("wrote 1000 deep chain rows")
PYEOF

timeit() { # label, N, cmd...
  local label="$1"; local n="$2"; shift 2
  local total=0 t0 t1
  for i in $(seq 1 "$n"); do
    t0=$(date +%s%N)
    "$@" >/dev/null 2>&1
    t1=$(date +%s%N)
    total=$(( total + (t1 - t0) / 1000 ))  # microseconds
  done
  echo "$label: $(( total / n )) us avg over $n runs"
}

step "REAL board (300 rows) warm"
timeit "validate" 15 $BIN -C "$RB" validate
timeit "stats" 15 $BIN -C "$RB" stats
timeit "ready" 15 $BIN -C "$RB" ready
timeit "list --json" 15 $BIN -C "$RB" list --json

step "5k-row parent/child board (500 parents, 4500 children) warm"
timeit "validate" 10 $BIN -C "$P/big" validate
timeit "ready" 10 $BIN -C "$P/big" ready
timeit "stats" 10 $BIN -C "$P/big" stats
timeit "show parent (rollup)" 10 $BIN -C "$P/big" show PERF-0250
timeit "next" 10 $BIN -C "$P/big" next

step "1k deep chain (parent+depends depth 999) warm"
timeit "validate" 5 $BIN -C "$P/deep" validate
timeit "ready" 5 $BIN -C "$P/deep" ready
timeit "next" 5 $BIN -C "$P/deep" next

step "comment append cost (restore-loop, 10 runs)"
CB=$P/cboard
rm -rf "$CB" && mkdir -p "$CB"
$BIN -C "$CB" init --project perfcomment >/dev/null 2>&1
$BIN -C "$CB" create --id CM-1 --title "comment perf" --status pending >/dev/null 2>&1
cp "$CB/.coding-hermes/board/events.jsonl" "$P/events.golden"
total=0
for i in $(seq 1 10); do
  cp "$P/events.golden" "$CB/.coding-hermes/board/events.jsonl"
  t0=$(date +%s%N)
  $BIN -C "$CB" comment CM-1 --body "perf probe $i" --author perf >/dev/null 2>&1
  t1=$(date +%s%N)
  total=$(( total + (t1 - t0) / 1000 ))
done
echo "comment: $(( total / 10 )) us avg over 10 runs"

step "decompose cost (fresh parent per run, 5 runs)"
DB=$P/dboard
rm -rf "$DB" && mkdir -p "$DB"
$BIN -C "$DB" init --project perfdecomp >/dev/null 2>&1
total=0
for i in $(seq 1 5); do
  rm -rf "$DB/.coding-hermes/board"
  $BIN -C "$DB" init --project perfdecomp >/dev/null 2>&1
  $BIN -C "$DB" create --id DC-$i --title "decompose perf $i" --priority P2 --expected-files "a.go,b.go" --status pending >/dev/null 2>&1
  t0=$(date +%s%N)
  $BIN -C "$DB" decompose DC-$i --count 5 >/dev/null 2>&1
  t1=$(date +%s%N)
  total=$(( total + (t1 - t0) / 1000 ))
done
echo "decompose-5-children: $(( total / 5 )) us avg over 5 runs (incl 5 guarded creates + 1 event)"

step "cold first-call (fresh copy, single run each)"
CC=$P/coldcopy
rm -rf "$CC" && mkdir -p "$CC/.coding-hermes"
cp -r /home/kara/coding-hermes-boardctl/.coding-hermes/board "$CC/.coding-hermes/board"
for verb in validate stats ready; do
  t0=$(date +%s%N); $BIN -C "$CC" $verb >/dev/null 2>&1; t1=$(date +%s%N)
  echo "cold $verb: $(( (t1 - t0) / 1000 )) us"
  rm -rf "$CC/.coding-hermes/board"; cp -r /home/kara/coding-hermes-boardctl/.coding-hermes/board "$CC/.coding-hermes/board"
done
echo "PERF DONE"
