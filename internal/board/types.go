package board

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// TicketType describes one board-local typed ticket declaration.
type TicketType struct {
	Slug     string          `json:"slug"`
	Display  string          `json:"display"`
	Rev      int             `json:"rev,omitempty"`
	Retired  bool            `json:"retired,omitempty"`
	Template *TicketTemplate `json:"template,omitempty"`
	Metadata []MetadataField `json:"metadata,omitempty"`
}

type TicketTemplate struct {
	TitlePrefix     string `json:"title_prefix,omitempty"`
	ACSkeleton      string `json:"ac_skeleton,omitempty"`
	DefaultPriority string `json:"default_priority,omitempty"`
}

var (
	metadataNameRE   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	ticketTypeSlugRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

type MetadataField struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required,omitempty"`
	Description string   `json:"description,omitempty"`
	Values      []string `json:"values,omitempty"`
	Ref         string   `json:"ref,omitempty"`
}

// TypeRegistry is the resolved, last-line-wins view of types.jsonl.
type TypeRegistry struct {
	Types    map[string]TicketType
	Warnings []string
	Present  bool
}

func BuiltinTicketTypes() []TicketType {
	return []TicketType{{Slug: "task", Display: "Task", Rev: 1}, {Slug: "bug", Display: "Bug", Rev: 1}, {Slug: "feat", Display: "Feature", Rev: 1}, {Slug: "chore", Display: "Chore", Rev: 1}}
}

func (r TypeRegistry) Sorted(active bool) []TicketType {
	out := make([]TicketType, 0, len(r.Types))
	for _, typ := range r.Types {
		if active && typ.Retired {
			continue
		}
		if !active && !typ.Retired {
			continue
		}
		out = append(out, typ)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

func (r TypeRegistry) Names() string {
	var names []string
	for _, typ := range r.Sorted(true) {
		names = append(names, typ.Slug)
	}
	return strings.Join(names, ", ")
}

// LoadTicketTypes reads the append-only board-local registry. An absent or
// empty file is deliberately indistinguishable from the built-in registry.
func LoadTicketTypes(dir string) (TypeRegistry, error) {
	path := filepath.Join(dir, "types.jsonl")
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return builtinRegistry(), nil
	}
	if err != nil {
		return TypeRegistry{}, err
	}
	defer func() { _ = f.Close() }()
	reg := TypeRegistry{Types: map[string]TicketType{}, Present: true}
	seenRev := map[string]int{}
	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return TypeRegistry{}, fmt.Errorf("types.jsonl line %d: %v — fix the file and re-run (nothing written)", line, err)
		}
		var typ TicketType
		if err := json.Unmarshal(raw, &typ); err != nil {
			return TypeRegistry{}, fmt.Errorf("types.jsonl line %d: %v — fix the file and re-run (nothing written)", line, err)
		}
		if err := validateTicketType(typ); err != nil {
			return TypeRegistry{}, fmt.Errorf("types.jsonl line %d: %v — fix the file and re-run (nothing written)", line, err)
		}
		if _, ok := obj["display"]; !ok || strings.TrimSpace(typ.Display) == "" {
			return TypeRegistry{}, fmt.Errorf("types.jsonl line %d: display is required — fix the file and re-run (nothing written)", line)
		}
		if typ.Rev == 0 {
			typ.Rev = 1
		}
		if previous, ok := seenRev[typ.Slug]; ok && previous == typ.Rev {
			reg.Warnings = append(reg.Warnings, fmt.Sprintf("types.jsonl line %d: slug %q repeats rev %d; last line wins", line, typ.Slug, typ.Rev))
		}
		if previous, ok := seenRev[typ.Slug]; ok && typ.Rev < previous {
			reg.Warnings = append(reg.Warnings, fmt.Sprintf("types.jsonl line %d: slug %q rev %d is lower than prior rev %d; last line wins", line, typ.Slug, typ.Rev, previous))
		}
		seenRev[typ.Slug] = typ.Rev
		for key := range obj {
			if !knownTicketTypeKey(key) {
				reg.Warnings = append(reg.Warnings, fmt.Sprintf("types.jsonl line %d: unknown key %q in type %q", line, key, typ.Slug))
			}
		}
		reg.Types[typ.Slug] = typ
	}
	if err := scanner.Err(); err != nil {
		return TypeRegistry{}, err
	}
	if len(reg.Types) == 0 {
		return builtinRegistry(), nil
	}
	return reg, nil
}

