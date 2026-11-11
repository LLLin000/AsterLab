package asterdoc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// StaticPreview renders one parsed module into deterministic plain text. It is
// the read-only data source for Notebook/Review/Run surfaces and PDF export;
// it never writes, edits, or opens a database handle.
func StaticPreview(m Module) string {
	registry := BuiltinRegistry()
	spec, ok := registry.Lookup(m.Kind, m.Schema)
	title := m.Kind
	if ok {
		title = spec.Descriptor.Title
	}
	lines := []string{fmt.Sprintf("%s (%s@%d) · %s", title, m.Kind, m.Schema, m.ID)}
	if m.Plate != nil {
		if m.Plate.Assay != "" {
			lines = append(lines, "  assay: "+m.Plate.Assay)
		}
		if m.Plate.Timepoint != "" {
			lines = append(lines, "  timepoint: "+m.Plate.Timepoint)
		}
		lines = append(lines, fmt.Sprintf("  format: %d", m.Plate.Format))
		var coordinates []string
		for row := 'A'; row <= 'H'; row++ {
			for column := 1; column <= 12; column++ {
				coordinates = append(coordinates, fmt.Sprintf("%c%d", row, column))
			}
		}
		lines = append(lines, "  coordinates: "+strings.Join(coordinates, ", "))
		for _, group := range m.Plate.GroupNames() {
			wells := append([]string(nil), m.Plate.Groups[group]...)
			sort.Strings(wells)
			lines = append(lines, fmt.Sprintf("  group %s: %s", group, strings.Join(wells, ", ")))
		}
		return strings.Join(lines, "\n")
	}
	if ok {
		for _, input := range spec.Descriptor.Inputs {
			if input.Name == "kind" || input.Name == "id" || input.Name == "schema" {
				continue
			}
			value, present := m.Data[input.Name]
			if !present || value == nil {
				continue
			}
			lines = append(lines, fmt.Sprintf("  %s: %v", input.Name, value))
		}
	} else {
		keys := make([]string, 0, len(m.Data))
		for key := range m.Data {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if key == "kind" || key == "id" || key == "schema" {
				continue
			}
			lines = append(lines, fmt.Sprintf("  %s: %v", key, m.Data[key]))
		}
	}
	return strings.Join(lines, "\n")
}

// StaticDocumentPreview renders every module in a parsed document in source
// order. It is the single read-only path used by Markdown surfaces.
func StaticDocumentPreview(doc *Document) string {
	if doc == nil {
		return ""
	}
	parts := make([]string, 0, len(doc.Modules))
	for _, m := range doc.Modules {
		parts = append(parts, StaticPreview(m))
	}
	return strings.Join(parts, "\n\n")
}

// FrozenAsterDoc is the immutable reference a Run pins. Rendering helpers
// take it by value; there is no live/latest fallback.
type FrozenAsterDoc struct {
	DesignID        string
	Revision        uint64
	Source          string
	SourceHash      string
	CompilerVersion string
}

// VerifySourceHash reports whether the supplied source still matches the hash
// pinned by a frozen Run reference.
func (f FrozenAsterDoc) VerifySourceHash(source string) bool {
	return sourceHash(source) == f.SourceHash
}


func sourceHash(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}
