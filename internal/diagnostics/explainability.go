package diagnostics

import (
	diagapi "ardents/internal/diagnostics/api"
	"ardents/internal/diagnostics/projection"
	"ardents/internal/diagnostics/timeline"
	hostingapi "ardents/internal/hosting/api"
)

func ExplainFailureSnapshot(scope, resourceID string, health HealthSummary, service *hostingapi.HostedServiceStatusSnapshot) diagapi.FailureExplanationSnapshot {
	return projection.ExplainFailure(scope, resourceID, health, service)
}

func RecentEventEnvelopes(in []EventRecord, limit int, cursor string) ([]diagapi.EventEnvelope, string) {
	return timeline.RecentEvents(in, limit, cursor)
}
