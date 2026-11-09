package state

import "sync/atomic"

// StaleGuard drops responses whose revision is older than the latest seen.
type StaleGuard struct {
	latest atomic.Uint64
}

func (g *StaleGuard) Accept(revision uint64) bool {
	for {
		cur := g.latest.Load()
		if revision < cur {
			return false
		}
		if revision == cur {
			// allow equal? For stale guard, ignore strictly older only per contract:
			// incoming.Revision < current.Revision => ignore. Equal handled separately.
			return true
		}
		if g.latest.CompareAndSwap(cur, revision) {
			return true
		}
	}
}

func (g *StaleGuard) Latest() uint64 { return g.latest.Load() }