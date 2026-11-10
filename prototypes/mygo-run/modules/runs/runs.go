// Package runs exposes the frozen Run and Design services to MyGo.
package runs

import "context"

// RunService loads a frozen Run and records measurements in it.
//
//mygo:native
type RunService interface {
	// LoadRun returns the current authoritative snapshot of a frozen Run.
	LoadRun(ctx context.Context, runID string) (RunSnapshot, error)
	// ListRuns returns recent Run summaries in Core's updated order.
	ListRuns(ctx context.Context) ([]RunSummary, error)
	// SaveReadoutsBatch atomically records a complete set of grid cells.
	SaveReadoutsBatch(ctx context.Context, input BatchReadoutInput) (BatchReadoutResult, error)
	// SaveReadout records measurements and returns the persisted snapshot.
	SaveReadout(ctx context.Context, input ReadoutInput) (RunSnapshot, error)
	// SaveNotebook appends the run's Markdown notebook revision. The event is
	// the audit trail; the latest notebook-stage observation is the document.
	SaveNotebook(ctx context.Context, input NotebookInput) (RunSnapshot, error)
	// SaveTaskChecks persists the operator's checklist acknowledgements for one
	// task. Checks belong to the frozen protocol step; Core rejects any check
	// that step does not define.
	SaveTaskChecks(ctx context.Context, input TaskCheckInput) (RunSnapshot, error)
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
	// ListProjects returns the projects known to Core.
	ListProjects(ctx context.Context) ([]ProjectSummary, error)
	// ListProtocols returns the protocol versions available for assay binding.
	ListProtocols(ctx context.Context) ([]ProtocolSummary, error)
	// LoadDesign returns the current authoritative design snapshot.
	LoadDesign(ctx context.Context, designID string) (DesignSnapshot, error)
	// SaveDesign revises a design and returns the persisted snapshot.
	SaveDesign(ctx context.Context, input DesignSaveInput) (DesignSnapshot, error)
	// CreateAsterDocDesign creates a Design and its first immutable AsterDoc
	// revision.
	CreateAsterDocDesign(ctx context.Context, input AsterDocCreateInput) (AsterDocSnapshot, error)
	// SaveAsterDoc appends one immutable AsterDoc revision to Core. Core
	// revalidates the source even when the caller already parsed it.
	SaveAsterDoc(ctx context.Context, input AsterDocSaveInput) (AsterDocSnapshot, error)
	// ApplyAsterDocCommand applies one semantic edit (assign wells, connect
	// steps, ...) and saves the resulting revision through SaveAsterDoc.
	ApplyAsterDocCommand(ctx context.Context, input AsterDocCommandInput) (AsterDocSnapshot, error)
	// LoadAsterDoc returns the latest immutable AsterDoc revision for a design.
	LoadAsterDoc(ctx context.Context, designID string) (AsterDocSnapshot, error)
	// LoadAsterDocAt returns one exact immutable revision; a missing revision
	// is an error, never a fallback to the latest.
	LoadAsterDocAt(ctx context.Context, designID string, revision uint64) (AsterDocSnapshot, error)
	// CompileAsterDoc returns the deterministic Design projection for one
	// exact document revision. It does not write anything.
	CompileAsterDoc(ctx context.Context, designID string, revision uint64) (AsterDocProjection, error)
	// AssessAsterDoc returns Core's derived design readiness.
	AssessAsterDoc(ctx context.Context, designID string, revision uint64) (AsterDocAssessment, error)
	// CreateRunFromAsterDoc freezes one document revision: it writes the
	// derived Design projection, creates the Run, and stores the document
	// version/hash on the Run in one transaction.
	CreateRunFromAsterDoc(ctx context.Context, input AsterDocRunInput) (RunSnapshot, error)
	// CreateRun creates a Run from a design.
	CreateRun(ctx context.Context, designID string, operationID string) (RunSnapshot, error)
	// Close releases the database connection.
	Close() error
}

type RunSnapshot struct {
	RunID          string        `json:"runId"`
	DesignID       string        `json:"designId"`
	DesignRevision uint64        `json:"designRevision"`
	DesignData     string        `json:"designData"`
	Revision       uint64        `json:"revision"`
	Title          string        `json:"title"`
	Status         string        `json:"status"`
	Tasks          []RunTask     `json:"tasks"`
	Measurements   []Measurement `json:"measurements"`
	Events         []RunEvent    `json:"events"`
	// CreationAudit is what Core revalidated and recorded when the Run was
	// frozen. Nil means the Run predates the audit.
	CreationAudit *CreationAudit `json:"creationAudit,omitempty"`
	// AsterDoc identifies the frozen document revision that produced this Run.
	AsterDoc *AsterDocRef `json:"asterDoc,omitempty"`
}

