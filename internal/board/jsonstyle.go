// Package board implements read/write access to coding-hermes foreman JSONL
// boards. JSONL is the canonical git-tracked store; .db / *.parquet files are
// untracked rebuildable caches and are NEVER written by this package.
package board

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// The two physical-line corruption classes GR-150 rejects. A JSONL line is
// EXACTLY one JSON value; anything after the first object is data no line
// reader will ever see.
//
// They are sentinel errors so callers can classify without matching text: the
// tolerant reader records them as skipped-line evidence, validate turns them
// into error findings, and the repair path REFUSES them (re-serializing such a
// line would keep the first object and silently drop the rest).
var (
	// ErrMultipleJSONValues marks a line carrying more than one JSON value —
	// the fused-record class (N complete objects concatenated on one physical
	// line, e.g. what a lost trailing newline plus an append produces, or a
	// 445-object line committed by hand). A json.Decoder stops after the FIRST
	// value, so the strict path used to read such a line as one record.
	ErrMultipleJSONValues = errors.New("multiple JSON values on one physical line")
	// ErrTrailingData marks a line whose object is followed by bytes that are
	// not another JSON value ("{...}oops", "{...}{oops}").
	ErrTrailingData = errors.New("trailing data after the JSON object")
)

// oneObjectPerLineHint is appended to both rejections: the invariant, stated
// where the operator will read it.
const oneObjectPerLineHint = "JSONL requires exactly one JSON object per line"

// Style describes the JSON serialization style of a board file, detected from
// its last line (python json.dumps conventions used by the fleet scripts):
//   - Spaced: separators (', ', ': ')  — python json.dumps defaults
//   - Compact: separators (',', ':')
//   - ASCII: ensure_ascii=True (non-ASCII emitted as \uXXXX escapes, astral
//     runes as surrogate pairs) — python json.dumps default
type Style struct {
	Spaced bool
	ASCII  bool
}

// DetectStyle sniffs a serialization style from one raw JSONL line.
//
// Spacing probe: python spaced dumps separate keys with `", "` (close-quote
// comma space open-quote). That byte sequence cannot occur inside JSON string
// content (embedded quotes are backslash-escaped) nor inside escaped `detail`
// payloads (escaped keys terminate in `\"`), so its presence reliably marks a
// spaced row. Compact rows never contain it.
//
// ASCII probe: presence of a literal `\u` escape in the raw line (the same
// probe the fleet scripts use on the last committed line).
func DetectStyle(line []byte) Style {
	return Style{
		Spaced: bytes.Contains(line, []byte(`", "`)),
		ASCII:  bytes.Contains(line, []byte(`\u`)),
	}
}

// DefaultStyle is used when a target file is empty / has no sample line:
// python json.dumps defaults (spaced + ensure_ascii=True).
func DefaultStyle() Style { return Style{Spaced: true, ASCII: true} }

// Row is a parsed JSONL object that preserves key ORDER and the raw bytes of
// every field value. Key order mirrors the writer's dict insertion order;
// untouched field bytes stay verbatim (byte-preserving surgery).
type Row struct {
	Keys []string
	Vals map[string]json.RawMessage
}

// ParseRow parses one JSONL line into an ordered Row. Whitespace between
// tokens is skipped; each field's value is kept as verbatim RawMessage bytes
// (including any inner spacing of nested values).
//
// GR-150: the line must carry EXACTLY ONE JSON value. json.Decoder stops after
// the first value, so a fused line ("{...}{...}") or a line with a malformed
// tail ("{...}oops") used to parse as the first object and every record after
// it vanished silently. Both are rejected now (ErrMultipleJSONValues /
// ErrTrailingData) with the byte offset named.
func ParseRow(line []byte) (*Row, error) {
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("not valid JSON: %v", err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("row is not a JSON object")
	}
	r := &Row{Vals: map[string]json.RawMessage{}}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := kt.(string)
		if !ok {
			return nil, fmt.Errorf("non-string object key %v", kt)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		if _, dup := r.Vals[key]; dup {
			return nil, fmt.Errorf("duplicate key %q in row", key)
		}
		r.Keys = append(r.Keys, key)
		r.Vals[key] = raw
	}
	if _, err := dec.Token(); err != nil { // consume closing '}'
		return nil, err
	}
	if err := checkOneValuePerLine(line); err != nil {
		return nil, err
	}
	if len(r.Keys) == 0 {
		return nil, fmt.Errorf("empty JSON object")
	}
	return r, nil
}

