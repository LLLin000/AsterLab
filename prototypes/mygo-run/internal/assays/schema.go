package assays

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/resultgrid"
)

const (
	CCK8       = "cck8"
	RTqPCR     = "qpcr"
	OD450      = "od450"
	Cq         = "cq"
	Missing    = "missing"
	Raw        = "raw"
	Invalid    = "invalid"
	Undetected = "undetected"
)

// Design is the assay-specific parameter set persisted under mygo.assays.
type Design struct {
	PlateInstanceID     string           `json:"plateInstanceId,omitempty"`
	Timepoints          []string         `json:"timepoints,omitempty"`
	TechnicalReplicates int              `json:"technicalReplicates,omitempty"`
	ControlGroup        string           `json:"controlGroup,omitempty"`
	CalibratorGroup     string           `json:"calibratorGroup,omitempty"`
	ReferenceGenes      []string         `json:"referenceGenes,omitempty"`
	Samples             []Sample         `json:"samples,omitempty"`
	Genes               []Gene           `json:"genes,omitempty"`
	Wells               []WellAssignment `json:"wells,omitempty"`
}

type WellAssignment struct {
	WellID    string `json:"wellId"`
	SampleID  string `json:"sampleId"`
	GroupID   string `json:"groupId"`
	Replicate int    `json:"replicate"`
	Timepoint string `json:"timepoint"`
}

type Sample struct {
	ID    string `json:"id"`
	Group string `json:"group,omitempty"`
}

type Gene struct {
	ID        string `json:"id"`
	Reference bool   `json:"reference,omitempty"`
}

type assaySpec struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Endpoint   string   `json:"endpoint"`
	GroupIDs   []string `json:"group_ids"`
	GroupNames []string `json:"group_names"`
	Timepoints []string `json:"timepoints"`
}

type groupSpec struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Role             string `json:"role"`
	TechnicalRepeats int    `json:"technical_repeats"`
}

type designDocument struct {
	GroupSpecs []groupSpec `json:"group_specs"`
	Assays     []assaySpec `json:"assays"`
	MyGo       struct {
		Instances map[string][]blocks.Instance `json:"instances"`
		Assays    map[string]Design            `json:"assays"`
	} `json:"mygo"`
}

// BuildSchema maps frozen design JSON to one assay/timepoint grid. Missing
// physical/sample allocation is kept missing; dimensions are never inferred
// from group counts or biological replicates.
func BuildSchema(designJSON []byte, assayID, timepoint string) (resultgrid.Schema, []resultgrid.Cell, error) {
	var doc designDocument
	if err := json.Unmarshal(designJSON, &doc); err != nil {
		return resultgrid.Schema{}, nil, fmt.Errorf("decode design: %w", err)
	}
	id := normalizeAssayID(assayID)
	params, hasParams := doc.MyGo.Assays[assayID]
	if !hasParams {
		params, hasParams = doc.MyGo.Assays[id]
	}
	spec := assaySpec{ID: assayID}
	for _, candidate := range doc.Assays {
		if normalizeAssayID(candidate.ID) == id {
			spec = candidate
			break
		}
	}
	if spec.ID == "" {
		return resultgrid.Schema{}, nil, fmt.Errorf("assay %q is not present in design", assayID)
	}
	if len(params.Timepoints) == 0 {
		params.Timepoints = spec.Timepoints
	}
	if timepoint == "" && len(params.Timepoints) > 0 {
		timepoint = params.Timepoints[0]
	}
	groups := make(map[string]groupSpec, len(doc.GroupSpecs))
	for _, group := range doc.GroupSpecs {
		groups[group.ID] = group
	}
	if params.ControlGroup == "" {
		for _, groupID := range spec.GroupIDs {
			if groups[groupID].Role == "control" {
				params.ControlGroup = groups[groupID].Name
				break
			}
		}
	}
	if params.TechnicalReplicates == 0 {
		var reps int
		consistent := true
		for _, groupID := range spec.GroupIDs {
			n := groups[groupID].TechnicalRepeats
			if n == 0 {
				continue
			}
			if reps != 0 && reps != n {
				consistent = false
				break
			}
			reps = n
		}
		if consistent {
			params.TechnicalReplicates = reps
		}
	}
	cfg, ok := LookupLayout(id)
	if !ok {
		return genericAssaySchema(spec, params, assayID, timepoint, groups)
	}
	if cfg.Build != nil {
		return cfg.Build(doc, spec, params, assayID, timepoint, groups)
	}
	return matrixSchema(cfg, spec, params, assayID, timepoint, groups)
}

