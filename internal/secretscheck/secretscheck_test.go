// Package secretscheck pins the gitleaks sk-api-key regex boundary so the
// mid-word false positive on prose like "task-complete-unavailable" cannot
// regress. BT-83.
package secretscheck

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// readSKRuleRegex extracts the sk-api-key rule's regex literal from the
// repo-root .gitleaks.toml. The file layout is stable (one [[rules]] block
// with id = "sk-api-key" followed by a regex = ”'...”' line); the parser
// here is intentionally narrow — it exists only to keep the regression test
// honest about which regex is actually deployed.
func readSKRuleRegex(t *testing.T) string {
	t.Helper()
	// Walk up from the test's working directory to find .gitleaks.toml.
	// `go test ./...` runs with the package dir as cwd, so ../../.gitleaks.toml
	// from internal/secretscheck/ reaches the repo root.
	candidates := []string{"../../.gitleaks.toml", ".gitleaks.toml"}
	var raw []byte
	var err error
	for _, p := range candidates {
		raw, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("read .gitleaks.toml: %v", err)
	}
	// Locate the sk-api-key rule block, then the regex = '''...''' line inside it.
	body := string(raw)
	idMarker := `id = "sk-api-key"`
	idIdx := strings.Index(body, idMarker)
	if idIdx < 0 {
		t.Fatalf("sk-api-key rule not found in .gitleaks.toml")
	}
	// The regex line follows within the same [[rules]] block.
	tail := body[idIdx:]
	reMarker := "regex = '''"
	reIdx := strings.Index(tail, reMarker)
	if reIdx < 0 {
		t.Fatalf("regex = '''...''' not found after sk-api-key rule id")
	}
	after := tail[reIdx+len(reMarker):]
	end := strings.Index(after, "'''")
	if end < 0 {
		t.Fatalf("closing ''' not found for sk-api-key regex")
	}
	return after[:end]
}

// TestSKAPIKeyBoundaryRejectsMidWordProve is the load-bearing regression:
// the pre-fix regex `(?i)sk-[a-zA-Z0-9_-]{20,}` matches the tail of
// "task-complete-unavailable" (token length 23, starts at the 'sk-' inside
// "task-"). The fixed regex must NOT.
func TestSKAPIKeyBoundaryRejectsMidWordProse(t *testing.T) {
	reStr := readSKRuleRegex(t)
	re, err := regexp.Compile(reStr)
	if err != nil {
		t.Fatalf("compile deployed sk-api-key regex: %v (regex=%q)", err, reStr)
	}
	// The exact prose that triggered the original FP (BT-83 filing).
	fp := "verdict=none:task-complete-unavailable"
	if re.MatchString(fp) {
		t.Errorf("deployed sk-api-key regex still matches mid-word prose %q — BT-83 regression", fp)
	}
	// Same class, different surrounding tokens.
	for _, s := range []string{
		"prefix=task-abcdefghijklmnopqrstuvwxyz",
		"word=desk-abcdefghijklmnopqrstuvwx",
		"mask=flask-01234567890123456789",
	} {
		if re.MatchString(s) {
			t.Errorf("deployed sk-api-key regex matches mid-word FP candidate %q", s)
		}
	}
}

// TestSKAPIKeyStillCatchesRealKeys is the other half: the boundary fix must
// not neuter the rule. A real sk- token, 20+ chars, preceded by a
// non-alphanumeric boundary (start-of-line, space, quote, equals) must still
// match.
func TestSKAPIKeyStillCatchesRealKeys(t *testing.T) {
	reStr := readSKRuleRegex(t)
	re, err := regexp.Compile(reStr)
	if err != nil {
		t.Fatalf("compile deployed sk-api-key regex: %v", err)
	}
	realKey := "sk-abcdefghijklmnopqrstuvwxyz" // 29 chars after sk-
	for _, ctx := range []string{
		realKey,               // start of string
		" " + realKey,         // space boundary
		"=" + realKey,         // punctuation boundary
		"\"" + realKey + "\"", // quote boundary
		"token:" + realKey,    // colon boundary
		"\n" + realKey,        // newline boundary
	} {
		if !re.MatchString(ctx) {
			t.Errorf("deployed sk-api-key regex missed real key in context %q", ctx)
		}
	}
	// Case-insensitive: SK-... must also match.
	if !re.MatchString("SK-ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		t.Errorf("deployed sk-api-key regex is not case-insensitive")
	}
}

// TestSKAPIKeyRejectsTooShort ensures the 20-char minimum is still enforced
// (the rule's other half, independent of the boundary fix).
func TestSKAPIKeyRejectsTooShort(t *testing.T) {
	reStr := readSKRuleRegex(t)
	re, err := regexp.Compile(reStr)
	if err != nil {
		t.Fatalf("compile deployed sk-api-key regex: %v", err)
	}
	// 19 chars after sk-: must NOT match.
	short := "sk-1234567890123456789"
	if got := len(short) - 3; got != 19 {
		t.Fatalf("test fixture length drift: want 19 after sk-, got %d", got)
	}
	if re.MatchString(" " + short) {
		t.Errorf("deployed sk-api-key regex matches too-short token %q (len after sk- = 19)", short)
	}
}

// TestSKAPIKeyRegexHasBoundaryAnchor is a structural guard: the deployed
// regex must carry a non-alphanumeric boundary prefix before sk- (the BT-83
// fix). This pins the shape directly so a future edit that silently drops
// the anchor is caught even if the prose fixtures above are also changed.
func TestSKAPIKeyRegexHasBoundaryAnchor(t *testing.T) {
	reStr := readSKRuleRegex(t)
	// The fix shape is `(?:^|[^a-zA-Z0-9])sk-` (case-insensitive flag may wrap).
	// Accept either a literal `[^a-zA-Z0-9]` class or a `\b` word-boundary
	// anchor before `sk-`; reject a regex that begins the match at `sk-`
	// with no preceding anchor.
	// Strip an optional leading (?i) flag for the shape check.
	shape := strings.TrimPrefix(reStr, "(?i)")
	hasAnchor := strings.Contains(shape, "[^a-zA-Z0-9]") ||
		strings.Contains(shape, `\b`) ||
		strings.HasPrefix(shape, "(?:^")
	if !hasAnchor {
		t.Errorf("deployed sk-api-key regex %q lacks a non-alnum boundary anchor before sk- (BT-83)", reStr)
	}
}
