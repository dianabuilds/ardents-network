package namelease

import (
	"bytes"
	"errors"
)

// Cache freshness errors. Unexported; tests compare directly.
var (
	errCacheStaleEpoch   = errors.New("name cache entry is stale: epoch_number behind current")
	errCacheStaleDigest  = errors.New("name cache entry is stale: epoch_digest does not match current")
	errCacheMissingProof = errors.New("name cache entry is missing freshness proof")
)

// cacheEntry is a single resolved Target cached at a specific
// authenticated Network Epoch (R-043 cache-bounded set). The
// freshness proof is the pair (EpochNumber, EpochDigest). A cache
// lookup must reject entries whose proof does not match the current
// authenticated epoch; this prevents replay of an old Target after a
// Network Epoch transition.
type cacheEntry struct {
	Name        string
	Target      string
	EpochNumber uint64
	EpochDigest [32]byte
}

// isFresh reports whether the entry is bound to the supplied current
// epoch. The entry is fresh iff its EpochNumber equals currentEpoch
// AND its EpochDigest equals currentDigest. Any other combination
// is a stale-proof, replay attack, or fork; the resolver must fail
// closed (return stale-proof) rather than serve the cached Target.
func (e cacheEntry) isFresh(currentEpoch uint64, currentDigest [32]byte) bool {
	if e.EpochNumber != currentEpoch {
		return false
	}
	return bytes.Equal(e.EpochDigest[:], currentDigest[:])
}

// checkFresh is the fail-closed wrapper. It returns nil if the entry
// is fresh and an unexported error otherwise. Callers in the resolver
// and verifier (R-043) compare against the package's stale-proof
// sentinel; production callers may use errors.Is on the unexported
// sentinel in the same package.
func (e cacheEntry) checkFresh(currentEpoch uint64, currentDigest [32]byte) error {
	if e.EpochNumber == 0 {
		return errCacheMissingProof
	}
	if e.EpochNumber != currentEpoch {
		return errCacheStaleEpoch
	}
	if !bytes.Equal(e.EpochDigest[:], currentDigest[:]) {
		return errCacheStaleDigest
	}
	return nil
}
