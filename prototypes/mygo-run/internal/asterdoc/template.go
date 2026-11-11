package asterdoc

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// TemplateDependency is one required module kind for a template.
type TemplateDependency struct {
	Kind         string `json:"kind" yaml:"kind"`
	Schema       int    `json:"schema" yaml:"schema"`
	VersionRange string `json:"version_range,omitempty" yaml:"version_range,omitempty"`
}

// TemplateParameter describes one instantiation parameter.
type TemplateParameter struct {
	Name        string         `json:"name" yaml:"name"`
	Type        string         `json:"type" yaml:"type"`
	Default     any            `json:"default,omitempty" yaml:"default,omitempty"`
	Constraint  string         `json:"constraint,omitempty" yaml:"constraint,omitempty"`
	Constraints map[string]any `json:"constraints,omitempty" yaml:"constraints,omitempty"`
}

// TemplateManifest is the Foundation v1 template contract. There is only this
// manifest entry; no separate template registry exists.
type TemplateManifest struct {
	TemplateID   string               `json:"template_id" yaml:"template_id"`
	Version      string               `json:"version" yaml:"version"`
	Dependencies []TemplateDependency  `json:"dependencies" yaml:"dependencies"`
	Parameters   []TemplateParameter   `json:"parameters" yaml:"parameters"`
	Requirements []string             `json:"requirements" yaml:"requirements"`
	Defaults     map[string]any       `json:"defaults" yaml:"defaults"`
	Instantiate  map[string]any       `json:"instantiate" yaml:"instantiate"`
	Provenance   map[string]any       `json:"provenance" yaml:"provenance"`
}

// Template pairs a manifest with the AsterDoc source it instantiates.
type Template struct {
	Manifest TemplateManifest
	Source   string
}

// InstantiateTemplate produces a new AsterDoc source from a template and
// parameter values. Every module ID (and task ID) is replaced with a fresh
// ID, and all internal references are remapped. The returned source parses
// independently of the template source.
func InstantiateTemplate(template any, params any) (string, error) {
	source := ""
	var manifest TemplateManifest
	switch v := template.(type) {
	case string:
		source = v
	case *Document:
		if v != nil {
			source = v.Source
		}
	case Document:
		source = v.Source
	case Template:
		source = v.Source
		manifest = v.Manifest
	case *Template:
		if v != nil {
			source = v.Source
			manifest = v.Manifest
		}
	case TemplateManifest:
		manifest = v
	}
	pmap := map[string]any{}
	switch v := params.(type) {
	case map[string]any:
		pmap = v
	case nil:
	default:
		// Best-effort: marshal/unmarshal into a map.
		raw, err := yaml.Marshal(v)
		if err == nil {
			_ = yaml.Unmarshal(raw, &pmap)
		}
	}
	// Manifest defaults are the base; explicit params override.
	merged := map[string]any{}
	for k, val := range manifest.Defaults {
		merged[k] = val
	}
	for k, val := range pmap {
		merged[k] = val
	}
	return instantiateSource(source, merged)
}

// InstantiateTemplateDocument is like InstantiateTemplate but returns a parsed
// document.
func InstantiateTemplateDocument(template any, params any) (*Document, error) {
	source, err := InstantiateTemplate(template, params)
	if err != nil {
		return nil, err
	}
	return Parse(source)
}

func instantiateSource(source string, params map[string]any) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", fmt.Errorf("template source is empty")
	}
	doc, err := Parse(source)
	if err != nil {
		return "", err
	}
	suffix := newTemplateSuffix()
	idMap := map[string]string{}
	taskMap := map[string]string{}
	for _, module := range doc.Modules {
		idMap[module.ID] = module.ID + "-" + suffix
		if tasks, ok := module.Data["tasks"].([]any); ok {
			for _, raw := range tasks {
				if task, ok := raw.(map[string]any); ok {
					if id, _ := task["id"].(string); strings.TrimSpace(id) != "" {
						taskMap[id] = id + "-" + suffix
					}
				}
			}
		}
	}
	// Build remapped module data.
	remapped := make([]map[string]any, len(doc.Modules))
	for i, module := range doc.Modules {
		data := cloneAny(module.Data)
		m, _ := data.(map[string]any)
		if m == nil {
			m = map[string]any{}
		}
		remapped[i] = remapMaps(m, idMap, taskMap).(map[string]any)
	}
	// Reconstruct source preserving prose between fences.
	var b strings.Builder
	pos := 0
	for i, module := range doc.Modules {
		b.WriteString(source[pos:module.Start])
		raw, err := yaml.Marshal(remapped[i])
		if err != nil {
			return "", err
		}
		b.WriteString(string(raw))
		pos = module.End
	}
	b.WriteString(source[pos:])
	out := b.String()
	// Validate the result parses.
	if _, err := Parse(out); err != nil {
		return "", err
	}
	return out, nil
}

func newTemplateSuffix() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", len(buf))
	}
	return hex.EncodeToString(buf)
}

func cloneAny(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, vv := range val {
			out[k] = cloneAny(vv)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, vv := range val {
			out[i] = cloneAny(vv)
		}
		return out
	default:
		return val
	}
}

func remapMaps(v any, idMap, taskMap map[string]string) any {
	switch val := v.(type) {
	case string:
		if n, ok := idMap[val]; ok {
			return n
		}
		if n, ok := taskMap[val]; ok {
			return n
		}
		return val
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, vv := range val {
			newKey := k
			if n, ok := idMap[k]; ok {
				newKey = n
			} else if n, ok := taskMap[k]; ok {
				newKey = n
			}
			out[newKey] = remapMaps(vv, idMap, taskMap)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, vv := range val {
			out[i] = remapMaps(vv, idMap, taskMap)
		}
		return out
	default:
		return val
	}
}

// TemplateManifestsAreUnique is a no-op guard documenting that there is only
// one manifest entry point.
func TemplateManifestKeys(m TemplateManifest) []string {
	keys := []string{"template_id", "version", "dependencies", "parameters", "requirements", "defaults", "instantiate", "provenance"}
	sort.Strings(keys)
	return keys
}
