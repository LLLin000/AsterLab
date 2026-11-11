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