func genericAssaySchema(spec assaySpec, params Design, assayID, timepoint string, groups map[string]groupSpec) (resultgrid.Schema, []resultgrid.Cell, error) {
	endpoint := strings.TrimSpace(spec.Endpoint)
	if endpoint == "" {
		endpoint = "value"
	}
	label := strings.TrimSpace(spec.Title)
	if label == "" {
		label = endpoint
	}
	reps := params.TechnicalReplicates
	if reps < 1 {
		reps = 1
	}
	metadata := map[string]string{
		"timepoint":           timepoint,
		"timepoints":          strings.Join(params.Timepoints, ","),
		"controlGroup":        params.ControlGroup,
		"technicalReplicates": fmt.Sprint(reps),
		"groups":              groupNames(spec, groups),
	}
	rows := []string{}
	seenRows := map[string]bool{}
	cells := []resultgrid.Cell{}
	for _, w := range params.Wells {
		if timepoint != "" && w.Timepoint != "" && w.Timepoint != timepoint {
			continue
		}
		id := strings.ToUpper(strings.TrimSpace(w.WellID))
		if id == "" {
			continue
		}
		if !seenRows[id] {
			seenRows[id] = true
			rows = append(rows, id)
		}
		cells = append(cells, resultgrid.Cell{
			Row:       len(rows) - 1,
			Column:    0,
			SampleID:  strings.TrimSpace(w.SampleID),
			Group:     strings.TrimSpace(w.GroupID),
			Replicate: w.Replicate,
			Field:     endpoint,
			Status:    Missing,
		})
	}
	if len(cells) == 0 {
		metadata["unresolved"] = "well or sample allocation absent"
	}
	schema := resultgrid.Schema{
		ID:           fmt.Sprintf("generic.%s.v1", strings.ToLower(strings.TrimSpace(assayID))),
		Version:      1,
		AssayID:      assayID,
		Layout:       "generic_table",
		RowLabels:    rows,
		ColumnLabels: []string{label},
		Fields:       []resultgrid.Field{{Key: endpoint, Label: label, DataType: "number"}},
		Metadata:     metadata,
	}
	return schema, cells, nil
}

