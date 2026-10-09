package board

import (
	"strconv"
	"strings"
	"time"
)

// Timestamp VALIDATION (DF-BOARDCTL-27).
//
// The report surface (internal/render load.go buildTaskRec) excludes a task
// row from time-based metrics when created_at/completed_at (and the other
// task timestamp fields) fail its parser — and says so in its notes. validate
// said nothing: the repo's own board carried created_at values in a
// double-dash dialect ("2026-09-09-22T15:13:00Z") and rows with NO created_at
// at all that validate passed silently while the report parser flagged them.
// The two integrity surfaces must agree.
//
// SINGLE GRAMMAR, MIRRORED: the report parser's parseStamp/stampLayouts live
// in internal/render and cannot be imported here (render imports board), so
// ValidTimestamp mirrors that grammar layout-for-layout (same list, same
// TrimSpace-empty-fails rule). A value parseStamp accepts, ValidTimestamp
// accepts, and nothing else — a value one surface flags, both flag. The
// internal/render df_boardctl_27_test.go pins the parity from the render
// side, which CAN import board.
//
// The per-field rules below mirror the report's EXCLUSION rules, not just
// its grammar (buildTaskRec):
//   - created_at: absent, null, non-string, empty, or unparseable ALWAYS
//     excludes the row (birth) — one warning each.
//   - completed_at: a present-but-unparseable value excludes the row only
//     when status is NOT complete; on a complete row the report falls back
//     to updated_at for the done day and never excludes (a silent
//     data-quality footnote), so validate stays silent there too. An
//     empty-string (or non-string) completed_at reads as absent on both
//     surfaces.
//   - updated_at / dispatched_at / blocked_since: a present-but-unparseable
//     value excludes the row on every status.
//
// Like every cross-check in validate.go this is an itemized WARNING: live
// boards carry legacy rows that must stay exit-0 while the drift stays
// visible and repairable.

// timestampLayouts mirrors internal/render/load.go stampLayouts (the
// DF-BOARDCTL-3 dialect set): space/T base x optional fraction x zone spelled
// Z07:00 (±hh:mm, Z) or -0700 (±hhmm), glued or space-separated. KEEP IN
// SYNC — ValidTimestamp must accept exactly what parseStamp accepts.
var timestampLayouts = []string{
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05.999999999-0700",
	"2006-01-02 15:04:05-0700",
	"2006-01-02 15:04:05.999999999 -0700",
	"2006-01-02 15:04:05 -0700",
	"2006-01-02 15:04:05.999999999 Z07:00",
	"2006-01-02 15:04:05 Z07:00",
	"2006-01-02T15:04:05.999999999Z07:00",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05.999999999-0700",
	"2006-01-02T15:04:05-0700",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
}

// ValidTimestamp reports whether s parses under the report parser's timestamp
// grammar (parseStamp/stampLayouts). Empty/whitespace-only returns false —
// parseStamp excludes empty values and so does the report.
func ValidTimestamp(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, layout := range timestampLayouts {
		if _, err := time.Parse(layout, s); err == nil {
			return true
		}
	}
	return false
}

// TimestampFinding is one itemized timestamp defect on one task row, for the
// DF-BOARDCTL-27 validate warning. Field names the offending key; Detail is
// the value clause for the message (the reason plus the raw value where a
// value exists).
type TimestampFinding struct {
	Field  string
	Detail string
}

// TimestampFieldFindings applies the report parser's timestamp grammar and
// exclusion rules to one task row, returning one finding per field that gets
// the row excluded from time-based metrics. See the file comment for the
// per-field rules.
func TimestampFieldFindings(row *Row) []TimestampFinding {
	st := NormalizeStatus(row.String("status"))
	var out []TimestampFinding
	// birth: created_at excludes the row when it is absent, null, a
	// non-string value, an empty string, or unparseable (report: the
	// parseStamp(String()) failure is unconditional).
	if raw := row.Get("created_at"); raw == nil {
		out = append(out, TimestampFinding{"created_at", "absent"})
	} else if string(raw) == "null" {
		out = append(out, TimestampFinding{"created_at", "absent (null)"})
	} else if s, ok := decodeJSONString(raw); !ok {
		out = append(out, TimestampFinding{"created_at", "not a timestamp string (JSON " + jsonKindName(raw) + ")"})
	} else if strings.TrimSpace(s) == "" {
		out = append(out, TimestampFinding{"created_at", `empty string ""`})
	} else if !ValidTimestamp(s) {
		out = append(out, TimestampFinding{"created_at", "unparseable " + strconv.Quote(s)})
	}
	// done day: completed_at excludes the row only on a NON-complete row
	// with a present-but-unparseable value (the report falls back to
	// updated_at on complete rows and never excludes there; empty and
	// non-string values read as absent on both surfaces).
	if v := row.String("completed_at"); strings.TrimSpace(v) != "" && !ValidTimestamp(v) && st != "complete" {
		out = append(out, TimestampFinding{"completed_at", "unparseable " + strconv.Quote(v)})
	}
	// the remaining timestamp fields exclude the row on every status when a
	// non-empty value fails to parse.
	for _, f := range []string{"updated_at", "dispatched_at", "blocked_since"} {
		if v := row.String(f); strings.TrimSpace(v) != "" && !ValidTimestamp(v) {
			out = append(out, TimestampFinding{f, "unparseable " + strconv.Quote(v)})
		}
	}
	return out
}

// TimestampFieldFixHint names the repair path for a broken timestamp field:
// update --completed-at exists for completed_at; created_at and the other
// fields are hand-edit repairs (no update flag writes them).
func TimestampFieldFixHint(id, field string) string {
	if field == "completed_at" {
		return "fix with 'boardctl update " + id + " --completed-at <RFC3339 timestamp>'"
	}
	return "hand-edit the row to add a parseable " + field
}
