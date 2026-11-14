package assays

import (
	"fmt"
	"strings"

	"asterlab-mygo-prototype/internal/resultgrid"
)

const WesternBlot = "western_blot"

// LayoutBuilder maps a frozen assay specification into a resultgrid Schema and initial cells.
type LayoutBuilder func(doc designDocument, spec assaySpec, params Design, assayID, timepoint string, groups map[string]groupSpec) (resultgrid.Schema, []resultgrid.Cell, error)

// LayoutConfig declares one assay recording layout over the shared resultgrid kernel.
// Adding a new matrix assay type only requires registering a LayoutConfig entry.
type LayoutConfig struct {
	AssayID           string
	Aliases           []string
	SchemaID          string
	Layout            string
	DefaultView       string // "plate" | "table"
	Field             resultgrid.Field
	DefaultColumns    []string
	DefaultReplicates int
	Build             LayoutBuilder // nil uses the declarative matrixSchema builder
}

var (
	layoutRegistry = map[string]LayoutConfig{}
	layoutAliases  = map[string]string{}
)

func init() {
	RegisterLayout(LayoutConfig{
		AssayID:     CCK8,
		Aliases:     []string{"cck8", "cck-8", "assay-cck8", "assay-cck-8"},
		SchemaID:    "cck8.plate96.od450.v1",
		Layout:      "plate96",
		DefaultView: "plate",
		Build:       cck8Schema,
	})
	RegisterLayout(LayoutConfig{
		AssayID:     RTqPCR,
		Aliases:     []string{"qpcr", "rt-qpcr", "rtqpcr", "rt-q-pcr", "rna", "assay-qpcr", "assay-rna"},
		SchemaID:    "qpcr.sample_gene.cq.v1",
		Layout:      "sample_gene",
		DefaultView: "table",
		Build: func(_ designDocument, spec assaySpec, params Design, assayID, timepoint string, groups map[string]groupSpec) (resultgrid.Schema, []resultgrid.Cell, error) {
			return qpcrSchema(spec, params, assayID, timepoint, groups)
		},
	})
	// Third assay type added purely via layout config (no resultgrid or custom builder changes).
	RegisterLayout(LayoutConfig{
		AssayID:           WesternBlot,
		Aliases:           []string{"western_blot", "western-blot", "westernblot", "wb", "immunoblot", "assay-wb", "assay-western-blot"},
		SchemaID:          "qpcr.sample_gene.cq.v1",
		Layout:            "sample_target",
		DefaultView:       "table",
		Field:             resultgrid.Field{Key: "intensity", Label: "条带灰度", Unit: "AU", DataType: "number", Required: true},
		DefaultColumns:    []string{"Target", "GAPDH"},
		DefaultReplicates: 1,
	})
}

// RegisterLayout registers an assay layout config and its normalized aliases.
func RegisterLayout(cfg LayoutConfig) {
	id := strings.ToLower(strings.TrimSpace(cfg.AssayID))
	if id == "" {
		return
	}
	cfg.AssayID = id
	if cfg.DefaultView == "" {
		cfg.DefaultView = "table"
	}
	layoutRegistry[id] = cfg
	layoutAliases[id] = id
	for _, alias := range cfg.Aliases {
		key := strings.ToLower(strings.TrimSpace(alias))
		if key != "" {
			layoutAliases[key] = id
		}
	}
}

// LookupLayout resolves an assay ID or alias to its registered LayoutConfig.
func LookupLayout(assayID string) (LayoutConfig, bool) {
	id := normalizeAssayID(assayID)
	cfg, ok := layoutRegistry[id]
	return cfg, ok
}

// DefaultViewForSchema returns the registered default view ("plate" or "table")
// for a resultgrid Schema, defaulting to "plate" when unspecified.
func DefaultViewForSchema(schema resultgrid.Schema) string {
	for _, cfg := range layoutRegistry {
		if (schema.ID != "" && cfg.SchemaID == schema.ID) || (schema.Layout != "" && cfg.Layout == schema.Layout) {
			if cfg.DefaultView != "" {
				return cfg.DefaultView
			}
		}
	}
	if schema.Layout == "sample_gene" || schema.Layout == "sample_target" {
		return "table"
	}
	return "plate"
}

func matrixSchema(cfg LayoutConfig, spec assaySpec, params Design, assayID, timepoint string, groups map[string]groupSpec) (resultgrid.Schema, []resultgrid.Cell, error) {
	reps := params.TechnicalReplicates
	if reps < 1 {
		reps = cfg.DefaultReplicates
	}
	if reps < 1 {
		reps = 1
	}
	samples := append([]Sample(nil), params.Samples...)
	if len(samples) == 0 {
		for _, gid := range spec.GroupIDs {
			name := groups[gid].Name
			if name == "" {
				name = gid
			}
			samples = append(samples, Sample{ID: name, Group: name})
		}
	}
	targets := make([]string, 0, len(params.Genes))
	for _, g := range params.Genes {
		if strings.TrimSpace(g.ID) != "" {
			targets = append(targets, g.ID)
		}
	}
	if len(targets) == 0 {
		targets = append(targets, cfg.DefaultColumns...)
	}

	metadata := map[string]string{
		"timepoint":           timepoint,
		"timepoints":          strings.Join(params.Timepoints, ","),
		"groups":              groupNames(spec, groups),
		"technicalReplicates": fmt.Sprint(reps),
	}
	if len(samples) == 0 || len(targets) == 0 {
		metadata["unresolved"] = "sample or target allocation absent"
	}

	rows := make([]string, len(samples))
	for i, s := range samples {
		rows[i] = s.ID
	}
	cols := make([]string, 0, len(targets)*reps)
	for _, target := range targets {
		for rep := 1; rep <= reps; rep++ {
			cols = append(cols, fmt.Sprintf("%s_R%d", target, rep))
		}
	}

	schema := resultgrid.Schema{
		ID:           cfg.SchemaID,
		Version:      1,
		AssayID:      assayID,
		Layout:       cfg.Layout,
		RowLabels:    rows,
		ColumnLabels: cols,
		Fields:       []resultgrid.Field{cfg.Field},
		Metadata:     metadata,
	}
	cells := make([]resultgrid.Cell, 0, len(rows)*len(cols))
	for row, s := range samples {
		for tIdx := range targets {
			for rep := 1; rep <= reps; rep++ {
				col := tIdx*reps + rep - 1
				cells = append(cells, resultgrid.Cell{
					Row:       row,
					Column:    col,
					SampleID:  s.ID,
					Group:     s.Group,
					Replicate: rep,
					Field:     cfg.Field.Key,
					Status:    Missing,
				})
			}
		}
	}
	return schema, cells, nil
}