// CreationAudit records the rules Core checked at freeze time, the warnings
// that were present, and who created the Run.
type CreationAudit struct {
	RulesVersion         int      `json:"rulesVersion"`
	DesignRevision       uint64   `json:"designRevision"`
	DesignVersionID      string   `json:"designVersionId"`
	ActorID              string   `json:"actorId"`
	RecordedAt           string   `json:"recordedAt"`
	Decision             string   `json:"decision"`
	AcknowledgedWarnings []string `json:"acknowledgedWarnings"`
	Warnings             []string `json:"warnings"`
	Errors               []string `json:"errors"`
}

type RunTask struct {
	Key               string `json:"key"`
	Title             string `json:"title"`
	Timepoint         string `json:"timepoint"`
	Status            string `json:"status"`
	AssayID           string `json:"assayId"`
	ProtocolVersionID string `json:"protocolVersionId"`
	// DesignStepID names the frozen design's workflow node this task executes.
	// Empty means the run predates the link, or the design linked its assay to
	// more than one step: the task is unresolved, never guessed from its title.
	DesignStepID string `json:"designStepId"`
	// Checks are the frozen protocol step's real checks. Empty means the step
	// defines none; the UI must not invent one.
	Checks []TaskCheck `json:"checks,omitempty"`
}

// TaskCheck is one checklist item defined by the frozen protocol step.
type TaskCheck struct {
	CheckID     string `json:"checkId"`
	Label       string `json:"label"`
	Instruction string `json:"instruction,omitempty"`
	Blocking    bool   `json:"blocking"`
}

// CheckAck is one operator verdict for a TaskCheck.
type CheckAck struct {
	CheckID string `json:"checkId"`
	Status  string `json:"status"`
}

type Measurement struct {
	SampleID string  `json:"sampleId"`
	Endpoint string  `json:"endpoint"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
}

type RunEvent struct {
	ID        string `json:"id"`
	EventType string `json:"eventType"`
	Content   string `json:"content"`
	// OccurredAt is when the event happened, supplied by an instrument or the
	// operator; empty when unknown. RecordedAt is when the system stored it.
	// Old events store only OccurredAt (stamped at creation) and leave
	// RecordedAt empty: surfaces must not invent the missing one.
	OccurredAt string `json:"occurredAt"`
	RecordedAt string `json:"recordedAt"`
	TaskKey    string `json:"taskKey"`
	StepStatus string `json:"stepStatus"`
	Timepoint  string `json:"timepoint"`
	// Stage marks special observations, for example the Run notebook.
	Stage string `json:"stage,omitempty"`
	// ProtocolStepPath, AssayID and TaskKey let surfaces match an event to the
	// task and frozen step it belongs to. Older events may lack them.
	ProtocolStepPath string       `json:"protocolStepPath,omitempty"`
	AssayID          string       `json:"assayId,omitempty"`
	BatchID          string       `json:"batchId"`
	SchemaID         string       `json:"schemaId"`
	SchemaVersion    int          `json:"schemaVersion"`
	ResultCells      []ResultCell `json:"resultCells"`
	Checks           []CheckAck   `json:"checks,omitempty"`
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

type NotebookInput struct {
	RunID       string `json:"runId"`
	Markdown    string `json:"markdown"`
	OperationID string `json:"operationId"`
}

type TaskCheckInput struct {
	RunID             string     `json:"runId"`
	OperationID       string     `json:"operationId"`
	TaskKey           string     `json:"taskKey"`
	AssayID           string     `json:"assayId"`
	Timepoint         string     `json:"timepoint"`
	ProtocolStepPath  string     `json:"protocolStepPath"`
	ProtocolVersionID string     `json:"protocolVersionId"`
	Checks            []CheckAck `json:"checks"`
}

type ResultCell struct {
	Row       int      `json:"row"`
	Column    int      `json:"column"`
	SampleID  string   `json:"sampleId"`
	Group     string   `json:"group"`
	Replicate int      `json:"replicate"`
	Field     string   `json:"field"`
	Value     *float64 `json:"value"`
	Status    string   `json:"status"`
}

type BatchReadoutInput struct {
	RunID             string       `json:"runId"`
	OperationID       string       `json:"operationId"`
	TaskKey           string       `json:"taskKey"`
	AssayID           string       `json:"assayId"`
	Timepoint         string       `json:"timepoint"`
	ProtocolStepPath  string       `json:"protocolStepPath"`
	ProtocolVersionID string       `json:"protocolVersionId"`
	SchemaID          string       `json:"schemaId"`
	SchemaVersion     int          `json:"schemaVersion"`
	PlateID           string       `json:"plateId,omitempty"`
	Cells             []ResultCell `json:"cells"`
	ExpectedRevision  uint64       `json:"expectedRevision,omitempty"`
}

type BatchRejection struct {
	Index  int    `json:"index"`
	Reason string `json:"reason"`
}

type BatchReadoutResult struct {
	RunID    string           `json:"runId"`
	Revision uint64           `json:"revision"`
	BatchID  string           `json:"batchId"`
	EventID  string           `json:"eventId"`
	Accepted int              `json:"accepted"`
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
	// Checks carries the acknowledgements shown in the UI. Core revalidates
	// blocking checks; completion without them is rejected.
	Checks []CheckAck `json:"checks,omitempty"`
}

type ProjectSummary struct {
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
}

type ProtocolSummary struct {
	ProtocolVersionID string `json:"protocolVersionId"`
	ProtocolID        string `json:"protocolId"`
	Title             string `json:"title"`
	Version           int    `json:"version"`
}

type RunSummary struct {
	RunID     string `json:"runId"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	DesignID  string `json:"designId"`
	UpdatedAt string `json:"updatedAt"`
}