func builtinRegistry() TypeRegistry {
	r := TypeRegistry{Types: map[string]TicketType{}}
	for _, typ := range BuiltinTicketTypes() {
		r.Types[typ.Slug] = typ
	}
	return r
}

func knownTicketTypeKey(key string) bool {
	switch key {
	case "slug", "display", "rev", "retired", "template", "metadata":
		return true
	default:
		return false
	}
}

func validateTicketType(typ TicketType) error {
	if len(typ.Slug) == 0 || len(typ.Slug) > 64 || !ticketTypeSlugRE.MatchString(typ.Slug) {
		return fmt.Errorf("invalid ticket type slug %q: use lowercase letters, digits and single hyphens (max 64)", typ.Slug)
	}
	if typ.Template != nil {
		priority := NormalizePriority(typ.Template.DefaultPriority)
		if typ.Template.DefaultPriority != "" && (priority < "P0" || priority > "P5") {
			return fmt.Errorf("type %q template default_priority %q is not P0-P5", typ.Slug, typ.Template.DefaultPriority)
		}
	}
	seen := map[string]bool{}
	for _, field := range typ.Metadata {
		if !metadataNameRE.MatchString(field.Name) {
			return fmt.Errorf("type %q metadata name %q is not lowercase snake_case", typ.Slug, field.Name)
		}
		if seen[field.Name] {
			return fmt.Errorf("type %q declares metadata field %q twice", typ.Slug, field.Name)
		}
		seen[field.Name] = true
		switch field.Type {
		case "string", "enum", "list", "ref":
		default:
			return fmt.Errorf("type %q metadata field %q has unsupported type %q", typ.Slug, field.Name, field.Type)
		}
		if field.Type == "enum" && len(field.Values) == 0 {
			return fmt.Errorf("type %q enum field %q must declare values", typ.Slug, field.Name)
		}
		if field.Type == "ref" && field.Ref == "" {
			return fmt.Errorf("type %q ref field %q must declare ref", typ.Slug, field.Name)
		}
	}
	return nil
}

func (typ TicketType) Field(name string) (MetadataField, bool) {
	for _, f := range typ.Metadata {
		if f.Name == name {
			return f, true
		}
	}
	return MetadataField{}, false
}

func metadataFieldSummary(f MetadataField) string {
	extra := ""
	if f.Type == "enum" {
		extra = ": " + strings.Join(f.Values, "|")
	}
	req := ""
	if f.Required {
		req = ", required"
	}
	return fmt.Sprintf("%s(%s%s)", f.Name, f.Type, req+extra)
}

func (r TypeRegistry) RenderText() string {
	var sb strings.Builder
	for _, typ := range r.Sorted(true) {
		fmt.Fprintf(&sb, "%s (%s)", typ.Slug, typ.Display)
		if typ.Template != nil {
			fmt.Fprintf(&sb, " template:")
			if typ.Template.TitlePrefix != "" {
				fmt.Fprintf(&sb, " prefix=%q", typ.Template.TitlePrefix)
			}
			if typ.Template.DefaultPriority != "" {
				fmt.Fprintf(&sb, " priority=%s", NormalizePriority(typ.Template.DefaultPriority))
			}
			if typ.Template.ACSkeleton != "" {
				fmt.Fprint(&sb, " skeleton")
			}
		}
		if len(typ.Metadata) > 0 {
			vals := make([]string, 0, len(typ.Metadata))
			for _, f := range typ.Metadata {
				vals = append(vals, metadataFieldSummary(f))
			}
			fmt.Fprintf(&sb, " metadata: %s", strings.Join(vals, ", "))
		}
		sb.WriteByte('\n')
	}
	retired := r.Sorted(false)
	if len(retired) > 0 {
		sb.WriteString("retired:\n")
		for _, typ := range retired {
			fmt.Fprintf(&sb, "  %s (%s)\n", typ.Slug, typ.Display)
		}
	}
	return sb.String()
}

func (r TypeRegistry) DoctorText() string {
	if !r.Present {
		return "types.jsonl: absent — built-in defaults active (task, bug, feat, chore)\nconfig health: OK\n"
	}
	if len(r.Warnings) == 0 {
		return "types.jsonl: present\nconfig health: OK\n"
	}
	var sb strings.Builder
	sb.WriteString("types.jsonl: present\nconfig health: WARN\n")
	for _, warning := range r.Warnings {
		fmt.Fprintf(&sb, "warning: %s\n", warning)
	}
	return sb.String()
}