func cck8Schema(doc designDocument, spec assaySpec, params Design, assayID, timepoint string, groups map[string]groupSpec) (resultgrid.Schema, []resultgrid.Cell, error) {
	rows, cols := []string{"A", "B", "C", "D", "E", "F", "G", "H"}, make([]string, 12)
	for i := range cols {
		cols[i] = fmt.Sprint(i + 1)
	}
	metadata := map[string]string{"timepoint": timepoint, "timepoints": strings.Join(params.Timepoints, ","), "controlGroup": params.ControlGroup, "technicalReplicates": fmt.Sprint(params.TechnicalReplicates)}
	metadata["groups"] = groupNames(spec, groups)
	metadata["unresolved"] = "plate allocation absent"
	if !contains(params.Timepoints, timepoint) {
		metadata["unresolved"] = "requested timepoint is not defined"
	}
	schema := resultgrid.Schema{ID: "cck8.plate96.od450.v1", Version: 1, AssayID: assayID, Layout: "plate96", RowLabels: rows, ColumnLabels: cols,
		Fields: []resultgrid.Field{{Key: OD450, Label: "OD450", Unit: "AU", DataType: "number", Required: true}}, Metadata: metadata}
	cells := make([]resultgrid.Cell, 96)
	for i := range cells {
		cells[i] = resultgrid.Cell{Row: i / 12, Column: i % 12, Field: OD450, Status: Missing}
	}
	if len(params.Wells) > 0 {
		seenWells := map[string]bool{}
		seenReplicates := map[string]string{}
		foundAtTimepoint, incomplete := false, false
		for _, well := range params.Wells {
			if well.Timepoint != timepoint {
				continue
			}
			foundAtTimepoint = true
			id := strings.ToUpper(strings.TrimSpace(well.WellID))
			if len(id) < 2 || id[0] < 'A' || id[0] > 'H' {
				return resultgrid.Schema{}, nil, fmt.Errorf("invalid well %q", well.WellID)
			}
			col, err := strconv.Atoi(id[1:])
			if err != nil || col < 1 || col > 12 || strconv.Itoa(col) != id[1:] {
				return resultgrid.Schema{}, nil, fmt.Errorf("invalid well %q", well.WellID)
			}
			group, sample := strings.TrimSpace(well.GroupID), strings.TrimSpace(well.SampleID)
			if group == "" || well.Replicate < 0 || (well.Replicate > 0 && sample == "") {
				return resultgrid.Schema{}, nil, fmt.Errorf("well %s has invalid group/sample/replicate mapping", id)
			}
			key := id + "@" + timepoint
			if seenWells[key] {
				return resultgrid.Schema{}, nil, fmt.Errorf("well %s assigned twice", id)
			}
			seenWells[key] = true
			if well.Replicate > 0 {
				replicateKey := sample + "\x00" + strconv.Itoa(well.Replicate)
				if previous, exists := seenReplicates[replicateKey]; exists {
					return resultgrid.Schema{}, nil, fmt.Errorf("sample %s replicate %d assigned to both %s and %s", sample, well.Replicate, previous, id)
				}
				seenReplicates[replicateKey] = id
			} else {
				incomplete = true
			}
			if sample == "" {
				incomplete = true
			}
			idx := (int(id[0]-'A'))*12 + col - 1
			cells[idx].SampleID, cells[idx].Group, cells[idx].Replicate = sample, group, well.Replicate
		}
		if !foundAtTimepoint {
			metadata["unresolved"] = "plate allocation absent for requested timepoint"
		} else if incomplete {
			metadata["unresolved"] = "well sample/replicate mapping is incomplete"
		} else {
			delete(metadata, "unresolved")
		}
		return schema, cells, nil
	}
	if !contains(params.Timepoints, timepoint) || params.PlateInstanceID == "" {
		return schema, cells, nil
	}
	var plate *blocks.Instance
	nodeIDs := make([]string, 0, len(doc.MyGo.Instances))
	for nodeID := range doc.MyGo.Instances {
		nodeIDs = append(nodeIDs, nodeID)
	}
	sort.Strings(nodeIDs)
	for _, nodeID := range nodeIDs {
		for i := range doc.MyGo.Instances[nodeID] {
			inst := &doc.MyGo.Instances[nodeID][i]
			if inst.ID == params.PlateInstanceID {
				if plate != nil {
					return resultgrid.Schema{}, nil, fmt.Errorf("cck8 plate instance %q is ambiguous", params.PlateInstanceID)
				}
				plate = inst
			}
		}
	}
	if plate == nil || plate.Type != blocks.TypePlate96 {
		return schema, cells, nil
	}
	var props blocks.Plate96Props
	if err := json.Unmarshal(plate.PropsJSON(), &props); err != nil {
		return resultgrid.Schema{}, nil, fmt.Errorf("decode cck8 plate: %w", err)
	}
	if (props.Rows != 0 && props.Rows != 8) || (props.Cols != 0 && props.Cols != 12) {
		return resultgrid.Schema{}, nil, fmt.Errorf("cck8 plate must be 8x12; got %dx%d", props.Rows, props.Cols)
	}
	wells := append([]int(nil), props.Selected...)
	sort.Ints(wells)
	seen := make(map[int]bool, len(wells))
	for _, well := range wells {
		if well < 0 || well >= 96 {
			return resultgrid.Schema{}, nil, fmt.Errorf("cck8 well index %d out of range", well)
		}
		if seen[well] {
			return resultgrid.Schema{}, nil, fmt.Errorf("cck8 well index %d selected more than once", well)
		}
		seen[well] = true
		cell := &cells[well]
		cell.Group, cell.SampleID = props.Groups[well], props.Labels[well]
	}
	if len(wells) == 0 {
		metadata["unresolved"] = "plate has no selected wells"
	} else {
		metadata["unresolved"] = "legacy plate has no explicit replicate mapping"
	}
	return schema, cells, nil
}

