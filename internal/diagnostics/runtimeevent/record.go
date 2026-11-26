package runtimeevent

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"time"
)

const (
	maxRecordBytes   = 1 << 20
	maxCategoryBytes = 64
	maxStateBytes    = 32
	maxReasonBytes   = 256
	maxPIDDigits     = 10
)

// Record contains only selected detached local facts, never raw input fields.
// At may be absent; JournalAt is separate envelope provenance. Resource and
// Hosting contain only the retained numeric names. Presence is not proof of a
// current producer's measurement coverage. Reason may contain local Node text
// and must not be copied into exported labels or safe monitoring output.
type Record struct {
	Schema, Kind, Owner, Role, Carrier, State, Reason string
	Phase, Outcome, Delivery                          string
	At, JournalAt                                     time.Time
	PID                                               string
	Resource, Hosting                                 map[string]float64
	UnknownCategories                                 uint64
}

// Decode accepts an event object or journal MESSAGE. Envelope keys never supply
// event fields, even when both are present. Unknown formats/kinds and ordinary
// journal text return present=false; malformed known fields return a fixed error
// without echoing input. Missing timestamps remain absent for the caller to
// handle according to its view's completeness contract.
func Decode(raw []byte) (record Record, present bool, err error) {
	if len(raw) > maxRecordBytes {
		return Record{}, false, errors.New("runtime diagnostic record exceeds bound")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return Record{}, false, errors.New("runtime diagnostic input has an invalid JSON record")
	}
	if message, wrapped := fields["MESSAGE"]; wrapped {
		var text string
		if json.Unmarshal(message, &text) != nil {
			return Record{}, false, errors.New("runtime diagnostic has an invalid journal message")
		}
		record.JournalAt = journalTime(stringValue(fields, "__REALTIME_TIMESTAMP"))
		record.PID = journalPID(stringValue(fields, "_PID"))
		var event map[string]json.RawMessage
		if json.Unmarshal([]byte(text), &event) != nil || event == nil {
			return Record{}, false, nil
		}
		raw, fields = []byte(text), event
	}
	if _, declared := fields["schema"]; !declared {
		return decodeCommandReport(raw, fields, record)
	}
	record.Schema = stringValue(fields, "schema")
	var names []string
	switch record.Schema {
	case NodeSchema:
		record.Owner = "node"
		names = []string{"kind", "at", "assignment", "carrier_profile", "state", "reason"}
	case SourceSchema:
		record.Owner = "source"
		names = []string{"kind", "at", "reason", "state", "carrier_profile"}
	case EndpointSchema:
		record.Owner = "endpoint"
		names = []string{"kind", "at", "surface", "failure", "state", "carrier_profile"}
	default:
		return Record{}, false, nil
	}
	for _, name := range names {
		if value, exists := fields[name]; exists {
			var text *string
			if json.Unmarshal(value, &text) != nil || text == nil {
				return Record{}, false, errors.New("runtime diagnostic contains an invalid field type")
			}
		}
	}
	record.Kind = stringValue(fields, "kind")
	if !KnownKind(record.Schema, record.Kind) {
		return Record{}, false, nil
	}
	record.State = stringValue(fields, "state")
	record.Carrier = stringValue(fields, "carrier_profile")
	switch record.Schema {
	case NodeSchema:
		record.Role, record.Reason = stringValue(fields, "assignment"), stringValue(fields, "reason")
	case SourceSchema:
		record.Reason = stringValue(fields, "reason")
		if record.Kind == "source-failed" {
			if !KnownFailure(SourceSchema, record.Reason) {
				return Record{}, false, errors.New("source diagnostic has an invalid reason category")
			}
		} else if record.Reason != "" {
			return Record{}, false, errors.New("source diagnostic has an invalid reason category")
		}
	case EndpointSchema:
		record.Role, record.Reason = stringValue(fields, "surface"), stringValue(fields, "failure")
	}
	if record.Role != "" && !token(record.Role, maxCategoryBytes) ||
		record.Carrier != "" && !token(record.Carrier, maxCategoryBytes) ||
		record.State != "" && !token(record.State, maxStateBytes) || len(record.Reason) > maxReasonBytes {
		return Record{}, false, errors.New("runtime diagnostic contains an invalid category")
	}
	if stamp := stringValue(fields, "at"); stamp != "" {
		record.At, err = time.Parse(time.RFC3339Nano, stamp)
		if err != nil {
			return Record{}, false, errors.New("runtime diagnostic time is invalid")
		}
	}
	if record.State != "" && !KnownState(record.State) {
		record.UnknownCategories++
	}
	if record.Carrier != "" && !KnownCarrier(record.Carrier) {
		record.UnknownCategories++
	}
	if record.Schema != NodeSchema && record.Reason != "" && !KnownFailure(record.Schema, record.Reason) {
		record.UnknownCategories++
	}
	record.Resource = numbers(fields["resource"], ResourceNames())
	var hosting map[string]json.RawMessage
	if json.Unmarshal(fields["hosting"], &hosting) == nil {
		record.Hosting = numbers(hosting["Observation"], HostingNames())
	}
	return record, true, nil
}

func stringValue(fields map[string]json.RawMessage, name string) string {
	var value string
	_ = json.Unmarshal(fields[name], &value)
	return value
}

func token(value string, limit int) bool {
	if value == "" || len(value) > limit {
		return false
	}
	for _, ch := range value {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' || ch == '.' {
			continue
		}
		return false
	}
	return true
}

func journalTime(raw string) time.Time {
	micros, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || micros <= 0 {
		return time.Time{}
	}
	return time.UnixMicro(micros).UTC()
}

func journalPID(raw string) string {
	if raw == "" || len(raw) > maxPIDDigits {
		return ""
	}
	for _, digit := range raw {
		if digit < '0' || digit > '9' {
			return ""
		}
	}
	return raw
}

var resourceFields = []string{
	"rss_bytes", "cpu_usage_usec", "memory_bytes", "go_memory_bytes", "socket_memory_bytes",
	"sockets", "fds", "goroutines", "threads", "timers", "queue_items", "queue_bytes",
	"storage_bytes", "storage_files", "cpu_pressure", "memory_pressure", "io_pressure",
	"high_events", "emergency_events", "admission_active", "admission_accepted", "admission_refused",
}
var hostingFields = []string{"UsedBytes", "ReservedBytes", "RemainingBytes"}

func numbers(raw json.RawMessage, names []string) map[string]float64 {
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	result := make(map[string]float64)
	for _, name := range names {
		var value *float64
		if json.Unmarshal(fields[name], &value) == nil && value != nil && *value >= 0 && !math.IsNaN(*value) && !math.IsInf(*value, 0) {
			result[name] = *value
		}
	}
	return result
}

// ResourceNames returns the detached retained numeric-field catalogue. A name
// being recognized does not establish that a producer measured its value.
func ResourceNames() []string { return append([]string{}, resourceFields...) }

// HostingNames returns detached retained Hosting observation field names.
func HostingNames() []string { return append([]string{}, hostingFields...) }
