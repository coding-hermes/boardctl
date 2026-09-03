package board

import (
	"regexp"
	"strings"
	"time"
)

// Timestamp mirroring. Boards write timestamps in several dialects, all
// produced by python fleet scripts:
//
//	space-naive:  "2026-09-02 05:15:00"            (strftime %Y-%m-%d %H:%M:%S)
//	space-const:  "2026-08-27 16:07:50.000000"     (strftime ... .000000)
//	space-trim:   "2026-08-01 00:22:08.92693"      (utcnow().isoformat(), trailing zeros trimmed)
//	T-naive:      "2026-08-27T08:22:43"
//	T-offset:     "2026-08-31T05:13:59.558464+00:00" (datetime.now(timezone.utc).isoformat())
//
// boardctl samples the timestamp field of the row/file it is about to touch
// and formats new timestamps in the same dialect.

var (
	tsSpaceRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`)
	tsTRe     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`)
	fracRe    = regexp.MustCompile(`\.\d+`)
	zoneRe    = regexp.MustCompile(`(?:Z|[+-]\d{2}:?\d{2})$`)
)

// DefaultTSLayout mirrors the spec default:
// datetime.now(timezone.utc).isoformat() -> microsecond fraction, +00:00.
const DefaultTSLayout = "2006-01-02T15:04:05.999999-07:00"

// TSFormat holds one detected timestamp dialect as a Go layout string.
type TSFormat struct {
	Layout string
}

// Now formats the current UTC time in the dialect.
func (f TSFormat) Now() string {
	return time.Now().UTC().Format(f.Layout)
}

// DetectTSLayout samples a timestamp string and returns a Go layout that
// reproduces its dialect. Empty/unparseable samples fall back to the spec
// default T-offset microsecond form.
func DetectTSLayout(sample string) TSFormat {
	sample = strings.TrimSpace(sample)
	if sample == "" {
		return TSFormat{Layout: DefaultTSLayout}
	}
	var base string
	switch {
	case tsTRe.MatchString(sample):
		base = "2006-01-02T15:04:05"
	case tsSpaceRe.MatchString(sample):
		base = "2006-01-02 15:04:05"
	default:
		return TSFormat{Layout: DefaultTSLayout}
	}
	frac := ""
	if m := fracRe.FindString(sample); m != "" {
		digits := strings.TrimPrefix(m, ".")
		if allZero(digits) {
			// constant-width fraction (strftime .000000 etc.)
			frac = m
		} else {
			// trailing-zero trimming fraction, like python isoformat
			frac = ".999999"
		}
	}
	zone := ""
	if zoneRe.MatchString(sample) {
		if strings.HasSuffix(sample, "Z") {
			zone = "Z07:00" // renders literal Z for UTC
		} else {
			zone = "-07:00" // renders +00:00 for UTC
		}
	}
	return TSFormat{Layout: base + frac + zone}
}

func allZero(digits string) bool {
	for _, c := range digits {
		if c != '0' {
			return false
		}
	}
	return true
}
