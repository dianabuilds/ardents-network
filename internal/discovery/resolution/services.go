package resolution

import (
	"time"

	discoveryrecord "ardents/internal/discovery/record"
)

func FindService(entries []discoveryrecord.Entry, serviceType string, now time.Time) []discoveryrecord.Entry {
	out := make([]discoveryrecord.Entry, 0)
	for _, item := range entries {
		if item.Record.Kind != "service" {
			continue
		}
		if isWithdrawnService(item.Record) {
			continue
		}
		if item.Record.Service != serviceType {
			continue
		}
		if !item.Record.ExpiresAt.IsZero() && now.After(item.Record.ExpiresAt) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func isWithdrawnService(record discoveryrecord.Record) bool {
	return record.Kind == "service" && len(record.Endpoints) == 0
}
