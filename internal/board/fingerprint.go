package board

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
)

// SG-126 board-row dedupe.
//
// QA and dogfood lanes re-filed the same finding 3-8x because the append path
// took a fresh id against a recycled (or suffix-variant) title and never asked
// the open board whether the finding was already there. The result is board
// churn: operators paging the board see noise, and re-enable audits read the
// lanes as idle-but-busy.
//
// The fix is a FINDING FINGERPRINT: a normalized digest over the row's
// title + reasoning. It is advisory metadata stored INSIDE the existing detail
// field (the board schema is git-tracked JSONL and gains no new column), so:
//
//	detail = {"fingerprint":"<sha256hex>","evidence":[{"run_id":...,"ts":...}]}
//
// A write path refuses to create an open duplicate of a fingerprinted finding
// (ErrDuplicateFinding) and records the suppression as a task_evidence EVENT —
// the board grows one audit line instead of one more row. Rows written before
// this rule carry no fingerprint and are GRANDFATHERED: they stay open and
// visible, they simply never match an incoming finding.

// findingFingerprintSep is the unit separator joining the two normalized
// halves. It cannot occur inside normalized text (strings.Fields collapses all
// whitespace, and control bytes never survive a JSON string in practice), so
// digest("ab"||sep||"c") can never collide with digest("abc").
const findingFingerprintSep = '\x1f'

var (
	// findingIDPrefixRe strips a leading finding-id token such as
	// "QA-CRIER-3 ", "DF-AI-PLAYS-POKE-12 " or "DOGFOOD-MESH-1 " — the id is
	// recycled per cycle, so it must not be part of the identity.
	findingIDPrefixRe = regexp.MustCompile(`^(?:qa|df|dogfood)-[a-z0-9-]+-\d+\s+`)

	// findingRecurrenceRe strips a trailing recurrence marker: "(recurrence 3)",
	// "(recurrence: 3)", "(3rd)" — the marker counts re-detections, it is not
	// part of the finding.
	findingRecurrenceRe = regexp.MustCompile(`\s*\(\s*(?:recurrence\s*:?\s*\d+|\d+(?:st|nd|rd|th))\s*\)\s*$`)

	// findingRunIDRe strips a trailing "[run-id-N]" marker (the lane's run
	// counter, likewise recycled).
	findingRunIDRe = regexp.MustCompile(`\s*\[\s*run-id-[a-z0-9-]+\s*\]\s*$`)
)

// NormalizeFindingText canonicalizes one half of a finding identity: lowercase,
// strip a leading finding-id prefix, collapse every whitespace run to a single
// space, strip trailing recurrence / run-id markers. Deliberately simple and
// total (never errors) so both the write gate and the backfill agree on the
// identity byte for byte.
//
// NOT normalized on purpose: a leading priority tag ("[P1] ") stays part of the
// fingerprint, so an escalation of the same symptom is a DIFFERENT finding and
// earns its own row.
func NormalizeFindingText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = findingIDPrefixRe.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	// Strip trailing markers repeatedly: a title can carry both a run-id and a
	// recurrence suffix ("... [run-id-4] (recurrence 2)").
	for i := 0; i < 4; i++ {
		trimmed := findingRunIDRe.ReplaceAllString(s, "")
		trimmed = findingRecurrenceRe.ReplaceAllString(trimmed, "")
		if trimmed == s {
			break
		}
		s = trimmed
	}
	return strings.Join(strings.Fields(s), " ")
}

// FindingFingerprint is the SHA-256 (hex) of the normalized title, the unit
// separator, and the normalized reasoning.
func FindingFingerprint(title, reasoning string) string {
	h := sha256.New()
	h.Write([]byte(NormalizeFindingText(title)))
	h.Write([]byte{findingFingerprintSep})
	h.Write([]byte(NormalizeFindingText(reasoning)))
	return hex.EncodeToString(h.Sum(nil))
}

