package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// command next-id — allocate the next free id for a board family.
//
// WHY (2026-11-03, the code-factory criterion): `boardctl create` REQUIRES an
// explicit --id and refuses a duplicate, which is correct but pushes allocation
// onto whoever is filing. A board worked by several lanes at once (code-factory
// runs five regularly) then has every lane computing "the next id" from the same
// snapshot: they all answer CF-153, one create wins, and the rest abort on
// "already exists" — a wasted tick each, and the row an operator expected is
// silently missing. That is the same collision the fleet has had to de-duplicate
// by hand (duplicate-id-dedup), arriving through a different door.
//
// SCOPE, STATED HONESTLY: this command reads the board and prints a free id. It
// does not reserve it, so two callers racing inside the same second can still
// both be told CF-153 — the SECOND create then refuses (exit 1) rather than
// writing a duplicate row, so the failure mode is a retry, never a split-brain
// board. Callers that must not race should take the id, create, and on refusal
// re-run next-id.
//
// Allocation is max+1 rather than gap-filling: a closed id is history that events
// and commits already reference, and re-issuing it would silently re-point them.
func cmdNextID(dir string, args []string) error {
	fs := newFlagSet("next-id")
	args = reorderArgs(args, valueFlags("C", "family", "width"))
	family := fs.String("family", "", "id family/prefix to allocate for (e.g. CF, BT, TR)")
	width := fs.Int("width", 0, "zero-pad the numeric part to this width (default: the width observed on the board, minimum 3)")
	asJSON := fs.Bool("json", false, "emit {\"id\":...} JSON instead of a bare id")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "boardctl next-id --family CF [--width N] [--json] [-C dir]\n")
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("next-id takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}
	if *family == "" {
		return fmt.Errorf("next-id requires --family (the id prefix, e.g. --family CF)")
	}
	fam := strings.TrimSuffix(strings.TrimSpace(*family), "-")
	if fam == "" {
		return fmt.Errorf("--family must contain at least one character")
	}

	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	rows, err := b.TaskRows()
	if err != nil {
		return err
	}

	// Case-insensitive family match, but the width comes from the ids that
	// actually exist so CF-015 style boards keep their padding.
	idRe := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(fam) + `-(\d+)$`)
	maxN, seenWidth, used := -1, 0, 0
	for _, r := range rows {
		id := r.String("id")
		m := idRe.FindStringSubmatch(id)
		if m == nil {
			continue
		}
		used++
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if n > maxN {
			maxN = n
		}
		if w := len(m[1]); w > seenWidth {
			seenWidth = w
		}
	}

	w := *width
	if w <= 0 {
		w = seenWidth
		if w < 3 {
			w = 3
		}
	}
	// A brand-new family starts at 1 (maxN stays -1 when nothing matched); "-000"
	// would be a legal id but nothing in the fleet has ever allocated a zero.
	seq := maxN + 1
	if seq < 1 {
		seq = 1
	}
	next := fam + "-" + fmt.Sprintf("%0*d", w, seq)

	if *asJSON {
		out, err := json.Marshal(map[string]any{
			"id":     next,
			"family": fam,
			"used":   used,
			"max":    maxN,
			"width":  w,
		})
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}
	fmt.Println(next)
	// stderr, so `id=$(boardctl next-id --family CF)` stays clean.
	if used > 0 {
		_, _ = fmt.Fprintf(fs.Output(), "boardctl: %s is the next free id after %d existing %s row(s) (max %d); create refuses a duplicate, so a lost race is a retry, not a second row\n",
			next, used, fam, maxN)
	}
	return nil
}
