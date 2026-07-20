package recorder

import "ardents/internal/diagnostics/event"

func (r *Recorder) SetMaxEvents(limit int) {
	if limit <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.maxEvents = limit
	if len(r.events) > limit {
		r.events = append([]event.Record(nil), r.events[len(r.events)-limit:]...)
	}
}

func (r *Recorder) SetDetailLevel(level string) {
	if level != "minimal" && level != "standard" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.detailLevel = level
}
