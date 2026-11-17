#!/usr/bin/env python3
"""File DF-BOARDCTL-59 (coverage-floor gate red at HEAD with a self-contradictory message)."""
import subprocess
import sys

BOARD = "/home/kara/coding-hermes-boardctl"
BIN = "/home/kara/.hermes/cache/scratch/df-2026-11-17/boardctl"

ROW = {
    "id": "DF-BOARDCTL-59",
    "type": "bug",
    "priority": "P2",
    "complexity": 2,
    "title": "P2 — make cover-check FAILS at HEAD (79.9867% vs floor 80.0) and prints \"80.0% is below floor 80.0\"",
    "reasoning": (
        "Measured 2026-11-17. On this host (go1.26.11 linux/amd64, the toolchain go.mod resolves) `make cover-check` at HEAD 0179f5b exits 1: "
        "\"covercheck: total 80.0% of statements (floor 80.0%)\" then \"covercheck: FAIL total coverage 80.0% is below floor 80.0%\". "
        "The two numbers the gate prints are EQUAL, so the operator cannot read how far below the tree is; the raw total computed from the same "
        "profile (awk over coverage.out, statements=10513 covered=8409) is 79.986683% — a 0.0133 percentage-point miss that the %.1f format "
        "rounds up to 80.0 (internal/covercheck/covercheck.go:258 compares exactly, :263 formats %.1f). "
        "The same step is RED on a fresh machine: JIT agent b11919a8 (bunker-mvp), act run of .github/workflows/ci.yml, step 'Coverage' "
        "(run: make cover-check) failed after 15.129s while Build/Vet/Gofmt/Version-check/Spec-check/Vendor-check/Vulnerability-check all passed "
        "(per-cell log ci.log pulled from the agent before destroy). "
        "AND the verdict is not a function of the tree: GitHub CI run 43810100006 (push of 20124ca, 2026-11-16T05:25Z) is green with the same "
        "Coverage step, and `git diff --stat 20124ca..HEAD -- cmd internal Makefile .github` is EMPTY, so the identical product tree passes on the "
        "GitHub runner and fails here and on the JIT agent. The module total sits ON the boundary, so a toolchain/host difference of 0.013pp flips "
        "the gate — precisely the class a floor at the exact achieved value produces. Lowest-coverage files in the same profile: "
        "cmd/boardctl/evidence.go 16.0%, cmd/boardctl/hook.go 52.9%, internal/describe/openapi.go 63.1%, internal/board/initiative_refs.go 64.6%, "
        "cmd/boardctl/initiatives_cmd.go 66.0% — the recently landed BT-098/BT-097 surface is where the margin is missing. "
        "NOT a release blocker by itself (the shipped artifact is unaffected) but it makes the documented five-gate block red for any operator who "
        "runs it on a host that lands on the wrong side of the boundary, and its failure text is not honest about the number it failed on."
    ),
    "ac": (
        "- The failure line prints the exact measured total (at least 3 decimals, e.g. 79.987%) beside the floor, so 'X% is below floor Y%' never shows two equal numbers.\n"
        "- A regression test asserts the formatted failure message contains a value strictly less than the floor as printed (no rounding collision with the floor).\n"
        "- The gate's verdict is host-stable at a fixed tree: either the module total is raised clear of the floor (cover the uncovered BT-098 paths — "
        "internal/board/initiative_refs.go, cmd/boardctl/initiatives_cmd.go, cmd/boardctl/evidence.go) or the floor/margin is made explicit so a 0.01pp toolchain difference cannot flip it.\n"
        "- `make cover-check` at HEAD passes on a fresh JIT agent (the same cell that is red in this run) and the evidence row names the measured total."
    ),
}


def main():
    args = [
        BIN, "create", "-C", BOARD,
        "--id", ROW["id"], "--title", ROW["title"],
        "--type", ROW["type"], "--priority", ROW["priority"],
        "--complexity", str(ROW["complexity"]),
        "--reasoning", ROW["reasoning"],
        "--acceptance-criteria", ROW["ac"],
        "--status", "pending",
    ]
    p = subprocess.run(args, text=True, capture_output=True)
    print("rc=", p.returncode, "out=", (p.stdout or "").strip()[:200], "err=", (p.stderr or "").strip()[:300])
    return p.returncode


if __name__ == "__main__":
    sys.exit(main())
