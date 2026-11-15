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
	// SaveAssetLink persists a validated local asset link for a Run and returns
	// the durable metadata. Only logical root ID and relative path are stored.
	SaveAssetLink(ctx context.Context, input AssetLinkInput) (Asset, error)
	// ListAssetLinks returns all asset links for a Run, including after a
	// fresh service reload.
	ListAssetLinks(ctx context.Context, runID string) ([]Asset, error)
	// CompleteTask marks a task completed and returns the persisted snapshot.
	CompleteTask(ctx context.Context, input CompleteTaskInput) (RunSnapshot, error)
	// WorkflowTree returns Core's read-only hierarchy projection for the
	// Run's frozen AsterDoc. The JSON is the single contract shared with the
	// CLI and must be decoded, never rebuilt in Go.
	WorkflowTree(ctx context.Context, runID string) (string, error)
	// SaveTimer stores one durable timer state for a frozen Run. Core keeps
	// paused time in AccumulatedS and never infers it from process uptime.
	SaveTimer(ctx context.Context, timer TimerRecord) (TimerRecord, error)
	// UpdateTimer applies one lifecycle action (start, pause or advance) at the
	// caller-supplied UTC instant, stores the result and returns it. The caller
	// supplies the clock so the transition is deterministic.
	UpdateTimer(ctx context.Context, input TimerActionInput) (TimerRecord, error)
	// ListTimers returns the Run's timers ordered by task key then timer id.
	ListTimers(ctx context.Context, runID string) ([]TimerRecord, error)
	// ListPantry returns the pantry ledger projection as of the given date.
	// AsOf is a strict YYYY-MM-DD date; empty means today (UTC).
	ListPantry(ctx context.Context, query PantryQuery) (PantryLedger, error)
	// UpsertPantry creates (empty ID) or replaces (existing ID) one ledger row.
	UpsertPantry(ctx context.Context, input PantryUpsert) (PantryItem, error)
	// DeletePantry retires one row. The row stays as a tombstone so its printed
	// label code is never reissued.
	DeletePantry(ctx context.Context, input PantryDelete) (PantryDeleteResult, error)
	// ExportPantryLabels returns the stable short-id manifest for label and
	// printer plugins. It never exposes retail barcode text.
	ExportPantryLabels(ctx context.Context, query PantryQuery) (PantryLabels, error)
	// RecordProjection returns Core's single read-only Record projection for one
	// frozen Run, as the exact CLI JSON text. Decode it in Go; never rebuild the
	// nested readings, timeline or archive members here.
	RecordProjection(ctx context.Context, input RecordProjectionInput) (string, error)
	// NormalizeRuntime recomputes the live mix volume when the actual well count
	// differs from the frozen plan. It writes nothing.
	NormalizeRuntime(ctx context.Context, input RuntimeNormalizeInput) (RuntimeNormalizeOutput, error)
	// SignAndArchiveRun ends a Run's record: a recording state is summarized
	// first, then signed and archived with a manifest fingerprinting every
	// linked asset, every note and the projection itself. Signed and archived
	// states are rejected, never re-signed.
	SignAndArchiveRun(ctx context.Context, input SignAndArchiveInput) (ArchiveState, error)
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
	// GetProtocolVersion returns one immutable version and its expanded read model.
	GetProtocolVersion(ctx context.Context, protocolVersionID string) (ProtocolVersionContent, error)
	// ListProtocolVersions returns all immutable versions owned by one Protocol.
	ListProtocolVersions(ctx context.Context, protocolID string) ([]ProtocolVersionContent, error)
	// PreviewProtocolBlock expands one immutable ProtocolBlock version for
	// read-only Design hierarchy browsing. It never writes or creates a Run.
	PreviewProtocolBlock(ctx context.Context, blockVersionID string) (ProtocolBlockPreview, error)
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
	// WorkflowTree returns Core's read-only hierarchy projection for one
	// design revision; revision 0 means the latest immutable revision.
	WorkflowTree(ctx context.Context, designID string, revision uint64) (string, error)
	// DeriveDesignFromRun derives the next Design revision from one finished
	// Run's measured parameters and timers. It writes nothing: persist
	// DesignJSON through SaveDesign with ExpectedRevision.
	DeriveDesignFromRun(ctx context.Context, input DeriveDesignInput) (DerivedDesign, error)
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
	Key   string `json:"key"`
	Title string `json:"title"`
	// Path is the frozen protocol step path. It is the stable identity used by
	// events; Title is the human step title and must never substitute for it.
	Path              string `json:"path"`
	Timepoint         string `json:"timepoint"`
	Status            string `json:"status"`
	AssayID           string `json:"assayId"`
	ProtocolVersionID string `json:"protocolVersionId"`
	// DesignStepID names the frozen design's workflow node this task executes.
	// Empty means the run predates the link, or the design linked its assay to
	// more than one step: the task is unresolved, never guessed from its title.
	DesignStepID string `json:"designStepId"`
	// Instruction is the frozen protocol step's operator instruction.
	Instruction string `json:"instruction,omitempty"`
	// ParametersJSON is the resolved protocol parameter list, serialized as
	// JSON so the native boundary does not need a second parameter type.
	ParametersJSON string `json:"parametersJson,omitempty"`
	// SourceBlockVersionID is the exact ProtocolBlock version that produced
	// this expanded task. Empty for legacy tasks or text-only steps.
	SourceBlockVersionID string `json:"sourceBlockVersionId,omitempty"`
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
	// Status comes only from a formal Core CheckAck event. Empty means no
	// authoritative ack is known; surfaces must not infer one from free text.
	Status string `json:"status,omitempty"`
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
	// TaskKey and ProtocolStepPath are the authoritative association copied
	// from the event. Empty means the historical measurement is unassociated.
	TaskKey          string `json:"taskKey,omitempty"`
	ProtocolStepPath string `json:"protocolStepPath,omitempty"`
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
	// DataJSON carries the projected event fields (materials, deviation,
	// lineage, measurements, sample_ids, check_ack, ...) without expanding the
	// generated native type for every optional field.
	DataJSON string `json:"dataJson,omitempty"`
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

