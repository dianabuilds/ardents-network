package resolution

import (
	"time"

	discoveryrecord "ardents/internal/discovery/record"
)

func Resolve(entries []discoveryrecord.Entry, subject, kind string, now time.Time) (discoveryrecord.Entry, string, bool) {
	for _, item := range entries {
		if item.Record.Subject != subject || item.Record.Kind != kind {
			continue
		}
		if isWithdrawnService(item.Record) {
			return item, "withdrawn", true
		}
		if !item.Record.ExpiresAt.IsZero() && now.After(item.Record.ExpiresAt) {
			return item, "expired", true
		}
		return item, "found", true
	}
	return discoveryrecord.Entry{}, "not_found", false
}

func Count(entries []discoveryrecord.Entry, kind string, now time.Time) int {
	count := 0
	for _, item := range entries {
		if !item.Record.ExpiresAt.IsZero() && now.After(item.Record.ExpiresAt) {
			continue
		}
		if isWithdrawnService(item.Record) && (kind == "" || kind == "service") {
			continue
		}
		if kind == "" || item.Record.Kind == kind {
			count++
		}
	}
	return count
}
