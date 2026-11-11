package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/modules/runs"
)

const storeDoc = "# CCK-8\n\n```asterlab\nkind: assay\nid: cck8\nschema: 1\ntitle: CCK-8\n```\n\n```asterlab\nkind: group\nid: control\nschema: 1\nname: Control\nrole: control\n```\n\n```asterlab\nkind: group\nid: material\nschema: 1\nname: Material\nrole: treatment\n```\n\n```asterlab\nkind: plate\nid: plate-d3\nschema: 1\nformat: 96\nassay: cck8\ngroups:\n  control: [A1, A2]\n  material: [B1]\n```\n"
const parityDilutionDoc = "```asterlab\nkind: dilution\nid: dilution-01\nschema: 1\nstock:\n  value: 20\n  unit: mg/mL\ntarget:\n  value: 2\n  unit: mg/mL\nfinal_volume:\n  value: 5\n  unit: mL\n```\n"

const parityInstrumentDoc = "```asterlab\nkind: instrument_profile\nid: instrument-reader\nschema: 1\nclass: plate_reader\nmodel: Synergy H1\ncapabilities:\n  - name: absorbance\n    wavelength: 450 nm\n  - name: fluorescence\nsource: built_in\n```\n"

func TestAsterDocStoreIsImmutableAndConflictChecked(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	designs, err := svc.ListDesigns(ctx)
	if err != nil || len(designs) == 0 {
		t.Fatalf("designs: %v %v", designs, err)
	}
	designID := designs[0].DesignID

	first, err := svc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{
		DesignID: designID, Source: storeDoc, ExpectedRevision: 0,
		OperationID: "asterdoc-save-1", ActorID: "tester",
	})
	if err != nil {
		t.Fatalf("SaveAsterDoc: %v", err)
	}
	if first.Revision != 1 || first.Source != storeDoc || first.SourceHash == "" {
		t.Fatalf("first snapshot = %+v", first)
	}
	if first.CompilerVersion == "" || first.SchemaVersion != 1 {
		t.Fatalf("missing compiler/schema version: %+v", first)
	}

	// Same operation and source is idempotent.
	again, err := svc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{
		DesignID: designID, Source: storeDoc, ExpectedRevision: 0,
		OperationID: "asterdoc-save-1", ActorID: "tester",
	})
	if err != nil || again.Revision != 1 {
		t.Fatalf("idempotent save = %+v, %v", again, err)
	}

	// Stale expected_revision must not overwrite.
	if _, err := svc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{
		DesignID: designID, Source: storeDoc + "\n", ExpectedRevision: 0,
		OperationID: "asterdoc-save-stale",
	}); err == nil {
		t.Fatal("stale AsterDoc save succeeded")
	}

	// Core rejects an unknown module even if a caller skipped Go validation.
	invalid := "```asterlab\nkind: calculator\nid: c1\nschema: 1\n```\n"
	if _, err := svc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{
		DesignID: designID, Source: invalid, ExpectedRevision: 1,
		OperationID: "asterdoc-save-invalid",
	}); err == nil || !strings.Contains(err.Error(), "unsupported asterlab module") {
		t.Fatalf("invalid module error = %v", err)
	}

	// A fresh service reads the same revision after restart.
	svc2, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc2.Close()
	loaded, err := svc2.LoadAsterDoc(ctx, designID)
	if err != nil {
		t.Fatalf("LoadAsterDoc: %v", err)
	}
	if loaded.Revision != 1 || loaded.Source != storeDoc || loaded.SourceHash != first.SourceHash {
		t.Fatalf("reloaded = %+v", loaded)
	}
}

