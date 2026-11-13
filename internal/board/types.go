package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// splitMetaList splits a comma-separated list value (the list-field CLI
// convention, same as --depends-on).
func splitMetaList(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// BT-098 — Ticket types + templates + configurable metadata.
//
// A board may declare its own ticket types in <boarddir>/types.jsonl (sibling
// of tasks.jsonl). The file is append-only, git-backed JSONL, exactly like the
// board itself: a type is never edited in place — a change appends a new line
// with the same slug and a bumped rev, and the LAST line for a slug wins. To
// retire a type, append {"slug": X, "retired": true, ...}.
//
// Zero migration: when types.jsonl does not exist (or is empty) the registry
// is the four built-ins (task|bug|feat|chore), which carry NO template and NO
// required metadata — every existing behavior is byte-for-byte unchanged.

// TypeTemplate is a ticket type's create-time template. All three keys are
// optional; a key that is absent simply does nothing. An explicitly passed
// flag ALWAYS beats the template (BT-071 precedence rule): the template fills
// only what the caller omitted.
type TypeTemplate struct {
	TitlePrefix     string // applied when --title does not already start with it (never doubled)
	ACSkeleton      string // applied when --acceptance-criteria is omitted
	DefaultPriority string // applied when --priority is omitted
}

// MetaFieldTypes is the closed set of metadata field types.
var MetaFieldTypes = map[string]bool{
	"string": true,
	"enum":   true,
	"list":   true,
	"ref":    true,
}

// RefTaskID is the only ref target v1 resolves: validate checks that the
// value matches an existing task row id (not the row itself, not a fixture).
const RefTaskID = "task_id"

// MetaField is one named field of a type's metadata schema.
type MetaField struct {
	Name        string
	Type        string // string | enum | list | ref
	Required    bool
	Description string
	Values      []string // enum only: the closed vocabulary
	Ref         string   // ref only: what the value references
}

// Constraint renders the field's constraint suffix used in refusals and
// findings: `enum: stable|beta|nightly`, `string`, `list`, `ref: task_id`.
func (f MetaField) Constraint() string {
	switch f.Type {
	case "enum":
		return "enum: " + strings.Join(f.Values, "|")
	case "ref":
		if f.Ref != "" {
			return "ref: " + f.Ref
		}
		return "ref"
	default:
		return f.Type
	}
}

// TicketType is one declared ticket type (the winning line for its slug).
type TicketType struct {
	Slug     string
	Display  string
	Rev      int64
	Retired  bool
	Template *TypeTemplate
	Metadata []MetaField
	Line     int // 1-based line in types.jsonl of the winning declaration (0 for built-ins)
}

// Field returns the type's declared metadata field with the given name.
func (t *TicketType) Field(name string) (MetaField, bool) {
	for _, f := range t.Metadata {
		if f.Name == name {
			return f, true
		}
	}
	return MetaField{}, false
}

// TypeConflict records a duplicate-slug line the loader degraded to
// last-line-wins (panel amendment 2: a strict rev rule would brick the whole
// registry after any two-lane git merge — flock serializes writers, not
// merges). `boardctl types --doctor` names the offending lines.
type TypeConflict struct {
	Slug   string
	Line   int    // the offending (losing) line
	Winner int    // the line that wins
	Detail string // human explanation
}

// TypeRegistry is the loaded types.jsonl (or the built-in defaults).
type TypeRegistry struct {
	Types     map[string]*TicketType // slug -> winning declaration
	Order     []string               // first-declaration slug order
	Conflicts []TypeConflict         // duplicate-slug degradations (last-line-wins)
	Warnings  []string               // canon-drift warnings (unknown keys in type rows)
	Source    string                 // path loaded, "" = built-in defaults
}

// Active returns the type for slug (nil when unknown). Retired types stay
// resolvable here — create refuses them one gate later.
func (r *TypeRegistry) Active(slug string) *TicketType {
	return r.Types[slug]
}

// ActiveSlugs returns every resolvable slug (retired included), sorted.
func (r *TypeRegistry) ActiveSlugs() []string {
	out := make([]string, 0, len(r.Types))
	for s := range r.Types {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// ActiveSorted returns the ACTIVE (non-retired) types sorted by slug — the
// order `boardctl types` lists them in.
func (r *TypeRegistry) ActiveSorted() []*TicketType {
	var out []*TicketType
	for _, s := range r.ActiveSlugs() {
		if t := r.Types[s]; !t.Retired {
			out = append(out, t)
		}
	}
	return out
}

// RetiredSorted returns the RETIRED types sorted by slug.
func (r *TypeRegistry) RetiredSorted() []*TicketType {
	var out []*TicketType
	for _, s := range r.ActiveSlugs() {
		if t := r.Types[s]; t.Retired {
			out = append(out, t)
		}
	}
	return out
}

// BuiltInSlugs are the four default types every board has when types.jsonl is
// absent or empty. Built-ins carry NO template and NO required metadata, so
// existing behavior is byte-for-byte unchanged until an operator writes a
// types.jsonl (zero migration).
var BuiltInSlugs = []string{"task", "bug", "feat", "chore"}

// BuiltInTypes builds the default registry (no types.jsonl).
func BuiltInTypes() *TypeRegistry {
	reg := &TypeRegistry{Types: map[string]*TicketType{}}
	builtins := map[string]string{
		"task":  "Task",
		"bug":   "Bug",
		"feat":  "Feature",
		"chore": "Chore",
	}
	for _, slug := range BuiltInSlugs {
		reg.Types[slug] = &TicketType{Slug: slug, Display: builtins[slug], Rev: 1}
		reg.Order = append(reg.Order, slug)
	}
	return reg
}

// TypesPath is the board dir's types.jsonl path.
func (b *Board) TypesPath() string { return filepath.Join(b.Dir, "types.jsonl") }

// LoadTypes loads this board's type registry: types.jsonl when present and
// non-empty, the built-in defaults otherwise. Corrupt (unparseable) rows
// REFUSE with the line named; conflicting rows (duplicate slug with a
// non-increasing rev) degrade to last-line-wins with a named finding in
// reg.Conflicts — never a refusal (panel amendment 2).
func (b *Board) LoadTypes() (*TypeRegistry, error) {
	return LoadTypesFromDir(b.Dir)
}

// LoadTypesFromDir loads the registry from a board dir without a Board value.
func LoadTypesFromDir(dir string) (*TypeRegistry, error) {
	path := filepath.Join(dir, "types.jsonl")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			reg := BuiltInTypes()
			return reg, nil
		}
		return nil, err
	}
	// 0 bytes (or all blank) is treated as absent, matching how an empty
	// tasks.jsonl is handled.
	if len(bytes.TrimSpace(content)) == 0 {
		return BuiltInTypes(), nil
	}
	reg := &TypeRegistry{Types: map[string]*TicketType{}, Source: path}
	lines := bytes.Split(content, []byte("\n"))
	for i, l := range lines {
		lineNo := i + 1
		trimmed := bytes.TrimSpace(l)
		if len(trimmed) == 0 {
			continue
		}
		row, err := ParseRow(trimmed)
		if err != nil {
			return nil, fmt.Errorf("types.jsonl line %d: %v — fix the file and re-run", lineNo, err)
		}
		t, warns, err := decodeTypeRow(row, lineNo)
		if err != nil {
			return nil, err
		}
		reg.Warnings = append(reg.Warnings, warns...)
		if prev, dup := reg.Types[t.Slug]; dup {
			// Panel amendment 2: a duplicate slug is a CONFLICT, not a
			// refusal — a git merge can interleave two lanes' appends with
			// the same rev, and refusing the whole registry would brick
			// every command on the board. Last line wins; the degradation
			// is named (validate warns, `types --doctor` reports lines).
			reg.Conflicts = append(reg.Conflicts, TypeConflict{
				Slug:   t.Slug,
				Line:   lineNo,
				Winner: lineNo,
				Detail: fmt.Sprintf("slug %q declared again on line %d (rev %d) over line %d (rev %d) — the LAST line wins",
					t.Slug, lineNo, t.Rev, prev.Line, prev.Rev),
			})
		} else {
			reg.Order = append(reg.Order, t.Slug)
		}
		reg.Types[t.Slug] = t // last line wins
	}
	return reg, nil
}

// knownTypeRowKeys is the type-row schema. Unknown keys are a WARN (canon
// drift: report, never guess, never refuse a config file the operator owns).
var knownTypeRowKeys = map[string]bool{
	"slug": true, "display": true, "rev": true, "retired": true,
	"template": true, "metadata": true,
}

var metaFieldNameRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// decodeTypeRow validates one parsed type-declaration line and returns the
// TicketType plus any unknown-key warnings.
func decodeTypeRow(row *Row, lineNo int) (*TicketType, []string, error) {
	var warns []string
	for _, k := range row.Keys {
		if !knownTypeRowKeys[k] {
			warns = append(warns, fmt.Sprintf("types.jsonl line %d: unknown key %q in type row — it was ignored (canon drift: declare only slug/display/rev/retired/template/metadata)", lineNo, k))
		}
	}
	t := &TicketType{Line: lineNo}
	t.Slug = row.String("slug")
	if t.Slug == "" {
		return nil, warns, fmt.Errorf("types.jsonl line %d: type row has no slug — every declaration names its type", lineNo)
	}
	if err := ValidateBoardSlug("ticket type", t.Slug); err != nil {
		return nil, warns, fmt.Errorf("types.jsonl line %d: %v", lineNo, err)
	}
	t.Display = row.String("display")
	if t.Display == "" {
		return nil, warns, fmt.Errorf("types.jsonl line %d: type %q has no display name", lineNo, t.Slug)
	}
	t.Rev = 1
	if raw := row.Get("rev"); raw != nil {
		rev, ok := row.Int("rev")
		if !ok {
			return nil, warns, fmt.Errorf("types.jsonl line %d: type %q rev is not an integer", lineNo, t.Slug)
		}
		t.Rev = rev
	}
	if raw := row.Get("retired"); raw != nil {
		if kind := jsonKindName(raw); kind != "boolean" && kind != "null" {
			return nil, warns, fmt.Errorf("types.jsonl line %d: type %q retired is not a boolean (%s)", lineNo, t.Slug, kind)
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("true")) {
			t.Retired = true
		}
	}
	if raw := row.Get("template"); raw != nil && jsonKindName(raw) == "object" {
		tt := &TypeTemplate{}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err == nil {
			decodeStr := func(key string) string {
				if v, ok := obj[key]; ok {
					var s string
					if err := json.Unmarshal(v, &s); err == nil {
						return s
					}
				}
				return ""
			}
			tt.TitlePrefix = decodeStr("title_prefix")
			tt.ACSkeleton = decodeStr("ac_skeleton")
			tt.DefaultPriority = decodeStr("default_priority")
			if tt.DefaultPriority != "" {
				if canon := NormalizePriority(tt.DefaultPriority); PriorityVocabulary[canon] {
					tt.DefaultPriority = canon
				} else {
					return nil, warns, fmt.Errorf("types.jsonl line %d: type %q template default_priority %q is not in vocabulary {P0,P1,P2,P3,P4,P5}", lineNo, t.Slug, tt.DefaultPriority)
				}
			}
			t.Template = tt
		}
	}
	if raw := row.Get("metadata"); raw != nil {
		if kind := jsonKindName(raw); kind != "array" {
			return nil, warns, fmt.Errorf("types.jsonl line %d: type %q metadata is not an array (%s)", lineNo, t.Slug, kind)
		}
		var rawFields []json.RawMessage
		if err := json.Unmarshal(raw, &rawFields); err != nil {
			return nil, warns, fmt.Errorf("types.jsonl line %d: type %q metadata is not an array of fields: %v", lineNo, t.Slug, err)
		}
		seen := map[string]bool{}
		for fi, rf := range rawFields {
			frow, err := ParseRow(rf)
			if err != nil {
				return nil, warns, fmt.Errorf("types.jsonl line %d: type %q metadata field %d is not a JSON object: %v", lineNo, t.Slug, fi+1, err)
			}
			for _, k := range frow.Keys {
				switch k {
				case "name", "type", "required", "description", "values", "ref":
				default:
					warns = append(warns, fmt.Sprintf("types.jsonl line %d: type %q metadata field %d has unknown key %q — it was ignored (canon drift: declare only name/type/required/description/values/ref)", lineNo, t.Slug, fi+1, k))
				}
			}
			f := MetaField{Type: "string"}
			f.Name = frow.String("name")
			if f.Name == "" {
				return nil, warns, fmt.Errorf("types.jsonl line %d: type %q metadata field %d has no name", lineNo, t.Slug, fi+1)
			}
			if len(f.Name) > 64 || !metaFieldNameRE.MatchString(f.Name) {
				return nil, warns, fmt.Errorf("types.jsonl line %d: type %q metadata field %d name %q is not a lowercase snake_case identifier (^[a-z][a-z0-9_]*$, max 64)", lineNo, t.Slug, fi+1, f.Name)
			}
			if seen[f.Name] {
				return nil, warns, fmt.Errorf("types.jsonl line %d: type %q declares metadata field %q twice", lineNo, t.Slug, f.Name)
			}
			seen[f.Name] = true
			if raw := frow.Get("type"); raw != nil {
				ft := frow.String("type")
				if !MetaFieldTypes[ft] {
					return nil, warns, fmt.Errorf("types.jsonl line %d: type %q metadata field %q has unknown type %q (valid: string, enum, list, ref)", lineNo, t.Slug, f.Name, ft)
				}
				f.Type = ft
			}
			if raw := frow.Get("required"); raw != nil {
				if bytes.Equal(bytes.TrimSpace(raw), []byte("true")) {
					f.Required = true
				} else if kind := jsonKindName(raw); kind != "boolean" && kind != "null" {
					return nil, warns, fmt.Errorf("types.jsonl line %d: type %q metadata field %q required is not a boolean (%s)", lineNo, t.Slug, f.Name, kind)
				}
			}
			f.Description = frow.String("description")
			if f.Type == "enum" {
				if err := json.Unmarshal(frow.Get("values"), &f.Values); err != nil || len(f.Values) == 0 {
					return nil, warns, fmt.Errorf("types.jsonl line %d: type %q enum field %q has no values — an enum without its closed vocabulary is a config error", lineNo, t.Slug, f.Name)
				}
			}
			if f.Type == "ref" {
				f.Ref = frow.String("ref")
				if f.Ref != RefTaskID {
					return nil, warns, fmt.Errorf("types.jsonl line %d: type %q ref field %q names ref target %q — %q is the only target v1 resolves", lineNo, t.Slug, f.Name, f.Ref, RefTaskID)
				}
			}
			t.Metadata = append(t.Metadata, f)
		}
	}
	return t, warns, nil
}

// ValidateBoardSlug is the shared slug rule (BT-098 reuses
// ValidateInitiativeSlug's grammar via this shared rule so a bad --type value
// gets the same "did you mean" hint). label names the value's role in error
// text ("initiative", "ticket type").
func ValidateBoardSlug(label, s string) error {
	if s == "" {
		return fmt.Errorf("%s must not be empty", label)
	}
	if len(s) > 64 {
		return fmt.Errorf("%s %q is %d chars; the limit is 64", label, s, len(s))
	}
	if initiativeSlugRE.MatchString(s) {
		return nil
	}
	// One suggestion, computed the way a human would fix it: lowercase and
	// collapse every run of non-slug characters into a single hyphen.
	suggestion := strings.ToLower(strings.Trim(s, " -_"))
	for _, r := range []string{" ", "_", "--", "..", "/", "\\"} {
		for strings.Contains(suggestion, r) {
			suggestion = strings.ReplaceAll(suggestion, r, "-")
		}
	}
	suggestion = strings.Trim(suggestion, "-")
	if suggestion == "" || len(suggestion) > 64 || !initiativeSlugRE.MatchString(suggestion) {
		return fmt.Errorf("%s %q is not a slug: use lowercase letters, digits and single hyphens, starting and ending with a letter or digit (e.g. \"lane-hooks\")", label, s)
	}
	return fmt.Errorf("%s %q is not a slug: use lowercase letters, digits and single hyphens, starting and ending with a letter or digit — did you mean %q?", label, s, suggestion)
}

// MetaKV is one --meta key=value entry in CLI order.
type MetaKV struct {
	Key   string
	Value string
}

// ParseMetaKVs splits raw key=value strings on the FIRST '=' (a value may
// itself contain '=' or newlines). Duplicate keys are a usage error caught
// before the board opens.
func ParseMetaKVs(raws []string) ([]MetaKV, error) {
	out := make([]MetaKV, 0, len(raws))
	seen := map[string]bool{}
	for _, raw := range raws {
		k, v, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("--meta %q is not key=value — split on the first '=' (e.g. --meta channel=stable)", raw)
		}
		k = strings.TrimSpace(k)
		if k == "" {
			return nil, fmt.Errorf("--meta %q has an empty key", raw)
		}
		if seen[k] {
			return nil, fmt.Errorf("--meta %q given twice — one value per field", k)
		}
		seen[k] = true
		out = append(out, MetaKV{Key: k, Value: v})
	}
	return out, nil
}