type DesignSummary struct {
	DesignID string `json:"designId"`
	Title    string `json:"title"`
	Revision uint64 `json:"revision"`
}

type AsterDocSaveInput struct {
	DesignID         string `json:"designId"`
	Source           string `json:"source"`
	ExpectedRevision uint64 `json:"expectedRevision"`
	OperationID      string `json:"operationId,omitempty"`
	ActorID          string `json:"actorId,omitempty"`
}

type AsterDocCreateInput struct {
	ProjectID   string `json:"projectId"`
	Title       string `json:"title"`
	Source      string `json:"source"`
	OperationID string `json:"operationId,omitempty"`
	ActorID     string `json:"actorId,omitempty"`
}

type AsterDocCommandInput struct {
	DesignID         string `json:"designId"`
	ExpectedRevision uint64 `json:"expectedRevision"`
	OperationID      string `json:"operationId,omitempty"`
	ActorID          string `json:"actorId,omitempty"`
	// Command is a JSON object such as
	// {"type":"assign_wells","moduleId":"plate-d1","group":"g-control","wells":["A1"]}.
	Command string `json:"command"`
}

type AsterDocAssessment struct {
	DesignID   string   `json:"designId"`
	Revision   uint64   `json:"revision"`
	SourceHash string   `json:"sourceHash"`
	State      string   `json:"state"`
	Errors     []string `json:"errors"`
	Warnings   []string `json:"warnings"`
}

type AsterDocRunInput struct {
	DesignID             string   `json:"designId"`
	DocumentRevision     uint64   `json:"documentRevision"`
	OperationID          string   `json:"operationId,omitempty"`
	Title                string   `json:"title,omitempty"`
	AcknowledgedWarnings []string `json:"acknowledgedWarnings,omitempty"`
}

type AsterDocRef struct {
	DesignID        string `json:"designId"`
	Revision        uint64 `json:"revision"`
	SourceHash      string `json:"sourceHash"`
	SchemaVersion   int    `json:"schemaVersion"`
	CompilerVersion string `json:"compilerVersion"`
}

type AsterDocProjection struct {
	Design   string `json:"design"`
	Document AsterDocSnapshot `json:"document"`
}

type AsterDocSnapshot struct {
	DesignID        string `json:"designId"`
	Revision        uint64 `json:"revision"`
	DesignRevision  uint64 `json:"designRevision"`
	Source          string `json:"source"`
	SourceHash      string `json:"sourceHash"`
	SchemaVersion   int    `json:"schemaVersion"`
	CompilerVersion string `json:"compilerVersion"`
	CreatedAt       string `json:"createdAt,omitempty"`
	ActorID         string `json:"actorId,omitempty"`
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
