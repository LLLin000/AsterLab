package asterdoc

import (
	"testing"
)

func TestApplyPlateSamplesMapsSharedSampleAndReplicates(t *testing.T) {
	geometry, _ := geometryFor(96)
	groups := map[string][]string{"g1": {"A1", "A2", "B1"}}
	samples := map[string]string{"C1": "other"}
	replicates := map[string]int{"C1": 2}

	gotSamples, gotReplicates, err := ApplyPlateSamples(geometry, groups, samples, replicates, []string{"A2", "A1"}, "S1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if gotSamples["A1"] != "S1" || gotSamples["A2"] != "S1" {
		t.Fatalf("sample mapping: %+v", gotSamples)
	}
	if gotSamples["C1"] != "other" {
		t.Fatalf("other sample mapping lost: %+v", gotSamples)
	}
	if gotReplicates["A1"] != 3 || gotReplicates["A2"] != 4 {
		t.Fatalf("replicates: %+v", gotReplicates)
	}
	if gotReplicates["C1"] != 2 {
		t.Fatalf("other replicate lost: %+v", gotReplicates)
	}
}

func TestApplyPlateSamplesNoReplicateLeavesAbsent(t *testing.T) {
	geometry, _ := geometryFor(96)
	groups := map[string][]string{"g1": {"A1"}}
	samples := map[string]string{}
	replicates := map[string]int{"A1": 7}

	gotSamples, gotReplicates, err := ApplyPlateSamples(geometry, groups, samples, replicates, []string{"A1"}, "S1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if gotSamples["A1"] != "S1" {
		t.Fatalf("sample: %+v", gotSamples)
	}
	if _, ok := gotReplicates["A1"]; ok {
		t.Fatalf("replicate not removed: %+v", gotReplicates)
	}
}

func TestApplyPlateSamplesRejectsInvalid(t *testing.T) {
	geometry, _ := geometryFor(96)
	groups := map[string][]string{"g1": {"A1", "B1"}}
	cases := []struct {
		name       string
		wells      []string
		sample     string
		start      int
		samples    map[string]string
		replicates map[string]int
	}{
		{name: "blank sample", wells: []string{"A1"}, sample: "  ", start: 1},
		{name: "negative replicate", wells: []string{"A1"}, sample: "S1", start: -1},
		{name: "ungrouped well", wells: []string{"C1"}, sample: "S1", start: 1},
		{name: "invalid well", wells: []string{"Z99"}, sample: "S1", start: 1},
		{name: "blank well", wells: []string{""}, sample: "S1", start: 1},
		{name: "duplicate technical replicate", wells: []string{"A1"}, sample: "S1", start: 1,
			samples: map[string]string{"B1": "S1"}, replicates: map[string]int{"B1": 1}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := ApplyPlateSamples(geometry, groups, tt.samples, tt.replicates, tt.wells, tt.sample, tt.start); err == nil {
				t.Fatal("invalid sample assignment accepted")
			}
		})
	}
}

func TestPlateFactorsScopeReplicatesAndBatchLayout(t *testing.T) {
	geometry, _ := geometryFor(96)
	groups := map[string][]string{"g1": {"A1", "A2", "A3", "B1", "B2", "B3"}}
	factors := map[string]map[string]string{
		"靶点基因": {
			"A1": "GAPDH", "A2": "GAPDH", "A3": "GAPDH",
			"B1": "VEGFA", "B2": "VEGFA", "B3": "VEGFA",
		},
	}
	// Assign S1 replicates 1..3 on Row A (GAPDH)
	s, r, err := ApplyPlateSamplesWithFactors(geometry, groups, nil, nil, factors, []string{"A1", "A2", "A3"}, "S1", 1)
	if err != nil {
		t.Fatalf("row A assign: %v", err)
	}
	// Same sample S1 with replicates 1..3 on Row B (VEGFA) must succeed because factor differs!
	s, r, err = ApplyPlateSamplesWithFactors(geometry, groups, s, r, factors, []string{"B1", "B2", "B3"}, "S1", 1)
	if err != nil {
		t.Fatalf("row B orthogonal target assign failed: %v", err)
	}
	if r["B1"] != 1 || r["B3"] != 3 {
		t.Fatalf("expected B1=1, B3=3, got %+v", r)
	}
	// Re-assigning A2 as S1 rep 1 under same GAPDH must fail
	if _, _, err := ApplyPlateSamplesWithFactors(geometry, groups, s, r, factors, []string{"A2"}, "S1", 1); err == nil {
		t.Fatal("duplicate replicate within identical factor level should fail")
	}

	// Batch layout: 2 samples x 3 replicates across 6 wells
	bs, br, err := ApplyPlateBatchSamples(geometry, groups, nil, nil, factors, []string{"A1", "A2", "A3", "B1", "B2", "B3"}, "cDNA-1, cDNA-2", 3)
	if err != nil {
		t.Fatalf("batch layout: %v", err)
	}
	if bs["A1"] != "cDNA-1" || br["A1"] != 1 || bs["A3"] != "cDNA-1" || br["A3"] != 3 ||
		bs["B1"] != "cDNA-2" || br["B1"] != 1 || bs["B3"] != "cDNA-2" || br["B3"] != 3 {
		t.Fatalf("unexpected batch result: samples=%+v reps=%+v", bs, br)
	}

	nextF, err := ApplyPlateFactor(geometry, groups, factors, []string{"A1", "B1"}, "加药浓度", "10 uM")
	if err != nil || nextF["加药浓度"]["A1"] != "10 uM" || nextF["加药浓度"]["B1"] != "10 uM" {
		t.Fatalf("factor apply err=%v nextF=%+v", err, nextF)
	}
}

