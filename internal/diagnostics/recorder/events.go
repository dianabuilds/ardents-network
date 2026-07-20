package recorder

import (
	"ardents/internal/diagnostics/event"
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
