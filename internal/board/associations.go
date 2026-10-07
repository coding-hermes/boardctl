package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// BT-077: structured PLURAL work-artifact associations on a task row.
//
// The singular fields (worktree, branch) and sessions record ONE build
// location per dimension; real work spans several PRs, branches and
// worktrees, and the relationship was being lost. These helpers define the
// three plural dimensions and their element validation:
//
//	pull_requests — array of association objects {number?, url?} accepting
//	                a GitHub PR URL or a bare number, normalized to
//	                {"number": N} (bare input) or {"number": N, "url": U}
//	                (URL input; the number is parsed from the URL when it
//	                carries one)
//	branches      — array of non-empty strings
//	worktrees     — array of non-empty strings (absolute paths)
//
// The singular fields and sessions are PRESERVED untouched (backward
// compatibility); the plural dimensions are additive and independent: an
// update that adds one element merges into its own dimension only and never
// overwrites siblings — not the other dimensions, not other elements of the
// same dimension. Round-trip support (create/update/show/list/render/export)
// rides the existing row machinery: the keys are canonical (sanctioned in
// keys.go), every result set returns whole rows, and export re-serializes
// the row verbatim.
//
// Validation discipline mirrors the house style (BT-076 deferred,
// BT-048 priority): WRITE paths reject malformed values loudly with a clear
// error naming the dimension and offending value; READ/validate paths
// itemize WARNINGS (existing and legacy boards stay exit-0) and read the
// malformed shape as "no associations in that dimension".

// PullRequestsKey / BranchesKey / WorktreesKey are the plural dimension keys
// on the task row.
const (
	PullRequestsKey = "pull_requests"
	BranchesKey     = "branches"
	WorktreesKey    = "worktrees"
)

// PRNumberKey / PRURLKey are the association-object field names inside a
// pull_requests element.
const (
	PRNumberKey = "number"
	PRURLKey    = "url"
)

// AssociationKeys lists the plural dimensions in canonical order (create
// appends the keys in this order; validate walks it the same way).
var AssociationKeys = []string{PullRequestsKey, BranchesKey, WorktreesKey}

// prURLHostRe pins the GitHub PR URL host. Only https/http URLs on
// github.com (or its www form) are accepted; everything else is malformed
// for this dimension (a GitLab PR is a different concept and must not
// silently become a GitHub number).
var prURLHostRe = regexp.MustCompile(`^https?://(www\.)?github\.com/[^/\s]+/[^/\s]+/pull/`)

// prURLNumberRe extracts the PR number from the URL path tail /pull/<n>
// (optionally followed by a fragment anchor or query — GitHub emits
// /pull/42#issuecomment-… and /pull/42/files links for the same PR).
var prURLNumberRe = regexp.MustCompile(`^/pull/([0-9]+)(?:[/?#].*)?$`)

// NormalizePullRequest canonicalizes one pull_requests element from user
// input. Accepted shapes:
//
//   - a bare positive integer ("42")            -> {"number": 42}
//   - a GitHub PR URL carrying a number
//     ("https://github.com/o/r/pull/42", with optional trailing anchor,
//     "/files" suffix or query)                 -> {"url": <cleaned>, "number": 42}
//   - a GitHub PR URL without a parseable number -> {"url": <cleaned>}
//   - an object carrying number and/or url — each field validated and
//     re-emitted in canonical {number?, url?} order; a url must be a
//     GitHub PR URL when present, a number must be a positive integer,
//     and a url-embedded number must agree with an explicit number
//
// The returned bytes are the canonical JSON for the element (compact, in
// number-before-url order). Malformed input returns an error naming the
// offending value and the accepted shapes — write paths surface it verbatim.
func NormalizePullRequest(v string) (json.RawMessage, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, fmt.Errorf("empty pull_requests value — provide a GitHub PR URL or a bare PR number")
	}
	// Bare number: digits only (positive integer; "007" normalizes to 7 —
	// the leading zeros are the caller's spelling, the integer value is
	// what the row stores).
	if n, err := strconv.Atoi(v); err == nil {
		if n <= 0 {
			return nil, fmt.Errorf("pull_requests %q: PR number must be a positive integer", v)
		}
		return prElemJSON(n, ""), nil
	}
	if strings.HasPrefix(v, "{") {
		return normalizePullRequestObject(v)
	}
	// URL form.
	if !prURLHostRe.MatchString(v) {
		return nil, fmt.Errorf("pull_requests %q is not a GitHub PR URL or a bare PR number (accepted: https://github.com/owner/repo/pull/123, or 123)", v)
	}
	cleaned := cleanPRURL(v)
	num := prNumberFromURL(cleaned)
	return prElemJSON(num, cleaned), nil
}