// checkOneValuePerLine enforces the JSONL invariant on a line whose FIRST
// value already parsed: everything after that object must be whitespace.
//
// It re-decodes the first value with a dedicated decoder to get a well-defined
// end offset (json.Decoder.InputOffset after the object), then classifies the
// leftover so the diagnostic says WHICH corruption it is:
//
//   - another complete JSON value follows  -> ErrMultipleJSONValues (fused record)
//   - a value that starts but is truncated -> ErrMultipleJSONValues (incomplete
//     second value: the line is fused AND damaged)
//   - anything else                        -> ErrTrailingData
//
// A line whose first value does not decode at all is left to ParseRow's own
// token-walk error (this helper returns nil), so a truncated single object
// keeps its existing EOF-class diagnostic.
func checkOneValuePerLine(line []byte) error {
	probe := json.NewDecoder(bytes.NewReader(line))
	probe.UseNumber()
	var first json.RawMessage
	if err := probe.Decode(&first); err != nil {
		return nil // the token walk already reported this line's parse failure
	}
	tail := line[probe.InputOffset():]
	rest := bytes.TrimSpace(tail)
	if len(rest) == 0 {
		return nil
	}
	// 0-based byte offset of the first leftover byte (leading JSON whitespace
	// between the object and the leftover is not part of the diagnosis).
	at := int(probe.InputOffset()) + (len(tail) - len(bytes.TrimLeft(tail, " 	\r\n")))

	next := json.NewDecoder(bytes.NewReader(rest))
	next.UseNumber()
	var second json.RawMessage
	err := next.Decode(&second)
	switch {
	case err == nil:
		detail := fmt.Sprintf("a second JSON value starts at byte %d: %s", at, diagnosticSnippet(rest))
		if more := bytes.TrimSpace(rest[next.InputOffset():]); len(more) > 0 {
			detail = fmt.Sprintf("a second JSON value starts at byte %d with further data after it: %s", at, diagnosticSnippet(rest))
		}
		return fmt.Errorf("%w (a fused record — %s): %s", ErrMultipleJSONValues, oneObjectPerLineHint, detail)
	case errors.Is(err, io.ErrUnexpectedEOF) || isEOFJSONErr(err):
		return fmt.Errorf("%w (a fused, damaged record — %s): the value after the first object is incomplete at byte %d: %v: %s",
			ErrMultipleJSONValues, oneObjectPerLineHint, at, err, diagnosticSnippet(rest))
	default:
		return fmt.Errorf("%w (%s): %s at byte %d is not a second JSON value: %v",
			ErrTrailingData, oneObjectPerLineHint, diagnosticSnippet(rest), at, err)
	}
}

// IsAmbiguousLineErr reports whether err is one of the physical-line
// corruption classes whose line holds more than one record's bytes — fused
// records and trailing data. Neither may be normalized away, because
// re-serializing the line would keep the first object and silently drop the
// remainder.
func IsAmbiguousLineErr(err error) bool {
	return errors.Is(err, ErrMultipleJSONValues) || errors.Is(err, ErrTrailingData)
}

// fusedLineNumbers returns the 1-based numbers of the physical lines in lines
// that carry more than one JSON value (or trailing data after the object).
// Blank lines and lines that fail in any OTHER way (truncated fragment,
// embedded raw newline) are not this class — they hold no hidden records and
// stay salvageable. Used by the repair path, which refuses ambiguous input.
func fusedLineNumbers(lines [][]byte) []int {
	var out []int
	for i, l := range lines {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		if _, err := ParseRow(l); err != nil && IsAmbiguousLineErr(err) {
			out = append(out, i+1)
		}
	}
	return out
}