// ProtocolVersionContent is the read-only expanded content of an immutable
// ProtocolVersion. The expanded arrays cross the module boundary as the exact
// CLI JSON text, the same contract WorkflowTree uses: decode it, never rebuild
// it in Go. Their members carry protocol values that are not bridge types.
type ProtocolVersionContent struct {
	ProtocolVersionID    string   `json:"protocol_version_id"`
	ProtocolID           string   `json:"protocol_id"`
	Version              int      `json:"version"`
	RootBlockVersionID   *string  `json:"root_block_version_id"`
	RootBindings         string   `json:"root_bindings"`
	ParameterSlots       string   `json:"parameter_slots"`
	Steps                string   `json:"steps"`
	ResolvedParameters   string   `json:"resolved_parameters"`
	UnresolvedParameters string   `json:"unresolved_parameters"`
	BlockVersions        []string `json:"block_version_ids"`
	Frozen               bool     `json:"frozen"`
}

// ProtocolBlockStep is one expanded definition step inside a ProtocolBlock
// version. Path is relative to the block root (for example "01-collect").
type ProtocolBlockParameter struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
	Unit  string `json:"unit,omitempty"`
}

type ProtocolBlockStep struct {
	UseID                   string                   `json:"useId"`
	Path                    string                   `json:"path"`
	Title                   string                   `json:"title"`
	Instruction             string                   `json:"instruction"`
	SourceBlockVersionID    string                   `json:"sourceBlockVersionId"`
	Parameters              []ProtocolBlockParameter `json:"parameters,omitempty"`
	UnresolvedParameterRefs []string                 `json:"unresolvedParameterRefs,omitempty"`
}