// metaProvided reports whether the --meta list supplies key with a non-empty
// value, and returns the value when it does.
func metaProvided(meta []MetaKV, key string) (string, bool) {
	for _, kv := range meta {
		if kv.Key == key {
			if strings.TrimSpace(kv.Value) == "" {
				return "", false
			}
			return kv.Value, true
		}
	}
	return "", false
}

// metadataValueBytes renders one metadata value in the given style: a list
// field's value is split on commas into a JSON array (the same convention as
// --depends-on); every other value is a JSON string verbatim (free text,
// including '=' and newlines — the row's own serializer escapes them).
func metadataValueBytes(f *MetaField, value string, style Style) ([]byte, error) {
	if f != nil && f.Type == "list" {
		return encodeValue(splitMetaList(value), style)
	}
	return encodeValue(value, style)
}

// encodeMetadataObject renders ordered pairs as one JSON object in the given
// style, using the type's declared field types for value shapes.
func encodeMetadataObject(meta []MetaKV, t *TicketType, style Style) ([]byte, error) {
	row := &Row{Vals: map[string]json.RawMessage{}}
	for _, kv := range meta {
		var f *MetaField
		if t != nil {
			if fld, ok := t.Field(kv.Key); ok {
				f = &fld
			}
		}
		enc, err := metadataValueBytes(f, kv.Value, style)
		if err != nil {
			return nil, err
		}
		row.SetRaw(kv.Key, enc)
	}
	if len(row.Keys) == 0 {
		return []byte("{}"), nil
	}
	return row.Marshal(style), nil
}