// diagnosticSnippet renders a bounded, rune-safe excerpt of leftover bytes for
// an error message. Never a dump of a whole fused line.
func diagnosticSnippet(b []byte) string {
	const max = 80
	s := string(bytes.Join(bytes.Fields(b), []byte(" ")))
	if s == "" {
		return "…"
	}
	if r := []rune(s); len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}

// Get returns the raw value bytes for key, or nil when absent.
func (r *Row) Get(key string) json.RawMessage { return r.Vals[key] }

// Has reports whether key is present in the row.
func (r *Row) Has(key string) bool {
	_, ok := r.Vals[key]
	return ok
}

// SetRaw updates (or appends at the end) a field with raw JSON value bytes.
func (r *Row) SetRaw(key string, raw json.RawMessage) {
	if _, ok := r.Vals[key]; !ok {
		r.Keys = append(r.Keys, key)
	}
	r.Vals[key] = raw
}

// DeleteKey removes a key from the row entirely (a no-op when absent).
// DF-BOARDCTL-29: update --deferred false needs to UN-write the deferred key
// so an un-deferred row is byte-identical to a never-deferred one. The key
// disappears from both the value map and the key order; every other key
// keeps its exact position (Delete reorders nothing), so a Marshal after a
// Delete re-serializes only the surviving keys.
func (r *Row) DeleteKey(key string) {
	if _, ok := r.Vals[key]; !ok {
		return
	}
	delete(r.Vals, key)
	for i, k := range r.Keys {
		if k == key {
			r.Keys = append(r.Keys[:i], r.Keys[i+1:]...)
			return
		}
	}
}

// String returns the string value of a key, or "" when absent/null.
func (r *Row) String(key string) string {
	raw := r.Vals[key]
	if raw == nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// Int returns the int value of a key (only genuine JSON numbers; strings that
// merely look numeric are ignored) and whether the key held a number.
func (r *Row) Int(key string) (int64, bool) {
	raw := r.Vals[key]
	if raw == nil {
		return 0, false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return 0, false
	}
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

// IntOrZero returns the int value of a key or 0.
func (r *Row) IntOrZero(key string) int64 {
	i, _ := r.Int(key)
	return i
}

// Marshal serializes the row with the given style. Unchanged fields keep
// their original RawMessage bytes verbatim; only values replaced through
// SetGoValue / SetJSONString are re-encoded.
func (r *Row) Marshal(s Style) []byte {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range r.Keys {
		if i > 0 {
			b.WriteByte(',')
			if s.Spaced {
				b.WriteByte(' ')
			}
		}
		b.Write(jsonString(k, s))
		b.WriteByte(':')
		if s.Spaced {
			b.WriteByte(' ')
		}
		b.Write(r.Vals[k])
	}
	b.WriteByte('}')
	return b.Bytes()
}

// SetGoValue encodes a Go value (string, bool, json.Number/int64/float64,
// []string, []any, nil, json.RawMessage) as raw JSON honoring the style and
// stores it. json.RawMessage values are stored verbatim.
func (r *Row) SetGoValue(key string, v any, s Style) error {
	enc, err := encodeValue(v, s)
	if err != nil {
		return err
	}
	r.SetRaw(key, enc)
	return nil
}

// encodeValue renders a Go value as a JSON fragment in the given style.
func encodeValue(v any, s Style) ([]byte, error) {
	switch x := v.(type) {
	case nil:
		return []byte("null"), nil
	case json.RawMessage:
		return x, nil
	case []string:
		var b bytes.Buffer
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
				if s.Spaced {
					b.WriteByte(' ')
				}
			}
			b.Write(jsonString(e, s))
		}
		b.WriteByte(']')
		return b.Bytes(), nil
	case []any:
		var b bytes.Buffer
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
				if s.Spaced {
					b.WriteByte(' ')
				}
			}
			eb, err := encodeValue(e, s)
			if err != nil {
				return nil, err
			}
			b.Write(eb)
		}
		b.WriteByte(']')
		return b.Bytes(), nil
	case string, bool, int, int64, uint64, float64, json.Number:
		// Fast path: scalar types marshal deterministically via encoding/json.
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(x); err != nil {
			return nil, err
		}
		out := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
		if s.ASCII {
			out = escapeNonASCII(out)
		}
		return out, nil
	default:
		// Maps / structs from decoded JSON: encode generically, then apply
		// ascii escaping. Note: map key order is lost (sorted by encoding/json);
		// for full fidelity prefer json.RawMessage or the ordered encode below.
		enc := json.NewEncoder(&bytes.Buffer{})
		_ = enc
		buf, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		if s.ASCII {
			buf = escapeNonASCII(buf)
		}
		return buf, nil
	}
}

