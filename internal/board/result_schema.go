package board

import (
	"bytes"
	"encoding/json"
)

// ResultObjectStatusVocabulary is the status vocabulary for the STRUCTURED
// form of guard_result/ci_result (SCHED-GAP-1572). A result field may hold
// EITHER a plain vocabulary string ({PASS,FAIL,SKIP} / {GREEN,RED,SKIP} —
// BT-007, unchanged) OR a JSON object with a `status` string drawn from this
// set and a `details` string carrying the prose the old free-form values
// held. Anything else stays exactly what BT-007/BT-049 already warn about.
var ResultObjectStatusVocabulary = map[string]bool{
	"pass":    true,
	"fail":    true,
	"skip":    true,
	"pending": true,
}

// ClassifyResultValue classifies one stored guard_result/ci_result VALUE for
// the schema check. field is the column name ("guard_result"/"ci_result");
// the PLAIN-STRING form is checked against that field's OWN vocabulary
// (BT-007/BT-049 semantics unchanged — a value from the sibling field's
// vocabulary is still a warning naming this field's vocabulary):
//
//   - "absent" — key missing, JSON null, or the empty string (never run;
//     not this check's business, same as today)
//   - "ok"     — schema-conformant: a plain in-vocabulary string (case-
//     tolerant, NormalizeResultValue) OR a valid structured object
//     {status, details}
//   - otherwise a diagnosis string describing exactly which schema rule the
//     stored value breaks, for the itemized warning.
//
// The structured form is {status: one of pass|fail|skip|pending (exact,
// lower-case), details: a JSON string}. status is compared case-sensitively
// — the plain-string form's case tolerance (BT-049 reads "pass" as PASS)
// covers case drift on the string spelling, but the object's status field
// is a NEW surface and starts strict; details must be a string so the
// value round-trips as prose. An object with EXTRA keys is still valid:
// rows may carry per-run sub-records alongside status/details.
func ClassifyResultValue(field string, raw []byte) string {
	if len(raw) == 0 {
		return "absent"
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "absent"
	}
	// Plain string form: BT-007 semantics, per-field vocabulary.
	var s string
	if err := json.Unmarshal(trimmed, &s); err == nil {
		if s == "" {
			return "absent"
		}
		vocab := GuardResultVocabulary
		if field == "ci_result" {
			vocab = CIResultVocabulary
		}
		if vocab[NormalizeResultValue(s)] {
			return "ok"
		}
		return "free-form string"
	}
	// Structured object form.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return "value is neither a vocabulary string nor a {status, details} object"
	}
	stRaw, hasStatus := obj["status"]
	if !hasStatus {
		return "object missing \"status\""
	}
	var st string
	if err := json.Unmarshal(stRaw, &st); err != nil {
		return "object \"status\" is not a string"
	}
	if !ResultObjectStatusVocabulary[st] {
		return "object \"status\" " + jsonQuote(st) + " not in vocabulary {pass,fail,skip,pending}"
	}
	dRaw, hasDetails := obj["details"]
	if !hasDetails {
		return "object missing \"details\""
	}
	var details string
	if err := json.Unmarshal(dRaw, &details); err != nil {
		return "object \"details\" is not a string"
	}
	return "ok"
}

// ResultFieldVocabString is the per-field vocabulary string the BT-049
// finding messages print (the field's OWN vocabulary, never the union).
var ResultFieldVocabString = map[string]string{
	"guard_result": "{PASS,FAIL,SKIP}",
	"ci_result":    "{GREEN,RED,SKIP}",
}

// decodeJSONStringOrEmpty decodes a stored field value as a JSON string for
// finding messages; a non-string or unparseable value yields "" (the schema
// diagnosis names the shape separately).
func decodeJSONStringOrEmpty(raw []byte) string {
	s, ok := decodeJSONString(raw)
	if !ok {
		return ""
	}
	return s
}

// jsonQuote renders s as a JSON string for embedding in a finding message.
func jsonQuote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `"` + s + `"`
	}
	return string(b)
}
