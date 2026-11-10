// Package asterdoc parses AsterML: ordinary Markdown plus fenced `asterlab`
// modules. The fenced YAML is the authored source; modules are the parsed,
// stable-ID view of the same text. Nothing here writes experimental facts.
package asterdoc

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Module is one parsed `asterlab` fence. Start and End bound the YAML body in
// Source, excluding the fence lines, so a change can replace only that body.
type Module struct {
	Kind   string
	ID     string
	Schema int
	Body   string
	Start  int
	End    int
	Plate  *Plate
	Data   map[string]any
}

// Plate is the plate96 module payload this prototype understands.
type Plate struct {
	Kind      string              `yaml:"kind" json:"kind"`
	ID        string              `yaml:"id" json:"id"`
	Schema    int                 `yaml:"schema" json:"schema"`
	Format    int                 `yaml:"format,omitempty" json:"format,omitempty"`
	Assay     string              `yaml:"assay,omitempty" json:"assay,omitempty"`
	Timepoint string              `yaml:"timepoint,omitempty" json:"timepoint,omitempty"`
	Groups    map[string][]string `yaml:"groups" json:"groups"`
}

// Document is a parsed AsterML source. Source is always the canonical text;
// Modules are derived from it.
type Document struct {
	Source  string
	Modules []Module
}

var rows = "ABCDEFGH"

// Parse reads every fenced asterlab block, validates stable IDs and plate
// assignments, and returns the document. Plain Markdown is left untouched.
func Parse(source string) (*Document, error) {
	return ParseWith(source, BuiltinRegistry())
}

// ParseWith reads fenced asterlab blocks and validates them against registry.
func ParseWith(source string, registry *Registry) (*Document, error) {
	if registry == nil {
		registry = BuiltinRegistry()
	}
	doc := &Document{Source: source}
	lines := strings.SplitAfter(source, "\n")
	offset := 0
	inFence := false
	fenceLang := ""
	bodyStart := 0
	var body strings.Builder
	seen := map[string]bool{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inFence {
			if strings.HasPrefix(trimmed, "```") {
				lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				if lang == "asterlab" {
					inFence = true
					fenceLang = lang
					bodyStart = offset + len(line)
					body.Reset()
				}
			}
			offset += len(line)
			continue
		}
		if trimmed == "```" {
			bodyEnd := offset
			module, err := parseModuleWith(body.String(), bodyStart, bodyEnd, registry)
			if err != nil {
				return nil, err
			}
			if seen[module.ID] {
				return nil, fmt.Errorf("asterlab module id %q is duplicated", module.ID)
			}
			seen[module.ID] = true
			doc.Modules = append(doc.Modules, module)
			inFence = false
			fenceLang = ""
			offset += len(line)
			continue
		}
		body.WriteString(line)
		offset += len(line)
	}
	if inFence {
		return nil, fmt.Errorf("asterlab block opened with %q is not closed", fenceLang)
	}
	if err := validateDocumentRefs(doc.Modules); err != nil {
		return nil, err
	}
	return doc, nil
}

// validateDocumentRefs checks stable-ID references across modules. It only
// enforces the kinds the compiler understands; unknown kinds are already
// rejected by the registry.
func validateDocumentRefs(modules []Module) error {
	if err := validateTypedReferences(modules); err != nil {
		return err
	}
	if err := validateStepUses(modules); err != nil {
		return err
	}
	groups := map[string]bool{}
	assays := map[string]bool{}
	steps := map[string]bool{}
	for _, module := range modules {
		switch module.Kind {
		case "group":
			groups[module.ID] = true
		case "assay":
			assays[module.ID] = true
		case "step":
			steps[module.ID] = true
		}
	}
	for _, module := range modules {
		switch module.Kind {
		case "assay":
			for _, ref := range stringList(module.Data, "group_refs") {
				if !groups[ref] {
					return fmt.Errorf("assay %q references missing group %q", module.ID, ref)
				}
			}
		case "step":
			if assay := textValue(module.Data, "assay"); assay != "" && !assays[assay] {
				return fmt.Errorf("step %q references missing assay %q", module.ID, assay)
			}
		case "workflow":
			declared := map[string]bool{}
			for _, step := range stringList(module.Data, "steps") {
				if !steps[step] {
					return fmt.Errorf("workflow %q references missing step %q", module.ID, step)
				}
				if declared[step] {
					return fmt.Errorf("workflow %q repeats step %q", module.ID, step)
				}
				declared[step] = true
			}
			edges, _ := module.Data["edges"].([]any)
			for _, raw := range edges {
				edge, _ := raw.(map[string]any)
				from, _ := edge["from"].(string)
				to, _ := edge["to"].(string)
				if !declared[from] || !declared[to] {
					return fmt.Errorf("workflow %q edge %q→%q does not join declared steps", module.ID, from, to)
				}
			}
			for _, step := range stringList(module.Data, "business_order") {
				if !declared[step] {
					return fmt.Errorf("workflow %q business_order references %q outside steps", module.ID, step)
				}
			}
		case "plate":
			if assay := textValue(module.Data, "assay"); assay != "" && !assays[assay] {
				return fmt.Errorf("plate %q references missing assay %q", module.ID, assay)
			}
			groupsRaw, _ := module.Data["groups"].(map[string]any)
			for group := range groupsRaw {
				if !groups[group] {
					return fmt.Errorf("plate %q references missing group %q", module.ID, group)
				}
			}
		}
	}
	return nil
}