// jsonString renders a Go string as a JSON string literal. When s.ASCII is
// set every non-ASCII rune is escaped \uXXXX (astral runes as surrogate
// pairs), mirroring python json.dumps(..., ensure_ascii=True). encoding/json's
// default <,>,& HTML escaping is disabled to match python.
func jsonString(str string, s Style) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(str); err != nil {
		return []byte(`""`)
	}
	out := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	if s.ASCII {
		out = escapeNonASCII(out)
	}
	return out
}

// escapeNonASCII converts raw UTF-8 runes into \uXXXX escapes inside a JSON
// fragment. Non-ASCII bytes only ever occur inside JSON string literals, so a
// single pass over the byte stream (tracking string state to skip escaped
// quotes) is sufficient. Astral runes become surrogate pairs exactly like
// python's ensure_ascii.
func escapeNonASCII(b []byte) []byte {
	var out bytes.Buffer
	inStr := false
	esc := false
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case esc:
			out.WriteByte(c)
			esc = false
			i++
		case inStr && c == '\\':
			out.WriteByte(c)
			esc = true
			i++
		case c == '"':
			out.WriteByte(c)
			inStr = !inStr
			i++
		case inStr && c >= 0x80:
			r, size := decodeRune(b[i:])
			switch {
			case r < 0x10000:
				fmt.Fprintf(&out, `\u%04x`, r)
			default:
				r1 := 0xD800 + ((r - 0x10000) >> 10)
				r2 := 0xDC00 + ((r - 0x10000) & 0x3FF)
				fmt.Fprintf(&out, `\u%04x\u%04x`, r1, r2)
			}
			i += size
		default:
			out.WriteByte(c)
			i++
		}
	}
	return out.Bytes()
}

// decodeRune is a minimal UTF-8 decoder for the escape pass (stdlib utf8
// could be used directly; this keeps the loop tight and dependency-free of
// any import beyond unicode/utf8 below).
func decodeRune(b []byte) (rune, int) {
	if len(b) == 0 {
		return 0xFFFD, 1
	}
	c := b[0]
	switch {
	case c < 0x80:
		return rune(c), 1
	case c&0xE0 == 0xC0 && len(b) >= 2:
		return rune(c&0x1F)<<6 | rune(b[1]&0x3F), 2
	case c&0xF0 == 0xE0 && len(b) >= 3:
		return rune(c&0x0F)<<12 | rune(b[1]&0x3F)<<6 | rune(b[2]&0x3F), 3
	case c&0xF8 == 0xF0 && len(b) >= 4:
		return rune(c&0x07)<<18 | rune(b[1]&0x3F)<<12 | rune(b[2]&0x3F)<<6 | rune(b[3]&0x3F), 4
	default:
		return 0xFFFD, 1
	}
}

// PrettyIndent re-indents a raw JSON line for human display.
func PrettyIndent(raw []byte) []byte {
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return raw
	}
	return out.Bytes()
}