// normalizePullRequestObject validates an already-structured element
// ({"number": …, "url": …} in either order; unknown keys are dropped) and
// re-emits it canonically.
func normalizePullRequestObject(v string) (json.RawMessage, error) {
	var obj struct {
		Number *int64  `json:"number"`
		URL    *string `json:"url"`
	}
	dec := json.NewDecoder(strings.NewReader(v))
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("pull_requests element %s is not a valid association object ({\"number\": N} and/or {\"url\": U})", v)
	}
	num := int64(0)
	if obj.Number != nil {
		if *obj.Number <= 0 {
			return nil, fmt.Errorf("pull_requests: number %d must be a positive integer", *obj.Number)
		}
		num = *obj.Number
	}
	url := ""
	if obj.URL != nil {
		u := strings.TrimSpace(*obj.URL)
		if u != "" {
			if !prURLHostRe.MatchString(u) {
				return nil, fmt.Errorf("pull_requests: url %q is not a GitHub PR URL (accepted: https://github.com/owner/repo/pull/123)", u)
			}
			cleaned := cleanPRURL(u)
			if n := prNumberFromURL(cleaned); n > 0 {
				if num > 0 && num != int64(n) {
					return nil, fmt.Errorf("pull_requests: number %d disagrees with the number in url %q", num, cleaned)
				}
				num = int64(n)
			}
			url = cleaned
		}
	}
	if num == 0 && url == "" {
		return nil, fmt.Errorf("pull_requests element carries neither a usable number nor a usable url: %s", v)
	}
	return prElemJSON(int(num), url), nil
}

// cleanPRURL strips a trailing fragment/query and a trailing slash so the
// stored spelling of the same PR is stable.
func cleanPRURL(u string) string {
	if i := strings.IndexAny(u, "#?"); i >= 0 {
		u = u[:i]
	}
	return strings.TrimRight(u, "/")
}

// prNumberFromURL extracts the PR number from a cleaned GitHub PR URL
// (0 when the path tail is not /pull/<digits>).
func prNumberFromURL(u string) int {
	i := strings.LastIndex(u, "/pull/")
	if i < 0 {
		return 0
	}
	path := u[i:]
	m := prURLNumberRe.FindStringSubmatch(path)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// prElemJSON renders the canonical association object: {"number": N} when
// only a number is known, {"url": U} when only a URL, {"number": N,
// "url": U} when both. Number precedes URL (stable order).
func prElemJSON(num int, url string) json.RawMessage {
	var sb strings.Builder
	sb.WriteByte('{')
	first := true
	emit := func(k string, v any) {
		if !first {
			sb.WriteByte(',')
		}
		first = false
		kb, _ := json.Marshal(k)
		vb, err := json.Marshal(v)
		if err != nil {
			return // unreachable for int/string values
		}
		sb.Write(kb)
		sb.WriteByte(':')
		sb.Write(vb)
	}
	if num > 0 {
		emit(PRNumberKey, num)
	}
	if url != "" {
		emit(PRURLKey, url)
	}
	sb.WriteByte('}')
	return json.RawMessage(sb.String())
}

// ValidateAssociationValue validates/normalizes one element of the named
// plural dimension from CLI string input. pull_requests accepts GitHub PR
// URLs and bare numbers (see NormalizePullRequest); branches and worktrees
// accept any non-empty string with surrounding whitespace trimmed.
func ValidateAssociationValue(dim, v string) (json.RawMessage, error) {
	switch dim {
	case PullRequestsKey:
		return NormalizePullRequest(v)
	case BranchesKey, WorktreesKey:
		t := strings.TrimSpace(v)
		if t == "" {
			return nil, fmt.Errorf("%s element must be a non-empty string (got %q)", dim, v)
		}
		b, _ := json.Marshal(t)
		return json.RawMessage(b), nil
	default:
		return nil, fmt.Errorf("unknown association dimension %q (known: %s)", dim, strings.Join(AssociationKeys, ", "))
	}
}

// normalizeStoredAssociation returns the canonical JSON bytes for one
// element ALREADY stored on a row (the read path): it re-validates through
// the same rules as the write path and reports whether the stored value was
// malformed. Malformed input yields ok=false; the caller decides
// error-vs-warning. The element may be a JSON string (branches/worktrees
// spelling) or an object (pull_requests).
func normalizeStoredAssociation(dim string, raw json.RawMessage) (canonical json.RawMessage, ok bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, false
	}
	var s string
	if err := json.Unmarshal(trimmed, &s); err == nil {
		c, err := ValidateAssociationValue(dim, s)
		if err != nil {
			return nil, false
		}
		return c, true
	}
	if dim == PullRequestsKey {
		c, err := normalizePullRequestObject(string(trimmed))
		if err != nil {
			return nil, false
		}
		return c, true
	}
	return nil, false
}

