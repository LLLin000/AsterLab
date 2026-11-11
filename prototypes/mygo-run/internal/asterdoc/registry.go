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
//
// Renders lists the renderer keys this implementation actually draws. A module
// may declare renderer keys in its Descriptor; Register refuses a declaration
// the implementation does not cover, so the manifest can never promise a
// surface the host cannot draw. Render may be nil for a headless module: a
// module with no interactive renderer is valid and simply reports a gap when
// a surface asks for one.
type ModuleSpec struct {
	Descriptor
	Validate func(data map[string]any) error
	Render   func(v *View, c *ui.Context, data map[string]any)
	// Renders are the implemented renderer keys, for example "plate96.editor"
	// and "plate96.viewer" when one function draws both surfaces.
	Renders []string
}

// Registry maps kind@schema to one implementation. Unknown kinds or versions
// are rejected; nothing falls back to a generic editable block.
type Registry struct {
	specs         map[string]ModuleSpec
	rendererOwner map[string]string
}

func NewRegistry() *Registry {
	return &Registry{specs: map[string]ModuleSpec{}, rendererOwner: map[string]string{}}
}

func key(kind string, schema int) string { return fmt.Sprintf("%s@%d", kind, schema) }

// Register rejects duplicate kinds and versions, incomplete validation, and a
// renderer declaration the implementation does not cover.
func (r *Registry) Register(spec ModuleSpec) error {
	if spec.Kind == "" || spec.Schema <= 0 {
		return fmt.Errorf("module spec needs a kind and positive schema")
	}
	if spec.Validate == nil {
		return fmt.Errorf("module %s needs Validate", key(spec.Kind, spec.Schema))
	}
	id := key(spec.Kind, spec.Schema)
	if _, exists := r.specs[id]; exists {
		return fmt.Errorf("module %s is registered twice", id)
	}
	if spec.Render == nil && len(spec.Renders) > 0 {
		return fmt.Errorf("module %s lists renderers but has no Render implementation", id)
	}
	implemented := map[string]bool{}
	for _, rk := range spec.Renders {
		if rk == "" {
			return fmt.Errorf("module %s has an empty renderer key", id)
		}
		// Two schema versions of the same kind may legitimately share one
		// renderer family (step@1 and step@2 both draw "step.viewer"). Only a
		// different kind claiming the same key is a real conflict.
		if owner := r.rendererOwner[rk]; owner != "" && owner != spec.Kind {
			return fmt.Errorf("renderer key %q is claimed by both %s and %s", rk, owner, spec.Kind)
		}
		implemented[rk] = true
	}
	// Every declared renderer must be covered by the implementation.
	if spec.Renderers != nil {
		for surface, rk := range spec.declaredRenderers() {
			if rk == "" {
				continue
			}
			if !implemented[rk] {
				return fmt.Errorf("module %s declares %s renderer %q but does not implement it", id, surface, rk)
			}
		}
	}
	if err := r.insert(spec); err != nil {
		return err
	}
	for rk := range implemented {
		r.rendererOwner[rk] = spec.Kind
	}
	return nil
}

// insert registers the spec without renderer bookkeeping; Register wraps it so
// a rejected spec never leaves partial renderer ownership behind.
func (r *Registry) insert(spec ModuleSpec) error {
	id := key(spec.Kind, spec.Schema)
	if _, exists := r.specs[id]; exists {
		return fmt.Errorf("module %s is registered twice", id)
	}
	r.specs[id] = spec
	return nil
}

// declaredRenderers maps a surface name to its declared renderer key.
func (d Descriptor) declaredRenderers() map[string]string {
	out := map[string]string{}
	if d.Renderers == nil {
		return out
	}
	if d.Renderers.Design != "" {
		out["design"] = d.Renderers.Design
	}
	if d.Renderers.Document != "" {
		out["document"] = d.Renderers.Document
	}
	if d.Renderers.Static != "" {
		out["static"] = d.Renderers.Static
	}
	return out
}

// surfaceFor maps a document mode to the renderer surface it needs. Design
// edits; every other reading mode uses the shared document viewer.
func surfaceFor(mode Mode) string {
	if mode == ModeDesign {
		return "design"
	}
	return "document"
}

// RendererKey returns the renderer a module declares for one mode, and whether
// it declares one at all. A missing key is a capability gap the host must show.
func (d Descriptor) RendererKey(mode Mode) (string, bool) {
	if d.Renderers == nil {
		return "", false
	}
	rk := d.declaredRenderers()[surfaceFor(mode)]
	return rk, rk != ""
}

// HasRenderer reports whether the registry holds an implementation for one
// renderer key. It is the check that keeps a declared-but-undrawn surface from
// silently falling back to a guess.
func (r *Registry) HasRenderer(rk string) bool {
	if rk == "" {
		return false
	}
	return r.rendererOwner[rk] != ""
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
	builtinErr  error
)

// BuiltinError reports the first shipped module that violated its own contract.
// It is non-nil only when a registry entry is malformed, which tests treat as a
// hard failure rather than a missing capability.
func BuiltinError() error { return builtinErr }

// BuiltinRegistry returns the process-wide registry with the shipped modules.
func BuiltinRegistry() *Registry {
	builtinOnce.Do(func() {
		builtin = NewRegistry()
		// A shipped module that fails its own contract is a build defect, not a
		// runtime condition: surface it instead of silently dropping the module.
		specs := []ModuleSpec{
			plate96Spec(), plateSpec(), referenceSpec(), experimentSpec(),
			groupSpec(), assaySpec(), stepSpec(), step2Spec(), workflowSpec(),
			dilutionSpec(), instrumentProfileSpec(), materialSpec(),
			sampleSpec(), containerSpec(), measurementProfileSpec(),
		}
		for _, spec := range specs {
			if err := builtin.Register(spec); err != nil {
				builtinErr = err
				return
			}
		}
	})
	return builtin
}
