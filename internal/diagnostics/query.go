package diagnostics

import "context"

type Refresh func(context.Context) error
type ServiceStatusLookup func(string) (*ServiceStatus, bool)

type Query struct {
	recorder *Recorder
	refresh  Refresh
	service  ServiceStatusLookup
}

func NewQuery(recorder *Recorder, refresh Refresh, service ServiceStatusLookup) *Query {
	return &Query{recorder: recorder, refresh: refresh, service: service}
}

func (q *Query) DiagnosticsSnapshot() DiagSnapshot {
	q.sync()
	snapshot := q.recorder.Snapshot()
	return ProjectDiagnostics(snapshot.Health, snapshot.RecentEvents, snapshot.PendingOperations)
}

func (q *Query) PendingOperations() []OperationSnapshot {
	q.sync()
	return ProjectOperations(q.recorder.PendingOperations())
}

func (q *Query) GetHealthSummary() HealthSnapshot {
	q.sync()
	return ProjectHealth(q.recorder.Health())
}

func (q *Query) ExplainFailure(scope, resourceID string) FailureExplanationSnapshot {
	q.sync()
	var service *ServiceStatus
	if scope == "service" && resourceID != "" && q.service != nil {
		service, _ = q.service(resourceID)
	}
	return ExplainFailure(scope, resourceID, q.recorder.Health(), service)
}

func (q *Query) ListRecentEvents(limit int, cursor string) ([]EventEnvelope, string) {
	q.sync()
	return RecentEventEnvelopes(q.recorder.Snapshot().RecentEvents, limit, cursor)
}

func (q *Query) sync() {
	if q.refresh != nil {
		if err := q.refresh(context.Background()); err != nil {
			q.recorder.RecordEvent("diagnostics", "refresh_failed", "runtime", "runtime truth refresh failed", "diagnostics.refresh_failed", map[string]any{"error": err.Error()})
		}
	}
}
