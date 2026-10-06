// Package bootstrap owns finite issuer-bootstrap transport capacity: one duty's
// aggregate output and queued-work limits, per-adjacency live claims and their
// original deadlines. Claims remain held through physical join. These limits
// include prepaid terminal output: its held space reduces the available burst
// until emission or joined disposal, without refunding its debit. Copied claims,
// adjacencies and terminal handles retain their original once-only state.
// grant neither Network authority nor Admission permission, tokens or signing.
package bootstrap