func qpcrSchema(spec assaySpec, params Design, assayID, timepoint string, groups map[string]groupSpec) (resultgrid.Schema, []resultgrid.Cell, error) {
	genes := append([]Gene(nil), params.Genes...)
	referenceGenes := append([]string(nil), params.ReferenceGenes...)
	for _, gene := range genes {
		if gene.Reference && !contains(referenceGenes, gene.ID) {
			referenceGenes = append(referenceGenes, gene.ID)
		}
	}
	metadata := map[string]string{"timepoint": timepoint, "timepoints": strings.Join(params.Timepoints, ","), "groups": groupNames(spec, groups),
		"referenceGenes": strings.Join(referenceGenes, ","), "calibratorGroup": params.CalibratorGroup,
		"technicalReplicates": fmt.Sprint(params.TechnicalReplicates)}
	if len(params.Samples) == 0 || len(genes) == 0 || params.TechnicalReplicates < 1 {
		metadata["unresolved"] = "sample, gene, or technical-replicate allocation absent"
	}
	rows := make([]string, len(params.Samples))
	for i, sample := range params.Samples {
		rows[i] = sample.ID
	}
	cols := make([]string, 0, len(genes)*params.TechnicalReplicates)
	for _, gene := range genes {
		for rep := 1; rep <= params.TechnicalReplicates; rep++ {
			cols = append(cols, fmt.Sprintf("%s_R%d", gene.ID, rep))
		}
	}
	schema := resultgrid.Schema{ID: "qpcr.sample_gene.cq.v1", Version: 1, AssayID: assayID, Layout: "sample_gene", RowLabels: rows, ColumnLabels: cols,
		Fields: []resultgrid.Field{{Key: Cq, Label: "Cq", Unit: "Cq", DataType: "number", Required: true}}, Metadata: metadata}
	cells := make([]resultgrid.Cell, 0, len(rows)*len(cols))
	for row, sample := range params.Samples {
		for geneIndex := range genes {
			for rep := 1; rep <= params.TechnicalReplicates; rep++ {
				col := geneIndex*params.TechnicalReplicates + rep - 1
				cells = append(cells, resultgrid.Cell{Row: row, Column: col, SampleID: sample.ID, Group: sample.Group, Replicate: rep, Field: Cq, Status: Missing})
			}
		}
	}
	if len(params.Samples) > 0 && len(genes) > 0 && params.TechnicalReplicates > 0 {
		delete(metadata, "unresolved")
	}
	return schema, cells, nil
}

func groupNames(spec assaySpec, groups map[string]groupSpec) string {
	names := append([]string(nil), spec.GroupNames...)
	if len(names) == 0 {
		for _, id := range spec.GroupIDs {
			if group := groups[id]; group.Name != "" {
				names = append(names, group.Name)
			}
		}
	}
	return strings.Join(names, ",")
}

func normalizeAssayID(id string) string {
	key := strings.ToLower(strings.TrimSpace(id))
	if canonical, ok := layoutAliases[key]; ok {
		return canonical
	}
	return key
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func ValidateWellAssignments(wells []WellAssignment) error {
	seenWells := map[string]bool{}
	seenReplicates := map[string]string{}
	for _, well := range wells {
		id := strings.ToUpper(strings.TrimSpace(well.WellID))
		if len(id) < 2 || id[0] < 'A' || id[0] > 'H' {
			return fmt.Errorf("invalid well %q", well.WellID)
		}
		col, err := strconv.Atoi(id[1:])
		if err != nil || col < 1 || col > 12 || strconv.Itoa(col) != id[1:] {
			return fmt.Errorf("invalid well %q", well.WellID)
		}
		if strings.TrimSpace(well.GroupID) == "" || strings.TrimSpace(well.Timepoint) == "" ||
			well.Replicate < 0 || (well.Replicate > 0 && strings.TrimSpace(well.SampleID) == "") {
			return fmt.Errorf("well %s requires a group and timepoint; replicate requires an explicit sample", id)
		}
		key := id + "\x00" + well.Timepoint
		if seenWells[key] {
			return fmt.Errorf("well %s assigned twice at %s", id, well.Timepoint)
		}
		seenWells[key] = true
		if well.Replicate > 0 {
			replicateKey := strings.TrimSpace(well.Timepoint) + "\x00" + strings.TrimSpace(well.SampleID) + "\x00" + strconv.Itoa(well.Replicate)
			if previous, exists := seenReplicates[replicateKey]; exists {
				return fmt.Errorf("sample %s replicate %d assigned to both %s and %s", well.SampleID, well.Replicate, previous, id)
			}
			seenReplicates[replicateKey] = id
		}
	}
	return nil
}
