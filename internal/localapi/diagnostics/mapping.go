package diagnostics

import (
	diagapi "ardents/internal/diagnostics"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"
)

func toDiagSnapshot(in diagapi.DiagSnapshot) *ardentsv1.DiagSnapshot {
	return &ardentsv1.DiagSnapshot{
		Health:            toHealthSnapshot(in.Health),
		RecentEvents:      toEventEnvelopes(in.RecentEvents),
		PendingOperations: toOperationSnapshots(in.PendingOperations),
	}
}

func DiagSnapshot(in diagapi.DiagSnapshot) *ardentsv1.DiagSnapshot {
	return toDiagSnapshot(in)
}

func toHealthSnapshot(in diagapi.HealthSnapshot) *ardentsv1.HealthSnapshot {
	if in.State == "" && in.PrimaryReason == nil && len(in.Subsystems) == 0 && in.UpdatedAt.IsZero() {
		return nil
	}
	out := &ardentsv1.HealthSnapshot{
		State:                  in.State,
		UpdatedAt:              rpc.Timestamp(in.UpdatedAt),
		OperatorActionRequired: in.OperatorActionRequired,
	}
	if in.PrimaryReason != nil {
		out.PrimaryReason = toReasonSnapshot(*in.PrimaryReason)
	}
	for _, item := range in.Subsystems {
		out.Subsystems = append(out.Subsystems, toSubsystemHealthSnapshot(item))
	}
	return out
}

func HealthSnapshot(in diagapi.HealthSnapshot) *ardentsv1.HealthSnapshot {
	return toHealthSnapshot(in)
}

func toReasonSnapshot(in diagapi.ReasonSnapshot) *ardentsv1.ReasonSnapshot {
	return &ardentsv1.ReasonSnapshot{
		Code: in.Code, Domain: in.Domain, Summary: in.Summary, Detail: in.Detail,
		Impact: in.Impact, Recovery: in.Recovery, OperatorActionRequired: in.OperatorActionRequired, Resource: in.Resource,
	}
}

func toSubsystemHealthSnapshot(in diagapi.SubsystemHealthSnapshot) *ardentsv1.SubsystemHealthSnapshot {
	out := &ardentsv1.SubsystemHealthSnapshot{Domain: in.Domain, State: in.State, UpdatedAt: rpc.Timestamp(in.UpdatedAt)}
	if in.Reason != nil {
		out.Reason = toReasonSnapshot(*in.Reason)
	}
	return out
}

func toEventEnvelope(in diagapi.EventEnvelope) *ardentsv1.EventEnvelope {
	return &ardentsv1.EventEnvelope{Seq: in.Seq, Time: rpc.Timestamp(in.Time), Domain: in.Domain, Type: in.Type, Resource: in.Resource, Payload: rpc.Struct(in.Payload)}
}

func toEventEnvelopes(items []diagapi.EventEnvelope) []*ardentsv1.EventEnvelope {
	out := make([]*ardentsv1.EventEnvelope, 0, len(items))
	for _, item := range items {
		out = append(out, toEventEnvelope(item))
	}
	return out
}

func toOperationSnapshots(items []diagapi.OperationSnapshot) []*ardentsv1.OperationSnapshot {
	out := make([]*ardentsv1.OperationSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, &ardentsv1.OperationSnapshot{
			Id: item.ID, Kind: item.Kind, State: item.State, Domain: item.Domain, Resource: item.Resource, Reason: item.Reason,
			Recoverable: item.Recoverable, RecoveryAction: item.RecoveryAction, StartedAt: rpc.Timestamp(item.StartedAt), UpdatedAt: rpc.Timestamp(item.UpdatedAt), FinishedAt: rpc.TimestampPointer(item.FinishedAt),
		})
	}
	return out
}
