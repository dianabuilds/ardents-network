// Package durable owns the exclusive Network State root and its physical
// generation, distribution-journal, and closed-profile transactions.
// It stores bounded opaque bytes; state verifies their meaning.
package durable

// Limits are the State verifier's framing bounds applied at the physical root.
type Limits struct {
	EpochBytes         int64
	RecordBytes        int64
	ClosedProfileBytes int64
}