// validateTypedReferences enforces the target kind declared in the module
// manifest. It does not guess from field names or ID prefixes.
func validateTypedReferences(modules []Module) error {
	idsByKind := map[string]map[string]bool{}
	allIDs := map[string]bool{}
	for _, module := range modules {
		if idsByKind[module.Kind] == nil {
			idsByKind[module.Kind] = map[string]bool{}
		}
		idsByKind[module.Kind][module.ID] = true
		allIDs[module.ID] = true
	}
	for _, module := range modules {
		spec, ok := specByKey[specKey(module.Kind, module.Schema)]
		if !ok {
			continue
		}
		for _, input := range spec.Inputs {
			if input.Type != "reference" && !(input.Type == "list" && input.ItemType == "reference") {
				continue
			}
			values := []string{}
			if input.Type == "reference" {
				if value, _ := module.Data[input.Name].(string); strings.TrimSpace(value) != "" {
					values = append(values, value)
				}
			} else if items, ok := module.Data[input.Name].([]any); ok {
				for _, item := range items {
					if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
						values = append(values, value)
					}
				}
			}
			for _, value := range values {
				target := input.Target
				if target == "module" {
					if !allIDs[value] {
						return fmt.Errorf("%s %q field %q references missing module %q", module.Kind, module.ID, input.Name, value)
					}
					continue
				}
				if !idsByKind[target][value] {
					return fmt.Errorf("%s %q field %q references missing %s %q", module.Kind, module.ID, input.Name, target, value)
				}
			}
		}
	}
	return nil
}

func validateStepUses(modules []Module) error {
	all := map[string]bool{}
	for _, module := range modules {
		all[module.ID] = true
	}
	for _, module := range modules {
		if module.Kind != "step" {
			continue
		}
		uses, _ := module.Data["uses"].([]any)
		for _, raw := range uses {
			item, _ := raw.(map[string]any)
			ref, _ := item["ref"].(string)
			if strings.TrimSpace(ref) != "" && !all[ref] {
				return fmt.Errorf("step %q uses missing module %q", module.ID, ref)
			}
		}
	}
	return nil
}

func parseModule(body string, start, end int) (Module, error) {
	return parseModuleWith(body, start, end, BuiltinRegistry())
}

func parseModuleWith(body string, start, end int, registry *Registry) (Module, error) {
	var header struct {
		Kind   string `yaml:"kind"`
		ID     string `yaml:"id"`
		Schema int    `yaml:"schema"`
	}
	if err := yaml.Unmarshal([]byte(body), &header); err != nil {
		return Module{}, fmt.Errorf("asterlab block: %w", err)
	}
	header.Kind = strings.TrimSpace(header.Kind)
	header.ID = strings.TrimSpace(header.ID)
	if header.ID == "" {
		return Module{}, fmt.Errorf("asterlab module needs a stable id")
	}
	if header.Schema <= 0 {
		return Module{}, fmt.Errorf("asterlab module %q needs a positive schema", header.ID)
	}
	var data map[string]any
	if err := yaml.Unmarshal([]byte(body), &data); err != nil {
		return Module{}, fmt.Errorf("asterlab module %q: %w", header.ID, err)
	}
	spec, ok := registry.Lookup(header.Kind, header.Schema)
	if !ok {
		return Module{}, fmt.Errorf("asterlab module %q uses unsupported %s@%d; supported: %s",
			header.ID, header.Kind, header.Schema, strings.Join(registry.supported(), ", "))
	}
	if err := spec.Validate(data); err != nil {
		return Module{}, fmt.Errorf("%s %q: %w", header.Kind, header.ID, err)
	}
	module := Module{Kind: header.Kind, ID: header.ID, Schema: header.Schema,
		Body: body, Start: start, End: end, Data: data}
	if header.Kind == "plate96" || header.Kind == "plate" {
		var plate Plate
		if err := yaml.Unmarshal([]byte(body), &plate); err != nil {
			return Module{}, fmt.Errorf("plate %q: %w", header.ID, err)
		}
		plate.Groups = normalizeGroups(plate.Groups)
		if plate.Format == 0 {
			plate.Format = 96
		}
		module.Plate = &plate
	}
	return module, nil
}

