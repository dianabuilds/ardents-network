package resolution

import (
	"time"

	discoveryrecord "ardents/internal/discovery/records"
)

func FindService(entries []discoveryrecord.Entry, serviceType string, now time.Time) []discoveryrecord.Entry {
	out, _ := findService(entries, serviceType, now, 0)
	return out
}

func findService(
	entries []discoveryrecord.Entry,
	serviceType string,
	now time.Time,
	endpointLimit int,
) ([]discoveryrecord.Entry, bool) {
	out := make([]discoveryrecord.Entry, 0)
	endpointCount := 0
	for _, item := range entries {
		if !eligibleService(item.Record, serviceType, now) {
			continue
		}
		if endpointLimit > 0 {
			endpoints := item.Record.EndpointList()
			if len(endpoints) > endpointLimit-endpointCount {
				return nil, true
			}
			endpointCount += len(endpoints)
		}
		out = append(out, item)
	}
	return out, false
}

func FindServiceBounded(
	entries []discoveryrecord.Entry,
	serviceType string,
	now time.Time,
	recordLimit int,
	endpointLimit int,
) ([]discoveryrecord.Entry, bool) {
	if recordLimit < 1 || endpointLimit < 1 || len(entries) > recordLimit {
		return nil, true
	}
	return findService(entries, serviceType, now, endpointLimit)
}

func eligibleService(record discoveryrecord.Record, serviceType string, now time.Time) bool {
	return record.Kind() == discoveryrecord.KindService &&
		!isWithdrawnService(record) &&
		record.ServiceType() == serviceType &&
		recordActiveAt(record, now)
}

func isWithdrawnService(record discoveryrecord.Record) bool {
	return record.Kind() == discoveryrecord.KindService && len(record.EndpointList()) == 0
}
