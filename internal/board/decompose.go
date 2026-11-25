package board

// BT-099 — `decompose`: scaffold N child rows from a parent row as template.
//
// Shape (panel amendments 2 and 6):
//   - the default is PARALLEL children (independent — a wave can absorb N
//     workers); --sequential chains child_i on child_{i-1} and the caller
//     prints the cost line (the CLI owns the wording).
//   - children inherit the parent's template: priority, capability_tags,
//     type + configured detail.metadata (which carries expected_files /
//     verify_steps per amendment 1). The final title is SET DIRECTLY — the
//     parent type's title_prefix is inherited exactly once through the parent
//     title, and create's prefix guard is a HasPrefix no-op on it (never
//     re-checked or double-applied).
//   - a child NEVER depends on its parent: the parent is a tracking shell,
//     the linkage is the parent key.
//
// Write path: one Create per child through the ordinary guarded create path —
// decompose is a LOOP over Create, not a bulk writer. A refusal mid-loop
// (duplicate id, duplicate finding, id-format) leaves the already-created
// children in place (append-only doctrine; re-running with --first-id
// resumes) and the parent's audit event is written only after ALL N children
// land.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// DecomposeOptions carries the CLI-resolved scaffold shape.
type DecomposeOptions struct {
	Count       int    // required, >= 1
	Parallel    bool   // independent children (the default when Sequential is false)
	Sequential  bool   // child_i depends on child_{i-1}
	FirstID     string // optional pin for the first child id
	TitlePrefix string // optional "<prefix> i/N" title form
}

// idFamilyRe splits a fleet id into its family prefix and numeric suffix
// ("BT-101" -> "BT", "101"). The family keeps interior hyphens (QA-BOARDCTL-1).
var idFamilyRe = regexp.MustCompile(`^(.+)-(\d+)$`)

// idFamily splits an id into (family, number, width). ok=false when the id
// has no `-<digits>` tail.
func idFamily(id string) (family string, num int, width int, ok bool) {
	m := idFamilyRe.FindStringSubmatch(id)
	if m == nil {
		return "", 0, 0, false
	}
	n, err := strconv.Atoi(m[2])
	if err != nil {
		return "", 0, 0, false
	}
	return m[1], n, len(m[2]), true
}

// mintFamilyID returns the next free id in the family: max+1 over the ids
// present in rows, observed width preserved (minimum 3), exactly the
// next-id allocation rule.
func mintFamilyID(rows []*Row, family string) string {
	idRe := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(family) + `-(\d+)$`)
	maxN, seenWidth := -1, 0
	for _, row := range rows {
		m := idRe.FindStringSubmatch(row.String("id"))
		if m == nil {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err == nil && n > maxN {
			maxN = n
		}
		if w := len(m[1]); w > seenWidth {
			seenWidth = w
		}
	}
	width := seenWidth
	if width < 3 {
		width = 3
	}
	seq := maxN + 1
	if seq < 1 {
		seq = 1
	}
	return family + "-" + fmt.Sprintf("%0*d", width, seq)
}

// inheritedMetadata decodes the parent's detail.metadata into the string map
// the create path re-encodes: declared list fields rejoin as comma strings,
// strings pass through, and any other JSON value rides its raw text so the
// bytes the CLI can author round-trip.
func inheritedMetadata(parent *Row) map[string]string {
	out := map[string]string{}
	pd := ParseFindingDetail(parent.Get("detail"))
	if pd.Extra == nil || len(pd.Extra["metadata"]) == 0 {
		return out
	}
	var stored map[string]json.RawMessage
	if err := json.Unmarshal(pd.Extra["metadata"], &stored); err != nil || stored == nil {
		return out
	}
	for key, raw := range stored {
		trimmed := bytes.TrimSpace(raw)
		var s string
		if err := json.Unmarshal(trimmed, &s); err == nil {
			out[key] = s
			continue
		}
		var list []string
		if err := json.Unmarshal(trimmed, &list); err == nil {
			out[key] = strings.Join(list, ",")
			continue
		}
		out[key] = string(trimmed)
	}
	return out
}