// mergeRowMetadata folds meta into an existing detail.metadata object (an
// ordered Row round-trip): existing keys keep their position and get new
// values, new keys append. cur is the raw metadata object bytes (nil/empty =
// none yet).
func mergeRowMetadata(cur []byte, meta []MetaKV, t *TicketType, style Style) ([]byte, error) {
	var mrow *Row
	if kb := bytes.TrimSpace(cur); len(kb) > 0 && jsonKindName(kb) == "object" {
		if r, err := ParseRow(kb); err == nil {
			mrow = r
		}
	}
	if mrow == nil {
		mrow = &Row{Vals: map[string]json.RawMessage{}}
	}
	for _, kv := range meta {
		var f *MetaField
		if t != nil {
			if fld, ok := t.Field(kv.Key); ok {
				f = &fld
			}
		}
		enc, err := metadataValueBytes(f, kv.Value, style)
		if err != nil {
			return nil, err
		}
		mrow.SetRaw(kv.Key, enc)
	}
	if len(mrow.Keys) == 0 {
		return []byte("{}"), nil
	}
	return mrow.Marshal(style), nil
}

// setRowMetadata merges ordered metadata pairs into a row's detail envelope
// at detail.metadata, preserving every existing detail key verbatim. Refuses
// a non-object detail (spec §6 edge case: metadata is a typed object at
// detail.metadata — never loose top-level keys, never a smashed prose
// detail).
func setRowMetadata(row *Row, t *TicketType, meta []MetaKV, style Style) error {
	if len(meta) == 0 {
		return nil
	}
	raw := row.Get("detail")
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) != 0 && !bytes.Equal(trimmed, []byte("null")) && jsonKindName(trimmed) != "object" {
		return fmt.Errorf("row %s: detail is not a JSON object — refusing to attach metadata (fix by hand)", row.String("id"))
	}
	var cur []byte
	if jsonKindName(trimmed) == "object" {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &obj); err == nil {
			if m, ok := obj["metadata"]; ok && jsonKindName(m) == "object" {
				cur = m
			}
		}
	}
	metaBytes, err := mergeRowMetadata(cur, meta, t, style)
	if err != nil {
		return err
	}
	pd := ParseFindingDetail(raw)
	pd.Object = true
	if pd.Extra == nil {
		pd.Extra = map[string]json.RawMessage{}
	}
	pd.Extra["metadata"] = metaBytes
	enc, err := EncodeFindingDetail(pd, style)
	if err != nil {
		return err
	}
	row.SetRaw("detail", enc)
	return nil
}

