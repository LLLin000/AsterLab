package state

import (
	"context"

	"asterlab-mygo-prototype/modules/runs"
)

// FakeRunService is an in-memory RunService for UI tests.
type FakeRunService struct {
	Snapshot runs.RunSnapshot
	Err      error
}

func (f *FakeRunService) ListRuns(ctx context.Context) ([]runs.RunSummary, error) {
	if f.Err != nil {
		return nil, MapError(f.Err)
	}
	return nil, nil
}
func (f *FakeRunService) LoadRun(ctx context.Context, runID string) (runs.RunSnapshot, error) {
	if f.Err != nil {
		return runs.RunSnapshot{}, MapError(f.Err)
	}
	s := f.Snapshot
	if s.RunID == "" {
		s.RunID = runID
	}
	return s, nil
}
func (f *FakeRunService) SaveReadout(ctx context.Context, input runs.ReadoutInput) (runs.RunSnapshot, error) {
	if f.Err != nil {
		return runs.RunSnapshot{}, MapError(f.Err)
	}
	return f.Snapshot, nil
}
func (f *FakeRunService) CompleteTask(ctx context.Context, input runs.CompleteTaskInput) (runs.RunSnapshot, error) {
	if f.Err != nil {
		return runs.RunSnapshot{}, MapError(f.Err)
	}
	return f.Snapshot, nil
}
func (f *FakeRunService) Close() error { return nil }

// FakeDesignService is an in-memory DesignService for UI tests.
type FakeDesignService struct {
	Designs  []runs.DesignSummary
	Snapshot runs.DesignSnapshot
	Run      runs.RunSnapshot
	Err      error
}

func (f *FakeDesignService) ListProjects(ctx context.Context) ([]runs.ProjectSummary, error) {
	if f.Err != nil {
		return nil, MapError(f.Err)
	}
	return nil, nil
}
func (f *FakeDesignService) ListProtocols(ctx context.Context) ([]runs.ProtocolSummary, error) {
	if f.Err != nil {
		return nil, MapError(f.Err)
	}
	return nil, nil
}
func (f *FakeDesignService) ListDesigns(ctx context.Context) ([]runs.DesignSummary, error) {
	if f.Err != nil {
		return nil, MapError(f.Err)
	}
	return f.Designs, nil
}
func (f *FakeDesignService) LoadDesign(ctx context.Context, designID string) (runs.DesignSnapshot, error) {
	if f.Err != nil {
		return runs.DesignSnapshot{}, MapError(f.Err)
	}
	s := f.Snapshot
	if s.DesignID == "" {
		s.DesignID = designID
	}
	return s, nil
}
func (f *FakeDesignService) SaveDesign(ctx context.Context, input runs.DesignSaveInput) (runs.DesignSnapshot, error) {
	if f.Err != nil {
		return runs.DesignSnapshot{}, MapError(f.Err)
	}
	return f.Snapshot, nil
}
func (f *FakeDesignService) CreateAsterDocDesign(ctx context.Context, input runs.AsterDocCreateInput) (runs.AsterDocSnapshot, error) {
	if f.Err != nil {
		return runs.AsterDocSnapshot{}, MapError(f.Err)
	}
	return runs.AsterDocSnapshot{DesignID: "design-1", Revision: 1, Source: input.Source}, nil
}
func (f *FakeDesignService) SaveAsterDoc(ctx context.Context, input runs.AsterDocSaveInput) (runs.AsterDocSnapshot, error) {
	if f.Err != nil {
		return runs.AsterDocSnapshot{}, MapError(f.Err)
	}
	return runs.AsterDocSnapshot{DesignID: input.DesignID, Revision: input.ExpectedRevision + 1, Source: input.Source}, nil
}
func (f *FakeDesignService) LoadAsterDoc(ctx context.Context, designID string) (runs.AsterDocSnapshot, error) {
	if f.Err != nil {
		return runs.AsterDocSnapshot{}, MapError(f.Err)
	}
	return runs.AsterDocSnapshot{DesignID: designID}, nil
}
func (f *FakeDesignService) LoadAsterDocAt(ctx context.Context, designID string, revision uint64) (runs.AsterDocSnapshot, error) {
	if f.Err != nil {
		return runs.AsterDocSnapshot{}, MapError(f.Err)
	}
	return runs.AsterDocSnapshot{DesignID: designID, Revision: revision}, nil
}
func (f *FakeDesignService) ApplyAsterDocCommand(ctx context.Context, input runs.AsterDocCommandInput) (runs.AsterDocSnapshot, error) {
	if f.Err != nil {
		return runs.AsterDocSnapshot{}, MapError(f.Err)
	}
	return runs.AsterDocSnapshot{DesignID: input.DesignID, Revision: input.ExpectedRevision + 1}, nil
}
func (f *FakeDesignService) CompileAsterDoc(ctx context.Context, designID string, revision uint64) (runs.AsterDocProjection, error) {
	if f.Err != nil {
		return runs.AsterDocProjection{}, MapError(f.Err)
	}
	return runs.AsterDocProjection{}, nil
}
func (f *FakeDesignService) AssessAsterDoc(ctx context.Context, designID string, revision uint64) (runs.AsterDocAssessment, error) {
	if f.Err != nil {
		return runs.AsterDocAssessment{}, MapError(f.Err)
	}
	return runs.AsterDocAssessment{DesignID: designID, Revision: revision, State: "ready"}, nil
}
func (f *FakeDesignService) CreateRunFromAsterDoc(ctx context.Context, input runs.AsterDocRunInput) (runs.RunSnapshot, error) {
	if f.Err != nil {
		return runs.RunSnapshot{}, MapError(f.Err)
	}
	return f.Run, nil
}
func (f *FakeDesignService) CreateRun(ctx context.Context, designID string, operationID string) (runs.RunSnapshot, error) {
	if f.Err != nil {
		return runs.RunSnapshot{}, MapError(f.Err)
	}
	return f.Run, nil
}
func (f *FakeDesignService) Close() error { return nil }
