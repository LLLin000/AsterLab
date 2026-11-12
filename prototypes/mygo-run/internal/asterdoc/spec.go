package asterdoc

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:embed spec/modules.json
var moduleSpecJSON []byte

//go:embed spec/foundation-v1.schema.json
var foundationSchemaJSON []byte

// FoundationSchema returns the machine-readable Foundation v1 schema shipped
// with the binary; it is a contract artifact, not the runtime validator.
func FoundationSchema() []byte { return append([]byte(nil), foundationSchemaJSON...) }

type specFile struct {
	SchemaVersion     int                 `json:"schemaVersion"`
	FoundationVersion int                 `json:"foundationVersion"`
	SDKVersion        string              `json:"sdkVersion"`
	RoleCapabilities  map[string][]string `json:"roleCapabilities"`
	Modules           []json.RawMessage   `json:"modules"`
}

type specModule struct {
	Descriptor
}

var (
	specDescriptors   []Descriptor
	specByKey         map[string]specModule
	specErr           error
	specFoundationVer int
	specSDKVersion    string
	roleCapabilities  map[string][]string
)

func init() { specErr = loadSpec() }

func loadSpec() error {
	var file specFile
	if err := json.Unmarshal(moduleSpecJSON, &file); err != nil {
		return fmt.Errorf("asterdoc module spec: %w", err)
	}
	if file.SchemaVersion != 1 || file.FoundationVersion != 1 {
		return fmt.Errorf("asterdoc module spec: unsupported schema/foundation version")
	}
	if strings.TrimSpace(file.SDKVersion) == "" {
		return fmt.Errorf("asterdoc module spec: sdkVersion is required")
	}
	specFoundationVer, specSDKVersion = file.FoundationVersion, file.SDKVersion
	roleCapabilities = file.RoleCapabilities
	if unitBySymbol == nil {
		if err := loadUnits(); err != nil {
			return err
		}
	}
	specByKey = map[string]specModule{}
	for _, raw := range file.Modules {
		var module specModule
		if err := json.Unmarshal(raw, &module); err != nil {
			return fmt.Errorf("asterdoc module spec: %w", err)
		}
		if err := validateSpecModule(module, file.SDKVersion); err != nil {
			return fmt.Errorf("asterdoc module spec: %w", err)
		}
		key := specKey(module.Kind, module.Schema)
		if _, exists := specByKey[key]; exists {
			return fmt.Errorf("asterdoc module spec repeats %s", key)
		}
		specByKey[key] = module
		specDescriptors = append(specDescriptors, module.Descriptor)
	}
	sort.Slice(specDescriptors, func(i, j int) bool {
		if specDescriptors[i].Kind != specDescriptors[j].Kind {
			return specDescriptors[i].Kind < specDescriptors[j].Kind
		}
		return specDescriptors[i].Schema < specDescriptors[j].Schema
	})
	return nil
}

func specKey(kind string, schema int) string { return fmt.Sprintf("%s@%d", kind, schema) }

var foundationPortTypes = map[string]bool{
	"string": true, "enum": true, "bool": true, "boolean": true,
	"int": true, "integer": true, "number": true, "decimal": true,
	"quantity": true, "reference": true, "list": true, "object": true,
	"table": true, "geometry": true, "artifact": true, "duration": true,
	"timepoint": true, "relative_time": true,
	// Legacy aliases remain valid for existing documents.
	"well-map": true, "string[]": true, "object-list": true,
}

var foundationCategories = map[string]bool{
	"workflow": true, "resource": true, "labware": true,
	"measurement": true, "document": true, "device": true,
}

// knownModuleRules lists every rule specValidatorFor implements. loadSpec
// rejects a manifest that declares anything else, so a manifest-only addition
// fails at startup instead of on a user's document (module-catalog-v1.md §4.1).
var knownModuleRules = map[string]bool{
	"plate-wells": true, "plate-geometry-wells": true, "reference-ref": true,
	"group-fields": true, "assay-fields": true, "step-fields": true,
	"step-execution": true, "step-uses": true, "workflow-fields": true,
	"dilution-fields": true, "instrument-profile-fields": true,
	"material-fields": true, "sample-fields": true, "container-fields": true,
	"measurement-profile-fields": true, "rna-reagent-fields": true,
}

