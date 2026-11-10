package state

import (
	"asterlab-mygo-prototype/modules/runs"
)

// ApplySnapshot applies an incoming snapshot according to the contract revision rule.
func ApplySnapshot(current runs.RunSnapshot, incoming runs.RunSnapshot) (runs.RunSnapshot, error) {
	if incoming.Revision < current.Revision {
		return current, nil
	}
	return incoming, nil
}

// ConflictIfDifferent returns ErrConflict when same revision but different snapshot.
func ConflictIfDifferent(current, incoming runs.RunSnapshot) error {
	if incoming.Revision == current.Revision {
		if current.RunID != incoming.RunID || current.Status != incoming.Status || len(current.Tasks) != len(incoming.Tasks) {
			return ErrConflict
		}
	}
	return nil
}
