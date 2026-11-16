package systemdproperty

import (
	"bytes"
	"encoding/json"
)

// Execution is a detached observation of systemd's main process clocks and
// exit status. It is not evidence of process identity or joined completion.
type Execution struct {
	StartedRealtime, StartedMonotonic, ExitedRealtime, ExitedMonotonic uint64
	PID                                                                uint32
	Code, Status                                                       int32
}

// ReadExecution checks agreement of the observed command and main-process fields.
func ReadExecution(service Properties) (result Execution, ok bool) {
	value := service["ExecStartEx"]
	var entries [][]json.RawMessage
	if value.Type != "a(sasasttttuii)" || Decode(value.Data, &entries) != nil || len(entries) != 1 || len(entries[0]) != 10 {
		return result, false
	}
	// ExecStartEx initially carries the command's fork clock. systemd replaces
	// it with the main process clock when reaping that same process. Retain the
	// separate ExecMain clocks, which identify the process throughout its life.
	var command Execution
	for index, target := range []any{&command.StartedRealtime, &command.StartedMonotonic, &command.ExitedRealtime, &command.ExitedMonotonic, &command.PID, &command.Code, &command.Status} {
		part := entries[0][index+3]
		if bytes.Equal(bytes.TrimSpace(part), []byte("null")) || Decode(part, target) != nil {
			return Execution{}, false
		}
	}
	for _, field := range []struct {
		name, signature string
		target          any
	}{
		{"ExecMainStartTimestamp", "t", &result.StartedRealtime},
		{"ExecMainStartTimestampMonotonic", "t", &result.StartedMonotonic},
		{"ExecMainExitTimestamp", "t", &result.ExitedRealtime},
		{"ExecMainExitTimestampMonotonic", "t", &result.ExitedMonotonic},
		{"ExecMainPID", "u", &result.PID},
		{"ExecMainCode", "i", &result.Code},
		{"ExecMainStatus", "i", &result.Status},
	} {
		value, present := service[field.name]
		if !present || value.Type != field.signature || bytes.Equal(bytes.TrimSpace(value.Data), []byte("null")) || Decode(value.Data, field.target) != nil {
			return Execution{}, false
		}
	}
	if command.PID != result.PID || command.Code != result.Code || command.Status != result.Status ||
		command.ExitedRealtime != result.ExitedRealtime || command.ExitedMonotonic != result.ExitedMonotonic ||
		(command.StartedRealtime == 0) != (result.StartedRealtime == 0) || (command.StartedMonotonic == 0) != (result.StartedMonotonic == 0) ||
		command.StartedRealtime > result.StartedRealtime || command.StartedMonotonic > result.StartedMonotonic {
		return Execution{}, false
	}
	if result.Code != 0 && (command.StartedRealtime != result.StartedRealtime || command.StartedMonotonic != result.StartedMonotonic) {
		return Execution{}, false
	}
	return result, true
}