func normalizeGroups(groups map[string][]string) map[string][]string {
	out := make(map[string][]string, len(groups))
	for group, wells := range groups {
		normalized := make([]string, 0, len(wells))
		for _, well := range wells {
			normalized = append(normalized, strings.ToUpper(strings.TrimSpace(well)))
		}
		out[strings.TrimSpace(group)] = normalized
	}
	return out
}

func validatePlate(plate Plate) error {
	geometry, ok := geometryFor(plate.Format)
	if !ok {
		return fmt.Errorf("format %d is not a registered plate geometry", plate.Format)
	}
	if plate.Format == 0 {
		plate.Format = geometry.Format
	}
	if len(plate.Groups) == 0 {
		return fmt.Errorf("needs at least one group")
	}
	seen := map[string]string{}
	groups := make([]string, 0, len(plate.Groups))
	for group := range plate.Groups {
		if strings.TrimSpace(group) == "" {
			return fmt.Errorf("group name cannot be empty")
		}
		groups = append(groups, group)
	}
	sort.Strings(groups)
	for _, group := range groups {
		for _, well := range plate.Groups[group] {
			if !validWell(well, geometry) {
				return fmt.Errorf("well %q is outside %dx%d", well, geometry.Rows, geometry.Columns)
			}
			if owner, ok := seen[well]; ok {
				return fmt.Errorf("well %s is assigned to both %q and %q", well, owner, group)
			}
			seen[well] = group
		}
	}
	return nil
}

// PlateByID returns the parsed plate with the stable ID.
func (d *Document) PlateByID(id string) (*Plate, bool) {
	for i := range d.Modules {
		if d.Modules[i].ID == id && d.Modules[i].Plate != nil {
			return d.Modules[i].Plate, true
		}
	}
	return nil, false
}

// UpdatePlate replaces one plate's groups and rewrites only that fence body.
// The returned source is regenerated deterministically; parsing it again must
// yield the same plate.
func (d *Document) UpdatePlate(id string, groups map[string][]string) (string, error) {
	index := -1
	for i := range d.Modules {
		if d.Modules[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return "", fmt.Errorf("plate %q is not in this document", id)
	}
	module := d.Modules[index]
	if module.Plate == nil {
		return "", fmt.Errorf("module %q is not a plate96", id)
	}
	previous := module.End
	plate := *module.Plate
	plate.Groups = normalizeGroups(groups)
	if err := validatePlate(plate); err != nil {
		return "", err
	}
	var data map[string]any
	if err := yaml.Unmarshal([]byte(module.Body), &data); err != nil {
		return "", err
	}
	data["groups"] = plate.Groups
	raw, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	body := string(raw)
	source := d.Source[:module.Start] + body + d.Source[previous:]
	next, err := Parse(source)
	if err != nil {
		return "", err
	}
	d.Source = source
	d.Modules = next.Modules
	return source, nil
}

// UpdateModule merges properties into one module's fence body and rewrites
// only that fence. It re-parses the document, so invalid edits are rejected.
func (d *Document) UpdateModule(id string, properties map[string]any) (string, error) {
	index := -1
	for i := range d.Modules {
		if d.Modules[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return "", fmt.Errorf("module %q is not in this document", id)
	}
	module := d.Modules[index]
	var data map[string]any
	if err := yaml.Unmarshal([]byte(module.Body), &data); err != nil {
		return "", err
	}
	if data == nil {
		data = map[string]any{}
	}
	for key, value := range properties {
		data[key] = value
	}
	raw, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	body := string(raw)
	source := d.Source[:module.Start] + body + d.Source[module.End:]
	next, err := Parse(source)
	if err != nil {
		return "", err
	}
	d.Source = source
	d.Modules = next.Modules
	return source, nil
}

// WellGroups inverts the plate into one group per well.
func (p Plate) WellGroups() map[string]string {
	out := map[string]string{}
	for group, wells := range p.Groups {
		for _, well := range wells {
			out[well] = group
		}
	}
	return out
}

// GroupNames returns the groups in stable order.
func (p Plate) GroupNames() []string {
	names := make([]string, 0, len(p.Groups))
	for name := range p.Groups {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func validWell(well string, geometry Geometry) bool {
	if len(well) < 2 {
		return false
	}
	row := int(well[0] - 'A')
	if row < 0 || row >= geometry.Rows {
		return false
	}
	col := 0
	for _, r := range well[1:] {
		if r < '0' || r > '9' {
			return false
		}
		col = col*10 + int(r-'0')
	}
	return col >= 1 && col <= geometry.Columns
}
