package freshness

import discoveryrecord "ardents/internal/discovery/record"

func Score(record discoveryrecord.Record) int64 {
	if !record.IssuedAt.IsZero() {
		return record.IssuedAt.UnixNano()
	}
	if !record.ExpiresAt.IsZero() {
		return record.ExpiresAt.UnixNano()
	}
	return 0
}
