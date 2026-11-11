package asterdoc

import (
	"context"
	"fmt"

	"asterlab-mygo-prototype/modules/runs"
)

// AsterDocLoader is the minimal Core-facing loader surface needed to read one
// exact immutable document revision.
type AsterDocLoader interface {
	LoadAsterDocAt(ctx context.Context, designID string, revision uint64) (runs.AsterDocSnapshot, error)
}

// LoadFrozenAsterDocAt reads one exact immutable revision and verifies its
// source hash before returning it. It never falls back to the latest revision.
func LoadFrozenAsterDocAt(ctx context.Context, loader AsterDocLoader, designID string, revision uint64, expectedHash string) (FrozenAsterDoc, error) {
	if loader == nil {
		return FrozenAsterDoc{}, fmt.Errorf("asterdoc loader is required")
	}
	doc, err := loader.LoadAsterDocAt(ctx, designID, revision)
	if err != nil {
		return FrozenAsterDoc{}, err
	}
	if doc.Revision != revision {
		return FrozenAsterDoc{}, fmt.Errorf("frozen revision %d not found (got %d)", revision, doc.Revision)
	}
	if expectedHash != "" && doc.SourceHash != expectedHash {
		return FrozenAsterDoc{}, fmt.Errorf("frozen source hash mismatch: got %s want %s", doc.SourceHash, expectedHash)
	}
	frozen := FrozenAsterDoc{
		DesignID:        doc.DesignID,
		Revision:        doc.Revision,
		Source:          doc.Source,
		SourceHash:      doc.SourceHash,
		CompilerVersion: doc.CompilerVersion,
	}
	if !frozen.VerifySourceHash(doc.Source) {
		return FrozenAsterDoc{}, fmt.Errorf("frozen source hash verification failed")
	}
	return frozen, nil
}
