// Package runs exposes the frozen Run and Design services to MyGo.
package runs

import "context"

// RunService loads a frozen Run and records measurements in it.
//
//mygo:native
type RunService interface {
	// LoadRun returns the current authoritative snapshot of a frozen Run.
	LoadRun(ctx context.Context, runID string) (RunSnapshot, error)
	// SaveReadoutsBatch atomically records a complete set of grid cells.
	SaveReadoutsBatch(ctx context.Context, input BatchReadoutInput) (BatchReadoutResult, error)
	// SaveReadout records measurements and returns the persisted snapshot.
	SaveReadout(ctx context.Context, input ReadoutInput) (RunSnapshot, error)
	// CompleteTask marks a task completed and returns the persisted snapshot.
	CompleteTask(ctx context.Context, input CompleteTaskInput) (RunSnapshot, error)
	// Close releases the database connection.
	Close() error
}

// DesignService manages experiment designs.
//
//mygo:native
type DesignService interface {
	// ListDesigns returns all designs.
	ListDesigns(ctx context.Context) ([]DesignSummary, error)
	// LoadDesign returns the current authoritative design snapshot.
	LoadDesign(ctx context.Context, designID string) (DesignSnapshot, error)
	// SaveDesign revises a design and returns the persisted snapshot.
	SaveDesign(ctx context.Context, input DesignSaveInput) (DesignSnapshot, error)
	// CreateRun creates a Run from a design.
	CreateRun(ctx context.Context, designID string, operationID string) (RunSnapshot, error)
	// Close releases the database connection.
	Close() error
}

type RunSnapshot struct {
	RunID          string        `json:"runId"`
	DesignID       string        `json:"designId"`
	DesignRevision uint64        `json:"designRevision"`
	Revision       uint64        `json:"revision"`
	Title          string        `json:"title"`
	Status         string        `json:"status"`
	Tasks          []RunTask     `json:"tasks"`
	Measurements   []Measurement `json:"measurements"`
	Events         []RunEvent    `json:"events"`
}

type RunTask struct {
	Key               string `json:"key"`
	Title             string `json:"title"`
	Timepoint         string `json:"timepoint"`
	Status            string `json:"status"`
	AssayID           string `json:"assayId"`
	ProtocolVersionID string `json:"protocolVersionId"`
}

type Measurement struct {
	SampleID string  `json:"sampleId"`
	Endpoint string  `json:"endpoint"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
}

type RunEvent struct {
	ID string `json:"id"`
	EventType string `json:"eventType"`
	Content string `json:"content"`
	TaskKey string `json:"taskKey"`
	StepStatus string `json:"stepStatus"`
	Timepoint string `json:"timepoint"`
	BatchID string `json:"batchId"`
	SchemaID string `json:"schemaId"`
	SchemaVersion int `json:"schemaVersion"`
	ResultCells []ResultCell `json:"resultCells"`
}

type ReadoutInput struct {
	RunID             string        `json:"runId"`
	TaskKey           string        `json:"taskKey"`
	Timepoint         string        `json:"timepoint"`
	Measurements      []Measurement `json:"measurements"`
	OperationID       string        `json:"operationId"`
	AssayID           string        `json:"assayId"`
	ProtocolStepPath  string        `json:"protocolStepPath"`
	ProtocolVersionID string        `json:"protocolVersionId"`
}

type ResultCell struct {
	Row int `json:"row"`
	Column int `json:"column"`
	SampleID string `json:"sampleId"`
	Group string `json:"group"`
	Replicate int `json:"replicate"`
	Field string `json:"field"`
	Value *float64 `json:"value"`
	Status string `json:"status"`
}

type BatchReadoutInput struct {
	RunID string `json:"runId"`
	OperationID string `json:"operationId"`
	TaskKey string `json:"taskKey"`
	AssayID string `json:"assayId"`
	Timepoint string `json:"timepoint"`
	ProtocolStepPath string `json:"protocolStepPath"`
	ProtocolVersionID string `json:"protocolVersionId"`
	SchemaID string `json:"schemaId"`
	SchemaVersion int `json:"schemaVersion"`
	PlateID string `json:"plateId,omitempty"`
	Cells []ResultCell `json:"cells"`
	ExpectedRevision uint64 `json:"expectedRevision,omitempty"`
}

type BatchRejection struct {
	Index int `json:"index"`
	Reason string `json:"reason"`
}

type BatchReadoutResult struct {
	RunID string `json:"runId"`
	Revision uint64 `json:"revision"`
	BatchID string `json:"batchId"`
	EventID string `json:"eventId"`
	Accepted int `json:"accepted"`
	Rejected []BatchRejection `json:"rejected"`
}

type CompleteTaskInput struct {
	RunID             string `json:"runId"`
	TaskKey           string `json:"taskKey"`
	OperationID       string `json:"operationId"`
	AssayID           string `json:"assayId"`
	Timepoint         string `json:"timepoint"`
	ProtocolStepPath  string `json:"protocolStepPath"`
	ProtocolVersionID string `json:"protocolVersionId"`
}

type DesignSummary struct {
	DesignID string `json:"designId"`
	Title    string `json:"title"`
	Revision uint64 `json:"revision"`
}

type DesignSnapshot struct {
	DesignID string `json:"designId"`
	Revision uint64 `json:"revision"`
	Title    string `json:"title"`
	Data     string `json:"data"`
}

type DesignSaveInput struct {
	DesignID         string `json:"designId"`
	ExpectedRevision uint64 `json:"expectedRevision"`
	Changes          string `json:"changes"`
	OperationID      string `json:"operationId"`
}