// AssociationElems returns the canonical element list stored under the
// named dimension on a row, skipping elements that fail validation. ok
// reports whether every stored element was canonical (false => the caller
// should warn about the malformed shape; use AssociationBadIndex to name
// it). Absent/null keys return (nil, true) — no associations is the neutral
// shape.
func AssociationElems(row *Row, dim string) (elems []json.RawMessage, ok bool) {
	raw := row.Get(dim)
	if raw == nil {
		return nil, true
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	var arr []json.RawMessage
	if err := dec.Decode(&arr); err != nil {
		return nil, false
	}
	for _, el := range arr {
		c, good := normalizeStoredAssociation(dim, el)
		if !good {
			return elems, false
		}
		elems = append(elems, c)
	}
	return elems, true
}

// AssociationBadIndex returns the 0-based index of the FIRST element of the
// named dimension that fails validation (-1 when none does). Only
// meaningful when AssociationElems reported ok=false.
func AssociationBadIndex(row *Row, dim string) int {
	raw := row.Get(dim)
	if raw == nil {
		return -1
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	var arr []json.RawMessage
	if err := dec.Decode(&arr); err != nil {
		return 0
	}
	for i, el := range arr {
		if _, good := normalizeStoredAssociation(dim, el); !good {
			return i
		}
	}
	return -1
}

// MergeAssociation appends one validated element to the named dimension on
// the row, MERGING rather than replacing: existing elements — and every
// other key on the row — are preserved byte-verbatim; the new element lands
// at the end of the dimension's array. A dimension key the row does not
// carry yet is appended at the end of the row's key order (Row.SetRaw
// semantics). Element style follows the row's serialization style so the
// rewritten line differs only where it must.
//
// Duplicates: an element already present (exact canonical equality) is not
// appended again — merging the same PR twice is a no-op for that element
// and reports no change.
func MergeAssociation(row *Row, dim string, value json.RawMessage, style Style) (added bool, err error) {
	canonical := bytes.TrimSpace(value)
	if len(canonical) == 0 {
		return false, fmt.Errorf("%s: empty association value", dim)
	}
	elems, _ := AssociationElems(row, dim)
	for _, e := range elems {
		if bytes.Equal(bytes.TrimSpace(e), canonical) {
			return false, nil // already present (exact element) — idempotent
		}
	}
	if dim == PullRequestsKey {
		// PR identity is the NUMBER: a bare 22 and a URL spelling of PR
		// 22 are the same association, so a merge that only carries a
		// number must not append a second element for a PR already
		// present under its URL (and vice versa). Both spellings
		// normalize to {"number": N} in the identity comparison; the
		// richer STORED element (url included) always wins.
		if nNew := prElemNumber(canonical); nNew > 0 {
			for _, e := range elems {
				if prElemNumber(e) == nNew {
					return false, nil // same PR by number — idempotent
				}
			}
		}
	}
	elems = append(elems, canonical)
	if err := setAssociationElems(row, dim, elems, style); err != nil {
		return false, err
	}
	return true, nil
}

// prElemNumber extracts the "number" field from one stored pull_requests
// element (0 when absent or not a positive integer).
func prElemNumber(raw json.RawMessage) int64 {
	var obj struct {
		Number int64 `json:"number"`
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(raw)))
	if err := dec.Decode(&obj); err != nil {
		return 0
	}
	if obj.Number <= 0 {
		return 0
	}
	return obj.Number
}

// setAssociationElems writes the element list back onto the row in the
// row's own style (SetGoValue with []any so spacing/ASCII follow Style).
func setAssociationElems(row *Row, dim string, elems []json.RawMessage, style Style) error {
	arr := make([]any, 0, len(elems))
	for _, e := range elems {
		arr = append(arr, e)
	}
	return row.SetGoValue(dim, arr, style)
}