func (r TypeRegistry) ValidateRow(row *Row, allIDs map[string]bool) []Finding {
	var findings []Finding
	typeName := row.String("type")
	if typeName == "" {
		return findings
	}
	typ, ok := r.Types[typeName]
	if !ok || typ.Retired {
		findings = append(findings, Finding{Level: "warn", Class: "type-conformance", Msg: fmt.Sprintf("BT-098: row %s claims type %q which types.jsonl does not declare", row.String("id"), typeName)})
		return findings
	}
	metadata := map[string]json.RawMessage{}
	if raw := row.Get("detail"); len(raw) > 0 {
		pd := ParseFindingDetail(raw)
		if pd.Object && pd.Extra != nil {
			if m := pd.Extra["metadata"]; m != nil {
				_ = json.Unmarshal(m, &metadata)
			}
		}
	}
	for _, field := range typ.Metadata {
		raw, present := metadata[field.Name]
		if !present || bytes.Equal(bytes.TrimSpace(raw), []byte(`""`)) || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			if field.Required {
				findings = append(findings, Finding{Level: "error", Class: "type-conformance", Msg: fmt.Sprintf("BT-098: row %s (type %q) is missing required metadata field %q (%s)", row.String("id"), typeName, field.Name, metadataFieldSummary(field))})
			}
			continue
		}
		if field.Type == "enum" {
			var value string
			_ = json.Unmarshal(raw, &value)
			allowed := false
			for _, v := range field.Values {
				if value == v {
					allowed = true
				}
			}
			if !allowed {
				findings = append(findings, Finding{Level: "error", Class: "type-conformance", Msg: fmt.Sprintf("BT-098: row %s (type %q) metadata field %q has value %q outside %s", row.String("id"), typeName, field.Name, value, strings.Join(field.Values, "|"))})
			}
		}
	}
	for key := range metadata {
		if _, declared := typ.Field(key); !declared {
			findings = append(findings, Finding{Level: "warn", Class: "type-conformance", Msg: fmt.Sprintf("BT-098: row %s (type %q) has undeclared metadata key %q", row.String("id"), typeName, key)})
		}
	}
	return findings
}

func (r TypeRegistry) ValidateMetadata(typeName string, metadata map[string]string, allIDs map[string]bool) ([]string, []string) {
	typ, ok := r.Types[typeName]
	if !ok {
		return nil, []string{fmt.Sprintf("unknown ticket type %q — configured types: %s", typeName, r.Names())}
	}
	if typ.Retired {
		return nil, []string{fmt.Sprintf("ticket type %q is retired in types.jsonl — pick an active type", typeName)}
	}
	var missing, invalid []string
	for _, field := range typ.Metadata {
		value, present := metadata[field.Name]
		if (!present || strings.TrimSpace(value) == "") && field.Required {
			missing = append(missing, fmt.Sprintf("%s (%s) — required: %s", field.Name, metadataFieldSummary(field), field.Description))
			continue
		}
		if !present || strings.TrimSpace(value) == "" {
			continue
		}
		switch field.Type {
		case "enum":
			allowed := false
			for _, candidate := range field.Values {
				if value == candidate {
					allowed = true
				}
			}
			if !allowed {
				invalid = append(invalid, fmt.Sprintf("%s=%q (allowed: %s)", field.Name, value, strings.Join(field.Values, "|")))
			}
		case "ref":
			if field.Ref == "task_id" && !allIDs[value] {
				invalid = append(invalid, fmt.Sprintf("%s=%q (task id does not exist)", field.Name, value))
			}
		}
	}
	return missing, invalid
}

func encodeMetadata(values map[string]string, typ TicketType, style Style) (json.RawMessage, error) {
	obj := map[string]any{}
	for _, field := range typ.Metadata {
		value, ok := values[field.Name]
		if !ok {
			continue
		}
		if field.Type == "list" {
			obj[field.Name] = splitMetadataList(value)
		} else {
			obj[field.Name] = value
		}
	}
	for key, value := range values {
		if _, ok := obj[key]; !ok {
			obj[key] = value
		}
	}
	return encodeValue(obj, style)
}

func splitMetadataList(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