// TestAsterDocGoRustParity pins that the Go renderer parser and Rust Core
// accept and reject the same module sources. The shared spec file is the
// single definition; this test catches drift in either implementation.
func TestAsterDocGoRustParity(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	designs, err := svc.ListDesigns(ctx)
	if err != nil || len(designs) == 0 {
		t.Fatalf("designs: %v %v", designs, err)
	}
	designID := designs[0].DesignID
	samples := []struct {
		name   string
		source string
	}{
		{"valid plate", storeDoc},
		{"valid explicit plate identities", "```asterlab\nkind: group\nid: g1\nschema: 1\nname: Control\n```\n```asterlab\nkind: assay\nid: a1\nschema: 1\ntitle: CCK-8\ngroup_refs: [g1]\ntimepoints: [D1]\n```\n```asterlab\nkind: plate\nid: p1\nschema: 1\nformat: 96\nassay: a1\ngroups:\n  g1: [A1, A2]\nsamples:\n  A1: S1\n  A2: S1\nreplicates:\n  A1: 1\n  A2: 2\n```\n"},
		{"invalid duplicate technical replicate", "```asterlab\nkind: group\nid: g1\nschema: 1\nname: Control\n```\n```asterlab\nkind: assay\nid: a1\nschema: 1\ntitle: CCK-8\ngroup_refs: [g1]\ntimepoints: [D1]\n```\n```asterlab\nkind: plate\nid: p1\nschema: 1\nformat: 96\nassay: a1\ngroups:\n  g1: [A1, A2]\nsamples:\n  A1: S1\n  A2: S1\nreplicates:\n  A1: 1\n  A2: 1\n```\n"},
		{"invalid replicate without sample", "```asterlab\nkind: group\nid: g1\nschema: 1\nname: Control\n```\n```asterlab\nkind: plate\nid: p1\nschema: 1\nformat: 96\ngroups:\n  g1: [A1]\nreplicates:\n  A1: 1\n```\n"},
		{"valid 24 plate", "```asterlab\nkind: group\nid: g1\nschema: 1\nname: A\n```\n```asterlab\nkind: plate\nid: p24\nschema: 1\nformat: 24\ngroups:\n  g1: [A1, B2]\n```\n"},
		{"missing group ref", "```asterlab\nkind: plate\nid: p24ref\nschema: 1\nformat: 24\ngroups:\n  ghost: [A1]\n```\n"},
		{"valid scientific doc", readTestDoc(t)},
		{"invalid well", "```asterlab\nkind: plate\nid: p1\nschema: 1\nformat: 96\ngroups:\n  a: [Z99]\n```\n"},
		{"duplicate well", "```asterlab\nkind: plate\nid: p2\nschema: 1\nformat: 96\ngroups:\n  a: [A1]\n  b: [A1]\n```\n"},
		{"missing groups", "```asterlab\nkind: plate\nid: p3\nschema: 1\nformat: 96\n```\n"},
		{"removed plate96 kind", "```asterlab\nkind: plate96\nid: p-legacy\nschema: 1\nformat: 96\ngroups: {}\n```\n"},
		{"unknown kind", "```asterlab\nkind: calculator\nid: c1\nschema: 1\n```\n"},
		{"duplicate id", "```asterlab\nkind: reference\nid: dup\nschema: 1\nref: a\n```\n```asterlab\nkind: reference\nid: dup\nschema: 1\nref: b\n```\n"},
		{"valid dilution", parityDilutionDoc},
		{"invalid dilution unit", strings.Replace(parityDilutionDoc, "unit: mg/mL", "unit: mg/ml", 1)},
		{"invalid dilution concentration", strings.Replace(parityDilutionDoc, "value: 2\n  unit: mg/mL", "value: 30\n  unit: mg/mL", 1)},
		{"valid instrument profile", parityInstrumentDoc},
		{"invalid instrument profile", strings.Replace(parityInstrumentDoc, "capabilities:\n  - name: absorbance\n    wavelength: 450 nm\n  - name: fluorescence\n", "", 1)},
		{"missing typed reference", "```asterlab\nkind: assay\nid: assay-1\nschema: 1\ntitle: A\ngroup_refs: [missing-group]\n```\n"},
		{"valid step2 uses", "```asterlab\nkind: assay\nid: assay-1\nschema: 1\ntitle: A\n```\n```asterlab\nkind: dilution\nid: dilution-01\nschema: 1\nstock: {value: 20, unit: mg/mL}\ntarget: {value: 2, unit: mg/mL}\nfinal_volume: {value: 5, unit: mL}\n```\n```asterlab\nkind: step\nid: step-uv\nschema: 2\ntitle: UV\nassay: assay-1\npath: root/uv\nuses:\n  - ref: dilution-01\n    role: calculator\n```\n"},
		{"invalid step2 missing use", "```asterlab\nkind: assay\nid: assay-1\nschema: 1\ntitle: A\n```\n```asterlab\nkind: step\nid: step-uv\nschema: 2\ntitle: UV\nassay: assay-1\npath: root/uv\nuses:\n  - ref: missing\n    role: calculator\n```\n"},
		{"invalid step2 role capability", "```asterlab\nkind: assay\nid: assay-1\nschema: 1\ntitle: A\n```\n```asterlab\nkind: dilution\nid: dilution-01\nschema: 1\nstock: {value: 20, unit: mg/mL}\ntarget: {value: 2, unit: mg/mL}\nfinal_volume: {value: 5, unit: mL}\n```\n```asterlab\nkind: step\nid: step-uv\nschema: 2\ntitle: UV\nassay: assay-1\npath: root/uv\nuses:\n  - ref: dilution-01\n    role: instrument\n```\n"},
		{"valid material", "```asterlab\nkind: material\nid: mat-1\nschema: 1\nname: GelMA\nrole: treatment\ncondition: 5%\nconcentration: {value: 5, unit: mg/mL}\nsource: lab_imported\n```\n"},
		{"invalid material role", "```asterlab\nkind: material\nid: mat-2\nschema: 1\nname: GelMA\nrole: \nconcentration: {value: 5, unit: mg/mL}\nsource: lab_imported\n```\n"},
		{"invalid material concentration", "```asterlab\nkind: material\nid: mat-3\nschema: 1\nname: GelMA\nrole: treatment\nconcentration: {value: 5, unit: mg/ml}\nsource: lab_imported\n```\n"},
		{"valid sample", "```asterlab\nkind: group\nid: g1\nschema: 1\nname: G\n```\n```asterlab\nkind: sample\nid: s1\nschema: 1\nsample_id: S1\ngroup_ref: g1\namount: {value: 1, unit: mL}\n```\n"},
		{"invalid sample amount", "```asterlab\nkind: group\nid: g1\nschema: 1\nname: G\n```\n```asterlab\nkind: sample\nid: s2\nschema: 1\nsample_id: S2\ngroup_ref: g1\namount: {value: 1, unit: mg}\n```\n"},
		{"valid container", "```asterlab\nkind: container\nid: c1\nschema: 1\nformat: 96-well\nwells: 96\n```\n"},
		{"invalid container format", "```asterlab\nkind: container\nid: c2\nschema: 1\nformat: \nwells: 96\n```\n"},
		{"valid measurement profile", "```asterlab\nkind: measurement_profile\nid: mp1\nschema: 1\nendpoint: Ct\nunit: Ct\n```\n"},
		{"invalid measurement profile unit", "```asterlab\nkind: measurement_profile\nid: mp2\nschema: 1\nendpoint: Ct\nunit: \n```\n"},
	}
	revision := uint64(0)
	for index, sample := range samples {
		_, goErr := asterdoc.Parse(sample.source)
		_, coreErr := svc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{
			DesignID: designID, Source: sample.source, ExpectedRevision: revision,
			OperationID: "parity-" + string(rune('a'+index)),
		})
		if (goErr == nil) != (coreErr == nil) {
			t.Fatalf("%s: Go err=%v, Core err=%v", sample.name, goErr, coreErr)
		}
		if coreErr == nil {
			revision++
		}
	}
}

func TestLoadAsterDocAtReturnsExactImmutableRevision(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	designs, err := svc.ListDesigns(ctx)
	if err != nil || len(designs) == 0 {
		t.Fatalf("designs: %v %v", designs, err)
	}
	designID := designs[0].DesignID
	firstSource := storeDoc
	secondSource := strings.Replace(storeDoc, "control: [A1, A2]", "control: [A1]", 1)
	if _, err := svc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{DesignID: designID,
		Source: firstSource, ExpectedRevision: 0, OperationID: "rev-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{DesignID: designID,
		Source: secondSource, ExpectedRevision: 1, OperationID: "rev-2"}); err != nil {
		t.Fatal(err)
	}
	one, err := svc.LoadAsterDocAt(ctx, designID, 1)
	if err != nil {
		t.Fatal(err)
	}
	two, err := svc.LoadAsterDocAt(ctx, designID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if one.Source != firstSource || two.Source != secondSource {
		t.Fatalf("exact revision mismatch: v1=%q v2=%q", one.Source, two.Source)
	}
	if _, err := svc.LoadAsterDocAt(ctx, designID, 99); err == nil {
		t.Fatal("missing revision did not fail")
	}
}

func readTestDoc(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("internal/asterdoc/testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
