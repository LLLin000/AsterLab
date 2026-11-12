package asterdoc

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/egoist/mygo/ui"
)

type Mode string

const (
	ModeDesign   Mode = "design"
	ModeProtocol Mode = "protocol"
	ModeRun      Mode = "run"
	ModeNotebook Mode = "notebook"
	ModeReview   Mode = "review"
)

// Density is presentation context; it never implies write access.
type Density string

const (
	DensityCompact   Density = "compact"
	DensityInspector Density = "inspector"
	DensityFull      Density = "full"
)

const (
	PermDocumentEdit = "document.edit"
	PermRunReadout   = "run.readout"
	PermNoteAppend   = "notebook.append"
)

// Port is one module input or derived output.
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

// Renderers declares the renderer key for each supported surface. Missing keys
// are explicit capability gaps, not invitations for the host to guess.
type Renderers struct {
	Design   string `json:"design,omitempty"`
	Document string `json:"document,omitempty"`
	Run      string `json:"run,omitempty"`
	Static   string `json:"static,omitempty"`
}

type Lifecycle struct {
	States []string `json:"states,omitempty"`
	Freeze string   `json:"freeze,omitempty"`
}

type Provenance struct {
	Origin         string `json:"origin,omitempty"`
	RecordBoundary string `json:"record_boundary,omitempty"`
}

type Action struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Permission string   `json:"permission"`
	Writes     []string `json:"writes"`
}

// Descriptor is the machine-readable contract for one kind and schema.
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

// ModuleSpec carries module validation and only module-specific callbacks.
// Reusable callbacks are registered once on Registry and shared by key.
type ModuleSpec struct {
	Descriptor
	Validate        func(data map[string]any) error
	Implementations map[string]RendererFunc
}

type Registry struct {
	specs     map[string]ModuleSpec
	renderers map[string]RendererFunc
}

type RendererFunc func(v *View, c *ui.Context, data map[string]any)

func NewRegistry() *Registry {
	return &Registry{specs: map[string]ModuleSpec{}, renderers: map[string]RendererFunc{}}
}

func key(kind string, schema int) string { return fmt.Sprintf("%s@%d", kind, schema) }

// Register validates renderer contracts before changing registry state.
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

	declared := map[string]bool{}
	for surface, rendererKey := range spec.declaredRenderers() {
		if rendererKey == "" {
			continue
		}
		declared[rendererKey] = true
		if spec.Implementations[rendererKey] == nil && r.renderers[rendererKey] == nil {
			return fmt.Errorf("module %s declares %s renderer %q without an implementation", id, surface, rendererKey)
		}
	}
	for rendererKey, fn := range spec.Implementations {
		if rendererKey == "" || fn == nil {
			return fmt.Errorf("module %s has an invalid renderer implementation", id)
		}
		if !declared[rendererKey] {
			return fmt.Errorf("module %s implements undeclared renderer %q", id, rendererKey)
		}
		if r.renderers[rendererKey] != nil {
			return fmt.Errorf("renderer key %q is already registered", rendererKey)
		}
	}

	r.specs[id] = spec
	for rendererKey, fn := range spec.Implementations {
		r.renderers[rendererKey] = fn
	}
	return nil
}

// RegisterRenderer installs a shared callback that any module may reference.
func (r *Registry) RegisterRenderer(rendererKey string, fn RendererFunc) error {
	if rendererKey == "" || fn == nil {
		return fmt.Errorf("renderer key and implementation are required")
	}
	if r.renderers[rendererKey] != nil {
		return fmt.Errorf("renderer key %q is already registered", rendererKey)
	}
	r.renderers[rendererKey] = fn
	return nil
}

func (r *Registry) Renderer(rendererKey string) (RendererFunc, bool) {
	fn, ok := r.renderers[rendererKey]
	return fn, ok
}

func (r *Registry) HasRenderer(rendererKey string) bool {
	return rendererKey != "" && r.renderers[rendererKey] != nil
}

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
	if d.Renderers.Run != "" {
		out["run"] = d.Renderers.Run
	}
	if d.Renderers.Static != "" {
		out["static"] = d.Renderers.Static
	}
	return out
}

func surfaceFor(mode Mode) string {
	switch mode {
	case ModeDesign:
		return "design"
	case ModeRun:
		return "run"
	default:
		return "document"
	}
}

// RendererKey returns the renderer for a mode and whether that surface is declared.
func (d Descriptor) RendererKey(mode Mode) (string, bool) {
	rendererKey := d.declaredRenderers()[surfaceFor(mode)]
	return rendererKey, rendererKey != ""
}

func (r *Registry) Lookup(kind string, schema int) (ModuleSpec, bool) {
	spec, ok := r.specs[key(kind, schema)]
	return spec, ok
}

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
	for id := range r.specs {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func specDescriptorsFor() []Descriptor { return append([]Descriptor(nil), specDescriptors...) }

var (
	builtinOnce sync.Once
	builtin     *Registry
	builtinErr  error
)

func BuiltinError() error { return builtinErr }

func BuiltinRegistry() *Registry {
	builtinOnce.Do(func() {
		builtin = NewRegistry()
		if err := builtin.RegisterRenderer("generic.editor", func(v *View, c *ui.Context, data map[string]any) {
			moduleID, _ := data["id"].(string)
			v.GenericModuleEditor(c, moduleID, data)
		}); err != nil {
			builtinErr = err
			return
		}
		if err := builtin.RegisterRenderer("generic.run", func(v *View, c *ui.Context, data map[string]any) {
			v.GenericModuleSummary(c, data)
		}); err != nil {
			builtinErr = err
			return
		}
		if err := builtin.RegisterRenderer("step.viewer", renderStepSummary); err != nil {
			builtinErr = err
			return
		}
		specs := []ModuleSpec{
			plateSpec(), referenceSpec(), experimentSpec(), groupSpec(),
			assaySpec(), stepSpec(), step2Spec(), step3Spec(), workflowSpec(),
			dilutionSpec(), instrumentProfileSpec(), materialSpec(),
			sampleSpec(), containerSpec(), measurementProfileSpec(),
			rnaReagentSpec(),
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
