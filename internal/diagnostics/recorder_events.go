package diagnostics

import (
	"ardents/internal/diagnostics/event"
	"ardents/internal/diagnostics/operation"
	"time"
)

func (r *Recorder) Add(message string) {
	r.RecordEvent("diagnostics", "note", "", message, "", nil)
}

func (r *Recorder) RecordEvent(domain, eventType, resource, message, reasonCode string, payload map[string]any) event.Record {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seq++
	record := event.Record{
		Seq:        r.seq,
		Time:       time.Now().UTC(),
		Domain:     domain,
		Type:       eventType,
		Resource:   resource,
		Message:    message,
		ReasonCode: reasonCode,
		Payload:    event.CloneMap(payload),
	}
	if r.detailLevel == "minimal" {
		record.Resource = ""
		record.Payload = nil
	}
	r.events = event.Append(r.events, record, r.maxEvents)
	r.persistLocked()
	return record
}

func (r *Recorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

func (r *Recorder) Last(n int) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n <= 0 || len(r.events) == 0 {
		return nil
	}
	if n > len(r.events) {
		n = len(r.events)
	}
	out := make([]string, 0, n)
	for _, item := range r.events[len(r.events)-n:] {
		switch {
		case item.Message != "":
			out = append(out, item.Message)
		case item.ReasonCode != "":
			out = append(out, item.ReasonCode)
		default:
			out = append(out, item.Domain+"."+item.Type)
		}
	}
	return out
}

func (r *Recorder) RecordEventCommand(cmd RecordEventCommand) EventEnvelope {
	record := r.RecordEvent(cmd.Domain, cmd.Type, cmd.Resource, cmd.Message, cmd.ReasonCode, cmd.Payload)
	return projectEventRecord(record)
}

func (r *Recorder) RecordEventCommandDurable(cmd RecordEventCommand) (EventEnvelope, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seq++
	record := event.Record{
		Seq:        r.seq,
		Time:       time.Now().UTC(),
		Domain:     cmd.Domain,
		Type:       cmd.Type,
		Resource:   cmd.Resource,
		Message:    cmd.Message,
		ReasonCode: cmd.ReasonCode,
		Payload:    event.CloneMap(cmd.Payload),
	}
	if r.detailLevel == "minimal" {
		record.Resource = ""
		record.Payload = nil
	}
	r.events = event.Append(r.events, record, r.maxEvents)
	if err := r.saveLocked(); err != nil {
		r.markPersistenceFailureLocked(err)
		return projectEventRecord(record), err
	}
	if r.clearPersistenceFailureLocked(time.Now().UTC()) {
		if err := r.saveLocked(); err != nil {
			r.markPersistenceFailureLocked(err)
			return projectEventRecord(record), err
		}
	}
	return projectEventRecord(record), nil
}

func projectEventRecord(record event.Record) EventEnvelope {
	items := ProjectEvents([]event.Record{record})
	if len(items) == 0 {
		return EventEnvelope{}
	}
	return items[0]
}

func (r *Recorder) BeginOperationCommand(cmd BeginOperationCommand) OperationSnapshot {
	record := r.BeginOperation(cmd.Kind, cmd.Domain, cmd.Resource, cmd.Recoverable, cmd.RecoveryAction)
	items := ProjectOperations([]operation.Record{record})
	if len(items) == 0 {
		return OperationSnapshot{}
	}
	return items[0]
}

func (r *Recorder) CompleteOperationCommand(cmd TransitionOperationCommand) {
	r.CompleteOperation(cmd.ID, cmd.Reason)
}

func (r *Recorder) FailOperationCommand(cmd TransitionOperationCommand) {
	r.FailOperation(cmd.ID, cmd.Reason)
}

func (r *Recorder) RecoverOperationCommand(cmd TransitionOperationCommand) {
	r.RecoverOperation(cmd.ID, cmd.Reason)
}

func (r *Recorder) AbandonOperationCommand(cmd TransitionOperationCommand) {
	r.AbandonOperation(cmd.ID, cmd.Reason)
}

func (r *Recorder) SetPrimaryHealth(cmd SetPrimaryHealthCommand) {
	r.SetPrimary(cmd.State, fromReasonSnapshot(cmd.Reason))
}

func (r *Recorder) SetSubsystemHealth(cmd SetSubsystemHealthCommand) {
	r.SetSubsystem(cmd.Domain, cmd.State, fromReasonSnapshot(cmd.Reason))
}

func fromReasonSnapshot(in *ReasonSnapshot) *Reason {
	if in == nil {
		return nil
	}
	return &Reason{
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

func CloneEventRecords(in []event.Record) []event.Record {
	return event.Clone(in)
}