// Decompose scaffolds opts.Count children of parentID through the ordinary
// guarded Create path, then appends ONE task_updated event on the parent with
// detail {"decomposed_into": [ids]} so the audit trail shows the act. The
// returned ids are in scaffold order (child 1..N).
func (b *Board) Decompose(parentID string, opts DecomposeOptions) ([]string, error) {
	if opts.Count < 1 {
		return nil, fmt.Errorf("decompose: --count must be at least 1 (got %d)", opts.Count)
	}
	rows, err := b.TaskRows()
	if err != nil {
		return nil, err
	}
	var parent *Row
	for _, row := range rows {
		if row.String("id") == parentID {
			parent = row
			break
		}
	}
	if parent == nil {
		return nil, fmt.Errorf("decompose refused: task %q not found in tasks.jsonl", parentID)
	}
	if st := NormalizeStatus(parent.String("status")); st == "complete" || st == "failed" {
		return nil, fmt.Errorf("decompose refused: parent %q is complete — open rows only", parentID)
	}
	// Id family: --first-id's family when pinned, else the parent's own
	// family ("BT-099" scaffolds BT-1xx children in the BT family).
	family := ""
	if opts.FirstID != "" {
		if err := ValidateFleetTaskID(opts.FirstID); err != nil {
			return nil, fmt.Errorf("decompose: --first-id: %w", err)
		}
		fam, _, _, ok := idFamily(opts.FirstID)
		if !ok {
			return nil, fmt.Errorf("decompose: --first-id %q has no -<number> tail — subsequent ids cannot be minted next-id-style", opts.FirstID)
		}
		family = fam
	} else {
		fam, _, _, ok := idFamily(parentID)
		if !ok {
			return nil, fmt.Errorf("decompose refused: cannot derive an id family from parent id %q — pass --first-id", parentID)
		}
		family = fam
	}

	parentType := parent.String("type")
	parentPriority := parent.String("priority")
	parentTags := rowStringSlice(parent, "capability_tags")
	parentMeta := inheritedMetadata(parent)
	parentTitle := parent.String("title")

	ids := make([]string, 0, opts.Count)
	prev := "" // sequential chain: child_i depends on child_{i-1}
	for i := 1; i <= opts.Count; i++ {
		id := ""
		if i == 1 && opts.FirstID != "" {
			id = opts.FirstID
		} else {
			fresh, err := b.TaskRows()
			if err != nil {
				return ids, err
			}
			id = mintFamilyID(fresh, family)
		}
		title := fmt.Sprintf("%s — step %d/%d", parentTitle, i, opts.Count)
		if opts.TitlePrefix != "" {
			title = fmt.Sprintf("%s %d/%d", opts.TitlePrefix, i, opts.Count)
		}
		spec := TaskRowSpec{
			ID:           id,
			Title:        title,
			Status:       "pending",
			Priority:     parentPriority,
			Parent:       &parentID,
			Reasoning:    "",
			DependsOn:    []string{},
			HasDependsOn: false,
		}
		if len(parentTags) > 0 {
			spec.CapabilityTags = append([]string{}, parentTags...)
			spec.HasTags = true
		}
		if parentType != "" {
			spec.Type = parentType
			spec.HasType = true
			spec.Metadata = parentMeta
		}
		if opts.Sequential && prev != "" {
			spec.DependsOn = []string{prev}
			spec.HasDependsOn = true
		}
		if _, err := b.Create(spec); err != nil {
			if len(ids) > 0 {
				return ids, fmt.Errorf("decompose stopped after %d child(ren) (%s remain — the board is append-only; re-run with --first-id to resume): %w", len(ids), strings.Join(ids, ", "), err)
			}
			return nil, err
		}
		ids = append(ids, id)
		prev = id
	}
	detail := map[string][]string{"decomposed_into": ids}
	enc, err := json.Marshal(detail)
	if err != nil {
		return ids, err
	}
	if _, err := b.AppendEvent(EventSpec{
		Type:   "task_updated",
		TaskID: parentID,
		Detail: enc,
	}); err != nil {
		return ids, fmt.Errorf("children scaffolded but the parent's decomposed_into event failed: %w", err)
	}
	return ids, nil
}
