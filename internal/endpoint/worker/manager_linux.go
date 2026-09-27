//go:build linux

package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Value retains the D-Bus signature: an absent, null or differently
// typed hardening property never becomes a zero-value accepting observation.
type Value struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Properties map[string]Value

func ValidUnit(unit, role string) bool {
	if role != "reader" && role != "publisher" {
		return false
	}
	prefix := OfUnit(unit).prefix() + "-" + role + "@"
	if !strings.HasPrefix(unit, prefix) || !strings.HasSuffix(unit, ".service") {
		return false
	}
	instance := strings.TrimSuffix(strings.TrimPrefix(unit, prefix), ".service")
	parts := strings.Split(instance, "-")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		value, err := strconv.ParseUint(part, 10, 32)
		if err != nil || strconv.FormatUint(value, 10) != part {
			return false
		}
	}
	return parts[1] != "0" && parts[2] != "0"
}

func ReadProperties(ctx context.Context, unit, role string) (Properties, Properties, error) {
	if ctx == nil || !ValidUnit(unit, role) {
		return nil, nil, errors.New("text worker unit identity is invalid")
	}
	var paths []string
	answer, err := managerCall(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager", "GetUnit", "s", unit)
	if err != nil || answer.Type != "o" || json.Unmarshal(answer.Data, &paths) != nil || len(paths) != 1 || !strings.HasPrefix(paths[0], "/org/freedesktop/systemd1/unit/") {
		return nil, nil, errors.New("text worker system manager binding is unavailable")
	}
	var observations [2]Properties
	for index, kind := range []string{"Unit", "Service"} {
		answer, err := managerCall(ctx, paths[0], "org.freedesktop.DBus.Properties", "GetAll", "s", "org.freedesktop.systemd1."+kind)
		var payload []Properties
		if err != nil || answer.Type != "a{sv}" || json.Unmarshal(answer.Data, &payload) != nil || len(payload) != 1 || payload[0] == nil {
			return nil, nil, errors.New("text worker effective properties are unavailable")
		}
		observations[index] = payload[0]
	}
	return observations[0], observations[1], nil
}

// managerCall reaches only the installed system manager on the local system
// bus. It is read-only, bounded and noninteractive; no Application supplies an
// executable, environment, bus address, method, signature or object path.
func managerCall(ctx context.Context, path, iface, method, signature string, arguments ...string) (Value, error) {
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	commandArguments := []string{"--system", "--json=short", "--no-pager", "call",
		"org.freedesktop.systemd1", path, iface, method, signature}
	commandArguments = append(commandArguments, arguments...)
	command := exec.CommandContext(bounded, "/usr/bin/busctl", commandArguments...)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	output := &managerOutput{}
	command.Stdout = output
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		// Keep a bounded, input-free reason while preserving the refusal.
		// The system bus response and stderr are never included in diagnostics.
		if ctx.Err() != nil {
			return Value{}, errors.New("text worker system manager query was cancelled")
		}
		if bounded.Err() != nil {
			return Value{}, errors.New("text worker system manager query timed out")
		}
		return Value{}, errors.New("text worker system manager query failed")
	}
	var answer Value
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&answer); err != nil {
		return Value{}, errors.New("text worker system manager response is invalid")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return Value{}, errors.New("text worker system manager response has trailing data")
	}
	return answer, nil
}

type managerOutput struct{ buffer bytes.Buffer }

func (output *managerOutput) Bytes() []byte { return output.buffer.Bytes() }

func (output *managerOutput) Write(body []byte) (int, error) {
	if len(body) > (512<<10)-output.buffer.Len() {
		return 0, errors.New("system manager response exceeds its bound")
	}
	return output.buffer.Write(body)
}

func (properties Properties) exact(name, signature string, expected any) bool {
	value, ok := properties[name]
	if !ok || value.Type != signature {
		return false
	}
	want, err := json.Marshal(expected)
	if err != nil {
		return false
	}
	var compact bytes.Buffer
	if json.Compact(&compact, value.Data) != nil {
		return false
	}
	return bytes.Equal(compact.Bytes(), want)
}