func TestUnifiedPlateAnnotationOrderIndependenceAndGradients(t *testing.T) {
	baseSrc := "```asterlab\nkind: experiment\nid: exp-unified\nschema: 1\ntitle: 统一孔板标注\nobjective: 验证样本与多维因子顺序无关性\n```\n\n```asterlab\nkind: plate\nid: pl-unified\nschema: 1\nformat: 24\ngroups: {}\n```\n"
	doc, err := Parse(baseSrc)
	if err != nil {
		t.Fatalf("initial parse failed: %v", err)
	}

	v := &View{
		Doc:     doc,
		Mode:    ModeDesign,
		Allowed: map[string]bool{PermDocumentEdit: true},
		OnChange: func(nextSrc string) {
			parsed, pErr := Parse(nextSrc)
			if pErr != nil {
				t.Fatalf("updated AsterML failed Parse: %v\nsource:\n%s", pErr, nextSrc)
			}
			doc = parsed
		},
	}

	pl, ok := doc.PlateByID("pl-unified")
	if !ok {
		t.Fatal("missing plate")
	}
	// Case A: plain treatment/sample label auto-creates group and triplicates 1..3
	if err := v.applyUnifiedPlateInput(pl, "", []string{"B1", "B2", "B3"}, "H2O2-100uM"); err != nil {
		t.Fatalf("apply plain label failed: %v", err)
	}
	v.Doc = doc
	pl, _ = doc.PlateByID("pl-unified")
	if err := v.applyUnifiedPlateInput(pl, "", []string{"B1", "B2", "B3"}, "靶点:GAPDH"); err != nil {
		t.Fatalf("apply factor after sample failed: %v", err)
	}
	v.Doc = doc
	pl, _ = doc.PlateByID("pl-unified")
	if err := v.applyUnifiedPlateInput(pl, "", []string{"B4", "B5", "B6"}, "H2O2-100uM"); err != nil {
		t.Fatalf("apply second block plain label failed: %v", err)
	}
	v.Doc = doc
	pl, _ = doc.PlateByID("pl-unified")
	if err := v.applyUnifiedPlateInput(pl, "", []string{"B4", "B5", "B6"}, "靶点:ACTB"); err != nil {
		t.Fatalf("apply orthogonal factor to second block failed: %v", err)
	}
	v.Doc = doc
	pl, _ = doc.PlateByID("pl-unified")
	if pl.Replicates["B1"] != 1 || pl.Replicates["B3"] != 3 || pl.Replicates["B4"] != 1 || pl.Replicates["B6"] != 3 {
		t.Fatalf("expected independent 1..3 replicates across GAPDH vs ACTB, got %+v", pl.Replicates)
	}

	// Case B: reverse order (factor first, then plain label)
	if err := v.applyUnifiedPlateInput(pl, "", []string{"C1", "C2", "C3"}, "靶点:GAPDH"); err != nil {
		t.Fatalf("reverse order factor first failed: %v", err)
	}
	v.Doc = doc
	pl, _ = doc.PlateByID("pl-unified")
	if err := v.applyUnifiedPlateInput(pl, "", []string{"C1", "C2", "C3"}, "Sample-1"); err != nil {
		t.Fatalf("reverse order label second failed: %v", err)
	}
	v.Doc = doc
	pl, _ = doc.PlateByID("pl-unified")
	if pl.Samples["C2"] != "Sample-1" || pl.Replicates["C2"] != 2 || pl.Factors["靶点"]["C2"] != "GAPDH" {
		t.Fatalf("reverse order state mismatch: samples=%+v reps=%+v factors=%+v", pl.Samples, pl.Replicates, pl.Factors)
	}

	// Case C: serial dilution gradient shorthand across 4 wells
	if err := v.applyUnifiedPlateInput(pl, "", []string{"A1", "A2", "A3", "A4"}, "浓度:100/2 uM"); err != nil {
		t.Fatalf("serial gradient failed: %v", err)
	}
	v.Doc = doc
	pl, _ = doc.PlateByID("pl-unified")
	if pl.Factors["浓度"]["A1"] != "100 uM" || pl.Factors["浓度"]["A2"] != "50 uM" ||
		pl.Factors["浓度"]["A3"] != "25 uM" || pl.Factors["浓度"]["A4"] != "12.5 uM" {
		t.Fatalf("unexpected serial gradient values: %+v", pl.Factors["浓度"])
	}

	// Case D: QC role alias
	if err := v.applyUnifiedPlateInput(pl, "", []string{"D6"}, "NTC"); err != nil {
		t.Fatalf("NTC role failed: %v", err)
	}
	v.Doc = doc
	pl, _ = doc.PlateByID("pl-unified")
	if pl.Roles["D6"] != "ntc" {
		t.Fatalf("expected Roles[D6]=ntc, got %+v", pl.Roles)
	}
}