// metaMissingLine renders one field-by-field missing line for the create /
// update refusal (spec §3.2 step 4 — ALL missing fields listed in
// types.jsonl declaration order, never fail-first):
//   - source_commit (string) — required: commit the release was cut from
func metaMissingLine(f MetaField) string {
	desc := "required"
	if f.Description != "" {
		desc += ": " + f.Description
	}
	return fmt.Sprintf("%s (%s) — %s", f.Name, f.Constraint(), desc)
}

// knownTaskIDs collects the ids of parsed task rows (for ref: task_id checks).
func knownTaskIDs(rows []*Row) map[string]bool {
	out := make(map[string]bool, len(rows))
	for _, r := range rows {
		if id := r.String("id"); id != "" {
			out[id] = true
		}
	}
	return out
}

// metaConformanceLine returns a refusal line when kv's value does not conform
// to its declared field (enum membership, ref: task_id resolution), "" when
// it conforms. A ref value must match an existing task id and may not point
// at the row itself or a fixture (a self-approval and a fixture are not
// approvals — spec §6).
func metaConformanceLine(f MetaField, kv MetaKV, selfID string, taskIDs, fixtureIDs map[string]bool) string {
	switch f.Type {
	case "enum":
		for _, v := range f.Values {
			if kv.Value == v {
				return ""
			}
		}
		return fmt.Sprintf("metadata field %q value %q is not in the declared enum vocabulary (%s)",
			kv.Key, kv.Value, strings.Join(f.Values, "|"))
	case "ref":
		id := kv.Value
		switch {
		case id == selfID:
			return fmt.Sprintf("metadata field %q references task id %q — a row cannot approve itself", kv.Key, id)
		case fixtureIDs[id]:
			return fmt.Sprintf("metadata field %q references task id %q — a fixture id is not an approval record", kv.Key, id)
		case !taskIDs[id]:
			return fmt.Sprintf("metadata field %q references task id %q which does not exist on the board — create the task first", kv.Key, id)
		}
	}
	return ""
}

