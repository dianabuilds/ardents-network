package timeline

import (
	diagapi "ardents/internal/diagnostics/api"
	"ardents/internal/diagnostics/event"
	"ardents/internal/diagnostics/projection"
)

func RecentEvents(in []event.Record, limit int, cursor string) ([]diagapi.EventEnvelope, string) {
	return projection.RecentEventEnvelopes(in, limit, cursor)
}
