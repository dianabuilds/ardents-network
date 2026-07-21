// Package event owns bounded operational event records.
// It does not own health aggregation or product state.
package event

import "time"

type Record struct {
	Seq        int64          `json:"seq"`
	Time       time.Time      `json:"time"`
	Domain     string         `json:"domain"`
	Type       string         `json:"type"`
	Resource   string         `json:"resource,omitempty"`
	Message    string         `json:"message,omitempty"`
	ReasonCode string         `json:"reason_code,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
}

func Clone(in []Record) []Record {
	if len(in) == 0 {
		return nil
	}
	out := make([]Record, 0, len(in))
	for _, item := range in {
		out = append(out, Record{
			Seq:        item.Seq,
			Time:       item.Time,
			Domain:     item.Domain,
			Type:       item.Type,
			Resource:   item.Resource,
			Message:    item.Message,
			ReasonCode: item.ReasonCode,
			Payload:    Map(item.Payload),
		})
	}
	return out
}

func Append(in []Record, record Record, max int) []Record {
	out := append(in, record)
	if max <= 0 || len(out) <= max {
		return out
	}
	return append([]Record(nil), out[len(out)-max:]...)
}
