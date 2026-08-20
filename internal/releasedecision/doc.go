// Package releasedecision owns the offline-authenticated release decision for
// one exact local platform, environment, and network. It verifies the accepted
// TUF-compatible H3 metadata profile, the exact artifact identity, the
// complete target identity, the consecutive root rotation, the build safety
// and protocol state machines, the protocol/emergency transition gate, and
// the durable non-decreasing release floors. It returns one bounded decision
// and atomically advances its own durable release floors. It does not
// download, install, activate, sign, or expose repository administration,
// delegated targets, multi-repository input, or ambient network/caching.
//
// The package enforces the exact profile frozen in R-049 and ADR-0006:
// one top-level targets role, no delegated targets, fixed bounded metadata
// size and counts, DisableLocalCache with UnsafeLocalMode false, one UTC
// reference time captured at evaluation start, an exclusive owned state
// root, atomic root-and-floor publication, and restart tamper detection.
// Distribution bytes are verified identically in every mode.
package releasedecision
