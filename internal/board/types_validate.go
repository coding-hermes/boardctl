package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// BT-098 §3.4: the validate-side type-conformance check. Runs only when a
// valid types.jsonl exists (built-ins have no required fields, so a
// registry-less board adds zero findings):
//
//   - Row carries type: X, X unknown or retired → WARNING with a census
//     (panel amendment 3: the legacy-type cliff is grandfathered at warning
//     grade — never a hard error, never blocking other rows).
//   - X requires field F, F absent from detail.metadata (or metadata absent
//     entirely) → ERROR naming the field.
//   - F present with a value outside the enum → ERROR naming field and value.
//   - ref: task_id value that does not resolve (unknown id, the row itself,
//     or a fixture) → ERROR.
//   - Unknown metadata key present → WARNING (never an error).
//   - Rows with NO type key → no finding (legacy tolerance; the fleet's
//     existing rows must stay validate-clean).
//
// Registry conflict rows (duplicate slug, last-line-wins — panel amendment 2)
// are reported as warnings naming the offending lines here too; a corrupt
// row REFUSES the whole validate (error finding).
const TypeConformanceClass = "type-conformance"

func (b *Board) validateTypesConformance(rep *Report) error {
	reg, err := b.LoadTypes()
	if err != nil {
		rep.Add("error", "types.jsonl: %v", err)
		return nil
	}
	if reg.Source == "" {
		return nil // built-ins: zero findings, zero migration
	}
	// Conflicting rows degraded to last-line-wins are NAMED (recovery via
	// `boardctl types --doctor`).
	for _, c := range reg.Conflicts {
		rep.AddClass("warn", TypeConformanceClass, "%s", c.Detail)
	}
	rows, err := b.TaskRows()
	if err != nil {
		rep.Add("error", "tasks.jsonl: %v", err)
		return nil
	}
	fixtureIDs := map[string]bool{}
	if fid, ferr := b.FixtureIDs(); ferr == nil {
		fixtureIDs = fid
	}
	taskIDs := knownTaskIDs(rows)
	// undeclared census (panel amendment 3): value -> row count.
	undeclared := map[string]int{}
	findings := func(row *Row) {
		id := row.String("id")
		tname := row.String("type")
		if tname == "" {
			return // legacy tolerance: rows without a type key add nothing
		}
		t := reg.Active(tname)
		if t == nil {
			undeclared[tname]++
			rep.AddClass("warn", TypeConformanceClass,
				"BT-098: row %s claims type %q which types.jsonl does not declare (grandfathered at warning grade — declare the type or re-file the row)", id, tname)
			return
		}
		if t.Retired {
			rep.AddClass("warn", TypeConformanceClass,
				"BT-098: row %s carries retired type %q — the type stays resolvable for existing rows, but 'create --type %s' refuses it", id, tname, tname)
		}
		meta, _ := RowMetadata(row)
		metaOK := true
		rawMeta := row.Get("detail")
		trimmedDetail := bytes.TrimSpace(rawMeta)
		if len(trimmedDetail) > 0 && !bytes.Equal(trimmedDetail, []byte("null")) {
			if jsonKindName(trimmedDetail) != "object" {
				rep.AddClass("error", TypeConformanceClass,
					"BT-098: row %s has a non-object detail — metadata is a typed object at detail.metadata", id)
				metaOK = false
			} else {
				var obj map[string]json.RawMessage
				if json.Unmarshal(trimmedDetail, &obj) == nil {
					if m, ok := obj["metadata"]; ok && jsonKindName(m) != "object" {
						rep.AddClass("error", TypeConformanceClass,
							"BT-098: row %s has a non-object detail.metadata (%s) — metadata is a typed object keyed by field name", id, jsonKindName(m))
						metaOK = false
					}
				}
			}
		}
		if !metaOK {
			return // malformed metadata shape; the detail findings above cover it
		}
		declared := map[string]bool{}
		for _, f := range t.Metadata {
			declared[f.Name] = true
			val, have := metaProvided(meta, f.Name)
			if !have {
				if f.Required {
					rep.AddClass("error", TypeConformanceClass,
						"BT-098: row %s (type %q) is missing required metadata field %q (%s)",
						id, tname, f.Name, f.Constraint())
				}
				continue
			}
			if line := metaConformanceLine(f, MetaKV{Key: f.Name, Value: val}, id, taskIDs, fixtureIDs); line != "" {
				rep.AddClass("error", TypeConformanceClass, "BT-098: row %s (type %q) %s", id, tname, line)
			}
		}
		for _, kv := range meta {
			if !declared[kv.Key] {
				rep.AddClass("warn", TypeConformanceClass,
					"BT-098: row %s (type %q) carries undeclared metadata key %q — remove it or declare it in types.jsonl", id, tname, kv.Key)
			}
		}
	}
	for _, row := range rows {
		findings(row)
	}
	if len(undeclared) > 0 {
		vals := make([]string, 0, len(undeclared))
		for v, n := range undeclared {
			vals = append(vals, fmt.Sprintf("%q: %d row(s)", v, n))
		}
		sort.Strings(vals)
		total := 0
		for _, n := range undeclared {
			total += n
		}
		rep.AddClass("warn", TypeConformanceClass,
			"BT-098: census — %d row(s) carry type values types.jsonl does not declare (%s); grandfathered at warning grade, other rows are never blocked",
			total, strings.Join(vals, ", "))
	}
	return nil
}
