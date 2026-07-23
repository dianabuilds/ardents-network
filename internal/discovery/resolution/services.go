package resolution

import (
	"time"

	discoveryrecord "ardents/internal/discovery/records"
)

func FindService(entries []discoveryrecord.Entry, serviceType string, now time.Time) []discoveryrecord.Entry {
	out := make([]discoveryrecord.Entry, 0)
	for _, item := range entries {
		if item.Record.Kind() != discoveryrecord.KindService {
			continue
		}
		if isWithdrawnService(item.Record) {
			continue
		}
		if item.Record.ServiceType() != serviceType {
			continue
		}
		if !recordActiveAt(item.Record, now) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func isWithdrawnService(record discoveryrecord.Record) bool {
	return record.Kind() == discoveryrecord.KindService && len(record.EndpointList()) == 0
}