// RowFindingReasoning renders a task row's reasoning field as the plain text
// that participates in the fingerprint. Live boards carry BOTH shapes: a plain
// string (crier/dogfood rows) and an object such as {"note":"QA foreman cycle
// …"} (qa-dagger rows). An object-form reasoning is fingerprinted on its "note"
// member when present (the human-readable half), else on its canonical JSON;
// null/absent contributes nothing.
func RowFindingReasoning(row *Row) string {
	raw := row.Get("reasoning")
	if len(raw) == 0 {
		return ""
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	var s string
	if err := json.Unmarshal(trimmed, &s); err == nil {
		return s
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err == nil {
		if note, ok := obj["note"]; ok {
			var ns string
			if err := json.Unmarshal(note, &ns); err == nil {
				return ns
			}
		}
		var v any
		if err := json.Unmarshal(trimmed, &v); err == nil {
			if b, err := json.Marshal(v); err == nil {
				return string(b)
			}
		}
	}
	return string(trimmed)
}

// FindingFingerprintForRow recomputes a row's fingerprint from its OWN stored
// title and reasoning. Used by the backfill (rows written before the rule) and
// to detect drift against a stored fingerprint.
func FindingFingerprintForRow(row *Row) string {
	return FindingFingerprint(row.String("title"), RowFindingReasoning(row))
}

// OpenFindingStatuses is the set of statuses that count as an OPEN finding.
// Positive list on purpose: an unknown or legacy spelling (e.g. "duplicate",
// "retired") never suppresses an incoming create — fail open, never silently
// swallow a filing.
//
// "dispatched" is not a write-vocabulary status but live boards carry it (a
// scheduler-dispatched row that has not been picked up yet); blocked/review are
// open work by definition, so a re-detected finding on them is still a
// duplicate.
var OpenFindingStatuses = map[string]bool{
	"pending":     true,
	"in_progress": true,
	"dispatched":  true,
	"blocked":     true,
	"review":      true,
}

// RowIsOpenFinding reports whether a task row counts as open for dedupe.
func RowIsOpenFinding(row *Row) bool {
	return OpenFindingStatuses[strings.ToLower(strings.TrimSpace(row.String("status")))]
}

// EvidenceEntry is one recorded re-observation of a finding: the run that saw
// it again (and, for a backfill merge, the row it was merged away from).
type EvidenceEntry struct {
	RunID  string `json:"run_id,omitempty"`
	TS     string `json:"ts,omitempty"`
	TaskID string `json:"task_id,omitempty"`
}

// FindingDetail is the SG-126 metadata carried inside a task row's detail
// field. Text preserves the row's original detail prose so the envelope is
// lossless for humans.
type FindingDetail struct {
	Fingerprint string          `json:"fingerprint,omitempty"`
	Evidence    []EvidenceEntry `json:"evidence,omitempty"`
	Text        string          `json:"text,omitempty"`
}

// parsedDetail is the classified form of a row's raw detail value.
type parsedDetail struct {
	Finding FindingDetail
	// Extra holds the unknown keys of an object-form detail verbatim, so a
	// write-back never discards another producer's payload.
	Extra map[string]json.RawMessage
	// Object reports that the detail is object-shaped: the next write keeps
	// object form (merging our keys) instead of wrapping prose into "text".
	Object bool
}

func (p parsedDetail) withFingerprint(fp string) parsedDetail {
	p.Finding.Fingerprint = fp
	return p
}

// withEvidence appends entries, skipping ones already present (idempotent
// merges: a re-run of the backfill adds nothing).
func (p parsedDetail) withEvidence(entries []EvidenceEntry) parsedDetail {
	for _, e := range entries {
		if e == (EvidenceEntry{}) {
			continue
		}
		dup := false
		for _, have := range p.Finding.Evidence {
			if have == e {
				dup = true
				break
			}
		}
		if !dup {
			p.Finding.Evidence = append(p.Finding.Evidence, e)
		}
	}
	return p
}

// ParseFindingDetail classifies a task row's raw detail value. It tolerates the
// three shapes live boards carry:
//
//	null / absent           -> empty
//	JSON object             -> envelope (our keys) + Extra (every other key,
//	                           preserved verbatim for the next write)
//	JSON string             -> if the string itself parses as an envelope
//	                           object, that object (a hand-written string
//	                           envelope); otherwise plain prose in Text
//	anything else (array,
//	  number, bool)         -> canonical JSON kept in Text
func ParseFindingDetail(raw json.RawMessage) parsedDetail {
	if len(raw) == 0 {
		return parsedDetail{}
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return parsedDetail{}
	}
	var s string
	if err := json.Unmarshal(trimmed, &s); err == nil {
		if p, ok := parseDetailObject([]byte(s)); ok {
			return p
		}
		return parsedDetail{Finding: FindingDetail{Text: s}}
	}
	if p, ok := parseDetailObject(trimmed); ok {
		return p
	}
	var v any
	if err := json.Unmarshal(trimmed, &v); err == nil {
		if b, err := json.Marshal(v); err == nil {
			return parsedDetail{Finding: FindingDetail{Text: string(b)}}
		}
	}
	return parsedDetail{Finding: FindingDetail{Text: string(trimmed)}}
}

// parseDetailObject converts a JSON object into a parsedDetail. ok=false when
// the bytes are not a JSON object at all (prose / array / scalar).
func parseDetailObject(b []byte) (parsedDetail, bool) {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return parsedDetail{}, false
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &all); err != nil {
		return parsedDetail{}, false
	}
	p := parsedDetail{Object: true, Extra: map[string]json.RawMessage{}}
	for k, v := range all {
		switch k {
		case "fingerprint":
			var fp string
			if err := json.Unmarshal(v, &fp); err == nil {
				p.Finding.Fingerprint = fp
			} else {
				p.Extra[k] = v
			}
		case "evidence":
			var ev []EvidenceEntry
			if err := json.Unmarshal(v, &ev); err == nil {
				p.Finding.Evidence = ev
			} else {
				p.Extra[k] = v
			}
		case "text":
			var t string
			if err := json.Unmarshal(v, &t); err == nil {
				p.Finding.Text = t
			} else {
				p.Extra[k] = v
			}
		default:
			p.Extra[k] = v
		}
	}
	return p, true
}

// EncodeFindingDetail renders a parsedDetail back to a JSON value in the
// board's own serialization style. An object-form detail keeps every unknown
// key verbatim (sorted for determinism); prose is carried under "text".
func EncodeFindingDetail(p parsedDetail, style Style) ([]byte, error) {
	row := &Row{Vals: map[string]json.RawMessage{}}
	if p.Object {
		for _, k := range SortedKeys(p.Extra) {
			row.SetRaw(k, p.Extra[k])
		}
	}
	if p.Finding.Fingerprint != "" {
		row.SetRaw("fingerprint", jsonString(p.Finding.Fingerprint, style))
	}
	if len(p.Finding.Evidence) > 0 {
		enc, err := encodeValue(p.Finding.Evidence, style)
		if err != nil {
			return nil, err
		}
		row.SetRaw("evidence", enc)
	}
	if p.Finding.Text != "" {
		row.SetRaw("text", jsonString(p.Finding.Text, style))
	}
	if len(row.Keys) == 0 {
		return []byte("null"), nil
	}
	return row.Marshal(style), nil
}

// StoredFingerprint returns the fingerprint a row already carries in its detail
// ("" when it has none — a grandfathered row).
func StoredFingerprint(row *Row) string {
	return ParseFindingDetail(row.Get("detail")).Finding.Fingerprint
}

// openFingerprintIndex maps stored fingerprint -> id for every OPEN row that
// carries one. The first occurrence in file order wins, so the earliest filing
// of a finding is the one a new duplicate is matched against.
func openFingerprintIndex(rows []*Row) map[string]string {
	idx := map[string]string{}
	for _, r := range rows {
		if !RowIsOpenFinding(r) {
			continue
		}
		fp := StoredFingerprint(r)
		if fp == "" {
			continue
		}
		if _, seen := idx[fp]; !seen {
			idx[fp] = r.String("id")
		}
	}
	return idx
}
