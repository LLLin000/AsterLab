package main

import "testing"

func TestFrozenAssayEndpointsReadsDesign(t *testing.T) {
	got := frozenAssayEndpoints(`{"assays":[{"id":"assay-cck8","endpoint":"OD450","unit":"AU"}]}`)
	if got["assay-cck8"] != [2]string{"OD450", "AU"} {
		t.Fatalf("endpoints = %+v", got)
	}
	if len(frozenAssayEndpoints("not json")) != 0 {
		t.Fatal("invalid design produced endpoints")
	}
}

func TestSDKModulesFromFrozenDesign(t *testing.T) {
	modules := sdkModulesFromDesign(`{"asterlab":{"foundationVersion":1,"sdkVersion":"1.0.0","modules":[{"kind":"dilution","id":"d1","schema":1}]}}`)
	if len(modules) != 1 || modules[0]["kind"] != "dilution" || modules[0]["id"] != "d1" {
		t.Fatalf("modules = %+v", modules)
	}
	if sdkModulesFromDesign(`{"title":"legacy"}`) != nil {
		t.Fatal("legacy design produced SDK modules")
	}
}

func TestStepModuleRefsUsesStableStepID(t *testing.T) {
	data := `{"mygo":{"stepUses":{"step-seed":[{"ref":"dilution-01","role":"calculator"}]}}}`
	refs := stepModuleRefs(data, "step-seed")
	if !refs["dilution-01"] || len(refs) != 1 {
		t.Fatalf("refs = %+v", refs)
	}
	if len(stepModuleRefs(data, "")) != 0 || len(stepModuleRefs(data, "step-other")) != 0 {
		t.Fatal("unbound step received modules")
	}
}
