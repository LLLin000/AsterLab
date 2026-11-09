package analysis

import (
	"math"
	"testing"
)

func ptr(v float64) *float64 { return &v }

func TestAnalyzeCCK8HandReference(t *testing.T) {
	cells := []Cell{
		{Group: "control", Value: ptr(2), Status: Raw}, {Group: "control", Value: ptr(4), Status: Raw},
		{Group: "control", Status: Missing}, {Group: "control", Status: Invalid}, {Group: "control", Status: Undetected},
		{Group: "treated", Value: ptr(3), Status: Raw}, {Group: "treated", Value: ptr(6), Status: Raw},
	}
	got, err := AnalyzeCCK8(cells, "control", Provenance{BatchID: "batch-1", EventID: "event-2"})
	if err != nil {
		t.Fatal(err)
	}
	if got.BatchID != "batch-1" || got.EventID != "event-2" || got.ControlGroup != "control" {
		t.Fatalf("provenance/config lost: %+v", got)
	}
	control, treated := got.Groups[0], got.Groups[1]
	if control.Group != "control" || control.N != 2 || control.Missing != 1 || control.Invalid != 1 || control.Undetected != 1 || control.Mean != 3 || math.Abs(control.SD-math.Sqrt(2)) > 1e-12 || control.RelativeActivity != 1 {
		t.Fatalf("control reference mismatch: %+v", control)
	}
	if treated.Group != "treated" || treated.N != 2 || treated.Mean != 4.5 || math.Abs(treated.SD-math.Sqrt(4.5)) > 1e-12 || treated.RelativeActivity != 1.5 {
		t.Fatalf("treated reference mismatch: %+v", treated)
	}
	if *cells[0].Value != 2 {
		t.Fatal("analysis mutated raw input")
	}
}

func TestAnalyzeQPCRHandReference(t *testing.T) {
	cells := []Cell{
		{SampleID: "cal", Gene: "GAPDH", Replicate: 1, Value: ptr(20), Status: Raw}, {SampleID: "cal", Gene: "GAPDH", Replicate: 2, Value: ptr(22), Status: Raw},
		{SampleID: "cal", Gene: "GENE", Replicate: 1, Value: ptr(25), Status: Raw}, {SampleID: "cal", Gene: "GENE", Replicate: 2, Value: ptr(27), Status: Raw},
		{SampleID: "treated", Gene: "GAPDH", Replicate: 1, Value: ptr(20), Status: Raw}, {SampleID: "treated", Gene: "GAPDH", Replicate: 2, Value: ptr(22), Status: Raw},
		{SampleID: "treated", Gene: "GENE", Replicate: 1, Value: ptr(28), Status: Raw}, {SampleID: "treated", Gene: "GENE", Replicate: 2, Value: ptr(30), Status: Raw},
		{SampleID: "treated", Gene: "GENE", Status: Undetected}, {SampleID: "treated", Gene: "GENE", Status: Missing}, {SampleID: "treated", Gene: "GENE", Status: Invalid},
	}
	got, err := AnalyzeQPCR(cells, "GENE", "GAPDH", "cal", Provenance{BatchID: "batch-q", EventID: "event-q"})
	if err != nil {
		t.Fatal(err)
	}
	if got.BatchID != "batch-q" || got.EventID != "event-q" || got.ReferenceGene != "GAPDH" || got.CalibratorSample != "cal" {
		t.Fatalf("provenance/config lost: %+v", got)
	}
	cal, treated := got.Groups[0], got.Groups[1]
	if cal.SampleID != "cal" || cal.TargetCq != 26 || cal.ReferenceCq != 21 || cal.DeltaCq != 5 || cal.DeltaDeltaCq != 0 || cal.FoldChange != 1 {
		t.Fatalf("calibrator reference mismatch: %+v", cal)
	}
	if treated.SampleID != "treated" || treated.TargetCq != 29 || treated.ReferenceCq != 21 || treated.DeltaCq != 8 || treated.CalibratorDeltaCq != 5 || treated.DeltaDeltaCq != 3 || treated.FoldChange != 0.125 {
		t.Fatalf("treated reference mismatch: %+v", treated)
	}
	if treated.TargetReplicates != 2 || treated.ReferenceReplicates != 2 || treated.Missing != 1 || treated.Invalid != 1 || treated.Undetected != 1 {
		t.Fatalf("replicate/status accounting mismatch: %+v", treated)
	}
	if *cells[0].Value != 20 {
		t.Fatal("analysis mutated raw input")
	}
}

func TestAnalyzeRejectsUnusableInputs(t *testing.T) {
	if _, err := AnalyzeCCK8([]Cell{{Group: "control", Status: Missing}}, "control", Provenance{}); err == nil {
		t.Fatal("expected empty control rejection")
	}
	if _, err := AnalyzeQPCR(nil, "G", "R", "cal", Provenance{}); err == nil {
		t.Fatal("expected missing calibrator rejection")
	}
}
