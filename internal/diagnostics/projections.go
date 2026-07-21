package diagnostics

import (
	"strconv"

	"ardents/internal/diagnostics/event"
	"ardents/internal/diagnostics/health"
	"ardents/internal/diagnostics/operation"
)

func ProjectDiagnostics(currentHealth health.Summary, recentEvents []event.Record, pendingOperations []operation.Record) DiagSnapshot {
	return DiagSnapshot{
		Health:            ProjectHealth(currentHealth),
		RecentEvents:      ProjectEvents(recentEvents),
		PendingOperations: ProjectOperations(pendingOperations),
	}
}

func ProjectSnapshot(snapshot Snapshot) DiagSnapshot {
	return ProjectDiagnostics(snapshot.Health, snapshot.RecentEvents, snapshot.PendingOperations)
}

func ProjectHealth(in health.Summary) HealthSnapshot {
	out := HealthSnapshot{
		State:                  in.State,
		PrimaryReason:          ProjectReasonPtr(in.PrimaryReason),
		OperatorActionRequired: in.PrimaryReason != nil && in.PrimaryReason.OperatorActionRequired,
		UpdatedAt:              in.UpdatedAt,
	}
	if len(in.Subsystems) == 0 {
		return out
	}
	out.Subsystems = make([]SubsystemHealthSnapshot, 0, len(in.Subsystems))
	for _, item := range in.Subsystems {
		if item.Reason != nil && item.Reason.OperatorActionRequired {
			out.OperatorActionRequired = true
		}
		out.Subsystems = append(out.Subsystems, SubsystemHealthSnapshot{
			Domain:    item.Domain,
			State:     item.State,
			Reason:    ProjectReasonPtr(item.Reason),
			UpdatedAt: item.UpdatedAt,
		})
	}
	return out
}

func ProjectOperations(in []operation.Record) []OperationSnapshot {
	if len(in) == 0 {
		return nil
	}
	out := make([]OperationSnapshot, 0, len(in))
	for _, item := range in {
		out = append(out, OperationSnapshot{
			ID:             item.ID,
			Kind:           item.Kind,
			State:          item.State,
			Domain:         item.Domain,
			Resource:       item.Resource,
			Reason:         item.Reason,
			Recoverable:    item.Recoverable,
			RecoveryAction: item.RecoveryAction,
			StartedAt:      item.StartedAt,
			UpdatedAt:      item.UpdatedAt,
			FinishedAt:     item.FinishedAt,
		})
	}
	return out
}

func ProjectEvents(in []event.Record) []EventEnvelope {
	if len(in) == 0 {
		return nil
	}
	out := make([]EventEnvelope, 0, len(in))
	for _, item := range in {
		out = append(out, EventEnvelope{
			Seq:      item.Seq,
			Time:     item.Time,
			Domain:   item.Domain,
			Type:     item.Type,
			Resource: item.Resource,
			Payload:  item.Payload,
		})
	}
	return out
}

func ProjectReason(in *health.Reason) ReasonSnapshot {
	return ReasonSnapshot{
		Code:                   in.Code,
		Domain:                 in.Domain,
		Summary:                in.Summary,
		Detail:                 in.Detail,
		Impact:                 in.Impact,
		Recovery:               in.Recovery,
		OperatorActionRequired: in.OperatorActionRequired,
		Resource:               in.Resource,
	}
}

func ProjectReasonPtr(in *health.Reason) *ReasonSnapshot {
	if in == nil {
		return nil
	}
	return new(ProjectReason(in))
}

func FailureExplanation(scope, resourceID, state string, item ReasonSnapshot) FailureExplanationSnapshot {
	nextSteps := []string{"inspect diagnostics for the affected scope"}
	if item.Recovery != "" {
		nextSteps = append(nextSteps, item.Recovery)
	}
	return FailureExplanationSnapshot{
		Scope:      scope,
		ResourceID: resourceID,
		State:      state,
		Reason:     &item,
		Impact:     item.Impact,
		Recovery:   item.Recovery,
		NextSteps:  nextSteps,
	}
}

type ServiceStatus struct {
	Published bool
	Reason    string
}

func ExplainFailure(scope, resourceID string, currentHealth health.Summary, service *ServiceStatus) FailureExplanationSnapshot {
	if scope == "service" && resourceID != "" && service != nil && (!service.Published || service.Reason != "") {
		item := ReasonSnapshot{
			Code:                   "service.publication.unavailable",
			Domain:                 "workload",
			Summary:                fallback(service.Reason, "hosted service is not published"),
			Impact:                 "service is not discoverable",
			Recovery:               "restore runtime backing or clear publication denial",
			OperatorActionRequired: true,
			Resource:               resourceID,
		}
		return FailureExplanation(scope, resourceID, health.Degraded, item)
	}
	for _, subsystem := range currentHealth.Subsystems {
		if subsystem.Domain == scope && subsystem.Reason != nil {
			return FailureExplanation(scope, resourceID, subsystem.State, ProjectReason(subsystem.Reason))
		}
	}
	if currentHealth.PrimaryReason != nil {
		return FailureExplanation(scope, resourceID, currentHealth.State, ProjectReason(currentHealth.PrimaryReason))
	}
	return FailureExplanationSnapshot{
		Scope:      scope,
		ResourceID: resourceID,
		State:      currentHealth.State,
		NextSteps:  []string{"inspect diagnostics and recent events for the affected scope"},
	}
}

func RecentEventEnvelopes(in []event.Record, limit int, cursor string) ([]EventEnvelope, string) {
	items := ProjectEvents(in)
	out := make([]EventEnvelope, 0, len(items))
	after := parseCursor(cursor)
	for _, item := range items {
		if item.Seq <= after {
			continue
		}
		out = append(out, item)
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	next := ""
	if len(out) > 0 {
		next = strconv.FormatInt(out[len(out)-1].Seq, 10)
	}
	return out, next
}

func parseCursor(cursor string) int64 {
	if cursor == "" {
		return 0
	}
	value, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func fallback(primary, secondary string) string {
	if primary != "" {
		return primary
	}
	return secondary
}