func validateSpecModule(module specModule, sdkVersion string) error {
	if strings.TrimSpace(module.Kind) == "" || module.Schema <= 0 {
		return fmt.Errorf("module needs kind and schema")
	}
	if module.SDK != "" && module.SDK != sdkVersion {
		return fmt.Errorf("%s@%d declares sdk %q, want %q", module.Kind, module.Schema, module.SDK, sdkVersion)
	}
	for _, rule := range module.Rules {
		if !knownModuleRules[rule] {
			return fmt.Errorf("%s@%d declares unregistered rule %q", module.Kind, module.Schema, rule)
		}
	}
	if module.Category != "" && !foundationCategories[module.Category] {
		return fmt.Errorf("%s@%d has unknown category %q", module.Kind, module.Schema, module.Category)
	}
	permissions := map[string]bool{}
	for _, permission := range module.Permissions {
		permissions[permission] = true
	}
	for _, action := range module.Actions {
		if action.Permission == "" || !permissions[action.Permission] {
			return fmt.Errorf("%s@%d action %q names undeclared permission %q",
				module.Kind, module.Schema, action.ID, action.Permission)
		}
	}
	for _, port := range append(append([]Port{}, module.Inputs...), module.Outputs...) {
		if !foundationPortTypes[port.Type] {
			return fmt.Errorf("%s@%d port %q has unknown type %q", module.Kind, module.Schema, port.Name, port.Type)
		}
		if port.ItemType != "" && !foundationPortTypes[port.ItemType] {
			return fmt.Errorf("%s@%d port %q has unknown item_type %q", module.Kind, module.Schema, port.Name, port.ItemType)
		}
		if port.Type == "reference" || port.ItemType == "reference" {
			if strings.TrimSpace(port.Target) == "" {
				return fmt.Errorf("%s@%d port %q is a reference without target", module.Kind, module.Schema, port.Name)
			}
		}
		if port.Type == "quantity" && port.Dimension != "" {
			found := false
			for _, unit := range unitBySymbol {
				if unit.Dimension == port.Dimension {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("%s@%d port %q has unknown dimension %q", module.Kind, module.Schema, port.Name, port.Dimension)
			}
		}
		if port.Origin == "derived" && port.Calculator == "" {
			return fmt.Errorf("%s@%d output %q is derived without calculator", module.Kind, module.Schema, port.Name)
		}
	}
	if module.Lifecycle != nil {
		for _, state := range module.Lifecycle.States {
			if state == "" {
				return fmt.Errorf("%s@%d has an empty lifecycle state", module.Kind, module.Schema)
			}
		}
	}
	if module.Provenance != nil && module.Provenance.Origin == "recorded" {
		return fmt.Errorf("%s@%d cannot store recorded facts in the document", module.Kind, module.Schema)
	}
	return nil
}

func builtinDescriptor(kind string) Descriptor {
	return descriptorFor(kind, 1)
}

func descriptorFor(kind string, schema int) Descriptor {
	module, ok := specByKey[specKey(kind, schema)]
	if !ok {
		if specErr != nil {
			panic("asterdoc: load module spec: " + specErr.Error())
		}
		panic(fmt.Sprintf("asterdoc: missing module spec %s@%d", kind, schema))
	}
	return module.Descriptor
}

// specValidator implements the declarative field and rule checks shared with
// Rust Core. Renderers still keep their typed parse, but the module contract is
// defined once in spec/modules.json.
func specValidator(kind string) func(map[string]any) error {
	return specValidatorFor(kind, 1)
}

func specValidatorFor(kind string, schema int) func(map[string]any) error {
	module, ok := specByKey[specKey(kind, schema)]
	if !ok {
		if specErr != nil {
			panic("asterdoc: load module spec: " + specErr.Error())
		}
		panic("asterdoc: missing module spec " + kind)
	}
	return func(data map[string]any) error {
		for _, input := range module.Inputs {
			value, present := data[input.Name]
			if !present || value == nil {
				if input.Required {
					return fmt.Errorf("missing required field %q", input.Name)
				}
				continue
			}
			if err := validatePortValue(input, value); err != nil {
				return err
			}
		}
		for _, rule := range module.Rules {
			switch rule {
			case "plate-wells", "plate-geometry-wells":
				if _, err := plateFromData(data); err != nil {
					return err
				}
			case "reference-ref":
				ref, _ := data["ref"].(string)
				if ref == "" {
					return fmt.Errorf("reference needs a ref")
				}
			case "group-fields":
				if name, _ := data["name"].(string); strings.TrimSpace(name) == "" {
					return fmt.Errorf("group needs a name")
				}
			case "assay-fields":
				if refs, ok := data["group_refs"].([]any); ok && len(refs) == 0 {
					return fmt.Errorf("assay needs at least one group_ref")
				}
			case "step-fields":
				if assay, _ := data["assay"].(string); assay == "" {
					return fmt.Errorf("step needs an assay")
				}
			case "step-execution":
				switch textValue(data, "execution") {
				case "executable":
					if strings.TrimSpace(textValue(data, "assay")) == "" {
						return fmt.Errorf("executable step needs an assay")
					}
				case "organizational":
					for _, field := range []string{"assay", "timepoints", "path", "checks", "tasks", "uses"} {
						value, ok := data[field]
						if !ok || value == nil {
							continue
						}
						switch value := value.(type) {
						case string:
							if strings.TrimSpace(value) != "" {
								return fmt.Errorf("organizational step cannot declare %s", field)
							}
						case []any:
							if len(value) > 0 {
								return fmt.Errorf("organizational step cannot declare %s", field)
							}
						default:
							return fmt.Errorf("organizational step cannot declare %s", field)
						}
					}
				default:
					return fmt.Errorf("step execution must be executable or organizational")
				}
			case "workflow-fields":
				steps, _ := data["steps"].([]any)
				if len(steps) == 0 {
					return fmt.Errorf("workflow needs at least one step")
				}
			case "step-uses":
				uses, _ := data["uses"].([]any)
				seen := map[string]bool{}
				for _, raw := range uses {
					item, _ := raw.(map[string]any)
					ref, _ := item["ref"].(string)
					role, _ := item["role"].(string)
					if strings.TrimSpace(ref) == "" || strings.TrimSpace(role) == "" {
						return fmt.Errorf("step use needs ref and role")
					}
					if seen[ref] {
						return fmt.Errorf("step use repeats %q", ref)
					}
					seen[ref] = true
				}
			case "dilution-fields":
				if _, err := dilutionResultFor(data); err != nil {
					return err
				}
			case "instrument-profile-fields":
				if strings.TrimSpace(textValue(data, "class")) == "" {
					return fmt.Errorf("instrument profile needs a class")
				}
				capabilities, _ := data["capabilities"].([]any)
				if len(capabilities) == 0 {
					return fmt.Errorf("instrument profile needs at least one capability")
				}
				for _, raw := range capabilities {
					item, _ := raw.(map[string]any)
					if name, _ := item["name"].(string); strings.TrimSpace(name) == "" {
						return fmt.Errorf("instrument capability needs a name")
					}
				}
			case "material-fields":
				if name, _ := data["name"].(string); strings.TrimSpace(name) == "" {
					return fmt.Errorf("material needs a name")
				}
				if role, _ := data["role"].(string); strings.TrimSpace(role) == "" {
					return fmt.Errorf("material needs a role")
				}
			case "sample-fields":
				if id, _ := data["sample_id"].(string); strings.TrimSpace(id) == "" {
					return fmt.Errorf("sample needs a sample_id")
				}
			case "container-fields":
				if format, _ := data["format"].(string); strings.TrimSpace(format) == "" {
					return fmt.Errorf("container needs a format")
				}
			case "rna-reagent-fields":
				if _, err := rnaReagentResultFor(data); err != nil {
					return err
				}
			case "measurement-profile-fields":
				if endpoint, _ := data["endpoint"].(string); strings.TrimSpace(endpoint) == "" {
					return fmt.Errorf("measurement profile needs an endpoint")
				}
				if unit, _ := data["unit"].(string); strings.TrimSpace(unit) == "" {
					return fmt.Errorf("measurement profile needs a unit")
				}
			default:
				return fmt.Errorf("unknown module rule %q", rule)
			}
		}
		return nil
	}
}

func validatePortValue(input Port, value any) error {
	switch input.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field %q must be a string", input.Name)
		}
	case "enum":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("field %q must be an enum string", input.Name)
		}
		if len(input.Values) > 0 {
			allowed := false
			for _, candidate := range input.Values {
				if text == candidate {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("field %q must be one of %s", input.Name, strings.Join(input.Values, ", "))
			}
		}
	case "int", "integer":
		if !isInt(value) {
			return fmt.Errorf("field %q must be an integer", input.Name)
		}
	case "number", "decimal":
		if !isNumber(value) {
			return fmt.Errorf("field %q must be a number", input.Name)
		}
	case "bool", "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field %q must be a boolean", input.Name)
		}
	case "well-map", "object", "geometry":
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("field %q must be a mapping", input.Name)
		}
	case "string[]":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("field %q must be a list of strings", input.Name)
		}
		for _, item := range items {
			if _, ok := item.(string); !ok {
				return fmt.Errorf("field %q must be a list of strings", input.Name)
			}
		}
	case "object-list", "table":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("field %q must be a list of objects", input.Name)
		}
		for _, item := range items {
			if _, ok := item.(map[string]any); !ok {
				return fmt.Errorf("field %q must be a list of objects", input.Name)
			}
		}
	case "list":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("field %q must be a list", input.Name)
		}
		for _, item := range items {
			if err := validateListItem(input, item); err != nil {
				return err
			}
		}
	case "quantity":
		if err := validateQuantity(value, input.Dimension); err != nil {
			return fmt.Errorf("field %q: %w", input.Name, err)
		}
	case "duration":
		if err := validateQuantity(value, "time"); err != nil {
			return fmt.Errorf("field %q: %w", input.Name, err)
		}
	case "reference":
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("field %q must be a non-empty reference", input.Name)
		}
	case "artifact":
		if err := validateArtifact(value); err != nil {
			return fmt.Errorf("field %q: %w", input.Name, err)
		}
	case "timepoint":
		if _, ok := value.(string); !ok {
			if mapping, mappingOK := value.(map[string]any); mappingOK {
				if text, _ := mapping["label"].(string); strings.TrimSpace(text) == "" {
					return fmt.Errorf("field %q timepoint needs a label", input.Name)
				}
				return nil
			}
			return fmt.Errorf("field %q must be a timepoint string or label mapping", input.Name)
		}
	case "relative_time":
		mapping, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("field %q must be a relative-time mapping", input.Name)
		}
		if text, _ := mapping["anchor"].(string); strings.TrimSpace(text) == "" {
			return fmt.Errorf("field %q relative time needs an anchor", input.Name)
		}
		if err := validateQuantity(mapping["offset"], "time"); err != nil {
			return fmt.Errorf("field %q offset: %w", input.Name, err)
		}
	}
	return nil
}

