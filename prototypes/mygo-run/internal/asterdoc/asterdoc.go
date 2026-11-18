// Package asterdoc parses AsterML: ordinary Markdown plus fenced `asterlab`
// modules. The fenced YAML is the authored source; modules are the parsed,
// stable-ID view of the same text. Nothing here writes experimental facts.
package asterdoc

import (
	"fmt"
	"sort"
	"strconv"
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

// Plate is the container module payload; topology + format/rows/columns select its geometry.
type Plate struct {
	Kind       string              `yaml:"kind" json:"kind"`
	ID         string              `yaml:"id" json:"id"`
	Schema     int                 `yaml:"schema" json:"schema"`
	Title      string              `yaml:"title,omitempty" json:"title,omitempty"`
	Format     int                 `yaml:"format,omitempty" json:"format,omitempty"`
	Topology   string              `yaml:"topology,omitempty" json:"topology,omitempty"`
	Rows       int                 `yaml:"rows,omitempty" json:"rows,omitempty"`
	Columns    int                 `yaml:"columns,omitempty" json:"columns,omitempty"`
	Assay      string              `yaml:"assay,omitempty" json:"assay,omitempty"`
	Timepoint  string              `yaml:"timepoint,omitempty" json:"timepoint,omitempty"`
	Groups     map[string][]string `yaml:"groups" json:"groups"`
	Samples    map[string]string   `yaml:"samples,omitempty" json:"samples,omitempty"`
	Replicates map[string]int      `yaml:"replicates,omitempty" json:"replicates,omitempty"`
	Roles      map[string]string   `yaml:"roles,omitempty" json:"roles,omitempty"`
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
			if module.Kind == "step" {
				if blocks, ok := module.Data["blocks"].([]any); ok {
					for _, rawBlock := range blocks {
						bm, _ := rawBlock.(map[string]any)
						bid := textValue(bm, "id")
						if bid == "" {
							return nil, fmt.Errorf("inline step block needs a stable id")
						}
						if seen[bid] {
							return nil, fmt.Errorf("asterlab module id %q is duplicated", bid)
						}
						seen[bid] = true
					}
				}
			}
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
func flattenModules(modules []Module) ([]Module, map[string]bool) {
	flat := make([]Module, 0, len(modules))
	topIDs := make(map[string]bool, len(modules))
	for _, m := range modules {
		topIDs[m.ID] = true
		flat = append(flat, m)
		if m.Kind == "step" {
			if blocks, ok := m.Data["blocks"].([]any); ok {
				for _, raw := range blocks {
					bm, _ := raw.(map[string]any)
					if bm == nil {
						continue
					}
					flat = append(flat, Module{
						Kind:   textValue(bm, "kind"),
						ID:     textValue(bm, "id"),
						Schema: int(numOrZero(bm["schema"])),
						Data:   bm,
					})
				}
			}
		}
	}
	return flat, topIDs
}

func numOrZero(v any) float64 {
	n, _ := numericValue(v)
	return n
}

func validateDocumentRefs(modules []Module) error {
	flat, topIDs := flattenModules(modules)
	if err := validateTypedReferences(flat); err != nil {
		return err
	}
	if err := validateStepUses(modules, flat, topIDs); err != nil {
		return err
	}
	groups := map[string]bool{}
	assays := map[string]bool{}
	steps := map[string]bool{}
	for _, module := range flat {
		switch module.Kind {
		case "group":
			groups[module.ID] = true
		case "group_table":
			rows, _ := module.Data["rows"].([]any)
			for _, raw := range rows {
				rm, _ := raw.(map[string]any)
				if gid := textValue(rm, "id"); gid != "" {
					groups[gid] = true
				}
			}
		case "assay":
			assays[module.ID] = true
		case "step":
			steps[module.ID] = true
		}
	}
	for _, module := range flat {
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
		if module.Kind == "group_table" {
			if idsByKind["group"] == nil {
				idsByKind["group"] = map[string]bool{}
			}
			rows, _ := module.Data["rows"].([]any)
			for _, raw := range rows {
				rm, _ := raw.(map[string]any)
				if gid := textValue(rm, "id"); gid != "" {
					idsByKind["group"][gid] = true
				}
			}
		}
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

func validateStepUses(modules, flat []Module, topIDs map[string]bool) error {
	byID := map[string]Module{}
	all := map[string]bool{}
	for _, module := range flat {
		byID[module.ID] = module
		all[module.ID] = true
	}
	for _, module := range modules {
		if module.Kind != "step" {
			continue
		}
		for _, ref := range stringList(module.Data, "block_refs") {
			ref = strings.TrimSpace(ref)
			if ref != "" && !topIDs[ref] && !all[ref] {
				return fmt.Errorf("step %q block_refs references missing module %q", module.ID, ref)
			}
		}
		uses, _ := module.Data["uses"].([]any)
		seenUses := map[string]bool{}
		for _, raw := range uses {
			item, _ := raw.(map[string]any)
			ref, _ := item["ref"].(string)
			role, _ := item["role"].(string)
			ref, role = strings.TrimSpace(ref), strings.TrimSpace(role)
			if ref == "" {
				continue
			}
			useKey := ref + "\x00" + role
			if seenUses[useKey] {
				return fmt.Errorf("step %q repeats use %q with role %q", module.ID, ref, role)
			}
			seenUses[useKey] = true
			target, ok := byID[ref]
			if !ok {
				return fmt.Errorf("step %q uses missing module %q", module.ID, ref)
			}
			spec, ok := specByKey[specKey(target.Kind, target.Schema)]
			if !ok {
				return fmt.Errorf("step %q uses unregistered module %q", module.ID, ref)
			}
			if !roleAllowedForModule(role, spec.Descriptor) {
				return fmt.Errorf("step %q use %q has role %q incompatible with %s", module.ID, ref, role, target.Kind)
			}
		}
	}
	return nil
}

func roleAllowedForModule(role string, module Descriptor) bool {
	required, builtin := roleCapabilities[role]
	if !builtin {
		// Namespaced extension roles are explicit capabilities on the target
		// module, e.g. role "acme:imaging" requires capability "acme:imaging".
		if !strings.Contains(role, ":") {
			return false
		}
		required = []string{role}
	}
	if len(required) == 0 {
		return true
	}
	for _, capability := range required {
		for _, declared := range module.Capabilities {
			if declared == capability {
				return true
			}
		}
	}
	return false
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
	if header.Kind == "plate" {
		var plate Plate
		if err := yaml.Unmarshal([]byte(body), &plate); err != nil {
			return Module{}, fmt.Errorf("plate %q: %w", header.ID, err)
		}
		plate.Groups = normalizeGroups(plate.Groups)
		plate.Samples = normalizeSamples(plate.Samples)
		plate.Replicates = normalizeReplicates(plate.Replicates)
		plate.Roles = normalizeRoles(plate.Roles)
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

func normalizeSamples(samples map[string]string) map[string]string {
	out := make(map[string]string, len(samples))
	for well, sample := range samples {
		out[strings.ToUpper(strings.TrimSpace(well))] = strings.TrimSpace(sample)
	}
	return out
}

func normalizeRoles(roles map[string]string) map[string]string {
	out := make(map[string]string, len(roles))
	for well, role := range roles {
		out[strings.ToUpper(strings.TrimSpace(well))] = strings.TrimSpace(role)
	}
	return out
}

func normalizeReplicates(replicates map[string]int) map[string]int {
	out := make(map[string]int, len(replicates))
	for well, replicate := range replicates {
		out[strings.ToUpper(strings.TrimSpace(well))] = replicate
	}
	return out
}

func validatePlate(plate Plate) error {
	topo := strings.TrimSpace(plate.Topology)
	if topo == "" {
		topo = "plate"
	}
	geometry, ok := geometryForPlate(plate.Format, topo, plate.Rows, plate.Columns)
	if !ok {
		return fmt.Errorf("format %d is not a registered plate geometry", plate.Format)
	}
	if plate.Format == 0 {
		plate.Format = geometry.Format
	}
	if plate.Groups == nil {
		return fmt.Errorf("plate needs groups")
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
			if !ValidPlateWell(well, topo, geometry.Rows, geometry.Columns) {
				return fmt.Errorf("well %q is outside %dx%d", well, geometry.Rows, geometry.Columns)
			}
			if owner, ok := seen[well]; ok {
				return fmt.Errorf("well %s is assigned to both %q and %q", well, owner, group)
			}
			seen[well] = group
		}
	}
	sampleByWell := map[string]string{}
	for well, sampleID := range plate.Samples {
		well = strings.ToUpper(strings.TrimSpace(well))
		if !ValidPlateWell(well, topo, geometry.Rows, geometry.Columns) {
			return fmt.Errorf("sample well %q is outside %dx%d", well, geometry.Rows, geometry.Columns)
		}
		if _, ok := seen[well]; !ok {
			return fmt.Errorf("sample well %s has no group assignment", well)
		}
		sampleID = strings.TrimSpace(sampleID)
		if sampleID == "" {
			return fmt.Errorf("sample ID for well %s cannot be empty", well)
		}
		if _, exists := sampleByWell[well]; exists {
			return fmt.Errorf("sample assignment for well %s is duplicated", well)
		}
		sampleByWell[well] = sampleID
	}
	seenReplicates := map[string]string{}
	for well, replicate := range plate.Replicates {
		well = strings.ToUpper(strings.TrimSpace(well))
		if !ValidPlateWell(well, topo, geometry.Rows, geometry.Columns) {
			return fmt.Errorf("replicate well %q is outside %dx%d", well, geometry.Rows, geometry.Columns)
		}
		sampleID := sampleByWell[well]
		if sampleID == "" {
			return fmt.Errorf("replicate well %s has no sample ID", well)
		}
		if replicate < 1 {
			return fmt.Errorf("replicate for well %s must be positive", well)
		}
		key := fmt.Sprintf("%s\x00%d", sampleID, replicate)
		if previous, exists := seenReplicates[key]; exists {
			return fmt.Errorf("sample %s replicate %d is assigned to both %s and %s", sampleID, replicate, previous, well)
		}
		seenReplicates[key] = well
	}
	for well, role := range plate.Roles {
		w := strings.ToUpper(strings.TrimSpace(well))
		if !ValidPlateWell(w, topo, geometry.Rows, geometry.Columns) {
			return fmt.Errorf("role well %q is outside %dx%d", w, geometry.Rows, geometry.Columns)
		}
		switch strings.TrimSpace(role) {
		case "sample", "blank", "vehicle", "positive_control", "negative_control", "standard", "ntc":
		default:
			return fmt.Errorf("invalid well role %q for %s", role, w)
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
		return "", fmt.Errorf("module %q is not a plate", id)
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
	return ValidPlateWell(well, "plate", geometry.Rows, geometry.Columns)
}

// ValidPlateWell validates well coordinates (A1..P24) or 1..N linear slot indices for non-plate topologies.
func ValidPlateWell(well, topology string, rows, cols int) bool {
	w := strings.ToUpper(strings.TrimSpace(well))
	if w == "" {
		return false
	}
	if len(w) >= 2 && w[0] >= 'A' && w[0] <= 'Z' {
		row := int(w[0] - 'A')
		if row >= 0 && row < rows {
			if col, err := strconv.Atoi(w[1:]); err == nil {
				return col >= 1 && col <= cols
			}
		}
	}
	if topology != "" && topology != "plate" {
		if idx, err := strconv.Atoi(w); err == nil {
			return idx >= 1 && idx <= rows*cols
		}
	}
	return false
}

// GroupInfo describes a canonical experimental group declared via group@1 or group_table@1.
type GroupInfo struct {
	ID               string
	Name             string
	Role             string
	Condition        string
	IndependentN     int
	TechnicalRepeats int
}

// GroupList collects all unique experimental groups declared in top-level group@1 and group_table@1 modules.
func (d *Document) GroupList() []GroupInfo {
	if d == nil {
		return nil
	}
	flat, _ := flattenModules(d.Modules)
	seen := map[string]bool{}
	var out []GroupInfo
	for _, m := range flat {
		switch m.Kind {
		case "group":
			if m.ID == "" || seen[m.ID] {
				continue
			}
			seen[m.ID] = true
			indN, _ := numericValue(m.Data["independent_n"])
			techR, _ := numericValue(m.Data["technical_repeats"])
			out = append(out, GroupInfo{
				ID: m.ID, Name: textValue(m.Data, "name"), Role: textValue(m.Data, "role"),
				Condition: textValue(m.Data, "condition"), IndependentN: int(indN), TechnicalRepeats: int(techR),
			})
		case "group_table":
			rows, _ := m.Data["rows"].([]any)
			for _, raw := range rows {
				rm, _ := raw.(map[string]any)
				gid := textValue(rm, "id")
				if gid == "" || seen[gid] {
					continue
				}
				seen[gid] = true
				indN, _ := numericValue(rm["independent_n"])
				techR, _ := numericValue(rm["technical_repeats"])
				out = append(out, GroupInfo{
					ID: gid, Name: textValue(rm, "name"), Role: textValue(rm, "role"),
					Condition: textValue(rm, "condition"), IndependentN: int(indN), TechnicalRepeats: int(techR),
				})
			}
		case "plate":
			if p, err := plateFromData(m.Data); err == nil {
				for _, g := range p.GroupNames() {
					if g == "" || seen[g] {
						continue
					}
					seen[g] = true
					out = append(out, GroupInfo{ID: g, Name: g})
				}
			}
		}
	}
	return out
}

// MoveStepBlockToIndex reorders one inline block inside stepID's blocks[] list to toIndex
// directly in AsterML Markdown source and validates the result via Parse.
func MoveStepBlockToIndex(source, stepID, blockID string, toIndex int) (string, error) {
	doc, err := Parse(source)
	if err != nil {
		return "", err
	}
	for _, m := range doc.Modules {
		if m.Kind != "step" || m.ID != stepID {
			continue
		}
		var data map[string]any
		if err := yaml.Unmarshal([]byte(m.Body), &data); err != nil {
			return "", err
		}
		seq, _ := data["blocks"].([]any)
		from := -1
		for i, raw := range seq {
			bm, _ := raw.(map[string]any)
			if textValue(bm, "id") == blockID {
				from = i
				break
			}
		}
		if from < 0 {
			return "", fmt.Errorf("step %q block %q does not exist", stepID, blockID)
		}
		if len(seq) <= 1 {
			return doc.Source, nil
		}
		if toIndex < 0 {
			toIndex = 0
		}
		if toIndex >= len(seq) {
			toIndex = len(seq) - 1
		}
		if from == toIndex {
			return doc.Source, nil
		}
		item := seq[from]
		without := append([]any{}, seq[:from]...)
		without = append(without, seq[from+1:]...)
		reordered := append([]any{}, without[:toIndex]...)
		reordered = append(reordered, item)
		reordered = append(reordered, without[toIndex:]...)
		data["blocks"] = reordered
		raw, err := yaml.Marshal(data)
		if err != nil {
			return "", err
		}
		next := doc.Source[:m.Start] + string(raw) + doc.Source[m.End:]
		if _, err := Parse(next); err != nil {
			return "", err
		}
		return next, nil
	}
	return "", fmt.Errorf("step %q does not exist", stepID)
}

// BudgetSummary summarizes document structure and timer duration budgets for the header bar.
type BudgetSummary struct {
	Steps           int
	ExecutableSteps int
	Groups          int
	Wells           int
	HandsOnSec      int
	TotalSpanSec    int
	HasOpenPhase    bool
}

// ComputeBudgetSummary calculates structural counts and timer budgets directly from AsterML v2 modules.
func ComputeBudgetSummary(doc *Document) BudgetSummary {
	if doc == nil {
		return BudgetSummary{}
	}
	var sum BudgetSummary
	sum.Groups = len(doc.GroupList())
	flat, _ := flattenModules(doc.Modules)
	for _, m := range flat {
		switch m.Kind {
		case "step":
			sum.Steps++
			exec := textValue(m.Data, "execution")
			if exec == "" || exec == "executable" {
				sum.ExecutableSteps++
			}
		case "plate":
			if p, err := plateFromData(m.Data); err == nil {
				sum.Wells += len(p.WellGroups())
			}
		case "timer":
			sec, span, open := timerBudgetSec(m.Data)
			sum.HandsOnSec += sec
			sum.TotalSpanSec += span
			if open {
				sum.HasOpenPhase = true
			}
		}
	}
	return sum
}

func quantitySeconds(val any) int {
	v, err := quantityIn(val, "s")
	if err != nil || v <= 0 {
		return 0
	}
	return int(v + 0.5)
}

func timerBudgetSec(data map[string]any) (handsOn, span int, open bool) {
	switch textValue(data, "mode") {
	case "countdown":
		s := quantitySeconds(data["duration"])
		return s, s, false
	case "stopwatch":
		s := quantitySeconds(data["duration"])
		return s, s, s == 0
	case "cycle":
		rounds := 1
		if r, ok := numericValue(data["rounds"]); ok && r >= 1 {
			rounds = int(r)
		}
		perRound := 0
		phases, _ := data["phases"].([]any)
		for _, raw := range phases {
			ph, _ := raw.(map[string]any)
			if textValue(ph, "mode") == "open" {
				open = true
			}
			perRound += quantitySeconds(ph["duration"])
		}
		tot := perRound * rounds
		return tot, tot, open
	case "window":
		off := quantitySeconds(data["offset"])
		return 0, off, false
	}
	return 0, 0, false
}
