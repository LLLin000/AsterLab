// Package runs exposes the frozen Run and its readout writes to MyGo.
package runs

import "context"

// RunService loads a frozen Run and records measurements in it.
//
//mygo:native
type RunService interface {
	// LoadRun returns the current authoritative snapshot of a frozen Run.
	LoadRun(ctx context.Context, runID string) (RunSnapshot, error)
	// SaveReadout records measurements and returns the persisted snapshot.
	SaveReadout(ctx context.Context, input ReadoutInput) (RunSnapshot, error)
}

type RunSnapshot struct {
	RunID        string        `json:"runId"`
	Title        string        `json:"title"`
	Status       string        `json:"status"`
	Tasks        []RunTask     `json:"tasks"`
	Measurements []Measurement `json:"measurements"`
}

type RunTask struct {
	Key       string `json:"key"`
	Title     string `json:"title"`
	Timepoint string `json:"timepoint"`
	Status    string `json:"status"`
}

type Measurement struct {
	SampleID string  `json:"sampleId"`
	Endpoint string  `json:"endpoint"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
}

type ReadoutInput struct {
	RunID        string        `json:"runId"`
	TaskKey      string        `json:"taskKey"`
	Timepoint    string        `json:"timepoint"`
	Measurements []Measurement `json:"measurements"`
}
