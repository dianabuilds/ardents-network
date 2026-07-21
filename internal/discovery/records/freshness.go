// Package records owns validated durable remote records and merge semantics.
// It does not own route selection or local publication.
package records

func Score(record Record) int64 {
	if !record.IssuedAt.IsZero() {
		return record.IssuedAt.UnixNano()
	}
	if !record.ExpiresAt.IsZero() {
		return record.ExpiresAt.UnixNano()
	}
	return 0
}
