package asterdoc

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/egoist/mygo/ui"
)

// Mode is the document surface a module is rendered in. It is informational;
// write access is decided only by explicit permissions.
type Mode string

const (
	ModeDesign   Mode = "design"
	ModeProtocol Mode = "protocol"
	ModeRun      Mode = "run"
	ModeNotebook Mode = "notebook"
	ModeReview   Mode = "review"
)

// Permission names the business command a module action needs. The renderer
// must never infer write access from Mode or ReadOnly alone.
const (
	PermDocumentEdit = "document.edit"
	PermRunReadout   = "run.readout"
	PermNoteAppend   = "notebook.append"
)

// Port is one module input or derived output. Type is the Foundation v1 type
// vocabulary. Target constrains a reference; ItemType constrains list items;
// Dimension constrains a quantity; Values constrains an enum.
type Port struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Description string   `json:"description"`
	Target      string   `json:"target,omitempty"`
	ItemType    string   `json:"item_type,omitempty"`
	Dimension   string   `json:"dimension,omitempty"`
	Values      []string `json:"values,omitempty"`
	Origin      string   `json:"origin,omitempty"`
	Calculator  string   `json:"calculator,omitempty"`
}

// Renderers declares which surfaces a module can render. A missing renderer is
// an explicit capability gap, not something the host may fake.
type Renderers struct {
	Design   string `json:"design,omitempty"`
	Document string `json:"document,omitempty"`
	Static   string `json:"static,omitempty"`
}

// Lifecycle declares the module instance states and the freeze rule.
type Lifecycle struct {
	States []string `json:"states,omitempty"`
	Freeze string   `json:"freeze,omitempty"`
}

// Provenance declares whether the module is planned document data or a
// recorded fact, and where recorded facts are allowed to live.
type Provenance struct {
	Origin         string `json:"origin,omitempty"`
	RecordBoundary string `json:"record_boundary,omitempty"`
}

// Action is a business command a module can invoke. Permission is checked by
// the host before the action runs; Writes names the document/fact fields the
// action may change.
type Action struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Permission string   `json:"permission"`
	Writes     []string `json:"writes"`
}

// Descriptor is the agent-facing contract for one module kind and schema.
// Optional contract fields are absent on caller-owned test modules; shipped
// modules declare them in spec/modules.json.
type Descriptor struct {
	Kind         string      `json:"kind"`
	Schema       int         `json:"schema"`
	Title        string      `json:"title"`
	Description  string      `json:"description"`
	SDK          string      `json:"sdk,omitempty"`
	Category     string      `json:"category,omitempty"`
	Capabilities []string    `json:"capabilities,omitempty"`
	Permissions  []string    `json:"permissions,omitempty"`
	Renderers    *Renderers  `json:"renderers,omitempty"`
	Lifecycle    *Lifecycle  `json:"lifecycle,omitempty"`
	Provenance   *Provenance `json:"provenance,omitempty"`
	Modes        []Mode      `json:"modes"`
	Inputs       []Port      `json:"inputs"`
	Outputs      []Port      `json:"outputs"`
	Actions      []Action    `json:"actions"`
	Rules        []string    `json:"rules,omitempty"`
}

// ModuleSpec is one registered module implementation.
type ModuleSpec struct {
	Descriptor
	Validate func(data map[string]any) error
	Render   func(v *View, c *ui.Context, data map[string]any)
}

// Registry maps kind@schema to one implementation. Unknown kinds or versions
// are rejected; nothing falls back to a generic editable block.
type Registry struct {
	specs map[string]ModuleSpec
}

func NewRegistry() *Registry { return &Registry{specs: map[string]ModuleSpec{}} }

func key(kind string, schema int) string { return fmt.Sprintf("%s@%d", kind, schema) }

// Register rejects duplicate kinds and versions and incomplete contracts.
func (r *Registry) Register(spec ModuleSpec) error {
	if spec.Kind == "" || spec.Schema <= 0 {
		return fmt.Errorf("module spec needs a kind and positive schema")
	}
	if spec.Validate == nil || spec.Render == nil {
		return fmt.Errorf("module %s needs Validate and Render", key(spec.Kind, spec.Schema))
	}
	if _, exists := r.specs[key(spec.Kind, spec.Schema)]; exists {
		return fmt.Errorf("module %s is registered twice", key(spec.Kind, spec.Schema))
	}
	r.specs[key(spec.Kind, spec.Schema)] = spec
	return nil
}

func (r *Registry) Lookup(kind string, schema int) (ModuleSpec, bool) {
	spec, ok := r.specs[key(kind, schema)]
	return spec, ok
}

// Descriptors lists the agent-facing contracts in stable order.
func (r *Registry) Descriptors() []Descriptor {
	out := make([]Descriptor, 0, len(r.specs))
	for _, spec := range r.specs {
		out = append(out, spec.Descriptor)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Schema < out[j].Schema
	})
	return out
}

// ManifestJSON is the machine-readable contract an Agent reads before
// authoring a module block. It contains no Go function values.
func (r *Registry) ManifestJSON() ([]byte, error) {
	return json.MarshalIndent(struct {
		SchemaVersion     int          `json:"schemaVersion"`
		FoundationVersion int          `json:"foundationVersion"`
		SDKVersion        string       `json:"sdkVersion"`
		Modules           []Descriptor `json:"modules"`
	}{1, specFoundationVer, specSDKVersion, r.Descriptors()}, "", "  ")
}

func (r *Registry) supported() []string {
	out := make([]string, 0, len(r.specs))
	for key := range r.specs {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// specDescriptorsFor returns the descriptors from the shared spec file in
// stable order; it is used by tests and tooling that need the builtin contract
// without constructing renderers.
func specDescriptorsFor() []Descriptor { return append([]Descriptor(nil), specDescriptors...) }

var (
	builtinOnce sync.Once
	builtin     *Registry
)

// BuiltinRegistry returns the process-wide registry with the shipped modules.
func BuiltinRegistry() *Registry {
	builtinOnce.Do(func() {
		builtin = NewRegistry()
		_ = builtin.Register(plate96Spec())
		_ = builtin.Register(plateSpec())
		_ = builtin.Register(referenceSpec())
		_ = builtin.Register(experimentSpec())
		_ = builtin.Register(groupSpec())
		_ = builtin.Register(assaySpec())
		_ = builtin.Register(stepSpec())
		_ = builtin.Register(step2Spec())
		_ = builtin.Register(workflowSpec())
		_ = builtin.Register(dilutionSpec())
		_ = builtin.Register(instrumentProfileSpec())
	})
	return builtin
}
