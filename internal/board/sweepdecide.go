package board

import (
	"strings"
)

// REVIEW-BOARDCTL-001 (--decide): machine-checkable decisions for the
// off-vocabulary guard_result/ci_result values the fleet census found on
// live boards (2026-09-30): prose like "PASS 5/5", "PASS (secrets)",
// "pipeline 1166 SUCCESS all 6 jobs (lint GREEN first time)", "N/A (bash
// script; local bash -n + check PASS)" — every one of them carries a
// canonical token, but NONE of them is writable, because the write
// vocabulary {PASS,FAIL,SKIP}/{GREEN,RED,SKIP} is still enforced on every
// surface (BT-007, unchanged by this feature).
//
// The table below is deliberately CONTENT RULES, not free-form guessing.
// Precedence, evaluated top to bottom:
//
//  1. AMBIGUITY GATE — a value whose standalone tokens include both PASS
//     and FAIL is refused regardless of position ("PASS but really FAIL"
//     must not become PASS just because PASS happens to lead): ambiguous
//     prose needs a human, the same refuse-to-guess rule as statuses.
//  2. FIRST TOKEN (the value's first field, upper-cased, surrounding
//     punctuation stripped): PASS|OK -> PASS, FAIL|ERROR -> FAIL,
//     GREEN -> GREEN, RED -> RED.
//  3. ANYWHERE rules: the standalone word SKIP, or the literal token
//     N/A, -> SKIP (N/A means the check does not apply).
//  4. STANDALONE map, only when no first-token rule matched:
//     SUCCESS -> GREEN for ci_result / PASS for guard_result.
//
// PENDING is never rewritten — pending means the check has not run, so
// SKIP would be a lie; it falls through every rule and stays an explicit
// decision, as does anything the rules do not match.
//
// The exact table is rendered by ResultDecideRuleDoc and surfaced in the
// sweep-status help text; the tests in sweep_decide_test.go pin every row
// of it, including the refusal arms.

// ResultDecideRuleDoc is the one-glance decision table shown in the
// sweep-status help. Keep it in lockstep with DecideResultValue (the unit
// tests cross-check the documented tokens against the function's arms).
const ResultDecideRuleDoc = "decide off-vocab guard_result/ci_result values by rule: " +
	"a value naming both PASS and FAIL stays an explicit decision (ambiguous); " +
	"else first token PASS|OK -> PASS, FAIL|ERROR -> FAIL, GREEN -> GREEN, RED -> RED; " +
	"else contains standalone SKIP or the literal N/A -> SKIP; " +
	"else contains standalone SUCCESS -> GREEN (ci_result) / PASS (guard_result); " +
	"PENDING and anything unmatched stays an explicit decision"

// resultTokens splits a raw result value into standalone upper-cased
// tokens: runs of letters/digits/underscore/slash, so "N/A" survives as
// ONE token while "(PASS)," still yields PASS and "5/5" yields a token no
// rule can mistake for a verdict.
func resultTokens(v string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.FieldsFunc(v, func(r rune) bool {
		isWord := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
		return !isWord && r != '_' && r != '/'
	}) {
		out[strings.ToUpper(f)] = true
	}
	return out
}

// resultFirstToken is the value's leading field with surrounding
// punctuation stripped — "(PASS)" and "PASS," read as PASS. A slash is
// NOT stripped: "N/A" as a first token must stay N/A.
func resultFirstToken(v string) string {
	fields := strings.Fields(strings.TrimSpace(v))
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(strings.Trim(fields[0], "()[],.;:'\""))
}

// DecideResultValue applies the --decide content rules to one present,
// off-vocabulary guard_result/ci_result value. decided=true means the
// value maps to a canonical vocabulary member (see the precedence table in
// the package comment); decided=false leaves the row an explicit decision
// (ambiguity, PENDING, anything unmatched). The returned reason quotes the
// rule that fired (or the refusal cause), for the report and the JSON
// decision entries.
func DecideResultValue(column, raw string) (canonical string, decided bool, reason string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false, "empty value is not a result"
	}
	tokens := resultTokens(trimmed)
	// 1. Ambiguity gate first — refuse-to-guess beats the first-token rule.
	if tokens["PASS"] && tokens["FAIL"] {
		return "", false, "value names both PASS and FAIL — ambiguous, needs an explicit decision"
	}
	// 2. First-token rules.
	switch resultFirstToken(trimmed) {
	case "PASS", "OK":
		return "PASS", true, "first token -> PASS"
	case "FAIL", "ERROR":
		return "FAIL", true, "first token -> FAIL"
	case "GREEN":
		return "GREEN", true, "first token -> GREEN"
	case "RED":
		return "RED", true, "first token -> RED"
	}
	// 3. Anywhere rules.
	switch {
	case tokens["SKIP"]:
		return "SKIP", true, `contains standalone "SKIP" -> SKIP`
	case tokens["N/A"]:
		return "SKIP", true, `contains the literal "N/A" -> SKIP (check does not apply)`
	}
	// 4. Standalone map (no first-token rule matched).
	if tokens["SUCCESS"] {
		if column == "ci_result" {
			return "GREEN", true, `contains standalone "SUCCESS" -> GREEN (ci_result)`
		}
		return "PASS", true, `contains standalone "SUCCESS" -> PASS (guard_result)`
	}
	if tokens["OK"] {
		return "PASS", true, `contains standalone "OK" -> PASS`
	}
	return "", false, "no decision rule matches — needs an explicit decision"
}