// ProtocolBlockPreview is a read-only expansion of one immutable block
// version. It is not a Run and carries no execution status.
type ProtocolBlockPreview struct {
	BlockVersionID string              `json:"blockVersionId"`
	Ready          bool                `json:"ready"`
	Steps          []ProtocolBlockStep `json:"steps"`
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
	Design   string           `json:"design"`
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

// Asset is the durable asset reference contract. It mirrors
// internal/assets.Asset so the service can carry the same metadata across
// the native boundary without embedding file bytes.
type Asset struct {
	ID           string    `json:"id"`
	RootID       string    `json:"rootId"`
	RelativePath string    `json:"relativePath"`
	MediaType    string    `json:"mediaType"`
	Size         int64     `json:"size"`
	ModTime      string    `json:"modTime"`
	SHA256       string    `json:"sha256,omitempty"`
	Availability string    `json:"availability"`
	Link         AssetLink `json:"link"`
	LinkedAt     string    `json:"linkedAt"`
	ActorID      string    `json:"actorId,omitempty"`
}

// AssetLink names the experiment object an asset belongs to.
type AssetLink struct {
	RunID    string `json:"runId"`
	TaskKey  string `json:"taskKey"`
	SampleID string `json:"sampleId,omitempty"`
}

// AssetLinkInput carries one asset reference to persist for a Run.
type AssetLinkInput struct {
	Asset Asset `json:"asset"`
}

// TimerRecord is one durable timer state. Mode is countdown, stopwatch or
// window; Status is ready, running, paused or completed. PlannedS is the
// planned duration and is empty for a stopwatch; StartedAtUTC is the running
// interval's UTC anchor and is empty while not running.
type TimerRecord struct {
	TimerID       string  `json:"timerId"`
	RunID         string  `json:"runId"`
	TaskKey       string  `json:"taskKey"`
	PhaseIndex    uint32  `json:"phaseIndex"`
	Mode          string  `json:"mode"`
	PlannedS      *uint64 `json:"plannedS,omitempty"`
	StartedAtUTC  *string `json:"startedAtUtc,omitempty"`
	AccumulatedS  uint64  `json:"accumulatedS"`
	Status        string  `json:"status"`
	RoundIndex    uint32  `json:"roundIndex"`
	RoundCount    *uint32 `json:"roundCount,omitempty"`
	AnchorEventID *string `json:"anchorEventId,omitempty"`
	AnchorTaskKey *string `json:"anchorTaskKey,omitempty"`
}

// TimerActionInput applies one lifecycle action to a timer. Action is start,
// pause or advance (advance completes a countdown round when its planned
// duration has elapsed). NowUTC is a caller-supplied RFC 3339 instant.
type TimerActionInput struct {
	Timer  TimerRecord `json:"timer"`
	Action string      `json:"action"`
	NowUTC string      `json:"nowUtc"`
}

// PantryQuery is the shared read input for the pantry ledger and label export.
// Empty AsOf means today; empty NearExpiryDays means Core's default (14 days).
// IncludeExhausted is a tri-state: nil keeps exhausted rows (Core's default).
type PantryQuery struct {
	ProjectID        string  `json:"projectId,omitempty"`
	AsOf             string  `json:"asOf,omitempty"`
	NearExpiryDays   *uint32 `json:"nearExpiryDays,omitempty"`
	IncludeExhausted *bool   `json:"includeExhausted,omitempty"`
}

// PantryItem is one ledger row with Core's derived expiry evaluation. The empty
// strings and nil pointers mean "not recorded", never a guessed value.
type PantryItem struct {
	ID                  string  `json:"id"`
	LabelCode           string  `json:"labelCode"`
	ProjectID           string  `json:"projectId,omitempty"`
	Name                string  `json:"name"`
	Location            string  `json:"location"`
	PurchasedOn         string  `json:"purchasedOn,omitempty"`
	OpenedOn            string  `json:"openedOn,omitempty"`
	ShelfLifeDays       *uint32 `json:"shelfLifeDays,omitempty"`
	OpenedShelfLifeDays *uint32 `json:"openedShelfLifeDays,omitempty"`
	Exhausted           bool    `json:"exhausted"`
	Revision            int64   `json:"revision"`
	// Status is unopened, opened, near_expiry, expired or exhausted.
	Status string `json:"status"`
	// ExpiresOn is the computed YYYY-MM-DD date; empty means no expiry rule.
	ExpiresOn string `json:"expiresOn,omitempty"`
	// DaysRemaining is negative once overdue; nil means no expiry rule.
	DaysRemaining *int64 `json:"daysRemaining,omitempty"`
	// ExpiryBasis is shelf_life or opened_shelf_life; empty means no rule.
	ExpiryBasis string `json:"expiryBasis,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// PantryLedger is the pantry read model: the pinned evaluation date plus rows.
type PantryLedger struct {
	AsOf           string       `json:"asOf"`
	NearExpiryDays uint32       `json:"nearExpiryDays"`
	Items          []PantryItem `json:"items"`
}

// PantryUpsert creates (empty ID) or replaces (existing ID) one ledger row.
// Omitted optional fields are cleared; ExpectedRevision guards replacement.
type PantryUpsert struct {
	ID                  string  `json:"id,omitempty"`
	ProjectID           string  `json:"projectId,omitempty"`
	Name                string  `json:"name"`
	Location            string  `json:"location"`
	PurchasedOn         string  `json:"purchasedOn,omitempty"`
	OpenedOn            string  `json:"openedOn,omitempty"`
	ShelfLifeDays       *uint32 `json:"shelfLifeDays,omitempty"`
	OpenedShelfLifeDays *uint32 `json:"openedShelfLifeDays,omitempty"`
	Exhausted           bool    `json:"exhausted"`
	ExpectedRevision    *int64  `json:"expectedRevision,omitempty"`
}

// PantryDelete retires one row; ExpectedRevision guards the write.
type PantryDelete struct {
	ID               string `json:"id"`
	ExpectedRevision *int64 `json:"expectedRevision,omitempty"`
}

// PantryDeleteResult confirms the tombstone; LabelCode stays reserved.
type PantryDeleteResult struct {
	ID        string `json:"id"`
	LabelCode string `json:"labelCode"`
	Revision  int64  `json:"revision"`
	Deleted   bool   `json:"deleted"`
}

// PantryLabel is one stable short-id label for an external printer plugin.
type PantryLabel struct {
	LabelCode     string `json:"labelCode"`
	PantryItemID  string `json:"pantryItemId"`
	Name          string `json:"name"`
	Location      string `json:"location"`
	Status        string `json:"status"`
	ExpiresOn     string `json:"expiresOn,omitempty"`
	DaysRemaining *int64 `json:"daysRemaining,omitempty"`
	Revision      int64  `json:"revision"`
}

// PantryLabels is the label manifest. GeneratedAt makes two calls differ; the
// label codes and ids are the stable contract.
type PantryLabels struct {
	SchemaVersion  int           `json:"schemaVersion"`
	GeneratedAt    string        `json:"generatedAt"`
	AsOf           string        `json:"asOf"`
	NearExpiryDays uint32        `json:"nearExpiryDays"`
	Labels         []PantryLabel `json:"labels"`
}

// RecordProjectionInput selects the view and the timer watermark for the
// read-only Record projection. View is blind or unblinded; empty is unblinded.
// FOVPerWell 0 means Core's default of one planned slot per well. NowUTC is
// required to measure a running timer; without it no elapsed time is invented
// and Core reports a timer_watermark_missing issue.
type RecordProjectionInput struct {
	RunID      string `json:"runId"`
	View       string `json:"view,omitempty"`
	FOVPerWell uint32 `json:"fovPerWell"`
	NowUTC     string `json:"nowUtc,omitempty"`
}

// RuntimeNormalizeInput is the frozen plan next to the actual well count.
type RuntimeNormalizeInput struct {
	PlannedWells     uint32  `json:"plannedWells"`
	ActualWells      uint32  `json:"actualWells"`
	VolumePerWellUL  float64 `json:"volumePerWellUl"`
	ExcessRatio      float64 `json:"excessRatio"`
	DeadVolumeUL     float64 `json:"deadVolumeUl"`
	PreparedVolumeUL float64 `json:"preparedVolumeUl"`
}

// RuntimeNormalizeOutput is the recomputed live mix. AdditionalVolumeUL and
// UnusedVolumeUL are both zero when the plan exactly matches the actual plate.
type RuntimeNormalizeOutput struct {
	PlannedVolumeUL    float64 `json:"plannedVolumeUl"`
	RequiredVolumeUL   float64 `json:"requiredVolumeUl"`
	DeadVolumeUL       float64 `json:"deadVolumeUl"`
	AdditionalVolumeUL float64 `json:"additionalVolumeUl"`
	UnusedVolumeUL     float64 `json:"unusedVolumeUl"`
}

// SignAndArchiveInput ends a Run's record. SignedAtUTC is a caller-supplied
// RFC 3339 instant, like every other timestamp; Core has no clock.
type SignAndArchiveInput struct {
	RunID       string `json:"runId"`
	View        string `json:"view,omitempty"`
	FOVPerWell  uint32 `json:"fovPerWell"`
	NowUTC      string `json:"nowUtc,omitempty"`
	SignedBy    string `json:"signedBy"`
	SignedAtUTC string `json:"signedAtUtc"`
	Statement   string `json:"statement,omitempty"`
}

// ArchiveState is the persisted lifecycle of a Run record. Status is recording,
// summarizing, signed or archived. ManifestJSON is the exact manifest text,
// empty until archived; NotesLocked is true once archived.
type ArchiveState struct {
	RunID        string `json:"runId"`
	Status       string `json:"status"`
	SignedBy     string `json:"signedBy,omitempty"`
	SignedAtUTC  string `json:"signedAtUtc,omitempty"`
	Statement    string `json:"statement,omitempty"`
	ManifestJSON string `json:"manifestJson,omitempty"`
	NotesLocked  bool   `json:"notesLocked"`
}

// DeriveDesignInput names the finished Run and optional replacements.
type DeriveDesignInput struct {
	RunID     string `json:"runId"`
	Title     string `json:"title,omitempty"`
	Objective string `json:"objective,omitempty"`
	Note      string `json:"note,omitempty"`
}

// DerivedDesign is the next Design version payload. DesignJSON is the exact
// design data JSON (including the derived_from evidence); persist it through
// SaveDesign with ExpectedRevision. The Run and its DesignVersion are only read.
type DerivedDesign struct {
	DesignID              string `json:"designId"`
	SourceDesignVersionID string `json:"sourceDesignVersionId"`
	SourceDesignRevision  int64  `json:"sourceDesignRevision"`
	ExpectedRevision      int64  `json:"expectedRevision"`
	SourceRunID           string `json:"sourceRunId"`
	DesignJSON            string `json:"designJson"`
	Note                  string `json:"note,omitempty"`
}
