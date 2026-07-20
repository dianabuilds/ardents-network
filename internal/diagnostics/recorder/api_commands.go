package recorder

import (
	diagapi "ardents/internal/diagnostics/api"
	"ardents/internal/diagnostics/event"
	"ardents/internal/diagnostics/operation"
	"ardents/internal/diagnostics/projection"
	"ardents/internal/diagnostics/reason"
)

func (r *Recorder) RecordEventCommand(cmd diagapi.RecordEventCommand) diagapi.EventEnvelope {
	record := r.RecordEvent(cmd.Domain, cmd.Type, cmd.Resource, cmd.Message, cmd.ReasonCode, cmd.Payload)
	items := projection.EventEnvelopes([]event.Record{record})
	if len(items) == 0 {
		return diagapi.EventEnvelope{}
	}
	return items[0]
}

func (r *Recorder) BeginOperationCommand(cmd diagapi.BeginOperationCommand) diagapi.OperationSnapshot {
	record := r.BeginOperation(cmd.Kind, cmd.Domain, cmd.Resource, cmd.Recoverable, cmd.RecoveryAction)
	items := projection.OperationSnapshots([]operation.Record{record})
	if len(items) == 0 {
		return diagapi.OperationSnapshot{}
	}
	return items[0]
}

func (r *Recorder) CompleteOperationCommand(cmd diagapi.TransitionOperationCommand) {
	r.CompleteOperation(cmd.ID, cmd.Reason)
}

func (r *Recorder) FailOperationCommand(cmd diagapi.TransitionOperationCommand) {
	r.FailOperation(cmd.ID, cmd.Reason)
}

func (r *Recorder) RecoverOperationCommand(cmd diagapi.TransitionOperationCommand) {
	r.RecoverOperation(cmd.ID, cmd.Reason)
}

func (r *Recorder) AbandonOperationCommand(cmd diagapi.TransitionOperationCommand) {
	r.AbandonOperation(cmd.ID, cmd.Reason)
}

func (r *Recorder) SetPrimaryHealth(cmd diagapi.SetPrimaryHealthCommand) {
	r.SetPrimary(cmd.State, fromReasonSnapshot(cmd.Reason))
}

func (r *Recorder) SetSubsystemHealth(cmd diagapi.SetSubsystemHealthCommand) {
	r.SetSubsystem(cmd.Domain, cmd.State, fromReasonSnapshot(cmd.Reason))
}

func fromReasonSnapshot(in *diagapi.ReasonSnapshot) *reason.Reason {
	if in == nil {
		return nil
	}
	return &reason.Reason{
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