// metaState merges a row's current detail.metadata with new pairs (updates
// in place, appends new) — the FINAL metadata state an update would write.
func metaState(target *Row, meta []MetaKV) []MetaKV {
	final, _ := RowMetadata(target)
	for _, kv := range meta {
		replaced := false
		for i := range final {
			if final[i].Key == kv.Key {
				final[i].Value = kv.Value
				replaced = true
				break
			}
		}
		if !replaced {
			final = append(final, kv)
		}
	}
	return final
}

// RowMetadata reads a row's detail.metadata as ordered key/value pairs
// (string values only; list values are comma-joined). Used by validate and
// update's re-validation.
func RowMetadata(row *Row) ([]MetaKV, bool) {
	raw := row.Get("detail")
	if raw == nil {
		return nil, false
	}
	trimmed := bytes.TrimSpace(raw)
	if jsonKindName(trimmed) != "object" {
		return nil, false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return nil, false
	}
	cur, ok := obj["metadata"]
	if !ok || jsonKindName(cur) != "object" {
		return nil, false
	}
	mrow, err := ParseRow(cur)
	if err != nil {
		return nil, false
	}
	var out []MetaKV
	for _, k := range mrow.Keys {
		switch jsonKindName(mrow.Get(k)) {
		case "array":
			out = append(out, MetaKV{Key: k, Value: strings.Join(rowStringSlice(mrow, k), ",")})
		case "string":
			out = append(out, MetaKV{Key: k, Value: mrow.String(k)})
		default:
			out = append(out, MetaKV{Key: k, Value: string(mrow.Get(k))})
		}
	}
	return out, true
}
