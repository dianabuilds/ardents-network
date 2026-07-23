// Package resolution owns freshness- and trust-aware record and service selection.
// It does not own record persistence or transport routing.
package resolution

import (
	"time"

	discoveryrecord "ardents/internal/discovery/records"
)

func Resolve(entries []discoveryrecord.Entry, subject, kind string, now time.Time) (discoveryrecord.Entry, string, bool) {
	for _, item := range entries {
		if item.Record.Subject() != subject || item.Record.Kind() != kind {
			continue
		}
		if isWithdrawnService(item.Record) {
			return item, "withdrawn", true
		}
		if !recordActiveAt(item.Record, now) {
			return item, "expired", true
		}
		return item, "found", true
	}
	return discoveryrecord.Entry{}, "not_found", false
}

func Count(entries []discoveryrecord.Entry, kind string, now time.Time) int {
	count := 0
	for _, item := range entries {
		if !recordActiveAt(item.Record, now) {
			continue
		}
		if isWithdrawnService(item.Record) && (kind == "" || kind == "service") {
			continue
		}
		if kind == "" || item.Record.Kind() == kind {
			count++
		}
	}
	return count
}

func recordActiveAt(record discoveryrecord.Record, now time.Time) bool {
	if !record.IssuedAt.IsZero() && now.Before(record.IssuedAt) {
		return false
	}
	return record.ExpiresAt.IsZero() || record.ExpiresAt.After(now)
}
