// Package analysis contains pure projections over recorded assay cells.
package analysis

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// CellStatus preserves the distinction between absent, invalid, and undetected data.
type CellStatus string

const (
	Raw        CellStatus = "raw"
	Missing    CellStatus = "missing"
	Invalid    CellStatus = "invalid"
	Undetected CellStatus = "undetected"
)

// Cell is an immutable input value. A nil Value is never interpreted as zero.
type Cell struct {
	Group     string
	SampleID  string
	Gene      string
	Replicate int
	Value     *float64
	Status    CellStatus
}

type Provenance struct {
	BatchID string
	EventID string
}

type CCK8Group struct {
	Group            string
	N                int
	Missing          int
	Invalid          int
	Undetected       int
	Mean             float64
	SD               float64
	RelativeActivity float64
}

type CCK8Result struct {
	Provenance
	ControlGroup string
	Groups       []CCK8Group
}

// AnalyzeCCK8 computes sample SD (n-1) and each group's mean/control-mean.
// Missing, invalid, undetected, and non-finite cells are excluded from statistics.
func AnalyzeCCK8(cells []Cell, controlGroup string, source Provenance) (CCK8Result, error) {
	if source.BatchID == "" || source.EventID == "" {
		return CCK8Result{}, errors.New("source batch and event IDs are required")
	}
	if controlGroup == "" {
		return CCK8Result{}, errors.New("control group is required")
	}
	type accum struct {
		values                       []float64
		missing, invalid, undetected int
	}
	groups := make(map[string]*accum)
	for _, c := range cells {
		a := groups[c.Group]
		if a == nil {
			a = &accum{}
			groups[c.Group] = a
		}
		switch c.Status {
		case Missing:
			a.missing++
		case Invalid:
			a.invalid++
		case Undetected:
			a.undetected++
		case Raw:
			if c.Value == nil || math.IsNaN(*c.Value) || math.IsInf(*c.Value, 0) {
				a.invalid++
				continue
			}
			a.values = append(a.values, *c.Value)
		default:
			return CCK8Result{}, fmt.Errorf("unknown cell status %q", c.Status)
		}
	}
	control := groups[controlGroup]
	if control == nil || len(control.values) == 0 {
		return CCK8Result{}, fmt.Errorf("control group %q has no valid measurements", controlGroup)
	}
	controlMean := mean(control.values)
	if controlMean == 0 {
		return CCK8Result{}, errors.New("control mean is zero")
	}
	result := CCK8Result{Provenance: source, ControlGroup: controlGroup}
	for group, a := range groups {
		if len(a.values) == 0 {
			return CCK8Result{}, fmt.Errorf("group %q has no valid measurements", group)
		}
		result.Groups = append(result.Groups, CCK8Group{Group: group, N: len(a.values), Missing: a.missing, Invalid: a.invalid, Undetected: a.undetected, Mean: mean(a.values), SD: sampleSD(a.values), RelativeActivity: mean(a.values) / controlMean})
	}
	sortCCK8(result.Groups)
	return result, nil
}

type QPCRGroup struct {
	SampleID            string
	TargetGene          string
	TargetCq            float64
	ReferenceCq         float64
	DeltaCq             float64
	CalibratorDeltaCq   float64
	DeltaDeltaCq        float64
	FoldChange          float64
	TargetReplicates    int
	ReferenceReplicates int
	Missing             int
	Invalid             int
	Undetected          int
}

type QPCRResult struct {
	Provenance
	ReferenceGene    string
	CalibratorSample string
	Groups           []QPCRGroup
}

// AnalyzeQPCR averages valid technical replicates by sample and gene, then
// computes ΔCq=target-reference, ΔΔCq=sample-calibrator, and 2^-ΔΔCq.
func AnalyzeQPCR(cells []Cell, targetGene, referenceGene, calibratorSample string, source Provenance) (QPCRResult, error) {
	if source.BatchID == "" || source.EventID == "" {
		return QPCRResult{}, errors.New("source batch and event IDs are required")
	}
	if targetGene == "" || referenceGene == "" || targetGene == referenceGene || calibratorSample == "" {
		return QPCRResult{}, errors.New("distinct target/reference genes and a calibrator sample are required")
	}
	type key struct{ sample, gene string }
	type accum struct {
		values                       []float64
		missing, invalid, undetected int
	}
	values := make(map[key]*accum)
	for _, c := range cells {
		k := key{c.SampleID, c.Gene}
		a := values[k]
		if a == nil {
			a = &accum{}
			values[k] = a
		}
		switch c.Status {
		case Missing:
			a.missing++
		case Invalid:
			a.invalid++
		case Undetected:
			a.undetected++
		case Raw:
			if c.Value == nil || math.IsNaN(*c.Value) || math.IsInf(*c.Value, 0) {
				a.invalid++
				continue
			}
			a.values = append(a.values, *c.Value)
		default:
			return QPCRResult{}, fmt.Errorf("unknown cell status %q", c.Status)
		}
	}
	get := func(sample, gene string) (*accum, bool) {
		a, ok := values[key{sample, gene}]
		return a, ok && len(a.values) > 0
	}
	calTarget, okT := get(calibratorSample, targetGene)
	calRef, okR := get(calibratorSample, referenceGene)
	if !okT || !okR {
		return QPCRResult{}, fmt.Errorf("calibrator %q lacks valid target or reference Cq", calibratorSample)
	}
	calDelta := mean(calTarget.values) - mean(calRef.values)
	result := QPCRResult{Provenance: source, ReferenceGene: referenceGene, CalibratorSample: calibratorSample}
	samples := make(map[string]bool)
	for k := range values {
		if k.gene == targetGene {
			samples[k.sample] = true
		}
	}
	for sample := range samples {
		target, hasTarget := get(sample, targetGene)
		ref, hasRef := get(sample, referenceGene)
		if !hasTarget || !hasRef {
			return QPCRResult{}, fmt.Errorf("sample %q lacks valid target or reference Cq", sample)
		}
		delta := mean(target.values) - mean(ref.values)
		dd := delta - calDelta
		counts := func(a *accum) (int, int, int) { return a.missing, a.invalid, a.undetected }
		g := QPCRGroup{SampleID: sample, TargetGene: targetGene, TargetCq: mean(target.values), ReferenceCq: mean(ref.values), DeltaCq: delta, CalibratorDeltaCq: calDelta, DeltaDeltaCq: dd, FoldChange: math.Pow(2, -dd), TargetReplicates: len(target.values), ReferenceReplicates: len(ref.values)}
		g.Missing, g.Invalid, g.Undetected = counts(target)
		m, i, u := counts(ref)
		g.Missing += m
		g.Invalid += i
		g.Undetected += u
		result.Groups = append(result.Groups, g)
	}
	sortQPCR(result.Groups)
	return result, nil
}

func mean(v []float64) float64 {
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}
func sampleSD(v []float64) float64 {
	if len(v) < 2 {
		return 0
	}
	m := mean(v)
	ss := 0.0
	for _, x := range v {
		d := x - m
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(v)-1))
}
func sortCCK8(v []CCK8Group) {
	sort.Slice(v, func(i, j int) bool { return v[i].Group < v[j].Group })
}

func sortQPCR(v []QPCRGroup) {
	sort.Slice(v, func(i, j int) bool { return v[i].SampleID < v[j].SampleID })
}