func validateListItem(input Port, item any) error {
	switch input.ItemType {
	case "", "object":
		if _, ok := item.(map[string]any); !ok {
			return fmt.Errorf("field %q must be a list of objects", input.Name)
		}
	case "string":
		if _, ok := item.(string); !ok {
			return fmt.Errorf("field %q must be a list of strings", input.Name)
		}
	case "reference":
		text, ok := item.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("field %q must be a list of non-empty references", input.Name)
		}
	case "quantity":
		if err := validateQuantity(item, input.Dimension); err != nil {
			return fmt.Errorf("field %q item: %w", input.Name, err)
		}
	case "artifact":
		if err := validateArtifact(item); err != nil {
			return fmt.Errorf("field %q item: %w", input.Name, err)
		}
	}
	return nil
}

func validateArtifact(value any) error {
	mapping, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("artifact must be a mapping")
	}
	uri, _ := mapping["uri"].(string)
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return fmt.Errorf("artifact needs a uri")
	}
	if strings.HasPrefix(strings.ToLower(uri), "data:") {
		return fmt.Errorf("artifact must reference external bytes, not inline data")
	}
	if hash, ok := mapping["sha256"].(string); ok && hash != "" {
		if len(hash) != 64 {
			return fmt.Errorf("artifact sha256 must be 64 hex characters")
		}
		for _, r := range hash {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return fmt.Errorf("artifact sha256 must be hex")
			}
		}
	}
	return nil
}

func isNumber(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	}
	return false
}

func isInt(value any) bool {
	switch n := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float64:
		return n == float64(int64(n))
	}
	return false
}
